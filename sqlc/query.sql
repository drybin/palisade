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

-- name: UpdateTradeLevels :exec
UPDATE trade_log
SET upLevel = $1, downLevel = $2
WHERE id = $3;

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

-- name: UpdateTradeFill :exec
UPDATE trade_log
SET buy_price = $1, amount = $2
WHERE id = $3;

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

-- name: UpsertMarketSnapshot :exec
INSERT INTO market_snapshot (
    symbol, collected_at, last_price, bid_price, bid_qty, ask_price, ask_qty,
    quote_volume_24h, price_change_percent
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (symbol) DO UPDATE SET
    collected_at = EXCLUDED.collected_at,
    last_price = EXCLUDED.last_price,
    bid_price = EXCLUDED.bid_price,
    bid_qty = EXCLUDED.bid_qty,
    ask_price = EXCLUDED.ask_price,
    ask_qty = EXCLUDED.ask_qty,
    quote_volume_24h = EXCLUDED.quote_volume_24h,
    price_change_percent = EXCLUDED.price_change_percent;

-- name: ListMarketSnapshots :many
SELECT symbol, collected_at, last_price, bid_price, bid_qty, ask_price, ask_qty,
       quote_volume_24h, price_change_percent
FROM market_snapshot
ORDER BY quote_volume_24h DESC;

-- name: GetLastPalisadeSignal :one
SELECT sent_at FROM palisade_signal WHERE symbol = $1;

-- name: SavePalisadeSignal :exec
INSERT INTO palisade_signal (symbol, sent_at, score)
VALUES ($1, $2, $3)
ON CONFLICT (symbol) DO UPDATE SET sent_at = EXCLUDED.sent_at, score = EXCLUDED.score;

-- name: SavePalisadeSignalState :exec
UPDATE palisade_signal
SET strategy_version = $2,
    support_price = $3,
    entry_price = $4,
    target_price = $5,
    min_exit_price = $6,
    net_profit = $7,
    score = $8,
    status = $9,
    invalidation_reason = $10,
    valid_until = $11,
    updated_at = $12
WHERE symbol = $1;

-- name: ListActivePalisadeSignals :many
SELECT symbol, sent_at, strategy_version, support_price, entry_price, target_price, min_exit_price, net_profit, score,
       status, invalidation_reason, valid_until, updated_at
FROM palisade_signal
WHERE status = 'ACTIVE'
  AND valid_until > now()
  AND target_price >= min_exit_price
ORDER BY score DESC, updated_at DESC;

-- name: CreateOrderIntent :one
INSERT INTO palisade_order_intent (
    client_order_id, symbol, side, price, quantity, open_balance, target_price, status,
    exchange_order_id, trade_id, executed_quantity, cumulative_quote_qty, last_error,
    created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING *;

-- name: UpdateOrderIntent :exec
UPDATE palisade_order_intent
SET status = $2,
    exchange_order_id = $3,
    executed_quantity = $4,
    cumulative_quote_qty = $5,
    last_error = $6,
    updated_at = now()
WHERE id = $1;

-- name: UpdateOrderIntentTradeID :exec
UPDATE palisade_order_intent SET trade_id = $2, updated_at = now() WHERE id = $1;

-- name: ListRecoverableOrderIntents :many
SELECT * FROM palisade_order_intent
WHERE status IN ('PLACING', 'UNKNOWN', 'ACKNOWLEDGED', 'RECOVERY_REQUIRED')
ORDER BY created_at;

-- name: ListOrderIntentsByTradeID :many
SELECT * FROM palisade_order_intent
WHERE trade_id = $1
ORDER BY id;

-- name: GetOpenPaperTradeBySymbol :one
SELECT * FROM paper_trade
WHERE symbol = $1
  AND strategy_version = $2
  AND status IN ('BUY_PENDING', 'PULLBACK_SEEN', 'POSITION_OPEN', 'SELL_PENDING')
ORDER BY id DESC
LIMIT 1;

-- name: GetPaperTradeBySignal :one
SELECT * FROM paper_trade
WHERE symbol = $1
  AND signal_at = $2
  AND strategy_version = $3
ORDER BY id DESC
LIMIT 1;

-- name: CreatePaperTrade :one
INSERT INTO paper_trade (
    strategy_version, symbol, signal_at, status, entry_mode, support_price, entry_price, target_price, min_exit_price, expected_net_profit, break_even_armed, max_bid_price, min_bid_price, entry_low_price, partial_profit_taken, quantity,
    filled_quantity, sold_quantity, buy_quote, sell_quote, fees, pnl, opened_at,
    closed_at, exit_reason, last_price, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27)
RETURNING *;

-- name: ListOpenPaperTrades :many
SELECT * FROM paper_trade
WHERE status IN ('BUY_PENDING', 'PULLBACK_SEEN', 'POSITION_OPEN', 'SELL_PENDING')
  AND strategy_version = $1
ORDER BY id;

-- name: UpdatePaperTrade :exec
UPDATE paper_trade SET
    status = $2,
    target_price = $3,
    break_even_armed = $4,
    max_bid_price = $5,
    min_bid_price = $6,
    entry_low_price = $7,
    partial_profit_taken = $8,
    filled_quantity = $9,
    sold_quantity = $10,
    buy_quote = $11,
    sell_quote = $12,
    fees = $13,
    pnl = $14,
    opened_at = $15,
    closed_at = $16,
    exit_reason = $17,
    last_price = $18,
    updated_at = $19
WHERE id = $1;

-- name: GetPaperTradeStats :one
SELECT
    COUNT(*)::int AS total,
    COUNT(*) FILTER (WHERE status = 'CLOSED')::int AS closed,
    COUNT(*) FILTER (WHERE status = 'CANCELED')::int AS canceled,
    COUNT(*) FILTER (WHERE status IN ('BUY_PENDING', 'PULLBACK_SEEN', 'POSITION_OPEN', 'SELL_PENDING'))::int AS open,
    COALESCE(SUM(pnl) FILTER (WHERE status = 'CLOSED'), 0)::double precision AS total_pnl,
    COALESCE(SUM(pnl) FILTER (WHERE status IN ('POSITION_OPEN', 'SELL_PENDING')), 0)::double precision AS open_pnl,
    COALESCE(AVG(pnl) FILTER (WHERE status = 'CLOSED'), 0)::double precision AS average_pnl,
    COUNT(*) FILTER (WHERE status = 'CLOSED' AND pnl > 0)::int AS wins,
    COUNT(*) FILTER (WHERE status = 'CLOSED' AND pnl < 0)::int AS losses
FROM paper_trade
WHERE strategy_version = $1;
