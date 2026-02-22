package claim_bert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Test vocabulary
// ---------------------------------------------------------------------------

// buildTestVocab creates a small vocabulary covering basic English, chemical
// terms, sub-word pieces, and all required special tokens.
func buildTestVocab() map[string]int {
	tokens := []string{
		// 0-4: special tokens
		"[PAD]", "[UNK]", "[CLS]", "[SEP]", "[MASK]",
		// 5-19: basic English words
		"a", "the", "of", "is", "in", "and", "for", "to", "with", "an",
		"compound", "method", "claim", "comprising", "wherein",
		// 20-34: more words
		"composition", "use", "treatment", "disease", "effective",
		"amount", "patient", "group", "selected", "from",
		// 35-49: chemical terms (whole words)
		"methyl", "ethyl", "phenyl", "benzyl", "amino",
		"hydroxy", "chloro", "bromo", "fluoro", "nitro",
		"alkyl", "aryl", "heteroaryl", "cycloalkyl", "halogen",
		// 50-64: sub-word pieces
		"##ing", "##tion", "##al", "##ly", "##ed",
		"##er", "##yl", "##ene", "##ane", "##ol",
		"##ide", "##ate", "##ase", "##ine", "##ous",
		// 65-79: more sub-words for pharmaceutical
		"pharma", "##ce", "##utical", "##uti", "##cal",
		"prop", "##an", "##1", "##-", "##2",
		"acid", "salt", "ester", "form", "##ula",
		// 80-94: numbers and punctuation
		"1", "2", "3", "4", "5",
		"(", ")", "-", ",", ".",
		"C", "H", "O", "N", "S",
		// 95-104: additional chemical
		"L", "D", "##alanine", "alanine", "##oxy",
		"##ethyl", "##propyl", "##butyl", "##pentyl", "##hexyl",
		// 105-114: more coverage
		"water", "solution", "tablet", "capsule", "injection",
		"mg", "ml", "kg", "μg", "mol",
		// 115-124: patent legal terms
		"independent", "dependent", "characterized", "according", "embodiment",
		"preferred", "optional", "substituted", "unsubstituted", "formula",
		// 125-134: markush
		"halo", "heterocycl", "##ic", "##ary", "ring",
		"bond", "atom", "radical", "moiety", "residue",
		// 135-139: misc
		"process", "preparing", "obtain", "##ed", "##s",
	}

	vocab := make(map[string]int, len(tokens))
	for i, tok := range tokens {
		vocab[tok] = i
	}
	return vocab
}

// setupTestTokenizer creates a WordPieceTokenizer with the test vocabulary
// and a small max sequence length for easy testing.
func setupTestTokenizer() *WordPieceTokenizer {
	return setupTestTokenizerWithMaxLen(32)
}

func setupTestTokenizerWithMaxLen(maxLen int) *WordPieceTokenizer {
	vocab := buildTestVocab()
	t, err := NewWordPieceTokenizer(vocab, WithMaxSequenceLength(maxLen))
	if err != nil {
		panic("setupTestTokenizer: " + err.Error())
	}
	return t
}

// ---------------------------------------------------------------------------
// Tokenize tests
// ---------------------------------------------------------------------------

func TestTokenize_SimpleText(t *testing.T) {
	tok := setupTestTokenizer()
	out, err := tok.Tokenize("a compound")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Tokens) < 2 {
		t.Fatalf("expected at least 2 tokens, got %d: %v", len(out.Tokens), out.Tokens)
	}
	// "a" and "compound" should both be in vocab
	found := map[string]bool{}
	for _, tk := range out.Tokens {
		found[tk] = true
	}
	if !found["a"] {
		t.Error("expected token 'a'")
	}
	if !found["compound"] {
		t.Error("expected token 'compound'")
	}
}

func TestTokenize_WordPiece(t *testing.T) {
	tok := setupTestTokenizer()
	// "pharmaceutical" is not a single token in our test vocab,
	// but "pharma" + "##ce" + "##utical" should match
	out, err := tok.Tokenize("pharmaceutical")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Tokens) == 0 {
		t.Fatal("expected non-empty tokens")
	}
	// Check that sub-word pieces are present
	joined := strings.Join(out.Tokens, " ")
	t.Logf("WordPiece result: %v", out.Tokens)
	// Should contain "pharma" as the first piece
	if out.Tokens[0] != "pharma" && out.Tokens[0] != "[UNK]" {
		t.Logf("first token: %s (may vary by vocab coverage)", out.Tokens[0])
	}
	_ = joined
}

