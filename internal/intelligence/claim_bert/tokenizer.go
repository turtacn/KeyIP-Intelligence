package claim_bert

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Tokenizer interface
// ---------------------------------------------------------------------------

// Tokenizer defines the tokenization contract for ClaimBERT.
type Tokenizer interface {
	Tokenize(text string) (*TokenizedOutput, error)
	Encode(text string) (*EncodedInput, error)
	EncodePair(textA, textB string) (*EncodedInput, error)
	Decode(ids []int) (string, error)
	BatchEncode(texts []string) ([]*EncodedInput, error)
	VocabSize() int
}

// ---------------------------------------------------------------------------
// Output types
// ---------------------------------------------------------------------------

// TokenizedOutput holds the raw tokenization result before ID conversion.
type TokenizedOutput struct {
	Tokens            []string `json:"tokens"`
	Offsets           [][2]int `json:"offsets"`
	SpecialTokensMask []int    `json:"special_tokens_mask"`
}

// EncodedInput holds the model-ready encoded representation.
type EncodedInput struct {
	InputIDs       []int    `json:"input_ids"`
	AttentionMask  []int    `json:"attention_mask"`
	TokenTypeIDs   []int    `json:"token_type_ids"`
	Offsets        [][2]int `json:"offsets"`
	OverflowTokens []int    `json:"overflow_tokens"`
	NumTruncated   int      `json:"num_truncated"`
}

// ---------------------------------------------------------------------------
// Special token defaults
// ---------------------------------------------------------------------------

const (
	defaultUnknownToken = "[UNK]"
	defaultCLSToken     = "[CLS]"
	defaultSEPToken     = "[SEP]"
	defaultPADToken     = "[PAD]"
	defaultMASKToken    = "[MASK]"
	defaultMaxSeqLen    = 512
)

// ---------------------------------------------------------------------------
// WordPieceTokenizer
// ---------------------------------------------------------------------------

// WordPieceTokenizer implements Tokenizer using the WordPiece algorithm with
// chemical-patent-domain extensions.
type WordPieceTokenizer struct {
	vocab              map[string]int
	inverseVocab       map[int]string
	maxSequenceLength  int
	unknownToken       string
	clsToken           string
	sepToken           string
	padToken           string
	maskToken          string
	doLowerCase        bool
	stripAccents       bool
	chemicalPatterns   []*regexp.Regexp
	maxWordLen         int
}

// TokenizerOption is a functional option for WordPieceTokenizer construction.
type TokenizerOption func(*WordPieceTokenizer)

// WithMaxSequenceLength overrides the default max sequence length.
func WithMaxSequenceLength(n int) TokenizerOption {
	return func(t *WordPieceTokenizer) {
		if n > 0 {
			t.maxSequenceLength = n
		}
	}
}

// WithDoLowerCase enables or disables lower-casing.
func WithDoLowerCase(v bool) TokenizerOption {
	return func(t *WordPieceTokenizer) { t.doLowerCase = v }
}

// WithStripAccents enables or disables accent stripping.
func WithStripAccents(v bool) TokenizerOption {
	return func(t *WordPieceTokenizer) { t.stripAccents = v }
}

// WithSpecialTokens overrides the default special tokens.
func WithSpecialTokens(cls, sep, unk, pad, mask string) TokenizerOption {
	return func(t *WordPieceTokenizer) {
		if cls != "" {
			t.clsToken = cls
		}
		if sep != "" {
			t.sepToken = sep
		}
		if unk != "" {
			t.unknownToken = unk
		}
		if pad != "" {
			t.padToken = pad
		}
		if mask != "" {
			t.maskToken = mask
		}
	}
}

