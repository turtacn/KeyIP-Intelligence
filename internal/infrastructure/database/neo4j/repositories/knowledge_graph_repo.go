package repositories

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	driver "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type KnowledgeGraphRepository interface {
	GetSubgraph(ctx context.Context, centerNodeID string, nodeType string, depth int, relationTypes []string) (*Subgraph, error)
	GetNeighborhood(ctx context.Context, nodeID string, nodeType string, maxNodes int) (*Subgraph, error)
	FindShortestPath(ctx context.Context, fromID, fromType, toID, toType string) (*GraphPath, error)
	FindAllPaths(ctx context.Context, fromID, fromType, toID, toType string, maxDepth int, limit int) ([]*GraphPath, error)
	GetEntityRelations(ctx context.Context, entityID string, entityType string, direction string) ([]*Relation, error)
	GetRelatedEntities(ctx context.Context, entityID string, entityType string, targetType string, relationType string, limit int) ([]*GraphNode, error)
	RunPageRank(ctx context.Context, nodeLabel string, relType string, iterations int, dampingFactor float64) error
	RunCommunityDetection(ctx context.Context, nodeLabel string, relType string, algorithm string) ([]Community, error)
	RunSimilarity(ctx context.Context, nodeLabel string, property string, topK int) ([]*SimilarityPair, error)
	GetGraphStats(ctx context.Context) (*GraphStats, error)
	GetNodeLabelCounts(ctx context.Context) (map[string]int64, error)
	GetRelationTypeCounts(ctx context.Context) (map[string]int64, error)
	FullTextSearch(ctx context.Context, indexName string, query string, limit int) ([]*GraphNode, error)
	BatchCreateNodes(ctx context.Context, label string, nodes []map[string]interface{}) (int64, error)
	BatchCreateRelations(ctx context.Context, relations []*RelationInput) (int64, error)
	EnsureIndexes(ctx context.Context) error
	EnsureConstraints(ctx context.Context) error
}

type neo4jKnowledgeGraphRepo struct {
	driver *driver.Driver
	log    logging.Logger
}

func NewNeo4jKnowledgeGraphRepo(d *driver.Driver, log logging.Logger) KnowledgeGraphRepository {
	return &neo4jKnowledgeGraphRepo{
		driver: d,
		log:    log,
	}
}

// Data structures
type GraphNode struct {
	ID         string                 `json:"id"`
	Labels     []string               `json:"labels"`
	Properties map[string]interface{} `json:"properties"`
	Score      float64                `json:"score,omitempty"`
}

type Relation struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	FromNode   *GraphNode             `json:"from_node"`
	ToNode     *GraphNode             `json:"to_node"`
	Properties map[string]interface{} `json:"properties"`
}

type RelationInput struct {
	FromID       string                 `json:"from_id"`
	FromLabel    string                 `json:"from_label"`
	ToID         string                 `json:"to_id"`
	ToLabel      string                 `json:"to_label"`
	RelationType string                 `json:"relation_type"`
	Properties   map[string]interface{} `json:"properties"`
}

type Subgraph struct {
	Nodes        []*GraphNode `json:"nodes"`
	Relations    []*Relation  `json:"relations"`
	CenterNodeID string       `json:"center_node_id"`
	Depth        int          `json:"depth"`
}

type GraphPath struct {
	Nodes     []*GraphNode `json:"nodes"`
	Relations []*Relation  `json:"relations"`
	Length    int          `json:"length"`
}

type Community struct {
	CommunityID         int64        `json:"community_id"`
	NodeCount           int64        `json:"node_count"`
	RepresentativeNodes []*GraphNode `json:"representative_nodes"`
}

type SimilarityPair struct {
	Node1ID string  `json:"node1_id"`
	Node2ID string  `json:"node2_id"`
	Score   float64 `json:"score"`
}

type GraphStats struct {
	TotalNodes     int64    `json:"total_nodes"`
	TotalRelations int64    `json:"total_relations"`
	Labels         []string `json:"labels"`
	RelationTypes  []string `json:"relation_types"`
	DatabaseSize   string   `json:"database_size"`
}

var (
	ErrGDSNotAvailable       = errors.New(errors.ErrCodeServiceUnavailable, "neo4j GDS plugin is not available")
	ErrInvalidArgument       = errors.New(errors.ErrCodeValidation, "invalid argument")
	ErrGraphProjectionFailed = errors.New(errors.ErrCodeInternal, "graph projection failed")
)

// safeLabel sanitizes a label/type string for use in Cypher (prevents injection).
var validLabelRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func safeLabel(s string) string {
	if validLabelRe.MatchString(s) {
		return s
	}
	return ""
}

