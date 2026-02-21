package errors

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCode_String(t *testing.T) {
	assert.Equal(t, "COMMON_001", ErrCodeInternal.String())
}

func TestHTTPStatusForCode(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected int
	}{
		{ErrCodeInternal, 500},
		{ErrCodeBadRequest, 400},
		{ErrCodeNotFound, 404},
		{ErrCodeConflict, 409},
		{ErrCodeValidation, 422},
		{ErrorCode("UNKNOWN"), 500},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, HTTPStatusForCode(tt.code))
	}
}

func TestDefaultMessageForCode(t *testing.T) {
	assert.Equal(t, "internal server error", DefaultMessageForCode(ErrCodeInternal))
	assert.Equal(t, "unknown error", DefaultMessageForCode(ErrorCode("UNKNOWN")))
}

func TestIsClientError(t *testing.T) {
	assert.True(t, IsClientError(ErrCodeBadRequest))
	assert.False(t, IsClientError(ErrCodeInternal))
}

func TestIsServerError(t *testing.T) {
	assert.True(t, IsServerError(ErrCodeInternal))
	assert.False(t, IsServerError(ErrCodeBadRequest))
}

func TestModuleForCode(t *testing.T) {
	assert.Equal(t, "COMMON", ModuleForCode(ErrCodeInternal))
	assert.Equal(t, "MOL", ModuleForCode(ErrCodeMoleculeNotFound))
	assert.Equal(t, "PAT", ModuleForCode(ErrCodePatentNotFound))
	assert.Equal(t, "INF", ModuleForCode(ErrCodeInfringementAnalysisFailed))
	assert.Equal(t, "FTO", ModuleForCode(ErrCodeFTOAnalysisFailed))
	assert.Equal(t, "DES", ModuleForCode(ErrCodeDesignAroundFailed))
	assert.Equal(t, "VAL", ModuleForCode(ErrCodeValuationFailed))
	assert.Equal(t, "WTC", ModuleForCode(ErrCodeWatchlistNotFound))
	assert.Equal(t, "SRC", ModuleForCode(ErrCodeDataSourceUnavailable))
	assert.Equal(t, "AI", ModuleForCode(ErrCodeAIModelNotAvailable))
	assert.Equal(t, "UNKNOWN", ModuleForCode(ErrorCode("")))
}

func TestErrorCodeFormat_Convention(t *testing.T) {
	re := regexp.MustCompile(`^[A-Z]+_\d{3}$`)
	allCodes := []ErrorCode{
		ErrCodeInternal, ErrCodeBadRequest, ErrCodeMoleculeNotFound, ErrCodePatentNotFound,
		ErrCodeInfringementAnalysisFailed, ErrCodeFTOAnalysisFailed, ErrCodeDesignAroundFailed,
		ErrCodeValuationFailed, ErrCodeWatchlistNotFound, ErrCodeDataSourceUnavailable,
		ErrCodeAIModelNotAvailable,
	}
	for _, code := range allCodes {
		assert.Regexp(t, re, string(code))
	}
}

func TestErrorCodeMappings_Completeness(t *testing.T) {
	// A sample of codes to check if they are in both maps
	allCodes := []ErrorCode{
		ErrCodeInternal, ErrCodeMoleculeInvalidSMILES, ErrCodePatentNotFound,
		ErrCodeInfringementAnalysisFailed, ErrCodeFTOAnalysisFailed,
		ErrCodeDesignAroundFailed, ErrCodeValuationFailed, ErrCodeWatchlistNotFound,
		ErrCodeDataSourceUnavailable, ErrCodeAIModelNotAvailable,
	}
	for _, code := range allCodes {
		_, hasStatus := ErrorCodeHTTPStatus[code]
		_, hasMessage := ErrorCodeMessage[code]
		assert.True(t, hasStatus, "missing status for %s", code)
		assert.True(t, hasMessage, "missing message for %s", code)
	}
}

//Personal.AI order the ending
