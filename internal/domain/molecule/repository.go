// Package molecule defines the repository interface for molecular entity persistence.
package molecule

import (
	"context"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	mtypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// Repository defines the persistence contract for Molecule aggregates.
// Implementations must ensure transactional consistency and handle concurrent
// access safely (optimistic locking via Version field).
type Repository interface {
	// Save persists a new molecule or updates an existing one based on ID.
	// Returns errors.CodeConflict if Version mismatch detected (optimistic lock).
	Save(ctx context.Context, mol *Molecule) error

	// FindByID retrieves a molecule by its unique identifier.
	// Returns errors.CodeNotFound if no molecule with the given ID exists.
	FindByID(ctx context.Context, id common.ID) (*Molecule, error)

	// FindBySMILES retrieves a molecule by its SMILES string.
	// Returns errors.CodeNotFound if no matching molecule exists.
	FindBySMILES(ctx context.Context, smiles string) (*Molecule, error)

	// FindByInChIKey retrieves a molecule by its InChIKey identifier.
	// Returns errors.CodeNotFound if no matching molecule exists.
	FindByInChIKey(ctx context.Context, inchiKey string) (*Molecule, error)

	// Search performs a paginated search for molecules matching the given criteria.
	// Supports filtering by type, molecular weight range, property ranges, and
	// full-text search on name/synonyms.
	Search(ctx context.Context, req mtypes.MoleculeSearchRequest) (*mtypes.MoleculeSearchResponse, error)

	// FindSimilar retrieves molecules with fingerprint similarity above the
	// threshold, ordered by descending similarity score.  This method delegates
	// to the Milvus vector search engine for efficient approximate nearest
	// neighbor (ANN) search.
	FindSimilar(ctx context.Context, fingerprint *Fingerprint, fpType mtypes.FingerprintType, threshold float64, maxResults int) ([]*Molecule, error)

	// SubstructureSearch finds molecules containing the specified substructure
	// pattern (SMARTS notation).  This requires substructure matching capability
	// in the backend (e.g., RDKit PostgreSQL cartridge).
	SubstructureSearch(ctx context.Context, smarts string, maxResults int) ([]*Molecule, error)

	// Update modifies an existing molecule.
	// Returns errors.CodeConflict if Version mismatch (optimistic lock).
	// Returns errors.CodeNotFound if the molecule does not exist.
	Update(ctx context.Context, mol *Molecule) error

	// Delete removes a molecule by ID (soft delete: sets DeletedAt timestamp).
	// Returns errors.CodeNotFound if the molecule does not exist.
	Delete(ctx context.Context, id common.ID) error

	// FindByPatentID retrieves all molecules extracted from a specific patent.
	// Returns an empty slice if no molecules are associated with the patent.
	FindByPatentID(ctx context.Context, patentID common.ID) ([]*Molecule, error)

	// BatchSave persists multiple molecules in a single transaction for efficiency.
	// Partially successful batches are rolled back; it's all-or-nothing.
	BatchSave(ctx context.Context, molecules []*Molecule) error

	// Count returns the total number of non-deleted molecules in the repository.
	Count(ctx context.Context) (int64, error)
}

