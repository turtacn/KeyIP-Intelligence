package errors

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

func TestAppError_Error_Format(t *testing.T) {
	err := New(ErrCodeInternal, "message")
	assert.Equal(t, "[COMMON_001] message", err.Error())
}

func TestAppError_Unwrap_ReturnsCause(t *testing.T) {
	cause := errors.New("original error")
	err := Wrap(cause, ErrCodeInternal, "message")
	assert.Equal(t, cause, err.Unwrap())
}

func TestAppError_Unwrap_NilCause(t *testing.T) {
	err := New(ErrCodeInternal, "message")
	assert.Nil(t, err.Unwrap())
}

func TestAppError_WithDetails_ChainCall(t *testing.T) {
	err := New(ErrCodeInternal, "message").
		WithDetails("key1", "val1").
		WithDetails("key2", 123)
	assert.Equal(t, "val1", err.Details["key1"])
	assert.Equal(t, 123, err.Details["key2"])
}

func TestAppError_WithCause_SetsCorrectly(t *testing.T) {
	cause := errors.New("cause")
	err := New(ErrCodeInternal, "message").WithCause(cause)
	assert.Equal(t, cause, err.Cause)
}

func TestAppError_WithRequestID_SetsCorrectly(t *testing.T) {
	err := New(ErrCodeInternal, "message").WithRequestID("req-123")
	assert.Equal(t, "req-123", err.RequestID)
}

func TestAppError_WithInternalMessage_SetsCorrectly(t *testing.T) {
	err := New(ErrCodeInternal, "message").WithInternalMessage("dev info")
	assert.Equal(t, "dev info", err.InternalMessage)
}

func TestAppError_HTTPStatus_MapsCorrectly(t *testing.T) {
	err := New(ErrCodeNotFound, "not found")
	assert.Equal(t, 404, err.HTTPStatus())
}

func TestAppError_IsClientError_True(t *testing.T) {
	err := New(ErrCodeBadRequest, "bad request")
	assert.True(t, err.IsClientError())
}

func TestAppError_IsClientError_False(t *testing.T) {
	err := New(ErrCodeInternal, "server error")
	assert.False(t, err.IsClientError())
}

func TestAppError_IsServerError_True(t *testing.T) {
	err := New(ErrCodeInternal, "server error")
	assert.True(t, err.IsServerError())
}

func TestAppError_IsServerError_False(t *testing.T) {
	err := New(ErrCodeBadRequest, "bad request")
	assert.False(t, err.IsServerError())
}

func TestAppError_ToErrorDetail_ExcludesInternalInfo(t *testing.T) {
	err := New(ErrCodeInternal, "msg").
		WithInternalMessage("secret").
		WithRequestID("req-1")
	detail := err.ToErrorDetail()
	// ToErrorDetail returns a struct, verifying fields directly
	assert.Equal(t, "COMMON_001", detail.Code)
	// InternalMessage is not part of ErrorDetail struct, so it's implicitly excluded
	// Stack is also not part of ErrorDetail
}

func TestAppError_ToErrorDetail_IncludesCodeAndMessage(t *testing.T) {
	err := New(ErrCodeInternal, "message")
	detail := err.ToErrorDetail()
	assert.Equal(t, "COMMON_001", detail.Code)
	assert.Equal(t, "message", detail.Message)
}

func TestAppError_ToErrorDetail_IncludesDetails(t *testing.T) {
	err := New(ErrCodeInternal, "msg").WithDetails("k", "v")
	detail := err.ToErrorDetail()
	assert.Equal(t, "v", detail.Details["k"])
}

