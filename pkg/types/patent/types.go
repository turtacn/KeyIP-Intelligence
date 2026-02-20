// Package patent defines all Data Transfer Objects (DTOs) and enumeration types
// used across the patent domain of the KeyIP-Intelligence platform.
//
// These types serve as the canonical contract between layers:
//   - HTTP handlers use them for request / response serialisation
//   - Application services accept and return them as parameters / results
//   - Domain entities expose conversion helpers to/from these DTOs
//   - The SDK client (pkg/client/patents.go) exposes them to external callers
//
// All types are safe for concurrent read access once constructed.
package patent

import (
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// Enumeration types
// ─────────────────────────────────────────────────────────────────────────────

// PatentStatus represents the lifecycle stage of a patent application or grant.
// The string underlying type is used so that JSON marshalling / unmarshalling
// produces human-readable values without a custom codec.
type PatentStatus string

const (
	// StatusFiled indicates the patent application has been filed with the
	// relevant patent office but has not yet been published.
	StatusFiled PatentStatus = "FILED"

	// StatusPublished indicates the application has been published (typically
	// 18 months after the earliest priority date) but has not yet been granted.
	StatusPublished PatentStatus = "PUBLISHED"

	// StatusGranted indicates the patent has been examined and granted by the
	// patent office.  The patent is in force subject to annuity payment.
	StatusGranted PatentStatus = "GRANTED"

	// StatusExpired indicates the patent term has elapsed (20 years from the
	// filing date for most jurisdictions) or annuities were not paid.
	StatusExpired PatentStatus = "EXPIRED"

	// StatusAbandoned indicates the applicant has voluntarily abandoned the
	// application or prosecution has been discontinued.
	StatusAbandoned PatentStatus = "ABANDONED"

	// StatusRevoked indicates a granted patent has been revoked by the patent
	// office or a court (e.g., following an opposition or invalidity proceeding).
	StatusRevoked PatentStatus = "REVOKED"
)

// IsValid reports whether the PatentStatus value is one of the recognised constants.
func (s PatentStatus) IsValid() bool {
	switch s {
	case StatusFiled, StatusPublished, StatusGranted,
		StatusExpired, StatusAbandoned, StatusRevoked:
		return true
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────

// ClaimType classifies a patent claim as either independent or dependent.
type ClaimType string

const (
	// ClaimIndependent identifies a claim that stands on its own without
	// reference to any other claim.
	ClaimIndependent ClaimType = "INDEPENDENT"

	// ClaimDependent identifies a claim that incorporates by reference all
	// limitations of one or more preceding claims.
	ClaimDependent ClaimType = "DEPENDENT"
)

// IsValid reports whether the ClaimType value is one of the recognised constants.
func (ct ClaimType) IsValid() bool {
	return ct == ClaimIndependent || ct == ClaimDependent
}

// ─────────────────────────────────────────────────────────────────────────────

// JurisdictionCode is an ISO/WIPO-based two-or-three-letter code identifying
// the patent office or treaty under which a patent application was filed.
type JurisdictionCode string

const (
	// JurisdictionCN represents the China National Intellectual Property
	// Administration (CNIPA).
	JurisdictionCN JurisdictionCode = "CN"

	// JurisdictionUS represents the United States Patent and Trademark Office
	// (USPTO).
	JurisdictionUS JurisdictionCode = "US"

	// JurisdictionEP represents the European Patent Office (EPO), covering
	// contracting states of the European Patent Convention.
	JurisdictionEP JurisdictionCode = "EP"

	// JurisdictionJP represents the Japan Patent Office (JPO).
	JurisdictionJP JurisdictionCode = "JP"

	// JurisdictionKR represents the Korean Intellectual Property Office (KIPO).
	JurisdictionKR JurisdictionCode = "KR"

	// JurisdictionWO represents a PCT (Patent Cooperation Treaty) international
	// application filed via WIPO.
	JurisdictionWO JurisdictionCode = "WO"

	// JurisdictionOther represents any other jurisdiction not explicitly listed.
	JurisdictionOther JurisdictionCode = "OTHER"
)

// IsValid reports whether the JurisdictionCode is one of the recognised constants.
func (j JurisdictionCode) IsValid() bool {
	switch j {
	case JurisdictionCN, JurisdictionUS, JurisdictionEP,
		JurisdictionJP, JurisdictionKR, JurisdictionWO, JurisdictionOther:
		return true
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// Core DTOs
// ─────────────────────────────────────────────────────────────────────────────

// PatentDTO is the primary data transfer object for a patent document.
// It carries all bibliographic data, claim summaries, and metadata required
// by the API layer, application services, and SDK clients.
//
// Pointer fields represent optional or not-yet-available data (e.g., grant date
// for a patent that has not yet been granted).  All dates are in UTC.
type PatentDTO struct {
	// BaseEntity supplies ID, CreatedAt, UpdatedAt, and Version from the common package.
	common.BaseEntity

	// PatentNumber is the official publication or grant number assigned by the
	// patent office (e.g., "CN202310001234A", "US11234567B2").
	PatentNumber string `json:"patent_number"`

	// Title is the invention title as it appears in the patent document.
	Title string `json:"title"`

	// Abstract is the abstract section of the patent document.
	Abstract string `json:"abstract"`

	// Applicant is the name of the primary applicant / assignee.
	Applicant string `json:"applicant"`

	// Inventors is an ordered list of inventor full names.
	Inventors []string `json:"inventors"`

	// FilingDate is the date on which the application was filed with the patent office.
	FilingDate time.Time `json:"filing_date"`

	// PublicationDate is the date on which the application was published.
	// Nil if the application has not yet been published.
	PublicationDate *time.Time `json:"publication_date,omitempty"`

	// GrantDate is the date on which the patent was granted.
	// Nil if the application has not yet been granted.
	GrantDate *time.Time `json:"grant_date,omitempty"`

	// ExpiryDate is the date on which the patent will expire or has expired.
	// Nil if the expiry date has not yet been calculated (e.g., application stage).
	ExpiryDate *time.Time `json:"expiry_date,omitempty"`

	// Status is the current lifecycle status of the patent.
	Status PatentStatus `json:"status"`

	// Jurisdiction identifies the patent office / treaty under which this
	// patent was filed or granted.
	Jurisdiction JurisdictionCode `json:"jurisdiction"`

	// IPCCodes contains the International Patent Classification codes assigned
	// to the patent (e.g., ["C07D209/14", "C09K11/06"]).
	IPCCodes []string `json:"ipc_codes,omitempty"`

	// CPCCodes contains the Cooperative Patent Classification codes assigned
	// to the patent (e.g., ["C07D209/14", "C09K11/06"]).
	CPCCodes []string `json:"cpc_codes,omitempty"`

	// Claims is a summary list of the patent's claims.  Full claim text
	// is stored in each ClaimDTO.
	Claims []ClaimDTO `json:"claims,omitempty"`

	// MarkushStructures is a list of chemical structures extracted from the claims.
	MarkushStructures []MarkushDTO `json:"markush_structures,omitempty"`

	// FamilyID is the canonical identifier of the patent family to which this
	// patent belongs, linking equivalent applications across jurisdictions.
	FamilyID string `json:"family_id,omitempty"`

	// Priority is the list of priority documents (earlier applications) from
	// which this patent claims priority.
	Priority []PriorityDTO `json:"priority,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────

// ClaimDTO represents a single claim within a patent document.
type ClaimDTO struct {
	// ID is the internal unique identifier for this claim record.
	ID common.ID `json:"id"`

	// Number is the sequential claim number as it appears in the patent document (1-based).
	Number int `json:"number"`

	// Text is the full verbatim text of the claim.
	Text string `json:"text"`

	// Type classifies the claim as independent or dependent.
	Type ClaimType `json:"type"`

	// ParentClaimNumber is the number of the claim that this dependent claim
	// refers back to.  Nil for independent claims.
	ParentClaimNumber *int `json:"parent_claim_number,omitempty"`

	// Elements contains the parsed structural elements of the claim produced by
	// the ClaimBERT model (preamble, transition, body elements).
	Elements []ClaimElementDTO `json:"elements,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────

// ClaimElementDTO represents a single parsed element within a claim, as
// produced by the ClaimBERT model and ChemExtractor NER pipeline.
type ClaimElementDTO struct {
	// ID is the internal unique identifier for this element record.
	ID common.ID `json:"id"`

	// Text is the verbatim text of the claim element.
	Text string `json:"text"`

	// IsStructural indicates whether the element constitutes a structural
	// limitation of the claim (as opposed to a functional or method step).
	IsStructural bool `json:"is_structural"`

	// ChemicalEntities is the list of chemical entity names or SMILES strings
	// extracted from the element text by the ChemExtractor NER model.
	ChemicalEntities []string `json:"chemical_entities,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────

// PriorityDTO represents a single priority claim in a patent application,
// referencing an earlier application filed in the same or a different jurisdiction.
type PriorityDTO struct {
	// Country is the ISO 3166-1 alpha-2 country code of the priority application.
	Country string `json:"country"`

	// Number is the application number of the priority document.
	Number string `json:"number"`

	// Date is the filing date of the priority application.
	Date time.Time `json:"date"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Markush structure DTOs
// ─────────────────────────────────────────────────────────────────────────────

// MarkushDTO represents a Markush structure extracted from a patent claim.
// Markush structures define families of chemical compounds using a core scaffold
// with enumerable R-group substituents, which is the standard way of claiming
// broad chemical space in pharmaceutical and chemistry patents.
type MarkushDTO struct {
	// ID is the internal unique identifier for this Markush record.
	ID common.ID `json:"id"`

	// PatentID is the identifier of the patent from which this structure was extracted.
	PatentID common.ID `json:"patent_id"`

	// ClaimID is the identifier of the specific claim that defines this Markush structure.
	ClaimID common.ID `json:"claim_id"`

	// CoreStructure is the SMILES representation of the invariant scaffold.
	// R-group attachment points are indicated using conventional R-notation (R1, R2, …).
	CoreStructure string `json:"core_structure"`

	// RGroups describes each substitution position and the set of allowed substituents.
	RGroups []RGroupDTO `json:"r_groups,omitempty"`

	// Description is a free-text summary of the Markush structure for human readers.
	Description string `json:"description,omitempty"`

	// EnumeratedCount is the computed cardinality of the Markush virtual library.
	EnumeratedCount int64 `json:"enumerated_count"`
}

// ─────────────────────────────────────────────────────────────────────────────

// RGroupDTO describes a single substitution position (R-group) in a Markush structure,
// listing all the substituents that are explicitly or generically claimed at that position.
type RGroupDTO struct {
	// Position is the label of the substitution point as it appears in the
	// CoreStructure SMILES (e.g., "R1", "R2", "R3").
	Position string `json:"position"`

	// Alternatives is an ordered list of SMILES strings representing the
	// individual substituents that are permitted at this position.
	// Wildcard or generic group notation (e.g., "alkyl") is stored as a
	// human-readable string prefixed with "~" to distinguish it from a
	// concrete SMILES (e.g., "~alkyl").
	Alternatives []string `json:"alternatives,omitempty"`

	// Description is a free-text explanation of the substituent class at this
	// position, as extracted from the claim text.
	Description string `json:"description,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Search request / response DTOs
// ─────────────────────────────────────────────────────────────────────────────

// PatentSearchRequest encapsulates the parameters for a full-text or structured
// patent search.  All filter fields are optional; supplying none is equivalent to
// a broad search across all patents accessible to the current tenant.
type PatentSearchRequest struct {
	// Query is the full-text search query string.  When empty, results are
	// returned ordered by relevance score descending.
	Query string `json:"query"`

	// Jurisdiction filters results to a specific patent office / treaty.
	// Nil means all jurisdictions.
	Jurisdiction *JurisdictionCode `json:"jurisdiction,omitempty"`

	// DateFrom restricts results to patents with a filing date on or after
	// this value.  Nil means no lower bound.
	DateFrom *time.Time `json:"date_from,omitempty"`

	// DateTo restricts results to patents with a filing date on or before
	// this value.  Nil means no upper bound.
	DateTo *time.Time `json:"date_to,omitempty"`

	// Applicant filters results to patents filed by the named applicant.
	// Nil means all applicants.  The value is matched as a case-insensitive
	// prefix in the OpenSearch index.
	Applicant *string `json:"applicant,omitempty"`

	// IPCCode restricts results to patents classified under the given IPC code
	// or any of its sub-codes.  Nil means all IPC codes.
	// Example: "C07D" matches C07D209/14, C07D401/00, etc.
	IPCCode *string `json:"ipc_code,omitempty"`

	// PageRequest embeds pagination parameters (page number and page size).
	common.PageRequest
}

// PatentSearchResponse is the paginated response type for patent search results.
// It embeds the generic PageResponse, which carries the total count, current page,
// and the slice of matched PatentDTOs.
type PatentSearchResponse = common.PageResponse[PatentDTO]

// ─────────────────────────────────────────────────────────────────────────────

// SimilaritySearchRequest encapsulates parameters for a molecule-to-patent
// similarity search driven by the MolPatent-GNN model.
// The search finds patents whose claimed chemical space overlaps with the
// query molecule above the specified similarity threshold.
type SimilaritySearchRequest struct {
	// SMILES is the canonical SMILES string of the query molecule.
	// Must be a valid, parseable SMILES; an invalid value returns CodeMoleculeInvalidSMILES.
	SMILES string `json:"smiles"`

	// Threshold is the minimum similarity score (Tanimoto coefficient or GNN
	// embedding cosine similarity, depending on the configured algorithm)
	// required for a patent to appear in the results.
	// Valid range: [0.0, 1.0].  Default: 0.7.
	Threshold float64 `json:"threshold"`

	// MaxResults caps the number of results returned.  Must be in [1, 1000].
	// Default: 50.
	MaxResults int `json:"max_results"`

	// Jurisdiction optionally restricts the search to a specific patent office.
	// Nil means all jurisdictions.
	Jurisdiction *JurisdictionCode `json:"jurisdiction,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────

// SimilaritySearchResponse is the response type for a molecule-to-patent
// similarity search, containing an ordered list of matching patents.
type SimilaritySearchResponse struct {
	// Results is the ordered list of matching patents, sorted by Score descending.
	Results []SimilarityResult `json:"results"`
}

// ─────────────────────────────────────────────────────────────────────────────

// SimilarityResult pairs a matched patent with its computed similarity score
// and the claim numbers that were most directly responsible for the match.
type SimilarityResult struct {
	// Patent is the matching patent document.
	Patent PatentDTO `json:"patent"`

	// Score is the similarity score between the query molecule and the patent's
	// claimed chemical space.  Range: [0.0, 1.0]; higher is more similar.
	Score float64 `json:"score"`

	// MatchedClaims contains the claim numbers (1-based) that contributed most
	// to the similarity score, allowing the user to focus review on the most
	// relevant claims.
	MatchedClaims []int `json:"matched_claims,omitempty"`
}

