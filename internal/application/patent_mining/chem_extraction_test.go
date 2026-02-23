// Phase 10 - File 215 of 349
// Phase: 应用层 - 业务服务
// SubModule: patent_mining
// File: internal/application/patent_mining/chem_extraction_test.go
//
// Generation Plan:
// - 功能定位: 对 ChemExtractionService 接口所有方法进行全面单元测试
// - 核心实现:
//   - 构建 mock 依赖: mockChemExtractor, mockMoleculeRepository, mockPatentRepository, mockEventPublisher, mockLogger
//   - 覆盖 ExtractFromPatent / ExtractFromText / ExtractFromDocument / BatchExtract / GetExtractionResult 全部方法
//   - 验证正常流程、参数校验失败、依赖错误传播、空结果处理
// - 测试用例:
//   - TestExtractFromPatent_Success / PatentNotFound / ExtractorError / NoEntitiesFound
//   - TestExtractFromText_Success / EmptyText / InvalidFormat
//   - TestExtractFromDocument_Success / UnsupportedType
//   - TestBatchExtract_Success / PartialFailure / EmptyInput
//   - TestGetExtractionResult_Success / NotFound
//   - TestExtractFromPatent_MoleculeValidation / DuplicateDetection
// - 依赖: internal/application/patent_mining/chem_extraction.go, pkg/errors, pkg/types
// - 被依赖: CI pipeline
// - 强制约束: 文件最后一行必须为 //Personal.AI order the ending

package patent_mining

import (
	"context"
	"errors"
	"testing"
	"time"

	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	commonTypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	moleculeTypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// ---------------------------------------------------------------------------
// Mock: ChemExtractorEngine
// ---------------------------------------------------------------------------

type mockChemExtractorEngine struct {
	extractFromTextFn func(ctx context.Context, text string, opts ExtractionOptions) (*RawExtractionResult, error)
	extractFromFileFn func(ctx context.Context, filePath string, fileType string, opts ExtractionOptions) (*RawExtractionResult, error)
}

func (m *mockChemExtractorEngine) ExtractFromText(ctx context.Context, text string, opts ExtractionOptions) (*RawExtractionResult, error) {
	if m.extractFromTextFn != nil {
		return m.extractFromTextFn(ctx, text, opts)
	}
	return &RawExtractionResult{}, nil
}

func (m *mockChemExtractorEngine) ExtractFromFile(ctx context.Context, filePath string, fileType string, opts ExtractionOptions) (*RawExtractionResult, error) {
	if m.extractFromFileFn != nil {
		return m.extractFromFileFn(ctx, filePath, fileType, opts)
	}
	return &RawExtractionResult{}, nil
}

// ---------------------------------------------------------------------------
// Mock: MoleculeRepoForExtraction
// ---------------------------------------------------------------------------

type mockMoleculeRepoForExtraction struct {
	findByInChIKeyFn func(ctx context.Context, inchiKey string) (*moleculeTypes.MoleculeDTO, error)
	saveFn           func(ctx context.Context, mol *moleculeTypes.MoleculeDTO) error
}

func (m *mockMoleculeRepoForExtraction) FindByInChIKey(ctx context.Context, inchiKey string) (*moleculeTypes.MoleculeDTO, error) {
	if m.findByInChIKeyFn != nil {
		return m.findByInChIKeyFn(ctx, inchiKey)
	}
	return nil, apperrors.NewNotFoundError("molecule", inchiKey)
}

func (m *mockMoleculeRepoForExtraction) Save(ctx context.Context, mol *moleculeTypes.MoleculeDTO) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, mol)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Mock: PatentRepoForExtraction
// ---------------------------------------------------------------------------

type mockPatentRepoForExtraction struct {
	getByIDFn     func(ctx context.Context, id string) (*PatentDocumentRef, error)
	getFullTextFn func(ctx context.Context, id string) (string, error)
}

func (m *mockPatentRepoForExtraction) GetByID(ctx context.Context, id string) (*PatentDocumentRef, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, apperrors.NewNotFoundError("patent", id)
}

func (m *mockPatentRepoForExtraction) GetFullText(ctx context.Context, id string) (string, error) {
	if m.getFullTextFn != nil {
		return m.getFullTextFn(ctx, id)
	}
	return "", apperrors.NewNotFoundError("patent_text", id)
}

