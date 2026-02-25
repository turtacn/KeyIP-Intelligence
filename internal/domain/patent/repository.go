package patent

import (
	"context"

	"github.com/google/uuid"
)

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

//Personal.AI order the ending
