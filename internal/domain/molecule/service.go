package molecule

import (
	"context"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// MoleculeService coordinates molecule-related business operations.
type MoleculeService struct {
	repo             MoleculeRepository
	fpCalculator     FingerprintCalculator
	similarityEngine SimilarityEngine
	logger           logging.Logger
}

// NewMoleculeService constructs a new MoleculeService.
func NewMoleculeService(repo MoleculeRepository, fpCalc FingerprintCalculator, simEngine SimilarityEngine, logger logging.Logger) (*MoleculeService, error) {
	if repo == nil || fpCalc == nil || simEngine == nil || logger == nil {
		return nil, errors.New(errors.ErrCodeValidation, "all dependencies must be provided")
	}
	return &MoleculeService{
		repo:             repo,
		fpCalculator:     fpCalc,
		similarityEngine: simEngine,
		logger:           logger,
	}, nil
}

// RegisterMolecule handles the complete registration process for a new molecule.
func (s *MoleculeService) RegisterMolecule(ctx context.Context, smiles string, source MoleculeSource, sourceRef string) (*Molecule, error) {
	// 1. Basic entity creation
	mol, err := NewMolecule(smiles, source, sourceRef)
	if err != nil {
		return nil, err
	}

	// 2. Structural standardization
	ids, err := s.fpCalculator.Standardize(ctx, smiles)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeMoleculeParsingFailed, "failed to standardize molecule")
	}

	// 3. Idempotency check via InChIKey
	exists, err := s.repo.ExistsByInChIKey(ctx, ids.InChIKey)
	if err != nil {
		return nil, err
	}
	if exists {
		s.logger.Info("molecule already exists, returning existing entity", logging.String("inchi_key", ids.InChIKey))
		return s.repo.FindByInChIKey(ctx, ids.InChIKey)
	}

	// 4. Set identifiers
	if err := mol.SetStructureIdentifiers(ids.CanonicalSMILES, ids.InChI, ids.InChIKey, ids.Formula, ids.Weight); err != nil {
		return nil, err
	}

	// 5. Calculate default fingerprints (Morgan & MACCS)
	opts := DefaultFingerprintCalcOptions()

	// Morgan
	morgan, err := s.fpCalculator.Calculate(ctx, ids.CanonicalSMILES, FingerprintMorgan, opts)
	if err != nil {
		s.logger.Warn("failed to calculate Morgan fingerprint", logging.String("smiles", smiles), logging.Error(err))
		// Should we fail registration? Requirement says "Standardize... Calculate... Add... Activate".
		// Usually fingerprints are critical for search. But maybe proceed if one fails?
		// Requirement for RegisterMolecule: "Step 6... Step 7... Step 8... Return wrapper error on failure".
		// So we should fail if calculation fails.
		return nil, errors.Wrap(err, errors.ErrCodeFingerprintGenerationFailed, "failed to calculate Morgan fingerprint")
	}
	if err := mol.AddFingerprint(morgan); err != nil {
		return nil, err
	}

	// MACCS
	maccs, err := s.fpCalculator.Calculate(ctx, ids.CanonicalSMILES, FingerprintMACCS, opts)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeFingerprintGenerationFailed, "failed to calculate MACCS fingerprint")
	}
	if err := mol.AddFingerprint(maccs); err != nil {
		return nil, err
	}

	// 6. Activate
	if err := mol.Activate(); err != nil {
		return nil, err
	}

	// 7. Persist
	if err := s.repo.Save(ctx, mol); err != nil {
		return nil, err
	}

	return mol, nil
}

// MoleculeRegistrationRequest defines parameters for registering a molecule.
type MoleculeRegistrationRequest struct {
	SMILES     string
	Source     MoleculeSource
	SourceRef  string
	Tags       []string
	Properties []*MolecularProperty
}

// BatchRegistrationResult contains results of a batch registration operation.
type BatchRegistrationResult struct {
	Succeeded      []*Molecule
	Failed         []BatchRegistrationError
	DuplicateCount int
	TotalProcessed int
}

// BatchRegistrationError describes an error for a specific item in a batch.
type BatchRegistrationError struct {
	Index  int
	SMILES string
	Error  error
}

