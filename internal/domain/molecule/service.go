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
	// 1. Basic validation and entity creation
	mol, err := NewMolecule(smiles, source, sourceRef)
	if err != nil {
		return nil, err
	}

	// 2. Structural identifiers using RDKit standardization
	ids, err := s.fpCalculator.Standardize(ctx, smiles)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeMoleculeParsingFailed, "failed to standardize molecule")
	}

	// 3. Duplicate check via InChIKey
	exists, err := s.repo.ExistsByInChIKey(ctx, ids.InChIKey)
	if err != nil {
		return nil, err
	}
	if exists {
		return s.repo.FindByInChIKey(ctx, ids.InChIKey)
	}

	if err := mol.SetStructureIdentifiers(ids.CanonicalSMILES, ids.InChI, ids.InChIKey, ids.Formula, ids.Weight); err != nil {
		return nil, err
	}

	// 4. Calculate mandatory fingerprints (Morgan and MACCS)
	opts := DefaultFingerprintCalcOptions()

	morgan, err := s.fpCalculator.Calculate(ctx, ids.CanonicalSMILES, FingerprintMorgan, opts)
	if err != nil {
		s.logger.Warn("failed to calculate Morgan fingerprint", logging.String("smiles", smiles), logging.Error(err))
	} else {
		_ = mol.AddFingerprint(morgan)
	}

	maccs, err := s.fpCalculator.Calculate(ctx, ids.CanonicalSMILES, FingerprintMACCS, opts)
	if err != nil {
		s.logger.Warn("failed to calculate MACCS fingerprint", logging.String("smiles", smiles), logging.Error(err))
	} else {
		_ = mol.AddFingerprint(maccs)
	}

	// 5. Activate and save
	if err := mol.Activate(); err != nil {
		return nil, err
	}

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

// BatchRegisterMolecules handles multiple molecule registrations with optimization.
func (s *MoleculeService) BatchRegisterMolecules(ctx context.Context, requests []MoleculeRegistrationRequest) (*BatchRegistrationResult, error) {
	result := &BatchRegistrationResult{
		TotalProcessed: len(requests),
	}

	var candidates []*Molecule
	var smilesToProcess []string

	for i, req := range requests {
		mol, err := NewMolecule(req.SMILES, req.Source, req.SourceRef)
		if err != nil {
			result.Failed = append(result.Failed, BatchRegistrationError{Index: i, SMILES: req.SMILES, Error: err})
			continue
		}

		ids, err := s.fpCalculator.Standardize(ctx, req.SMILES)
		if err != nil {
			result.Failed = append(result.Failed, BatchRegistrationError{Index: i, SMILES: req.SMILES, Error: err})
			continue
		}

		exists, err := s.repo.ExistsByInChIKey(ctx, ids.InChIKey)
		if err == nil && exists {
			existing, _ := s.repo.FindByInChIKey(ctx, ids.InChIKey)
			if existing != nil {
				result.Succeeded = append(result.Succeeded, existing)
				result.DuplicateCount++
				continue
			}
		}

		_ = mol.SetStructureIdentifiers(ids.CanonicalSMILES, ids.InChI, ids.InChIKey, ids.Formula, ids.Weight)
		// Add properties and tags from request
		for _, tag := range req.Tags { _ = mol.AddTag(tag) }
		for _, prop := range req.Properties { _ = mol.AddProperty(prop) }

		candidates = append(candidates, mol)
		smilesToProcess = append(smilesToProcess, ids.CanonicalSMILES)
	}

	if len(candidates) > 0 {
		opts := DefaultFingerprintCalcOptions()
		// Batch calculate Morgan
		morgans, _ := s.fpCalculator.BatchCalculate(ctx, smilesToProcess, FingerprintMorgan, opts)
		// Batch calculate MACCS
		maccss, _ := s.fpCalculator.BatchCalculate(ctx, smilesToProcess, FingerprintMACCS, opts)

		toSave := make([]*Molecule, 0, len(candidates))
		for j, mol := range candidates {
			if j < len(morgans) && morgans[j] != nil { _ = mol.AddFingerprint(morgans[j]) }
			if j < len(maccss) && maccss[j] != nil { _ = mol.AddFingerprint(maccss[j]) }

			if err := mol.Activate(); err == nil {
				toSave = append(toSave, mol)
			}
		}

		if len(toSave) > 0 {
			_, err := s.repo.BatchSave(ctx, toSave)
			if err != nil {
				s.logger.Error("batch save failed", logging.Error(err))
			}
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
			continue
		}
		fp, err := s.fpCalculator.Calculate(ctx, mol.SMILES, ft, opts)
		if err != nil {
			s.logger.Error("failed to calculate fingerprint", logging.String("molecule_id", moleculeID), logging.String("type", string(ft)), logging.Error(err))
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

// FindSimilarMolecules searches for molecules similar to a target SMILES.
func (s *MoleculeService) FindSimilarMolecules(ctx context.Context, targetSMILES string, fpType FingerprintType, threshold float64, limit int) ([]*SimilarityResult, error) {
	if targetSMILES == "" {
		return nil, errors.New(errors.ErrCodeValidation, "targetSMILES cannot be empty")
	}
	if threshold < 0 || threshold > 1 {
		return nil, errors.New(errors.ErrCodeValidation, "threshold must be between 0 and 1")
	}

	opts := DefaultFingerprintCalcOptions()
	fp, err := s.fpCalculator.Calculate(ctx, targetSMILES, fpType, opts)
	if err != nil {
		return nil, err
	}

	metric := MetricTanimoto
	if fp.IsDenseVector() {
		metric = MetricCosine
	}

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

	ids1, err1 := s.fpCalculator.Standardize(ctx, smiles1)
	ids2, err2 := s.fpCalculator.Standardize(ctx, smiles2)

	opts := DefaultFingerprintCalcOptions()
	scores := make(map[FingerprintType]float64)

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

	identical := false
	if err1 == nil && err2 == nil {
		identical = ids1.InChIKey == ids2.InChIKey
	}

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

// AddMoleculeProperties adds properties to an existing molecule.
func (s *MoleculeService) AddMoleculeProperties(ctx context.Context, moleculeID string, properties []*MolecularProperty) error {
	mol, err := s.repo.FindByID(ctx, moleculeID)
	if err != nil { return err }
	for _, p := range properties {
		if err := mol.AddProperty(p); err != nil { return err }
	}
	return s.repo.Update(ctx, mol)
}

// TagMolecule adds tags to an existing molecule.
func (s *MoleculeService) TagMolecule(ctx context.Context, moleculeID string, tags []string) error {
	mol, err := s.repo.FindByID(ctx, moleculeID)
	if err != nil { return err }
	for _, t := range tags {
		if err := mol.AddTag(t); err != nil { return err }
	}
	return s.repo.Update(ctx, mol)
}

//Personal.AI order the ending
