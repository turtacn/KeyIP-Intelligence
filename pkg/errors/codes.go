package errors

import (
	"net/http"
	"strings"
)

// ErrorCode is a string representation of a specific error condition.
type ErrorCode string

func (c ErrorCode) String() string {
	return string(c)
}

// Common Error Codes
const (
	ErrCodeInternal           ErrorCode = "COMMON_001"
	ErrCodeBadRequest         ErrorCode = "COMMON_002"
	ErrCodeUnauthorized       ErrorCode = "COMMON_003"
	ErrCodeForbidden          ErrorCode = "COMMON_004"
	ErrCodeNotFound           ErrorCode = "COMMON_005"
	ErrCodeConflict           ErrorCode = "COMMON_006"
	ErrCodeTooManyRequests    ErrorCode = "COMMON_007"
	ErrCodeServiceUnavailable ErrorCode = "COMMON_008"
	ErrCodeTimeout             ErrorCode = "COMMON_009"
	ErrCodeValidation         ErrorCode = "COMMON_010"
	ErrCodeSerialization      ErrorCode = "COMMON_011"
	ErrCodeDatabaseError      ErrorCode = "COMMON_012"
	ErrCodeCacheError         ErrorCode = "COMMON_013"
	ErrCodeExternalService    ErrorCode = "COMMON_014"
	ErrCodeFeatureDisabled    ErrorCode = "COMMON_015"
	ErrCodeNotImplemented     ErrorCode = "COMMON_016"
)

// Aliases for backward compatibility
const (
	CodeInternal       = ErrCodeInternal
	CodeInvalidParam   = ErrCodeBadRequest
	CodeUnauthorized   = ErrCodeUnauthorized
	CodeForbidden      = ErrCodeForbidden
	CodeNotFound       = ErrCodeNotFound
	CodeConflict       = ErrCodeConflict
	CodeRateLimit      = ErrCodeTooManyRequests
	CodeNotImplemented = ErrCodeNotImplemented
	CodeOK             = ErrorCode("OK")

	// Domain specific aliases
	CodePatentNotFound        = ErrCodePatentNotFound
	CodePortfolioNotFound     = ErrCodePortfolioNotFound
	CodeMoleculeInvalidSMILES = ErrCodeMoleculeInvalidSMILES
	CodeMoleculeNotFound      = ErrCodeMoleculeNotFound
)

// Molecule Module Error Codes
const (
	ErrCodeMoleculeInvalidSMILES      ErrorCode = "MOL_001"
	ErrCodeMoleculeInvalidInChI       ErrorCode = "MOL_002"
	ErrCodeMoleculeInvalidFormat      ErrorCode = "MOL_003"
	ErrCodeMoleculeNotFound           ErrorCode = "MOL_004"
	ErrCodeMoleculeAlreadyExists      ErrorCode = "MOL_005"
	ErrCodeMoleculeParsingFailed      ErrorCode = "MOL_006"
	ErrCodeFingerprintGenerationFailed ErrorCode = "MOL_007"
	ErrCodeFingerprintTypeUnsupported ErrorCode = "MOL_008"
	ErrCodeSimilaritySearchFailed     ErrorCode = "MOL_009"
	ErrCodeSimilarityThresholdInvalid ErrorCode = "MOL_010"
	ErrCodeMoleculeConversionFailed   ErrorCode = "MOL_011"
	ErrCodeSubstructureSearchFailed   ErrorCode = "MOL_012"
	ErrCodePropertyPredictionFailed   ErrorCode = "MOL_013"
	ErrCodeGNNModelError              ErrorCode = "MOL_014"
	ErrCodeGNNModelNotLoaded          ErrorCode = "MOL_015"
)

// Patent Module Error Codes
const (
	ErrCodePatentNotFound        ErrorCode = "PAT_001"
	ErrCodePatentAlreadyExists   ErrorCode = "PAT_002"
	ErrCodePatentNumberInvalid   ErrorCode = "PAT_003"
	ErrCodePatentOfficeUnsupported ErrorCode = "PAT_004"
	ErrCodePatentFetchFailed     ErrorCode = "PAT_005"
	ErrCodePatentParseFailed     ErrorCode = "PAT_006"
	ErrCodeClaimAnalysisFailed   ErrorCode = "PAT_007"
	ErrCodeMarkushParseFailed     ErrorCode = "PAT_008"
	ErrCodePatentFamilyNotFound  ErrorCode = "PAT_009"
	ErrCodePatentExpired         ErrorCode = "PAT_010"
	ErrCodePatentStatusInvalid   ErrorCode = "PAT_011"
	ErrCodePortfolioNotFound     ErrorCode = "PAT_012"
)