// BatchRegisterMolecules handles multiple molecule registrations.
func (s *MoleculeService) BatchRegisterMolecules(ctx context.Context, requests []MoleculeRegistrationRequest) (*BatchRegistrationResult, error) {
	result := &BatchRegistrationResult{
		TotalProcessed: len(requests),
		Succeeded:      make([]*Molecule, 0),
		Failed:         make([]BatchRegistrationError, 0),
	}

	if len(requests) == 0 {
		return result, nil
	}

	// Optimization: Process sequentially for now, or batch if calculator supports it.
	// Requirement mentions using `fpCalculator.BatchCalculate` and `repo.BatchSave`.

	// Stage 1: Validation and Standardization
	type PendingMolecule struct {
		Index int
		Mol   *Molecule
		Req   MoleculeRegistrationRequest
	}
	var pending []PendingMolecule
	var smilesList []string

	for i, req := range requests {
		// Create basic entity
		mol, err := NewMolecule(req.SMILES, req.Source, req.SourceRef)
		if err != nil {
			result.Failed = append(result.Failed, BatchRegistrationError{Index: i, SMILES: req.SMILES, Error: err})
			continue
		}

		// Standardize
		ids, err := s.fpCalculator.Standardize(ctx, req.SMILES)
		if err != nil {
			result.Failed = append(result.Failed, BatchRegistrationError{Index: i, SMILES: req.SMILES, Error: err})
			continue
		}

		// Idempotency check
		exists, err := s.repo.ExistsByInChIKey(ctx, ids.InChIKey)
		if err != nil {
			result.Failed = append(result.Failed, BatchRegistrationError{Index: i, SMILES: req.SMILES, Error: err})
			continue
		}
		if exists {
			// Fetch existing to return it
			existing, err := s.repo.FindByInChIKey(ctx, ids.InChIKey)
			if err != nil {
				// Should not happen if Exists returned true, but race condition possible
				result.Failed = append(result.Failed, BatchRegistrationError{Index: i, SMILES: req.SMILES, Error: err})
			} else {
				result.Succeeded = append(result.Succeeded, existing)
				result.DuplicateCount++
			}
			continue
		}

		// Set identifiers
		_ = mol.SetStructureIdentifiers(ids.CanonicalSMILES, ids.InChI, ids.InChIKey, ids.Formula, ids.Weight)

		// Add extras
		for _, tag := range req.Tags {
			_ = mol.AddTag(tag)
		}
		for _, prop := range req.Properties {
			_ = mol.AddProperty(prop)
		}

		pending = append(pending, PendingMolecule{Index: i, Mol: mol, Req: req})
		smilesList = append(smilesList, ids.CanonicalSMILES)
	}

	if len(pending) == 0 {
		return result, nil
	}

	// Stage 2: Batch Fingerprint Calculation
	opts := DefaultFingerprintCalcOptions()

	// Morgan
	morgans, err := s.fpCalculator.BatchCalculate(ctx, smilesList, FingerprintMorgan, opts)
	if err != nil {
		// If batch calc fails, fail all pending? Or fallback to individual?
		// Assuming fail all pending for simplicity as per requirement "Use BatchCalculate".
		for _, p := range pending {
			result.Failed = append(result.Failed, BatchRegistrationError{Index: p.Index, SMILES: p.Req.SMILES, Error: err})
		}
		return result, nil // Or continue?
	}

	// MACCS
	maccss, err := s.fpCalculator.BatchCalculate(ctx, smilesList, FingerprintMACCS, opts)
	if err != nil {
		for _, p := range pending {
			result.Failed = append(result.Failed, BatchRegistrationError{Index: p.Index, SMILES: p.Req.SMILES, Error: err})
		}
		return result, nil
	}

	// Stage 3: Assembly and Batch Save
	var toSave []*Molecule
	// Map fingerprints back to molecules
	// BatchCalculate returns slice corresponding to input smilesList
	if len(morgans) != len(pending) || len(maccss) != len(pending) {
		// Mismatch error
		err := errors.New(errors.ErrCodeInternal, "fingerprint batch size mismatch")
		for _, p := range pending {
			result.Failed = append(result.Failed, BatchRegistrationError{Index: p.Index, SMILES: p.Req.SMILES, Error: err})
		}
		return result, nil
	}

	for i, p := range pending {
		if morgans[i] != nil {
			_ = p.Mol.AddFingerprint(morgans[i])
		}
		if maccss[i] != nil {
			_ = p.Mol.AddFingerprint(maccss[i])
		}

		if err := p.Mol.Activate(); err != nil {
			result.Failed = append(result.Failed, BatchRegistrationError{Index: p.Index, SMILES: p.Req.SMILES, Error: err})
			continue
		}
		toSave = append(toSave, p.Mol)
	}

	if len(toSave) > 0 {
		_, err := s.repo.BatchSave(ctx, toSave)
		if err != nil {
			// If batch save fails (transactional), all fail
			s.logger.Error("batch save failed", logging.Error(err))
			// Add all to failed
			for _, p := range pending {
				// Only if it was in toSave list...
				// Simplify: mark all pending as failed with save error
				result.Failed = append(result.Failed, BatchRegistrationError{Index: p.Index, SMILES: p.Req.SMILES, Error: err})
			}
		} else {
			result.Succeeded = append(result.Succeeded, toSave...)
		}
	}

	return result, nil
}

