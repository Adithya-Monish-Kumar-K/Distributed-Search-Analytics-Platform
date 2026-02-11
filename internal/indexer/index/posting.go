package index

type Posting struct {
	DocID     string
	Frequency int
	Positions []int
}

type PostingList []Posting

type TermEntry struct {
	Term     string
	Postings PostingList
}

type DocStats struct {
	DocID    string
	DocLen   int
	TermFreq int
}
