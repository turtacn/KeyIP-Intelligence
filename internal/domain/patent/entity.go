// Package patent implements the Patent bounded context aggregate root, value
// objects, domain services, and invariant enforcement for the
// KeyIP-Intelligence platform.  All business rules that concern patents live
// here; infrastructure concerns (persistence, search) are handled by separate
// repository and adapter layers.
package patent

import (
	"fmt"
	"regexp"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// Patent number validation patterns per jurisdiction
// ─────────────────────────────────────────────────────────────────────────────

var (
	// rePatentCN matches CN application/publication numbers:
	//   CN202310001234A, CN202310001234B, CN1234567A, CN1234567C …
	rePatentCN = regexp.MustCompile(`^CN\d{7,}[A-Z]?\d?$`)

	// rePatentUS matches US application and granted patent numbers:
	//   US1234567, US20230012345A1, US10123456B2 …
	rePatentUS = regexp.MustCompile(`^US\d{6,}[A-Z]?\d?$`)

	// rePatentEP matches EP publication numbers:
	//   EP1234567, EP1234567A1, EP1234567B1 …
	rePatentEP = regexp.MustCompile(`^EP\d{6,}[A-Z]?\d?$`)

	// rePatentWO matches PCT/WIPO numbers: WO2023123456
	rePatentWO = regexp.MustCompile(`^WO\d{10,}$`)

	// rePatentJP matches JP numbers: JP2023123456A
	rePatentJP = regexp.MustCompile(`^JP\d{8,}[A-Z]?$`)

	// rePatentKR matches KR numbers: KR1020230012345
	rePatentKR = regexp.MustCompile(`^KR\d{10,}$`)
)

// validJurisdictions is the set of jurisdiction codes the platform currently
// supports.  Jurisdiction codes map 1:1 to the JurisdictionCode enum in
// pkg/types/patent.
var validJurisdictions = map[ptypes.JurisdictionCode]bool{
	ptypes.JurisdictionCN:     true,
	ptypes.JurisdictionUS:     true,
	ptypes.JurisdictionEP:     true,
	ptypes.JurisdictionWO:     true,
	ptypes.JurisdictionJP:     true,
	ptypes.JurisdictionKR:     true,
	ptypes.JurisdictionOther:  true,
}

// patentLifespanYears is the standard patent term in years from the filing
// date, keyed by jurisdiction.  Where a jurisdiction is absent the default
// 20-year term applies.
var patentLifespanYears = map[ptypes.JurisdictionCode]int{
	ptypes.JurisdictionCN: 20,
	ptypes.JurisdictionUS: 20,
	ptypes.JurisdictionEP: 20,
	ptypes.JurisdictionWO: 20,
	ptypes.JurisdictionJP: 20,
	ptypes.JurisdictionKR: 20,
}

const defaultPatentLifespanYears = 20

// ─────────────────────────────────────────────────────────────────────────────
// State machine: allowed status transitions
// ─────────────────────────────────────────────────────────────────────────────

// allowedTransitions defines the valid next states reachable from each status.
// Transitions not listed are illegal and will be rejected by UpdateStatus.
//
//   Filed ──► Published ──► Granted ──► Expired
//     │                        │
//     └──────► Abandoned       └──► Revoked
var allowedTransitions = map[ptypes.PatentStatus][]ptypes.PatentStatus{
	ptypes.StatusFiled: {
		ptypes.StatusPublished,
		ptypes.StatusAbandoned,
	},
	ptypes.StatusPublished: {
		ptypes.StatusGranted,
		ptypes.StatusAbandoned,
	},
	ptypes.StatusGranted: {
		ptypes.StatusExpired,
		ptypes.StatusRevoked,
	},
	// Terminal states: no outgoing transitions.
	ptypes.StatusExpired:   {},
	ptypes.StatusRevoked:   {},
	ptypes.StatusAbandoned: {},
}

// ─────────────────────────────────────────────────────────────────────────────
// Priority value object
// ─────────────────────────────────────────────────────────────────────────────

// Priority represents a priority claim referencing an earlier filing in
// another jurisdiction, as defined by the Paris Convention.
type Priority struct {
	// Country is the ISO 3166-1 alpha-2 country code of the priority filing.
	Country string `json:"country"`

	// Number is the application number of the priority filing.
	Number string `json:"number"`

	// Date is the filing date of the priority application.
	Date time.Time `json:"date"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Patent aggregate root
// ─────────────────────────────────────────────────────────────────────────────

// Patent is the aggregate root of the Patent bounded context.  It encapsulates
// all invariants related to a patent document: structural integrity of claims,
// Markush structures, status lifecycle, priority chains, and IPC/CPC
// classifications.
//
// Consumers of this type must never modify its fields directly; all mutations
// must go through the exported methods so that invariants and domain events are
// correctly maintained.
type Patent struct {
	// ── Identity and audit ───────────────────────────────────────────────────
	common.BaseEntity

	// ── Core bibliographic fields ─────────────────────────────────────────────
	PatentNumber string         `json:"patent_number"`
	Title        string         `json:"title"`
	Abstract     string         `json:"abstract"`
	Applicant    string         `json:"applicant"`
	Inventors    []string       `json:"inventors,omitempty"`
	FilingDate   time.Time      `json:"filing_date"`

	// ── Optional dates (set as the patent progresses through prosecution) ────
	PublicationDate *time.Time `json:"publication_date,omitempty"`
	GrantDate       *time.Time `json:"grant_date,omitempty"`
	ExpiryDate      *time.Time `json:"expiry_date,omitempty"`

	// ── Lifecycle and classification ─────────────────────────────────────────
	Status       ptypes.PatentStatus    `json:"status"`
	Jurisdiction ptypes.JurisdictionCode `json:"jurisdiction"`
	IPCCodes     []string               `json:"ipc_codes,omitempty"`
	CPCCodes     []string               `json:"cpc_codes,omitempty"`

	// ── Structural content ───────────────────────────────────────────────────
	Claims            []Claim   `json:"claims,omitempty"`
	MarkushStructures []Markush `json:"markush_structures,omitempty"`

	// ── Family and priority ──────────────────────────────────────────────────
	FamilyID string     `json:"family_id,omitempty"`
	Priority []Priority `json:"priority,omitempty"`

	// ── Domain event collector (unexported — never persisted directly) ────────
	events []DomainEvent
}

// ─────────────────────────────────────────────────────────────────────────────
// Factory function
// ─────────────────────────────────────────────────────────────────────────────

// NewPatent creates a new Patent aggregate root, enforcing all construction
// invariants:
//   - number, title, abstract, and applicant must be non-empty.
//   - jurisdiction must be one of the supported JurisdictionCode values.
//   - the patent number must match the format expected for the given jurisdiction.
//   - filingDate must be a non-zero time.
//
// On success the patent is initialised with StatusFiled and a PatentCreated
// domain event is recorded.
func NewPatent(
	number, title, abstract, applicant string,
	jurisdiction ptypes.JurisdictionCode,
	filingDate time.Time,
) (*Patent, error) {
	// ── Required-field guards ─────────────────────────────────────────────────
	if number == "" {
		return nil, errors.InvalidParam("patent number must not be empty")
	}
	if title == "" {
		return nil, errors.InvalidParam("patent title must not be empty")
	}
	if abstract == "" {
		return nil, errors.InvalidParam("patent abstract must not be empty")
	}
	if applicant == "" {
		return nil, errors.InvalidParam("patent applicant must not be empty")
	}
	if filingDate.IsZero() {
		return nil, errors.InvalidParam("filing date must not be zero")
	}

	// ── Jurisdiction guard ────────────────────────────────────────────────────
	if !validJurisdictions[jurisdiction] {
		return nil, errors.InvalidParam(
			fmt.Sprintf("unsupported jurisdiction code: %q", jurisdiction),
		)
	}

	// ── Patent number format guard ────────────────────────────────────────────
	if err := validatePatentNumber(number, jurisdiction); err != nil {
		return nil, err
	}

	// ── Construct and initialise ──────────────────────────────────────────────
	now := time.Now().UTC()
	p := &Patent{
		BaseEntity: common.BaseEntity{
			ID:        common.NewID(),
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
		},
		PatentNumber: number,
		Title:        title,
		Abstract:     abstract,
		Applicant:    applicant,
		FilingDate:   filingDate,
		Status:       ptypes.StatusFiled,
		Jurisdiction: jurisdiction,
		Claims:       make([]Claim, 0),
		MarkushStructures: make([]Markush, 0),
		Priority:     make([]Priority, 0),
	}

	// Record the creation domain event.
	p.recordEvent(NewPatentCreatedEvent(p.ID, p.PatentNumber, p.Title, p.Jurisdiction))

	return p, nil
}

// validatePatentNumber checks that the patent number matches the expected
// format for the given jurisdiction.  For JurisdictionOther no format
// validation is applied.
func validatePatentNumber(number string, jurisdiction ptypes.JurisdictionCode) error {
	var re *regexp.Regexp
	switch jurisdiction {
	case ptypes.JurisdictionCN:
		re = rePatentCN
	case ptypes.JurisdictionUS:
		re = rePatentUS
	case ptypes.JurisdictionEP:
		re = rePatentEP
	case ptypes.JurisdictionWO:
		re = rePatentWO
	case ptypes.JurisdictionJP:
		re = rePatentJP
	case ptypes.JurisdictionKR:
		re = rePatentKR
	default:
		// JurisdictionOther — no format constraint.
		return nil
	}

	if !re.MatchString(number) {
		return errors.New(errors.CodeInvalidParam,
			fmt.Sprintf("patent number %q does not match expected format for jurisdiction %q",
				number, jurisdiction),
		)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Claim management
// ─────────────────────────────────────────────────────────────────────────────

// AddClaim appends a Claim to the patent, enforcing:
//   - no two claims share the same Number.
//   - if the claim has a non-nil ParentClaimNumber, that parent must already
//     exist in p.Claims (forward references are not allowed).
func (p *Patent) AddClaim(claim Claim) error {
	// Duplicate number check.
	for _, existing := range p.Claims {
		if existing.Number == claim.Number {
			return errors.New(errors.CodeInvalidParam,
				fmt.Sprintf("claim number %d already exists on patent %s",
					claim.Number, p.PatentNumber),
			)
		}
	}

	// Dependency check: dependent claims must reference an existing parent.
	if claim.ParentClaimNumber != nil {
		found := false
		for _, existing := range p.Claims {
			if existing.Number == *claim.ParentClaimNumber {
				found = true
				break
			}
		}
		if !found {
			return errors.New(errors.CodeInvalidParam,
				fmt.Sprintf("dependent claim %d references non-existent parent claim %d on patent %s",
					claim.Number, *claim.ParentClaimNumber, p.PatentNumber),
			)
		}
	}

	p.Claims = append(p.Claims, claim)
	p.touch()
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Markush structure management
// ─────────────────────────────────────────────────────────────────────────────

// AddMarkush appends a Markush structure to the patent, enforcing that the
// ClaimID referenced by the Markush structure corresponds to an existing claim.
func (p *Patent) AddMarkush(markush Markush) error {
	// Verify the associated claim exists.
	if markush.ClaimID != "" {
		found := false
		for _, c := range p.Claims {
			if c.ID == markush.ClaimID {
				found = true
				break
			}
		}
		if !found {
			return errors.New(errors.CodeInvalidParam,
				fmt.Sprintf("markush structure references non-existent claim ID %q on patent %s",
					markush.ClaimID, p.PatentNumber),
			)
		}
	}

	p.MarkushStructures = append(p.MarkushStructures, markush)
	p.touch()
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Status lifecycle
// ─────────────────────────────────────────────────────────────────────────────

// UpdateStatus transitions the patent to a new status, enforcing the state
// machine defined by allowedTransitions.  A PatentStatusChanged domain event
// is recorded on success.
func (p *Patent) UpdateStatus(status ptypes.PatentStatus) error {
	allowed, ok := allowedTransitions[p.Status]
	if !ok {
		// Current status is unrecognised — treat as terminal.
		return errors.New(errors.CodeInvalidParam,
			fmt.Sprintf("unknown current status %q for patent %s", p.Status, p.PatentNumber),
		)
	}

	for _, s := range allowed {
		if s == status {
			prev := p.Status
			p.Status = status
			p.touch()
			p.recordEvent(NewPatentStatusChangedEvent(p.ID, p.PatentNumber, prev, status))
			return nil
		}
	}

	return errors.New(errors.CodeInvalidParam,
		fmt.Sprintf("illegal status transition %q → %q for patent %s",
			p.Status, status, p.PatentNumber),
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// Date setters
// ─────────────────────────────────────────────────────────────────────────────

// SetPublicationDate records the publication date after validating that it
// falls on or after the filing date.
func (p *Patent) SetPublicationDate(date time.Time) error {
	if !date.After(p.FilingDate) && !date.Equal(p.FilingDate) {
		return errors.New(errors.CodeInvalidParam,
			fmt.Sprintf("publication date %s must be on or after filing date %s for patent %s",
				date.Format(time.DateOnly), p.FilingDate.Format(time.DateOnly), p.PatentNumber),
		)
	}
	p.PublicationDate = &date
	p.touch()
	return nil
}

// SetGrantDate records the grant date after validating that it falls on or
// after the publication date (which must already be set).
func (p *Patent) SetGrantDate(date time.Time) error {
	if p.PublicationDate == nil {
		return errors.New(errors.CodeInvalidParam,
			fmt.Sprintf("cannot set grant date before publication date for patent %s",
				p.PatentNumber),
		)
	}
	if !date.After(*p.PublicationDate) && !date.Equal(*p.PublicationDate) {
		return errors.New(errors.CodeInvalidParam,
			fmt.Sprintf("grant date %s must be on or after publication date %s for patent %s",
				date.Format(time.DateOnly), p.PublicationDate.Format(time.DateOnly),
				p.PatentNumber),
		)
	}
	p.GrantDate = &date
	p.touch()
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Expiry calculation and status helpers
// ─────────────────────────────────────────────────────────────────────────────

// CalculateExpiryDate returns the statutory expiry date of the patent based on
// the jurisdiction-specific lifespan (defaulting to 20 years from the filing
// date for all currently supported jurisdictions).
//
// Note: maintenance fees, extensions (SPC, PTA, PTE) and other jurisdiction-
// specific adjustments are not modelled here; this method provides the baseline
// statutory term only.
func (p *Patent) CalculateExpiryDate() time.Time {
	years, ok := patentLifespanYears[p.Jurisdiction]
	if !ok {
		years = defaultPatentLifespanYears
	}
	return p.FilingDate.AddDate(years, 0, 0)
}

// IsExpired reports whether the patent's statutory term has elapsed relative
// to the current UTC time.  A patent with StatusExpired or StatusRevoked is
// also considered expired regardless of the calculated date.
func (p *Patent) IsExpired() bool {
	switch p.Status {
	case ptypes.StatusExpired, ptypes.StatusRevoked, ptypes.StatusAbandoned:
		return true
	}
	return time.Now().UTC().After(p.CalculateExpiryDate())
}

// ─────────────────────────────────────────────────────────────────────────────
// Claim accessors
// ─────────────────────────────────────────────────────────────────────────────

// GetIndependentClaims returns all claims with a nil ParentClaimNumber
// (i.e., claims that do not depend on any other claim).
func (p *Patent) GetIndependentClaims() []Claim {
	result := make([]Claim, 0)
	for _, c := range p.Claims {
		if c.ParentClaimNumber == nil {
			result = append(result, c)
		}
	}
	return result
}

// GetDependentClaims returns all claims that directly or transitively depend
// on the claim with the given independentClaimNumber.  Only direct dependents
// are returned (single level); callers that need the full dependency tree
// should call this method recursively.
func (p *Patent) GetDependentClaims(independentClaimNumber int) []Claim {
	result := make([]Claim, 0)
	for _, c := range p.Claims {
		if c.ParentClaimNumber != nil && *c.ParentClaimNumber == independentClaimNumber {
			result = append(result, c)
		}
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Domain event collection
// ─────────────────────────────────────────────────────────────────────────────

// Events returns the slice of domain events accumulated since the last call to
// Events (or since creation), and clears the internal event buffer.  Callers
// (typically application services) are responsible for publishing these events
// after the unit of work commits.
func (p *Patent) Events() []DomainEvent {
	evts := p.events
	p.events = nil
	return evts
}

// recordEvent appends a domain event to the internal buffer.
func (p *Patent) recordEvent(evt DomainEvent) {
	p.events = append(p.events, evt)
}

// ─────────────────────────────────────────────────────────────────────────────
// DTO conversion
// ─────────────────────────────────────────────────────────────────────────────

// ToDTO converts the Patent aggregate root to a ptypes.PatentDTO suitable for
// transport across layer boundaries.  The unexported events field is not
// included in the DTO.
func (p *Patent) ToDTO() ptypes.PatentDTO {
	dto := ptypes.PatentDTO{
		BaseEntity:      p.BaseEntity,
		PatentNumber:    p.PatentNumber,
		Title:           p.Title,
		Abstract:        p.Abstract,
		Applicant:       p.Applicant,
		Inventors:       p.Inventors,
		FilingDate:      p.FilingDate,
		PublicationDate: p.PublicationDate,
		GrantDate:       p.GrantDate,
		ExpiryDate:      p.ExpiryDate,
		Status:          p.Status,
		Jurisdiction:    p.Jurisdiction,
		IPCCodes:        p.IPCCodes,
		CPCCodes:        p.CPCCodes,
		FamilyID:        p.FamilyID,
	}

	// Convert Claims.
	if len(p.Claims) > 0 {
		dto.Claims = make([]ptypes.ClaimDTO, len(p.Claims))
		for i, c := range p.Claims {
			dto.Claims[i] = c.ToDTO()
		}
	}

	// Convert Markush structures.
	if len(p.MarkushStructures) > 0 {
		dto.MarkushStructures = make([]ptypes.MarkushDTO, len(p.MarkushStructures))
		for i, m := range p.MarkushStructures {
			dto.MarkushStructures[i] = m.ToDTO()
		}
	}

	// Convert Priority entries.
	if len(p.Priority) > 0 {
		dto.Priority = make([]ptypes.PriorityDTO, len(p.Priority))
		for i, pr := range p.Priority {
			dto.Priority[i] = ptypes.PriorityDTO{
				Country: pr.Country,
				Number:  pr.Number,
				Date:    pr.Date,
			}
		}
	}

	return dto
}

// PatentFromDTO reconstructs a Patent aggregate root from a PatentDTO.  This
// function is used exclusively by the repository (infrastructure) layer to
// rehydrate persisted entities; it bypasses the factory-function invariant
// checks because the data has already been validated at write time.
//
// The returned Patent has an empty event buffer; no domain events are emitted
// during rehydration.
func PatentFromDTO(dto ptypes.PatentDTO) *Patent {
	p := &Patent{
		BaseEntity:      dto.BaseEntity,
		PatentNumber:    dto.PatentNumber,
		Title:           dto.Title,
		Abstract:        dto.Abstract,
		Applicant:       dto.Applicant,
		Inventors:       dto.Inventors,
		FilingDate:      dto.FilingDate,
		PublicationDate: dto.PublicationDate,
		GrantDate:       dto.GrantDate,
		ExpiryDate:      dto.ExpiryDate,
		Status:          dto.Status,
		Jurisdiction:    dto.Jurisdiction,
		IPCCodes:        dto.IPCCodes,
		CPCCodes:        dto.CPCCodes,
		FamilyID:        dto.FamilyID,
	}

	// Rehydrate Claims.
	p.Claims = make([]Claim, len(dto.Claims))
	for i, cdto := range dto.Claims {
		p.Claims[i] = ClaimFromDTO(cdto)
	}

	// Rehydrate Markush structures.
	p.MarkushStructures = make([]Markush, len(dto.MarkushStructures))
	for i, mdto := range dto.MarkushStructures {
		p.MarkushStructures[i] = MarkushFromDTO(mdto)
	}

	// Rehydrate Priority entries.
	p.Priority = make([]Priority, len(dto.Priority))
	for i, prdto := range dto.Priority {
		p.Priority[i] = Priority{
			Country: prdto.Country,
			Number:  prdto.Number,
			Date:    prdto.Date,
		}
	}

	return p
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

// touch updates UpdatedAt and bumps the optimistic-lock Version.
// It must be called at the end of every mutating method.
func (p *Patent) touch() {
	p.UpdatedAt = time.Now().UTC()
	p.Version++
}

//Personal.AI order the ending
