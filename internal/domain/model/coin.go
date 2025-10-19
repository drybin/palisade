package model

type Coin struct {
    Name     string
    CoinId   string
    SymbolId string
}

func NewCoin(name string, coinId string, symbolId string) Coin {
    return Coin{Name: name, CoinId: coinId, SymbolId: symbolId}
}

func (c Coin) String() string {
    return c.Name
}

func (c Coin) StringPtr() *string {
    return &c.Name
}
