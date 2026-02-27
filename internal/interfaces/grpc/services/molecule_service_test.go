// Phase 11 - File 255: internal/interfaces/grpc/services/molecule_service_test.go
package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/turtacn/KeyIP-Intelligence/api/proto/molecule/v1"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// MockMoleculeRepo mocks the MoleculeRepository interface
type MockMoleculeRepo struct {
	mock.Mock
}

func (m *MockMoleculeRepo) FindByID(ctx context.Context, id string) (*molecule.Molecule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.Molecule), args.Error(1)
}

func (m *MockMoleculeRepo) FindBySMILES(ctx context.Context, smiles string) ([]*molecule.Molecule, error) {
	args := m.Called(ctx, smiles)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*molecule.Molecule), args.Error(1)
}

func (m *MockMoleculeRepo) Save(ctx context.Context, mol *molecule.Molecule) error {
	args := m.Called(ctx, mol)
	return args.Error(0)
}

func (m *MockMoleculeRepo) Update(ctx context.Context, mol *molecule.Molecule) error {
	args := m.Called(ctx, mol)
	return args.Error(0)
}

func (m *MockMoleculeRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockMoleculeRepo) Search(ctx context.Context, query *molecule.MoleculeQuery) (*molecule.MoleculeSearchResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.MoleculeSearchResult), args.Error(1)
}

func (m *MockMoleculeRepo) FindByInChIKey(ctx context.Context, inchiKey string) (*molecule.Molecule, error) {
	args := m.Called(ctx, inchiKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.Molecule), args.Error(1)
}

func (m *MockMoleculeRepo) BatchSave(ctx context.Context, molecules []*molecule.Molecule) (int, error) {
	args := m.Called(ctx, molecules)
	return args.Int(0), args.Error(1)
}

// Implement other missing methods to satisfy the interface if necessary
// Assuming the server implementation doesn't call them, we can leave them out or add stubs.
// But Go compiler requires full interface implementation.
// Adding dummy implementations for other methods in MoleculeRepository interface.

func (m *MockMoleculeRepo) FindByIDs(ctx context.Context, ids []string) ([]*molecule.Molecule, error) {
	return nil, nil
}
func (m *MockMoleculeRepo) Exists(ctx context.Context, id string) (bool, error) {
	return false, nil
}
func (m *MockMoleculeRepo) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) {
	return false, nil
}
func (m *MockMoleculeRepo) Count(ctx context.Context, query *molecule.MoleculeQuery) (int64, error) {
	return 0, nil
}
func (m *MockMoleculeRepo) FindBySource(ctx context.Context, source molecule.MoleculeSource, offset, limit int) ([]*molecule.Molecule, error) {
	return nil, nil
}
func (m *MockMoleculeRepo) FindByStatus(ctx context.Context, status molecule.MoleculeStatus, offset, limit int) ([]*molecule.Molecule, error) {
	return nil, nil
}
func (m *MockMoleculeRepo) FindByTags(ctx context.Context, tags []string, offset, limit int) ([]*molecule.Molecule, error) {
	return nil, nil
}
func (m *MockMoleculeRepo) FindByMolecularWeightRange(ctx context.Context, minWeight, maxWeight float64, offset, limit int) ([]*molecule.Molecule, error) {
	return nil, nil
}
func (m *MockMoleculeRepo) FindWithFingerprint(ctx context.Context, fpType molecule.FingerprintType, offset, limit int) ([]*molecule.Molecule, error) {
	return nil, nil
}
func (m *MockMoleculeRepo) FindWithoutFingerprint(ctx context.Context, fpType molecule.FingerprintType, offset, limit int) ([]*molecule.Molecule, error) {
	return nil, nil
}

// MockSimilaritySearch mocks the SimilaritySearchService interface
type MockSimilaritySearch struct {
	mock.Mock
}

func (m *MockSimilaritySearch) Search(ctx context.Context, query *patent_mining.SimilarityQuery) ([]patent_mining.SimilarityResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]patent_mining.SimilarityResult), args.Error(1)
}

func (m *MockSimilaritySearch) SearchByStructure(ctx context.Context, req *patent_mining.SearchByStructureRequest) (*patent_mining.SimilaritySearchResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent_mining.SimilaritySearchResult), args.Error(1)
}

func (m *MockSimilaritySearch) SearchByFingerprint(ctx context.Context, req *patent_mining.SearchByFingerprintRequest) (*patent_mining.SimilaritySearchResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent_mining.SimilaritySearchResult), args.Error(1)
}

func (m *MockSimilaritySearch) SearchBySemantic(ctx context.Context, req *patent_mining.SearchBySemanticRequest) (*patent_mining.SimilaritySearchResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent_mining.SimilaritySearchResult), args.Error(1)
}

