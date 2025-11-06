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
-- drop table state;
-- drop table logs;