// ---------------------------------------------------------------------------
// Subgraph
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) GetSubgraph(ctx context.Context, centerNodeID string, nodeType string, depth int, relationTypes []string) (*Subgraph, error) {
	if depth > 5 {
		return nil, ErrInvalidArgument
	}

	query := `
		MATCH (start)
		WHERE (start.id = $centerId OR start.patent_number = $centerId)
		CALL apoc.path.subgraphAll(start, {
			maxLevel: $depth,
			relationshipFilter: $relFilter
		})
		YIELD nodes, relationships
		RETURN nodes, relationships
	`

	relFilter := ""
	if len(relationTypes) > 0 {
		for i, rt := range relationTypes {
			if i > 0 {
				relFilter += "|"
			}
			relFilter += rt
		}
	}

	params := map[string]interface{}{
		"centerId":  centerNodeID,
		"depth":     depth,
		"relFilter": relFilter,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		if result.Next(ctx) {
			rec := result.Record()
			nodesVal, _ := rec.Get("nodes")
			relsVal, _ := rec.Get("relationships")

			subgraph := &Subgraph{
				CenterNodeID: centerNodeID,
				Depth:        depth,
			}

			if nodesList, ok := nodesVal.([]interface{}); ok {
				for _, n := range nodesList {
					if node, ok := n.(neo4j.Node); ok {
						gn := &GraphNode{
							ID:         fmt.Sprintf("%d", node.Id),
							Labels:     node.Labels,
							Properties: node.Props,
						}
						if idProp, ok := node.Props["id"].(string); ok {
							gn.ID = idProp
						}
						subgraph.Nodes = append(subgraph.Nodes, gn)
					}
				}
			}

			if relsList, ok := relsVal.([]interface{}); ok {
				for _, r := range relsList {
					if rel, ok := r.(neo4j.Relationship); ok {
						subgraph.Relations = append(subgraph.Relations, &Relation{
							ID:         fmt.Sprintf("%d", rel.Id),
							Type:       rel.Type,
							Properties: rel.Props,
						})
					}
				}
			}
			return subgraph, nil
		}
		return nil, nil
	})

	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errors.New(errors.ErrCodeNotFound, "subgraph not found")
	}
	return res.(*Subgraph), nil
}

// ---------------------------------------------------------------------------
// Neighborhood
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) GetNeighborhood(ctx context.Context, nodeID string, nodeType string, maxNodes int) (*Subgraph, error) {
	if nodeID == "" {
		return nil, ErrInvalidArgument
	}
	if maxNodes <= 0 {
		maxNodes = 50
	}

	// Use nodeType as a label hint when provided
	var query string
	params := map[string]interface{}{
		"nodeId":   nodeID,
		"maxNodes": maxNodes,
	}

	if nodeType != "" && safeLabel(nodeType) != "" {
		query = fmt.Sprintf(`
			MATCH (n:%s)
			WHERE (n.id = $nodeId OR n.patent_number = $nodeId)
			OPTIONAL MATCH (n)-[r]-(connected:%s)
			WITH n, collect(DISTINCT r) AS rels, collect(DISTINCT connected) AS connectedNodes
			RETURN n, rels, connectedNodes
		`, safeLabel(nodeType), safeLabel(nodeType))
	} else {
		query = `
			MATCH (n)
			WHERE (n.id = $nodeId OR n.patent_number = $nodeId)
			OPTIONAL MATCH (n)-[r]-(connected)
			WITH n, collect(DISTINCT r) AS rels, collect(DISTINCT connected) AS connectedNodes
			RETURN n, rels, connectedNodes
		`
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			rec := result.Record()
			subgraph := &Subgraph{
				CenterNodeID: nodeID,
				Depth:        1,
			}
			if nVal, ok := rec.Get("n"); ok && nVal != nil {
				if node, ok := nVal.(neo4j.Node); ok {
					subgraph.Nodes = append(subgraph.Nodes, neo4jNodeToGraphNode(node))
					subgraph.CenterNodeID = extractGraphNodeID(node)
				}
			}
			if relsVal, ok := rec.Get("rels"); ok && relsVal != nil {
				if relsList, ok := relsVal.([]interface{}); ok {
					for _, r := range relsList {
						if rel, ok := r.(neo4j.Relationship); ok {
							subgraph.Relations = append(subgraph.Relations, &Relation{
								ID:         fmt.Sprintf("%d", rel.Id),
								Type:       rel.Type,
								Properties: rel.Props,
							})
						}
					}
				}
			}
			if connVal, ok := rec.Get("connectedNodes"); ok && connVal != nil {
				if connList, ok := connVal.([]interface{}); ok {
					for _, c := range connList {
						if node, ok := c.(neo4j.Node); ok {
							subgraph.Nodes = append(subgraph.Nodes, neo4jNodeToGraphNode(node))
						}
					}
				}
			}
			return subgraph, nil
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errors.New(errors.ErrCodeNotFound, "center node not found")
	}
	return res.(*Subgraph), nil
}

