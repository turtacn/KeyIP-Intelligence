package molecule

import (
	"context"
	"testing"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// --- Mocks ---

type mockMoleculeRepository struct {
	SaveFunc               func(ctx context.Context, m *Molecule) error
	UpdateFunc             func(ctx context.Context, m *Molecule) error
	DeleteFunc             func(ctx context.Context, id string) error
	BatchSaveFunc          func(ctx context.Context, molecules []*Molecule) (int, error)
	FindByIDFunc           func(ctx context.Context, id string) (*Molecule, error)
	FindByInChIKeyFunc     func(ctx context.Context, inchiKey string) (*Molecule, error)
	ExistsByInChIKeyFunc   func(ctx context.Context, inchiKey string) (bool, error)
	SearchFunc             func(ctx context.Context, query *MoleculeQuery) (*MoleculeSearchResult, error)

	// Track calls
	SaveCalls              int
	UpdateCalls            int
	DeleteCalls            int
	BatchSaveCalls         int
	FindByIDCalls          int
	FindByInChIKeyCalls    int
	ExistsByInChIKeyCalls  int
	SearchCalls            int
}

func (m *mockMoleculeRepository) Save(ctx context.Context, molecule *Molecule) error {
	m.SaveCalls++
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, molecule)
	}
	return nil
}

func (m *mockMoleculeRepository) Update(ctx context.Context, molecule *Molecule) error {
	m.UpdateCalls++
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, molecule)
	}
	return nil
}

func (m *mockMoleculeRepository) Delete(ctx context.Context, id string) error {
	m.DeleteCalls++
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

func (m *mockMoleculeRepository) BatchSave(ctx context.Context, molecules []*Molecule) (int, error) {
	m.BatchSaveCalls++
	if m.BatchSaveFunc != nil {
		return m.BatchSaveFunc(ctx, molecules)
	}
	return len(molecules), nil
}

func (m *mockMoleculeRepository) FindByID(ctx context.Context, id string) (*Molecule, error) {
	m.FindByIDCalls++
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, id)
	}
	return nil, errors.New(errors.ErrCodeEntityNotFound, "not found")
}

func (m *mockMoleculeRepository) FindByInChIKey(ctx context.Context, inchiKey string) (*Molecule, error) {
	m.FindByInChIKeyCalls++
	if m.FindByInChIKeyFunc != nil {
		return m.FindByInChIKeyFunc(ctx, inchiKey)
	}
	return nil, errors.New(errors.ErrCodeEntityNotFound, "not found")
}

func (m *mockMoleculeRepository) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) {
	m.ExistsByInChIKeyCalls++
	if m.ExistsByInChIKeyFunc != nil {
		return m.ExistsByInChIKeyFunc(ctx, inchiKey)
	}
	return false, nil
}

func (m *mockMoleculeRepository) Search(ctx context.Context, query *MoleculeQuery) (*MoleculeSearchResult, error) {
	m.SearchCalls++
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, query)
	}
	return &MoleculeSearchResult{}, nil
}

// Implement other interface methods with panic or no-op if not used in service logic
func (m *mockMoleculeRepository) FindBySMILES(ctx context.Context, smiles string) ([]*Molecule, error) { return nil, nil }
func (m *mockMoleculeRepository) FindByIDs(ctx context.Context, ids []string) ([]*Molecule, error) { return nil, nil }
func (m *mockMoleculeRepository) Exists(ctx context.Context, id string) (bool, error) { return false, nil }
func (m *mockMoleculeRepository) Count(ctx context.Context, query *MoleculeQuery) (int64, error) { return 0, nil }
func (m *mockMoleculeRepository) FindBySource(ctx context.Context, source MoleculeSource, offset, limit int) ([]*Molecule, error) { return nil, nil }
func (m *mockMoleculeRepository) FindByStatus(ctx context.Context, status MoleculeStatus, offset, limit int) ([]*Molecule, error) { return nil, nil }
func (m *mockMoleculeRepository) FindByTags(ctx context.Context, tags []string, offset, limit int) ([]*Molecule, error) { return nil, nil }
func (m *mockMoleculeRepository) FindByMolecularWeightRange(ctx context.Context, minWeight, maxWeight float64, offset, limit int) ([]*Molecule, error) { return nil, nil }
func (m *mockMoleculeRepository) FindWithFingerprint(ctx context.Context, fpType FingerprintType, offset, limit int) ([]*Molecule, error) { return nil, nil }
func (m *mockMoleculeRepository) FindWithoutFingerprint(ctx context.Context, fpType FingerprintType, offset, limit int) ([]*Molecule, error) { return nil, nil }


