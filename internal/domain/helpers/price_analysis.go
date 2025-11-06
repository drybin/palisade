package helpers

import (
	"encoding/json"
	"math"
	"strconv"

	"mexc-sdk/mexcsdk"
)

// KlineData представляет данные одной свечи
type KlineData struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

// PriceLevels содержит определенные уровни поддержки и сопротивления
type PriceLevels struct {
	Support      float64 // Нижний уровень (поддержка)
	Resistance   float64 // Верхний уровень (сопротивление)
	Range        float64 // Диапазон между уровнями
	RangePercent float64 // Диапазон в процентах
}

// FlatAnalysisResult содержит результат анализа флета
type FlatAnalysisResult struct {
	IsFlat      bool
	Levels      PriceLevels
	Volatility  float64 // Волатильность в процентах
	AvgPrice    float64 // Средняя цена
	MaxDrawdown float64 // Максимальная просадка в процентах
	MaxRise     float64 // Максимальный рост в процентах
}

// ParseKlines парсит данные свечей из ответа MEXC SDK
func ParseKlines(rawData interface{}) ([]KlineData, error) {
	bytes, err := json.Marshal(rawData)
	if err != nil {
		return nil, err
	}

	var klinesArray [][]interface{}
	if err := json.Unmarshal(bytes, &klinesArray); err != nil {
		return nil, err
	}

	klines := make([]KlineData, 0, len(klinesArray))
	for _, kline := range klinesArray {
		if len(kline) < 7 {
			continue
		}

		klineData := KlineData{
			OpenTime:  int64(toFloat64(kline[0])),
			Open:      toFloat64(kline[1]),
			High:      toFloat64(kline[2]),
			Low:       toFloat64(kline[3]),
			Close:     toFloat64(kline[4]),
			Volume:    toFloat64(kline[5]),
			CloseTime: int64(toFloat64(kline[6])),
		}
		klines = append(klines, klineData)
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
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

// AnalyzeFlat анализирует свечи на предмет флета (бокового движения)
func AnalyzeFlat(klines []KlineData, maxVolatilityPercent float64) FlatAnalysisResult {
	if len(klines) == 0 {
		return FlatAnalysisResult{}
	}

	// Находим минимальную и максимальную цены
	minPrice := klines[0].Low
	maxPrice := klines[0].High
	var totalPrice float64

	for _, kline := range klines {
		if kline.Low < minPrice {
			minPrice = kline.Low
		}
		if kline.High > maxPrice {
			maxPrice = kline.High
		}
		totalPrice += kline.Close
	}

	avgPrice := totalPrice / float64(len(klines))
	priceRange := maxPrice - minPrice
	rangePercent := (priceRange / avgPrice) * 100

	// Вычисляем волатильность (стандартное отклонение цен закрытия)
	var sumSquaredDiff float64
	for _, kline := range klines {
		diff := kline.Close - avgPrice
		sumSquaredDiff += diff * diff
	}
	volatility := math.Sqrt(sumSquaredDiff/float64(len(klines))) / avgPrice * 100

	// Вычисляем максимальную просадку и рост от средней цены
	maxDrawdown := ((avgPrice - minPrice) / avgPrice) * 100
	maxRise := ((maxPrice - avgPrice) / avgPrice) * 100

	// Определяем, находится ли монета во флете
	// Критерии: волатильность не превышает заданный порог
	isFlat := volatility <= maxVolatilityPercent &&
		maxDrawdown <= maxVolatilityPercent &&
		maxRise <= maxVolatilityPercent

	return FlatAnalysisResult{
		IsFlat: isFlat,
		Levels: PriceLevels{
			Support:      minPrice,
			Resistance:   maxPrice,
			Range:        priceRange,
			RangePercent: rangePercent,
		},
		Volatility:  volatility,
		AvgPrice:    avgPrice,
		MaxDrawdown: maxDrawdown,
		MaxRise:     maxRise,
	}
}

// GetKlinesFromMexc получает данные свечей через MEXC SDK
func GetKlinesFromMexc(spot mexcsdk.Spot, symbol string, interval string, limit int) ([]KlineData, error) {
	options := map[string]interface{}{
		"limit": limit,
	}

	rawData := spot.Klines(&symbol, &interval, options)
	return ParseKlines(rawData)
}

// CheckPriceStability проверяет стабильность цены на основе последних сделок
func CheckPriceStability(klines []KlineData, periodsToCheck int, maxChangePercent float64) bool {
	if len(klines) < periodsToCheck {
		return false
	}

	// Берем последние N свечей
	recentKlines := klines[len(klines)-periodsToCheck:]

	// Проверяем изменение цены между свечами
	for i := 1; i < len(recentKlines); i++ {
		priceChange := math.Abs(recentKlines[i].Close - recentKlines[i-1].Close)
		percentChange := (priceChange / recentKlines[i-1].Close) * 100

		if percentChange > maxChangePercent {
			return false
		}
	}

	return true
}

// CalculateSupportResistanceLevels вычисляет уровни поддержки и сопротивления
// используя метод кластеризации локальных минимумов и максимумов
func CalculateSupportResistanceLevels(klines []KlineData, tolerance float64) PriceLevels {
	if len(klines) == 0 {
		return PriceLevels{}
	}

	// Находим локальные минимумы и максимумы
	var lows []float64
	var highs []float64

	for i := 1; i < len(klines)-1; i++ {
		// Локальный минимум
		if klines[i].Low < klines[i-1].Low && klines[i].Low < klines[i+1].Low {
			lows = append(lows, klines[i].Low)
		}
		// Локальный максимум
		if klines[i].High > klines[i-1].High && klines[i].High > klines[i+1].High {
			highs = append(highs, klines[i].High)
		}
	}

	// Если не нашли локальные экстремумы, используем общие мин/макс
	if len(lows) == 0 || len(highs) == 0 {
		minPrice := klines[0].Low
		maxPrice := klines[0].High

		for _, kline := range klines {
			if kline.Low < minPrice {
				minPrice = kline.Low
			}
			if kline.High > maxPrice {
				maxPrice = kline.High
			}
		}

		avgPrice := (minPrice + maxPrice) / 2
		return PriceLevels{
			Support:      minPrice,
			Resistance:   maxPrice,
			Range:        maxPrice - minPrice,
			RangePercent: ((maxPrice - minPrice) / avgPrice) * 100,
		}
	}

	// Находим среднее значение локальных минимумов и максимумов
	var sumLows, sumHighs float64
	for _, low := range lows {
		sumLows += low
	}
	for _, high := range highs {
		sumHighs += high
	}

	support := sumLows / float64(len(lows))
	resistance := sumHighs / float64(len(highs))
	priceRange := resistance - support
	avgPrice := (support + resistance) / 2

	return PriceLevels{
		Support:      support,
		Resistance:   resistance,
		Range:        priceRange,
		RangePercent: (priceRange / avgPrice) * 100,
	}
}

