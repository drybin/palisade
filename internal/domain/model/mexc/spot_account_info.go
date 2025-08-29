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
