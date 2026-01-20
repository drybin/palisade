package model

import (
    "fmt"
    "strconv"
    
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
    if o.Price == float64(int64(o.Price)) {
        return fmt.Sprintf("%.0f", o.Price)
    }
    return strconv.FormatFloat(o.Price, 'f', -1, 64)
}

func (o OrderParams) GetQuantity() string {
    if o.Quantity == float64(int64(o.Quantity)) {
        return fmt.Sprintf("%.0f", o.Quantity)
    }
    return strconv.FormatFloat(o.Quantity, 'f', -1, 64)
}
