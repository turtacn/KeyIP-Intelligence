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
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// ─────────────────────────────────────────────────────────────────────────────
// mockRepository — full in-memory Repository for service tests
// ─────────────────────────────────────────────────────────────────────────────

// mockRepository is a thread-safe, in-memory Repository used exclusively in
// service unit tests.  It differs from the contract-test helper in that it
// intentionally exposes control knobs (e.g., forced errors) for behaviour-
// driven test scenarios.
type mockRepository struct {
	mu       sync.RWMutex
	byID     map[common.ID]*patent.Patent
	byNum    map[string]*patent.Patent
	byFamily map[string][]*patent.Patent

	// Hooks for injecting failures.
	saveErr   error
	updateErr error
}

func newMockRepo() *mockRepository {
	return &mockRepository{
		byID:     make(map[common.ID]*patent.Patent),
		byNum:    make(map[string]*patent.Patent),
		byFamily: make(map[string][]*patent.Patent),
	}
}

func (r *mockRepository) Save(ctx context.Context, p *patent.Patent) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byNum[p.PatentNumber]; exists {
		return pkgerrors.New(pkgerrors.CodeConflict, "patent number already exists")
	}
	clone := *p
	r.byID[p.BaseEntity.ID] = &clone
	r.byNum[p.PatentNumber] = &clone
	if p.FamilyID != "" {
		r.byFamily[p.FamilyID] = append(r.byFamily[p.FamilyID], &clone)
	}
	return nil
}

func (r *mockRepository) FindByID(ctx context.Context, id common.ID) (*patent.Patent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byID[id]
	if !ok {
		return nil, pkgerrors.NotFound("patent not found").WithDetail("id=" + string(id))
	}
	clone := *p
	return &clone, nil
}

func (r *mockRepository) FindByNumber(ctx context.Context, number string) (*patent.Patent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byNum[number]
	if !ok {
		return nil, pkgerrors.NotFound("patent not found").WithDetail("number=" + number)
	}
	clone := *p
	return &clone, nil
}

func (r *mockRepository) FindByFamilyID(ctx context.Context, familyID string) ([]*patent.Patent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byFamily[familyID], nil
}

func (r *mockRepository) Search(ctx context.Context, req ptypes.PatentSearchRequest) (*ptypes.PatentSearchResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var dtos []ptypes.PatentDTO
	for _, p := range r.byID {
		dtos = append(dtos, p.ToDTO())
	}
	resp := common.NewPageResponse(dtos, int64(len(dtos)), req.Pagination)
	pr := ptypes.PatentSearchResponse(resp)
	return &pr, nil
}

func (r *mockRepository) Update(ctx context.Context, p *patent.Patent) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[p.BaseEntity.ID]; !ok {
		return pkgerrors.NotFound("patent not found")
	}
	clone := *p
	r.byID[p.BaseEntity.ID] = &clone
	r.byNum[p.PatentNumber] = &clone
	return nil
}

func (r *mockRepository) Delete(ctx context.Context, id common.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byID[id]
	if !ok {
		return pkgerrors.NotFound("patent not found")
	}
	delete(r.byNum, p.PatentNumber)
	delete(r.byID, id)
	return nil
}

func (r *mockRepository) FindByApplicant(ctx context.Context, applicant string, page common.PageRequest) (*common.PageResponse[*patent.Patent], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var items []*patent.Patent
	for _, p := range r.byID {
		for _, a := range []string{p.Applicant} {
			if a == applicant {
				clone := *p
				items = append(items, &clone)
				break
			}
		}
	}
	resp := common.NewPageResponse(items, int64(len(items)), page)
	return &resp, nil
}