// ---------------------------------------------------------------------------
// Path finding
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) FindShortestPath(ctx context.Context, fromID, fromType, toID, toType string) (*GraphPath, error) {
	if fromID == "" || toID == "" {
		return nil, ErrInvalidArgument
	}

	// Build label-specific or generic match patterns
	fromLabel := safeLabel(fromType)
	toLabel := safeLabel(toType)

	var query string
	if fromLabel != "" && toLabel != "" {
		query = fmt.Sprintf(`
			MATCH (a:%s), (b:%s)
			WHERE (a.id = $fromId OR a.patent_number = $fromId)
			  AND (b.id = $toId OR b.patent_number = $toId)
			MATCH path = shortestPath((a)-[*]-(b))
			RETURN path
		`, fromLabel, toLabel)
	} else {
		query = `
			MATCH (a), (b)
			WHERE (a.id = $fromId OR a.patent_number = $fromId)
			  AND (b.id = $toId OR b.patent_number = $toId)
			MATCH path = shortestPath((a)-[*]-(b))
			RETURN path
		`
	}

	params := map[string]interface{}{
		"fromId": fromID,
		"toId":   toID,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			rec := result.Record()
			return buildGraphPathFromRecord(rec, "path")
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errors.New(errors.ErrCodeNotFound, "no path found")
	}
	return res.(*GraphPath), nil
}

func (r *neo4jKnowledgeGraphRepo) FindAllPaths(ctx context.Context, fromID, fromType, toID, toType string, maxDepth int, limit int) ([]*GraphPath, error) {
	if fromID == "" || toID == "" {
		return nil, ErrInvalidArgument
	}
	if maxDepth <= 0 || maxDepth > 10 {
		maxDepth = 5
	}
	if limit <= 0 {
		limit = 10
	}

	fromLabel := safeLabel(fromType)
	toLabel := safeLabel(toType)

	var query string
	if fromLabel != "" && toLabel != "" {
		query = fmt.Sprintf(`
			MATCH (a:%s), (b:%s)
			WHERE (a.id = $fromId OR a.patent_number = $fromId)
			  AND (b.id = $toId OR b.patent_number = $toId)
			MATCH path = (a)-[*1..$maxDepth]-(b)
			RETURN path
			LIMIT $limit
		`, fromLabel, toLabel)
	} else {
		query = `
			MATCH (a), (b)
			WHERE (a.id = $fromId OR a.patent_number = $fromId)
			  AND (b.id = $toId OR b.patent_number = $toId)
			MATCH path = (a)-[*1..$maxDepth]-(b)
			RETURN path
			LIMIT $limit
		`
	}

	params := map[string]interface{}{
		"fromId":   fromID,
		"toId":     toID,
		"maxDepth": maxDepth,
		"limit":    limit,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var paths []*GraphPath
		for result.Next(ctx) {
			rec := result.Record()
			gp, err := buildGraphPathFromRecord(rec, "path")
			if err != nil {
				return nil, err
			}
			paths = append(paths, gp)
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		if paths == nil {
			paths = []*GraphPath{}
		}
		return paths, nil
	})
	if err != nil {
		return nil, err
	}
	return res.([]*GraphPath), nil
}

// ---------------------------------------------------------------------------
// Entity relations
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) GetEntityRelations(ctx context.Context, entityID string, entityType string, direction string) ([]*Relation, error) {
	if entityID == "" {
		return nil, ErrInvalidArgument
	}

	label := safeLabel(entityType)
	var query string
	params := map[string]interface{}{
		"entityId": entityID,
	}

	dir := strings.ToUpper(direction)
	switch dir {
	case "INCOMING", "IN":
		if label != "" {
			query = fmt.Sprintf(`
				MATCH (n:%s)
				WHERE (n.id = $entityId OR n.patent_number = $entityId)
				MATCH (connected)-[r]->(n)
				RETURN r, connected AS from, n AS to
			`, label)
		} else {
			query = `
				MATCH (n)
				WHERE (n.id = $entityId OR n.patent_number = $entityId)
				MATCH (connected)-[r]->(n)
				RETURN r, connected AS from, n AS to
			`
		}
	case "OUTGOING", "OUT":
		if label != "" {
			query = fmt.Sprintf(`
				MATCH (n:%s)
				WHERE (n.id = $entityId OR n.patent_number = $entityId)
				MATCH (n)-[r]->(connected)
				RETURN r, n AS from, connected AS to
			`, label)
		} else {
			query = `
				MATCH (n)
				WHERE (n.id = $entityId OR n.patent_number = $entityId)
				MATCH (n)-[r]->(connected)
				RETURN r, n AS from, connected AS to
			`
		}
	default: // BOTH
		if label != "" {
			query = fmt.Sprintf(`
				MATCH (n:%s)
				WHERE (n.id = $entityId OR n.patent_number = $entityId)
				MATCH (n)-[r]-(connected)
				RETURN r, connected AS from, n AS to
			`, label)
		} else {
			query = `
				MATCH (n)
				WHERE (n.id = $entityId OR n.patent_number = $entityId)
				MATCH (n)-[r]-(connected)
				RETURN r, connected AS from, n AS to
			`
		}
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var relations []*Relation
		for result.Next(ctx) {
			rec := result.Record()
			rel := &Relation{}

			if rVal, ok := rec.Get("r"); ok && rVal != nil {
				if relData, ok := rVal.(neo4j.Relationship); ok {
					rel.ID = fmt.Sprintf("%d", relData.Id)
					rel.Type = relData.Type
					rel.Properties = relData.Props
				}
			}
			if fromVal, ok := rec.Get("from"); ok && fromVal != nil {
				if fromNode, ok := fromVal.(neo4j.Node); ok {
					rel.FromNode = neo4jNodeToGraphNode(fromNode)
				}
			}
			if toVal, ok := rec.Get("to"); ok && toVal != nil {
				if toNode, ok := toVal.(neo4j.Node); ok {
					rel.ToNode = neo4jNodeToGraphNode(toNode)
				}
			}
			relations = append(relations, rel)
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		if relations == nil {
			relations = []*Relation{}
		}
		return relations, nil
	})
	if err != nil {
		return nil, err
	}
	return res.([]*Relation), nil
}