// NewWordPieceTokenizer creates a tokenizer from a vocabulary map.
// Use NewWordPieceTokenizerFromFile to load from disk.
func NewWordPieceTokenizer(vocab map[string]int, opts ...TokenizerOption) (*WordPieceTokenizer, error) {
	if len(vocab) == 0 {
		return nil, errors.NewInvalidInputError("vocab must not be empty")
	}

	inv := make(map[int]string, len(vocab))
	for tok, id := range vocab {
		inv[id] = tok
	}

	t := &WordPieceTokenizer{
		vocab:             vocab,
		inverseVocab:      inv,
		maxSequenceLength: defaultMaxSeqLen,
		unknownToken:      defaultUnknownToken,
		clsToken:          defaultCLSToken,
		sepToken:          defaultSEPToken,
		padToken:          defaultPADToken,
		maskToken:         defaultMASKToken,
		doLowerCase:       false, // chemical names are case-sensitive
		stripAccents:      false,
		maxWordLen:        200,
	}
	for _, o := range opts {
		o(t)
	}

	t.chemicalPatterns = compileChemicalPatterns()

	// Ensure special tokens exist in vocab
	for _, st := range []string{t.unknownToken, t.clsToken, t.sepToken, t.padToken, t.maskToken} {
		if _, ok := t.vocab[st]; !ok {
			return nil, fmt.Errorf("special token %q not found in vocab", st)
		}
	}

	return t, nil
}

// NewWordPieceTokenizerFromFile loads a vocabulary file (one token per line)
// and returns a ready tokenizer.
func NewWordPieceTokenizerFromFile(path string, opts ...TokenizerOption) (*WordPieceTokenizer, error) {
	vocab, err := LoadVocabFromFile(path)
	if err != nil {
		return nil, err
	}
	return NewWordPieceTokenizer(vocab, opts...)
}

// LoadVocabFromFile reads a vocabulary file where each line is a token.
// The line number (0-based) becomes the token ID.
// Duplicate tokens are allowed; the last occurrence wins (with a warning).
func LoadVocabFromFile(path string) (map[string]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open vocab file: %w", err)
	}
	defer f.Close()

	vocab := make(map[string]int)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	id := 0
	for scanner.Scan() {
		token := strings.TrimSpace(scanner.Text())
		if token == "" {
			id++
			continue
		}
		vocab[token] = id
		id++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading vocab file: %w", err)
	}
	if len(vocab) == 0 {
		return nil, errors.NewInvalidInputError("vocab file is empty")
	}
	return vocab, nil
}

// LoadVocab loads a vocabulary from a file path into the tokenizer,
// replacing the current vocabulary.
func (t *WordPieceTokenizer) LoadVocab(path string) error {
	vocab, err := LoadVocabFromFile(path)
	if err != nil {
		return err
	}
	inv := make(map[int]string, len(vocab))
	for tok, id := range vocab {
		inv[id] = tok
	}
	t.vocab = vocab
	t.inverseVocab = inv
	return nil
}

// VocabSize returns the number of tokens in the vocabulary.
func (t *WordPieceTokenizer) VocabSize() int {
	return len(t.vocab)
}

// ---------------------------------------------------------------------------
// Tokenize
// ---------------------------------------------------------------------------

// Tokenize performs the full tokenization pipeline:
//  1. Unicode NFC normalization & control character removal
//  2. Chemical entity pre-tokenization (regex-based)
//  3. Whitespace / punctuation splitting
//  4. WordPiece sub-word tokenization
//  5. Offset tracking
func (t *WordPieceTokenizer) Tokenize(text string) (*TokenizedOutput, error) {
	if text == "" {
		return &TokenizedOutput{
			Tokens:            []string{},
			Offsets:           [][2]int{},
			SpecialTokensMask: []int{},
		}, nil
	}

	// Step 1: clean
	cleaned := t.cleanText(text)

	// Step 2 + 3: pre-tokenize into word spans
	spans := t.pretokenize(cleaned)

	// Step 4: WordPiece each span
	var tokens []string
	var offsets [][2]int
	for _, sp := range spans {
		word := cleaned[sp[0]:sp[1]]
		subTokens := t.wordPiece(word)
		// distribute offsets across sub-tokens
		charIdx := sp[0]
		for i, st := range subTokens {
			tokLen := len(st)
			if i > 0 && strings.HasPrefix(st, "##") {
				tokLen -= 2 // ## prefix is virtual
			}
			end := charIdx + tokLen
			if end > sp[1] {
				end = sp[1]
			}
			offsets = append(offsets, [2]int{charIdx, end})
			tokens = append(tokens, st)
			charIdx = end
		}
	}

	mask := make([]int, len(tokens))
	return &TokenizedOutput{
		Tokens:            tokens,
		Offsets:           offsets,
		SpecialTokensMask: mask,
	}, nil
}

