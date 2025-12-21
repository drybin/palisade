package mexc

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"

	"github.com/drybin/palisade/pkg/wrap"
)

// Kline представляет данные одной свечи (candlestick)
type Kline struct {
	OpenTime         int64   // 0: Open time
	Open             float64 // 1: Open price
	High             float64 // 2: High price
	Low              float64 // 3: Low price
	Close            float64 // 4: Close price
	Volume           float64 // 5: Volume
	CloseTime        int64   // 6: Close time
	QuoteAssetVolume float64 // 7: Quote asset volume
}

// Klines представляет массив свечей
type Klines []Kline

// FlatAnalysisResult содержит результат анализа флета
type FlatAnalysisResult struct {
	IsFlat           bool    // Находится ли монета во флете
	Support          float64 // Нижняя граница (поддержка)
	Resistance       float64 // Верхняя граница (сопротивление)
	Range            float64 // Диапазон между границами
	RangePercent     float64 // Диапазон в процентах
	Volatility       float64 // Волатильность в процентах
	AvgPrice         float64 // Средняя цена
	MaxDrawdown      float64 // Максимальная просадка в процентах
	MaxRise          float64 // Максимальный рост в процентах
}

// AnalyzeFlat анализирует свечи на предмет флета (бокового движения)
// maxVolatilityPercent - максимальный процент волатильности для определения флета
func (k Klines) AnalyzeFlat(maxVolatilityPercent float64) FlatAnalysisResult {
	if len(k) == 0 {
		return FlatAnalysisResult{}
	}

	// Находим минимальную и максимальную цены
	minPrice := k[0].Low
	maxPrice := k[0].High
	var totalPrice float64

	for _, kline := range k {
		if kline.Low < minPrice {
			minPrice = kline.Low
		}
		if kline.High > maxPrice {
			maxPrice = kline.High
		}
		totalPrice += kline.Close
	}

	avgPrice := totalPrice / float64(len(k))
	priceRange := maxPrice - minPrice
	rangePercent := (priceRange / avgPrice) * 100

	// Вычисляем волатильность (стандартное отклонение цен закрытия)
	var sumSquaredDiff float64
	for _, kline := range k {
		diff := kline.Close - avgPrice
		sumSquaredDiff += diff * diff
	}
	volatility := math.Sqrt(sumSquaredDiff/float64(len(k))) / avgPrice * 100

	// Вычисляем максимальную просадку и рост от средней цены
	maxDrawdown := ((avgPrice - minPrice) / avgPrice) * 100
	maxRise := ((maxPrice - avgPrice) / avgPrice) * 100

	// Определяем, находится ли монета во флете
	// Критерии: волатильность не превышает заданный порог
	isFlat := volatility <= maxVolatilityPercent &&
		maxDrawdown <= maxVolatilityPercent &&
		maxRise <= maxVolatilityPercent

	return FlatAnalysisResult{
		IsFlat:       isFlat,
		Support:      minPrice,
		Resistance:   maxPrice,
		Range:        priceRange,
		RangePercent: rangePercent,
		Volatility:   volatility,
		AvgPrice:     avgPrice,
		MaxDrawdown:  maxDrawdown,
		MaxRise:      maxRise,
	}
}

// ParseKlinesFromArray парсит массив массивов интерфейсов в типизированную структуру Klines
func ParseKlinesFromArray(rawData [][]interface{}) (Klines, error) {
	klines := make(Klines, 0, len(rawData))

	for _, klineArray := range rawData {
		if len(klineArray) < 8 {
			continue
		}

		kline := Kline{
			OpenTime:         toInt64(klineArray[0]),
			Open:             toFloat64(klineArray[1]),
			High:             toFloat64(klineArray[2]),
			Low:              toFloat64(klineArray[3]),
			Close:            toFloat64(klineArray[4]),
			Volume:           toFloat64(klineArray[5]),
			CloseTime:        toInt64(klineArray[6]),
			QuoteAssetVolume: toFloat64(klineArray[7]),
		}

		klines = append(klines, kline)
	}

	return klines, nil
}

// toFloat64 безопасно конвертирует interface{} в float64
func toFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}

// toInt64 безопасно конвертирует interface{} в int64
func toInt64(val interface{}) int64 {
	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0
		}
		return i
	default:
		return 0
	}
}

// ParseKlinesFromJSON парсит JSON массив массивов в типизированную структуру Klines
func ParseKlinesFromJSON(data []byte) (Klines, error) {
	var klinesArray [][]interface{}
	if err := json.Unmarshal(data, &klinesArray); err != nil {
		return nil, wrap.Errorf("failed to unmarshal klines JSON: %w", err)
	}

	return ParseKlinesFromArray(klinesArray)
}

// FilterByPercentile фильтрует свечи, убирая верхние и нижние значения выше указанного перцентиля
// percentile должен быть в диапазоне от 1 до 100 (например, 95 означает убрать верхние и нижние 5%)
func (k Klines) FilterByPercentile(percentile float64) Klines {
	if len(k) == 0 || percentile < 1 || percentile > 100 {
		return k
	}

	// Преобразуем из диапазона 1-100 в диапазон 0-1
	percentileNormalized := percentile / 100.0

	// Собираем все High и Low значения
	highs := make([]float64, len(k))
	lows := make([]float64, len(k))
	for i, kline := range k {
		highs[i] = kline.High
		lows[i] = kline.Low
	}

	// Сортируем для нахождения перцентилей
	sort.Float64s(highs)
	sort.Float64s(lows)

	// Вычисляем индексы для перцентилей
	highIndex := int(math.Ceil(float64(len(highs)) * percentileNormalized))
	lowIndex := int(math.Floor(float64(len(lows)) * (1 - percentileNormalized)))

	if highIndex >= len(highs) {
		highIndex = len(highs) - 1
	}
	if lowIndex < 0 {
		lowIndex = 0
	}

	// Получаем граничные значения
	highThreshold := highs[highIndex]
	lowThreshold := lows[lowIndex]

	// Фильтруем свечи: оставляем только те, у которых High <= highThreshold и Low >= lowThreshold
	filtered := make(Klines, 0)
	for _, kline := range k {
		if kline.High <= highThreshold && kline.Low >= lowThreshold {
			filtered = append(filtered, kline)
		}
	}

	return filtered
}
