package molecule

import (
	"context"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// MoleculeDomainService defines the interface for molecule domain operations.
type MoleculeDomainService interface {
	Canonicalize(ctx context.Context, smiles string) (string, string, error)
	CanonicalizeFromInChI(ctx context.Context, inchi string) (string, string, error)
}

// Service defines the application interface for molecule operations.
type Service interface {
	RegisterMolecule(ctx context.Context, smiles string, source MoleculeSource, sourceRef string) (*Molecule, error)
	BatchRegisterMolecules(ctx context.Context, requests []MoleculeRegistrationRequest) (*BatchRegistrationResult, error)
	GetMolecule(ctx context.Context, id string) (*Molecule, error)
	GetMoleculeByInChIKey(ctx context.Context, inchiKey string) (*Molecule, error)
	SearchMolecules(ctx context.Context, query *MoleculeQuery) (*MoleculeSearchResult, error)
	CalculateFingerprints(ctx context.Context, moleculeID string, fpTypes []FingerprintType) error
	FindSimilarMolecules(ctx context.Context, targetSMILES string, fpType FingerprintType, threshold float64, limit int) ([]*SimilarityResult, error)
	CompareMolecules(ctx context.Context, smiles1, smiles2 string, fpTypes []FingerprintType) (*MoleculeComparisonResult, error)
	ArchiveMolecule(ctx context.Context, id string) error
	DeleteMolecule(ctx context.Context, id string) error
	AddMoleculeProperties(ctx context.Context, moleculeID string, properties []*MolecularProperty) error
	TagMolecule(ctx context.Context, moleculeID string, tags []string) error
	CreateFromSMILES(ctx context.Context, smiles string, metadata map[string]string) (*Molecule, error)

	// Embed DomainService methods as MoleculeService implements them
	MoleculeDomainService
}

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
	mol, err := NewMolecule(smiles, source, sourceRef)
	if err != nil {
		return nil, err
	}

	canonical, inchi, inchiKey, formula, weight, err := s.fpCalculator.Standardize(ctx, smiles)
	if err != nil {
		s.logger.Error("Standardization failed", logging.String("smiles", smiles), logging.Error(err))
		return nil, errors.Wrap(err, errors.ErrCodeMoleculeParsingFailed, "failed to standardize molecule")
	}

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

	if err := mol.SetStructureIdentifiers(canonical, inchi, inchiKey, formula, weight); err != nil {
		return nil, err
	}

	opts := DefaultFingerprintCalcOptions()

	morgan, err := s.fpCalculator.Calculate(ctx, canonical, FingerprintMorgan, opts)
	if err != nil {
		s.logger.Warn("Failed to calculate Morgan fingerprint", logging.String("smiles", canonical), logging.Error(err))
	} else {
		_ = mol.AddFingerprint(morgan)
	}

	maccs, err := s.fpCalculator.Calculate(ctx, canonical, FingerprintMACCS, opts)
	if err != nil {
		s.logger.Warn("Failed to calculate MACCS fingerprint", logging.String("smiles", canonical), logging.Error(err))
	} else {
		_ = mol.AddFingerprint(maccs)
	}

	if err := mol.Activate(); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, mol); err != nil {
		return nil, err
	}

	return mol, nil
}

