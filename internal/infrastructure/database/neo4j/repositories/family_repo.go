package repositories

import (
	"context"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/family"
	driver "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type neo4jFamilyRepo struct {
	driver *driver.Driver
	log    logging.Logger
}

func NewNeo4jFamilyRepo(d *driver.Driver, log logging.Logger) family.FamilyRepository {
	return &neo4jFamilyRepo{
		driver: d,
		log:    log,
	}
}

// ---------------------------------------------------------------------------
// Family node management
// ---------------------------------------------------------------------------

func (r *neo4jFamilyRepo) EnsureFamilyNode(ctx context.Context, familyID string, familyType string, metadata map[string]interface{}) error {
	query := `
		MERGE (f:PatentFamily {family_id: $familyId})
		ON CREATE SET f.family_type = $type, f.created_at = datetime(), f += $metadata
		ON MATCH SET f.family_type = $type, f.updated_at = datetime()
	`
	params := map[string]interface{}{
		"familyId": familyID,
		"type":     familyType,
		"metadata": metadata,
	}
	_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

func (r *neo4jFamilyRepo) GetFamily(ctx context.Context, familyID string) (*family.FamilyAggregate, error) {
	query := `
		MATCH (f:PatentFamily {family_id: $familyId})
		OPTIONAL MATCH (p:Patent)-[r:MEMBER_OF]->(f)
		RETURN f, p, r.role AS role, r.added_at AS added_at
	`
	params := map[string]interface{}{
		"familyId": familyID,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var agg *family.FamilyAggregate
		for result.Next(ctx) {
			rec := result.Record()
			if agg == nil {
				fVal, ok := rec.Get("f")
				if !ok || fVal == nil {
					return nil, errors.New(errors.ErrCodeNotFound, "family not found")
				}
				fNode, ok := fVal.(neo4j.Node)
				if !ok {
					return nil, errors.New(errors.ErrCodeInternal, "unexpected family node type")
				}
				agg = buildFamilyAggregateFromNode(fNode)
			}
			// Extract member from this row
			pVal, ok := rec.Get("p")
			if !ok || pVal == nil {
				continue
			}
			if pNode, ok := pVal.(neo4j.Node); ok {
				member := buildFamilyMember(pNode, rec)
				agg.Members = append(agg.Members, *member)
			}
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		if agg == nil {
			return nil, errors.New(errors.ErrCodeNotFound, "family not found")
		}
		return agg, nil
	})
	if err != nil {
		return nil, err
	}
	return res.(*family.FamilyAggregate), nil
}

func (r *neo4jFamilyRepo) DeleteFamily(ctx context.Context, familyID string) error {
	query := `
		MATCH (f:PatentFamily {family_id: $familyId})
		OPTIONAL MATCH (p:Patent)-[r:MEMBER_OF]->(f)
		DELETE r, f
	`
	params := map[string]interface{}{
		"familyId": familyID,
	}
	_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

func (r *neo4jFamilyRepo) ListFamilies(ctx context.Context, familyType *string, limit, offset int) ([]*family.FamilyAggregate, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	typeFilter := ""
	params := map[string]interface{}{
		"limit":  limit,
		"offset": offset,
	}
	if familyType != nil && *familyType != "" {
		typeFilter = "WHERE f.family_type = $familyType"
		params["familyType"] = *familyType
	}

	// Count query
	countQuery := `
		MATCH (f:PatentFamily)
		` + typeFilter + `
		RETURN count(f) AS total
	`

	// List query with members
	listQuery := `
		MATCH (f:PatentFamily)
		` + typeFilter + `
		WITH f
		ORDER BY f.created_at DESC
		SKIP $offset
		LIMIT $limit
		OPTIONAL MATCH (p:Patent)-[r:MEMBER_OF]->(f)
		RETURN f, collect(CASE WHEN p IS NOT NULL THEN {
			patent_id: p.id,
			patent_number: p.patent_number,
			jurisdiction: p.jurisdiction,
			filing_date: p.filing_date,
			role: r.role,
			added_at: r.added_at
		} END) AS members
	`

	totalCount := int64(0)

	_, countErr := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, countQuery, params)
		if err != nil {
			return nil, err
		}
		if result.Next(ctx) {
			rec := result.Record()
			if t, ok := rec.Get("total"); ok && t != nil {
				totalCount = toInt64(t)
			}
		}
		return nil, result.Err()
	})
	if countErr != nil {
		return nil, 0, countErr
	}

	res, listErr := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, listQuery, params)
		if err != nil {
			return nil, err
		}

		var families []*family.FamilyAggregate
		for result.Next(ctx) {
			rec := result.Record()
			fVal, ok := rec.Get("f")
			if !ok || fVal == nil {
				continue
			}
			fNode, ok := fVal.(neo4j.Node)
			if !ok {
				continue
			}
			agg := buildFamilyAggregateFromNode(fNode)

			// Parse members array
			if mVal, ok := rec.Get("members"); ok && mVal != nil {
				if mList, ok := mVal.([]interface{}); ok {
					for _, m := range mList {
						if mMap, ok := m.(map[string]interface{}); ok {
							member := familyMemberFromMap(mMap)
							if member != nil {
								agg.Members = append(agg.Members, *member)
							}
						}
					}
				}
			}
			families = append(families, agg)
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return families, nil
	})
	if listErr != nil {
		return nil, 0, listErr
	}

	return res.([]*family.FamilyAggregate), totalCount, nil
}

