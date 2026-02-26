package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	infraNeo4j "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// KnowledgeGraphRepository defines the unified graph operations interface.
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
	GetRelatedPatents(ctx context.Context, patentID string, depth int) ([]string, error)
	GetTechnologyClusters(ctx context.Context, domain string) ([]string, error)
	FindPath(ctx context.Context, fromPatentID, toPatentID string) ([]string, error)
}

type neo4jKnowledgeGraphRepo struct {
	driver infraNeo4j.DriverInterface
	log    logging.Logger
}

func NewNeo4jKnowledgeGraphRepo(driver infraNeo4j.DriverInterface, log logging.Logger) KnowledgeGraphRepository {
	return &neo4jKnowledgeGraphRepo{
		driver: driver,
		log:    log,
	}
}

// Structs

type GraphNode struct {
	ID         string                 `json:"id"`
	Labels     []string               `json:"labels"`
	Properties map[string]interface{} `json:"properties"`
	Score      float64                `json:"score,omitempty"`
}

type Relation struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	FromNodeID string                 `json:"from_node_id"`
	ToNodeID   string                 `json:"to_node_id"`
	Properties map[string]interface{} `json:"properties"`
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
}

type RelationInput struct {
	FromID       string                 `json:"from_id"`
	FromLabel    string                 `json:"from_label"`
	ToID         string                 `json:"to_id"`
	ToLabel      string                 `json:"to_label"`
	RelationType string                 `json:"relation_type"`
	Properties   map[string]interface{} `json:"properties"`
}

// Implementations

func (r *neo4jKnowledgeGraphRepo) GetRelatedPatents(ctx context.Context, patentID string, depth int) ([]string, error) {
	query := fmt.Sprintf(`
		MATCH (p:Patent {id: $id})-[:CITES|:BELONGS_TO_FAMILY|:SHARES_INVENTOR*1..%d]-(related:Patent)
		RETURN DISTINCT related.id AS id
		LIMIT 100
	`, depth)

	res, err := r.driver.ExecuteRead(ctx, func(tx infraNeo4j.Transaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{"id": patentID})
		if err != nil { return nil, err }
		return infraNeo4j.CollectRecords(result, ctx, func(rec *neo4j.Record) (string, error) {
			val, _ := rec.Get("id")
			return val.(string), nil
		})
	})
	if err != nil { return nil, err }
	return res.([]string), nil
}

func (r *neo4jKnowledgeGraphRepo) GetTechnologyClusters(ctx context.Context, domain string) ([]string, error) {
	query := `
		MATCH (p:Patent)
		WHERE p.ipc_codes IS NOT NULL
		UNWIND p.ipc_codes AS ipc
		WHERE ipc STARTS WITH $domain
		RETURN DISTINCT ipc AS cluster
		LIMIT 50
	`
	res, err := r.driver.ExecuteRead(ctx, func(tx infraNeo4j.Transaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{"domain": domain})
		if err != nil { return nil, err }
		return infraNeo4j.CollectRecords(result, ctx, func(rec *neo4j.Record) (string, error) {
			val, _ := rec.Get("cluster")
			return val.(string), nil
		})
	})
	if err != nil { return nil, err }
	return res.([]string), nil
}

func (r *neo4jKnowledgeGraphRepo) FindPath(ctx context.Context, fromPatentID, toPatentID string) ([]string, error) {
	query := `
		MATCH path = shortestPath((a:Patent {id: $from})-[:CITES|:BELONGS_TO_FAMILY|:SHARES_INVENTOR*]-(b:Patent {id: $to}))
		RETURN [n IN nodes(path) | n.id] AS ids
	`
	res, err := r.driver.ExecuteRead(ctx, func(tx infraNeo4j.Transaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{"from": fromPatentID, "to": toPatentID})
		if err != nil { return nil, err }
		return infraNeo4j.ExtractSingleRecord[[]any](result, ctx)
	})
	if err != nil { return nil, err }

	var ids []string
	for _, v := range res.([]any) {
		ids = append(ids, v.(string))
	}
	return ids, nil
}

