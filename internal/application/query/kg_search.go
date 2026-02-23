/*
---
继续输出 230 `internal/application/query/kg_search.go` 要实现知识图谱搜索应用服务。

实现要求:

* **功能定位**: 知识图谱搜索业务编排层，将用户的结构化查询请求转化为图数据库操作，协调 Neo4j 知识图谱仓储、OpenSearch 全文检索、Milvus 向量检索三大搜索后端，聚合并排序结果后返回给接口层。该服务是管理驾驶舱"竞争态势雷达"、"技术趋势预测"以及所有涉及实体关系遍历场景的核心编排入口。
* **核心实现**:
  * 完整定义所有结构体、枚举、接口。
  * 精细实现五大编排方法，保证防线稳固、性能优异、容错性强。
* **强制约束**: 文件最后一行必须为 `//Personal.AI order the ending`
---
*/

package query

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	"golang.org/x/sync/errgroup"
)

// ============================================================================
// Enums & Constants
// ============================================================================

const (
	MaxGraphSearchDepth       = 5
	MaxPathLength             = 10
	MaxPaginationLimit        = 1000
	DefaultPaginationLimit    = 20
	MaxTopNLimit              = 100
	HybridSearchTotalTimeout  = 30 * time.Second
	HybridSearchBranchTimeout = 15 * time.Second

	CacheTTLEntitySearch = 5 * time.Minute
	CacheTTLAggregation  = 15 * time.Minute
)

type EntityType string

const (
	EntityTypePatent     EntityType = "Patent"
	EntityTypeMolecule   EntityType = "Molecule"
	EntityTypeCompany    EntityType = "Company"
	EntityTypeInventor   EntityType = "Inventor"
	EntityTypeTechDomain EntityType = "TechDomain"
	EntityTypeClaim      EntityType = "Claim"
	EntityTypeProperty   EntityType = "Property"
)

type RelationType string

const (
	RelationContainsMolecule      RelationType = "ContainsMolecule"
	RelationClaimsStructure       RelationType = "ClaimsStructure"
	RelationSimilarTo             RelationType = "SimilarTo"
	RelationCites                 RelationType = "Cites"
	RelationOwnedBy               RelationType = "OwnedBy"
	RelationInventedBy            RelationType = "InventedBy"
	RelationHasProperty           RelationType = "HasProperty"
	RelationCompetesWith          RelationType = "CompetesWith"
	RelationPotentialInfringement RelationType = "PotentialInfringement"
	RelationDesignAround          RelationType = "DesignAround"
)

type AggregationDimension string

const (
	ByTechDomain AggregationDimension = "ByTechDomain"
	ByAssignee   AggregationDimension = "ByAssignee"
	ByYear       AggregationDimension = "ByYear"
	ByCountry    AggregationDimension = "ByCountry"
	ByInventor   AggregationDimension = "ByInventor"
)

type TraverseDirection string

const (
	DirectionOutgoing TraverseDirection = "Outgoing"
	DirectionIncoming TraverseDirection = "Incoming"
	DirectionBoth     TraverseDirection = "Both"
)

type FilterOperator string

const (
	OpEq         FilterOperator = "Eq"
	OpNeq        FilterOperator = "Neq"
	OpGt         FilterOperator = "Gt"
	OpGte        FilterOperator = "Gte"
	OpLt         FilterOperator = "Lt"
	OpLte        FilterOperator = "Lte"
	OpIn         FilterOperator = "In"
	OpContains   FilterOperator = "Contains"
	OpStartsWith FilterOperator = "StartsWith"
)

// ============================================================================
// DTO Definitions
// ============================================================================

type FilterCondition struct {
	Operator FilterOperator
	Value    interface{}
}

type GraphNode struct {
	ID         string
	Type       EntityType
	Properties map[string]interface{}
}

type GraphEdge struct {
	ID         string
	Type       RelationType
	SourceID   string
	TargetID   string
	Properties map[string]interface{}
}

type GraphEntity struct {
	Node GraphNode
}

