package patent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

func TestPatentStatus_IsValid_AllStatuses(t *testing.T) {
	statuses := []PatentStatus{
		StatusPending, StatusUnderExamination, StatusGranted, StatusExpired, StatusAbandoned, StatusRevoked, StatusLapsed,
	}
	for _, s := range statuses {
		assert.True(t, s.IsValid())
	}
}

func TestPatentStatus_IsValid_Unknown(t *testing.T) {
	assert.False(t, PatentStatus("unknown").IsValid())
}

func TestPatentStatus_IsActive_ActiveStatuses(t *testing.T) {
	statuses := []PatentStatus{
		StatusPending, StatusUnderExamination, StatusGranted,
	}
	for _, s := range statuses {
		assert.True(t, s.IsActive())
	}
}

func TestPatentStatus_IsActive_InactiveStatuses(t *testing.T) {
	statuses := []PatentStatus{
		StatusExpired, StatusAbandoned, StatusRevoked, StatusLapsed,
	}
	for _, s := range statuses {
		assert.False(t, s.IsActive())
	}
}

func TestPatentOffice_IsValid_AllOffices(t *testing.T) {
	offices := []PatentOffice{
		OfficeCNIPA, OfficeUSPTO, OfficeEPO, OfficeJPO, OfficeKIPO, OfficeWIPO,
	}
	for _, o := range offices {
		assert.True(t, o.IsValid())
	}
}

func TestPatentOffice_IsValid_Unknown(t *testing.T) {
	assert.False(t, PatentOffice("UNKNOWN").IsValid())
}

func TestInfringementRiskLevel_IsValid_AllLevels(t *testing.T) {
	levels := []InfringementRiskLevel{
		RiskCritical, RiskHigh, RiskMedium, RiskLow, RiskNone,
	}
	for _, l := range levels {
		assert.True(t, l.IsValid())
	}
}

func TestInfringementRiskLevel_Severity_Ordering(t *testing.T) {
	assert.Greater(t, RiskCritical.Severity(), RiskHigh.Severity())
	assert.Greater(t, RiskHigh.Severity(), RiskMedium.Severity())
	assert.Greater(t, RiskMedium.Severity(), RiskLow.Severity())
	assert.Greater(t, RiskLow.Severity(), RiskNone.Severity())
}

func TestInfringementRiskLevel_Severity_Values(t *testing.T) {
	assert.Equal(t, 4, RiskCritical.Severity())
	assert.Equal(t, 3, RiskHigh.Severity())
	assert.Equal(t, 2, RiskMedium.Severity())
	assert.Equal(t, 1, RiskLow.Severity())
	assert.Equal(t, 0, RiskNone.Severity())
}

func TestFTOAnalysisRequest_Validate_Valid(t *testing.T) {
	req := FTOAnalysisRequest{
		TargetMolecules: []molecule.MoleculeInput{{Format: molecule.FormatSMILES, Value: "C"}},
		Jurisdictions:   []PatentOffice{OfficeUSPTO},
	}
	assert.NoError(t, req.Validate())
}

func TestFTOAnalysisRequest_Validate_NoMolecules(t *testing.T) {
	req := FTOAnalysisRequest{
		Jurisdictions: []PatentOffice{OfficeUSPTO},
	}
	assert.Error(t, req.Validate())
}

func TestFTOAnalysisRequest_Validate_NoJurisdictions(t *testing.T) {
	req := FTOAnalysisRequest{
		TargetMolecules: []molecule.MoleculeInput{{Format: molecule.FormatSMILES, Value: "C"}},
	}
	assert.Error(t, req.Validate())
}

func TestFTOAnalysisRequest_Validate_InvalidMolecule(t *testing.T) {
	req := FTOAnalysisRequest{
		TargetMolecules: []molecule.MoleculeInput{{Format: molecule.FormatSMILES, Value: ""}},
		Jurisdictions:   []PatentOffice{OfficeUSPTO},
	}
	assert.Error(t, req.Validate())
}

func TestPatentSearchRequest_Validate_WithQuery(t *testing.T) {
	req := PatentSearchRequest{
		Query:      "OLED",
		Pagination: common.Pagination{Page: 1, PageSize: 20},
	}
	assert.NoError(t, req.Validate())
}

func TestPatentSearchRequest_Validate_WithFilters(t *testing.T) {
	req := PatentSearchRequest{
		Assignees:  []string{"Company A"},
		Pagination: common.Pagination{Page: 1, PageSize: 20},
	}
	assert.NoError(t, req.Validate())
}

func TestPatentSearchRequest_Validate_Empty(t *testing.T) {
	req := PatentSearchRequest{
		Pagination: common.Pagination{Page: 1, PageSize: 20},
	}
	assert.Error(t, req.Validate())
}

func TestPatentLandscapeRequest_Validate_WithTechDomain(t *testing.T) {
	req := PatentLandscapeRequest{
		TechDomain: "OLED",
		Pagination: common.Pagination{Page: 1, PageSize: 20},
	}
	assert.NoError(t, req.Validate())
}

func TestPatentLandscapeRequest_Validate_WithIPCCodes(t *testing.T) {
	req := PatentLandscapeRequest{
		IPCCodes:   []string{"H01L"},
		Pagination: common.Pagination{Page: 1, PageSize: 20},
	}
	assert.NoError(t, req.Validate())
}

func TestPatentLandscapeRequest_Validate_Empty(t *testing.T) {
	req := PatentLandscapeRequest{
		Pagination: common.Pagination{Page: 1, PageSize: 20},
	}
	assert.Error(t, req.Validate())
}

func TestPatentabilityRequest_Validate_Valid(t *testing.T) {
	req := PatentabilityRequest{
		Molecule: molecule.MoleculeInput{Format: molecule.FormatSMILES, Value: "C"},
		Offices:  []PatentOffice{OfficeUSPTO},
	}
	assert.NoError(t, req.Validate())
}

