package parser

import (
	"strings"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/tokenizer"
)

type QueryType int

const (
	QueryAND QueryType = iota
	QueryOR
)

type QueryPlan struct {
	Terms        []string
	Type         QueryType
	ExcludeTerms []string
	RawQuery     string
}

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