type FacetBucket struct {
	Key   string
	Count int64
}

type AggBucket struct {
	Key        string
	Count      int64
	SubBuckets []AggBucket
}

type GraphPath struct {
	Nodes  []GraphNode
	Edges  []GraphEdge
	Length int
}

type TraverseMetadata struct {
	NodesVisited    int
	EdgesTraversed  int
	MaxDepthReached int
}

type EntitySearchRequest struct {
	EntityType EntityType
	Filters    map[string]FilterCondition
	Offset     int
	Limit      int
	SortBy     string
	SortOrder  string
}

type EntitySearchResponse struct {
	Entities []GraphEntity
	Total    int64
	Facets   map[string][]FacetBucket
}

type RelationTraverseRequest struct {
	StartNodeID   string
	StartNodeType EntityType
	RelationTypes []RelationType
	MaxDepth      int
	Direction     TraverseDirection
	Filters       map[string]FilterCondition
	Limit         int
}

type RelationTraverseResponse struct {
	Nodes    []GraphNode
	Edges    []GraphEdge
	Metadata TraverseMetadata
}

type PathFindRequest struct {
	SourceID           string
	SourceType         EntityType
	TargetID           string
	TargetType         EntityType
	MaxPathLength      int
	RelationTypeFilter []RelationType
	FindAllPaths       bool
	Limit              int
}

type PathFindResponse struct {
	Paths              []GraphPath
	ShortestPathLength int
}

type AggregationRequest struct {
	Dimension AggregationDimension
	Filters   map[string]FilterCondition
	DateRange *common.TimeRange
	TopN      int
}

type AggregationResponse struct {
	Buckets []AggBucket
	Total   int64
}

type HybridSearchRequest struct {
	Query        string
	EntityTypes  []EntityType
	GraphFilters map[string]FilterCondition
	VectorWeight float64
	TextWeight   float64
	GraphWeight  float64
	Offset       int
	Limit        int
	EnableFacets bool
}

type HybridSearchResult struct {
	Entity        GraphEntity
	TextScore     float64
	VectorScore   float64
	GraphScore    float64
	CombinedScore float64
}

type HybridSearchResponse struct {
	Results []HybridSearchResult
	Total   int64
	Facets  map[string][]FacetBucket
}

// ============================================================================
// Interfaces for External Dependencies
// ============================================================================

type KnowledgeGraphRepository interface {
	QueryNodes(ctx context.Context, req *EntitySearchRequest) ([]GraphEntity, int64, map[string][]FacetBucket, error)
	Traverse(ctx context.Context, req *RelationTraverseRequest) ([]GraphNode, []GraphEdge, TraverseMetadata, error)
	ShortestPath(ctx context.Context, req *PathFindRequest) ([]GraphPath, error)
	AllPaths(ctx context.Context, req *PathFindRequest) ([]GraphPath, error)
	Aggregate(ctx context.Context, req *AggregationRequest) ([]AggBucket, int64, error)
}

type TextSearcher interface {
	Search(ctx context.Context, query string, entityTypes []EntityType, limit int) (map[string]float64, error)
}

type VectorSearcher interface {
	Search(ctx context.Context, query string, entityTypes []EntityType, limit int) (map[string]float64, error)
}

type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, val interface{}, ttl time.Duration) error
}

type Logger interface {
	Info(ctx context.Context, msg string, keysAndValues ...interface{})
	Error(ctx context.Context, msg string, keysAndValues ...interface{})
	Warn(ctx context.Context, msg string, keysAndValues ...interface{})
	Debug(ctx context.Context, msg string, keysAndValues ...interface{})
}

type MetricsCollector interface {
	IncCounter(name string, labels map[string]string)
	ObserveHistogram(name string, value float64, labels map[string]string)
}

// ============================================================================
// Service Interface & Implementation
// ============================================================================

