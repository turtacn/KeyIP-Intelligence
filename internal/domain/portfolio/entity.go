// Package portfolio implements the patent portfolio domain aggregate,
// which models collections of patents owned or managed by a user and
// provides valuation capabilities based on multiple technical, legal,
// and market factors.
package portfolio

import (
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// Portfolio — aggregate root for a patent collection
// ─────────────────────────────────────────────────────────────────────────────

// Portfolio is an aggregate root representing a curated collection of patents
// owned or managed by a single user.  It supports valuation, tagging, and
// lifecycle management (active/archived/deleted).
//
// Business rules:
//   - Each portfolio must have a non-empty name.
//   - A patent cannot be added to the same portfolio twice.
//   - Valuation results are cached in TotalValue and must be refreshed via
//     the domain service when factors change.
type Portfolio struct {
	// BaseEntity provides ID, TenantID, CreatedAt, UpdatedAt, Version.
	common.BaseEntity

	// Name is the user-assigned title for this portfolio (e.g., "Core EV Patents").
	Name string `json:"name"`

	// Description provides additional context about the portfolio's purpose.
	Description string `json:"description"`

	// OwnerID identifies the user who owns this portfolio.
	OwnerID common.UserID `json:"owner_id"`

	// PatentIDs lists the IDs of patents included in this portfolio.
	// Maintain uniqueness invariant: no duplicate IDs allowed.
	PatentIDs []common.ID `json:"patent_ids"`

	// TotalValue holds the most recent valuation result computed by the
	// domain service.  Nil if no valuation has been performed yet.
	TotalValue *ValuationResult `json:"total_value,omitempty"`

	// Tags enables free-form categorization (e.g., ["lithium-ion", "OLED", "5G"]).
	Tags []string `json:"tags,omitempty"`

	// Status tracks the portfolio's lifecycle state.
	Status common.Status `json:"status"`
}

// ─────────────────────────────────────────────────────────────────────────────
// ValuationResult — value object holding aggregated valuation metrics
// ─────────────────────────────────────────────────────────────────────────────

// ValuationResult encapsulates the financial valuation of an entire portfolio,
// computed as the sum of individual patent valuations plus statistical summaries.
type ValuationResult struct {
	// TotalValue is the sum of all patent valuations in USD.
	TotalValue float64 `json:"total_value"`

	// AverageValue is TotalValue / count(patents).
	AverageValue float64 `json:"average_value"`

	// MedianValue is the 50th percentile of individual patent values.
	MedianValue float64 `json:"median_value"`

	// HighestValue is the maximum single-patent value in the portfolio.
	HighestValue float64 `json:"highest_value"`

	// LowestValue is the minimum single-patent value in the portfolio.
	LowestValue float64 `json:"lowest_value"`

	// ValuationDate is the UTC timestamp when this valuation was computed.
	ValuationDate time.Time `json:"valuation_date"`

	// Method identifies the valuation algorithm used (e.g., "MultiFactorV1").
	Method string `json:"method"`

	// Breakdown lists the per-patent valuations that comprise TotalValue.
	Breakdown []PatentValuation `json:"breakdown"`
}

// ─────────────────────────────────────────────────────────────────────────────
// PatentValuation — value object for a single patent's valuation
// ─────────────────────────────────────────────────────────────────────────────

// PatentValuation holds the financial value and contributing factors for one
// patent within a portfolio valuation.
type PatentValuation struct {
	// PatentID identifies which patent this valuation applies to.
	PatentID common.ID `json:"patent_id"`

	// Value is the computed monetary value in USD.
	Value float64 `json:"value"`

	// Factors are the normalised scores and metrics that contributed to Value.
	Factors ValuationFactors `json:"factors"`
}

// ─────────────────────────────────────────────────────────────────────────────
// ValuationFactors — value object holding multi-dimensional scoring
// ─────────────────────────────────────────────────────────────────────────────

// ValuationFactors encapsulates the normalised scores (0.0–1.0) and raw counts
// used by the valuation algorithm to estimate a patent's worth.
type ValuationFactors struct {
	// TechnicalScore represents the technical innovation quality (0.0–1.0).
	// Higher scores indicate novel, non-obvious inventions with broad applicability.
	TechnicalScore float64 `json:"technical_score"`

	// LegalScore represents the legal strength and enforceability (0.0–1.0).
	// Considers claim clarity, prosecution history, litigation outcomes.
	LegalScore float64 `json:"legal_score"`

	// MarketScore represents commercial relevance and market size (0.0–1.0).
	// Higher scores indicate large addressable markets or strategic importance.
	MarketScore float64 `json:"market_score"`

	// RemainingLife is the number of years until the patent expires.
	// Used to apply time-decay to the valuation.
	RemainingLife float64 `json:"remaining_life"`

	// CitationCount is the number of times this patent has been cited by
	// subsequent patents, indicating technical influence.
	CitationCount int `json:"citation_count"`

	// FamilySize is the count of patent family members across jurisdictions,
	// indicating geographic coverage.
	FamilySize int `json:"family_size"`

	// ClaimBreadth quantifies the scope of the patent claims (0.0–1.0).
	// Computed from independent claim count, element count, and dependency structure.
	ClaimBreadth float64 `json:"claim_breadth"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Factory function
// ─────────────────────────────────────────────────────────────────────────────

// NewPortfolio constructs a new Portfolio with the given attributes and sets
// default values for lifecycle fields.
//
// Returns an error if:
//   - name is empty (portfolio must have an identifiable name)
func NewPortfolio(name, description string, ownerID common.UserID) (*Portfolio, error) {
	if name == "" {
		return nil, errors.InvalidParam("portfolio name cannot be empty")
	}
	if ownerID == "" {
		return nil, errors.InvalidParam("owner ID cannot be empty")
	}

	return &Portfolio{
		BaseEntity: common.BaseEntity{
			ID:        common.NewID(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
			Version:   1,
		},
		Name:        name,
		Description: description,
		OwnerID:     ownerID,
		PatentIDs:   make([]common.ID, 0),
		Tags:        make([]string, 0),
		Status:      common.StatusActive,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Methods
// ─────────────────────────────────────────────────────────────────────────────

// AddPatent appends a patent ID to the portfolio's patent list.
//
// Returns an error if:
//   - patentID is already in the portfolio (duplicates not allowed)
func (p *Portfolio) AddPatent(patentID common.ID) error {
	if patentID == "" {
		return errors.InvalidParam("patent ID cannot be empty")
	}
	if p.ContainsPatent(patentID) {
		return errors.Conflict(fmt.Sprintf("patent %s is already in portfolio %s", patentID, p.ID))
	}

	p.PatentIDs = append(p.PatentIDs, patentID)
	p.UpdatedAt = time.Now().UTC()
	p.Version++
	return nil
}

// RemovePatent removes a patent ID from the portfolio's patent list.
//
// Returns an error if:
//   - patentID is not found in the portfolio
func (p *Portfolio) RemovePatent(patentID common.ID) error {
	if patentID == "" {
		return errors.InvalidParam("patent ID cannot be empty")
	}

	idx := -1
	for i, id := range p.PatentIDs {
		if id == patentID {
			idx = i
			break
		}
	}

	if idx == -1 {
		return errors.NotFound(fmt.Sprintf("patent %s not found in portfolio %s", patentID, p.ID))
	}

	// Remove by swapping with last element and truncating.
	p.PatentIDs[idx] = p.PatentIDs[len(p.PatentIDs)-1]
	p.PatentIDs = p.PatentIDs[:len(p.PatentIDs)-1]
	p.UpdatedAt = time.Now().UTC()
	p.Version++
	return nil
}

// SetValuation updates the portfolio's cached valuation result.
// Typically called by the domain service after computing valuations.
func (p *Portfolio) SetValuation(result ValuationResult) {
	p.TotalValue = &result
	p.UpdatedAt = time.Now().UTC()
	p.Version++
}

// Size returns the number of patents currently in the portfolio.
func (p *Portfolio) Size() int {
	return len(p.PatentIDs)
}

// ContainsPatent reports whether the given patent ID is in the portfolio.
func (p *Portfolio) ContainsPatent(patentID common.ID) bool {
	for _, id := range p.PatentIDs {
		if id == patentID {
			return true
		}
	}
	return false
}

//Personal.AI order the ending
