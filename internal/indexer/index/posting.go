// Package index defines the in-memory inverted-index data structures used by
// the indexer. It provides PostingList, TermEntry, and a concurrent
// MemoryIndex that supports add, search, snapshot, and reset operations.
package index

// Posting records a single document's occurrence data for a term.
type Posting struct {
	DocID     string
	Frequency int
	Positions []int
}

// PostingList is a slice of Posting entries for one term.
type PostingList []Posting

// TermEntry pairs a term string with its PostingList, used when
// snapshotting the memory index for segment flushing.
type TermEntry struct {
	Term     string
	Postings PostingList
}

// DocStats holds per-document statistics used for relevance scoring.
type DocStats struct {
	DocID    string
	DocLen   int
	TermFreq int
}
