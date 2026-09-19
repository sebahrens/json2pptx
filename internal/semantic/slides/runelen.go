package slides

import "unicode/utf8"

// runeLen counts the CHARACTERS in a string. The budgets in this package are
// the pattern's own character budgets, and they decide whether a kind compiles
// to a visual or falls back to bullets — so counting bytes silently downgraded
// any deck written in a currency symbol or an umlaut: "€186.4M" is 7
// characters and 9 bytes (go-slide-creator-5ok4).
func runeLen(s string) int { return utf8.RuneCountInString(s) }
