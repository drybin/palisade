package mexc

import (
	"encoding/json"

	"github.com/drybin/palisade/pkg/wrap"
)

// QueryOrderResult представляет результат запроса информации об ордере
// Использует ту же структуру, что и OpenOrder, так как формат ответа идентичен
type QueryOrderResult = OpenOrder

// queryOrderRaw представляет сырой JSON ответ от API, где числа могут быть строками
type queryOrderRaw = openOrderRaw

// ParseQueryOrderFromJSON парсит JSON ответ запроса ордера в типизированную структуру QueryOrderResult
func ParseQueryOrderFromJSON(data []byte) (*QueryOrderResult, error) {
	var raw queryOrderRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, wrap.Errorf("failed to unmarshal query order JSON: %w", err)
	}

	result := mapRawOrderToOpenOrder(raw)
	return &result, nil
}

