// Local (non-server) implementations of core dependencies for the CLI.
package main

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	domainMol "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/search/milvus"
	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ============================================================================
// localFingerprintEngine — implements patent_mining.FingerprintEngine
// using domain-level bit operations (PopCount, BitAnd) for Tanimoto / Dice
// and an optional RemoteFingerprintCalculator for computing fingerprints from
// SMILES when a chemistry service is available.
// ============================================================================

// localFingerprintEngine is a real implementation of FingerprintEngine that:
//   - Computes similarity (Tanimoto, Dice, Cosine, Euclidean) using the
//     domain's PopCount and BitAnd functions on raw byte slices.
//   - Caches computed fingerprints in an in-memory store so that
//     SearchSimilar can return results within a session.
//   - For ComputeFingerprint, delegates to a FingerprintCalculator when one is
//     available (e.g. a chemistry micro-service); otherwise returns a clear
//     error message directing users to the API server.
type localFingerprintEngine struct {
	calculator domainMol.FingerprintCalculator
	entries    []fingerprintEntry
	mu         sync.RWMutex
}

type fingerprintEntry struct {
	smiles string
	fp     []byte
	fpType string
	radius int
	nBits  int
}

func newLocalFingerprintEngine(calculator domainMol.FingerprintCalculator) *localFingerprintEngine {
	return &localFingerprintEngine{
		calculator: calculator,
	}
}

func (e *localFingerprintEngine) ComputeFingerprint(ctx context.Context, smiles string, fpType string, radius int, nBits int) ([]byte, error) {
	if e.calculator != nil {
		parsedType, err := domainMol.ParseFingerprintType(fpType)
		if err != nil {
			return nil, fmt.Errorf("invalid fingerprint type %q: %w", fpType, err)
		}
		opts := &domainMol.FingerprintCalcOptions{
			Radius:  radius,
			NumBits: nBits,
		}
		fp, err := e.calculator.Calculate(ctx, smiles, parsedType, opts)
		if err != nil {
			return nil, fmt.Errorf("fingerprint calculation: %w", err)
		}
		e.mu.Lock()
		e.entries = append(e.entries, fingerprintEntry{
			smiles: smiles,
			fp:     fp.Bits,
			fpType: fpType,
			radius: radius,
			nBits:  nBits,
		})
		e.mu.Unlock()
		return fp.Bits, nil
	}
	return nil, apperrors.NewMsg("fingerprint computation requires a chemistry engine; use --server <addr> to connect to the KeyIP API server")
}

func (e *localFingerprintEngine) ComputeSimilarity(ctx context.Context, fp1 []byte, fp2 []byte, metric patent_mining.SimilarityMetric) (float64, error) {
	if len(fp1) == 0 || len(fp2) == 0 {
		return 0, apperrors.NewMsg("fingerprint byte slices must not be empty")
	}
	if len(fp1) != len(fp2) {
		return 0, apperrors.NewMsg("fingerprint byte slices must have equal length")
	}
	switch metric {
	case patent_mining.MetricTanimoto:
		return computeTanimoto(fp1, fp2)
	case patent_mining.MetricDice:
		return computeDice(fp1, fp2)
	case patent_mining.MetricCosine:
		return computeCosineBit(fp1, fp2)
	case patent_mining.MetricEuclidean:
		return computeEuclideanBit(fp1, fp2)
	default:
		return computeTanimoto(fp1, fp2)
	}
}

func (e *localFingerprintEngine) SearchSimilar(ctx context.Context, queryFP []byte, metric patent_mining.SimilarityMetric, threshold float64, maxResults int) ([]patent_mining.SimilarityHit, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var hits []patent_mining.SimilarityHit
	for _, entry := range e.entries {
		sim, err := e.ComputeSimilarity(ctx, queryFP, entry.fp, metric)
		if err != nil {
			continue
		}
		if sim >= threshold {
			hits = append(hits, patent_mining.SimilarityHit{
				SMILES: entry.smiles,
				Score:  sim,
				Metric: metric,
			})
		}
	}

	sort.Slice(hits, func(i, j int) bool {
		return hits[i].Score > hits[j].Score
	})
	if len(hits) > maxResults {
		hits = hits[:maxResults]
	}
	return hits, nil
}

