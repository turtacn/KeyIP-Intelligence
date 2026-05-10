package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/citation"
	driver "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type neo4jCitationRepo struct {
	driver *driver.Driver
	log    logging.Logger
}

func NewNeo4jCitationRepo(d *driver.Driver, log logging.Logger) citation.CitationRepository {
	return &neo4jCitationRepo{
		driver: d,
		log:    log,
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// neo4jNodeToPatentNode converts a neo4j.Node to a domain PatentNode, extracting
// properties with best-effort type assertions.
func neo4jNodeToPatentNode(n neo4j.Node) *citation.PatentNode {
	pn := &citation.PatentNode{}
	if id, ok := n.Props["id"].(string); ok {
		if parsed, err := uuid.Parse(id); err == nil {
			pn.ID = parsed
		}
	}
	if num, ok := n.Props["patent_number"].(string); ok {
		pn.PatentNumber = num
	}
	if j, ok := n.Props["jurisdiction"].(string); ok {
		pn.Jurisdiction = j
	}
	if fd, ok := n.Props["filing_date"]; ok && fd != nil {
		pn.FilingDate = extractTime(fd)
	}
	if ca, ok := n.Props["created_at"]; ok && ca != nil {
		pn.CreatedAt = extractTimeValue(ca)
	}
	return pn
}

// extractTime attempts to convert a neo4j property value to *time.Time.
func extractTime(v interface{}) *time.Time {
	switch val := v.(type) {
	case neo4j.Date:
		t := val.Time()
		return &t
	case time.Time:
		return &val
	case string:
		if t, err := time.Parse("2006-01-02", val); err == nil {
			return &t
		}
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			return &t
		}
	}
	return nil
}

// extractTimeValue converts a neo4j property value to time.Time (default to zero).
// Cypher DateTime values are mapped by the driver to Go time.Time directly.
// Cypher Date values are mapped to neo4j.Date.
func extractTimeValue(v interface{}) time.Time {
	switch val := v.(type) {
	case time.Time:
		return val
	case neo4j.Date:
		return val.Time()
	case string:
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			return t
		}
	}
	return time.Time{}
}

// neo4jPathToCitationPath converts a neo4j.Path into a domain CitationPath.
func neo4jPathToCitationPath(path neo4j.Path) *citation.CitationPath {
	nodes := make([]*citation.PatentNode, 0, len(path.Nodes))
	for _, n := range path.Nodes {
		nodes = append(nodes, neo4jNodeToPatentNode(n))
	}
	rels := make([]string, 0, len(path.Relationships))
	for _, r := range path.Relationships {
		rels = append(rels, r.Type)
	}
	return &citation.CitationPath{
		Nodes:     nodes,
		Relations: rels,
		Length:    len(path.Relationships),
	}
}

// ---------------------------------------------------------------------------
// Node management
// ---------------------------------------------------------------------------

func (r *neo4jCitationRepo) EnsurePatentNode(ctx context.Context, patentID uuid.UUID, patentNumber string, jurisdiction string, filingDate *time.Time) error {
	query := `
		MERGE (p:Patent {id: $id})
		ON CREATE SET p.patent_number = $number, p.jurisdiction = $jurisdiction, p.filing_date = $filingDate, p.created_at = datetime()
		ON MATCH SET p.patent_number = $number, p.jurisdiction = $jurisdiction
	`
	params := map[string]interface{}{
		"id":           patentID.String(),
		"number":       patentNumber,
		"jurisdiction": jurisdiction,
	}
	if filingDate != nil {
		params["filingDate"] = neo4j.DateOf(*filingDate)
	} else {
		params["filingDate"] = nil
	}

	_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

func (r *neo4jCitationRepo) BatchEnsurePatentNodes(ctx context.Context, patents []*citation.PatentNodeData) error {
	if len(patents) == 0 {
		return nil
	}
	query := `
		UNWIND $batch AS row
		MERGE (p:Patent {id: row.id})
		ON CREATE SET p.patent_number = row.number, p.jurisdiction = row.jurisdiction, p.filing_date = date(row.filing_date), p.created_at = datetime()
		ON MATCH SET p.patent_number = row.number, p.jurisdiction = row.jurisdiction
	`
	var batch []map[string]interface{}
	for _, p := range patents {
		row := map[string]interface{}{
			"id":           p.ID.String(),
			"number":       p.PatentNumber,
			"jurisdiction": p.Jurisdiction,
		}
		if p.FilingDate != nil {
			row["filing_date"] = p.FilingDate.Format("2006-01-02")
		}
		batch = append(batch, row)
	}

	_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, map[string]interface{}{"batch": batch})
		return nil, err
	})
	return err
}

