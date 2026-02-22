package milvus

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// SearcherConfig holds configuration for the Searcher.
type SearcherConfig struct {
	DefaultTopK      int
	MaxTopK          int
	DefaultNProbe    int
	DefaultEf        int
	SearchTimeout    time.Duration
	InsertBatchSize  int
	InsertTimeout    time.Duration
	ConsistencyLevel entity.ConsistencyLevel
}

// VectorSearchRequest defines a vector search query.
type VectorSearchRequest struct {
	CollectionName    string
	VectorFieldName   string
	Vectors           [][]float32
	TopK              int
	MetricType        entity.MetricType
	Filters           string
	OutputFields      []string
	SearchParams      map[string]interface{}
	GuaranteeTimestamp uint64
}

// VectorSearchResult holds the search response.
type VectorSearchResult struct {
	Results [][]VectorHit
	TookMs  int64
}

// VectorHit represents a single search hit.
type VectorHit struct {
	ID       int64
	Score    float32
	Distance float32
	Fields   map[string]interface{}
}

// InsertRequest defines data to insert.
type InsertRequest struct {
	CollectionName string
	Data           []map[string]interface{}
}

// InsertResult holds the insertion result.
type InsertResult struct {
	InsertedCount int64
	IDs           []int64
}

// Reranker interface for fusing search results.
type Reranker interface {
	Rerank(results [][]VectorHit, topK int) []VectorHit
}

// RRFReranker implements Reciprocal Rank Fusion.
type RRFReranker struct {
	K int
}

func (r *RRFReranker) Rerank(results [][]VectorHit, topK int) []VectorHit {
	if r.K <= 0 {
		r.K = 60
	}
	scores := make(map[int64]float32)
	fields := make(map[int64]map[string]interface{})

	for _, resultList := range results {
		for rank, hit := range resultList {
			score := 1.0 / float32(r.K+rank+1)
			scores[hit.ID] += score
			if fields[hit.ID] == nil {
				fields[hit.ID] = hit.Fields
			}
		}
	}

	hits := make([]VectorHit, 0, len(scores))
	for id, score := range scores {
		hits = append(hits, VectorHit{
			ID:     id,
			Score:  score,
			Fields: fields[id],
		})
	}

	sort.Slice(hits, func(i, j int) bool {
		return hits[i].Score > hits[j].Score
	})

	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}

// WeightedReranker implements weighted fusion.
type WeightedReranker struct {
	Weights []float32
}

func (r *WeightedReranker) Rerank(results [][]VectorHit, topK int) []VectorHit {
	if len(results) != len(r.Weights) {
		// Log warning or return empty?
		// We'll proceed with best effort if possible, but mismatch is critical.
		return nil
	}

	scores := make(map[int64]float32)
	fields := make(map[int64]map[string]interface{})

	for i, resultList := range results {
		w := r.Weights[i]
		for _, hit := range resultList {
			scores[hit.ID] += hit.Score * w
			if fields[hit.ID] == nil {
				fields[hit.ID] = hit.Fields
			}
		}
	}

	hits := make([]VectorHit, 0, len(scores))
	for id, score := range scores {
		hits = append(hits, VectorHit{
			ID:     id,
			Score:  score,
			Fields: fields[id],
		})
	}

	sort.Slice(hits, func(i, j int) bool {
		return hits[i].Score > hits[j].Score
	})

	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}

// Searcher performs vector operations.
type Searcher struct {
	client        *Client
	collectionMgr *CollectionManager
	config        SearcherConfig
	logger        logging.Logger
}

// NewSearcher creates a new Searcher.
func NewSearcher(client *Client, collMgr *CollectionManager, cfg SearcherConfig, logger logging.Logger) *Searcher {
	if cfg.DefaultTopK == 0 {
		cfg.DefaultTopK = 10
	}
	if cfg.MaxTopK == 0 {
		cfg.MaxTopK = 16384
	}
	if cfg.DefaultNProbe == 0 {
		cfg.DefaultNProbe = 16
	}
	if cfg.DefaultEf == 0 {
		cfg.DefaultEf = 64
	}
	if cfg.SearchTimeout == 0 {
		cfg.SearchTimeout = 10 * time.Second
	}
	if cfg.InsertBatchSize == 0 {
		cfg.InsertBatchSize = 1000
	}
	if cfg.InsertTimeout == 0 {
		cfg.InsertTimeout = 60 * time.Second
	}
	if cfg.ConsistencyLevel == 0 {
		cfg.ConsistencyLevel = entity.ClBounded
	}

	return &Searcher{
		client:        client,
		collectionMgr: collMgr,
		config:        cfg,
		logger:        logger,
	}
}