// ---------------------------------------------------------------------------
// Related entities
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) GetRelatedEntities(ctx context.Context, entityID string, entityType string, targetType string, relationType string, limit int) ([]*GraphNode, error) {
	if entityID == "" {
		return nil, ErrInvalidArgument
	}
	if limit <= 0 {
		limit = 20
	}

	label := safeLabel(entityType)
	targetLabel := safeLabel(targetType)
	relType := safeLabel(relationType)

	params := map[string]interface{}{
		"entityId": entityID,
		"limit":    limit,
	}

	var query string
	switch {
	case label != "" && targetLabel != "" && relType != "":
		query = fmt.Sprintf(`
			MATCH (n:%s)
			WHERE (n.id = $entityId OR n.patent_number = $entityId)
			MATCH (n)-[r:%s]-(target:%s)
			RETURN DISTINCT target
			LIMIT $limit
		`, label, relType, targetLabel)
	case label != "" && targetLabel != "":
		query = fmt.Sprintf(`
			MATCH (n:%s)
			WHERE (n.id = $entityId OR n.patent_number = $entityId)
			MATCH (n)-[r]-(target:%s)
			RETURN DISTINCT target
			LIMIT $limit
		`, label, targetLabel)
	case label != "" && relType != "":
		query = fmt.Sprintf(`
			MATCH (n:%s)
			WHERE (n.id = $entityId OR n.patent_number = $entityId)
			MATCH (n)-[r:%s]-(target)
			RETURN DISTINCT target
			LIMIT $limit
		`, label, relType)
	case targetLabel != "" && relType != "":
		query = fmt.Sprintf(`
			MATCH (n)
			WHERE (n.id = $entityId OR n.patent_number = $entityId)
			MATCH (n)-[r:%s]-(target:%s)
			RETURN DISTINCT target
			LIMIT $limit
		`, relType, targetLabel)
	default:
		query = `
			MATCH (n)
			WHERE (n.id = $entityId OR n.patent_number = $entityId)
			MATCH (n)-[r]-(target)
			RETURN DISTINCT target
			LIMIT $limit
		`
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var nodes []*GraphNode
		for result.Next(ctx) {
			rec := result.Record()
			if tVal, ok := rec.Get("target"); ok && tVal != nil {
				if tNode, ok := tVal.(neo4j.Node); ok {
					nodes = append(nodes, neo4jNodeToGraphNode(tNode))
				}
			}
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		if nodes == nil {
			nodes = []*GraphNode{}
		}
		return nodes, nil
	})
	if err != nil {
		return nil, err
	}
	return res.([]*GraphNode), nil
}

// ---------------------------------------------------------------------------
// GDS PageRank
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) RunPageRank(ctx context.Context, nodeLabel string, relType string, iterations int, dampingFactor float64) error {
	if nodeLabel == "" || safeLabel(nodeLabel) == "" {
		return ErrInvalidArgument
	}
	if iterations <= 0 {
		iterations = 20
	}
	if dampingFactor <= 0 {
		dampingFactor = 0.85
	}
	return r.runKGPageRank(ctx, nodeLabel, relType, iterations, dampingFactor, false)
}

func (r *neo4jKnowledgeGraphRepo) runKGPageRank(ctx context.Context, nodeLabel, relType string, iterations int, dampingFactor float64, isRetry bool) error {
	safeNodeLabel := safeLabel(nodeLabel)
	safeRelType := safeLabel(relType)
	graphName := fmt.Sprintf("pagerank-%s-%s", safeNodeLabel, safeRelType)

	var relFilter string
	if safeRelType != "" {
		relFilter = fmt.Sprintf(`{ %s: { orientation: 'NATURAL' } }`, safeRelType)
	} else {
		relFilter = "'*'"
	}

	query := fmt.Sprintf(`
		CALL gds.graph.project(
			$graphName,
			$nodeLabel,
			%s
		)
		YIELD graphName, nodeCount, relationshipCount
		WITH graphName
		CALL gds.pageRank.write(graphName, {
			maxIterations: $iterations,
			dampingFactor: $dampingFactor,
			writeProperty: 'pagerank'
		})
		YIELD nodePropertiesWritten, ranIterations
		RETURN nodePropertiesWritten, ranIterations
	`, relFilter)

	params := map[string]interface{}{
		"graphName":     graphName,
		"nodeLabel":     safeNodeLabel,
		"iterations":    iterations,
		"dampingFactor": dampingFactor,
	}

	_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		if err != nil {
			if isGDSNotFound(err) {
				return nil, ErrGDSNotAvailable
			}
			return nil, err
		}
		return nil, nil
	})

	if err != nil {
		if isGDSNotAvailableError(err) {
			return ErrGDSNotAvailable
		}
		rawCause := unwrapDeep(err)
		if !isRetry && isGraphAlreadyExistsError(rawCause) {
			r.dropGraphProjection(ctx, graphName)
			return r.runKGPageRank(ctx, nodeLabel, relType, iterations, dampingFactor, true)
		}
		return err
	}
	return nil
}

