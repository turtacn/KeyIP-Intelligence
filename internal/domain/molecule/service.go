// Package molecule provides the domain service layer for molecular operations.
package molecule

import (
	"context"
	"fmt"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	mtypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// Service coordinates molecule-related business logic and repository operations.
// It enforces domain rules, orchestrates complex workflows, and provides a
// high-level API for application services.
type Service struct {
	repo   Repository
	logger logging.Logger
}

// NewService constructs a new molecule domain service.
func NewService(repo Repository, logger logging.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Core CRUD Operations
// ─────────────────────────────────────────────────────────────────────────────

// CreateMolecule creates a new molecule from a SMILES string, performing
// deduplication via SMILES lookup.  If the molecule already exists, returns
// the existing entity rather than creating a duplicate.
func (s *Service) CreateMolecule(ctx context.Context, smiles string, molType mtypes.MoleculeType) (*Molecule, error) {
	// Check if molecule already exists (deduplication)
	existing, err := s.repo.FindBySMILES(ctx, smiles)
	if err == nil {
		s.logger.Info("molecule already exists", logging.String("smiles", smiles), logging.String("id", string(existing.ID)))
		return existing, nil
	}
	if !errors.IsNotFound(err) {
		return nil, errors.Wrap(err, errors.CodeDatabaseError, "failed to check for existing molecule")
	}

	// Create new molecule
	mol, err := NewMolecule(smiles, molType)
	if err != nil {
		return nil, err
	}

	// Calculate default fingerprint (Morgan)
	if err := mol.CalculateFingerprint(mtypes.FPMorgan); err != nil {
		s.logger.Warn("failed to calculate Morgan fingerprint", logging.Error(err))
		// Non-fatal: continue without fingerprint
	}

	// Calculate basic properties
	if err := mol.CalculateProperties(); err != nil {
		s.logger.Warn("failed to calculate molecular properties", logging.Error(err))
		// Non-fatal: continue without properties
	}

	// Persist
	if err := s.repo.Save(ctx, mol); err != nil {
		return nil, errors.Wrap(err, errors.CodeDatabaseError, "failed to save molecule")
	}

	s.logger.Info("created new molecule",
		logging.String("id", string(mol.ID)),
		logging.String("smiles", smiles),
		logging.String("type", string(molType)))

	return mol, nil
}

// GetMolecule retrieves a molecule by its ID.
func (s *Service) GetMolecule(ctx context.Context, id common.ID) (*Molecule, error) {
	mol, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeNotFound, "molecule not found")
	}
	return mol, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Search Operations
// ─────────────────────────────────────────────────────────────────────────────

// SearchMolecules performs a paginated search with filtering.
func (s *Service) SearchMolecules(ctx context.Context, req mtypes.MoleculeSearchRequest) (*mtypes.MoleculeSearchResponse, error) {
	// Validate pagination parameters
	if err := req.Page.Validate(); err != nil {
		return nil, err
	}

	resp, err := s.repo.Search(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeDatabaseError, "search failed")
	}

	s.logger.Debug("molecule search executed",
		logging.Int("results", len(resp.Items)),
		logging.Int64("total", resp.Total))

	return resp, nil
}

// FindSimilarMolecules finds molecules similar to the given SMILES string
// using fingerprint-based similarity search.
func (s *Service) FindSimilarMolecules(ctx context.Context, smiles string, threshold float64, fpType mtypes.FingerprintType, maxResults int) ([]*Molecule, error) {
	// Create query molecule
	queryMol, err := NewMolecule(smiles, mtypes.TypeSmallMolecule)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeMoleculeInvalidSMILES, "invalid query SMILES")
	}

	// Calculate fingerprint
	if err := queryMol.CalculateFingerprint(fpType); err != nil {
		return nil, errors.Wrap(err, errors.CodeMoleculeInvalidSMILES, "failed to calculate query fingerprint")
	}

	fp := queryMol.Fingerprints[fpType]
	if fp == nil {
		return nil, errors.New(errors.CodeMoleculeInvalidSMILES, "fingerprint not available")
	}

	// Perform similarity search
	results, err := s.repo.FindSimilar(ctx, fp, fpType, threshold, maxResults)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeDatabaseError, "similarity search failed")
	}

	s.logger.Info("similarity search completed",
		logging.String("query_smiles", smiles),
		logging.Float64("threshold", threshold),
		logging.Int("results", len(results)))

	return results, nil
}

// SubstructureSearch finds molecules containing the specified substructure.
func (s *Service) SubstructureSearch(ctx context.Context, smarts string, maxResults int) ([]*Molecule, error) {
	if smarts == "" {
		return nil, errors.InvalidParam("SMARTS pattern cannot be empty")
	}

	results, err := s.repo.SubstructureSearch(ctx, smarts, maxResults)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeDatabaseError, "substructure search failed")
	}

	s.logger.Info("substructure search completed",
		logging.String("smarts", smarts),
		logging.Int("results", len(results)))

	return results, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Batch Operations
// ─────────────────────────────────────────────────────────────────────────────

// BatchImportMolecules imports multiple molecules from SMILES strings, skipping
// invalid entries and duplicates.  Returns the count of successfully imported
// molecules.
func (s *Service) BatchImportMolecules(ctx context.Context, smilesLines []string, molType mtypes.MoleculeType) (int, error) {
	if len(smilesLines) == 0 {
		return 0, errors.InvalidParam("no SMILES strings provided")
	}

	validMols := make([]*Molecule, 0, len(smilesLines))
	skipped := 0

	for i, smiles := range smilesLines {
		// Check for duplicates via SMILES lookup
		_, err := s.repo.FindBySMILES(ctx, smiles)
		if err == nil {
			s.logger.Debug("skipping duplicate molecule",
				logging.Int("line", i),
				logging.String("smiles", smiles))
			skipped++
			continue
		}

		// Create molecule
		mol, err := NewMolecule(smiles, molType)
		if err != nil {
			s.logger.Warn("skipping invalid SMILES",
				logging.Int("line", i),
				logging.String("smiles", smiles),
				logging.Error(err))
			skipped++
			continue
		}

		// Calculate fingerprint and properties (non-blocking errors)
		_ = mol.CalculateFingerprint(mtypes.FPMorgan)
		_ = mol.CalculateProperties()

		validMols = append(validMols, mol)
	}

	if len(validMols) == 0 {
		return 0, fmt.Errorf("no valid molecules to import")
	}

	// Batch save
	if err := s.repo.BatchSave(ctx, validMols); err != nil {
		return 0, errors.Wrap(err, errors.CodeDatabaseError, "batch save failed")
	}

	s.logger.Info("batch import completed",
		logging.Int("total", len(smilesLines)),
		logging.Int("imported", len(validMols)),
		logging.Int("skipped", skipped))

	return len(validMols), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Patent Association
// ─────────────────────────────────────────────────────────────────────────────

// GetMoleculesByPatent retrieves all molecules extracted from a specific patent.
func (s *Service) GetMoleculesByPatent(ctx context.Context, patentID common.ID) ([]*Molecule, error) {
	if patentID == "" {
		return nil, errors.InvalidParam("patent ID cannot be empty")
	}

	mols, err := s.repo.FindByPatentID(ctx, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeDatabaseError, "failed to retrieve molecules by patent")
	}

	s.logger.Debug("retrieved molecules by patent",
		logging.String("patent_id", string(patentID)),
		logging.Int("count", len(mols)))

	return mols, nil
}

//Personal.AI order the ending
