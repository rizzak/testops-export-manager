package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"
	"time"

	"testops-export/pkg/config"
	"testops-export/pkg/models"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Storage представляет S3 хранилище
type S3Storage struct {
	client *s3.Client
	bucket string
	config *config.Config
}

// NewS3Storage создает новый экземпляр S3 хранилища
func NewS3Storage(cfg *config.Config) (*S3Storage, error) {
	if !cfg.S3Enabled {
		return nil, fmt.Errorf("S3 не включен в конфигурации")
	}

	var awsConfig aws.Config
	var err error

	creds := credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, "")

	if cfg.S3Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: cfg.S3Endpoint,
			}, nil
		})
		awsConfig, err = awsconfig.LoadDefaultConfig(context.TODO(),
			awsconfig.WithEndpointResolverWithOptions(customResolver),
			awsconfig.WithCredentialsProvider(creds),
			awsconfig.WithRegion(cfg.S3Region),
		)
	} else {
		awsConfig, err = awsconfig.LoadDefaultConfig(context.TODO(),
			awsconfig.WithCredentialsProvider(creds),
			awsconfig.WithRegion(cfg.S3Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("ошибка загрузки AWS конфигурации: %v", err)
	}

	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	// Проверяем доступность бакета
	_, err = client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(cfg.S3Bucket),
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка доступа к S3 бакету %s: %v", cfg.S3Bucket, err)
	}

	log.Printf("✅ S3 хранилище инициализировано: бакет %s", cfg.S3Bucket)

	return &S3Storage{
		client: client,
		bucket: cfg.S3Bucket,
		config: cfg,
	}, nil
}

// SaveFile сохраняет файл в S3
func (s *S3Storage) SaveFile(data []byte, filename string) error {
	ctx := context.TODO()

	// Создаем ключ для S3 (путь к файлу)
	key := s.generateS3Key(filename)

	// Загружаем файл в S3
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader(string(data)),
		ContentType: aws.String("text/csv"),
		Metadata: map[string]string{
			"original-filename": filename,
			"upload-time":       time.Now().Format(time.RFC3339),
		},
	})

	if err != nil {
		return fmt.Errorf("ошибка загрузки файла в S3: %v", err)
	}

	log.Printf("✅ Файл сохранен в S3: %s", key)
	return nil
}

// GetFile возвращает содержимое файла из S3
func (s *S3Storage) GetFile(filename string) ([]byte, error) {
	ctx := context.TODO()
	key := s.generateS3Key(filename)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка получения файла из S3: %v", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения содержимого файла: %v", err)
	}

	return data, nil
}

// ListFiles возвращает список файлов из S3
func (s *S3Storage) ListFiles() ([]models.ExportFile, error) {
	ctx := context.TODO()

	var files []models.ExportFile
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(s.bucket),
			Prefix: aws.String("exports/"), // Префикс для файлов экспорта
		}

		if continuationToken != nil {
			input.ContinuationToken = continuationToken
		}

		result, err := s.client.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения списка файлов из S3: %v", err)
		}

		for _, obj := range result.Contents {
			// Пропускаем директории и не-CSV файлы
			if strings.HasSuffix(*obj.Key, "/") || !strings.HasSuffix(*obj.Key, ".csv") {
				continue
			}

			// Извлекаем имя файла из ключа
			filename := filepath.Base(*obj.Key)

			exportFile := models.ExportFile{
				Name:          filename,
				Size:          *obj.Size,
				ModifiedTime:  *obj.LastModified,
				FormattedSize: s.formatFileSize(*obj.Size),
				FormattedDate: obj.LastModified.Format("02.01.2006 15:04:05"),
			}
			files = append(files, exportFile)
		}

		if !*result.IsTruncated {
			break
		}
		continuationToken = result.NextContinuationToken
	}

	// Сортируем по дате изменения (новые сверху)
	sortExportFiles(files)

	return files, nil
}

// DeleteFile удаляет файл из S3
func (s *S3Storage) DeleteFile(filename string) error {
	ctx := context.TODO()
	key := s.generateS3Key(filename)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("ошибка удаления файла из S3: %v", err)
	}

	log.Printf("✅ Файл удален из S3: %s", key)
	return nil
}

// CleanupOldFiles удаляет файлы старше месяца из S3
func (s *S3Storage) CleanupOldFiles() error {
	files, err := s.ListFiles()
	if err != nil {
		return fmt.Errorf("ошибка получения списка файлов для очистки: %v", err)
	}

	monthAgo := time.Now().AddDate(0, -1, 0)
	deletedCount := 0

	for _, file := range files {
		if file.ModifiedTime.Before(monthAgo) {
			if err := s.DeleteFile(file.Name); err != nil {
				log.Printf("Ошибка удаления старого файла %s: %v", file.Name, err)
			} else {
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		log.Printf("Удалено %d старых файлов из S3", deletedCount)
	}

	return nil
}

// generateS3Key генерирует ключ для S3 на основе имени файла
func (s *S3Storage) generateS3Key(filename string) string {
	timestamp := time.Now().Format("2006-01-02")
	return fmt.Sprintf("exports/%s/%s", timestamp, filename)
}

// formatFileSize форматирует размер файла в читаемый вид
func (s *S3Storage) formatFileSize(size int64) string {
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

// sortExportFiles сортирует файлы по дате изменения (новые сверху)
func sortExportFiles(files []models.ExportFile) {
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].ModifiedTime.Before(files[j].ModifiedTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
}
