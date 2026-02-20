// Package common_test provides unit tests for the foundational types defined
// in pkg/types/common/types.go.
package common_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestNewID
// ─────────────────────────────────────────────────────────────────────────────

func TestNewID_ReturnsValidUUIDFormat(t *testing.T) {
	t.Parallel()

	id := common.NewID()
	s := string(id)

	// UUID v4 canonical form: 8-4-4-4-12 hex groups separated by hyphens.
	parts := strings.Split(s, "-")
	require.Len(t, parts, 5, "UUID must have 5 hyphen-separated groups, got: %s", s)
	assert.Len(t, parts[0], 8)
	assert.Len(t, parts[1], 4)
	assert.Len(t, parts[2], 4)
	assert.Len(t, parts[3], 4)
	assert.Len(t, parts[4], 12)
}

func TestNewID_IsUnique(t *testing.T) {
	t.Parallel()

	const n = 1000
	seen := make(map[common.ID]struct{}, n)
	for i := 0; i < n; i++ {
		id := common.NewID()
		_, dup := seen[id]
		assert.False(t, dup, "NewID() generated a duplicate: %s", id)
		seen[id] = struct{}{}
	}
}

func TestNewID_TotalLength(t *testing.T) {
	t.Parallel()

	id := common.NewID()
	// Standard UUID string length is 36 characters (32 hex + 4 hyphens).
	assert.Len(t, string(id), 36)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestPageRequest_Validate
// ─────────────────────────────────────────────────────────────────────────────

func TestPageRequest_Validate_ValidParams(t *testing.T) {
	t.Parallel()

	cases := []common.PageRequest{
		{Page: 1, PageSize: 1},
		{Page: 1, PageSize: 20},
		{Page: 100, PageSize: 1000},
		{Page: 1, PageSize: 50, SortBy: "created_at", SortOrder: "asc"},
		{Page: 1, PageSize: 50, SortBy: "name", SortOrder: "desc"},
		{Page: 1, PageSize: 50, SortOrder: ""}, // empty SortOrder is allowed
		{Page: 1, PageSize: 50, SortBy: ""},    // empty SortBy is allowed
	}

	for _, req := range cases {
		req := req
		t.Run("", func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, req.Validate())
		})
	}
}

func TestPageRequest_Validate_PageLessThanOne(t *testing.T) {
	t.Parallel()

	cases := []int{0, -1, -100}
	for _, p := range cases {
		p := p
		t.Run("", func(t *testing.T) {
			t.Parallel()
			req := common.PageRequest{Page: p, PageSize: 20}
			err := req.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "page")
		})
	}
}

func TestPageRequest_Validate_PageSizeLessThanOne(t *testing.T) {
	t.Parallel()

	cases := []int{0, -1, -50}
	for _, ps := range cases {
		ps := ps
		t.Run("", func(t *testing.T) {
			t.Parallel()
			req := common.PageRequest{Page: 1, PageSize: ps}
			err := req.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "page_size")
		})
	}
}

func TestPageRequest_Validate_PageSizeExceedsMax(t *testing.T) {
	t.Parallel()

	cases := []int{1001, 5000, 100000}
	for _, ps := range cases {
		ps := ps
		t.Run("", func(t *testing.T) {
			t.Parallel()
			req := common.PageRequest{Page: 1, PageSize: ps}
			err := req.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "page_size")
		})
	}
}

func TestPageRequest_Validate_InvalidSortOrder(t *testing.T) {
	t.Parallel()

	cases := []string{"ASC", "DESC", "ascending", "1", "up", "random"}
	for _, so := range cases {
		so := so
		t.Run(so, func(t *testing.T) {
			t.Parallel()
			req := common.PageRequest{Page: 1, PageSize: 20, SortOrder: so}
			err := req.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "sort_order")
		})
	}
}

func TestPageRequest_Validate_ValidSortOrders(t *testing.T) {
	t.Parallel()

	for _, so := range []string{"asc", "desc", ""} {
		so := so
		t.Run(so, func(t *testing.T) {
			t.Parallel()
			req := common.PageRequest{Page: 1, PageSize: 20, SortOrder: so}
			assert.NoError(t, req.Validate())
		})
	}
}

