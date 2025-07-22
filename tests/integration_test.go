package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"testops-export/pkg/config"
	"testops-export/pkg/export"
	"testops-export/pkg/models"
	"testops-export/pkg/web"

	"github.com/joho/godotenv"
)

// TestIntegrationExport выполняет полный интеграционный тест экспорта (60 секунд)
func TestIntegrationExport(t *testing.T) {
	// 1. Проверяем, что экспортов в директории нет
	exportPath := "../test_exports" // Путь относительно папки tests/
	cleanupTestDir(t, exportPath)

	// Загружаем токен из .env файла (путь относительно папки tests/)
	if err := godotenv.Load("../.env"); err != nil {
		t.Logf("Файл .env не найден: %v", err)
	}

	// Создаем тестовую конфигурацию
	cfg := &config.Config{
		BaseURL:      os.Getenv("TESTOPS_BASE_URL"), // URL из .env файла
		Token:        os.Getenv("TESTOPS_TOKEN"),    // Токен из .env файла
		ExportPath:   exportPath,
		WebPort:      "9091", // Используем другой порт для тестов
		MaxRetries:   3,
		RetryDelay:   5 * time.Second,
		CronSchedule: "0 7 * * *",
		Projects: []models.ProjectConfig{
			{
				ProjectID: 17,
				TreeID:    937,
				Groups: []models.ExportGroupConfig{
					{GroupID: 26961091, GroupName: "API"},
				},
			},
			{
				ProjectID: 15,
				TreeID:    868,
				Groups: []models.ExportGroupConfig{
					{GroupID: 21360405, GroupName: "Seller-Analitics"},
				},
			},
		},
	}

	if cfg.Token == "" {
		t.Skip("TESTOPS_TOKEN не установлен в .env файле, пропускаем тест")
	}

	// Запускаем сервер в горутине
	server := startTestServer(t, cfg)
	defer server.Close()

	// 2. Ждем 10 секунд
	t.Log("Ждем 10 секунд...")
	time.Sleep(10 * time.Second)

	// 3. Запрашиваем веб-страницу
	t.Log("Запрашиваем веб-страницу...")
	resp, err := http.Get(fmt.Sprintf("http://localhost:%s/", cfg.WebPort))
	if err != nil {
		t.Fatalf("Ошибка запроса веб-страницы: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Неожиданный статус ответа: %d", resp.StatusCode)
	}

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Ошибка чтения ответа: %v", err)
	}

	// 4. Проверяем, что новых экспортов не появилось
	filesBefore := countFilesInDir(t, exportPath)
	if filesBefore > 0 {
		t.Errorf("Ожидалось 0 файлов, найдено: %d", filesBefore)
	}

	// 5. Вызываем /export и проверяем количество экспортов
	t.Log("Вызываем /export...")
	exportResp, err := http.Post(fmt.Sprintf("http://localhost:%s/export", cfg.WebPort), "application/json", nil)
	if err != nil {
		t.Fatalf("Ошибка запроса экспорта: %v", err)
	}
	defer exportResp.Body.Close()

	if exportResp.StatusCode != http.StatusOK {
		t.Fatalf("Неожиданный статус ответа экспорта: %d", exportResp.StatusCode)
	}

	// Ждем завершения экспорта (может занять время)
	t.Log("Ждем завершения экспорта...")
	time.Sleep(45 * time.Second) // Уменьшаем время ожидания

	// 6. Проверяем, что в директории появились экспорты
	filesAfter := countFilesInDir(t, exportPath)
	expectedFiles := 0
	for _, project := range cfg.Projects {
		expectedFiles += len(project.Groups)
	}
	if filesAfter != expectedFiles {
		t.Errorf("Ожидалось %d файлов (по количеству групп), найдено: %d", expectedFiles, filesAfter)
	}

	// 7. Проверяем, что количество файлов на странице совпадает с количеством групп
	t.Log("Проверяем количество файлов на странице...")
	resp2, err := http.Get(fmt.Sprintf("http://localhost:%s/", cfg.WebPort))
	if err != nil {
		t.Fatalf("Ошибка запроса веб-страницы: %v", err)
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		t.Fatalf("Ошибка чтения ответа: %v", err)
	}

	// Парсим HTML и проверяем количество файлов
	htmlContent := string(body2)
	pageFilesCount := extractFilesCountFromHTML(htmlContent)

	// Отладочная информация
	t.Logf("Количество групп в конфигурации: %d", expectedFiles)
	t.Logf("Файлов в директории: %d", filesAfter)
	t.Logf("Файлов на странице: %d", pageFilesCount)

	if pageFilesCount != expectedFiles {
		t.Errorf("На странице показано %d файлов, ожидалось %d (по количеству групп)", pageFilesCount, expectedFiles)
		// Выводим часть HTML для отладки
		t.Logf("HTML фрагмент (первые 500 символов): %s", htmlContent[:min(500, len(htmlContent))])
	}

	// 8. Проверяем скачивание файлов
	t.Log("Проверяем скачивание файлов...")
	files, err := os.ReadDir(exportPath)
	if err != nil {
		t.Fatalf("Ошибка чтения директории: %v", err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".csv") {
			continue
		}

		// Скачиваем файл
		downloadURL := fmt.Sprintf("http://localhost:%s/download/%s", cfg.WebPort, file.Name())
		downloadResp, err := http.Get(downloadURL)
		if err != nil {
			t.Fatalf("Ошибка скачивания файла %s: %v", file.Name(), err)
		}
		defer downloadResp.Body.Close()

		if downloadResp.StatusCode != http.StatusOK {
			t.Errorf("Неожиданный статус скачивания файла %s: %d", file.Name(), downloadResp.StatusCode)
			continue
		}

		// 9. Проверяем, что файл CSV и не пустой
		content, err := io.ReadAll(downloadResp.Body)
		if err != nil {
			t.Fatalf("Ошибка чтения содержимого файла %s: %v", file.Name(), err)
		}

		if len(content) == 0 {
			t.Errorf("Файл %s пустой", file.Name())
		}

		// Проверяем, что это CSV файл (содержит запятые или точки с запятой)
		contentStr := string(content)
		if !strings.Contains(contentStr, ",") && !strings.Contains(contentStr, ";") {
			t.Errorf("Файл %s не является CSV (не содержит разделителей)", file.Name())
		}

		t.Logf("Файл %s успешно скачан, размер: %d байт", file.Name(), len(content))
	}

	t.Log("Интеграционный тест завершен успешно")

	// Очищаем тестовую директорию после теста
	cleanupTestDir(t, exportPath)
}

