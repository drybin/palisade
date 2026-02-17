package model

import (
    "fmt"
    "strconv"
    "strings"

    "github.com/drybin/palisade/internal/domain/enum/order"
)

// formatFloatAPI форматирует float с фиксированной точностью (8 знаков) и убирает хвостовые нули,
// чтобы в API не уходил float-шум (например 0.036000000000000004) и не нарушался scale.
func formatFloatAPI(v float64) string {
    if v == float64(int64(v)) {
        return fmt.Sprintf("%.0f", v)
    }
    s := strconv.FormatFloat(v, 'f', 8, 64)
    s = strings.TrimRight(s, "0")
    s = strings.TrimRight(s, ".")
    return s
}

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
    return formatFloatAPI(o.Price)
}

func (o OrderParams) GetQuantity() string {
    return formatFloatAPI(o.Quantity)
}
