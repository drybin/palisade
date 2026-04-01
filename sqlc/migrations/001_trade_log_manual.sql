-- Выполнить вручную на существующей БД (новые инсталляции: см. sqlc/schema.sql).
CREATE TABLE IF NOT EXISTS trade_log_manual (
   id   SERIAL PRIMARY KEY,
   open_date TIMESTAMPTZ      NOT NULL,
   deal_date TIMESTAMPTZ     ,
   close_date TIMESTAMPTZ     ,
   cancel_date TIMESTAMPTZ    ,
   open_balance  DOUBLE PRECISION NOT NULL,
   close_balance  DOUBLE PRECISION,
   symbol  TEXT NOT NULL,
   buy_price  DOUBLE PRECISION NOT NULL,
   sell_price  DOUBLE PRECISION,
   amount  DOUBLE PRECISION NOT NULL,
   orderId TEXT NOT NULL,
   orderId_sell TEXT,
   upLevel  DOUBLE PRECISION NOT NULL,
   downLevel  DOUBLE PRECISION NOT NULL
);
