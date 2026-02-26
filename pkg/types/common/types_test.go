package common

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestID_Validate_ValidUUID(t *testing.T) {
	id := ID(uuid.New().String())
	assert.NoError(t, id.Validate())
}

func TestID_Validate_EmptyString(t *testing.T) {
	id := ID("")
	assert.Error(t, id.Validate())
}

func TestID_Validate_InvalidFormat(t *testing.T) {
	id := ID("invalid-uuid")
	assert.Error(t, id.Validate())
}

func TestNewID_GeneratesValidUUID(t *testing.T) {
	id := NewID()
	assert.NoError(t, id.Validate())
}

func TestTimestamp_MarshalJSON(t *testing.T) {
	now := time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)
	ts := Timestamp(now)
	bytes, err := json.Marshal(ts)
	assert.NoError(t, err)
	// Check contains quote and ISO format
	assert.Contains(t, string(bytes), "2023-10-01T12:00:00")
}

func TestTimestamp_UnmarshalJSON_Valid(t *testing.T) {
	jsonStr := "\"2023-10-01T12:00:00Z\""
	var ts Timestamp
	err := json.Unmarshal([]byte(jsonStr), &ts)
	assert.NoError(t, err)
	assert.Equal(t, int64(1696161600000), ts.ToUnixMilli())
}

func TestTimestamp_UnmarshalJSON_Invalid(t *testing.T) {
	jsonStr := "\"invalid-date\""
	var ts Timestamp
	err := json.Unmarshal([]byte(jsonStr), &ts)
	assert.Error(t, err)
}

func TestTimestamp_ToUnixMilli_RoundTrip(t *testing.T) {
	now := NewTimestamp()
	milli := now.ToUnixMilli()
	fromMilli := FromUnixMilli(milli)
	assert.Equal(t, milli, fromMilli.ToUnixMilli())
	// Note: Precision loss might occur if nanoseconds matter, but Timestamp stores time.Time
	// FromUnixMilli truncates to milliseconds.
	// Let's ensure round trip logic is correct for millisecond precision.
	assert.WithinDuration(t, time.Time(now), time.Time(fromMilli), time.Millisecond)
}

func TestPagination_Validate_Valid(t *testing.T) {
	p := Pagination{Page: 1, PageSize: 20}
	assert.NoError(t, p.Validate())
}

func TestPagination_Validate_PageZero(t *testing.T) {
	p := Pagination{Page: 0, PageSize: 20}
	assert.Error(t, p.Validate())
}

func TestPagination_Validate_PageSizeExceedsMax(t *testing.T) {
	p := Pagination{Page: 1, PageSize: 501}
	assert.Error(t, p.Validate())
}

func TestPagination_Validate_PageSizeZero(t *testing.T) {
	p := Pagination{Page: 1, PageSize: 0}
	assert.Error(t, p.Validate())
}

func TestPagination_Offset(t *testing.T) {
	p := Pagination{Page: 3, PageSize: 20}
	assert.Equal(t, 40, p.Offset())
}

func TestDateRange_Validate_Valid(t *testing.T) {
	from := NewTimestamp()
	to := Timestamp(time.Time(from).Add(time.Hour))
	dr := DateRange{From: from, To: to}
	assert.NoError(t, dr.Validate())
}

func TestDateRange_Validate_FromAfterTo(t *testing.T) {
	to := NewTimestamp()
	from := Timestamp(time.Time(to).Add(time.Hour))
	dr := DateRange{From: from, To: to}
	assert.Error(t, dr.Validate())
}

func TestDateRange_Validate_Equal(t *testing.T) {
	now := NewTimestamp()
	dr := DateRange{From: now, To: now}
	assert.NoError(t, dr.Validate())
}

func TestNewSuccessResponse(t *testing.T) {
	data := map[string]string{"foo": "bar"}
	resp := NewSuccessResponse(data)
	assert.True(t, resp.Success)
	assert.Equal(t, data, resp.Data)
	assert.Nil(t, resp.Error)
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse("ERR_001", "error occurred")
	assert.False(t, resp.Success)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "ERR_001", resp.Error.Code)
	assert.Equal(t, "error occurred", resp.Error.Message)
}

func TestNewPaginatedResponse(t *testing.T) {
	data := []string{"a", "b"}
	p := Pagination{Page: 1, PageSize: 10, Total: 2}
	resp := NewPaginatedResponse(data, p)
	assert.True(t, resp.Success)
	assert.Equal(t, data, resp.Data)
	assert.NotNil(t, resp.Pagination)
	assert.Equal(t, p, *resp.Pagination)
}

func TestAPIResponse_JSONRoundTrip(t *testing.T) {
	data := map[string]string{"foo": "bar"}
	resp := NewSuccessResponse(data)
	bytes, err := json.Marshal(resp)
	assert.NoError(t, err)

	var decoded APIResponse[map[string]string]
	err = json.Unmarshal(bytes, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, resp.Success, decoded.Success)
	assert.Equal(t, resp.Data, decoded.Data)
}

func TestBatchRequest_StopOnError_Default(t *testing.T) {
	req := BatchRequest[string]{}
	assert.False(t, req.StopOnError)
}

func TestBatchResponse_Counts(t *testing.T) {
	resp := BatchResponse[string]{
		Succeeded:      []string{"a", "b"},
		Failed:         []BatchError{{Index: 2, Error: ErrorDetail{Code: "ERR"}}},
		TotalProcessed: 3,
	}
	assert.Equal(t, 3, resp.TotalProcessed)
	assert.Len(t, resp.Succeeded, 2)
	assert.Len(t, resp.Failed, 1)
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