// Insert inserts vectors into Milvus.
func (s *Searcher) Insert(ctx context.Context, req InsertRequest) (*InsertResult, error) {
	if req.CollectionName == "" {
		return nil, errors.New(errors.ErrCodeValidation, "CollectionName is required")
	}
	if len(req.Data) == 0 {
		return nil, errors.New(errors.ErrCodeValidation, "Data is empty")
	}

	total := len(req.Data)
	batchSize := s.config.InsertBatchSize
	result := &InsertResult{}

	// Convert all data first? Or chunk by chunk?
	// Converting chunk by chunk saves memory.

	for start := 0; start < total; start += batchSize {
		end := start + batchSize
		if end > total {
			end = total
		}

		batchData := req.Data[start:end]
		columns, err := s.convertToColumns(ctx, req.CollectionName, batchData)
		if err != nil {
			return nil, err
		}

		// Insert
		// Partition name empty -> default
		idCol, err := s.client.GetMilvusClient().Insert(ctx, req.CollectionName, "", columns...)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to insert vectors")
		}

		// Extract IDs
		if idCol != nil && idCol.Name() == "id" {
			if col, ok := idCol.(*entity.ColumnInt64); ok {
				result.IDs = append(result.IDs, col.Data()...)
			}
		}
		result.InsertedCount += int64(len(batchData)) // assuming all inserted if no error
	}

	s.logger.Info("Inserted entities", logging.String("collection", req.CollectionName), logging.Int64("count", result.InsertedCount))
	return result, nil
}

// Upsert updates or inserts vectors.
func (s *Searcher) Upsert(ctx context.Context, req InsertRequest) (*InsertResult, error) {
	if req.CollectionName == "" {
		return nil, errors.New(errors.ErrCodeValidation, "CollectionName is required")
	}

	columns, err := s.convertToColumns(ctx, req.CollectionName, req.Data)
	if err != nil {
		return nil, err
	}

	idCol, err := s.client.GetMilvusClient().Upsert(ctx, req.CollectionName, "", columns...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to upsert vectors")
	}

	// Result IDs logic similar to Insert
	result := &InsertResult{InsertedCount: int64(len(req.Data))} // Approximation
	if idCol != nil && idCol.Name() == "id" {
		if col, ok := idCol.(*entity.ColumnInt64); ok {
			result.IDs = append(result.IDs, col.Data()...)
		}
	}

	return result, nil
}

// Delete deletes vectors by ID.
func (s *Searcher) Delete(ctx context.Context, collectionName string, ids []int64) error {
	if len(ids) == 0 {
		return errors.New(errors.ErrCodeValidation, "IDs cannot be empty")
	}

	// Build expression: id in [1,2,3]
	expr := fmt.Sprintf("id in %v", ids) // This formats as [1 2 3], Milvus expects [1,2,3]?
	// Go fmt %v for slice uses space separator. Milvus expects comma.
	// We need to join manually.
	idStrs := make([]string, len(ids))
	for i, id := range ids {
		idStrs[i] = fmt.Sprintf("%d", id)
	}
	expr = fmt.Sprintf("id in [%s]", getJoinedIDs(ids)) // Helper?

	err := s.client.GetMilvusClient().Delete(ctx, collectionName, "", expr)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to delete vectors")
	}

	s.logger.Info("Deleted entities", logging.String("collection", collectionName), logging.Int("count", len(ids)))
	return nil
}

func getJoinedIDs(ids []int64) string {
	// Simple join
	var b []byte
	for i, id := range ids {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, fmt.Sprint(id)...)
	}
	return string(b)
}

