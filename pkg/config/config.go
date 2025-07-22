package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"testops-export/pkg/models"

	"github.com/joho/godotenv"
)

// Config представляет конфигурацию приложения
type Config struct {
	BaseURL      string
	Token        string // Используется для получения access_token
	ExportPath   string
	WebPort      string
	Projects     []models.ProjectConfig
	MaxRetries   int
	RetryDelay   time.Duration
	CronSchedule string // Добавляем настройку расписания

	// S3 конфигурация
	S3Enabled   bool
	S3Bucket    string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Region    string
}

type ProjectsFile struct {
	Projects []models.ProjectConfig `json:"projects"`
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Printf("⚠️  Файл .env не найден или не загружен: %v\n", err)
		fmt.Println("   Переменные будут загружены из системного окружения")
	} else {
		fmt.Println("✅ Файл .env загружен успешно")
	}

	projectsPath := getEnv("PROJECTS_CONFIG", "projects.json")
	var projectsFile ProjectsFile
	data, err := os.ReadFile(projectsPath)
	if err != nil {
		return nil, fmt.Errorf("Ошибка чтения файла проектов: %v", err)
	}
	if err := json.Unmarshal(data, &projectsFile); err != nil {
		return nil, fmt.Errorf("Ошибка парсинга JSON проектов: %v", err)
	}

	config := &Config{
		BaseURL:      getEnv("TESTOPS_BASE_URL", "https://your-testops.ru"),
		Token:        getEnv("TESTOPS_TOKEN", ""),
		ExportPath:   getEnv("EXPORT_PATH", "./exports"),
		WebPort:      getEnv("WEB_PORT", "9090"),
		MaxRetries:   10,
		RetryDelay:   15 * time.Minute,
		CronSchedule: getEnv("CRON_SCHEDULE", "0 7 * * *"), // По умолчанию 7:00 UTC
		Projects:     projectsFile.Projects,
		// S3 конфигурация
		S3Enabled:   getEnvBool("S3_ENABLED", false),
		S3Bucket:    getEnv("S3_BUCKET", ""),
		S3Endpoint:  getEnv("S3_ENDPOINT", ""),
		S3AccessKey: getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey: getEnv("S3_SECRET_KEY", ""),
		S3Region:    getEnv("S3_REGION", "us-east-1"),
	}

	if config.Token == "" {
		return nil, fmt.Errorf("TESTOPS_TOKEN не установлен")
	}

	// Проверяем S3 конфигурацию если она включена
	if config.S3Enabled {
		if config.S3Bucket == "" {
			return nil, fmt.Errorf("S3_BUCKET должен быть установлен когда S3_ENABLED=true")
		}
		if config.S3AccessKey == "" {
			return nil, fmt.Errorf("S3_ACCESS_KEY должен быть установлен когда S3_ENABLED=true")
		}
		if config.S3SecretKey == "" {
			return nil, fmt.Errorf("S3_SECRET_KEY должен быть установлен когда S3_ENABLED=true")
		}
	}

	return config, nil
}

// getEnv получает значение переменной окружения или возвращает значение по умолчанию
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		// Защищаем токен от вывода в лог
		if key == "TESTOPS_TOKEN" || key == "S3_SECRET_KEY" {
			if len(value) > 8 {
				fmt.Printf("🔍 %s: '%s...%s' (из окружения)\n", key, value[:4], value[len(value)-4:])
			} else {
				fmt.Printf("🔍 %s: '[СКРЫТ]' (из окружения)\n", key)
			}
		} else {
			fmt.Printf("🔍 %s: '%s' (из окружения)\n", key, value)
		}
		return value
	}
	fmt.Printf("🔍 %s: '%s' (по умолчанию)\n", key, defaultValue)
	return defaultValue
}

// getEnvBool получает булево значение переменной окружения
func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		fmt.Printf("🔍 %s: %t (по умолчанию)\n", key, defaultValue)
		return defaultValue
	}

	result := value == "true" || value == "1" || value == "yes"
	fmt.Printf("🔍 %s: %t (из окружения)\n", key, result)
	return result
}