func TestPageRequest_Offset(t *testing.T) {
	t.Parallel()

	cases := []struct {
		page     int
		pageSize int
		want     int
	}{
		{1, 20, 0},
		{2, 20, 20},
		{3, 20, 40},
		{1, 50, 0},
		{5, 10, 40},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			req := common.PageRequest{Page: tc.page, PageSize: tc.pageSize}
			assert.Equal(t, tc.want, req.Offset())
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestPageResponse — generic instantiation
// ─────────────────────────────────────────────────────────────────────────────

func TestPageResponse_WithStringItems(t *testing.T) {
	t.Parallel()

	items := []string{"alpha", "beta", "gamma"}
	req := common.PageRequest{Page: 1, PageSize: 10}
	resp := common.NewPageResponse(items, 3, req)

	assert.Equal(t, items, resp.Items)
	assert.Equal(t, int64(3), resp.Total)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 10, resp.PageSize)
	assert.Equal(t, 1, resp.TotalPages)
}

func TestPageResponse_TotalPagesCalculation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		total     int64
		pageSize  int
		wantPages int
	}{
		{0, 10, 0},
		{1, 10, 1},
		{10, 10, 1},
		{11, 10, 2},
		{100, 10, 10},
		{101, 10, 11},
		{999, 100, 10},
		{1000, 100, 10},
		{1001, 100, 11},
	}

	type item struct{ V int }
	for _, tc := range cases {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			req := common.PageRequest{Page: 1, PageSize: tc.pageSize}
			resp := common.NewPageResponse([]item{}, tc.total, req)
			assert.Equal(t, tc.wantPages, resp.TotalPages,
				"total=%d pageSize=%d", tc.total, tc.pageSize)
		})
	}
}

func TestPageResponse_WithCustomStruct(t *testing.T) {
	t.Parallel()

	type Record struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	records := []Record{
		{ID: "1", Name: "alpha"},
		{ID: "2", Name: "beta"},
	}
	req := common.PageRequest{Page: 2, PageSize: 2}
	resp := common.NewPageResponse(records, 10, req)

	assert.Len(t, resp.Items, 2)
	assert.Equal(t, int64(10), resp.Total)
	assert.Equal(t, 2, resp.Page)
	assert.Equal(t, 2, resp.PageSize)
	assert.Equal(t, 5, resp.TotalPages)
}

func TestPageResponse_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	type Widget struct {
		ID    string `json:"id"`
		Color string `json:"color"`
	}

	original := common.NewPageResponse(
		[]Widget{{ID: "w1", Color: "red"}, {ID: "w2", Color: "blue"}},
		42,
		common.PageRequest{Page: 3, PageSize: 10},
	)

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded common.PageResponse[Widget]
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, int64(42), decoded.Total)
	assert.Equal(t, 3, decoded.Page)
	assert.Equal(t, 10, decoded.PageSize)
	assert.Equal(t, 5, decoded.TotalPages)
	require.Len(t, decoded.Items, 2)
	assert.Equal(t, "red", decoded.Items[0].Color)
	assert.Equal(t, "blue", decoded.Items[1].Color)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestStatus — constant values
// ─────────────────────────────────────────────────────────────────────────────

func TestStatus_ConstantValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		val  common.Status
		want string
	}{
		{common.StatusActive, "active"},
		{common.StatusInactive, "inactive"},
		{common.StatusPending, "pending"},
		{common.StatusArchived, "archived"},
		{common.StatusDeleted, "deleted"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, common.Status(tc.want), tc.val)
		})
	}
}

func TestStatus_Distinct(t *testing.T) {
	t.Parallel()

	all := []common.Status{
		common.StatusActive,
		common.StatusInactive,
		common.StatusPending,
		common.StatusArchived,
		common.StatusDeleted,
	}
	seen := make(map[common.Status]bool)
	for _, s := range all {
		assert.False(t, seen[s], "duplicate Status value: %s", s)
		seen[s] = true
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestBaseEntity — struct field accessibility and JSON tags
// ─────────────────────────────────────────────────────────────────────────────

func TestBaseEntity_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	import_time := "2025-06-01T12:00:00Z"
	_ = import_time

	original := common.BaseEntity{
		ID:        common.NewID(),
		TenantID:  common.TenantID("tenant-xyz"),
		CreatedBy: common.UserID("user-abc"),
		Version:   3,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded common.BaseEntity
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original.ID, decoded.ID)
	assert.Equal(t, original.TenantID, decoded.TenantID)
	assert.Equal(t, original.CreatedBy, decoded.CreatedBy)
	assert.Equal(t, original.Version, decoded.Version)
}

func TestBaseEntity_VersionOptimisticLock(t *testing.T) {
	t.Parallel()

	e := common.BaseEntity{Version: 0}
	assert.Equal(t, 0, e.Version)

	e.Version++
	assert.Equal(t, 1, e.Version)
}

//Personal.AI order the ending
