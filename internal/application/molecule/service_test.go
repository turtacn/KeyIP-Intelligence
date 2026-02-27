package molecule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	domainMol "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/testutil"
)

// mockMoleculeRepository is a mock implementation of domainMol.MoleculeRepository
type mockMoleculeRepository struct {
	mock.Mock
}

func (m *mockMoleculeRepository) Save(ctx context.Context, molecule *domainMol.Molecule) error {
	args := m.Called(ctx, molecule)
	return args.Error(0)
}

func (m *mockMoleculeRepository) Update(ctx context.Context, molecule *domainMol.Molecule) error {
	args := m.Called(ctx, molecule)
	return args.Error(0)
}

func (m *mockMoleculeRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockMoleculeRepository) BatchSave(ctx context.Context, molecules []*domainMol.Molecule) (int, error) {
	args := m.Called(ctx, molecules)
	return args.Int(0), args.Error(1)
}

func (m *mockMoleculeRepository) FindByID(ctx context.Context, id string) (*domainMol.Molecule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainMol.Molecule), args.Error(1)
}

func (m *mockMoleculeRepository) FindByInChIKey(ctx context.Context, inchiKey string) (*domainMol.Molecule, error) {
	args := m.Called(ctx, inchiKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainMol.Molecule), args.Error(1)
}

func (m *mockMoleculeRepository) FindBySMILES(ctx context.Context, smiles string) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, smiles)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *mockMoleculeRepository) FindByIDs(ctx context.Context, ids []string) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *mockMoleculeRepository) Exists(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *mockMoleculeRepository) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) {
	args := m.Called(ctx, inchiKey)
	return args.Bool(0), args.Error(1)
}

func (m *mockMoleculeRepository) Search(ctx context.Context, query *domainMol.MoleculeQuery) (*domainMol.MoleculeSearchResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainMol.MoleculeSearchResult), args.Error(1)
}

func (m *mockMoleculeRepository) Count(ctx context.Context, query *domainMol.MoleculeQuery) (int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockMoleculeRepository) FindBySource(ctx context.Context, source domainMol.MoleculeSource, offset, limit int) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, source, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *mockMoleculeRepository) FindByStatus(ctx context.Context, status domainMol.MoleculeStatus, offset, limit int) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, status, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *mockMoleculeRepository) FindByTags(ctx context.Context, tags []string, offset, limit int) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, tags, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *mockMoleculeRepository) FindByMolecularWeightRange(ctx context.Context, minWeight, maxWeight float64, offset, limit int) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, minWeight, maxWeight, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *mockMoleculeRepository) FindWithFingerprint(ctx context.Context, fpType domainMol.FingerprintType, offset, limit int) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, fpType, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func (m *mockMoleculeRepository) FindWithoutFingerprint(ctx context.Context, fpType domainMol.FingerprintType, offset, limit int) ([]*domainMol.Molecule, error) {
	args := m.Called(ctx, fpType, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainMol.Molecule), args.Error(1)
}

func TestCreate(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	// Test case 1: Success
	input := &CreateInput{
		Name:       "Aspirin",
		SMILES:     "CC(=O)OC1=CC=CC=C1C(=O)O",
		InChI:      "InChI=1S/C9H8O4/c1-6(10)13-8-5-3-2-4-7(8)9(11)12/h2-5H,1H3,(H,11,12)",
		MolFormula: "C9H8O4",
		UserID:     "user1",
	}

	mockRepo.On("Save", mock.Anything, mock.MatchedBy(func(m *domainMol.Molecule) bool {
		return m.SMILES == input.SMILES && m.Name == input.Name
	})).Return(nil)

	result, err := service.Create(context.Background(), input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, input.Name, result.Name)
	assert.Equal(t, input.SMILES, result.SMILES)
	assert.NotEmpty(t, result.ID)

	// Test case 2: Empty SMILES
	inputEmpty := &CreateInput{
		Name: "Empty",
	}
	resultEmpty, errEmpty := service.Create(context.Background(), inputEmpty)
	assert.Error(t, errEmpty)
	assert.Nil(t, resultEmpty)

	// Test case 3: Repo Error
	inputRepoErr := &CreateInput{
		Name:   "Error",
		SMILES: "C",
	}
	mockRepo.On("Save", mock.Anything, mock.MatchedBy(func(m *domainMol.Molecule) bool {
		return m.SMILES == "C"
	})).Return(errors.New("db error"))

	resultRepoErr, errRepoErr := service.Create(context.Background(), inputRepoErr)
	assert.Error(t, errRepoErr)
	assert.Nil(t, resultRepoErr)
	assert.Contains(t, errRepoErr.Error(), "db error")
}

