package executor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/index"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/parser"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/ranker"
)

type ShardResult struct {
	ShardID   int
	Postings  map[string]index.PostingList
	TotalDocs int64
	AvgDocLen float64
	Engine    *indexer.Engine
}

type ShardedExecutor struct {
	engines map[int]*indexer.Engine
	logger  *slog.Logger
}

func NewSharded(engines map[int]*indexer.Engine) *ShardedExecutor {
	return &ShardedExecutor{
		engines: engines,
		logger:  slog.Default().With("component", "sharded-executor"),
	}
}

func (se *ShardedExecutor) Execute(ctx context.Context, plan *parser.QueryPlan, limit int) (*SearchResult, error) {
	if len(plan.Terms) == 0 {
		return &SearchResult{
			Query:   plan.RawQuery,
			Results: []ranker.ScoredDoc{},
		}, nil
	}
	shardResults, err := se.fanOut(ctx, plan)
	if err != nil {
		return nil, fmt.Errorf("shard fan-out: %w", err)
	}
	mergedPostings := make(map[string]index.PostingList)
	termStats := make(map[string]int)
	var globalTotalDocs int64
	var globalTotalTokens float64
	engineLookup := make(map[string]*indexer.Engine)
	for _, sr := range shardResults {
		globalTotalDocs += sr.TotalDocs
		globalTotalTokens += sr.AvgDocLen * float64(sr.TotalDocs)
		for term, postings := range sr.Postings {
			mergedPostings[term] = append(mergedPostings[term], postings...)
			termStats[term] += len(postings)
		}
		for term := range sr.Postings {
			for _, p := range sr.Postings[term] {
				engineLookup[p.DocID] = sr.Engine
			}
		}
	}
	var globalAvgDocLen float64
	if globalTotalDocs > 0 {
		globalAvgDocLen = globalTotalTokens / float64(globalTotalDocs)
	}
	excludeDocIDs := make(map[string]struct{})
	for _, sr := range shardResults {
		for _, term := range plan.ExcludeTerms {
			for _, p := range sr.Postings[term] {
				excludeDocIDs[p.DocID] = struct{}{}
			}
		}
	}
	searchPostings := make(map[string]index.PostingList)
	for _, term := range plan.Terms {
		if postings, ok := mergedPostings[term]; ok {
			searchPostings[term] = postings
		}
	}

	var candidateDocIDs map[string]struct{}
	switch plan.Type {
	case parser.QueryAND:
		candidateDocIDs = intersectPostings(searchPostings)
	case parser.QueryOR:
		candidateDocIDs = unionPostings(searchPostings)
	}

	for docID := range excludeDocIDs {
		delete(candidateDocIDs, docID)
	}
	filteredPostings := make(map[string]index.PostingList)
	for term, postings := range searchPostings {
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
		TotalDocs:    globalTotalDocs,
		AvgDocLength: globalAvgDocLen,
	}

	getDocInfo := func(docID string) ranker.DocInfo {
		if engine, ok := engineLookup[docID]; ok {
			return ranker.DocInfo{
				DocLength: engine.GetDocLength(docID),
			}
		}
		return ranker.DocInfo{DocLength: 0}
	}
	ranked := ranker.Rank(filteredPostings, params, getDocInfo, limit)
	se.logger.Info("sharded query executed",
		"query", plan.RawQuery,
		"shards_queried", len(shardResults),
		"global_candidates", len(candidateDocIDs),
		"results", len(ranked),
	)
	return &SearchResult{
		Query:     plan.RawQuery,
		TotalHits: len(candidateDocIDs),
		Results:   ranked,
		TermStats: termStats,
	}, nil
}

func (se *ShardedExecutor) fanOut(ctx context.Context, plan *parser.QueryPlan) ([]ShardResult, error) {
	type result struct {
		sr  ShardResult
		err error
	}
	allTerms := append(plan.Terms, plan.ExcludeTerms...)
	results := make([]result, len(se.engines))
	var wg sync.WaitGroup
	i := 0
	for shardID, engine := range se.engines {
		wg.Add(1)
		go func(idx int, sid int, eng *indexer.Engine) {
			defer wg.Done()
			sr := ShardResult{
				ShardID:   sid,
				Postings:  make(map[string]index.PostingList),
				TotalDocs: eng.GetTotalDocs(),
				AvgDocLen: eng.GetAvgDocLength(),
				Engine:    eng,
			}
			for _, term := range allTerms {
				postings, err := eng.Search(term)
				if err != nil {
					results[idx] = result{err: fmt.Errorf("shard %d, term %q: %w", sid, term, err)}
					return
				}
				if len(postings) > 0 {
					sr.Postings[term] = postings
				}
			}
			results[idx] = result{sr: sr}
		}(i, shardID, engine)
		i++
	}
	wg.Wait()
	shardResults := make([]ShardResult, 0, len(se.engines))
	for _, r := range results {
		if r.err != nil {
			se.logger.Error("shard query failed", "error", r.err)
			continue
		}
		shardResults = append(shardResults, r.sr)
	}
	if len(shardResults) == 0 && len(se.engines) > 0 {
		return nil, fmt.Errorf("all %d shards failed", len(se.engines))
	}
	return shardResults, nil
}
