package molecule

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	domainMol "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/testutil"
)

// MockMoleculeRepository is a mock implementation of domainMol.MoleculeRepository
type MockMoleculeRepository struct {
	mock.Mock
}

func (m *MockMoleculeRepository) Save(ctx context.Context, molecule *domainMol.Molecule) error {
	args := m.Called(ctx, molecule)
	return args.Error(0)
}

func (m *MockMoleculeRepository) Update(ctx context.Context, molecule *domainMol.Molecule) error {
	args := m.Called(ctx, molecule)
	return args.Error(0)
}

func (m *MockMoleculeRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockMoleculeRepository) BatchSave(ctx context.Context, molecules []*domainMol.Molecule) (int, error) {
	args := m.Called(ctx, molecules)
	return args.Int(0), args.Error(1)
}

func (m *MockMoleculeRepository) FindByID(ctx context.Context, id string) (*domainMol.Molecule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainMol.Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByInChIKey(ctx context.Context, inchiKey string) (*domainMol.Molecule, error) {
	args := m.Called(ctx, inchiKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainMol.Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindBySMILES(ctx context.Context, smiles string) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, smiles)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByIDs(ctx context.Context, ids []string) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) Exists(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockMoleculeRepository) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) {
	args := m.Called(ctx, inchiKey)
	return args.Bool(0), args.Error(1)
}

func (m *MockMoleculeRepository) Search(ctx context.Context, query *domainMol.MoleculeQuery) (*domainMol.MoleculeSearchResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainMol.MoleculeSearchResult), args.Error(1)
}

func (m *MockMoleculeRepository) Count(ctx context.Context, query *domainMol.MoleculeQuery) (int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockMoleculeRepository) FindBySource(ctx context.Context, source domainMol.MoleculeSource, offset, limit int) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, source, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByStatus(ctx context.Context, status domainMol.MoleculeStatus, offset, limit int) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, status, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByTags(ctx context.Context, tags []string, offset, limit int) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, tags, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByMolecularWeightRange(ctx context.Context, minWeight, maxWeight float64, offset, limit int) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, minWeight, maxWeight, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindWithFingerprint(ctx context.Context, fpType domainMol.FingerprintType, offset, limit int) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, fpType, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindWithoutFingerprint(ctx context.Context, fpType domainMol.FingerprintType, offset, limit int) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, fpType, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func TestCreate(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		input := &CreateInput{
			Name:   "Aspirin",
			SMILES: "CC(=O)OC1=CC=CC=C1C(=O)O",
			UserID: "user-123",
		}

		mockRepo.On("Save", ctx, mock.AnythingOfType("*molecule.Molecule")).Return(nil)

		mol, err := service.Create(ctx, input)
		assert.NoError(t, err)
		assert.NotNil(t, mol)
		assert.Equal(t, input.Name, mol.Name)
		assert.Equal(t, input.SMILES, mol.SMILES)

		mockRepo.AssertExpectations(t)
	})

	t.Run("validation error", func(t *testing.T) {
		input := &CreateInput{
			Name:   "Invalid",
			SMILES: "", // Empty SMILES
			UserID: "user-123",
		}

		mol, err := service.Create(ctx, input)
		assert.Error(t, err)
		assert.Nil(t, mol)

		// Assert error message or type if possible, but basic error check is sufficient for now
	})
}

func TestGetByID(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	ctx := context.Background()
	molID := "mol-123"

	t.Run("success", func(t *testing.T) {
		domainMolObj, _ := domainMol.NewMolecule("CC(=O)OC1=CC=CC=C1C(=O)O", domainMol.SourceManual, "")
		// Need to set ID manually as NewMolecule generates a new UUID but internal ID is uuid.UUID
		// Wait, domainMol.Molecule ID is uuid.UUID? Let's assume NewMolecule sets it.
		// However, FindByID mocks return *domainMol.Molecule.

		mockRepo.On("FindByID", ctx, molID).Return(domainMolObj, nil)

		mol, err := service.GetByID(ctx, molID)
		assert.NoError(t, err)
		assert.NotNil(t, mol)
		assert.Equal(t, domainMolObj.SMILES, mol.SMILES)

		mockRepo.AssertExpectations(t)
	})
}

func TestList(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		input := &ListInput{
			Page:     1,
			PageSize: 10,
		}

		domainMols := []*domainMol.Molecule{
			{SMILES: "C"},
			{SMILES: "CC"},
		}
		searchResult := &domainMol.MoleculeSearchResult{
			Molecules: domainMols,
			Total:     2,
		}

		mockRepo.On("Search", ctx, mock.AnythingOfType("*molecule.MoleculeQuery")).Return(searchResult, nil)

		result, err := service.List(ctx, input)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, int64(2), result.Total)
		assert.Len(t, result.Molecules, 2)

		mockRepo.AssertExpectations(t)
	})
}
