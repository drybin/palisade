package model

type PairWithLevels struct {
    Pair Pair
    Max  float64
    Min  float64
}

func NewPairWithLevels(pair Pair, max float64, min float64) PairWithLevels {
    return PairWithLevels{
        Pair: pair,
        Max:  max,
        Min:  min,
    }
}
