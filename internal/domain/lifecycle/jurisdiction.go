package lifecycle

import (
	"sort"
	"strings"
	"time"

	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// JurisdictionInfo represents a legal jurisdiction's rules and metadata.
type JurisdictionInfo struct {
	Code                 string `json:"code"`
	Name                 string `json:"name"`
	PatentOffice         string `json:"patent_office"`
	InventionTermYears   int    `json:"invention_term_years"`
	UtilityModelTermYears int    `json:"utility_model_term_years"`
	DesignTermYears      int    `json:"design_term_years"`
	AnnuityStartYear     int    `json:"annuity_start_year"`
	GracePeriodMonths    int    `json:"grace_period_months"`
	SupportsUtilityModel bool   `json:"supports_utility_model"`
	SupportsPCT          bool   `json:"supports_pct"`
	OAResponseMonths     int    `json:"oa_response_months"`
	OAExtensionAvailable bool   `json:"oa_extension_available"`
	OAMaxExtensionMonths int    `json:"oa_max_extension_months"`
	Currency             string `json:"currency"`
	Language             string `json:"language"`
}

// AnnuityRules defines rules for annuity payments.
type AnnuityRules struct {
	JurisdictionCode string             `json:"jurisdiction_code"`
	StartYear        int                `json:"start_year"`
	EndYear          int                `json:"end_year"`
	IsAnnual         bool               `json:"is_annual"`
	PaymentSchedule  []PaymentMilestone `json:"payment_schedule"`
	GracePeriodMonths int               `json:"grace_period_months"`
	LateFeeMultiplier float64           `json:"late_fee_multiplier"`
}

// PaymentMilestone defines a specific payment point for non-annual systems.
type PaymentMilestone struct {
	YearMark    float64 `json:"year_mark"`
	Description string  `json:"description"`
}

// OAResponseRules defines rules for office action responses.
type OAResponseRules struct {
	JurisdictionCode     string `json:"jurisdiction_code"`
	ResponseMonths       int    `json:"response_months"`
	ExtensionAvailable   bool   `json:"extension_available"`
	MaxExtensionMonths   int    `json:"max_extension_months"`
	ExtensionFeeRequired bool   `json:"extension_fee_required"`
}

// JurisdictionRegistry defines the interface for accessing jurisdiction data.
type JurisdictionRegistry interface {
	Get(code string) (*JurisdictionInfo, error)
	List() []*JurisdictionInfo
	IsSupported(code string) bool
	GetPatentTerm(code string, patentType string) (int, error)
	GetAnnuityRules(code string) (*AnnuityRules, error)
	GetOAResponseRules(code string) (*OAResponseRules, error)
}

// inMemoryJurisdictionRegistry is an in-memory implementation of JurisdictionRegistry.
type inMemoryJurisdictionRegistry struct {
	jurisdictions map[string]*JurisdictionInfo
}

// NewJurisdictionRegistry creates a new pre-loaded registry.
func NewJurisdictionRegistry() JurisdictionRegistry {
	r := &inMemoryJurisdictionRegistry{
		jurisdictions: make(map[string]*JurisdictionInfo),
	}
	r.preload()
	return r
}

func (r *inMemoryJurisdictionRegistry) preload() {
	list := []*JurisdictionInfo{
		{
			Code:                 "CN",
			Name:                 "China",
			PatentOffice:         "CNIPA",
			InventionTermYears:   20,
			UtilityModelTermYears: 10,
			DesignTermYears:      15,
			AnnuityStartYear:     3,
			GracePeriodMonths:    6,
			SupportsUtilityModel: true,
			SupportsPCT:          true,
			OAResponseMonths:     4,
			OAExtensionAvailable: true,
			OAMaxExtensionMonths: 2,
			Currency:             "CNY",
			Language:             "zh",
		},
		{
			Code:                 "US",
			Name:                 "United States",
			PatentOffice:         "USPTO",
			InventionTermYears:   20,
			UtilityModelTermYears: 0,
			DesignTermYears:      15,
			AnnuityStartYear:     0,
			GracePeriodMonths:    6,
			SupportsUtilityModel: false,
			SupportsPCT:          true,
			OAResponseMonths:     3,
			OAExtensionAvailable: true,
			OAMaxExtensionMonths: 6,
			Currency:             "USD",
			Language:             "en",
		},
		{
			Code:                 "EP",
			Name:                 "European Patent Office",
			PatentOffice:         "EPO",
			InventionTermYears:   20,
			UtilityModelTermYears: 0,
			DesignTermYears:      25,
			AnnuityStartYear:     3,
			GracePeriodMonths:    6,
			SupportsUtilityModel: false,
			SupportsPCT:          true,
			OAResponseMonths:     4,
			OAExtensionAvailable: true,
			OAMaxExtensionMonths: 2,
			Currency:             "EUR",
			Language:             "en",
		},
		{
			Code:                 "JP",
			Name:                 "Japan",
			PatentOffice:         "JPO",
			InventionTermYears:   20,
			UtilityModelTermYears: 10,
			DesignTermYears:      25,
			AnnuityStartYear:     1,
			GracePeriodMonths:    6,
			SupportsUtilityModel: true,
			SupportsPCT:          true,
			OAResponseMonths:     2,
			OAExtensionAvailable: true,
			OAMaxExtensionMonths: 2,
			Currency:             "JPY",
			Language:             "ja",
		},
		{
			Code:                 "KR",
			Name:                 "South Korea",
			PatentOffice:         "KIPO",
			InventionTermYears:   20,
			UtilityModelTermYears: 10,
			DesignTermYears:      20,
			AnnuityStartYear:     1,
			GracePeriodMonths:    6,
			SupportsUtilityModel: true,
			SupportsPCT:          true,
			OAResponseMonths:     2,
			OAExtensionAvailable: true,
			OAMaxExtensionMonths: 2,
			Currency:             "KRW",
			Language:             "ko",
		},
		{
			Code:                 "WO",
			Name:                 "WIPO (PCT)",
			PatentOffice:         "WIPO",
			InventionTermYears:   0,
			UtilityModelTermYears: 0,
			DesignTermYears:      0,
			AnnuityStartYear:     0,
			GracePeriodMonths:    0,
			SupportsUtilityModel: false,
			SupportsPCT:          true,
			OAResponseMonths:     0,
			OAExtensionAvailable: false,
			OAMaxExtensionMonths: 0,
			Currency:             "CHF",
			Language:             "en",
		},
		{
			Code:                 "DE",
			Name:                 "Germany",
			PatentOffice:         "DPMA",
			InventionTermYears:   20,
			UtilityModelTermYears: 10,
			DesignTermYears:      25,
			AnnuityStartYear:     3,
			GracePeriodMonths:    6,
			SupportsUtilityModel: true,
			SupportsPCT:          true,
			OAResponseMonths:     4,
			OAExtensionAvailable: true,
			OAMaxExtensionMonths: 2,
			Currency:             "EUR",
			Language:             "de",
		},
		{
			Code:                 "GB",
			Name:                 "United Kingdom",
			PatentOffice:         "UKIPO",
			InventionTermYears:   20,
			UtilityModelTermYears: 0,
			DesignTermYears:      25,
			AnnuityStartYear:     5,
			GracePeriodMonths:    1,
			SupportsUtilityModel: false,
			SupportsPCT:          true,
			OAResponseMonths:     4,
			OAExtensionAvailable: true,
			OAMaxExtensionMonths: 2,
			Currency:             "GBP",
			Language:             "en",
		},
		{
			Code:                 "IN",
			Name:                 "India",
			PatentOffice:         "IPO",
			InventionTermYears:   20,
			UtilityModelTermYears: 0,
			DesignTermYears:      15,
			AnnuityStartYear:     3,
			GracePeriodMonths:    6,
			SupportsUtilityModel: false,
			SupportsPCT:          true,
			OAResponseMonths:     6,
			OAExtensionAvailable: true,
			OAMaxExtensionMonths: 3,
			Currency:             "INR",
			Language:             "en",
		},
	}

	for _, j := range list {
		r.jurisdictions[j.Code] = j
	}
}

func (r *inMemoryJurisdictionRegistry) Get(code string) (*JurisdictionInfo, error) {
	normalized := NormalizeJurisdictionCode(code)
	if j, ok := r.jurisdictions[normalized]; ok {
		return j, nil
	}
	return nil, apperrors.NewNotFound("jurisdiction not found: %s", code)
}

func (r *inMemoryJurisdictionRegistry) List() []*JurisdictionInfo {
	list := make([]*JurisdictionInfo, 0, len(r.jurisdictions))
	for _, j := range r.jurisdictions {
		list = append(list, j)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Code < list[j].Code
	})
	return list
}

