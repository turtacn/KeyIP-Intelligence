package patent

import (
	"context"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// PatentSearchCriteria defines the filters for searching patents.
type PatentSearchCriteria struct {
	PatentNumbers    []string       `json:"patent_numbers,omitempty"`
	TitleKeywords    []string       `json:"title_keywords,omitempty"`
	AbstractKeywords []string       `json:"abstract_keywords,omitempty"`
	FullTextKeywords []string       `json:"full_text_keywords,omitempty"`
	Offices          []PatentOffice `json:"offices,omitempty"`
	Statuses         []PatentStatus `json:"statuses,omitempty"`
	IPCCodes         []string       `json:"ipc_codes,omitempty"`
	ApplicantNames   []string       `json:"applicant_names,omitempty"`
	InventorNames    []string       `json:"inventor_names,omitempty"`
	MoleculeIDs      []string       `json:"molecule_ids,omitempty"`
	FamilyID         string         `json:"family_id,omitempty"`
	FilingDateFrom   *time.Time     `json:"filing_date_from,omitempty"`
	FilingDateTo     *time.Time     `json:"filing_date_to,omitempty"`
	GrantDateFrom    *time.Time     `json:"grant_date_from,omitempty"`
	GrantDateTo      *time.Time     `json:"grant_date_to,omitempty"`
	ExpiryDateFrom   *time.Time     `json:"expiry_date_from,omitempty"`
	ExpiryDateTo     *time.Time     `json:"expiry_date_to,omitempty"`
	HasMarkushStructure *bool       `json:"has_markush_structure,omitempty"`
	MinClaimCount    *int           `json:"min_claim_count,omitempty"`
	MaxClaimCount    *int           `json:"max_claim_count,omitempty"`
	Language         string         `json:"language,omitempty"`
	SortBy           string         `json:"sort_by,omitempty"`
	SortOrder        string         `json:"sort_order,omitempty"`
	Offset           int            `json:"offset"`
	Limit            int            `json:"limit"`
}

func (c PatentSearchCriteria) Validate() error {
	if c.Limit < 0 {
		return errors.InvalidParam("limit cannot be negative")
	}
	if c.Limit > 1000 {
		return errors.InvalidParam("limit cannot exceed 1000")
	}
	if c.Offset < 0 {
		return errors.InvalidParam("offset cannot be negative")
	}
	if c.FilingDateFrom != nil && c.FilingDateTo != nil && c.FilingDateFrom.After(*c.FilingDateTo) {
		return errors.InvalidParam("filing date from cannot be after filing date to")
	}

	validSortBy := map[string]bool{
		"":               true,
		"filing_date":    true,
		"grant_date":     true,
		"relevance":      true,
		"citation_count": true,
	}
	if !validSortBy[c.SortBy] {
		return errors.InvalidParam("invalid sort_by field: " + c.SortBy)
	}

	validSortOrder := map[string]bool{
		"":     true,
		"asc":  true,
		"desc": true,
	}
	if !validSortOrder[c.SortOrder] {
		return errors.InvalidParam("invalid sort_order: " + c.SortOrder)
	}

	return nil
}

func (c PatentSearchCriteria) HasFilters() bool {
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
		c.FilingDateTo != nil ||
		c.GrantDateFrom != nil ||
		c.GrantDateTo != nil ||
		c.ExpiryDateFrom != nil ||
		c.ExpiryDateTo != nil ||
		c.HasMarkushStructure != nil ||
		c.MinClaimCount != nil ||
		c.MaxClaimCount != nil ||
		c.Language != ""
}

// PatentSearchResult represents the result of a patent search.
type PatentSearchResult struct {
	Patents  []*Patent `json:"patents"`
	Total    int64     `json:"total"`
	Offset   int       `json:"offset"`
	Limit    int       `json:"limit"`
	HasMore  bool      `json:"has_more"`
}

func (r PatentSearchResult) PageCount() int {
	if r.Limit <= 0 {
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

// PatentRepository defines the interface for patent persistence.
type PatentRepository interface {
	Save(ctx context.Context, patent *Patent) error
	FindByID(ctx context.Context, id string) (*Patent, error)
	FindByPatentNumber(ctx context.Context, patentNumber string) (*Patent, error)
	Delete(ctx context.Context, id string) error // Soft delete
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
}

// MarkushRepository defines the interface for Markush structure persistence.
type MarkushRepository interface {
	Save(ctx context.Context, markush *MarkushStructure) error
	FindByID(ctx context.Context, id string) (*MarkushStructure, error)
	FindByPatentID(ctx context.Context, patentID string) ([]*MarkushStructure, error)
	FindByClaimNumber(ctx context.Context, patentID string, claimNumber int) ([]*MarkushStructure, error)
	FindMatchingMolecule(ctx context.Context, smiles string) ([]*MarkushStructure, error)
	Delete(ctx context.Context, id string) error
	CountByPatentID(ctx context.Context, patentID string) (int64, error)
}

// PatentEventRepository defines the interface for patent domain event persistence.
type PatentEventRepository interface {
	SaveEvents(ctx context.Context, events ...DomainEvent) error
	FindByAggregateID(ctx context.Context, aggregateID string) ([]DomainEvent, error)
	FindByEventType(ctx context.Context, eventType EventType, offset, limit int) ([]DomainEvent, error)
	FindSince(ctx context.Context, aggregateID string, version int) ([]DomainEvent, error)
}

//Personal.AI order the ending
