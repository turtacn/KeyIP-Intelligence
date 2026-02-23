package patent

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// PatentStatus represents the legal lifecycle stage of a patent.
type PatentStatus uint8

const (
	PatentStatusUnknown          PatentStatus = 0
	PatentStatusDraft            PatentStatus = 1
	PatentStatusFiled            PatentStatus = 2
	PatentStatusPublished        PatentStatus = 3
	PatentStatusUnderExamination PatentStatus = 4
	PatentStatusGranted          PatentStatus = 5
	PatentStatusRejected         PatentStatus = 6
	PatentStatusWithdrawn        PatentStatus = 7
	PatentStatusExpired          PatentStatus = 8
	PatentStatusInvalidated       PatentStatus = 9
	PatentStatusLapsed           PatentStatus = 10
)

func (s PatentStatus) String() string {
	switch s {
	case PatentStatusDraft:
		return "draft"
	case PatentStatusFiled:
		return "filed"
	case PatentStatusPublished:
		return "published"
	case PatentStatusUnderExamination:
		return "under_examination"
	case PatentStatusGranted:
		return "granted"
	case PatentStatusRejected:
		return "rejected"
	case PatentStatusWithdrawn:
		return "withdrawn"
	case PatentStatusExpired:
		return "expired"
	case PatentStatusInvalidated:
		return "invalidated"
	case PatentStatusLapsed:
		return "lapsed"
	default:
		return "unknown"
	}
}

func (s PatentStatus) IsValid() bool {
	return s >= PatentStatusDraft && s <= PatentStatusLapsed
}

func (s PatentStatus) IsActive() bool {
	return s == PatentStatusFiled || s == PatentStatusPublished || s == PatentStatusUnderExamination || s == PatentStatusGranted
}

// PatentOffice identifies a national or regional patent office.
type PatentOffice string

const (
	OfficeCNIPA PatentOffice = "CNIPA"
	OfficeUSPTO PatentOffice = "USPTO"
	OfficeEPO   PatentOffice = "EPO"
	OfficeJPO   PatentOffice = "JPO"
	OfficeKIPO  PatentOffice = "KIPO"
	OfficeWIPO  PatentOffice = "WIPO"
)

func (o PatentOffice) IsValid() bool {
	switch o {
	case OfficeCNIPA, OfficeUSPTO, OfficeEPO, OfficeJPO, OfficeKIPO, OfficeWIPO:
		return true
	default:
		return false
	}
}

// Applicant represents a patent applicant.
type Applicant struct {
	Name    string `json:"name"`
	Country string `json:"country"`
	Type    string `json:"type"`
}

// Inventor represents a patent inventor.
type Inventor struct {
	Name        string `json:"name"`
	Country     string `json:"country"`
	Affiliation string `json:"affiliation"`
	Sequence    int    `json:"sequence"`
}

// PriorityClaim represents a priority claim.
type PriorityClaim struct {
	ID             uuid.UUID `json:"id"`
	PatentID       uuid.UUID `json:"patent_id"`
	PriorityNumber string    `json:"priority_number"`
	PriorityDate   time.Time `json:"priority_date"`
	PriorityCountry string   `json:"priority_country"`
}

// PatentDate encapsulates important lifecycle dates.
type PatentDate struct {
	FilingDate      *time.Time `json:"filing_date"`
	PublicationDate *time.Time `json:"publication_date"`
	GrantDate       *time.Time `json:"grant_date"`
	ExpiryDate      *time.Time `json:"expiry_date"`
	PriorityDate    *time.Time `json:"priority_date"`
}

func (d PatentDate) RemainingLifeYears() float64 {
	if d.ExpiryDate == nil {
		return 0
	}
	now := time.Now().UTC()
	if now.After(*d.ExpiryDate) {
		return 0
	}
	duration := d.ExpiryDate.Sub(now)
	return duration.Hours() / (24 * 365.25)
}

func (d PatentDate) Validate() error {
	if d.FilingDate == nil {
		return errors.InvalidParam("filing date is required")
	}
	return nil
}