// ---------------------------------------------------------------------------
// Mock: ExtractionEventPublisher
// ---------------------------------------------------------------------------

type mockExtractionEventPublisher struct {
	publishFn func(ctx context.Context, event ExtractionEvent) error
	calls     []ExtractionEvent
}

func (m *mockExtractionEventPublisher) Publish(ctx context.Context, event ExtractionEvent) error {
	m.calls = append(m.calls, event)
	if m.publishFn != nil {
		return m.publishFn(ctx, event)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Mock: ExtractionResultStore
// ---------------------------------------------------------------------------

type mockExtractionResultStore struct {
	saveFn func(ctx context.Context, result *ExtractionResult) error
	getFn  func(ctx context.Context, id string) (*ExtractionResult, error)
}

func (m *mockExtractionResultStore) Save(ctx context.Context, result *ExtractionResult) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, result)
	}
	return nil
}

func (m *mockExtractionResultStore) Get(ctx context.Context, id string) (*ExtractionResult, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return nil, apperrors.NewNotFoundError("extraction_result", id)
}

// ---------------------------------------------------------------------------
// Mock: Logger (reuse pattern)
// ---------------------------------------------------------------------------

type mockExtLogger struct{}

func (m *mockExtLogger) Info(msg string, fields ...interface{})  {}
func (m *mockExtLogger) Error(msg string, fields ...interface{}) {}
func (m *mockExtLogger) Warn(msg string, fields ...interface{})  {}
func (m *mockExtLogger) Debug(msg string, fields ...interface{}) {}

// ---------------------------------------------------------------------------
// Helper: build service under test
// ---------------------------------------------------------------------------

func newTestChemExtractionService(
	extractor ChemExtractorEngine,
	molRepo MoleculeRepoForExtraction,
	patRepo PatentRepoForExtraction,
	publisher ExtractionEventPublisher,
	store ExtractionResultStore,
) ChemExtractionService {
	return NewChemExtractionService(ChemExtractionDeps{
		Extractor:   extractor,
		MolRepo:     molRepo,
		PatentRepo:  patRepo,
		Publisher:   publisher,
		ResultStore: store,
		Logger:      &mockExtLogger{},
	})
}

func sampleRawExtractionResult() *RawExtractionResult {
	return &RawExtractionResult{
		Molecules: []ExtractedMolecule{
			{
				SMILES:   "c1ccc2c(c1)[nH]c1ccccc12",
				Name:     "Carbazole",
				InChIKey: "TVFDJXOCBHFTFK-UHFFFAOYSA-N",
				Source:   "Example 1",
				Confidence: 0.95,
			},
			{
				SMILES:   "c1ccc(-c2ccccc2)cc1",
				Name:     "Biphenyl",
				InChIKey: "ZUOUZKKEUPVFJK-UHFFFAOYSA-N",
				Source:   "Claim 1",
				Confidence: 0.88,
			},
		},
		Properties: []ExtractedProperty{
			{
				MoleculeRef: "Carbazole",
				Name:        "quantum_efficiency",
				Value:       "22.5",
				Unit:        "%",
				Confidence:  0.90,
			},
		},
		ProcessingTimeMs: 1200,
	}
}

// ===========================================================================
// Tests: ExtractFromPatent
// ===========================================================================