// ---------------------------------------------------------------------------
// Encode
// ---------------------------------------------------------------------------

// Encode tokenizes and converts to model-ready input IDs with special tokens,
// truncation, and padding.
func (t *WordPieceTokenizer) Encode(text string) (*EncodedInput, error) {
	tok, err := t.Tokenize(text)
	if err != nil {
		return nil, err
	}

	// Reserve 2 slots for [CLS] and [SEP]
	maxContentLen := t.maxSequenceLength - 2
	if maxContentLen < 0 {
		maxContentLen = 0
	}

	truncated := 0
	var overflowIDs []int
	contentTokens := tok.Tokens
	contentOffsets := tok.Offsets

	if len(contentTokens) > maxContentLen {
		truncated = len(contentTokens) - maxContentLen
		overflowTokens := contentTokens[maxContentLen:]
		overflowIDs = t.tokensToIDs(overflowTokens)
		contentTokens = contentTokens[:maxContentLen]
		contentOffsets = contentOffsets[:maxContentLen]
	}

	// Build sequence: [CLS] + content + [SEP]
	seqTokens := make([]string, 0, len(contentTokens)+2)
	seqTokens = append(seqTokens, t.clsToken)
	seqTokens = append(seqTokens, contentTokens...)
	seqTokens = append(seqTokens, t.sepToken)

	ids := t.tokensToIDs(seqTokens)

	// Offsets: special tokens get (-1,-1)
	seqOffsets := make([][2]int, 0, len(ids))
	seqOffsets = append(seqOffsets, [2]int{-1, -1}) // [CLS]
	seqOffsets = append(seqOffsets, contentOffsets...)
	seqOffsets = append(seqOffsets, [2]int{-1, -1}) // [SEP]

	seqLen := len(ids)
	attnMask := ones(seqLen)
	typeIDs := zeros(seqLen)

	// Pad
	if seqLen < t.maxSequenceLength {
		padLen := t.maxSequenceLength - seqLen
		padID := t.vocab[t.padToken]
		ids = appendN(ids, padID, padLen)
		attnMask = appendN(attnMask, 0, padLen)
		typeIDs = appendN(typeIDs, 0, padLen)
		for i := 0; i < padLen; i++ {
			seqOffsets = append(seqOffsets, [2]int{-1, -1})
		}
	}

	return &EncodedInput{
		InputIDs:       ids,
		AttentionMask:  attnMask,
		TokenTypeIDs:   typeIDs,
		Offsets:        seqOffsets,
		OverflowTokens: overflowIDs,
		NumTruncated:   truncated,
	}, nil
}

// ---------------------------------------------------------------------------
// EncodePair
// ---------------------------------------------------------------------------

