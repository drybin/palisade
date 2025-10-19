package service

import "github.com/drybin/palisade/internal/domain/model"

type TradingPair struct {
}

func NewTradingPair() *TradingPair {
    return &TradingPair{}
}

func (s *TradingPair) GetPalisade() model.PairWithLevels {
    return model.PairWithLevels{
        Pair: model.Pair{
            CoinFirst: model.Coin{
                Name:     "DOLZ",
                CoinId:   "858638dfeb884b02a6e3bc40e92234fd",
                SymbolId: "cde4ae10239c49ac952465b3853cc740",
            },
            CoinSecond: model.Coin{
                Name: "USDT",
            },
        },
        Max: 0.00567,
        Min: 0.00562,
    }
}
