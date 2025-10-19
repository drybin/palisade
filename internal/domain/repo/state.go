package repo

import (
    "context"
    
    "github.com/drybin/palisade/internal/domain/model"
    "github.com/drybin/palisade/internal/domain/model/mexc"
)

type IStateRepository interface {
    GetCoinState(context.Context, model.Coin, model.Coin) (*model.State, error)
    GetCountLogsByCoin(context.Context, model.Coin, model.Coin) (*int, error)
    SaveCoin(context.Context, *mexc.SymbolDetail) error
    GetCoinInfo(context.Context, string) (*mexc.SymbolDetail, error)
}