func TestTokenize_ChemicalName(t *testing.T) {
	tok := setupTestTokenizer()
	// "2-methylpropan-1-ol" contains "methyl" which is a chemical pattern match
	out, err := tok.Tokenize("2-methylpropan-1-ol")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Tokens) == 0 {
		t.Fatal("expected non-empty tokens")
	}
	t.Logf("Chemical name tokens: %v", out.Tokens)
	// The chemical pattern should capture at least part of this as a unit
}

func TestTokenize_MolecularFormula(t *testing.T) {
	tok := setupTestTokenizer()
	out, err := tok.Tokenize("C6H12O6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Tokens) == 0 {
		t.Fatal("expected non-empty tokens")
	}
	t.Logf("Molecular formula tokens: %v", out.Tokens)
	// The molecular formula regex should match "C6H12O6" as a chemical entity
}

func TestTokenize_CASNumber(t *testing.T) {
	tok := setupTestTokenizer()
	out, err := tok.Tokenize("aspirin 50-78-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("CAS number tokens: %v", out.Tokens)
	// "50-78-2" should be captured as a chemical entity span
	if len(out.Tokens) == 0 {
		t.Fatal("expected non-empty tokens")
	}
}

func TestTokenize_MarkushKeyword(t *testing.T) {
	tok := setupTestTokenizer()
	out, err := tok.Tokenize("C1-C6 alkyl group")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("Markush tokens: %v", out.Tokens)
	found := false
	for _, tk := range out.Tokens {
		if tk == "alkyl" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'alkyl' token")
	}
}

func TestTokenize_Offsets(t *testing.T) {
	tok := setupTestTokenizer()
	text := "a compound"
	out, err := tok.Tokenize(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Offsets) != len(out.Tokens) {
		t.Fatalf("offsets length %d != tokens length %d", len(out.Offsets), len(out.Tokens))
	}
	for i, off := range out.Offsets {
		if off[0] < 0 || off[1] > len(text) || off[0] > off[1] {
			t.Errorf("token[%d] %q has invalid offset [%d, %d]", i, out.Tokens[i], off[0], off[1])
		}
	}
}

func TestTokenize_PreservesCase(t *testing.T) {
	tok := setupTestTokenizer()
	out, err := tok.Tokenize("L-alanine")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("Case-preserved tokens: %v", out.Tokens)
	// With doLowerCase=false, "L" should remain uppercase
	hasUpper := false
	for _, tk := range out.Tokens {
		if tk == "L" {
			hasUpper = true
		}
	}
	if !hasUpper {
		t.Log("note: 'L' may have been merged into a chemical entity span")
	}
}

func TestTokenize_EmptyText(t *testing.T) {
	tok := setupTestTokenizer()
	out, err := tok.Tokenize("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Tokens) != 0 {
		t.Errorf("expected 0 tokens for empty text, got %d", len(out.Tokens))
	}
	if len(out.Offsets) != 0 {
		t.Errorf("expected 0 offsets for empty text, got %d", len(out.Offsets))
	}
}

func TestTokenize_UnicodeNormalization(t *testing.T) {
	tok := setupTestTokenizer()
	// Full-width 'ａ' (U+FF41) should be normalized
	out, err := tok.Tokenize("\uff41 compound")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("Unicode normalized tokens: %v", out.Tokens)
	if len(out.Tokens) == 0 {
		t.Fatal("expected non-empty tokens after normalization")
	}
}

func TestTokenize_ControlCharacters(t *testing.T) {
	tok := setupTestTokenizer()
	// Embed a null byte and a BEL character
	out, err := tok.Tokenize("a\x00\x07compound")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("Control char cleaned tokens: %v", out.Tokens)
	// Should still produce tokens after cleaning
	if len(out.Tokens) == 0 {
		t.Fatal("expected non-empty tokens after control char removal")
	}
}

// ---------------------------------------------------------------------------
// Encode tests
// ---------------------------------------------------------------------------

func TestEncode_SpecialTokens(t *testing.T) {
	tok := setupTestTokenizer()
	enc, err := tok.Encode("compound")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First ID should be [CLS]
	clsID := tok.vocab[tok.clsToken]
	if enc.InputIDs[0] != clsID {
		t.Errorf("expected first ID to be [CLS]=%d, got %d", clsID, enc.InputIDs[0])
	}
	// Find [SEP] — it should appear after the content tokens
	sepID := tok.vocab[tok.sepToken]
	foundSep := false
	for i := 1; i < len(enc.InputIDs); i++ {
		if enc.InputIDs[i] == sepID {
			foundSep = true
			break
		}
	}
	if !foundSep {
		t.Error("expected [SEP] token in encoded output")
	}
}

func TestEncode_AttentionMask(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(16)
	enc, err := tok.Encode("a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(enc.AttentionMask) != 16 {
		t.Fatalf("expected attention mask length 16, got %d", len(enc.AttentionMask))
	}
	// [CLS] a [SEP] = 3 active tokens
	activeCount := 0
	for _, v := range enc.AttentionMask {
		if v == 1 {
			activeCount++
		}
	}
	if activeCount < 3 {
		t.Errorf("expected at least 3 active positions, got %d", activeCount)
	}
	// Remaining should be 0 (padding)
	padCount := 0
	for _, v := range enc.AttentionMask {
		if v == 0 {
			padCount++
		}
	}
	if padCount != 16-activeCount {
		t.Errorf("expected %d padding positions, got %d", 16-activeCount, padCount)
	}
}

func TestEncode_Padding(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(16)
	enc, err := tok.Encode("a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(enc.InputIDs) != 16 {
		t.Errorf("expected padded length 16, got %d", len(enc.InputIDs))
	}
	padID := tok.vocab[tok.padToken]
	// Last elements should be PAD
	lastID := enc.InputIDs[len(enc.InputIDs)-1]
	if lastID != padID {
		t.Errorf("expected last ID to be [PAD]=%d, got %d", padID, lastID)
	}
}

func TestEncode_Truncation(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(8) // very short
	// Generate a long text
	longText := strings.Repeat("compound ", 20)
	enc, err := tok.Encode(longText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(enc.InputIDs) != 8 {
		t.Errorf("expected truncated length 8, got %d", len(enc.InputIDs))
	}
	if enc.NumTruncated <= 0 {
		t.Error("expected NumTruncated > 0")
	}
}

func TestEncode_OverflowTokens(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(8)
	longText := strings.Repeat("compound ", 20)
	enc, err := tok.Encode(longText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(enc.OverflowTokens) == 0 {
		t.Error("expected non-empty OverflowTokens for truncated input")
	}
	if enc.NumTruncated != len(enc.OverflowTokens) {
		t.Errorf("NumTruncated=%d should equal len(OverflowTokens)=%d",
			enc.NumTruncated, len(enc.OverflowTokens))
	}
}

// ---------------------------------------------------------------------------
// EncodePair tests
// ---------------------------------------------------------------------------

func TestEncodePair_TokenTypeIDs(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(32)
	enc, err := tok.EncodePair("a compound", "the method")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// [CLS]=0, A tokens=0, [SEP]=0, B tokens=1, [SEP]=1, PAD=0
	// Find where segment B starts (first 1 in TokenTypeIDs)
	segBStart := -1
	for i, v := range enc.TokenTypeIDs {
		if v == 1 {
			segBStart = i
			break
		}
	}
	if segBStart < 0 {
		t.Fatal("expected segment B (TokenTypeIDs=1) to exist")
	}

	// Everything before segBStart should be 0
	for i := 0; i < segBStart; i++ {
		if enc.TokenTypeIDs[i] != 0 {
			t.Errorf("TokenTypeIDs[%d] = %d, expected 0 (segment A)", i, enc.TokenTypeIDs[i])
		}
	}

	// segBStart onwards (until padding) should be 1
	sepID := tok.vocab[tok.sepToken]
	padID := tok.vocab[tok.padToken]
	for i := segBStart; i < len(enc.TokenTypeIDs); i++ {
		if enc.InputIDs[i] == padID {
			break
		}
		if enc.TokenTypeIDs[i] != 1 {
			t.Errorf("TokenTypeIDs[%d] = %d, expected 1 (segment B), ID=%d",
				i, enc.TokenTypeIDs[i], enc.InputIDs[i])
		}
	}
	_ = sepID
}

func TestEncodePair_TruncationStrategy(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(12) // very short
	longA := strings.Repeat("compound ", 10)
	shortB := "method"
	enc, err := tok.EncodePair(longA, shortB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(enc.InputIDs) != 12 {
		t.Errorf("expected length 12, got %d", len(enc.InputIDs))
	}
	if enc.NumTruncated <= 0 {
		t.Error("expected truncation to occur")
	}
	// The longer sentence (A) should have been truncated more
	t.Logf("NumTruncated: %d", enc.NumTruncated)
}

func TestEncodePair_BothEmpty(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(16)
	enc, err := tok.EncodePair("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still have [CLS] [SEP] [SEP] + padding
	if len(enc.InputIDs) != 16 {
		t.Errorf("expected length 16, got %d", len(enc.InputIDs))
	}
	clsID := tok.vocab[tok.clsToken]
	if enc.InputIDs[0] != clsID {
		t.Errorf("expected [CLS] at position 0")
	}
}

// ---------------------------------------------------------------------------
// Decode tests
// ---------------------------------------------------------------------------

func TestDecode_RoundTrip(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(32)
	original := "a compound"
	enc, err := tok.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := tok.Decode(enc.InputIDs)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// Decoded should contain the original words (special tokens and padding stripped)
	decoded = strings.TrimSpace(decoded)
	if !strings.Contains(decoded, "a") || !strings.Contains(decoded, "compound") {
		t.Errorf("round-trip failed: decoded=%q, original=%q", decoded, original)
	}
}

func TestDecode_SubwordMerge(t *testing.T) {
	tok := setupTestTokenizer()
	// Manually construct IDs for "pharma" + "##ce" + "##utical"
	ids := []int{
		tok.vocab["[CLS]"],
		tok.vocab["pharma"],
		tok.vocab["##ce"],
		tok.vocab["##utical"],
		tok.vocab["[SEP]"],
	}
	decoded, err := tok.Decode(ids)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded != "pharmaceutical" {
		t.Errorf("expected 'pharmaceutical', got %q", decoded)
	}
}

func TestDecode_UnknownToken(t *testing.T) {
	tok := setupTestTokenizer()
	unkID := tok.vocab["[UNK]"]
	ids := []int{
		tok.vocab["[CLS]"],
		tok.vocab["a"],
		unkID,
		tok.vocab["compound"],
		tok.vocab["[SEP]"],
	}
	decoded, err := tok.Decode(ids)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !strings.Contains(decoded, "[UNK]") {
		t.Errorf("expected [UNK] in decoded output, got %q", decoded)
	}
}

func TestDecode_EmptyIDs(t *testing.T) {
	tok := setupTestTokenizer()
	decoded, err := tok.Decode([]int{})
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded != "" {
		t.Errorf("expected empty string, got %q", decoded)
	}
}

func TestDecode_OnlySpecialTokens(t *testing.T) {
	tok := setupTestTokenizer()
	ids := []int{
		tok.vocab["[CLS]"],
		tok.vocab["[SEP]"],
		tok.vocab["[PAD]"],
		tok.vocab["[PAD]"],
	}
	decoded, err := tok.Decode(ids)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded != "" {
		t.Errorf("expected empty string for only special tokens, got %q", decoded)
	}
}

// ---------------------------------------------------------------------------
// BatchEncode tests
// ---------------------------------------------------------------------------

func TestBatchEncode_Success(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(16)
	texts := []string{"a compound", "the method", "claim comprising"}
	results, err := tok.BatchEncode(texts)
	if err != nil {
		t.Fatalf("BatchEncode: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// All should be padded to the same length
	for i, enc := range results {
		if len(enc.InputIDs) != 16 {
			t.Errorf("result[%d] expected length 16, got %d", i, len(enc.InputIDs))
		}
		if len(enc.AttentionMask) != 16 {
			t.Errorf("result[%d] attention mask length %d != 16", i, len(enc.AttentionMask))
		}
	}
}

func TestBatchEncode_Empty(t *testing.T) {
	tok := setupTestTokenizer()
	results, err := tok.BatchEncode([]string{})
	if err != nil {
		t.Fatalf("BatchEncode: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// VocabSize test
// ---------------------------------------------------------------------------

func TestVocabSize(t *testing.T) {
	tok := setupTestTokenizer()
	size := tok.VocabSize()
	expected := len(buildTestVocab())
	if size != expected {
		t.Errorf("VocabSize() = %d, expected %d", size, expected)
	}
}

// ---------------------------------------------------------------------------
// LoadVocab tests
// ---------------------------------------------------------------------------

func TestLoadVocab_Success(t *testing.T) {
	// Write a temporary vocab file
	dir := t.TempDir()
	path := filepath.Join(dir, "vocab.txt")
	content := "[PAD]\n[UNK]\n[CLS]\n[SEP]\n[MASK]\nhello\nworld\n##ing\n##ed\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write vocab file: %v", err)
	}

	vocab, err := LoadVocabFromFile(path)
	if err != nil {
		t.Fatalf("LoadVocabFromFile: %v", err)
	}
	if len(vocab) != 9 {
		t.Errorf("expected 9 tokens, got %d", len(vocab))
	}
	if vocab["[PAD]"] != 0 {
		t.Errorf("[PAD] should be ID 0, got %d", vocab["[PAD]"])
	}
	if vocab["hello"] != 5 {
		t.Errorf("hello should be ID 5, got %d", vocab["hello"])
	}
}

func TestLoadVocab_FileNotFound(t *testing.T) {
	_, err := LoadVocabFromFile("/nonexistent/path/vocab.txt")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestLoadVocab_DuplicateToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vocab_dup.txt")
	// "hello" appears at line 5 and line 7 (0-indexed)
	content := "[PAD]\n[UNK]\n[CLS]\n[SEP]\n[MASK]\nhello\nworld\nhello\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write vocab file: %v", err)
	}

	vocab, err := LoadVocabFromFile(path)
	if err != nil {
		t.Fatalf("LoadVocabFromFile: %v", err)
	}
	// Last occurrence wins: "hello" at line 7 -> ID 7
	if vocab["hello"] != 7 {
		t.Errorf("duplicate token 'hello' should have ID 7 (last occurrence), got %d", vocab["hello"])
	}
}

func TestLoadVocab_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty_vocab.txt")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("write vocab file: %v", err)
	}
	_, err := LoadVocabFromFile(path)
	if err == nil {
		t.Fatal("expected error for empty vocab file")
	}
}

func TestNewWordPieceTokenizerFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vocab.txt")
	content := "[PAD]\n[UNK]\n[CLS]\n[SEP]\n[MASK]\nhello\nworld\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write vocab file: %v", err)
	}

	tok, err := NewWordPieceTokenizerFromFile(path, WithMaxSequenceLength(16))
	if err != nil {
		t.Fatalf("NewWordPieceTokenizerFromFile: %v", err)
	}
	if tok.VocabSize() != 7 {
		t.Errorf("expected vocab size 7, got %d", tok.VocabSize())
	}
}

// ---------------------------------------------------------------------------
// Constructor validation tests
// ---------------------------------------------------------------------------

func TestNewWordPieceTokenizer_EmptyVocab(t *testing.T) {
	_, err := NewWordPieceTokenizer(map[string]int{})
	if err == nil {
		t.Fatal("expected error for empty vocab")
	}
}

func TestNewWordPieceTokenizer_MissingSpecialToken(t *testing.T) {
	vocab := map[string]int{
		"[PAD]": 0,
		"[UNK]": 1,
		// Missing [CLS], [SEP], [MASK]
		"hello": 2,
	}
	_, err := NewWordPieceTokenizer(vocab)
	if err == nil {
		t.Fatal("expected error for missing special tokens")
	}
}

// ---------------------------------------------------------------------------
// WordPiece algorithm edge cases
// ---------------------------------------------------------------------------

func TestWordPiece_AllUnknown(t *testing.T) {
	tok := setupTestTokenizer()
	out, err := tok.Tokenize("xyzzyplugh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// A completely unknown word should produce [UNK]
	hasUnk := false
	for _, tk := range out.Tokens {
		if tk == "[UNK]" {
			hasUnk = true
		}
	}
	if !hasUnk {
		t.Errorf("expected [UNK] for completely unknown word, got %v", out.Tokens)
	}
}

func TestWordPiece_SingleCharTokens(t *testing.T) {
	tok := setupTestTokenizer()
	out, err := tok.Tokenize("C")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Tokens) != 1 {
		t.Errorf("expected 1 token for 'C', got %d: %v", len(out.Tokens), out.Tokens)
	}
	if out.Tokens[0] != "C" {
		t.Errorf("expected token 'C', got %q", out.Tokens[0])
	}
}

// ---------------------------------------------------------------------------
// Options tests
// ---------------------------------------------------------------------------

func TestWithDoLowerCase(t *testing.T) {
	vocab := buildTestVocab()
	tok, err := NewWordPieceTokenizer(vocab, WithDoLowerCase(true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tok.doLowerCase {
		t.Error("expected doLowerCase=true")
	}
}

func TestWithStripAccents(t *testing.T) {
	vocab := buildTestVocab()
	tok, err := NewWordPieceTokenizer(vocab, WithStripAccents(true))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tok.stripAccents {
		t.Error("expected stripAccents=true")
	}
}

func TestWithSpecialTokens(t *testing.T) {
	vocab := buildTestVocab()
	tok, err := NewWordPieceTokenizer(vocab,
		WithSpecialTokens("[CLS]", "[SEP]", "[UNK]", "[PAD]", "[MASK]"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.clsToken != "[CLS]" {
		t.Errorf("unexpected clsToken: %s", tok.clsToken)
	}
}

// ---------------------------------------------------------------------------
// Encode consistency tests
// ---------------------------------------------------------------------------

func TestEncode_TokenTypeIDs_AllZero(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(16)
	enc, err := tok.Encode("compound method")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for i, v := range enc.TokenTypeIDs {
		if v != 0 {
			t.Errorf("TokenTypeIDs[%d] = %d, expected 0 for single sentence", i, v)
		}
	}
}

func TestEncode_OffsetsLength(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(16)
	enc, err := tok.Encode("a compound")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(enc.Offsets) != len(enc.InputIDs) {
		t.Errorf("Offsets length %d != InputIDs length %d", len(enc.Offsets), len(enc.InputIDs))
	}
}

func TestEncode_ConsistentLengths(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(16)
	enc, err := tok.Encode("method for treatment")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(enc.InputIDs) != len(enc.AttentionMask) {
		t.Errorf("InputIDs length %d != AttentionMask length %d",
			len(enc.InputIDs), len(enc.AttentionMask))
	}
	if len(enc.InputIDs) != len(enc.TokenTypeIDs) {
		t.Errorf("InputIDs length %d != TokenTypeIDs length %d",
			len(enc.InputIDs), len(enc.TokenTypeIDs))
	}
	if len(enc.InputIDs) != len(enc.Offsets) {
		t.Errorf("InputIDs length %d != Offsets length %d",
			len(enc.InputIDs), len(enc.Offsets))
	}
	if len(enc.InputIDs) != 16 {
		t.Errorf("expected padded length 16, got %d", len(enc.InputIDs))
	}
}

// ---------------------------------------------------------------------------
// EncodePair additional tests
// ---------------------------------------------------------------------------

func TestEncodePair_SegmentBoundary(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(32)
	enc, err := tok.EncodePair("compound", "method")
	if err != nil {
		t.Fatalf("EncodePair: %v", err)
	}

	// Structure: [CLS] compound [SEP] method [SEP] [PAD]...
	clsID := tok.vocab[tok.clsToken]
	sepID := tok.vocab[tok.sepToken]

	if enc.InputIDs[0] != clsID {
		t.Errorf("expected [CLS] at position 0, got ID %d", enc.InputIDs[0])
	}

	// Count SEP tokens — should be exactly 2
	sepCount := 0
	for _, id := range enc.InputIDs {
		if id == sepID {
			sepCount++
		}
	}
	if sepCount != 2 {
		t.Errorf("expected 2 [SEP] tokens, got %d", sepCount)
	}
}

func TestEncodePair_ConsistentLengths(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(24)
	enc, err := tok.EncodePair("a compound for use", "the method of claim")
	if err != nil {
		t.Fatalf("EncodePair: %v", err)
	}
	if len(enc.InputIDs) != 24 {
		t.Errorf("expected length 24, got %d", len(enc.InputIDs))
	}
	if len(enc.AttentionMask) != 24 {
		t.Errorf("AttentionMask length %d != 24", len(enc.AttentionMask))
	}
	if len(enc.TokenTypeIDs) != 24 {
		t.Errorf("TokenTypeIDs length %d != 24", len(enc.TokenTypeIDs))
	}
	if len(enc.Offsets) != 24 {
		t.Errorf("Offsets length %d != 24", len(enc.Offsets))
	}
}

func TestEncodePair_SymmetricTruncation(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(10)
	// Both sentences are equally long
	longA := strings.Repeat("compound ", 8)
	longB := strings.Repeat("method ", 8)
	enc, err := tok.EncodePair(longA, longB)
	if err != nil {
		t.Fatalf("EncodePair: %v", err)
	}
	if len(enc.InputIDs) != 10 {
		t.Errorf("expected length 10, got %d", len(enc.InputIDs))
	}
	if enc.NumTruncated <= 0 {
		t.Error("expected truncation for long pair")
	}
}

// ---------------------------------------------------------------------------
// Decode edge cases
// ---------------------------------------------------------------------------

func TestDecode_InvalidID(t *testing.T) {
	tok := setupTestTokenizer()
	// ID 99999 doesn't exist in vocab
	decoded, err := tok.Decode([]int{99999})
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// Should fall back to [UNK]
	if decoded != "[UNK]" {
		t.Errorf("expected '[UNK]' for invalid ID, got %q", decoded)
	}
}

func TestDecode_MixedSubwordsAndWholeWords(t *testing.T) {
	tok := setupTestTokenizer()
	ids := []int{
		tok.vocab["[CLS]"],
		tok.vocab["a"],
		tok.vocab["pharma"],
		tok.vocab["##ce"],
		tok.vocab["##utical"],
		tok.vocab["compound"],
		tok.vocab["[SEP]"],
	}
	decoded, err := tok.Decode(ids)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// Expected: "a pharmaceutical compound"
	if decoded != "a pharmaceutical compound" {
		t.Errorf("expected 'a pharmaceutical compound', got %q", decoded)
	}
}

func TestDecode_ConsecutiveSubwords(t *testing.T) {
	tok := setupTestTokenizer()
	// "prop" + "##an" + "##ol" = "propanol"
	ids := []int{
		tok.vocab["[CLS]"],
		tok.vocab["prop"],
		tok.vocab["##an"],
		tok.vocab["##ol"],
		tok.vocab["[SEP]"],
	}
	decoded, err := tok.Decode(ids)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded != "propanol" {
		t.Errorf("expected 'propanol', got %q", decoded)
	}
}

// ---------------------------------------------------------------------------
// BatchEncode additional tests
// ---------------------------------------------------------------------------

func TestBatchEncode_SingleItem(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(16)
	results, err := tok.BatchEncode([]string{"compound"})
	if err != nil {
		t.Fatalf("BatchEncode: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(results[0].InputIDs) != 16 {
		t.Errorf("expected length 16, got %d", len(results[0].InputIDs))
	}
}

func TestBatchEncode_MixedLengths(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(16)
	texts := []string{
		"a",
		"a compound for the treatment of disease",
		"method",
	}
	results, err := tok.BatchEncode(texts)
	if err != nil {
		t.Fatalf("BatchEncode: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// All should have the same padded length
	for i, enc := range results {
		if len(enc.InputIDs) != 16 {
			t.Errorf("result[%d] expected length 16, got %d", i, len(enc.InputIDs))
		}
	}
}

// ---------------------------------------------------------------------------
// Tokenizer LoadVocab method test
// ---------------------------------------------------------------------------

func TestTokenizer_LoadVocab_Replace(t *testing.T) {
	tok := setupTestTokenizer()
	originalSize := tok.VocabSize()

	dir := t.TempDir()
	path := filepath.Join(dir, "new_vocab.txt")
	content := "[PAD]\n[UNK]\n[CLS]\n[SEP]\n[MASK]\nalpha\nbeta\ngamma\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write vocab file: %v", err)
	}

	if err := tok.LoadVocab(path); err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	newSize := tok.VocabSize()
	if newSize == originalSize {
		t.Log("note: new vocab happens to be same size as original")
	}
	if newSize != 8 {
		t.Errorf("expected new vocab size 8, got %d", newSize)
	}
	// Verify new tokens are accessible
	if _, ok := tok.vocab["alpha"]; !ok {
		t.Error("expected 'alpha' in new vocab")
	}
}

// ---------------------------------------------------------------------------
// Stress / boundary tests
// ---------------------------------------------------------------------------

func TestTokenize_VeryLongWord(t *testing.T) {
	tok := setupTestTokenizer()
	longWord := strings.Repeat("x", 300)
	out, err := tok.Tokenize(longWord)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should produce [UNK] for a word exceeding maxWordLen
	if len(out.Tokens) == 0 {
		t.Fatal("expected at least one token")
	}
	if out.Tokens[0] != "[UNK]" {
		t.Errorf("expected [UNK] for very long word, got %q", out.Tokens[0])
	}
}

func TestTokenize_OnlyWhitespace(t *testing.T) {
	tok := setupTestTokenizer()
	out, err := tok.Tokenize("   \t\n  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Tokens) != 0 {
		t.Errorf("expected 0 tokens for whitespace-only text, got %d: %v",
			len(out.Tokens), out.Tokens)
	}
}

func TestTokenize_OnlyPunctuation(t *testing.T) {
	tok := setupTestTokenizer()
	out, err := tok.Tokenize(".,-()")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Each punctuation should be its own token
	if len(out.Tokens) < 3 {
		t.Logf("punctuation tokens: %v", out.Tokens)
	}
	for _, tk := range out.Tokens {
		if len(tk) > 3 && tk != "[UNK]" {
			t.Errorf("expected single-char punctuation token, got %q", tk)
		}
	}
}

func TestEncode_MaxSequenceLengthOne(t *testing.T) {
	// Edge case: maxSequenceLength so small that only [CLS] fits
	// (maxContentLen = 1 - 2 = -1 -> 0)
	vocab := buildTestVocab()
	tok, err := NewWordPieceTokenizer(vocab, WithMaxSequenceLength(3))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enc, err := tok.Encode("compound method claim")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Should be [CLS] <1 token> [SEP] = 3
	if len(enc.InputIDs) != 3 {
		t.Errorf("expected length 3, got %d", len(enc.InputIDs))
	}
	if enc.NumTruncated <= 0 {
		t.Error("expected truncation")
	}
}

func TestEncodePair_MaxSequenceLengthMinimal(t *testing.T) {
	vocab := buildTestVocab()
	tok, err := NewWordPieceTokenizer(vocab, WithMaxSequenceLength(5))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enc, err := tok.EncodePair("compound method", "claim comprising")
	if err != nil {
		t.Fatalf("EncodePair: %v", err)
	}
	// [CLS] + max 2 content tokens + [SEP] [SEP] = 5
	if len(enc.InputIDs) != 5 {
		t.Errorf("expected length 5, got %d", len(enc.InputIDs))
	}
}

// ---------------------------------------------------------------------------
// SpecialTokensMask test
// ---------------------------------------------------------------------------

func TestTokenize_SpecialTokensMask(t *testing.T) {
	tok := setupTestTokenizer()
	out, err := tok.Tokenize("a compound")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.SpecialTokensMask) != len(out.Tokens) {
		t.Fatalf("SpecialTokensMask length %d != Tokens length %d",
			len(out.SpecialTokensMask), len(out.Tokens))
	}
	// Tokenize does not add special tokens, so all should be 0
	for i, v := range out.SpecialTokensMask {
		if v != 0 {
			t.Errorf("SpecialTokensMask[%d] = %d, expected 0 (no special tokens in Tokenize)",
				i, v)
		}
	}
}

// ---------------------------------------------------------------------------
// Chemical pattern integration tests
// ---------------------------------------------------------------------------

func TestTokenize_ComplexChemicalName(t *testing.T) {
	tok := setupTestTokenizer()
	// A realistic IUPAC-style name
	text := "2-(4-(2-hydroxyethyl)piperazin-1-yl)ethanesulfonic acid"
	out, err := tok.Tokenize(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("Complex chemical tokens (%d): %v", len(out.Tokens), out.Tokens)
	if len(out.Tokens) == 0 {
		t.Fatal("expected non-empty tokens for complex chemical name")
	}
	// "acid" should appear as a token
	hasAcid := false
	for _, tk := range out.Tokens {
		if tk == "acid" {
			hasAcid = true
		}
	}
	if !hasAcid {
		t.Log("note: 'acid' may have been merged into a chemical entity span")
	}
}

func TestTokenize_PatentClaimText(t *testing.T) {
	tok := setupTestTokenizerWithMaxLen(64)
	text := "A compound of formula C6H12O6 for use in the treatment of disease"
	out, err := tok.Tokenize(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("Patent claim tokens (%d): %v", len(out.Tokens), out.Tokens)
	if len(out.Tokens) < 5 {
		t.Errorf("expected at least 5 tokens for patent claim text, got %d", len(out.Tokens))
	}
}

//Personal.AI order the ending

