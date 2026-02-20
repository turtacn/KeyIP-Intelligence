package patent_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	pkgerrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	common "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// In-memory Repository implementation for contract tests
// ─────────────────────────────────────────────────────────────────────────────

// memRepository is a thread-safe, in-memory implementation of patent.Repository
// used exclusively for contract testing.  It must not be used in production.
type memRepository struct {
	mu      sync.RWMutex
	byID    map[common.ID]*patent.Patent
	byNum   map[string]*patent.Patent
}

func newMemRepository() *memRepository {
	return &memRepository{
		byID:  make(map[common.ID]*patent.Patent),
		byNum: make(map[string]*patent.Patent),
	}
}

func (r *memRepository) Save(ctx context.Context, p *patent.Patent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byNum[p.PatentNumber]; exists {
		return pkgerrors.New(pkgerrors.CodeConflict, "patent number already exists").
			WithDetail("number=" + p.PatentNumber)
	}
	clone := *p
	r.byID[p.BaseEntity.ID] = &clone
	r.byNum[p.PatentNumber] = &clone
	return nil
}

func (r *memRepository) FindByID(ctx context.Context, id common.ID) (*patent.Patent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byID[id]
	if !ok {
		return nil, pkgerrors.NotFound("patent not found").WithDetail("id=" + string(id))
	}
	clone := *p
	return &clone, nil
}

func (r *memRepository) FindByNumber(ctx context.Context, number string) (*patent.Patent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byNum[number]
	if !ok {
		return nil, pkgerrors.NotFound("patent not found").WithDetail("number=" + number)
	}
	clone := *p
	return &clone, nil
}

