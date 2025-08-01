-- Удаление индекса для fraud_detection
DROP INDEX IF EXISTS idx_transactions_fraud;

-- Удаление полей для обнаружения мошенничества
ALTER TABLE transactions 
DROP COLUMN IF EXISTS is_fraudulent,
DROP COLUMN IF EXISTS fraud_reason,
DROP COLUMN IF EXISTS risk_score; 