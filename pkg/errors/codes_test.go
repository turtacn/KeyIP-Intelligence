package errors

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCode_String(t *testing.T) {
	code := ErrorCode("TEST_CODE")
	assert.Equal(t, "TEST_CODE", code.String())
}

func TestHTTPStatusForCode_CommonErrors(t *testing.T) {
	assert.Equal(t, http.StatusInternalServerError, HTTPStatusForCode(ErrCodeInternal))
	assert.Equal(t, http.StatusBadRequest, HTTPStatusForCode(ErrCodeBadRequest))
	assert.Equal(t, http.StatusUnauthorized, HTTPStatusForCode(ErrCodeUnauthorized))
	assert.Equal(t, http.StatusNotFound, HTTPStatusForCode(ErrCodeNotFound))
}

func TestHTTPStatusForCode_MolErrors(t *testing.T) {
	assert.Equal(t, http.StatusBadRequest, HTTPStatusForCode(ErrCodeMoleculeInvalidSMILES))
	assert.Equal(t, http.StatusNotFound, HTTPStatusForCode(ErrCodeMoleculeNotFound))
	assert.Equal(t, http.StatusInternalServerError, HTTPStatusForCode(ErrCodeFingerprintGenerationFailed))
}

func TestHTTPStatusForCode_PatErrors(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, HTTPStatusForCode(ErrCodePatentNotFound))
	assert.Equal(t, http.StatusBadRequest, HTTPStatusForCode(ErrCodePatentNumberInvalid))
}

func TestHTTPStatusForCode_InfErrors(t *testing.T) {
	assert.Equal(t, http.StatusInternalServerError, HTTPStatusForCode(ErrCodeInfringementAnalysisFailed))
}

func TestHTTPStatusForCode_FTOErrors(t *testing.T) {
	assert.Equal(t, http.StatusInternalServerError, HTTPStatusForCode(ErrCodeFTOAnalysisFailed))
}

func TestHTTPStatusForCode_DesErrors(t *testing.T) {
	assert.Equal(t, http.StatusInternalServerError, HTTPStatusForCode(ErrCodeDesignAroundFailed))
}

func TestHTTPStatusForCode_ValErrors(t *testing.T) {
	assert.Equal(t, http.StatusInternalServerError, HTTPStatusForCode(ErrCodeValuationFailed))
}

func TestHTTPStatusForCode_WtcErrors(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, HTTPStatusForCode(ErrCodeWatchlistNotFound))
}

func TestHTTPStatusForCode_SrcErrors(t *testing.T) {
	assert.Equal(t, http.StatusServiceUnavailable, HTTPStatusForCode(ErrCodeDataSourceUnavailable))
}

func TestHTTPStatusForCode_AIErrors(t *testing.T) {
	assert.Equal(t, http.StatusServiceUnavailable, HTTPStatusForCode(ErrCodeAIModelNotAvailable))
}

func TestHTTPStatusForCode_UnknownCode(t *testing.T) {
	assert.Equal(t, http.StatusInternalServerError, HTTPStatusForCode(ErrorCode("UNKNOWN")))
}

func TestDefaultMessageForCode_AllCodes(t *testing.T) {
	for code := range ErrorCodeMessage {
		assert.NotEmpty(t, DefaultMessageForCode(code))
	}
}

func TestDefaultMessageForCode_UnknownCode(t *testing.T) {
	assert.Equal(t, "unknown error", DefaultMessageForCode(ErrorCode("UNKNOWN")))
}

func TestIsClientError_4xxCodes(t *testing.T) {
	assert.True(t, IsClientError(ErrCodeBadRequest))
	assert.True(t, IsClientError(ErrCodeNotFound))
}

func TestIsClientError_5xxCodes(t *testing.T) {
	assert.False(t, IsClientError(ErrCodeInternal))
}

func TestIsServerError_5xxCodes(t *testing.T) {
	assert.True(t, IsServerError(ErrCodeInternal))
}

func TestIsServerError_4xxCodes(t *testing.T) {
	assert.False(t, IsServerError(ErrCodeBadRequest))
}

func TestModuleForCode_AllModules(t *testing.T) {
	tests := []struct {
		code   ErrorCode
		module string
	}{
		{ErrCodeInternal, "COMMON"},
		{ErrCodeMoleculeInvalidSMILES, "MOL"},
		{ErrCodePatentNotFound, "PAT"},
		{ErrCodeInfringementAnalysisFailed, "INF"},
		{ErrCodeFTOAnalysisFailed, "FTO"},
		{ErrCodeDesignAroundFailed, "DES"},
		{ErrCodeValuationFailed, "VAL"},
		{ErrCodeWatchlistNotFound, "WTC"},
		{ErrCodeDataSourceUnavailable, "SRC"},
		{ErrCodeAIModelNotAvailable, "AI"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.module, ModuleForCode(tt.code))
	}
}

func TestModuleForCode_UnknownFormat(t *testing.T) {
	assert.Equal(t, "UNKNOWN", ModuleForCode(ErrorCode("INVALID")))
}

func TestErrorCodeHTTPStatus_Completeness(t *testing.T) {
	// Check coverage in a basic way - just ensure key maps are populated
	assert.NotEmpty(t, ErrorCodeHTTPStatus)
}

func TestErrorCodeMessage_Completeness(t *testing.T) {
	assert.NotEmpty(t, ErrorCodeMessage)
}

func TestErrorCodeFormat_Convention(t *testing.T) {
	re := regexp.MustCompile(`^[A-Z]+_\d{3}$`)
	// We only check a sample because iterating all constants isn't trivial without reflection on package
	// But we can check the keys in the map
	for code := range ErrorCodeHTTPStatus {
		assert.True(t, re.MatchString(string(code)), "Code %s does not match format", code)
	}
}

func TestHTTPStatusForCode_ClientErrorRange(t *testing.T) {
	for code := range ErrorCodeHTTPStatus {
		if IsClientError(code) {
			status := HTTPStatusForCode(code)
			assert.True(t, status >= 400 && status < 500, "Code %s status %d not in 4xx range", code, status)
		}
	}
}

func TestHTTPStatusForCode_ServerErrorRange(t *testing.T) {
	for code := range ErrorCodeHTTPStatus {
		if IsServerError(code) {
			status := HTTPStatusForCode(code)
			assert.True(t, status >= 500 && status < 600, "Code %s status %d not in 5xx range", code, status)
		}
	}
}

//Personal.AI order the ending
