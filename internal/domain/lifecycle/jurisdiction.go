// Package lifecycle implements jurisdiction-specific rules for patent lifecycle management.
package lifecycle

import (
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// JurisdictionRules
// ─────────────────────────────────────────────────────────────────────────────

// JurisdictionRules encapsulates all statutory rules and regulations for a
// specific patent jurisdiction. These rules determine deadline calculations,
// annuity schedules, and other lifecycle parameters.
type JurisdictionRules struct {
	// Code is the jurisdiction identifier (CN, US, EP, JP, KR, WO, etc.).
	Code ptypes.JurisdictionCode `json:"code"`

	// PatentTermYears is the statutory patent term in years (typically 20).
	PatentTermYears int `json:"patent_term_years"`

	// AnnuityStartYear is the first year when annuity payments are required.
	// For CN/EP: year 3; for JP/KR: year 1; for US: not applicable (milestone-based).
	AnnuityStartYear int `json:"annuity_start_year"`

	// AnnuityFrequency indicates the payment schedule: "annual" or "milestone".
	AnnuityFrequency string `json:"annuity_frequency"`

	// GracePeriodMonths is the grace period after due date during which payment
	// can still be made with a surcharge.
	GracePeriodMonths int `json:"grace_period_months"`

	// SurchargeRate is the penalty rate for late payment during grace period.
	SurchargeRate float64 `json:"surcharge_rate"`

	// ExaminationDeadlineMonths is the deadline to request substantive examination
	// (applicable in two-stage examination systems like CN).
	ExaminationDeadlineMonths int `json:"examination_deadline_months"`

	// OAResponseDeadlineMonths is the standard deadline to respond to office actions.
	OAResponseDeadlineMonths int `json:"oa_response_deadline_months"`

	// PCTNationalPhaseMonths is the deadline to enter national phase from PCT
	// international filing.
	PCTNationalPhaseMonths int `json:"pct_national_phase_months"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Jurisdiction registry
// ─────────────────────────────────────────────────────────────────────────────

// jurisdictionRegistry is the internal database of jurisdiction rules.
// In production, this should be externalized to a configuration file or database.
var jurisdictionRegistry = map[ptypes.JurisdictionCode]JurisdictionRules{
	ptypes.JurisdictionCN: {
		Code:                      ptypes.JurisdictionCN,
		PatentTermYears:           20,
		AnnuityStartYear:          3,
		AnnuityFrequency:          "annual",
		GracePeriodMonths:         6,
		SurchargeRate:             0.25,
		ExaminationDeadlineMonths: 36,
		OAResponseDeadlineMonths:  4,
		PCTNationalPhaseMonths:    30,
	},
	ptypes.JurisdictionUS: {
		Code:                      ptypes.JurisdictionUS,
		PatentTermYears:           20,
		AnnuityStartYear:          0, // US uses milestone-based maintenance fees
		AnnuityFrequency:          "milestone",
		GracePeriodMonths:         6,
		SurchargeRate:             0.50,
		ExaminationDeadlineMonths: 0, // US has automatic examination
		OAResponseDeadlineMonths:  3,
		PCTNationalPhaseMonths:    30,
	},
	ptypes.JurisdictionEP: {
		Code:                      ptypes.JurisdictionEP,
		PatentTermYears:           20,
		AnnuityStartYear:          3,
		AnnuityFrequency:          "annual",
		GracePeriodMonths:         6,
		SurchargeRate:             0.10,
		ExaminationDeadlineMonths: 0, // EP has automatic examination
		OAResponseDeadlineMonths:  4,
		PCTNationalPhaseMonths:    31,
	},
	ptypes.JurisdictionJP: {
		Code:                      ptypes.JurisdictionJP,
		PatentTermYears:           20,
		AnnuityStartYear:          1,
		AnnuityFrequency:          "annual",
		GracePeriodMonths:         6,
		SurchargeRate:             0.20,
		ExaminationDeadlineMonths: 36,
		OAResponseDeadlineMonths:  3,
		PCTNationalPhaseMonths:    30,
	},
	ptypes.JurisdictionKR: {
		Code:                      ptypes.JurisdictionKR,
		PatentTermYears:           20,
		AnnuityStartYear:          1,
		AnnuityFrequency:          "annual",
		GracePeriodMonths:         6,
		SurchargeRate:             0.30,
		ExaminationDeadlineMonths: 36,
		OAResponseDeadlineMonths:  2,
		PCTNationalPhaseMonths:    31,
	},
	ptypes.JurisdictionWO: {
		Code:                      ptypes.JurisdictionWO,
		PatentTermYears:           0, // PCT is not a granting jurisdiction
		AnnuityStartYear:          0,
		AnnuityFrequency:          "none",
		GracePeriodMonths:         0,
		SurchargeRate:             0,
		ExaminationDeadlineMonths: 0,
		OAResponseDeadlineMonths:  3,
		PCTNationalPhaseMonths:    30, // or 31 depending on region
	},
}

// ─────────────────────────────────────────────────────────────────────────────
// Public API
// ─────────────────────────────────────────────────────────────────────────────

// GetJurisdictionRules retrieves the rules for a specific jurisdiction.
func GetJurisdictionRules(code ptypes.JurisdictionCode) (*JurisdictionRules, error) {
	rules, ok := jurisdictionRegistry[code]
	if !ok {
		return nil, errors.NotFound(fmt.Sprintf("jurisdiction %s not found in registry", code))
	}
	return &rules, nil
}

// GetAllJurisdictions returns a list of all registered jurisdictions.
func GetAllJurisdictions() []JurisdictionRules {
	var all []JurisdictionRules
	for _, rules := range jurisdictionRegistry {
		all = append(all, rules)
	}
	return all
}

// CalculateExpiryDate computes the statutory expiry date for a patent.
// For most jurisdictions, this is FilingDate + PatentTermYears.
func CalculateExpiryDate(jurisdiction ptypes.JurisdictionCode, filingDate time.Time) time.Time {
	rules, err := GetJurisdictionRules(jurisdiction)
	if err != nil {
		// Fallback to 20 years if jurisdiction not found.
		return filingDate.AddDate(20, 0, 0)
	}
	return filingDate.AddDate(rules.PatentTermYears, 0, 0)
}

// GenerateInitialDeadlines creates the standard deadlines that apply to a
// newly filed patent application in the specified jurisdiction.
func GenerateInitialDeadlines(jurisdiction ptypes.JurisdictionCode, filingDate time.Time) ([]Deadline, error) {
	rules, err := GetJurisdictionRules(jurisdiction)
	if err != nil {
		return nil, err
	}

	var deadlines []Deadline

	// Add examination request deadline if applicable.
	if rules.ExaminationDeadlineMonths > 0 {
		examDueDate := filingDate.AddDate(0, rules.ExaminationDeadlineMonths, 0)
		examDeadline, _ := NewDeadline(
			DeadlineExamination,
			examDueDate,
			PriorityHigh,
			fmt.Sprintf("Request substantive examination (within %d months of filing)", rules.ExaminationDeadlineMonths),
		)
		examDeadline.ExtensionAvailable = false
		deadlines = append(deadlines, *examDeadline)
	}

	// PCT national phase entry deadline (only if jurisdiction is WO or if this is a PCT case).
	if jurisdiction == ptypes.JurisdictionWO && rules.PCTNationalPhaseMonths > 0 {
		pctDueDate := filingDate.AddDate(0, rules.PCTNationalPhaseMonths, 0)
		pctDeadline, _ := NewDeadline(
			DeadlinePCTNationalPhase,
			pctDueDate,
			PriorityCritical,
			fmt.Sprintf("Enter national phase (within %d months of priority date)", rules.PCTNationalPhaseMonths),
		)
		pctDeadline.ExtensionAvailable = false
		deadlines = append(deadlines, *pctDeadline)
	}

	return deadlines, nil
}

