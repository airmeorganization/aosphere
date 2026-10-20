package finance

import (
	"fmt"
	"math"
)

// CurrencyUSD is the canonical currency code
const CurrencyUSD = "USD"

// DefaultTaxRateBasisPoints is 8.00% (800 basis points)
const DefaultTaxRateBasisPoints int64 = 800

// DollarsToCents converts floating point dollars into integer cents with proper rounding
func DollarsToCents(amount float64) int64 {
	return int64(math.Round(amount * 100.0))
}

// CentsToDollars converts integer cents back to float64 dollars
func CentsToDollars(cents int64) float64 {
	return float64(cents) / 100.0
}

// FormatUSD formats cents into standard dollar representation "$X,XXX.XX"
func FormatUSD(cents int64) string {
	dollars := float64(cents) / 100.0
	return fmt.Sprintf("$%.2f", dollars)
}

// CalculateTax computes sales tax using integer basis points (1 basis point = 0.01%)
// Example: 800 basis points = 8.00% tax. Rounding is half-up.
func CalculateTax(subtotalCents int64, rateBasisPoints int64) int64 {
	if subtotalCents <= 0 || rateBasisPoints <= 0 {
		return 0
	}
	// (subtotalCents * rateBasisPoints + 5000) / 10000 for half-up rounding
	product := subtotalCents * rateBasisPoints
	return (product + 5000) / 10000
}
