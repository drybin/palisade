package model

import "time"

type State struct {
	ID             int
	Date           time.Time
	AccountBalance float64
	CoinFirst      string
	CoinSecond     string
	Price          float64
	Amount         float64
	State          string
	Orderid        string
	Uplevel        float64
	Downlevel      float64
}
