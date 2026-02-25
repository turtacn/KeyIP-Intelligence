// File: internal/interfaces/grpc/services/patent_service_test.go
package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/reporting"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	pb "github.com/turtacn/KeyIP-Intelligence/api/proto/patent/v1"
)

// Mock patent repository
type mockPatentRepository struct {
	mock.Mock
}

func (m *mockPatentRepository) FindByNumber(ctx context.Context, number string) (*patent.Patent, error) {
	args := m.Called(ctx, number)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent.Patent), args.Error(1)
}

func (m *mockPatentRepository) Search(ctx context.Context, filter *patent.SearchFilter) ([]*patent.Patent, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*patent.Patent), args.Get(1).(int64), args.Error(2)
}

func (m *mockPatentRepository) GetFamily(ctx context.Context, patentNumber string) (*patent.PatentFamily, error) {
	args := m.Called(ctx, patentNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent.PatentFamily), args.Error(1)
}

func (m *mockPatentRepository) GetCitationNetwork(ctx context.Context, query *patent.CitationNetworkQuery) (*patent.CitationNetwork, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent.CitationNetwork), args.Error(1)
}

// Mock FTO service
type mockFTOReportService struct {
	mock.Mock
}

func (m *mockFTOReportService) QuickCheck(ctx context.Context, req *reporting.FTOQuickCheckRequest) (*reporting.FTOQuickCheckResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.FTOQuickCheckResult), args.Error(1)
}

func createTestPatent(number string) *patent.Patent {
	pat, _ := patent.NewPatent(
		number,
		"Test Patent Title",
		"This is a test abstract",
		[]string{"Applicant Corp"},
		[]string{"John Inventor"},
		[]string{"C07D"},
		time.Now(),
	)
	return pat
}

