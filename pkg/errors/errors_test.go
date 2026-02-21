package errors

import (
	stdliberrors "errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAppError_Error_Format(t *testing.T) {
	err := New(ErrCodeInternal, "test message")
	assert.Equal(t, "[COMMON_001] test message", err.Error())
}

func TestAppError_Unwrap(t *testing.T) {
	cause := stdliberrors.New("original error")
	err := Wrap(cause, ErrCodeInternal, "wrapped message")
	assert.Equal(t, cause, err.Unwrap())

	err2 := New(ErrCodeInternal, "no cause")
	assert.Nil(t, err2.Unwrap())
}

func TestAppError_WithDetails(t *testing.T) {
	err := New(ErrCodeInternal, "message").
		WithDetails("key1", "value1").
		WithDetails("key2", 2)
	assert.Equal(t, "value1", err.Details["key1"])
	assert.Equal(t, 2, err.Details["key2"])
}

func TestAppError_WithCause(t *testing.T) {
	cause := stdliberrors.New("cause")
	err := New(ErrCodeInternal, "message").WithCause(cause)
	assert.Equal(t, cause, err.Cause)
}

func TestAppError_WithRequestID(t *testing.T) {
	err := New(ErrCodeInternal, "message").WithRequestID("req-123")
	assert.Equal(t, "req-123", err.RequestID)
}

func TestAppError_WithInternalMessage(t *testing.T) {
	err := New(ErrCodeInternal, "message").WithInternalMessage("internal")
	assert.Equal(t, "internal", err.InternalMessage)
}

func TestAppError_HTTPStatus(t *testing.T) {
	assert.Equal(t, 500, New(ErrCodeInternal, "m").HTTPStatus())
	assert.Equal(t, 400, New(ErrCodeBadRequest, "m").HTTPStatus())
}

func TestAppError_IsClientError(t *testing.T) {
	assert.True(t, New(ErrCodeBadRequest, "m").IsClientError())
	assert.False(t, New(ErrCodeInternal, "m").IsClientError())
}

func TestAppError_IsServerError(t *testing.T) {
	assert.True(t, New(ErrCodeInternal, "m").IsServerError())
	assert.False(t, New(ErrCodeBadRequest, "m").IsServerError())
}

func TestAppError_ToErrorDetail(t *testing.T) {
	err := New(ErrCodeInternal, "message").WithDetails("k", "v")
	detail := err.ToErrorDetail()
	assert.Equal(t, "COMMON_001", detail.Code)
	assert.Equal(t, "message", detail.Message)
	assert.Equal(t, "v", detail.Details["k"])
}

func TestAppError_LogFields(t *testing.T) {
	cause := stdliberrors.New("cause")
	err := Wrap(cause, ErrCodeInternal, "message").
		WithInternalMessage("internal").
		WithRequestID("req-1").
		WithDetails("k", "v")
	fields := err.LogFields()
	assert.Equal(t, ErrCodeInternal, fields["error_code"])
	assert.Equal(t, "message", fields["message"])
	assert.Equal(t, "COMMON", fields["module"])
	assert.Equal(t, "internal", fields["internal_message"])
	assert.Equal(t, "req-1", fields["request_id"])
	assert.Equal(t, "cause", fields["cause"])
	assert.Equal(t, "v", fields["detail.k"])
	assert.NotEmpty(t, fields["stack"])
}

func TestNew_CreatesAppError(t *testing.T) {
	err := New(ErrCodeInternal, "message")
	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeInternal, err.Code)
	assert.Equal(t, "message", err.Message)
	assert.WithinDuration(t, time.Now(), err.Timestamp, time.Second)
	assert.Equal(t, "COMMON", err.Module)
	assert.NotEmpty(t, err.Stack)
}

func TestNewf(t *testing.T) {
	err := Newf(ErrCodeInternal, "hello %s", "world")
	assert.Equal(t, "hello world", err.Message)
}

func TestWrap(t *testing.T) {
	cause := stdliberrors.New("cause")
	err := Wrap(cause, ErrCodeInternal, "wrapped")
	assert.Equal(t, cause, err.Cause)
	assert.Equal(t, "wrapped", err.Message)
	assert.True(t, stdliberrors.Is(err, cause))
}

func TestFromCode(t *testing.T) {
	err := FromCode(ErrCodeInternal)
	assert.Equal(t, "internal server error", err.Message)
}

