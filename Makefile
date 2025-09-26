# Load variables from .env to use them in commands
ifneq (,$(wildcard .env))
    include .env
	export $(shell sed 's/=.*//' .env 2>/dev/null || echo "")	
endif

# Variables for connecting to the database (with fallback values)
POSTGRES_HOST ?= localhost
POSTGRES_PORT ?= 5432
POSTGRES_USER ?= postgres
POSTGRES_PASSWORD ?= password
POSTGRES_DB ?= payment_system
DB_SSL_MODE ?= disable

# Database connection string
DATABASE_URL = postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=$(DB_SSL_MODE)

# Path to migration
MIGRATIONS_PATH = migrations_postgres

.PHONY: help migrate-up migrate-down migrate-force migrate-version migrate-create

help: ## Show help
	@echo "Доступные команды:"
	@echo "=================="
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | sort | awk -F ':.*## ' '{printf "make %-20s - %s\n", $$1, $$2}'

migrate-up: ## Apply all migrations
	@if [ "$(POSTGRES_PASSWORD)" = "password" ]; then \
        echo "❌ Опасность: используется пароль по умолчанию 'password'"; \
        echo "💡 Установите POSTGRES_PASSWORDD в .env или через export"; \
        exit 1; \
    fi
    @echo "Применение миграций..."
    migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" up

migrate-down: ## Roll back all migrations
	@echo "Откат всех миграций..."
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" down

migrate-force: ## Force migration version (use with caution)
	@echo "Укажите версию: make migrate-force VERSION=<номер_версии>"
	@if [ -z "$(VERSION)" ]; then echo "Ошибка: не указана версия"; exit 1; fi
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" force $(VERSION)

migrate-version: ## Show the current migration version
	@echo "Текущая версия миграции:"
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" version

migrate-create: ## Create a new migration
	@echo "Укажите имя миграции: make migrate-create NAME=<имя_миграции>"
	@if [ -z "$(NAME)" ]; then echo "Ошибка: не указано имя миграции"; exit 1; fi
	migrate create -ext sql -dir $(MIGRATIONS_PATH) -seq $(NAME).sql

migrate-status: ## Show migration status
	@echo "Статус миграций:"
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" version



# ----- Testing commands
test-migrate: ## Running tests with migrations
	@echo "Запуск тестов..."
	go test ./...

# ----- Production commands
prod-migrate: ## Applying Migrations in Production
    @echo "⚠️  Применение миграций в продакшене!"
    @echo "❗ Убедитесь, что у вас есть резервная копия!"
    @read -p "Продолжить? (y/N): " -n 1 -r; echo
    @if [[ ! $$REPLY =~ ^[Yy]$$ ]]; then exit 1; fi
    $(MAKE) migrate-up

# ----- Commands for launching the application
run: ## Launching the application
	@echo "Запуск payment-processing-system..."
	./scripts/run.sh

run-dev: ## Running the application in development mode
	@echo "Запуск payment-processing-system в режиме разработки..."
	$(MAKE) dev-docker
	@echo "Запуск приложения..."
	go run cmd/main.go

test-api: ## API Testing
	@echo "Тестирование API..."
	./scripts/test-api.sh

build: ## Building the application
	@echo "Сборка payment-processing-system..."
	go build -o bin/payment-processing-system cmd/main.go

# ------- Commands for Docker
docker-up: ## Start all Docker containers
	@echo "Запуск всех Docker контейнеров..."
	docker compose up -d
	@echo "Ожидание готовности сервисов..."
	@sleep 10
	@echo "Все контейнеры запущены!"

docker-down: ## Stopping Docker containers
	@echo "Остановка Docker контейнеров..."
	docker compose down

docker-logs: ## Viewing Docker container logs
	docker compose logs -f

docker-reset: ## Reset Docker containers and data
	@echo "Сброс Docker контейнеров и данных..."
	docker compose down -v
	$(MAKE) docker-up

# ----- Commands for Docker Development
dev-docker: ## Complete environment setup with Docker
	@echo "Настройка окружения для разработки с Docker..."
	@if [ ! -f .env ]; then cp env.example .env; echo "Создан .env файл из примера"; fi
	$(MAKE) docker-up
	@echo "Окружение готово!"
	@echo "Payment-processing-system: http://localhost:$(APP_PORT)/health"
	@echo "pgAdmin: http://localhost:$(PGADMIN_PORT:-8082) ($(PGADMIN_EMAIL) / $(PGADMIN_PASSWORD))"
	@echo "Grafana: http://localhost:$(GRAFANA_PORT)"
	@echo "Prometheus: http://localhost:$(PROMETHEUS_PORT)"
	@echo "Jaeger: http://localhost:$(JAEGER_PORT)"