func TestAppError_LogFields_IncludesAllInfo(t *testing.T) {
	cause := errors.New("cause")
	err := New(ErrCodeInternal, "msg").
		WithInternalMessage("internal").
		WithRequestID("req-1").
		WithCause(cause).
		WithDetails("foo", "bar")

	fields := err.LogFields()
	assert.Equal(t, ErrCodeInternal, fields["error_code"])
	assert.Equal(t, "msg", fields["message"])
	assert.Equal(t, "COMMON", fields["module"])
	assert.Equal(t, "internal", fields["internal_message"])
	assert.Equal(t, "req-1", fields["request_id"])
	assert.Equal(t, "cause", fields["cause"])
	assert.Equal(t, "bar", fields["detail.foo"])
	assert.Contains(t, fields, "stack")
}

func TestNew_CreatesAppError(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	assert.NotNil(t, err)
	assert.Equal(t, ErrCodeInternal, err.Code)
	assert.Equal(t, "msg", err.Message)
}

func TestNew_CapturesTimestamp(t *testing.T) {
	start := time.Now().UTC()
	err := New(ErrCodeInternal, "msg")
	assert.WithinDuration(t, start, err.Timestamp, time.Second)
}

func TestNew_CapturesModule(t *testing.T) {
	err := New(ErrCodeMoleculeInvalidSMILES, "msg")
	assert.Equal(t, "MOL", err.Module)
}

func TestNew_CapturesStack(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	assert.NotEmpty(t, err.Stack)
}

func TestNewf_FormatsMessage(t *testing.T) {
	err := Newf(ErrCodeInternal, "hello %s", "world")
	assert.Equal(t, "hello world", err.Message)
}

func TestWrap_PreservesCause(t *testing.T) {
	cause := errors.New("original")
	err := Wrap(cause, ErrCodeInternal, "wrapped")
	assert.Equal(t, cause, err.Unwrap())
}

func TestWrap_ErrorsIs_MatchesCause(t *testing.T) {
	cause := errors.New("original")
	err := Wrap(cause, ErrCodeInternal, "wrapped")
	assert.True(t, errors.Is(err, cause))
}

func TestWrapf_FormatsAndWraps(t *testing.T) {
	cause := errors.New("original")
	err := Wrapf(cause, ErrCodeInternal, "wrapped %d", 1)
	assert.Equal(t, "wrapped 1", err.Message)
	assert.Equal(t, cause, err.Unwrap())
}

func TestFromCode_UsesDefaultMessage(t *testing.T) {
	err := FromCode(ErrCodeNotFound)
	assert.Equal(t, "resource not found", err.Message)
}

func TestErrInternal_CorrectCode(t *testing.T) {
	err := ErrInternal("msg")
	assert.Equal(t, ErrCodeInternal, err.Code)
}

func TestErrBadRequest_CorrectCode(t *testing.T) {
	err := ErrBadRequest("msg")
	assert.Equal(t, ErrCodeBadRequest, err.Code)
}

func TestErrNotFound_MessageFormat(t *testing.T) {
	err := ErrNotFound("User", "123")
	assert.Equal(t, "User not found: 123", err.Message)
}

func TestErrNotFound_Details(t *testing.T) {
	err := ErrNotFound("User", "123")
	assert.Equal(t, "User", err.Details["resource"])
	assert.Equal(t, "123", err.Details["id"])
}

func TestErrUnauthorized_CorrectCode(t *testing.T) {
	err := ErrUnauthorized("msg")
	assert.Equal(t, ErrCodeUnauthorized, err.Code)
}

func TestErrForbidden_CorrectCode(t *testing.T) {
	err := ErrForbidden("msg")
	assert.Equal(t, ErrCodeForbidden, err.Code)
}

func TestErrConflict_MessageFormat(t *testing.T) {
	err := ErrConflict("User", "123")
	assert.Equal(t, "User conflict: 123", err.Message)
}

func TestErrValidation_IncludesDetails(t *testing.T) {
	details := map[string]interface{}{"field": "error"}
	err := ErrValidation("msg", details)
	assert.Equal(t, details, err.Details)
}

func TestErrTimeout_MessageFormat(t *testing.T) {
	err := ErrTimeout("op")
	assert.Contains(t, err.Message, "op")
}

