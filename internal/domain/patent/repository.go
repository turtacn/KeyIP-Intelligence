package patent

import (
	"context"
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// PatentSearchCriteria encapsulates filters for querying patents.
type PatentSearchCriteria struct {
	PatentNumbers     []string       `json:"patent_numbers"`
	TitleKeywords     []string       `json:"title_keywords"`
	AbstractKeywords  []string       `json:"abstract_keywords"`
	FullTextKeywords  []string       `json:"full_text_keywords"`
	Offices           []PatentOffice `json:"offices"`
	Statuses          []PatentStatus `json:"statuses"`
	IPCCodes          []string       `json:"ipc_codes"`
	ApplicantNames    []string       `json:"applicant_names"`
	InventorNames     []string       `json:"inventor_names"`
	MoleculeIDs       []string       `json:"molecule_ids"`
	FamilyID          string         `json:"family_id"`
	FilingDateFrom    *time.Time     `json:"filing_date_from"`
	FilingDateTo      *time.Time     `json:"filing_date_to"`
	GrantDateFrom     *time.Time     `json:"grant_date_from"`
	GrantDateTo       *time.Time     `json:"grant_date_to"`
	ExpiryDateFrom    *time.Time     `json:"expiry_date_from"`
	ExpiryDateTo      *time.Time     `json:"expiry_date_to"`
	HasMarkushStructure *bool        `json:"has_markush_structure"`
	MinClaimCount     *int           `json:"min_claim_count"`
	MaxClaimCount     *int           `json:"max_claim_count"`
	Language          string         `json:"language"`
	SortBy            string         `json:"sort_by"`    // "filing_date", "grant_date", "relevance"
	SortOrder         string         `json:"sort_order"` // "asc", "desc"
	Offset            int            `json:"offset"`
	Limit             int            `json:"limit"`
}

func (c PatentSearchCriteria) Validate() error {
	if c.Limit > 1000 {
		return errors.InvalidParam("limit cannot exceed 1000")
	}
	if c.Limit < 0 {
		return errors.InvalidParam("limit cannot be negative")
	}
	if c.Offset < 0 {
		return errors.InvalidParam("offset cannot be negative")
	}
	if c.SortBy != "" && c.SortBy != "filing_date" && c.SortBy != "grant_date" && c.SortBy != "relevance" && c.SortBy != "citation_count" {
		return errors.InvalidParam("invalid sort field")
	}
	if c.SortOrder != "" && c.SortOrder != "asc" && c.SortOrder != "desc" {
		return errors.InvalidParam("invalid sort order")
	}
	if c.FilingDateFrom != nil && c.FilingDateTo != nil && c.FilingDateFrom.After(*c.FilingDateTo) {
		return errors.InvalidParam("filing date from cannot be after filing date to")
	}
	// Similar checks for other date ranges could be added
	return nil
}

func (c PatentSearchCriteria) HasFilters() bool {
	return len(c.PatentNumbers) > 0 || len(c.TitleKeywords) > 0 || len(c.AbstractKeywords) > 0 ||
		len(c.FullTextKeywords) > 0 || len(c.Offices) > 0 || len(c.Statuses) > 0 ||
		len(c.IPCCodes) > 0 || len(c.ApplicantNames) > 0 || len(c.InventorNames) > 0 ||
		len(c.MoleculeIDs) > 0 || c.FamilyID != "" ||
		c.FilingDateFrom != nil || c.FilingDateTo != nil ||
		c.GrantDateFrom != nil || c.GrantDateTo != nil ||
		c.ExpiryDateFrom != nil || c.ExpiryDateTo != nil ||
		c.HasMarkushStructure != nil || c.MinClaimCount != nil || c.MaxClaimCount != nil ||
		c.Language != ""
}

// PatentSearchResult wraps search results with pagination info.
type PatentSearchResult struct {
	Patents []*Patent `json:"patents"`
	Total   int64     `json:"total"`
	Offset  int       `json:"offset"`
	Limit   int       `json:"limit"`
	HasMore bool      `json:"has_more"`
}

func (r PatentSearchResult) PageCount() int {
	if r.Limit <= 0 {
		if r.Total > 0 {
			return 0 // Or 1? Conventionally 0 if unlimited/unknown or undefined behavior
		}
		return 0
	}
	if r.Total == 0 {
		return 0
	}
	return int((r.Total + int64(r.Limit) - 1) / int64(r.Limit))
}

func (r PatentSearchResult) CurrentPage() int {
	if r.Limit <= 0 {
		return 1
	}
	return (r.Offset / r.Limit) + 1
}

// PatentRepository defines the interface for patent data access.
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

	// Legacy methods aliases if needed by existing code calling PatentRepository
	// ... (Implementation specific, interface should be clean)
}

// MarkushRepository defines the interface for Markush structure data access.
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
	SaveEvents(ctx context.Context, events ...common.DomainEvent) error
	FindByAggregateID(ctx context.Context, aggregateID string) ([]common.DomainEvent, error)
	FindByEventType(ctx context.Context, eventType common.EventType, offset, limit int) ([]common.DomainEvent, error)
	FindSince(ctx context.Context, aggregateID string, version int) ([]common.DomainEvent, error)
}

// Alias for cleaner usage
type Repository = PatentRepository

// Errors
var (
	ErrPatentNotFound = fmt.Errorf("patent not found")
)

//Personal.AI order the ending
