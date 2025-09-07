# Миграции базы данных

Этот проект использует [golang-migrate](https://github.com/golang-migrate/migrate) для управления миграциями базы данных.

## Установка golang-migrate

```bash
# Установка с поддержкой PostgreSQL
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

## Структура миграций

Миграции находятся в папке `migrations/` и следуют формату:

- `000001_<название>_up.sql` - миграция для применения
- `000001_<название>_down.sql` - миграция для отката

## Основные команды

### Показать справку

```bash
make help
```

### Применить все миграции

```bash
make migrate-up
```

### Откатить все миграции

```bash
make migrate-down
```

### Показать текущую версию

```bash
make migrate-version
```

### Создать новую миграцию

```bash
make migrate-create NAME=add_users_table
```

### Принудительно установить версию (осторожно!)

```bash
make migrate-force VERSION=2
```

## Настройка окружения

### Переменные окружения

Вы можете настроить подключение к базе данных через переменные:

```bash
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=your_password
export POSTGRES_DB=payment_gateway
export DB_SSL_MODE=disable
```

### Настройка для разработки

```bash
# Создание базы данных и применение миграций
make dev-setup

# Сброс базы данных (удаление и пересоздание)
make dev-reset
```

## Существующие миграции

### 000001_create_transactions_table

Создает основную таблицу транзакций с полями:

- `id` - UUID первичный ключ
- `status` - статус транзакции
- `amount` - сумма транзакции
- `currency` - валюта
- `card_number_hash` - хэш номера карты
- `idempotency_key` - ключ идемпотентности
- `created_at` - время создания
- `updated_at` - время обновления

### 000002_create_transaction_statuses_table

Создает таблицу для истории статусов транзакций:

- Связь с основной таблицей транзакций
- Enum для статусов (processing, completed, failed, cancelled)
- Поле для причины изменения статуса

## Лучшие практики

1. **Всегда создавайте пару файлов** - `.up.sql` и `.down.sql`
2. **Используйте транзакции** в миграциях для атомарности
3. **Тестируйте откат** миграций перед применением в продакшене
4. **Документируйте сложные миграции** в комментариях
5. **Используйте IF NOT EXISTS** для безопасного создания объектов
6. **Создавайте индексы** для оптимизации запросов

## Пример создания новой миграции

```bash
# Создание миграции
make migrate-create NAME=add_merchant_table

# Редактирование созданных файлов
# migrations/000003_add_merchant_table.up.sql
# migrations/000003_add_merchant_table.down.sql

# Применение миграции
make migrate-up
```

## Отладка

Если миграция не применяется:

1. Проверьте подключение к базе данных
2. Убедитесь, что golang-migrate установлен с поддержкой PostgreSQL
3. Проверьте синтаксис SQL в файлах миграций
4. Используйте `make migrate-version` для проверки текущего состояния