func (r *neo4jKnowledgeGraphRepo) dropGraphProjection(ctx context.Context, graphName string) {
	dropQuery := `CALL gds.graph.drop($graphName) YIELD graphName RETURN graphName`
	_, _ = r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, dropQuery, map[string]interface{}{"graphName": graphName})
		return nil, err
	})
}

// ---------------------------------------------------------------------------
// GDS Community Detection
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) RunCommunityDetection(ctx context.Context, nodeLabel string, relType string, algorithm string) ([]Community, error) {
	if nodeLabel == "" || safeLabel(nodeLabel) == "" {
		return nil, ErrInvalidArgument
	}

	safeNodeLabel := safeLabel(nodeLabel)
	safeRelType := safeLabel(relType)
	graphName := "community-" + safeNodeLabel
	writeProperty := "community"

	var relFilter string
	if safeRelType != "" {
		relFilter = fmt.Sprintf(`{ %s: { orientation: 'NATURAL' } }`, safeRelType)
	} else {
		relFilter = "'*'"
	}

	// Determine the GDS procedure name based on the algorithm parameter
	var algoProc string
	switch strings.ToLower(algorithm) {
	case "lpa", "labelpropagation", "label_propagation":
		algoProc = "gds.labelPropagation.write"
	case "louvain":
		algoProc = "gds.louvain.write"
	case "wcc", "weakly", "weakly_connected":
		algoProc = "gds.wcc.write"
	default:
		algoProc = "gds.labelPropagation.write"
	}

	projectQuery := fmt.Sprintf(`
		CALL gds.graph.project(
			$graphName,
			$nodeLabel,
			%s
		)
		YIELD graphName
	`, relFilter)

	algoQuery := fmt.Sprintf(`
		CALL %s($graphName, {
			writeProperty: $writeProperty
		})
		YIELD communityCount
		RETURN communityCount
	`, algoProc)

	readQuery := fmt.Sprintf(`
		MATCH (n:%s)
		WHERE n[$writeProperty] IS NOT NULL
		WITH n[$writeProperty] AS community_id, collect(n) AS nodes, count(*) AS node_count
		RETURN community_id, node_count,
		       [x IN nodes[0..3] | {id: coalesce(x.id, toString(id(x))), labels: labels(x), properties: x{.*}}] AS samples
		ORDER BY node_count DESC
	`, safeNodeLabel)

	type intermediateResult struct {
		communityCount int64
	}

	// Attempt projection
	_, projErr := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, projectQuery, map[string]interface{}{
			"graphName": graphName,
			"nodeLabel": safeNodeLabel,
		})
		if err != nil {
			if isGDSNotFound(err) {
				return nil, ErrGDSNotAvailable
			}
			return nil, err
		}
		return nil, nil
	})
	if projErr != nil {
		if isGDSNotAvailableError(projErr) {
			return nil, ErrGDSNotAvailable
		}
		// If projection already exists, try to drop and retry
		if !isRetryableProjectionError(projErr) {
			return nil, projErr
		}
		r.dropGraphProjection(ctx, graphName)
	}

	// Run algorithm
	var communityCount int64
	_, algoErr := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, algoQuery, map[string]interface{}{
			"graphName":     graphName,
			"writeProperty": writeProperty,
		})
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			rec := result.Record()
			if cc, ok := rec.Get("communityCount"); ok && cc != nil {
				communityCount = toInt64(cc)
			}
		}
		return communityCount, result.Err()
	})
	if algoErr != nil {
		// If GDS error, cleanup and return
		r.dropGraphProjection(ctx, graphName)
		return nil, algoErr
	}

	// Read communities
	res, readErr := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, readQuery, map[string]interface{}{
			"writeProperty": writeProperty,
		})
		if err != nil {
			return nil, err
		}

		var communities []Community
		for result.Next(ctx) {
			rec := result.Record()
			c := Community{}
			if cid, ok := rec.Get("community_id"); ok && cid != nil {
				c.CommunityID = toInt64(cid)
			}
			if nc, ok := rec.Get("node_count"); ok && nc != nil {
				c.NodeCount = toInt64(nc)
			}
			if samples, ok := rec.Get("samples"); ok && samples != nil {
				if sampleList, ok := samples.([]interface{}); ok {
					for _, s := range sampleList {
						if sMap, ok := s.(map[string]interface{}); ok {
							gn := &GraphNode{}
							if id, ok := sMap["id"].(string); ok {
								gn.ID = id
							}
							if labels, ok := sMap["labels"].([]interface{}); ok {
								for _, l := range labels {
									if ls, ok := l.(string); ok {
										gn.Labels = append(gn.Labels, ls)
									}
								}
							}
							if props, ok := sMap["properties"].(map[string]interface{}); ok {
								gn.Properties = props
							}
							c.RepresentativeNodes = append(c.RepresentativeNodes, gn)
						}
					}
				}
			}
			communities = append(communities, c)
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return communities, nil
	})
	if readErr != nil {
		r.dropGraphProjection(ctx, graphName)
		return nil, readErr
	}

	// Clean up projection
	r.dropGraphProjection(ctx, graphName)
	return res.([]Community), nil
}