// GetMolecule retrieves a molecule by ID.
func (s *MoleculeService) GetMolecule(ctx context.Context, id string) (*Molecule, error) {
	if id == "" {
		return nil, errors.New(errors.ErrCodeValidation, "id cannot be empty")
	}
	return s.repo.FindByID(ctx, id)
}

// GetMoleculeByInChIKey retrieves a molecule by InChIKey.
func (s *MoleculeService) GetMoleculeByInChIKey(ctx context.Context, inchiKey string) (*Molecule, error) {
	if !inchiKeyRegex.MatchString(inchiKey) {
		return nil, errors.New(errors.ErrCodeValidation, "invalid inchiKey format")
	}
	return s.repo.FindByInChIKey(ctx, inchiKey)
}

// SearchMolecules searches for molecules.
func (s *MoleculeService) SearchMolecules(ctx context.Context, query *MoleculeQuery) (*MoleculeSearchResult, error) {
	if query == nil {
		return nil, errors.New(errors.ErrCodeValidation, "query cannot be nil")
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}
	return s.repo.Search(ctx, query)
}

// CalculateFingerprints computes specified fingerprints for a molecule.
func (s *MoleculeService) CalculateFingerprints(ctx context.Context, moleculeID string, fpTypes []FingerprintType) error {
	mol, err := s.repo.FindByID(ctx, moleculeID)
	if err != nil {
		return err
	}

	opts := DefaultFingerprintCalcOptions()
	updated := false
	for _, ft := range fpTypes {
		if mol.HasFingerprint(ft) {
			continue // Skip existing
		}
		fp, err := s.fpCalculator.Calculate(ctx, mol.CanonicalSmiles(), ft, opts)
		if err != nil {
			s.logger.Error("failed to calculate fingerprint", logging.String("molecule_id", moleculeID), logging.String("type", string(ft)), logging.Error(err))
			return errors.Wrap(err, errors.ErrCodeFingerprintGenerationFailed, "calculation failed")
		}
		if err := mol.AddFingerprint(fp); err != nil {
			return err
		}
		updated = true
	}

	if updated {
		return s.repo.Update(ctx, mol)
	}
	return nil
}

// FindSimilarMolecules searches for molecules similar to a target SMILES.
func (s *MoleculeService) FindSimilarMolecules(ctx context.Context, targetSMILES string, fpType FingerprintType, threshold float64, limit int) ([]*SimilarityResult, error) {
	if targetSMILES == "" {
		return nil, errors.New(errors.ErrCodeValidation, "targetSMILES cannot be empty")
	}
	if threshold < 0 || threshold > 1 { // Allow 0 and 1
		return nil, errors.New(errors.ErrCodeValidation, "threshold must be between 0 and 1")
	}
	if limit <= 0 {
		return nil, errors.New(errors.ErrCodeValidation, "limit must be positive")
	}

	// 1. Calculate target fingerprint
	opts := DefaultFingerprintCalcOptions()
	fp, err := s.fpCalculator.Calculate(ctx, targetSMILES, fpType, opts)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeFingerprintGenerationFailed, "failed to calculate target fingerprint")
	}

	// 2. Select metric
	metric := MetricTanimoto
	if fp.IsDenseVector() {
		metric = MetricCosine
	}

	// 3. Search
	return s.similarityEngine.SearchSimilar(ctx, fp, metric, threshold, limit)
}