// --- Similarity computation helpers (use domain bit operations) ---

func computeTanimoto(a, b []byte) (float64, error) {
	and, err := domainMol.BitAnd(a, b)
	if err != nil {
		return 0, err
	}
	popA := float64(domainMol.PopCount(a))
	popB := float64(domainMol.PopCount(b))
	popAnd := float64(domainMol.PopCount(and))
	denom := popA + popB - popAnd
	if denom == 0 {
		return 0, nil
	}
	return round4(popAnd / denom), nil
}

func computeDice(a, b []byte) (float64, error) {
	and, err := domainMol.BitAnd(a, b)
	if err != nil {
		return 0, err
	}
	popA := float64(domainMol.PopCount(a))
	popB := float64(domainMol.PopCount(b))
	popAnd := float64(domainMol.PopCount(and))
	denom := popA + popB
	if denom == 0 {
		return 0, nil
	}
	return round4(2 * popAnd / denom), nil
}

func computeCosineBit(a, b []byte) (float64, error) {
	and, err := domainMol.BitAnd(a, b)
	if err != nil {
		return 0, err
	}
	popA := float64(domainMol.PopCount(a))
	popB := float64(domainMol.PopCount(b))
	popAnd := float64(domainMol.PopCount(and))
	if popA == 0 || popB == 0 {
		return 0, nil
	}
	return round4(popAnd / (math.Sqrt(popA) * math.Sqrt(popB))), nil
}

func computeEuclideanBit(a, b []byte) (float64, error) {
	if len(a) != len(b) {
		return 0, apperrors.NewMsg("byte slices must have equal length for euclidean distance")
	}
	var sumSq float64
	for i := range a {
		diff := float64(popcountByte(a[i] ^ b[i]))
		sumSq += diff * diff
	}
	dist := math.Sqrt(sumSq)
	maxDist := math.Sqrt(float64(len(a) * 8))
	if maxDist == 0 {
		return 0, nil
	}
	sim := 1.0 - dist/maxDist
	if sim < 0 {
		sim = 0
	}
	return round4(sim), nil
}

