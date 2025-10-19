package mexc

type TickersWithPrice []TickerWithPrice

type TickerWithPrice struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}
