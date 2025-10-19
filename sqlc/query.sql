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