// min возвращает минимальное из двух чисел
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// startTestServer запускает тестовый сервер
func startTestServer(t *testing.T, cfg *config.Config) *http.Server {
	// Создаем веб-сервер
	manager := export.NewManager(cfg)
	server := web.NewServer(manager)

	// Создаем HTTP сервер для правильного завершения
	httpServer := &http.Server{
		Addr: ":" + cfg.WebPort,
	}

	// Запускаем сервер в горутине
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("Ошибка запуска сервера: %v", err)
		}
	}()

	// Ждем запуска сервера
	time.Sleep(2 * time.Second)

	return httpServer
}

// cleanupTestDir очищает тестовую директорию
func cleanupTestDir(t *testing.T, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("Ошибка удаления директории %s: %v", dir, err)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Ошибка создания директории %s: %v", dir, err)
	}
}

// countFilesInDir подсчитывает количество файлов в директории
func countFilesInDir(t *testing.T, dir string) int {
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("Ошибка чтения директории %s: %v", dir, err)
	}

	count := 0
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".csv") {
			count++
		}
	}
	return count
}

// extractFilesCountFromHTML извлекает количество файлов из HTML
func extractFilesCountFromHTML(html string) int {
	// Ищем количество файлов в статистике
	// Ищем паттерн: <div class="stat-number">X</div> где X - количество файлов

	// Ищем строки с class="stat-number"
	lines := strings.Split(html, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, `class="stat-number"`) {
			// Извлекаем число из строки вида: <div class="stat-number">2</div>
			start := strings.Index(line, ">")
			end := strings.Index(line, "</div>")
			if start != -1 && end != -1 && start < end {
				countStr := strings.TrimSpace(line[start+1 : end])
				var count int
				if _, err := fmt.Sscanf(countStr, "%d", &count); err == nil {
					return count
				}
			}
		}
	}

	return 0
}
