// File: pkg/client/client_test.go
package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_NewClient_Valid(t *testing.T) {
	client, err := NewClient("https://api.example.com", "test-api-key")
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestClient_NewClient_EmptyBaseURL(t *testing.T) {
	_, err := NewClient("", "test-api-key")
	assert.Error(t, err)
}

func TestClient_NewClient_EmptyAPIKey(t *testing.T) {
	_, err := NewClient("https://api.example.com", "")
	assert.Error(t, err)
}

func TestAPIError_Error(t *testing.T) {
	apiErr := &APIError{
		StatusCode: 404,
		Code:       "NOT_FOUND",
		Message:    "Resource not found",
		RequestID:  "req-123",
	}
	errStr := apiErr.Error()
	assert.NotEmpty(t, errStr)
}

func TestAPIError_IsNotFound(t *testing.T) {
	apiErr := &APIError{StatusCode: 404}
	assert.True(t, apiErr.IsNotFound())
}

func TestAPIError_IsUnauthorized(t *testing.T) {
	apiErr := &APIError{StatusCode: 401}
	assert.True(t, apiErr.IsUnauthorized())
}

func TestAPIError_IsRateLimited(t *testing.T) {
	apiErr := &APIError{StatusCode: 429}
	assert.True(t, apiErr.IsRateLimited())
}

func TestAPIError_IsServerError(t *testing.T) {
	for _, code := range []int{500, 502, 503, 504} {
		apiErr := &APIError{StatusCode: code}
		assert.True(t, apiErr.IsServerError())
	}
	apiErr := &APIError{StatusCode: 400}
	assert.False(t, apiErr.IsServerError())
}

//Personal.AI order the ending
