// Package errors_test provides comprehensive table-driven unit tests for the
// error code definitions in pkg/errors/codes.go.
package errors_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test data — exhaustive table of every declared ErrorCode
// ─────────────────────────────────────────────────────────────────────────────

type codeEntry struct {
	code           errors.ErrorCode
	expectedString string
	expectedHTTP   int
}

// allCodes enumerates every ErrorCode constant defined in codes.go together
// with its expected String() output and expected HTTPStatus() mapping.
// The table is the single source of truth for both test functions below.
var allCodes = []codeEntry{
	// ── General ──────────────────────────────────────────────────────────────
	{errors.CodeOK, "OK", http.StatusOK},
	{errors.CodeUnknown, "UNKNOWN", http.StatusInternalServerError},
	{errors.CodeInvalidParam, "INVALID_PARAM", http.StatusBadRequest},
	{errors.CodeUnauthorized, "UNAUTHORIZED", http.StatusUnauthorized},
	{errors.CodeForbidden, "FORBIDDEN", http.StatusForbidden},
	{errors.CodeNotFound, "NOT_FOUND", http.StatusNotFound},
	{errors.CodeConflict, "CONFLICT", http.StatusConflict},
	{errors.CodeRateLimit, "RATE_LIMIT", http.StatusTooManyRequests},
	{errors.CodeInternal, "INTERNAL_ERROR", http.StatusInternalServerError},

	// ── Patent ────────────────────────────────────────────────────────────────
	{errors.CodePatentNotFound, "PATENT_NOT_FOUND", http.StatusNotFound},
	{errors.CodePatentDuplicate, "PATENT_DUPLICATE", http.StatusConflict},
	{errors.CodeClaimParseError, "CLAIM_PARSE_ERROR", http.StatusBadRequest},
	{errors.CodeMarkushInvalid, "MARKUSH_INVALID", http.StatusBadRequest},

	// ── Molecule ──────────────────────────────────────────────────────────────
	{errors.CodeMoleculeInvalidSMILES, "MOLECULE_INVALID_SMILES", http.StatusBadRequest},
	{errors.CodeMoleculeNotFound, "MOLECULE_NOT_FOUND", http.StatusNotFound},
	{errors.CodeFingerprintError, "FINGERPRINT_ERROR", http.StatusInternalServerError},
	{errors.CodeSimilarityCalcError, "SIMILARITY_CALC_ERROR", http.StatusInternalServerError},

	// ── Portfolio ─────────────────────────────────────────────────────────────
	{errors.CodePortfolioNotFound, "PORTFOLIO_NOT_FOUND", http.StatusNotFound},
	{errors.CodeValuationError, "VALUATION_ERROR", http.StatusInternalServerError},

	// ── Lifecycle ─────────────────────────────────────────────────────────────
	{errors.CodeDeadlineMissed, "DEADLINE_MISSED", http.StatusConflict},
	{errors.CodeAnnuityCalcError, "ANNUITY_CALC_ERROR", http.StatusInternalServerError},
	{errors.CodeJurisdictionUnknown, "JURISDICTION_UNKNOWN", http.StatusBadRequest},

	// ── Intelligence ──────────────────────────────────────────────────────────
	{errors.CodeModelLoadError, "MODEL_LOAD_ERROR", http.StatusInternalServerError},
	{errors.CodeInferenceTimeout, "INFERENCE_TIMEOUT", http.StatusGatewayTimeout},
	{errors.CodeModelNotReady, "MODEL_NOT_READY", http.StatusServiceUnavailable},

	// ── Infrastructure ────────────────────────────────────────────────────────
	{errors.CodeDBConnectionError, "DB_CONNECTION_ERROR", http.StatusServiceUnavailable},
	{errors.CodeCacheError, "CACHE_ERROR", http.StatusInternalServerError},
	{errors.CodeSearchError, "SEARCH_ERROR", http.StatusInternalServerError},
	{errors.CodeMessageQueueError, "MESSAGE_QUEUE_ERROR", http.StatusServiceUnavailable},
	{errors.CodeStorageError, "STORAGE_ERROR", http.StatusServiceUnavailable},
}

// ─────────────────────────────────────────────────────────────────────────────
// TestErrorCode_String
// ─────────────────────────────────────────────────────────────────────────────