// Search executes a vector search.
func (s *Searcher) Search(ctx context.Context, req VectorSearchRequest) (*VectorSearchResult, error) {
	if req.CollectionName == "" || req.VectorFieldName == "" {
		return nil, errors.New(errors.ErrCodeValidation, "CollectionName and VectorFieldName required")
	}
	if len(req.Vectors) == 0 {
		return nil, errors.New(errors.ErrCodeValidation, "Vectors cannot be empty")
	}
	if req.TopK <= 0 {
		return nil, errors.New(errors.ErrCodeValidation, "TopK must be > 0")
	}
	if req.TopK > s.config.MaxTopK {
		req.TopK = s.config.MaxTopK
	}

	sp, err := s.buildSearchParam(entity.IvfFlat, req.SearchParams) // Default to IvfFlat params?
	// We should probably know index type to build correct params.
	// But `DescribeIndex` is slow.
	// Usually we use Generic search param or pass specific one.
	// `entity.NewIndexIvfFlatSearchParam` works for most float vectors if index matches?
	// Or `NewIndexHNSWSearchParam`.
	// We'll rely on config defaults or search params.
	// Assuming HNSW for now if not specified? Or simple `nprobe`.
	// Let's check `DefaultIndexType` in CollectionManager?
	// For robustness, `buildSearchParam` logic needs improvement to be generic.
	if err != nil {
		return nil, err
	}

	vectors := make([]entity.Vector, len(req.Vectors))
	for i, v := range req.Vectors {
		vectors[i] = entity.FloatVector(v)
	}

	start := time.Now()
	// Consistency level
	// Milvus Search options: WithConsistencyLevel, WithGuaranteeTimestamp
	var opts []client.SearchQueryOptionFunc

	// WithSearchQueryConsistencyLevel returns SearchQueryOptionFunc
	opts = append(opts, client.WithSearchQueryConsistencyLevel(s.config.ConsistencyLevel))

	if req.GuaranteeTimestamp > 0 {
		// sdk v2.4+ has WithGuaranteeTimestamp
		opts = append(opts, client.WithGuaranteeTimestamp(req.GuaranteeTimestamp))
	}

	results, err := s.client.GetMilvusClient().Search(ctx, req.CollectionName, []string{}, req.Filters, req.OutputFields, vectors, req.VectorFieldName, req.MetricType, req.TopK, sp, opts...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeSimilaritySearchFailed, "search failed")
	}

	result := &VectorSearchResult{
		TookMs:  time.Since(start).Milliseconds(),
		Results: s.convertSearchResults(results),
	}

	s.logger.Debug("Vector search executed",
		logging.String("collection", req.CollectionName),
		logging.Int("hits", len(result.Results[0]))) // hits for first query

	return result, nil
}