// ---------------------------------------------------------------------------
// Citation edge management
// ---------------------------------------------------------------------------

func (r *neo4jCitationRepo) CreateCitation(ctx context.Context, fromPatentID, toPatentID uuid.UUID, citationType string, metadata map[string]interface{}) error {
	query := `
		MATCH (a:Patent {id: $fromId}), (b:Patent {id: $toId})
		MERGE (a)-[r:CITES {type: $type}]->(b)
		ON CREATE SET r.created_at = datetime(), r += $metadata
	`
	params := map[string]interface{}{
		"fromId":   fromPatentID.String(),
		"toId":     toPatentID.String(),
		"type":     citationType,
		"metadata": metadata,
	}
	_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		summary, err := result.Consume(ctx)
		if err == nil && summary.Counters().RelationshipsCreated() == 0 {
			// Already existing.
		}
		return nil, err
	})
	return err
}

func (r *neo4jCitationRepo) DeleteCitation(ctx context.Context, fromPatentID, toPatentID uuid.UUID, citationType string) error {
	query := `
		MATCH (a:Patent {id: $fromId})-[r:CITES {type: $type}]->(b:Patent {id: $toId})
		DELETE r
	`
	params := map[string]interface{}{
		"fromId": fromPatentID.String(),
		"toId":   toPatentID.String(),
		"type":   citationType,
	}
	_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

func (r *neo4jCitationRepo) BatchCreateCitations(ctx context.Context, citations []*citation.CitationEdge) error {
	if len(citations) == 0 {
		return nil
	}
	query := `
		UNWIND $batch AS row
		MATCH (a:Patent {id: row.fromId}), (b:Patent {id: row.toId})
		MERGE (a)-[r:CITES {type: row.type}]->(b)
		ON CREATE SET r.created_at = datetime()
		FOREACH (ignore IN CASE WHEN row.metadata IS NOT NULL THEN [1] ELSE [] END |
			SET r += row.metadata
		)
	`
	var batch []map[string]interface{}
	for _, c := range citations {
		row := map[string]interface{}{
			"fromId": c.FromPatentID.String(),
			"toId":   c.ToPatentID.String(),
			"type":   c.Type,
		}
		if c.Metadata != nil {
			row["metadata"] = c.Metadata
		} else {
			row["metadata"] = nil
		}
		batch = append(batch, row)
	}

	_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, map[string]interface{}{"batch": batch})
		return nil, err
	})
	return err
}

// ---------------------------------------------------------------------------
// Forward citations — patents that cite *this* patent (incoming CITES)
// ---------------------------------------------------------------------------

func (r *neo4jCitationRepo) GetForwardCitations(ctx context.Context, patentID uuid.UUID, depth int, limit int) ([]*citation.CitationPath, error) {
	if depth <= 0 {
		return []*citation.CitationPath{}, nil
	}
	if depth > 5 {
		depth = 5
	}
	query := `
		MATCH (p:Patent {id: $id})
		MATCH path = (p)<-[:CITES*1..$depth]-(citing:Patent)
		RETURN path
		LIMIT $limit
	`
	params := map[string]interface{}{
		"id":    patentID.String(),
		"depth": depth,
		"limit": limit,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		return driver.CollectRecords(ctx, result, func(rec *neo4j.Record) (*citation.CitationPath, error) {
			pathVal, ok := rec.Get("path")
			if !ok {
				return nil, errors.New(errors.ErrCodeInternal, "path not found in record")
			}
			path, ok := pathVal.(neo4j.Path)
			if !ok {
				return nil, errors.New(errors.ErrCodeInternal, "unexpected path type")
			}
			return neo4jPathToCitationPath(path), nil
		})
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return []*citation.CitationPath{}, nil
	}
	paths, ok := res.([]*citation.CitationPath)
	if !ok {
		return []*citation.CitationPath{}, nil
	}
	return paths, nil
}