func isRetryableProjectionError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*errors.AppError)
	if !ok {
		return false
	}
	return isGraphAlreadyExistsError(unwrapDeep(err))
}

// ---------------------------------------------------------------------------
// GDS Similarity
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) RunSimilarity(ctx context.Context, nodeLabel string, property string, topK int) ([]*SimilarityPair, error) {
	if nodeLabel == "" || safeLabel(nodeLabel) == "" {
		return nil, ErrInvalidArgument
	}
	if property == "" {
		return nil, ErrInvalidArgument
	}
	if topK <= 0 {
		topK = 10
	}

	safeNodeLabel := safeLabel(nodeLabel)
	graphName := "similarity-" + safeNodeLabel

	projectQuery := fmt.Sprintf(`
		CALL gds.graph.project($graphName, $nodeLabel, '*')
		YIELD graphName
	`)

	algoQuery := `
		CALL gds.nodeSimilarity.write($graphName, {
			topK: $topK,
			similarityCutoff: 0.0,
			writeProperty: 'similarity'
		})
		YIELD nodesCompared, relationshipsWritten
		RETURN nodesCompared, relationshipsWritten
	`

	readQuery := fmt.Sprintf(`
		MATCH (a:%s)-[r:SIMILAR]-(b:%s)
		RETURN a.id AS node1, b.id AS node2, r.similarity AS score
		ORDER BY score DESC
		LIMIT $limit
	`, safeNodeLabel, safeNodeLabel)

	// Project
	_, projErr := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, projectQuery, map[string]interface{}{
			"graphName": graphName,
			"nodeLabel": safeNodeLabel,
		})
		if err != nil {
			if isGDSNotFound(err) {
				return nil, ErrGDSNotAvailable
			}
			return nil, err
		}
		return nil, nil
	})
	if projErr != nil {
		if isGDSNotAvailableError(projErr) {
			return nil, ErrGDSNotAvailable
		}
		if isRetryableProjectionError(projErr) {
			r.dropGraphProjection(ctx, graphName)
		} else {
			return nil, projErr
		}
	}

	// Run similarity
	_, algoErr := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, algoQuery, map[string]interface{}{
			"graphName": graphName,
			"topK":      topK,
		})
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			_ = result.Record() // consume result
		}
		return nil, result.Err()
	})
	if algoErr != nil {
		r.dropGraphProjection(ctx, graphName)
		return nil, algoErr
	}

	// Read results
	res, readErr := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, readQuery, map[string]interface{}{
			"limit": topK * 100, // read enough results
		})
		if err != nil {
			return nil, err
		}

		var pairs []*SimilarityPair
		for result.Next(ctx) {
			rec := result.Record()
			pair := &SimilarityPair{}
			if n1, ok := rec.Get("node1"); ok && n1 != nil {
				pair.Node1ID = fmt.Sprintf("%v", n1)
			}
			if n2, ok := rec.Get("node2"); ok && n2 != nil {
				pair.Node2ID = fmt.Sprintf("%v", n2)
			}
			if sc, ok := rec.Get("score"); ok && sc != nil {
				pair.Score = toFloat64(sc)
			}
			pairs = append(pairs, pair)
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		if pairs == nil {
			pairs = []*SimilarityPair{}
		}
		return pairs, nil
	})
	if readErr != nil {
		r.dropGraphProjection(ctx, graphName)
		return nil, readErr
	}

	// Clean up and return
	r.dropGraphProjection(ctx, graphName)
	return res.([]*SimilarityPair), nil
}

