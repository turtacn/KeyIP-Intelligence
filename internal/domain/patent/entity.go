package patent

import (
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// PatentStatus represents the lifecycle stage of a patent.
type PatentStatus uint8

const (
	PatentStatusDraft             PatentStatus = 0
	PatentStatusFiled             PatentStatus = 1
	PatentStatusPublished         PatentStatus = 2
	PatentStatusUnderExamination  PatentStatus = 3
	PatentStatusGranted           PatentStatus = 4
	PatentStatusRejected          PatentStatus = 5
	PatentStatusWithdrawn         PatentStatus = 6
	PatentStatusExpired           PatentStatus = 7
	PatentStatusInvalidated       PatentStatus = 8
	PatentStatusLapsed            PatentStatus = 9
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

// PatentOffice represents a patent authority.
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

func (ipc *IPCClassification) Validate() error {
	if ipc.Full == "" {
		return errors.NewValidation("IPC code cannot be empty")
	}
	// Basic regex for IPC format e.g. "C09K 11/06"
	matched, _ := regexp.MatchString(`^[A-H]\d{2}[A-Z] \d{1,4}/\d{2,6}$`, ipc.Full)
	if !matched {
		return errors.NewValidation(fmt.Sprintf("invalid IPC format: %s", ipc.Full))
	}
	return nil
}

func (ipc *IPCClassification) String() string {
	return ipc.Full
}

// Applicant represents a patent applicant.
type Applicant struct {
	Name    string `json:"name"`
	Country string `json:"country"` // ISO 3166-1 alpha-2
	Type    string `json:"type"`    // "company", "individual", etc.
}

// Inventor represents a patent inventor.
type Inventor struct {
	Name        string `json:"name"`
	Country     string `json:"country"`
	Affiliation string `json:"affiliation,omitempty"`
}

// PatentDate encapsulates critical patent dates.
type PatentDate struct {
	FilingDate      *time.Time `json:"filing_date"`
	PublicationDate *time.Time `json:"publication_date,omitempty"`
	GrantDate       *time.Time `json:"grant_date,omitempty"`
	ExpiryDate      *time.Time `json:"expiry_date,omitempty"`
	PriorityDate    *time.Time `json:"priority_date,omitempty"`
}

func (d *PatentDate) RemainingLifeYears() float64 {
	if d.ExpiryDate == nil {
		return 0
	}
	now := time.Now().UTC()
	if now.After(*d.ExpiryDate) {
		return 0
	}
	duration := d.ExpiryDate.Sub(now)
	years := duration.Hours() / (24 * 365.25)
	return float64(int(years*100)) / 100 // Round to 2 decimal places
}

func (d *PatentDate) Validate() error {
	if d.FilingDate == nil {
		return errors.NewValidation("filing date is required")
	}
	if d.PublicationDate != nil && d.PublicationDate.Before(*d.FilingDate) {
		return errors.NewValidation("publication date cannot be before filing date")
	}
	if d.GrantDate != nil {
		if d.PublicationDate != nil && d.GrantDate.Before(*d.PublicationDate) {
			return errors.NewValidation("grant date cannot be before publication date")
		}
		if d.GrantDate.Before(*d.FilingDate) {
			return errors.NewValidation("grant date cannot be before filing date")
		}
	}
	if d.ExpiryDate != nil {
		if d.GrantDate != nil && d.ExpiryDate.Before(*d.GrantDate) {
			return errors.NewValidation("expiry date cannot be before grant date")
		}
		if d.ExpiryDate.Before(*d.FilingDate) {
			return errors.NewValidation("expiry date cannot be before filing date")
		}
	}
	return nil
}

// Patent is the aggregate root entity for a patent.
type Patent struct {
	ID                string              `json:"id"`
	PatentNumber      string              `json:"patent_number"`
	Title             string              `json:"title"`
	Abstract          string              `json:"abstract"`
	Office            PatentOffice        `json:"office"`
	Status            PatentStatus        `json:"status"`
	Dates             PatentDate          `json:"dates"`
	Applicants        []Applicant         `json:"applicants"`
	Inventors         []Inventor          `json:"inventors"`
	IPCCodes          []IPCClassification `json:"ipc_codes"`
	Claims            ClaimSet            `json:"claims,omitempty"`
	MoleculeIDs       []string            `json:"molecule_ids,omitempty"`
	FamilyID          string              `json:"family_id,omitempty"`
	PriorityNumbers   []string            `json:"priority_numbers,omitempty"`
	CitedBy           []string            `json:"cited_by,omitempty"`
	Cites             []string            `json:"cites,omitempty"`
	FullText          string              `json:"full_text,omitempty"`
	Language          string              `json:"language"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
	Version           int                 `json:"version"`

	// Deprecated fields that might be used by existing consumers until refactored
	AssigneeName      string              `json:"assignee_name,omitempty"`
	Jurisdiction      string              `json:"jurisdiction,omitempty"`
	ApplicationNumber string              `json:"application_number,omitempty"`
	CPCCodes          []string            `json:"cpc_codes,omitempty"`

	// Domain events
	domainEvents []DomainEvent
}

// NewPatent creates a new Patent entity.
func NewPatent(patentNumber, title string, office PatentOffice, filingDate time.Time) (*Patent, error) {
	if patentNumber == "" {
		return nil, errors.NewValidation("patent number cannot be empty")
	}
	if title == "" {
		return nil, errors.NewValidation("title cannot be empty")
	}
	if len(title) > 1000 {
		return nil, errors.NewValidation("title too long")
	}
	if !office.IsValid() {
		return nil, errors.NewValidation("invalid patent office")
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
		Metadata:     make(map[string]interface{}),
		CreatedAt:    now,
		UpdatedAt:    now,
		Version:      1,
		Jurisdiction: string(office),
		domainEvents: make([]DomainEvent, 0),
	}
	// Note: Creation event is typically raised by service after saving, or here if using pure DDD.
	// The service.go spec says: "CreatePatent... publishes PatentCreatedEvent".
	// So we don't need to add it here, or we can.
	// But usually constructor returns entity, service saves it and publishes event.
	// The spec for `NewPatent` doesn't mention adding event.
	return p, nil
}

// Validate checks the consistency of the patent aggregate.
func (p *Patent) Validate() error {
	if p.PatentNumber == "" {
		return errors.NewValidation("patent number cannot be empty")
	}
	if p.Title == "" {
		return errors.NewValidation("title cannot be empty")
	}
	if !p.Office.IsValid() {
		return errors.NewValidation("invalid patent office")
	}
	if !p.Status.IsValid() {
		return errors.NewValidation("invalid patent status")
	}
	if err := p.Dates.Validate(); err != nil {
		return err
	}
	if len(p.Applicants) == 0 {
		return errors.NewValidation("at least one applicant required")
	}
	if len(p.Inventors) == 0 {
		return errors.NewValidation("at least one inventor required")
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

// Domain Events methods

func (p *Patent) DomainEvents() []DomainEvent {
	return p.domainEvents
}

func (p *Patent) ClearEvents() {
	p.domainEvents = []DomainEvent{}
}

func (p *Patent) addEvent(event DomainEvent) {
	p.domainEvents = append(p.domainEvents, event)
}

// State transition methods

func (p *Patent) Publish(publicationDate time.Time) error {
	if p.Status != PatentStatusFiled {
		return errors.NewValidation("can only publish from Filed status")
	}
	p.Status = PatentStatusPublished
	p.Dates.PublicationDate = &publicationDate
	p.updateTimestamp()
	p.addEvent(NewPatentPublishedEvent(p))
	return nil
}

func (p *Patent) EnterExamination() error {
	if p.Status != PatentStatusPublished {
		return errors.NewValidation("can only enter examination from Published status")
	}
	p.Status = PatentStatusUnderExamination
	p.updateTimestamp()
	p.addEvent(NewPatentExaminationStartedEvent(p))
	return nil
}

func (p *Patent) Grant(grantDate, expiryDate time.Time) error {
	if p.Status != PatentStatusUnderExamination {
		return errors.NewValidation("can only grant from UnderExamination status")
	}
	p.Status = PatentStatusGranted
	p.Dates.GrantDate = &grantDate
	p.Dates.ExpiryDate = &expiryDate
	p.updateTimestamp()
	p.addEvent(NewPatentGrantedEvent(p))
	return nil
}

func (p *Patent) Reject() error {
	if p.Status != PatentStatusUnderExamination {
		return errors.NewValidation("can only reject from UnderExamination status")
	}
	p.Status = PatentStatusRejected
	p.updateTimestamp()
	p.addEvent(NewPatentRejectedEvent(p, "Rejected during examination"))
	return nil
}

func (p *Patent) Withdraw() error {
	if p.Status.IsTerminal() || p.Status == PatentStatusGranted {
		return errors.NewValidation("cannot withdraw from current status")
	}
	prevStatus := p.Status
	p.Status = PatentStatusWithdrawn
	p.updateTimestamp()
	p.addEvent(NewPatentWithdrawnEvent(p, prevStatus))
	return nil
}

func (p *Patent) Expire() error {
	if p.Status != PatentStatusGranted {
		return errors.NewValidation("can only expire from Granted status")
	}
	p.Status = PatentStatusExpired
	p.updateTimestamp()
	p.addEvent(NewPatentExpiredEvent(p))
	return nil
}

func (p *Patent) Invalidate() error {
	if p.Status != PatentStatusGranted {
		return errors.NewValidation("can only invalidate from Granted status")
	}
	p.Status = PatentStatusInvalidated
	p.updateTimestamp()
	p.addEvent(NewPatentInvalidatedEvent(p, "Invalidated by legal process"))
	return nil
}

func (p *Patent) Lapse() error {
	if p.Status != PatentStatusGranted {
		return errors.NewValidation("can only lapse from Granted status")
	}
	p.Status = PatentStatusLapsed
	p.updateTimestamp()
	p.addEvent(NewPatentLapsedEvent(p))
	return nil
}

func (p *Patent) updateTimestamp() {
	p.UpdatedAt = time.Now().UTC()
	p.Version++
}

// Association methods

func (p *Patent) AddMolecule(moleculeID string) error {
	if moleculeID == "" {
		return errors.NewValidation("molecule ID cannot be empty")
	}
	for _, id := range p.MoleculeIDs {
		if id == moleculeID {
			return errors.NewValidation("duplicate molecule ID")
		}
	}
	p.MoleculeIDs = append(p.MoleculeIDs, moleculeID)
	p.updateTimestamp()
	p.addEvent(NewPatentMoleculeLinkedEvent(p, moleculeID))
	return nil
}

func (p *Patent) RemoveMolecule(moleculeID string) error {
	for i, id := range p.MoleculeIDs {
		if id == moleculeID {
			p.MoleculeIDs = append(p.MoleculeIDs[:i], p.MoleculeIDs[i+1:]...)
			p.updateTimestamp()
			p.addEvent(NewPatentMoleculeUnlinkedEvent(p, moleculeID))
			return nil
		}
	}
	return errors.NewValidation("molecule ID not found")
}

func (p *Patent) AddCitation(patentNumber string) error {
	for _, n := range p.Cites {
		if n == patentNumber {
			return errors.NewValidation("duplicate citation")
		}
	}
	p.Cites = append(p.Cites, patentNumber)
	p.updateTimestamp()
	p.addEvent(NewPatentCitationAddedEvent(p, patentNumber, "forward"))
	return nil
}

func (p *Patent) AddCitedBy(patentNumber string) error {
	for _, n := range p.CitedBy {
		if n == patentNumber {
			return errors.NewValidation("duplicate cited by")
		}
	}
	p.CitedBy = append(p.CitedBy, patentNumber)
	p.updateTimestamp()
	p.addEvent(NewPatentCitationAddedEvent(p, patentNumber, "backward"))
	return nil
}

func (p *Patent) SetClaims(claims ClaimSet) error {
	if err := claims.Validate(); err != nil {
		return err
	}
	p.Claims = claims
	p.updateTimestamp()
	p.addEvent(NewPatentClaimsUpdatedEvent(p))
	return nil
}

// Query methods

func (p *Patent) IsActive() bool {
	return p.Status.IsActive()
}

func (p *Patent) IsGranted() bool {
	return p.Status == PatentStatusGranted
}

func (p *Patent) RemainingLife() float64 {
	return p.Dates.RemainingLifeYears()
}

func (p *Patent) HasMolecule(moleculeID string) bool {
	for _, id := range p.MoleculeIDs {
		if id == moleculeID {
			return true
		}
	}
	return false
}

func (p *Patent) ClaimCount() int {
	return len(p.Claims)
}

func (p *Patent) IndependentClaimCount() int {
	return len(p.Claims.IndependentClaims())
}

//Personal.AI order the ending