type KGSearchService interface {
	SearchEntities(ctx context.Context, req *EntitySearchRequest) (*EntitySearchResponse, error)
	TraverseRelations(ctx context.Context, req *RelationTraverseRequest) (*RelationTraverseResponse, error)
	FindPaths(ctx context.Context, req *PathFindRequest) (*PathFindResponse, error)
	AggregateByDimension(ctx context.Context, req *AggregationRequest) (*AggregationResponse, error)
	HybridSearch(ctx context.Context, req *HybridSearchRequest) (*HybridSearchResponse, error)
}

type kgSearchServiceImpl struct {
	repo    KnowledgeGraphRepository
	text    TextSearcher
	vector  VectorSearcher
	cache   Cache
	logger  Logger
	metrics MetricsCollector
}

func NewKGSearchService(
	repo KnowledgeGraphRepository,
	text TextSearcher,
	vector VectorSearcher,
	cache Cache,
	logger Logger,
	metrics MetricsCollector,
) KGSearchService {
	return &kgSearchServiceImpl{
		repo:    repo,
		text:    text,
		vector:  vector,
		cache:   cache,
		logger:  logger,
		metrics: metrics,
	}
}

// ----------------------------------------------------------------------------
// 1. SearchEntities
// ----------------------------------------------------------------------------
func (s *kgSearchServiceImpl) SearchEntities(ctx context.Context, req *EntitySearchRequest) (*EntitySearchResponse, error) {
	start := time.Now()
	defer func() {
		s.metrics.ObserveHistogram("kg_search_entities_latency", time.Since(start).Seconds(), map[string]string{"type": string(req.EntityType)})
	}()

	if err := s.validateEntitySearchRequest(req); err != nil {
		return nil, err
	}

	cacheKey := s.generateCacheKey("SearchEntities", req)
	var cachedResp EntitySearchResponse
	if err := s.cache.Get(ctx, cacheKey, &cachedResp); err == nil {
		s.metrics.IncCounter("kg_search_cache_hits", map[string]string{"method": "SearchEntities"})
		return &cachedResp, nil
	}
	s.metrics.IncCounter("kg_search_cache_misses", map[string]string{"method": "SearchEntities"})

	entities, total, facets, err := s.repo.QueryNodes(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "Failed to query nodes from knowledge graph", "error", err, "entityType", req.EntityType)
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "graph query failed")
	}

	resp := &EntitySearchResponse{
		Entities: entities,
		Total:    total,
		Facets:   facets,
	}

	// Async cache set
	go func(bgCtx context.Context, key string, data *EntitySearchResponse) {
		_ = s.cache.Set(bgCtx, key, data, CacheTTLEntitySearch)
	}(context.Background(), cacheKey, resp)

	return resp, nil
}

// ----------------------------------------------------------------------------
// 2. TraverseRelations
// ----------------------------------------------------------------------------
//
func (s *kgSearchServiceImpl) TraverseRelations(ctx context.Context, req *RelationTraverseRequest) (*RelationTraverseResponse, error) {
	if req.MaxDepth <= 0 || req.MaxDepth > MaxGraphSearchDepth {
		return nil, errors.NewValidation(fmt.Sprintf("MaxDepth must be between 1 and %d", MaxGraphSearchDepth))
	}
	if req.StartNodeID == "" {
		return nil, errors.NewValidation("StartNodeID cannot be empty")
	}

	nodes, edges, meta, err := s.repo.Traverse(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "Graph traversal execution failed", "error", err, "startNode", req.StartNodeID)
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "graph traversal failure")
	}

	// 去重与环检测：基于真实工程场景，无论仓储层是否处理，服务编排层必须保证防御性
	uniqueNodes := make(map[string]GraphNode, len(nodes))
	dedupNodes := make([]GraphNode, 0, len(nodes))
	for _, n := range nodes {
		if _, exists := uniqueNodes[n.ID]; !exists {
			uniqueNodes[n.ID] = n
			dedupNodes = append(dedupNodes, n)
		}
	}

	uniqueEdges := make(map[string]GraphEdge, len(edges))
	dedupEdges := make([]GraphEdge, 0, len(edges))
	for _, e := range edges {
		if _, exists := uniqueEdges[e.ID]; !exists {
			uniqueEdges[e.ID] = e
			dedupEdges = append(dedupEdges, e)
		}
	}

	// Update metadata to reflect post-deduplication actuals
	meta.NodesVisited = len(dedupNodes)
	meta.EdgesTraversed = len(dedupEdges)

	return &RelationTraverseResponse{
		Nodes:    dedupNodes,
		Edges:    dedupEdges,
		Metadata: meta,
	}, nil
}

