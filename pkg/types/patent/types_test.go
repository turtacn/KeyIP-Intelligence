package patent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

func TestPatentStatus_IsValid_AllStatuses(t *testing.T) {
	statuses := []PatentStatus{StatusPending, StatusUnderExamination, StatusGranted, StatusExpired, StatusAbandoned, StatusRevoked, StatusLapsed}
	for _, s := range statuses {
		assert.True(t, s.IsValid())
	}
}

func TestPatentStatus_IsValid_Unknown(t *testing.T) {
	assert.False(t, PatentStatus("unknown").IsValid())
}

func TestPatentStatus_IsActive_ActiveStatuses(t *testing.T) {
	assert.True(t, StatusPending.IsActive())
	assert.True(t, StatusUnderExamination.IsActive())
	assert.True(t, StatusGranted.IsActive())
}

func TestPatentStatus_IsActive_InactiveStatuses(t *testing.T) {
	assert.False(t, StatusExpired.IsActive())
	assert.False(t, StatusAbandoned.IsActive())
	assert.False(t, StatusRevoked.IsActive())
	assert.False(t, StatusLapsed.IsActive())
}

func TestPatentOffice_IsValid_AllOffices(t *testing.T) {
	offices := []PatentOffice{OfficeCNIPA, OfficeUSPTO, OfficeEPO, OfficeJPO, OfficeKIPO, OfficeWIPO}
	for _, o := range offices {
		assert.True(t, o.IsValid())
	}
}

func TestPatentOffice_IsValid_Unknown(t *testing.T) {
	assert.False(t, PatentOffice("unknown").IsValid())
}

func TestInfringementRiskLevel_IsValid_AllLevels(t *testing.T) {
	levels := []InfringementRiskLevel{RiskCritical, RiskHigh, RiskMedium, RiskLow, RiskNone}
	for _, l := range levels {
		assert.True(t, l.IsValid())
	}
}

func TestInfringementRiskLevel_Severity_Ordering(t *testing.T) {
	assert.True(t, RiskCritical.Severity() > RiskHigh.Severity())
	assert.True(t, RiskHigh.Severity() > RiskMedium.Severity())
	assert.True(t, RiskMedium.Severity() > RiskLow.Severity())
	assert.True(t, RiskLow.Severity() > RiskNone.Severity())
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
		TargetMolecules: []molecule.MoleculeInput{{Format: molecule.FormatSMILES, Value: "CCO"}},
		Jurisdictions:   []PatentOffice{OfficeCNIPA},
	}
	assert.NoError(t, req.Validate())
}

func TestFTOAnalysisRequest_Validate_NoMolecules(t *testing.T) {
	req := FTOAnalysisRequest{
		TargetMolecules: []molecule.MoleculeInput{},
		Jurisdictions:   []PatentOffice{OfficeCNIPA},
	}
	assert.Error(t, req.Validate())
}

func TestFTOAnalysisRequest_Validate_NoJurisdictions(t *testing.T) {
	req := FTOAnalysisRequest{
		TargetMolecules: []molecule.MoleculeInput{{Format: molecule.FormatSMILES, Value: "CCO"}},
		Jurisdictions:   []PatentOffice{},
	}
	assert.Error(t, req.Validate())
}

func TestFTOAnalysisRequest_Validate_InvalidMolecule(t *testing.T) {
	req := FTOAnalysisRequest{
		TargetMolecules: []molecule.MoleculeInput{{Format: "invalid", Value: "CCO"}},
		Jurisdictions:   []PatentOffice{OfficeCNIPA},
	}
	assert.Error(t, req.Validate())
}

func TestPatentSearchRequest_Validate_WithQuery(t *testing.T) {
	req := NewPatentSearchRequest("oled")
	assert.NoError(t, req.Validate())
}

func TestPatentSearchRequest_Validate_WithFilters(t *testing.T) {
	req := PatentSearchRequest{
		Assignees:  []string{"Samsung"},
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
		TechDomain: "Blue OLED",
		DateRange:  common.DateRange{From: common.NewTimestamp(), To: common.NewTimestamp()},
	}
	assert.NoError(t, req.Validate())
}

func TestPatentLandscapeRequest_Validate_WithIPCCodes(t *testing.T) {
	req := PatentLandscapeRequest{
		IPCCodes:  []string{"C09K11/06"},
		DateRange: common.DateRange{From: common.NewTimestamp(), To: common.NewTimestamp()},
	}
	assert.NoError(t, req.Validate())
}

func TestPatentLandscapeRequest_Validate_Empty(t *testing.T) {
	req := PatentLandscapeRequest{
		DateRange: common.DateRange{From: common.NewTimestamp(), To: common.NewTimestamp()},
	}
	assert.Error(t, req.Validate())
}

func TestPatentabilityRequest_Validate_Valid(t *testing.T) {
	req := PatentabilityRequest{
		Molecule: molecule.MoleculeInput{Format: molecule.FormatSMILES, Value: "CCO"},
		Offices:  []PatentOffice{OfficeCNIPA},
	}
	assert.NoError(t, req.Validate())
}

func TestPatentabilityRequest_Validate_InvalidMolecule(t *testing.T) {
	req := PatentabilityRequest{
		Molecule: molecule.MoleculeInput{Format: "invalid", Value: "CCO"},
		Offices:  []PatentOffice{OfficeCNIPA},
	}
	assert.Error(t, req.Validate())
}