// Infringement Module Error Codes
const (
	ErrCodeInfringementAnalysisFailed ErrorCode = "INF_001"
	ErrCodeInfringementDataInsufficient ErrorCode = "INF_002"
	ErrCodeEquivalentsAnalysisFailed ErrorCode = "INF_003"
	ErrCodeProsecutionHistoryUnavailable ErrorCode = "INF_004"
)

// FTO Module Error Codes
const (
	ErrCodeFTOAnalysisFailed           ErrorCode = "FTO_001"
	ErrCodeFTOJurisdictionUnsupported  ErrorCode = "FTO_002"
	ErrCodeFTOReportGenerationFailed   ErrorCode = "FTO_003"
)

// Design Around Module Error Codes
const (
	ErrCodeDesignAroundFailed             ErrorCode = "DES_001"
	ErrCodeDesignAroundNoSuggestions      ErrorCode = "DES_002"
	ErrCodeDesignAroundConstraintConflict ErrorCode = "DES_003"
)

// Valuation Module Error Codes
const (
	ErrCodeValuationFailed         ErrorCode = "VAL_001"
	ErrCodeValuationDataInsufficient ErrorCode = "VAL_002"
)

// Watchlist Module Error Codes
const (
	ErrCodeWatchlistNotFound      ErrorCode = "WTC_001"
	ErrCodeWatchlistLimitExceeded ErrorCode = "WTC_002"
	ErrCodeWatchlistConfigInvalid ErrorCode = "WTC_003"
	ErrCodeAlertDeliveryFailed    ErrorCode = "WTC_004"
	ErrCodeAlertChannelUnsupported ErrorCode = "WTC_005"
)

// Data Source Error Codes
const (
	ErrCodeDataSourceUnavailable ErrorCode = "SRC_001"
	ErrCodeDataSourceRateLimited ErrorCode = "SRC_002"
	ErrCodeDataSourceAuthFailed  ErrorCode = "SRC_003"
	ErrCodeDataSourceParseError  ErrorCode = "SRC_004"
)

// AI/ML Module Error Codes
const (
	ErrCodeAIModelNotAvailable    ErrorCode = "AI_001"
	ErrCodeAIInferenceFailed      ErrorCode = "AI_002"
	ErrCodeAIModelVersionMismatch ErrorCode = "AI_003"
	ErrCodeAIInputInvalid         ErrorCode = "AI_004"
	ErrCodeAIResourceExhausted    ErrorCode = "AI_005"
)

// Infrastructure Error Codes (mapped from old names)
const (
	CodeDBConnectionError = ErrCodeDatabaseError
	CodeDatabaseError     = ErrCodeDatabaseError
	CodeDBQueryError      = ErrCodeDatabaseError
	CodeCacheError        = ErrCodeCacheError
	CodeSearchError       = ErrCodeSimilaritySearchFailed
	CodeMessageQueueError = ErrCodeInternal
	CodeStorageError      = ErrCodeInternal
)