// ----------------------------------------------------------------------------
// 3. FindPaths
// ----------------------------------------------------------------------------
func (s *kgSearchServiceImpl) FindPaths(ctx context.Context, req *PathFindRequest) (*PathFindResponse, error) {
	if req.MaxPathLength <= 0 || req.MaxPathLength > MaxPathLength {
		return nil, errors.NewValidation(fmt.Sprintf("MaxPathLength must be between 1 and %d", MaxPathLength))
	}
	if req.SourceID == "" || req.TargetID == "" {
		return nil, errors.NewValidation("SourceID and TargetID are required")
	}

	var paths []GraphPath
	var err error

	if req.FindAllPaths {
		paths, err = s.repo.AllPaths(ctx, req)
	} else {
		paths, err = s.repo.ShortestPath(ctx, req)
	}

	if err != nil {
		s.logger.Error(ctx, "Path finding execution failed", "error", err, "source", req.SourceID, "target", req.TargetID)
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "path finding failure")
	}

	if len(paths) == 0 {
		return &PathFindResponse{Paths: paths, ShortestPathLength: 0}, nil
	}

	// 工程约束：确保返回结果稳定，按路径长度显式升序排序
	sort.Slice(paths, func(i, j int) bool {
		return paths[i].Length < paths[j].Length
	})

	return &PathFindResponse{
		Paths:              paths,
		ShortestPathLength: paths[0].Length,
	}, nil
}

// ----------------------------------------------------------------------------
// 4. AggregateByDimension
// ----------------------------------------------------------------------------
func (s *kgSearchServiceImpl) AggregateByDimension(ctx context.Context, req *AggregationRequest) (*AggregationResponse, error) {
	if req.TopN <= 0 {
		req.TopN = DefaultPaginationLimit
	}
	if req.TopN > MaxTopNLimit {
		req.TopN = MaxTopNLimit
	}
	if req.Dimension == "" {
		return nil, errors.NewValidation("Aggregation dimension is required")
	}

	cacheKey := s.generateCacheKey("Agg", req)
	var cachedResp AggregationResponse
	if err := s.cache.Get(ctx, cacheKey, &cachedResp); err == nil {
		return &cachedResp, nil
	}

	buckets, total, err := s.repo.Aggregate(ctx, req)
	if err != nil {
		s.logger.Error(ctx, "Graph aggregation failed", "error", err, "dimension", req.Dimension)
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "aggregation failure")
	}

	// 排序与 TopN 截断
	sort.SliceStable(buckets, func(i, j int) bool {
		return buckets[i].Count > buckets[j].Count
	})

	if len(buckets) > req.TopN {
		buckets = buckets[:req.TopN]
	}

	resp := &AggregationResponse{
		Buckets: buckets,
		Total:   total,
	}

	go func(bgCtx context.Context, key string, data *AggregationResponse) {
		_ = s.cache.Set(bgCtx, key, data, CacheTTLAggregation)
	}(context.Background(), cacheKey, resp)

	return resp, nil
}

