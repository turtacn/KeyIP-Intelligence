// Package lifecycle_test provides unit tests for the lifecycle repository interface.
package lifecycle_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock repository for testing domain logic
// ─────────────────────────────────────────────────────────────────────────────

// MockLifecycleRepository is an in-memory implementation of the Repository
// interface for testing purposes.
type MockLifecycleRepository struct {
	lifecycles map[common.ID]*lifecycle.PatentLifecycle
}

// NewMockLifecycleRepository creates a new mock repository.
func NewMockLifecycleRepository() *MockLifecycleRepository {
	return &MockLifecycleRepository{
		lifecycles: make(map[common.ID]*lifecycle.PatentLifecycle),
	}
}

func (r *MockLifecycleRepository) Save(ctx context.Context, lc *lifecycle.PatentLifecycle) error {
	r.lifecycles[lc.ID] = lc
	return nil
}

func (r *MockLifecycleRepository) FindByID(ctx context.Context, id common.ID) (*lifecycle.PatentLifecycle, error) {
	lc, ok := r.lifecycles[id]
	if !ok {
		return nil, errors.NotFound(fmt.Sprintf("PatentLifecycle %s not found", id))
	}
	return lc, nil
}

func (r *MockLifecycleRepository) FindByPatentID(ctx context.Context, patentID common.ID) (*lifecycle.PatentLifecycle, error) {
	for _, lc := range r.lifecycles {
		if lc.PatentID == patentID {
			return lc, nil
		}
	}
	return nil, errors.NotFound(fmt.Sprintf("PatentLifecycle for patent %s not found", patentID))
}

func (r *MockLifecycleRepository) FindByPatentNumber(ctx context.Context, patentNumber string) (*lifecycle.PatentLifecycle, error) {
	for _, lc := range r.lifecycles {
		if lc.PatentNumber == patentNumber {
			return lc, nil
		}
	}
	return nil, errors.NotFound(fmt.Sprintf("PatentLifecycle for patent number %s not found", patentNumber))
}

func (r *MockLifecycleRepository) FindUpcomingDeadlines(ctx context.Context, withinDays int, tenantID *common.TenantID) ([]*lifecycle.PatentLifecycle, error) {
	var result []*lifecycle.PatentLifecycle
	for _, lc := range r.lifecycles {
		if tenantID != nil && lc.TenantID != *tenantID {
			continue
		}
		upcoming := lc.GetUpcomingDeadlines(withinDays)
		if len(upcoming) > 0 {
			result = append(result, lc)
		}
	}
	return result, nil
}

func (r *MockLifecycleRepository) FindOverdueDeadlines(ctx context.Context, tenantID *common.TenantID) ([]*lifecycle.PatentLifecycle, error) {
	var result []*lifecycle.PatentLifecycle
	for _, lc := range r.lifecycles {
		if tenantID != nil && lc.TenantID != *tenantID {
			continue
		}
		overdue := lc.GetOverdueDeadlines()
		if len(overdue) > 0 {
			result = append(result, lc)
		}
	}
	return result, nil
}

func (r *MockLifecycleRepository) FindUpcomingAnnuities(ctx context.Context, withinDays int, tenantID *common.TenantID) ([]*lifecycle.PatentLifecycle, error) {
	var result []*lifecycle.PatentLifecycle
	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, withinDays)

	for _, lc := range r.lifecycles {
		if tenantID != nil && lc.TenantID != *tenantID {
			continue
		}
		for _, annuity := range lc.AnnuitySchedule {
			if !annuity.Paid && annuity.DueDate.Before(cutoff) {
				result = append(result, lc)
				break
			}
		}
	}
	return result, nil
}

func (r *MockLifecycleRepository) FindByJurisdiction(ctx context.Context, jurisdiction ptypes.JurisdictionCode, tenantID *common.TenantID) ([]*lifecycle.PatentLifecycle, error) {
	var result []*lifecycle.PatentLifecycle
	for _, lc := range r.lifecycles {
		if tenantID != nil && lc.TenantID != *tenantID {
			continue
		}
		if lc.Jurisdiction == jurisdiction {
			result = append(result, lc)
		}
	}
	return result, nil
}

func (r *MockLifecycleRepository) Delete(ctx context.Context, id common.ID) error {
	delete(r.lifecycles, id)
	return nil
}

