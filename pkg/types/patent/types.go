package patent

import (
	"fmt"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// PatentStatus represents the lifecycle stage of a patent.
type PatentStatus string

const (
	StatusPending          PatentStatus = "pending"
	StatusUnderExamination PatentStatus = "under_examination"
	StatusGranted          PatentStatus = "granted"
	StatusExpired          PatentStatus = "expired"
	StatusAbandoned        PatentStatus = "abandoned"
	StatusRevoked          PatentStatus = "revoked"
	StatusLapsed           PatentStatus = "lapsed"
)

// IsValid checks if the PatentStatus is valid.
func (s PatentStatus) IsValid() bool {
	switch s {
	case StatusPending, StatusUnderExamination, StatusGranted, StatusExpired, StatusAbandoned, StatusRevoked, StatusLapsed:
		return true
	default:
		return false
	}
}

// IsActive checks if the patent is in an active state.
func (s PatentStatus) IsActive() bool {
	return s == StatusPending || s == StatusUnderExamination || s == StatusGranted
}

// PatentOffice identifies a patent office.
type PatentOffice string

const (
	OfficeCNIPA PatentOffice = "CNIPA"
	OfficeUSPTO PatentOffice = "USPTO"
	OfficeEPO   PatentOffice = "EPO"
	OfficeJPO   PatentOffice = "JPO"
	OfficeKIPO  PatentOffice = "KIPO"
	OfficeWIPO  PatentOffice = "WIPO"
)

// IsValid checks if the PatentOffice is supported.
func (o PatentOffice) IsValid() bool {
	switch o {
	case OfficeCNIPA, OfficeUSPTO, OfficeEPO, OfficeJPO, OfficeKIPO, OfficeWIPO:
		return true
	default:
		return false
	}
}

// ClaimType classifies a claim as independent or dependent.
type ClaimType string

const (
	ClaimIndependent ClaimType = "independent"
	ClaimDependent   ClaimType = "dependent"
)

// ClaimCategory classifies a claim by its subject matter.
type ClaimCategory string

const (
	ClaimProduct     ClaimCategory = "product"
	ClaimMethod      ClaimCategory = "method"
	ClaimUse         ClaimCategory = "use"
	ClaimComposition ClaimCategory = "composition"
)

// InfringementRiskLevel defines levels of infringement risk.
type InfringementRiskLevel string

const (
	RiskCritical InfringementRiskLevel = "critical"
	RiskHigh     InfringementRiskLevel = "high"
	RiskMedium   InfringementRiskLevel = "medium"
	RiskLow      InfringementRiskLevel = "low"
	RiskNone     InfringementRiskLevel = "none"
)

// IsValid checks if the InfringementRiskLevel is valid.
func (l InfringementRiskLevel) IsValid() bool {
	switch l {
	case RiskCritical, RiskHigh, RiskMedium, RiskLow, RiskNone:
		return true
	default:
		return false
	}
}

// Severity returns a numerical value representing the severity of the risk.
func (l InfringementRiskLevel) Severity() int {
	switch l {
	case RiskCritical:
		return 4
	case RiskHigh:
		return 3
	case RiskMedium:
		return 2
	case RiskLow:
		return 1
	default:
		return 0
	}
}

// PatentDTO represents a patent document.
type PatentDTO struct {
	ID                common.ID              `json:"id"`
	PatentNumber      string                 `json:"patent_number"`
	Title             string                 `json:"title"`
	Abstract          string                 `json:"abstract"`
	Assignee          string                 `json:"assignee"`
	Assignees         []string               `json:"assignees"`
	Inventors         []string               `json:"inventors"`
	FilingDate        common.Timestamp       `json:"filing_date"`
	PublicationDate   common.Timestamp       `json:"publication_date"`
	GrantDate         *common.Timestamp      `json:"grant_date,omitempty"`
	ExpiryDate        *common.Timestamp      `json:"expiry_date,omitempty"`
	Status            PatentStatus           `json:"status"`
	Office            PatentOffice           `json:"office"`
	IPCCodes          []string               `json:"ipc_codes"`
	CPCCodes          []string               `json:"cpc_codes"`
	FamilyID          string                 `json:"family_id"`
	Priority          []PriorityInfo         `json:"priority"`
	Claims            []ClaimDTO             `json:"claims"`
	Molecules         []molecule.MoleculeDTO `json:"molecules"`
	CitedBy           []string               `json:"cited_by"`
	Cites             []string               `json:"cites"`
	FullTextAvailable bool                   `json:"full_text_available"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt         common.Timestamp       `json:"created_at"`
	UpdatedAt         common.Timestamp       `json:"updated_at"`

	// Legacy fields
	Applicant         string           `json:"applicant,omitempty"`
	Jurisdiction      JurisdictionCode `json:"jurisdiction,omitempty"`
	MarkushStructures []MarkushDTO     `json:"markush_structures,omitempty"`
	TenantID          common.TenantID  `json:"tenant_id,omitempty"`
	Version           int              `json:"version"`
}

// PriorityInfo represents a priority claim.
type PriorityInfo struct {
	PatentNumber string           `json:"patent_number"`
	FilingDate   common.Timestamp `json:"filing_date"`
	Office       PatentOffice     `json:"office"`

	// Legacy fields
	Country string           `json:"country,omitempty"`
	Number  string           `json:"number,omitempty"`
	Date    common.Timestamp `json:"date,omitempty"`
}

// ClaimDTO represents a patent claim.
type ClaimDTO struct {
	ClaimNumber   int               `json:"claim_number"`
	Type          ClaimType         `json:"type"`
	Category      ClaimCategory     `json:"category"`
	Text          string            `json:"text"`
	DependsOn     []int             `json:"depends_on"`
	Elements      []ClaimElement    `json:"elements"`
	HasMarkush    bool              `json:"has_markush"`
	MarkushGroups []MarkushGroupDTO `json:"markush_groups,omitempty"`

	// Legacy fields
	ID                common.ID `json:"id,omitempty"`
	Number            int       `json:"number,omitempty"`
	ParentClaimNumber *int      `json:"parent_claim_number,omitempty"`
}

// ClaimElement represents a parsed element of a claim.
type ClaimElement struct {
	Index    int      `json:"index"`
	Text     string   `json:"text"`
	Type     string   `json:"type"` // structural / functional / process / property
	Keywords []string `json:"keywords"`

	// Legacy fields
	ID               common.ID `json:"id,omitempty"`
	IsStructural     bool      `json:"is_structural,omitempty"`
	ChemicalEntities []string  `json:"chemical_entities,omitempty"`
}

// MarkushGroupDTO represents a Markush structure group.
type MarkushGroupDTO struct {
	Position              string   `json:"position"`
	Options               []string `json:"options"`
	Description           string   `json:"description"`
	EstimatedCombinations int64    `json:"estimated_combinations"`
}

// InfringementRiskDTO represents the result of an infringement risk analysis.
type InfringementRiskDTO struct {
	Level                        InfringementRiskLevel `json:"level"`
	Score                        float64               `json:"score"`
	LiteralInfringementProbability float64               `json:"literal_infringement_probability"`
	EquivalentsProbability       float64               `json:"equivalents_probability"`
	ProsecutionHistoryEstoppel   bool                  `json:"prosecution_history_estoppel"`
	RelevantClaims               []int                 `json:"relevant_claims"`
	Recommendation               string                `json:"recommendation"`
	Confidence                   float64               `json:"confidence"`
	AnalyzedAt                   common.Timestamp      `json:"analyzed_at"`
}

// FTOAnalysisRequest carries parameters for a Freedom to Operate analysis.
type FTOAnalysisRequest struct {
	TargetMolecules     []molecule.MoleculeInput `json:"target_molecules"`
	Jurisdictions       []PatentOffice           `json:"jurisdictions"`
	IncludeExpired      bool                     `json:"include_expired"`
	IncludeEquivalents  bool                     `json:"include_equivalents"`
	IncludeDesignAround bool                     `json:"include_design_around"`
}

// Validate checks if the FTOAnalysisRequest is valid.
func (r FTOAnalysisRequest) Validate() error {
	if len(r.TargetMolecules) == 0 {
		return fmt.Errorf("at least one target molecule is required")
	}
	for _, m := range r.TargetMolecules {
		if err := m.Validate(); err != nil {
			return err
		}
	}
	if len(r.Jurisdictions) == 0 {
		return fmt.Errorf("at least one jurisdiction is required")
	}
	return nil
}

// FTOAnalysisResponse represents the result of an FTO analysis.
type FTOAnalysisResponse struct {
	RequestID       common.ID              `json:"request_id"`
	TargetMolecules []molecule.MoleculeDTO `json:"target_molecules"`
	RiskPatents     []FTORiskPatent        `json:"risk_patents"`
	OverallRisk     InfringementRiskLevel  `json:"overall_risk"`
	Summary         string                 `json:"summary"`
	AnalyzedAt      common.Timestamp       `json:"analyzed_at"`
}

// FTORiskPatent pairs a patent with its infringement risk for a specific molecule.
type FTORiskPatent struct {
	Patent                  PatentDTO                   `json:"patent"`
	Risk                    InfringementRiskDTO         `json:"risk"`
	MatchedMolecules        []molecule.SimilarityResult `json:"matched_molecules"`
	DesignAroundSuggestions []molecule.MoleculeDTO      `json:"design_around_suggestions,omitempty"`
}

// PatentValueDTO represents the assessed value of a patent.
type PatentValueDTO struct {
	PatentID            common.ID              `json:"patent_id"`
	PatentNumber        string                 `json:"patent_number"`
	TechnicalValue      ScoreDimension         `json:"technical_value"`
	LegalValue          ScoreDimension         `json:"legal_value"`
	CommercialValue     ScoreDimension         `json:"commercial_value"`
	StrategicValue      ScoreDimension         `json:"strategic_value"`
	OverallScore        float64                `json:"overall_score"`
	Tier                string                 `json:"tier"` // S / A / B / C / D
	TierDescription     string                 `json:"tier_description"`
	WeightedCalculation string                 `json:"weighted_calculation"`
	Recommendations     []PatentRecommendation `json:"recommendations"`
	AssessedAt          common.Timestamp       `json:"assessed_at"`
}

// ScoreDimension represents a scored dimension of a patent.
type ScoreDimension struct {
	Score       float64            `json:"score"`
	MaxScore    float64            `json:"max_score"`
	Factors     map[string]float64 `json:"factors"`
	Explanation string             `json:"explanation"`
}

// PatentRecommendation represents a recommendation for a patent.
type PatentRecommendation struct {
	Type     string `json:"type"` // maintain / strengthen / enforce / abandon / license
	Priority string `json:"priority"` // critical / high / medium / low
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// PatentSearchRequest carries parameters for searching patents.
type PatentSearchRequest struct {
	Query       string             `json:"query"`
	Offices     []PatentOffice     `json:"offices"`
	Assignees   []string           `json:"assignees"`
	Inventors   []string           `json:"inventors"`
	IPCCodes    []string           `json:"ipc_codes"`
	DateRange   *common.DateRange  `json:"date_range"`
	Status      []PatentStatus     `json:"status"`
	HasMolecule *bool              `json:"has_molecule"`
	Pagination  common.Pagination  `json:"pagination"`
	Sort        []common.SortField `json:"sort"`
}

// Validate checks if the PatentSearchRequest is valid.
func (r PatentSearchRequest) Validate() error {
	if r.Query == "" && len(r.Offices) == 0 && len(r.Assignees) == 0 && len(r.Inventors) == 0 &&
		len(r.IPCCodes) == 0 && r.DateRange == nil && len(r.Status) == 0 && r.HasMolecule == nil {
		return fmt.Errorf("search request must contain at least one query or filter")
	}
	return nil
}

// CompetitorActivity represents recent patent activity of a competitor.
type CompetitorActivity struct {
	CompetitorName string           `json:"competitor_name"`
	Office         PatentOffice     `json:"office"`
	RecentFilings  int              `json:"recent_filings"`
	TechDomains    []string         `json:"tech_domains"`
	KeyInventors   []string         `json:"key_inventors"`
	TrendDirection string           `json:"trend_direction"` // increasing / stable / decreasing
	Period         common.DateRange `json:"period"`
}

// WatchlistConfig represents a configuration for monitoring patent activity.
type WatchlistConfig struct {
	ID           common.ID          `json:"id"`
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	WatchType    string             `json:"watch_type"` // molecule_similarity / keyword / assignee / inventor
	Targets      []WatchTarget      `json:"targets"`
	Scope        WatchScope         `json:"scope"`
	AlertConfig  AlertConfig        `json:"alert_config"`
	Schedule     ScheduleConfig     `json:"schedule"`
	AutoAnalysis AutoAnalysisConfig `json:"auto_analysis"`
	Status       string             `json:"status"` // active / paused / archived
	CreatedAt    common.Timestamp   `json:"created_at"`
	UpdatedAt    common.Timestamp   `json:"updated_at"`
}

// Validate checks if the WatchlistConfig is valid.
func (c WatchlistConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if c.WatchType == "" {
		return fmt.Errorf("watch_type cannot be empty")
	}
	if len(c.Targets) == 0 {
		return fmt.Errorf("at least one target is required")
	}
	if err := c.Schedule.Validate(); err != nil {
		return err
	}
	return nil
}

// WatchTarget represents a target to be monitored.
type WatchTarget struct {
	Type      string                 `json:"type"` // molecule / keyword / assignee / inventor / ipc_code
	Value     string                 `json:"value"`
	Threshold float64                `json:"threshold"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// WatchScope defines the scope of monitoring.
type WatchScope struct {
	Offices          []PatentOffice    `json:"offices"`
	IPCCodes         []string          `json:"ipc_codes"`
	Assignees        []string          `json:"assignees"`
	ExcludeAssignees []string          `json:"exclude_assignees"`
	DateRange        *common.DateRange `json:"date_range"`
}

// AlertConfig defines how alerts should be delivered.
type AlertConfig struct {
	Channels            []string              `json:"channels"` // email / webhook / slack / dingtalk
	MinRiskLevel        InfringementRiskLevel `json:"min_risk_level"`
	Recipients          []string              `json:"recipients"`
	WebhookURL          string                `json:"webhook_url,omitempty"`
	IncludeFullAnalysis bool                  `json:"include_full_analysis"`
	Cooldown            string                `json:"cooldown"`
}

// ScheduleConfig defines the execution schedule.
type ScheduleConfig struct {
	Frequency  string `json:"frequency"` // daily / weekly / biweekly / monthly / realtime
	DayOfWeek  *int   `json:"day_of_week,omitempty"`
	DayOfMonth *int   `json:"day_of_month,omitempty"`
	TimeOfDay  string `json:"time_of_day"` // HH:MM
	Timezone   string `json:"timezone"`
}

// Validate checks if the ScheduleConfig is valid.
func (s ScheduleConfig) Validate() error {
	switch s.Frequency {
	case "daily", "weekly", "biweekly", "monthly", "realtime":
	default:
		return fmt.Errorf("invalid frequency: %s", s.Frequency)
	}
	if s.TimeOfDay != "" {
		var h, m int
		if _, err := fmt.Sscanf(s.TimeOfDay, "%d:%d", &h, &m); err != nil {
			return fmt.Errorf("invalid time_of_day format: %s", s.TimeOfDay)
		}
		if h < 0 || h > 23 || m < 0 || m > 59 {
			return fmt.Errorf("invalid time values in time_of_day: %s", s.TimeOfDay)
		}
	}
	return nil
}

// AutoAnalysisConfig defines automatic analysis settings.
type AutoAnalysisConfig struct {
	EnableSimilaritySearch bool                       `json:"enable_similarity_search"`
	EnableInfringementRisk bool                       `json:"enable_infringement_risk"`
	EnableClaimAnalysis    bool                       `json:"enable_claim_analysis"`
	EnableDesignAround     bool                       `json:"enable_design_around"`
	EnablePatentValue      bool                       `json:"enable_patent_value"`
	SimilarityThreshold    float64                    `json:"similarity_threshold"`
	FingerprintTypes       []molecule.FingerprintType `json:"fingerprint_types"`
}

// WatchAlert represents an alert triggered by a watchlist.
type WatchAlert struct {
	ID               common.ID                 `json:"id"`
	WatchlistID      common.ID                 `json:"watchlist_id"`
	WatchlistName    string                    `json:"watchlist_name"`
	AlertType        string                    `json:"alert_type"` // new_patent / status_change / similarity_match / competitor_activity
	Severity         string                    `json:"severity"`   // critical / high / medium / low / info
	Title            string                    `json:"title"`
	Summary          string                    `json:"summary"`
	Patent           *PatentDTO                `json:"patent,omitempty"`
	SimilarityResult *molecule.SimilarityResult `json:"similarity_result,omitempty"`
	InfringementRisk *InfringementRiskDTO      `json:"infringement_risk,omitempty"`
	IsRead           bool                      `json:"is_read"`
	IsAcknowledged   bool                      `json:"is_acknowledged"`
	CreatedAt        common.Timestamp          `json:"created_at"`
}

// PatentLandscapeRequest carries parameters for generating a patent landscape.
type PatentLandscapeRequest struct {
	TechDomain                string            `json:"tech_domain"`
	IPCCodes                  []string          `json:"ipc_codes"`
	Offices                   []PatentOffice    `json:"offices"`
	DateRange                 common.DateRange  `json:"date_range"`
	TopAssignees              int               `json:"top_assignees"`
	IncludeTrends             bool              `json:"include_trends"`
	IncludeWhiteSpace         bool              `json:"include_white_space"`
	IncludeCompetitorAnalysis bool              `json:"include_competitor_analysis"`
	Pagination                common.Pagination `json:"pagination"`
}

// Validate checks if the PatentLandscapeRequest is valid.
func (r PatentLandscapeRequest) Validate() error {
	if r.TechDomain == "" && len(r.IPCCodes) == 0 {
		return fmt.Errorf("either tech_domain or ipc_codes must be specified")
	}
	if err := r.DateRange.Validate(); err != nil {
		return err
	}
	return nil
}

// PatentLandscapeResponse represents the result of a patent landscape analysis.
type PatentLandscapeResponse struct {
	TechDomain                string                 `json:"tech_domain"`
	TotalPatents              int64                  `json:"total_patents"`
	ActivePatents             int64                  `json:"active_patents"`
	TopAssignees              []AssigneeStats        `json:"top_assignees"`
	YearlyTrends              []YearlyTrend          `json:"yearly_trends"`
	TechDistribution          []TechDistributionItem `json:"tech_distribution"`
	WhiteSpaceAreas           []WhiteSpaceArea       `json:"white_space_areas,omitempty"`
	CompetitorActivities      []CompetitorActivity   `json:"competitor_activities,omitempty"`
	GeneratedAt               common.Timestamp       `json:"generated_at"`
}

// AssigneeStats represents statistics for a patent assignee.
type AssigneeStats struct {
	Name                string   `json:"name"`
	TotalPatents        int      `json:"total_patents"`
	ActivePatents       int      `json:"active_patents"`
	RecentFilings       int      `json:"recent_filings"`
	TopIPCCodes         []string `json:"top_ipc_codes"`
	AveragePatentValue  float64  `json:"average_patent_value"`
}

// YearlyTrend represents patent trends for a specific year.
type YearlyTrend struct {
	Year             int      `json:"year"`
	FilingCount      int      `json:"filing_count"`
	GrantCount       int      `json:"grant_count"`
	TopAssignees     []string `json:"top_assignees"`
	EmergingKeywords []string `json:"emerging_keywords"`
}

// TechDistributionItem represents technical distribution of patents.
type TechDistributionItem struct {
	IPCCode     string  `json:"ipc_code"`
	Description string  `json:"description"`
	Count       int     `json:"count"`
	Percentage  float64 `json:"percentage"`
}

// WhiteSpaceArea represents a potential opportunity area in the patent landscape.
type WhiteSpaceArea struct {
	Description      string                 `json:"description"`
	RelatedIPCCodes  []string               `json:"related_ipc_codes"`
	OpportunityScore float64                `json:"opportunity_score"`
	Rationale        string                 `json:"rationale"`
	SuggestedMolecules []molecule.MoleculeDTO `json:"suggested_molecules,omitempty"`
}

// PatentabilityRequest carries parameters for a patentability analysis.
type PatentabilityRequest struct {
	Molecule                     molecule.MoleculeInput `json:"molecule"`
	TechDomain                   string                 `json:"tech_domain"`
	Offices                      []PatentOffice         `json:"offices"`
	IncludeNoveltyAnalysis       bool                   `json:"include_novelty_analysis"`
	IncludeInventiveStepAnalysis bool                   `json:"include_inventive_step_analysis"`
	IncludePriorArtSearch        bool                   `json:"include_prior_art_search"`
}

// Validate checks if the PatentabilityRequest is valid.
func (r PatentabilityRequest) Validate() error {
	if err := r.Molecule.Validate(); err != nil {
		return err
	}
	if len(r.Offices) == 0 {
		return fmt.Errorf("at least one office is required")
	}
	return nil
}

// PatentabilityResponse represents the result of a patentability analysis.
type PatentabilityResponse struct {
	Molecule                     molecule.MoleculeDTO `json:"molecule"`
	OverallScore                float64              `json:"overall_score"`
	NoveltyScore                float64              `json:"novelty_score"`
	InventiveStepScore          float64              `json:"inventive_step_score"`
	IndustrialApplicabilityScore float64              `json:"industrial_applicability_score"`
	PriorArt                    []PatentDTO          `json:"prior_art"`
	NoveltyAnalysis             string               `json:"novelty_analysis"`
	InventiveStepAnalysis       string               `json:"inventive_step_analysis"`
	Recommendations             []string             `json:"recommendations"`
	AnalyzedAt                  common.Timestamp     `json:"analyzed_at"`
}

// DesignAroundRequest carries parameters for a design-around analysis.
type DesignAroundRequest struct {
	TargetPatent PatentDTO               `json:"target_patent"`
	TargetClaims []int                   `json:"target_claims"`
	BaseMolecule molecule.MoleculeInput  `json:"base_molecule"`
	Constraints  DesignAroundConstraints `json:"constraints"`
}

// Validate checks if the DesignAroundRequest is valid.
func (r DesignAroundRequest) Validate() error {
	if len(r.TargetClaims) == 0 {
		return fmt.Errorf("at least one target claim is required")
	}
	if err := r.BaseMolecule.Validate(); err != nil {
		return err
	}
	return nil
}

// DesignAroundConstraints defines constraints for a design-around task.
type DesignAroundConstraints struct {
	MaintainActivity      bool                        `json:"maintain_activity"`
	MaxStructuralChanges int                         `json:"max_structural_changes"`
	PreserveScaffold      bool                        `json:"preserve_scaffold"`
	ExcludedSubstructures []string                    `json:"excluded_substructures"`
	TargetProperties      []molecule.MaterialProperty `json:"target_properties"`
}

// DesignAroundResponse represents the result of a design-around analysis.
type DesignAroundResponse struct {
	BaseMolecule molecule.MoleculeDTO     `json:"base_molecule"`
	TargetPatent PatentDTO               `json:"target_patent"`
	Suggestions  []DesignAroundSuggestion `json:"suggestions"`
	AnalyzedAt   common.Timestamp         `json:"analyzed_at"`
}

// DesignAroundSuggestion represents a suggested molecule for design-around.
type DesignAroundSuggestion struct {
	Molecule           molecule.MoleculeDTO       `json:"molecule"`
	Modifications      []string                   `json:"modifications"`
	InfringementRisk   InfringementRiskDTO        `json:"infringement_risk"`
	PredictedProperties []molecule.MaterialProperty `json:"predicted_properties"`
	Confidence         float64                    `json:"confidence"`
	Rationale          string                     `json:"rationale"`
}

// Constructors

func NewPatentSearchRequest(query string) PatentSearchRequest {
	return PatentSearchRequest{
		Query: query,
		Pagination: common.Pagination{
			Page:     1,
			PageSize: 20,
		},
	}
}

func NewFTOAnalysisRequest(molecules []molecule.MoleculeInput, jurisdictions []PatentOffice) FTOAnalysisRequest {
	return FTOAnalysisRequest{
		TargetMolecules: molecules,
		Jurisdictions:   jurisdictions,
	}
}

func NewWatchlistConfig(name string, watchType string) WatchlistConfig {
	return WatchlistConfig{
		Name:      name,
		WatchType: watchType,
		Status:    "active",
		Schedule: ScheduleConfig{
			Frequency: "daily",
			TimeOfDay: "00:00",
			Timezone:  "UTC",
		},
		AlertConfig: AlertConfig{
			Channels:     []string{"email"},
			MinRiskLevel: RiskMedium,
		},
	}
}

// --- Legacy Types ---

// PatentID is a string alias for a patent identifier.
type PatentID string

// PatentSearchResponse is a generic wrapper for patent search results.
type PatentSearchResponse = common.PageResponse[PatentDTO]

// JurisdictionCode is an alias for PatentOffice for backward compatibility.
type JurisdictionCode = PatentOffice

const (
	JurisdictionCN = OfficeCNIPA
	JurisdictionUS = OfficeUSPTO
	JurisdictionEP = OfficeEPO
	JurisdictionJP = OfficeJPO
	JurisdictionKR = OfficeKIPO
	JurisdictionWO = OfficeWIPO
	// OfficeOther is not in prompt but was in legacy, mapping it if needed.
)

const (
	StatusPublished PatentStatus = "published" // Legacy status
	StatusFiled     PatentStatus = "pending"   // Alias
)

// ClaimElementDTO is an alias for ClaimElement for backward compatibility.
type ClaimElementDTO = ClaimElement

// PriorityDTO is an alias for PriorityInfo.
type PriorityDTO = PriorityInfo

// MarkushDTO represents a Markush structure.
type MarkushDTO struct {
	ID              common.ID   `json:"id"`
	PatentID        common.ID   `json:"patent_id"`
	ClaimID         common.ID   `json:"claim_id"`
	CoreStructure   string      `json:"core_structure"`
	RGroups         []RGroupDTO `json:"r_groups,omitempty"`
	Description     string      `json:"description,omitempty"`
	EnumeratedCount int64       `json:"enumerated_count"`
}

// RGroupDTO is an alias for MarkushGroupDTO or similar?
type RGroupDTO struct {
	Position     string   `json:"position"`
	Alternatives []string `json:"alternatives,omitempty"`
	Description  string   `json:"description,omitempty"`
}

// ValuationItem represents a single patent valuation item in assessment.
type ValuationItem struct {
	PatentID        string             `json:"patent_id"`
	PatentNumber    string             `json:"patent_number"`
	Title           string             `json:"title"`
	OverallScore    float64            `json:"overall_score"`
	TechnicalScore  float64            `json:"technical_score"`
	LegalScore      float64            `json:"legal_score"`
	CommercialScore float64            `json:"commercial_score"`
	StrategicScore  float64            `json:"strategic_score"`
	RiskLevel       string             `json:"risk_level"`
	Factors         map[string]float64 `json:"factors,omitempty"`
}

// HighRiskPatent represents a patent identified as high risk in portfolio assessment.
type HighRiskPatent struct {
	PatentID     string  `json:"patent_id,omitempty"`
	PatentNumber string  `json:"patent_number"`
	RiskReason   string  `json:"risk_reason"`
	RiskLevel    string  `json:"risk_level,omitempty"`
	RiskScore    float64 `json:"risk_score,omitempty"`
}

//Personal.AI order the ending