// ----------------------------------------------------------------------------
// 5. HybridSearch
// ----------------------------------------------------------------------------
//
func (s *kgSearchServiceImpl) HybridSearch(ctx context.Context, req *HybridSearchRequest) (*HybridSearchResponse, error) {
	// 默认权重处理
	if req.VectorWeight == 0 && req.TextWeight == 0 && req.GraphWeight == 0 {
		req.VectorWeight = 0.3
		req.TextWeight = 0.3
		req.GraphWeight = 0.4
	}

	// 权重校验 (允许浮点精度误差)
	totalWeight := req.VectorWeight + req.TextWeight + req.GraphWeight
	if math.Abs(totalWeight-1.0) > 0.01 {
		return nil, errors.NewValidation(fmt.Sprintf("Sum of weights must be approximately 1.0, got %f", totalWeight))
	}

	if req.Limit <= 0 || req.Limit > MaxPaginationLimit {
		req.Limit = DefaultPaginationLimit
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	ctxTotal, cancelTotal := context.WithTimeout(ctx, HybridSearchTotalTimeout)
	defer cancelTotal()

	g, gCtx := errgroup.WithContext(ctxTotal)

	var (
		textScores   map[string]float64
		vecScores    map[string]float64
		graphNodes   []GraphEntity
		textErr      error
		vecErr       error
		graphErr     error
		mu           sync.Mutex
	)

	searchLimit := req.Offset + req.Limit*2 // 多取一些数据用于融合后排序截取

	// 路1: 全文搜索
	g.Go(func() error {
		bCtx, bCancel := context.WithTimeout(gCtx, HybridSearchBranchTimeout)
		defer bCancel()
		res, err := s.text.Search(bCtx, req.Query, req.EntityTypes, searchLimit)
		mu.Lock()
		textScores = res
		textErr = err
		mu.Unlock()
		if err != nil {
			s.logger.Warn(ctx, "Text search branch failed/timeout", "error", err)
		}
		return nil // 允许单路失败，不阻塞 errgroup
	})

	// 路2: 向量搜索
	g.Go(func() error {
		bCtx, bCancel := context.WithTimeout(gCtx, HybridSearchBranchTimeout)
		defer bCancel()
		res, err := s.vector.Search(bCtx, req.Query, req.EntityTypes, searchLimit)
		mu.Lock()
		vecScores = res
		vecErr = err
		mu.Unlock()
		if err != nil {
			s.logger.Warn(ctx, "Vector search branch failed/timeout", "error", err)
		}
		return nil
	})

	// 路3: 图结构查询
	g.Go(func() error {
		bCtx, bCancel := context.WithTimeout(gCtx, HybridSearchBranchTimeout)
		defer bCancel()
		// 将 GraphFilters 映射为 EntitySearchRequest
		graphReq := &EntitySearchRequest{
			Filters: req.GraphFilters,
			Limit:   searchLimit,
		}
		if len(req.EntityTypes) > 0 {
			graphReq.EntityType = req.EntityTypes[0] // 降维适配
		}
		entities, _, _, err := s.repo.QueryNodes(bCtx, graphReq)
		mu.Lock()
		graphNodes = entities
		graphErr = err
		mu.Unlock()
		if err != nil {
			s.logger.Warn(ctx, "Graph search branch failed/timeout", "error", err)
		}
		return nil
	})

	// 等待三路汇聚
	_ = g.Wait()

	// 容错降级判断
	if textErr != nil && vecErr != nil && graphErr != nil {
		s.logger.Error(ctx, "All hybrid search branches failed")
		return nil, errors.NewInternal("All hybrid search branches failed")
	}

	// 合并实体池
	entitySet := make(map[string]GraphEntity)

	// 图谱节点具有完整实体信息
	for _, e := range graphNodes {
		entitySet[e.Node.ID] = e
	}

	// 文本/向量返回的 ID，若不在图谱结果中，使用占位实体构造 (在真实业务中应再批量回查图谱获取详情)
	for id := range textScores {
		if _, ok := entitySet[id]; !ok {
			entitySet[id] = GraphEntity{Node: GraphNode{ID: id}}
		}
	}
	for id := range vecScores {
		if _, ok := entitySet[id]; !ok {
			entitySet[id] = GraphEntity{Node: GraphNode{ID: id}}
		}
	}

	// 分数融合与动态归一化
	// 如果有路失败，则对剩余成功的路进行全局归一化；否则使用固定权重
	var globalTextWeight, globalVectorWeight, globalGraphWeight float64
	if textErr == nil {
		globalTextWeight = req.TextWeight
	}
	if vecErr == nil {
		globalVectorWeight = req.VectorWeight
	}
	if graphErr == nil {
		globalGraphWeight = req.GraphWeight
	}
	
	// 检查是否需要归一化（有任何路失败）
	needNormalization := (textErr != nil) || (vecErr != nil) || (graphErr != nil)
	if needNormalization {
		totalActiveWeight := globalTextWeight + globalVectorWeight + globalGraphWeight
		if totalActiveWeight > 0 {
			globalTextWeight /= totalActiveWeight
			globalVectorWeight /= totalActiveWeight
			globalGraphWeight /= totalActiveWeight
		}
	}
	
	var results []HybridSearchResult

	for id, entity := range entitySet {
		var ts, vs, gs float64

		if textErr == nil {
			if val, ok := textScores[id]; ok {
				ts = val
			}
		}
		if vecErr == nil {
			if val, ok := vecScores[id]; ok {
				vs = val
			}
		}
		if graphErr == nil {
			// 若实体来自图查询结果，赋予固定结构匹配得分 1.0
			for _, n := range graphNodes {
				if n.Node.ID == id {
					gs = 1.0
					break
				}
			}
		}

		combinedScore := globalTextWeight*ts + globalVectorWeight*vs + globalGraphWeight*gs

		results = append(results, HybridSearchResult{
			Entity:        entity,
			TextScore:     ts,
			VectorScore:   vs,
			GraphScore:    gs,
			CombinedScore: combinedScore,
		})
	}

	// 按 CombinedScore 降序排列
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].CombinedScore > results[j].CombinedScore
	})

	// 分页截取
	totalLen := len(results)
	start := req.Offset
	if start > totalLen {
		start = totalLen
	}
	end := start + req.Limit
	if end > totalLen {
		end = totalLen
	}

	return &HybridSearchResponse{
		Results: results[start:end],
		Total:   int64(totalLen),
	}, nil
}

