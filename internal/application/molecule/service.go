// Package molecule provides the application-level service for molecule operations.
// This package serves as the interface between HTTP/gRPC handlers and domain logic.
package molecule

import (
	"context"
	"time"

	domainMol "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Service defines the interface for molecule application operations.
type Service interface {
	Create(ctx context.Context, input *CreateInput) (*Molecule, error)
	GetByID(ctx context.Context, id string) (*Molecule, error)
	List(ctx context.Context, input *ListInput) (*ListResult, error)
	Update(ctx context.Context, input *UpdateInput) (*Molecule, error)
	Delete(ctx context.Context, id string, userID string) error
	SearchByStructure(ctx context.Context, input *StructureSearchInput) (*SearchResult, error)
	SearchBySimilarity(ctx context.Context, input *SimilaritySearchInput) (*SearchResult, error)
	CalculateProperties(ctx context.Context, input *CalculatePropertiesInput) (*PropertiesResult, error)
}

// CreateInput contains input for creating a molecule.
type CreateInput struct {
	Name       string
	SMILES     string
	InChI      string
	MolFormula string
	Properties map[string]interface{}
	Tags       []string
	UserID     string
}

// UpdateInput contains input for updating a molecule.
type UpdateInput struct {
	ID         string
	Name       *string
	Properties map[string]interface{}
	Tags       []string
	UserID     string
}

// ListInput contains input for listing molecules.
type ListInput struct {
	Page     int
	PageSize int
	Query    string
	Tag      string
}

// StructureSearchInput contains input for structure search.
type StructureSearchInput struct {
	SMILES     string
	SearchType string
	MaxResults int
}

// SimilaritySearchInput contains input for similarity search.
type SimilaritySearchInput struct {
	SMILES     string
	Threshold  float64
	MaxResults int
}

// CalculatePropertiesInput contains input for property calculation.
type CalculatePropertiesInput struct {
	SMILES     string
	Properties []string
}

// Molecule represents an application-level molecule DTO.
type Molecule struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	SMILES     string                 `json:"smiles"`
	InChI      string                 `json:"inchi,omitempty"`
	MolFormula string                 `json:"mol_formula,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// ListResult represents a paginated list of molecules.
type ListResult struct {
	Molecules  []*Molecule `json:"molecules"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// SearchResult represents molecule search results.
type SearchResult struct {
	Molecules []*MoleculeMatch `json:"molecules"`
	Total     int64            `json:"total"`
}

// MoleculeMatch represents a molecule match with similarity score.
type MoleculeMatch struct {
	Molecule   *Molecule `json:"molecule"`
	Similarity float64   `json:"similarity,omitempty"`
	MatchType  string    `json:"match_type,omitempty"`
}

// PropertiesResult represents calculated properties.
type PropertiesResult struct {
	SMILES     string                 `json:"smiles"`
	Properties map[string]interface{} `json:"properties"`
}

// serviceImpl implements the Service interface.
type serviceImpl struct {
	repo   domainMol.MoleculeRepository
	logger logging.Logger
}

// NewService creates a new molecule application service.
func NewService(repo domainMol.MoleculeRepository, logger logging.Logger) Service {
	return &serviceImpl{
		repo:   repo,
		logger: logger,
	}
}

func (s *serviceImpl) Create(ctx context.Context, input *CreateInput) (*Molecule, error) {
	if input.SMILES == "" {
		return nil, errors.NewValidationError("smiles", "smiles is required")
	}

	mol, err := domainMol.NewMolecule(input.SMILES, domainMol.SourceManual, "")
	if err != nil {
		return nil, errors.NewValidationError("smiles", err.Error())
	}

	mol.Name = input.Name
	mol.InChI = input.InChI
	mol.MolecularFormula = input.MolFormula

	if err := s.repo.Save(ctx, mol); err != nil {
		s.logger.Error("failed to create molecule")
		return nil, err
	}

	return domainToDTO(mol), nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id string) (*Molecule, error) {
	mol, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return domainToDTO(mol), nil
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
	query := &domainMol.MoleculeQuery{
		Offset: offset,
		Limit:  input.PageSize,
	}

	result, err := s.repo.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	dtos := make([]*Molecule, len(result.Molecules))
	for i, mol := range result.Molecules {
		dtos[i] = domainToDTO(mol)
	}

	totalPages := int(result.Total) / input.PageSize
	if int(result.Total)%input.PageSize > 0 {
		totalPages++
	}

	return &ListResult{
		Molecules:  dtos,
		Total:      result.Total,
		Page:       input.Page,
		PageSize:   input.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *serviceImpl) Update(ctx context.Context, input *UpdateInput) (*Molecule, error) {
	mol, err := s.repo.FindByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		mol.Name = *input.Name
	}

	if err := s.repo.Update(ctx, mol); err != nil {
		return nil, err
	}

	return domainToDTO(mol), nil
}

func (s *serviceImpl) Delete(ctx context.Context, id string, userID string) error {
	return s.repo.Delete(ctx, id)
}

func (s *serviceImpl) SearchByStructure(ctx context.Context, input *StructureSearchInput) (*SearchResult, error) {
	// Placeholder implementation - actual implementation would use RDKit/similarity search
	return &SearchResult{
		Molecules: []*MoleculeMatch{},
		Total:     0,
	}, nil
}

func (s *serviceImpl) SearchBySimilarity(ctx context.Context, input *SimilaritySearchInput) (*SearchResult, error) {
	// Placeholder implementation - actual implementation would use vector search
	return &SearchResult{
		Molecules: []*MoleculeMatch{},
		Total:     0,
	}, nil
}

func (s *serviceImpl) CalculateProperties(ctx context.Context, input *CalculatePropertiesInput) (*PropertiesResult, error) {
	// Placeholder implementation - actual implementation would use RDKit
	mol, err := domainMol.NewMolecule(input.SMILES, domainMol.SourceManual, "")
	if err != nil {
		return nil, errors.NewValidationError("smiles", err.Error())
	}

	_ = mol // suppress unused warning
	return &PropertiesResult{
		SMILES:     input.SMILES,
		Properties: map[string]interface{}{},
	}, nil
}

func domainToDTO(mol *domainMol.Molecule) *Molecule {
	if mol == nil {
		return nil
	}
	
	// Convert properties to map
	propsMap := make(map[string]interface{})
	for _, prop := range mol.Properties {
		propsMap[prop.Name] = prop.Value
	}
	
	return &Molecule{
		ID:         mol.ID.String(),
		Name:       mol.Name,
		SMILES:     mol.SMILES,
		InChI:      mol.InChI,
		MolFormula: mol.MolecularFormula,
		Properties: propsMap,
		CreatedAt:  mol.CreatedAt,
		UpdatedAt:  mol.UpdatedAt,
	}
}

//Personal.AI order the ending