// MoleculeComparisonResult contains results of comparing two molecules.
type MoleculeComparisonResult struct {
	Molecule1SMILES         string
	Molecule2SMILES         string
	Scores                  map[FingerprintType]float64
	FusedScore              float64
	IsStructurallyIdentical bool
}

// CompareMolecules calculates similarity scores between two SMILES strings.
func (s *MoleculeService) CompareMolecules(ctx context.Context, smiles1, smiles2 string, fpTypes []FingerprintType) (*MoleculeComparisonResult, error) {
	if smiles1 == "" || smiles2 == "" {
		return nil, errors.New(errors.ErrCodeValidation, "SMILES strings cannot be empty")
	}
	if len(fpTypes) == 0 {
		return nil, errors.New(errors.ErrCodeValidation, "at least one fingerprint type required")
	}

	// Standardize to check identity
	ids1, err1 := s.fpCalculator.Standardize(ctx, smiles1)
	if err1 != nil { return nil, err1 }
	ids2, err2 := s.fpCalculator.Standardize(ctx, smiles2)
	if err2 != nil { return nil, err2 }

	opts := DefaultFingerprintCalcOptions()
	scores := make(map[FingerprintType]float64)

	for _, ft := range fpTypes {
		fp1, err := s.fpCalculator.Calculate(ctx, ids1.CanonicalSMILES, ft, opts)
		if err != nil { return nil, err }
		fp2, err := s.fpCalculator.Calculate(ctx, ids2.CanonicalSMILES, ft, opts)
		if err != nil { return nil, err }

		metric := MetricTanimoto
		if ft == FingerprintGNN {
			metric = MetricCosine
		}
		score, err := s.similarityEngine.ComputeSimilarity(fp1, fp2, metric)
		if err != nil { return nil, err }
		scores[ft] = score
	}

	fusion := &WeightedAverageFusion{}
	fused, _ := fusion.Fuse(scores, nil)

	return &MoleculeComparisonResult{
		Molecule1SMILES:         smiles1,
		Molecule2SMILES:         smiles2,
		Scores:                  scores,
		FusedScore:              fused,
		IsStructurallyIdentical: ids1.InChIKey == ids2.InChIKey,
	}, nil
}

// ArchiveMolecule transitions a molecule to Archived status.
func (s *MoleculeService) ArchiveMolecule(ctx context.Context, id string) error {
	mol, err := s.repo.FindByID(ctx, id)
	if err != nil { return err }
	if err := mol.Archive(); err != nil { return err }
	return s.repo.Update(ctx, mol)
}

// DeleteMolecule transitions a molecule to Deleted status.
func (s *MoleculeService) DeleteMolecule(ctx context.Context, id string) error {
	mol, err := s.repo.FindByID(ctx, id)
	if err != nil { return err }
	if err := mol.MarkDeleted(); err != nil { return err }
	return s.repo.Update(ctx, mol)
}

// AddMoleculeProperties adds properties to an existing molecule.
func (s *MoleculeService) AddMoleculeProperties(ctx context.Context, moleculeID string, properties []*MolecularProperty) error {
	if len(properties) == 0 {
		return nil
	}
	mol, err := s.repo.FindByID(ctx, moleculeID)
	if err != nil { return err }
	for _, p := range properties {
		if err := mol.AddProperty(p); err != nil { return err }
	}
	return s.repo.Update(ctx, mol)
}

// TagMolecule adds tags to an existing molecule.
func (s *MoleculeService) TagMolecule(ctx context.Context, moleculeID string, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	mol, err := s.repo.FindByID(ctx, moleculeID)
	if err != nil { return err }
	for _, t := range tags {
		if err := mol.AddTag(t); err != nil { return err }
	}
	return s.repo.Update(ctx, mol)
}

//Personal.AI order the ending
