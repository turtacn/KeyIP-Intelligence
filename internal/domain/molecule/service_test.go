package molecule

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// MockMoleculeRepository is a mock implementation of MoleculeRepository
type MockMoleculeRepository struct {
	mock.Mock
}

func (m *MockMoleculeRepository) Save(ctx context.Context, mol *Molecule) error {
	args := m.Called(ctx, mol)
	return args.Error(0)
}

func (m *MockMoleculeRepository) FindByID(ctx context.Context, id string) (*Molecule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByInChIKey(ctx context.Context, inchiKey string) (*Molecule, error) {
	args := m.Called(ctx, inchiKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) {
	args := m.Called(ctx, inchiKey)
	return args.Bool(0), args.Error(1)
}

func (m *MockMoleculeRepository) Update(ctx context.Context, mol *Molecule) error {
	args := m.Called(ctx, mol)
	return args.Error(0)
}

func (m *MockMoleculeRepository) Search(ctx context.Context, query *MoleculeQuery) (*MoleculeSearchResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MoleculeSearchResult), args.Error(1)
}

func (m *MockMoleculeRepository) BatchSave(ctx context.Context, mols []*Molecule) (int, error) {
	args := m.Called(ctx, mols)
	return args.Int(0), args.Error(1)
}

func (m *MockMoleculeRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockMoleculeRepository) FindBySMILES(ctx context.Context, smiles string) ([]*Molecule, error) {
	args := m.Called(ctx, smiles)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByIDs(ctx context.Context, ids []string) ([]*Molecule, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) Exists(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockMoleculeRepository) Count(ctx context.Context, query *MoleculeQuery) (int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockMoleculeRepository) FindBySource(ctx context.Context, source MoleculeSource, offset, limit int) ([]*Molecule, error) {
	args := m.Called(ctx, source, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByStatus(ctx context.Context, status MoleculeStatus, offset, limit int) ([]*Molecule, error) {
	args := m.Called(ctx, status, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByTags(ctx context.Context, tags []string, offset, limit int) ([]*Molecule, error) {
	args := m.Called(ctx, tags, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByMolecularWeightRange(ctx context.Context, minWeight, maxWeight float64, offset, limit int) ([]*Molecule, error) {
	args := m.Called(ctx, minWeight, maxWeight, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindWithFingerprint(ctx context.Context, fpType FingerprintType, offset, limit int) ([]*Molecule, error) {
	args := m.Called(ctx, fpType, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindWithoutFingerprint(ctx context.Context, fpType FingerprintType, offset, limit int) ([]*Molecule, error) {
	args := m.Called(ctx, fpType, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Molecule), args.Error(1)
}

// MockFingerprintCalculator is a mock implementation of FingerprintCalculator
type MockFingerprintCalculator struct {
	mock.Mock
}

func (m *MockFingerprintCalculator) Standardize(ctx context.Context, smiles string) (*StructuralIdentifiers, error) {
	args := m.Called(ctx, smiles)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*StructuralIdentifiers), args.Error(1)
}

func (m *MockFingerprintCalculator) Calculate(ctx context.Context, smiles string, fpType FingerprintType, opts FingerprintCalcOptions) (*Fingerprint, error) {
	args := m.Called(ctx, smiles, fpType, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Fingerprint), args.Error(1)
}

func (m *MockFingerprintCalculator) BatchCalculate(ctx context.Context, smilesList []string, fpType FingerprintType, opts FingerprintCalcOptions) ([]*Fingerprint, error) {
	args := m.Called(ctx, smilesList, fpType, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Fingerprint), args.Error(1)
}

// MockSimilarityEngine is a mock implementation of SimilarityEngine
type MockSimilarityEngine struct {
	mock.Mock
}

func (m *MockSimilarityEngine) ComputeSimilarity(fp1, fp2 *Fingerprint, metric SimilarityMetric) (float64, error) {
	args := m.Called(fp1, fp2, metric)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockSimilarityEngine) SearchSimilar(ctx context.Context, queryFP *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error) {
	args := m.Called(ctx, queryFP, metric, threshold, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*SimilarityResult), args.Error(1)
}

// MockLogger is a mock implementation of logging.Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...logging.Field) {}
func (m *MockLogger) Info(msg string, fields ...logging.Field)  {}
func (m *MockLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *MockLogger) Error(msg string, fields ...logging.Field) {}
func (m *MockLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *MockLogger) With(fields ...logging.Field) logging.Logger {
	return m
}
func (m *MockLogger) WithContext(ctx context.Context) logging.Logger {
	return m
}
func (m *MockLogger) WithError(err error) logging.Logger {
	return m
}
func (m *MockLogger) Sync() error { return nil }

// Test cases

func TestNewMoleculeService_Success(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, err := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	assert.NoError(t, err)
	assert.NotNil(t, service)
}

func TestNewMoleculeService_NilDependencies(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	// Test nil repo
	service, err := NewMoleculeService(nil, mockFpCalc, mockSimEngine, mockLogger)
	assert.Error(t, err)
	assert.Nil(t, service)

	// Test nil fpCalc
	service, err = NewMoleculeService(mockRepo, nil, mockSimEngine, mockLogger)
	assert.Error(t, err)
	assert.Nil(t, service)

	// Test nil simEngine
	service, err = NewMoleculeService(mockRepo, mockFpCalc, nil, mockLogger)
	assert.Error(t, err)
	assert.Nil(t, service)

	// Test nil logger
	service, err = NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, nil)
	assert.Error(t, err)
	assert.Nil(t, service)
}

func TestRegisterMolecule_Success(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	smiles := "CCO"
	ids := &StructuralIdentifiers{
		CanonicalSMILES: "CCO",
		InChI:           "InChI=1S/C2H6O/c1-2-3/h3H,2H2,1H3",
		InChIKey:        "LFQSCWFLJHTTHZ-UHFFFAOYSA-N",
		Formula:         "C2H6O",
		Weight:          46.07,
	}

	mockFpCalc.On("Standardize", mock.Anything, smiles).Return(ids, nil)
	mockRepo.On("ExistsByInChIKey", mock.Anything, ids.InChIKey).Return(false, nil)

	opts := DefaultFingerprintCalcOptions()
	morganFP := &Fingerprint{Type: string(FingerprintMorgan), Hash: "morgan_hash"}
	maccsFP := &Fingerprint{Type: string(FingerprintMACCS), Hash: "maccs_hash"}

	mockFpCalc.On("Calculate", mock.Anything, ids.CanonicalSMILES, FingerprintMorgan, opts).Return(morganFP, nil)
	mockFpCalc.On("Calculate", mock.Anything, ids.CanonicalSMILES, FingerprintMACCS, opts).Return(maccsFP, nil)
	mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*molecule.Molecule")).Return(nil)

	result, err := service.RegisterMolecule(context.Background(), smiles, SourceManual, "test-ref")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, ids.CanonicalSMILES, result.CanonicalSMILES)
	assert.Equal(t, ids.InChIKey, result.InChIKey)
	assert.Equal(t, StatusActive, result.Status)
	mockRepo.AssertExpectations(t)
	mockFpCalc.AssertExpectations(t)
}

func TestRegisterMolecule_DuplicateFound(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	smiles := "CCO"
	ids := &StructuralIdentifiers{
		CanonicalSMILES: "CCO",
		InChI:           "InChI=1S/C2H6O/c1-2-3/h3H,2H2,1H3",
		InChIKey:        "LFQSCWFLJHTTHZ-UHFFFAOYSA-N",
		Formula:         "C2H6O",
		Weight:          46.07,
	}

	existingMol := &Molecule{
		ID:       uuid.New(),
		SMILES:   smiles,
		InChIKey: ids.InChIKey,
	}

	mockFpCalc.On("Standardize", mock.Anything, smiles).Return(ids, nil)
	mockRepo.On("ExistsByInChIKey", mock.Anything, ids.InChIKey).Return(true, nil)
	mockRepo.On("FindByInChIKey", mock.Anything, ids.InChIKey).Return(existingMol, nil)

	result, err := service.RegisterMolecule(context.Background(), smiles, SourceManual, "test-ref")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, existingMol.ID, result.ID)
	mockRepo.AssertExpectations(t)
}

func TestRegisterMolecule_EmptySMILES(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	result, err := service.RegisterMolecule(context.Background(), "", SourceManual, "test-ref")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetMolecule_Success(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	molID := uuid.New()
	expectedMol := &Molecule{ID: molID, SMILES: "CCO"}

	mockRepo.On("FindByID", mock.Anything, molID.String()).Return(expectedMol, nil)

	result, err := service.GetMolecule(context.Background(), molID.String())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedMol.ID, result.ID)
	mockRepo.AssertExpectations(t)
}

func TestGetMolecule_EmptyID(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	result, err := service.GetMolecule(context.Background(), "")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "id cannot be empty")
}

func TestGetMoleculeByInChIKey_Success(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	inchiKey := "LFQSCWFLJHTTHZ-UHFFFAOYSA-N"
	expectedMol := &Molecule{ID: uuid.New(), SMILES: "CCO", InChIKey: inchiKey}

	mockRepo.On("FindByInChIKey", mock.Anything, inchiKey).Return(expectedMol, nil)

	result, err := service.GetMoleculeByInChIKey(context.Background(), inchiKey)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, inchiKey, result.InChIKey)
	mockRepo.AssertExpectations(t)
}

func TestGetMoleculeByInChIKey_InvalidFormat(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	result, err := service.GetMoleculeByInChIKey(context.Background(), "invalid-inchikey")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid inchiKey format")
}

func TestFindSimilarMolecules_Success(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	targetSMILES := "CCO"
	opts := DefaultFingerprintCalcOptions()
	fp := &Fingerprint{Type: string(FingerprintMorgan), Bits: []byte{1, 2, 3}}

	similarResults := []*SimilarityResult{
		{MoleculeID: uuid.New().String(), Score: 0.95, SMILES: "CC(O)C"},
		{MoleculeID: uuid.New().String(), Score: 0.85, SMILES: "CCCO"},
	}

	mockFpCalc.On("Calculate", mock.Anything, targetSMILES, FingerprintMorgan, opts).Return(fp, nil)
	mockSimEngine.On("SearchSimilar", mock.Anything, fp, MetricTanimoto, 0.7, 10).Return(similarResults, nil)

	results, err := service.FindSimilarMolecules(context.Background(), targetSMILES, FingerprintMorgan, 0.7, 10)

	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results, 2)
	mockFpCalc.AssertExpectations(t)
	mockSimEngine.AssertExpectations(t)
}

func TestFindSimilarMolecules_EmptySMILES(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	results, err := service.FindSimilarMolecules(context.Background(), "", FingerprintMorgan, 0.7, 10)

	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "targetSMILES cannot be empty")
}

func TestFindSimilarMolecules_InvalidThreshold(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	// Threshold below 0
	results, err := service.FindSimilarMolecules(context.Background(), "CCO", FingerprintMorgan, -0.1, 10)
	assert.Error(t, err)
	assert.Nil(t, results)

	// Threshold above 1
	results, err = service.FindSimilarMolecules(context.Background(), "CCO", FingerprintMorgan, 1.5, 10)
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestCompareMolecules_Success(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	smiles1 := "CCO"
	smiles2 := "CCCO"
	opts := DefaultFingerprintCalcOptions()

	ids1 := &StructuralIdentifiers{CanonicalSMILES: "CCO", InChIKey: "LFQSCWFLJHTTHZ-UHFFFAOYSA-N"}
	ids2 := &StructuralIdentifiers{CanonicalSMILES: "CCCO", InChIKey: "BDERNNFJNOPAEC-UHFFFAOYSA-N"}

	fp1 := &Fingerprint{Type: string(FingerprintMorgan), Bits: []byte{1, 2}}
	fp2 := &Fingerprint{Type: string(FingerprintMorgan), Bits: []byte{1, 3}}

	mockFpCalc.On("Standardize", mock.Anything, smiles1).Return(ids1, nil)
	mockFpCalc.On("Standardize", mock.Anything, smiles2).Return(ids2, nil)
	mockFpCalc.On("Calculate", mock.Anything, smiles1, FingerprintMorgan, opts).Return(fp1, nil)
	mockFpCalc.On("Calculate", mock.Anything, smiles2, FingerprintMorgan, opts).Return(fp2, nil)
	mockSimEngine.On("ComputeSimilarity", fp1, fp2, MetricTanimoto).Return(0.85, nil)

	result, err := service.CompareMolecules(context.Background(), smiles1, smiles2, []FingerprintType{FingerprintMorgan})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, smiles1, result.Molecule1SMILES)
	assert.Equal(t, smiles2, result.Molecule2SMILES)
	assert.False(t, result.IsStructurallyIdentical)
	assert.InDelta(t, 0.85, result.FusedScore, 0.01)
	mockFpCalc.AssertExpectations(t)
	mockSimEngine.AssertExpectations(t)
}

func TestCompareMolecules_EmptySMILES(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	// Empty first SMILES
	result, err := service.CompareMolecules(context.Background(), "", "CCO", []FingerprintType{FingerprintMorgan})
	assert.Error(t, err)
	assert.Nil(t, result)

	// Empty second SMILES
	result, err = service.CompareMolecules(context.Background(), "CCO", "", []FingerprintType{FingerprintMorgan})
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCompareMolecules_NoFingerprintTypes(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	result, err := service.CompareMolecules(context.Background(), "CCO", "CCCO", []FingerprintType{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "at least one fingerprint type required")
}

func TestArchiveMolecule_Success(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	molID := uuid.New()
	mol := &Molecule{ID: molID, SMILES: "CCO", Status: StatusActive}

	mockRepo.On("FindByID", mock.Anything, molID.String()).Return(mol, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*molecule.Molecule")).Return(nil)

	err := service.ArchiveMolecule(context.Background(), molID.String())

	assert.NoError(t, err)
	assert.Equal(t, StatusArchived, mol.Status)
	mockRepo.AssertExpectations(t)
}

func TestDeleteMolecule_Success(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	molID := uuid.New()
	mol := &Molecule{ID: molID, SMILES: "CCO", Status: StatusActive}

	mockRepo.On("FindByID", mock.Anything, molID.String()).Return(mol, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*molecule.Molecule")).Return(nil)

	err := service.DeleteMolecule(context.Background(), molID.String())

	assert.NoError(t, err)
	assert.Equal(t, StatusDeleted, mol.Status)
	assert.NotNil(t, mol.DeletedAt)
	mockRepo.AssertExpectations(t)
}

func TestCanonicalize_Success(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	smiles := "CCO"
	ids := &StructuralIdentifiers{
		CanonicalSMILES: "CCO",
		InChIKey:        "LFQSCWFLJHTTHZ-UHFFFAOYSA-N",
	}

	mockFpCalc.On("Standardize", mock.Anything, smiles).Return(ids, nil)

	canonical, inchiKey, err := service.Canonicalize(context.Background(), smiles)

	assert.NoError(t, err)
	assert.Equal(t, "CCO", canonical)
	assert.Equal(t, "LFQSCWFLJHTTHZ-UHFFFAOYSA-N", inchiKey)
	mockFpCalc.AssertExpectations(t)
}

func TestCanonicalize_Error(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	mockFpCalc.On("Standardize", mock.Anything, "invalid").Return(nil, errors.New("parse error"))

	canonical, inchiKey, err := service.Canonicalize(context.Background(), "invalid")

	assert.Error(t, err)
	assert.Empty(t, canonical)
	assert.Empty(t, inchiKey)
	mockFpCalc.AssertExpectations(t)
}

func TestCanonicalizeFromInChI_NotImplemented(t *testing.T) {
	mockRepo := new(MockMoleculeRepository)
	mockFpCalc := new(MockFingerprintCalculator)
	mockSimEngine := new(MockSimilarityEngine)
	mockLogger := new(MockLogger)

	service, _ := NewMoleculeService(mockRepo, mockFpCalc, mockSimEngine, mockLogger)

	canonical, inchiKey, err := service.CanonicalizeFromInChI(context.Background(), "InChI=1S/C2H6O/c1-2-3/h3H,2H2,1H3")

	assert.Error(t, err)
	assert.Empty(t, canonical)
	assert.Empty(t, inchiKey)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestWeightedAverageFusion_Fuse(t *testing.T) {
	fusion := &WeightedAverageFusion{}

	// Test with no weights
	scores := map[FingerprintType]float64{
		FingerprintMorgan: 0.8,
		FingerprintMACCS:  0.6,
	}

	result, err := fusion.Fuse(scores, nil)
	assert.NoError(t, err)
	assert.InDelta(t, 0.7, result, 0.01)

	// Test with custom weights
	weights := map[FingerprintType]float64{
		FingerprintMorgan: 2.0,
		FingerprintMACCS:  1.0,
	}

	result, err = fusion.Fuse(scores, weights)
	assert.NoError(t, err)
	// (0.8*2 + 0.6*1) / (2+1) = 2.2/3 ≈ 0.733
	assert.InDelta(t, 0.733, result, 0.01)

	// Test empty scores
	result, err = fusion.Fuse(map[FingerprintType]float64{}, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0.0, result)
}

func TestStatus_IsValid(t *testing.T) {
	assert.True(t, StatusPending.IsValid())
	assert.True(t, StatusActive.IsValid())
	assert.True(t, StatusArchived.IsValid())
	assert.True(t, StatusDeleted.IsValid())
	assert.False(t, Status("invalid").IsValid())
}

func TestFingerprint_IsDenseVector(t *testing.T) {
	// With bits only
	fp1 := &Fingerprint{Bits: []byte{1, 2, 3}}
	assert.False(t, fp1.IsDenseVector())

	// With vector
	fp2 := &Fingerprint{Vector: []float32{0.1, 0.2, 0.3}}
	assert.True(t, fp2.IsDenseVector())

	// Empty
	fp3 := &Fingerprint{}
	assert.False(t, fp3.IsDenseVector())
}

func TestDefaultFingerprintCalcOptions(t *testing.T) {
	opts := DefaultFingerprintCalcOptions()
	assert.Equal(t, 2, opts.Radius)
	assert.Equal(t, 2048, opts.Bits)
}

func TestSimilarityMetric_Constants(t *testing.T) {
	assert.Equal(t, SimilarityMetric("tanimoto"), MetricTanimoto)
	assert.Equal(t, SimilarityMetric("dice"), MetricDice)
	assert.Equal(t, SimilarityMetric("cosine"), MetricCosine)
}

func TestMolecule_GetSMILES(t *testing.T) {
	// Without canonical SMILES
	mol := &Molecule{SMILES: "CCO"}
	assert.Equal(t, "CCO", mol.GetSMILES())

	// With canonical SMILES
	mol.CanonicalSMILES = "OCC"
	assert.Equal(t, "OCC", mol.GetSMILES())
}

func TestMolecule_GetID(t *testing.T) {
	id := uuid.New()
	mol := &Molecule{ID: id}
	assert.Equal(t, id.String(), mol.GetID())
}

func TestMolecule_String(t *testing.T) {
	id := uuid.New()
	mol := &Molecule{ID: id, SMILES: "CCO", Status: StatusActive}
	str := mol.String()
	assert.Contains(t, str, id.String())
	assert.Contains(t, str, "CCO")
	assert.Contains(t, str, "active")
}

func TestMolecule_Validate(t *testing.T) {
	// Valid molecule
	mol := &Molecule{ID: uuid.New(), SMILES: "CCO", MolecularWeight: 46.07}
	assert.NoError(t, mol.Validate())

	// Missing ID
	mol2 := &Molecule{SMILES: "CCO"}
	assert.Error(t, mol2.Validate())

	// Missing SMILES
	mol3 := &Molecule{ID: uuid.New()}
	assert.Error(t, mol3.Validate())

	// Negative molecular weight
	mol4 := &Molecule{ID: uuid.New(), SMILES: "CCO", MolecularWeight: -1}
	assert.Error(t, mol4.Validate())
}
