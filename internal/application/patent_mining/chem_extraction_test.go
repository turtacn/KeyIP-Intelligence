// Phase 10 - File 215 of 349
// Phase: 应用层 - 业务服务
// SubModule: patent_mining
// File: internal/application/patent_mining/chem_extraction_test.go

package patent_mining

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	storageminio "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/storage/minio"
	chemextractor "github.com/turtacn/KeyIP-Intelligence/internal/intelligence/chem_extractor"
	"github.com/turtacn/KeyIP-Intelligence/internal/testutil"
	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// MockExtractor
type MockExtractor struct {
	ExtractFunc  func(ctx context.Context, text string) (*chemextractor.ExtractionResult, error)
	ResolveFunc  func(ctx context.Context, entity *chemextractor.RawChemicalEntity) (*chemextractor.ResolvedChemicalEntity, error)
}

func (m *MockExtractor) Extract(ctx context.Context, text string) (*chemextractor.ExtractionResult, error) {
	if m.ExtractFunc != nil {
		return m.ExtractFunc(ctx, text)
	}
	return &chemextractor.ExtractionResult{}, nil
}

func (m *MockExtractor) Resolve(ctx context.Context, entity *chemextractor.RawChemicalEntity) (*chemextractor.ResolvedChemicalEntity, error) {
	if m.ResolveFunc != nil {
		return m.ResolveFunc(ctx, entity)
	}
	return &chemextractor.ResolvedChemicalEntity{}, nil
}

func (m *MockExtractor) ExtractBatch(ctx context.Context, texts []string) ([]*chemextractor.ExtractionResult, error) { return nil, nil }
func (m *MockExtractor) ExtractFromClaim(ctx context.Context, claim *chemextractor.ParsedClaim) (*chemextractor.ClaimExtractionResult, error) { return nil, nil }
func (m *MockExtractor) LinkToMolecule(ctx context.Context, entity *chemextractor.ResolvedChemicalEntity) (*chemextractor.MoleculeLink, error) { return nil, nil }

// MockMoleculeService
type MockMoleculeService struct {
	CreateFromSMILESFunc func(ctx context.Context, smiles string, metadata map[string]string) (*molecule.Molecule, error)
}

func (m *MockMoleculeService) CreateFromSMILES(ctx context.Context, smiles string, metadata map[string]string) (*molecule.Molecule, error) {
	if m.CreateFromSMILESFunc != nil {
		return m.CreateFromSMILESFunc(ctx, smiles, metadata)
	}
	return &molecule.Molecule{ID: googleUUID("1")}, nil
}

func (m *MockMoleculeService) RegisterMolecule(ctx context.Context, smiles string, source molecule.MoleculeSource, sourceRef string) (*molecule.Molecule, error) { return nil, nil }
func (m *MockMoleculeService) BatchRegisterMolecules(ctx context.Context, requests []molecule.MoleculeRegistrationRequest) (*molecule.BatchRegistrationResult, error) { return nil, nil }
func (m *MockMoleculeService) GetMolecule(ctx context.Context, id string) (*molecule.Molecule, error) { return nil, nil }
func (m *MockMoleculeService) GetMoleculeByInChIKey(ctx context.Context, inchiKey string) (*molecule.Molecule, error) { return nil, nil }
func (m *MockMoleculeService) SearchMolecules(ctx context.Context, query *molecule.MoleculeQuery) (*molecule.MoleculeSearchResult, error) { return nil, nil }
func (m *MockMoleculeService) CalculateFingerprints(ctx context.Context, moleculeID string, fpTypes []molecule.FingerprintType) error { return nil }
func (m *MockMoleculeService) FindSimilarMolecules(ctx context.Context, targetSMILES string, fpType molecule.FingerprintType, threshold float64, limit int) ([]*molecule.SimilarityResult, error) { return nil, nil }
func (m *MockMoleculeService) CompareMolecules(ctx context.Context, smiles1, smiles2 string, fpTypes []molecule.FingerprintType) (*molecule.MoleculeComparisonResult, error) { return nil, nil }
func (m *MockMoleculeService) ArchiveMolecule(ctx context.Context, id string) error { return nil }
func (m *MockMoleculeService) DeleteMolecule(ctx context.Context, id string) error { return nil }
func (m *MockMoleculeService) AddMoleculeProperties(ctx context.Context, moleculeID string, properties []*molecule.MolecularProperty) error { return nil }
func (m *MockMoleculeService) TagMolecule(ctx context.Context, moleculeID string, tags []string) error { return nil }
func (m *MockMoleculeService) Canonicalize(ctx context.Context, smiles string) (string, string, error) { return "", "", nil }
func (m *MockMoleculeService) CanonicalizeFromInChI(ctx context.Context, inchi string) (string, string, error) { return "", "", nil }

// MockMoleculeRepo
type MockMoleculeRepo struct {
	FindByInChIKeyFunc func(ctx context.Context, inchiKey string) (*molecule.Molecule, error)
}

