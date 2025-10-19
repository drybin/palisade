package mexc

type OpenOrders []OpenOrder

type OpenOrder struct {
    ClientOrderID       string `json:"clientOrderId"`
    CummulativeQuoteQty string `json:"cummulativeQuoteQty"`
    ExecutedQty         string `json:"executedQty"`
    IcebergQty          string `json:"icebergQty"`
    IsWorking           bool   `json:"isWorking"`
    OrderID             string `json:"orderId"`
    OrderListID         int    `json:"orderListId"`
    OrigQty             string `json:"origQty"`
    OrigQuoteOrderQty   string `json:"origQuoteOrderQty"`
    Price               string `json:"price"`
    Side                string `json:"side"`
    Status              string `json:"status"`
    StopPrice           string `json:"stopPrice"`
    StpMode             string `json:"stpMode"`
    Symbol              string `json:"symbol"`
    Time                int64  `json:"time"`
    TimeInForce         string `json:"timeInForce"`
    Type                string `json:"type"`
    UpdateTime          string `json:"updateTime"`
}
