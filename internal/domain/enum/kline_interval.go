package enum

const (
	MINUTES_1  KlineInterval = iota + 1
	MINUTES_5  KlineInterval = iota + 1
	MINUTES_15 KlineInterval = iota + 1
	MINUTES_30 KlineInterval = iota + 1
	MINUTES_60 KlineInterval = iota + 1
	HOURS_4    KlineInterval = iota + 1
	DAY_1      KlineInterval = iota + 1
	WEEK_1     KlineInterval = iota + 1
	MONTH_1    KlineInterval = iota + 1
)

type KlineInterval int

func (p KlineInterval) IsValid() bool {
	switch p {
	case MINUTES_1:
		return true
	case MINUTES_5:
		return true
	case MINUTES_15:
		return true
	case MINUTES_30:
		return true
	case MINUTES_60:
		return true
	case HOURS_4:
		return true
	case DAY_1:
		return true
	case WEEK_1:
		return true
	case MONTH_1:
		return true
	default:
		return false
	}
}

func (p KlineInterval) String() string {
	switch p {
	case MINUTES_1:
		return "1m"
	case MINUTES_5:
		return "5m"
	case MINUTES_15:
		return "15m"
	case MINUTES_30:
		return "30m"
	case MINUTES_60:
		return "60m"
	case HOURS_4:
		return "4h"
	case DAY_1:
		return "1d"
	case WEEK_1:
		return "1W"
	case MONTH_1:
		return "1M"
	default:
		return "UNKNOWN"
	}
}

func (p KlineInterval) StringPtr() *string {
	res := p.String()
	return &res
}
