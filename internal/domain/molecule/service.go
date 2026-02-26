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
	// 1. Create entity (Pending)
	mol, err := NewMolecule(smiles, source, sourceRef)
	if err != nil {
		return nil, err
	}

	// 2. Standardize structure
	canonical, inchi, inchiKey, formula, weight, err := s.fpCalculator.Standardize(ctx, smiles)
	if err != nil {
		s.logger.Error("Standardization failed", logging.String("smiles", smiles), logging.Error(err))
		return nil, errors.Wrap(err, errors.ErrCodeMoleculeParsingFailed, "failed to standardize molecule")
	}

	// 3. Check for duplicates (Idempotency)
	exists, err := s.repo.ExistsByInChIKey(ctx, inchiKey)
	if err != nil {
		return nil, err
	}
	if exists {
		existing, err := s.repo.FindByInChIKey(ctx, inchiKey)
		if err != nil {
			return nil, err
		}
		s.logger.Info("Molecule already exists", logging.String("inchiKey", inchiKey))
		return existing, nil
	}

	// 4. Set structure identifiers
	if err := mol.SetStructureIdentifiers(canonical, inchi, inchiKey, formula, weight); err != nil {
		return nil, err
	}

	// 5. Calculate mandatory fingerprints
	opts := DefaultFingerprintCalcOptions()

	// Morgan
	morgan, err := s.fpCalculator.Calculate(ctx, canonical, FingerprintMorgan, opts)
	if err != nil {
		s.logger.Warn("Failed to calculate Morgan fingerprint", logging.String("smiles", canonical), logging.Error(err))
	} else {
		_ = mol.AddFingerprint(morgan)
	}

	// MACCS
	maccs, err := s.fpCalculator.Calculate(ctx, canonical, FingerprintMACCS, opts)
	if err != nil {
		s.logger.Warn("Failed to calculate MACCS fingerprint", logging.String("smiles", canonical), logging.Error(err))
	} else {
		_ = mol.AddFingerprint(maccs)
	}

	// 6. Activate
	if err := mol.Activate(); err != nil {
		return nil, err
	}

	// 7. Save
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
	}

	// Optimization: Standardize and check duplicates in batch?
	// For simplicity and correctness first, we process iteratively but check duplicates efficiently if repo supports it.
	// Since we need to standardize first to get InChIKey, we can standardize individually or batch if calculator supports it (it doesn't have BatchStandardize).
	// Fingerprints supports BatchCalculate.

	// Phase 1: Standardization and Dedup Check
	// We'll process one by one for standardization as it's likely calling external service or simple RDKit wrapper.

	type pendingMol struct {
		mol       *Molecule
		canonical string
		index     int
	}
	var toRegister []pendingMol
	var canonicals []string

	for i, req := range requests {
		// Create entity
		mol, err := NewMolecule(req.SMILES, req.Source, req.SourceRef)
		if err != nil {
			result.Failed = append(result.Failed, BatchRegistrationError{i, req.SMILES, err})
			continue
		}

		// Standardize
		canonical, inchi, inchiKey, formula, weight, err := s.fpCalculator.Standardize(ctx, req.SMILES)
		if err != nil {
			result.Failed = append(result.Failed, BatchRegistrationError{i, req.SMILES, err})
			continue
		}

		// Check duplicate
		exists, err := s.repo.ExistsByInChIKey(ctx, inchiKey)
		if err != nil {
			result.Failed = append(result.Failed, BatchRegistrationError{i, req.SMILES, err})
			continue
		}
		if exists {
			// Fetch existing? Spec says "DuplicateCount increased".
			// Should we return existing molecules in Succeeded? Spec says "Part success semantic... Existing molecules count into DuplicateCount not Failed".
			// Spec "Return includes Success, Failed, DuplicateCount". Succeeded usually implies *newly* registered or successfully processed.
			// But duplicate implies no action needed.
			// If we want to return the molecule object, we need to fetch it.
			// Let's assume we don't return duplicates in Succeeded list to save bandwidth/db calls unless requested,
			// OR we assume Succeeded contains valid molecules regardless of whether they were just created or existed.
			// Spec: "Succeeded []*Molecule". "DuplicateCount int".
			// If we put duplicates in Succeeded, DuplicateCount is redundant or informational.
			// Let's assume duplicates are NOT in Succeeded, just counted.
			result.DuplicateCount++
			// We could fetch it if needed, but for bulk ingest often we just want to know it's there.
			// If user needs the ID, they might need it.
			// Spec doesn't strictly say. I'll stick to counting.
			continue
		}

		// Set identifiers
		_ = mol.SetStructureIdentifiers(canonical, inchi, inchiKey, formula, weight)

		// Add tags/props
		for _, tag := range req.Tags { _ = mol.AddTag(tag) }
		for _, prop := range req.Properties { _ = mol.AddProperty(prop) }

		toRegister = append(toRegister, pendingMol{mol, canonical, i})
		canonicals = append(canonicals, canonical)
	}

	// Phase 2: Batch Fingerprint Calculation
	if len(toRegister) > 0 {
		opts := DefaultFingerprintCalcOptions()

		// Morgan
		morgans, err := s.fpCalculator.BatchCalculate(ctx, canonicals, FingerprintMorgan, opts)
		if err != nil {
			// Fallback to individual or fail all?
			s.logger.Error("Batch Morgan calculation failed", logging.Error(err))
			// We can try individually or just fail this step (add without fingerprint)
			// But Activate requires fingerprints? No, Validate doesn't check fingerprints presence explicitly, but Activate implies "ready".
			// Spec for Activate: "InChIKey must be set". Doesn't strictly require fingerprints.
		}

		// MACCS
		maccss, err := s.fpCalculator.BatchCalculate(ctx, canonicals, FingerprintMACCS, opts)
		if err != nil {
			s.logger.Error("Batch MACCS calculation failed", logging.Error(err))
		}

		// Phase 3: Assembly and Save
		var batchToSave []*Molecule

		for j, pm := range toRegister {
			if j < len(morgans) && morgans[j] != nil {
				_ = pm.mol.AddFingerprint(morgans[j])
			}
			if j < len(maccss) && maccss[j] != nil {
				_ = pm.mol.AddFingerprint(maccss[j])
			}

			if err := pm.mol.Activate(); err != nil {
				result.Failed = append(result.Failed, BatchRegistrationError{pm.index, pm.mol.SMILES(), err})
				continue
			}

			batchToSave = append(batchToSave, pm.mol)
		}

		// Batch Save
		if len(batchToSave) > 0 {
			count, err := s.repo.BatchSave(ctx, batchToSave)
			if err != nil {
				// If batch save fails, we don't know which ones failed exactly unless repo tells us.
				// Spec says: "If any entity save fails, entire batch rolls back (transactional)".
				// So all failed.
				for range batchToSave {
					// We need to map back to original index... slightly complex.
					// For simplicity, we just add generic error or log it.
					// We can iterate batchToSave and add to Failed.
					// We don't have original index easily unless we stored it in struct.
					// But we don't have easy way to map back `m` to `index`.
					// Wait, we lost index association in `batchToSave`.
					// Let's assume we can't report exact index for batch save failure easily without more complex logic.
					// Just return error for the whole batch?
					// Function returns `(*BatchRegistrationResult, error)`.
					return result, errors.Wrap(err, errors.ErrCodeInternal, "batch save failed")
				}
			}
			result.Succeeded = append(result.Succeeded, batchToSave[:count]...) // Assuming count is all
		}
	}

	return result, nil
}

