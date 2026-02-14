// Package tokenizer provides text tokenisation for the search engine.
// It lower-cases input, splits on non-alphanumeric boundaries, removes
// stop-words, and applies a simple suffix-based stemmer.
package tokenizer

import (
	"strings"
	"unicode"
)

var stopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {},
	"be": {}, "by": {}, "for": {}, "from": {}, "has": {}, "he": {},
	"in": {}, "is": {}, "it": {}, "its": {}, "of": {}, "on": {},
	"or": {}, "that": {}, "the": {}, "to": {}, "was": {}, "were": {},
	"will": {}, "with": {}, "this": {}, "but": {}, "they": {},
	"have": {}, "had": {}, "what": {}, "when": {}, "where": {},
	"who": {}, "which": {}, "their": {}, "if": {}, "each": {},
	"do": {}, "not": {}, "no": {}, "so": {}, "can": {},
}

// Token represents a single normalised term and its position in the
// original text.
type Token struct {
	Term     string
	Position int
}

// Tokenize breaks text into a slice of stemmed, lowercased Tokens with
// stop-words removed.
func Tokenize(text string) []Token {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	tokens := make([]Token, 0, len(words)/2)
	pos := 0
	for _, word := range words {
		if len(word) < 2 {
			continue
		}
		if _, isStop := stopWords[word]; isStop {
			continue
		}
		stemmed := stem(word)
		if stemmed == "" {
			continue
		}
		tokens = append(tokens, Token{
			Term:     stemmed,
			Position: pos,
		})
		pos++
	}
	return tokens
}

// stem applies a simple suffix-stripping stemmer to the given word.
func stem(word string) string {
	suffixes := []struct {
		suffix      string
		replacement string
		minLen      int
	}{
		{"ational", "ate", 2},
		{"tional", "tion", 2},
		{"encies", "ence", 2},
		{"ances", "ance", 2},
		{"ments", "ment", 2},
		{"izing", "ize", 2},
		{"ating", "ate", 2},
		{"iness", "y", 2},
		{"ously", "ous", 2},
		{"ively", "ive", 2},
		{"eness", "ene", 2},
		{"ments", "ment", 2},
		{"tion", "t", 3},
		{"sion", "s", 3},
		{"ying", "y", 2},
		{"ling", "l", 3},
		{"ies", "y", 2},
		{"ing", "", 3},
		{"ers", "er", 2},
		{"est", "", 3},
		{"ful", "", 3},
		{"ous", "", 3},
		{"ess", "", 3},
		{"ble", "", 3},
		{"ed", "", 3},
		{"er", "", 3},
		{"ly", "", 3},
		{"es", "", 3},
		{"ss", "ss", 2},
		{"s", "", 3},
	}
	for _, rule := range suffixes {
		if strings.HasSuffix(word, rule.suffix) {
			newWord := word[:len(word)-len(rule.suffix)] + rule.replacement
			if len(newWord) >= rule.minLen {
				return newWord
			}
		}
	}
	return word
}
