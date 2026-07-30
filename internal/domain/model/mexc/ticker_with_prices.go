package mexc

type TickersWithPrice []TickerWithPrice

type TickerWithPrice struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type Ticker24h struct {
	Symbol             string `json:"symbol"`
	LastPrice          string `json:"lastPrice"`
	PriceChangePercent string `json:"priceChangePercent"`
	QuoteVolume        string `json:"quoteVolume"`
}

type Tickers24h []Ticker24h