func TestExtractFromPatent_Success(t *testing.T) {
	patentText := "This patent describes carbazole-based OLED host materials..."
	rawResult := sampleRawExtractionResult()

	patRepo := &mockPatentRepoForExtraction{
		getByIDFn: func(ctx context.Context, id string) (*PatentDocumentRef, error) {
			return &PatentDocumentRef{
				ID:            id,
				PatentNumber:  "CN115000001A",
				Title:         "Carbazole OLED Host",
				FilingDate:    time.Date(2022, 3, 15, 0, 0, 0, 0, time.UTC),
			}, nil
		},
		getFullTextFn: func(ctx context.Context, id string) (string, error) {
			return patentText, nil
		},
	}

	extractor := &mockChemExtractorEngine{
		extractFromTextFn: func(ctx context.Context, text string, opts ExtractionOptions) (*RawExtractionResult, error) {
			if text != patentText {
				t.Errorf("unexpected text passed to extractor")
			}
			return rawResult, nil
		},
	}

	molRepo := &mockMoleculeRepoForExtraction{
		findByInChIKeyFn: func(ctx context.Context, inchiKey string) (*moleculeTypes.MoleculeDTO, error) {
			return nil, apperrors.NewNotFoundError("molecule", inchiKey)
		},
		saveFn: func(ctx context.Context, mol *moleculeTypes.MoleculeDTO) error {
			return nil
		},
	}

	publisher := &mockExtractionEventPublisher{}
	store := &mockExtractionResultStore{
		saveFn: func(ctx context.Context, result *ExtractionResult) error {
			return nil
		},
	}

	svc := newTestChemExtractionService(extractor, molRepo, patRepo, publisher, store)

	req := &ExtractFromPatentRequest{
		PatentID: "pat-001",
		Options: ExtractionOptions{
			ExtractMolecules:  true,
			ExtractProperties: true,
			MinConfidence:     0.80,
		},
	}

	result, err := svc.ExtractFromPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Molecules) != 2 {
		t.Errorf("expected 2 molecules, got %d", len(result.Molecules))
	}
	if len(result.Properties) != 1 {
		t.Errorf("expected 1 property, got %d", len(result.Properties))
	}
	if result.PatentID != "pat-001" {
		t.Errorf("expected patent ID pat-001, got %s", result.PatentID)
	}
	if len(publisher.calls) == 0 {
		t.Error("expected at least one event published")
	}
}

func TestExtractFromPatent_PatentNotFound(t *testing.T) {
	patRepo := &mockPatentRepoForExtraction{
		getByIDFn: func(ctx context.Context, id string) (*PatentDocumentRef, error) {
			return nil, apperrors.NewNotFoundError("patent", id)
		},
	}

	svc := newTestChemExtractionService(
		&mockChemExtractorEngine{},
		&mockMoleculeRepoForExtraction{},
		patRepo,
		&mockExtractionEventPublisher{},
		&mockExtractionResultStore{},
	)

	req := &ExtractFromPatentRequest{PatentID: "nonexistent"}
	_, err := svc.ExtractFromPatent(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for nonexistent patent")
	}
	if !apperrors.IsNotFoundError(err) {
		t.Errorf("expected NotFoundError, got: %v", err)
	}
}

func TestExtractFromPatent_ExtractorError(t *testing.T) {
	patRepo := &mockPatentRepoForExtraction{
		getByIDFn: func(ctx context.Context, id string) (*PatentDocumentRef, error) {
			return &PatentDocumentRef{ID: id, PatentNumber: "CN115000001A"}, nil
		},
		getFullTextFn: func(ctx context.Context, id string) (string, error) {
			return "some text", nil
		},
	}

	extractor := &mockChemExtractorEngine{
		extractFromTextFn: func(ctx context.Context, text string, opts ExtractionOptions) (*RawExtractionResult, error) {
			return nil, errors.New("model inference timeout")
		},
	}

	svc := newTestChemExtractionService(
		extractor,
		&mockMoleculeRepoForExtraction{},
		patRepo,
		&mockExtractionEventPublisher{},
		&mockExtractionResultStore{},
	)

	req := &ExtractFromPatentRequest{PatentID: "pat-001"}
	_, err := svc.ExtractFromPatent(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from extractor failure")
	}
}

func TestExtractFromPatent_NoEntitiesFound(t *testing.T) {
	patRepo := &mockPatentRepoForExtraction{
		getByIDFn: func(ctx context.Context, id string) (*PatentDocumentRef, error) {
			return &PatentDocumentRef{ID: id, PatentNumber: "CN115000001A"}, nil
		},
		getFullTextFn: func(ctx context.Context, id string) (string, error) {
			return "This patent has no chemical structures.", nil
		},
	}

	extractor := &mockChemExtractorEngine{
		extractFromTextFn: func(ctx context.Context, text string, opts ExtractionOptions) (*RawExtractionResult, error) {
			return &RawExtractionResult{
				Molecules:        []ExtractedMolecule{},
				Properties:       []ExtractedProperty{},
				ProcessingTimeMs: 500,
			}, nil
		},
	}

	store := &mockExtractionResultStore{
		saveFn: func(ctx context.Context, result *ExtractionResult) error { return nil },
	}

	svc := newTestChemExtractionService(
		extractor,
		&mockMoleculeRepoForExtraction{},
		patRepo,
		&mockExtractionEventPublisher{},
		store,
	)

	req := &ExtractFromPatentRequest{PatentID: "pat-002"}
	result, err := svc.ExtractFromPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error for empty extraction, got: %v", err)
	}
	if len(result.Molecules) != 0 {
		t.Errorf("expected 0 molecules, got %d", len(result.Molecules))
	}
}