type mockFingerprintCalculator struct {
	CalculateFunc      func(ctx context.Context, smiles string, fpType FingerprintType, opts *FingerprintCalcOptions) (*Fingerprint, error)
	BatchCalculateFunc func(ctx context.Context, smilesSlice []string, fpType FingerprintType, opts *FingerprintCalcOptions) ([]*Fingerprint, error)
	StandardizeFunc    func(ctx context.Context, smiles string) (string, string, string, string, float64, error)

	CalculateCalls      int
	BatchCalculateCalls int
	StandardizeCalls    int
}

func (m *mockFingerprintCalculator) Calculate(ctx context.Context, smiles string, fpType FingerprintType, opts *FingerprintCalcOptions) (*Fingerprint, error) {
	m.CalculateCalls++
	if m.CalculateFunc != nil {
		return m.CalculateFunc(ctx, smiles, fpType, opts)
	}
	return &Fingerprint{Type: fpType}, nil
}

func (m *mockFingerprintCalculator) BatchCalculate(ctx context.Context, smilesSlice []string, fpType FingerprintType, opts *FingerprintCalcOptions) ([]*Fingerprint, error) {
	m.BatchCalculateCalls++
	if m.BatchCalculateFunc != nil {
		return m.BatchCalculateFunc(ctx, smilesSlice, fpType, opts)
	}
	res := make([]*Fingerprint, len(smilesSlice))
	for i := range smilesSlice {
		res[i] = &Fingerprint{Type: fpType}
	}
	return res, nil
}

func (m *mockFingerprintCalculator) Standardize(ctx context.Context, smiles string) (string, string, string, string, float64, error) {
	m.StandardizeCalls++
	if m.StandardizeFunc != nil {
		return m.StandardizeFunc(ctx, smiles)
	}
	// Default dummy with valid InChIKey format
	return smiles, "InChI=...", "AAAAAAAAAAAAAA-BBBBBBBBBB-C", "C", 100.0, nil
}

func (m *mockFingerprintCalculator) SupportedTypes() []FingerprintType { return nil }

type mockSimilarityEngine struct {
	SearchSimilarFunc func(ctx context.Context, target *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error)
	ComputeSimilarityFunc func(fp1, fp2 *Fingerprint, metric SimilarityMetric) (float64, error)

	SearchSimilarCalls int
	ComputeSimilarityCalls int
}

func (m *mockSimilarityEngine) SearchSimilar(ctx context.Context, target *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error) {
	m.SearchSimilarCalls++
	if m.SearchSimilarFunc != nil {
		return m.SearchSimilarFunc(ctx, target, metric, threshold, limit)
	}
	return nil, nil
}

func (m *mockSimilarityEngine) ComputeSimilarity(fp1, fp2 *Fingerprint, metric SimilarityMetric) (float64, error) {
	m.ComputeSimilarityCalls++
	if m.ComputeSimilarityFunc != nil {
		return m.ComputeSimilarityFunc(fp1, fp2, metric)
	}
	return 0.0, nil
}

func (m *mockSimilarityEngine) BatchComputeSimilarity(target *Fingerprint, candidates []*Fingerprint, metric SimilarityMetric) ([]float64, error) { return nil, nil }
func (m *mockSimilarityEngine) RankBySimilarity(ctx context.Context, target *Fingerprint, candidateIDs []string, metric SimilarityMetric) ([]*SimilarityResult, error) { return nil, nil }

type mockLogger struct {
	// Simple no-op logger or capture logs
}

