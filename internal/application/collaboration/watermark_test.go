// ---
// Phase 10 - File #195: internal/application/collaboration/watermark_test.go
//
// 测试用例:
//   - TestGenerateWatermarkRequest_Validate: 参数校验
//   - TestGenerate_Success / NilRequest / PersistError
//   - TestEmbed_Success / NilRequest / NotFound / DocumentMismatch
//   - TestVerify_Success / NoMatch / NilRequest
//   - TestExtract_Found / NotFound / NilRequest
//   - TestListByDocument_Success / EmptyDocumentID / PaginationDefaults
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
// ---

package collaboration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// --- Mock WatermarkRepository ---

type mockWatermarkRepo struct {
	createFn                     func(ctx context.Context, record *WatermarkRecord) error
	getByIDFn                    func(ctx context.Context, id string) (*WatermarkRecord, error)
	getByDocAndFingerprintFn     func(ctx context.Context, documentID, fingerprint string) (*WatermarkRecord, error)
	listByDocumentFn             func(ctx context.Context, documentID string, p commontypes.Pagination) ([]*WatermarkRecord, int, error)
	updateFn                     func(ctx context.Context, record *WatermarkRecord) error
}

func (m *mockWatermarkRepo) Create(ctx context.Context, record *WatermarkRecord) error {
	if m.createFn != nil {
		return m.createFn(ctx, record)
	}
	return nil
}

func (m *mockWatermarkRepo) GetByID(ctx context.Context, id string) (*WatermarkRecord, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("not found")
}

func (m *mockWatermarkRepo) GetByDocumentAndFingerprint(ctx context.Context, documentID, fingerprint string) (*WatermarkRecord, error) {
	if m.getByDocAndFingerprintFn != nil {
		return m.getByDocAndFingerprintFn(ctx, documentID, fingerprint)
	}
	return nil, errors.New("not found")
}

func (m *mockWatermarkRepo) ListByDocument(ctx context.Context, documentID string, p commontypes.Pagination) ([]*WatermarkRecord, int, error) {
	if m.listByDocumentFn != nil {
		return m.listByDocumentFn(ctx, documentID, p)
	}
	return nil, 0, nil
}

func (m *mockWatermarkRepo) Update(ctx context.Context, record *WatermarkRecord) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, record)
	}
	return nil
}

type mockWatermarkLogger struct{}

func (m *mockWatermarkLogger) Debug(msg string, fields ...logging.Field) {}
func (m *mockWatermarkLogger) Info(msg string, fields ...logging.Field)  {}
func (m *mockWatermarkLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *mockWatermarkLogger) Error(msg string, fields ...logging.Field) {}
func (m *mockWatermarkLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *mockWatermarkLogger) With(fields ...logging.Field) logging.Logger { return m }
func (m *mockWatermarkLogger) WithContext(ctx context.Context) logging.Logger { return m }
func (m *mockWatermarkLogger) WithError(err error) logging.Logger { return m }
func (m *mockWatermarkLogger) Sync() error { return nil }

func newTestWatermarkService(repo *mockWatermarkRepo) WatermarkService {
	if repo == nil {
		repo = &mockWatermarkRepo{}
	}
	return NewWatermarkService(repo, &mockWatermarkLogger{})
}

// --- GenerateWatermarkRequest.Validate ---

