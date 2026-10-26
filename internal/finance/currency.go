package finance

import (
	"fmt"
	"math"
	"strings"

	"aosphere/internal/db"
)

// SupportedCurrencies maps currency ISO codes to their metadata
var SupportedCurrencies = map[string]db.CurrencyInfo{
	// South Asia
	"INR": {Code: "INR", Name: "Indian Rupee", Symbol: "₹", Decimals: 2, Region: "South Asia", DefaultRate: 83.50},
	"PKR": {Code: "PKR", Name: "Pakistani Rupee", Symbol: "₨", Decimals: 2, Region: "South Asia", DefaultRate: 278.50},
	"BDT": {Code: "BDT", Name: "Bangladeshi Taka", Symbol: "৳", Decimals: 2, Region: "South Asia", DefaultRate: 119.50},
	"LKR": {Code: "LKR", Name: "Sri Lankan Rupee", Symbol: "Rs", Decimals: 2, Region: "South Asia", DefaultRate: 293.00},
	"NPR": {Code: "NPR", Name: "Nepalese Rupee", Symbol: "रू", Decimals: 2, Region: "South Asia", DefaultRate: 133.50},
	"BTN": {Code: "BTN", Name: "Bhutanese Ngultrum", Symbol: "Nu.", Decimals: 2, Region: "South Asia", DefaultRate: 83.50},
	"MVR": {Code: "MVR", Name: "Maldivian Rufiyaa", Symbol: "Rf", Decimals: 2, Region: "South Asia", DefaultRate: 15.45},
	"AFN": {Code: "AFN", Name: "Afghan Afghani", Symbol: "؋", Decimals: 2, Region: "South Asia", DefaultRate: 67.50},

	// East Asia
	"JPY": {Code: "JPY", Name: "Japanese Yen", Symbol: "¥", Decimals: 0, Region: "East Asia", DefaultRate: 153.20},
	"CNY": {Code: "CNY", Name: "Chinese Yuan", Symbol: "¥", Decimals: 2, Region: "East Asia", DefaultRate: 7.23},
	"HKD": {Code: "HKD", Name: "Hong Kong Dollar", Symbol: "HK$", Decimals: 2, Region: "East Asia", DefaultRate: 7.78},
	"TWD": {Code: "TWD", Name: "New Taiwan Dollar", Symbol: "NT$", Decimals: 2, Region: "East Asia", DefaultRate: 32.20},
	"KRW": {Code: "KRW", Name: "South Korean Won", Symbol: "₩", Decimals: 0, Region: "East Asia", DefaultRate: 1380.00},
	"MOP": {Code: "MOP", Name: "Macanese Pataca", Symbol: "MOP$", Decimals: 2, Region: "East Asia", DefaultRate: 8.01},
	"MNT": {Code: "MNT", Name: "Mongolian Tögrög", Symbol: "₮", Decimals: 2, Region: "East Asia", DefaultRate: 3450.00},

	// Southeast Asia
	"SGD": {Code: "SGD", Name: "Singapore Dollar", Symbol: "S$", Decimals: 2, Region: "Southeast Asia", DefaultRate: 1.32},
	"MYR": {Code: "MYR", Name: "Malaysian Ringgit", Symbol: "RM", Decimals: 2, Region: "Southeast Asia", DefaultRate: 4.45},
	"THB": {Code: "THB", Name: "Thai Baht", Symbol: "฿", Decimals: 2, Region: "Southeast Asia", DefaultRate: 36.80},
	"IDR": {Code: "IDR", Name: "Indonesian Rupiah", Symbol: "Rp", Decimals: 0, Region: "Southeast Asia", DefaultRate: 15850.00},
	"PHP": {Code: "PHP", Name: "Philippine Peso", Symbol: "₱", Decimals: 2, Region: "Southeast Asia", DefaultRate: 58.20},
	"VND": {Code: "VND", Name: "Vietnamese Dong", Symbol: "₫", Decimals: 0, Region: "Southeast Asia", DefaultRate: 25400.00},
	"KHR": {Code: "KHR", Name: "Cambodian Riel", Symbol: "៛", Decimals: 0, Region: "Southeast Asia", DefaultRate: 4050.00},
	"LAK": {Code: "LAK", Name: "Lao Kip", Symbol: "₭", Decimals: 0, Region: "Southeast Asia", DefaultRate: 21800.00},
	"MMK": {Code: "MMK", Name: "Myanmar Kyat", Symbol: "K", Decimals: 0, Region: "Southeast Asia", DefaultRate: 2095.00},
	"BND": {Code: "BND", Name: "Brunei Dollar", Symbol: "B$", Decimals: 2, Region: "Southeast Asia", DefaultRate: 1.32},

	// West Asia / Middle East
	"AED": {Code: "AED", Name: "UAE Dirham", Symbol: "AED", Decimals: 2, Region: "Middle East", DefaultRate: 3.67},
	"SAR": {Code: "SAR", Name: "Saudi Riyal", Symbol: "SAR", Decimals: 2, Region: "Middle East", DefaultRate: 3.75},
	"QAR": {Code: "QAR", Name: "Qatari Riyal", Symbol: "QAR", Decimals: 2, Region: "Middle East", DefaultRate: 3.64},
	"KWD": {Code: "KWD", Name: "Kuwaiti Dinar", Symbol: "KWD", Decimals: 3, Region: "Middle East", DefaultRate: 0.31},
	"BHD": {Code: "BHD", Name: "Bahraini Dinar", Symbol: "BHD", Decimals: 3, Region: "Middle East", DefaultRate: 0.38},
	"OMR": {Code: "OMR", Name: "Omani Rial", Symbol: "OMR", Decimals: 3, Region: "Middle East", DefaultRate: 0.38},
	"JOD": {Code: "JOD", Name: "Jordanian Dinar", Symbol: "JOD", Decimals: 3, Region: "Middle East", DefaultRate: 0.71},
	"ILS": {Code: "ILS", Name: "Israeli New Shekel", Symbol: "₪", Decimals: 2, Region: "Middle East", DefaultRate: 3.72},

	// Central Asia
	"KZT": {Code: "KZT", Name: "Kazakhstani Tenge", Symbol: "₸", Decimals: 2, Region: "Central Asia", DefaultRate: 488.00},
	"UZS": {Code: "UZS", Name: "Uzbekistani Som", Symbol: "so'm", Decimals: 0, Region: "Central Asia", DefaultRate: 12800.00},

	// Global Base Currencies
	"USD": {Code: "USD", Name: "US Dollar", Symbol: "$", Decimals: 2, Region: "Global", DefaultRate: 1.00},
	"EUR": {Code: "EUR", Name: "Euro", Symbol: "€", Decimals: 2, Region: "Global", DefaultRate: 0.92},
	"GBP": {Code: "GBP", Name: "British Pound", Symbol: "£", Decimals: 2, Region: "Global", DefaultRate: 0.77},
	"CAD": {Code: "CAD", Name: "Canadian Dollar", Symbol: "C$", Decimals: 2, Region: "Global", DefaultRate: 1.38},
	"AUD": {Code: "AUD", Name: "Australian Dollar", Symbol: "A$", Decimals: 2, Region: "Global", DefaultRate: 1.51},
}

