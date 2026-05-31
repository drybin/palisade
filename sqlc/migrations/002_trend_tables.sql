-- Выполнить вручную на существующей БД (новые инсталляции: см. sqlc/schema.sql).
CREATE TABLE IF NOT EXISTS market_daily_bar (
    symbol  TEXT NOT NULL,
    day_utc DATE NOT NULL,
    close   DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (symbol, day_utc)
);

CREATE TABLE IF NOT EXISTS market_minute_bar (
    symbol    TEXT NOT NULL,
    open_time TIMESTAMPTZ NOT NULL,
    open      DOUBLE PRECISION NOT NULL,
    high      DOUBLE PRECISION NOT NULL,
    low       DOUBLE PRECISION NOT NULL,
    close     DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (symbol, open_time)
);

CREATE INDEX IF NOT EXISTS market_minute_bar_symbol_open_time_idx ON market_minute_bar (symbol, open_time);

CREATE TABLE IF NOT EXISTS trend_retest_state (
    symbol                   TEXT NOT NULL,
    sma_period               INT NOT NULL,
    day_utc                  DATE NOT NULL,
    wait_retest              BOOLEAN NOT NULL DEFAULT false,
    retest_until             TIMESTAMPTZ,
    last_processed_open_time TIMESTAMPTZ,
    PRIMARY KEY (symbol, sma_period, day_utc)
);

CREATE TABLE IF NOT EXISTS trend_signal_sent (
    id          SERIAL PRIMARY KEY,
    symbol      TEXT NOT NULL,
    sma_period  INT NOT NULL,
    day_utc     DATE NOT NULL,
    signal_kind TEXT NOT NULL,
    sent_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (symbol, sma_period, day_utc, signal_kind)
);
