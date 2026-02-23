package errors

import (
	stdliberrors "errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// AppError is the custom error type for the platform.
type AppError struct {
	Code            ErrorCode              `json:"code"`
	Message         string                 `json:"message"`
	InternalMessage string                 `json:"internal_message,omitempty"`
	Details         map[string]interface{} `json:"details,omitempty"`
	Cause           error                  `json:"-"`
	Stack           string                 `json:"stack,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
	RequestID       string                 `json:"request_id,omitempty"`
	Module          string                 `json:"module"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap implements the error unwrap interface.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithDetails adds details to the error.
func (e *AppError) WithDetails(key string, value interface{}) *AppError {
	if e == nil {
		return nil
	}
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithDetail is an alias for WithDetails for backward compatibility.
func (e *AppError) WithDetail(detail string) *AppError {
	return e.WithDetails("detail", detail)
}

// WithCause sets the underlying cause of the error.
func (e *AppError) WithCause(err error) *AppError {
	if e == nil {
		return nil
	}
	e.Cause = err
	return e
}

// WithRequestID sets the request ID associated with the error.
func (e *AppError) WithRequestID(requestID string) *AppError {
	if e == nil {
		return nil
	}
	e.RequestID = requestID
	return e
}

// WithInternalMessage sets the internal developer-focused message.
func (e *AppError) WithInternalMessage(msg string) *AppError {
	if e == nil {
		return nil
	}
	e.InternalMessage = msg
	return e
}

// HTTPStatus returns the associated HTTP status code.
func (e *AppError) HTTPStatus() int {
	return HTTPStatusForCode(e.Code)
}

// IsClientError returns true if the error is a client-side error.
func (e *AppError) IsClientError() bool {
	return IsClientError(e.Code)
}

// IsServerError returns true if the error is a server-side error.
func (e *AppError) IsServerError() bool {
	return IsServerError(e.Code)
}

// ToErrorDetail converts the AppError to a common.ErrorDetail.
func (e *AppError) ToErrorDetail() common.ErrorDetail {
	return common.ErrorDetail{
		Code:    string(e.Code),
		Message: e.Message,
		Details: e.Details,
	}
}

// LogFields returns a map of fields for structured logging.
func (e *AppError) LogFields() map[string]interface{} {
	fields := map[string]interface{}{
		"error_code":       e.Code,
		"message":          e.Message,
		"module":           e.Module,
		"timestamp":        e.Timestamp,
		"internal_message": e.InternalMessage,
		"request_id":       e.RequestID,
	}
	if e.Cause != nil {
		fields["cause"] = e.Cause.Error()
	}
	if e.Stack != "" {
		fields["stack"] = e.Stack
	}
	for k, v := range e.Details {
		fields["detail."+k] = v
	}
	return fields
}

func captureStack(skip int) string {
	const maxDepth = 32
	pcs := make([]uintptr, maxDepth)
	n := runtime.Callers(skip+2, pcs)
	if n == 0 {
		return ""
	}
	frames := runtime.CallersFrames(pcs[:n])
	var sb strings.Builder
	for {
		f, more := frames.Next()
		if !strings.Contains(f.File, "runtime/") {
			fmt.Fprintf(&sb, "\n\t%s:%d %s", f.File, f.Line, f.Function)
		}
		if !more {
			break
		}
	}
	return sb.String()
}

func newAppError(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:      code,
		Message:   message,
		Timestamp: time.Now().UTC(),
		Module:    ModuleForCode(code),
		Stack:     captureStack(2),
	}
}

// Factory Functions

func New(code ErrorCode, message string) *AppError {
	return newAppError(code, message)
}

func Newf(code ErrorCode, format string, args ...interface{}) *AppError {
	return newAppError(code, fmt.Sprintf(format, args...))
}

func Wrap(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}
	// Preserve original code if current code is common or unknown
	if code == ErrCodeInternal || code == "" {
		var existing *AppError
		if stdliberrors.As(err, &existing) {
			code = existing.Code
		}
	}
	ae := newAppError(code, message)
	ae.Cause = err
	return ae
}

func Wrapf(err error, code ErrorCode, format string, args ...interface{}) *AppError {
	if err == nil {
		return nil
	}
	ae := newAppError(code, fmt.Sprintf(format, args...))
	ae.Cause = err
	return ae
}

func FromCode(code ErrorCode) *AppError {
	return newAppError(code, DefaultMessageForCode(code))
}

// Common convenient factory functions

func ErrInternal(message string) *AppError {
	return New(ErrCodeInternal, message)
}

func ErrBadRequest(message string) *AppError {
	return New(ErrCodeBadRequest, message)
}

func NewInvalidInputError(message string) *AppError {
	return ErrBadRequest(message)
}

func NewInvalidParameterError(message string) *AppError {
	return ErrBadRequest(message)
}

func NewValidation(message string, args ...interface{}) *AppError {
	if len(args) > 0 {
		// If additional args are provided, treat the first one as context/field if string
		if ctx, ok := args[0].(string); ok {
			return New(ErrCodeValidation, message).WithDetail(ctx)
		}
	}
	return New(ErrCodeValidation, message)
}

func NewValidationError(field, message string) *AppError {
	return New(ErrCodeValidation, message).WithDetail(field)
}

func NewInternal(format string, args ...interface{}) *AppError {
	return Newf(ErrCodeInternal, format, args...)
}

func NewNotFound(format string, args ...interface{}) *AppError {
	return Newf(ErrCodeNotFound, format, args...)
}

func ErrNotFound(resource string, id string) *AppError {
	return New(ErrCodeNotFound, fmt.Sprintf("%s not found: %s", resource, id)).
		WithDetails("resource", resource).
		WithDetails("id", id)
}

func ErrUnauthorized(message string) *AppError {
	return New(ErrCodeUnauthorized, message)
}

func ErrForbidden(message string) *AppError {
	return New(ErrCodeForbidden, message)
}

func ErrConflict(resource string, identifier string) *AppError {
	return New(ErrCodeConflict, fmt.Sprintf("%s conflict: %s", resource, identifier)).
		WithDetails("resource", resource).
		WithDetails("identifier", identifier)
}

// Aliases for backward compatibility
func Internal(message string) *AppError     { return ErrInternal(message) }
func InvalidParam(message string) *AppError { return ErrBadRequest(message) }
func Unauthorized(message string) *AppError { return ErrUnauthorized(message) }
func Forbidden(message string) *AppError    { return ErrForbidden(message) }
func NotFound(message string) *AppError     { return New(ErrCodeNotFound, message) }
func Conflict(message string) *AppError     { return New(ErrCodeConflict, message) }
func InvalidState(message string) *AppError { return New(ErrCodeConflict, message) }
func RateLimit(message string) *AppError    { return New(ErrCodeTooManyRequests, message) }

func ErrValidation(message string, details map[string]interface{}) *AppError {
	ae := New(ErrCodeValidation, message)
	ae.Details = details
	return ae
}

func ErrTimeout(operation string) *AppError {
	return New(ErrCodeTimeout, fmt.Sprintf("operation timed out: %s", operation)).
		WithDetails("operation", operation)
}

func ErrServiceUnavailable(service string) *AppError {
	return New(ErrCodeServiceUnavailable, fmt.Sprintf("service unavailable: %s", service)).
		WithDetails("service", service)
}

func ErrExternalService(service string, err error) *AppError {
	return Wrap(err, ErrCodeExternalService, fmt.Sprintf("external service error: %s", service)).
		WithDetails("service", service)
}

func ErrDatabase(operation string, err error) *AppError {
	return Wrap(err, ErrCodeDatabaseError, fmt.Sprintf("database error during %s", operation)).
		WithDetails("operation", operation)
}

func ErrCache(operation string, err error) *AppError {
	return Wrap(err, ErrCodeCacheError, fmt.Sprintf("cache error during %s", operation)).
		WithDetails("operation", operation)
}

// Molecule module convenient factory functions

func ErrInvalidSMILES(smiles string) *AppError {
	return New(ErrCodeMoleculeInvalidSMILES, "invalid SMILES format").
		WithDetails("smiles", smiles)
}

func ErrInvalidInChI(inchi string) *AppError {
	return New(ErrCodeMoleculeInvalidInChI, "invalid InChI format").
		WithDetails("inchi", inchi)
}

func ErrMoleculeNotFound(id string) *AppError {
	return New(ErrCodeMoleculeNotFound, fmt.Sprintf("molecule not found: %s", id)).
		WithDetails("molecule_id", id)
}

func ErrMoleculeAlreadyExists(inchiKey string) *AppError {
	return New(ErrCodeMoleculeAlreadyExists, "molecule already exists").
		WithDetails("inchi_key", inchiKey)
}

func ErrFingerprintGeneration(moleculeID string, fpType string, err error) *AppError {
	return Wrap(err, ErrCodeFingerprintGenerationFailed, "failed to generate fingerprint").
		WithDetails("molecule_id", moleculeID).
		WithDetails("fingerprint_type", fpType)
}

func ErrGNNModel(operation string, err error) *AppError {
	return Wrap(err, ErrCodeGNNModelError, fmt.Sprintf("GNN model error during %s", operation)).
		WithDetails("operation", operation)
}

func ErrSimilaritySearch(err error) *AppError {
	return Wrap(err, ErrCodeSimilaritySearchFailed, "similarity search failed")
}

// Patent module convenient factory functions

func ErrPatentNotFound(patentNumber string) *AppError {
	return New(ErrCodePatentNotFound, fmt.Sprintf("patent not found: %s", patentNumber)).
		WithDetails("patent_number", patentNumber)
}

func ErrPatentAlreadyExists(patentNumber string) *AppError {
	return New(ErrCodePatentAlreadyExists, fmt.Sprintf("patent already exists: %s", patentNumber)).
		WithDetails("patent_number", patentNumber)
}

func ErrPatentNumberInvalid(patentNumber string) *AppError {
	return New(ErrCodePatentNumberInvalid, fmt.Sprintf("invalid patent number: %s", patentNumber)).
		WithDetails("patent_number", patentNumber)
}

func ErrClaimAnalysis(patentNumber string, err error) *AppError {
	return Wrap(err, ErrCodeClaimAnalysisFailed, fmt.Sprintf("claim analysis failed for patent %s", patentNumber)).
		WithDetails("patent_number", patentNumber)
}

func ErrMarkushParse(patentNumber string, err error) *AppError {
	return Wrap(err, ErrCodeMarkushParseFailed, fmt.Sprintf("Markush parse failed for patent %s", patentNumber)).
		WithDetails("patent_number", patentNumber)
}

// FTO/Infringement module convenient factory functions

func ErrFTOAnalysis(err error) *AppError {
	return Wrap(err, ErrCodeFTOAnalysisFailed, "FTO analysis failed")
}

func ErrInfringementAnalysis(err error) *AppError {
	return Wrap(err, ErrCodeInfringementAnalysisFailed, "infringement analysis failed")
}

func ErrDesignAround(err error) *AppError {
	return Wrap(err, ErrCodeDesignAroundFailed, "design-around failed")
}

// Judgment Functions

func IsCode(err error, code ErrorCode) bool {
	var ae *AppError
	if stdliberrors.As(err, &ae) {
		return ae.Code == code
	}
	return false
}

func IsNotFound(err error) bool {
	var ae *AppError
	if stdliberrors.As(err, &ae) {
		switch ae.Code {
		case ErrCodeNotFound, ErrCodeMoleculeNotFound, ErrCodePatentNotFound, ErrCodePatentFamilyNotFound, ErrCodeWatchlistNotFound, ErrCodeDesignAroundNoSuggestions, ErrCodePortfolioNotFound:
			return true
		}
	}
	return false
}

func IsConflict(err error) bool {
	var ae *AppError
	if stdliberrors.As(err, &ae) {
		switch ae.Code {
		case ErrCodeConflict, ErrCodeMoleculeAlreadyExists, ErrCodePatentAlreadyExists:
			return true
		}
	}
	return false
}

func IsValidation(err error) bool {
	return IsCode(err, ErrCodeValidation)
}

func IsClientErr(err error) bool {
	var ae *AppError
	if stdliberrors.As(err, &ae) {
		return ae.IsClientError()
	}
	return false
}

func IsServerErr(err error) bool {
	var ae *AppError
	if stdliberrors.As(err, &ae) {
		return ae.IsServerError()
	}
	return false
}

func GetCode(err error) (ErrorCode, bool) {
	var ae *AppError
	if stdliberrors.As(err, &ae) {
		return ae.Code, true
	}
	return "", false
}

func GetAppError(err error) (*AppError, bool) {
	var ae *AppError
	if stdliberrors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// Sentinel Errors

var (
	ErrInvalidPagination = New(ErrCodeValidation, "invalid pagination parameters")
	ErrInvalidDateRange  = New(ErrCodeValidation, "invalid date range: from must be before or equal to to")
	ErrInvalidSortOrder  = New(ErrCodeValidation, "invalid sort order")
	ErrParsingFailed     = New(ErrCodeValidation, "parsing failed")
	ErrInvalidMolecule   = New(ErrCodeMoleculeInvalidFormat, "invalid molecule")
	ErrInvalidConfig     = New(ErrCodeValidation, "invalid configuration")
)

func NewParsingError(message string) *AppError {
	return New(ErrCodeValidation, message)
}

func NewValidationOp(operation string, message string) *AppError {
	return New(ErrCodeValidation, message).WithDetails("operation", operation)
}

func NewNotFoundOp(operation string, message string) *AppError {
	return New(ErrCodeNotFound, message).WithDetails("operation", operation)
}

func NewInternalOp(operation string, message string) *AppError {
	return New(ErrCodeInternal, message).WithDetails("operation", operation)
}

var (
	ErrInferenceTimeout        = New(ErrCodeTimeout, "inference timed out")
	ErrModelBackendUnavailable = New(ErrCodeServiceUnavailable, "model backend unavailable")
	ErrServingUnavailable      = New(ErrCodeServiceUnavailable, "serving unavailable")
)

func Is(err, target error) bool {
	return stdliberrors.Is(err, target)
}

func As(err error, target interface{}) bool {
	return stdliberrors.As(err, target)
}

//Personal.AI order the ending
