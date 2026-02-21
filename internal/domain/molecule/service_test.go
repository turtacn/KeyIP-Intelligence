package molecule

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// Mock Implementations

type mockMoleculeRepository struct {
	SaveFunc               func(ctx context.Context, m *Molecule) error
	UpdateFunc             func(ctx context.Context, m *Molecule) error
	DeleteFunc             func(ctx context.Context, id string) error
	FindByIDFunc           func(ctx context.Context, id string) (*Molecule, error)
	FindByInChIKeyFunc     func(ctx context.Context, inchiKey string) (*Molecule, error)
	ExistsByInChIKeyFunc   func(ctx context.Context, inchiKey string) (bool, error)
	SearchFunc             func(ctx context.Context, q *MoleculeQuery) (*MoleculeSearchResult, error)
	BatchSaveFunc          func(ctx context.Context, molecules []*Molecule) (int, error)
	SaveCallCount          int
	UpdateCallCount        int
	BatchSaveCallCount     int
	ExistsByInChIKeyCount int
}

func (m *mockMoleculeRepository) Save(ctx context.Context, mol *Molecule) error {
	m.SaveCallCount++
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, mol)
	}
	return nil
}
func (m *mockMoleculeRepository) Update(ctx context.Context, mol *Molecule) error {
	m.UpdateCallCount++
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, mol)
	}
	return nil
}
func (m *mockMoleculeRepository) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}
func (m *mockMoleculeRepository) BatchSave(ctx context.Context, molecules []*Molecule) (int, error) {
	m.BatchSaveCallCount++
	if m.BatchSaveFunc != nil {
		return m.BatchSaveFunc(ctx, molecules)
	}
	return len(molecules), nil
}
func (m *mockMoleculeRepository) FindByID(ctx context.Context, id string) (*Molecule, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, id)
	}
	return nil, nil
}
func (m *mockMoleculeRepository) FindByInChIKey(ctx context.Context, inchiKey string) (*Molecule, error) {
	if m.FindByInChIKeyFunc != nil {
		return m.FindByInChIKeyFunc(ctx, inchiKey)
	}
	return nil, nil
}
func (m *mockMoleculeRepository) FindBySMILES(ctx context.Context, smiles string) ([]*Molecule, error) { return nil, nil }
func (m *mockMoleculeRepository) FindByIDs(ctx context.Context, ids []string) ([]*Molecule, error) { return nil, nil }
func (m *mockMoleculeRepository) Exists(ctx context.Context, id string) (bool, error)         { return false, nil }
func (m *mockMoleculeRepository) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) {
	m.ExistsByInChIKeyCount++
	if m.ExistsByInChIKeyFunc != nil {
		return m.ExistsByInChIKeyFunc(ctx, inchiKey)
	}
	return false, nil
}
func (m *mockMoleculeRepository) Search(ctx context.Context, q *MoleculeQuery) (*MoleculeSearchResult, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, q)
	}
	return nil, nil
}
func (m *mockMoleculeRepository) Count(ctx context.Context, q *MoleculeQuery) (int64, error) { return 0, nil }
func (m *mockMoleculeRepository) FindBySource(ctx context.Context, s MoleculeSource, o, l int) ([]*Molecule, error) {
	return nil, nil
}
func (m *mockMoleculeRepository) FindByStatus(ctx context.Context, s MoleculeStatus, o, l int) ([]*Molecule, error) {
	return nil, nil
}
func (m *mockMoleculeRepository) FindByTags(ctx context.Context, t []string, o, l int) ([]*Molecule, error) {
	return nil, nil
}
func (m *mockMoleculeRepository) FindByMolecularWeightRange(ctx context.Context, min, max float64, o, l int) ([]*Molecule, error) {
	return nil, nil
}
func (m *mockMoleculeRepository) FindWithFingerprint(ctx context.Context, t FingerprintType, o, l int) ([]*Molecule, error) {
	return nil, nil
}
func (m *mockMoleculeRepository) FindWithoutFingerprint(ctx context.Context, t FingerprintType, o, l int) ([]*Molecule, error) {
	return nil, nil
}

type mockFingerprintCalculator struct {
	CalculateFunc      func(ctx context.Context, smiles string, fpType FingerprintType, opts *FingerprintCalcOptions) (*Fingerprint, error)
	StandardizeFunc    func(ctx context.Context, smiles string) (*StructureIdentifiers, error)
	BatchCalculateFunc func(ctx context.Context, smilesSlice []string, fpType FingerprintType, opts *FingerprintCalcOptions) ([]*Fingerprint, error)
}

func (m *mockFingerprintCalculator) Standardize(ctx context.Context, smiles string) (*StructureIdentifiers, error) {
	if m.StandardizeFunc != nil {
		return m.StandardizeFunc(ctx, smiles)
	}
	return &StructureIdentifiers{
		CanonicalSMILES: smiles,
		InChIKey:        "AAAAAAAAAAAAAA-BBBBBBBBBB-C",
		Formula:         "C6H6",
		Weight:          78.11,
		InChI:           "InChI=1S/C6H6/h1-6H",
	}, nil
}
func (m *mockFingerprintCalculator) Calculate(ctx context.Context, smiles string, fpType FingerprintType, opts *FingerprintCalcOptions) (*Fingerprint, error) {
	if m.CalculateFunc != nil {
		return m.CalculateFunc(ctx, smiles, fpType, opts)
	}
	return nil, nil
}
func (m *mockFingerprintCalculator) BatchCalculate(ctx context.Context, smilesSlice []string, fpType FingerprintType, opts *FingerprintCalcOptions) ([]*Fingerprint, error) {
	if m.BatchCalculateFunc != nil {
		return m.BatchCalculateFunc(ctx, smilesSlice, fpType, opts)
	}
	res := make([]*Fingerprint, len(smilesSlice))
	for i := range smilesSlice {
		res[i], _ = NewBitFingerprint(fpType, make([]byte, 256), 2048, 2)
	}
	return res, nil
}
func (m *mockFingerprintCalculator) SupportedTypes() []FingerprintType { return nil }

