CREATE TABLE state (
     id   SERIAL PRIMARY KEY, -- id
     date TIMESTAMPTZ      NOT NULL, --реальная дата
     account_balance  DOUBLE PRECISION NOT NULL, --баланс usdt на бирже
     coinFirst  TEXT NOT NULL, -- монета c которой совершаем операцию
     coinSecond  TEXT NOT NULL, -- монета к которой совершаем операцию
     price  DOUBLE PRECISION NOT NULL, -- цена монеты
     amount  DOUBLE PRECISION NOT NULL, -- объем монеты
     state TEXT NOT NULL, -- состояние
     orderId TEXT NOT NULL, -- состояние
     upLevel  DOUBLE PRECISION NOT NULL, -- верхний уровень коридора
     downLevel  DOUBLE PRECISION NOT NULL -- нижний уровень коридора
);

CREATE TABLE logs (
   id   SERIAL PRIMARY KEY, -- id
   date TIMESTAMPTZ      NOT NULL, --реальная дата
   account_balance  DOUBLE PRECISION NOT NULL, --баланс usdt на бирже
   coinFirst  TEXT NOT NULL, -- монета c которой совершаем операцию
   coinSecond  TEXT NOT NULL, -- монета к которой совершаем операцию
   price  DOUBLE PRECISION NOT NULL, -- цена монеты
   amount  DOUBLE PRECISION NOT NULL, -- объем монеты
   state TEXT NOT NULL, -- состояние
   orderId TEXT NOT NULL, -- состояние
   upLevel  DOUBLE PRECISION NOT NULL, -- верхний уровень коридора
   downLevel  DOUBLE PRECISION NOT NULL -- нижний уровень коридора
);

CREATE TABLE trade_log (
   id   SERIAL PRIMARY KEY, -- id
   open_date TIMESTAMPTZ      NOT NULL, --дата открытия ордера
   deal_date TIMESTAMPTZ     , --дата принятия ордера
   close_date TIMESTAMPTZ     , --дата закрытия ордера
   cancel_date TIMESTAMPTZ    , --дата отмены ордера
   open_balance  DOUBLE PRECISION NOT NULL, --баланс usdt на бирже при открытии ордера
   close_balance  DOUBLE PRECISION, --баланс usdt на бирже при закрытии ордера
   symbol  TEXT NOT NULL, -- монета c которой совершаем операцию
   buy_price  DOUBLE PRECISION NOT NULL, -- цена покупки
   sell_price  DOUBLE PRECISION, -- цена продажи
   amount  DOUBLE PRECISION NOT NULL, -- объем монеты
   orderId TEXT NOT NULL, -- id ордера на бирже
   orderId_sell TEXT,  -- id ордера на продажу на бирже
   upLevel  DOUBLE PRECISION NOT NULL, -- верхний уровень коридора
   downLevel  DOUBLE PRECISION NOT NULL -- нижний уровень коридора
);

CREATE TABLE trade_log_manual (
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

CREATE TABLE coins
(
    id                         SERIAL PRIMARY KEY,        -- id
    date                       TIMESTAMPTZ      NOT NULL, --дата проверки
    symbol                     TEXT             NOT NULL, -- монета c которой совершаем операцию
    status                     SERIAL           NOT NULL, -- status
    baseAsset                  TEXT             NOT NULL,
    baseAssetPrecision         DOUBLE PRECISION NOT NULL,
    quoteAsset                 TEXT             NOT NULL,
    quotePrecision             SERIAL           NOT NULL, -- status
    quoteAssetPrecision        SERIAL           NOT NULL, -- status
    baseCommissionPrecision    SERIAL           NOT NULL, -- status
    quoteCommissionPrecision   SERIAL           NOT NULL, -- status
    orderTypes                 TEXT[],
    isSpotTradingAllowed       BOOLEAN NOT NULL,
    isMarginTradingAllowed     BOOLEAN NOT NULL,
    quoteAmountPrecision       DOUBLE PRECISION NOT NULL,
    baseSizePrecision          DOUBLE PRECISION NOT NULL,
    permissions                TEXT[],
    maxQuoteAmount             SERIAL NOT NULL,
    makerCommission            DOUBLE PRECISION NOT NULL,
    takerCommission            DOUBLE PRECISION NOT NULL,
    quoteAmountPrecisionMarket DOUBLE PRECISION NOT NULL,
    maxQuoteAmountMarket       SERIAL NOT NULL,
    fullName                   TEXT             NOT NULL,
    tradeSideType              SERIAL           NOT NULL,
    isPalisade                 BOOLEAN NOT NULL,
    lastCheck                  TIMESTAMPTZ,
    support       DOUBLE PRECISION, -- нижняя граница
    resistance       DOUBLE PRECISION, -- верхняя граница
    rangeValue       DOUBLE PRECISION, -- диапазон между границами
    rangePercent     DOUBLE PRECISION, -- диапазон в процентах
    avgPrice         DOUBLE PRECISION, -- средняя цена
    volatility       DOUBLE PRECISION, -- волатильность в процентах
    maxDrawdown      DOUBLE PRECISION, -- максимальная просадка в процентах
    maxRise          DOUBLE PRECISION -- максимальный рост в процентах
);

CREATE TABLE market_daily_bar (
    symbol  TEXT NOT NULL,
    day_utc DATE NOT NULL,
    close   DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (symbol, day_utc)
);

CREATE TABLE market_minute_bar (
    symbol    TEXT NOT NULL,
    open_time TIMESTAMPTZ NOT NULL,
    open      DOUBLE PRECISION NOT NULL,
    high      DOUBLE PRECISION NOT NULL,
    low       DOUBLE PRECISION NOT NULL,
    close     DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (symbol, open_time)
);

CREATE INDEX market_minute_bar_symbol_open_time_idx ON market_minute_bar (symbol, open_time);

CREATE TABLE market_snapshot (
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

CREATE TABLE palisade_signal (
	symbol                TEXT PRIMARY KEY,
	sent_at               TIMESTAMPTZ NOT NULL,
	score                 DOUBLE PRECISION NOT NULL,
	entry_price           DOUBLE PRECISION NOT NULL DEFAULT 0,
	target_price          DOUBLE PRECISION NOT NULL DEFAULT 0,
	min_exit_price        DOUBLE PRECISION NOT NULL DEFAULT 0,
	net_profit            DOUBLE PRECISION NOT NULL DEFAULT 0,
	status                TEXT NOT NULL DEFAULT 'ACTIVE',
	invalidation_reason   TEXT NOT NULL DEFAULT '',
	valid_until           TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE palisade_order_intent (
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

CREATE INDEX palisade_order_intent_recovery_idx
    ON palisade_order_intent (status, updated_at);

CREATE TABLE paper_trade (
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

CREATE INDEX paper_trade_symbol_status_idx ON paper_trade (symbol, status);

CREATE TABLE trend_retest_state (
    symbol                  TEXT NOT NULL,
    sma_period              INT NOT NULL,
    day_utc                 DATE NOT NULL,
    wait_retest             BOOLEAN NOT NULL DEFAULT false,
    retest_until            TIMESTAMPTZ,
    last_processed_open_time TIMESTAMPTZ,
    PRIMARY KEY (symbol, sma_period, day_utc)
);

CREATE TABLE trend_signal_sent (
    id          SERIAL PRIMARY KEY,
    symbol      TEXT NOT NULL,
    sma_period  INT NOT NULL,
    day_utc     DATE NOT NULL,
    signal_kind TEXT NOT NULL,
    sent_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (symbol, sma_period, day_utc, signal_kind)
);

-- drop table state;
-- drop table logs;
