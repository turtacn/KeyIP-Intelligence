package portfolio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyOptions_Defaults(t *testing.T) {
	opts := ApplyOptions()
	assert.Equal(t, 0, opts.Offset)
	assert.Equal(t, 20, opts.Limit)
	assert.Empty(t, opts.TagFilters)
}

func TestApplyOptions_WithPagination(t *testing.T) {
	opts := ApplyOptions(WithPagination(10, 50))
	assert.Equal(t, 10, opts.Offset)
	assert.Equal(t, 50, opts.Limit)
}

func TestApplyOptions_LimitCap(t *testing.T) {
	opts := ApplyOptions(WithPagination(10, 200))
	assert.Equal(t, 10, opts.Offset)
	assert.Equal(t, 100, opts.Limit)
}

func TestApplyOptions_NegativeOffset(t *testing.T) {
	opts := ApplyOptions(WithPagination(-10, 50))
	assert.Equal(t, 0, opts.Offset)
	assert.Equal(t, 50, opts.Limit)
}

func TestApplyOptions_WithSortBy(t *testing.T) {
	opts := ApplyOptions(WithSortBy("name", true))
	assert.Equal(t, "name", opts.SortField)
	assert.True(t, opts.SortAscending)
}

func TestApplyOptions_WithNameFilter(t *testing.T) {
	opts := ApplyOptions(WithNameFilter("test"))
	assert.Equal(t, "test", opts.NameKeyword)
}

func TestApplyOptions_WithTagFilter(t *testing.T) {
	opts := ApplyOptions(
		WithTagFilter("key1", "value1"),
		WithTagFilter("key2", "value2"),
	)
	assert.Equal(t, 2, len(opts.TagFilters))
	assert.Equal(t, "value1", opts.TagFilters["key1"])
	assert.Equal(t, "value2", opts.TagFilters["key2"])
}

func TestApplyOptions_Combined(t *testing.T) {
	opts := ApplyOptions(
		WithPagination(10, 50),
		WithSortBy("created_at", false),
		WithNameFilter("project"),
		WithTagFilter("status", "active"),
	)
	assert.Equal(t, 10, opts.Offset)
	assert.Equal(t, 50, opts.Limit)
	assert.Equal(t, "created_at", opts.SortField)
	assert.False(t, opts.SortAscending)
	assert.Equal(t, "project", opts.NameKeyword)
	assert.Equal(t, "active", opts.TagFilters["status"])
}

//Personal.AI order the ending