func (r *mockRepository) FindByJurisdiction(ctx context.Context, j ptypes.JurisdictionCode, page common.PageRequest) (*common.PageResponse[*patent.Patent], error) {
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

func (r *mockRepository) FindByIPCCode(ctx context.Context, ipcCode string, page common.PageRequest) (*common.PageResponse[*patent.Patent], error) {
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

func (r *mockRepository) CountByStatus(ctx context.Context) (map[ptypes.PatentStatus]int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	counts := make(map[ptypes.PatentStatus]int64)
	for _, p := range r.byID {
		counts[p.Status]++
	}
	return counts, nil
}

func (r *mockRepository) FindExpiring(ctx context.Context, before time.Time) ([]*patent.Patent, error) {
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
// Service test fixtures
// ─────────────────────────────────────────────────────────────────────────────

func newTestService(repo patent.Repository) *patent.Service {
	return patent.NewService(repo, logging.NewNopLogger())
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCreatePatent
// ─────────────────────────────────────────────────────────────────────────────

func TestService_CreatePatent_Success(t *testing.T) {
	t.Parallel()

	repo := newMockRepo()
	svc := newTestService(repo)

	p, err := svc.CreatePatent(
		context.Background(),
		"CN202310001234A",
		"Novel OLED Material",
		"An abstract about OLED.",
		ptypes.JurisdictionCN,
		[]string{"ACME Corp"},
		[]string{"Dr. Smith"},
		time.Now().Add(-365*24*time.Hour),
	)

	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "CN202310001234A", p.PatentNumber)
	assert.NotEmpty(t, string(p.BaseEntity.ID))
}

func TestService_CreatePatent_ThenGetPatent(t *testing.T) {
	t.Parallel()

	repo := newMockRepo()
	svc := newTestService(repo)

	created, err := svc.CreatePatent(
		context.Background(),
		"CN202310001235A",
		"Title",
		"Abstract",
		ptypes.JurisdictionCN,
		[]string{"Applicant"},
		[]string{"Inventor"},
		time.Now().Add(-100*24*time.Hour),
	)
	require.NoError(t, err)

	found, err := svc.GetPatent(context.Background(), created.BaseEntity.ID)
	require.NoError(t, err)
	assert.Equal(t, created.BaseEntity.ID, found.BaseEntity.ID)
	assert.Equal(t, "CN202310001235A", found.PatentNumber)
}

func TestService_CreatePatent_InvalidParams_EmptyNumber(t *testing.T) {
	t.Parallel()

	svc := newTestService(newMockRepo())
	_, err := svc.CreatePatent(
		context.Background(),
		"", // invalid: empty number
		"Title",
		"Abstract",
		ptypes.JurisdictionCN,
		[]string{"Applicant"},
		[]string{"Inventor"},
		time.Now(),
	)
	require.Error(t, err)
}

func TestService_CreatePatent_InvalidParams_EmptyTitle(t *testing.T) {
	t.Parallel()

	svc := newTestService(newMockRepo())
	_, err := svc.CreatePatent(
		context.Background(),
		"CN000001A",
		"", // invalid: empty title
		"Abstract",
		ptypes.JurisdictionCN,
		[]string{"Applicant"},
		[]string{"Inventor"},
		time.Now(),
	)
	require.Error(t, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestAddClaimToPatent
// ─────────────────────────────────────────────────────────────────────────────

func TestService_AddClaimToPatent_Success(t *testing.T) {
	t.Parallel()

	repo := newMockRepo()
	svc := newTestService(repo)

	p, err := svc.CreatePatent(
		context.Background(),
		"CN202310002000A",
		"Test Patent",
		"Abstract.",
		ptypes.JurisdictionCN,
		[]string{"Corp"},
		[]string{"Inv"},
		time.Now().Add(-200*24*time.Hour),
	)
	require.NoError(t, err)

	claim, err := patent.NewClaim(1, "A compound comprising indole.", ptypes.ClaimIndependent, nil)
	require.NoError(t, err)

	require.NoError(t, svc.AddClaimToPatent(context.Background(), p.BaseEntity.ID, *claim))

	// Verify the claim was persisted on the aggregate.
	updated, err := svc.GetPatent(context.Background(), p.BaseEntity.ID)
	require.NoError(t, err)
	require.NotEmpty(t, updated.Claims)
	assert.Equal(t, 1, updated.Claims[0].Number)
}

func TestService_AddClaimToPatent_PatentNotFound(t *testing.T) {
	t.Parallel()

	svc := newTestService(newMockRepo())
	claim, _ := patent.NewClaim(1, "A compound.", ptypes.ClaimIndependent, nil)

	err := svc.AddClaimToPatent(context.Background(), common.NewID(), *claim)
	require.Error(t, err)
	assert.True(t, pkgerrors.IsCode(err, pkgerrors.CodeNotFound))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestUpdatePatentStatus
// ─────────────────────────────────────────────────────────────────────────────

func TestService_UpdatePatentStatus_ValidTransition(t *testing.T) {
	t.Parallel()

	repo := newMockRepo()
	svc := newTestService(repo)

	p, err := svc.CreatePatent(
		context.Background(),
		"CN202310003000A",
		"Status Test Patent",
		"Abstract.",
		ptypes.JurisdictionCN,
		[]string{"Corp"},
		[]string{"Inv"},
		time.Now().Add(-300*24*time.Hour),
	)
	require.NoError(t, err)
	// Initial status should be Filed.
	assert.Equal(t, ptypes.StatusFiled, p.Status)

	// Filed → Published → Granted are valid transitions.
	require.NoError(t, svc.UpdatePatentStatus(
		context.Background(), p.BaseEntity.ID, ptypes.StatusPublished))
	require.NoError(t, svc.UpdatePatentStatus(
		context.Background(), p.BaseEntity.ID, ptypes.StatusGranted))

	updated, err := svc.GetPatent(context.Background(), p.BaseEntity.ID)
	require.NoError(t, err)
	assert.Equal(t, ptypes.StatusGranted, updated.Status)
}

func TestService_UpdatePatentStatus_InvalidTransition(t *testing.T) {
	t.Parallel()

	repo := newMockRepo()
	svc := newTestService(repo)

	p, err := svc.CreatePatent(
		context.Background(),
		"CN202310004000A",
		"Status Test Patent 2",
		"Abstract.",
		ptypes.JurisdictionCN,
		[]string{"Corp"},
		[]string{"Inv"},
		time.Now().Add(-10*24*time.Hour),
	)
	require.NoError(t, err)

	// Pending → Expired is not a valid direct transition in the domain model.
	err = svc.UpdatePatentStatus(
		context.Background(), p.BaseEntity.ID, ptypes.StatusExpired)
	require.Error(t, err, "invalid status transition should return an error")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestFindExpiringPatents
// ─────────────────────────────────────────────────────────────────────────────

func TestService_FindExpiringPatents_ReturnsExpiring(t *testing.T) {
	t.Parallel()

	repo := newMockRepo()
	svc := newTestService(repo)

	// Create a patent expiring in 10 days.
	p, err := svc.CreatePatent(
		context.Background(),
		"CN202310005000A",
		"Expiring Patent",
		"Abstract.",
		ptypes.JurisdictionCN,
		[]string{"Corp"},
		[]string{"Inv"},
		time.Now().Add(-20*365*24*time.Hour),
	)
	require.NoError(t, err)

	// Manually set the expiry date via the repository.
	expiry := time.Now().UTC().Add(10 * 24 * time.Hour)
	p.ExpiryDate = &expiry
	p.Version++
	require.NoError(t, repo.Update(context.Background(), p))

	results, err := svc.FindExpiringPatents(context.Background(), 30)
	require.NoError(t, err)

	found := false
	for _, r := range results {
		if r.BaseEntity.ID == p.BaseEntity.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "expiring patent should appear in results")
}

func TestService_FindExpiringPatents_ZeroDaysReturnsError(t *testing.T) {
	t.Parallel()

	svc := newTestService(newMockRepo())
	_, err := svc.FindExpiringPatents(context.Background(), 0)
	require.Error(t, err)
	assert.True(t, pkgerrors.IsCode(err, pkgerrors.CodeInvalidParam))
}

func TestService_FindExpiringPatents_NegativeDaysReturnsError(t *testing.T) {
	t.Parallel()

	svc := newTestService(newMockRepo())
	_, err := svc.FindExpiringPatents(context.Background(), -5)
	require.Error(t, err)
	assert.True(t, pkgerrors.IsCode(err, pkgerrors.CodeInvalidParam))
}

func TestService_FindExpiringPatents_NoneExpiring(t *testing.T) {
	t.Parallel()

	svc := newTestService(newMockRepo())
	results, err := svc.FindExpiringPatents(context.Background(), 7)
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestGetPatentFamily
// ─────────────────────────────────────────────────────────────────────────────

func TestService_GetPatentFamily_EmptyFamilyIDReturnsError(t *testing.T) {
	t.Parallel()

	svc := newTestService(newMockRepo())
	_, err := svc.GetPatentFamily(context.Background(), "")
	require.Error(t, err)
	assert.True(t, pkgerrors.IsCode(err, pkgerrors.CodeInvalidParam))
}

