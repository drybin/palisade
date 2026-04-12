package mexc

// BookTickers — ответ GET /api/v3/ticker/bookTicker (массив или один элемент).
type BookTickers []BookTicker

// BookTicker — лучший bid/ask по символу.
type BookTicker struct {
	Symbol   string `json:"symbol"`
	BidPrice string `json:"bidPrice"`
	AskPrice string `json:"askPrice"`
}