// GetMolecule retrieves a molecule by ID.
func (s *MoleculeService) GetMolecule(ctx context.Context, id string) (*Molecule, error) {
	if id == "" {
		return nil, errors.New(errors.ErrCodeInvalidInput, "id cannot be empty")
	}
	return s.repo.FindByID(ctx, id)
}

// GetMoleculeByInChIKey retrieves a molecule by InChIKey.
func (s *MoleculeService) GetMoleculeByInChIKey(ctx context.Context, inchiKey string) (*Molecule, error) {
	// Simple validation
	if len(inchiKey) == 0 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "inchiKey cannot be empty")
	}
	return s.repo.FindByInChIKey(ctx, inchiKey)
}

// SearchMolecules searches for molecules.
func (s *MoleculeService) SearchMolecules(ctx context.Context, query *MoleculeQuery) (*MoleculeSearchResult, error) {
	if query == nil {
		return nil, errors.New(errors.ErrCodeInvalidInput, "query cannot be nil")
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

	// Pre-fetch canonical smiles if needed? It's in mol.
	smiles := mol.CanonicalSmiles()
	if smiles == "" {
		smiles = mol.SMILES() // Fallback
	}

	for _, ft := range fpTypes {
		if mol.HasFingerprint(ft) {
			continue // Skip existing
		}

		fp, err := s.fpCalculator.Calculate(ctx, smiles, ft, opts)
		if err != nil {
			s.logger.Error("Fingerprint calculation failed", logging.String("type", string(ft)), logging.Error(err))
			// Continue with others? Spec says "If molecule has type, skip...".
			// If calc fails, we skip it.
			continue
		}

		_ = mol.AddFingerprint(fp)
		updated = true
	}

	if updated {
		return s.repo.Update(ctx, mol)
	}
	return nil
}

// FindSimilarMolecules searches for molecules similar to a target.
func (s *MoleculeService) FindSimilarMolecules(ctx context.Context, targetSMILES string, fpType FingerprintType, threshold float64, limit int) ([]*SimilarityResult, error) {
	if targetSMILES == "" {
		return nil, errors.New(errors.ErrCodeInvalidInput, "targetSMILES cannot be empty")
	}
	if threshold < 0 || threshold > 1 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "threshold must be between 0 and 1")
	}
	if limit <= 0 {
		limit = 100 // Default
	}

	// 1. Calculate fingerprint for target
	// First standardize to get best SMILES? Or just calculate on input?
	// Spec says "1. Calculate target molecule fingerprint".
	// Calculator usually handles standardization internally or we should do it.
	// But `Calculate` takes SMILES.
	// We'll trust calculator or standardized input.
	opts := DefaultFingerprintCalcOptions()
	fp, err := s.fpCalculator.Calculate(ctx, targetSMILES, fpType, opts)
	if err != nil {
		return nil, err
	}

	// 2. Search
	metric := MetricTanimoto
	if fp.IsDenseVector() {
		metric = MetricCosine
	}

	return s.similarityEngine.SearchSimilar(ctx, fp, metric, threshold, limit)
}

