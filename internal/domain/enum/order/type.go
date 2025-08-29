package order

const (
	LIMIT               Type = iota + 1
	MARKET              Type = iota + 1
	LIMIT_MAKER         Type = iota + 1
	IMMEDIATE_OR_CANCEL Type = iota + 1
	FILL_OR_KILL        Type = iota + 1
)

type Type int

func (p Type) IsValid() bool {
	switch p {
	case LIMIT:
		return true
	case MARKET:
		return true
	case LIMIT_MAKER:
		return true
	case IMMEDIATE_OR_CANCEL:
		return true
	case FILL_OR_KILL:
		return true
	default:
		return false
	}
}

func (p Type) String() string {
	switch p {
	case LIMIT:
		return "LIMIT"
	case MARKET:
		return "MARKET"
	case LIMIT_MAKER:
		return "LIMIT_MAKER"
	case IMMEDIATE_OR_CANCEL:
		return "IMMEDIATE_OR_CANCEL"
	case FILL_OR_KILL:
		return "FILL_OR_KILL"
	default:
		return "UNKNOWN"
	}
}
