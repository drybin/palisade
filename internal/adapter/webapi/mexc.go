package webapi

import (
    "context"
    "fmt"
    
    "github.com/drybin/palisade/mexc-sdk/mexcsdk"
    
    "github.com/drybin/palisade/internal/app/cli/config"
)

type MexcWebapi struct {
    config config.MexcConfig
}

func NewMexcWebapi(
    config config.MexcConfig,
) *MexcWebapi {
    return &MexcWebapi{
        config: config,
    }
}

func (m *MexcWebapi) GetBalance(ctx context.Context) error {
    
    spot := mexcsdk.NewSpot(m.config.ApiKey, m.config.Secret)
    res := spot.AccountInfo()
    fmt.Printf("AccountInfo: %+v\n", res)
    return nil
}
