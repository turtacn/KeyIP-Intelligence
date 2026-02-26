package patent

import (
	"context"
	"math"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// PatentSearchCriteria encapsulates search parameters.
type PatentSearchCriteria struct {
	PatentNumbers     []string       `json:"patent_numbers,omitempty"`
	TitleKeywords     []string       `json:"title_keywords,omitempty"`
	AbstractKeywords  []string       `json:"abstract_keywords,omitempty"`
	FullTextKeywords  []string       `json:"full_text_keywords,omitempty"`
	Offices           []PatentOffice `json:"offices,omitempty"`
	Statuses          []PatentStatus `json:"statuses,omitempty"`
	IPCCodes          []string       `json:"ipc_codes,omitempty"`
	ApplicantNames    []string       `json:"applicant_names,omitempty"`
	InventorNames     []string       `json:"inventor_names,omitempty"`
	MoleculeIDs       []string       `json:"molecule_ids,omitempty"`
	FamilyID          string         `json:"family_id,omitempty"`
	FilingDateFrom    *time.Time     `json:"filing_date_from,omitempty"`
	FilingDateTo      *time.Time     `json:"filing_date_to,omitempty"`
	GrantDateFrom     *time.Time     `json:"grant_date_from,omitempty"`
	GrantDateTo       *time.Time     `json:"grant_date_to,omitempty"`
	ExpiryDateFrom    *time.Time     `json:"expiry_date_from,omitempty"`
	ExpiryDateTo      *time.Time     `json:"expiry_date_to,omitempty"`
	HasMarkushStructure *bool        `json:"has_markush_structure,omitempty"`
	MinClaimCount     *int           `json:"min_claim_count,omitempty"`
	MaxClaimCount     *int           `json:"max_claim_count,omitempty"`
	Language          string         `json:"language,omitempty"`
	SortBy            string         `json:"sort_by,omitempty"` // "filing_date", "grant_date", "relevance"
	SortOrder         string         `json:"sort_order,omitempty"` // "asc", "desc"
	Offset            int            `json:"offset"`
	Limit             int            `json:"limit"`
}

func (c *PatentSearchCriteria) Validate() error {
	if c.Limit < 0 {
		return errors.NewValidation("limit cannot be negative")
	}
	if c.Limit > 1000 {
		return errors.NewValidation("limit cannot exceed 1000")
	}
	if c.Offset < 0 {
		return errors.NewValidation("offset cannot be negative")
	}
	if c.SortBy != "" {
		validSorts := map[string]bool{
			"filing_date": true, "grant_date": true, "relevance": true, "citation_count": true,
		}
		if !validSorts[c.SortBy] {
			return errors.NewValidation("invalid sort by field")
		}
	}
	if c.SortOrder != "" {
		if c.SortOrder != "asc" && c.SortOrder != "desc" {
			return errors.NewValidation("invalid sort order")
		}
	}
	if c.FilingDateFrom != nil && c.FilingDateTo != nil && c.FilingDateFrom.After(*c.FilingDateTo) {
		return errors.NewValidation("filing date from cannot be after filing date to")
	}

	if c.Limit == 0 {
		c.Limit = 20
	}
	return nil
}

func (c *PatentSearchCriteria) HasFilters() bool {
	return len(c.PatentNumbers) > 0 ||
		len(c.TitleKeywords) > 0 ||
		len(c.AbstractKeywords) > 0 ||
		len(c.FullTextKeywords) > 0 ||
		len(c.Offices) > 0 ||
		len(c.Statuses) > 0 ||
		len(c.IPCCodes) > 0 ||
		len(c.ApplicantNames) > 0 ||
		len(c.InventorNames) > 0 ||
		len(c.MoleculeIDs) > 0 ||
		c.FamilyID != "" ||
		c.FilingDateFrom != nil ||
		c.HasMarkushStructure != nil ||
		c.MinClaimCount != nil ||
		c.Language != ""
}

// SimilaritySearchRequest defines parameters for molecule similarity search.
type SimilaritySearchRequest struct {
	SMILES          string         `json:"smiles"`
	Threshold       float64        `json:"threshold"`
	MaxResults      int            `json:"max_results"`
	PatentOffices   []string       `json:"patent_offices,omitempty"`
	Assignees       []string       `json:"assignees,omitempty"`
	TechDomains     []string       `json:"tech_domains,omitempty"`
	DateFrom        *time.Time     `json:"date_from,omitempty"`
	DateTo          *time.Time     `json:"date_to,omitempty"`
	ExcludePatents  []string       `json:"exclude_patents,omitempty"`
}

// PatentSearchResultWithSimilarity extends search result with similarity scores.
type PatentSearchResultWithSimilarity struct {
	PatentNumber       string    `json:"patent_number"`
	Title              string    `json:"title"`
	Assignee           string    `json:"assignee"`
	FilingDate         time.Time `json:"filing_date"`
	LegalStatus        string    `json:"legal_status"`
	IPCCodes           []string  `json:"ipc_codes"`
	MorganSimilarity   float64   `json:"morgan_similarity"`
	RDKitSimilarity    float64   `json:"rdkit_similarity"`
	AtomPairSimilarity float64   `json:"atom_pair_similarity"`
}

// SearchQuery is an alias for PatentSearchCriteria to support existing consumers if any.
type SearchQuery = PatentSearchCriteria

// PatentSearchResult represents the result of a search.
type PatentSearchResult struct {
	Items      []*Patent `json:"items"`
	Patents    []*Patent `json:"patents"` // Alias or duplicates Items? Using Items as primary.
	Total      int64     `json:"total"`
	Offset     int       `json:"offset"`
	Limit      int       `json:"limit"`
	HasMore    bool      `json:"has_more"`
}

func (r *PatentSearchResult) PageCount() int {
	if r.Limit == 0 {
		return 0
	}
	if r.Total == 0 {
		return 0
	}
	return int(math.Ceil(float64(r.Total) / float64(r.Limit)))
}

func (r *PatentSearchResult) CurrentPage() int {
	if r.Limit == 0 {
		return 1
	}
	return (r.Offset / r.Limit) + 1
}

// PatentRepository defines the interface for patent persistence.
// Alias for backward compatibility
type Repository = PatentRepository

type PatentRepository interface {
	Save(ctx context.Context, patent *Patent) error
	FindByID(ctx context.Context, id string) (*Patent, error)
	FindByPatentNumber(ctx context.Context, patentNumber string) (*Patent, error)
	Delete(ctx context.Context, id string) error
	Exists(ctx context.Context, patentNumber string) (bool, error)

	SaveBatch(ctx context.Context, patents []*Patent) error
	FindByIDs(ctx context.Context, ids []string) ([]*Patent, error)
	FindByPatentNumbers(ctx context.Context, numbers []string) ([]*Patent, error)

	Search(ctx context.Context, criteria PatentSearchCriteria) (*PatentSearchResult, error)
	// New method for vector/similarity search support
	SearchBySimilarity(ctx context.Context, req *SimilaritySearchRequest) ([]*PatentSearchResultWithSimilarity, error)

	FindByMoleculeID(ctx context.Context, moleculeID string) ([]*Patent, error)
	FindByFamilyID(ctx context.Context, familyID string) ([]*Patent, error)
	FindByIPCCode(ctx context.Context, ipcCode string) ([]*Patent, error)
	FindByApplicant(ctx context.Context, applicantName string) ([]*Patent, error)
	FindCitedBy(ctx context.Context, patentNumber string) ([]*Patent, error)
	FindCiting(ctx context.Context, patentNumber string) ([]*Patent, error)

	CountByStatus(ctx context.Context) (map[PatentStatus]int64, error)
	CountByOffice(ctx context.Context) (map[PatentOffice]int64, error)
	CountByIPCSection(ctx context.Context) (map[string]int64, error)
	CountByYear(ctx context.Context, field string) (map[int]int64, error)

	FindExpiringBefore(ctx context.Context, date time.Time) ([]*Patent, error)
	FindActiveByIPCCode(ctx context.Context, ipcCode string) ([]*Patent, error)
	FindWithMarkushStructures(ctx context.Context, offset, limit int) ([]*Patent, error)
}

// MarkushRepository defines the interface for markush structure persistence.
type MarkushRepository interface {
	Save(ctx context.Context, markush *MarkushStructure) error
	FindByID(ctx context.Context, id string) (*MarkushStructure, error)
	FindByPatentID(ctx context.Context, patentID string) ([]*MarkushStructure, error)
	FindByClaimNumber(ctx context.Context, patentID string, claimNumber int) ([]*MarkushStructure, error)
	FindMatchingMolecule(ctx context.Context, smiles string) ([]*MarkushStructure, error)
	Delete(ctx context.Context, id string) error
	CountByPatentID(ctx context.Context, patentID string) (int64, error)
}

// PatentEventRepository defines the interface for event persistence.
type PatentEventRepository interface {
	SaveEvents(ctx context.Context, events ...DomainEvent) error
	FindByAggregateID(ctx context.Context, aggregateID string) ([]DomainEvent, error)
	FindByEventType(ctx context.Context, eventType EventType, offset, limit int) ([]DomainEvent, error)
	FindSince(ctx context.Context, aggregateID string, version int) ([]DomainEvent, error)
}

//Personal.AI order the ending
