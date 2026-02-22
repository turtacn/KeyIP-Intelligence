package chem_extractor

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"golang.org/x/text/unicode/norm"
)

// ---------------------------------------------------------------------------
// Interfaces
// ---------------------------------------------------------------------------

// NERModel defines the chemical named entity recognition interface.
type NERModel interface {
	Predict(ctx context.Context, text string) (*NERPrediction, error)
	PredictBatch(ctx context.Context, texts []string) ([]*NERPrediction, error)
	GetLabelSet() []string
}

// ---------------------------------------------------------------------------
// Label constants
// ---------------------------------------------------------------------------

const (
	LabelO        = "O"
	LabelBIUPAC   = "B-IUPAC"
	LabelIIUPAC   = "I-IUPAC"
	LabelBCAS     = "B-CAS"
	LabelICAS     = "I-CAS"
	LabelBFORMULA = "B-FORMULA"
	LabelIFORMULA = "I-FORMULA"
	LabelBSMILES  = "B-SMILES"
	LabelISMILES  = "I-SMILES"
	LabelBCOMMON  = "B-COMMON"
	LabelICOMMON  = "I-COMMON"
	LabelBGENERIC = "B-GENERIC"
	LabelIGENERIC = "I-GENERIC"
	LabelBMARKUSH = "B-MARKUSH"
	LabelIMARKUSH = "I-MARKUSH"
)

// DefaultLabelSet is the full BIO label set for chemical NER.
var DefaultLabelSet = []string{
	LabelO,
	LabelBIUPAC, LabelIIUPAC,
	LabelBCAS, LabelICAS,
	LabelBFORMULA, LabelIFORMULA,
	LabelBSMILES, LabelISMILES,
	LabelBCOMMON, LabelICOMMON,
	LabelBGENERIC, LabelIGENERIC,
	LabelBMARKUSH, LabelIMARKUSH,
}

// ---------------------------------------------------------------------------
// Data structures
// ---------------------------------------------------------------------------

// NERPrediction holds the full output of a NER prediction.
type NERPrediction struct {
	Tokens        []string      `json:"tokens"`
	Labels        []string      `json:"labels"`
	Probabilities [][]float64   `json:"probabilities"`
	Entities      []*NEREntity  `json:"entities"`
}

// NEREntity represents a single recognized chemical entity span.
type NEREntity struct {
	Text       string  `json:"text"`
	Label      string  `json:"label"`
	StartToken int     `json:"start_token"`
	EndToken   int     `json:"end_token"`
	StartChar  int     `json:"start_char"`
	EndChar    int     `json:"end_char"`
	Score      float64 `json:"score"`
}

// NERModelConfig holds configuration for the NER model.
type NERModelConfig struct {
	ModelID             string   `json:"model_id" yaml:"model_id"`
	ModelPath           string   `json:"model_path" yaml:"model_path"`
	BackendType         string   `json:"backend_type" yaml:"backend_type"`
	MaxSequenceLength   int      `json:"max_sequence_length" yaml:"max_sequence_length"`
	LabelSet            []string `json:"label_set" yaml:"label_set"`
	ConfidenceThreshold float64  `json:"confidence_threshold" yaml:"confidence_threshold"`
	UseCRF              bool     `json:"use_crf" yaml:"use_crf"`
	TimeoutMs           int      `json:"timeout_ms" yaml:"timeout_ms"`
	MaxBatchSize        int      `json:"max_batch_size" yaml:"max_batch_size"`
}

// DefaultNERModelConfig returns a sensible default configuration.
func DefaultNERModelConfig() *NERModelConfig {
	return &NERModelConfig{
		ModelID:             "chem-ner-bert-crf-v1",
		ModelPath:           "",
		BackendType:         "triton",
		MaxSequenceLength:   256,
		LabelSet:            DefaultLabelSet,
		ConfidenceThreshold: 0.60,
		UseCRF:              true,
		TimeoutMs:           2000,
		MaxBatchSize:        64,
	}
}