// ErrorCodeHTTPStatus maps ErrorCodes to HTTP status codes.
var ErrorCodeHTTPStatus = map[ErrorCode]int{
	ErrCodeInternal:           http.StatusInternalServerError,
	ErrCodeBadRequest:         http.StatusBadRequest,
	ErrCodeUnauthorized:       http.StatusUnauthorized,
	ErrCodeForbidden:          http.StatusForbidden,
	ErrCodeNotFound:           http.StatusNotFound,
	ErrCodeConflict:           http.StatusConflict,
	ErrCodeTooManyRequests:    http.StatusTooManyRequests,
	ErrCodeServiceUnavailable: http.StatusServiceUnavailable,
	ErrCodeTimeout:             http.StatusGatewayTimeout,
	ErrCodeValidation:         http.StatusUnprocessableEntity,
	ErrCodeSerialization:      http.StatusInternalServerError,
	ErrCodeDatabaseError:      http.StatusInternalServerError,
	ErrCodeCacheError:         http.StatusInternalServerError,
	ErrCodeExternalService:    http.StatusInternalServerError,
	ErrCodeFeatureDisabled:    http.StatusForbidden,

	ErrCodeMoleculeInvalidSMILES:      http.StatusBadRequest,
	ErrCodeMoleculeInvalidInChI:       http.StatusBadRequest,
	ErrCodeMoleculeInvalidFormat:      http.StatusBadRequest,
	ErrCodeMoleculeNotFound:           http.StatusNotFound,
	ErrCodeMoleculeAlreadyExists:      http.StatusConflict,
	ErrCodeMoleculeParsingFailed:      http.StatusInternalServerError,
	ErrCodeFingerprintGenerationFailed: http.StatusInternalServerError,
	ErrCodeFingerprintTypeUnsupported: http.StatusInternalServerError,
	ErrCodeSimilaritySearchFailed:     http.StatusInternalServerError,
	ErrCodeSimilarityThresholdInvalid: http.StatusBadRequest,
	ErrCodeMoleculeConversionFailed:   http.StatusInternalServerError,
	ErrCodeSubstructureSearchFailed:   http.StatusInternalServerError,
	ErrCodePropertyPredictionFailed:   http.StatusInternalServerError,
	ErrCodeGNNModelError:              http.StatusInternalServerError,
	ErrCodeGNNModelNotLoaded:          http.StatusInternalServerError,

	ErrCodePatentNotFound:        http.StatusNotFound,
	ErrCodePatentAlreadyExists:   http.StatusConflict,
	ErrCodePatentNumberInvalid:   http.StatusBadRequest,
	ErrCodePatentOfficeUnsupported: http.StatusBadRequest,
	ErrCodePatentFetchFailed:     http.StatusInternalServerError,
	ErrCodePatentParseFailed:     http.StatusInternalServerError,
	ErrCodeClaimAnalysisFailed:   http.StatusInternalServerError,
	ErrCodeMarkushParseFailed:     http.StatusInternalServerError,
	ErrCodePatentFamilyNotFound:  http.StatusNotFound,
	ErrCodePatentExpired:         http.StatusInternalServerError,
	ErrCodePatentStatusInvalid:   http.StatusBadRequest,
	ErrCodePortfolioNotFound:     http.StatusNotFound,

	ErrCodeInfringementAnalysisFailed:    http.StatusInternalServerError,
	ErrCodeInfringementDataInsufficient: http.StatusInternalServerError,
	ErrCodeEquivalentsAnalysisFailed:     http.StatusInternalServerError,
	ErrCodeProsecutionHistoryUnavailable: http.StatusInternalServerError,

	ErrCodeFTOAnalysisFailed:          http.StatusInternalServerError,
	ErrCodeFTOJurisdictionUnsupported: http.StatusBadRequest,
	ErrCodeFTOReportGenerationFailed:  http.StatusInternalServerError,

	ErrCodeDesignAroundFailed:             http.StatusInternalServerError,
	ErrCodeDesignAroundNoSuggestions:      http.StatusNotFound,
	ErrCodeDesignAroundConstraintConflict: http.StatusBadRequest,

	ErrCodeValuationFailed:         http.StatusInternalServerError,
	ErrCodeValuationDataInsufficient: http.StatusInternalServerError,

	ErrCodeWatchlistNotFound:      http.StatusNotFound,
	ErrCodeWatchlistLimitExceeded: http.StatusTooManyRequests,
	ErrCodeWatchlistConfigInvalid: http.StatusBadRequest,
	ErrCodeAlertDeliveryFailed:    http.StatusInternalServerError,
	ErrCodeAlertChannelUnsupported: http.StatusInternalServerError,

	ErrCodeDataSourceUnavailable: http.StatusServiceUnavailable,
	ErrCodeDataSourceRateLimited: http.StatusTooManyRequests,
	ErrCodeDataSourceAuthFailed:  http.StatusBadGateway,
	ErrCodeDataSourceParseError:  http.StatusBadGateway,

	ErrCodeAIModelNotAvailable:    http.StatusServiceUnavailable,
	ErrCodeAIInferenceFailed:      http.StatusInternalServerError,
	ErrCodeAIModelVersionMismatch: http.StatusInternalServerError,
	ErrCodeAIInputInvalid:         http.StatusBadRequest,
	ErrCodeAIResourceExhausted:    http.StatusServiceUnavailable,
	ErrCodeNotImplemented:         http.StatusNotImplemented,
}

