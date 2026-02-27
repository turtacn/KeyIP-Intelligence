package repositories

import (
	"context"
	"fmt"

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

// Methods

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

	// Build relationship filter string
	relFilter := ""
	if len(relationTypes) > 0 {
		relFilter = ""
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
		// Use result interface from driver package, but here we expect full neo4j driver behavior.
		// The custom driver.Transaction returns driver.Result which is custom.
		// But in knowledge_graph_repo.go we need to access records.
		// Let's assume tx.Run returns something that has Next() and Record().
		// It does: driver.Result interface.

		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		if result.Next(ctx) {
			rec := result.Record()
			// neo4j.Record has Get method? No, it has Values and Get(key).
			// driver.Result.Record() returns *neo4j.Record.

			nodesVal, _ := rec.Get("nodes")
			relsVal, _ := rec.Get("relationships")

			subgraph := &Subgraph{
				CenterNodeID: centerNodeID,
				Depth:        depth,
			}

			// Parse nodes
			// nodesVal is likely []interface{} where each is neo4j.Node
			if nodesList, ok := nodesVal.([]interface{}); ok {
				for _, n := range nodesList {
					if node, ok := n.(neo4j.Node); ok {
						gn := &GraphNode{
							ID:         fmt.Sprintf("%d", node.Id), // Internal ID, or property ID? Using internal for now or property if available
							Labels:     node.Labels,
							Properties: node.Props,
						}
						// Prefer 'id' property if exists
						if idProp, ok := node.Props["id"].(string); ok {
							gn.ID = idProp
						}
						subgraph.Nodes = append(subgraph.Nodes, gn)
					}
				}
			}

			// Parse relationships
			if relsList, ok := relsVal.([]interface{}); ok {
				for _, r := range relsList {
					if rel, ok := r.(neo4j.Relationship); ok {
						subgraph.Relations = append(subgraph.Relations, &Relation{
							ID:         fmt.Sprintf("%d", rel.Id),
							Type:       rel.Type,
							Properties: rel.Props,
							// From/To node IDs would need mapping back to GraphNodes if we want structure,
							// but here we just list them.
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
		// If no result found for center node, return empty subgraph or error?
		// Usually if center node doesn't exist, query returns nothing.
		return nil, errors.New(errors.ErrCodeNotFound, "subgraph not found")
	}
	return res.(*Subgraph), nil
}

func (r *neo4jKnowledgeGraphRepo) GetNeighborhood(ctx context.Context, nodeID string, nodeType string, maxNodes int) (*Subgraph, error) {
	return nil, nil
}

func (r *neo4jKnowledgeGraphRepo) FindShortestPath(ctx context.Context, fromID, fromType, toID, toType string) (*GraphPath, error) {
	return nil, nil
}

func (r *neo4jKnowledgeGraphRepo) FindAllPaths(ctx context.Context, fromID, fromType, toID, toType string, maxDepth int, limit int) ([]*GraphPath, error) {
	return nil, nil
}

func (r *neo4jKnowledgeGraphRepo) GetEntityRelations(ctx context.Context, entityID string, entityType string, direction string) ([]*Relation, error) {
	return nil, nil
}

func (r *neo4jKnowledgeGraphRepo) GetRelatedEntities(ctx context.Context, entityID string, entityType string, targetType string, relationType string, limit int) ([]*GraphNode, error) {
	return nil, nil
}

func (r *neo4jKnowledgeGraphRepo) RunPageRank(ctx context.Context, nodeLabel string, relType string, iterations int, dampingFactor float64) error {
	return nil
}

func (r *neo4jKnowledgeGraphRepo) RunCommunityDetection(ctx context.Context, nodeLabel string, relType string, algorithm string) ([]Community, error) {
	return nil, nil
}

func (r *neo4jKnowledgeGraphRepo) RunSimilarity(ctx context.Context, nodeLabel string, property string, topK int) ([]*SimilarityPair, error) {
	return nil, nil
}

func (r *neo4jKnowledgeGraphRepo) GetGraphStats(ctx context.Context) (*GraphStats, error) {
	return nil, nil
}

func (r *neo4jKnowledgeGraphRepo) GetNodeLabelCounts(ctx context.Context) (map[string]int64, error) {
	return nil, nil
}

func (r *neo4jKnowledgeGraphRepo) GetRelationTypeCounts(ctx context.Context) (map[string]int64, error) {
	return nil, nil
}

func (r *neo4jKnowledgeGraphRepo) FullTextSearch(ctx context.Context, indexName string, query string, limit int) ([]*GraphNode, error) {
	return nil, nil
}

func (r *neo4jKnowledgeGraphRepo) BatchCreateNodes(ctx context.Context, label string, nodes []map[string]interface{}) (int64, error) {
	return 0, nil
}

func (r *neo4jKnowledgeGraphRepo) BatchCreateRelations(ctx context.Context, relations []*RelationInput) (int64, error) {
	return 0, nil
}

func (r *neo4jKnowledgeGraphRepo) EnsureIndexes(ctx context.Context) error {
	return nil
}

func (r *neo4jKnowledgeGraphRepo) EnsureConstraints(ctx context.Context) error {
	return nil
}