// EncodePair encodes a sentence pair as [CLS] A [SEP] B [SEP] with
// appropriate token type IDs and truncation (longest-first).
func (t *WordPieceTokenizer) EncodePair(textA, textB string) (*EncodedInput, error) {
	tokA, err := t.Tokenize(textA)
	if err != nil {
		return nil, fmt.Errorf("tokenizing textA: %w", err)
	}
	tokB, err := t.Tokenize(textB)
	if err != nil {
		return nil, fmt.Errorf("tokenizing textB: %w", err)
	}

	// Reserve 3 slots: [CLS], [SEP] after A, [SEP] after B
	maxContentLen := t.maxSequenceLength - 3
	if maxContentLen < 0 {
		maxContentLen = 0
	}

	tokensA := tokA.Tokens
	tokensB := tokB.Tokens
	offsetsA := tokA.Offsets
	offsetsB := tokB.Offsets

	totalTruncated := 0

	// Longest-first truncation
	for len(tokensA)+len(tokensB) > maxContentLen {
		if len(tokensA) >= len(tokensB) {
			tokensA = tokensA[:len(tokensA)-1]
			offsetsA = offsetsA[:len(offsetsA)-1]
		} else {
			tokensB = tokensB[:len(tokensB)-1]
			offsetsB = offsetsB[:len(offsetsB)-1]
		}
		totalTruncated++
	}

	// Build: [CLS] A [SEP] B [SEP]
	seqTokens := make([]string, 0, len(tokensA)+len(tokensB)+3)
	seqTokens = append(seqTokens, t.clsToken)
	seqTokens = append(seqTokens, tokensA...)
	seqTokens = append(seqTokens, t.sepToken)
	seqTokens = append(seqTokens, tokensB...)
	seqTokens = append(seqTokens, t.sepToken)

	ids := t.tokensToIDs(seqTokens)

	// Offsets
	seqOffsets := make([][2]int, 0, len(ids))
	seqOffsets = append(seqOffsets, [2]int{-1, -1}) // [CLS]
	seqOffsets = append(seqOffsets, offsetsA...)
	seqOffsets = append(seqOffsets, [2]int{-1, -1}) // [SEP]
	seqOffsets = append(seqOffsets, offsetsB...)
	seqOffsets = append(seqOffsets, [2]int{-1, -1}) // [SEP]

	// Token type IDs: segment A = 0, segment B = 1
	// [CLS]=0, A tokens=0, [SEP]=0, B tokens=1, [SEP]=1
	typeIDs := make([]int, len(ids))
	segBStart := 1 + len(tokensA) + 1 // after [CLS] + A + [SEP]
	for i := segBStart; i < len(ids); i++ {
		typeIDs[i] = 1
	}

	seqLen := len(ids)
	attnMask := ones(seqLen)

	// Pad
	if seqLen < t.maxSequenceLength {
		padLen := t.maxSequenceLength - seqLen
		padID := t.vocab[t.padToken]
		ids = appendN(ids, padID, padLen)
		attnMask = appendN(attnMask, 0, padLen)
		typeIDs = appendN(typeIDs, 0, padLen)
		for i := 0; i < padLen; i++ {
			seqOffsets = append(seqOffsets, [2]int{-1, -1})
		}
	}

	return &EncodedInput{
		InputIDs:       ids,
		AttentionMask:  attnMask,
		TokenTypeIDs:   typeIDs,
		Offsets:        seqOffsets,
		OverflowTokens: nil,
		NumTruncated:   totalTruncated,
	}, nil
}

// ---------------------------------------------------------------------------
// Decode
// ---------------------------------------------------------------------------

// Decode converts an ID sequence back to text, merging WordPiece sub-tokens
// and stripping special tokens and padding.
func (t *WordPieceTokenizer) Decode(ids []int) (string, error) {
	if len(ids) == 0 {
		return "", nil
	}

	specialSet := map[string]bool{
		t.clsToken: true,
		t.sepToken: true,
		t.padToken: true,
	}

	var parts []string
	for _, id := range ids {
		tok, ok := t.inverseVocab[id]
		if !ok {
			tok = t.unknownToken
		}
		if specialSet[tok] {
			continue
		}
		parts = append(parts, tok)
	}

	// Merge sub-word tokens: "##xyz" is appended to the previous word
	if len(parts) == 0 {
		return "", nil
	}

	var builder strings.Builder
	for i, p := range parts {
		if strings.HasPrefix(p, "##") {
			builder.WriteString(p[2:])
		} else {
			if i > 0 {
				builder.WriteByte(' ')
			}
			builder.WriteString(p)
		}
	}
	return builder.String(), nil
}