// ErrorCodeMessage maps ErrorCodes to default messages.
var ErrorCodeMessage = map[ErrorCode]string{
	ErrCodeInternal:           "internal server error",
	ErrCodeBadRequest:         "bad request",
	ErrCodeUnauthorized:       "unauthorized",
	ErrCodeForbidden:          "forbidden",
	ErrCodeNotFound:           "resource not found",
	ErrCodeConflict:           "resource conflict",
	ErrCodeTooManyRequests:    "too many requests",
	ErrCodeServiceUnavailable: "service unavailable",
	ErrCodeTimeout:             "request timeout",
	ErrCodeValidation:         "validation failed",
	ErrCodeSerialization:      "serialization failed",
	ErrCodeDatabaseError:      "database error",
	ErrCodeCacheError:         "cache error",
	ErrCodeExternalService:    "external service error",
	ErrCodeFeatureDisabled:    "feature disabled",

	ErrCodeMoleculeInvalidSMILES:      "invalid SMILES format",
	ErrCodeMoleculeInvalidInChI:       "invalid InChI format",
	ErrCodeMoleculeInvalidFormat:      "unsupported molecule format",
	ErrCodeMoleculeNotFound:           "molecule not found",
	ErrCodeMoleculeAlreadyExists:      "molecule already exists",
	ErrCodeMoleculeParsingFailed:      "failed to parse molecule",
	ErrCodeFingerprintGenerationFailed: "failed to generate fingerprint",
	ErrCodeFingerprintTypeUnsupported: "unsupported fingerprint type",
	ErrCodeSimilaritySearchFailed:     "similarity search failed",
	ErrCodeSimilarityThresholdInvalid: "invalid similarity threshold",
	ErrCodeMoleculeConversionFailed:   "molecule format conversion failed",
	ErrCodeSubstructureSearchFailed:   "substructure search failed",
	ErrCodePropertyPredictionFailed:   "property prediction failed",
	ErrCodeGNNModelError:              "GNN model inference error",
	ErrCodeGNNModelNotLoaded:          "GNN model not loaded",

	ErrCodePatentNotFound:        "patent not found",
	ErrCodePatentAlreadyExists:   "patent already exists",
	ErrCodePatentNumberInvalid:   "invalid patent number",
	ErrCodePatentOfficeUnsupported: "unsupported patent office",
	ErrCodePatentFetchFailed:     "failed to fetch patent data",
	ErrCodePatentParseFailed:     "failed to parse patent document",
	ErrCodeClaimAnalysisFailed:   "claim analysis failed",
	ErrCodeMarkushParseFailed:     "Markush structure parsing failed",
	ErrCodePatentFamilyNotFound:  "patent family not found",
	ErrCodePatentExpired:         "patent has expired",
	ErrCodePatentStatusInvalid:   "invalid patent status",
	ErrCodePortfolioNotFound:     "portfolio not found",

	ErrCodeInfringementAnalysisFailed:    "infringement analysis failed",
	ErrCodeInfringementDataInsufficient: "insufficient data for infringement analysis",
	ErrCodeEquivalentsAnalysisFailed:     "doctrine of equivalents analysis failed",
	ErrCodeProsecutionHistoryUnavailable: "prosecution history not available",

	ErrCodeFTOAnalysisFailed:          "FTO analysis failed",
	ErrCodeFTOJurisdictionUnsupported: "unsupported FTO jurisdiction",
	ErrCodeFTOReportGenerationFailed:  "failed to generate FTO report",

	ErrCodeDesignAroundFailed:             "design-around failed",
	ErrCodeDesignAroundNoSuggestions:      "no design-around suggestions found",
	ErrCodeDesignAroundConstraintConflict: "design-around constraints conflict",

	ErrCodeValuationFailed:         "patent valuation failed",
	ErrCodeValuationDataInsufficient: "insufficient data for valuation",

	ErrCodeWatchlistNotFound:      "watchlist not found",
	ErrCodeWatchlistLimitExceeded: "watchlist limit exceeded",
	ErrCodeWatchlistConfigInvalid: "invalid watchlist configuration",
	ErrCodeAlertDeliveryFailed:    "failed to deliver alert",
	ErrCodeAlertChannelUnsupported: "unsupported alert channel",

	ErrCodeDataSourceUnavailable: "data source unavailable",
	ErrCodeDataSourceRateLimited: "data source rate limited",
	ErrCodeDataSourceAuthFailed:  "data source authentication failed",
	ErrCodeDataSourceParseError:  "failed to parse data source response",

	ErrCodeAIModelNotAvailable:    "AI model not available",
	ErrCodeAIInferenceFailed:      "AI inference failed",
	ErrCodeAIModelVersionMismatch: "AI model version mismatch",
	ErrCodeAIInputInvalid:         "invalid input for AI model",
	ErrCodeAIResourceExhausted:    "AI calculation resource exhausted",
	ErrCodeNotImplemented:         "not implemented",
}

// HTTPStatusForCode returns the HTTP status code for an ErrorCode.
func HTTPStatusForCode(code ErrorCode) int {
	if status, ok := ErrorCodeHTTPStatus[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// DefaultMessageForCode returns the default message for an ErrorCode.
func DefaultMessageForCode(code ErrorCode) string {
	if msg, ok := ErrorCodeMessage[code]; ok {
		return msg
	}
	return "unknown error"
}

// IsClientError returns true if the ErrorCode corresponds to a 4xx HTTP status.
func IsClientError(code ErrorCode) bool {
	status := HTTPStatusForCode(code)
	return status >= 400 && status < 500
}

// IsServerError returns true if the ErrorCode corresponds to a 5xx HTTP status.
func IsServerError(code ErrorCode) bool {
	status := HTTPStatusForCode(code)
	return status >= 500 && status < 600
}

// ModuleForCode returns the module prefix of an ErrorCode.
func ModuleForCode(code ErrorCode) string {
	parts := strings.Split(string(code), "_")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return "UNKNOWN"
}

//Personal.AI order the ending
