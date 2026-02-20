// Package molecule_test provides unit tests for the molecule domain service.
package molecule_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	mtypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock Repository
// ─────────────────────────────────────────────────────────────────────────────

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) Save(ctx context.Context, mol *molecule.Molecule) error {
	args := m.Called(ctx, mol)
	return args.Error(0)
}

func (m *mockRepository) FindByID(ctx context.Context, id common.ID) (*molecule.Molecule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.Molecule), args.Error(1)
}

func (m *mockRepository) FindBySMILES(ctx context.Context, smiles string) (*molecule.Molecule, error) {
	args := m.Called(ctx, smiles)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.Molecule), args.Error(1)
}

func (m *mockRepository) FindByInChIKey(ctx context.Context, inchiKey string) (*molecule.Molecule, error) {
	args := m.Called(ctx, inchiKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.Molecule), args.Error(1)
}

func (m *mockRepository) Search(ctx context.Context, req mtypes.MoleculeSearchRequest) (*mtypes.MoleculeSearchResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mtypes.MoleculeSearchResponse), args.Error(1)
}

func (m *mockRepository) FindSimilar(ctx context.Context, fp *molecule.Fingerprint, fpType mtypes.FingerprintType, threshold float64, maxResults int) ([]*molecule.Molecule, error) {
	args := m.Called(ctx, fp, fpType, threshold, maxResults)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*molecule.Molecule), args.Error(1)
}

func (m *mockRepository) SubstructureSearch(ctx context.Context, smarts string, maxResults int) ([]*molecule.Molecule, error) {
	args := m.Called(ctx, smarts, maxResults)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*molecule.Molecule), args.Error(1)
}

func (m *mockRepository) Update(ctx context.Context, mol *molecule.Molecule) error {
	args := m.Called(ctx, mol)
	return args.Error(0)
}

func (m *mockRepository) Delete(ctx context.Context, id common.ID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockRepository) FindByPatentID(ctx context.Context, patentID common.ID) ([]*molecule.Molecule, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*molecule.Molecule), args.Error(1)
}

func (m *mockRepository) BatchSave(ctx context.Context, mols []*molecule.Molecule) error {
	args := m.Called(ctx, mols)
	return args.Error(0)
}

func (m *mockRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

// Mock logger
type mockLogger struct{}

func (mockLogger) Debug(msg string, fields ...logging.Field) {}
func (mockLogger) Info(msg string, fields ...logging.Field)  {}
func (mockLogger) Warn(msg string, fields ...logging.Field)  {}
func (mockLogger) Error(msg string, fields ...logging.Field) {}
func (mockLogger) Fatal(msg string, fields ...logging.Field) {}
func (l mockLogger) With(fields ...logging.Field) logging.Logger {
	return l
}
func (l mockLogger) Named(name string) logging.Logger {
	return l
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestCreateMolecule_NewMolecule(t *testing.T) {
	t.Parallel()

	mockRepo := new(mockRepository)
	svc := molecule.NewService(mockRepo, mockLogger{})
	ctx := context.Background()

	smiles := "c1ccccc1"

	// Mock: molecule does not exist
	mockRepo.On("FindBySMILES", ctx, smiles).Return(nil, errors.NotFound("not found"))

	// Mock: save succeeds
	mockRepo.On("Save", ctx, mock.AnythingOfType("*molecule.Molecule")).Return(nil)

	mol, err := svc.CreateMolecule(ctx, smiles, mtypes.TypeSmallMolecule)
	require.NoError(t, err)
	require.NotNil(t, mol)

	assert.Equal(t, smiles, mol.SMILES)
	mockRepo.AssertExpectations(t)
}

func TestCreateMolecule_DuplicateReturnsExisting(t *testing.T) {
	t.Parallel()

	mockRepo := new(mockRepository)
	svc := molecule.NewService(mockRepo, mockLogger{})
	ctx := context.Background()

	smiles := "c1ccccc1"

	existingMol, err := molecule.NewMolecule(smiles, mtypes.TypeSmallMolecule)
	require.NoError(t, err)

	// Mock: molecule already exists
	mockRepo.On("FindBySMILES", ctx, smiles).Return(existingMol, nil)

	mol, err := svc.CreateMolecule(ctx, smiles, mtypes.TypeSmallMolecule)
	require.NoError(t, err)
	assert.Equal(t, existingMol.ID, mol.ID)

	// Save should NOT be called
	mockRepo.AssertNotCalled(t, "Save")
}

func TestFindSimilarMolecules(t *testing.T) {
	t.Parallel()

	mockRepo := new(mockRepository)
	svc := molecule.NewService(mockRepo, mockLogger{})
	ctx := context.Background()

	smiles := "c1ccccc1"

	similarMol, err := molecule.NewMolecule("Cc1ccccc1", mtypes.TypeSmallMolecule)
	require.NoError(t, err)

	mockRepo.On("FindSimilar", ctx, mock.AnythingOfType("*molecule.Fingerprint"),
		mtypes.FPMorgan, 0.8, 10).Return([]*molecule.Molecule{similarMol}, nil)

	results, err := svc.FindSimilarMolecules(ctx, smiles, 0.8, mtypes.FPMorgan, 10)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, similarMol.ID, results[0].ID)

	mockRepo.AssertExpectations(t)
}

func TestBatchImportMolecules_SuccessCount(t *testing.T) {
	t.Parallel()

	mockRepo := new(mockRepository)
	svc := molecule.NewService(mockRepo, mockLogger{})
	ctx := context.Background()

	smilesLines := []string{"c1ccccc1", "CCO", "invalid!!!"}

	// Mock: none exist
	mockRepo.On("FindBySMILES", ctx, "c1ccccc1").Return(nil, errors.NotFound("not found"))
	mockRepo.On("FindBySMILES", ctx, "CCO").Return(nil, errors.NotFound("not found"))
	mockRepo.On("FindBySMILES", ctx, "invalid!!!").Return(nil, errors.NotFound("not found"))

	// Mock: batch save succeeds
	mockRepo.On("BatchSave", ctx, mock.AnythingOfType("[]*molecule.Molecule")).Return(nil)

	count, err := svc.BatchImportMolecules(ctx, smilesLines, mtypes.TypeSmallMolecule)
	require.NoError(t, err)

	// Should import 2 valid molecules, skip 1 invalid
	assert.Equal(t, 2, count)

	mockRepo.AssertExpectations(t)
}

func TestBatchImportMolecules_SkipsInvalid(t *testing.T) {
	t.Parallel()

	mockRepo := new(mockRepository)
	svc := molecule.NewService(mockRepo, mockLogger{})
	ctx := context.Background()

	// All invalid SMILES
	smilesLines := []string{"!!!", "$$$", "^^^"}

	// Mock: none exist
	mockRepo.On("FindBySMILES", ctx, "!!!").Return(nil, errors.NotFound("not found"))
	mockRepo.On("FindBySMILES", ctx, "$$$").Return(nil, errors.NotFound("not found"))
	mockRepo.On("FindBySMILES", ctx, "^^^").Return(nil, errors.NotFound("not found"))

	count, err := svc.BatchImportMolecules(ctx, smilesLines, mtypes.TypeSmallMolecule)
	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "no valid molecules")

	mockRepo.AssertNotCalled(t, "BatchSave")
}