func TestExtractFromPatent_MoleculeValidation(t *testing.T) {
	patRepo := &mockPatentRepoForExtraction{
		getByIDFn: func(ctx context.Context, id string) (*PatentDocumentRef, error) {
			return &PatentDocumentRef{ID: id, PatentNumber: "CN115000001A"}, nil
		},
		getFullTextFn: func(ctx context.Context, id string) (string, error) {
			return "text with molecules", nil
		},
	}

	extractor := &mockChemExtractorEngine{
		extractFromTextFn: func(ctx context.Context, text string, opts ExtractionOptions) (*RawExtractionResult, error) {
			return &RawExtractionResult{
				Molecules: []ExtractedMolecule{
					{SMILES: "c1ccc2c(c1)[nH]c1ccccc12", InChIKey: "TVFDJXOCBHFTFK-UHFFFAOYSA-N", Confidence: 0.95},
					{SMILES: "INVALID_SMILES", InChIKey: "", Confidence: 0.30},
				},
			}, nil
		},
	}

	molRepo := &mockMoleculeRepoForExtraction{
		findByInChIKeyFn: func(ctx context.Context, inchiKey string) (*moleculeTypes.MoleculeDTO, error) {
			return nil, apperrors.NewNotFoundError("molecule", inchiKey)
		},
		saveFn: func(ctx context.Context, mol *moleculeTypes.MoleculeDTO) error { return nil },
	}

	store := &mockExtractionResultStore{
		saveFn: func(ctx context.Context, result *ExtractionResult) error { return nil },
	}

	svc := newTestChemExtractionService(
		extractor,
		molRepo,
		patRepo,
		&mockExtractionEventPublisher{},
		store,
	)

	req := &ExtractFromPatentRequest{
		PatentID: "pat-003",
		Options:  ExtractionOptions{MinConfidence: 0.80, ExtractMolecules: true},
	}
	result, err := svc.ExtractFromPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Only the high-confidence valid molecule should pass
	if len(result.Molecules) != 1 {
		t.Errorf("expected 1 valid molecule after filtering, got %d", len(result.Molecules))
	}
}

func TestExtractFromPatent_DuplicateDetection(t *testing.T) {
	patRepo := &mockPatentRepoForExtraction{
		getByIDFn: func(ctx context.Context, id string) (*PatentDocumentRef, error) {
			return &PatentDocumentRef{ID: id, PatentNumber: "CN115000001A"}, nil
		},
		getFullTextFn: func(ctx context.Context, id string) (string, error) {
			return "text with duplicate molecules", nil
		},
	}

	extractor := &mockChemExtractorEngine{
		extractFromTextFn: func(ctx context.Context, text string, opts ExtractionOptions) (*RawExtractionResult, error) {
			return &RawExtractionResult{
				Molecules: []ExtractedMolecule{
					{SMILES: "c1ccc2c(c1)[nH]c1ccccc12", InChIKey: "TVFDJXOCBHFTFK-UHFFFAOYSA-N", Confidence: 0.95},
					{SMILES: "c1ccc2c(c1)[nH]c1ccccc12", InChIKey: "TVFDJXOCBHFTFK-UHFFFAOYSA-N", Confidence: 0.92},
				},
			}, nil
		},
	}

	existingMol := &moleculeTypes.MoleculeDTO{
		ID:       "mol-existing",
		SMILES:   "c1ccc2c(c1)[nH]c1ccccc12",
		InChIKey: "TVFDJXOCBHFTFK-UHFFFAOYSA-N",
	}

	molRepo := &mockMoleculeRepoForExtraction{
		findByInChIKeyFn: func(ctx context.Context, inchiKey string) (*moleculeTypes.MoleculeDTO, error) {
			if inchiKey == "TVFDJXOCBHFTFK-UHFFFAOYSA-N" {
				return existingMol, nil
			}
			return nil, apperrors.NewNotFoundError("molecule", inchiKey)
		},
	}

	store := &mockExtractionResultStore{
		saveFn: func(ctx context.Context, result *ExtractionResult) error { return nil },
	}

	svc := newTestChemExtractionService(
		extractor,
		molRepo,
		patRepo,
		&mockExtractionEventPublisher{},
		store,
	)

	req := &ExtractFromPatentRequest{
		PatentID: "pat-004",
		Options:  ExtractionOptions{ExtractMolecules: true, MinConfidence: 0.80},
	}
	result, err := svc.ExtractFromPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// Duplicates should be deduplicated, existing molecule should be referenced
	if len(result.Molecules) != 1 {
		t.Errorf("expected 1 deduplicated molecule, got %d", len(result.Molecules))
	}
	if result.Molecules[0].ExistingID != "mol-existing" {
		t.Errorf("expected existing molecule reference, got: %s", result.Molecules[0].ExistingID)
	}
}

