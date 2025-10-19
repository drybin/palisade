package helpers

import (
    "fmt"
    "math"
    "strconv"
    
    "github.com/drybin/palisade/internal/domain/model"
)

func AnalyzeFloatArray(values []float64) model.ArrayStats {
    stats := model.ArrayStats{
        Min:    math.Inf(1),  // +∞
        Max:    math.Inf(-1), // -∞
        Counts: make(map[float64]int),
    }
    
    for _, v := range values {
        if v < stats.Min {
            stats.Min = v
        }
        if v > stats.Max {
            stats.Max = v
        }
        stats.Counts[v]++
    }
    
    // Если массив пустой — вернём NaN в Min/Max
    if len(values) == 0 {
        stats.Min = math.NaN()
        stats.Max = math.NaN()
    }
    
    return stats
}

func StringsToFloats(strs []string) ([]float64, error) {
    floats := make([]float64, 0, len(strs))
    for _, s := range strs {
        if s == "" {
            continue // можно пропускать пустые строки
        }
        f, err := strconv.ParseFloat(s, 64)
        if err != nil {
            return nil, fmt.Errorf("can't parse float '%s': %w", s, err)
        }
        floats = append(floats, f)
    }
    return floats, nil
}

// Фильтрация относительно лидера
func FilterByRelativeThreshold(stat *model.ArrayStats, threshold float64) model.ArrayStats {
    s := *stat
    if len(s.Counts) == 0 {
        return model.ArrayStats{}
    }
    
    // ищем максимальное количество
    maxCount := 0
    for _, c := range s.Counts {
        if c > maxCount {
            maxCount = c
        }
    }
    
    // считаем минимально допустимое количество
    limit := int(float64(maxCount) * threshold)
    
    // удаляем все ниже лимита
    for v, c := range s.Counts {
        if c < limit {
            delete(s.Counts, v)
        }
    }
    
    return s
}

func GetPrices(stat *model.ArrayStats) []float64 {
    res := []float64{}
    
    // удаляем все ниже лимита
    for v, c := range stat.Counts {
        for i := 0; i < c; i++ {
            res = append(res, v)
        }
        
    }
    
    return res
}

func AnalyzeFloatArrayWithThreshold(values []float64, threshold float64) model.ArrayStats {
    resFirst := AnalyzeFloatArray(values)
    resSecond := FilterByRelativeThreshold(&resFirst, threshold)
    res := AnalyzeFloatArray(GetPrices(&resSecond))
    return res
}
