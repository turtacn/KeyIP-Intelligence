// internal/application/patent_mining/chem_extraction.go

package patent_mining

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	storageminio "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/storage/minio"
	chemextractor "github.com/turtacn/KeyIP-Intelligence/internal/intelligence/chem_extractor"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// DefaultConfidenceThreshold is the minimum confidence score for an extracted entity
// to be accepted without manual review.
const DefaultConfidenceThreshold = 0.6

// DocumentFormat enumerates supported input document formats.
type DocumentFormat string

const (
	DocumentFormatPDF  DocumentFormat = "pdf"
	DocumentFormatDOCX DocumentFormat = "docx"
	DocumentFormatXML  DocumentFormat = "xml"
	DocumentFormatHTML DocumentFormat = "html"
)

// ValidDocumentFormats returns the set of supported formats for validation.
func ValidDocumentFormats() map[DocumentFormat]struct{} {
	return map[DocumentFormat]struct{}{
		DocumentFormatPDF:  {},
		DocumentFormatDOCX: {},
		DocumentFormatXML:  {},
		DocumentFormatHTML: {},
	}
}

// EntityType enumerates the kinds of chemical entities that can be extracted.
type EntityType string

const (
	EntityTypeSMILES    EntityType = "smiles"
	EntityTypeInChI     EntityType = "inchi"
	EntityTypeChemName  EntityType = "chemical_name"
	EntityTypeCAS       EntityType = "cas_number"
	EntityTypeFormula   EntityType = "molecular_formula"
)

// ReviewStatus indicates whether an extracted entity requires human review.
type ReviewStatus string

const (
	ReviewStatusAccepted    ReviewStatus = "accepted"
	ReviewStatusNeedsReview ReviewStatus = "needs_review"
	ReviewStatusRejected    ReviewStatus = "rejected"
)

// JobStatus represents the lifecycle state of an asynchronous batch extraction job.
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusRunning    JobStatus = "running"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCancelled  JobStatus = "cancelled"
)

// ---------------------------------------------------------------------------
// Request / Response DTOs
// ---------------------------------------------------------------------------

// ExtractionRequest represents a request to extract chemical entities from a single document.
type ExtractionRequest struct {
	DocumentID          string         `json:"document_id" validate:"required"`
	DocumentStoragePath string         `json:"document_storage_path" validate:"required"`
	Format              DocumentFormat `json:"format" validate:"required"`
	PatentID            string         `json:"patent_id,omitempty"`
	ConfidenceThreshold float64        `json:"confidence_threshold,omitempty"`
	EntityTypes         []EntityType   `json:"entity_types,omitempty"`
}

// TextExtractionRequest represents a request to extract chemical entities from raw text.
type TextExtractionRequest struct {
	Text                string       `json:"text" validate:"required"`
	ConfidenceThreshold float64      `json:"confidence_threshold,omitempty"`
	EntityTypes         []EntityType `json:"entity_types,omitempty"`
}

// BatchExtractionRequest represents a request to extract from multiple documents asynchronously.
type BatchExtractionRequest struct {
	Documents           []ExtractionRequest `json:"documents" validate:"required,min=1"`
	ConfidenceThreshold float64             `json:"confidence_threshold,omitempty"`
}

// ExtractedEntity is a single chemical entity found during extraction.
type ExtractedEntity struct {
	EntityType   EntityType   `json:"entity_type"`
	RawValue     string       `json:"raw_value"`
	Canonical    string       `json:"canonical,omitempty"`
	InChIKey     string       `json:"inchi_key,omitempty"`
	Confidence   float64      `json:"confidence"`
	ReviewStatus ReviewStatus `json:"review_status"`
	SourcePage   int          `json:"source_page,omitempty"`
	SourceOffset int          `json:"source_offset,omitempty"`
	MoleculeID   string       `json:"molecule_id,omitempty"`
	IsDuplicate  bool         `json:"is_duplicate"`
}

