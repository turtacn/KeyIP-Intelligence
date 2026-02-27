package lifecycle

import (
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Jurisdiction represents a legal jurisdiction.
type Jurisdiction string

const (
	JurisdictionCN Jurisdiction = "CN"
	JurisdictionUS Jurisdiction = "US"
	JurisdictionEP Jurisdiction = "EP"
	JurisdictionJP Jurisdiction = "JP"
	JurisdictionKR Jurisdiction = "KR"
	// Add others as needed
)

// JurisdictionInfo holds metadata about a jurisdiction.
type JurisdictionInfo struct {
	Code              Jurisdiction
	Name              string
	Currency          string
	AnnuityRuleSet    string
	OfficialLanguages []string
}

// JurisdictionRegistry provides access to jurisdiction data.
type JurisdictionRegistry interface {
	Get(code string) (*JurisdictionInfo, error)
	Normalize(code string) (Jurisdiction, error)
	List() []*JurisdictionInfo
}

// InMemoryJurisdictionRegistry is an in-memory implementation of JurisdictionRegistry.
type InMemoryJurisdictionRegistry struct {
	jurisdictions map[Jurisdiction]*JurisdictionInfo
	aliases       map[string]Jurisdiction
}

// NewJurisdictionRegistry creates a new registry with default data.
func NewJurisdictionRegistry() JurisdictionRegistry {
	r := &InMemoryJurisdictionRegistry{
		jurisdictions: make(map[Jurisdiction]*JurisdictionInfo),
		aliases:       make(map[string]Jurisdiction),
	}
	r.init()
	return r
}

func (r *InMemoryJurisdictionRegistry) init() {
	// CN
	r.add(JurisdictionCN, "China", "CNY", "CN_ANNUITY_V1", []string{"zh"})
	r.addAlias("CHN", JurisdictionCN)
	r.addAlias("CHINA", JurisdictionCN)

	// US
	r.add(JurisdictionUS, "United States", "USD", "US_ANNUITY_V1", []string{"en"})
	r.addAlias("USA", JurisdictionUS)
	r.addAlias("UNITED STATES", JurisdictionUS)

	// EP
	r.add(JurisdictionEP, "European Patent Office", "EUR", "EP_ANNUITY_V1", []string{"en", "fr", "de"})
	r.addAlias("EPO", JurisdictionEP)
	r.addAlias("EU", JurisdictionEP) // Colloquial

	// JP
	r.add(JurisdictionJP, "Japan", "JPY", "JP_ANNUITY_V1", []string{"ja"})
	r.addAlias("JPN", JurisdictionJP)
	r.addAlias("JAPAN", JurisdictionJP)

	// KR
	r.add(JurisdictionKR, "South Korea", "KRW", "KR_ANNUITY_V1", []string{"ko"})
	r.addAlias("KOR", JurisdictionKR)
	r.addAlias("KOREA", JurisdictionKR)
}

func (r *InMemoryJurisdictionRegistry) add(code Jurisdiction, name, currency, ruleSet string, langs []string) {
	r.jurisdictions[code] = &JurisdictionInfo{
		Code:              code,
		Name:              name,
		Currency:          currency,
		AnnuityRuleSet:    ruleSet,
		OfficialLanguages: langs,
	}
}

func (r *InMemoryJurisdictionRegistry) addAlias(alias string, target Jurisdiction) {
	r.aliases[strings.ToUpper(alias)] = target
}

// Get returns information for a specific jurisdiction code.
func (r *InMemoryJurisdictionRegistry) Get(code string) (*JurisdictionInfo, error) {
	normalized, err := r.Normalize(code)
	if err != nil {
		return nil, err
	}
	if info, ok := r.jurisdictions[normalized]; ok {
		return info, nil
	}
	return nil, errors.NewNotFound("jurisdiction not found: %s", code)
}

// Normalize converts a code or alias to the standard Jurisdiction type.
func (r *InMemoryJurisdictionRegistry) Normalize(code string) (Jurisdiction, error) {
	upper := strings.ToUpper(strings.TrimSpace(code))
	if j, ok := r.jurisdictions[Jurisdiction(upper)]; ok {
		return j.Code, nil
	}
	if j, ok := r.aliases[upper]; ok {
		return j, nil
	}
	return "", errors.NewValidation("invalid jurisdiction code: %s", code)
}

// List returns all supported jurisdictions.
func (r *InMemoryJurisdictionRegistry) List() []*JurisdictionInfo {
	list := make([]*JurisdictionInfo, 0, len(r.jurisdictions))
	for _, info := range r.jurisdictions {
		list = append(list, info)
	}
	return list
}

//Personal.AI order the ending
