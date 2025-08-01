CREATE TABLE IF NOT EXISTS default.fraud_reports (
    transaction_id UUID,
    is_fraudulent  UInt8,
    reason         String,
    card_hash      String,
    amount         Float64,
    processed_at   DateTime
) ENGINE = MergeTree()
ORDER BY (processed_at, transaction_id); 