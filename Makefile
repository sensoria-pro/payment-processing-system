# Загрузка переменных окружения из .env файла
ifneq (,$(wildcard .env))
    include .env
	export $(shell sed 's/=.*//' .env 2>/dev/null || echo "")	
endif

# Переменные для подключения к базе данных (с fallback значениями)
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_USER ?= postgres
DB_PASSWORD ?= password
DB_NAME ?= payment_system
DB_SSL_MODE ?= disable

# Строка подключения к базе данных
DATABASE_URL = postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)

# Путь к миграциям
MIGRATIONS_PATH = migrations_postgres

.PHONY: help migrate-up migrate-down migrate-force migrate-version migrate-create

help: ## Показать справку
	@echo "Доступные команды:"
	@echo "=================="
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | sort | awk -F ':.*## ' '{printf "make %-20s - %s\n", $$1, $$2}'

migrate-up: ## Применить все миграции
	@if [ "$(DB_PASSWORD)" = "password" ]; then \
        echo "❌ Опасность: используется пароль по умолчанию 'password'"; \
        echo "💡 Установите DB_PASSWORD в .env или через export"; \
        exit 1; \
    fi
    @echo "Применение миграций..."
    migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" up

migrate-down: ## Откатить все миграции
	@echo "Откат всех миграций..."
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" down

migrate-force: ## Принудительно установить версию миграции (использовать с осторожностью)
	@echo "Укажите версию: make migrate-force VERSION=<номер_версии>"
	@if [ -z "$(VERSION)" ]; then echo "Ошибка: не указана версия"; exit 1; fi
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" force $(VERSION)

migrate-version: ## Показать текущую версию миграции
	@echo "Текущая версия миграции:"
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" version

migrate-create: ## Создать новую миграцию
	@echo "Укажите имя миграции: make migrate-create NAME=<имя_миграции>"
	@if [ -z "$(NAME)" ]; then echo "Ошибка: не указано имя миграции"; exit 1; fi
	migrate create -ext sql -dir $(MIGRATIONS_PATH) -seq $(NAME).sql

migrate-status: ## Показать статус миграций
	@echo "Статус миграций:"
	migrate -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" version



# Команды для тестирования
test-migrate: ## Запуск тестов с миграциями
	@echo "Запуск тестов..."
	go test ./...

# Команды для продакшена
prod-migrate: ## Применение миграций в продакшене
    @echo "⚠️  Применение миграций в продакшене!"
    @echo "❗ Убедитесь, что у вас есть резервная копия!"
    @read -p "Продолжить? (y/N): " -n 1 -r; echo
    @if [[ ! $$REPLY =~ ^[Yy]$$ ]]; then exit 1; fi
    $(MAKE) migrate-up

# Команды для запуска приложения
run: ## Запуск приложения
	@echo "Запуск payment-processing-system..."
	./scripts/run.sh

run-dev: ## Запуск приложения в режиме разработки
	@echo "Запуск payment-processing-system в режиме разработки..."
	$(MAKE) dev-docker
	@echo "Запуск приложения..."
	go run cmd/main.go

test-api: ## Тестирование API
	@echo "Тестирование API..."
	./scripts/test-api.sh

build: ## Сборка приложения
	@echo "Сборка payment-processing-system..."
	go build -o bin/payment-processing-system cmd/main.go

# Команды для Docker
docker-up: ## Запуск всех Docker контейнеров
	@echo "Запуск всех Docker контейнеров..."
	docker compose up -d
	@echo "Ожидание готовности сервисов..."
	@sleep 10
	@echo "Все контейнеры запущены!"

docker-down: ## Остановка Docker контейнеров
	@echo "Остановка Docker контейнеров..."
	docker compose down

docker-logs: ## Просмотр логов Docker контейнеров
	docker compose logs -f

docker-reset: ## Сброс Docker контейнеров и данных
	@echo "Сброс Docker контейнеров и данных..."
	docker compose down -v
	$(MAKE) docker-up

