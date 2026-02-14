// Package parser converts raw search query strings into structured QueryPlan
// objects, recognising AND, OR, and NOT operators and delegating token
// normalisation to the indexer tokenizer.
package parser

import (
	"strings"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/tokenizer"
)

// QueryType indicates the Boolean combination mode for query terms.
type QueryType int

const (
	QueryAND QueryType = iota
	QueryOR
)

// QueryPlan is the parsed representation of a search query, containing the
// include terms, exclude terms, Boolean type, and the original query string.
type QueryPlan struct {
	Terms        []string
	Type         QueryType
	ExcludeTerms []string
	RawQuery     string
}

// Parse tokenises the query string and produces a QueryPlan. Operators AND,
// OR, and NOT are recognised case-insensitively.
func Parse(query string) *QueryPlan {
	plan := &QueryPlan{
		Terms:        make([]string, 0),
		ExcludeTerms: make([]string, 0),
		Type:         QueryAND,
		RawQuery:     query,
	}
	if strings.TrimSpace(query) == "" {
		return plan
	}
	words := strings.Fields(query)
	excludeNext := false
	for i := 0; i < len(words); i++ {
		upper := strings.ToUpper(words[i])
		switch upper {
		case "AND":
			plan.Type = QueryAND
			continue
		case "OR":
			plan.Type = QueryOR
			continue
		case "NOT":
			excludeNext = true
			continue
		}
		tokens := tokenizer.Tokenize(words[i])
		if len(tokens) == 0 {
			continue
		}
		term := tokens[0].Term
		if excludeNext {
			plan.ExcludeTerms = append(plan.ExcludeTerms, term)
			excludeNext = false
		} else {
			plan.Terms = append(plan.Terms, term)
		}
	}
	return plan

}
