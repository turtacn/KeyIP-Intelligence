package lifecycle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestApplyLifecycleOptions_Defaults(t *testing.T) {
	opts := ApplyLifecycleOptions()
	assert.Equal(t, 0, opts.Offset)
	assert.Equal(t, 20, opts.Limit)
}

func TestApplyLifecycleOptions_WithPagination(t *testing.T) {
	opts := ApplyLifecycleOptions(WithLifecyclePagination(10, 50))
	assert.Equal(t, 10, opts.Offset)
	assert.Equal(t, 50, opts.Limit)
}

func TestApplyLifecycleOptions_LimitCap(t *testing.T) {
	opts := ApplyLifecycleOptions(WithLifecyclePagination(10, 200))
	assert.Equal(t, 10, opts.Offset)
	assert.Equal(t, 100, opts.Limit)
}

func TestApplyLifecycleOptions_NegativeOffset(t *testing.T) {
	opts := ApplyLifecycleOptions(WithLifecyclePagination(-10, 50))
	assert.Equal(t, 0, opts.Offset)
	assert.Equal(t, 50, opts.Limit)
}

func TestApplyLifecycleOptions_WithSortBy(t *testing.T) {
	opts := ApplyLifecycleOptions(WithLifecycleSortBy("created_at", false))
	assert.Equal(t, "created_at", opts.SortField)
	assert.False(t, opts.SortAscending)
}

func TestApplyLifecycleOptions_WithOwnerFilter(t *testing.T) {
	opts := ApplyLifecycleOptions(WithOwnerFilter("user1"))
	assert.Equal(t, "user1", opts.OwnerID)
}

func TestApplyLifecycleOptions_WithDateRange(t *testing.T) {
	from := time.Now()
	to := from.AddDate(0, 1, 0)
	opts := ApplyLifecycleOptions(WithDateRange(from, to))
	assert.Equal(t, from, opts.FromDate)
	assert.Equal(t, to, opts.ToDate)
}

func TestApplyLifecycleOptions_Combined(t *testing.T) {
	opts := ApplyLifecycleOptions(
		WithLifecyclePagination(10, 50),
		WithLifecycleSortBy("title", true),
		WithOwnerFilter("user1"),
	)
	assert.Equal(t, 10, opts.Offset)
	assert.Equal(t, 50, opts.Limit)
	assert.Equal(t, "title", opts.SortField)
	assert.True(t, opts.SortAscending)
	assert.Equal(t, "user1", opts.OwnerID)
}

//Personal.AI order the ending