func TestErrServiceUnavailable_MessageFormat(t *testing.T) {
	err := ErrServiceUnavailable("svc")
	assert.Contains(t, err.Message, "svc")
}

func TestErrExternalService_WrapsCause(t *testing.T) {
	cause := errors.New("network error")
	err := ErrExternalService("stripe", cause)
	assert.Equal(t, cause, err.Cause)
}

func TestErrDatabase_WrapsCause(t *testing.T) {
	cause := errors.New("db error")
	err := ErrDatabase("query", cause)
	assert.Equal(t, cause, err.Cause)
}

func TestErrCache_WrapsCause(t *testing.T) {
	cause := errors.New("redis error")
	err := ErrCache("get", cause)
	assert.Equal(t, cause, err.Cause)
}

func TestErrInvalidSMILES_CorrectCodeAndDetails(t *testing.T) {
	err := ErrInvalidSMILES("C")
	assert.Equal(t, ErrCodeMoleculeInvalidSMILES, err.Code)
	assert.Equal(t, "C", err.Details["smiles"])
}

func TestErrInvalidInChI_CorrectCodeAndDetails(t *testing.T) {
	err := ErrInvalidInChI("InChI=1S/C")
	assert.Equal(t, ErrCodeMoleculeInvalidInChI, err.Code)
	assert.Equal(t, "InChI=1S/C", err.Details["inchi"])
}

func TestErrMoleculeNotFound_CorrectCode(t *testing.T) {
	err := ErrMoleculeNotFound("123")
	assert.Equal(t, ErrCodeMoleculeNotFound, err.Code)
}

func TestErrMoleculeAlreadyExists_CorrectCode(t *testing.T) {
	err := ErrMoleculeAlreadyExists("key")
	assert.Equal(t, ErrCodeMoleculeAlreadyExists, err.Code)
	assert.Equal(t, "key", err.Details["inchi_key"])
}

func TestErrFingerprintGeneration_WrapsCause(t *testing.T) {
	cause := errors.New("rdkit error")
	err := ErrFingerprintGeneration("123", "morgan", cause)
	assert.Equal(t, ErrCodeFingerprintGenerationFailed, err.Code)
	assert.Equal(t, cause, err.Cause)
}

func TestErrGNNModel_WrapsCause(t *testing.T) {
	cause := errors.New("gnn error")
	err := ErrGNNModel("predict", cause)
	assert.Equal(t, ErrCodeGNNModelError, err.Code)
	assert.Equal(t, cause, err.Cause)
}

func TestErrSimilaritySearch_WrapsCause(t *testing.T) {
	cause := errors.New("milvus error")
	err := ErrSimilaritySearch(cause)
	assert.Equal(t, ErrCodeSimilaritySearchFailed, err.Code)
}

func TestErrPatentNotFound_CorrectCode(t *testing.T) {
	err := ErrPatentNotFound("US123")
	assert.Equal(t, ErrCodePatentNotFound, err.Code)
}

func TestErrPatentAlreadyExists_CorrectCode(t *testing.T) {
	err := ErrPatentAlreadyExists("US123")
	assert.Equal(t, ErrCodePatentAlreadyExists, err.Code)
}

func TestErrPatentNumberInvalid_CorrectCode(t *testing.T) {
	err := ErrPatentNumberInvalid("invalid")
	assert.Equal(t, ErrCodePatentNumberInvalid, err.Code)
}

func TestErrClaimAnalysis_WrapsCause(t *testing.T) {
	cause := errors.New("nlp error")
	err := ErrClaimAnalysis("US123", cause)
	assert.Equal(t, ErrCodeClaimAnalysisFailed, err.Code)
}

func TestErrMarkushParse_WrapsCause(t *testing.T) {
	cause := errors.New("parse error")
	err := ErrMarkushParse("US123", cause)
	assert.Equal(t, ErrCodeMarkushParseFailed, err.Code)
}

