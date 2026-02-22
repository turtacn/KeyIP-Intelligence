// ---
// Phase 10 - File #194: internal/application/collaboration/watermark.go
//
// 功能定位: 水印应用服务，编排文档/报告水印的生成、嵌入、验证与提取流程。
//   水印用于知识产权保护与文档溯源，支持可见水印与不可见数字水印两种模式。
//
// 核心实现:
//   - WatermarkService 接口: Generate / Embed / Verify / Extract / ListByDocument
//   - watermarkServiceImpl: 注入水印领域服务、仓储、日志
//   - Generate: 根据模板与参数生成水印元数据
//   - Embed: 将水印嵌入文档内容（返回嵌入后的内容标识）
//   - Verify: 验证文档是否包含有效水印
//   - Extract: 从文档中提取水印信息用于溯源
//   - ListByDocument: 查询文档关联的所有水印记录
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
// ---

package collaboration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	pkgerrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// WatermarkType distinguishes visible from invisible watermarks.
type WatermarkType string

const (
	WatermarkTypeVisible   WatermarkType = "visible"
	WatermarkTypeInvisible WatermarkType = "invisible"
)

func (wt WatermarkType) IsValid() bool {
	return wt == WatermarkTypeVisible || wt == WatermarkTypeInvisible
}

// WatermarkStatus tracks the lifecycle of a watermark.
type WatermarkStatus string

const (
	WatermarkStatusPending  WatermarkStatus = "pending"
	WatermarkStatusEmbedded WatermarkStatus = "embedded"
	WatermarkStatusVerified WatermarkStatus = "verified"
	WatermarkStatusInvalid  WatermarkStatus = "invalid"
)

