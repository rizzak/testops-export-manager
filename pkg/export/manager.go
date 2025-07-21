package export

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"testops-export/pkg/api"
	"testops-export/pkg/config"
	"testops-export/pkg/models"
	"testops-export/pkg/storage"
)

// Manager представляет менеджер экспорта
type Manager struct {
	config *config.Config
	client *api.Client
}

// NewManager создает новый менеджер экспорта
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config: cfg,
		client: api.NewClient(cfg),
	}
}

// PerformExport выполняет экспорт всех групп с повторными попытками
func (m *Manager) PerformExport() {
	log.Println("Начинаем экспорт тесткейсов...")

	successCount := 0
	totalCount := len(m.config.Exports)

	for _, exportConfig := range m.config.Exports {
		if err := m.performExportWithRetry(exportConfig); err != nil {
			log.Printf("❌ %v", err)
		} else {
			successCount++
		}
	}

	// Очищаем старые файлы только если был хотя бы один успешный экспорт
	if successCount > 0 {
		if err := m.cleanupOldExports(); err != nil {
			log.Printf("Ошибка очистки старых файлов: %v", err)
		}
	}

	log.Printf("Экспорт завершен: %d/%d групп успешно", successCount, totalCount)
}

// performExportWithRetry выполняет экспорт с повторными попытками
func (m *Manager) performExportWithRetry(exportConfig models.ExportConfig) error {
	for attempt := 1; attempt <= m.config.MaxRetries; attempt++ {
		log.Printf("Попытка %d/%d экспорта группы %s (ID: %d)...", attempt, m.config.MaxRetries, exportConfig.GroupName, exportConfig.GroupID)

		// Запрашиваем экспорт
		exportResp, err := m.client.RequestExport(exportConfig.GroupID)
		if err != nil {
			log.Printf("Ошибка запроса экспорта для группы %s (попытка %d/%d): %v", exportConfig.GroupName, attempt, m.config.MaxRetries, err)

			if attempt < m.config.MaxRetries {
				delay := time.Duration(attempt) * m.config.RetryDelay
				log.Printf("Повторная попытка через %v...", delay)
				time.Sleep(delay)
				continue
			}
			return fmt.Errorf("экспорт группы %s не удался после %d попыток: %v", exportConfig.GroupName, m.config.MaxRetries, err)
		}

		log.Printf("Экспорт для группы %s запрошен, ID: %d", exportConfig.GroupName, exportResp.ID)

		// Ждем немного для обработки
		time.Sleep(5 * time.Second)

		// Скачиваем экспорт
		data, err := m.client.DownloadExport(exportResp.ID)
		if err != nil {
			log.Printf("Ошибка скачивания экспорта для группы %s (попытка %d/%d): %v", exportConfig.GroupName, attempt, m.config.MaxRetries, err)

			if attempt < m.config.MaxRetries {
				delay := time.Duration(attempt) * m.config.RetryDelay
				log.Printf("Повторная попытка через %v...", delay)
				time.Sleep(delay)
				continue
			}
			return fmt.Errorf("скачивание экспорта группы %s не удалось после %d попыток: %v", exportConfig.GroupName, m.config.MaxRetries, err)
		}

		// Сохраняем файл
		if err := m.saveExport(data, exportConfig.GroupName); err != nil {
			log.Printf("Ошибка сохранения экспорта для группы %s (попытка %d/%d): %v", exportConfig.GroupName, attempt, m.config.MaxRetries, err)

			if attempt < m.config.MaxRetries {
				delay := time.Duration(attempt) * m.config.RetryDelay
				log.Printf("Повторная попытка через %v...", delay)
				time.Sleep(delay)
				continue
			}
			return fmt.Errorf("сохранение экспорта группы %s не удалось после %d попыток: %v", exportConfig.GroupName, m.config.MaxRetries, err)
		}

		log.Printf("Экспорт для группы %s завершен успешно (попытка %d/%d)", exportConfig.GroupName, attempt, m.config.MaxRetries)
		return nil
	}

	return fmt.Errorf("экспорт группы %s не удался после %d попыток", exportConfig.GroupName, m.config.MaxRetries)
}

