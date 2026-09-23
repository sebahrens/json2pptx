package patterns

import (
	"strings"
	"unicode"
)

// intentWords splits punctuation-separated intent text into words. Matching
// these words instead of substrings prevents "stat" in "state", "cons" in
// "consolidation", and "vs" in "advisors" from activating unrelated rules.
func intentWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func stemIntentWord(word string) string {
	if len(word) < 4 {
		return word
	}
	if strings.HasSuffix(word, "ies") && len(word) > 5 {
		return word[:len(word)-3] + "y"
	}
	if strings.HasSuffix(word, "ing") && len(word) > 6 {
		return word[:len(word)-3]
	}
	if strings.HasSuffix(word, "ed") && len(word) > 5 {
		return word[:len(word)-2]
	}
	if strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") {
		return word[:len(word)-1]
	}
	return word
}

func intentContainsPhrase(intent, phrase []string) bool {
	if len(phrase) == 0 || len(phrase) > len(intent) {
		return false
	}
	for start := 0; start <= len(intent)-len(phrase); start++ {
		matched := true
		for i, word := range phrase {
			if stemIntentWord(intent[start+i]) != stemIntentWord(word) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// matchIntentKeywords returns the number of distinct rule keywords that match
// and the longest matching phrase. The latter rewards specific phrases over a
// generic single token when their base scores tie.
func matchIntentKeywords(keywords []string, intentLower string) (count, longest int) {
	intent := intentWords(intentLower)
	for _, kw := range keywords {
		words := intentWords(kw)
		if intentContainsPhrase(intent, words) {
			count++
			if len(words) > longest {
				longest = len(words)
			}
		}
	}
	return count, longest
}

func keywordSpecificityBonus(longest int) float64 {
	if longest <= 1 {
		return 0
	}
	return 0.04 * float64(min(longest-1, 2))
}