func (r *inMemoryJurisdictionRegistry) IsSupported(code string) bool {
	_, ok := r.jurisdictions[NormalizeJurisdictionCode(code)]
	return ok
}

func (r *inMemoryJurisdictionRegistry) GetPatentTerm(code, patentType string) (int, error) {
	j, err := r.Get(code)
	if err != nil {
		return 0, err
	}
	switch patentType {
	case "invention":
		return j.InventionTermYears, nil
	case "utility_model":
		if !j.SupportsUtilityModel {
			return 0, apperrors.NewValidation("utility model not supported in %s", j.Name)
		}
		return j.UtilityModelTermYears, nil
	case "design":
		return j.DesignTermYears, nil
	default:
		return 0, apperrors.NewValidation("invalid patent type: %s", patentType)
	}
}

func (r *inMemoryJurisdictionRegistry) GetAnnuityRules(code string) (*AnnuityRules, error) {
	j, err := r.Get(code)
	if err != nil {
		return nil, err
	}

	rules := &AnnuityRules{
		JurisdictionCode:  j.Code,
		StartYear:         j.AnnuityStartYear,
		EndYear:           j.InventionTermYears,
		IsAnnual:          true,
		GracePeriodMonths: j.GracePeriodMonths,
		LateFeeMultiplier: 1.5,
	}

	if j.Code == "US" {
		rules.IsAnnual = false
		rules.PaymentSchedule = []PaymentMilestone{
			{YearMark: 3.5, Description: "3.5 Year Maintenance Fee"},
			{YearMark: 7.5, Description: "7.5 Year Maintenance Fee"},
			{YearMark: 11.5, Description: "11.5 Year Maintenance Fee"},
		}
	}

	return rules, nil
}