func TestGetByID(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	id := uuid.New().String()
	domainMolecule := &domainMol.Molecule{
		ID:        uuid.MustParse(id),
		SMILES:    "C",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Success
	mockRepo.On("FindByID", mock.Anything, id).Return(domainMolecule, nil)

	result, err := service.GetByID(context.Background(), id)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, id, result.ID)

	// Not Found
	mockRepo.On("FindByID", mock.Anything, "missing").Return(nil, errors.New("not found"))
	resultMissing, errMissing := service.GetByID(context.Background(), "missing")
	assert.Error(t, errMissing)
	assert.Nil(t, resultMissing)
}

func TestList(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	// Success
	input := &ListInput{
		Page:     1,
		PageSize: 10,
	}

	domainMolecules := []*domainMol.Molecule{
		{ID: uuid.New(), SMILES: "C1"},
		{ID: uuid.New(), SMILES: "C2"},
	}

	searchResult := &domainMol.MoleculeSearchResult{
		Molecules: domainMolecules,
		Total:     20,
	}

	mockRepo.On("Search", mock.Anything, mock.MatchedBy(func(q *domainMol.MoleculeQuery) bool {
		return q.Offset == 0 && q.Limit == 10
	})).Return(searchResult, nil)

	result, err := service.List(context.Background(), input)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(20), result.Total)
	assert.Len(t, result.Molecules, 2)
	assert.Equal(t, 2, result.TotalPages)

	// Error
	mockRepo.On("Search", mock.Anything, mock.MatchedBy(func(q *domainMol.MoleculeQuery) bool {
		return q.Limit == 100 // Test default max logic if needed, or just standard call
	})).Return(nil, errors.New("db error")).Once() // Ensure we don't conflict with previous match if logic is same

	// Let's test error case with specific input to differentiate
	inputErr := &ListInput{Page: 2, PageSize: 10}
	mockRepo.On("Search", mock.Anything, mock.MatchedBy(func(q *domainMol.MoleculeQuery) bool {
		return q.Offset == 10
	})).Return(nil, errors.New("db error"))

	resultErr, errErr := service.List(context.Background(), inputErr)
	assert.Error(t, errErr)
	assert.Nil(t, resultErr)
}

func TestUpdate(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	id := uuid.New().String()
	input := &UpdateInput{
		ID:     id,
		UserID: "user1",
	}
	newName := "New Name"
	input.Name = &newName

	existingMol := &domainMol.Molecule{
		ID:     uuid.MustParse(id),
		SMILES: "C",
		Name:   "Old Name",
	}

	mockRepo.On("FindByID", mock.Anything, id).Return(existingMol, nil)
	mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(m *domainMol.Molecule) bool {
		return m.Name == "New Name"
	})).Return(nil)

	result, err := service.Update(context.Background(), input)
	assert.NoError(t, err)
	assert.Equal(t, "New Name", result.Name)

	// Not Found
	mockRepo.On("FindByID", mock.Anything, "missing").Return(nil, errors.New("not found"))
	inputMissing := &UpdateInput{ID: "missing"}
	resultMissing, errMissing := service.Update(context.Background(), inputMissing)
	assert.Error(t, errMissing)
	assert.Nil(t, resultMissing)
}

func TestDelete(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	mockRepo.On("Delete", mock.Anything, "id1").Return(nil)
	err := service.Delete(context.Background(), "id1", "user1")
	assert.NoError(t, err)
}

func TestSearchByStructure(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	result, err := service.SearchByStructure(context.Background(), &StructureSearchInput{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Molecules)
	assert.Equal(t, int64(0), result.Total)
}

func TestSearchBySimilarity(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	result, err := service.SearchBySimilarity(context.Background(), &SimilaritySearchInput{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Molecules)
	assert.Equal(t, int64(0), result.Total)
}

func TestCalculateProperties(t *testing.T) {
	mockRepo := new(mockMoleculeRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	// Valid SMILES
	result, err := service.CalculateProperties(context.Background(), &CalculatePropertiesInput{SMILES: "C"})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "C", result.SMILES)

	// Invalid SMILES (validation error)
	// domainMol.NewMolecule validates SMILES
	resultInv, errInv := service.CalculateProperties(context.Background(), &CalculatePropertiesInput{SMILES: ""})
	assert.Error(t, errInv)
	assert.Nil(t, resultInv)
}
