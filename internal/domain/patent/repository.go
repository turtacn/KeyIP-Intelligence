package patent

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SearchFilter defines search criteria for patents (used by gRPC services).
type SearchFilter struct {
	Query         string
	IPCClasses    []string
	CPCClasses    []string
	PatentOffices []string
	DateFrom      time.Time
	DateTo        time.Time
	PageSize      int
	PageToken     string
	SortBy        string
	SortOrder     string
}

// PatentFamily represents a group of related patents.
type PatentFamily struct {
	FamilyID string
	Members  []*FamilyMember
}

// FamilyMember represents a member of a patent family.
type FamilyMember struct {
	PatentNumber     string
	PatentOffice     string
	ApplicationDate  time.Time
	PublicationDate  time.Time
	LegalStatus      string
	IsRepresentative bool
}

// CitationNetworkQuery defines parameters for citation network queries.
type CitationNetworkQuery struct {
	PatentNumber  string
	Depth         int32
	IncludeCiting bool
	IncludeCited  bool
}

// CitationNetwork represents a patent citation network.
type CitationNetwork struct {
	Nodes       []*CitationNode
	Edges       []*CitationEdge
	IsTruncated bool
}

// CitationNode represents a node in a citation network.
type CitationNode struct {
	PatentNumber string
	Title        string
	IsRoot       bool
	Level        int
}

// CitationEdge represents an edge in a citation network.
type CitationEdge struct {
	FromPatent string
	ToPatent   string
	EdgeType   string
}

// PatentRepository defines the persistence contract for patent domain.
type PatentRepository interface {
	// Patent
	Create(ctx context.Context, p *Patent) error
	GetByID(ctx context.Context, id uuid.UUID) (*Patent, error)
	GetByPatentNumber(ctx context.Context, number string) (*Patent, error)
	Update(ctx context.Context, p *Patent) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	Restore(ctx context.Context, id uuid.UUID) error
	HardDelete(ctx context.Context, id uuid.UUID) error

	// Search
	Search(ctx context.Context, query SearchQuery) (*SearchResult, error)
	ListByPortfolio(ctx context.Context, portfolioID string) ([]*Patent, error)
	GetByFamilyID(ctx context.Context, familyID string) ([]*Patent, error)
	GetByAssignee(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*Patent, int64, error)
	GetByJurisdiction(ctx context.Context, jurisdiction string, limit, offset int) ([]*Patent, int64, error)
	GetExpiringPatents(ctx context.Context, daysAhead int, limit, offset int) ([]*Patent, int64, error)
	FindDuplicates(ctx context.Context, fullTextHash string) ([]*Patent, error)
	FindByMoleculeID(ctx context.Context, moleculeID string) ([]*Patent, error)
	AssociateMolecule(ctx context.Context, patentID string, moleculeID string) error

	// Claims
	CreateClaim(ctx context.Context, claim *Claim) error
	GetClaimsByPatent(ctx context.Context, patentID uuid.UUID) ([]*Claim, error)
	UpdateClaim(ctx context.Context, claim *Claim) error
	DeleteClaimsByPatent(ctx context.Context, patentID uuid.UUID) error
	BatchCreateClaims(ctx context.Context, claims []*Claim) error
	GetIndependentClaims(ctx context.Context, patentID uuid.UUID) ([]*Claim, error)

	// Inventors
	SetInventors(ctx context.Context, patentID uuid.UUID, inventors []*Inventor) error
	GetInventors(ctx context.Context, patentID uuid.UUID) ([]*Inventor, error)
	SearchByInventor(ctx context.Context, inventorName string, limit, offset int) ([]*Patent, int64, error)

	// Assignee
	SearchByAssigneeName(ctx context.Context, assigneeName string, limit, offset int) ([]*Patent, int64, error)

	// Priority
	SetPriorityClaims(ctx context.Context, patentID uuid.UUID, claims []*PriorityClaim) error
	GetPriorityClaims(ctx context.Context, patentID uuid.UUID) ([]*PriorityClaim, error)

	// Batch
	BatchCreate(ctx context.Context, patents []*Patent) (int, error)
	BatchUpdateStatus(ctx context.Context, ids []uuid.UUID, status PatentStatus) (int64, error)

	// Stats
	CountByStatus(ctx context.Context) (map[PatentStatus]int64, error)
	CountByJurisdiction(ctx context.Context) (map[string]int64, error)
	CountByYear(ctx context.Context, field string) (map[int]int64, error)
	GetIPCDistribution(ctx context.Context, level int) (map[string]int64, error)

	// Transaction
	WithTx(ctx context.Context, fn func(PatentRepository) error) error
}

// Repository alias for PatentRepository
type Repository = PatentRepository

// CitationRepository handles patent citation relationships.
type CitationRepository interface {
	// GetCitations returns citations for a patent
	GetCitations(ctx context.Context, patentID string) ([]string, error)
	// AddCitation adds a citation relationship
	AddCitation(ctx context.Context, patentID, citedPatentID string) error
	// GetCitedBy returns patents citing the given patent
	GetCitedBy(ctx context.Context, patentID string) ([]string, error)
}

// FamilyRepository handles patent family relationships.
type FamilyRepository interface {
	// GetFamilyMembers returns family members for a patent
	GetFamilyMembers(ctx context.Context, patentID string) ([]string, error)
	// AddFamilyMember adds a family relationship
	AddFamilyMember(ctx context.Context, patentID, familyPatentID string) error
	// GetFamilyID returns the family ID for a patent
	GetFamilyID(ctx context.Context, patentID string) (string, error)
}

// KnowledgeGraphRepository handles patent knowledge graph operations.
type KnowledgeGraphRepository interface {
	// GetRelatedPatents returns related patents based on knowledge graph
	GetRelatedPatents(ctx context.Context, patentID string, depth int) ([]string, error)
	// GetTechnologyClusters returns technology clusters
	GetTechnologyClusters(ctx context.Context, domain string) ([]string, error)
	// FindPath finds shortest path between two patents
	FindPath(ctx context.Context, fromPatentID, toPatentID string) ([]string, error)
}

//Personal.AI order the ending
