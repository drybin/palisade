package model

type ArrayStats struct {
    Min    float64
    Max    float64
    Counts map[float64]int
}
