package index

import (
	"sort"
	"sync"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/tokenizer"
)

type MemoryIndex struct {
	mu       sync.RWMutex
	index    map[string]map[string]*Posting
	docCount int
	size     int64
}

func NewMemoryIndex() *MemoryIndex {
	return &MemoryIndex{
		index: make(map[string]map[string]*Posting),
	}
}

func (m *MemoryIndex) AddDocument(docID string, title string, body string) {
	fullText := title + " " + body
	tokens := tokenizer.Tokenize(fullText)

	termData := make(map[string]*Posting)

	for _, token := range tokens {
		p, exists := termData[token.Term]
		if !exists {
			p = &Posting{
				DocID:     docID,
				Frequency: 0,
				Positions: make([]int, 0, 4),
			}
			termData[token.Term] = p
		}
		p.Frequency++
		p.Positions = append(p.Positions, token.Position)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for term, posting := range termData {
		if _, exists := m.index[term]; !exists {
			m.index[term] = make(map[string]*Posting)
		}
		m.index[term][docID] = posting
		m.size += int64(len(term) + len(docID) + len(posting.Positions)*8 + 64)
	}
	m.docCount++
}

func (m *MemoryIndex) Search(term string) PostingList {
	m.mu.RLock()
	defer m.mu.RUnlock()
	docs, exists := m.index[term]
	if !exists {
		return nil
	}
	result := make(PostingList, 0, len(docs))
	for _, posting := range docs {
		result = append(result, *posting)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].DocID < result[j].DocID
	})
	return result
}

func (m *MemoryIndex) Snapshot() []TermEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entries := make([]TermEntry, 0, len(m.index))
	for term, docs := range m.index {
		postings := make(PostingList, 0, len(docs))
		for _, posting := range docs {
			postings = append(postings, *posting)
		}
		sort.Slice(postings, func(i, j int) bool {
			return postings[i].DocID < postings[j].DocID
		})
		entries = append(entries, TermEntry{
			Term:     term,
			Postings: postings,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Term < entries[j].Term
	})
	return entries
}

func (m *MemoryIndex) Size() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size
}

func (m *MemoryIndex) DocCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.docCount
}

func (m *MemoryIndex) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.index = make(map[string]map[string]*Posting)
	m.docCount = 0
	m.size = 0
}
