// File: internal/interfaces/grpc/services/molecule_service_test.go
package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	pb "github.com/turtacn/KeyIP-Intelligence/api/proto/molecule/v1"
)

// Mock repository
type mockMoleculeRepository struct {
	mock.Mock
}

func (m *mockMoleculeRepository) FindByID(ctx context.Context, id string) (*molecule.Molecule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.Molecule), args.Error(1)
}

func (m *mockMoleculeRepository) FindBySMILES(ctx context.Context, smiles string) (*molecule.Molecule, error) {
	args := m.Called(ctx, smiles)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.Molecule), args.Error(1)
}

func (m *mockMoleculeRepository) Create(ctx context.Context, mol *molecule.Molecule) error {
	args := m.Called(ctx, mol)
	return args.Error(0)
}

func (m *mockMoleculeRepository) Update(ctx context.Context, mol *molecule.Molecule) error {
	args := m.Called(ctx, mol)
	return args.Error(0)
}

func (m *mockMoleculeRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockMoleculeRepository) List(ctx context.Context, filter *molecule.ListFilter) ([]*molecule.Molecule, int64, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*molecule.Molecule), args.Get(1).(int64), args.Error(2)
}

// Mock similarity search service
type mockSimilaritySearchService struct {
	mock.Mock
}

func (m *mockSimilaritySearchService) Search(ctx context.Context, query *patent_mining.SimilarityQuery) ([]*patent_mining.SimilarityResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*patent_mining.SimilarityResult), args.Error(1)
}

// Mock logger
type mockLogger struct {
	mock.Mock
}

func (m *mockLogger) Error(msg string, keysAndValues ...interface{}) {
	m.Called(msg, keysAndValues)
}

func (m *mockLogger) Info(msg string, keysAndValues ...interface{}) {
	m.Called(msg, keysAndValues)
}

func (m *mockLogger) Debug(msg string, keysAndValues ...interface{}) {
	m.Called(msg, keysAndValues)
}

func (m *mockLogger) Warn(msg string, keysAndValues ...interface{}) {
	m.Called(msg, keysAndValues)
}

func createTestMolecule() *molecule.Molecule {
	mol, _ := molecule.NewMolecule(
		"c1ccccc1",
		"InChI=1S/C6H6/c1-2-4-6-5-3-1/h1-6H",
		"Benzene",
		"organic",
		"ETL",
	)
	return mol
}

