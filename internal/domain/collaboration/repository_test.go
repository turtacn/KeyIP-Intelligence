package collaboration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyCollabOptions_Defaults(t *testing.T) {
	opts := ApplyCollabOptions()
	assert.Equal(t, 0, opts.Offset)
	assert.Equal(t, 20, opts.Limit)
	assert.False(t, opts.ActiveOnly)
}

func TestApplyCollabOptions_WithPagination(t *testing.T) {
	opts := ApplyCollabOptions(WithCollabPagination(10, 50))
	assert.Equal(t, 10, opts.Offset)
	assert.Equal(t, 50, opts.Limit)
}

func TestApplyCollabOptions_LimitCap(t *testing.T) {
	opts := ApplyCollabOptions(WithCollabPagination(10, 200))
	assert.Equal(t, 10, opts.Offset)
	assert.Equal(t, 100, opts.Limit)
}

func TestApplyCollabOptions_WithActiveOnly(t *testing.T) {
	opts := ApplyCollabOptions(WithActiveOnly())
	assert.True(t, opts.ActiveOnly)
}

func TestApplyCollabOptions_WithAcceptedOnly(t *testing.T) {
	opts := ApplyCollabOptions(WithAcceptedOnly())
	assert.True(t, opts.AcceptedOnly)
}

func TestApplyCollabOptions_WithRoleFilter(t *testing.T) {
	opts := ApplyCollabOptions(WithRoleFilter(RoleAdmin))
	assert.Equal(t, RoleAdmin, opts.RoleFilter)
}

func TestApplyCollabOptions_Combined(t *testing.T) {
	opts := ApplyCollabOptions(
		WithCollabPagination(5, 10),
		WithActiveOnly(),
		WithRoleFilter(RoleViewer),
	)
	assert.Equal(t, 5, opts.Offset)
	assert.Equal(t, 10, opts.Limit)
	assert.True(t, opts.ActiveOnly)
	assert.Equal(t, RoleViewer, opts.RoleFilter)
}

//Personal.AI order the ending
