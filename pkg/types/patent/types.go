package patent

import (
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// PatentStatus represents the legal status of a patent.
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

// IsValid checks if the patent status is valid.
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
	switch s {
	case StatusPending, StatusUnderExamination, StatusGranted:
		return true
	default:
		return false
	}
}

// PatentOffice represents a patent office.
type PatentOffice string

const (
	OfficeCNIPA PatentOffice = "CNIPA"
	OfficeUSPTO PatentOffice = "USPTO"
	OfficeEPO   PatentOffice = "EPO"
	OfficeJPO   PatentOffice = "JPO"
	OfficeKIPO  PatentOffice = "KIPO"
	OfficeWIPO  PatentOffice = "WIPO"
)

// IsValid checks if the patent office is valid.
func (o PatentOffice) IsValid() bool {
	switch o {
	case OfficeCNIPA, OfficeUSPTO, OfficeEPO, OfficeJPO, OfficeKIPO, OfficeWIPO:
		return true
	default:
		return false
	}
}

// ClaimType defines the type of a claim.
type ClaimType string

const (
	ClaimIndependent ClaimType = "independent"
	ClaimDependent   ClaimType = "dependent"
)

// ClaimCategory defines the category of a claim.
type ClaimCategory string

const (
	ClaimProduct     ClaimCategory = "product"
	ClaimMethod      ClaimCategory = "method"
	ClaimUse         ClaimCategory = "use"
	ClaimComposition ClaimCategory = "composition"
)

// InfringementRiskLevel defines the level of infringement risk.
type InfringementRiskLevel string

const (
	RiskCritical InfringementRiskLevel = "critical"
	RiskHigh     InfringementRiskLevel = "high"
	RiskMedium   InfringementRiskLevel = "medium"
	RiskLow      InfringementRiskLevel = "low"
	RiskNone     InfringementRiskLevel = "none"
)

// IsValid checks if the risk level is valid.
func (l InfringementRiskLevel) IsValid() bool {
	switch l {
	case RiskCritical, RiskHigh, RiskMedium, RiskLow, RiskNone:
		return true
	default:
		return false
	}
}

// Severity returns a numerical representation of the risk level.
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

// PatentDTO represents a patent data transfer object.
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
}

// PriorityInfo represents patent priority information.
type PriorityInfo struct {
	PatentNumber string           `json:"patent_number"`
	FilingDate   common.Timestamp `json:"filing_date"`
	Office       PatentOffice     `json:"office"`
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
}

// ClaimElement represents a decomposed element of a claim.
type ClaimElement struct {
	Index    int      `json:"index"`
	Text     string   `json:"text"`
	Type     string   `json:"type"`
	Keywords []string `json:"keywords"`
}

// MarkushGroupDTO represents a Markush group structure.
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

// FTOAnalysisRequest represents a request for FTO analysis.
type FTOAnalysisRequest struct {
	TargetMolecules     []molecule.MoleculeInput `json:"target_molecules"`
	Jurisdictions       []PatentOffice           `json:"jurisdictions"`
	IncludeExpired      bool                     `json:"include_expired"`
	IncludeEquivalents  bool                     `json:"include_equivalents"`
	IncludeDesignAround bool                     `json:"include_design_around"`
}

// Validate checks if the FTO analysis request is valid.
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
	for _, j := range r.Jurisdictions {
		if !j.IsValid() {
			return fmt.Errorf("invalid jurisdiction: %s", j)
		}
	}
	return nil
}

// FTOAnalysisResponse represents the response of an FTO analysis.
type FTOAnalysisResponse struct {
	RequestID       common.ID               `json:"request_id"`
	TargetMolecules []molecule.MoleculeDTO  `json:"target_molecules"`
	RiskPatents     []FTORiskPatent         `json:"risk_patents"`
	OverallRisk     InfringementRiskLevel   `json:"overall_risk"`
	Summary         string                  `json:"summary"`
	AnalyzedAt      common.Timestamp        `json:"analyzed_at"`
}

