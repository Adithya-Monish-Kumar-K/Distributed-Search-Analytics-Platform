package merger

import (
	"container/heap"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/ranker"
)

func Merge(shardResults [][]ranker.ScoredDoc, limit int) []ranker.ScoredDoc {
	if limit <= 0 {
		limit = 10
	}
	h := &scoredDocHeap{}
	heap.Init(h)
	for _, results := range shardResults {
		for _, doc := range results {
			heap.Push(h, doc)
			if h.Len() > limit {
				heap.Pop(h)
			}
		}
	}
	result := make([]ranker.ScoredDoc, h.Len())
	for i := len(result) - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(ranker.ScoredDoc)
	}
	return result
}

type scoredDocHeap []ranker.ScoredDoc

func (h scoredDocHeap) Len() int { return len(h) }

func (h scoredDocHeap) Less(i, j int) bool {
	if h[i].Score != h[j].Score {
		return h[i].Score < h[j].Score
	}
	return h[i].DocID > h[j].DocID
}

func (h scoredDocHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *scoredDocHeap) Push(x interface{}) {
	*h = append(*h, x.(ranker.ScoredDoc))
}

func (h *scoredDocHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