func popcountByte(x byte) int {
	x = (x & 0x55) + ((x >> 1) & 0x55)
	x = (x & 0x33) + ((x >> 2) & 0x33)
	x = (x & 0x0f) + ((x >> 4) & 0x0f)
	return int(x)
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// ============================================================================
// milvusVectorStore — adapts the Milvus Searcher to patent_mining.VectorStore.
// Degrades gracefully when Milvus is not configured or unreachable.
// ============================================================================

type milvusVectorStore struct {
	searcher        *milvus.Searcher
	logger          logging.Logger
	collectionName  string
	vectorFieldName string
}

// newMilvusVectorStore attempts to initialise a Milvus-backed VectorStore.
// If the config does not specify a Milvus address, or the connection fails,
// the store logs a warning and SearchByVector will return a helpful error.
func newMilvusVectorStore(cfg *config.Config, logger logging.Logger) *milvusVectorStore {
	milCfg := cfg.Search.Milvus
	if milCfg.Address == "" {
		logger.Warn("Milvus address not configured; vector search requires --server <addr>")
		return &milvusVectorStore{logger: logger}
	}

	clientCfg := milvus.ClientConfig{
		Address:        fmt.Sprintf("%s:%d", milCfg.Address, milCfg.Port),
		Username:       milCfg.Username,
		Password:       milCfg.Password,
		ConnectTimeout: milCfg.ConnectTimeout,
	}
	milvusClient, err := milvus.NewClient(clientCfg, logger)
	if err != nil {
		logger.Warn("Milvus connection failed, vector search unavailable", logging.Err(err))
		return &milvusVectorStore{logger: logger}
	}

	collMgr := milvus.NewCollectionManager(milvusClient, milvus.CollectionConfig{}, logger)
	searcher := milvus.NewSearcher(milvusClient, collMgr, milvus.SearcherConfig{}, logger)

	return &milvusVectorStore{
		searcher:        searcher,
		logger:          logger,
		collectionName:  "molecules",
		vectorFieldName: "embedding",
	}
}

func (s *milvusVectorStore) SearchByVector(ctx context.Context, vector []float64, threshold float64, maxResults int, filters map[string]string) ([]patent_mining.SimilarityHit, error) {
	if s.searcher == nil {
		return nil, apperrors.NewMsg("vector search requires connection to the KeyIP API server or a local Milvus instance; use --server <addr>")
	}

	filterExpr := buildFilterExpr(filters)
	req := common.VectorSearchRequest{
		CollectionName:  s.collectionName,
		VectorFieldName: s.vectorFieldName,
		Vectors:         [][]float32{{}},
		TopK:            maxResults,
		Filters:         filterExpr,
		OutputFields:    []string{"molecule_id", "smiles", "name"},
	}
	req.Vectors[0] = float64sToFloat32s(vector)

	result, err := s.searcher.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	if len(result.Results) == 0 || len(result.Results[0]) == 0 {
		return []patent_mining.SimilarityHit{}, nil
	}

	hits := make([]patent_mining.SimilarityHit, 0, len(result.Results[0]))
	for _, vh := range result.Results[0] {
		score := float64(vh.Score)
		if score < threshold {
			continue
		}
		hit := patent_mining.SimilarityHit{
			Score:  score,
			Metric: patent_mining.MetricCosine,
		}
		if vh.Fields != nil {
			if id, ok := vh.Fields["molecule_id"].(string); ok {
				hit.ID = id
			}
			if smi, ok := vh.Fields["smiles"].(string); ok {
				hit.SMILES = smi
			}
			if name, ok := vh.Fields["name"].(string); ok {
				hit.Name = name
			}
		}
		if hit.ID == "" {
			hit.ID = fmt.Sprintf("milvus-%d", vh.ID)
		}
		hits = append(hits, hit)
	}
	return hits, nil
}

func (s *milvusVectorStore) EmbedText(ctx context.Context, text string, model string) ([]float64, error) {
	return nil, apperrors.NewMsg("text embedding requires connection to the KeyIP API server; use --server <addr>")
}

func (s *milvusVectorStore) EmbedMolecule(ctx context.Context, smiles string) ([]float64, error) {
	return nil, apperrors.NewMsg("molecule embedding requires connection to the KeyIP API server; use --server <addr>")
}

// ============================================================================
// localPatentIndex — implements patent_mining.PatentIndexForSearch.
// Maintains an in-memory map for locally-referenced patents and returns
// actionable errors for data that requires the API server.
// ============================================================================

type localPatentIndex struct {
	molecules map[string][]string // patentID -> SMILES list
	texts     map[string]string   // patentID -> patent text
	logger    logging.Logger
}

func newLocalPatentIndex(logger logging.Logger) *localPatentIndex {
	return &localPatentIndex{
		molecules: make(map[string][]string),
		texts:     make(map[string]string),
		logger:    logger,
	}
}

func (p *localPatentIndex) GetPatentMolecules(ctx context.Context, patentID string) ([]string, error) {
	if smiles, ok := p.molecules[patentID]; ok && len(smiles) > 0 {
		return smiles, nil
	}
	return nil, apperrors.NewMsg("patent molecule data requires the KeyIP API server; use --server <addr>")
}

func (p *localPatentIndex) GetPatentText(ctx context.Context, patentID string) (string, error) {
	if text, ok := p.texts[patentID]; ok && text != "" {
		return text, nil
	}
	return "", apperrors.NewMsg("patent text data requires the KeyIP API server; use --server <addr>")
}

func (p *localPatentIndex) SearchByText(ctx context.Context, query string, maxResults int) ([]patent_mining.SimilarityHit, error) {
	return nil, apperrors.NewMsg("full-text patent search requires the KeyIP API server; use --server <addr>")
}

// ============================================================================
// Helpers
// ============================================================================

func buildFilterExpr(filters map[string]string) string {
	if len(filters) == 0 {
		return ""
	}
	parts := make([]string, 0, len(filters))
	for k, v := range filters {
		parts = append(parts, fmt.Sprintf("%s == \"%s\"", k, v))
	}
	return strings.Join(parts, " && ")
}

func float64sToFloat32s(src []float64) []float32 {
	dst := make([]float32, len(src))
	for i, v := range src {
		dst[i] = float32(v)
	}
	return dst
}

//Personal.AI order the ending
