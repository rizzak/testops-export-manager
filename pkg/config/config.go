package config

import (
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
	Exports      []models.ExportConfig
	MaxRetries   int
	RetryDelay   time.Duration
	CronSchedule string // Добавляем настройку расписания
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Printf("⚠️  Файл .env не найден или не загружен: %v\n", err)
		fmt.Println("   Переменные будут загружены из системного окружения")
	} else {
		fmt.Println("✅ Файл .env загружен успешно")
	}

	config := &Config{
		BaseURL:      getEnv("TESTOPS_BASE_URL", "https://allure-testops.wb.ru"),
		Token:        getEnv("TESTOPS_TOKEN", ""),
		ExportPath:   getEnv("EXPORT_PATH", "./exports"),
		WebPort:      getEnv("WEB_PORT", "9090"),
		MaxRetries:   10,
		RetryDelay:   15 * time.Minute,
		CronSchedule: getEnv("CRON_SCHEDULE", "0 7 * * *"), // По умолчанию 7:00 UTC
		Exports: []models.ExportConfig{
			{GroupID: 26961091, GroupName: "API"},
			{GroupID: 24545654, GroupName: "UI"},
		},
	}

	if config.Token == "" {
		return nil, fmt.Errorf("TESTOPS_TOKEN не установлен")
	}

	return config, nil
}

// getEnv получает значение переменной окружения или возвращает значение по умолчанию
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value != "" {
		// Защищаем токен от вывода в лог
		if key == "TESTOPS_TOKEN" {
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
