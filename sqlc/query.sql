-- name: GetCoinState :one
SELECT * FROM state
WHERE coinFirst = $1 AND coinSecond = $2 LIMIT 1;

-- name: GetCountLogsByCoin :one
SELECT COUNT(*) FROM logs
WHERE coinFirst = $1 AND coinSecond = $2 LIMIT 1;

-- name: SaveCoin :one
INSERT INTO
    coins (
    date,
    symbol,
    status,
    baseAsset,
    baseAssetPrecision,
    quoteAsset,
    quotePrecision,
    quoteAssetPrecision,
    baseCommissionPrecision,
    quoteCommissionPrecision,
    orderTypes,
    isSpotTradingAllowed,
    isMarginTradingAllowed,
    quoteAmountPrecision,
    baseSizePrecision,
    permissions,
    maxQuoteAmount,
    makerCommission,
    takerCommission,
    quoteAmountPrecisionMarket,
    maxQuoteAmountMarket,
    fullName,
    tradeSideType,
    isPalisade
)
VALUES (
           $1,
           $2,
           $3,
           $4,
           $5,
           $6,
           $7,
           $8,
           $9,
           $10,
           $11,
           $12,
           $13,
           $14,
           $15,
           $16,
           $17,
           $18,
           $19,
           $20,
           $21,
           $22,
           $23,
           $24
       )
    RETURNING *;

-- name: GetCoinInfo :one
SELECT * FROM coins
WHERE symbol = $1 LIMIT 1;

-- name: GetCoins :many
SELECT * FROM coins
WHERE 
    ($3::boolean IS NULL OR isSpotTradingAllowed = $3)
    AND ($4::boolean IS NULL OR isPalisade = $4)
ORDER BY id
LIMIT $1
OFFSET $2;

-- name: UpdateIsPalisade :exec
UPDATE coins
SET isPalisade = $1, lastCheck = $2
WHERE symbol = $3;

-- name: UpdatePalisadeParams :exec
UPDATE coins
SET support = $1, resistance = $2, rangeValue = $3, rangePercent = $4, avgPrice = $5, volatility = $6, maxDrawdown = $7, maxRise = $8
WHERE symbol = $9;

-- name: GetCoinsToProcessTPTU :many
SELECT * FROM coins
WHERE 
    isSpotTradingAllowed = true
    AND isPalisade = true
    --AND volatility > 0.1
    --AND volatility < 0.4
    AND quoteasset = 'USDT'
    AND symbol = 'TPTUUSDT'
    --AND symbol = 'LTCUSDT'
ORDER BY lastcheck DESC
LIMIT $1
OFFSET $2;

-- name: GetCoinsToProcess :many
SELECT * FROM coins
WHERE 
    isSpotTradingAllowed = true
    AND isPalisade = true
    AND volatility > 0.2
    AND volatility < 0.5
    AND quoteasset = 'USDT'
ORDER BY lastcheck DESC
LIMIT $1
OFFSET $2;

-- name: SaveTradeLog :one
INSERT INTO trade_log (
   open_date,
   open_balance,
   symbol,
   buy_price,
   amount,
   orderId,
   upLevel,
   downLevel
   )
   VALUES (
           $1,
           $2,
           $3,
           $4,
           $5,
           $6,
           $7,
           $8
   )
   RETURNING *;

-- name: UpdateDealDateTradeLog :exec
UPDATE trade_log
SET deal_date = $1
WHERE id = $2;

-- name: UpdateCancelDateTradeLog :exec
UPDATE trade_log
SET cancel_date = $1
WHERE id = $2;

-- name: UpdateSellOrderIdTradeLog :exec
UPDATE trade_log
SET orderId_sell = $1
WHERE id = $2;

-- name: UpdateAmountTradeLog :exec
UPDATE trade_log
SET amount = $1
WHERE id = $2;

-- name: UpdateSuccesTradeLog :exec
UPDATE trade_log
SET close_date = $1, close_balance = $2, sell_price = $3
WHERE id = $4;