// ---------------------------------------------------------------------------
// Graph stats
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) GetGraphStats(ctx context.Context) (*GraphStats, error) {
	stats := &GraphStats{}

	// Node count
	_, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, "MATCH (n) RETURN count(n) AS total", nil)
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			rec := result.Record()
			if t, ok := rec.Get("total"); ok && t != nil {
				stats.TotalNodes = toInt64(t)
			}
		}
		return nil, result.Err()
	})
	if err != nil {
		return nil, err
	}

	// Relation count
	_, err = r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, "MATCH ()-[r]->() RETURN count(r) AS total", nil)
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			rec := result.Record()
			if t, ok := rec.Get("total"); ok && t != nil {
				stats.TotalRelations = toInt64(t)
			}
		}
		return nil, result.Err()
	})
	if err != nil {
		return nil, err
	}

	// Labels
	_, err = r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, "CALL db.labels() YIELD label RETURN label ORDER BY label", nil)
		if err != nil {
			return nil, err
		}
		for result.Next(ctx) {
			rec := result.Record()
			if l, ok := rec.Get("label"); ok && l != nil {
				if label, ok := l.(string); ok {
					stats.Labels = append(stats.Labels, label)
				}
			}
		}
		return nil, result.Err()
	})
	if err != nil {
		return nil, err
	}

	// Relation types
	_, err = r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, "CALL db.relationshipTypes() YIELD relationshipType RETURN relationshipType ORDER BY relationshipType", nil)
		if err != nil {
			return nil, err
		}
		for result.Next(ctx) {
			rec := result.Record()
			if rt, ok := rec.Get("relationshipType"); ok && rt != nil {
				if relType, ok := rt.(string); ok {
					stats.RelationTypes = append(stats.RelationTypes, relType)
				}
			}
		}
		return nil, result.Err()
	})
	if err != nil {
		return nil, err
	}

	// Store database identifier (size not easily accessible via Cypher)
	stats.DatabaseSize = fmt.Sprintf("%d nodes, %d relations", stats.TotalNodes, stats.TotalRelations)

	if stats.Labels == nil {
		stats.Labels = []string{}
	}
	if stats.RelationTypes == nil {
		stats.RelationTypes = []string{}
	}
	return stats, nil
}

// ---------------------------------------------------------------------------
// Node label counts
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) GetNodeLabelCounts(ctx context.Context) (map[string]int64, error) {
	query := `
		MATCH (n)
		UNWIND labels(n) AS label
		RETURN label, count(*) AS cnt
		ORDER BY label
	`
	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}
		counts := make(map[string]int64)
		for result.Next(ctx) {
			rec := result.Record()
			if l, ok := rec.Get("label"); ok && l != nil {
				if label, ok := l.(string); ok {
					if c, ok := rec.Get("cnt"); ok && c != nil {
						counts[label] = toInt64(c)
					}
				}
			}
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return counts, nil
	})
	if err != nil {
		return nil, err
	}
	return res.(map[string]int64), nil
}

// ---------------------------------------------------------------------------
// Relation type counts
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) GetRelationTypeCounts(ctx context.Context) (map[string]int64, error) {
	query := `
		MATCH ()-[r]->()
		RETURN type(r) AS relType, count(*) AS cnt
		ORDER BY relType
	`
	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}
		counts := make(map[string]int64)
		for result.Next(ctx) {
			rec := result.Record()
			if rt, ok := rec.Get("relType"); ok && rt != nil {
				if relType, ok := rt.(string); ok {
					if c, ok := rec.Get("cnt"); ok && c != nil {
						counts[relType] = toInt64(c)
					}
				}
			}
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return counts, nil
	})
	if err != nil {
		return nil, err
	}
	return res.(map[string]int64), nil
}

// ---------------------------------------------------------------------------
// Full-text search
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) FullTextSearch(ctx context.Context, indexName string, query string, limit int) ([]*GraphNode, error) {
	if indexName == "" || query == "" {
		return nil, ErrInvalidArgument
	}
	if limit <= 0 {
		limit = 20
	}

	q := `
		CALL db.index.fulltext.queryNodes($indexName, $query)
		YIELD node, score
		RETURN node, score
		LIMIT $limit
	`
	params := map[string]interface{}{
		"indexName": indexName,
		"query":     query,
		"limit":     limit,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, q, params)
		if err != nil {
			return nil, err
		}

		var nodes []*GraphNode
		for result.Next(ctx) {
			rec := result.Record()
			gn := &GraphNode{}
			if nVal, ok := rec.Get("node"); ok && nVal != nil {
				if node, ok := nVal.(neo4j.Node); ok {
					gn = neo4jNodeToGraphNode(node)
				}
			}
			if scoreVal, ok := rec.Get("score"); ok && scoreVal != nil {
				gn.Score = toFloat64(scoreVal)
			}
			nodes = append(nodes, gn)
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		if nodes == nil {
			nodes = []*GraphNode{}
		}
		return nodes, nil
	})
	if err != nil {
		return nil, err
	}
	return res.([]*GraphNode), nil
}

// ---------------------------------------------------------------------------
// Batch create nodes
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) BatchCreateNodes(ctx context.Context, label string, nodes []map[string]interface{}) (int64, error) {
	if len(nodes) == 0 {
		return 0, nil
	}
	safeNodeLabel := safeLabel(label)
	if safeNodeLabel == "" {
		return 0, ErrInvalidArgument
	}

	query := fmt.Sprintf(`
		UNWIND $batch AS props
		CREATE (n:%s)
		SET n = props
		RETURN count(*) AS created
	`, safeNodeLabel)

	res, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, map[string]interface{}{"batch": nodes})
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			rec := result.Record()
			if c, ok := rec.Get("created"); ok && c != nil {
				return toInt64(c), nil
			}
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return int64(0), nil
	})
	if err != nil {
		return 0, err
	}
	return res.(int64), nil
}