// ---------------------------------------------------------------------------
// Backward citations — patents *this* patent cites (outgoing CITES)
// ---------------------------------------------------------------------------

func (r *neo4jCitationRepo) GetBackwardCitations(ctx context.Context, patentID uuid.UUID, depth int, limit int) ([]*citation.CitationPath, error) {
	if depth <= 0 {
		return []*citation.CitationPath{}, nil
	}
	if depth > 5 {
		depth = 5
	}
	query := `
		MATCH (p:Patent {id: $id})
		MATCH path = (p)-[:CITES*1..$depth]->(cited:Patent)
		RETURN path
		LIMIT $limit
	`
	params := map[string]interface{}{
		"id":    patentID.String(),
		"depth": depth,
		"limit": limit,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		return driver.CollectRecords(ctx, result, func(rec *neo4j.Record) (*citation.CitationPath, error) {
			pathVal, ok := rec.Get("path")
			if !ok {
				return nil, errors.New(errors.ErrCodeInternal, "path not found in record")
			}
			path, ok := pathVal.(neo4j.Path)
			if !ok {
				return nil, errors.New(errors.ErrCodeInternal, "unexpected path type")
			}
			return neo4jPathToCitationPath(path), nil
		})
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return []*citation.CitationPath{}, nil
	}
	paths, ok := res.([]*citation.CitationPath)
	if !ok {
		return []*citation.CitationPath{}, nil
	}
	return paths, nil
}

// ---------------------------------------------------------------------------
// Citation network — full undirected subgraph at given depth
// ---------------------------------------------------------------------------

