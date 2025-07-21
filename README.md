# TestOps Export Manager

Приложение для автоматического экспорта тесткейсов из TestOps с веб-интерфейсом.

## Возможности

- 🔄 Автоматический экспорт по расписанию (cron)
- 🌐 Веб-интерфейс для управления экспортами
- 📁 Скачивание экспортированных файлов
- 🔄 Повторные попытки при ошибках
- 🧹 Автоматическая очистка старых файлов

## Архитектура

Проект разделен на модули для лучшей организации кода:

```
pkg/
├── config/     # Конфигурация приложения
├── models/     # Модели данных
├── api/        # API клиент для TestOps
├── export/     # Менеджер экспорта
└── web/        # Веб-сервер и интерфейс
```

## Установка и запуск

**Создайте файл .env:**
   ```bash
   TESTOPS_TOKEN=your_testops_token_here
   TESTOPS_BASE_URL="https://your-testops.ru"
   EXPORT_PATH=./exports
   WEB_PORT=9090
   CRON_SCHEDULE=0 7 * * *
   ```

### Локальный запуск

1. **Клонируйте репозиторий:**
   ```bash
   git clone <repository-url>
   cd testops-export
   ```

2. **Запустите приложение:**
   ```bash
   go run main.go
   ```

3. **Откройте веб-интерфейс:**
   ```
   http://localhost:9090
   ```

### Docker запуск

1. **Запустите с Docker Compose:**
   ```bash
   docker-compose up -d --build
   ```

2. **Проверьте логи:**
   ```bash
   docker-compose logs -f
   ```

---

## Конфигурация

### Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `TESTOPS_TOKEN` | Токен доступа к TestOps | персональный API-токен пользователя для получения access_token (обязателен) |
| `TESTOPS_BASE_URL` | URL TestOps сервера | `https://your-testops.ru` |
| `EXPORT_PATH` | Путь для сохранения экспортов | `./exports` |
| `WEB_PORT` | Порт веб-сервера | `9090` |
| `CRON_SCHEDULE` | Расписание экспорта (cron формат) | `0 7 * * *` (7:00 UTC) |


### Группы экспорта

По умолчанию настроены две группы:
- **API** (ID: 26961091)
- **UI** (ID: 24545654)

Для изменения групп отредактируйте `pkg/config/config.go`.  
Узнать группу можно посмотрев запрос экспорта дирректории через браузер, параметр groupsInclude

### Настройка времени экспорта

Время экспорта настраивается через переменную `CRON_SCHEDULE`:

```bash
# Экспорт каждый день в 9:00 UTC
CRON_SCHEDULE="0 9 * * *"

# Экспорт каждый день в 6:30 UTC
CRON_SCHEDULE="30 6 * * *"

# Экспорт по будням в 8:00 UTC
CRON_SCHEDULE="0 8 * * 1-5"

# Экспорт каждый день в полночь UTC
CRON_SCHEDULE="0 0 * * *"
```

**Формат cron:** `минуты часы день_месяца месяц день_недели`

## Хранение файлов

Файлы экспорта сохраняются в папке `exports/` в корне проекта:

```
testops-export/
├── exports/                   # Папка с файлами экспорта
│   ├── testops_export_API_2025-07-15_12-02-56.csv
│   ├── testops_export_UI_2025-07-15_12-03-01.csv
│   └── ...
├── docker-compose.yml
└── ...
```

- **Docker Compose:** файлы сохраняются в `./exports` на хосте (bind mount)

### Автоматическая очистка

Система автоматически удаляет файлы старше 30 дней для экономии места.

### Резервное копирование

Для резервного копирования файлов экспорта:

```bash
# Копирование всех файлов экспорта
cp -r exports/ backup_exports/

# Архивирование
tar -czf exports_backup_$(date +%Y%m%d).tar.gz exports/
```

## Хранение в S3 (MinIO, AWS S3 и др.)

Приложение поддерживает хранение экспортов в S3-совместимых хранилищах (например, MinIO, Яндекс S3, AWS S3).

### Как включить S3

1. В .env или переменных окружения укажите:
   ```
   S3_ENABLED=true
   S3_BUCKET=your-bucket
   S3_ENDPOINT=http://minio:9000
   S3_ACCESS_KEY=admin
   S3_SECRET_KEY=password
   S3_REGION=us-east-1
   ```
2. При включённом S3 все экспорты будут сохраняться и отображаться только в S3.
3. Автоматическая очистка старых файлов также работает через S3.

### Пример MinIO для docker-compose (для локального тестирования)

```yaml
# minio:
#   image: minio/minio:latest
#   container_name: minio
#   environment:
#     - MINIO_ROOT_USER=admin
#     - MINIO_ROOT_PASSWORD=password
#   command: server /data --console-address ":9001"
#   ports:
#     - "9000:9000" # S3 API
#     - "9001:9001" # Web консоль
#   volumes:
#     - minio-data:/data
#   networks:
#     - testops-network

# networks:
#   testops-network:
#     driver: bridge
# volumes:
#   minio-data:
```

## Веб-интерфейс

Веб-интерфейс предоставляет:

- 📊 **Статистика экспортов** (количество файлов, общий размер)
- 📅 **Информация о последнем экспорте**
- 🚀 **Кнопка ручного запуска экспорта**
- 📥 **Скачивание файлов экспорта**
- 📱 **Адаптивный дизайн** для мобильных устройств

## Логирование

Приложение логирует:
- Запуск и завершение экспортов
- Ошибки и повторные попытки
- Удаление старых файлов
- Статус веб-сервера

## Мониторинг

### Health Check

Docker Compose включает health check:
```yaml
healthcheck:
  test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:9090"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

### Система повторных попыток

При ошибках экспорта система автоматически:
- Повторяет попытку до 10 раз
- Использует экспоненциальную задержку (15-150 минут)
- Логирует все попытки и результаты

## Разработка

### Структура проекта

```
testops-export/
├── main.go                    # Точка входа приложения
├── pkg/                       # Модули приложения
│   ├── config/               # Конфигурация
│   ├── models/               # Модели данных
│   ├── api/                  # API клиент
│   ├── export/               # Менеджер экспорта
│   └── web/                  # Веб-сервер
├── docker-compose.yml        # Docker Compose
├── nginx.conf                # Nginx конфигурация
├── Dockerfile                # Docker образ
├── go.mod                    # Go модули
└── README.md                 # Документация
```

### Добавление новых групп

1. Отредактируйте `pkg/config/config.go`
2. Добавьте новую группу в массив `Exports`:
   ```go
   Exports: []models.ExportConfig{
       {GroupID: 26961091, GroupName: "API"},
       {GroupID: 24545654, GroupName: "UI"},
       {GroupID: 12345678, GroupName: "NewGroup"},
   },
   ```

### Сборка

```bash
# Локальная сборка
go build -o testops-export

# Docker сборка
docker build -t testops-export .

# Запуск через Docker Compose
docker-compose up -d --build
```

### Тесты
`go test -v ./tests`

## Troubleshooting

### Проблемы с обновлениями

```bash
# Принудительная пересборка и перезапуск
docker-compose up -d --build
```