// GetAllCurrencies returns a slice of all supported currencies
func GetAllCurrencies() []db.CurrencyInfo {
	list := make([]db.CurrencyInfo, 0, len(SupportedCurrencies))
	for _, c := range SupportedCurrencies {
		list = append(list, c)
	}
	return list
}

// GetCurrency returns metadata for a given currency code (defaults to USD if unknown)
func GetCurrency(code string) db.CurrencyInfo {
	code = strings.ToUpper(strings.TrimSpace(code))
	if c, ok := SupportedCurrencies[code]; ok {
		return c
	}
	return SupportedCurrencies["USD"]
}

// Convert converts an amount from fromCur to toCur using provided exchange rates (or default rates)
func Convert(amount float64, fromCur, toCur string, rates map[string]float64) float64 {
	fromCur = strings.ToUpper(strings.TrimSpace(fromCur))
	toCur = strings.ToUpper(strings.TrimSpace(toCur))

	if fromCur == toCur || amount == 0 {
		return amount
	}

	rateFrom := 1.0
	if fromCur != "USD" {
		if r, ok := rates[fromCur]; ok && r > 0 {
			rateFrom = r
		} else if c, ok := SupportedCurrencies[fromCur]; ok && c.DefaultRate > 0 {
			rateFrom = c.DefaultRate
		}
	}

	rateTo := 1.0
	if toCur != "USD" {
		if r, ok := rates[toCur]; ok && r > 0 {
			rateTo = r
		} else if c, ok := SupportedCurrencies[toCur]; ok && c.DefaultRate > 0 {
			rateTo = c.DefaultRate
		}
	}

	// amount (in fromCur) -> USD -> toCur
	amountInUSD := amount / rateFrom
	return amountInUSD * rateTo
}

// FormatCurrency formats a float amount with currency symbol, standard commas or Indian Lakh/Crore numbering
func FormatCurrency(amount float64, curCode string) string {
	cur := GetCurrency(curCode)
	dec := cur.Decimals

	if cur.Code == "INR" {
		return formatINR(amount, dec)
	}

	// Zero decimal currencies (e.g. JPY, KRW, VND, IDR)
	if dec == 0 {
		rounded := int64(math.Round(amount))
		return fmt.Sprintf("%s%s", cur.Symbol, formatWithCommas(rounded))
	}

	// Standard decimal representation
	if dec == 3 {
		return fmt.Sprintf("%s%.3f", cur.Symbol, amount)
	}
	return fmt.Sprintf("%s%.2f", cur.Symbol, amount)
}

// formatINR formats Indian Rupee using the Indian numbering system (Lakhs and Crores, e.g. ₹1,25,000.00)
func formatINR(amount float64, decimals int) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}

	intPart := int64(math.Floor(amount))
	decPart := amount - float64(intPart)

	s := fmt.Sprintf("%d", intPart)
	n := len(s)
	if n <= 3 {
		if decimals > 0 {
			return fmt.Sprintf("%s₹%s.%02d", sign, s, int64(math.Round(decPart*100)))
		}
		return fmt.Sprintf("%s₹%s", sign, s)
	}

	lastThree := s[n-3:]
	otherNumbers := s[:n-3]

	var formattedOther []string
	for len(otherNumbers) > 2 {
		formattedOther = append([]string{otherNumbers[len(otherNumbers)-2:]}, formattedOther...)
		otherNumbers = otherNumbers[:len(otherNumbers)-2]
	}
	if len(otherNumbers) > 0 {
		formattedOther = append([]string{otherNumbers}, formattedOther...)
	}

	res := strings.Join(formattedOther, ",") + "," + lastThree
	if decimals > 0 {
		return fmt.Sprintf("%s₹%s.%02d", sign, res, int64(math.Round(decPart*100)))
	}
	return fmt.Sprintf("%s₹%s", sign, res)
}

func formatWithCommas(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return sign + s
	}

	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	if len(s) > 0 {
		parts = append([]string{s}, parts...)
	}
	return sign + strings.Join(parts, ",")
}