func (r *neo4jCitationRepo) GetCitationNetwork(ctx context.Context, patentID uuid.UUID, depth int) (*citation.CitationNetwork, error) {
	if depth <= 0 {
		depth = 1
	}
	if depth > 5 {
		depth = 5
	}
	query := `
		MATCH (center:Patent {id: $id})
		OPTIONAL MATCH (center)-[r:CITES*1..$depth]-(connected)
		WITH center, collect(DISTINCT connected) AS connectedNodes, collect(DISTINCT r) AS rels
		RETURN [center] + connectedNodes AS nodes, rels AS edges
	`
	params := map[string]interface{}{
		"id":    patentID.String(),
		"depth": depth,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			rec := result.Record()
			return buildCitationNetwork(rec, patentID)
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return &citation.CitationNetwork{
			Nodes: []*citation.PatentNode{},
			Edges: []*citation.CitationEdge{},
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return res.(*citation.CitationNetwork), nil
}

func buildCitationNetwork(rec *neo4j.Record, centerID uuid.UUID) (*citation.CitationNetwork, error) {
	network := &citation.CitationNetwork{
		Nodes: []*citation.PatentNode{},
		Edges: []*citation.CitationEdge{},
	}

	nodesVal, ok := rec.Get("nodes")
	if ok && nodesVal != nil {
		if nodeList, ok := nodesVal.([]interface{}); ok {
			seen := make(map[string]bool)
			for _, n := range nodeList {
				if node, ok := n.(neo4j.Node); ok {
					pn := neo4jNodeToPatentNode(node)
					idKey := pn.ID.String()
					if !seen[idKey] {
						network.Nodes = append(network.Nodes, pn)
						seen[idKey] = true
					}
				}
			}
		}
	}

	relsVal, ok := rec.Get("edges")
	if ok && relsVal != nil {
		if relList, ok := relsVal.([]interface{}); ok {
			seen := make(map[string]bool)
			for _, r := range relList {
				if rel, ok := r.(neo4j.Relationship); ok {
					edgeKey := extractEdgeKey(rel)
					if !seen[edgeKey] {
						network.Edges = append(network.Edges, &citation.CitationEdge{
							FromPatentID: centerID, // approximate; caller may refine
							ToPatentID:   centerID,
							Type:         rel.Type,
							Metadata:     rel.Props,
						})
						seen[edgeKey] = true
					}
				}
			}
		}
	}

	return network, nil
}

func extractEdgeKey(rel neo4j.Relationship) string {
	return rel.Type
}

// ---------------------------------------------------------------------------
// Citation stats
// ---------------------------------------------------------------------------

func (r *neo4jCitationRepo) GetCitationCount(ctx context.Context, patentID uuid.UUID) (*citation.CitationStats, error) {
	query := `
		MATCH (p:Patent {id: $id})
		OPTIONAL MATCH (p)<-[:CITES]-(forward:Patent)
		OPTIONAL MATCH (p)-[:CITES]->(backward:Patent)
		RETURN count(DISTINCT forward) AS forward_count,
		       count(DISTINCT backward) AS backward_count
	`
	params := map[string]interface{}{
		"id": patentID.String(),
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			rec := result.Record()
			stats := &citation.CitationStats{}
			if f, ok := rec.Get("forward_count"); ok && f != nil {
				stats.ForwardCount = toInt64(f)
			}
			if b, ok := rec.Get("backward_count"); ok && b != nil {
				stats.BackwardCount = toInt64(b)
			}
			stats.TotalCount = stats.ForwardCount + stats.BackwardCount
			return stats, nil
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return &citation.CitationStats{}, nil
	})
	if err != nil {
		return nil, err
	}
	return res.(*citation.CitationStats), nil
}

// ---------------------------------------------------------------------------
// Most cited patents
// ---------------------------------------------------------------------------

func (r *neo4jCitationRepo) GetMostCitedPatents(ctx context.Context, jurisdiction *string, limit int) ([]*citation.PatentWithCitationCount, error) {
	if limit <= 0 {
		limit = 10
	}

	var query string
	params := map[string]interface{}{
		"limit": limit,
	}

	if jurisdiction != nil && *jurisdiction != "" {
		query = `
			MATCH (p:Patent {jurisdiction: $jurisdiction})
			OPTIONAL MATCH ()-[r:CITES]->(p)
			WITH p, count(r) AS citation_count
			RETURN p, citation_count
			ORDER BY citation_count DESC
			LIMIT $limit
		`
		params["jurisdiction"] = *jurisdiction
	} else {
		query = `
			MATCH (p:Patent)
			OPTIONAL MATCH ()-[r:CITES]->(p)
			WITH p, count(r) AS citation_count
			RETURN p, citation_count
			ORDER BY citation_count DESC
			LIMIT $limit
		`
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		return driver.CollectRecords(ctx, result, func(rec *neo4j.Record) (*citation.PatentWithCitationCount, error) {
			pn := &citation.PatentWithCitationCount{}
			if pVal, ok := rec.Get("p"); ok {
				if node, ok := pVal.(neo4j.Node); ok {
					pn.Patent = neo4jNodeToPatentNode(node)
				}
			}
			if cc, ok := rec.Get("citation_count"); ok && cc != nil {
				pn.CitationCount = toInt64(cc)
			}
			return pn, nil
		})
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return []*citation.PatentWithCitationCount{}, nil
	}
	return res.([]*citation.PatentWithCitationCount), nil
}

// ---------------------------------------------------------------------------
// Citation chain — shortest path between two patents
// ---------------------------------------------------------------------------

func (r *neo4jCitationRepo) GetCitationChain(ctx context.Context, fromPatentID, toPatentID uuid.UUID) ([]*citation.CitationPath, error) {
	query := `
		MATCH path = shortestPath(
			(a:Patent {id: $from})-[:CITES*]-(b:Patent {id: $to})
		)
		RETURN path
	`
	params := map[string]interface{}{
		"from": fromPatentID.String(),
		"to":   toPatentID.String(),
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			rec := result.Record()
			pathVal, ok := rec.Get("path")
			if !ok || pathVal == nil {
				return []*citation.CitationPath{}, nil
			}
			path, ok := pathVal.(neo4j.Path)
			if !ok {
				return []*citation.CitationPath{}, nil
			}
			return []*citation.CitationPath{neo4jPathToCitationPath(path)}, nil
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return []*citation.CitationPath{}, nil
	})
	if err != nil {
		return nil, err
	}
	return res.([]*citation.CitationPath), nil
}