// Patent is the aggregate root for a patent.
type Patent struct {
	ID              uuid.UUID           `json:"id"`
	PatentNumber    string              `json:"patent_number"`
	Title           string              `json:"title"`
	TitleEn         string              `json:"title_en,omitempty"`
	Abstract        string              `json:"abstract"`
	AbstractEn      string              `json:"abstract_en,omitempty"`
	Type            string              `json:"patent_type"`
	Status          PatentStatus        `json:"status"`
	Office          PatentOffice        `json:"office"`
	Dates           PatentDate          `json:"dates"`
	FilingDate      *time.Time          `json:"filing_date,omitempty"` // Kept for DB compat
	PublicationDate *time.Time          `json:"publication_date,omitempty"`
	GrantDate       *time.Time          `json:"grant_date,omitempty"`
	ExpiryDate      *time.Time          `json:"expiry_date,omitempty"`
	PriorityDate    *time.Time          `json:"priority_date,omitempty"`
	AssigneeID      *uuid.UUID          `json:"assignee_id,omitempty"`
	AssigneeName    string              `json:"assignee_name,omitempty"`
	Jurisdiction    string              `json:"jurisdiction"`
	IPCCodes        []string            `json:"ipc_codes"`
	CPCCodes        []string            `json:"cpc_codes"`
	KeyIPTechCodes  []string            `json:"keyip_tech_codes"`
	FamilyID        string              `json:"family_id,omitempty"`
	ApplicationNumber string            `json:"application_number,omitempty"`
	FullTextHash    string              `json:"full_text_hash,omitempty"`
	Source          string              `json:"source"`
	RawData         map[string]any      `json:"raw_data,omitempty"`
	Metadata        map[string]any      `json:"metadata,omitempty"`
	Claims          []*Claim            `json:"claims,omitempty"`
	Inventors       []*Inventor         `json:"inventors,omitempty"`
	PriorityClaims  []*PriorityClaim    `json:"priority_claims,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	DeletedAt       *time.Time          `json:"deleted_at,omitempty"`
	Version         int                 `json:"version"`

	domainEvents []common.DomainEvent
	CitedBy []string // Added for service
	Cites []string // Added for service
	MoleculeIDs []string // Added for service
}

// NewPatent creates a new patent instance.
func NewPatent(patentNumber, title string, office PatentOffice, filingDate time.Time) (*Patent, error) {
	if patentNumber == "" {
		return nil, errors.InvalidParam("patent number cannot be empty")
	}
	if title == "" {
		return nil, errors.InvalidParam("title cannot be empty")
	}
	if !office.IsValid() {
		return nil, errors.InvalidParam("invalid office")
	}

	now := time.Now().UTC()
	return &Patent{
		ID:           uuid.New(),
		PatentNumber: patentNumber,
		Title:        title,
		Office:       office,
		Status:       PatentStatusFiled,
		Dates: PatentDate{
			FilingDate: &filingDate,
		},
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}, nil
}

func (p *Patent) Validate() error {
	if p.PatentNumber == "" {
		return errors.InvalidParam("patent number cannot be empty")
	}
	return nil
}

// Publish transitions the patent from Filed to Published.
func (p *Patent) Publish(publicationDate time.Time) error {
	if p.Status != PatentStatusFiled {
		return errors.InvalidParam(fmt.Sprintf("cannot publish from status %s", p.Status))
	}
	p.Status = PatentStatusPublished
	p.Dates.PublicationDate = &publicationDate
	p.touch()
	return nil
}

// EnterExamination transitions the patent from Published to UnderExamination.
func (p *Patent) EnterExamination() error {
	if p.Status != PatentStatusPublished {
		return errors.InvalidParam(fmt.Sprintf("cannot enter examination from status %s", p.Status))
	}
	p.Status = PatentStatusUnderExamination
	p.touch()
	return nil
}

// Grant transitions the patent from UnderExamination to Granted.
func (p *Patent) Grant(grantDate, expiryDate time.Time) error {
	if p.Status != PatentStatusUnderExamination {
		return errors.InvalidParam(fmt.Sprintf("cannot grant from status %s", p.Status))
	}
	p.Status = PatentStatusGranted
	p.Dates.GrantDate = &grantDate
	p.Dates.ExpiryDate = &expiryDate
	p.touch()
	return nil
}

// Reject transitions the patent from UnderExamination to Rejected.
func (p *Patent) Reject() error {
	if p.Status != PatentStatusUnderExamination {
		return errors.InvalidParam(fmt.Sprintf("cannot reject from status %s", p.Status))
	}
	p.Status = PatentStatusRejected
	p.touch()
	return nil
}

// Withdraw transitions the patent to Withdrawn.
func (p *Patent) Withdraw() error {
	if p.Status == PatentStatusGranted {
		return errors.InvalidParam(fmt.Sprintf("cannot withdraw from status %s", p.Status))
	}
	p.Status = PatentStatusWithdrawn
	p.touch()
	return nil
}

// Expire transitions the patent from Granted to Expired.
func (p *Patent) Expire() error {
	if p.Status != PatentStatusGranted {
		return errors.InvalidParam(fmt.Sprintf("cannot expire from status %s", p.Status))
	}
	p.Status = PatentStatusExpired
	p.touch()
	return nil
}

// Invalidate transitions the patent from Granted to Invalidated.
func (p *Patent) Invalidate() error {
	if p.Status != PatentStatusGranted {
		return errors.InvalidParam(fmt.Sprintf("cannot invalidate from status %s", p.Status))
	}
	p.Status = PatentStatusInvalidated
	p.touch()
	return nil
}

// Lapse transitions the patent from Granted to Lapsed.
func (p *Patent) Lapse() error {
	if p.Status != PatentStatusGranted {
		return errors.InvalidParam(fmt.Sprintf("cannot lapse from status %s", p.Status))
	}
	p.Status = PatentStatusLapsed
	p.touch()
	return nil
}

func (p *Patent) AddMolecule(moleculeID string) error {
	p.MoleculeIDs = append(p.MoleculeIDs, moleculeID)
	p.touch()
	return nil
}

func (p *Patent) RemoveMolecule(moleculeID string) error {
	// simplified
	return nil
}

func (p *Patent) AddCitation(patentNumber string) error {
	p.Cites = append(p.Cites, patentNumber)
	p.touch()
	return nil
}

func (p *Patent) AddCitedBy(patentNumber string) error {
	p.CitedBy = append(p.CitedBy, patentNumber)
	p.touch()
	return nil
}

func (p *Patent) SetClaims(claims []*Claim) error {
	p.Claims = claims
	p.touch()
	return nil
}

func (p *Patent) touch() {
	p.UpdatedAt = time.Now().UTC()
	p.Version++
}

// DomainEvents returns the list of uncommitted domain events.
func (p *Patent) DomainEvents() []common.DomainEvent {
	return p.domainEvents
}

// ClearEvents clears the domain events collector.
func (p *Patent) ClearEvents() {
	p.domainEvents = nil
}

func (p *Patent) addEvent(event common.DomainEvent) {
	p.domainEvents = append(p.domainEvents, event)
}

// SearchQuery represents criteria for searching patents.
type SearchQuery struct {
	Keyword        string
	Status         *PatentStatus
	PatentType     string
	Jurisdiction   string
	FilingDateFrom *time.Time
	FilingDateTo   *time.Time
	AssigneeID     *uuid.UUID
	FamilyID       string
	IPCCode        string
	Limit          int
	Offset         int
	SortBy         string
}

// SearchResult contains search hits.
type SearchResult struct {
	Items      []*Patent
	TotalCount int64
	Facets     map[string]map[string]int64
}

// PatentSearchCriteria (alias for SearchQuery to fix service.go error if needed, or just rename in service.go)
type PatentSearchCriteria = SearchQuery
type PatentSearchResult = SearchResult

// ClaimCount returns the total number of claims.
func (p *Patent) ClaimCount() int {
	return len(p.Claims)
}

// GetPrimaryTechDomain returns the primary technology domain for the patent.
func (p *Patent) GetPrimaryTechDomain() string {
	if len(p.KeyIPTechCodes) > 0 {
		return p.KeyIPTechCodes[0]
	}
	if len(p.IPCCodes) > 0 {
		return p.IPCCodes[0]
	}
	return ""
}

// GetValueScore returns the value score from metadata, or 0 if not available.
func (p *Patent) GetValueScore() float64 {
	if p.Metadata != nil {
		if score, ok := p.Metadata["value_score"].(float64); ok {
			return score
		}
	}
	return 0.0
}

// GetFilingDate returns the filing date of the patent.
func (p *Patent) GetFilingDate() *time.Time {
	if p.Dates.FilingDate != nil {
		return p.Dates.FilingDate
	}
	return p.FilingDate
}

// GetLegalStatus returns the status as a string.
func (p *Patent) GetLegalStatus() string {
	return p.Status.String()
}

// GetAssignee returns the assignee name.
func (p *Patent) GetAssignee() string {
	return p.AssigneeName
}

// GetMoleculeIDs returns the list of associated molecule IDs.
func (p *Patent) GetMoleculeIDs() []string {
	return p.MoleculeIDs
}

// GetID returns the patent ID as a string.
func (p *Patent) GetID() string {
	return p.ID.String()
}

// GetPatentNumber returns the patent number.
func (p *Patent) GetPatentNumber() string {
	return p.PatentNumber
}

//Personal.AI order the ending
