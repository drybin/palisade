package mexc

import "time"

type SymbolInfo struct {
	Timezone        string         `json:"timezone"`
	ServerTime      int64          `json:"serverTime"`
	RateLimits      []interface{}  `json:"rateLimits"`
	ExchangeFilters []interface{}  `json:"exchangeFilters"`
	Symbols         []SymbolDetail `json:"symbols"`
}

type SymbolDetail struct {
	Symbol                   string   `json:"symbol"`
	Status                   string   `json:"status"`
	BaseAsset                string   `json:"baseAsset"`
	BaseAssetPrecision       float64  `json:"baseAssetPrecision"`
	QuoteAsset               string   `json:"quoteAsset"`
	QuotePrecision           int      `json:"quotePrecision"`
	QuoteAssetPrecision      int      `json:"quoteAssetPrecision"`
	BaseCommissionPrecision  int      `json:"baseCommissionPrecision"`
	QuoteCommissionPrecision int      `json:"quoteCommissionPrecision"`
	OrderTypes               []string `json:"orderTypes"`
	IsSpotTradingAllowed     bool     `json:"isSpotTradingAllowed"`
	IsMarginTradingAllowed   bool     `json:"isMarginTradingAllowed"`
	QuoteAmountPrecision     string   `json:"quoteAmountPrecision"`
	BaseSizePrecision        string   `json:"baseSizePrecision"`
	Permissions              []string `json:"permissions"`
	Filters                  []struct {
		FilterType        string `json:"filterType"`
		BidMultiplierUp   string `json:"bidMultiplierUp"`
		AskMultiplierDown string `json:"askMultiplierDown"`
	} `json:"filters"`
	MaxQuoteAmount             string    `json:"maxQuoteAmount"`
	MakerCommission            string    `json:"makerCommission"`
	TakerCommission            string    `json:"takerCommission"`
	QuoteAmountPrecisionMarket string    `json:"quoteAmountPrecisionMarket"`
	MaxQuoteAmountMarket       string    `json:"maxQuoteAmountMarket"`
	FullName                   string    `json:"fullName"`
	TradeSideType              int       `json:"tradeSideType"`
	ContractAddress            string    `json:"contractAddress"`
	St                         bool      `json:"st"`
	LastCheck                  time.Time `json:"lastCheck"` // Дата последней проверки (date из таблицы coins)
	IsPalisade                 bool      `json:"isPalisade"`
	Support                    float64   `json:"support"`      // нижняя граница
	Resistance                 float64   `json:"resistance"`   // верхняя граница
	RangeValue                 float64   `json:"rangeValue"`   // диапазон между границами
	RangePercent               float64   `json:"rangePercent"` // диапазон в процентах
	AvgPrice                   float64   `json:"avgPrice"`     // средняя цена
	Volatility                 float64   `json:"volatility"`   // волатильность в процентах
	MaxDrawdown                float64   `json:"maxDrawdown"`  // максимальная просадка в процентах
	MaxRise                    float64   `json:"maxRise"`      // максимальный рост в процентах
}