func TestGenerateWatermarkRequest_Validate_Success(t *testing.T) {
	req := &GenerateWatermarkRequest{
		DocumentID: "doc-1",
		Type:       WatermarkTypeVisible,
		CreatedBy:  "user-1",
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestGenerateWatermarkRequest_Validate_MissingDocumentID(t *testing.T) {
	req := &GenerateWatermarkRequest{
		Type:      WatermarkTypeVisible,
		CreatedBy: "user-1",
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for missing document_id")
	}
}

func TestGenerateWatermarkRequest_Validate_InvalidType(t *testing.T) {
	req := &GenerateWatermarkRequest{
		DocumentID: "doc-1",
		Type:       "hologram",
		CreatedBy:  "user-1",
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestGenerateWatermarkRequest_Validate_MissingCreatedBy(t *testing.T) {
	req := &GenerateWatermarkRequest{
		DocumentID: "doc-1",
		Type:       WatermarkTypeInvisible,
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for missing created_by")
	}
}

// --- Generate ---

func TestGenerate_Success(t *testing.T) {
	svc := newTestWatermarkService(nil)
	resp, err := svc.Generate(context.Background(), &GenerateWatermarkRequest{
		DocumentID: "doc-1",
		Type:       WatermarkTypeInvisible,
		CreatedBy:  "user-1",
		Metadata:   map[string]string{"org": "acme"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.WatermarkID == "" {
		t.Fatal("expected non-empty watermark_id")
	}
	if resp.Fingerprint == "" {
		t.Fatal("expected non-empty fingerprint")
	}
	if resp.Type != WatermarkTypeInvisible {
		t.Fatalf("expected invisible, got %s", resp.Type)
	}
}

func TestGenerate_NilRequest(t *testing.T) {
	svc := newTestWatermarkService(nil)
	_, err := svc.Generate(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestGenerate_PersistError(t *testing.T) {
	repo := &mockWatermarkRepo{
		createFn: func(ctx context.Context, record *WatermarkRecord) error {
			return errors.New("db error")
		},
	}
	svc := newTestWatermarkService(repo)
	_, err := svc.Generate(context.Background(), &GenerateWatermarkRequest{
		DocumentID: "doc-1",
		Type:       WatermarkTypeVisible,
		CreatedBy:  "user-1",
	})
	if err == nil {
		t.Fatal("expected error for persist failure")
	}
}

// --- Embed ---

func TestEmbed_Success(t *testing.T) {
	repo := &mockWatermarkRepo{
		getByIDFn: func(ctx context.Context, id string) (*WatermarkRecord, error) {
			return &WatermarkRecord{
				ID:         id,
				DocumentID: "doc-1",
				Status:     WatermarkStatusPending,
				Metadata:   make(map[string]string),
			}, nil
		},
	}
	svc := newTestWatermarkService(repo)
	resp, err := svc.Embed(context.Background(), &EmbedWatermarkRequest{
		WatermarkID: "wm-1",
		DocumentID:  "doc-1",
		Content:     []byte("document content here"),
		EmbeddedBy:  "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ContentHash == "" {
		t.Fatal("expected non-empty content_hash")
	}
	if resp.EmbeddedAt.IsZero() {
		t.Fatal("expected non-zero embedded_at")
	}
}

func TestEmbed_NilRequest(t *testing.T) {
	svc := newTestWatermarkService(nil)
	_, err := svc.Embed(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestEmbed_NotFound(t *testing.T) {
	svc := newTestWatermarkService(&mockWatermarkRepo{})
	_, err := svc.Embed(context.Background(), &EmbedWatermarkRequest{
		WatermarkID: "wm-missing",
		DocumentID:  "doc-1",
		Content:     []byte("content"),
		EmbeddedBy:  "user-1",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestEmbed_DocumentMismatch(t *testing.T) {
	repo := &mockWatermarkRepo{
		getByIDFn: func(ctx context.Context, id string) (*WatermarkRecord, error) {
			return &WatermarkRecord{
				ID:         id,
				DocumentID: "doc-other",
				Status:     WatermarkStatusPending,
			}, nil
		},
	}
	svc := newTestWatermarkService(repo)
	_, err := svc.Embed(context.Background(), &EmbedWatermarkRequest{
		WatermarkID: "wm-1",
		DocumentID:  "doc-1",
		Content:     []byte("content"),
		EmbeddedBy:  "user-1",
	})
	if err == nil {
		t.Fatal("expected error for document mismatch")
	}
}

// --- Verify ---

func TestVerify_Success(t *testing.T) {
	content := []byte("verified document content")
	// Pre-compute hash
	repo := &mockWatermarkRepo{}
	svc := newTestWatermarkService(repo)

	// First generate and embed to get the content hash
	var capturedRecord *WatermarkRecord
	repo.createFn = func(ctx context.Context, record *WatermarkRecord) error {
		capturedRecord = record
		return nil
	}
	genResp, _ := svc.Generate(context.Background(), &GenerateWatermarkRequest{
		DocumentID: "doc-v",
		Type:       WatermarkTypeInvisible,
		CreatedBy:  "user-1",
	})

	repo.getByIDFn = func(ctx context.Context, id string) (*WatermarkRecord, error) {
		if capturedRecord != nil && capturedRecord.ID == id {
			return capturedRecord, nil
		}
		return nil, errors.New("not found")
	}

	_, _ = svc.Embed(context.Background(), &EmbedWatermarkRequest{
		WatermarkID: genResp.WatermarkID,
		DocumentID:  "doc-v",
		Content:     content,
		EmbeddedBy:  "user-1",
	})

	// Now set up list to return the embedded record
	repo.listByDocumentFn = func(ctx context.Context, documentID string, p commontypes.Pagination) ([]*WatermarkRecord, int, error) {
		if capturedRecord != nil {
			return []*WatermarkRecord{capturedRecord}, 1, nil
		}
		return nil, 0, nil
	}

	verifyResp, err := svc.Verify(context.Background(), &VerifyWatermarkRequest{
		DocumentID: "doc-v",
		Content:    content,
		VerifiedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !verifyResp.IsValid {
		t.Fatal("expected watermark to be valid")
	}
	if verifyResp.WatermarkID == "" {
		t.Fatal("expected non-empty watermark_id in verify response")
	}
}

func TestVerify_NoMatch(t *testing.T) {
	repo := &mockWatermarkRepo{
		listByDocumentFn: func(ctx context.Context, documentID string, p commontypes.Pagination) ([]*WatermarkRecord, int, error) {
			return []*WatermarkRecord{
				{
					ID:       "wm-1",
					Status:   WatermarkStatusEmbedded,
					Metadata: map[string]string{"content_hash": "aaaa"},
				},
			}, 1, nil
		},
	}
	svc := newTestWatermarkService(repo)
	resp, err := svc.Verify(context.Background(), &VerifyWatermarkRequest{
		DocumentID: "doc-1",
		Content:    []byte("different content"),
		VerifiedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.IsValid {
		t.Fatal("expected watermark to be invalid for non-matching content")
	}
}

func TestVerify_NilRequest(t *testing.T) {
	svc := newTestWatermarkService(nil)
	_, err := svc.Verify(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestVerify_EmptyDocumentID(t *testing.T) {
	svc := newTestWatermarkService(nil)
	_, err := svc.Verify(context.Background(), &VerifyWatermarkRequest{
		Content:    []byte("content"),
		VerifiedBy: "user-1",
	})
	if err == nil {
		t.Fatal("expected validation error for empty document_id")
	}
}

func TestVerify_EmptyContent(t *testing.T) {
	svc := newTestWatermarkService(nil)
	_, err := svc.Verify(context.Background(), &VerifyWatermarkRequest{
		DocumentID: "doc-1",
		VerifiedBy: "user-1",
	})
	if err == nil {
		t.Fatal("expected validation error for empty content")
	}
}

func TestVerify_RepoError(t *testing.T) {
	repo := &mockWatermarkRepo{
		listByDocumentFn: func(ctx context.Context, documentID string, p commontypes.Pagination) ([]*WatermarkRecord, int, error) {
			return nil, 0, errors.New("db error")
		},
	}
	svc := newTestWatermarkService(repo)
	_, err := svc.Verify(context.Background(), &VerifyWatermarkRequest{
		DocumentID: "doc-1",
		Content:    []byte("content"),
		VerifiedBy: "user-1",
	})
	if err == nil {
		t.Fatal("expected error for repo failure")
	}
}

// --- Extract ---

func TestExtract_Found(t *testing.T) {
	content := []byte("traceable document")

	repo := &mockWatermarkRepo{}
	svc := newTestWatermarkService(repo)

	var capturedRecord *WatermarkRecord
	repo.createFn = func(ctx context.Context, record *WatermarkRecord) error {
		capturedRecord = record
		return nil
	}
	genResp, _ := svc.Generate(context.Background(), &GenerateWatermarkRequest{
		DocumentID: "doc-e",
		Type:       WatermarkTypeVisible,
		CreatedBy:  "user-tracer",
		Metadata:   map[string]string{"dept": "legal"},
	})
	repo.getByIDFn = func(ctx context.Context, id string) (*WatermarkRecord, error) {
		if capturedRecord != nil && capturedRecord.ID == id {
			return capturedRecord, nil
		}
		return nil, errors.New("not found")
	}
	_, _ = svc.Embed(context.Background(), &EmbedWatermarkRequest{
		WatermarkID: genResp.WatermarkID,
		DocumentID:  "doc-e",
		Content:     content,
		EmbeddedBy:  "user-tracer",
	})

	repo.listByDocumentFn = func(ctx context.Context, documentID string, p commontypes.Pagination) ([]*WatermarkRecord, int, error) {
		if capturedRecord != nil {
			return []*WatermarkRecord{capturedRecord}, 1, nil
		}
		return nil, 0, nil
	}

	resp, err := svc.Extract(context.Background(), &ExtractWatermarkRequest{
		DocumentID:  "doc-e",
		Content:     content,
		ExtractedBy: "auditor-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Found {
		t.Fatal("expected watermark to be found")
	}
	if resp.CreatedBy != "user-tracer" {
		t.Fatalf("expected created_by user-tracer, got %s", resp.CreatedBy)
	}
}

func TestExtract_NotFound(t *testing.T) {
	repo := &mockWatermarkRepo{
		listByDocumentFn: func(ctx context.Context, documentID string, p commontypes.Pagination) ([]*WatermarkRecord, int, error) {
			return nil, 0, nil
		},
	}
	svc := newTestWatermarkService(repo)
	resp, err := svc.Extract(context.Background(), &ExtractWatermarkRequest{
		DocumentID:  "doc-1",
		Content:     []byte("unknown content"),
		ExtractedBy: "auditor-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Found {
		t.Fatal("expected watermark not to be found")
	}
}

func TestExtract_NilRequest(t *testing.T) {
	svc := newTestWatermarkService(nil)
	_, err := svc.Extract(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestExtract_EmptyDocumentID(t *testing.T) {
	svc := newTestWatermarkService(nil)
	_, err := svc.Extract(context.Background(), &ExtractWatermarkRequest{
		Content:     []byte("content"),
		ExtractedBy: "user-1",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// --- ListByDocument ---

func TestListByDocument_Success(t *testing.T) {
	now := time.Now()
	repo := &mockWatermarkRepo{
		listByDocumentFn: func(ctx context.Context, documentID string, p commontypes.Pagination) ([]*WatermarkRecord, int, error) {
			return []*WatermarkRecord{
				{ID: "wm-1", DocumentID: documentID, CreatedAt: now},
				{ID: "wm-2", DocumentID: documentID, CreatedAt: now},
			}, 2, nil
		},
	}
	svc := newTestWatermarkService(repo)
	records, total, err := svc.ListByDocument(context.Background(), "doc-1", commontypes.Pagination{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestListByDocument_EmptyDocumentID(t *testing.T) {
	svc := newTestWatermarkService(nil)
	_, _, err := svc.ListByDocument(context.Background(), "", commontypes.Pagination{Page: 1, PageSize: 10})
	if err == nil {
		t.Fatal("expected validation error for empty document_id")
	}
}

func TestListByDocument_PaginationDefaults(t *testing.T) {
	var capturedPagination commontypes.Pagination
	repo := &mockWatermarkRepo{
		listByDocumentFn: func(ctx context.Context, documentID string, p commontypes.Pagination) ([]*WatermarkRecord, int, error) {
			capturedPagination = p
			return nil, 0, nil
		},
	}
	svc := newTestWatermarkService(repo)
	_, _, _ = svc.ListByDocument(context.Background(), "doc-1", commontypes.Pagination{Page: 0, PageSize: 0})
	if capturedPagination.Page != 1 {
		t.Fatalf("expected page default 1, got %d", capturedPagination.Page)
	}
	if capturedPagination.PageSize != 20 {
		t.Fatalf("expected pageSize default 20, got %d", capturedPagination.PageSize)
	}
}

func TestListByDocument_RepoError(t *testing.T) {
	repo := &mockWatermarkRepo{
		listByDocumentFn: func(ctx context.Context, documentID string, p commontypes.Pagination) ([]*WatermarkRecord, int, error) {
			return nil, 0, errors.New("db error")
		},
	}
	svc := newTestWatermarkService(repo)
	_, _, err := svc.ListByDocument(context.Background(), "doc-1", commontypes.Pagination{Page: 1, PageSize: 10})
	if err == nil {
		t.Fatal("expected error for repo failure")
	}
}

func TestWatermarkType_IsValid(t *testing.T) {
	tests := []struct {
		wt    WatermarkType
		valid bool
	}{
		{WatermarkTypeVisible, true},
		{WatermarkTypeInvisible, true},
		{"hologram", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := tt.wt.IsValid(); got != tt.valid {
			t.Errorf("WatermarkType(%q).IsValid() = %v, want %v", tt.wt, got, tt.valid)
		}
	}
}

//Personal.AI order the ending
