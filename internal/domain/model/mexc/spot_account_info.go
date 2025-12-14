package mexc

type SpotAccountInfo struct {
	MakerCommission  interface{} `json:"makerCommission"`
	TakerCommission  interface{} `json:"takerCommission"`
	BuyerCommission  interface{} `json:"buyerCommission"`
	SellerCommission interface{} `json:"sellerCommission"`
	CanTrade         bool        `json:"canTrade"`
	CanWithdraw      bool        `json:"canWithdraw"`
	CanDeposit       bool        `json:"canDeposit"`
	UpdateTime       interface{} `json:"updateTime"`
	AccountType      string      `json:"accountType"`
	Balances         []struct {
		Asset     string `json:"asset"`
		Free      string `json:"free"`
		Locked    string `json:"locked"`
		Available string `json:"available"`
	} `json:"balances"`
	Permissions []string `json:"permissions"`
}

type Balance struct {
	Asset     string
	Free      float64
	Locked    float64
	Available float64
}

type AccountInfo struct {
	MakerCommission  int64
	TakerCommission  int64
	BuyerCommission  int64
	SellerCommission int64
	CanTrade         bool
	CanWithdraw      bool
	CanDeposit       bool
	UpdateTime       int64
	AccountType      string
	Balances         []Balance
	Permissions      []string
}