func (m *MockMoleculeRepo) FindByInChIKey(ctx context.Context, inchiKey string) (*molecule.Molecule, error) {
	if m.FindByInChIKeyFunc != nil {
		return m.FindByInChIKeyFunc(ctx, inchiKey)
	}
	return nil, apperrors.New(apperrors.ErrCodeNotFound, "not found")
}

func (m *MockMoleculeRepo) Save(ctx context.Context, molecule *molecule.Molecule) error { return nil }
func (m *MockMoleculeRepo) Update(ctx context.Context, molecule *molecule.Molecule) error { return nil }
func (m *MockMoleculeRepo) Delete(ctx context.Context, id string) error { return nil }
func (m *MockMoleculeRepo) BatchSave(ctx context.Context, molecules []*molecule.Molecule) (int, error) { return 0, nil }
func (m *MockMoleculeRepo) FindByID(ctx context.Context, id string) (*molecule.Molecule, error) { return nil, nil }
func (m *MockMoleculeRepo) FindBySMILES(ctx context.Context, smiles string) ([]*molecule.Molecule, error) { return nil, nil }
func (m *MockMoleculeRepo) FindByIDs(ctx context.Context, ids []string) ([]*molecule.Molecule, error) { return nil, nil }
func (m *MockMoleculeRepo) Exists(ctx context.Context, id string) (bool, error) { return false, nil }
func (m *MockMoleculeRepo) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) { return false, nil }
func (m *MockMoleculeRepo) Search(ctx context.Context, query *molecule.MoleculeQuery) (*molecule.MoleculeSearchResult, error) { return nil, nil }
func (m *MockMoleculeRepo) Count(ctx context.Context, query *molecule.MoleculeQuery) (int64, error) { return 0, nil }
func (m *MockMoleculeRepo) FindBySource(ctx context.Context, source molecule.MoleculeSource, offset, limit int) ([]*molecule.Molecule, error) { return nil, nil }
func (m *MockMoleculeRepo) FindByStatus(ctx context.Context, status molecule.MoleculeStatus, offset, limit int) ([]*molecule.Molecule, error) { return nil, nil }
func (m *MockMoleculeRepo) FindByTags(ctx context.Context, tags []string, offset, limit int) ([]*molecule.Molecule, error) { return nil, nil }
func (m *MockMoleculeRepo) FindByMolecularWeightRange(ctx context.Context, minWeight, maxWeight float64, offset, limit int) ([]*molecule.Molecule, error) { return nil, nil }
func (m *MockMoleculeRepo) FindWithFingerprint(ctx context.Context, fpType molecule.FingerprintType, offset, limit int) ([]*molecule.Molecule, error) { return nil, nil }
func (m *MockMoleculeRepo) FindWithoutFingerprint(ctx context.Context, fpType molecule.FingerprintType, offset, limit int) ([]*molecule.Molecule, error) { return nil, nil }

// MockPatentRepo
type MockPatentRepo struct {
	testutil.BasePatentRepoMock
	AssociateMoleculeFunc func(ctx context.Context, patentID string, moleculeID string) error
}

func (m *MockPatentRepo) AssociateMolecule(ctx context.Context, patentID string, moleculeID string) error {
	if m.AssociateMoleculeFunc != nil {
		return m.AssociateMoleculeFunc(ctx, patentID, moleculeID)
	}
	return nil
}

func (m *MockPatentRepo) ListByPortfolio(ctx context.Context, portfolioID string) ([]*patent.Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*patent.Patent, error) { return nil, nil }
func (m *MockPatentRepo) WithTx(ctx context.Context, fn func(patent.PatentRepository) error) error { return nil }

// MockStorage
type MockStorage struct {
	GetFunc func(ctx context.Context, path string) ([]byte, error)
}

func (m *MockStorage) Get(ctx context.Context, path string) ([]byte, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, path)
	}
	return nil, nil
}

func (m *MockStorage) Upload(ctx context.Context, req *storageminio.UploadRequest) (*storageminio.UploadResult, error) { return nil, nil }
func (m *MockStorage) UploadStream(ctx context.Context, req *storageminio.StreamUploadRequest) (*storageminio.UploadResult, error) { return nil, nil }
func (m *MockStorage) Download(ctx context.Context, bucket, objectKey string) (*storageminio.DownloadResult, error) { return nil, nil }
func (m *MockStorage) DownloadToWriter(ctx context.Context, bucket, objectKey string, writer io.Writer) error { return nil }
func (m *MockStorage) Delete(ctx context.Context, bucket, objectKey string) error { return nil }
func (m *MockStorage) DeleteBatch(ctx context.Context, bucket string, objectKeys []string) ([]storageminio.DeleteError, error) { return nil, nil }
func (m *MockStorage) Exists(ctx context.Context, bucket, objectKey string) (bool, error) { return false, nil }
func (m *MockStorage) GetMetadata(ctx context.Context, bucket, objectKey string) (*storageminio.ObjectMetadata, error) { return nil, nil }
func (m *MockStorage) List(ctx context.Context, bucket, prefix string, opts *storageminio.ListOptions) (*storageminio.ListResult, error) { return nil, nil }
func (m *MockStorage) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error { return nil }
func (m *MockStorage) Move(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error { return nil }
func (m *MockStorage) GetPresignedDownloadURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) { return "", nil }
func (m *MockStorage) GetPresignedUploadURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) { return "", nil }
func (m *MockStorage) SetTags(ctx context.Context, bucket, objectKey string, tags map[string]string) error { return nil }
func (m *MockStorage) GetTags(ctx context.Context, bucket, objectKey string) (map[string]string, error) { return nil, nil }