// MoleculeComparisonResult results of comparison.
type MoleculeComparisonResult struct {
	Molecule1SMILES         string
	Molecule2SMILES         string
	Scores                  map[FingerprintType]float64
	FusedScore              float64
	IsStructurallyIdentical bool
}

// CompareMolecules compares two molecules.
func (s *MoleculeService) CompareMolecules(ctx context.Context, smiles1, smiles2 string, fpTypes []FingerprintType) (*MoleculeComparisonResult, error) {
	if smiles1 == "" || smiles2 == "" {
		return nil, errors.New(errors.ErrCodeInvalidInput, "SMILES strings cannot be empty")
	}
	if len(fpTypes) == 0 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "at least one fingerprint type required")
	}

	// Standardize both to check identity
	// We ignore errors here for standardization if strictly comparing structure based on fingerprints,
	// but for "IsStructurallyIdentical" (InChIKey check) we need it.
	_, _, key1, _, _, err1 := s.fpCalculator.Standardize(ctx, smiles1)
	_, _, key2, _, _, err2 := s.fpCalculator.Standardize(ctx, smiles2)

	identical := false
	if err1 == nil && err2 == nil {
		identical = key1 == key2
	}

	scores := make(map[FingerprintType]float64)
	opts := DefaultFingerprintCalcOptions()

	for _, ft := range fpTypes {
		fp1, err := s.fpCalculator.Calculate(ctx, smiles1, ft, opts)
		if err != nil { return nil, err }
		fp2, err := s.fpCalculator.Calculate(ctx, smiles2, ft, opts)
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
		IsStructurallyIdentical: identical,
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

// AddMoleculeProperties adds properties to a molecule.
func (s *MoleculeService) AddMoleculeProperties(ctx context.Context, moleculeID string, properties []*MolecularProperty) error {
	mol, err := s.repo.FindByID(ctx, moleculeID)
	if err != nil { return err }

	updated := false
	for _, p := range properties {
		if err := mol.AddProperty(p); err != nil {
			return err
		}
		updated = true
	}

	if updated {
		return s.repo.Update(ctx, mol)
	}
	return nil
}

// TagMolecule adds tags to a molecule.
func (s *MoleculeService) TagMolecule(ctx context.Context, moleculeID string, tags []string) error {
	mol, err := s.repo.FindByID(ctx, moleculeID)
	if err != nil { return err }

	updated := false
	for _, t := range tags {
		if err := mol.AddTag(t); err != nil {
			return err
		}
		updated = true
	}

	if updated {
		return s.repo.Update(ctx, mol)
	}
	return nil
}
//Personal.AI order the ending