func TestConvenientFactories(t *testing.T) {
	assert.Equal(t, ErrCodeInternal, ErrInternal("m").Code)
	assert.Equal(t, ErrCodeBadRequest, ErrBadRequest("m").Code)

	errNotFound := ErrNotFound("patent", "123")
	assert.Equal(t, ErrCodeNotFound, errNotFound.Code)
	assert.Contains(t, errNotFound.Message, "patent")
	assert.Contains(t, errNotFound.Message, "123")

	assert.Equal(t, ErrCodeUnauthorized, ErrUnauthorized("m").Code)
	assert.Equal(t, ErrCodeForbidden, ErrForbidden("m").Code)

	errConflict := ErrConflict("patent", "123")
	assert.Equal(t, ErrCodeConflict, errConflict.Code)

	errValidation := ErrValidation("m", map[string]interface{}{"f": "e"})
	assert.Equal(t, ErrCodeValidation, errValidation.Code)
	assert.Equal(t, "e", errValidation.Details["f"])

	assert.Equal(t, ErrCodeTimeout, ErrTimeout("op").Code)
	assert.Equal(t, ErrCodeServiceUnavailable, ErrServiceUnavailable("svc").Code)

	cause := stdliberrors.New("cause")
	assert.Equal(t, ErrCodeExternalService, ErrExternalService("svc", cause).Code)
	assert.Equal(t, ErrCodeDatabaseError, ErrDatabase("op", cause).Code)
	assert.Equal(t, ErrCodeCacheError, ErrCache("op", cause).Code)
}

func TestModuleSpecificFactories(t *testing.T) {
	cause := stdliberrors.New("cause")
	assert.Equal(t, ErrCodeMoleculeInvalidSMILES, ErrInvalidSMILES("C").Code)
	assert.Equal(t, ErrCodeMoleculeInvalidInChI, ErrInvalidInChI("I").Code)
	assert.Equal(t, ErrCodeMoleculeNotFound, ErrMoleculeNotFound("1").Code)
	assert.Equal(t, ErrCodeMoleculeAlreadyExists, ErrMoleculeAlreadyExists("K").Code)
	assert.Equal(t, ErrCodeFingerprintGenerationFailed, ErrFingerprintGeneration("1", "M", cause).Code)
	assert.Equal(t, ErrCodeGNNModelError, ErrGNNModel("op", cause).Code)
	assert.Equal(t, ErrCodeSimilaritySearchFailed, ErrSimilaritySearch(cause).Code)

	assert.Equal(t, ErrCodePatentNotFound, ErrPatentNotFound("1").Code)
	assert.Equal(t, ErrCodePatentAlreadyExists, ErrPatentAlreadyExists("1").Code)
	assert.Equal(t, ErrCodePatentNumberInvalid, ErrPatentNumberInvalid("1").Code)
	assert.Equal(t, ErrCodeClaimAnalysisFailed, ErrClaimAnalysis("1", cause).Code)
	assert.Equal(t, ErrCodeMarkushParseFailed, ErrMarkushParse("1", cause).Code)

	assert.Equal(t, ErrCodeFTOAnalysisFailed, ErrFTOAnalysis(cause).Code)
	assert.Equal(t, ErrCodeInfringementAnalysisFailed, ErrInfringementAnalysis(cause).Code)
	assert.Equal(t, ErrCodeDesignAroundFailed, ErrDesignAround(cause).Code)
}

func TestJudgmentFunctions(t *testing.T) {
	err := ErrNotFound("p", "1")
	wrapped := fmt.Errorf("wrapped: %w", err)

	assert.True(t, IsCode(err, ErrCodeNotFound))
	assert.True(t, IsCode(wrapped, ErrCodeNotFound))
	assert.False(t, IsCode(err, ErrCodeInternal))

	assert.True(t, IsNotFound(err))
	assert.True(t, IsNotFound(ErrMoleculeNotFound("1")))
	assert.False(t, IsNotFound(ErrInternal("m")))

	assert.True(t, IsConflict(ErrConflict("p", "1")))
	assert.True(t, IsConflict(ErrMoleculeAlreadyExists("k")))
	assert.False(t, IsConflict(err))

	assert.True(t, IsValidation(ErrValidation("m", nil)))

	assert.True(t, IsClientErr(ErrBadRequest("m")))
	assert.False(t, IsClientErr(ErrInternal("m")))

	assert.True(t, IsServerErr(ErrInternal("m")))
	assert.False(t, IsServerErr(ErrBadRequest("m")))

	code, ok := GetCode(err)
	assert.True(t, ok)
	assert.Equal(t, ErrCodeNotFound, code)

	ae, ok := GetAppError(wrapped)
	assert.True(t, ok)
	assert.Equal(t, err, ae)
}

func TestErrorsCompatibility(t *testing.T) {
	cause := stdliberrors.New("cause")
	err := Wrap(cause, ErrCodeInternal, "message")

	assert.True(t, stdliberrors.Is(err, cause))

	var ae *AppError
	assert.True(t, stdliberrors.As(err, &ae))
	assert.Equal(t, ErrCodeInternal, ae.Code)
}

func TestSentinelErrors(t *testing.T) {
	assert.Equal(t, ErrCodeValidation, ErrInvalidPagination.Code)
	assert.Equal(t, ErrCodeValidation, ErrInvalidDateRange.Code)
	assert.Equal(t, ErrCodeValidation, ErrInvalidSortOrder.Code)
}

func TestCaptureStack(t *testing.T) {
	stack := captureStack(0)
	assert.Contains(t, stack, "TestCaptureStack")
}

//Personal.AI order the ending
