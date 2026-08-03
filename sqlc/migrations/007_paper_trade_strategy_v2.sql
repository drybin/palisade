ALTER TABLE paper_trade
    ADD COLUMN IF NOT EXISTS strategy_version INT NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS paper_trade_strategy_status_idx
    ON paper_trade (strategy_version, status);

CREATE INDEX IF NOT EXISTS paper_trade_signal_idx
    ON paper_trade (strategy_version, symbol, signal_at);