func TestGetMolecule_Success(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	expectedMol := createTestMolecule()
	mockRepo.On("FindByID", mock.Anything, "mol-123").Return(expectedMol, nil)

	resp, err := service.GetMolecule(context.Background(), &pb.GetMoleculeRequest{
		MoleculeId: "mol-123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "c1ccccc1", resp.Molecule.Smiles)
	mockRepo.AssertExpectations(t)
}

func TestGetMolecule_NotFound(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	mockRepo.On("FindByID", mock.Anything, "nonexistent").Return(nil, errors.NewNotFoundError("molecule not found"))
	mockLog.On("Error", mock.Anything, mock.Anything).Return()

	resp, err := service.GetMolecule(context.Background(), &pb.GetMoleculeRequest{
		MoleculeId: "nonexistent",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestGetMolecule_EmptyID(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	resp, err := service.GetMolecule(context.Background(), &pb.GetMoleculeRequest{
		MoleculeId: "",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateMolecule_Success(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*molecule.Molecule")).Return(nil)

	resp, err := service.CreateMolecule(context.Background(), &pb.CreateMoleculeRequest{
		Smiles:       "c1ccccc1",
		Name:         "Test Molecule",
		MoleculeType: "organic",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "c1ccccc1", resp.Molecule.Smiles)
	mockRepo.AssertExpectations(t)
}

func TestCreateMolecule_InvalidSMILES(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	resp, err := service.CreateMolecule(context.Background(), &pb.CreateMoleculeRequest{
		Smiles: "",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateMolecule_Duplicate(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	mockRepo.On("Create", mock.Anything, mock.Anything).Return(errors.NewConflictError("molecule already exists"))
	mockLog.On("Error", mock.Anything, mock.Anything).Return()

	resp, err := service.CreateMolecule(context.Background(), &pb.CreateMoleculeRequest{
		Smiles: "c1ccccc1",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestUpdateMolecule_Success(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	existingMol := createTestMolecule()
	mockRepo.On("FindByID", mock.Anything, "mol-123").Return(existingMol, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*molecule.Molecule")).Return(nil)

	resp, err := service.UpdateMolecule(context.Background(), &pb.UpdateMoleculeRequest{
		MoleculeId: "mol-123",
		Name:       "Updated Name",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	mockRepo.AssertExpectations(t)
}

func TestUpdateMolecule_NotFound(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	mockRepo.On("FindByID", mock.Anything, "nonexistent").Return(nil, errors.NewNotFoundError("molecule not found"))
	mockLog.On("Error", mock.Anything, mock.Anything).Return()

	resp, err := service.UpdateMolecule(context.Background(), &pb.UpdateMoleculeRequest{
		MoleculeId: "nonexistent",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestDeleteMolecule_Success(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	mockRepo.On("Delete", mock.Anything, "mol-123").Return(nil)

	resp, err := service.DeleteMolecule(context.Background(), &pb.DeleteMoleculeRequest{
		MoleculeId: "mol-123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Success)
}

func TestListMolecules_FirstPage(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	molecules := []*molecule.Molecule{createTestMolecule(), createTestMolecule()}
	mockRepo.On("List", mock.Anything, mock.Anything).Return(molecules, int64(50), nil)

	resp, err := service.ListMolecules(context.Background(), &pb.ListMoleculesRequest{
		PageSize: 20,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Molecules, 2)
	assert.Equal(t, int64(50), resp.TotalCount)
}

func TestListMolecules_InvalidPageSize(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	resp, err := service.ListMolecules(context.Background(), &pb.ListMoleculesRequest{
		PageSize: 200,
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSearchSimilar_BySMILES(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	searchResults := []*patent_mining.SimilarityResult{
		{
			Molecule:   createTestMolecule(),
			Similarity: 0.95,
			Method:     "tanimoto",
		},
	}

	mockSearch.On("Search", mock.Anything, mock.Anything).Return(searchResults, nil)

	resp, err := service.SearchSimilar(context.Background(), &pb.SearchSimilarRequest{
		Smiles:    "c1ccccc1",
		Threshold: 0.8,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.SimilarMolecules, 1)
	assert.Equal(t, float32(0.95), resp.SimilarMolecules[0].Similarity)
}

func TestSearchSimilar_InvalidThreshold(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	resp, err := service.SearchSimilar(context.Background(), &pb.SearchSimilarRequest{
		Smiles:    "c1ccccc1",
		Threshold: 1.5,
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestPredictProperties_Success(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockSearch := new(mockSimilaritySearchService)
	mockLog := new(mockLogger)

	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLog)

	testMol := createTestMolecule()
	mockRepo.On("FindBySMILES", mock.Anything, "c1ccccc1").Return(testMol, nil)

	resp, err := service.PredictProperties(context.Background(), &pb.PredictPropertiesRequest{
		Smiles: "c1ccccc1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	mockRepo.AssertExpectations(t)
}

func TestMapDomainError_AllCodes(t *testing.T) {
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
			grpcErr := mapDomainError(tt.domainError)
			assert.Equal(t, tt.expectedCode, status.Code(grpcErr))
		})
	}
}

func TestPageTokenEncoding_RoundTrip(t *testing.T) {
	originalOffset := int64(42)
	token := base64.StdEncoding.EncodeToString([]byte("42"))

	decoded, err := base64.StdEncoding.DecodeString(token)
	assert.NoError(t, err)

	var decodedOffset int64
	_, err = fmt.Sscanf(string(decoded), "%d", &decodedOffset)
	assert.NoError(t, err)
	assert.Equal(t, originalOffset, decodedOffset)
}

func TestDomainToProto_FullConversion(t *testing.T) {
	domainMol := createTestMolecule()
	domainMol.SetProperties(map[string]string{"color": "blue"})
	domainMol.SetMetadata(map[string]string{"source": "pubchem"})

	protoMol := domainToProto(domainMol)

	assert.Equal(t, "c1ccccc1", protoMol.Smiles)
	assert.Equal(t, "Benzene", protoMol.Name)
	assert.Equal(t, "ETL", protoMol.OledLayer)
	assert.NotNil(t, protoMol.Properties)
}

//Personal.AI order the ending