// CreateFromSMILES creates a molecule from SMILES string with metadata, delegating to RegisterMolecule.
// This provides backward compatibility with legacy service interface.
func (s *MoleculeService) CreateFromSMILES(ctx context.Context, smiles string, metadata map[string]string) (*Molecule, error) {
	source := SourceManual
	sourceRef := "unknown"

	if val, ok := metadata["source_document"]; ok && val != "" {
		source = SourcePatent
		sourceRef = val
	} else if val, ok := metadata["extraction_id"]; ok && val != "" {
		source = SourcePrediction
		sourceRef = val
	}

	mol, err := s.RegisterMolecule(ctx, smiles, source, sourceRef)
	if err != nil {
		return nil, err
	}

	if len(metadata) > 0 {
		for k, v := range metadata {
			mol.SetMetadata(k, v)
		}
		if err := s.repo.Update(ctx, mol); err != nil {
			s.logger.Warn("failed to update molecule metadata", logging.Error(err))
		}
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

	type pendingMol struct {
		mol       *Molecule
		canonical string
		index     int
	}
	var toRegister []pendingMol
	var canonicals []string

	for i, req := range requests {
		mol, err := NewMolecule(req.SMILES, req.Source, req.SourceRef)
		if err != nil {
			result.Failed = append(result.Failed, BatchRegistrationError{i, req.SMILES, err})
			continue
		}

		canonical, inchi, inchiKey, formula, weight, err := s.fpCalculator.Standardize(ctx, req.SMILES)
		if err != nil {
			result.Failed = append(result.Failed, BatchRegistrationError{i, req.SMILES, err})
			continue
		}

		exists, err := s.repo.ExistsByInChIKey(ctx, inchiKey)
		if err != nil {
			result.Failed = append(result.Failed, BatchRegistrationError{i, req.SMILES, err})
			continue
		}
		if exists {
			result.DuplicateCount++
			continue
		}

		_ = mol.SetStructureIdentifiers(canonical, inchi, inchiKey, formula, weight)

		for _, tag := range req.Tags { _ = mol.AddTag(tag) }
		for _, prop := range req.Properties { _ = mol.AddProperty(prop) }

		toRegister = append(toRegister, pendingMol{mol, canonical, i})
		canonicals = append(canonicals, canonical)
	}

	if len(toRegister) > 0 {
		opts := DefaultFingerprintCalcOptions()

		morgans, err := s.fpCalculator.BatchCalculate(ctx, canonicals, FingerprintMorgan, opts)
		if err != nil {
			s.logger.Error("Batch Morgan calculation failed", logging.Error(err))
		}

		maccss, err := s.fpCalculator.BatchCalculate(ctx, canonicals, FingerprintMACCS, opts)
		if err != nil {
			s.logger.Error("Batch MACCS calculation failed", logging.Error(err))
		}

		var batchToSave []*Molecule

		for j, pm := range toRegister {
			if j < len(morgans) && morgans[j] != nil {
				_ = pm.mol.AddFingerprint(morgans[j])
			}
			if j < len(maccss) && maccss[j] != nil {
				_ = pm.mol.AddFingerprint(maccss[j])
			}

			if err := pm.mol.Activate(); err != nil {
				result.Failed = append(result.Failed, BatchRegistrationError{pm.index, pm.mol.SMILES, err})
				continue
			}

			batchToSave = append(batchToSave, pm.mol)
		}

		if len(batchToSave) > 0 {
			count, err := s.repo.BatchSave(ctx, batchToSave)
			if err != nil {
				for range batchToSave {
					// All failed if batch save fails
				}
				return result, errors.Wrap(err, errors.ErrCodeInternal, "batch save failed")
			}
			result.Succeeded = append(result.Succeeded, batchToSave[:count]...)
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

	smiles := mol.CanonicalSMILES
	if smiles == "" {
		smiles = mol.SMILES
	}

	for _, ft := range fpTypes {
		if mol.HasFingerprint(ft) {
			continue
		}

		fp, err := s.fpCalculator.Calculate(ctx, smiles, ft, opts)
		if err != nil {
			s.logger.Error("Fingerprint calculation failed", logging.String("type", string(ft)), logging.Error(err))
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
		limit = 100
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

// Canonicalize implements MoleculeDomainService.
func (s *MoleculeService) Canonicalize(ctx context.Context, smiles string) (string, string, error) {
	canonical, _, inchiKey, _, _, err := s.fpCalculator.Standardize(ctx, smiles)
	if err != nil {
		return "", "", err
	}
	return canonical, inchiKey, nil
}

// CanonicalizeFromInChI implements MoleculeDomainService.
func (s *MoleculeService) CanonicalizeFromInChI(ctx context.Context, inchi string) (string, string, error) {
	canonical, _, inchiKey, _, _, err := s.fpCalculator.Standardize(ctx, inchi)
	if err != nil {
		return "", "", err
	}
	return canonical, inchiKey, nil
}

// Compile-time check
var _ MoleculeDomainService = (*MoleculeService)(nil)
var _ Service = (*MoleculeService)(nil)

//Personal.AI order the ending