func (r *memRepository) FindByFamilyID(ctx context.Context, familyID string) ([]*patent.Patent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*patent.Patent
	for _, p := range r.byID {
		if p.FamilyID == familyID {
			clone := *p
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (r *memRepository) Search(ctx context.Context, req ptypes.PatentSearchRequest) (*ptypes.PatentSearchResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var items []ptypes.PatentDTO
	for _, p := range r.byID {
		items = append(items, p.ToDTO())
	}
	resp := common.NewPageResponse(items, int64(len(items)), req.PageRequest)
	patentResp := ptypes.PatentSearchResponse(resp)
	return &patentResp, nil
}

func (r *memRepository) Update(ctx context.Context, p *patent.Patent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.byID[p.BaseEntity.ID]
	if !ok {
		return pkgerrors.NotFound("patent not found").WithDetail("id=" + string(p.BaseEntity.ID))
	}
	if existing.Version != p.Version-1 {
		return pkgerrors.New(pkgerrors.CodeConflict, "optimistic lock conflict")
	}
	clone := *p
	r.byID[p.BaseEntity.ID] = &clone
	if existing.PatentNumber != p.PatentNumber {
		delete(r.byNum, existing.PatentNumber)
	}
	r.byNum[p.PatentNumber] = &clone
	return nil
}

func (r *memRepository) Delete(ctx context.Context, id common.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byID[id]
	if !ok {
		return pkgerrors.NotFound("patent not found").WithDetail("id=" + string(id))
	}
	delete(r.byNum, p.PatentNumber)
	delete(r.byID, id)
	return nil
}

func (r *memRepository) FindByApplicant(ctx context.Context, applicant string, page common.PageRequest) (*common.PageResponse[*patent.Patent], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var items []*patent.Patent
	for _, p := range r.byID {
		if p.Applicant == applicant {
			clone := *p
			items = append(items, &clone)
		}
	}
	resp := common.NewPageResponse(items, int64(len(items)), page)
	return &resp, nil
}

func (r *memRepository) FindByJurisdiction(ctx context.Context, j ptypes.JurisdictionCode, page common.PageRequest) (*common.PageResponse[*patent.Patent], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var items []*patent.Patent
	for _, p := range r.byID {
		if p.Jurisdiction == j {
			clone := *p
			items = append(items, &clone)
		}
	}
	resp := common.NewPageResponse(items, int64(len(items)), page)
	return &resp, nil
}

func (r *memRepository) FindByIPCCode(ctx context.Context, ipcCode string, page common.PageRequest) (*common.PageResponse[*patent.Patent], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var items []*patent.Patent
	for _, p := range r.byID {
		for _, code := range p.IPCCodes {
			if len(code) >= len(ipcCode) && code[:len(ipcCode)] == ipcCode {
				clone := *p
				items = append(items, &clone)
				break
			}
		}
	}
	resp := common.NewPageResponse(items, int64(len(items)), page)
	return &resp, nil
}

func (r *memRepository) CountByStatus(ctx context.Context) (map[ptypes.PatentStatus]int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	counts := make(map[ptypes.PatentStatus]int64)
	for _, p := range r.byID {
		counts[p.Status]++
	}
	return counts, nil
}

func (r *memRepository) FindExpiring(ctx context.Context, before time.Time) ([]*patent.Patent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*patent.Patent
	for _, p := range r.byID {
		if p.ExpiryDate != nil && !p.ExpiryDate.After(before) {
			clone := *p
			result = append(result, &clone)
		}
	}
	return result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Contract test suite
// ─────────────────────────────────────────────────────────────────────────────

// RepositoryContractTest executes the standardised repository contract test
// suite against any patent.Repository implementation.  Both the in-memory
// stub and the Postgres implementation must pass this suite.
func RepositoryContractTest(t *testing.T, repo patent.Repository) {
	t.Helper()

	t.Run("SaveAndFindByID", func(t *testing.T) {
		ctx := context.Background()
		p := buildTestPatent(t, "CN202300001A", ptypes.JurisdictionCN)
		require.NoError(t, repo.Save(ctx, p))

		found, err := repo.FindByID(ctx, p.BaseEntity.ID)
		require.NoError(t, err)
		assert.Equal(t, p.BaseEntity.ID, found.BaseEntity.ID)
		assert.Equal(t, p.PatentNumber, found.PatentNumber)
		assert.Equal(t, p.Title, found.Title)
	})

	t.Run("FindByNumber", func(t *testing.T) {
		ctx := context.Background()
		p := buildTestPatent(t, "CN202300002A", ptypes.JurisdictionCN)
		require.NoError(t, repo.Save(ctx, p))

		found, err := repo.FindByNumber(ctx, p.PatentNumber)
		require.NoError(t, err)
		assert.Equal(t, p.PatentNumber, found.PatentNumber)
	})

	t.Run("FindByID_NotFound", func(t *testing.T) {
		ctx := context.Background()
		_, err := repo.FindByID(ctx, common.NewID())
		require.Error(t, err)
		assert.True(t, pkgerrors.IsCode(err, pkgerrors.CodeNotFound),
			"expected CodeNotFound, got: %v", err)
	})

	t.Run("UpdateAndFindByID", func(t *testing.T) {
		ctx := context.Background()
		p := buildTestPatent(t, "CN202300003A", ptypes.JurisdictionCN)
		require.NoError(t, repo.Save(ctx, p))

		p.Title = "Updated Title"
		p.Version++ // simulate version bump
		require.NoError(t, repo.Update(ctx, p))

		found, err := repo.FindByID(ctx, p.BaseEntity.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Title", found.Title)
	})

	t.Run("DeleteAndFindByID", func(t *testing.T) {
		ctx := context.Background()
		p := buildTestPatent(t, "CN202300004A", ptypes.JurisdictionCN)
		require.NoError(t, repo.Save(ctx, p))
		require.NoError(t, repo.Delete(ctx, p.BaseEntity.ID))

		_, err := repo.FindByID(ctx, p.BaseEntity.ID)
		require.Error(t, err)
		assert.True(t, pkgerrors.IsCode(err, pkgerrors.CodeNotFound))
	})

	t.Run("Search_ReturnsResults", func(t *testing.T) {
		ctx := context.Background()
		p := buildTestPatent(t, "CN202300005A", ptypes.JurisdictionCN)
		require.NoError(t, repo.Save(ctx, p))

		req := ptypes.PatentSearchRequest{
			PageRequest: common.PageRequest{Page: 1, PageSize: 20},
		}
		resp, err := repo.Search(ctx, req)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.Total, int64(1))
	})

	t.Run("FindExpiring", func(t *testing.T) {
		ctx := context.Background()
		expiry := time.Now().Add(24 * time.Hour)
		p := buildTestPatentWithExpiry(t, "CN202300006A", ptypes.JurisdictionCN, &expiry)
		require.NoError(t, repo.Save(ctx, p))

		results, err := repo.FindExpiring(ctx, time.Now().Add(48*time.Hour))
		require.NoError(t, err)

		found := false
		for _, r := range results {
			if r.BaseEntity.ID == p.BaseEntity.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "expiring patent should appear in FindExpiring results")
	})

	t.Run("CountByStatus", func(t *testing.T) {
		ctx := context.Background()
		counts, err := repo.CountByStatus(ctx)
		require.NoError(t, err)
		assert.NotNil(t, counts)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Test entry point — runs the contract suite against the in-memory repository
// ─────────────────────────────────────────────────────────────────────────────

func TestRepository_InMemoryContractSuite(t *testing.T) {
	t.Parallel()
	RepositoryContractTest(t, newMemRepository())
}

// ─────────────────────────────────────────────────────────────────────────────
// Fixture builders
// ─────────────────────────────────────────────────────────────────────────────

func buildTestPatent(t *testing.T, number string, jurisdiction ptypes.JurisdictionCode) *patent.Patent {
	t.Helper()
	p, err := patent.NewPatent(
		number,
		"Test Patent Title "+number,
		"Test abstract.",
		"Test Corp",
		jurisdiction,
		time.Now(),
	)
	require.NoError(t, err)
	p.Inventors = []string{"Inventor A"}
	return p
}

func buildTestPatentWithExpiry(t *testing.T, number string, jurisdiction ptypes.JurisdictionCode, expiry *time.Time) *patent.Patent {
	t.Helper()
	p := buildTestPatent(t, number, jurisdiction)
	p.ExpiryDate = expiry
	return p
}

