CREATE TABLE IF NOT EXISTS palisade_order_intent (
    id                    SERIAL PRIMARY KEY,
    client_order_id       TEXT NOT NULL UNIQUE,
    symbol                TEXT NOT NULL,
    side                  TEXT NOT NULL,
    price                 DOUBLE PRECISION NOT NULL,
    quantity              DOUBLE PRECISION NOT NULL,
    open_balance          DOUBLE PRECISION NOT NULL DEFAULT 0,
    target_price          DOUBLE PRECISION NOT NULL DEFAULT 0,
    status                TEXT NOT NULL,
    exchange_order_id     TEXT NOT NULL DEFAULT '',
    trade_id              INT,
    executed_quantity     DOUBLE PRECISION NOT NULL DEFAULT 0,
    cumulative_quote_qty DOUBLE PRECISION NOT NULL DEFAULT 0,
    last_error            TEXT NOT NULL DEFAULT '',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS palisade_order_intent_recovery_idx
    ON palisade_order_intent (status, updated_at);
