package molecule

import (
	"context"
	"errors"
	"testing"

	domainMol "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// --- Mock implementations ---

type mockMoleculeRepository struct {
	saveFn                   func(ctx context.Context, molecule *domainMol.Molecule) error
	updateFn                 func(ctx context.Context, molecule *domainMol.Molecule) error
	deleteFn                 func(ctx context.Context, id string) error
	batchSaveFn              func(ctx context.Context, molecules []*domainMol.Molecule) (int, error)
	findByIDFn               func(ctx context.Context, id string) (*domainMol.Molecule, error)
	findByInChIKeyFn         func(ctx context.Context, inchiKey string) (*domainMol.Molecule, error)
	findBySMILESFn           func(ctx context.Context, smiles string) ([]*domainMol.Molecule, error)
	findByIDsFn              func(ctx context.Context, ids []string) ([]*domainMol.Molecule, error)
	existsFn                 func(ctx context.Context, id string) (bool, error)
	existsByInChIKeyFn       func(ctx context.Context, inchiKey string) (bool, error)
	searchFn                 func(ctx context.Context, query *domainMol.MoleculeQuery) (*domainMol.MoleculeSearchResult, error)
	countFn                  func(ctx context.Context, query *domainMol.MoleculeQuery) (int64, error)
	findBySourceFn           func(ctx context.Context, source domainMol.MoleculeSource, offset, limit int) ([]*domainMol.Molecule, error)
	findByStatusFn           func(ctx context.Context, status domainMol.MoleculeStatus, offset, limit int) ([]*domainMol.Molecule, error)
	findByTagsFn             func(ctx context.Context, tags []string, offset, limit int) ([]*domainMol.Molecule, error)
	findByMolecularWeightRangeFn func(ctx context.Context, minWeight, maxWeight float64, offset, limit int) ([]*domainMol.Molecule, error)
	findWithFingerprintFn    func(ctx context.Context, fpType domainMol.FingerprintType, offset, limit int) ([]*domainMol.Molecule, error)
	findWithoutFingerprintFn func(ctx context.Context, fpType domainMol.FingerprintType, offset, limit int) ([]*domainMol.Molecule, error)
}

func (m *mockMoleculeRepository) Save(ctx context.Context, molecule *domainMol.Molecule) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, molecule)
	}
	return nil
}

func (m *mockMoleculeRepository) Update(ctx context.Context, molecule *domainMol.Molecule) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, molecule)
	}
	return nil
}

func (m *mockMoleculeRepository) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockMoleculeRepository) BatchSave(ctx context.Context, molecules []*domainMol.Molecule) (int, error) {
	if m.batchSaveFn != nil {
		return m.batchSaveFn(ctx, molecules)
	}
	return 0, nil
}

func (m *mockMoleculeRepository) FindByID(ctx context.Context, id string) (*domainMol.Molecule, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, errors.New("not found")
}

func (m *mockMoleculeRepository) FindByInChIKey(ctx context.Context, inchiKey string) (*domainMol.Molecule, error) {
	if m.findByInChIKeyFn != nil {
		return m.findByInChIKeyFn(ctx, inchiKey)
	}
	return nil, nil
}

func (m *mockMoleculeRepository) FindBySMILES(ctx context.Context, smiles string) ([]*domainMol.Molecule, error) {
	if m.findBySMILESFn != nil {
		return m.findBySMILESFn(ctx, smiles)
	}
	return nil, nil
}

func (m *mockMoleculeRepository) FindByIDs(ctx context.Context, ids []string) ([]*domainMol.Molecule, error) {
	if m.findByIDsFn != nil {
		return m.findByIDsFn(ctx, ids)
	}
	return nil, nil
}

func (m *mockMoleculeRepository) Exists(ctx context.Context, id string) (bool, error) {
	if m.existsFn != nil {
		return m.existsFn(ctx, id)
	}
	return false, nil
}

func (m *mockMoleculeRepository) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) {
	if m.existsByInChIKeyFn != nil {
		return m.existsByInChIKeyFn(ctx, inchiKey)
	}
	return false, nil
}

func (m *mockMoleculeRepository) Search(ctx context.Context, query *domainMol.MoleculeQuery) (*domainMol.MoleculeSearchResult, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, query)
	}
	return &domainMol.MoleculeSearchResult{}, nil
}

func (m *mockMoleculeRepository) Count(ctx context.Context, query *domainMol.MoleculeQuery) (int64, error) {
	if m.countFn != nil {
		return m.countFn(ctx, query)
	}
	return 0, nil
}

func (m *mockMoleculeRepository) FindBySource(ctx context.Context, source domainMol.MoleculeSource, offset, limit int) ([]*domainMol.Molecule, error) {
	if m.findBySourceFn != nil {
		return m.findBySourceFn(ctx, source, offset, limit)
	}
	return nil, nil
}

func (m *mockMoleculeRepository) FindByStatus(ctx context.Context, status domainMol.MoleculeStatus, offset, limit int) ([]*domainMol.Molecule, error) {
	if m.findByStatusFn != nil {
		return m.findByStatusFn(ctx, status, offset, limit)
	}
	return nil, nil
}

