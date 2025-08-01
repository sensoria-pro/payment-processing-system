-- Удаление триггера
DROP TRIGGER IF EXISTS update_transactions_updated_at ON transactions;

-- Удаление функции
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Удаление индексов
DROP INDEX IF EXISTS idx_transactions_card_hash;
DROP INDEX IF EXISTS idx_transactions_created_at;
DROP INDEX IF EXISTS idx_transactions_status;

-- Удаление таблицы
DROP TABLE IF EXISTS transactions; 