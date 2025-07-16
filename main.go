package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"

	"testops-export/pkg/config"
	"testops-export/pkg/export"
	"testops-export/pkg/web"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// Создаем менеджер экспорта
	exportManager := export.NewManager(cfg)

	// Создаем веб-сервер
	server := web.NewServer(cfg)

	// Запускаем веб-сервер в горутине
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Ошибка запуска веб-сервера: %v", err)
		}
	}()

	// Создаем планировщик cron
	c := cron.New(cron.WithLocation(time.UTC))

	// Добавляем задачу на ежедневный экспорт по расписанию из конфигурации
	// Формат: "минуты часы день_месяца месяц день_недели"
	// Примеры:
	// "0 7 * * *"     - каждый день в 7:00 UTC
	// "0 9 * * *"     - каждый день в 9:00 UTC
	// "30 6 * * *"    - каждый день в 6:30 UTC
	// "0 8 * * 1-5"   - по будням в 8:00 UTC
	// "0 0 * * *"     - каждый день в 00:00 UTC
	_, err = c.AddFunc(cfg.CronSchedule, func() {
		log.Printf("🕐 Запуск автоматического экспорта по расписанию (%s)...", cfg.CronSchedule)
		exportManager.PerformExport()
	})
	if err != nil {
		log.Fatalf("Ошибка добавления cron задачи: %v", err)
	}

	// Запускаем планировщик
	c.Start()
	log.Printf("📅 Планировщик запущен. Автоматический экспорт будет выполняться по расписанию: %s", cfg.CronSchedule)

	// Ждем сигнала для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("🛑 Получен сигнал завершения. Останавливаем сервис...")

	// Останавливаем планировщик
	ctx := c.Stop()
	<-ctx.Done()

	log.Println("✅ Сервис остановлен")
}
