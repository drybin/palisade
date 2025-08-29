package order

const (
	BUY  Side = iota + 1
	SELL Side = iota + 1
)

type Side int

func (p Side) IsValid() bool {
	switch p {
	case BUY:
		return true
	case SELL:
		return true
	default:
		return false
	}
}

func (p Side) String() string {
	switch p {
	case BUY:
		return "BUY"
	case SELL:
		return "SELL"
	default:
		return "UNKNOWN"
	}
}