// ---------------------------------------------------------------------------
// Member management
// ---------------------------------------------------------------------------

func (r *neo4jFamilyRepo) AddMember(ctx context.Context, familyID string, patentID string, memberRole string) error {
	query := `
		MATCH (f:PatentFamily {family_id: $familyId}), (p:Patent {id: $patentId})
		MERGE (p)-[r:MEMBER_OF {role: $role}]->(f)
		ON CREATE SET r.added_at = datetime()
	`
	params := map[string]interface{}{
		"familyId":  familyID,
		"patentId":  patentID,
		"role":      memberRole,
	}
	_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

func (r *neo4jFamilyRepo) RemoveMember(ctx context.Context, familyID string, patentID string) error {
	query := `
		MATCH (f:PatentFamily {family_id: $familyId})<-[r:MEMBER_OF]-(p:Patent {id: $patentId})
		DELETE r
	`
	params := map[string]interface{}{
		"familyId":  familyID,
		"patentId":  patentID,
	}
	_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

func (r *neo4jFamilyRepo) GetMembers(ctx context.Context, familyID string, memberRole *string) ([]*family.FamilyMember, error) {
	query := `
		MATCH (f:PatentFamily {family_id: $familyId})<-[r:MEMBER_OF]-(p:Patent)
		WHERE ($role IS NULL OR r.role = $role)
		RETURN p, r.role AS role, r.added_at AS added_at
	`
	params := map[string]interface{}{
		"familyId": familyID,
	}
	if memberRole != nil && *memberRole != "" {
		params["role"] = *memberRole
	} else {
		params["role"] = nil
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var members []*family.FamilyMember
		for result.Next(ctx) {
			rec := result.Record()
			pVal, ok := rec.Get("p")
			if !ok || pVal == nil {
				continue
			}
			pNode, ok := pVal.(neo4j.Node)
			if !ok {
				continue
			}
			member := buildFamilyMember(pNode, rec)
			members = append(members, member)
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return members, nil
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return []*family.FamilyMember{}, nil
	}
	return res.([]*family.FamilyMember), nil
}

func (r *neo4jFamilyRepo) GetFamilyByPatent(ctx context.Context, patentID string) ([]*family.FamilyAggregate, error) {
	query := `
		MATCH (p:Patent {id: $patentId})-[r:MEMBER_OF]->(f:PatentFamily)
		OPTIONAL MATCH (member:Patent)-[mr:MEMBER_OF]->(f)
		RETURN f, member, mr.role AS role, mr.added_at AS added_at
	`
	params := map[string]interface{}{
		"patentId": patentID,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		// Map from family_id to aggregate
		familyMap := make(map[string]*family.FamilyAggregate)
		for result.Next(ctx) {
			rec := result.Record()
			fVal, ok := rec.Get("f")
			if !ok || fVal == nil {
				continue
			}
			fNode, ok := fVal.(neo4j.Node)
			if !ok {
				continue
			}
			fid := ""
			if id, ok := fNode.Props["family_id"].(string); ok {
				fid = id
			}
			if _, exists := familyMap[fid]; !exists {
				agg := buildFamilyAggregateFromNode(fNode)
				familyMap[fid] = agg
			}

			mVal, ok := rec.Get("member")
			if !ok || mVal == nil {
				continue
			}
			if mNode, ok := mVal.(neo4j.Node); ok {
				member := buildFamilyMember(mNode, rec)
				familyMap[fid].Members = append(familyMap[fid].Members, *member)
			}
		}
		if err := result.Err(); err != nil {
			return nil, err
		}

		resultFamilies := make([]*family.FamilyAggregate, 0, len(familyMap))
		for _, agg := range familyMap {
			resultFamilies = append(resultFamilies, agg)
		}
		return resultFamilies, nil
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return []*family.FamilyAggregate{}, nil
	}
	return res.([]*family.FamilyAggregate), nil
}

func (r *neo4jFamilyRepo) BatchAddMembers(ctx context.Context, familyID string, members []*family.FamilyMemberInput) error {
	if len(members) == 0 {
		return nil
	}
	query := `
		UNWIND $batch AS row
		MATCH (f:PatentFamily {family_id: $familyId}), (p:Patent {id: row.patentId})
		MERGE (p)-[r:MEMBER_OF {role: row.role}]->(f)
		ON CREATE SET r.added_at = datetime()
	`
	var batch []map[string]interface{}
	for _, m := range members {
		batch = append(batch, map[string]interface{}{
			"patentId": m.PatentID,
			"role":     m.Role,
		})
	}
	params := map[string]interface{}{
		"familyId": familyID,
		"batch":    batch,
	}
	_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

// ---------------------------------------------------------------------------
// Priority links
// ---------------------------------------------------------------------------

func (r *neo4jFamilyRepo) CreatePriorityLink(ctx context.Context, fromPatentID, toPatentID string, priorityDate string, priorityCountry string) error {
	query := `
		MATCH (a:Patent {id: $from}), (b:Patent {id: $to})
		MERGE (a)-[r:PRIORITY_OF]->(b)
		ON CREATE SET r.priority_date = $date, r.priority_country = $country, r.created_at = datetime()
	`
	params := map[string]interface{}{
		"from":    fromPatentID,
		"to":      toPatentID,
		"date":    priorityDate,
		"country": priorityCountry,
	}
	_, err := r.driver.ExecuteWrite(ctx, func(tx driver.Transaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})
	return err
}

func (r *neo4jFamilyRepo) GetPriorityChain(ctx context.Context, patentID string) ([]*family.PriorityLink, error) {
	query := `
		MATCH path = (p:Patent {id: $patentId})-[:PRIORITY_OF*]->(other:Patent)
		RETURN path
		ORDER BY length(path) ASC
	`
	params := map[string]interface{}{
		"patentId": patentID,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var links []*family.PriorityLink
		seen := make(map[string]bool)
		for result.Next(ctx) {
			rec := result.Record()
			pathVal, ok := rec.Get("path")
			if !ok || pathVal == nil {
				continue
			}
			path, ok := pathVal.(neo4j.Path)
			if !ok {
				continue
			}
			for _, rel := range path.Relationships {
				link := buildPriorityLink(rel, path)
				if link == nil {
					continue
				}
				key := link.FromPatentID + "->" + link.ToPatentID
				if !seen[key] {
					links = append(links, link)
					seen[key] = true
				}
			}
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return links, nil
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return []*family.PriorityLink{}, nil
	}
	return res.([]*family.PriorityLink), nil
}

func (r *neo4jFamilyRepo) GetDerivedPatents(ctx context.Context, patentID string) ([]*family.PriorityLink, error) {
	query := `
		MATCH path = (other:Patent)-[:PRIORITY_OF*]->(p:Patent {id: $patentId})
		RETURN path
		ORDER BY length(path) ASC
	`
	params := map[string]interface{}{
		"patentId": patentID,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var links []*family.PriorityLink
		seen := make(map[string]bool)
		for result.Next(ctx) {
			rec := result.Record()
			pathVal, ok := rec.Get("path")
			if !ok || pathVal == nil {
				continue
			}
			path, ok := pathVal.(neo4j.Path)
			if !ok {
				continue
			}
			for _, rel := range path.Relationships {
				link := buildPriorityLink(rel, path)
				if link == nil {
					continue
				}
				key := link.FromPatentID + "->" + link.ToPatentID
				if !seen[key] {
					links = append(links, link)
					seen[key] = true
				}
			}
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return links, nil
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return []*family.PriorityLink{}, nil
	}
	return res.([]*family.PriorityLink), nil
}

// ---------------------------------------------------------------------------
// Family stats
// ---------------------------------------------------------------------------

func (r *neo4jFamilyRepo) GetFamilyStats(ctx context.Context, familyID string) (*family.FamilyStats, error) {
	query := `
		MATCH (f:PatentFamily {family_id: $familyId})<-[:MEMBER_OF]-(p:Patent)
		RETURN p.jurisdiction AS jurisdiction, p.status AS status,
		       p.filing_date AS filingDate, p.expiry_date AS expiryDate
	`
	params := map[string]interface{}{
		"familyId": familyID,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		stats := &family.FamilyStats{
			JurisdictionCounts: make(map[string]int64),
			StatusDistribution: make(map[string]int64),
		}
		jurSet := make(map[string]struct{})

		for result.Next(ctx) {
			rec := result.Record()
			stats.TotalMembers++

			if jurVal, ok := rec.Get("jurisdiction"); ok && jurVal != nil {
				if jur, ok := jurVal.(string); ok && jur != "" {
					jurSet[jur] = struct{}{}
					stats.JurisdictionCounts[jur]++
				}
			}
			if statusVal, ok := rec.Get("status"); ok && statusVal != nil {
				if s, ok := statusVal.(string); ok && s != "" {
					stats.StatusDistribution[s]++
				}
			}
			if fdVal, ok := rec.Get("filingDate"); ok && fdVal != nil {
				fd := extractTime(fdVal)
				if fd != nil {
					if stats.EarliestFiling == nil || fd.Before(*stats.EarliestFiling) {
						stats.EarliestFiling = fd
					}
				}
			}
			if edVal, ok := rec.Get("expiryDate"); ok && edVal != nil {
				ed := extractTime(edVal)
				if ed != nil {
					if stats.LatestExpiry == nil || ed.After(*stats.LatestExpiry) {
						stats.LatestExpiry = ed
					}
				}
			}
		}
		if err := result.Err(); err != nil {
			return nil, err
		}

		stats.Jurisdictions = make([]string, 0, len(jurSet))
		for j := range jurSet {
			stats.Jurisdictions = append(stats.Jurisdictions, j)
		}
		return stats, nil
	})
	if err != nil {
		return nil, err
	}
	return res.(*family.FamilyStats), nil
}

func (r *neo4jFamilyRepo) GetFamilyCoverage(ctx context.Context, familyID string) (map[string]int64, error) {
	query := `
		MATCH (f:PatentFamily {family_id: $familyId})<-[:MEMBER_OF]-(p:Patent)
		RETURN p.jurisdiction AS jurisdiction, count(*) AS cnt
	`
	params := map[string]interface{}{
		"familyId": familyID,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}
		coverage := make(map[string]int64)
		for result.Next(ctx) {
			rec := result.Record()
			jurVal, _ := rec.Get("jurisdiction")
			cntVal, _ := rec.Get("cnt")
			if jur, ok := jurVal.(string); ok {
				coverage[jur] = toInt64(cntVal)
			}
		}
		return coverage, result.Err()
	})
	if err != nil {
		return nil, err
	}
	return res.(map[string]int64), nil
}

func (r *neo4jFamilyRepo) FindRelatedFamilies(ctx context.Context, familyID string, maxDistance int) ([]*family.RelatedFamily, error) {
	if maxDistance <= 0 {
		maxDistance = 1
	}
	if maxDistance > 5 {
		maxDistance = 5
	}

	query := `
		MATCH (f:PatentFamily {family_id: $familyId})<-[:MEMBER_OF]-(p:Patent)-[:MEMBER_OF]->(other:PatentFamily)
		WHERE other.family_id <> $familyId
		WITH other, count(DISTINCT p) AS overlap
		ORDER BY overlap DESC
		RETURN other.family_id AS family_id, other.family_type AS family_type,
		       overlap AS overlap_count
		LIMIT $limit
	`
	params := map[string]interface{}{
		"familyId": familyID,
		"limit":    50,
	}

	res, err := r.driver.ExecuteRead(ctx, func(tx driver.Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var related []*family.RelatedFamily
		for result.Next(ctx) {
			rec := result.Record()
			rf := &family.RelatedFamily{}
			if fid, ok := rec.Get("family_id"); ok && fid != nil {
				rf.FamilyID = fid.(string)
			}
			if ft, ok := rec.Get("family_type"); ok && ft != nil {
				rf.FamilyType = ft.(string)
			}
			if oc, ok := rec.Get("overlap_count"); ok && oc != nil {
				rf.OverlapCount = toInt64(oc)
			}
			related = append(related, rf)
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return related, nil
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return []*family.RelatedFamily{}, nil
	}
	return res.([]*family.RelatedFamily), nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// buildFamilyAggregateFromNode constructs a FamilyAggregate from a PatentFamily
// Neo4j node. Members are not populated here; callers add them separately.
func buildFamilyAggregateFromNode(node neo4j.Node) *family.FamilyAggregate {
	agg := &family.FamilyAggregate{
		Members:  []family.FamilyMember{},
		Metadata: make(map[string]interface{}),
	}
	if fid, ok := node.Props["family_id"].(string); ok {
		agg.FamilyID = fid
	}
	if ft, ok := node.Props["family_type"].(string); ok {
		agg.FamilyType = ft
	}
	if ca, ok := node.Props["created_at"]; ok && ca != nil {
		agg.CreatedAt = extractTimeValue(ca)
	}
	if ua, ok := node.Props["updated_at"]; ok && ua != nil {
		agg.UpdatedAt = extractTimeValue(ua)
	}
	// Gather remaining props into metadata, excluding known keys
	for k, v := range node.Props {
		switch k {
		case "family_id", "family_type", "created_at", "updated_at":
			continue
		}
		agg.Metadata[k] = v
	}
	return agg
}

// buildFamilyMember constructs a FamilyMember from a Patent Neo4j node and the
// current record which holds the role and added_at from the relationship.
func buildFamilyMember(pNode neo4j.Node, rec *neo4j.Record) *family.FamilyMember {
	member := &family.FamilyMember{}
	if pid, ok := pNode.Props["id"].(string); ok {
		member.PatentID = pid
	}
	if pn, ok := pNode.Props["patent_number"].(string); ok {
		member.PatentNumber = pn
	}
	if j, ok := pNode.Props["jurisdiction"].(string); ok {
		member.Jurisdiction = j
	}
	if fd, ok := pNode.Props["filing_date"]; ok && fd != nil {
		member.FilingDate = extractTime(fd)
	}
	if role, ok := rec.Get("role"); ok && role != nil {
		if r, ok := role.(string); ok {
			member.Role = r
		}
	}
	if aa, ok := rec.Get("added_at"); ok && aa != nil {
		member.AddedAt = extractTimeValue(aa)
	}
	return member
}

// familyMemberFromMap reconstructs a FamilyMember from a map returned by a
// Cypher collect / projection query.
func familyMemberFromMap(m map[string]interface{}) *family.FamilyMember {
	member := &family.FamilyMember{}
	if pid, ok := m["patent_id"].(string); ok {
		member.PatentID = pid
	}
	if pn, ok := m["patent_number"].(string); ok {
		member.PatentNumber = pn
	}
	if j, ok := m["jurisdiction"].(string); ok {
		member.Jurisdiction = j
	}
	if fd, ok := m["filing_date"]; ok && fd != nil {
		member.FilingDate = extractTime(fd)
	}
	if role, ok := m["role"].(string); ok {
		member.Role = role
	}
	if aa, ok := m["added_at"]; ok && aa != nil {
		member.AddedAt = extractTimeValue(aa)
	}
	return member
}

// buildPriorityLink constructs a PriorityLink from a relationship within a path.
func buildPriorityLink(rel neo4j.Relationship, path neo4j.Path) *family.PriorityLink {
	link := &family.PriorityLink{}
	// Find start and end node IDs from the relationship's internal IDs
	for _, n := range path.Nodes {
		if n.Id == rel.StartId {
			if id, ok := n.Props["id"].(string); ok {
				link.FromPatentID = id
			}
		}
		if n.Id == rel.EndId {
			if id, ok := n.Props["id"].(string); ok {
				link.ToPatentID = id
			}
		}
	}
	if pd, ok := rel.Props["priority_date"]; ok && pd != nil {
		switch v := pd.(type) {
		case string:
			if t, err := time.Parse("2006-01-02", v); err == nil {
				link.PriorityDate = t
			}
		case time.Time:
			link.PriorityDate = v
		case neo4j.Date:
			link.PriorityDate = v.Time()
		}
	}
	if pc, ok := rel.Props["priority_country"].(string); ok {
		link.PriorityCountry = pc
	}
	return link
}