func (l *mockLogger) Debug(msg string, fields ...logging.Field) {}
func (l *mockLogger) Info(msg string, fields ...logging.Field) {}
func (l *mockLogger) Warn(msg string, fields ...logging.Field) {}
func (l *mockLogger) Error(msg string, fields ...logging.Field) {}
func (l *mockLogger) Fatal(msg string, fields ...logging.Field) {}
func (l *mockLogger) With(fields ...logging.Field) logging.Logger { return l }
func (l *mockLogger) Sync() error { return nil }
func (l *mockLogger) WithContext(ctx context.Context) logging.Logger { return l }
func (l *mockLogger) WithError(err error) logging.Logger { return l }

// --- Tests ---

func TestNewMoleculeService(t *testing.T) {
	repo := &mockMoleculeRepository{}
	fpCalc := &mockFingerprintCalculator{}
	simEngine := &mockSimilarityEngine{}
	logger := &mockLogger{}

	t.Run("success", func(t *testing.T) {
		svc, err := NewMoleculeService(repo, fpCalc, simEngine, logger)
		if err != nil {
			t.Errorf("NewMoleculeService failed: %v", err)
		}
		if svc == nil {
			t.Error("NewMoleculeService returned nil")
		}
	})

	t.Run("nil dependency", func(t *testing.T) {
		_, err := NewMoleculeService(nil, fpCalc, simEngine, logger)
		if err == nil {
			t.Error("NewMoleculeService allowed nil repo")
		}
	})
}

func TestMoleculeService_RegisterMolecule(t *testing.T) {
	repo := &mockMoleculeRepository{}
	fpCalc := &mockFingerprintCalculator{}
	simEngine := &mockSimilarityEngine{}
	logger := &mockLogger{}
	svc, _ := NewMoleculeService(repo, fpCalc, simEngine, logger)

	t.Run("success_new", func(t *testing.T) {
		repo.ExistsByInChIKeyFunc = func(ctx context.Context, k string) (bool, error) { return false, nil }
		repo.SaveFunc = func(ctx context.Context, m *Molecule) error { return nil }

		mol, err := svc.RegisterMolecule(context.Background(), "c1ccccc1", SourceManual, "ref")
		if err != nil {
			t.Fatalf("RegisterMolecule failed: %v", err)
		}
		if mol.Status() != MoleculeStatusActive {
			t.Errorf("Status = %v, want Active", mol.Status())
		}
		if repo.SaveCalls != 1 {
			t.Errorf("SaveCalls = %d, want 1", repo.SaveCalls)
		}
		if fpCalc.StandardizeCalls != 1 {
			t.Errorf("StandardizeCalls = %d, want 1", fpCalc.StandardizeCalls)
		}
		// Morgan + MACCS
		if fpCalc.CalculateCalls != 2 {
			t.Errorf("CalculateCalls = %d, want 2", fpCalc.CalculateCalls)
		}
	})

	t.Run("idempotent_existing", func(t *testing.T) {
		existingMol, _ := NewMolecule("c1ccccc1", SourceManual, "ref")
		repo.ExistsByInChIKeyFunc = func(ctx context.Context, k string) (bool, error) { return true, nil }
		repo.FindByInChIKeyFunc = func(ctx context.Context, k string) (*Molecule, error) { return existingMol, nil }
		repo.SaveCalls = 0 // reset

		mol, err := svc.RegisterMolecule(context.Background(), "c1ccccc1", SourceManual, "ref")
		if err != nil {
			t.Fatalf("RegisterMolecule failed: %v", err)
		}
		if mol != existingMol {
			t.Error("Did not return existing molecule")
		}
		if repo.SaveCalls != 0 {
			t.Error("Save called for existing molecule")
		}
	})
}

