package mexc

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
	MaxQuoteAmount             string `json:"maxQuoteAmount"`
	MakerCommission            string `json:"makerCommission"`
	TakerCommission            string `json:"takerCommission"`
	QuoteAmountPrecisionMarket string `json:"quoteAmountPrecisionMarket"`
	MaxQuoteAmountMarket       string `json:"maxQuoteAmountMarket"`
	FullName                   string `json:"fullName"`
	TradeSideType              int    `json:"tradeSideType"`
	ContractAddress            string `json:"contractAddress"`
	St                         bool   `json:"st"`
}