func TestGetPatent_Success(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	expectedPatent := createTestPatent("CN115123456A")
	mockRepo.On("FindByNumber", mock.Anything, "CN115123456A").Return(expectedPatent, nil)

	resp, err := service.GetPatent(context.Background(), &pb.GetPatentRequest{
		PatentNumber: "CN115123456A",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "CN115123456A", resp.Patent.PatentNumber)
	mockRepo.AssertExpectations(t)
}

func TestGetPatent_NotFound(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	mockRepo.On("FindByNumber", mock.Anything, "CN999999999A").Return(nil, errors.NewNotFoundError("patent not found"))
	mockLog.On("Error", mock.Anything, mock.Anything).Return()

	resp, err := service.GetPatent(context.Background(), &pb.GetPatentRequest{
		PatentNumber: "CN999999999A",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestGetPatent_InvalidNumber(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	resp, err := service.GetPatent(context.Background(), &pb.GetPatentRequest{
		PatentNumber: "INVALID",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "invalid patent number format")
}

func TestSearchPatents_BasicQuery(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	patents := []*patent.Patent{
		createTestPatent("CN115123456A"),
		createTestPatent("CN115123457A"),
	}

	mockRepo.On("Search", mock.Anything, mock.AnythingOfType("*patent.SearchFilter")).Return(patents, int64(50), nil)

	resp, err := service.SearchPatents(context.Background(), &pb.SearchPatentsRequest{
		Query:    "organic light emitting",
		PageSize: 20,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Patents, 2)
	assert.Equal(t, int64(50), resp.TotalCount)
	mockRepo.AssertExpectations(t)
}

func TestSearchPatents_WithFilters(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	patents := []*patent.Patent{createTestPatent("CN115123456A")}
	mockRepo.On("Search", mock.Anything, mock.AnythingOfType("*patent.SearchFilter")).Return(patents, int64(1), nil)

	resp, err := service.SearchPatents(context.Background(), &pb.SearchPatentsRequest{
		Query:         "OLED",
		IpcClasses:    []string{"C07D"},
		PatentOffices: []string{"CN"},
		PageSize:      10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Patents, 1)
}

func TestSearchPatents_Pagination(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	// First page
	patents := make([]*patent.Patent, 20)
	for i := 0; i < 20; i++ {
		patents[i] = createTestPatent("CN11512345" + string(rune('0'+i)) + "A")
	}

	mockRepo.On("Search", mock.Anything, mock.AnythingOfType("*patent.SearchFilter")).Return(patents, int64(100), nil)

	resp, err := service.SearchPatents(context.Background(), &pb.SearchPatentsRequest{
		Query:    "test",
		PageSize: 20,
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, resp.NextPageToken)
}

func TestSearchPatents_EmptyResults(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	mockRepo.On("Search", mock.Anything, mock.AnythingOfType("*patent.SearchFilter")).Return([]*patent.Patent{}, int64(0), nil)

	resp, err := service.SearchPatents(context.Background(), &pb.SearchPatentsRequest{
		Query: "nonexistent query xyz123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Patents)
	assert.Equal(t, int64(0), resp.TotalCount)
}

func TestAnalyzeClaims_Success(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	testPatent := createTestPatent("CN115123456A")
	mockRepo.On("FindByNumber", mock.Anything, "CN115123456A").Return(testPatent, nil)

	resp, err := service.AnalyzeClaims(context.Background(), &pb.AnalyzeClaimsRequest{
		PatentNumber: "CN115123456A",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "CN115123456A", resp.PatentNumber)
	assert.NotNil(t, resp.ClaimTree)
}

func TestAnalyzeClaims_ComplexClaimTree(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	testPatent := createTestPatent("CN115123456A")
	// Assume patent has complex claim structure
	mockRepo.On("FindByNumber", mock.Anything, "CN115123456A").Return(testPatent, nil)

	resp, err := service.AnalyzeClaims(context.Background(), &pb.AnalyzeClaimsRequest{
		PatentNumber: "CN115123456A",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Greater(t, resp.TotalClaims, int32(0))
}

func TestAnalyzeClaims_NotFound(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	mockRepo.On("FindByNumber", mock.Anything, "CN999999999A").Return(nil, errors.NewNotFoundError("patent not found"))
	mockLog.On("Error", mock.Anything, mock.Anything).Return()

	resp, err := service.AnalyzeClaims(context.Background(), &pb.AnalyzeClaimsRequest{
		PatentNumber: "CN999999999A",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestCheckFTO_NoRisk(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	ftoResult := &reporting.FTOQuickCheckResult{
		CanOperate:      true,
		RiskLevel:       "low",
		Confidence:      0.95,
		BlockingPatents: []*reporting.BlockingPatent{},
		Recommendation:  "Free to operate",
	}

	mockFTO.On("QuickCheck", mock.Anything, mock.AnythingOfType("*reporting.FTOQuickCheckRequest")).Return(ftoResult, nil)

	resp, err := service.CheckFTO(context.Background(), &pb.CheckFTORequest{
		TargetSmiles:  "c1ccccc1",
		Jurisdiction:  "CN",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.CanOperate)
	assert.Equal(t, "low", resp.RiskLevel)
	assert.Empty(t, resp.BlockingPatents)
}

func TestCheckFTO_HighRisk(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	ftoResult := &reporting.FTOQuickCheckResult{
		CanOperate: false,
		RiskLevel:  "high",
		Confidence: 0.88,
		BlockingPatents: []*reporting.BlockingPatent{
			{
				PatentNumber:  "CN115123456A",
				Title:         "Blocking Patent",
				RiskLevel:     "high",
				Similarity:    0.92,
				ExpiryDate:    time.Now().Add(5 * 365 * 24 * time.Hour),
				LegalStatus:   "granted",
				MatchedClaims: []string{"1", "3"},
			},
		},
		Recommendation: "High infringement risk detected",
	}

	mockFTO.On("QuickCheck", mock.Anything, mock.AnythingOfType("*reporting.FTOQuickCheckRequest")).Return(ftoResult, nil)

	resp, err := service.CheckFTO(context.Background(), &pb.CheckFTORequest{
		TargetSmiles: "c1ccc2ccccc2c1",
		Jurisdiction: "CN",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.CanOperate)
	assert.Equal(t, "high", resp.RiskLevel)
	assert.Len(t, resp.BlockingPatents, 1)
}

func TestCheckFTO_Timeout(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	mockFTO.On("QuickCheck", mock.Anything, mock.AnythingOfType("*reporting.FTOQuickCheckRequest")).Return(
		nil, context.DeadlineExceeded)
	mockLog.On("Error", mock.Anything, mock.Anything).Return()

	resp, err := service.CheckFTO(context.Background(), &pb.CheckFTORequest{
		TargetSmiles: "c1ccccc1",
		Jurisdiction: "CN",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	// Note: In actual implementation, the timeout context would trigger
	assert.Contains(t, err.Error(), "timeout")
}

func TestCheckFTO_InvalidSMILES(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	resp, err := service.CheckFTO(context.Background(), &pb.CheckFTORequest{
		TargetSmiles: "",
		Jurisdiction: "CN",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetPatentFamily_Success(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	family := &patent.PatentFamily{
		FamilyID: "FAM123",
		Members: []*patent.FamilyMember{
			{
				PatentNumber:      "CN115123456A",
				PatentOffice:      "CN",
				ApplicationDate:   time.Now(),
				PublicationDate:   time.Now(),
				LegalStatus:       "granted",
				IsRepresentative:  true,
			},
			{
				PatentNumber:      "US11234567B2",
				PatentOffice:      "US",
				ApplicationDate:   time.Now(),
				PublicationDate:   time.Now(),
				LegalStatus:       "granted",
				IsRepresentative:  false,
			},
		},
	}

	mockRepo.On("GetFamily", mock.Anything, "CN115123456A").Return(family, nil)

	resp, err := service.GetPatentFamily(context.Background(), &pb.GetPatentFamilyRequest{
		PatentNumber: "CN115123456A",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "FAM123", resp.FamilyId)
	assert.Len(t, resp.FamilyMembers, 2)
	assert.Equal(t, int32(2), resp.TotalMembers)
}

func TestGetPatentFamily_NoFamily(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	family := &patent.PatentFamily{
		FamilyID: "",
		Members:  []*patent.FamilyMember{},
	}

	mockRepo.On("GetFamily", mock.Anything, "CN115123456A").Return(family, nil)

	resp, err := service.GetPatentFamily(context.Background(), &pb.GetPatentFamilyRequest{
		PatentNumber: "CN115123456A",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.FamilyMembers)
}

func TestGetCitationNetwork_DefaultDepth(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	network := &patent.CitationNetwork{
		Nodes: []*patent.CitationNode{
			{PatentNumber: "CN115123456A", IsRoot: true, Level: 0},
			{PatentNumber: "CN115123457A", IsRoot: false, Level: 1},
		},
		Edges: []*patent.CitationEdge{
			{FromPatent: "CN115123456A", ToPatent: "CN115123457A", EdgeType: "cites"},
		},
		IsTruncated: false,
	}

	mockRepo.On("GetCitationNetwork", mock.Anything, mock.AnythingOfType("*patent.CitationNetworkQuery")).Return(network, nil)

	resp, err := service.GetCitationNetwork(context.Background(), &pb.GetCitationNetworkRequest{
		PatentNumber:  "CN115123456A",
		IncludeCiting: true,
		IncludeCited:  true,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Nodes, 2)
	assert.Len(t, resp.Edges, 1)
	assert.False(t, resp.IsTruncated)
}

func TestGetCitationNetwork_MaxDepth(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	network := &patent.CitationNetwork{
		Nodes:       []*patent.CitationNode{},
		Edges:       []*patent.CitationEdge{},
		IsTruncated: false,
	}

	mockRepo.On("GetCitationNetwork", mock.Anything, mock.AnythingOfType("*patent.CitationNetworkQuery")).Return(network, nil)

	resp, err := service.GetCitationNetwork(context.Background(), &pb.GetCitationNetworkRequest{
		PatentNumber: "CN115123456A",
		Depth:        5, // Max allowed
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGetCitationNetwork_ExceedDepthLimit(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	resp, err := service.GetCitationNetwork(context.Background(), &pb.GetCitationNetworkRequest{
		PatentNumber: "CN115123456A",
		Depth:        10, // Exceeds max
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetCitationNetwork_Truncated(t *testing.T) {
	mockRepo := new(mockPatentRepository)
	mockFTO := new(mockFTOReportService)
	mockLog := new(mockLogger)

	service := NewPatentServiceServer(mockRepo, mockFTO, mockLog)

	// Create large network that exceeds limit
	nodes := make([]*patent.CitationNode, 1001)
	for i := range nodes {
		nodes[i] = &patent.CitationNode{
			PatentNumber: fmt.Sprintf("CN11512%04dA", i),
			Level:        i / 100,
		}
	}

	network := &patent.CitationNetwork{
		Nodes:       nodes[:1000], // Truncated to max
		Edges:       []*patent.CitationEdge{},
		IsTruncated: true,
	}

	mockRepo.On("GetCitationNetwork", mock.Anything, mock.AnythingOfType("*patent.CitationNetworkQuery")).Return(network, nil)

	resp, err := service.GetCitationNetwork(context.Background(), &pb.GetCitationNetworkRequest{
		PatentNumber: "CN115123456A",
		Depth:        3,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.IsTruncated)
	assert.LessOrEqual(t, resp.TotalNodes, int32(maxNetworkNodes))
}

func TestPatentDomainToProto_FullConversion(t *testing.T) {
	domainPatent := createTestPatent("CN115123456A")

	protoPatent := patentDomainToProto(domainPatent)

	assert.Equal(t, "CN115123456A", protoPatent.PatentNumber)
	assert.Equal(t, "Test Patent Title", protoPatent.Title)
	assert.NotEmpty(t, protoPatent.Applicants)
	assert.NotEmpty(t, protoPatent.Inventors)
}

func TestClaimTreeToProto_NestedStructure(t *testing.T) {
	// Create nested claim tree
	claimTree := &patent.ClaimTree{
		IndependentClaims: []*patent.Claim{
			{
				ClaimNumber:   "1",
				ClaimText:     "Independent claim 1",
				IsIndependent: true,
				DependentClaims: []*patent.Claim{
					{
						ClaimNumber:   "2",
						ClaimText:     "Dependent claim 2",
						IsIndependent: false,
						DependsOn:     []string{"1"},
					},
				},
			},
		},
	}

	protoTree := claimTreeToProto(claimTree)

	assert.NotNil(t, protoTree)
	assert.Len(t, protoTree.IndependentClaims, 1)
	assert.Len(t, protoTree.IndependentClaims[0].DependentClaims, 1)
}

func TestMapPatentError_AllCodes(t *testing.T) {
	tests := []struct {
		name         string
		domainError  error
		expectedCode codes.Code
	}{
		{"NotFound", errors.NewNotFoundError("not found"), codes.NotFound},
		{"Validation", errors.NewValidationError("invalid"), codes.InvalidArgument},
		{"Conflict", errors.NewConflictError("conflict"), codes.AlreadyExists},
		{"Unauthorized", errors.NewUnauthorizedError("unauthorized"), codes.PermissionDenied},
		{"Internal", errors.New("unknown"), codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grpcErr := mapPatentError(tt.domainError)
			assert.Equal(t, tt.expectedCode, status.Code(grpcErr))
		})
	}
}

func TestIsValidPatentNumber(t *testing.T) {
	tests := []struct {
		number string
		valid  bool
	}{
		{"CN115123456A", true},
		{"US1234567B2", true},
		{"EP1234567A1", true},
		{"JP2021123456A", true},
		{"KR1020210001234B1", true},
		{"WO2021/123456A1", true},
		{"INVALID", false},
		{"CN123", false},
		{"US123456789012345", false},
	}

	for _, tt := range tests {
		t.Run(tt.number, func(t *testing.T) {
			result := isValidPatentNumber(tt.number)
			assert.Equal(t, tt.valid, result)
		})
	}
}

//Personal.AI order the ending
