CREATE TABLE IF NOT EXISTS market_snapshot (
    symbol               TEXT PRIMARY KEY,
    collected_at         TIMESTAMPTZ NOT NULL,
    last_price           DOUBLE PRECISION NOT NULL,
    bid_price            DOUBLE PRECISION NOT NULL,
    bid_qty              DOUBLE PRECISION NOT NULL,
    ask_price            DOUBLE PRECISION NOT NULL,
    ask_qty              DOUBLE PRECISION NOT NULL,
    quote_volume_24h     DOUBLE PRECISION NOT NULL,
    price_change_percent DOUBLE PRECISION NOT NULL
);

CREATE TABLE IF NOT EXISTS palisade_signal (
    symbol       TEXT PRIMARY KEY,
    sent_at      TIMESTAMPTZ NOT NULL,
    score        DOUBLE PRECISION NOT NULL
);
