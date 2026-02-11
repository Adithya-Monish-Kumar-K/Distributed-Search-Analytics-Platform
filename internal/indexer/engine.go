package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/index"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/segment"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/tokenizer"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
)

type Engine struct {
	memIndex     *index.MemoryIndex
	writer       *segment.Writer
	readers      []*segment.Reader
	readerMu     sync.RWMutex
	cfg          config.IndexerConfig
	logger       *slog.Logger
	docLengths   map[string]int
	docLengthsMu sync.RWMutex
	totalDocs    int64
	totalTokens  int64
}

func NewEngine(cfg config.IndexerConfig) (*Engine, error) {
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("creating index data directory: %w", err)
	}
	e := &Engine{
		memIndex:   index.NewMemoryIndex(),
		writer:     segment.NewWriter(cfg.DataDir),
		cfg:        cfg,
		logger:     slog.Default().With("component", "indexer"),
		docLengths: make(map[string]int),
	}
	if err := e.loadExistingSegments(); err != nil {
		return nil, fmt.Errorf("loading existing segments: %w", err)
	}
	return e, nil
}

func (e *Engine) IndexDocument(docID string, title string, body string) error {
	fullText := title + " " + body
	tokens := tokenizer.Tokenize(fullText)

	e.docLengthsMu.Lock()
	e.docLengths[docID] = len(tokens)
	e.totalDocs++
	e.totalTokens += int64(len(tokens))
	e.docLengthsMu.Unlock()

	e.memIndex.AddDocument(docID, title, body)
	e.logger.Debug("document indexed in memory",
		"doc_id", docID,
		"token_count", len(tokens),
		"mem_size", e.memIndex.Size(),
	)
	if e.memIndex.Size() >= e.cfg.SegmentMaxSize {
		e.logger.Info("memory index reached max size, flushing to disk",
			"size", e.memIndex.Size(),
			"threshold", e.cfg.SegmentMaxSize,
		)
		if err := e.Flush(); err != nil {
			return fmt.Errorf("flushing memory index: %w", err)
		}
	}
	return nil
}

func (e *Engine) Flush() error {
	snapshot := e.memIndex.Snapshot()
	if len(snapshot) == 0 {
		return nil
	}
	segmentName, err := e.writer.Write(snapshot)
	if err != nil {
		return fmt.Errorf("writing segment: %w", err)
	}

	segPath := filepath.Join(e.cfg.DataDir, segmentName)
	reader, err := segment.OpenReader(segPath)
	if err != nil {
		return fmt.Errorf("opening new segment for reading: %w", err)
	}
	e.readerMu.Lock()
	e.readers = append(e.readers, reader)
	e.readerMu.Unlock()
	e.memIndex.Reset()
	e.logger.Info("segment flushed",
		"segment", segmentName,
		"terms", reader.Terms(),
		"docs", reader.DocCount(),
		"active_segments", len(e.readers),
	)
	return nil
}

func (e *Engine) Search(term string) (index.PostingList, error) {
	tokens := tokenizer.Tokenize(term)
	if len(tokens) == 0 {
		return nil, nil
	}
	normalizedTerm := tokens[0].Term
	allPostings := e.memIndex.Search(normalizedTerm)
	e.readerMu.RLock()
	readers := make([]*segment.Reader, len(e.readers))
	copy(readers, e.readers)
	e.readerMu.RUnlock()

	for _, reader := range readers {
		postings, err := reader.Search(normalizedTerm)
		if err != nil {
			e.logger.Error("segment search failed",
				"error", err,
			)
			continue
		}
		allPostings = append(allPostings, postings...)
	}
	allPostings = deduplicatePostings(allPostings)
	return allPostings, nil
}

func (e *Engine) GetDocLength(docID string) int {
	e.docLengthsMu.RLock()
	defer e.docLengthsMu.RUnlock()
	return e.docLengths[docID]
}

func (e *Engine) GetAvgDocLength() float64 {
	e.docLengthsMu.RLock()
	defer e.docLengthsMu.RUnlock()
	if e.totalDocs == 0 {
		return 0
	}
	return float64(e.totalTokens) / float64(e.totalDocs)
}

func (e *Engine) GetTotalDocs() int64 {
	e.docLengthsMu.RLock()
	defer e.docLengthsMu.RUnlock()
	return e.totalDocs
}

func (e *Engine) StartFlushLoop(ctx context.Context) {
	ticker := time.NewTicker(e.cfg.FlushInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				e.logger.Info("flush loop stopping, performing final flush")
				if err := e.Flush(); err != nil {
					e.logger.Error("final flush failed", "error", err)
				}
				return
			case <-ticker.C:
				if e.memIndex.DocCount() > 0 {
					if err := e.Flush(); err != nil {
						e.logger.Error("periodic flush failed", "error", err)
					}
				}
			}
		}
	}()
}

func (e *Engine) Close() error {
	if err := e.Flush(); err != nil {
		e.logger.Error("final flush on close failed", "error", err)
	}
	e.readerMu.Lock()
	defer e.readerMu.Unlock()
	for _, reader := range e.readers {
		if err := reader.Close(); err != nil {
			e.logger.Error("closing segment reader", "error", err)
		}
	}
	e.readers = nil
	return nil
}

func (e *Engine) loadExistingSegments() error {
	entries, err := os.ReadDir(e.cfg.DataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading data directory: %w", err)
	}
	segFiles := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".spdx") {
			segFiles = append(segFiles, entry.Name())
		}
	}
	sort.Strings(segFiles)

	for _, name := range segFiles {
		path := filepath.Join(e.cfg.DataDir, name)
		reader, err := segment.OpenReader(path)
		if err != nil {
			e.logger.Error("failed to open segment, skipping",
				"segment", name,
				"error", err,
			)
			continue
		}
		e.readers = append(e.readers, reader)
		e.logger.Info("loaded existing segment",
			"segment", name,
			"terms", reader.Terms(),
			"docs", reader.DocCount(),
		)
	}
	e.logger.Info("segment recovery complete", "segments_loaded", len(e.readers))
	return nil
}

func deduplicatePostings(postings index.PostingList) index.PostingList {
	if len(postings) <= 1 {
		return postings
	}
	seen := make(map[string]int)
	result := make(index.PostingList, 0, len(postings))
	for _, p := range postings {
		if idx, exists := seen[p.DocID]; exists {
			if p.Frequency > result[idx].Frequency {
				result[idx] = p
			}
		} else {
			seen[p.DocID] = len(result)
			result = append(result, p)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].DocID < result[j].DocID
	})
	return result
}
