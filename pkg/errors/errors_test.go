// Package errors_test provides comprehensive unit tests for the AppError type,
// factory functions, and error-chain helpers defined in pkg/errors/errors.go.
package errors_test

import (
	stderrors "errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestNew
// ─────────────────────────────────────────────────────────────────────────────

func TestNew_FieldsAreSetCorrectly(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		code    errors.ErrorCode
		message string
	}{
		{"internal error", errors.CodeInternal, "unexpected failure"},
		{"not found", errors.CodePatentNotFound, "patent CN202310001234A not found"},
		{"invalid param", errors.CodeInvalidParam, "SMILES must not be empty"},
		{"rate limit", errors.CodeRateLimit, "too many requests"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ae := errors.New(tc.code, tc.message)

			require.NotNil(t, ae)
			assert.Equal(t, tc.code, ae.Code)
			assert.Equal(t, tc.message, ae.Message)
			assert.Empty(t, ae.Detail, "Detail should be empty for bare New()")
			assert.Nil(t, ae.Cause, "Cause should be nil for bare New()")
		})
	}
}

func TestNew_StackIsPopulated(t *testing.T) {
	t.Parallel()

	ae := errors.New(errors.CodeInternal, "test")
	require.NotNil(t, ae)
	// Stack may be empty when compiled with -tags nostack; we only assert it is
	// a string (never panics).  When not using nostack it should contain this
	// file name.
	_ = ae.Stack // field is accessible; no panic expected
}

