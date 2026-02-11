package ranker

import (
	"math"
	"sort"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/index"
)

const (
	k1 = 1.2
	b  = 0.75
)

type ScoredDoc struct {
	DocID string  `json:"doc_id"`
	Score float64 `json:"score"`
}

type RankParams struct {
	TotalDocs    int64
	AvgDocLength float64
}

type DocInfo struct {
	DocLength int
}

func Rank(
	postingsPerTerm map[string]index.PostingList,
	params RankParams,
	getDocInfo func(docID string) DocInfo,
	limit int,
) []ScoredDoc {
	scores := make(map[string]float64)
	for term, postings := range postingsPerTerm {
		docFreq := len(postings)
		idf := computeIDF(params.TotalDocs, int64(docFreq))
		for _, posting := range postings {
			info := getDocInfo(posting.DocID)
			tfNorm := computeTFNorm(
				float64(posting.Frequency),
				float64(info.DocLength),
				params.AvgDocLength,
			)
			scores[posting.DocID] += idf * tfNorm
			_ = term
		}
	}
	result := make([]ScoredDoc, 0, len(scores))
	for docID, score := range scores {
		result = append(result, ScoredDoc{
			DocID: docID,
			Score: math.Round(score*10000) / 10000,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		return result[i].DocID < result[j].DocID
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

func computeIDF(totalDocs int64, docFreq int64) float64 {
	numerator := float64(totalDocs) - float64(docFreq)
	denominator := float64(docFreq) + 0.5
	return math.Log(numerator/denominator + 1)
}

func computeTFNorm(termFreq float64, docLength float64, avgDocLength float64) float64 {
	if avgDocLength == 0 {
		return 0
	}
	lengthRatio := docLength / avgDocLength
	denominator := termFreq + k1*(1-b+b*lengthRatio)
	return (termFreq * (k1 + 1)) / denominator
}