func TestPatentabilityRequest_Validate_InvalidMolecule(t *testing.T) {
	req := PatentabilityRequest{
		Molecule: molecule.MoleculeInput{Format: molecule.FormatSMILES, Value: ""},
		Offices:  []PatentOffice{OfficeUSPTO},
	}
	assert.Error(t, req.Validate())
}

func TestPatentabilityRequest_Validate_NoOffices(t *testing.T) {
	req := PatentabilityRequest{
		Molecule: molecule.MoleculeInput{Format: molecule.FormatSMILES, Value: "C"},
	}
	assert.Error(t, req.Validate())
}

func TestDesignAroundRequest_Validate_Valid(t *testing.T) {
	req := DesignAroundRequest{
		TargetClaims: []int{1},
		BaseMolecule: molecule.MoleculeInput{Format: molecule.FormatSMILES, Value: "C"},
	}
	assert.NoError(t, req.Validate())
}

func TestDesignAroundRequest_Validate_NoClaims(t *testing.T) {
	req := DesignAroundRequest{
		BaseMolecule: molecule.MoleculeInput{Format: molecule.FormatSMILES, Value: "C"},
	}
	assert.Error(t, req.Validate())
}

func TestDesignAroundRequest_Validate_InvalidBaseMolecule(t *testing.T) {
	req := DesignAroundRequest{
		TargetClaims: []int{1},
		BaseMolecule: molecule.MoleculeInput{Format: molecule.FormatSMILES, Value: ""},
	}
	assert.Error(t, req.Validate())
}

func TestPatentDTO_JSONRoundTrip(t *testing.T) {
	dto := PatentDTO{
		PatentNumber: "US123456",
		Status:       StatusGranted,
	}
	bytes, err := json.Marshal(dto)
	assert.NoError(t, err)
	var decoded PatentDTO
	err = json.Unmarshal(bytes, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, dto.PatentNumber, decoded.PatentNumber)
}

func TestClaimDTO_JSONSerialization(t *testing.T) {
	dto := ClaimDTO{
		ClaimNumber: 1,
		MarkushGroups: []MarkushGroupDTO{
			{Position: "R1"},
		},
	}
	bytes, err := json.Marshal(dto)
	assert.NoError(t, err)
	assert.Contains(t, string(bytes), "markush_groups")
}

func TestWatchlistConfig_Validate_Valid(t *testing.T) {
	cfg := WatchlistConfig{
		Name:      "My List",
		WatchType: "molecule_similarity",
		Targets:   []WatchTarget{{Type: "molecule", Value: "C"}},
		Schedule:  ScheduleConfig{Frequency: "daily", TimeOfDay: "09:00"},
	}
	assert.NoError(t, cfg.Validate())
}

func TestWatchlistConfig_Validate_EmptyName(t *testing.T) {
	cfg := WatchlistConfig{
		Name:      "",
		WatchType: "molecule_similarity",
		Targets:   []WatchTarget{{Type: "molecule", Value: "C"}},
	}
	assert.Error(t, cfg.Validate())
}

func TestWatchlistConfig_Validate_InvalidWatchType(t *testing.T) {
	cfg := WatchlistConfig{
		Name:      "List",
		WatchType: "invalid",
		Targets:   []WatchTarget{{Type: "molecule", Value: "C"}},
	}
	assert.Error(t, cfg.Validate())
}

func TestWatchlistConfig_Validate_EmptyTargets(t *testing.T) {
	cfg := WatchlistConfig{
		Name:      "List",
		WatchType: "molecule_similarity",
	}
	assert.Error(t, cfg.Validate())
}

func TestScheduleConfig_Validate_Valid(t *testing.T) {
	cfg := ScheduleConfig{Frequency: "daily", TimeOfDay: "09:00"}
	assert.NoError(t, cfg.Validate())
}

func TestScheduleConfig_Validate_InvalidFrequency(t *testing.T) {
	cfg := ScheduleConfig{Frequency: "invalid", TimeOfDay: "09:00"}
	assert.Error(t, cfg.Validate())
}

func TestScheduleConfig_Validate_InvalidTimeOfDay(t *testing.T) {
	cfg := ScheduleConfig{Frequency: "daily", TimeOfDay: "25:00"}
	assert.Error(t, cfg.Validate())
}

func TestNewPatentSearchRequest_Defaults(t *testing.T) {
	req := NewPatentSearchRequest("query")
	assert.Equal(t, 1, req.Pagination.Page)
	assert.Equal(t, 20, req.Pagination.PageSize)
}

func TestNewFTOAnalysisRequest_Defaults(t *testing.T) {
	req := NewFTOAnalysisRequest(nil, nil)
	assert.False(t, req.IncludeExpired)
}

func TestNewWatchlistConfig_Defaults(t *testing.T) {
	cfg := NewWatchlistConfig("name", "type")
	assert.Equal(t, "weekly", cfg.Schedule.Frequency)
	assert.Equal(t, RiskHigh, cfg.AlertConfig.MinRiskLevel)
}

func TestClaimType_Values(t *testing.T) {
	assert.Equal(t, ClaimType("independent"), ClaimIndependent)
	assert.Equal(t, ClaimType("dependent"), ClaimDependent)
}

func TestClaimCategory_Values(t *testing.T) {
	assert.Equal(t, ClaimCategory("product"), ClaimProduct)
	assert.Equal(t, ClaimCategory("method"), ClaimMethod)
	assert.Equal(t, ClaimCategory("use"), ClaimUse)
	assert.Equal(t, ClaimCategory("composition"), ClaimComposition)
}

//Personal.AI order the ending
