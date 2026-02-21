package patent

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
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
		return "Draft"
	case PatentStatusFiled:
		return "Filed"
	case PatentStatusPublished:
		return "Published"
	case PatentStatusUnderExamination:
		return "UnderExamination"
	case PatentStatusGranted:
		return "Granted"
	case PatentStatusRejected:
		return "Rejected"
	case PatentStatusWithdrawn:
		return "Withdrawn"
	case PatentStatusExpired:
		return "Expired"
	case PatentStatusInvalidated:
		return "Invalidated"
	case PatentStatusLapsed:
		return "Lapsed"
	default:
		return "Unknown"
	}
}

func (s PatentStatus) IsValid() bool {
	return s >= PatentStatusDraft && s <= PatentStatusLapsed
}

func (s PatentStatus) IsActive() bool {
	return s == PatentStatusFiled || s == PatentStatusPublished || s == PatentStatusUnderExamination || s == PatentStatusGranted
}

func (s PatentStatus) IsTerminal() bool {
	return s == PatentStatusRejected || s == PatentStatusWithdrawn || s == PatentStatusExpired || s == PatentStatusInvalidated || s == PatentStatusLapsed
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

// IPCClassification represents an International Patent Classification code.
type IPCClassification struct {
	Section  string `json:"section"`
	Class    string `json:"class"`
	Subclass string `json:"subclass"`
	Group    string `json:"group"`
	Subgroup string `json:"subgroup"`
	Full     string `json:"full"`
}

func (c IPCClassification) Validate() error {
	if c.Full == "" {
		return errors.InvalidParam("IPC full code cannot be empty")
	}
	// Basic format validation can be added here
	return nil
}

func (c IPCClassification) String() string {
	return c.Full
}

// Applicant represents a patent applicant.
type Applicant struct {
	Name    string `json:"name"`
	Country string `json:"country"`
	Type    string `json:"type"` // company, individual, university, research_institute
}

// Inventor represents a patent inventor.
type Inventor struct {
	Name        string `json:"name"`
	Country     string `json:"country"`
	Affiliation string `json:"affiliation"`
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
	if d.PublicationDate != nil && d.PublicationDate.Before(*d.FilingDate) {
		return errors.InvalidParam("publication date cannot be before filing date")
	}
	if d.GrantDate != nil {
		if d.PublicationDate != nil && d.GrantDate.Before(*d.PublicationDate) {
			return errors.InvalidParam("grant date cannot be before publication date")
		}
		if d.GrantDate.Before(*d.FilingDate) {
			return errors.InvalidParam("grant date cannot be before filing date")
		}
	}
	if d.ExpiryDate != nil && d.GrantDate != nil && d.ExpiryDate.Before(*d.GrantDate) {
		return errors.InvalidParam("expiry date cannot be before grant date")
	}
	return nil
}

// Patent is the aggregate root for a patent.
type Patent struct {
	ID              string              `json:"id"`
	PatentNumber    string              `json:"patent_number"`
	Title           string              `json:"title"`
	Abstract        string              `json:"abstract"`
	Office          PatentOffice        `json:"office"`
	Status          PatentStatus        `json:"status"`
	Dates           PatentDate          `json:"dates"`
	Applicants      []Applicant         `json:"applicants"`
	Inventors       []Inventor          `json:"inventors"`
	IPCCodes        []IPCClassification `json:"ipc_codes"`
	Claims          ClaimSet            `json:"claims"`
	MoleculeIDs     []string            `json:"molecule_ids"`
	FamilyID        string              `json:"family_id"`
	PriorityNumbers []string            `json:"priority_numbers"`
	CitedBy         []string            `json:"cited_by"`
	Cites           []string            `json:"cites"`
	FullText        string              `json:"full_text"`
	Language        string              `json:"language"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	Version         int                 `json:"version"`

	// Domain events collector
	domainEvents []DomainEvent
}

// NewPatent creates a new patent instance.
func NewPatent(patentNumber, title string, office PatentOffice, filingDate time.Time) (*Patent, error) {
	if patentNumber == "" {
		return nil, errors.InvalidParam("patent number cannot be empty")
	}
	if title == "" || len(title) > 1000 {
		return nil, errors.InvalidParam("title must be non-empty and less than 1000 characters")
	}
	if !office.IsValid() {
		return nil, errors.InvalidParam("invalid patent office")
	}

	now := time.Now().UTC()
	p := &Patent{
		ID:           uuid.New().String(),
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
	}
	return p, nil
}

// Validate checks the patent invariants.
func (p *Patent) Validate() error {
	if p.PatentNumber == "" {
		return errors.InvalidParam("patent number is required")
	}
	if p.Title == "" {
		return errors.InvalidParam("title is required")
	}
	if !p.Office.IsValid() {
		return errors.InvalidParam("invalid office")
	}
	if !p.Status.IsValid() {
		return errors.InvalidParam("invalid status")
	}
	if err := p.Dates.Validate(); err != nil {
		return err
	}
	if len(p.Applicants) == 0 {
		return errors.InvalidParam("at least one applicant is required")
	}
	if len(p.Inventors) == 0 {
		return errors.InvalidParam("at least one inventor is required")
	}
	if len(p.Claims) > 0 {
		if err := p.Claims.Validate(); err != nil {
			return err
		}
	}
	for _, ipc := range p.IPCCodes {
		if err := ipc.Validate(); err != nil {
			return err
		}
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
	if p.Status == PatentStatusGranted || p.Status.IsTerminal() {
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

// AddMolecule associates a molecule with the patent.
func (p *Patent) AddMolecule(moleculeID string) error {
	if moleculeID == "" {
		return errors.InvalidParam("molecule ID cannot be empty")
	}
	for _, id := range p.MoleculeIDs {
		if id == moleculeID {
			return errors.InvalidParam("molecule already associated")
		}
	}
	p.MoleculeIDs = append(p.MoleculeIDs, moleculeID)
	p.touch()
	return nil
}

// RemoveMolecule removes a molecule association.
func (p *Patent) RemoveMolecule(moleculeID string) error {
	for i, id := range p.MoleculeIDs {
		if id == moleculeID {
			p.MoleculeIDs = append(p.MoleculeIDs[:i], p.MoleculeIDs[i+1:]...)
			p.touch()
			return nil
		}
	}
	return errors.NotFound(fmt.Sprintf("molecule ID %s not associated", moleculeID))
}

// AddCitation adds a cited patent.
func (p *Patent) AddCitation(patentNumber string) error {
	if patentNumber == "" {
		return errors.InvalidParam("patent number cannot be empty")
	}
	for _, cn := range p.Cites {
		if cn == patentNumber {
			return errors.InvalidParam("citation already exists")
		}
	}
	p.Cites = append(p.Cites, patentNumber)
	p.touch()
	return nil
}

// AddCitedBy adds a patent that cites this one.
func (p *Patent) AddCitedBy(patentNumber string) error {
	if patentNumber == "" {
		return errors.InvalidParam("patent number cannot be empty")
	}
	for _, cn := range p.CitedBy {
		if cn == patentNumber {
			return errors.InvalidParam("citation already exists")
		}
	}
	p.CitedBy = append(p.CitedBy, patentNumber)
	p.touch()
	return nil
}

// SetClaims sets the claim set for the patent.
func (p *Patent) SetClaims(claims ClaimSet) error {
	if err := claims.Validate(); err != nil {
		return err
	}
	p.Claims = claims
	p.touch()
	return nil
}

// IsActive returns true if the patent is in an active legal state.
func (p *Patent) IsActive() bool {
	return p.Status.IsActive()
}

// IsGranted returns true if the patent is granted.
func (p *Patent) IsGranted() bool {
	return p.Status == PatentStatusGranted
}

// RemainingLife returns the remaining life in years.
func (p *Patent) RemainingLife() float64 {
	return p.Dates.RemainingLifeYears()
}

// HasMolecule checks if the patent is associated with a specific molecule.
func (p *Patent) HasMolecule(moleculeID string) bool {
	for _, id := range p.MoleculeIDs {
		if id == moleculeID {
			return true
		}
	}
	return false
}

// ClaimCount returns the total number of claims.
func (p *Patent) ClaimCount() int {
	return len(p.Claims)
}

// IndependentClaimCount returns the number of independent claims.
func (p *Patent) IndependentClaimCount() int {
	count := 0
	for _, c := range p.Claims {
		if c.Type == ClaimTypeIndependent {
			count++
		}
	}
	return count
}

func (p *Patent) touch() {
	p.UpdatedAt = time.Now().UTC()
	p.Version++
}

// DomainEvents returns the list of uncommitted domain events.
func (p *Patent) DomainEvents() []DomainEvent {
	return p.domainEvents
}

// ClearEvents clears the domain events collector.
func (p *Patent) ClearEvents() {
	p.domainEvents = nil
}

func (p *Patent) addEvent(event DomainEvent) {
	p.domainEvents = append(p.domainEvents, event)
}

//Personal.AI order the ending
