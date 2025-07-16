#!/bin/bash

# Скрипт для развертывания TestOps Export Manager в Docker Swarm
# Использование: ./deploy-swarm.sh [init|deploy|update|remove|status]

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Функции
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Проверка наличия .env файла и экспорт переменных
check_env() {
    if [ ! -f .env ]; then
        log_error "Файл .env не найден!"
        log_info "Создайте файл .env с переменной TESTOPS_TOKEN"
        exit 1
    fi
    
    if ! grep -q "TESTOPS_TOKEN" .env; then
        log_error "TESTOPS_TOKEN не найден в .env файле!"
        exit 1
    fi
    
    # Экспорт переменных из .env файла
    log_info "Экспорт переменных окружения из .env файла..."
    set -a
    source .env
    set +a
    
    log_success "Конфигурация проверена и переменные экспортированы"
}

# Инициализация Swarm
init_swarm() {
    log_info "Инициализация Docker Swarm..."
    
    if ! docker info | grep -q "Swarm: active"; then
        docker swarm init
        log_success "Swarm инициализирован"
    else
        log_warning "Swarm уже активен"
    fi
    
    # Создание overlay сети
    if ! docker network ls | grep -q "testops-network"; then
        docker network create --driver overlay --attachable testops-network
        log_success "Сеть testops-network создана"
    else
        log_warning "Сеть testops-network уже существует"
    fi
}

# Развертывание сервисов
deploy_services() {
    log_info "Развертывание сервисов в Swarm..."
    
    # Развертывание stack
    docker stack deploy -c docker-compose.swarm.yml testops
    log_success "Stack testops развернут"
    
    # Ожидание запуска сервисов
    log_info "Ожидание запуска сервисов..."
    sleep 10
    
    # Проверка статуса
    docker stack services testops
}

# Обновление сервисов
update_services() {
    log_info "Обновление сервисов..."
    
    log_info "Пересборка Docker-образа без кэша..."
    docker build --no-cache -t testops-export:latest .
    
    # Обновление stack
    docker stack deploy -c docker-compose.swarm.yml testops
    
    log_success "Сервисы обновлены"
}

# Удаление сервисов
remove_services() {
    log_warning "Удаление сервисов из Swarm..."
    
    docker stack rm testops
    
    log_success "Сервисы удалены"
}

# Показать статус
show_status() {
    log_info "Статус Swarm кластера:"
    docker node ls
    
    echo
    log_info "Статус сервисов:"
    docker stack services testops
    
    echo
    log_info "Задачи сервисов:"
    docker stack ps testops
    
    echo
    log_info "Логи сервиса:"
    docker service logs testops_testops-export --tail 20
}

# Мониторинг
monitor() {
    log_info "Мониторинг сервисов (Ctrl+C для выхода)..."
    watch -n 5 'docker stack services testops && echo && docker stack ps testops'
}

# Основная логика
case "${1:-help}" in
    "init")
        check_env
        init_swarm
        deploy_services
        ;;
    "deploy")
        check_env
        deploy_services
        ;;
    "update")
        check_env
        update_services
        ;;
    "remove")
        remove_services
        ;;
    "status")
        show_status
        ;;
    "monitor")
        monitor
        ;;
    "help"|*)
        echo "Использование: $0 {init|deploy|update|remove|status|monitor}"
        echo
        echo "Команды:"
        echo "  init     - Инициализация Swarm и развертывание"
        echo "  deploy   - Развертывание сервисов"
        echo "  update   - Обновление сервисов (всегда без кэша)"
        echo "  remove   - Удаление сервисов"
        echo "  status   - Показать статус"
        echo "  monitor  - Мониторинг в реальном времени"
        echo
        echo "Примеры:"
        echo "  $0 init      # Первый запуск"
        echo "  $0 update    # Обновление с полной пересборкой"
        echo "  $0 status    # Проверка статуса"
        ;;
esac 