// Package segment implements a custom binary segment file format (.spdx) for
// persisting inverted-index data to disk. Each segment has a fixed-size header,
// a postings region, a JSON dictionary, and a CRC32 footer. The Writer creates
// new segments atomically, and the Reader provides random-access search over
// them.
package segment

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/index"
)

// Reader provides read-only access to a single .spdx segment file. It
// memory-maps the dictionary on open and performs random-access reads for
// posting lists.
type Reader struct {
	file     *os.File
	filePath string
	header   SegmentHeader
	dict     []DictEntry
	postBase int64
}

// OpenReader opens an existing segment file, validates the magic bytes, and
// loads the term dictionary into memory.
func OpenReader(path string) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening segment file: %w", err)
	}
	headerBytes := make([]byte, HeaderSize)
	if _, err := f.ReadAt(headerBytes, 0); err != nil {
		f.Close()
		return nil, fmt.Errorf("opening segment file: %w", err)
	}
	magic := binary.LittleEndian.Uint32(headerBytes[0:4])
	if magic != MagicBytes {
		f.Close()
		return nil, fmt.Errorf("invalid segment file: bad magic bytes %x", magic)
	}
	header := SegmentHeader{
		Magic:      magic,
		Version:    binary.LittleEndian.Uint32(headerBytes[4:8]),
		TermCount:  binary.LittleEndian.Uint32(headerBytes[8:12]),
		DocCount:   binary.LittleEndian.Uint32(headerBytes[12:16]),
		DictOffset: int64(binary.LittleEndian.Uint64(headerBytes[16:24])),
		DictSize:   int64(binary.LittleEndian.Uint64(headerBytes[24:32])),
		PostOffset: int64(binary.LittleEndian.Uint64(headerBytes[32:40])),
		PostSize:   int64(binary.LittleEndian.Uint64(headerBytes[40:48])),
	}
	dictBytes := make([]byte, header.DictSize)
	if _, err := f.ReadAt(dictBytes, header.DictOffset); err != nil {
		f.Close()
		return nil, fmt.Errorf("reading dictionary: %w", err)
	}
	var dict []DictEntry
	if err := json.Unmarshal(dictBytes, &dict); err != nil {
		f.Close()
		return nil, fmt.Errorf("parsing dictionary: %w", err)
	}
	return &Reader{
		file:     f,
		filePath: path,
		header:   header,
		dict:     dict,
		postBase: header.PostOffset,
	}, nil
}

// Search performs a binary search over the term dictionary and reads the
// matching PostingList from disk.
func (r *Reader) Search(term string) (index.PostingList, error) {
	idx := sort.Search(len(r.dict), func(i int) bool {
		return r.dict[i].Term >= term
	})
	if idx >= len(r.dict) || r.dict[idx].Term != term {
		return nil, nil
	}
	entry := r.dict[idx]
	postingsBytes := make([]byte, entry.PostLen)
	if _, err := r.file.ReadAt(postingsBytes, r.postBase+entry.PostOffset); err != nil {
		return nil, fmt.Errorf("reading postings: %w", err)
	}
	var postings index.PostingList
	if err := json.Unmarshal(postingsBytes, &postings); err != nil {
		return nil, fmt.Errorf("parsing postings: %w", err)
	}
	return postings, nil
}

// Terms returns the number of unique terms stored in this segment.
func (r *Reader) Terms() int {
	return len(r.dict)
}

// DocCount returns the number of unique documents stored in this segment.
func (r *Reader) DocCount() uint32 {
	return r.header.DocCount
}

// Close releases the underlying file handle.
func (r *Reader) Close() error {
	return r.file.Close()
}

// Name returns the base file name (e.g. "seg_123456.spdx") of this segment.
func (r *Reader) Name() string {
	return filepath.Base(r.filePath)
}
