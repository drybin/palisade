package enum

const (
    D7 KlineIntervalV2 = iota + 1
)

type KlineIntervalV2 int

func (p KlineIntervalV2) IsValid() bool {
    switch p {
    case D7:
        return true
    default:
        return false
    }
}

func (p KlineIntervalV2) String() string {
    switch p {
    case D7:
        return "7D"
    default:
        return "UNKNOWN"
    }
}

func (p KlineIntervalV2) StringPtr() *string {
    res := p.String()
    return &res
}