// FTORiskPatent represents a patent posing a risk in FTO analysis.
type FTORiskPatent struct {
	Patent                  PatentDTO                 `json:"patent"`
	Risk                    InfringementRiskDTO       `json:"risk"`
	MatchedMolecules        []molecule.SimilarityResult `json:"matched_molecules"`
	DesignAroundSuggestions []molecule.MoleculeDTO    `json:"design_around_suggestions,omitempty"`
}

// PatentValueDTO represents the valuation of a patent.
type PatentValueDTO struct {
	PatentID            common.ID              `json:"patent_id"`
	PatentNumber        string                 `json:"patent_number"`
	TechnicalValue      ScoreDimension         `json:"technical_value"`
	LegalValue          ScoreDimension         `json:"legal_value"`
	CommercialValue     ScoreDimension         `json:"commercial_value"`
	StrategicValue      ScoreDimension         `json:"strategic_value"`
	OverallScore        float64                `json:"overall_score"`
	Tier                string                 `json:"tier"`
	TierDescription     string                 `json:"tier_description"`
	WeightedCalculation string                 `json:"weighted_calculation"`
	Recommendations     []PatentRecommendation `json:"recommendations"`
	AssessedAt          common.Timestamp       `json:"assessed_at"`
}

// ScoreDimension represents a dimension of the patent score.
type ScoreDimension struct {
	Score       float64            `json:"score"`
	MaxScore    float64            `json:"max_score"`
	Factors     map[string]float64 `json:"factors"`
	Explanation string             `json:"explanation"`
}

