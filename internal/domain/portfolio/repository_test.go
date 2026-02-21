package portfolio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyOptions_Defaults(t *testing.T) {
	opts := ApplyOptions()
	assert.Equal(t, 0, opts.Offset)
	assert.Equal(t, 20, opts.Limit)
}

func TestApplyOptions_WithPagination(t *testing.T) {
	opts := ApplyOptions(WithPagination(10, 50))
	assert.Equal(t, 10, opts.Offset)
	assert.Equal(t, 50, opts.Limit)
}

func TestApplyOptions_LimitCap(t *testing.T) {
	opts := ApplyOptions(WithPagination(0, 500))
	assert.Equal(t, 100, opts.Limit)
}

func TestApplyOptions_NegativeOffset(t *testing.T) {
	opts := ApplyOptions(WithPagination(-5, 20))
	assert.Equal(t, 0, opts.Offset)
}

func TestApplyOptions_WithSortBy(t *testing.T) {
	opts := ApplyOptions(WithSortBy("name", false))
	assert.Equal(t, "name", opts.SortField)
	assert.False(t, opts.SortAscending)
}

func TestApplyOptions_WithNameFilter(t *testing.T) {
	opts := ApplyOptions(WithNameFilter("blue"))
	assert.Equal(t, "blue", opts.NameKeyword)
}

func TestApplyOptions_WithTagFilter(t *testing.T) {
	opts := ApplyOptions(WithTagFilter("priority", "high"), WithTagFilter("status", "active"))
	assert.Equal(t, "high", opts.TagFilters["priority"])
	assert.Equal(t, "active", opts.TagFilters["status"])
	assert.Len(t, opts.TagFilters, 2)
}

func TestApplyOptions_Combined(t *testing.T) {
	opts := ApplyOptions(
		WithPagination(20, 30),
		WithSortBy("created_at", true),
		WithNameFilter("oled"),
		WithTagFilter("type", "material"),
	)

	assert.Equal(t, 20, opts.Offset)
	assert.Equal(t, 30, opts.Limit)
	assert.Equal(t, "created_at", opts.SortField)
	assert.True(t, opts.SortAscending)
	assert.Equal(t, "oled", opts.NameKeyword)
	assert.Equal(t, "material", opts.TagFilters["type"])
}

//Personal.AI order the ending