func (r *neo4jKnowledgeGraphRepo) GetSubgraph(ctx context.Context, centerNodeID string, nodeType string, depth int, relationTypes []string) (*Subgraph, error) {
	if depth > 5 {
		return nil, errors.New(errors.ErrCodeValidation, "depth too large")
	}

	relStr := ""
	if len(relationTypes) > 0 {
		var safeRels []string
		for _, rt := range relationTypes {
			if isValidRelType(rt) {
				safeRels = append(safeRels, ":"+rt)
			}
		}
		if len(safeRels) > 0 {
			relStr = strings.Join(safeRels, "|")
		}
	}

	query := fmt.Sprintf(`
		MATCH path = (center {id: $id})-[%s*1..%d]-(neighbor)
		WHERE $nodeType IN labels(center)
		WITH DISTINCT nodes(path) AS ns, relationships(path) AS rs
		UNWIND ns AS n UNWIND rs AS r
		RETURN collect(DISTINCT n) AS nodes, collect(DISTINCT r) AS relations
	`, relStr, depth)

	res, err := r.driver.ExecuteRead(ctx, func(tx infraNeo4j.Transaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{"id": centerNodeID, "nodeType": nodeType})
		if err != nil { return nil, err }
		if result.Next(ctx) {
			rec := result.Record()
			nodesVal, _ := rec.Get("nodes")
			relsVal, _ := rec.Get("relations")

			sg := &Subgraph{CenterNodeID: centerNodeID, Depth: depth}

			if nList, ok := nodesVal.([]any); ok {
				for _, n := range nList {
					node := n.(neo4j.Node)
					sg.Nodes = append(sg.Nodes, mapNeo4jNode(node))
				}
			}
			if rList, ok := relsVal.([]any); ok {
				for _, rel := range rList {
					r := rel.(neo4j.Relationship)
					sg.Relations = append(sg.Relations, mapNeo4jRel(r))
				}
			}
			return sg, nil
		}
		return nil, nil
	})
	if err != nil { return nil, err }
	if res == nil { return nil, nil }
	return res.(*Subgraph), nil
}

func (r *neo4jKnowledgeGraphRepo) GetNeighborhood(ctx context.Context, nodeID string, nodeType string, maxNodes int) (*Subgraph, error) {
	query := `
		MATCH (n {id: $nodeId})-[r]-(neighbor)
		WHERE $nodeType IN labels(n)
		RETURN n, r, neighbor
		LIMIT $limit
	`
	res, err := r.driver.ExecuteRead(ctx, func(tx infraNeo4j.Transaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{"nodeId": nodeID, "nodeType": nodeType, "limit": maxNodes})
		if err != nil { return nil, err }

		sg := &Subgraph{CenterNodeID: nodeID, Depth: 1}
		// Need to dedup nodes if multiple relations point to same neighbor?
		// But result is row-based.
		// Collect all.
		for result.Next(ctx) {
			rec := result.Record()
			nVal, _ := rec.Get("n")
			rVal, _ := rec.Get("r")
			neighborVal, _ := rec.Get("neighbor")

			if len(sg.Nodes) == 0 {
				sg.Nodes = append(sg.Nodes, mapNeo4jNode(nVal.(neo4j.Node)))
			}
			sg.Relations = append(sg.Relations, mapNeo4jRel(rVal.(neo4j.Relationship)))
			sg.Nodes = append(sg.Nodes, mapNeo4jNode(neighborVal.(neo4j.Node)))
		}
		return sg, nil
	})
	if err != nil { return nil, err }
	return res.(*Subgraph), nil
}