wait-for-db: ## Wait for PostgreSQL to be ready
    @echo "Ожидание готовности PostgreSQL..."
    @until docker exec -i postgres-db pg_isready -U $(POSTGRES_USER) -d $(POSTGRES_DB); do \
        echo "⏳ PostgreSQL недоступен, ждём..."; \
        sleep 2; \
    done
    @echo "✅ PostgreSQL готов!"

# ------- Development commands
dev-setup: ## Setting up the development environment
    @echo "Настройка окружения для разработки..."
    @if [ ! -f .env ]; then cp env.example .env; echo "Создан .env файл из примера"; fi
    @echo "Запуск PostgreSQL..."
    $(MAKE) docker-up
    @echo "Ожидание готовности PostgreSQL..."
    $(MAKE) wait-for-db
    @echo "Создание базы данных..."
    docker exec -i postgres-db createdb -U $(POSTGRES_USER) $(POSTGRES_DB) || echo "База уже существует"
    $(MAKE) migrate-up

dev-reset: ## Resetting the development database
	@echo "Сброс базы данных..."
	dropdb $(POSTGRES_DB) 2>/dev/null || echo "База данных не существует"
	createdb $(POSTGRES_DB)
	$(MAKE) migrate-up

# ---- Commands for all services
build-alerter: ## Building alerter-service
	@echo "Сборка alerter-service..."
	go build -o bin/alerter-service cmd/alerter-service/main.go

run-alerter: ## Launch alerter-service
	@echo "Запуск alerter-service..."
	go run cmd/alerter-service/main.go	

build-antifraud: ## Building anti-fraud-analyzer
	@echo "Сборка anti-fraud-analyzer..."
	go build -o bin/anti-fraud-analyzer cmd/anti-fraud-analyzer/main.go

run-antifraud: ## Launch anti-fraud-analyzer
	@echo "Запуск anti-fraud-analyzer..."
	go run cmd/anti-fraud-analyzer/main.go

build-ch-query-tool: ## Building ch-query-tool
	@echo "Сборка ch-query-tool..."
	go build -o bin/ch-query-tool cmd/ch-query-tool/main.go

run-ch-query-tool: ## Launch ch-query-tool
	@echo "Запуск ch-query-tool..."
	go run cmd/ch-query-tool/main.go

build-dlq-tool: ## Building dlq-tool
	@echo "Сборка dlq-tool..."
	go build -o bin/dlq-tool cmd/dlq-tool/main.go

run-dlq-tool: ## Launch dlq-tool
	@echo "Запуск dlq-tool..."
	./bin/dlq-tool view --brokers=localhost:9092 --dlq-topic=transactions.created.dlq --limit=15

build-service-doctor: ## Building service-doctor
	@echo "Сборка service-doctor..."
	go build -o bin/service-doctor cmd/service-doctor/main.go

run-service-doctor: ## Launch service-doctor
	@echo "Запуск service-doctor..."
	go run cmd/service-doctor/main.go

build-txn-generator: ## Building txn-generator
	@echo "Сборка txn-generator..."
	go build -o bin/txn-generator cmd/txn-generator/main.go

run-txn-generator: ## Launch txn-generator
	@echo "Запуск txn-generator..."
	go run cmd/txn-generator/main.go			

build-all: build build-alerter build-antifraud build-ch-query-tool build-dlq-tool build-service-doctor build-txn-generator ## Building all services
	@echo "Все сервисы собраны!"

# ---- Commands for a full system startup
start-all: ## Launching the entire system
	@echo "Запуск всей системы Payment-processing-system..."
	@if [ ! -f .env ]; then cp env.example .env; echo "Создан .env файл из примера"; fi
	docker compose up -d
	@echo "Ожидание готовности сервисов..."
	@sleep 15
	@echo "Система запущена!"
	@echo "Payment-processing-system: http://localhost:$(APP_PORT)/health"
	@echo "pgAdmin: http://localhost:$(PGADMIN_PORT:-8082) ($(PGADMIN_EMAIL) / $(PGADMIN_PASSWORD))"
	@echo "Grafana: http://localhost:$(GRAFANA_PORT)"
	@echo "Prometheus: http://localhost:$(PROMETHEUS_PORT)"
	@echo "Jaeger: http://localhost:$(JAEGER_PORT)"

stop-all: ## Stopping the entire system
	@echo "Остановка всей системы..."
	docker compose down

health-check: ## System Health Check
	@echo "Проверка здоровья системы..."
	./scripts/health-check.sh 
