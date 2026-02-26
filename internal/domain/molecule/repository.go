package molecule

import (
	"context"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// MoleculeRepository defines the interface for molecular data persistence.
type MoleculeRepository interface {
	// Command methods (Write)
	Save(ctx context.Context, molecule *Molecule) error
	Update(ctx context.Context, molecule *Molecule) error
	Delete(ctx context.Context, id string) error
	BatchSave(ctx context.Context, molecules []*Molecule) (int, error)

	// Query methods (Read)
	FindByID(ctx context.Context, id string) (*Molecule, error)
	FindByInChIKey(ctx context.Context, inchiKey string) (*Molecule, error)
	FindBySMILES(ctx context.Context, smiles string) ([]*Molecule, error)
	FindByIDs(ctx context.Context, ids []string) ([]*Molecule, error)
	Exists(ctx context.Context, id string) (bool, error)
	ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error)

	// Search methods
	Search(ctx context.Context, query *MoleculeQuery) (*MoleculeSearchResult, error)
	Count(ctx context.Context, query *MoleculeQuery) (int64, error)
	FindBySource(ctx context.Context, source MoleculeSource, offset, limit int) ([]*Molecule, error)
	FindByStatus(ctx context.Context, status MoleculeStatus, offset, limit int) ([]*Molecule, error)
	FindByTags(ctx context.Context, tags []string, offset, limit int) ([]*Molecule, error)
	FindByMolecularWeightRange(ctx context.Context, minWeight, maxWeight float64, offset, limit int) ([]*Molecule, error)

	// Fingerprint related
	FindWithFingerprint(ctx context.Context, fpType FingerprintType, offset, limit int) ([]*Molecule, error)
	FindWithoutFingerprint(ctx context.Context, fpType FingerprintType, offset, limit int) ([]*Molecule, error)
}

// MoleculeQuery defines criteria for searching molecules.
type MoleculeQuery struct {
	IDs                 []string
	SMILES              string // Exact or partial? Usually exact or use Pattern
	SMILESPattern       string // LIKE %pattern%
	InChIKeys           []string
	MolecularFormula    string
	MinMolecularWeight  *float64
	MaxMolecularWeight  *float64
	Sources             []MoleculeSource
	Statuses            []MoleculeStatus
	Tags                []string
	HasFingerprintTypes []FingerprintType
	PropertyFilters     []PropertyFilter
	CreatedAfter        *time.Time
	CreatedBefore       *time.Time
	Keyword             string // Fulltext search
	Offset              int
	Limit               int
	SortBy              string
	SortOrder           string
}

// PropertyFilter defines range criteria for molecular properties.
type PropertyFilter struct {
	Name     string
	MinValue *float64
	MaxValue *float64
	Unit     string
}

// MoleculeSearchResult contains results of a molecule search.
type MoleculeSearchResult struct {
	Molecules []*Molecule
	Total     int64
	Offset    int
	Limit     int
	HasMore   bool
}

// IsEmpty checks if the result is empty.
func (r *MoleculeSearchResult) IsEmpty() bool {
	return len(r.Molecules) == 0
}

// Validate ensures query parameters are valid.
func (q *MoleculeQuery) Validate() error {
	if q.Limit < 0 {
		return errors.New(errors.ErrCodeValidation, "limit cannot be negative")
	}
	if q.Limit == 0 {
		q.Limit = 20
	}
	if q.Limit > 1000 {
		return errors.New(errors.ErrCodeValidation, "limit cannot exceed 1000")
	}
	if q.Offset < 0 {
		return errors.New(errors.ErrCodeValidation, "offset cannot be negative")
	}

	// SortBy whitelist
	if q.SortBy != "" {
		switch q.SortBy {
		case "created_at", "updated_at", "molecular_weight", "smiles":
			// valid
		default:
			return errors.New(errors.ErrCodeValidation, "invalid sort_by field: "+q.SortBy)
		}
	}

	// SortOrder
	if q.SortOrder == "" {
		q.SortOrder = "desc"
	}
	if q.SortOrder != "asc" && q.SortOrder != "desc" {
		return errors.New(errors.ErrCodeValidation, "invalid sort_order: "+q.SortOrder)
	}

	// Weight range
	if q.MinMolecularWeight != nil && q.MaxMolecularWeight != nil {
		if *q.MinMolecularWeight > *q.MaxMolecularWeight {
			return errors.New(errors.ErrCodeValidation, "min_molecular_weight cannot be greater than max_molecular_weight")
		}
	}
	if q.MinMolecularWeight != nil && *q.MinMolecularWeight < 0 {
		return errors.New(errors.ErrCodeValidation, "min_molecular_weight cannot be negative")
	}

	// Statuses
	for _, s := range q.Statuses {
		if !s.IsValid() {
			return errors.New(errors.ErrCodeValidation, "invalid status in query")
		}
	}

	// Sources
	for _, s := range q.Sources {
		if !s.IsValid() {
			return errors.New(errors.ErrCodeValidation, "invalid source in query")
		}
	}

	// Property filters
	for _, f := range q.PropertyFilters {
		if f.Name == "" {
			return errors.New(errors.ErrCodeValidation, "property filter name cannot be empty")
		}
		if f.MinValue != nil && f.MaxValue != nil {
			if *f.MinValue > *f.MaxValue {
				return errors.New(errors.ErrCodeValidation, "min_value cannot be greater than max_value for property "+f.Name)
			}
		}
	}

	return nil
}

// MoleculeUnitOfWork defines an interface for atomic operations across repositories.
type MoleculeUnitOfWork interface {
	MoleculeRepo() MoleculeRepository
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

//Personal.AI order the ending
