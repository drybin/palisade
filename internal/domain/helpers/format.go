package helpers

import (
	"strconv"
	"strings"
)

// FormatFloatTrimZeros formats a float64 without trailing zeros after the decimal point.
func FormatFloatTrimZeros(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// FormatFloatSignTrimZeros like FormatFloatTrimZeros but adds "+" for positive values (for P/L style).
func FormatFloatSignTrimZeros(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if f > 0 {
		return "+" + s
	}
	return s
}

// TrimDecimalZeros trims trailing zeros from a decimal string (e.g. "12.3400" -> "12.34").
func TrimDecimalZeros(s string) string {
	if s == "" || !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
