package mexc

import (
	"encoding/json"
	"strconv"

	"github.com/drybin/palisade/pkg/wrap"
)

type OpenOrders []OpenOrder

type OpenOrder struct {
	Symbol              string  `json:"symbol"`
	OrderID             string  `json:"orderId"`
	OrderListID         int64   `json:"orderListId"`
	ClientOrderID       string  `json:"clientOrderId"`
	Price               string  `json:"price"`
	OrigQty             string  `json:"origQty"`
	ExecutedQty         string  `json:"executedQty"`
	CummulativeQuoteQty string  `json:"cummulativeQuoteQty"`
	Status              string  `json:"status"`
	TimeInForce         string  `json:"timeInForce"`
	Type                string  `json:"type"`
	Side                string  `json:"side"`
	StopPrice           string  `json:"stopPrice"`
	IcebergQty          string  `json:"icebergQty"`
	Time                int64   `json:"time"`
	UpdateTime          int64   `json:"updateTime"`
	IsWorking           bool    `json:"isWorking"`
	StpMode             string  `json:"stpMode"`
	CancelReason        *string `json:"cancelReason,omitempty"`
	OrigQuoteOrderQty   string  `json:"origQuoteOrderQty"`
}

// openOrderRaw представляет сырой JSON ответ от API, где числа могут быть строками
type openOrderRaw struct {
	Symbol              interface{} `json:"symbol"`
	OrderID             interface{} `json:"orderId"`
	OrderListID         interface{} `json:"orderListId"`
	ClientOrderID       interface{} `json:"clientOrderId"`
	Price               interface{} `json:"price"`
	OrigQty             interface{} `json:"origQty"`
	ExecutedQty         interface{} `json:"executedQty"`
	CummulativeQuoteQty interface{} `json:"cummulativeQuoteQty"`
	Status              interface{} `json:"status"`
	TimeInForce         interface{} `json:"timeInForce"`
	Type                interface{} `json:"type"`
	Side                interface{} `json:"side"`
	StopPrice           interface{} `json:"stopPrice"`
	IcebergQty          interface{} `json:"icebergQty"`
	Time                interface{} `json:"time"`
	UpdateTime          interface{} `json:"updateTime"`
	IsWorking           interface{} `json:"isWorking"`
	StpMode             interface{} `json:"stpMode"`
	CancelReason        interface{} `json:"cancelReason,omitempty"`
	OrigQuoteOrderQty   interface{} `json:"origQuoteOrderQty"`
}

// ParseOpenOrdersFromJSON парсит JSON массив открытых ордеров в типизированную структуру OpenOrders
func ParseOpenOrdersFromJSON(data []byte) (OpenOrders, error) {
	var rawOrders []openOrderRaw
	if err := json.Unmarshal(data, &rawOrders); err != nil {
		return nil, wrap.Errorf("failed to unmarshal open orders JSON: %w", err)
	}

	orders := make(OpenOrders, 0, len(rawOrders))
	for _, raw := range rawOrders {
		order := mapRawOrderToOpenOrder(raw)
		orders = append(orders, order)
	}

	return orders, nil
}

// mapRawOrderToOpenOrder конвертирует сырой ордер в типизированный OpenOrder
func mapRawOrderToOpenOrder(raw openOrderRaw) OpenOrder {
	order := OpenOrder{}

	order.Symbol = toString(raw.Symbol)
	order.OrderID = toString(raw.OrderID)
	order.OrderListID = toInt64(raw.OrderListID)
	order.ClientOrderID = toString(raw.ClientOrderID)
	order.Price = toString(raw.Price)
	order.OrigQty = toString(raw.OrigQty)
	order.ExecutedQty = toString(raw.ExecutedQty)
	order.CummulativeQuoteQty = toString(raw.CummulativeQuoteQty)
	order.Status = toString(raw.Status)
	order.TimeInForce = toString(raw.TimeInForce)
	order.Type = toString(raw.Type)
	order.Side = toString(raw.Side)
	order.StopPrice = toString(raw.StopPrice)
	order.IcebergQty = toString(raw.IcebergQty)
	order.Time = toInt64(raw.Time)
	order.UpdateTime = toInt64(raw.UpdateTime)
	order.IsWorking = toBool(raw.IsWorking)
	order.StpMode = toString(raw.StpMode)
	order.OrigQuoteOrderQty = toString(raw.OrigQuoteOrderQty)

	if raw.CancelReason != nil {
		reason := toString(raw.CancelReason)
		order.CancelReason = &reason
	}

	return order
}

// toString безопасно конвертирует interface{} в string
func toString(val interface{}) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	case bool:
		return strconv.FormatBool(v)
	default:
		return ""
	}
}

// toBool безопасно конвертирует interface{} в bool
func toBool(val interface{}) bool {
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false
		}
		return b
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}