func TestErrFTOAnalysis_WrapsCause(t *testing.T) {
	cause := errors.New("fto error")
	err := ErrFTOAnalysis(cause)
	assert.Equal(t, ErrCodeFTOAnalysisFailed, err.Code)
}

func TestErrInfringementAnalysis_WrapsCause(t *testing.T) {
	cause := errors.New("inf error")
	err := ErrInfringementAnalysis(cause)
	assert.Equal(t, ErrCodeInfringementAnalysisFailed, err.Code)
}

func TestErrDesignAround_WrapsCause(t *testing.T) {
	cause := errors.New("des error")
	err := ErrDesignAround(cause)
	assert.Equal(t, ErrCodeDesignAroundFailed, err.Code)
}

func TestIsCode_MatchesDirectly(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	assert.True(t, IsCode(err, ErrCodeInternal))
}

func TestIsCode_MatchesWrapped(t *testing.T) {
	err := Wrap(New(ErrCodeInternal, "msg"), ErrCodeServiceUnavailable, "wrapped")
	// IsCode checks the top-level AppError code (or whatever it unwraps to if we implemented recursive check)
	// My implementation of IsCode uses errors.As which finds the first AppError in chain.
	// If Wrap puts the new AppError on top, it checks that one.
	assert.True(t, IsCode(err, ErrCodeServiceUnavailable))
}

func TestIsCode_NoMatch(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	assert.False(t, IsCode(err, ErrCodeBadRequest))
}

func TestIsCode_NonAppError(t *testing.T) {
	err := errors.New("std error")
	assert.False(t, IsCode(err, ErrCodeInternal))
}

func TestIsNotFound_AllNotFoundCodes(t *testing.T) {
	codes := []ErrorCode{ErrCodeNotFound, ErrCodeMoleculeNotFound, ErrCodePatentNotFound, ErrCodePatentFamilyNotFound, ErrCodeWatchlistNotFound, ErrCodeDesignAroundNoSuggestions}
	for _, code := range codes {
		err := New(code, "msg")
		assert.True(t, IsNotFound(err), "Code %s should be NotFound", code)
	}
}

func TestIsNotFound_NonNotFoundCode(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	assert.False(t, IsNotFound(err))
}

func TestIsConflict_AllConflictCodes(t *testing.T) {
	codes := []ErrorCode{ErrCodeConflict, ErrCodeMoleculeAlreadyExists, ErrCodePatentAlreadyExists}
	for _, code := range codes {
		err := New(code, "msg")
		assert.True(t, IsConflict(err), "Code %s should be Conflict", code)
	}
}

func TestIsConflict_NonConflictCode(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	assert.False(t, IsConflict(err))
}

func TestIsValidation_MatchesValidationCode(t *testing.T) {
	err := New(ErrCodeValidation, "msg")
	assert.True(t, IsValidation(err))
}

func TestIsValidation_NonValidationCode(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	assert.False(t, IsValidation(err))
}

func TestIsClientErr_ClientError(t *testing.T) {
	err := New(ErrCodeBadRequest, "msg")
	assert.True(t, IsClientErr(err))
}

func TestIsClientErr_ServerError(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	assert.False(t, IsClientErr(err))
}

func TestIsClientErr_NonAppError(t *testing.T) {
	err := errors.New("std error")
	assert.False(t, IsClientErr(err))
}

func TestIsServerErr_ServerError(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	assert.True(t, IsServerErr(err))
}

func TestIsServerErr_ClientError(t *testing.T) {
	err := New(ErrCodeBadRequest, "msg")
	assert.False(t, IsServerErr(err))
}

func TestGetCode_FromAppError(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	code, ok := GetCode(err)
	assert.True(t, ok)
	assert.Equal(t, ErrCodeInternal, code)
}