// MockLogger
type MockLogger struct{}
func (m *MockLogger) Info(msg string, fields ...logging.Field) {}
func (m *MockLogger) Warn(msg string, fields ...logging.Field) {}
func (m *MockLogger) Error(msg string, fields ...logging.Field) {}
func (m *MockLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *MockLogger) Debug(msg string, fields ...logging.Field) {}
func (m *MockLogger) With(fields ...logging.Field) logging.Logger { return m }
func (m *MockLogger) WithContext(ctx context.Context) logging.Logger { return m }
func (m *MockLogger) WithError(err error) logging.Logger { return m }
func (m *MockLogger) Sync() error { return nil }

// Helper
func googleUUID(s string) uuid.UUID {
    return uuid.New() // Just random for test
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestExtractFromDocument_Success(t *testing.T) {
	// Setup Mocks
	extractor := &MockExtractor{
		ExtractFunc: func(ctx context.Context, text string) (*chemextractor.ExtractionResult, error) {
			return &chemextractor.ExtractionResult{
				Entities: []*chemextractor.RawChemicalEntity{
					{EntityType: chemextractor.EntitySMILES, Text: "C1=CC=CC=C1", Confidence: 0.9, StartOffset: 10},
				},
			}, nil
		},
		ResolveFunc: func(ctx context.Context, entity *chemextractor.RawChemicalEntity) (*chemextractor.ResolvedChemicalEntity, error) {
			return &chemextractor.ResolvedChemicalEntity{
				SMILES:   "c1ccccc1",
				InChIKey: "UHOVQNZJYSORNB-UHFFFAOYSA-N",
				IsResolved: true,
			}, nil
		},
	}

	molRepo := &MockMoleculeRepo{
		FindByInChIKeyFunc: func(ctx context.Context, inchiKey string) (*molecule.Molecule, error) {
			return nil, apperrors.New(apperrors.ErrCodeNotFound, "not found")
		},
	}

	molService := &MockMoleculeService{
		CreateFromSMILESFunc: func(ctx context.Context, smiles string, metadata map[string]string) (*molecule.Molecule, error) {
			return &molecule.Molecule{ID: googleUUID("1"), SMILES: smiles}, nil
		},
	}

	patentRepo := &MockPatentRepo{
		AssociateMoleculeFunc: func(ctx context.Context, patentID string, moleculeID string) error {
			return nil
		},
	}

	storage := &MockStorage{
		GetFunc: func(ctx context.Context, path string) ([]byte, error) {
			return []byte("dummy pdf content"), nil
		},
	}

	svc := NewChemExtractionService(extractor, molService, molRepo, patentRepo, storage, &MockLogger{})

	req := &ExtractionRequest{
		DocumentID:          "doc-1",
		DocumentStoragePath: "bucket/doc-1.pdf",
		Format:              DocumentFormatPDF,
		PatentID:            "pat-1",
	}

	res, err := svc.ExtractFromDocument(context.Background(), req)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.TotalExtracted != 1 {
		t.Errorf("expected 1 extracted, got %d", res.TotalExtracted)
	}
	if res.TotalAccepted != 1 {
		t.Errorf("expected 1 accepted, got %d", res.TotalAccepted)
	}
}

func TestExtractFromText_Success(t *testing.T) {
	extractor := &MockExtractor{
		ExtractFunc: func(ctx context.Context, text string) (*chemextractor.ExtractionResult, error) {
			return &chemextractor.ExtractionResult{
				Entities: []*chemextractor.RawChemicalEntity{
					{EntityType: chemextractor.EntitySMILES, Text: "C", Confidence: 0.8},
				},
			}, nil
		},
		ResolveFunc: func(ctx context.Context, entity *chemextractor.RawChemicalEntity) (*chemextractor.ResolvedChemicalEntity, error) {
			return &chemextractor.ResolvedChemicalEntity{
				SMILES:   "C",
				InChIKey: "Methane",
				IsResolved: true,
			}, nil
		},
	}

	svc := NewChemExtractionService(extractor, &MockMoleculeService{}, &MockMoleculeRepo{}, &MockPatentRepo{}, &MockStorage{}, &MockLogger{})

	req := &TextExtractionRequest{
		Text: "Methane is C",
	}

	res, err := svc.ExtractFromText(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.TotalExtracted != 1 {
		t.Errorf("expected 1 extracted, got %d", res.TotalExtracted)
	}
}
