package model

import "github.com/drybin/palisade/internal/domain/enum/order"

type OrderParams struct {
	Symbol           string
	Side             order.Side
	OrderType        order.Type
	Quantity         float64
	QuoteOrderQty    float64
	Price            float64
	NewClientOrderId string
}
