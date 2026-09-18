package patterns

import (
	"math"
	"strconv"
	"strings"
)

// currencySymbols are rendered BEFORE the number ("$210m", not "210$m").
const currencySymbols = "$€£¥₹₩₽₺₪₫฿₦₱"

// thinSpace separates a number from an alphabetic unit ("1,240.5 EUR M").
// U+2009 is narrower than a word space, which is the typographic convention
// for a value + unit pair and keeps the label compact inside a bar.
const thinSpace = " "

// FormatMagnitudeLabel renders a numeric value with an optional unit for use as
// a chart / bar value label. It is the single formatter shared by the
// value-bearing patterns so they cannot drift apart (go-slide-creator-d6zo,
// which is the same defect go-slide-creator-8u9k fixed for waterfall-bridge):
//
//   - thousands separators: 1240.5 -> "1,240.5"
//   - at most one decimal:  96.44  -> "96.4";  12.0 -> "12"
//   - currency units lead:  ("210", "$m")     -> "$210m"
//   - other units follow, separated by a thin space: ("1240.5", "EUR M") ->
//     "1,240.5 EUR M". A single-character symbolic unit such as "%" is
//     appended tight ("96.4%") since that is the convention.
//
// When signed is true the label carries an explicit "+" / "−" so a delta's
// direction is unambiguous.
func FormatMagnitudeLabel(v float64, unit string, signed bool) string {
	abs := math.Abs(v)
	body := groupThousands(roundToOneDecimal(abs))

	prefix, suffix := splitUnit(unit)
	body = prefix + body + suffix

	switch {
	case signed && v < 0:
		return "−" + body
	case signed && v > 0:
		return "+" + body
	default:
		return body
	}
}

// roundToOneDecimal renders a non-negative value with at most one decimal
// place, dropping a trailing ".0".
func roundToOneDecimal(abs float64) string {
	rounded := math.Round(abs*10) / 10
	if rounded == math.Trunc(rounded) {
		return strconv.FormatInt(int64(rounded), 10)
	}
	return strconv.FormatFloat(rounded, 'f', 1, 64)
}

// groupThousands inserts thousands separators into the integer part of an
// already-formatted decimal string.
func groupThousands(s string) string {
	intPart, frac := s, ""
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		intPart, frac = s[:dot], s[dot:]
	}
	if len(intPart) <= 3 {
		return intPart + frac
	}

	var b strings.Builder
	lead := len(intPart) % 3
	if lead > 0 {
		b.WriteString(intPart[:lead])
	}
	for i := lead; i < len(intPart); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(intPart[i : i+3])
	}
	return b.String() + frac
}

// splitUnit splits a unit into the part rendered before the number (a currency
// symbol, optionally preceded by a short country code: "$", "US$", "€") and the
// part rendered after it. A suffix made of letters is separated from the number
// by a thin space; a symbolic suffix such as "%" is kept tight.
func splitUnit(unit string) (prefix, suffix string) {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return "", ""
	}

	runes := []rune(unit)
	for i, r := range runes {
		if !strings.ContainsRune(currencySymbols, r) {
			continue
		}
		// Allow a short country code before the symbol (US$, A$, HK$).
		lead := string(runes[:i])
		if len(lead) <= 2 && strings.ToUpper(lead) == lead && !strings.ContainsAny(lead, "0123456789 ") {
			return string(runes[:i+1]), strings.TrimSpace(string(runes[i+1:]))
		}
		break
	}

	if unitNeedsSpace(unit) {
		return "", thinSpace + unit
	}
	return "", unit
}

// unitNeedsSpace reports whether a suffix unit reads as a word (or words) and
// therefore needs separating from the number.
//
// Tight: symbols ("%", "x") and the short magnitude suffixes that are
// conventionally set against the number ("12bn", "7m", "70kg").
// Separated: multi-word units ("EUR M") and words of three or more letters
// ("pts", "USD").
func unitNeedsSpace(unit string) bool {
	hasLetter := false
	for _, r := range unit {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r == ' ':
			return true // multi-word unit, e.g. "EUR M"
		case r == '.':
			// part of an abbreviation ("pts.")
		default:
			return false // contains a symbol: tight suffix ("%", "°C")
		}
	}
	return hasLetter && len([]rune(unit)) >= 3
}