// TestErrorCode_String verifies that every declared ErrorCode returns the
// expected non-empty string representation from its String() method.
func TestErrorCode_String(t *testing.T) {
	t.Parallel()

	for _, tc := range allCodes {
		tc := tc // capture range variable
		t.Run(tc.expectedString, func(t *testing.T) {
			t.Parallel()

			got := tc.code.String()

			// Must never be empty.
			assert.NotEmpty(t, got,
				"String() for code %d must not be empty", int(tc.code))

			// Must match the exact expected name.
			assert.Equal(t, tc.expectedString, got,
				"String() for code %d returned unexpected value", int(tc.code))
		})
	}
}

// TestErrorCode_String_Unknown verifies that an ErrorCode value that does not
// correspond to any declared constant returns the sentinel string "UNKNOWN_CODE".
func TestErrorCode_String_Unknown(t *testing.T) {
	t.Parallel()

	unknownCodes := []errors.ErrorCode{
		errors.ErrorCode(99999),
		errors.ErrorCode(-1),
		errors.ErrorCode(1),
		errors.ErrorCode(12345),
	}

	for _, code := range unknownCodes {
		code := code
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := code.String()
			assert.NotEmpty(t, got,
				"String() must never return an empty string even for unknown codes")
			assert.Equal(t, "UNKNOWN_CODE", got,
				"String() for undeclared code %d should return UNKNOWN_CODE", int(code))
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestErrorCode_HTTPStatus
// ─────────────────────────────────────────────────────────────────────────────

// TestErrorCode_HTTPStatus verifies that every declared ErrorCode returns the
// correct HTTP status code from its HTTPStatus() method.
func TestErrorCode_HTTPStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range allCodes {
		tc := tc
		t.Run(tc.expectedString, func(t *testing.T) {
			t.Parallel()

			got := tc.code.HTTPStatus()

			assert.Equal(t, tc.expectedHTTP, got,
				"HTTPStatus() for %s (code %d) returned %d, want %d",
				tc.expectedString, int(tc.code), got, tc.expectedHTTP)
		})
	}
}

// TestErrorCode_HTTPStatus_SpecificMappings provides explicit, named test cases
// for the most commonly referenced mappings so that failures produce maximally
// descriptive output.  These cases are a subset of allCodes but stated as
// first-class test scenarios per the implementation requirements.
func TestErrorCode_HTTPStatus_SpecificMappings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		code errors.ErrorCode
		want int
	}{
		{"NotFound→404", errors.CodeNotFound, http.StatusNotFound},
		{"Unauthorized→401", errors.CodeUnauthorized, http.StatusUnauthorized},
		{"InvalidParam→400", errors.CodeInvalidParam, http.StatusBadRequest},
		{"Internal→500", errors.CodeInternal, http.StatusInternalServerError},
		{"RateLimit→429", errors.CodeRateLimit, http.StatusTooManyRequests},
		{"PatentNotFound→404", errors.CodePatentNotFound, http.StatusNotFound},
		{"MoleculeInvalidSMILES→400", errors.CodeMoleculeInvalidSMILES, http.StatusBadRequest},
		{"InferenceTimeout→504", errors.CodeInferenceTimeout, http.StatusGatewayTimeout},
		{"ModelNotReady→503", errors.CodeModelNotReady, http.StatusServiceUnavailable},
		{"DBConnectionError→503", errors.CodeDBConnectionError, http.StatusServiceUnavailable},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.code.HTTPStatus(),
				"HTTPStatus() mismatch for %s", tc.name)
		})
	}
}