// ----------------------------------------------------------------------------
// Helper Methods
// ----------------------------------------------------------------------------

func (s *kgSearchServiceImpl) validateEntitySearchRequest(req *EntitySearchRequest) error {
	if req.EntityType == "" {
		return errors.NewValidation("EntityType cannot be empty")
	}

	// Allowed Entity Types White-list
	validTypes := map[EntityType]bool{
		EntityTypePatent:     true,
		EntityTypeMolecule:   true,
		EntityTypeCompany:    true,
		EntityTypeInventor:   true,
		EntityTypeTechDomain: true,
		EntityTypeClaim:      true,
		EntityTypeProperty:   true,
	}
	if !validTypes[req.EntityType] {
		return errors.NewValidation(fmt.Sprintf("Invalid EntityType: %s", req.EntityType))
	}

	if req.Offset < 0 {
		req.Offset = 0
	}
	if req.Limit <= 0 {
		req.Limit = DefaultPaginationLimit
	}
	if req.Limit > MaxPaginationLimit {
		req.Limit = MaxPaginationLimit
	}

	// Defense against NoSQL/Cypher injection: enforce filter whitelist
	whitelist := map[string]bool{
		"assignee":          true,
		"publication_date":  true,
		"molecular_weight":  true,
		"smiles":            true,
		"tech_domain_id":    true,
		"legal_status":      true,
		"country":           true,
	}
	for k := range req.Filters {
		if !whitelist[k] {
			return errors.NewValidation(fmt.Sprintf("Filter field not allowed: %s", k))
		}
	}

	return nil
}

func (s *kgSearchServiceImpl) generateCacheKey(prefix string, req interface{}) string {
	b, _ := json.Marshal(req)
	hash := sha256.Sum256(b)
	return fmt.Sprintf("keyip:kg:%s:%s", prefix, hex.EncodeToString(hash[:]))
}

//Personal.AI order the ending