func TestMoleculeService_BatchRegisterMolecules(t *testing.T) {
	repo := &mockMoleculeRepository{}
	fpCalc := &mockFingerprintCalculator{}
	simEngine := &mockSimilarityEngine{}
	logger := &mockLogger{}
	svc, _ := NewMoleculeService(repo, fpCalc, simEngine, logger)

	reqs := []MoleculeRegistrationRequest{
		{SMILES: "C"},
		{SMILES: "CC"},
	}

	t.Run("success_batch", func(t *testing.T) {
		repo.ExistsByInChIKeyFunc = func(ctx context.Context, k string) (bool, error) { return false, nil }

		res, err := svc.BatchRegisterMolecules(context.Background(), reqs)
		if err != nil {
			t.Fatalf("BatchRegisterMolecules failed: %v", err)
		}
		if len(res.Succeeded) != 2 {
			t.Errorf("Succeeded count = %d, want 2", len(res.Succeeded))
		}
		if repo.BatchSaveCalls != 1 {
			t.Errorf("BatchSaveCalls = %d, want 1", repo.BatchSaveCalls)
		}
		if fpCalc.BatchCalculateCalls != 2 { // Morgan + MACCS
			t.Errorf("BatchCalculateCalls = %d, want 2", fpCalc.BatchCalculateCalls)
		}
	})
}

func TestMoleculeService_FindSimilarMolecules(t *testing.T) {
	repo := &mockMoleculeRepository{}
	fpCalc := &mockFingerprintCalculator{}
	simEngine := &mockSimilarityEngine{}
	logger := &mockLogger{}
	svc, _ := NewMoleculeService(repo, fpCalc, simEngine, logger)

	t.Run("success", func(t *testing.T) {
		simEngine.SearchSimilarFunc = func(ctx context.Context, t *Fingerprint, m SimilarityMetric, th float64, l int) ([]*SimilarityResult, error) {
			return []*SimilarityResult{{Score: 0.9}}, nil
		}

		res, err := svc.FindSimilarMolecules(context.Background(), "C", FingerprintMorgan, 0.7, 10)
		if err != nil {
			t.Fatalf("FindSimilarMolecules failed: %v", err)
		}
		if len(res) != 1 {
			t.Errorf("Result count = %d, want 1", len(res))
		}
		if fpCalc.CalculateCalls != 1 {
			t.Errorf("CalculateCalls = %d, want 1", fpCalc.CalculateCalls)
		}
	})
}

func TestMoleculeService_CompareMolecules(t *testing.T) {
	repo := &mockMoleculeRepository{}
	fpCalc := &mockFingerprintCalculator{}
	simEngine := &mockSimilarityEngine{}
	logger := &mockLogger{}
	svc, _ := NewMoleculeService(repo, fpCalc, simEngine, logger)

	t.Run("success", func(t *testing.T) {
		fpCalc.StandardizeFunc = func(ctx context.Context, s string) (string, string, string, string, float64, error) {
			return s, "InChI="+s, "KEY-"+s, "C", 10.0, nil
		}
		simEngine.ComputeSimilarityFunc = func(fp1, fp2 *Fingerprint, m SimilarityMetric) (float64, error) {
			return 0.5, nil
		}

		res, err := svc.CompareMolecules(context.Background(), "C", "CC", []FingerprintType{FingerprintMorgan})
		if err != nil {
			t.Fatalf("CompareMolecules failed: %v", err)
		}
		if res.FusedScore != 0.5 {
			t.Errorf("FusedScore = %f, want 0.5", res.FusedScore)
		}
		if res.IsStructurallyIdentical { // "KEY-C" != "KEY-CC"
			t.Error("IsStructurallyIdentical should be false")
		}
	})
}

func TestMoleculeService_CalculateFingerprints(t *testing.T) {
	repo := &mockMoleculeRepository{}
	fpCalc := &mockFingerprintCalculator{}
	simEngine := &mockSimilarityEngine{}
	logger := &mockLogger{}
	svc, _ := NewMoleculeService(repo, fpCalc, simEngine, logger)

	t.Run("success", func(t *testing.T) {
		mol, _ := NewMolecule("C", SourceManual, "ref")
		repo.FindByIDFunc = func(ctx context.Context, id string) (*Molecule, error) { return mol, nil }

		err := svc.CalculateFingerprints(context.Background(), "id", []FingerprintType{FingerprintMorgan})
		if err != nil {
			t.Fatalf("CalculateFingerprints failed: %v", err)
		}
		if repo.UpdateCalls != 1 {
			t.Errorf("UpdateCalls = %d, want 1", repo.UpdateCalls)
		}
		if !mol.HasFingerprint(FingerprintMorgan) {
			t.Error("Fingerprint not added to molecule")
		}
	})
}

//Personal.AI order the ending