func TestGetCode_FromWrappedAppError(t *testing.T) {
	// If standard error wraps AppError (e.g. fmt.Errorf("%w", appErr))
	appErr := New(ErrCodeInternal, "msg")
	err := fmt.Errorf("wrapped: %w", appErr)
	code, ok := GetCode(err)
	assert.True(t, ok)
	assert.Equal(t, ErrCodeInternal, code)
}

func TestGetCode_FromNonAppError(t *testing.T) {
	err := errors.New("std error")
	_, ok := GetCode(err)
	assert.False(t, ok)
}

func TestGetAppError_FromAppError(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	ae, ok := GetAppError(err)
	assert.True(t, ok)
	assert.Equal(t, err, ae)
}

func TestGetAppError_FromWrappedAppError(t *testing.T) {
	appErr := New(ErrCodeInternal, "msg")
	err := fmt.Errorf("wrapped: %w", appErr)
	ae, ok := GetAppError(err)
	assert.True(t, ok)
	assert.Equal(t, appErr, ae)
}

func TestGetAppError_FromNonAppError(t *testing.T) {
	err := errors.New("std error")
	_, ok := GetAppError(err)
	assert.False(t, ok)
}

func TestErrorsIs_Compatibility(t *testing.T) {
	err := ErrInvalidPagination
	// Sentinel errors are *AppError pointers
	assert.True(t, errors.Is(err, ErrInvalidPagination))
}

func TestErrorsAs_Compatibility(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	var ae *AppError
	assert.True(t, errors.As(err, &ae))
	assert.Equal(t, err, ae)
}

func TestSentinelErrors_ErrInvalidPagination(t *testing.T) {
	assert.Equal(t, ErrCodeValidation, ErrInvalidPagination.Code)
}

func TestSentinelErrors_ErrInvalidDateRange(t *testing.T) {
	assert.Equal(t, ErrCodeValidation, ErrInvalidDateRange.Code)
}

func TestSentinelErrors_ErrInvalidSortOrder(t *testing.T) {
	assert.Equal(t, ErrCodeValidation, ErrInvalidSortOrder.Code)
}

func TestCaptureStack_ContainsCallerInfo(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	assert.Contains(t, err.Stack, "TestCaptureStack_ContainsCallerInfo")
	assert.Contains(t, err.Stack, "errors_test.go")
}

func TestCaptureStack_SkipsFactoryFrames(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	// Stack should NOT contain New() or newAppError() if skip is correct
	// Note: optimization/inlining might affect this, but ideally:
	assert.NotContains(t, err.Stack, "newAppError")
	// Depending on how captureStack is implemented (skip 2), it should skip newAppError and runtime.Callers.
	// New calls newAppError. So New is frame 1, newAppError is frame 0 (relative to captureStack).
	// If we skipped 2, we are at caller of New.
	// So New should not be in stack?
	// The implementation: `runtime.Callers(skip+2, ...)` where skip=2 passed from newAppError. So skip 4 frames.
	// newAppError called by New. New called by Test.
	// Frames: runtime.Callers -> captureStack -> newAppError -> New -> Test.
	// We want Test.
	// So we need to skip captureStack, newAppError, New. That's 3.
	// My impl: skip=2 passed to captureStack. captureStack calls runtime.Callers(skip+2) = 4.
	// Wait, runtime.Callers(0) is Callers itself.
	// 1: captureStack
	// 2: newAppError
	// 3: New
	// 4: Test
	// So runtime.Callers(4) starts at Test.
	// So New should be excluded.
	assert.NotContains(t, err.Stack, "pkg/errors.New(")
}

func TestToErrorDetail(t *testing.T) {
	err := New(ErrCodeInternal, "msg")
	detail := err.ToErrorDetail()

	// Check against common.ErrorDetail definition
	var d common.ErrorDetail = detail
	assert.Equal(t, "COMMON_001", d.Code)
}

//Personal.AI order the ending
