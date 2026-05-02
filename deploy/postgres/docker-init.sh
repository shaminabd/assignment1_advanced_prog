#!/usr/bin/env bash
set -euo pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<-EOSQL
CREATE DATABASE order_db;
CREATE DATABASE payment_db;
CREATE DATABASE notification_db;
EOSQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname order_db <<-EOSQL
CREATE TABLE IF NOT EXISTS orders (
    id VARCHAR(36) PRIMARY KEY,
    customer_id VARCHAR(255) NOT NULL,
    item_name VARCHAR(255) NOT NULL,
    amount BIGINT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'Pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    idempotency_key VARCHAR(255) UNIQUE
);
EOSQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname payment_db <<-EOSQL
CREATE TABLE IF NOT EXISTS payments (
    id VARCHAR(36) PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL,
    transaction_id VARCHAR(36) NOT NULL,
    amount BIGINT NOT NULL,
    status VARCHAR(50) NOT NULL
);
EOSQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname notification_db <<-EOSQL
CREATE TABLE IF NOT EXISTS processed_events (
    event_id UUID PRIMARY KEY,
    processed_at TIMESTAMP NOT NULL DEFAULT NOW()
);
EOSQL