func TestPatentabilityRequest_Validate_NoOffices(t *testing.T) {
	req := PatentabilityRequest{
		Molecule: molecule.MoleculeInput{Format: molecule.FormatSMILES, Value: "CCO"},
		Offices:  []PatentOffice{},
	}
	assert.Error(t, req.Validate())
}

func TestDesignAroundRequest_Validate_Valid(t *testing.T) {
	req := DesignAroundRequest{
		TargetClaims: []int{1},
		BaseMolecule: molecule.MoleculeInput{Format: molecule.FormatSMILES, Value: "CCO"},
	}
	assert.NoError(t, req.Validate())
}

func TestDesignAroundRequest_Validate_NoClaims(t *testing.T) {
	req := DesignAroundRequest{
		TargetClaims: []int{},
		BaseMolecule: molecule.MoleculeInput{Format: molecule.FormatSMILES, Value: "CCO"},
	}
	assert.Error(t, req.Validate())
}

func TestDesignAroundRequest_Validate_InvalidBaseMolecule(t *testing.T) {
	req := DesignAroundRequest{
		TargetClaims: []int{1},
		BaseMolecule: molecule.MoleculeInput{Format: "invalid", Value: "CCO"},
	}
	assert.Error(t, req.Validate())
}

func TestPatentDTO_JSONRoundTrip(t *testing.T) {
	dto := PatentDTO{
		ID:           common.ID("550e8400-e29b-41d4-a716-446655440000"),
		CreatedAt:    common.NewTimestamp(),
		PatentNumber: "CN1234567A",
		Status:       StatusGranted,
		Office:       OfficeCNIPA,
	}
	data, err := json.Marshal(dto)
	assert.NoError(t, err)

	var dto2 PatentDTO
	err = json.Unmarshal(data, &dto2)
	assert.NoError(t, err)
	assert.Equal(t, dto.PatentNumber, dto2.PatentNumber)
	assert.Equal(t, dto.Office, dto2.Office)
	// Compare UnixMilli
	assert.Equal(t, time.Time(dto.CreatedAt).UnixMilli(), time.Time(dto2.CreatedAt).UnixMilli())
}

func TestClaimDTO_JSONSerialization(t *testing.T) {
	dto := ClaimDTO{
		ClaimNumber: 1,
		HasMarkush:  true,
		MarkushGroups: []MarkushGroupDTO{
			{Position: "R1", Options: []string{"C", "O"}},
		},
	}
	data, err := json.Marshal(dto)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "markush_groups")
}

func TestWatchlistConfig_Validate_Valid(t *testing.T) {
	config := NewWatchlistConfig("Test Watch", "keyword")
	config.Targets = []WatchTarget{{Type: "keyword", Value: "oled"}}
	// Schedule is initialized by NewWatchlistConfig
	assert.NoError(t, config.Validate())
}

func TestWatchlistConfig_Validate_EmptyName(t *testing.T) {
	config := NewWatchlistConfig("", "keyword")
	config.Targets = []WatchTarget{{Type: "keyword", Value: "oled"}}
	assert.Error(t, config.Validate())
}

func TestWatchlistConfig_Validate_InvalidWatchType(t *testing.T) {
	config := NewWatchlistConfig("Test", "")
	config.Targets = []WatchTarget{{Type: "keyword", Value: "oled"}}
	assert.Error(t, config.Validate())
}

func TestWatchlistConfig_Validate_EmptyTargets(t *testing.T) {
	config := NewWatchlistConfig("Test", "keyword")
	config.Targets = []WatchTarget{}
	assert.Error(t, config.Validate())
}

func TestScheduleConfig_Validate_Valid(t *testing.T) {
	s := ScheduleConfig{Frequency: "daily", TimeOfDay: "14:30"}
	assert.NoError(t, s.Validate())
}

func TestScheduleConfig_Validate_InvalidFrequency(t *testing.T) {
	s := ScheduleConfig{Frequency: "invalid", TimeOfDay: "14:30"}
	assert.Error(t, s.Validate())
}

func TestScheduleConfig_Validate_InvalidTimeOfDay(t *testing.T) {
	s := ScheduleConfig{Frequency: "daily", TimeOfDay: "25:00"}
	assert.Error(t, s.Validate())
}

func TestNewPatentSearchRequest_Defaults(t *testing.T) {
	req := NewPatentSearchRequest("test")
	assert.Equal(t, 1, req.Pagination.Page)
	assert.Equal(t, 20, req.Pagination.PageSize)
}

func TestNewFTOAnalysisRequest_Defaults(t *testing.T) {
	req := NewFTOAnalysisRequest(nil, nil)
	assert.False(t, req.IncludeExpired)
}

func TestNewWatchlistConfig_Defaults(t *testing.T) {
	config := NewWatchlistConfig("test", "type")
	assert.Equal(t, "daily", config.Schedule.Frequency)
	assert.Equal(t, "active", config.Status)
}

func TestClaimType_Values(t *testing.T) {
	assert.Equal(t, "independent", string(ClaimIndependent))
	assert.Equal(t, "dependent", string(ClaimDependent))
}

func TestClaimCategory_Values(t *testing.T) {
	assert.Equal(t, "product", string(ClaimProduct))
	assert.Equal(t, "method", string(ClaimMethod))
	assert.Equal(t, "use", string(ClaimUse))
	assert.Equal(t, "composition", string(ClaimComposition))
}
