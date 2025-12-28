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

-- name: GetCoinsToProcess :many
SELECT * FROM coins
WHERE 
    isSpotTradingAllowed = true
    AND isPalisade = true
    --AND volatility > 0.1
    --AND volatility < 0.4
    AND quoteasset = 'USDT'
    AND symbol = 'TPTUUSDT'
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