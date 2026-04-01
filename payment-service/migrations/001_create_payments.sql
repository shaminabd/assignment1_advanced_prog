CREATE TABLE IF NOT EXISTS payments (
    id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL,
    transaction_id VARCHAR(36) NOT NULL,
    amount BIGINT NOT NULL,
    status VARCHAR(50) NOT NULL
);
