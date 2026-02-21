package lifecycle

import (
	"fmt"
	"strings"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Jurisdiction represents the rules of a patent jurisdiction.
type Jurisdiction struct {
	Code                      string `json:"code"`
	Name                      string `json:"name"`
	PatentOffice              string `json:"patent_office"`
	InventionTermYears        int    `json:"invention_term_years"`
	UtilityModelTermYears     int    `json:"utility_model_term_years"`
	DesignTermYears           int    `json:"design_term_years"`
	AnnuityStartYear          int    `json:"annuity_start_year"`
	GracePeriodMonths         int    `json:"grace_period_months"`
	SupportsUtilityModel      bool   `json:"supports_utility_model"`
	SupportsPCT               bool   `json:"supports_pct"`
	OAResponseMonths          int    `json:"oa_response_months"`
	OAExtensionAvailable      bool   `json:"oa_extension_available"`
	OAMaxExtensionMonths      int    `json:"oa_max_extension_months"`
	Currency                  string `json:"currency"`
	Language                  string `json:"language"`
}

// AnnuityRules encapsulates annuity-specific rules.
type AnnuityRules struct {
	JurisdictionCode   string             `json:"jurisdiction_code"`
	StartYear          int                `json:"start_year"`
	EndYear            int                `json:"end_year"`
	IsAnnual           bool               `json:"is_annual"`
	PaymentSchedule    []PaymentMilestone `json:"payment_schedule"`
	GracePeriodMonths  int                `json:"grace_period_months"`
	LateFeeMultiplier float64            `json:"late_fee_multiplier"`
}

// PaymentMilestone represents a non-annual payment point.
type PaymentMilestone struct {
	YearMark    float64 `json:"year_mark"`
	Description string  `json:"description"`
}

// OAResponseRules encapsulates office action response rules.
type OAResponseRules struct {
	JurisdictionCode    string `json:"jurisdiction_code"`
	ResponseMonths      int    `json:"response_months"`
	ExtensionAvailable  bool   `json:"extension_available"`
	MaxExtensionMonths  int    `json:"max_extension_months"`
	ExtensionFeeRequired bool   `json:"extension_fee_required"`
}

// JurisdictionRegistry defines the interface for jurisdiction rule management.
type JurisdictionRegistry interface {
	Get(code string) (*Jurisdiction, error)
	List() []*Jurisdiction
	IsSupported(code string) bool
	GetPatentTerm(code string, patentType string) (int, error)
	GetAnnuityRules(code string) (*AnnuityRules, error)
	GetOAResponseRules(code string) (*OAResponseRules, error)
}

type inMemoryJurisdictionRegistry struct {
	jurisdictions map[string]*Jurisdiction
}

// NewJurisdictionRegistry creates a new registry with preloaded data.
func NewJurisdictionRegistry() JurisdictionRegistry {
	r := &inMemoryJurisdictionRegistry{
		jurisdictions: make(map[string]*Jurisdiction),
	}
	r.preload()
	return r
}

func (r *inMemoryJurisdictionRegistry) preload() {
	r.jurisdictions["CN"] = &Jurisdiction{
		Code: "CN", Name: "China", PatentOffice: "CNIPA",
		InventionTermYears: 20, UtilityModelTermYears: 10, DesignTermYears: 15,
		AnnuityStartYear: 3, GracePeriodMonths: 6,
		SupportsUtilityModel: true, SupportsPCT: true,
		OAResponseMonths: 4, OAExtensionAvailable: true, OAMaxExtensionMonths: 2,
		Currency: "CNY", Language: "zh",
	}
	r.jurisdictions["US"] = &Jurisdiction{
		Code: "US", Name: "United States", PatentOffice: "USPTO",
		InventionTermYears: 20, UtilityModelTermYears: 0, DesignTermYears: 15,
		AnnuityStartYear: 0, GracePeriodMonths: 6,
		SupportsUtilityModel: false, SupportsPCT: true,
		OAResponseMonths: 3, OAExtensionAvailable: true, OAMaxExtensionMonths: 6,
		Currency: "USD", Language: "en",
	}
	r.jurisdictions["EP"] = &Jurisdiction{
		Code: "EP", Name: "Europe", PatentOffice: "EPO",
		InventionTermYears: 20, UtilityModelTermYears: 0, DesignTermYears: 25,
		AnnuityStartYear: 3, GracePeriodMonths: 6,
		SupportsUtilityModel: false, SupportsPCT: true,
		OAResponseMonths: 4, OAExtensionAvailable: true, OAMaxExtensionMonths: 2,
		Currency: "EUR", Language: "en",
	}
	r.jurisdictions["JP"] = &Jurisdiction{
		Code: "JP", Name: "Japan", PatentOffice: "JPO",
		InventionTermYears: 20, UtilityModelTermYears: 10, DesignTermYears: 25,
		AnnuityStartYear: 1, GracePeriodMonths: 6,
		SupportsUtilityModel: true, SupportsPCT: true,
		OAResponseMonths: 3, OAExtensionAvailable: true, OAMaxExtensionMonths: 2, // Simplified OAResponse
		Currency: "JPY", Language: "ja",
	}
	r.jurisdictions["KR"] = &Jurisdiction{
		Code: "KR", Name: "South Korea", PatentOffice: "KIPO",
		InventionTermYears: 20, UtilityModelTermYears: 10, DesignTermYears: 20,
		AnnuityStartYear: 1, GracePeriodMonths: 6,
		SupportsUtilityModel: true, SupportsPCT: true,
		OAResponseMonths: 2, OAExtensionAvailable: true, OAMaxExtensionMonths: 2,
		Currency: "KRW", Language: "ko",
	}
	r.jurisdictions["WO"] = &Jurisdiction{
		Code: "WO", Name: "PCT", PatentOffice: "WIPO",
		InventionTermYears: 0, SupportsPCT: true,
		OAResponseMonths: 3, Language: "en",
	}
	r.jurisdictions["DE"] = &Jurisdiction{
		Code: "DE", Name: "Germany", PatentOffice: "DPMA",
		InventionTermYears: 20, UtilityModelTermYears: 10, DesignTermYears: 25,
		AnnuityStartYear: 3, GracePeriodMonths: 6,
		Currency: "EUR", Language: "de",
	}
	r.jurisdictions["GB"] = &Jurisdiction{
		Code: "GB", Name: "United Kingdom", PatentOffice: "UKIPO",
		InventionTermYears: 20, DesignTermYears: 25,
		AnnuityStartYear: 5, GracePeriodMonths: 1,
		Currency: "GBP", Language: "en",
	}
	r.jurisdictions["IN"] = &Jurisdiction{
		Code: "IN", Name: "India", PatentOffice: "IPO",
		InventionTermYears: 20, DesignTermYears: 15,
		AnnuityStartYear: 3, GracePeriodMonths: 6,
		Currency: "INR", Language: "en",
	}
}

func (r *inMemoryJurisdictionRegistry) Get(code string) (*Jurisdiction, error) {
	j, ok := r.jurisdictions[NormalizeJurisdictionCode(code)]
	if !ok {
		return nil, errors.NotFound(fmt.Sprintf("jurisdiction %s not found", code))
	}
	return j, nil
}

func (r *inMemoryJurisdictionRegistry) List() []*Jurisdiction {
	var list []*Jurisdiction
	codes := []string{"CN", "DE", "EP", "GB", "IN", "JP", "KR", "US", "WO"} // Alphabetical
	for _, c := range codes {
		if j, ok := r.jurisdictions[c]; ok {
			list = append(list, j)
		}
	}
	return list
}

func (r *inMemoryJurisdictionRegistry) IsSupported(code string) bool {
	_, ok := r.jurisdictions[NormalizeJurisdictionCode(code)]
	return ok
}

func (r *inMemoryJurisdictionRegistry) GetPatentTerm(code string, patentType string) (int, error) {
	j, err := r.Get(code)
	if err != nil {
		return 0, err
	}
	switch patentType {
	case "invention":
		return j.InventionTermYears, nil
	case "utility_model":
		if !j.SupportsUtilityModel {
			return 0, errors.InvalidParam(fmt.Sprintf("%s does not support utility models", code))
		}
		return j.UtilityModelTermYears, nil
	case "design":
		return j.DesignTermYears, nil
	default:
		return 0, errors.InvalidParam(fmt.Sprintf("invalid patent type: %s", patentType))
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
		IsAnnual:          j.Code != "US",
		GracePeriodMonths: j.GracePeriodMonths,
		LateFeeMultiplier: 0.25, // Default
	}
	if j.Code == "US" {
		rules.PaymentSchedule = []PaymentMilestone{
			{YearMark: 3.5, Description: "1st Maintenance Fee"},
			{YearMark: 7.5, Description: "2nd Maintenance Fee"},
			{YearMark: 11.5, Description: "3rd Maintenance Fee"},
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
		JurisdictionCode:    j.Code,
		ResponseMonths:      j.OAResponseMonths,
		ExtensionAvailable:  j.OAExtensionAvailable,
		MaxExtensionMonths:  j.OAMaxExtensionMonths,
		ExtensionFeeRequired: j.Code == "US",
	}, nil
}

// NormalizeJurisdictionCode standardizes jurisdiction codes.
func NormalizeJurisdictionCode(code string) string {
	c := strings.ToUpper(strings.TrimSpace(code))
	switch c {
	case "CHINA":
		return "CN"
	case "USA", "UNITED STATES":
		return "US"
	case "EUROPE":
		return "EP"
	case "JAPAN":
		return "JP"
	}
	return c
}

// CalculateExpirationDate computes the expiration date.
func CalculateExpirationDate(filingDate time.Time, jurisdictionCode, patentType string) (time.Time, error) {
	reg := NewJurisdictionRegistry()
	term, err := reg.GetPatentTerm(jurisdictionCode, patentType)
	if err != nil {
		return time.Time{}, err
	}
	return filingDate.AddDate(term, 0, 0), nil
}

// CalculateAnnuityDueDate computes the due date for a specific year.
func CalculateAnnuityDueDate(filingDate time.Time, yearNumber int) time.Time {
	return filingDate.AddDate(yearNumber, 0, 0)
}

// CalculateGraceDeadline computes the grace period deadline.
func CalculateGraceDeadline(dueDate time.Time, gracePeriodMonths int) time.Time {
	return dueDate.AddDate(0, gracePeriodMonths, 0)
}

//Personal.AI order the ending
