package molecule

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// Mock implementations
type MockMoleculeRepository struct {
	mock.Mock
}

func (m *MockMoleculeRepository) Save(ctx context.Context, mol *Molecule) error {
	args := m.Called(ctx, mol)
	return args.Error(0)
}

func (m *MockMoleculeRepository) Update(ctx context.Context, mol *Molecule) error {
	args := m.Called(ctx, mol)
	return args.Error(0)
}

func (m *MockMoleculeRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockMoleculeRepository) BatchSave(ctx context.Context, mols []*Molecule) (int, error) {
	args := m.Called(ctx, mols)
	return args.Int(0), args.Error(1)
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

func (m *MockMoleculeRepository) FindBySMILES(ctx context.Context, smiles string) ([]*Molecule, error) {
	args := m.Called(ctx, smiles)
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByIDs(ctx context.Context, ids []string) ([]*Molecule, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) Exists(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockMoleculeRepository) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) {
	args := m.Called(ctx, inchiKey)
	return args.Bool(0), args.Error(1)
}

func (m *MockMoleculeRepository) Search(ctx context.Context, query *MoleculeQuery) (*MoleculeSearchResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MoleculeSearchResult), args.Error(1)
}

func (m *MockMoleculeRepository) Count(ctx context.Context, query *MoleculeQuery) (int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockMoleculeRepository) FindBySource(ctx context.Context, source MoleculeSource, offset, limit int) ([]*Molecule, error) {
	args := m.Called(ctx, source, offset, limit)
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByStatus(ctx context.Context, status MoleculeStatus, offset, limit int) ([]*Molecule, error) {
	args := m.Called(ctx, status, offset, limit)
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByTags(ctx context.Context, tags []string, offset, limit int) ([]*Molecule, error) {
	args := m.Called(ctx, tags, offset, limit)
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindByMolecularWeightRange(ctx context.Context, min, max float64, offset, limit int) ([]*Molecule, error) {
	args := m.Called(ctx, min, max, offset, limit)
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindWithFingerprint(ctx context.Context, fpType FingerprintType, offset, limit int) ([]*Molecule, error) {
	args := m.Called(ctx, fpType, offset, limit)
	return args.Get(0).([]*Molecule), args.Error(1)
}

func (m *MockMoleculeRepository) FindWithoutFingerprint(ctx context.Context, fpType FingerprintType, offset, limit int) ([]*Molecule, error) {
	args := m.Called(ctx, fpType, offset, limit)
	return args.Get(0).([]*Molecule), args.Error(1)
}

type MockFingerprintCalculator struct {
	mock.Mock
}

func (m *MockFingerprintCalculator) Calculate(ctx context.Context, smiles string, fpType FingerprintType, opts *FingerprintCalcOptions) (*Fingerprint, error) {
	args := m.Called(ctx, smiles, fpType, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Fingerprint), args.Error(1)
}

func (m *MockFingerprintCalculator) BatchCalculate(ctx context.Context, smiles []string, fpType FingerprintType, opts *FingerprintCalcOptions) ([]*Fingerprint, error) {
	args := m.Called(ctx, smiles, fpType, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Fingerprint), args.Error(1)
}

func (m *MockFingerprintCalculator) SupportedTypes() []FingerprintType {
	args := m.Called()
	return args.Get(0).([]FingerprintType)
}

func (m *MockFingerprintCalculator) Standardize(ctx context.Context, smiles string) (*StructuralIdentifiers, error) {
	args := m.Called(ctx, smiles)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*StructuralIdentifiers), args.Error(1)
}

type MockSimilarityEngine struct {
	mock.Mock
}

func (m *MockSimilarityEngine) SearchSimilar(ctx context.Context, target *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error) {
	args := m.Called(ctx, target, metric, threshold, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*SimilarityResult), args.Error(1)
}

func (m *MockSimilarityEngine) ComputeSimilarity(fp1, fp2 *Fingerprint, metric SimilarityMetric) (float64, error) {
	args := m.Called(fp1, fp2, metric)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockSimilarityEngine) BatchComputeSimilarity(target *Fingerprint, candidates []*Fingerprint, metric SimilarityMetric) ([]float64, error) {
	args := m.Called(target, candidates, metric)
	return args.Get(0).([]float64), args.Error(1)
}

func (m *MockSimilarityEngine) RankBySimilarity(ctx context.Context, target *Fingerprint, ids []string, metric SimilarityMetric) ([]*SimilarityResult, error) {
	args := m.Called(ctx, target, ids, metric)
	return args.Get(0).([]*SimilarityResult), args.Error(1)
}

// MockLogger (minimal for test)
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...logging.Field) {}
func (m *MockLogger) Info(msg string, fields ...logging.Field)  {}
func (m *MockLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *MockLogger) Error(msg string, fields ...logging.Field) {}
func (m *MockLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *MockLogger) With(fields ...logging.Field) logging.Logger { return m }
func (m *MockLogger) WithContext(ctx context.Context) logging.Logger { return m }
func (m *MockLogger) WithError(err error) logging.Logger { return m }
func (m *MockLogger) Sync() error { return nil }

func setupService(t *testing.T) (*MoleculeService, *MockMoleculeRepository, *MockFingerprintCalculator, *MockSimilarityEngine) {
	repo := new(MockMoleculeRepository)
	fpCalc := new(MockFingerprintCalculator)
	simEngine := new(MockSimilarityEngine)
	logger := new(MockLogger)

	svc, err := NewMoleculeService(repo, fpCalc, simEngine, logger)
	assert.NoError(t, err)

	return svc, repo, fpCalc, simEngine
}

func TestNewMoleculeService(t *testing.T) {
	repo := new(MockMoleculeRepository)
	fpCalc := new(MockFingerprintCalculator)
	simEngine := new(MockSimilarityEngine)
	logger := new(MockLogger)

	svc, err := NewMoleculeService(repo, fpCalc, simEngine, logger)
	assert.NoError(t, err)
	assert.NotNil(t, svc)

	_, err = NewMoleculeService(nil, fpCalc, simEngine, logger)
	assert.Error(t, err)
}

func TestMoleculeService_RegisterMolecule(t *testing.T) {
	svc, repo, fpCalc, _ := setupService(t)
	ctx := context.Background()

	smiles := "c1ccccc1"
	ids := &StructuralIdentifiers{
		CanonicalSMILES: "c1ccccc1",
		InChIKey:        "UHOVQNZJYSORNB-UHFFFAOYSA-N",
		InChI:           "InChI=1S/C6H6/c1-2-4-6-5-3-1/h1-6H",
		Formula:         "C6H6",
		Weight:          78.11,
	}

	// Mock Standardize
	fpCalc.On("Standardize", ctx, smiles).Return(ids, nil)

	// Mock Exists
	repo.On("ExistsByInChIKey", ctx, ids.InChIKey).Return(false, nil)

	// Mock Calculate Fingerprints
	fpMorgan, _ := NewBitFingerprint(FingerprintMorgan, make([]byte, 256), 2048, 2)
	fpMACCS, _ := NewBitFingerprint(FingerprintMACCS, make([]byte, 21), 166, 0) // Valid MACCS

	fpCalc.On("Calculate", ctx, ids.CanonicalSMILES, FingerprintMorgan, mock.Anything).Return(fpMorgan, nil)
	fpCalc.On("Calculate", ctx, ids.CanonicalSMILES, FingerprintMACCS, mock.Anything).Return(fpMACCS, nil)

	// Mock Save
	repo.On("Save", ctx, mock.AnythingOfType("*molecule.Molecule")).Return(nil)

	mol, err := svc.RegisterMolecule(ctx, smiles, SourceManual, "ref")
	assert.NoError(t, err)
	assert.NotNil(t, mol)
	assert.Equal(t, MoleculeStatusActive, mol.Status())
	assert.True(t, mol.HasFingerprint(FingerprintMorgan))
}

func TestMoleculeService_RegisterMolecule_Exists(t *testing.T) {
	svc, repo, fpCalc, _ := setupService(t)
	ctx := context.Background()

	smiles := "c1ccccc1"
	ids := &StructuralIdentifiers{InChIKey: "KEY"}

	existingMol, _ := NewMolecule(smiles, SourceManual, "ref")

	fpCalc.On("Standardize", ctx, smiles).Return(ids, nil)
	repo.On("ExistsByInChIKey", ctx, "KEY").Return(true, nil)
	repo.On("FindByInChIKey", ctx, "KEY").Return(existingMol, nil)

	mol, err := svc.RegisterMolecule(ctx, smiles, SourceManual, "ref")
	assert.NoError(t, err)
	assert.Equal(t, existingMol, mol)
}

func TestMoleculeService_FindSimilarMolecules(t *testing.T) {
	svc, _, fpCalc, simEngine := setupService(t)
	ctx := context.Background()

	smiles := "c1ccccc1"
	fp, _ := NewBitFingerprint(FingerprintMorgan, make([]byte, 256), 2048, 2)
	results := []*SimilarityResult{{Score: 0.9}}

	fpCalc.On("Calculate", ctx, smiles, FingerprintMorgan, mock.Anything).Return(fp, nil)
	simEngine.On("SearchSimilar", ctx, fp, MetricTanimoto, 0.7, 10).Return(results, nil)

	got, err := svc.FindSimilarMolecules(ctx, smiles, FingerprintMorgan, 0.7, 10)
	assert.NoError(t, err)
	assert.Equal(t, results, got)
}

func TestMoleculeService_ArchiveMolecule(t *testing.T) {
	svc, repo, _, _ := setupService(t)
	ctx := context.Background()

	mol, _ := NewMolecule("c1ccccc1", SourceManual, "ref")
	// Must be active to archive
	err := mol.SetStructureIdentifiers("s", "i", "AAAAAAAAAAAAAA-BBBBBBBBBB-C", "f", 10.0)
	assert.NoError(t, err)
	err = mol.Activate()
	assert.NoError(t, err)

	repo.On("FindByID", ctx, "id").Return(mol, nil)
	repo.On("Update", ctx, mol).Return(nil)

	err = svc.ArchiveMolecule(ctx, "id")
	assert.NoError(t, err)
	assert.True(t, mol.IsArchived())
}

//Personal.AI order the ending
