package mexcV2

type Price struct {
    ResponseTime int64  `json:"responseTime"`
    Msg          string `json:"msg"`
    Code         int    `json:"code"`
    Data         struct {
        Performance []struct {
            Change      string `json:"change"`
            PriceChange string `json:"priceChange"`
            Amount      string `json:"amount"`
        } `json:"performance"`
        PriceInformation struct {
            Low24H         string `json:"low24h"`
            High24H        string `json:"high24h"`
            AllTimeHigh    string `json:"allTimeHigh"`
            PriceChange1H  string `json:"priceChange1h"`
            PriceChange24H string `json:"priceChange24h"`
            PriceChange7D  string `json:"priceChange7d"`
        } `json:"priceInformation"`
        MarketInformation struct {
            Volume24H         string `json:"volume24h"`
            MarketCap         string `json:"marketCap"`
            CirculationSupply string `json:"circulationSupply"`
            CurrentPrice      string `json:"currentPrice"`
        } `json:"marketInformation"`
    } `json:"data"`
}
