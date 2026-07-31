CREATE TABLE IF NOT EXISTS paper_trade (
    id              SERIAL PRIMARY KEY,
    symbol          TEXT NOT NULL,
    signal_at       TIMESTAMPTZ NOT NULL,
    status          TEXT NOT NULL,
    entry_price     DOUBLE PRECISION NOT NULL,
    target_price    DOUBLE PRECISION NOT NULL,
    min_exit_price  DOUBLE PRECISION NOT NULL,
    quantity        DOUBLE PRECISION NOT NULL,
    filled_quantity DOUBLE PRECISION NOT NULL DEFAULT 0,
    sold_quantity   DOUBLE PRECISION NOT NULL DEFAULT 0,
    buy_quote      DOUBLE PRECISION NOT NULL DEFAULT 0,
    sell_quote     DOUBLE PRECISION NOT NULL DEFAULT 0,
    fees            DOUBLE PRECISION NOT NULL DEFAULT 0,
    pnl             DOUBLE PRECISION NOT NULL DEFAULT 0,
    opened_at       TIMESTAMPTZ,
    closed_at       TIMESTAMPTZ,
    exit_reason     TEXT NOT NULL DEFAULT '',
    last_price      DOUBLE PRECISION NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS paper_trade_symbol_status_idx ON paper_trade (symbol, status);