// saveExport сохраняет экспорт в файл
func (m *Manager) saveExport(data []byte, groupName string) error {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("testops_export_%s_%s.csv", groupName, timestamp)
	filepathOnDisk := filepath.Join(m.config.ExportPath, filename)

	if m.config.S3Enabled {
		s3storage, err := storage.NewS3Storage(m.config)
		if err != nil {
			log.Printf("Ошибка инициализации S3: %v", err)
			return err
		}
		err = s3storage.SaveFile(data, filename)
		if err != nil {
			log.Printf("Ошибка сохранения в S3: %v", err)
			return err
		}
		return nil
	}

	// Если S3 не включён — сохраняем локально
	if err := os.MkdirAll(m.config.ExportPath, 0755); err != nil {
		return fmt.Errorf("ошибка создания директории: %v", err)
	}
	if err := os.WriteFile(filepathOnDisk, data, 0644); err != nil {
		return fmt.Errorf("ошибка сохранения файла: %v", err)
	}
	log.Printf("Экспорт %s сохранен локально: %s", groupName, filepathOnDisk)
	return nil
}

// GetExportFiles возвращает список файлов экспорта
func (m *Manager) GetExportFiles() ([]models.ExportFile, error) {
	if m.config.S3Enabled {
		s3storage, err := storage.NewS3Storage(m.config)
		if err != nil {
			return nil, err
		}
		return s3storage.ListFiles()
	}
	files, err := os.ReadDir(m.config.ExportPath)
	if err != nil {
		return nil, err
	}

	var exportFiles []models.ExportFile
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".csv") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		exportFile := models.ExportFile{
			Name:          file.Name(),
			Size:          info.Size(),
			ModifiedTime:  info.ModTime(),
			FormattedSize: m.FormatFileSize(info.Size()),
			FormattedDate: info.ModTime().Format("02.01.2006 15:04:05"),
		}
		exportFiles = append(exportFiles, exportFile)
	}

	// Сортируем по дате изменения (новые сверху)
	sort.Slice(exportFiles, func(i, j int) bool {
		return exportFiles[i].ModifiedTime.After(exportFiles[j].ModifiedTime)
	})

	return exportFiles, nil
}

// cleanupOldExports удаляет файлы старше месяца
func (m *Manager) cleanupOldExports() error {
	if m.config.S3Enabled {
		s3storage, err := storage.NewS3Storage(m.config)
		if err != nil {
			return fmt.Errorf("ошибка инициализации S3: %v", err)
		}
		return s3storage.CleanupOldFiles()
	}
	// Локальный режим
	files, err := os.ReadDir(m.config.ExportPath)
	if err != nil {
		return fmt.Errorf("ошибка чтения директории: %v", err)
	}

	monthAgo := time.Now().AddDate(0, -1, 0)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(monthAgo) {
			filepath := filepath.Join(m.config.ExportPath, file.Name())
			if err := os.Remove(filepath); err != nil {
				log.Printf("Ошибка удаления старого файла %s: %v", filepath, err)
			} else {
				log.Printf("Удален старый файл: %s", filepath)
			}
		}
	}

	return nil
}

// FormatFileSize форматирует размер файла в читаемый вид
func (m *Manager) FormatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// DownloadExportFile возвращает содержимое файла экспорта
func (m *Manager) DownloadExportFile(filename string) ([]byte, error) {
	if m.config.S3Enabled {
		s3storage, err := storage.NewS3Storage(m.config)
		if err != nil {
			return nil, err
		}
		return s3storage.GetFile(filename)
	}
	// Локальный режим
	filePath := filepath.Join(m.config.ExportPath, filename)
	return os.ReadFile(filePath)
}

// DeleteExportFile удаляет файл экспорта
func (m *Manager) DeleteExportFile(filename string) error {
	if m.config.S3Enabled {
		s3storage, err := storage.NewS3Storage(m.config)
		if err != nil {
			return err
		}
		return s3storage.DeleteFile(filename)
	}
	// Локальный режим
	filePath := filepath.Join(m.config.ExportPath, filename)
	return os.Remove(filePath)
}