// ---------------------------------------------------------------------------
// Co-citation — patents that are cited together with this patent
// ---------------------------------------------------------------------------

func (r *neo4jCitationRepo) GetCoCitationPatents(ctx context.Context, patentID uuid.UUID, minCommonCitations int, limit int) ([]*citation.CoCitationResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if minCommonCitations < 1 {
		minCommonCitations = 1
	}
	query := `
		MATCH (p:Patent {id: $id})<-[:CITES]-(citing:Patent)-[:CITES]->(co:Patent)
		WHERE co.id <> $id
		WITH co, count(DISTINCT citing) AS common_count
		WHERE common_count >= $minCommon
		RETURN co, common_count
		ORDER BY common_count DESC
		LIMIT $limit
	`
	params := map[string]interface{}{
		"id":        patentID.String(),
		"minCommon": minCommonCitations,
		"limit":     limit,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		return driver.CollectRecords(ctx, result, func(rec *neo4j.Record) (*citation.CoCitationResult, error) {
			cr := &citation.CoCitationResult{}
			if coVal, ok := rec.Get("co"); ok {
				if node, ok := coVal.(neo4j.Node); ok {
					cr.Patent = neo4jNodeToPatentNode(node)
				}
			}
			if cc, ok := rec.Get("common_count"); ok && cc != nil {
				cr.CommonCount = toInt64(cc)
			}
			return cr, nil
		})
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return []*citation.CoCitationResult{}, nil
	}
	return res.([]*citation.CoCitationResult), nil
}

// ---------------------------------------------------------------------------
// Bibliographic coupling — patents that share cited references with this patent
// ---------------------------------------------------------------------------

func (r *neo4jCitationRepo) GetBibliographicCoupling(ctx context.Context, patentID uuid.UUID, minCommonReferences int, limit int) ([]*citation.CouplingResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if minCommonReferences < 1 {
		minCommonReferences = 1
	}
	query := `
		MATCH (p:Patent {id: $id})-[:CITES]->(cited:Patent)<-[:CITES]-(co:Patent)
		WHERE co.id <> $id
		WITH co, count(DISTINCT cited) AS common_count
		WHERE common_count >= $minCommon
		RETURN co, common_count
		ORDER BY common_count DESC
		LIMIT $limit
	`
	params := map[string]interface{}{
		"id":        patentID.String(),
		"minCommon": minCommonReferences,
		"limit":     limit,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		return driver.CollectRecords(ctx, result, func(rec *neo4j.Record) (*citation.CouplingResult, error) {
			cr := &citation.CouplingResult{}
			if coVal, ok := rec.Get("co"); ok {
				if node, ok := coVal.(neo4j.Node); ok {
					cr.Patent = neo4jNodeToPatentNode(node)
				}
			}
			if cc, ok := rec.Get("common_count"); ok && cc != nil {
				cr.CommonCount = toInt64(cc)
			}
			return cr, nil
		})
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return []*citation.CouplingResult{}, nil
	}
	return res.([]*citation.CouplingResult), nil
}

// ---------------------------------------------------------------------------
// PageRank via GDS plugin
// ---------------------------------------------------------------------------

func (r *neo4jCitationRepo) CalculatePageRank(ctx context.Context, iterations int, dampingFactor float64) error {
	if iterations <= 0 {
		iterations = 20
	}
	if dampingFactor <= 0 {
		dampingFactor = 0.85
	}
	return r.runPageRank(ctx, iterations, dampingFactor, false)
}