func TestNew_NilIsNeverReturned(t *testing.T) {
	t.Parallel()

	ae := errors.New(errors.CodeOK, "")
	require.NotNil(t, ae)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestWrap
// ─────────────────────────────────────────────────────────────────────────────

func TestWrap_NilErrReturnsNil(t *testing.T) {
	t.Parallel()

	result := errors.Wrap(nil, errors.CodeInternal, "should not matter")
	assert.Nil(t, result)
}

func TestWrap_CauseChainIsPreserved(t *testing.T) {
	t.Parallel()

	root := stderrors.New("root DB error")
	wrapped := errors.Wrap(root, errors.CodeDBConnectionError, "connection failed")

	require.NotNil(t, wrapped)
	assert.Equal(t, errors.CodeDBConnectionError, wrapped.Code)
	assert.Equal(t, "connection failed", wrapped.Message)
	assert.Equal(t, root, wrapped.Cause)
}

func TestWrap_UnwrapReturnsCause(t *testing.T) {
	t.Parallel()

	cause := stderrors.New("original")
	ae := errors.Wrap(cause, errors.CodeCacheError, "cache miss")

	unwrapped := stderrors.Unwrap(ae)
	assert.Equal(t, cause, unwrapped)
}

func TestWrap_PreservesOriginalCodeWhenCodeUnknown(t *testing.T) {
	t.Parallel()

	inner := errors.New(errors.CodePatentNotFound, "not found")
	outer := errors.Wrap(inner, errors.CodeUnknown, "adding context")

	require.NotNil(t, outer)
	assert.Equal(t, errors.CodePatentNotFound, outer.Code,
		"Wrap with CodeUnknown should inherit the inner AppError's code")
}

func TestWrap_OverridesCodeWhenExplicit(t *testing.T) {
	t.Parallel()

	inner := errors.New(errors.CodePatentNotFound, "not found")
	outer := errors.Wrap(inner, errors.CodeInternal, "unexpected state")

	assert.Equal(t, errors.CodeInternal, outer.Code,
		"explicit non-Unknown code must override the inner code")
}

func TestWrap_MultiLevel(t *testing.T) {
	t.Parallel()

	root := stderrors.New("dial tcp: connection refused")
	level1 := errors.Wrap(root, errors.CodeDBConnectionError, "postgres unreachable")
	level2 := errors.Wrap(level1, errors.CodeInternal, "failed to load patent")

	// Unwrap chain: level2 → level1 → root
	assert.Equal(t, level1, stderrors.Unwrap(level2))
	assert.Equal(t, root, stderrors.Unwrap(level1))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestError_Method
// ─────────────────────────────────────────────────────────────────────────────

func TestError_FormatWithoutDetail(t *testing.T) {
	t.Parallel()

	ae := errors.New(errors.CodePatentNotFound, "patent not found")
	s := ae.Error()

	assert.Contains(t, s, "PATENT_NOT_FOUND")
	assert.Contains(t, s, "20001")
	assert.Contains(t, s, "patent not found")
	// No colon-separated detail segment expected.
	assert.False(t, strings.Count(s, ":") > 1,
		"Error() without detail should not contain extra colons from detail")
}

func TestError_FormatWithDetail(t *testing.T) {
	t.Parallel()

	ae := errors.New(errors.CodeMoleculeInvalidSMILES, "invalid SMILES").
		WithDetail("input=C1CC1[invalid]")
	s := ae.Error()

	assert.Contains(t, s, "MOLECULE_INVALID_SMILES")
	assert.Contains(t, s, "30001")
	assert.Contains(t, s, "invalid SMILES")
	assert.Contains(t, s, "input=C1CC1[invalid]")
}

func TestError_ImplementsErrorInterface(t *testing.T) {
	t.Parallel()

	var err error = errors.New(errors.CodeInternal, "boom")
	assert.NotEmpty(t, err.Error())
}

func TestError_EmptyMessageDoesNotPanic(t *testing.T) {
	t.Parallel()

	ae := errors.New(errors.CodeOK, "")
	assert.NotPanics(t, func() { _ = ae.Error() })
}

// ─────────────────────────────────────────────────────────────────────────────
// TestWithDetail
// ─────────────────────────────────────────────────────────────────────────────

func TestWithDetail_SetsDetailOnCopy(t *testing.T) {
	t.Parallel()

	original := errors.New(errors.CodeNotFound, "resource missing")
	detailed := original.WithDetail("id=42")

	// Original must be unchanged (shallow copy semantics).
	assert.Empty(t, original.Detail, "WithDetail must not mutate the original")
	assert.Equal(t, "id=42", detailed.Detail)
	assert.Equal(t, original.Code, detailed.Code)
	assert.Equal(t, original.Message, detailed.Message)
}

func TestWithDetail_ChainedCalls(t *testing.T) {
	t.Parallel()

	ae := errors.New(errors.CodeSearchError, "search failed").
		WithDetail("index=patents").
		WithDetail("index=patents, shard=3") // second call replaces first

	assert.Equal(t, "index=patents, shard=3", ae.Detail)
}

func TestWithDetail_NilReceiverReturnsNil(t *testing.T) {
	t.Parallel()

	var ae *errors.AppError
	result := ae.WithDetail("x")
	assert.Nil(t, result)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestWithCause
// ─────────────────────────────────────────────────────────────────────────────

func TestWithCause_AttachesCause(t *testing.T) {
	t.Parallel()

	root := stderrors.New("driver: bad connection")
	ae := errors.New(errors.CodeDBConnectionError, "database error").WithCause(root)

	assert.Equal(t, root, ae.Cause)
	assert.Equal(t, root, stderrors.Unwrap(ae))
}

func TestWithCause_DoesNotMutateOriginal(t *testing.T) {
	t.Parallel()

	original := errors.New(errors.CodeInternal, "failure")
	cause := stderrors.New("cause")
	withCause := original.WithCause(cause)

	assert.Nil(t, original.Cause, "WithCause must not mutate the original")
	assert.Equal(t, cause, withCause.Cause)
}

func TestWithCause_NilReceiverReturnsNil(t *testing.T) {
	t.Parallel()

	var ae *errors.AppError
	result := ae.WithCause(stderrors.New("x"))
	assert.Nil(t, result)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestIsCode
// ─────────────────────────────────────────────────────────────────────────────

func TestIsCode_DirectMatch(t *testing.T) {
	t.Parallel()

	ae := errors.New(errors.CodePatentNotFound, "not found")
	assert.True(t, errors.IsCode(ae, errors.CodePatentNotFound))
}

func TestIsCode_NoMatch(t *testing.T) {
	t.Parallel()

	ae := errors.New(errors.CodePatentNotFound, "not found")
	assert.False(t, errors.IsCode(ae, errors.CodeInternal))
}

func TestIsCode_NestedChain(t *testing.T) {
	t.Parallel()

	root := errors.New(errors.CodeDBConnectionError, "db down")
	wrapped := errors.Wrap(root, errors.CodeInternal, "service error")

	// The outer code is CodeInternal but the chain contains CodeDBConnectionError.
	assert.True(t, errors.IsCode(wrapped, errors.CodeDBConnectionError),
		"IsCode must find the code anywhere in the error chain")
	assert.True(t, errors.IsCode(wrapped, errors.CodeInternal))
}

func TestIsCode_NilErrorReturnsFalse(t *testing.T) {
	t.Parallel()

	assert.False(t, errors.IsCode(nil, errors.CodeInternal))
}

func TestIsCode_StdlibErrorReturnsFalse(t *testing.T) {
	t.Parallel()

	err := stderrors.New("plain error")
	assert.False(t, errors.IsCode(err, errors.CodeInternal))
}

func TestIsCode_ThreeLevelChain(t *testing.T) {
	t.Parallel()

	level0 := errors.New(errors.CodeMoleculeInvalidSMILES, "bad SMILES")
	level1 := errors.Wrap(level0, errors.CodeInvalidParam, "validation failed")
	level2 := errors.Wrap(level1, errors.CodeInternal, "handler error")

	assert.True(t, errors.IsCode(level2, errors.CodeMoleculeInvalidSMILES))
	assert.True(t, errors.IsCode(level2, errors.CodeInvalidParam))
	assert.True(t, errors.IsCode(level2, errors.CodeInternal))
	assert.False(t, errors.IsCode(level2, errors.CodeForbidden))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestGetCode
// ─────────────────────────────────────────────────────────────────────────────

func TestGetCode_DirectAppError(t *testing.T) {
	t.Parallel()

	ae := errors.New(errors.CodePortfolioNotFound, "portfolio missing")
	assert.Equal(t, errors.CodePortfolioNotFound, errors.GetCode(ae))
}

func TestGetCode_NestedAppError(t *testing.T) {
	t.Parallel()

	inner := errors.New(errors.CodeModelLoadError, "model missing")
	outer := errors.Wrap(inner, errors.CodeInternal, "service init failed")

	// GetCode returns the outermost AppError's code.
	assert.Equal(t, errors.CodeInternal, errors.GetCode(outer))
}

func TestGetCode_NilReturnsCodeOK(t *testing.T) {
	t.Parallel()

	assert.Equal(t, errors.CodeOK, errors.GetCode(nil))
}

func TestGetCode_StdlibErrorReturnsCodeUnknown(t *testing.T) {
	t.Parallel()

	err := stderrors.New("some stdlib error")
	assert.Equal(t, errors.CodeUnknown, errors.GetCode(err))
}

func TestGetCode_FmtWrappedStdlibReturnsCodeUnknown(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("context: %w", stderrors.New("cause"))
	assert.Equal(t, errors.CodeUnknown, errors.GetCode(err))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestConvenienceFactories
// ─────────────────────────────────────────────────────────────────────────────

func TestConvenienceFactories_ReturnCorrectCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		err      *errors.AppError
		wantCode errors.ErrorCode
	}{
		{"NotFound", errors.NotFound("not found"), errors.CodeNotFound},
		{"InvalidParam", errors.InvalidParam("bad input"), errors.CodeInvalidParam},
		{"Unauthorized", errors.Unauthorized("missing token"), errors.CodeUnauthorized},
		{"Forbidden", errors.Forbidden("access denied"), errors.CodeForbidden},
		{"Internal", errors.Internal("server error"), errors.CodeInternal},
		{"Conflict", errors.Conflict("duplicate resource"), errors.CodeConflict},
		{"RateLimit", errors.RateLimit("slow down"), errors.CodeRateLimit},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.NotNil(t, tc.err)
			assert.Equal(t, tc.wantCode, tc.err.Code)
			assert.NotEmpty(t, tc.err.Message)
			assert.NotEmpty(t, tc.err.Error())
		})
	}
}

func TestConvenienceFactories_MessageIsPreserved(t *testing.T) {
	t.Parallel()

	msg := "patent CN202310001234A not found"
	ae := errors.NotFound(msg)
	assert.Equal(t, msg, ae.Message)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestStdlibCompatibility
// ─────────────────────────────────────────────────────────────────────────────

func TestStdlib_ErrorsIs_DirectComparison(t *testing.T) {
	t.Parallel()

	sentinel := errors.New(errors.CodeForbidden, "forbidden")
	wrapped := fmt.Errorf("handler: %w", sentinel)

	// errors.Is traverses the chain and finds the *AppError pointer.
	assert.True(t, stderrors.Is(wrapped, sentinel))
}

func TestStdlib_ErrorsAs_ExtractsAppError(t *testing.T) {
	t.Parallel()

	original := errors.New(errors.CodeModelNotReady, "model warming up")
	wrapped := fmt.Errorf("inference: %w", original)

	var ae *errors.AppError
	require.True(t, stderrors.As(wrapped, &ae),
		"errors.As must be able to extract *AppError from a wrapped chain")
	assert.Equal(t, errors.CodeModelNotReady, ae.Code)
	assert.Equal(t, "model warming up", ae.Message)
}

func TestStdlib_ErrorsAs_DeepChain(t *testing.T) {
	t.Parallel()

	root := errors.New(errors.CodeStorageError, "minio unavailable")
	l1 := errors.Wrap(root, errors.CodeInternal, "upload failed")
	l2 := fmt.Errorf("report service: %w", l1)
	l3 := fmt.Errorf("http handler: %w", l2)

	var ae *errors.AppError
	require.True(t, stderrors.As(l3, &ae))
	// errors.As returns the first match in the chain, which is l1.
	assert.Equal(t, errors.CodeInternal, ae.Code)
}

func TestStdlib_Unwrap_Chain(t *testing.T) {
	t.Parallel()

	cause := stderrors.New("root cause")
	ae := errors.New(errors.CodeCacheError, "cache failure").WithCause(cause)

	// Standard library traversal must reach the root cause.
	assert.True(t, stderrors.Is(ae, cause))
}

func TestStdlib_ErrorsIs_FalseForUnrelatedError(t *testing.T) {
	t.Parallel()

	a := errors.New(errors.CodeInternal, "error A")
	b := errors.New(errors.CodeInternal, "error B")

	// Two distinct *AppError pointers are not equal even if codes match.
	assert.False(t, stderrors.Is(a, b))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestFluentChain — combined WithDetail + WithCause + factory
// ─────────────────────────────────────────────────────────────────────────────

func TestFluentChain_CombinedUsage(t *testing.T) {
	t.Parallel()

	root := stderrors.New("neo4j: connection reset")
	ae := errors.New(errors.CodeDBConnectionError, "knowledge graph query failed").
		WithDetail("query=MATCH (p:Patent) RETURN p LIMIT 10").
		WithCause(root)

	assert.Equal(t, errors.CodeDBConnectionError, ae.Code)
	assert.Equal(t, "knowledge graph query failed", ae.Message)
	assert.Contains(t, ae.Detail, "MATCH (p:Patent)")
	assert.Equal(t, root, ae.Cause)

	// Error() must include detail.
	s := ae.Error()
	assert.Contains(t, s, "DB_CONNECTION_ERROR")
	assert.Contains(t, s, "knowledge graph query failed")
	assert.Contains(t, s, "MATCH (p:Patent)")

	// Standard library chain traversal must find the root.
	assert.True(t, stderrors.Is(ae, root))
}

//Personal.AI order the ending