type mockSimilarityEngine struct {
	SearchSimilarFunc     func(ctx context.Context, target *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error)
	ComputeSimilarityFunc func(fp1, fp2 *Fingerprint, metric SimilarityMetric) (float64, error)
}

func (m *mockSimilarityEngine) SearchSimilar(ctx context.Context, target *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error) {
	if m.SearchSimilarFunc != nil {
		return m.SearchSimilarFunc(ctx, target, metric, threshold, limit)
	}
	return nil, nil
}
func (m *mockSimilarityEngine) ComputeSimilarity(fp1, fp2 *Fingerprint, metric SimilarityMetric) (float64, error) {
	if m.ComputeSimilarityFunc != nil {
		return m.ComputeSimilarityFunc(fp1, fp2, metric)
	}
	return 0, nil
}
func (m *mockSimilarityEngine) BatchComputeSimilarity(target *Fingerprint, candidates []*Fingerprint, metric SimilarityMetric) ([]float64, error) {
	return nil, nil
}
func (m *mockSimilarityEngine) RankBySimilarity(ctx context.Context, target *Fingerprint, candidateIDs []string, metric SimilarityMetric) ([]*SimilarityResult, error) {
	return nil, nil
}

func setupService(t *testing.T) (*MoleculeService, *mockMoleculeRepository, *mockFingerprintCalculator, *mockSimilarityEngine) {
	repo := &mockMoleculeRepository{}
	fpCalc := &mockFingerprintCalculator{}
	simEngine := &mockSimilarityEngine{}
	logger := logging.NewNopLogger()
	svc, _ := NewMoleculeService(repo, fpCalc, simEngine, logger)
	return svc, repo, fpCalc, simEngine
}

func TestMoleculeService_RegisterMolecule(t *testing.T) {
	svc, repo, fpCalc, _ := setupService(t)
	ctx := context.Background()

	t.Run("success_new_molecule", func(t *testing.T) {
		repo.ExistsByInChIKeyFunc = func(ctx context.Context, inchiKey string) (bool, error) { return false, nil }
		fpCalc.CalculateFunc = func(ctx context.Context, smiles string, fpType FingerprintType, opts *FingerprintCalcOptions) (*Fingerprint, error) {
			return NewBitFingerprint(fpType, make([]byte, 256), 2048, 2)
		}

		mol, err := svc.RegisterMolecule(ctx, "c1ccccc1", SourcePatent, "REF123")
		assert.NoError(t, err)
		assert.NotNil(t, mol)
		assert.True(t, mol.IsActive())
		assert.Equal(t, 1, repo.SaveCallCount)
	})

	t.Run("idempotent_existing_molecule", func(t *testing.T) {
		repo.SaveCallCount = 0
		repo.ExistsByInChIKeyFunc = func(ctx context.Context, inchiKey string) (bool, error) { return true, nil }
		repo.FindByInChIKeyFunc = func(ctx context.Context, inchiKey string) (*Molecule, error) {
			return NewMolecule("c1ccccc1", SourcePatent, "REF123")
		}

		_, err := svc.RegisterMolecule(ctx, "c1ccccc1", SourcePatent, "REF123")
		assert.NoError(t, err)
		assert.Equal(t, 0, repo.SaveCallCount)
	})
}

func TestMoleculeService_BatchRegisterMolecules(t *testing.T) {
	svc, repo, _, _ := setupService(t)
	ctx := context.Background()

	reqs := []MoleculeRegistrationRequest{
		{SMILES: "C", Source: SourceManual},
		{SMILES: "CC", Source: SourceManual},
		{SMILES: "INVALID!!!", Source: SourceManual},
	}

	result, err := svc.BatchRegisterMolecules(ctx, reqs)
	assert.NoError(t, err)
	assert.Len(t, result.Succeeded, 2)
	assert.Len(t, result.Failed, 1)
	assert.Equal(t, 3, result.TotalProcessed)
	assert.Equal(t, 1, repo.BatchSaveCallCount)
}

func TestMoleculeService_FindSimilarMolecules(t *testing.T) {
	svc, _, fpCalc, simEngine := setupService(t)
	ctx := context.Background()

	fpCalc.CalculateFunc = func(ctx context.Context, smiles string, fpType FingerprintType, opts *FingerprintCalcOptions) (*Fingerprint, error) {
		return NewBitFingerprint(fpType, make([]byte, 256), 2048, 2)
	}
	simEngine.SearchSimilarFunc = func(ctx context.Context, target *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error) {
		return []*SimilarityResult{{MoleculeID: "mol1", Score: 0.9}}, nil
	}

	results, err := svc.FindSimilarMolecules(ctx, "c1ccccc1", FingerprintMorgan, 0.8, 10)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "mol1", results[0].MoleculeID)
}

//Personal.AI order the ending