# Команды для разработки с Docker
dev-docker: ## Полная настройка окружения с Docker
	@echo "Настройка окружения для разработки с Docker..."
	@if [ ! -f .env ]; then cp env.example .env; echo "Создан .env файл из примера"; fi
	$(MAKE) docker-up
	@echo "Окружение готово!"
	@echo "Payment-processing-system: http://localhost:$(APP_PORT)/health"
	@echo "pgAdmin: http://localhost:$(PGADMIN_PORT:-8082) ($(PGADMIN_EMAIL) / $(PGADMIN_PASSWORD))"
	@echo "Grafana: http://localhost:$(GRAFANA_PORT)"
	@echo "Prometheus: http://localhost:$(PROMETHEUS_PORT)"
	@echo "Jaeger: http://localhost:$(JAEGER_PORT)"

wait-for-db: ## Ждать готовности PostgreSQL
    @echo "Ожидание готовности PostgreSQL..."
    @until docker exec -i postgres-db pg_isready -U $(DB_USER) -d $(DB_NAME); do \
        echo "⏳ PostgreSQL недоступен, ждём..."; \
        sleep 2; \
    done
    @echo "✅ PostgreSQL готов!"

# Команды для разработки
dev-setup: ## Настройка окружения для разработки
    @echo "Настройка окружения для разработки..."
    @if [ ! -f .env ]; then cp env.example .env; echo "Создан .env файл из примера"; fi
    @echo "Запуск PostgreSQL..."
    $(MAKE) docker-up
    @echo "Ожидание готовности PostgreSQL..."
    $(MAKE) wait-for-db
    @echo "Создание базы данных..."
    docker exec -i postgres-db createdb -U $(DB_USER) $(DB_NAME) || echo "База уже существует"
    $(MAKE) migrate-up

dev-reset: ## Сброс базы данных для разработки
	@echo "Сброс базы данных..."
	dropdb $(DB_NAME) 2>/dev/null || echo "База данных не существует"
	createdb $(DB_NAME)
	$(MAKE) migrate-up

# Команды для всех сервисов
build-alerter: ## Сборка alerter-service
	@echo "Сборка alerter-service..."
	go build -o bin/alerter-service cmd/alerter-service/main.go

run-alerter: ## Запуск alerter-service
	@echo "Запуск alerter-service..."
	go run cmd/alerter-service/main.go	

build-antifraud: ## Сборка anti-fraud-analyzer
	@echo "Сборка anti-fraud-analyzer..."
	go build -o bin/anti-fraud-analyzer cmd/anti-fraud-analyzer/main.go

run-antifraud: ## Запуск anti-fraud-analyzer
	@echo "Запуск anti-fraud-analyzer..."
	go run cmd/anti-fraud-analyzer/main.go

build-ch-query-tool: ## Сборка ch-query-tool
	@echo "Сборка ch-query-tool..."
	go build -o bin/ch-query-tool cmd/ch-query-tool/main.go

run-ch-query-tool: ## Запуск ch-query-tool
	@echo "Запуск ch-query-tool..."
	go run cmd/ch-query-tool/main.go

build-dlq-tool: ## Сборка dlq-tool
	@echo "Сборка dlq-tool..."
	go build -o bin/dlq-tool cmd/dlq-tool/main.go

run-dlq-tool: ## Запуск dlq-tool
	@echo "Запуск dlq-tool..."
	go run cmd/dlq-tool/main.go

build-service-doctor: ## Сборка service-doctor
	@echo "Сборка service-doctor..."
	go build -o bin/service-doctor cmd/service-doctor/main.go

run-service-doctor: ## Запуск service-doctor
	@echo "Запуск service-doctor..."
	go run cmd/service-doctor/main.go

build-txn-generator: ## Сборка txn-generator
	@echo "Сборка txn-generator..."
	go build -o bin/txn-generator cmd/txn-generator/main.go

run-txn-generator: ## Запуск txn-generator
	@echo "Запуск txn-generator..."
	go run cmd/txn-generator/main.go			

build-all: build build-alerter build-antifraud build-ch-query-tool build-dlq-tool build-service-doctor build-txn-generator ## Сборка всех сервисов
	@echo "Все сервисы собраны!"

# Команды для полного запуска системы
start-all: ## Запуск всей системы
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

stop-all: ## Остановка всей системы
	@echo "Остановка всей системы..."
	docker compose down

health-check: ## Проверка здоровья системы
	@echo "Проверка здоровья системы..."
	./scripts/health-check.sh 