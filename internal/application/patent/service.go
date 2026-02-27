// Package patent provides the application-level service for patent operations.
// This package serves as the interface between HTTP/gRPC handlers and domain logic.
package patent

import (
	"context"
	"time"

	"github.com/google/uuid"
	domainPatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Service defines the interface for patent application operations.
type Service interface {
	Create(ctx context.Context, input *CreateInput) (*Patent, error)
	GetByID(ctx context.Context, id string) (*Patent, error)
	List(ctx context.Context, input *ListInput) (*ListResult, error)
	Update(ctx context.Context, input *UpdateInput) (*Patent, error)
	Delete(ctx context.Context, id string, userID string) error
	Search(ctx context.Context, input *SearchInput) (*SearchResult, error)
	AdvancedSearch(ctx context.Context, input *AdvancedSearchInput) (*SearchResult, error)
	GetStats(ctx context.Context, input *StatsInput) (*Stats, error)
}

// CreateInput contains input for creating a patent.
type CreateInput struct {
	Title           string
	Abstract        string
	ApplicationNo   string
	PublicationNo   string
	Applicant       string
	Inventors       []string
	IPCCodes        []string
	FilingDate      string
	PublicationDate string
	Claims          string
	Description     string
	Jurisdiction    string
	UserID          string
}

// UpdateInput contains input for updating a patent.
type UpdateInput struct {
	ID          string
	Title       *string
	Abstract    *string
	Claims      *string
	Description *string
	IPCCodes    []string
	UserID      string
}

// ListInput contains input for listing patents.
type ListInput struct {
	Page         int
	PageSize     int
	Jurisdiction string
	Applicant    string
}

// SearchInput contains input for searching patents.
type SearchInput struct {
	Query     string
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}

// AdvancedSearchInput contains input for advanced patent search.
type AdvancedSearchInput struct {
	Title          string
	Abstract       string
	Applicant      string
	Inventor       string
	IPCCode        string
	Jurisdiction   string
	FilingDateFrom string
	FilingDateTo   string
	Keywords       []string
	Page           int
	PageSize       int
}

// StatsInput contains input for patent statistics.
type StatsInput struct {
	Jurisdiction string
	Applicant    string
}

// Patent represents an application-level patent DTO.
type Patent struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Abstract        string    `json:"abstract"`
	ApplicationNo   string    `json:"application_no"`
	PublicationNo   string    `json:"publication_no,omitempty"`
	Applicant       string    `json:"applicant"`
	Inventors       []string  `json:"inventors"`
	IPCCodes        []string  `json:"ipc_codes,omitempty"`
	FilingDate      string    `json:"filing_date"`
	PublicationDate string    `json:"publication_date,omitempty"`
	Claims          string    `json:"claims,omitempty"`
	Description     string    `json:"description,omitempty"`
	Jurisdiction    string    `json:"jurisdiction"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ListResult represents a paginated list of patents.
type ListResult struct {
	Patents    []*Patent `json:"patents"`
	Total      int64     `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	TotalPages int       `json:"total_pages"`
}

