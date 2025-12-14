package mexc

import (
	"encoding/json"
	"strconv"

	"github.com/drybin/palisade/pkg/wrap"
)

// CancelOrderResponse представляет ответ от API при отмене ордера
type CancelOrderResponse struct {
	Success bool                `json:"success"`
	Code    int                 `json:"code"`
	Data    []CancelOrderResult `json:"data"`
}

// CancelOrderResult представляет результат отмены одного ордера
type CancelOrderResult struct {
	OrderID   string `json:"orderId"`
	ErrorCode int    `json:"errorCode"`
	ErrorMsg  string `json:"errorMsg"`
}

// cancelOrderRaw представляет сырой JSON ответ, где orderId может быть числом или строкой
type cancelOrderRaw struct {
	Success bool                   `json:"success"`
	Code    interface{}            `json:"code"`
	Data    []cancelOrderResultRaw `json:"data"`
}

type cancelOrderResultRaw struct {
	OrderID   interface{} `json:"orderId"`
	ErrorCode interface{} `json:"errorCode"`
	ErrorMsg  interface{} `json:"errorMsg"`
}

// ParseCancelOrderResponseFromJSON парсит JSON ответ отмены ордера в типизированную структуру
func ParseCancelOrderResponseFromJSON(data []byte) (*CancelOrderResponse, error) {
	var raw cancelOrderRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, wrap.Errorf("failed to unmarshal cancel order JSON: %w", err)
	}

	result := &CancelOrderResponse{
		Success: raw.Success,
		Code:    toInt(raw.Code),
		Data:    make([]CancelOrderResult, 0, len(raw.Data)),
	}

	for _, item := range raw.Data {
		result.Data = append(result.Data, CancelOrderResult{
			OrderID:   cancelOrderToString(item.OrderID),
			ErrorCode: toInt(item.ErrorCode),
			ErrorMsg:  cancelOrderToString(item.ErrorMsg),
		})
	}

	return result, nil
}

// cancelOrderToString безопасно конвертирует interface{} в string
func cancelOrderToString(val interface{}) string {
	if val == nil {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v
	case float64:
		// Для больших чисел (orderId) используем форматирование без экспоненты
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
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

// toInt безопасно конвертирует interface{} в int
func toInt(val interface{}) int {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}
