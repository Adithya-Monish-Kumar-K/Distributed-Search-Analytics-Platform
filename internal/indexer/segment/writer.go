package segment

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/index"
)

// MagicBytes identifies a valid .spdx segment file.
const (
	MagicBytes    uint32 = 0x53504458
	FormatVersion uint32 = 1
	HeaderSize    int    = 64
	FooterSize    int    = 32
)

// SegmentHeader is the 64-byte header written at the start of every segment.
type SegmentHeader struct {
	Magic      uint32
	Version    uint32
	TermCount  uint32
	DocCount   uint32
	CreatedAt  int64
	DictOffset int64
	DictSize   int64
	PostOffset int64
	PostSize   int64
}

// DictEntry maps a term to its postings offset, length, and document frequency
// in the segment file.
type DictEntry struct {
	Term       string `json:"t"`
	PostOffset int64  `json:"o"`
	PostLen    int    `json:"l"`
	DocFreq    int    `json:"d"`
}

// Writer serialises TermEntry slices into new .spdx segment files.
type Writer struct {
	dataDir string
}

// NewWriter creates a Writer that writes segments into the given directory.
func NewWriter(dataDir string) *Writer {
	return &Writer{dataDir: dataDir}
}

// Write atomically creates a new segment file containing the given term
// entries. It writes to a .tmp file first and renames on success.
func (w *Writer) Write(entries []index.TermEntry) (string, error) {
	if len(entries) == 0 {
		return "", fmt.Errorf("cannot write empty segment")
	}
	segmentName := fmt.Sprintf("seg_%d.spdx", time.Now().UnixNano())
	finalPath := filepath.Join(w.dataDir, segmentName)
	tmpPath := finalPath + ".tmp"

	if err := os.MkdirAll(w.dataDir, 0755); err != nil {
		return "", fmt.Errorf("creating segment directory: %w", err)
	}
	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("creating temp segment file: %w", err)
	}
	defer f.Close()
	header := SegmentHeader{
		Magic:     MagicBytes,
		Version:   FormatVersion,
		TermCount: uint32(len(entries)),
		CreatedAt: time.Now().Unix(),
	}
	headerBytes := make([]byte, HeaderSize)
	binary.LittleEndian.PutUint32(headerBytes[0:4], header.Magic)
	binary.LittleEndian.PutUint32(headerBytes[4:8], header.Version)
	binary.LittleEndian.PutUint32(headerBytes[8:12], header.TermCount)

	if _, err := f.Write(headerBytes); err != nil {
		return "", fmt.Errorf("writing header: %w", err)
	}

	postingsStart, _ := f.Seek(0, 1)
	dict := make([]DictEntry, 0, len(entries))
	docIDs := make(map[string]struct{})
	for _, entry := range entries {
		offset, _ := f.Seek(0, 1)
		relativeOffset := offset - postingsStart
		postingsData, err := json.Marshal(entry.Postings)
		if err != nil {
			return "", fmt.Errorf("marshaling postings for term %q: %w", entry.Term, err)
		}
		if _, err := f.Write(postingsData); err != nil {
			return "", fmt.Errorf("writing postings for term %q: %w", entry.Term, err)
		}
		dict = append(dict, DictEntry{
			Term:       entry.Term,
			PostOffset: relativeOffset,
			PostLen:    len(postingsData),
			DocFreq:    len(entry.Postings),
		})
		for _, p := range entry.Postings {
			docIDs[p.DocID] = struct{}{}
		}
	}

	postingsEnd, _ := f.Seek(0, 1)
	postingsSize := postingsEnd - postingsStart
	dictStart := postingsEnd
	dictData, err := json.Marshal(dict)

	if err != nil {
		return "", fmt.Errorf("marshaling dictionary: %w", err)
	}
	if _, err := f.Write(dictData); err != nil {
		return "", fmt.Errorf("writing dictionary: %w", err)
	}
	dictEnd, _ := f.Seek(0, 1)
	dictSize := dictEnd - dictStart
	checksum := crc32.ChecksumIEEE(dictData)
	footer := make([]byte, FooterSize)
	binary.LittleEndian.PutUint32(footer[0:4], checksum)
	binary.LittleEndian.PutUint32(footer[4:8], uint32(len(docIDs)))
	binary.LittleEndian.PutUint64(footer[8:16], uint64(dictStart))
	binary.LittleEndian.PutUint64(footer[16:24], uint64(dictSize))
	binary.LittleEndian.PutUint64(footer[24:32], uint64(postingsSize))
	if _, err := f.Write(footer); err != nil {
		return "", fmt.Errorf("writing footer: %w", err)
	}
	binary.LittleEndian.PutUint32(headerBytes[12:16], uint32(len(docIDs)))
	binary.LittleEndian.PutUint64(headerBytes[16:24], uint64(dictStart))
	binary.LittleEndian.PutUint64(headerBytes[24:32], uint64(dictSize))
	binary.LittleEndian.PutUint64(headerBytes[32:40], uint64(postingsStart))
	binary.LittleEndian.PutUint64(headerBytes[40:48], uint64(postingsSize))
	if _, err := f.WriteAt(headerBytes, 0); err != nil {
		return "", fmt.Errorf("updating header: %w", err)
	}
	if err := f.Sync(); err != nil {
		return "", fmt.Errorf("syncing segment file: %w", err)
	}
	f.Close()
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", fmt.Errorf("renaming segment file: %w", err)
	}
	return segmentName, nil
}