// ===========================================================================
// Tests: ExtractFromText
// ===========================================================================

func TestExtractFromText_Success(t *testing.T) {
	extractor := &mockChemExtractorEngine{
		extractFromTextFn: func(ctx context.Context, text string, opts ExtractionOptions) (*RawExtractionResult, error) {
			return sampleRawExtractionResult(), nil
		},
	}

	molRepo := &mockMoleculeRepoForExtraction{
		findByInChIKeyFn: func(ctx context.Context, inchiKey string) (*moleculeTypes.MoleculeDTO, error) {
			return nil, apperrors.NewNotFoundError("molecule", inchiKey)
		},
		saveFn: func(ctx context.Context, mol *moleculeTypes.MoleculeDTO) error { return nil },
	}

	store := &mockExtractionResultStore{
		saveFn: func(ctx context.Context, result *ExtractionResult) error { return nil },
	}

	svc := newTestChemExtractionService(
		extractor,
		molRepo,
		&mockPatentRepoForExtraction{},
		&mockExtractionEventPublisher{},
		store,
	)

	req := &ExtractFromTextRequest{
		Text:   "The compound carbazole (SMILES: c1ccc2c(c1)[nH]c1ccccc12) was synthesized.",
		Format: "plain_text",
		Options: ExtractionOptions{
			ExtractMolecules:  true,
			ExtractProperties: true,
			MinConfidence:     0.80,
		},
	}

	result, err := svc.ExtractFromText(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Molecules) == 0 {
		t.Error("expected at least one molecule extracted")
	}
}

func TestExtractFromText_EmptyText(t *testing.T) {
	svc := newTestChemExtractionService(
		&mockChemExtractorEngine{},
		&mockMoleculeRepoForExtraction{},
		&mockPatentRepoForExtraction{},
		&mockExtractionEventPublisher{},
		&mockExtractionResultStore{},
	)

	req := &ExtractFromTextRequest{Text: "", Format: "plain_text"}
	_, err := svc.ExtractFromText(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty text")
	}
	if !apperrors.IsValidationError(err) {
		t.Errorf("expected ValidationError, got: %v", err)
	}
}