-- name: GetLastTradeId :one
SELECT MAX(id) FROM trade_log;

-- name: GetOpenOrders :many
SELECT * FROM trade_log
WHERE 
    close_date IS NULL
    AND cancel_date IS NULL;

-- name: SaveTradeLogManual :one
INSERT INTO trade_log_manual (
   open_date,
   open_balance,
   symbol,
   buy_price,
   amount,
   orderId,
   upLevel,
   downLevel
   )
   VALUES (
           $1,
           $2,
           $3,
           $4,
           $5,
           $6,
           $7,
           $8
   )
   RETURNING *;

-- name: UpdateDealDateTradeLogManual :exec
UPDATE trade_log_manual
SET deal_date = $1
WHERE id = $2;

-- name: UpdateCancelDateTradeLogManual :exec
UPDATE trade_log_manual
SET cancel_date = $1
WHERE id = $2;

-- name: UpdateSellOrderIdTradeLogManual :exec
UPDATE trade_log_manual
SET orderId_sell = $1
WHERE id = $2;

-- name: UpdateAmountTradeLogManual :exec
UPDATE trade_log_manual
SET amount = $1
WHERE id = $2;

-- name: UpdateSuccesTradeLogManual :exec
UPDATE trade_log_manual
SET close_date = $1, close_balance = $2, sell_price = $3
WHERE id = $4;

-- name: GetLastTradeIdManual :one
SELECT COALESCE(MAX(id), 0) FROM trade_log_manual;

-- name: GetOpenOrdersManual :many
SELECT * FROM trade_log_manual
WHERE 
    close_date IS NULL
    AND cancel_date IS NULL;

-- name: GetTradeLogManualById :one
SELECT * FROM trade_log_manual WHERE id = $1;

-- name: UpsertMarketDailyBar :exec
INSERT INTO market_daily_bar (symbol, day_utc, close)
VALUES ($1, $2, $3)
ON CONFLICT (symbol, day_utc) DO UPDATE SET close = EXCLUDED.close;

-- name: CountMarketDailyBars :one
SELECT COUNT(*)::bigint FROM market_daily_bar WHERE symbol = $1;

-- name: ListMarketDailyBars :many
SELECT symbol, day_utc, close FROM market_daily_bar
WHERE symbol = $1
ORDER BY day_utc ASC;

-- name: UpsertMarketMinuteBar :exec
INSERT INTO market_minute_bar (symbol, open_time, open, high, low, close)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (symbol, open_time) DO UPDATE SET
    open = EXCLUDED.open,
    high = EXCLUDED.high,
    low = EXCLUDED.low,
    close = EXCLUDED.close;

-- name: GetLastMinuteBarOpenTime :one
SELECT open_time FROM market_minute_bar
WHERE symbol = $1
ORDER BY open_time DESC
LIMIT 1;

-- name: ListMarketMinuteBarsFrom :many
SELECT symbol, open_time, open, high, low, close FROM market_minute_bar
WHERE symbol = $1 AND open_time >= $2
ORDER BY open_time ASC;

-- name: GetTrendRetestState :one
SELECT * FROM trend_retest_state
WHERE symbol = $1 AND sma_period = $2 AND day_utc = $3;

-- name: UpsertTrendRetestState :exec
INSERT INTO trend_retest_state (
    symbol, sma_period, day_utc, wait_retest, retest_until, last_processed_open_time
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (symbol, sma_period, day_utc) DO UPDATE SET
    wait_retest = EXCLUDED.wait_retest,
    retest_until = EXCLUDED.retest_until,
    last_processed_open_time = EXCLUDED.last_processed_open_time;

-- name: TrendSignalWasSent :one
SELECT EXISTS(
    SELECT 1 FROM trend_signal_sent
    WHERE symbol = $1 AND sma_period = $2 AND day_utc = $3 AND signal_kind = $4
) AS sent;

-- name: InsertTrendSignalSent :exec
INSERT INTO trend_signal_sent (symbol, sma_period, day_utc, signal_kind)
VALUES ($1, $2, $3, $4);
