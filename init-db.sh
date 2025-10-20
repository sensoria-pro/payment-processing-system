#!/bin/bash

# Скрипт для инициализации базы данных
set -e

# Загрузка переменных окружения
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Функция для вывода сообщений
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Проверка наличия Docker
check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker не установлен. Установите Docker и попробуйте снова."
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        log_error "Docker не запущен. Запустите Docker и попробуйте снова."
        exit 1
    fi
}

# Проверка наличия golang-migrate
check_migrate() {
    if ! command -v migrate &> /dev/null; then
        log_warn "golang-migrate не установлен. Устанавливаем..."
        go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
    fi
}

# Запуск PostgreSQL в Docker
start_postgres() {
    log_step "Запуск PostgreSQL в Docker..."
    
    if ! docker compose ps postgres | grep -q "Up"; then
        docker compose up -d postgres
        log_info "PostgreSQL запущен"
        
        # Ожидание готовности
        log_info "Ожидание готовности PostgreSQL..."
        until docker compose exec -T postgres pg_isready -U postgres > /dev/null 2>&1; do
            sleep 2
        done
        log_info "PostgreSQL готов к работе"
    else
        log_info "PostgreSQL уже запущен"
    fi
}

# Создание базы данных
create_database() {
    local db_name=${POSTGRES_DB:-payment_processing_system}
    local db_user=${POSTGRES_USER:-postgres}
    
    log_step "Создание базы данных '$db_name'..."
    
    if docker compose exec -T postgres createdb -U "$db_user" "$db_name" 2>/dev/null; then
        log_info "База данных '$db_name' создана успешно."
    else
        log_warn "База данных '$db_name' уже существует или не удалось создать."
    fi
}

# Применение миграций PostgreSQL
apply_postgres_migrations() {
    log_step "Применение миграций PostgreSQL..."
    
    if make migrate-up; then
        log_info "Миграции PostgreSQL применены успешно."
    else
        log_error "Ошибка при применении миграций PostgreSQL."
        exit 1
    fi
}

# Применение миграций ClickHouse
apply_clickhouse_migrations() {
    log_step "Применение миграций ClickHouse..."
    
    # Проверяем, запущен ли ClickHouse
    if docker compose ps clickhouse | grep -q "Up"; then
        log_info "ClickHouse запущен, применяем миграции..."
        
        # Применяем миграции через clickhouse-migrator
        if docker compose up clickhouse-migrator; then
            log_info "Миграции ClickHouse применены успешно."
        else
            log_warn "Ошибка при применении миграций ClickHouse."
        fi
    else
        log_warn "ClickHouse не запущен. Запустите всю систему: make start-all"
    fi
}

# Основная функция
main() {
    log_step "🗄️ Инициализация баз данных Payment Gateway"
    echo
    
    # Проверки
    check_docker
    check_migrate
    
    # Запуск PostgreSQL
    start_postgres
    
    # Создание базы данных
    create_database
    
    # Применение миграций
    apply_postgres_migrations
    apply_clickhouse_migrations
    
    echo
    log_info "✅ Инициализация баз данных завершена успешно!"
    echo
    echo "📊 Статус:"
    echo "   • PostgreSQL: готов"
    echo "   • ClickHouse: готов (если запущен)"
    echo
    echo "🔧 Полезные команды:"
    echo "   • make migrate-version - показать версию миграций"
    echo "   • make migrate-status - показать статус миграций"
    echo "   • make health-check - проверить здоровье системы"
    echo "   • make start-all - запустить всю систему"
}

# Запуск скрипта
main "$@" 