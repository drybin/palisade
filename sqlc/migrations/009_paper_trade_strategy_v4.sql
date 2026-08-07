ALTER TABLE paper_trade
    ADD COLUMN IF NOT EXISTS break_even_armed BOOLEAN NOT NULL DEFAULT false;
