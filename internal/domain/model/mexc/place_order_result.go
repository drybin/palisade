package mexc

type PlaceOrderResult struct {
	Symbol       string `json:"symbol"`
	OrderID      string `json:"orderId"`
	OrderListID  int    `json:"orderListId"`
	Price        string `json:"price"`
	OrigQty      string `json:"origQty"`
	Type         string `json:"type"`
	StpMode      string `json:"stpMode"`
	Side         string `json:"side"`
	TransactTime int64  `json:"transactTime"`
}
