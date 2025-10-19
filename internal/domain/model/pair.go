package model

import "fmt"

type Pair struct {
    CoinFirst  Coin
    CoinSecond Coin
}

func NewPair(first Coin, second Coin) Pair {
    return Pair{
        CoinFirst:  first,
        CoinSecond: second,
    }
}

func (p Pair) String() string {
    return fmt.Sprintf("%s%s", p.CoinFirst, p.CoinSecond)
}

func (p Pair) StringPtr() *string {
    res := fmt.Sprintf("%s%s", p.CoinFirst, p.CoinSecond)
    return &res
}
