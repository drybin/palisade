package model

import (
    "fmt"
    
    "github.com/drybin/palisade/internal/domain/enum/order"
)

type OrderParams struct {
    Symbol           string
    Side             order.Side
    OrderType        order.Type
    Quantity         float64
    QuoteOrderQty    float64
    Price            float64
    NewClientOrderId string
}

func (o OrderParams) GetSymbol() *string {
    return &o.Symbol
}

func (o OrderParams) GetSide() *string {
    side := o.Side.String()
    return &side
}

func (o OrderParams) GetOrderType() *string {
    orderType := o.OrderType.String()
    return &orderType
}

func (o OrderParams) GetPrice() string {
    return fmt.Sprintf("%f", o.Price)
}

func (o OrderParams) GetQuantity() string {
    return fmt.Sprintf("%f", o.Quantity)
}
