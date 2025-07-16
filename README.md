# TestOps Export Manager

Утилита для ежедневного экспорта тесткейсов из Allure TestOps с веб-интерфейсом для управления.

## Возможности

- ✅ **Автоматический экспорт** каждый день в 7:00 UTC
- ✅ **Веб-интерфейс** для просмотра и скачивания экспортов
- ✅ **Поддержка нескольких групп** (API и UI)
- ✅ **Система повторных попыток** при ошибках (до 3 попыток)
- ✅ **Автоматическая очистка** файлов старше месяца
- ✅ **Docker контейнеризация** для простого развертывания
- ✅ **Docker Swarm оркестрация** для высокой доступности
- ✅ **Graceful shutdown** при получении сигналов завершения

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

### Локальный запуск

1. **Клонируйте репозиторий:**
   ```bash
   git clone <repository-url>
   cd testops-export
   ```

2. **Создайте файл .env с вашим токеном:**
   ```bash
   TESTOPS_TOKEN=your_testops_token_here
   ```

3. **Запустите приложение:**
   ```bash
   go run main.go
   ```

4. **Откройте веб-интерфейс:**
   ```
   http://localhost:9090
   ```

### Docker запуск

1. **Создайте .env файл:**
   ```bash
   TESTOPS_TOKEN=your_testops_token_here
   ```

2. **Запустите с Docker Compose:**
   ```bash
   docker-compose up -d
   ```

3. **Проверьте логи:**
   ```bash
   docker-compose logs -f
   ```

### Docker Swarm (Production)

Для высокой доступности и отказоустойчивости используйте Docker Swarm:

#### Быстрый старт

```bash
# Инициализация и развертывание
./deploy-swarm.sh init

# Проверка статуса
./deploy-swarm.sh status

# Мониторинг
./deploy-swarm.sh monitor
```

#### Подробные команды

```bash
# 1. Инициализация Swarm
docker swarm init

# 2. Создание секрета для токена
docker secret create testops_token .env

# 3. Развертывание stack
docker stack deploy -c docker-compose.swarm.yml testops

# 4. Проверка статуса
docker stack services testops
docker stack ps testops

# 5. Просмотр логов
docker service logs testops_testops-export -f
```

#### Преимущества Swarm развертывания

- 🔄 **Автоматический перезапуск** при сбоях
- 📈 **Масштабируемость** - легко добавить реплики
- 🛡️ **Отказоустойчивость** - сервис продолжает работать при падении узлов
- 🔄 **Zero-downtime обновления** - обновления без простоя
- 📊 **Балансировка нагрузки** через Nginx
- 🔒 **Безопасность** - секреты для токенов
- 📈 **Мониторинг** - встроенные метрики

#### Управление в Swarm

```bash
# Масштабирование
docker service scale testops_testops-export=3

# Обновление сервиса
./deploy-swarm.sh update

# Удаление
./deploy-swarm.sh remove

# Мониторинг в реальном времени
./deploy-swarm.sh monitor
```

## ⚡ Быстрое обновление сервиса

Для обновления кода и перезапуска сервиса используйте:

```bash
./deploy-swarm.sh update
```

- Скрипт пересоберёт Docker-образ и обновит сервис в Swarm.
- Это рекомендуемый способ для разработки и production.

---

## Конфигурация

### Переменные окружения

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `TESTOPS_TOKEN` | Токен доступа к TestOps | (обязательно) |
| `TESTOPS_BASE_URL` | URL TestOps сервера | `https://allure-testops.wb.ru` |
| `EXPORT_PATH` | Путь для сохранения экспортов | `./exports` |
| `WEB_PORT` | Порт веб-сервера | `9090` |
| `CRON_SCHEDULE` | Расписание экспорта (cron формат) | `0 7 * * *` (7:00 UTC) |

- `TESTOPS_TOKEN` — персональный API-токен пользователя для получения access_token (обязателен)
- `TESTOPS_BASE_URL` — базовый адрес TestOps (например, https://allure-testops.wb.ru)

Теперь access_token для авторизации получается автоматически через API:

```
curl -s -X POST "${TESTOPS_BASE_URL}/api/uaa/oauth/token" \
     --header "Expect:" \
     --header "Accept: application/json" \
     --form "grant_type=apitoken" \
     --form "scope=openid" \
     --form "token=${TESTOPS_TOKEN}" \
     | jq -r .access_token
```

Время жизни access_token — 1 час. Клиент обновляет его автоматически.

### Группы экспорта

По умолчанию настроены две группы:
- **API** (ID: 26961091)
- **UI** (ID: 24545654)

Для изменения групп отредактируйте `pkg/config/config.go`.

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

### Локальное хранение

Файлы экспорта сохраняются в папке `exports/` в корне проекта:

```
testops-export/
├── exports/                   # Папка с файлами экспорта
│   ├── testops_export_API_2025-07-15_12-02-56.csv
│   ├── testops_export_UI_2025-07-15_12-03-01.csv
│   └── ...
├── docker-compose.yml
├── docker-compose.swarm.yml
└── ...
```

### Docker и Swarm

- **Docker Compose:** файлы сохраняются в `./exports` на хосте
- **Docker Swarm:** файлы также сохраняются в `./exports` на хосте (bind mount)

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
- Повторяет попытку до 3 раз
- Использует экспоненциальную задержку (5, 10, 15 минут)
- Логирует все попытки и результаты

### Swarm мониторинг

```bash
# Статус узлов
docker node ls

# Статус сервисов
docker stack services testops

# Задачи сервисов
docker stack ps testops

# Логи сервиса
docker service logs testops_testops-export -f

# Метрики
docker stats
```

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
├── docker-compose.yml        # Docker Compose (dev)
├── docker-compose.swarm.yml  # Docker Swarm (prod)
├── nginx.conf                # Nginx конфигурация
├── deploy-swarm.sh           # Скрипт развертывания
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

# Swarm развертывание
./deploy-swarm.sh update
```

## Troubleshooting

### Проблемы с Swarm

```bash
# Проверка статуса Swarm
docker info | grep Swarm

# Переинициализация Swarm
docker swarm leave --force
docker swarm init

# Очистка секретов
docker secret rm testops_token

# Пересоздание секрета
docker secret create testops_token .env
```

### Проблемы с сетью

```bash
# Проверка overlay сети
docker network ls

# Создание сети
docker network create --driver overlay --attachable testops-network
```

### Проблемы с обновлениями

```bash
# Принудительное обновление
docker service update --force testops_testops-export

# Откат к предыдущей версии
docker service rollback testops_testops-export
```

## Лицензия

MIT License 