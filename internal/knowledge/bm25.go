package knowledge

import (
	"math"
	"strings"
	"sync"
	"unicode"
)

// BM25Params holds tunable BM25 Okapi parameters.
type BM25Params struct {
	// K1 is the term-frequency saturation parameter (typical range 1.2–2.0).
	K1 float64
	// B is the length-normalisation parameter (0 = no normalisation, 1 = full).
	B float64
	// Delta is the lower-bound term-frequency offset used in BM25+.
	// Set to 0 for standard BM25 Okapi behaviour.
	Delta float64
}

// DefaultBM25Params returns sensible defaults matching the BM25 Okapi spec.
func DefaultBM25Params() BM25Params {
	return BM25Params{K1: 1.5, B: 0.75, Delta: 0}
}

// BM25Index is a corpus-level BM25 index.
//
// Usage:
//
//	idx := NewBM25Index(DefaultBM25Params())
//	idx.Add("doc1", "sql injection union select")
//	idx.Add("doc2", "cross site scripting xss payload")
//	idx.Build()
//	scores := idx.ScoreAll("sql injection")
type BM25Index struct {
	params BM25Params

	mu sync.RWMutex

	// Corpus statistics (built by Build()).
	docCount    int                // total number of documents
	avgDocLen   float64            // average document length in tokens
	df          map[string]int     // document frequency per term
	docLens     map[string]int     // document length (token count) per doc ID
	tf          map[string]map[string]int // tf[docID][term] = count

	built bool
}

// NewBM25Index creates an empty BM25Index with the given parameters.
func NewBM25Index(params BM25Params) *BM25Index {
	return &BM25Index{
		params: params,
		df:     make(map[string]int),
		docLens: make(map[string]int),
		tf:     make(map[string]map[string]int),
	}
}

// Add registers a document in the index. Call Build() after all documents have
// been added and before calling Score or ScoreAll.
// Adding a document with the same ID replaces the previous version.
func (idx *BM25Index) Add(docID, text string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	terms := tokenise(text)

	// If the document already exists, undo its contribution to df.
	if oldTF, exists := idx.tf[docID]; exists {
		for term := range oldTF {
			idx.df[term]--
			if idx.df[term] <= 0 {
				delete(idx.df, term)
			}
		}
	}

	// Build term-frequency map for this document.
	tfMap := make(map[string]int, len(terms))
	for _, t := range terms {
		tfMap[t]++
	}
	idx.tf[docID] = tfMap
	idx.docLens[docID] = len(terms)

	// Update document frequencies.
	for term := range tfMap {
		idx.df[term]++
	}

	idx.built = false
}

// Build computes corpus-level statistics (document count, average document
// length). Must be called after all documents are added and before scoring.
func (idx *BM25Index) Build() {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.docCount = len(idx.tf)
	if idx.docCount == 0 {
		idx.avgDocLen = 0
		idx.built = true
		return
	}
	totalLen := 0
	for _, l := range idx.docLens {
		totalLen += l
	}
	idx.avgDocLen = float64(totalLen) / float64(idx.docCount)
	idx.built = true
}

// Score returns the BM25 Okapi score for a single document given a query.
// Returns 0 if the document is not in the index or the index has not been built.
func (idx *BM25Index) Score(docID, query string) float64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if !idx.built || idx.docCount == 0 {
		return 0
	}

	docTF, ok := idx.tf[docID]
	if !ok {
		return 0
	}

	docLen := float64(idx.docLens[docID])
	return idx.scoreInternal(tokenise(query), docTF, docLen)
}

// ScoreAll scores every document in the index for the given query and returns a
// map of docID → score. Documents with score 0 are included.
func (idx *BM25Index) ScoreAll(query string) map[string]float64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	scores := make(map[string]float64, len(idx.tf))
	if !idx.built || idx.docCount == 0 {
		return scores
	}

	queryTerms := tokenise(query)
	for docID, docTF := range idx.tf {
		docLen := float64(idx.docLens[docID])
		scores[docID] = idx.scoreInternal(queryTerms, docTF, docLen)
	}
	return scores
}

// ScoreText computes a BM25-style score for an arbitrary text snippet against
// the given query without requiring the text to be part of the index.
// This is equivalent to the single-document version and uses corpus statistics
// (avgDocLen, docCount, df) from the built index for IDF calculation.
// If the index is empty or not built, it falls back to a simple TF/length ratio.
func (idx *BM25Index) ScoreText(query, text string) float64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	queryTerms := tokenise(query)
	textTerms := tokenise(text)
	if len(queryTerms) == 0 || len(textTerms) == 0 {
		return 0
	}

	// Build local tf map for the text.
	tfMap := make(map[string]int, len(textTerms))
	for _, t := range textTerms {
		tfMap[t]++
	}

	if !idx.built || idx.docCount == 0 {
		// Fallback: simple TF normalised by text length.
		return idx.fallbackScore(queryTerms, tfMap, float64(len(textTerms)))
	}

	return idx.scoreInternal(queryTerms, tfMap, float64(len(textTerms)))
}

// scoreInternal is the BM25 Okapi (+ optional BM25+ delta) kernel.
func (idx *BM25Index) scoreInternal(queryTerms []string, docTF map[string]int, docLen float64) float64 {
	k1 := idx.params.K1
	b := idx.params.B
	delta := idx.params.Delta
	N := float64(idx.docCount)
	avgLen := idx.avgDocLen
	if avgLen == 0 {
		avgLen = 1
	}

	score := 0.0
	for _, term := range queryTerms {
		tf := float64(docTF[term])
		if tf == 0 {
			continue
		}

		// Robertson–Sparck Jones IDF (Robertson et al. 1994):
		// IDF(t) = log((N - n(t) + 0.5) / (n(t) + 0.5) + 1)
		n := float64(idx.df[term])
		idf := math.Log((N-n+0.5)/(n+0.5) + 1)

		// BM25 TF part with length normalisation.
		lenNorm := 1 - b + b*(docLen/avgLen)
		tfScore := (tf * (k1 + 1)) / (tf + k1*lenNorm)

		// BM25+ adds a delta floor so every matching document gets a boost.
		score += idf * (tfScore + delta)
	}
	return score
}