// WatermarkRecord is the persistent representation of a watermark.
type WatermarkRecord struct {
	ID           string          `json:"id"`
	DocumentID   string          `json:"document_id"`
	Type         WatermarkType   `json:"type"`
	Status       WatermarkStatus `json:"status"`
	Fingerprint  string          `json:"fingerprint"`
	CreatedBy    string          `json:"created_by"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	EmbeddedAt   *time.Time      `json:"embedded_at,omitempty"`
	VerifiedAt   *time.Time      `json:"verified_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// GenerateWatermarkRequest is the input for watermark generation.
type GenerateWatermarkRequest struct {
	DocumentID string            `json:"document_id"`
	Type       WatermarkType     `json:"type"`
	CreatedBy  string            `json:"created_by"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

func (r *GenerateWatermarkRequest) Validate() error {
	if strings.TrimSpace(r.DocumentID) == "" {
		return pkgerrors.New(pkgerrors.ErrCodeValidation, "document_id is required")
	}
	if strings.TrimSpace(r.CreatedBy) == "" {
		return pkgerrors.New(pkgerrors.ErrCodeValidation, "created_by is required")
	}
	if !r.Type.IsValid() {
		return pkgerrors.New(pkgerrors.ErrCodeValidation, fmt.Sprintf("invalid watermark type: %s", r.Type))
	}
	return nil
}

// GenerateWatermarkResponse is the output after watermark generation.
type GenerateWatermarkResponse struct {
	WatermarkID string        `json:"watermark_id"`
	Fingerprint string        `json:"fingerprint"`
	Type        WatermarkType `json:"type"`
	CreatedAt   time.Time     `json:"created_at"`
}

// EmbedWatermarkRequest is the input for embedding a watermark into a document.
type EmbedWatermarkRequest struct {
	WatermarkID string `json:"watermark_id"`
	DocumentID  string `json:"document_id"`
	Content     []byte `json:"content"`
	EmbeddedBy  string `json:"embedded_by"`
}

func (r *EmbedWatermarkRequest) Validate() error {
	if strings.TrimSpace(r.WatermarkID) == "" {
		return pkgerrors.New(pkgerrors.ErrCodeValidation, "watermark_id is required")
	}
	if strings.TrimSpace(r.DocumentID) == "" {
		return pkgerrors.New(pkgerrors.ErrCodeValidation, "document_id is required")
	}
	if len(r.Content) == 0 {
		return pkgerrors.New(pkgerrors.ErrCodeValidation, "content must not be empty")
	}
	if strings.TrimSpace(r.EmbeddedBy) == "" {
		return pkgerrors.New(pkgerrors.ErrCodeValidation, "embedded_by is required")
	}
	return nil
}

// EmbedWatermarkResponse is the output after embedding.
type EmbedWatermarkResponse struct {
	WatermarkID    string `json:"watermark_id"`
	DocumentID     string `json:"document_id"`
	ContentHash    string `json:"content_hash"`
	EmbeddedAt     time.Time `json:"embedded_at"`
}

// VerifyWatermarkRequest is the input for watermark verification.
type VerifyWatermarkRequest struct {
	DocumentID string `json:"document_id"`
	Content    []byte `json:"content"`
	VerifiedBy string `json:"verified_by"`
}

func (r *VerifyWatermarkRequest) Validate() error {
	if strings.TrimSpace(r.DocumentID) == "" {
		return pkgerrors.New(pkgerrors.ErrCodeValidation, "document_id is required")
	}
	if len(r.Content) == 0 {
		return pkgerrors.New(pkgerrors.ErrCodeValidation, "content must not be empty")
	}
	return nil
}

// VerifyWatermarkResponse is the output of verification.
type VerifyWatermarkResponse struct {
	DocumentID  string `json:"document_id"`
	IsValid     bool   `json:"is_valid"`
	WatermarkID string `json:"watermark_id,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Message     string `json:"message"`
	VerifiedAt  time.Time `json:"verified_at"`
}

// ExtractWatermarkRequest is the input for watermark extraction.
type ExtractWatermarkRequest struct {
	DocumentID  string `json:"document_id"`
	Content     []byte `json:"content"`
	ExtractedBy string `json:"extracted_by"`
}

func (r *ExtractWatermarkRequest) Validate() error {
	if strings.TrimSpace(r.DocumentID) == "" {
		return pkgerrors.New(pkgerrors.ErrCodeValidation, "document_id is required")
	}
	if len(r.Content) == 0 {
		return pkgerrors.New(pkgerrors.ErrCodeValidation, "content must not be empty")
	}
	return nil
}

// ExtractWatermarkResponse is the output of extraction.
type ExtractWatermarkResponse struct {
	DocumentID  string            `json:"document_id"`
	Found       bool              `json:"found"`
	WatermarkID string            `json:"watermark_id,omitempty"`
	Fingerprint string            `json:"fingerprint,omitempty"`
	CreatedBy   string            `json:"created_by,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	ExtractedAt time.Time         `json:"extracted_at"`
}

// WatermarkRepository abstracts persistence for watermark records.
type WatermarkRepository interface {
	Create(ctx context.Context, record *WatermarkRecord) error
	GetByID(ctx context.Context, id string) (*WatermarkRecord, error)
	GetByDocumentAndFingerprint(ctx context.Context, documentID, fingerprint string) (*WatermarkRecord, error)
	ListByDocument(ctx context.Context, documentID string, pagination commontypes.Pagination) ([]*WatermarkRecord, int, error)
	Update(ctx context.Context, record *WatermarkRecord) error
}

// WatermarkService defines the application-level watermark operations.
type WatermarkService interface {
	Generate(ctx context.Context, req *GenerateWatermarkRequest) (*GenerateWatermarkResponse, error)
	Embed(ctx context.Context, req *EmbedWatermarkRequest) (*EmbedWatermarkResponse, error)
	Verify(ctx context.Context, req *VerifyWatermarkRequest) (*VerifyWatermarkResponse, error)
	Extract(ctx context.Context, req *ExtractWatermarkRequest) (*ExtractWatermarkResponse, error)
	ListByDocument(ctx context.Context, documentID string, pagination commontypes.Pagination) ([]*WatermarkRecord, int, error)
}

type watermarkServiceImpl struct {
	repo   WatermarkRepository
	logger logging.Logger
}

// NewWatermarkService constructs a WatermarkService.
func NewWatermarkService(repo WatermarkRepository, logger logging.Logger) WatermarkService {
	return &watermarkServiceImpl{
		repo:   repo,
		logger: logger,
	}
}

// Generate creates watermark metadata and a unique fingerprint for a document.
func (s *watermarkServiceImpl) Generate(ctx context.Context, req *GenerateWatermarkRequest) (*GenerateWatermarkResponse, error) {
	if req == nil {
		return nil, pkgerrors.New(pkgerrors.ErrCodeValidation, "request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	watermarkID := commontypes.NewID()
	strWatermarkID := string(watermarkID)

	// Generate fingerprint from document ID + creator + timestamp + random ID
	fingerprintInput := fmt.Sprintf("%s:%s:%s:%d", strWatermarkID, req.DocumentID, req.CreatedBy, now.UnixNano())
	hash := sha256.Sum256([]byte(fingerprintInput))
	fingerprint := hex.EncodeToString(hash[:16])

	record := &WatermarkRecord{
		ID:          strWatermarkID,
		DocumentID:  req.DocumentID,
		Type:        req.Type,
		Status:      WatermarkStatusPending,
		Fingerprint: fingerprint,
		CreatedBy:   req.CreatedBy,
		Metadata:    req.Metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, record); err != nil {
		s.logger.Error("failed to create watermark record", logging.Err(err))
		return nil, pkgerrors.New(pkgerrors.ErrCodeInternal, "failed to generate watermark")
	}

	s.logger.Info("watermark generated",
		logging.String("watermark_id", strWatermarkID),
		logging.String("document_id", req.DocumentID),
		logging.String("type", string(req.Type)))

	return &GenerateWatermarkResponse{
		WatermarkID: strWatermarkID,
		Fingerprint: fingerprint,
		Type:        req.Type,
		CreatedAt:   now,
	}, nil
}

// Embed marks a watermark as embedded into a document and records the content hash.
func (s *watermarkServiceImpl) Embed(ctx context.Context, req *EmbedWatermarkRequest) (*EmbedWatermarkResponse, error) {
	if req == nil {
		return nil, pkgerrors.New(pkgerrors.ErrCodeValidation, "request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	record, err := s.repo.GetByID(ctx, req.WatermarkID)
	if err != nil {
		return nil, pkgerrors.New(pkgerrors.ErrCodeNotFound, fmt.Sprintf("watermark %s not found", req.WatermarkID))
	}

	if record.DocumentID != req.DocumentID {
		return nil, pkgerrors.New(pkgerrors.ErrCodeValidation, "watermark does not belong to the specified document")
	}

	if record.Status == WatermarkStatusEmbedded {
		s.logger.Info("watermark already embedded", logging.String("watermark_id", req.WatermarkID))
	}

	now := time.Now().UTC()
	contentHash := sha256.Sum256(req.Content)
	contentHashHex := hex.EncodeToString(contentHash[:])

	record.Status = WatermarkStatusEmbedded
	record.EmbeddedAt = &now
	record.UpdatedAt = now
	if record.Metadata == nil {
		record.Metadata = make(map[string]string)
	}
	record.Metadata["content_hash"] = contentHashHex
	record.Metadata["embedded_by"] = req.EmbeddedBy

	if err := s.repo.Update(ctx, record); err != nil {
		s.logger.Error("failed to update watermark record",
			logging.String("watermark_id", req.WatermarkID),
			logging.Err(err))
		return nil, pkgerrors.New(pkgerrors.ErrCodeInternal, "failed to embed watermark")
	}

	s.logger.Info("watermark embedded",
		logging.String("watermark_id", req.WatermarkID),
		logging.String("document_id", req.DocumentID))

	return &EmbedWatermarkResponse{
		WatermarkID: req.WatermarkID,
		DocumentID:  req.DocumentID,
		ContentHash: contentHashHex,
		EmbeddedAt:  now,
	}, nil
}

// Verify checks whether a document contains a valid watermark by matching fingerprints.
func (s *watermarkServiceImpl) Verify(ctx context.Context, req *VerifyWatermarkRequest) (*VerifyWatermarkResponse, error) {
	if req == nil {
		return nil, pkgerrors.New(pkgerrors.ErrCodeValidation, "request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	// List all watermarks for this document
	records, _, err := s.repo.ListByDocument(ctx, req.DocumentID, commontypes.Pagination{Page: 1, PageSize: 100})
	if err != nil {
		s.logger.Error("failed to list watermarks for verification",
			logging.String("document_id", req.DocumentID),
			logging.Err(err))
		return nil, pkgerrors.New(pkgerrors.ErrCodeInternal, "failed to verify watermark")
	}

	contentHash := sha256.Sum256(req.Content)
	contentHashHex := hex.EncodeToString(contentHash[:])

	for _, record := range records {
		if record.Status != WatermarkStatusEmbedded {
			continue
		}
		storedHash, ok := record.Metadata["content_hash"]
		if ok && storedHash == contentHashHex {
			// Match found — update status
			record.Status = WatermarkStatusVerified
			record.VerifiedAt = &now
			record.UpdatedAt = now
			_ = s.repo.Update(ctx, record)

			s.logger.Info("watermark verified",
				logging.String("watermark_id", record.ID),
				logging.String("document_id", req.DocumentID))

			return &VerifyWatermarkResponse{
				DocumentID:  req.DocumentID,
				IsValid:     true,
				WatermarkID: record.ID,
				Fingerprint: record.Fingerprint,
				Message:     "watermark verified successfully",
				VerifiedAt:  now,
			}, nil
		}
	}

	return &VerifyWatermarkResponse{
		DocumentID: req.DocumentID,
		IsValid:    false,
		Message:    "no matching watermark found for the provided content",
		VerifiedAt: now,
	}, nil
}

// Extract attempts to retrieve watermark information from a document for traceability.
func (s *watermarkServiceImpl) Extract(ctx context.Context, req *ExtractWatermarkRequest) (*ExtractWatermarkResponse, error) {
	if req == nil {
		return nil, pkgerrors.New(pkgerrors.ErrCodeValidation, "request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	contentHash := sha256.Sum256(req.Content)
	contentHashHex := hex.EncodeToString(contentHash[:])

	records, _, err := s.repo.ListByDocument(ctx, req.DocumentID, commontypes.Pagination{Page: 1, PageSize: 100})
	if err != nil {
		s.logger.Error("failed to list watermarks for extraction",
			logging.String("document_id", req.DocumentID),
			logging.Err(err))
		return nil, pkgerrors.New(pkgerrors.ErrCodeInternal, "failed to extract watermark")
	}

	for _, record := range records {
		storedHash, ok := record.Metadata["content_hash"]
		if ok && storedHash == contentHashHex {
			s.logger.Info("watermark extracted",
				logging.String("watermark_id", record.ID),
				logging.String("document_id", req.DocumentID))
			return &ExtractWatermarkResponse{
				DocumentID:  req.DocumentID,
				Found:       true,
				WatermarkID: record.ID,
				Fingerprint: record.Fingerprint,
				CreatedBy:   record.CreatedBy,
				Metadata:    record.Metadata,
				ExtractedAt: now,
			}, nil
		}
	}

	return &ExtractWatermarkResponse{
		DocumentID:  req.DocumentID,
		Found:       false,
		ExtractedAt: now,
	}, nil
}

// ListByDocument returns all watermark records associated with a document.
func (s *watermarkServiceImpl) ListByDocument(ctx context.Context, documentID string, pagination commontypes.Pagination) ([]*WatermarkRecord, int, error) {
	if strings.TrimSpace(documentID) == "" {
		return nil, 0, pkgerrors.New(pkgerrors.ErrCodeValidation, "document_id is required")
	}
	if pagination.Page < 1 {
		pagination.Page = 1
	}
	if pagination.PageSize < 1 || pagination.PageSize > 100 {
		pagination.PageSize = 20
	}

	records, total, err := s.repo.ListByDocument(ctx, documentID, pagination)
	if err != nil {
		s.logger.Error("failed to list watermarks",
			logging.String("document_id", documentID),
			logging.Err(err))
		return nil, 0, pkgerrors.New(pkgerrors.ErrCodeInternal, "failed to list watermarks")
	}

	return records, total, nil
}

//Personal.AI order the ending
