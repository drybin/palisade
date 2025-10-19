package mexcV2

type KlinesResult struct {
    ResponseTime int64  `json:"responseTime"`
    Msg          string `json:"msg"`
    Code         int    `json:"code"`
    Data         struct {
        O           []string `json:"o"`
        T           []int64  `json:"t"`
        Price       string   `json:"price"`
        PriceChange string   `json:"priceChange"`
    } `json:"data"`
}