// SearchResult represents patent search results.
type SearchResult struct {
	Patents    []*Patent `json:"patents"`
	Total      int64     `json:"total"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	TotalPages int       `json:"total_pages"`
}

// Stats represents patent statistics.
type Stats struct {
	TotalPatents         int64            `json:"total_patents"`
	ByJurisdiction       map[string]int64 `json:"by_jurisdiction"`
	ByYear               map[string]int64 `json:"by_year"`
	TopApplicants        []ApplicantStat  `json:"top_applicants"`
	TopIPCCodes          []IPCStat        `json:"top_ipc_codes"`
	AverageClaimsPerDoc  float64          `json:"average_claims_per_doc"`
	ActivePatentPerc     float64          `json:"active_patent_perc"`
}

// ApplicantStat represents applicant statistics.
type ApplicantStat struct {
	Applicant string `json:"applicant"`
	Count     int64  `json:"count"`
}

// IPCStat represents IPC code statistics.
type IPCStat struct {
	IPCCode string `json:"ipc_code"`
	Count   int64  `json:"count"`
}

// serviceImpl implements the Service interface.
type serviceImpl struct {
	repo   domainPatent.PatentRepository
	logger logging.Logger
}

// NewService creates a new patent application service.
func NewService(repo domainPatent.PatentRepository, logger logging.Logger) Service {
	return &serviceImpl{
		repo:   repo,
		logger: logger,
	}
}

func (s *serviceImpl) Create(ctx context.Context, input *CreateInput) (*Patent, error) {
	if input.Title == "" {
		return nil, errors.NewValidationError("title", "title is required")
	}
	if input.ApplicationNo == "" {
		return nil, errors.NewValidationError("application_no", "application_no is required")
	}

	// Parse filing date, use current time if not provided
	filingDate := time.Now().UTC()
	if input.FilingDate != "" {
		parsedDate, err := time.Parse("2006-01-02", input.FilingDate)
		if err == nil {
			filingDate = parsedDate
		}
	}

	// Convert jurisdiction to office
	office := domainPatent.OfficeCNIPA
	switch input.Jurisdiction {
	case "US":
		office = domainPatent.OfficeUSPTO
	case "EP":
		office = domainPatent.OfficeEPO
	case "JP":
		office = domainPatent.OfficeJPO
	case "KR":
		office = domainPatent.OfficeKIPO
	}

	patent, err := domainPatent.NewPatent(
		input.ApplicationNo,
		input.Title,
		office,
		filingDate,
	)
	if err != nil {
		return nil, errors.NewValidationError("patent", err.Error())
	}

	patent.Abstract = input.Abstract
	patent.AssigneeName = input.Applicant
	patent.IPCCodes = input.IPCCodes
	patent.Jurisdiction = input.Jurisdiction

	// Convert inventors
	inventors := make([]*domainPatent.Inventor, len(input.Inventors))
	for i, inv := range input.Inventors {
		inventors[i] = &domainPatent.Inventor{
			Name:     inv,
			Sequence: i + 1,
		}
	}
	patent.Inventors = inventors

	if err := s.repo.Save(ctx, patent); err != nil {
		s.logger.Error("failed to create patent")
		return nil, err
	}

	return domainToDTO(patent), nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id string) (*Patent, error) {
	patent, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return domainToDTO(patent), nil
}

func (s *serviceImpl) List(ctx context.Context, input *ListInput) (*ListResult, error) {
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 20
	}
	if input.PageSize > 100 {
		input.PageSize = 100
	}

	offset := (input.Page - 1) * input.PageSize
	criteria := domainPatent.PatentSearchCriteria{
		Offset: offset,
		Limit:  input.PageSize,
	}
	if input.Jurisdiction != "" {
		// Assuming we can convert string to []PatentOffice or add string field if flexible
		// For now simple mapping if exists
		// criteria.Offices = ...
	}

	result, err := s.repo.Search(ctx, criteria)
	if err != nil {
		return nil, err
	}

	dtos := make([]*Patent, len(result.Patents))
	for i, p := range result.Patents {
		dtos[i] = domainToDTO(p)
	}

	return &ListResult{
		Patents:    dtos,
		Total:      result.Total,
		Page:       input.Page,
		PageSize:   input.PageSize,
		TotalPages: result.PageCount(),
	}, nil
}

func (s *serviceImpl) Update(ctx context.Context, input *UpdateInput) (*Patent, error) {
	patent, err := s.repo.FindByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	if input.Title != nil {
		patent.Title = *input.Title
	}
	if input.Abstract != nil {
		patent.Abstract = *input.Abstract
	}
	if len(input.IPCCodes) > 0 {
		patent.IPCCodes = input.IPCCodes
	}

	if err := s.repo.Save(ctx, patent); err != nil {
		return nil, err
	}

	return domainToDTO(patent), nil
}

func (s *serviceImpl) Delete(ctx context.Context, id string, userID string) error {
	// Assuming SoftDelete takes string ID now based on new interface, or convert
	// The new interface uses string ID for FindByID, checking Delete...
	// Repo interface: Delete(ctx context.Context, id string) error.
	return s.repo.Delete(ctx, id)
}

func (s *serviceImpl) Search(ctx context.Context, input *SearchInput) (*SearchResult, error) {
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 20
	}

	offset := (input.Page - 1) * input.PageSize
	criteria := domainPatent.PatentSearchCriteria{
		TitleKeywords: []string{input.Query}, // Simple mapping
		Offset:        offset,
		Limit:         input.PageSize,
		SortBy:        input.SortBy,
	}

	result, err := s.repo.Search(ctx, criteria)
	if err != nil {
		return nil, err
	}

	dtos := make([]*Patent, len(result.Patents))
	for i, p := range result.Patents {
		dtos[i] = domainToDTO(p)
	}

	return &SearchResult{
		Patents:    dtos,
		Total:      result.Total,
		Page:       input.Page,
		PageSize:   input.PageSize,
		TotalPages: result.PageCount(),
	}, nil
}

func (s *serviceImpl) AdvancedSearch(ctx context.Context, input *AdvancedSearchInput) (*SearchResult, error) {
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 20
	}

	offset := (input.Page - 1) * input.PageSize
	criteria := domainPatent.PatentSearchCriteria{
		Offset: offset,
		Limit:  input.PageSize,
	}
	if input.IPCCode != "" {
		criteria.IPCCodes = []string{input.IPCCode}
	}
	// Add other mappings...

	result, err := s.repo.Search(ctx, criteria)
	if err != nil {
		return nil, err
	}

	dtos := make([]*Patent, len(result.Patents))
	for i, p := range result.Patents {
		dtos[i] = domainToDTO(p)
	}

	return &SearchResult{
		Patents:    dtos,
		Total:      result.Total,
		Page:       input.Page,
		PageSize:   input.PageSize,
		TotalPages: result.PageCount(),
	}, nil
}

func (s *serviceImpl) GetStats(ctx context.Context, input *StatsInput) (*Stats, error) {
	byJurisdiction, _ := s.repo.CountByJurisdiction(ctx)
	byStatus, _ := s.repo.CountByStatus(ctx)

	var total int64
	for _, count := range byStatus {
		total += count
	}

	return &Stats{
		TotalPatents:        total,
		ByJurisdiction:      byJurisdiction,
		ByYear:              map[string]int64{},
		TopApplicants:       []ApplicantStat{},
		TopIPCCodes:         []IPCStat{},
		AverageClaimsPerDoc: 0,
		ActivePatentPerc:    0,
	}, nil
}

func domainToDTO(patent *domainPatent.Patent) *Patent {
	if patent == nil {
		return nil
	}

	// Convert inventors to string slice
	inventors := make([]string, len(patent.Inventors))
	for i, inv := range patent.Inventors {
		inventors[i] = inv.Name
	}

	// Format dates
	var filingDate, publicationDate string
	if patent.Dates.FilingDate != nil {
		filingDate = patent.Dates.FilingDate.Format("2006-01-02")
	}
	if patent.Dates.PublicationDate != nil {
		publicationDate = patent.Dates.PublicationDate.Format("2006-01-02")
	}

	return &Patent{
		ID:              patent.ID.String(),
		Title:           patent.Title,
		Abstract:        patent.Abstract,
		ApplicationNo:   patent.ApplicationNumber,
		PublicationNo:   patent.PatentNumber,
		Applicant:       patent.AssigneeName,
		Inventors:       inventors,
		IPCCodes:        patent.IPCCodes,
		FilingDate:      filingDate,
		PublicationDate: publicationDate,
		Jurisdiction:    patent.Jurisdiction,
		Status:          patent.Status.String(),
		CreatedAt:       patent.CreatedAt,
		UpdatedAt:       patent.UpdatedAt,
	}
}

//Personal.AI order the ending
