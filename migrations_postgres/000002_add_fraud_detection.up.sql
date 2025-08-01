-- Добавление полей для обнаружения мошенничества
ALTER TABLE transactions 
ADD COLUMN IF NOT EXISTS is_fraudulent BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS fraud_reason VARCHAR(255),
ADD COLUMN IF NOT EXISTS risk_score DECIMAL(3,2) DEFAULT 0.0;

-- Создание индекса для fraud_detection
CREATE INDEX IF NOT EXISTS idx_transactions_fraud ON transactions(is_fraudulent, risk_score); 