// ---------------------------------------------------------------------------
// BatchEncode
// ---------------------------------------------------------------------------

// BatchEncode encodes multiple texts, padding all to the same length
// (the max of the batch or maxSequenceLength, whichever is smaller).
func (t *WordPieceTokenizer) BatchEncode(texts []string) ([]*EncodedInput, error) {
	if len(texts) == 0 {
		return []*EncodedInput{}, nil
	}

	results := make([]*EncodedInput, len(texts))
	for i, text := range texts {
		enc, err := t.Encode(text)
		if err != nil {
			return nil, fmt.Errorf("encoding text[%d]: %w", i, err)
		}
		results[i] = enc
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Text cleaning
// ---------------------------------------------------------------------------

func (t *WordPieceTokenizer) cleanText(text string) string {
	// NFC normalization
	text = norm.NFC.String(text)

	// Remove control characters (keep whitespace)
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if r == 0 || r == 0xFFFD {
			continue
		}
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(r)
	}
	text = b.String()

	if t.doLowerCase {
		text = strings.ToLower(text)
	}

	if t.stripAccents {
		text = stripAccentsFromText(text)
	}

	return text
}

func stripAccentsFromText(text string) string {
	// NFD decompose, then remove combining marks
	decomposed := norm.NFD.String(text)
	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if !unicode.Is(unicode.Mn, r) { // Mn = Mark, Nonspacing
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Pre-tokenization: chemical entity detection + whitespace/punctuation split
// ---------------------------------------------------------------------------

// pretokenizeSpan represents a byte-offset span in the cleaned text.
type pretokenizeSpan = [2]int

func (t *WordPieceTokenizer) pretokenize(text string) []pretokenizeSpan {
	// First, find chemical entity spans that should be kept as whole tokens
	chemSpans := t.findChemicalSpans(text)

	// Build a set of byte positions covered by chemical entities
	covered := make(map[int]bool)
	for _, sp := range chemSpans {
		for i := sp[0]; i < sp[1]; i++ {
			covered[i] = true
		}
	}

	// Split the remaining text by whitespace and punctuation
	var spans []pretokenizeSpan

	// Merge chemical spans and regular word spans in order
	chemIdx := 0
	pos := 0

	for pos < len(text) {
		// Check if a chemical span starts here
		if chemIdx < len(chemSpans) && pos == chemSpans[chemIdx][0] {
			spans = append(spans, chemSpans[chemIdx])
			pos = chemSpans[chemIdx][1]
			chemIdx++
			continue
		}

		// Skip if inside a chemical span (shouldn't happen with ordered processing)
		if covered[pos] {
			pos++
			continue
		}

		r, size := utf8.DecodeRuneInString(text[pos:])

		// Skip whitespace
		if unicode.IsSpace(r) {
			pos += size
			continue
		}

		// Punctuation is its own token
		if isPunctuation(r) {
			spans = append(spans, pretokenizeSpan{pos, pos + size})
			pos += size
			continue
		}

		// Accumulate a word
		start := pos
		for pos < len(text) {
			if covered[pos] {
				break
			}
			r, size = utf8.DecodeRuneInString(text[pos:])
			if unicode.IsSpace(r) || isPunctuation(r) {
				break
			}
			pos += size
		}
		if pos > start {
			spans = append(spans, pretokenizeSpan{start, pos})
		}
	}

	return spans
}

func (t *WordPieceTokenizer) findChemicalSpans(text string) []pretokenizeSpan {
	type match struct {
		start, end int
	}
	var allMatches []match

	for _, pat := range t.chemicalPatterns {
		locs := pat.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			allMatches = append(allMatches, match{loc[0], loc[1]})
		}
	}

	if len(allMatches) == 0 {
		return nil
	}

	// Sort by start, then by length descending (longer match wins)
	sortMatches(allMatches)

	// Greedy non-overlapping selection
	var selected []pretokenizeSpan
	lastEnd := 0
	for _, m := range allMatches {
		if m.start >= lastEnd {
			selected = append(selected, pretokenizeSpan{m.start, m.end})
			lastEnd = m.end
		}
	}
	return selected
}

func sortMatches(matches []match) {
	// Simple insertion sort (chemical matches are typically few)
	type match = struct{ start, end int }
	for i := 1; i < len(matches); i++ {
		key := matches[i]
		j := i - 1
		for j >= 0 && (matches[j].start > key.start ||
			(matches[j].start == key.start && (matches[j].end-matches[j].start) < (key.end-key.start))) {
			matches[j+1] = matches[j]
			j--
		}
		matches[j+1] = key
	}
}

func isPunctuation(r rune) bool {
	if (r >= 33 && r <= 47) || (r >= 58 && r <= 64) ||
		(r >= 91 && r <= 96) || (r >= 123 && r <= 126) {
		return true
	}
	return unicode.IsPunct(r)
}

// ---------------------------------------------------------------------------
// Chemical entity patterns
// ---------------------------------------------------------------------------

func compileChemicalPatterns() []*regexp.Regexp {
	patterns := []string{
		// CAS numbers: e.g. 50-78-2, 7732-18-5
		`\b\d{2,7}-\d{2}-\d\b`,

		// Molecular formulas: e.g. C6H12O6, H2SO4, Ca(OH)2
		`\b[A-Z][a-z]?\d*(?:\([A-Z][a-z]?\d*\)\d*)*(?:[A-Z][a-z]?\d*)*\b`,

		// Markush keywords: C1-C6 alkyl, aryl, heteroaryl, etc.
		`(?:C\d+-C\d+\s*)?(?:alkyl|aryl|heteroaryl|cycloalkyl|heterocycl\w*|halogen|halo)\b`,

		// IUPAC-style chemical names with numbers and hyphens
		// e.g. 2-methylpropan-1-ol, 4-(2-hydroxyethyl)piperazine
		`\b\d*[,-]?\s*(?:methyl|ethyl|propyl|butyl|pentyl|hexyl|phenyl|benzyl|amino|hydroxy|oxo|chloro|bromo|fluoro|nitro|cyano|carboxy|sulfo)\w*(?:-\w+)*\b`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			continue // skip invalid patterns silently
		}
		compiled = append(compiled, re)
	}
	return compiled
}

// ---------------------------------------------------------------------------
// WordPiece algorithm
// ---------------------------------------------------------------------------

func (t *WordPieceTokenizer) wordPiece(word string) []string {
	if len(word) == 0 {
		return nil
	}
	if len(word) > t.maxWordLen {
		return []string{t.unknownToken}
	}

	var tokens []string
	start := 0
	wordRunes := []rune(word)
	wordLen := len(wordRunes)

	for start < wordLen {
		end := wordLen
		if end-start > t.maxWordLen {
			end = start + t.maxWordLen
		}
		found := false

		for end > start {
			substr := string(wordRunes[start:end])
			if start > 0 {
				substr = "##" + substr
			}
			if _, ok := t.vocab[substr]; ok {
				tokens = append(tokens, substr)
				found = true
				break
			}
			end--
		}

		if !found {
			tokens = append(tokens, t.unknownToken)
			break
		}
		start = end
	}

	return tokens
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (t *WordPieceTokenizer) tokensToIDs(tokens []string) []int {
	unkID := t.vocab[t.unknownToken]
	ids := make([]int, len(tokens))
	for i, tok := range tokens {
		id, ok := t.vocab[tok]
		if !ok {
			id = unkID
		}
		ids[i] = id
	}
	return ids
}

func ones(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = 1
	}
	return s
}

func zeros(n int) []int {
	return make([]int, n)
}

func appendN(s []int, val, n int) []int {
	for i := 0; i < n; i++ {
		s = append(s, val)
	}
	return s
}

// Ensure interface compliance at compile time.
var _ Tokenizer = (*WordPieceTokenizer)(nil)

// Suppress unused import for math in edge cases.
var _ = math.MaxInt

//Personal.AI order the ending