func (r *neo4jCitationRepo) runPageRank(ctx context.Context, iterations int, dampingFactor float64, isRetry bool) error {
	// Project the citation graph, run PageRank, and write scores back to Patent nodes.
	query := `
		CALL gds.graph.project(
			'citation-pagerank',
			'Patent',
			{ CITES: { orientation: 'NATURAL' } }
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
	`
	params := map[string]interface{}{
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
		// GDS sentinel was returned directly by the inner function.
		// Check the error chain for the sentinel by unwrapping.
		if isGDSNotAvailableError(err) {
			return ErrGDSNotAvailable
		}
		// Check the root cause for graph-already-exists.
		rawCause := unwrapDeep(err)
		if !isRetry && isGraphAlreadyExistsError(rawCause) {
			_ = r.dropPageRankGraph(ctx)
			return r.runPageRank(ctx, iterations, dampingFactor, true)
		}
		return err
	}
	return nil
}

func (r *neo4jCitationRepo) dropPageRankGraph(ctx context.Context) error {
	dropQuery := `CALL gds.graph.drop('citation-pagerank') YIELD graphName RETURN graphName`
	_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, innerErr := tx.Run(ctx, dropQuery, nil)
		return nil, innerErr
	})
	return err
}

// isGDSNotFound checks whether the error indicates GDS procedures are unavailable.
func isGDSNotFound(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return containsSub(errMsg, "There is no procedure with the name `gds") ||
		containsSub(errMsg, "ProcedureNotFound") ||
		containsSub(errMsg, "gds.graph.project")
}

// isGraphAlreadyExistsError checks if a raw driver error indicates that a GDS
// graph projection with the same name already exists.
func isGraphAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return (containsSub(errMsg, "already exists") || containsSub(errMsg, "already loaded")) &&
		(containsSub(errMsg, "Graph") || containsSub(errMsg, "graph") || containsSub(errMsg, "projection"))
}

// isGDSNotAvailableError checks if any error in the chain is the GDS sentinel.
func isGDSNotAvailableError(err error) bool {
	if err == nil {
		return false
	}
	if err == ErrGDSNotAvailable {
		return true
	}
	// Walk the AppError cause chain manually.
	for e := err; e != nil; {
		if e == ErrGDSNotAvailable {
			return true
		}
		if appErr, ok := e.(*errors.AppError); ok {
			e = appErr.Cause
		} else {
			break
		}
	}
	return false
}

// unwrapDeep walks the entire AppError cause chain and returns the innermost
// error (the root cause).
func unwrapDeep(err error) error {
	if err == nil {
		return nil
	}
	for {
		if appErr, ok := err.(*errors.AppError); ok && appErr.Cause != nil {
			err = appErr.Cause
			continue
		}
		break
	}
	return err
}

func containsSub(s, sub string) bool {
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Top PageRank patents
// ---------------------------------------------------------------------------

func (r *neo4jCitationRepo) GetTopPageRankPatents(ctx context.Context, limit int) ([]*citation.PatentWithPageRank, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `
		MATCH (p:Patent)
		WHERE p.pagerank IS NOT NULL
		RETURN p, p.pagerank AS score
		ORDER BY score DESC
		LIMIT $limit
	`
	params := map[string]interface{}{
		"limit": limit,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		return driver.CollectRecords(ctx, result, func(rec *neo4j.Record) (*citation.PatentWithPageRank, error) {
			pr := &citation.PatentWithPageRank{}
			if pVal, ok := rec.Get("p"); ok {
				if node, ok := pVal.(neo4j.Node); ok {
					pr.Patent = neo4jNodeToPatentNode(node)
				}
			}
			if score, ok := rec.Get("score"); ok && score != nil {
				pr.PageRank = toFloat64(score)
			}
			return pr, nil
		})
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return []*citation.PatentWithPageRank{}, nil
	}
	return res.([]*citation.PatentWithPageRank), nil
}

// ---------------------------------------------------------------------------
// Numeric conversion helpers
// ---------------------------------------------------------------------------

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case float64:
		return int64(val)
	case int:
		return int64(val)
	case int32:
		return int64(val)
	}
	return 0
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	}
	return 0
}