func TestExtractFromText_InvalidFormat(t *testing.T) {
	svc := newTestChemExtractionService(
		&mockChemExtractorEngine{},
		&mockMoleculeRepoForExtraction{},
		&mockPatentRepoForExtraction{},
		&mockExtractionEventPublisher{},
		&mockExtractionResultStore{},
	)

	req := &ExtractFromTextRequest{Text: "some text", Format: "unsupported_format"}
	_, err := svc.ExtractFromText(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if !apperrors.IsValidationError(err) {
		t.Errorf("expected ValidationError, got: %v", err)
	}
}

// ===========================================================================
// Tests: ExtractFromDocument
// ===========================================================================

func TestExtractFromDocument_Success(t *testing.T) {
	extractor := &mockChemExtractorEngine{
		extractFromFileFn: func(ctx context.Context, filePath string, fileType string, opts ExtractionOptions) (*RawExtractionResult, error) {
			return sampleRawExtractionResult(), nil
		},
	}

	molRepo := &mockMoleculeRepoForExtraction{
		findByInChIKeyFn: func(ctx context.Context, inchiKey string) (*moleculeTypes.MoleculeDTO, error) {
			return nil, apperrors.NewNotFoundError("molecule", inchiKey)
		},
		saveFn: func(ctx context.Context, mol *moleculeTypes.MoleculeDTO) error { return nil },
	}

	store := &mockExtractionResultStore{
		saveFn: func(ctx context.Context, result *ExtractionResult) error { return nil },
	}

	svc := newTestChemExtractionService(
		extractor,
		molRepo,
		&mockPatentRepoForExtraction{},
		&mockExtractionEventPublisher{},
		store,
	)

	req := &ExtractFromDocumentRequest{
		FilePath: "/data/patents/CN115000001A.pdf",
		FileType: "pdf",
		Options:  ExtractionOptions{ExtractMolecules: true, MinConfidence: 0.80},
	}

	result, err := svc.ExtractFromDocument(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestExtractFromDocument_UnsupportedType(t *testing.T) {
	svc := newTestChemExtractionService(
		&mockChemExtractorEngine{},
		&mockMoleculeRepoForExtraction{},
		&mockPatentRepoForExtraction{},
		&mockExtractionEventPublisher{},
		&mockExtractionResultStore{},
	)

	req := &ExtractFromDocumentRequest{
		FilePath: "/data/file.exe",
		FileType: "exe",
	}
	_, err := svc.ExtractFromDocument(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unsupported document type")
	}
	if !apperrors.IsValidationError(err) {
		t.Errorf("expected ValidationError, got: %v", err)
	}
}

// ===========================================================================
// Tests: BatchExtract
// ===========================================================================

func TestBatchExtract_Success(t *testing.T) {
	callCount := 0
	extractor := &mockChemExtractorEngine{
		extractFromTextFn: func(ctx context.Context, text string, opts ExtractionOptions) (*RawExtractionResult, error) {
			callCount++
			return sampleRawExtractionResult(), nil
		},
	}

	patRepo := &mockPatentRepoForExtraction{
		getByIDFn: func(ctx context.Context, id string) (*PatentDocumentRef, error) {
			return &PatentDocumentRef{ID: id, PatentNumber: "CN11500000" + id}, nil
		},
		getFullTextFn: func(ctx context.Context, id string) (string, error) {
			return "patent text for " + id, nil
		},
	}

	molRepo := &mockMoleculeRepoForExtraction{
		findByInChIKeyFn: func(ctx context.Context, inchiKey string) (*moleculeTypes.MoleculeDTO, error) {
			return nil, apperrors.NewNotFoundError("molecule", inchiKey)
		},
		saveFn: func(ctx context.Context, mol *moleculeTypes.MoleculeDTO) error { return nil },
	}

	store := &mockExtractionResultStore{
		saveFn: func(ctx context.Context, result *ExtractionResult) error { return nil },
	}

	svc := newTestChemExtractionService(
		extractor,
		molRepo,
		patRepo,
		&mockExtractionEventPublisher{},
		store,
	)

	req := &BatchExtractRequest{
		PatentIDs: []string{"1", "2", "3"},
		Options:   ExtractionOptions{ExtractMolecules: true, MinConfidence: 0.80},
	}

	results, err := svc.BatchExtract(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results.Results))
	}
	if results.TotalProcessed != 3 {
		t.Errorf("expected TotalProcessed=3, got %d", results.TotalProcessed)
	}
	if results.FailedCount != 0 {
		t.Errorf("expected FailedCount=0, got %d", results.FailedCount)
	}
	if callCount != 3 {
		t.Errorf("expected extractor called 3 times, got %d", callCount)
	}
}

func TestBatchExtract_PartialFailure(t *testing.T) {
	patRepo := &mockPatentRepoForExtraction{
		getByIDFn: func(ctx context.Context, id string) (*PatentDocumentRef, error) {
			if id == "bad" {
				return nil, apperrors.NewNotFoundError("patent", id)
			}
			return &PatentDocumentRef{ID: id, PatentNumber: "CN115000001A"}, nil
		},
		getFullTextFn: func(ctx context.Context, id string) (string, error) {
			return "text for " + id, nil
		},
	}

	extractor := &mockChemExtractorEngine{
		extractFromTextFn: func(ctx context.Context, text string, opts ExtractionOptions) (*RawExtractionResult, error) {
			return sampleRawExtractionResult(), nil
		},
	}

	molRepo := &mockMoleculeRepoForExtraction{
		findByInChIKeyFn: func(ctx context.Context, inchiKey string) (*moleculeTypes.MoleculeDTO, error) {
			return nil, apperrors.NewNotFoundError("molecule", inchiKey)
		},
		saveFn: func(ctx context.Context, mol *moleculeTypes.MoleculeDTO) error { return nil },
	}

	store := &mockExtractionResultStore{
		saveFn: func(ctx context.Context, result *ExtractionResult) error { return nil },
	}

	svc := newTestChemExtractionService(
		extractor,
		molRepo,
		patRepo,
		&mockExtractionEventPublisher{},
		store,
	)

	req := &BatchExtractRequest{
		PatentIDs: []string{"good1", "bad", "good2"},
		Options:   ExtractionOptions{ExtractMolecules: true, MinConfidence: 0.80},
	}

	results, err := svc.BatchExtract(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no top-level error for partial failure, got: %v", err)
	}
	if results.TotalProcessed != 3 {
		t.Errorf("expected TotalProcessed=3, got %d", results.TotalProcessed)
	}
	if results.FailedCount != 1 {
		t.Errorf("expected FailedCount=1, got %d", results.FailedCount)
	}
	if results.SuccessCount != 2 {
		t.Errorf("expected SuccessCount=2, got %d", results.SuccessCount)
	}
}

func TestBatchExtract_EmptyInput(t *testing.T) {
	svc := newTestChemExtractionService(
		&mockChemExtractorEngine{},
		&mockMoleculeRepoForExtraction{},
		&mockPatentRepoForExtraction{},
		&mockExtractionEventPublisher{},
		&mockExtractionResultStore{},
	)

	req := &BatchExtractRequest{PatentIDs: []string{}}
	_, err := svc.BatchExtract(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !apperrors.IsValidationError(err) {
		t.Errorf("expected ValidationError, got: %v", err)
	}
}

// ===========================================================================
// Tests: GetExtractionResult
// ===========================================================================

func TestGetExtractionResult_Success(t *testing.T) {
	expected := &ExtractionResult{
		ID:       "ext-001",
		PatentID: "pat-001",
		Status:   ExtractionStatusCompleted,
		Molecules: []ExtractionMoleculeResult{
			{SMILES: "c1ccc2c(c1)[nH]c1ccccc12", InChIKey: "TVFDJXOCBHFTFK-UHFFFAOYSA-N"},
		},
		CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
	}

	store := &mockExtractionResultStore{
		getFn: func(ctx context.Context, id string) (*ExtractionResult, error) {
			if id == "ext-001" {
				return expected, nil
			}
			return nil, apperrors.NewNotFoundError("extraction_result", id)
		},
	}

	svc := newTestChemExtractionService(
		&mockChemExtractorEngine{},
		&mockMoleculeRepoForExtraction{},
		&mockPatentRepoForExtraction{},
		&mockExtractionEventPublisher{},
		store,
	)

	result, err := svc.GetExtractionResult(context.Background(), "ext-001")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.ID != "ext-001" {
		t.Errorf("expected ID ext-001, got %s", result.ID)
	}
	if result.PatentID != "pat-001" {
		t.Errorf("expected PatentID pat-001, got %s", result.PatentID)
	}
	if result.Status != ExtractionStatusCompleted {
		t.Errorf("expected status Completed, got %s", result.Status)
	}
	if len(result.Molecules) != 1 {
		t.Errorf("expected 1 molecule, got %d", len(result.Molecules))
	}
}

func TestGetExtractionResult_NotFound(t *testing.T) {
	store := &mockExtractionResultStore{
		getFn: func(ctx context.Context, id string) (*ExtractionResult, error) {
			return nil, apperrors.NewNotFoundError("extraction_result", id)
		},
	}

	svc := newTestChemExtractionService(
		&mockChemExtractorEngine{},
		&mockMoleculeRepoForExtraction{},
		&mockPatentRepoForExtraction{},
		&mockExtractionEventPublisher{},
		store,
	)

	_, err := svc.GetExtractionResult(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent result")
	}
	if !apperrors.IsNotFoundError(err) {
		t.Errorf("expected NotFoundError, got: %v", err)
	}
}

// suppress unused import warnings
var (
	_ commonTypes.Pagination
	_ time.Time
)

//Personal.AI order the ending