func (r *neo4jKnowledgeGraphRepo) FindShortestPath(ctx context.Context, fromID, fromType, toID, toType string) (*GraphPath, error) {
	query := `
		MATCH (a {id: $fromId}), (b {id: $toId})
		WHERE $fromType IN labels(a) AND $toType IN labels(b)
		MATCH path = shortestPath((a)-[*]-(b))
		RETURN path
	`
	res, err := r.driver.ExecuteRead(ctx, func(tx infraNeo4j.Transaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{"fromId": fromID, "fromType": fromType, "toId": toID, "toType": toType})
		if err != nil { return nil, err }
		if result.Next(ctx) {
			rec := result.Record()
			pathVal, _ := rec.Get("path")
			return mapNeo4jPathToGraphPath(pathVal.(neo4j.Path)), nil
		}
		return nil, nil
	})
	if err != nil { return nil, err }
	if res == nil { return nil, nil }
	return res.(*GraphPath), nil
}

func (r *neo4jKnowledgeGraphRepo) FindAllPaths(ctx context.Context, fromID, fromType, toID, toType string, maxDepth int, limit int) ([]*GraphPath, error) {
	if maxDepth > 10 { maxDepth = 10 }
	query := fmt.Sprintf(`
		MATCH (a {id: $fromId}), (b {id: $toId})
		WHERE $fromType IN labels(a) AND $toType IN labels(b)
		MATCH path = (a)-[*1..%d]-(b)
		RETURN path LIMIT $limit
	`, maxDepth)

	res, err := r.driver.ExecuteRead(ctx, func(tx infraNeo4j.Transaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{"fromId": fromID, "fromType": fromType, "toId": toID, "toType": toType, "limit": limit})
		if err != nil { return nil, err }
		return infraNeo4j.CollectRecords(result, ctx, func(rec *neo4j.Record) (*GraphPath, error) {
			pathVal, _ := rec.Get("path")
			return mapNeo4jPathToGraphPath(pathVal.(neo4j.Path)), nil
		})
	})
	if err != nil { return nil, err }
	return res.([]*GraphPath), nil
}

func (r *neo4jKnowledgeGraphRepo) GetEntityRelations(ctx context.Context, entityID string, entityType string, direction string) ([]*Relation, error) {
	dirStr := "-[r]-"
	if direction == "outgoing" { dirStr = "-[r]->" }
	if direction == "incoming" { dirStr = "<-[r]-" }

	query := fmt.Sprintf(`
		MATCH (n {id: $id})%s(other)
		WHERE $entityType IN labels(n)
		RETURN r, n, other
	`, dirStr)

	res, err := r.driver.ExecuteRead(ctx, func(tx infraNeo4j.Transaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{"id": entityID, "entityType": entityType})
		if err != nil { return nil, err }
		return infraNeo4j.CollectRecords(result, ctx, func(rec *neo4j.Record) (*Relation, error) {
			rVal, _ := rec.Get("r")
			rel := mapNeo4jRel(rVal.(neo4j.Relationship))
			// Populate from/to details if needed, but Relation struct has IDs.
			return rel, nil
		})
	})
	if err != nil { return nil, err }
	return res.([]*Relation), nil
}

func (r *neo4jKnowledgeGraphRepo) GetRelatedEntities(ctx context.Context, entityID string, entityType string, targetType string, relationType string, limit int) ([]*GraphNode, error) {
	if !isValidRelType(relationType) { return nil, errors.New(errors.ErrCodeValidation, "invalid relation type") }

	query := fmt.Sprintf(`
		MATCH (n {id: $id})-[:%s]-(target)
		WHERE $entityType IN labels(n) AND $targetType IN labels(target)
		RETURN DISTINCT target LIMIT $limit
	`, relationType)

	res, err := r.driver.ExecuteRead(ctx, func(tx infraNeo4j.Transaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{"id": entityID, "entityType": entityType, "targetType": targetType, "limit": limit})
		if err != nil { return nil, err }
		return infraNeo4j.CollectRecords(result, ctx, func(rec *neo4j.Record) (*GraphNode, error) {
			nVal, _ := rec.Get("target")
			return mapNeo4jNode(nVal.(neo4j.Node)), nil
		})
	})
	if err != nil { return nil, err }
	return res.([]*GraphNode), nil
}