// fallbackScore is a simple TF-based scorer used when the index has no corpus stats.
func (idx *BM25Index) fallbackScore(queryTerms []string, docTF map[string]int, docLen float64) float64 {
	if docLen == 0 {
		return 0
	}
	score := 0.0
	for _, term := range queryTerms {
		if tf, ok := docTF[term]; ok {
			score += float64(tf) / docLen
		}
	}
	return score / float64(len(queryTerms))
}

// bm25StopWords is a set of common English stop words that carry little
// discriminating power in BM25 scoring.  Security-domain terms are NOT
// included here because short technical abbreviations like "rce", "xss",
// "sqli", "lfi", "ssrf", etc. are intentionally meaningful.
var bm25StopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {},
	"be": {}, "been": {}, "being": {}, "by": {},
	"do": {}, "does": {}, "did": {},
	"for": {}, "from": {},
	"has": {}, "have": {}, "he": {}, "her": {}, "him": {}, "his": {},
	"how": {},
	"i": {}, "if": {}, "in": {}, "into": {}, "is": {}, "it": {}, "its": {},
	"me": {},
	"no": {}, "not": {},
	"of": {}, "on": {}, "or": {},
	"our": {},
	"s": {}, "she": {}, "so": {},
	"that": {}, "the": {}, "their": {}, "them": {}, "then": {}, "there": {},
	"these": {}, "they": {}, "this": {}, "those": {}, "through": {}, "to": {},
	"too": {},
	"up": {}, "use": {}, "used": {}, "uses": {}, "using": {},
	"was": {}, "we": {}, "were": {}, "what": {}, "when": {}, "where": {},
	"which": {}, "while": {}, "who": {}, "will": {}, "with": {},
	"you": {}, "your": {},
}

// securityAliases maps common security abbreviations / misspellings to a
// canonical normalised form so that, for example, "sqli" and "sql injection"
// both produce the token "sqlinj" in the index and in queries.
var securityAliases = map[string]string{
	"sqli":          "sqlinj",
	"sql-injection": "sqlinj",
	"sqlinjection":  "sqlinj",
	"xss":           "xss",
	"cross-site":    "xss",
	"crosssite":     "xss",
	"lfi":           "lfi",
	"rfi":           "rfi",
	"rce":           "rce",
	"ssrf":          "ssrf",
	"ssti":          "ssti",
	"idor":          "idor",
	"csrf":          "csrf",
	"xxe":           "xxe",
	"oob":           "ooob",  // out-of-band
	"ooob":          "ooob",
	"open-redirect": "openredirect",
	"openredirect":  "openredirect",
	"path-traversal": "pathtraversal",
	"pathtraversal": "pathtraversal",
	"dir-traversal": "pathtraversal",
	"deserializ":    "deserial",
	"deserialization": "deserial",
	"cmd-injection": "cmdinj",
	"cmdinjection":  "cmdinj",
	"command-injection": "cmdinj",
}

// tokenise lower-cases, normalises punctuation, removes stop words, applies
// security-domain aliases, and splits text into tokens suitable for BM25
// indexing and scoring.
func tokenise(text string) []string {
	// Replace common punctuation with spaces to split on them.
	var buf strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			buf.WriteRune(r)
		} else {
			buf.WriteRune(' ')
		}
	}
	raw := strings.Fields(buf.String())

	tokens := make([]string, 0, len(raw))
	for _, tok := range raw {
		// Remove leading/trailing hyphens and underscores (artifact of splitting).
		tok = strings.Trim(tok, "-_")
		if len(tok) < 2 {
			// Drop single-character tokens; they are almost always noise.
			continue
		}

		// Apply stop-word filter.
		if _, isStop := bm25StopWords[tok]; isStop {
			continue
		}

		// Apply security alias normalisation.
		if alias, ok := securityAliases[tok]; ok {
			tok = alias
		}

		tokens = append(tokens, tok)
	}
	return tokens
}

// BM25CorpusIndexer is a convenience wrapper that keeps a live BM25Index in
// sync with the knowledge embeddings database.  It is rebuilt from all current
// chunk texts whenever Rebuild() is called.
type BM25CorpusIndexer struct {
	index  *BM25Index
	mu     sync.RWMutex
	params BM25Params
}

// NewBM25CorpusIndexer creates a BM25CorpusIndexer with default parameters.
func NewBM25CorpusIndexer() *BM25CorpusIndexer {
	return &BM25CorpusIndexer{
		params: DefaultBM25Params(),
		index:  NewBM25Index(DefaultBM25Params()),
	}
}

// Rebuild recreates the BM25 index from the provided chunk corpus.
// chunks maps chunkID → chunk text.
func (ci *BM25CorpusIndexer) Rebuild(chunks map[string]string) {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	idx := NewBM25Index(ci.params)
	for id, text := range chunks {
		idx.Add(id, text)
	}
	idx.Build()
	ci.index = idx
}

// ScoreText delegates to the underlying index.
func (ci *BM25CorpusIndexer) ScoreText(query, text string) float64 {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.index.ScoreText(query, text)
}

// Score scores a specific document by ID.
func (ci *BM25CorpusIndexer) Score(docID, query string) float64 {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.index.Score(docID, query)
}

// Index returns the underlying BM25Index (read-only after Build).
func (ci *BM25CorpusIndexer) Index() *BM25Index {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.index
}
