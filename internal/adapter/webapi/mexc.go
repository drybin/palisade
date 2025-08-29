package webapi

import (
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "net/url"
    "time"
    
    "github.com/drybin/palisade/internal/app/cli/config"
    "github.com/drybin/palisade/internal/domain/model"
    "github.com/drybin/palisade/internal/domain/model/mexc"
    "github.com/drybin/palisade/pkg/wrap"
    "github.com/go-resty/resty/v2"
)

const mexc_spot_account_info_url = "/api/v3/account"
const mexc_new_order_url = "/api/v3/order"

type MexcWebapi struct {
    client *resty.Client
    config config.MexcConfig
}

func NewMexcWebapi(
    client *resty.Client,
    config config.MexcConfig,
) *MexcWebapi {
    return &MexcWebapi{
        client: client,
        config: config,
    }
}

func (m *MexcWebapi) GetBalance(ctx context.Context) (*mexc.SpotAccountInfo, error) {
    params := url.Values{}
    params = generateSignatureAndAddToParam(m.config.Secret, params)
    
    req := m.client.R()
    for key, values := range params {
        for _, value := range values {
            req.SetQueryParam(key, value)
        }
    }
    
    res, err := req.Get(mexc_spot_account_info_url)
    if err != nil {
        return nil, wrap.Errorf("failed to get spot acoount info: %w", err)
    }
    
    result := mexc.SpotAccountInfo{}
    err = json.Unmarshal(res.Body(), &result)
    if err != nil {
        return nil, wrap.Errorf("failed to unmarshal swap info: %w", err)
    }
    
    return &result, nil
}

func (m *MexcWebapi) NewOrder(
    ctx context.Context,
    orderParams model.OrderParams,
) (*mexc.PlaceOrderResult, error) {
    params := url.Values{}
    params.Set("symbol", orderParams.Symbol)
    params.Set("side", orderParams.Side.String())
    params.Set("type", orderParams.OrderType.String())
    params.Set("quantity", fmt.Sprintf("%f", orderParams.Quantity))
    //params.Set("quoteOrderQty", fmt.Sprintf("%f", orderParams.QuoteOrderQty))
    params.Set("price", fmt.Sprintf("%f", orderParams.Price))
    //params.Set("newClientOrderId", fmt.Sprintf("%f", orderParams.NewClientOrderId))
    params.Set("recvWindow", "5000")
    params = generateSignatureAndAddToParam(m.config.Secret, params)
    
    req := m.client.R()
    //for key, values := range params {
    //	for _, value := range values {
    //		req.SetQueryParam(key, value)
    //	}
    //}
    
    req.SetBody(params.Encode())
    res, err := req.Post(mexc_new_order_url)
    if err != nil {
        return nil, wrap.Errorf("failed to place order: %w", err)
    }
    
    result := mexc.PlaceOrderResult{}
    err = json.Unmarshal(res.Body(), &result)
    if err != nil {
        return nil, wrap.Errorf("failed to unmarshal swap info: %w", err)
    }
    
    return &result, nil
}

func generateSignatureAndAddToParam(apiSecret string, params url.Values) url.Values {
    timestamp := time.Now().UnixMilli()
    params.Set("timestamp", fmt.Sprintf("%d", timestamp))
    
    signature := sign(params.Encode(), apiSecret)
    params.Set("signature", signature)
    
    return params
}

// функция генерации подписи HMAC-SHA256
func sign(message, secret string) string {
    h := hmac.New(sha256.New, []byte(secret))
    h.Write([]byte(message))
    return hex.EncodeToString(h.Sum(nil))
}
