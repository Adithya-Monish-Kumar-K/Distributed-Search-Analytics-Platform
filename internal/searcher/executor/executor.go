package executor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/index"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/parser"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/ranker"
)

type SearchResult struct {
	Query     string             `json:"query"`
	TotalHits int                `json:"total_hits"`
	Results   []ranker.ScoredDoc `json:"results"`
	TermStats map[string]int     `json:"term_stats"`
}

type Executor struct {
	engine *indexer.Engine
	logger *slog.Logger
}

func New(engine *indexer.Engine) *Executor {
	return &Executor{
		engine: engine,
		logger: slog.Default().With("component", "query-executor"),
	}
}

func (e *Executor) Execute(ctx context.Context, plan *parser.QueryPlan, limit int) (*SearchResult, error) {
	if len(plan.Terms) == 0 {
		return &SearchResult{
			Query:   plan.RawQuery,
			Results: []ranker.ScoredDoc{},
		}, nil
	}

	postingsPerTerm := make(map[string]index.PostingList)
	termStats := make(map[string]int)
	for _, term := range plan.Terms {
		postings, err := e.engine.Search(term)
		if err != nil {
			return nil, fmt.Errorf("searching term %q: %w", term, err)
		}
		if len(postings) > 0 {
			postingsPerTerm[term] = postings
			termStats[term] = len(postings)
		}
	}
	excludeDocIDs := make(map[string]struct{})
	for _, term := range plan.ExcludeTerms {
		postings, err := e.engine.Search(term)
		if err != nil {
			e.logger.Error("searching exclude term failed", "term", term, "error", err)
			continue
		}
		for _, p := range postings {
			excludeDocIDs[p.DocID] = struct{}{}
		}
	}
	var candidateDocIDs map[string]struct{}
	switch plan.Type {
	case parser.QueryAND:
		candidateDocIDs = intersectPostings(postingsPerTerm)
	case parser.QueryOR:
		candidateDocIDs = unionPostings(postingsPerTerm)
	}
	for docID := range excludeDocIDs {
		delete(candidateDocIDs, docID)
	}
	filteredPostings := make(map[string]index.PostingList)
	for term, postings := range postingsPerTerm {
		filtered := make(index.PostingList, 0)
		for _, p := range postings {
			if _, ok := candidateDocIDs[p.DocID]; ok {
				filtered = append(filtered, p)
			}
		}
		if len(filtered) > 0 {
			filteredPostings[term] = filtered
		}
	}
	params := ranker.RankParams{
		TotalDocs:    e.engine.GetTotalDocs(),
		AvgDocLength: e.engine.GetAvgDocLength(),
	}
	getDocInfo := func(docID string) ranker.DocInfo {
		return ranker.DocInfo{
			DocLength: e.engine.GetDocLength(docID),
		}
	}
	ranked := ranker.Rank(filteredPostings, params, getDocInfo, limit)
	e.logger.Info("query executed",
		"query", plan.RawQuery,
		"terms", plan.Terms,
		"candidates", len(candidateDocIDs),
		"results", len(ranked),
	)
	return &SearchResult{
		Query:     plan.RawQuery,
		TotalHits: len(candidateDocIDs),
		Results:   ranked,
		TermStats: termStats,
	}, nil
}

func intersectPostings(postingsPerTerm map[string]index.PostingList) map[string]struct{} {
	if len(postingsPerTerm) == 0 {
		return make(map[string]struct{})
	}
	var shortestTerm string
	shortestLen := int(^uint(0) >> 1)
	for term, postings := range postingsPerTerm {
		if len(postings) < shortestLen {
			shortestLen = len(postings)
			shortestTerm = term
		}
	}
	candidates := make(map[string]struct{})
	for _, p := range postingsPerTerm[shortestTerm] {
		candidates[p.DocID] = struct{}{}
	}
	for term, postings := range postingsPerTerm {
		if term == shortestTerm {
			continue
		}
		docSet := make(map[string]struct{}, len(postings))
		for _, p := range postings {
			docSet[p.DocID] = struct{}{}
		}
		for docID := range candidates {
			if _, exists := docSet[docID]; !exists {
				delete(candidates, docID)
			}
		}
	}
	return candidates
}

func unionPostings(postingsPerTerm map[string]index.PostingList) map[string]struct{} {
	result := make(map[string]struct{})
	for _, postings := range postingsPerTerm {
		for _, p := range postings {
			result[p.DocID] = struct{}{}
		}
	}
	return result
}