// TestErrorCode_HTTPStatus_Unknown verifies that any undeclared ErrorCode
// falls through to the default branch and returns 500 Internal Server Error.
func TestErrorCode_HTTPStatus_Unknown(t *testing.T) {
	t.Parallel()

	unknownCodes := []errors.ErrorCode{
		errors.ErrorCode(99999),
		errors.ErrorCode(-1),
		errors.ErrorCode(1),
	}

	for _, code := range unknownCodes {
		code := code
		t.Run("", func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, http.StatusInternalServerError, code.HTTPStatus(),
				"HTTPStatus() for undeclared code %d should default to 500", int(code))
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestErrorCode_AllCodesHaveValidHTTPStatus ensures that every code in the
// master table maps to a valid, well-known HTTP status code (i.e. one of the
// values defined in net/http).  This guards against typos such as returning
// 40 instead of 400.
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorCode_AllCodesHaveValidHTTPStatus(t *testing.T) {
	t.Parallel()

	// Accepted status codes used by the platform.
	validStatuses := map[int]bool{
		http.StatusOK:                  true,
		http.StatusBadRequest:          true,
		http.StatusUnauthorized:        true,
		http.StatusForbidden:           true,
		http.StatusNotFound:            true,
		http.StatusConflict:            true,
		http.StatusTooManyRequests:     true,
		http.StatusInternalServerError: true,
		http.StatusServiceUnavailable:  true,
		http.StatusGatewayTimeout:      true,
	}

	for _, tc := range allCodes {
		tc := tc
		t.Run(tc.expectedString, func(t *testing.T) {
			t.Parallel()
			status := tc.code.HTTPStatus()
			assert.True(t, validStatuses[status],
				"HTTPStatus() for %s returned unexpected status code %d",
				tc.expectedString, status)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestErrorCode_DomainRanges validates that each error code integer value falls
// within the expected numeric range for its business domain.  This prevents
// accidental cross-domain code collisions as the codebase grows.
// ─────────────────────────────────────────────────────────────────────────────
func TestErrorCode_DomainRanges(t *testing.T) {
	t.Parallel()

	type rangeEntry struct {
		code errors.ErrorCode
		low  int
		high int
		name string
	}

	ranges := []rangeEntry{
		// General
		{errors.CodeOK, 0, 0, "CodeOK"},
		{errors.CodeUnknown, 10000, 10999, "CodeUnknown"},
		{errors.CodeInvalidParam, 10000, 10999, "CodeInvalidParam"},
		{errors.CodeUnauthorized, 10000, 10999, "CodeUnauthorized"},
		{errors.CodeForbidden, 10000, 10999, "CodeForbidden"},
		{errors.CodeNotFound, 10000, 10999, "CodeNotFound"},
		{errors.CodeConflict, 10000, 10999, "CodeConflict"},
		{errors.CodeRateLimit, 10000, 10999, "CodeRateLimit"},
		{errors.CodeInternal, 10000, 10999, "CodeInternal"},
		// Patent
		{errors.CodePatentNotFound, 20000, 29999, "CodePatentNotFound"},
		{errors.CodePatentDuplicate, 20000, 29999, "CodePatentDuplicate"},
		{errors.CodeClaimParseError, 20000, 29999, "CodeClaimParseError"},
		{errors.CodeMarkushInvalid, 20000, 29999, "CodeMarkushInvalid"},
		// Molecule
		{errors.CodeMoleculeInvalidSMILES, 30000, 39999, "CodeMoleculeInvalidSMILES"},
		{errors.CodeMoleculeNotFound, 30000, 39999, "CodeMoleculeNotFound"},
		{errors.CodeFingerprintError, 30000, 39999, "CodeFingerprintError"},
		{errors.CodeSimilarityCalcError, 30000, 39999, "CodeSimilarityCalcError"},
		// Portfolio
		{errors.CodePortfolioNotFound, 40000, 49999, "CodePortfolioNotFound"},
		{errors.CodeValuationError, 40000, 49999, "CodeValuationError"},
		// Lifecycle
		{errors.CodeDeadlineMissed, 50000, 59999, "CodeDeadlineMissed"},
		{errors.CodeAnnuityCalcError, 50000, 59999, "CodeAnnuityCalcError"},
		{errors.CodeJurisdictionUnknown, 50000, 59999, "CodeJurisdictionUnknown"},
		// Intelligence
		{errors.CodeModelLoadError, 60000, 69999, "CodeModelLoadError"},
		{errors.CodeInferenceTimeout, 60000, 69999, "CodeInferenceTimeout"},
		{errors.CodeModelNotReady, 60000, 69999, "CodeModelNotReady"},
		// Infrastructure
		{errors.CodeDBConnectionError, 70000, 79999, "CodeDBConnectionError"},
		{errors.CodeCacheError, 70000, 79999, "CodeCacheError"},
		{errors.CodeSearchError, 70000, 79999, "CodeSearchError"},
		{errors.CodeMessageQueueError, 70000, 79999, "CodeMessageQueueError"},
		{errors.CodeStorageError, 70000, 79999, "CodeStorageError"},
	}

	for _, r := range ranges {
		r := r
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()
			v := int(r.code)
			assert.GreaterOrEqual(t, v, r.low,
				"%s value %d is below domain lower bound %d", r.name, v, r.low)
			assert.LessOrEqual(t, v, r.high,
				"%s value %d is above domain upper bound %d", r.name, v, r.high)
		})
	}
}

//Personal.AI order the ending