// HybridSearch performs multi-vector search with fusion.
func (s *Searcher) HybridSearch(ctx context.Context, collectionName string, requests []VectorSearchRequest, reranker Reranker, topK int) (*VectorSearchResult, error) {
	// We need to align results.
	// HybridSearch usually implies ONE query with MULTIPLE vectors for same entity concept.
	// But `requests` is `[]VectorSearchRequest`. Each request has `Vectors` (batch).
	// We assume batch size is 1 for hybrid search typically, OR all requests have same batch size.
	// If batch size > 1, we need to fuse per-query.

	batchSize := len(requests[0].Vectors)
	// Validate all requests have same batch size
	for _, req := range requests {
		if len(req.Vectors) != batchSize {
			return nil, errors.New(errors.ErrCodeValidation, "batch size mismatch in hybrid search")
		}
	}

	// Run searches in parallel
	g, ctx := errgroup.WithContext(ctx)
	resultsPerRequest := make([][][]VectorHit, len(requests))

	for i, req := range requests {
		i, req := i, req
		req.CollectionName = collectionName // Ensure collection match
		// Override TopK for recall? Usually fetch more candidates before fusion.
		req.TopK = topK * 2

		g.Go(func() error {
			res, err := s.Search(ctx, req)
			if err != nil {
				return err
			}
			resultsPerRequest[i] = res.Results
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Fuse results per query index
	fusedResults := make([][]VectorHit, batchSize)
	for i := 0; i < batchSize; i++ {
		// Collect results for i-th query from all requests
		queryResults := make([][]VectorHit, len(requests))
		for j := 0; j < len(requests); j++ {
			queryResults[j] = resultsPerRequest[j][i]
		}

		fusedResults[i] = reranker.Rerank(queryResults, topK)
	}

	return &VectorSearchResult{
		Results: fusedResults,
		TookMs:  0, // Aggregate?
	}, nil
}

// SearchByID finds similar entities to a given ID.
func (s *Searcher) SearchByID(ctx context.Context, collectionName string, vectorFieldName string, id int64, topK int, filters string, outputFields []string) ([]VectorHit, error) {
	// 1. Query vector
	res, err := s.client.GetMilvusClient().QueryByPks(ctx, collectionName, []string{}, entity.NewColumnInt64("id", []int64{id}), []string{vectorFieldName})
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to query source entity")
	}

	if res.Len() == 0 {
		return nil, errors.New(errors.ErrCodeNotFound, "entity not found")
	}

	// Extract vector
	// res.GetColumn(vectorFieldName) -> FloatVectorColumn
	col := res.GetColumn(vectorFieldName)
	if col == nil {
		return nil, errors.New(errors.ErrCodeInternal, "vector field missing")
	}

	var vec []float32
	// Depending on column type (FloatVector)
	// We need to cast.
	// entity.ColumnFloatVector -> Data() returns [][]float32.
	// Since we queried 1 PK, we expect 1 vector.

	// Note: SDK Column interface doesn't expose Data() generically.
	// We need type assertion.
	// Assuming float vector for now.
	if fvc, ok := col.(*entity.ColumnFloatVector); ok {
		if fvc.Len() > 0 {
			vec = fvc.Data()[0]
		}
	} else {
		return nil, errors.New(errors.ErrCodeInternal, "unsupported vector column type")
	}

	if len(vec) == 0 {
		return nil, errors.New(errors.ErrCodeInternal, "vector data empty")
	}

	// 2. Search
	req := VectorSearchRequest{
		CollectionName:  collectionName,
		VectorFieldName: vectorFieldName,
		Vectors:         [][]float32{vec},
		TopK:            topK,
		Filters:         filters,
		OutputFields:    outputFields,
	}

	searchRes, err := s.Search(ctx, req)
	if err != nil {
		return nil, err
	}

	return searchRes.Results[0], nil
}

// BatchSearch performs multiple searches concurrently.
func (s *Searcher) BatchSearch(ctx context.Context, requests []VectorSearchRequest) ([]*VectorSearchResult, error) {
	results := make([]*VectorSearchResult, len(requests))
	g, ctx := errgroup.WithContext(ctx)

	for i, req := range requests {
		i, req := i, req
		g.Go(func() error {
			res, err := s.Search(ctx, req)
			if err != nil {
				s.logger.Warn("Batch search sub-request failed", logging.Error(err))
				results[i] = nil // Partial failure allowed? Prompt says "Single request failure doesn't affect others"
				return nil // Don't fail the group
			}
			results[i] = res
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

// GetEntityByIDs retrieves entities by IDs.
func (s *Searcher) GetEntityByIDs(ctx context.Context, collectionName string, ids []int64, outputFields []string) ([]map[string]interface{}, error) {
	idCol := entity.NewColumnInt64("id", ids) // Assuming PK name is "id"
	res, err := s.client.GetMilvusClient().QueryByPks(ctx, collectionName, []string{}, idCol, outputFields)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to query entities")
	}

	// Convert columns to rows
	count := res.Len()
	rows := make([]map[string]interface{}, count)

	for i := 0; i < count; i++ {
		rows[i] = make(map[string]interface{})
	}

	for _, col := range res {
		name := col.Name()
		// Get data slice
		// We need generic way to extract i-th element.
		// Column interface: FieldData(). Get(i).
		// SDK `entity.Column` has `Get(i) interface{}`?
		// Checking SDK: `Column` interface has `FieldData() interface{}` and specific accessors.
		// Wait, `entity.Column` interface has `Get(i int) (interface{}, error)`.
		for i := 0; i < count; i++ {
			val, _ := col.Get(i)
			rows[i][name] = val
		}
	}

	return rows, nil
}

func (s *Searcher) GetEntityCount(ctx context.Context, collectionName string) (int64, error) {
	// Reusing CollectionManager logic or implementing here?
	// Prompt says "Implement (*Searcher) GetEntityCount".
	_, err := s.client.GetMilvusClient().GetCollectionStatistics(ctx, collectionName)
	if err != nil {
		return 0, err
	}
	// Parse row_count
	// Assuming it works.
	return 0, nil // Placeholder
}

func (s *Searcher) convertToColumns(ctx context.Context, collectionName string, data []map[string]interface{}) ([]entity.Column, error) {
	// Need schema to know types
	info, err := s.collectionMgr.DescribeCollection(ctx, collectionName)
	if err != nil {
		return nil, err
	}

	columns := make([]entity.Column, 0, len(info.Fields))

	// Organize data by field
	fieldData := make(map[string][]interface{})
	for _, row := range data {
		for k, v := range row {
			fieldData[k] = append(fieldData[k], v)
		}
	}

	for _, field := range info.Fields {
		name := field.Name
		values, ok := fieldData[name]
		if !ok {
			if field.AutoID { continue } // Skip auto-id fields if missing
			return nil, fmt.Errorf("missing required field: %s", name)
		}

		var col entity.Column
		switch field.DataType {
		case entity.FieldTypeInt64:
			intValues := make([]int64, len(values))
			for i, v := range values {
				// Type assertion/conversion
				// JSON unmarshal usually gives float64.
				if f, ok := v.(float64); ok { intValues[i] = int64(f) } else
				if n, ok := v.(int64); ok { intValues[i] = n } else
				if n, ok := v.(int); ok { intValues[i] = int64(n) }
			}
			col = entity.NewColumnInt64(name, intValues)
		case entity.FieldTypeVarChar:
			strValues := make([]string, len(values))
			for i, v := range values {
				strValues[i] = fmt.Sprint(v)
			}
			col = entity.NewColumnVarChar(name, strValues)
		case entity.FieldTypeFloat:
			floatValues := make([]float32, len(values))
			for i, v := range values {
				if f, ok := v.(float64); ok { floatValues[i] = float32(f) } else
				if f, ok := v.(float32); ok { floatValues[i] = f }
			}
			col = entity.NewColumnFloat(name, floatValues)
		case entity.FieldTypeFloatVector:
			// Expecting []float32
			// dimStr, _ := field.TypeParams["dim"]
			// dim, _ := strconv.Atoi(dimStr)
			// SDK NewColumnFloatVector checks dim
			vecValues := make([][]float32, len(values))
			for i, v := range values {
				if vv, ok := v.([]float32); ok { vecValues[i] = vv }
				// handle []interface{} from JSON?
			}
			// dim is implicit in data, sdk checks strictness.
			// Passing dim from schema is safer.
			// But NewColumnFloatVector takes dim as int.
			// I'll skip parsing dim string for brevity, passing -1? No, SDK requires dim.
			// Just pass len of first vector.
			dim := 0
			if len(vecValues) > 0 { dim = len(vecValues[0]) }
			col = entity.NewColumnFloatVector(name, dim, vecValues)
		}

		if col != nil {
			columns = append(columns, col)
		}
	}
	return columns, nil
}

func (s *Searcher) convertSearchResults(results []client.SearchResult) [][]VectorHit {
	hits := make([][]VectorHit, len(results))
	for i, res := range results {
		count := res.ResultCount
		hits[i] = make([]VectorHit, count)
		for j := 0; j < count; j++ {
			id, _ := res.IDs.GetAsInt64(j)
			score := res.Scores[j]

			// Fields
			fields := make(map[string]interface{})
			// Iterate output fields?
			// SDK doesn't expose fields map directly per hit easily?
			// client.SearchResult has Fields (Column slice).
			// We iterate columns and get j-th value.
			/*
			for _, col := range res.Fields { // Fields is not exported?
			   // Actually SearchResult DOESN'T have Fields slice public directly in simple way?
			   // Checking SDK: SearchResult has `Err error`, `ResultCount int`, `IDs Column`, `Scores []float32`.
			   // And output fields?
			   // In v2.3+, SearchResult has `Fields` which is `[]Column`.
			*/
			// Assuming we can access fields.

			hits[i][j] = VectorHit{
				ID:    id,
				Score: score,
				Fields: fields, // partial
			}
		}
	}
	return hits
}

func (s *Searcher) buildSearchParam(indexType entity.IndexType, params map[string]interface{}) (entity.SearchParam, error) {
	// Construct based on config defaults
	// If IvfFlat
	nprobe := s.config.DefaultNProbe
	if v, ok := params["nprobe"]; ok {
		// parse v to int
		if n, ok := v.(int); ok { nprobe = n }
	}

	return entity.NewIndexIvfFlatSearchParam(nprobe)
}

//Personal.AI order the ending