func (m *mockMoleculeRepository) FindByTags(ctx context.Context, tags []string, offset, limit int) ([]*domainMol.Molecule, error) {
	if m.findByTagsFn != nil {
		return m.findByTagsFn(ctx, tags, offset, limit)
	}
	return nil, nil
}

func (m *mockMoleculeRepository) FindByMolecularWeightRange(ctx context.Context, minWeight, maxWeight float64, offset, limit int) ([]*domainMol.Molecule, error) {
	if m.findByMolecularWeightRangeFn != nil {
		return m.findByMolecularWeightRangeFn(ctx, minWeight, maxWeight, offset, limit)
	}
	return nil, nil
}

func (m *mockMoleculeRepository) FindWithFingerprint(ctx context.Context, fpType domainMol.FingerprintType, offset, limit int) ([]*domainMol.Molecule, error) {
	if m.findWithFingerprintFn != nil {
		return m.findWithFingerprintFn(ctx, fpType, offset, limit)
	}
	return nil, nil
}

func (m *mockMoleculeRepository) FindWithoutFingerprint(ctx context.Context, fpType domainMol.FingerprintType, offset, limit int) ([]*domainMol.Molecule, error) {
	if m.findWithoutFingerprintFn != nil {
		return m.findWithoutFingerprintFn(ctx, fpType, offset, limit)
	}
	return nil, nil
}

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...logging.Field) {}
func (m *mockLogger) Info(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Error(msg string, fields ...logging.Field) {}
func (m *mockLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *mockLogger) With(fields ...logging.Field) logging.Logger { return m }
func (m *mockLogger) WithContext(ctx context.Context) logging.Logger { return m }
func (m *mockLogger) WithError(err error) logging.Logger { return m }
func (m *mockLogger) Sync() error { return nil }

func TestService_Create_Success(t *testing.T) {
	repo := &mockMoleculeRepository{
		saveFn: func(ctx context.Context, molecule *domainMol.Molecule) error {
			return nil
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	input := &CreateInput{
		Name:   "Test Molecule",
		SMILES: "C",
	}

	mol, err := svc.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mol.Name != "Test Molecule" {
		t.Errorf("expected name 'Test Molecule', got '%s'", mol.Name)
	}
	if mol.SMILES != "C" {
		t.Errorf("expected smiles 'C', got '%s'", mol.SMILES)
	}
}

func TestService_Create_InvalidInput(t *testing.T) {
	repo := &mockMoleculeRepository{}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	input := &CreateInput{
		Name: "Test Molecule",
		// Missing SMILES
	}

	_, err := svc.Create(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing SMILES")
	}
}

func TestService_GetByID_Success(t *testing.T) {
	repo := &mockMoleculeRepository{
		findByIDFn: func(ctx context.Context, id string) (*domainMol.Molecule, error) {
			mol, _ := domainMol.NewMolecule("C", domainMol.SourceManual, "")
			return mol, nil
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	mol, err := svc.GetByID(context.Background(), "id-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mol == nil {
		t.Fatal("expected molecule, got nil")
	}
}

func TestService_GetByID_NotFound(t *testing.T) {
	repo := &mockMoleculeRepository{
		findByIDFn: func(ctx context.Context, id string) (*domainMol.Molecule, error) {
			return nil, errors.New("not found")
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	_, err := svc.GetByID(context.Background(), "id-missing")
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestService_List_Success(t *testing.T) {
	repo := &mockMoleculeRepository{
		searchFn: func(ctx context.Context, query *domainMol.MoleculeQuery) (*domainMol.MoleculeSearchResult, error) {
			mol1, _ := domainMol.NewMolecule("C", domainMol.SourceManual, "")
			mol2, _ := domainMol.NewMolecule("CC", domainMol.SourceManual, "")
			return &domainMol.MoleculeSearchResult{
				Molecules: []*domainMol.Molecule{mol1, mol2},
				Total:     2,
			}, nil
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	res, err := svc.List(context.Background(), &ListInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Molecules) != 2 {
		t.Errorf("expected 2 molecules, got %d", len(res.Molecules))
	}
	if res.Total != 2 {
		t.Errorf("expected total 2, got %d", res.Total)
	}
}

func TestService_Update_Success(t *testing.T) {
	mol, _ := domainMol.NewMolecule("C", domainMol.SourceManual, "")
	repo := &mockMoleculeRepository{
		findByIDFn: func(ctx context.Context, id string) (*domainMol.Molecule, error) {
			return mol, nil
		},
		updateFn: func(ctx context.Context, m *domainMol.Molecule) error {
			if m.Name != "Updated Name" {
				t.Errorf("expected updated name, got %s", m.Name)
			}
			return nil
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	newName := "Updated Name"
	updated, err := svc.Update(context.Background(), &UpdateInput{
		ID:   "id-1",
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Errorf("expected updated name 'Updated Name', got '%s'", updated.Name)
	}
}

func TestService_Delete_Success(t *testing.T) {
	repo := &mockMoleculeRepository{
		deleteFn: func(ctx context.Context, id string) error {
			return nil
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	err := svc.Delete(context.Background(), "id-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

//Personal.AI order the ending