func (m *MockSimilaritySearch) SearchByPatent(ctx context.Context, req *patent_mining.SearchByPatentRequest) (*patent_mining.SimilaritySearchResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent_mining.SimilaritySearchResult), args.Error(1)
}

func (m *MockSimilaritySearch) GetSearchHistory(ctx context.Context, userID string, limit int) ([]patent_mining.SearchHistoryEntry, error) {
	args := m.Called(ctx, userID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]patent_mining.SearchHistoryEntry), args.Error(1)
}

func (m *MockSimilaritySearch) SearchByText(ctx context.Context, req *patent_mining.PatentTextSearchRequest) ([]*patent_mining.CLIPatentSearchResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*patent_mining.CLIPatentSearchResult), args.Error(1)
}

// MockLogger mocks the logging.Logger interface
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...logging.Field) { m.Called(msg, fields) }
func (m *MockLogger) Info(msg string, fields ...logging.Field)  { m.Called(msg, fields) }
func (m *MockLogger) Warn(msg string, fields ...logging.Field)  { m.Called(msg, fields) }
func (m *MockLogger) Error(msg string, fields ...logging.Field) { m.Called(msg, fields) }
func (m *MockLogger) Fatal(msg string, fields ...logging.Field) { m.Called(msg, fields) }
func (m *MockLogger) With(fields ...logging.Field) logging.Logger {
	args := m.Called(fields)
	return args.Get(0).(logging.Logger)
}
func (m *MockLogger) WithContext(ctx context.Context) logging.Logger {
	args := m.Called(ctx)
	return args.Get(0).(logging.Logger)
}
func (m *MockLogger) WithError(err error) logging.Logger {
	args := m.Called(err)
	return args.Get(0).(logging.Logger)
}
func (m *MockLogger) Sync() error { return nil }

func TestGetMolecule(t *testing.T) {
	mockRepo := new(MockMoleculeRepo)
	mockSearch := new(MockSimilaritySearch)
	mockLogger := new(MockLogger)
	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLogger)

	ctx := context.Background()
	molID := "mol-123"

	// Create molecule using constructor
	expectedMol, _ := molecule.NewMolecule("C", molecule.SourceManual, "test")
	// The service expects the ID to match what's returned.
	// Since ID is a private field in NewMolecule (generated by uuid.New), we can't set it directly easily if it's not exported.
	// However, Molecule struct has ID field which is uuid.UUID. We can rely on the repo returning *this* object.
	// But the service response check expects `MoleculeId` to match.
	// We can update the expected check to match whatever ID was generated, OR we can try to set it if we can.
	// Since we can't set ID easily without helper, let's just let it be random and capture it from the object.

	t.Run("Success", func(t *testing.T) {
		mockRepo.On("FindByID", ctx, molID).Return(expectedMol, nil)

		resp, err := service.GetMolecule(ctx, &pb.GetMoleculeRequest{MoleculeId: molID})
		assert.NoError(t, err)
		assert.Equal(t, expectedMol.ID.String(), resp.Molecule.MoleculeId) // Check against generated ID
		assert.Equal(t, "C", resp.Molecule.Smiles)
		mockRepo.AssertExpectations(t)
	})

	t.Run("NotFound", func(t *testing.T) {
		// NewNotFound takes a format string and args, so "%s" is correct
		mockRepo.On("FindByID", ctx, "unknown").Return(nil, errors.NewNotFound("%s not found", "molecule"))
		mockLogger.On("Error", mock.Anything, mock.Anything).Return()

		resp, err := service.GetMolecule(ctx, &pb.GetMoleculeRequest{MoleculeId: "unknown"})
		assert.Error(t, err)
		assert.Nil(t, resp)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})

	t.Run("InvalidArgument", func(t *testing.T) {
		resp, err := service.GetMolecule(ctx, &pb.GetMoleculeRequest{MoleculeId: ""})
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestCreateMolecule(t *testing.T) {
	mockRepo := new(MockMoleculeRepo)
	mockSearch := new(MockSimilaritySearch)
	mockLogger := new(MockLogger)
	service := NewMoleculeServiceServer(mockRepo, mockSearch, mockLogger)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		req := &pb.CreateMoleculeRequest{
			Smiles: "C",
			Name:   "Methane",
		}

		mockRepo.On("Save", ctx, mock.AnythingOfType("*molecule.Molecule")).Return(nil)

		resp, err := service.CreateMolecule(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp.Molecule)
		assert.Equal(t, "Methane", resp.Molecule.Name)
		mockRepo.AssertExpectations(t)
	})

	t.Run("MissingSMILES", func(t *testing.T) {
		req := &pb.CreateMoleculeRequest{Name: "Invalid"}
		resp, err := service.CreateMolecule(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

//Personal.AI order the ending