// Validate checks the configuration for consistency.
func (c *NERModelConfig) Validate() error {
	if c.ModelID == "" {
		return errors.NewInvalidInputError("model_id is required")
	}
	if c.MaxSequenceLength <= 0 {
		return errors.NewInvalidInputError("max_sequence_length must be positive")
	}
	if len(c.LabelSet) == 0 {
		return errors.NewInvalidInputError("label_set must not be empty")
	}
	if c.ConfidenceThreshold < 0 || c.ConfidenceThreshold > 1 {
		return errors.NewInvalidInputError("confidence_threshold must be in [0, 1]")
	}
	if c.MaxBatchSize <= 0 {
		return errors.NewInvalidInputError("max_batch_size must be positive")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Token with character offsets
// ---------------------------------------------------------------------------

// tokenSpan holds a token string and its character offsets in the original text.
type tokenSpan struct {
	Text      string
	StartChar int
	EndChar   int
}

// ---------------------------------------------------------------------------
// Sliding window types
// ---------------------------------------------------------------------------

// windowResult holds the prediction for a single sliding window.
type windowResult struct {
	StartToken    int
	EndToken      int
	Labels        []string
	Probabilities [][]float64
}

// ---------------------------------------------------------------------------
// nerModelImpl
// ---------------------------------------------------------------------------

// nerModelImpl implements NERModel.
type nerModelImpl struct {
	backend    common.ModelBackend
	config     *NERModelConfig
	logger     common.Logger
	metrics    common.IntelligenceMetrics
	labelIndex map[string]int
	indexLabel  map[int]string
	transition [][]float64 // CRF transition matrix [from][to]
	mu         sync.RWMutex
}

// NewNERModel creates a new NER model instance.
func NewNERModel(
	backend common.ModelBackend,
	config *NERModelConfig,
	logger common.Logger,
	metrics common.IntelligenceMetrics,
) (NERModel, error) {
	if backend == nil {
		return nil, errors.NewInvalidInputError("backend is required")
	}
	if config == nil {
		config = DefaultNERModelConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = common.NewNoopLogger()
	}
	if metrics == nil {
		metrics = common.NewNoopIntelligenceMetrics()
	}

	labelIdx := make(map[string]int, len(config.LabelSet))
	idxLabel := make(map[int]string, len(config.LabelSet))
	for i, l := range config.LabelSet {
		labelIdx[l] = i
		idxLabel[i] = l
	}

	m := &nerModelImpl{
		backend:    backend,
		config:     config,
		logger:     logger,
		metrics:    metrics,
		labelIndex: labelIdx,
		indexLabel:  idxLabel,
	}

	if config.UseCRF {
		m.transition = buildBIOTransitionMatrix(config.LabelSet, labelIdx)
	}

	return m, nil
}

// GetLabelSet returns the full label set.
func (m *nerModelImpl) GetLabelSet() []string {
	out := make([]string, len(m.config.LabelSet))
	copy(out, m.config.LabelSet)
	return out
}

// ---------------------------------------------------------------------------
// Predict — single text
// ---------------------------------------------------------------------------

func (m *nerModelImpl) Predict(ctx context.Context, text string) (*NERPrediction, error) {
	if text == "" {
		return &NERPrediction{
			Tokens:        []string{},
			Labels:        []string{},
			Probabilities: [][]float64{},
			Entities:      []*NEREntity{},
		}, nil
	}

	start := time.Now()

	// 1. Preprocess
	normalized := normalizeText(text)

	// 2. Tokenize
	spans := tokenize(normalized)
	if len(spans) == 0 {
		return &NERPrediction{
			Tokens:        []string{},
			Labels:        []string{},
			Probabilities: [][]float64{},
			Entities:      []*NEREntity{},
		}, nil
	}

	tokens := make([]string, len(spans))
	for i, s := range spans {
		tokens[i] = s.Text
	}

	// 3. Sliding window
	maxLen := m.config.MaxSequenceLength
	windows := buildSlidingWindows(len(tokens), maxLen)

	// 4. Predict each window
	windowResults := make([]*windowResult, len(windows))
	for i, w := range windows {
		windowTokens := tokens[w[0]:w[1]]
		emission, err := m.invokeBackend(ctx, windowTokens)
		if err != nil {
			return nil, fmt.Errorf("backend predict window %d: %w", i, err)
		}

		var labels []string
		if m.config.UseCRF && m.transition != nil {
			labels = viterbiDecode(emission, m.transition, m.config.LabelSet, m.labelIndex)
		} else {
			labels = argmaxDecode(emission, m.indexLabel)
		}

		windowResults[i] = &windowResult{
			StartToken:    w[0],
			EndToken:      w[1],
			Labels:        labels,
			Probabilities: emission,
		}
	}

	// 5. Merge windows
	mergedLabels, mergedProbs := mergeWindows(windowResults, len(tokens), len(m.config.LabelSet))

	// 6. Fix BIO legality
	mergedLabels = fixBIOLegality(mergedLabels)

	// 7. Extract entities from BIO
	rawEntities := bioToEntities(tokens, mergedLabels, mergedProbs, spans)

	// 8. Filter by confidence
	var entities []*NEREntity
	for _, e := range rawEntities {
		if e.Score >= m.config.ConfidenceThreshold {
			entities = append(entities, e)
		}
	}
	if entities == nil {
		entities = []*NEREntity{}
	}

	elapsed := time.Since(start)
	m.metrics.RecordInference(ctx, &common.InferenceMetricParams{
		ModelName:  m.config.ModelID,
		TaskType:   "ner",
		DurationMs: float64(elapsed.Milliseconds()),
		Success:    true,
		BatchSize:  1,
	})

	return &NERPrediction{
		Tokens:        tokens,
		Labels:        mergedLabels,
		Probabilities: mergedProbs,
		Entities:      entities,
	}, nil
}

// ---------------------------------------------------------------------------
// PredictBatch
// ---------------------------------------------------------------------------

func (m *nerModelImpl) PredictBatch(ctx context.Context, texts []string) ([]*NERPrediction, error) {
	if len(texts) == 0 {
		return []*NERPrediction{}, nil
	}

	results := make([]*NERPrediction, len(texts))
	errs := make([]error, len(texts))

	batchSize := m.config.MaxBatchSize
	if batchSize <= 0 {
		batchSize = 64
	}

	for chunkStart := 0; chunkStart < len(texts); chunkStart += batchSize {
		chunkEnd := chunkStart + batchSize
		if chunkEnd > len(texts) {
			chunkEnd = len(texts)
		}
		chunk := texts[chunkStart:chunkEnd]

		var wg sync.WaitGroup
		for i, t := range chunk {
			idx := chunkStart + i
			wg.Add(1)
			go func(index int, text string) {
				defer wg.Done()
				pred, err := m.Predict(ctx, text)
				results[index] = pred
				errs[index] = err
			}(idx, t)
		}
		wg.Wait()
	}

	// Collect first error
	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("batch item %d: %w", i, err)
		}
	}

	return results, nil
}

