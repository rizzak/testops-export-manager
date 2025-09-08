package main

import (
	"context"
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
	server := web.NewServer(exportManager)

	// Запускаем веб-сервер в горутине
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Ошибка запуска веб-сервера: %v", err)
		}
	}()

	log.Println("Запускаю leader election...")
	export.RunWithLeaderElection(func(ctx context.Context) {
		log.Println("🔧 Начинаю инициализацию cron планировщика...")

		// Создаем планировщик cron
		c := cron.New(cron.WithLocation(time.UTC))

		// Добавляем диагностическую задачу каждый час для проверки
		_, err = c.AddFunc("0 * * * *", func() {
			// Выводим информацию о следующем экспорте
			nextExportInfo := exportManager.GetNextExportInfo()
			if nextExportInfo.HasError {
				log.Printf("🔔 Cron жив! Текущее время: %s UTC, %s", time.Now().UTC().Format("2006-01-02 15:04:05"), nextExportInfo.ErrorMessage)
			} else {
				log.Printf("🔔 Cron жив! Текущее время: %s UTC, следующий экспорт через %s (%s)",
					time.Now().UTC().Format("2006-01-02 15:04:05"),
					nextExportInfo.FormattedTime,
					nextExportInfo.NextRunFormatted)
			}
		})
		if err != nil {
			log.Printf("Ошибка добавления диагностической задачи: %v", err)
		}

		// Добавляем задачу на ежедневный экспорт по расписанию из конфигурации
		// Формат: "минуты часы день_месяца месяц день_недели"
		// Примеры:
		// "0 7 * * *"     - каждый день в 7:00 UTC
		// "0 9 * * *"     - каждый день в 9:00 UTC
		// "30 6 * * *"    - каждый день в 6:30 UTC
		// "0 8 * * 1-5"   - по будням в 8:00 UTC
		// "0 0 * * *"     - каждый день в 00:00 UTC
		_, err = c.AddFunc(cfg.CronSchedule, func() {
			log.Printf("⏰ Запуск автоматического экспорта по расписанию (%s)...", cfg.CronSchedule)
			exportManager.PerformExport()
		})
		if err != nil {
			log.Fatalf("Ошибка добавления cron задачи: %v", err)
		}

		// Запускаем планировщик
		c.Start()
		log.Printf("📅 Планировщик запущен. Автоматический экспорт будет выполняться по расписанию: %s", cfg.CronSchedule)

		// Запускаем горутину для мониторинга контекста
		go func() {
			<-ctx.Done()
			log.Println("⚠️ Контекст leader election был отменен, останавливаем cron...")
			c.Stop()
		}()

		// Ждём сигнала для graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-ctx.Done():
			log.Println("Контекст завершён, останавливаем сервис...")
		case <-sigChan:
			log.Println("🛑 Получен сигнал завершения. Останавливаем сервис...")
		}
		ctxStop := c.Stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Ошибка при завершении веб-сервера: %v", err)
		}
		<-ctxStop.Done()
		log.Println("✅ Сервис остановлен")
	})
}
