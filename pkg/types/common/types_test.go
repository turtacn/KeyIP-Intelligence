package common

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestID_Validate_ValidUUID(t *testing.T) {
	id := ID("550e8400-e29b-41d4-a716-446655440000")
	err := id.Validate()
	assert.NoError(t, err)
}

func TestID_Validate_EmptyString(t *testing.T) {
	id := ID("")
	err := id.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestID_Validate_InvalidFormat(t *testing.T) {
	id := ID("not-a-uuid")
	err := id.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ID format")
}

func TestNewID_GeneratesValidUUID(t *testing.T) {
	id := NewID()
	err := id.Validate()
	assert.NoError(t, err)
}

func TestTimestamp_MarshalJSON(t *testing.T) {
	now := time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC)
	ts := Timestamp(now)
	data, err := json.Marshal(ts)
	assert.NoError(t, err)
	assert.Equal(t, "\"2023-10-27T10:00:00Z\"", string(data))
}

func TestTimestamp_UnmarshalJSON_Valid(t *testing.T) {
	data := []byte("\"2023-10-27T10:00:00Z\"")
	var ts Timestamp
	err := json.Unmarshal(data, &ts)
	assert.NoError(t, err)
	assert.Equal(t, time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC), time.Time(ts))
}

func TestTimestamp_UnmarshalJSON_Invalid(t *testing.T) {
	data := []byte("\"invalid-date\"")
	var ts Timestamp
	err := json.Unmarshal(data, &ts)
	assert.Error(t, err)
}

func TestTimestamp_ToUnixMilli_RoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	ts := Timestamp(now)
	msec := ts.ToUnixMilli()
	ts2 := FromUnixMilli(msec)
	assert.Equal(t, ts, ts2)
}

func TestPagination_Validate_Valid(t *testing.T) {
	p := Pagination{Page: 1, PageSize: 20}
	err := p.Validate()
	assert.NoError(t, err)
}

func TestPagination_Validate_PageZero(t *testing.T) {
	p := Pagination{Page: 0, PageSize: 20}
	err := p.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "page must be >= 1")
}

func TestPagination_Validate_PageSizeExceedsMax(t *testing.T) {
	p := Pagination{Page: 1, PageSize: 501}
	err := p.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "page_size must be between 1 and 500")
}

func TestPagination_Validate_PageSizeZero(t *testing.T) {
	p := Pagination{Page: 1, PageSize: 0}
	err := p.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "page_size must be between 1 and 500")
}

func TestPagination_Offset(t *testing.T) {
	p := Pagination{Page: 3, PageSize: 20}
	assert.Equal(t, 40, p.Offset())
}

func TestDateRange_Validate_Valid(t *testing.T) {
	from := NewTimestamp()
	to := Timestamp(time.Time(from).Add(time.Hour))
	dr := DateRange{From: from, To: to}
	err := dr.Validate()
	assert.NoError(t, err)
}

func TestDateRange_Validate_FromAfterTo(t *testing.T) {
	to := NewTimestamp()
	from := Timestamp(time.Time(to).Add(time.Hour))
	dr := DateRange{From: from, To: to}
	err := dr.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be before or equal to")
}

func TestDateRange_Validate_Equal(t *testing.T) {
	now := NewTimestamp()
	dr := DateRange{From: now, To: now}
	err := dr.Validate()
	assert.NoError(t, err)
}

func TestNewSuccessResponse(t *testing.T) {
	data := "test-data"
	resp := NewSuccessResponse(data)
	assert.True(t, resp.Success)
	assert.Equal(t, data, resp.Data)
	assert.Nil(t, resp.Error)
}

func TestNewErrorResponse(t *testing.T) {
	code := "ERR001"
	message := "error message"
	resp := NewErrorResponse(code, message)
	assert.False(t, resp.Success)
	require.NotNil(t, resp.Error)
	assert.Equal(t, code, resp.Error.Code)
	assert.Equal(t, message, resp.Error.Message)
}

func TestNewPaginatedResponse(t *testing.T) {
	data := []string{"item1", "item2"}
	pagination := Pagination{Page: 1, PageSize: 10, Total: 2}
	resp := NewPaginatedResponse(data, pagination)
	assert.True(t, resp.Success)
	assert.Equal(t, data, resp.Data)
	require.NotNil(t, resp.Pagination)
	assert.Equal(t, pagination, *resp.Pagination)
}

func TestAPIResponse_JSONRoundTrip(t *testing.T) {
	resp := NewSuccessResponse("data")
	resp.RequestID = "req-123"

	data, err := json.Marshal(resp)
	assert.NoError(t, err)

	var resp2 APIResponse[string]
	err = json.Unmarshal(data, &resp2)
	assert.NoError(t, err)

	assert.Equal(t, resp.Success, resp2.Success)
	assert.Equal(t, resp.Data, resp2.Data)
	assert.Equal(t, resp.RequestID, resp2.RequestID)
	assert.Equal(t, resp.Timestamp.ToUnixMilli(), resp2.Timestamp.ToUnixMilli())
}

func TestBatchRequest_StopOnError_Default(t *testing.T) {
	req := BatchRequest[string]{}
	assert.False(t, req.StopOnError)
}

func TestBatchResponse_Counts(t *testing.T) {
	resp := BatchResponse[string]{
		Succeeded:      []string{"ok1", "ok2"},
		Failed:         []BatchError{{Index: 2, Error: ErrorDetail{Code: "ERR"}}},
		TotalProcessed: 3,
	}
	assert.Equal(t, 3, len(resp.Succeeded)+len(resp.Failed))
	assert.Equal(t, 3, resp.TotalProcessed)
}

func TestSortOrder_Values(t *testing.T) {
	assert.Equal(t, SortOrder("asc"), SortAsc)
	assert.Equal(t, SortOrder("desc"), SortDesc)
}

func TestHealthStatus_Values(t *testing.T) {
	assert.Equal(t, HealthStatus("up"), HealthUp)
	assert.Equal(t, HealthStatus("down"), HealthDown)
	assert.Equal(t, HealthStatus("degraded"), HealthDegraded)
}

//Personal.AI order the ending