// PatentRecommendation represents a recommendation based on patent valuation.
type PatentRecommendation struct {
	Type     string `json:"type"`
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// PatentSearchRequest represents a request for patent search.
type PatentSearchRequest struct {
	Query       string             `json:"query"`
	Offices     []PatentOffice     `json:"offices,omitempty"`
	Assignees   []string           `json:"assignees,omitempty"`
	Inventors   []string           `json:"inventors,omitempty"`
	IPCCodes    []string           `json:"ipc_codes,omitempty"`
	DateRange   *common.DateRange  `json:"date_range,omitempty"`
	Status      []PatentStatus     `json:"status,omitempty"`
	HasMolecule *bool              `json:"has_molecule,omitempty"`
	Pagination  common.Pagination  `json:"pagination"`
	Sort        []common.SortField `json:"sort,omitempty"`
}

// Validate checks if the patent search request is valid.
func (r PatentSearchRequest) Validate() error {
	if r.Query == "" && len(r.Offices) == 0 && len(r.Assignees) == 0 && len(r.Inventors) == 0 && len(r.IPCCodes) == 0 && r.DateRange == nil && len(r.Status) == 0 && r.HasMolecule == nil {
		return fmt.Errorf("at least one search criterion (query or filter) must be provided")
	}
	if r.DateRange != nil {
		if err := r.DateRange.Validate(); err != nil {
			return err
		}
	}
	if err := r.Pagination.Validate(); err != nil {
		return err
	}
	return nil
}

// CompetitorActivity represents activity of a competitor.
type CompetitorActivity struct {
	CompetitorName string            `json:"competitor_name"`
	Office         PatentOffice      `json:"office"`
	RecentFilings  int               `json:"recent_filings"`
	TechDomains    []string          `json:"tech_domains"`
	KeyInventors   []string          `json:"key_inventors"`
	TrendDirection string            `json:"trend_direction"`
	Period         common.DateRange  `json:"period"`
}

// WatchlistConfig represents configuration for a watchlist.
type WatchlistConfig struct {
	ID           common.ID          `json:"id"`
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	WatchType    string             `json:"watch_type"`
	Targets      []WatchTarget      `json:"targets"`
	Scope        WatchScope         `json:"scope"`
	AlertConfig  AlertConfig        `json:"alert_config"`
	Schedule     ScheduleConfig     `json:"schedule"`
	AutoAnalysis AutoAnalysisConfig `json:"auto_analysis"`
	Status       string             `json:"status"`
	CreatedAt    common.Timestamp   `json:"created_at"`
	UpdatedAt    common.Timestamp   `json:"updated_at"`
}

// Validate checks if the watchlist config is valid.
func (c WatchlistConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if c.WatchType != "molecule_similarity" && c.WatchType != "keyword" && c.WatchType != "assignee" && c.WatchType != "inventor" {
		return fmt.Errorf("invalid watch type: %s", c.WatchType)
	}
	if len(c.Targets) == 0 {
		return fmt.Errorf("at least one target is required")
	}
	if err := c.Schedule.Validate(); err != nil {
		return err
	}
	return nil
}

// WatchTarget represents a target in a watchlist.
type WatchTarget struct {
	Type      string                 `json:"type"`
	Value     string                 `json:"value"`
	Threshold float64                `json:"threshold,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// WatchScope represents the scope of a watchlist.
type WatchScope struct {
	Offices          []PatentOffice    `json:"offices,omitempty"`
	IPCCodes         []string          `json:"ipc_codes,omitempty"`
	Assignees        []string          `json:"assignees,omitempty"`
	ExcludeAssignees []string          `json:"exclude_assignees,omitempty"`
	DateRange        *common.DateRange `json:"date_range,omitempty"`
}

// AlertConfig represents configuration for alerts.
type AlertConfig struct {
	Channels           []string              `json:"channels"`
	MinRiskLevel       InfringementRiskLevel `json:"min_risk_level"`
	Recipients         []string              `json:"recipients"`
	WebhookURL         string                `json:"webhook_url,omitempty"`
	IncludeFullAnalysis bool                  `json:"include_full_analysis"`
	Cooldown           string                `json:"cooldown"`
}

// ScheduleConfig represents configuration for scheduling.
type ScheduleConfig struct {
	Frequency  string `json:"frequency"`
	DayOfWeek  *int   `json:"day_of_week,omitempty"`
	DayOfMonth *int   `json:"day_of_month,omitempty"`
	TimeOfDay  string `json:"time_of_day"`
	Timezone   string `json:"timezone"`
}

// Validate checks if the schedule config is valid.
func (s ScheduleConfig) Validate() error {
	switch s.Frequency {
	case "daily", "weekly", "biweekly", "monthly", "realtime":
		// valid
	default:
		return fmt.Errorf("invalid frequency: %s", s.Frequency)
	}
	if _, err := time.Parse("15:04", s.TimeOfDay); err != nil && s.Frequency != "realtime" {
		return fmt.Errorf("invalid time of day format (HH:MM): %s", s.TimeOfDay)
	}
	return nil
}

// AutoAnalysisConfig represents configuration for automatic analysis.
type AutoAnalysisConfig struct {
	EnableSimilaritySearch bool                       `json:"enable_similarity_search"`
	EnableInfringementRisk bool                       `json:"enable_infringement_risk"`
	EnableClaimAnalysis    bool                       `json:"enable_claim_analysis"`
	EnableDesignAround     bool                       `json:"enable_design_around"`
	EnablePatentValue      bool                       `json:"enable_patent_value"`
	SimilarityThreshold    float64                    `json:"similarity_threshold"`
	FingerprintTypes       []molecule.FingerprintType `json:"fingerprint_types"`
}

// WatchAlert represents an alert generated by a watchlist.
type WatchAlert struct {
	ID              common.ID                   `json:"id"`
	WatchlistID     common.ID                   `json:"watchlist_id"`
	WatchlistName   string                      `json:"watchlist_name"`
	AlertType       string                      `json:"alert_type"`
	Severity        string                      `json:"severity"`
	Title           string                      `json:"title"`
	Summary         string                      `json:"summary"`
	Patent          *PatentDTO                  `json:"patent,omitempty"`
	SimilarityResult *molecule.SimilarityResult  `json:"similarity_result,omitempty"`
	InfringementRisk *InfringementRiskDTO        `json:"infringement_risk,omitempty"`
	IsRead          bool                        `json:"is_read"`
	IsAcknowledged  bool                        `json:"is_acknowledged"`
	CreatedAt       common.Timestamp            `json:"created_at"`
}

// PatentLandscapeRequest represents a request for patent landscape analysis.
type PatentLandscapeRequest struct {
	TechDomain              string            `json:"tech_domain"`
	IPCCodes                []string          `json:"ipc_codes,omitempty"`
	Offices                 []PatentOffice    `json:"offices,omitempty"`
	DateRange               common.DateRange  `json:"date_range"`
	TopAssignees            int               `json:"top_assignees"`
	IncludeTrends           bool              `json:"include_trends"`
	IncludeWhiteSpace       bool              `json:"include_white_space"`
	IncludeCompetitorAnalysis bool              `json:"include_competitor_analysis"`
	Pagination              common.Pagination `json:"pagination"`
}

// Validate checks if the patent landscape request is valid.
func (r PatentLandscapeRequest) Validate() error {
	if r.TechDomain == "" && len(r.IPCCodes) == 0 {
		return fmt.Errorf("tech_domain or ipc_codes must be provided")
	}
	if err := r.DateRange.Validate(); err != nil {
		return err
	}
	if err := r.Pagination.Validate(); err != nil {
		return err
	}
	return nil
}

// PatentLandscapeResponse represents the response of a patent landscape analysis.
type PatentLandscapeResponse struct {
	TechDomain          string               `json:"tech_domain"`
	TotalPatents        int64                `json:"total_patents"`
	ActivePatents       int64                `json:"active_patents"`
	TopAssignees        []AssigneeStats      `json:"top_assignees"`
	YearlyTrends        []YearlyTrend        `json:"yearly_trends"`
	TechDistribution    []TechDistributionItem `json:"tech_distribution"`
	WhiteSpaceAreas     []WhiteSpaceArea     `json:"white_space_areas,omitempty"`
	CompetitorActivities []CompetitorActivity `json:"competitor_activities,omitempty"`
	GeneratedAt         common.Timestamp     `json:"generated_at"`
}

// AssigneeStats represents statistics for an assignee.
type AssigneeStats struct {
	Name               string   `json:"name"`
	TotalPatents       int      `json:"total_patents"`
	ActivePatents      int      `json:"active_patents"`
	RecentFilings      int      `json:"recent_filings"`
	TopIPCCodes        []string `json:"top_ipc_codes"`
	AveragePatentValue float64  `json:"average_patent_value"`
}

// YearlyTrend represents patent trends over a year.
type YearlyTrend struct {
	Year             int      `json:"year"`
	FilingCount      int      `json:"filing_count"`
	GrantCount       int      `json:"grant_count"`
	TopAssignees     []string `json:"top_assignees"`
	EmergingKeywords []string `json:"emerging_keywords"`
}

// TechDistributionItem represents distribution of technologies.
type TechDistributionItem struct {
	IPCCode     string  `json:"ipc_code"`
	Description string  `json:"description"`
	Count       int     `json:"count"`
	Percentage  float64 `json:"percentage"`
}

// WhiteSpaceArea represents an identified white space area.
type WhiteSpaceArea struct {
	Description         string                 `json:"description"`
	RelatedIPCCodes     []string               `json:"related_ipc_codes"`
	OpportunityScore    float64                `json:"opportunity_score"`
	Rationale           string                 `json:"rationale"`
	SuggestedMolecules  []molecule.MoleculeDTO `json:"suggested_molecules,omitempty"`
}

// PatentabilityRequest represents a request for patentability analysis.
type PatentabilityRequest struct {
	Molecule                   molecule.MoleculeInput `json:"molecule"`
	TechDomain                 string                 `json:"tech_domain"`
	Offices                    []PatentOffice         `json:"offices"`
	IncludeNoveltyAnalysis     bool                   `json:"include_novelty_analysis"`
	IncludeInventiveStepAnalysis bool                   `json:"include_inventive_step_analysis"`
	IncludePriorArtSearch      bool                   `json:"include_prior_art_search"`
}

// Validate checks if the patentability request is valid.
func (r PatentabilityRequest) Validate() error {
	if err := r.Molecule.Validate(); err != nil {
		return err
	}
	if len(r.Offices) == 0 {
		return fmt.Errorf("at least one office is required")
	}
	return nil
}

// PatentabilityResponse represents the response of a patentability analysis.
type PatentabilityResponse struct {
	Molecule                   molecule.MoleculeDTO `json:"molecule"`
	OverallScore               float64              `json:"overall_score"`
	NoveltyScore               float64              `json:"novelty_score"`
	InventiveStepScore         float64              `json:"inventive_step_score"`
	IndustrialApplicabilityScore float64              `json:"industrial_applicability_score"`
	PriorArt                   []PatentDTO          `json:"prior_art"`
	NoveltyAnalysis            string               `json:"novelty_analysis"`
	InventiveStepAnalysis      string               `json:"inventive_step_analysis"`
	Recommendations            []string             `json:"recommendations"`
	AnalyzedAt                 common.Timestamp     `json:"analyzed_at"`
}

// DesignAroundRequest represents a request for design-around analysis.
type DesignAroundRequest struct {
	TargetPatent   PatentDTO               `json:"target_patent"`
	TargetClaims   []int                   `json:"target_claims"`
	BaseMolecule   molecule.MoleculeInput  `json:"base_molecule"`
	Constraints    DesignAroundConstraints `json:"constraints"`
}

// Validate checks if the design around request is valid.
func (r DesignAroundRequest) Validate() error {
	if len(r.TargetClaims) == 0 {
		return fmt.Errorf("at least one target claim is required")
	}
	if err := r.BaseMolecule.Validate(); err != nil {
		return err
	}
	return nil
}

// DesignAroundConstraints represents constraints for design-around analysis.
type DesignAroundConstraints struct {
	MaintainActivity      bool                        `json:"maintain_activity"`
	MaxStructuralChanges  int                         `json:"max_structural_changes"`
	PreserveScaffold      bool                        `json:"preserve_scaffold"`
	ExcludedSubstructures []string                    `json:"excluded_substructures"`
	TargetProperties      []molecule.MaterialProperty `json:"target_properties"`
}

// DesignAroundResponse represents the response of a design-around analysis.
type DesignAroundResponse struct {
	BaseMolecule molecule.MoleculeDTO     `json:"base_molecule"`
	TargetPatent PatentDTO                `json:"target_patent"`
	Suggestions  []DesignAroundSuggestion `json:"suggestions"`
	AnalyzedAt   common.Timestamp         `json:"analyzed_at"`
}

// DesignAroundSuggestion represents a suggestion for design-around.
type DesignAroundSuggestion struct {
	Molecule            molecule.MoleculeDTO        `json:"molecule"`
	Modifications       []string                    `json:"modifications"`
	InfringementRisk    InfringementRiskDTO         `json:"infringement_risk"`
	PredictedProperties []molecule.MaterialProperty `json:"predicted_properties"`
	Confidence          float64                     `json:"confidence"`
	Rationale           string                      `json:"rationale"`
}

// NewPatentSearchRequest creates a new PatentSearchRequest with default pagination.
func NewPatentSearchRequest(query string) PatentSearchRequest {
	return PatentSearchRequest{
		Query: query,
		Pagination: common.Pagination{
			Page:     1,
			PageSize: 20,
		},
	}
}

// NewFTOAnalysisRequest creates a new FTOAnalysisRequest with default settings.
func NewFTOAnalysisRequest(molecules []molecule.MoleculeInput, jurisdictions []PatentOffice) FTOAnalysisRequest {
	return FTOAnalysisRequest{
		TargetMolecules:    molecules,
		Jurisdictions:      jurisdictions,
		IncludeExpired:     false,
		IncludeEquivalents: true,
	}
}

// NewWatchlistConfig creates a new WatchlistConfig with default schedule and alert config.
func NewWatchlistConfig(name string, watchType string) WatchlistConfig {
	return WatchlistConfig{
		Name:      name,
		WatchType: watchType,
		Schedule: ScheduleConfig{
			Frequency: "weekly",
			TimeOfDay: "00:00",
			Timezone:  "UTC",
		},
		AlertConfig: AlertConfig{
			Channels:     []string{"email"},
			MinRiskLevel: RiskHigh,
		},
		Status: "active",
	}
}

//Personal.AI order the ending