func (r *inMemoryJurisdictionRegistry) GetOAResponseRules(code string) (*OAResponseRules, error) {
	j, err := r.Get(code)
	if err != nil {
		return nil, err
	}

	return &OAResponseRules{
		JurisdictionCode:     j.Code,
		ResponseMonths:       j.OAResponseMonths,
		ExtensionAvailable:   j.OAExtensionAvailable,
		MaxExtensionMonths:   j.OAMaxExtensionMonths,
		ExtensionFeeRequired: j.Code == "US",
	}, nil
}

// NormalizeJurisdictionCode normalizes the jurisdiction code.
func NormalizeJurisdictionCode(code string) string {
	code = strings.TrimSpace(strings.ToUpper(code))
	switch code {
	case "CHINA":
		return "CN"
	case "USA":
		return "US"
	case "EUROPE":
		return "EP"
	case "JAPAN":
		return "JP"
	}
	return code
}

// CalculateExpirationDate calculates the patent expiration date.
func CalculateExpirationDate(filingDate time.Time, jurisdictionCode, patentType string) (time.Time, error) {
	reg := NewJurisdictionRegistry()
	term, err := reg.GetPatentTerm(jurisdictionCode, patentType)
	if err != nil {
		return time.Time{}, err
	}
	return filingDate.AddDate(term, 0, 0), nil
}

// CalculateAnnuityDueDate calculates the due date for a specific annuity year.
func CalculateAnnuityDueDate(filingDate time.Time, yearNumber int) time.Time {
	return filingDate.AddDate(yearNumber, 0, 0)
}

// CalculateGraceDeadline calculates the deadline including grace period.
func CalculateGraceDeadline(dueDate time.Time, gracePeriodMonths int) time.Time {
	return dueDate.AddDate(0, gracePeriodMonths, 0)
}

//Personal.AI order the ending