// ---------------------------------------------------------------------------
// Batch create relations
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) BatchCreateRelations(ctx context.Context, relations []*RelationInput) (int64, error) {
	if len(relations) == 0 {
		return 0, nil
	}

	// Validate all labels and relation types upfront
	for _, rel := range relations {
		if safeLabel(rel.FromLabel) == "" || safeLabel(rel.ToLabel) == "" || safeLabel(rel.RelationType) == "" {
			return 0, ErrInvalidArgument
		}
	}

	// Use the first entry's labels/type (assumes homogeneous batch)
	fromLabel := safeLabel(relations[0].FromLabel)
	toLabel := safeLabel(relations[0].ToLabel)
	relType := safeLabel(relations[0].RelationType)

	query := fmt.Sprintf(`
		UNWIND $batch AS row
		MATCH (a:%s {id: row.fromId})
		MATCH (b:%s {id: row.toId})
		CREATE (a)-[r:%s]->(b)
		SET r += row.properties
		RETURN count(*) AS created
	`, fromLabel, toLabel, relType)

	var batch []map[string]interface{}
	for _, rel := range relations {
		batch = append(batch, map[string]interface{}{
			"fromId":     rel.FromID,
			"toId":       rel.ToID,
			"properties": rel.Properties,
		})
	}

	res, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, map[string]interface{}{"batch": batch})
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			rec := result.Record()
			if c, ok := rec.Get("created"); ok && c != nil {
				return toInt64(c), nil
			}
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return int64(0), nil
	})
	if err != nil {
		return 0, err
	}
	return res.(int64), nil
}

// ---------------------------------------------------------------------------
// Indexes and constraints
// ---------------------------------------------------------------------------

func (r *neo4jKnowledgeGraphRepo) EnsureIndexes(ctx context.Context) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS FOR (p:Patent) ON (p.id)",
		"CREATE INDEX IF NOT EXISTS FOR (p:Patent) ON (p.patent_number)",
		"CREATE INDEX IF NOT EXISTS FOR (p:Patent) ON (p.jurisdiction)",
		"CREATE INDEX IF NOT EXISTS FOR (f:PatentFamily) ON (f.family_id)",
	}

	for _, idx := range indexes {
		_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
			_, err := tx.Run(ctx, idx, nil)
			return nil, err
		})
		if err != nil {
			r.log.Warn("Failed to create index", logging.String("index", idx), logging.Err(err))
		}
	}
	return nil
}

func (r *neo4jKnowledgeGraphRepo) EnsureConstraints(ctx context.Context) error {
	constraints := []string{
		"CREATE CONSTRAINT IF NOT EXISTS FOR (p:Patent) REQUIRE p.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (f:PatentFamily) REQUIRE f.family_id IS UNIQUE",
	}

	for _, c := range constraints {
		_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
			_, err := tx.Run(ctx, c, nil)
			return nil, err
		})
		if err != nil {
			r.log.Warn("Failed to create constraint", logging.String("constraint", c), logging.Err(err))
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers for GraphNode / GraphPath
// ---------------------------------------------------------------------------

// neo4jNodeToGraphNode converts a neo4j.Node to a GraphNode.
func neo4jNodeToGraphNode(node neo4j.Node) *GraphNode {
	gn := &GraphNode{
		ID:         fmt.Sprintf("%d", node.Id),
		Labels:     node.Labels,
		Properties: node.Props,
	}
	if idProp, ok := node.Props["id"].(string); ok {
		gn.ID = idProp
	}
	return gn
}

// extractGraphNodeID extracts the preferred ID from a neo4j.Node.
func extractGraphNodeID(node neo4j.Node) string {
	if id, ok := node.Props["id"].(string); ok {
		return id
	}
	return fmt.Sprintf("%d", node.Id)
}

// buildGraphPathFromRecord constructs a GraphPath from a record containing a
// neo4j.Path under the given key.
func buildGraphPathFromRecord(rec *neo4j.Record, key string) (*GraphPath, error) {
	pathVal, ok := rec.Get(key)
	if !ok || pathVal == nil {
		return nil, fmt.Errorf("%s not found in record", key)
	}
	path, ok := pathVal.(neo4j.Path)
	if !ok {
		return nil, fmt.Errorf("%s is not a neo4j.Path", key)
	}

	gp := &GraphPath{
		Length: len(path.Relationships),
	}

	for _, n := range path.Nodes {
		gp.Nodes = append(gp.Nodes, neo4jNodeToGraphNode(n))
	}
	for _, rel := range path.Relationships {
		graphRel := &Relation{
			ID:         fmt.Sprintf("%d", rel.Id),
			Type:       rel.Type,
			Properties: rel.Props,
		}
		// Attach from/to nodes by matching internal IDs to path nodes
		for _, n := range path.Nodes {
			if n.Id == rel.StartId {
				graphRel.FromNode = neo4jNodeToGraphNode(n)
			}
			if n.Id == rel.EndId {
				graphRel.ToNode = neo4jNodeToGraphNode(n)
			}
		}
		gp.Relations = append(gp.Relations, graphRel)
	}
	return gp, nil
}

// Compile-time interface check
var _ KnowledgeGraphRepository = (*neo4jKnowledgeGraphRepo)(nil)