// ---------------------------------------------------------------------------
// Backend invocation
// ---------------------------------------------------------------------------

func (m *nerModelImpl) invokeBackend(ctx context.Context, tokens []string) ([][]float64, error) {
	payload := common.EncodeTokenList(tokens)
	req := &common.PredictRequest{
		ModelName:   m.config.ModelID,
		InputData:   payload,
		InputFormat: common.FormatJSON,
		Metadata:    map[string]string{"task": "ner", "num_tokens": fmt.Sprintf("%d", len(tokens))},
	}

	timeoutDur := time.Duration(m.config.TimeoutMs) * time.Millisecond
	if timeoutDur <= 0 {
		timeoutDur = 2 * time.Second
	}
	ctx2, cancel := context.WithTimeout(ctx, timeoutDur)
	defer cancel()

	resp, err := m.backend.Predict(ctx2, req)
	if err != nil {
		return nil, err
	}

	emission, err := common.DecodeFloat64Matrix(resp.Outputs["emission"])
	if err != nil {
		return nil, fmt.Errorf("decode emission matrix: %w", err)
	}

	// Validate dimensions
	if len(emission) != len(tokens) {
		return nil, fmt.Errorf("emission rows %d != tokens %d", len(emission), len(tokens))
	}
	numLabels := len(m.config.LabelSet)
	for i, row := range emission {
		if len(row) != numLabels {
			return nil, fmt.Errorf("emission row %d cols %d != labels %d", i, len(row), numLabels)
		}
	}

	return emission, nil
}

