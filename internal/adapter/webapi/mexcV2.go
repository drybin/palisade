package webapi

import (
    "encoding/json"
    "fmt"
    
    "github.com/drybin/palisade/internal/app/cli/config"
    "github.com/drybin/palisade/internal/domain/model"
    "github.com/drybin/palisade/internal/domain/model/mexcV2"
    "github.com/drybin/palisade/pkg/wrap"
    "github.com/go-resty/resty/v2"
)

//const mexc_spot_account_info_url = "/api/v3/account"
const mexc_klines_url = "/api/market/v2/kline?interval=%s&symbolId=%s"
const mexc_price_url = "/api/market/v2/price?coinId=%s&symbolId=%s"

type MexcV2Webapi struct {
    client *resty.Client
    config config.MexcConfig
}

func NewMexcV2Webapi(
    client *resty.Client,
    config config.MexcConfig,
) *MexcV2Webapi {
    return &MexcV2Webapi{
        client: client,
        config: config,
    }
}

func (m *MexcV2Webapi) GetKLines(pair model.PairWithLevels, interval string, symbolId string) (*mexcV2.KlinesResult, error) {
    res, err := m.client.R().Get(
        m.config.BaseUrlV2 + fmt.Sprintf(mexc_klines_url, interval, symbolId),
    )
    if err != nil {
        return nil, wrap.Errorf("failed to get klines from api v2: %w", err)
    }
    
    result := mexcV2.KlinesResult{}
    err = json.Unmarshal(res.Body(), &result)
    if err != nil {
        return nil, wrap.Errorf("failed to unmarshal klines: %w", err)
    }
    
    return &result, nil
}

func (m *MexcV2Webapi) GetPrice(coinId string, symbolId string) (*mexcV2.Price, error) {
    res, err := m.client.R().Get(
        m.config.BaseUrlV2 + fmt.Sprintf(mexc_price_url, coinId, symbolId),
    )
    if err != nil {
        return nil, wrap.Errorf("failed to get price from api v2: %w", err)
    }
    
    result := mexcV2.Price{}
    err = json.Unmarshal(res.Body(), &result)
    if err != nil {
        return nil, wrap.Errorf("failed to unmarshal price: %w", err)
    }
    
    return &result, nil
}
