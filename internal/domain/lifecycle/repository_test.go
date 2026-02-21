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
	opts := ApplyLifecycleOptions(WithLifecyclePagination(0, 500))
	assert.Equal(t, 100, opts.Limit)
}

func TestApplyLifecycleOptions_NegativeOffset(t *testing.T) {
	opts := ApplyLifecycleOptions(WithLifecyclePagination(-5, 20))
	assert.Equal(t, 0, opts.Offset)
}

func TestApplyLifecycleOptions_WithSortBy(t *testing.T) {
	opts := ApplyLifecycleOptions(WithLifecycleSortBy("due_date", false))
	assert.Equal(t, "due_date", opts.SortField)
	assert.False(t, opts.SortAscending)
}

func TestApplyLifecycleOptions_WithOwnerFilter(t *testing.T) {
	opts := ApplyLifecycleOptions(WithOwnerFilter("user123"))
	assert.Equal(t, "user123", opts.OwnerID)
}

func TestApplyLifecycleOptions_WithDateRange(t *testing.T) {
	from := time.Now()
	to := from.AddDate(0, 1, 0)
	opts := ApplyLifecycleOptions(WithDateRange(from, to))
	assert.Equal(t, from, opts.FromDate)
	assert.Equal(t, to, opts.ToDate)
}

func TestApplyLifecycleOptions_Combined(t *testing.T) {
	from := time.Now()
	to := from.AddDate(0, 1, 0)
	opts := ApplyLifecycleOptions(
		WithLifecyclePagination(20, 30),
		WithLifecycleSortBy("patent_id", true),
		WithDateRange(from, to),
	)
	assert.Equal(t, 20, opts.Offset)
	assert.Equal(t, 30, opts.Limit)
	assert.Equal(t, "patent_id", opts.SortField)
	assert.True(t, opts.SortAscending)
	assert.Equal(t, from, opts.FromDate)
	assert.Equal(t, to, opts.ToDate)
}

//Personal.AI order the ending