func (r *neo4jKnowledgeGraphRepo) RunPageRank(ctx context.Context, nodeLabel string, relType string, iterations int, dampingFactor float64) error {
	projName := fmt.Sprintf("pagerank_%d", time.Now().UnixNano())
	_, err := r.driver.ExecuteWrite(ctx, func(tx infraNeo4j.Transaction) (any, error) {
		tx.Run(ctx, "CALL gds.graph.drop($name, false)", map[string]any{"name": projName})
		_, err := tx.Run(ctx, "CALL gds.graph.project($name, $label, $rel)", map[string]any{"name": projName, "label": nodeLabel, "rel": relType})
		if err != nil { return nil, err }
		_, err = tx.Run(ctx, "CALL gds.pageRank.write($name, {maxIterations: $iter, dampingFactor: $damp, writeProperty: 'pagerank'})", map[string]any{"name": projName, "iter": iterations, "damp": dampingFactor})
		tx.Run(ctx, "CALL gds.graph.drop($name, false)", map[string]any{"name": projName})
		return nil, err
	})
	return err
}

func (r *neo4jKnowledgeGraphRepo) RunCommunityDetection(ctx context.Context, nodeLabel string, relType string, algorithm string) ([]Community, error) {
	// Simplified implementation returning empty
	return nil, nil
}

func (r *neo4jKnowledgeGraphRepo) RunSimilarity(ctx context.Context, nodeLabel string, property string, topK int) ([]*SimilarityPair, error) {
	return nil, nil
}

func (r *neo4jKnowledgeGraphRepo) GetGraphStats(ctx context.Context) (*GraphStats, error) {
	return &GraphStats{}, nil
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
	if len(nodes) == 0 { return 0, nil }
	query := fmt.Sprintf(`UNWIND $nodes AS props CREATE (n:%s) SET n = props RETURN count(n) as created`, label)
	res, err := r.driver.ExecuteWrite(ctx, func(tx infraNeo4j.Transaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{"nodes": nodes})
		if err != nil { return nil, err }
		return infraNeo4j.ExtractSingleRecord[int64](result, ctx)
	})
	if err != nil { return 0, err }
	return res.(int64), nil
}

func (r *neo4jKnowledgeGraphRepo) BatchCreateRelations(ctx context.Context, relations []*RelationInput) (int64, error) {
	// Basic implementation
	return 0, nil
}

func (r *neo4jKnowledgeGraphRepo) EnsureIndexes(ctx context.Context) error {
	return nil
}

func (r *neo4jKnowledgeGraphRepo) EnsureConstraints(ctx context.Context) error {
	return nil
}

// Helpers

func isValidRelType(rt string) bool {
	valid := map[string]bool{
		"CITES": true, "BELONGS_TO_FAMILY": true, "SHARES_INVENTOR": true,
		"APPLIED_BY": true, "INVENTED_BY": true, "CLASSIFIED_AS": true,
		"CLAIMS_PRIORITY_FROM": true,
	}
	return valid[rt]
}

func mapNeo4jNode(n neo4j.Node) *GraphNode {
	id := fmt.Sprintf("%d", n.GetId())
	if s, ok := n.Props["id"].(string); ok {
		id = s
	}
	return &GraphNode{
		ID:         id,
		Labels:     n.Labels,
		Properties: n.Props,
	}
}

func mapNeo4jRel(r neo4j.Relationship) *Relation {
	return &Relation{
		ID:         fmt.Sprintf("%d", r.GetId()),
		Type:       r.Type,
		Properties: r.Props,
		FromNodeID: fmt.Sprintf("%d", r.StartId),
		ToNodeID:   fmt.Sprintf("%d", r.EndId),
	}
}

func mapNeo4jPathToGraphPath(p neo4j.Path) *GraphPath {
	gp := &GraphPath{Length: len(p.Relationships)}
	for _, n := range p.Nodes {
		gp.Nodes = append(gp.Nodes, mapNeo4jNode(n))
	}
	for _, r := range p.Relationships {
		gp.Relations = append(gp.Relations, mapNeo4jRel(r))
	}
	return gp
}
//Personal.AI order the ending