// ExtractionResult is the response for a single-document or text extraction.
type ExtractionResult struct {
	RequestID       string            `json:"request_id"`
	DocumentID      string            `json:"document_id,omitempty"`
	Entities        []ExtractedEntity `json:"entities"`
	TotalExtracted  int               `json:"total_extracted"`
	TotalAccepted   int               `json:"total_accepted"`
	TotalDuplicated int               `json:"total_duplicated"`
	TotalReview     int               `json:"total_review"`
	DurationMs      int64             `json:"duration_ms"`
	ExtractedAt     time.Time         `json:"extracted_at"`
}

// ExtractionJob tracks the state of an asynchronous batch extraction.
type ExtractionJob struct {
	JobID           string             `json:"job_id"`
	Status          JobStatus          `json:"status"`
	TotalDocuments  int                `json:"total_documents"`
	ProcessedCount  int                `json:"processed_count"`
	FailedCount     int                `json:"failed_count"`
	Results         []ExtractionResult `json:"results,omitempty"`
	ErrorMessage    string             `json:"error_message,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	CompletedAt     *time.Time         `json:"completed_at,omitempty"`
}

// ListExtractionOpts provides pagination and filtering for extraction history queries.
type ListExtractionOpts struct {
	commontypes.Pagination
	DocumentID string     `json:"document_id,omitempty"`
	PatentID   string     `json:"patent_id,omitempty"`
	Since      *time.Time `json:"since,omitempty"`
	Until      *time.Time `json:"until,omitempty"`
}

// ExtractionHistoryPage is a paginated list of past extraction results.
type ExtractionHistoryPage struct {
	Items      []ExtractionResult     `json:"items"`
	Pagination commontypes.Pagination `json:"pagination"`
	Total      int64                  `json:"total"`
}

// ---------------------------------------------------------------------------
// Service Interface
// ---------------------------------------------------------------------------

// ChemExtractionService defines the application-layer contract for chemical entity extraction.
type ChemExtractionService interface {
	// ExtractFromDocument extracts chemical entities from a stored document.
	ExtractFromDocument(ctx context.Context, req *ExtractionRequest) (*ExtractionResult, error)

	// ExtractFromText extracts chemical entities from raw text input.
	ExtractFromText(ctx context.Context, req *TextExtractionRequest) (*ExtractionResult, error)

	// BatchExtract creates an asynchronous job to extract from multiple documents.
	BatchExtract(ctx context.Context, req *BatchExtractionRequest) (*ExtractionJob, error)

	// GetExtractionJob returns the current state of a batch extraction job.
	GetExtractionJob(ctx context.Context, jobID string) (*ExtractionJob, error)

	// ListExtractionHistory returns paginated extraction history.
	ListExtractionHistory(ctx context.Context, opts *ListExtractionOpts) (*ExtractionHistoryPage, error)
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

// chemExtractionServiceImpl orchestrates chemical extraction across intelligence and domain layers.
type chemExtractionServiceImpl struct {
	extractor   chemextractor.ChemicalExtractor
	molService  molecule.Service
	molRepo     molecule.Repository
	patentRepo  patent.Repository
	storage     storageminio.ObjectRepository
	logger      logging.Logger
	jobs        map[string]*ExtractionJob
	jobsMu      sync.RWMutex
}

// NewChemExtractionService constructs a ChemExtractionService with all required dependencies.
func NewChemExtractionService(
	extractor chemextractor.ChemicalExtractor,
	molService molecule.Service,
	molRepo molecule.Repository,
	patentRepo patent.Repository,
	storage storageminio.ObjectRepository,
	logger logging.Logger,
) ChemExtractionService {
	if extractor == nil {
		panic("chem_extraction: extractor must not be nil")
	}
	if molService == nil {
		panic("chem_extraction: molService must not be nil")
	}
	if molRepo == nil {
		panic("chem_extraction: molRepo must not be nil")
	}
	if patentRepo == nil {
		panic("chem_extraction: patentRepo must not be nil")
	}
	if storage == nil {
		panic("chem_extraction: storage must not be nil")
	}
	if logger == nil {
		panic("chem_extraction: logger must not be nil")
	}
	return &chemExtractionServiceImpl{
		extractor:  extractor,
		molService: molService,
		molRepo:    molRepo,
		patentRepo: patentRepo,
		storage:    storage,
		logger:     logger,
		jobs:       make(map[string]*ExtractionJob),
	}
}

// effectiveThreshold returns the confidence threshold to use, falling back to the default.
func effectiveThreshold(requested float64) float64 {
	if requested > 0 && requested <= 1.0 {
		return requested
	}
	return DefaultConfidenceThreshold
}

// validateExtractionRequest performs input validation on an ExtractionRequest.
func validateExtractionRequest(req *ExtractionRequest) error {
	if req == nil {
		return errors.NewValidationOp("extraction_request", "must not be nil")
	}
	if req.DocumentID == "" {
		return errors.NewValidationOp("extraction_request", "document_id is required")
	}
	if req.DocumentStoragePath == "" {
		return errors.NewValidationOp("extraction_request", "document_storage_path is required")
	}
	if _, ok := ValidDocumentFormats()[req.Format]; !ok {
		return errors.NewValidationOp("extraction_request", fmt.Sprintf("unsupported document format: %s", req.Format))
	}
	return nil
}

// validateTextExtractionRequest performs input validation on a TextExtractionRequest.
func validateTextExtractionRequest(req *TextExtractionRequest) error {
	if req == nil {
		return errors.NewValidationOp("text_extraction_request", "must not be nil")
	}
	if req.Text == "" {
		return errors.NewValidationOp("text_extraction_request", "text is required")
	}
	return nil
}

// ExtractFromDocument fetches a document from object storage, runs the intelligence-layer
// extractor, standardizes and validates each entity, deduplicates by InChIKey, persists
// new molecules, and optionally associates them with a patent.
func (s *chemExtractionServiceImpl) ExtractFromDocument(ctx context.Context, req *ExtractionRequest) (*ExtractionResult, error) {
	if err := validateExtractionRequest(req); err != nil {
		return nil, err
	}

	start := time.Now()
	threshold := effectiveThreshold(req.ConfidenceThreshold)

	s.logger.Info("starting document extraction",
		logging.String("document_id", req.DocumentID),
		logging.String("format", string(req.Format)),
		logging.Float64("threshold", threshold),
	)

	// 1. Fetch document bytes from object storage.
	docBytes, err := s.storage.Get(ctx, req.DocumentStoragePath)
	if err != nil {
		s.logger.Error("failed to fetch document from storage",
			logging.String("path", req.DocumentStoragePath),
			logging.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to fetch document from storage")
	}

	// 2. Invoke intelligence-layer extractor.
	// Currently assumes text data. Future: Integrate document parsing/OCR.
	textData := string(docBytes)
	extractionResult, err := s.extractor.Extract(ctx, textData)
	if err != nil {
		s.logger.Error("intelligence extractor failed",
			logging.String("document_id", req.DocumentID),
			logging.Error(err),
		)
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "chemical extraction failed")
	}

	// 3. Process each raw entity: standardize, validate, deduplicate, persist.
	result := &ExtractionResult{
		RequestID:  string(commontypes.NewID()),
		DocumentID: req.DocumentID,
		ExtractedAt: time.Now(),
	}

	seen := make(map[string]struct{}) // InChIKey dedup within this extraction

	for _, raw := range extractionResult.Entities {
		entity := ExtractedEntity{
			EntityType:   EntityType(raw.EntityType),
			RawValue:     raw.Text,
			Confidence:   raw.Confidence,
			SourcePage:   1,
			SourceOffset: raw.StartOffset,
		}

		// Confidence filtering.
		if raw.Confidence < threshold {
			entity.ReviewStatus = ReviewStatusNeedsReview
			result.TotalReview++
			result.Entities = append(result.Entities, entity)
			continue
		}

		// Standardize: resolve to canonical SMILES and compute InChIKey.
		resolved, err := s.extractor.Resolve(ctx, raw)
		if err != nil {
			s.logger.Warn("failed to resolve entity, marking for review",
				logging.String("raw", raw.Text),
				logging.String("type", string(raw.EntityType)),
				logging.Error(err),
			)
			entity.ReviewStatus = ReviewStatusNeedsReview
			result.TotalReview++
			result.Entities = append(result.Entities, entity)
			continue
		}
		entity.Canonical = resolved.SMILES
		entity.InChIKey = resolved.InChIKey

		// Deduplicate within this batch.
		if _, dup := seen[resolved.InChIKey]; dup {
			entity.IsDuplicate = true
			entity.ReviewStatus = ReviewStatusAccepted
			result.TotalDuplicated++
			result.Entities = append(result.Entities, entity)
			continue
		}
		seen[resolved.InChIKey] = struct{}{}

		// Deduplicate against existing repository.
		existing, err := s.molRepo.FindByInChIKey(ctx, resolved.InChIKey)
		if err != nil && !errors.IsNotFound(err) {
			s.logger.Error("repository lookup failed",
				logging.String("inchi_key", resolved.InChIKey),
				logging.Error(err),
			)
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "molecule repository lookup failed")
		}
		if existing != nil {
			entity.IsDuplicate = true
			entity.MoleculeID = existing.ID.String()
			entity.ReviewStatus = ReviewStatusAccepted
			result.TotalDuplicated++
		} else {
			// Persist new molecule via domain service.
			mol, createErr := s.molService.CreateFromSMILES(ctx, resolved.SMILES, map[string]string{
				"source_document": req.DocumentID,
				"source_page":    fmt.Sprintf("%d", 1),
				"extraction_id":  result.RequestID,
			})
			if createErr != nil {
				s.logger.Error("failed to persist molecule",
					logging.String("smiles", resolved.SMILES),
					logging.Error(createErr),
				)
				entity.ReviewStatus = ReviewStatusNeedsReview
				result.TotalReview++
				result.Entities = append(result.Entities, entity)
				continue
			}
			entity.MoleculeID = mol.ID.String()
			entity.ReviewStatus = ReviewStatusAccepted
			result.TotalAccepted++
		}

		// Associate molecule with patent if patent_id is provided.
		if req.PatentID != "" && entity.MoleculeID != "" {
			pat, getErr := s.patentRepo.FindByID(ctx, req.PatentID)
			if getErr == nil && pat != nil {
				if addErr := pat.AddMolecule(entity.MoleculeID); addErr == nil {
					if saveErr := s.patentRepo.Save(ctx, pat); saveErr != nil {
						s.logger.Warn("failed to save patent association",
							logging.String("patent_id", req.PatentID),
							logging.Error(saveErr),
						)
					}
				}
			} else {
				s.logger.Warn("failed to find patent for association",
					logging.String("patent_id", req.PatentID),
					logging.Error(getErr),
				)
			}
		}

		result.Entities = append(result.Entities, entity)
	}

	result.TotalExtracted = len(result.Entities)
	result.DurationMs = time.Since(start).Milliseconds()

	s.logger.Info("document extraction completed",
		logging.String("document_id", req.DocumentID),
		logging.Int("total_extracted", result.TotalExtracted),
		logging.Int("total_accepted", result.TotalAccepted),
		logging.Int("total_duplicated", result.TotalDuplicated),
		logging.Int("total_review", result.TotalReview),
		logging.Int64("duration_ms", result.DurationMs),
	)

	return result, nil
}

// ExtractFromText extracts chemical entities from raw text without document storage interaction.
func (s *chemExtractionServiceImpl) ExtractFromText(ctx context.Context, req *TextExtractionRequest) (*ExtractionResult, error) {
	if err := validateTextExtractionRequest(req); err != nil {
		return nil, err
	}

	start := time.Now()
	threshold := effectiveThreshold(req.ConfidenceThreshold)

	s.logger.Info("starting text extraction",
		logging.Int("text_length", len(req.Text)),
		logging.Float64("threshold", threshold),
	)

	// Invoke NER on raw text.
	extractionResult, err := s.extractor.Extract(ctx, req.Text)
	if err != nil {
		s.logger.Error("text extraction failed", logging.Error(err))
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "text chemical extraction failed")
	}

	result := &ExtractionResult{
		RequestID:   string(commontypes.NewID()),
		ExtractedAt: time.Now(),
	}

	seen := make(map[string]struct{})

	for _, raw := range extractionResult.Entities {
		entity := ExtractedEntity{
			EntityType:   EntityType(raw.EntityType),
			RawValue:     raw.Text,
			Confidence:   raw.Confidence,
			SourceOffset: raw.StartOffset,
		}

		if raw.Confidence < threshold {
			entity.ReviewStatus = ReviewStatusNeedsReview
			result.TotalReview++
			result.Entities = append(result.Entities, entity)
			continue
		}

		resolved, err := s.extractor.Resolve(ctx, raw)
		if err != nil {
			entity.ReviewStatus = ReviewStatusNeedsReview
			result.TotalReview++
			result.Entities = append(result.Entities, entity)
			continue
		}
		entity.Canonical = resolved.SMILES
		entity.InChIKey = resolved.InChIKey

		if _, dup := seen[resolved.InChIKey]; dup {
			entity.IsDuplicate = true
			result.TotalDuplicated++
		} else {
			seen[resolved.InChIKey] = struct{}{}
			result.TotalAccepted++
		}
		entity.ReviewStatus = ReviewStatusAccepted
		result.Entities = append(result.Entities, entity)
	}

	result.TotalExtracted = len(result.Entities)
	result.DurationMs = time.Since(start).Milliseconds()

	s.logger.Info("text extraction completed",
		logging.Int("total_extracted", result.TotalExtracted),
		logging.Int64("duration_ms", result.DurationMs),
	)

	return result, nil
}

// BatchExtract creates an asynchronous extraction job that processes multiple documents.
// The job runs in a background goroutine; callers poll via GetExtractionJob.
func (s *chemExtractionServiceImpl) BatchExtract(ctx context.Context, req *BatchExtractionRequest) (*ExtractionJob, error) {
	if req == nil || len(req.Documents) == 0 {
		return nil, errors.NewValidationOp("batch_extract", "batch extraction requires at least one document")
	}

	for i := range req.Documents {
		if err := validateExtractionRequest(&req.Documents[i]); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeValidation, fmt.Sprintf("document[%d] validation failed", i))
		}
		if req.ConfidenceThreshold > 0 && req.Documents[i].ConfidenceThreshold == 0 {
			req.Documents[i].ConfidenceThreshold = req.ConfidenceThreshold
		}
	}

	now := time.Now()
	job := &ExtractionJob{
		JobID:          string(commontypes.NewID()),
		Status:         JobStatusPending,
		TotalDocuments: len(req.Documents),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.jobsMu.Lock()
	s.jobs[job.JobID] = job
	s.jobsMu.Unlock()

	s.logger.Info("batch extraction job created",
		logging.String("job_id", job.JobID),
		logging.Int("total_documents", job.TotalDocuments),
	)

	// Launch background processing. We detach from the request context to allow
	// the job to outlive the HTTP request, but respect a generous timeout.
	go s.runBatchJob(job, req.Documents)

	return job, nil
}

// runBatchJob processes each document sequentially, updating job state as it progresses.
func (s *chemExtractionServiceImpl) runBatchJob(job *ExtractionJob, docs []ExtractionRequest) {
	s.updateJobStatus(job, JobStatusRunning)

	// Use a background context with a generous timeout per document.
	ctx := context.Background()

	for i := range docs {
		docCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		result, err := s.ExtractFromDocument(docCtx, &docs[i])
		cancel()

		s.jobsMu.Lock()
		if err != nil {
			s.logger.Error("batch extraction: document failed",
				logging.String("job_id", job.JobID),
				logging.String("document_id", docs[i].DocumentID),
				logging.Error(err),
			)
			job.FailedCount++
		} else {
			job.Results = append(job.Results, *result)
		}
		job.ProcessedCount++
		job.UpdatedAt = time.Now()
		s.jobsMu.Unlock()
	}

	s.jobsMu.Lock()
	now := time.Now()
	job.CompletedAt = &now
	if job.FailedCount == job.TotalDocuments {
		job.Status = JobStatusFailed
		job.ErrorMessage = "all documents failed extraction"
	} else {
		job.Status = JobStatusCompleted
	}
	job.UpdatedAt = now
	s.jobsMu.Unlock()

	s.logger.Info("batch extraction job completed",
		logging.String("job_id", job.JobID),
		logging.Int("processed", job.ProcessedCount),
		logging.Int("failed", job.FailedCount),
	)
}

// updateJobStatus is a helper to safely transition job state.
func (s *chemExtractionServiceImpl) updateJobStatus(job *ExtractionJob, status JobStatus) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	job.Status = status
	job.UpdatedAt = time.Now()
}

// GetExtractionJob returns the current state of a batch extraction job.
func (s *chemExtractionServiceImpl) GetExtractionJob(ctx context.Context, jobID string) (*ExtractionJob, error) {
	if jobID == "" {
		return nil, errors.NewValidationOp("get_extraction_job", "job_id is required")
	}

	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return nil, errors.NewNotFoundOp("get_extraction_job", fmt.Sprintf("extraction job not found: %s", jobID))
	}

	// Return a shallow copy to avoid data races on read.
	snapshot := *job
	return &snapshot, nil
}

// ListExtractionHistory returns a paginated view of past extraction results.
func (s *chemExtractionServiceImpl) ListExtractionHistory(ctx context.Context, opts *ListExtractionOpts) (*ExtractionHistoryPage, error) {
	if opts == nil {
		opts = &ListExtractionOpts{}
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}

	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	// Collect all results across completed jobs, applying filters.
	var all []ExtractionResult
	for _, job := range s.jobs {
		for _, r := range job.Results {
			if opts.DocumentID != "" && r.DocumentID != opts.DocumentID {
				continue
			}
			if opts.Since != nil && r.ExtractedAt.Before(*opts.Since) {
				continue
			}
			if opts.Until != nil && r.ExtractedAt.After(*opts.Until) {
				continue
			}
			all = append(all, r)
		}
	}

	total := int64(len(all))
	start := (opts.Page - 1) * opts.PageSize
	if start >= int(total) {
		return &ExtractionHistoryPage{
			Items:      []ExtractionResult{},
			Pagination: opts.Pagination,
			Total:      total,
		}, nil
	}
	end := start + opts.PageSize
	if end > int(total) {
		end = int(total)
	}

	return &ExtractionHistoryPage{
		Items:      all[start:end],
		Pagination: opts.Pagination,
		Total:      total,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// toExtractorEntityTypes converts application-layer EntityType slice to intelligence-layer strings.
func toExtractorEntityTypes(types []EntityType) []string {
	if len(types) == 0 {
		return nil
	}
	out := make([]string, len(types))
	for i, t := range types {
		out[i] = string(t)
	}
	return out
}

// Compile-time interface satisfaction check.
var _ ChemExtractionService = (*chemExtractionServiceImpl)(nil)