func (r *MockLifecycleRepository) List(ctx context.Context, offset, limit int, tenantID *common.TenantID) ([]*lifecycle.PatentLifecycle, int64, error) {
	var result []*lifecycle.PatentLifecycle
	for _, lc := range r.lifecycles {
		if tenantID != nil && lc.TenantID != *tenantID {
			continue
		}
		result = append(result, lc)
	}

	total := int64(len(result))
	if offset >= len(result) {
		return []*lifecycle.PatentLifecycle{}, total, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Repository contract tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMockRepository_SaveAndFindByID(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	ctx := context.Background()

	lc, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)

	err = repo.Save(ctx, lc)
	require.NoError(t, err)

	retrieved, err := repo.FindByID(ctx, lc.ID)
	require.NoError(t, err)
	assert.Equal(t, lc.ID, retrieved.ID)
	assert.Equal(t, lc.PatentNumber, retrieved.PatentNumber)
}

func TestMockRepository_FindByPatentNumber(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	ctx := context.Background()

	lc, _ := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	_ = repo.Save(ctx, lc)

	retrieved, err := repo.FindByPatentNumber(ctx, "CN202010000001A")
	require.NoError(t, err)
	assert.Equal(t, lc.ID, retrieved.ID)
}

func TestMockRepository_FindByPatentID(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	ctx := context.Background()

	patentID := common.NewID()
	lc, _ := lifecycle.NewPatentLifecycle(
		patentID,
		"CN202010000001A",
		ptypes.JurisdictionCN,
		time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	_ = repo.Save(ctx, lc)

	retrieved, err := repo.FindByPatentID(ctx, patentID)
	require.NoError(t, err)
	assert.Equal(t, patentID, retrieved.PatentID)
}

func TestMockRepository_FindUpcomingDeadlines(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	ctx := context.Background()

	lc, _ := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	)

	// Add a deadline due in 5 days.
	deadline, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 5),
		lifecycle.PriorityHigh,
		"Test deadline",
	)
	_ = lc.AddDeadline(*deadline)
	_ = repo.Save(ctx, lc)

	results, err := repo.FindUpcomingDeadlines(ctx, 10, nil)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, lc.ID, results[0].ID)
}

func TestMockRepository_FindOverdueDeadlines(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	ctx := context.Background()

	lc, _ := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	)

	// Add an overdue deadline.
	deadline, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, -5),
		lifecycle.PriorityCritical,
		"Overdue deadline",
	)
	_ = lc.AddDeadline(*deadline)
	_ = repo.Save(ctx, lc)

	results, err := repo.FindOverdueDeadlines(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, lc.ID, results[0].ID)
}

func TestMockRepository_FindByJurisdiction(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	ctx := context.Background()

	lc1, _ := lifecycle.NewPatentLifecycle(common.NewID(), "CN202010000001A", ptypes.JurisdictionCN, time.Now())
	lc2, _ := lifecycle.NewPatentLifecycle(common.NewID(), "US11123456B2", ptypes.JurisdictionUS, time.Now())
	_ = repo.Save(ctx, lc1)
	_ = repo.Save(ctx, lc2)

	cnLifecycles, err := repo.FindByJurisdiction(ctx, ptypes.JurisdictionCN, nil)
	require.NoError(t, err)
	assert.Len(t, cnLifecycles, 1)
	assert.Equal(t, ptypes.JurisdictionCN, cnLifecycles[0].Jurisdiction)
}

func TestMockRepository_Delete(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	ctx := context.Background()

	lc, _ := lifecycle.NewPatentLifecycle(common.NewID(), "CN202010000001A", ptypes.JurisdictionCN, time.Now())
	_ = repo.Save(ctx, lc)

	err := repo.Delete(ctx, lc.ID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, lc.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMockRepository_List(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	ctx := context.Background()

	// Add 5 lifecycles.
	for i := 0; i < 5; i++ {
		lc, _ := lifecycle.NewPatentLifecycle(
			common.NewID(),
			"CN20201000000"+string(rune('1'+i))+"A",
			ptypes.JurisdictionCN,
			time.Now(),
		)
		_ = repo.Save(ctx, lc)
	}

	results, total, err := repo.List(ctx, 0, 3, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, results, 3)
}