// ---------------------------------------------------------------------------
// Text preprocessing
// ---------------------------------------------------------------------------

// normalizeText applies Unicode NFC normalization and whitespace standardization.
func normalizeText(text string) string {
	text = norm.NFC.String(text)
	// Collapse whitespace
	var b strings.Builder
	b.Grow(len(text))
	prevSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

// tokenize splits text on whitespace and punctuation, preserving character offsets.
func tokenize(text string) []tokenSpan {
	var spans []tokenSpan
	runes := []rune(text)
	n := len(runes)
	i := 0
	for i < n {
		// Skip whitespace
		if unicode.IsSpace(runes[i]) {
			i++
			continue
		}

		// Check if punctuation (single-char token)
		if isPunctuation(runes[i]) {
			charStart := runeOffsetToByteOffset(text, i)
			charEnd := runeOffsetToByteOffset(text, i+1)
			spans = append(spans, tokenSpan{
				Text:      string(runes[i]),
				StartChar: charStart,
				EndChar:   charEnd,
			})
			i++
			continue
		}

		// Accumulate word characters
		start := i
		for i < n && !unicode.IsSpace(runes[i]) && !isPunctuation(runes[i]) {
			i++
		}
		charStart := runeOffsetToByteOffset(text, start)
		charEnd := runeOffsetToByteOffset(text, i)
		spans = append(spans, tokenSpan{
			Text:      string(runes[start:i]),
			StartChar: charStart,
			EndChar:   charEnd,
		})
	}
	return spans
}

func isPunctuation(r rune) bool {
	// Chemical-aware: don't split on hyphens inside words, but split on common punctuation
	switch r {
	case ',', '.', ';', ':', '!', '?', '(', ')', '[', ']', '{', '}', '"', '\'':
		return true
	}
	return false
}

func runeOffsetToByteOffset(s string, runeIdx int) int {
	byteOff := 0
	for i := 0; i < runeIdx && byteOff < len(s); i++ {
		_, size := decodeRuneAt(s, byteOff)
		byteOff += size
	}
	return byteOff
}

func decodeRuneAt(s string, byteOff int) (rune, int) {
	if byteOff >= len(s) {
		return 0, 0
	}
	r := rune(s[byteOff])
	size := 1
	if r >= 0x80 {
		// Multi-byte UTF-8
		for _, ch := range s[byteOff:] {
			r = ch
			size = len(string(ch))
			break
		}
	}
	return r, size
}

// ---------------------------------------------------------------------------
// Sliding windows
// ---------------------------------------------------------------------------

// buildSlidingWindows returns [start, end) pairs for sliding windows.
func buildSlidingWindows(numTokens, maxLen int) [][2]int {
	if numTokens <= maxLen {
		return [][2]int{{0, numTokens}}
	}

	step := maxLen / 2
	if step <= 0 {
		step = 1
	}

	var windows [][2]int
	for start := 0; start < numTokens; start += step {
		end := start + maxLen
		if end > numTokens {
			end = numTokens
		}
		windows = append(windows, [2]int{start, end})
		if end == numTokens {
			break
		}
	}
	return windows
}

// mergeWindows merges overlapping window predictions.
// For overlapping tokens, the prediction with higher max probability wins.
func mergeWindows(results []*windowResult, totalTokens, numLabels int) ([]string, [][]float64) {
	labels := make([]string, totalTokens)
	probs := make([][]float64, totalTokens)
	maxProb := make([]float64, totalTokens)

	for i := range labels {
		labels[i] = LabelO
		probs[i] = make([]float64, numLabels)
		maxProb[i] = -1
	}

	for _, wr := range results {
		for j := 0; j < len(wr.Labels); j++ {
			globalIdx := wr.StartToken + j
			if globalIdx >= totalTokens {
				break
			}
			tokenMaxProb := maxFloat64Slice(wr.Probabilities[j])
			if tokenMaxProb > maxProb[globalIdx] {
				maxProb[globalIdx] = tokenMaxProb
				labels[globalIdx] = wr.Labels[j]
				copy(probs[globalIdx], wr.Probabilities[j])
			}
		}
	}

	return labels, probs
}

func maxFloat64Slice(s []float64) float64 {
	if len(s) == 0 {
		return 0
	}
	m := s[0]
	for _, v := range s[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// ---------------------------------------------------------------------------
// BIO legality fix
// ---------------------------------------------------------------------------

// fixBIOLegality ensures no I-X tag appears without a preceding B-X or I-X.
func fixBIOLegality(labels []string) []string {
	fixed := make([]string, len(labels))
	copy(fixed, labels)

	for i, l := range fixed {
		if !strings.HasPrefix(l, "I-") {
			continue
		}
		entityType := l[2:]
		if i == 0 {
			// I at start -> convert to B
			fixed[i] = "B-" + entityType
			continue
		}
		prev := fixed[i-1]
		prevType := ""
		if strings.HasPrefix(prev, "B-") {
			prevType = prev[2:]
		} else if strings.HasPrefix(prev, "I-") {
			prevType = prev[2:]
		}
		if prevType != entityType {
			// Orphan I -> convert to B
			fixed[i] = "B-" + entityType
		}
	}
	return fixed
}

// ---------------------------------------------------------------------------
// BIO -> Entity extraction
// ---------------------------------------------------------------------------

// bioToEntities converts BIO label sequences to entity spans.
func bioToEntities(tokens, labels []string, probs [][]float64, spans []tokenSpan) []*NEREntity {
	var entities []*NEREntity
	n := len(tokens)
	i := 0

	for i < n {
		label := labels[i]
		if !strings.HasPrefix(label, "B-") {
			i++
			continue
		}

		entityType := label[2:]
		startToken := i
		i++

		// Collect continuation I- tokens
		for i < n && labels[i] == "I-"+entityType {
			i++
		}
		endToken := i // exclusive

		// Build entity text
		var textParts []string
		for j := startToken; j < endToken; j++ {
			textParts = append(textParts, tokens[j])
		}
		entityText := strings.Join(textParts, " ")

		// Character offsets
		startChar := 0
		endChar := 0
		if startToken < len(spans) {
			startChar = spans[startToken].StartChar
		}
		if endToken-1 < len(spans) {
			endChar = spans[endToken-1].EndChar
		}

		// Confidence: geometric mean of max probabilities
		score := computeEntityConfidence(probs, startToken, endToken)

		entities = append(entities, &NEREntity{
			Text:       entityText,
			Label:      entityType,
			StartToken: startToken,
			EndToken:   endToken,
			StartChar:  startChar,
			EndChar:    endChar,
			Score:      score,
		})
	}

	if entities == nil {
		entities = []*NEREntity{}
	}
	return entities
}

// computeEntityConfidence computes the geometric mean of max token probabilities.
func computeEntityConfidence(probs [][]float64, startToken, endToken int) float64 {
	n := endToken - startToken
	if n <= 0 {
		return 0
	}

	logSum := 0.0
	for i := startToken; i < endToken; i++ {
		if i >= len(probs) {
			break
		}
		p := maxFloat64Slice(probs[i])
		if p <= 0 {
			return 0
		}
		logSum += math.Log(p)
	}

	return math.Exp(logSum / float64(n))
}

// ---------------------------------------------------------------------------
// Viterbi decoding
// ---------------------------------------------------------------------------

// viterbiDecode finds the optimal label sequence given emission and transition scores.
// Time complexity: O(seq_len * num_labels^2)
func viterbiDecode(emission [][]float64, transition [][]float64, labelSet []string, labelIndex map[string]int) []string {
	seqLen := len(emission)
	numLabels := len(labelSet)

	if seqLen == 0 {
		return []string{}
	}

	// dp[t][j] = best log-score ending at time t with label j
	dp := make([][]float64, seqLen)
	backptr := make([][]int, seqLen)
	for t := 0; t < seqLen; t++ {
		dp[t] = make([]float64, numLabels)
		backptr[t] = make([]int, numLabels)
	}

	// Initialize t=0
	for j := 0; j < numLabels; j++ {
		score := safeLog(emission[0][j])
		// Only O or B-* labels are valid at position 0
		if strings.HasPrefix(labelSet[j], "I-") {
			score = math.Inf(-1)
		}
		dp[0][j] = score
		backptr[0][j] = -1
	}

	// Forward pass
	for t := 1; t < seqLen; t++ {
		for j := 0; j < numLabels; j++ {
			bestScore := math.Inf(-1)
			bestPrev := 0
			for k := 0; k < numLabels; k++ {
				s := dp[t-1][k] + safeLog(transition[k][j]) + safeLog(emission[t][j])
				if s > bestScore {
					bestScore = s
					bestPrev = k
				}
			}
			dp[t][j] = bestScore
			backptr[t][j] = bestPrev
		}
	}

	// Find best final label
	bestFinal := 0
	bestScore := dp[seqLen-1][0]
	for j := 1; j < numLabels; j++ {
		if dp[seqLen-1][j] > bestScore {
			bestScore = dp[seqLen-1][j]
			bestFinal = j
		}
	}

	// Backtrack
	path := make([]int, seqLen)
	path[seqLen-1] = bestFinal
	for t := seqLen - 2; t >= 0; t-- {
		path[t] = backptr[t+1][path[t+1]]
	}

	// Convert to labels
	labels := make([]string, seqLen)
	for t, idx := range path {
		if idx >= 0 && idx < numLabels {
			labels[t] = labelSet[idx]
		} else {
			labels[t] = LabelO
		}
	}

	return labels
}

func safeLog(x float64) float64 {
	if x <= 0 {
		return -1e10
	}
	return math.Log(x)
}

// ---------------------------------------------------------------------------
// Argmax decoding (no CRF)
// ---------------------------------------------------------------------------

func argmaxDecode(emission [][]float64, indexLabel map[int]string) []string {
	labels := make([]string, len(emission))
	for i, row := range emission {
		bestIdx := 0
		bestVal := row[0]
		for j := 1; j < len(row); j++ {
			if row[j] > bestVal {
				bestVal = row[j]
				bestIdx = j
			}
		}
		l, ok := indexLabel[bestIdx]
		if !ok {
			l = LabelO
		}
		labels[i] = l
	}
	return labels
}

// ---------------------------------------------------------------------------
// BIO transition matrix construction
// ---------------------------------------------------------------------------

// buildBIOTransitionMatrix creates a transition probability matrix that encodes
// BIO constraints. Legal transitions get probability 1.0, illegal ones get 0.0.
//
// Legal transitions:
//   O  -> O, B-*           (O cannot go to I-*)
//   B-X -> I-X, O, B-*     (B-X can continue with I-X or start new)
//   I-X -> I-X, O, B-*     (I-X can continue or end)
//
// Illegal:
//   O  -> I-*
//   B-X -> I-Y (where Y != X)
//   I-X -> I-Y (where Y != X)
func buildBIOTransitionMatrix(labelSet []string, labelIndex map[string]int) [][]float64 {
	n := len(labelSet)
	trans := make([][]float64, n)
	for i := range trans {
		trans[i] = make([]float64, n)
	}

	for i, from := range labelSet {
		for j, to := range labelSet {
			if isLegalBIOTransition(from, to) {
				trans[i][j] = 1.0
			} else {
				trans[i][j] = 0.0
			}
		}
	}

	return trans
}

func isLegalBIOTransition(from, to string) bool {
	// Any -> O is always legal
	if to == LabelO {
		return true
	}
	// Any -> B-* is always legal
	if strings.HasPrefix(to, "B-") {
		return true
	}
	// to is I-X
	if !strings.HasPrefix(to, "I-") {
		return false
	}
	toType := to[2:]

	// O -> I-X is illegal
	if from == LabelO {
		return false
	}

	// B-X -> I-X is legal, B-X -> I-Y is illegal
	if strings.HasPrefix(from, "B-") {
		return from[2:] == toType
	}

	// I-X -> I-X is legal, I-X -> I-Y is illegal
	if strings.HasPrefix(from, "I-") {
		return from[2:] == toType
	}

	return false
}

