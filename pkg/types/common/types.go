package common

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ID is a string alias for UUID v4.
type ID string

// UserID is a string alias for a user identifier.
type UserID string

// TenantID is a string alias for a tenant identifier.
type TenantID string

// Metadata is an open-ended key-value bag.
type Metadata map[string]interface{}

// Status represents the lifecycle state of a platform entity.
type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusPending  Status = "pending"
	StatusArchived Status = "archived"
	StatusDeleted  Status = "deleted"
)

// BaseEntity carries audit metadata for domain entities and DTOs.
type BaseEntity struct {
	ID        ID        `json:"id"`
	TenantID  TenantID  `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   int       `json:"version"`
}

// Timestamp is a time.Time alias with custom JSON serialization.
type Timestamp time.Time

// Pagination defines parameters for paginated requests.
type Pagination struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total,omitempty"`
}

// SortOrder defines the direction of sorting.
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// SortField defines a field and its sort order.
type SortField struct {
	Field string    `json:"field"`
	Order SortOrder `json:"order"`
}

// DateRange defines a time interval.
type DateRange struct {
	From Timestamp `json:"from"`
	To   Timestamp `json:"to"`
}

// TimeRange is an alias for DateRange for backward compatibility.
type TimeRange = DateRange

// ErrorDetail provides structured error information for API responses.
type ErrorDetail struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// APIResponse is the generic wrapper for all API responses.
type APIResponse[T any] struct {
	Success    bool         `json:"success"`
	Data       T            `json:"data,omitempty"`
	Error      *ErrorDetail `json:"error,omitempty"`
	Pagination *Pagination  `json:"pagination,omitempty"`
	RequestID  string       `json:"request_id"`
	Timestamp  Timestamp    `json:"timestamp"`
}

// ListRequest carries parameters for listing resources.
type ListRequest struct {
	Pagination Pagination             `json:"pagination"`
	Sort       []SortField            `json:"sort,omitempty"`
	Filters    map[string]interface{} `json:"filters,omitempty"`
}

// BatchRequest carries a list of items for batch operations.
type BatchRequest[T any] struct {
	Items       []T  `json:"items"`
	StopOnError bool `json:"stop_on_error"`
}

// BatchError describes an error in a batch operation.
type BatchError struct {
	Index int         `json:"index"`
	Error ErrorDetail `json:"error"`
}

// BatchResponse summarizes the result of a batch operation.
type BatchResponse[T any] struct {
	Succeeded      []T          `json:"succeeded"`
	Failed         []BatchError `json:"failed"`
	TotalProcessed int          `json:"total_processed"`
}

// HealthStatus indicates the health of a component or service.
type HealthStatus string

const (
	HealthUp       HealthStatus = "up"
	HealthDown     HealthStatus = "down"
	HealthDegraded HealthStatus = "degraded"
)

// ComponentHealth provides health information for a specific component.
type ComponentHealth struct {
	Name    string        `json:"name"`
	Status  HealthStatus  `json:"status"`
	Latency time.Duration `json:"latency"`
	Message string        `json:"message"`
}

// PaginationResult holds the pagination metadata for a response.
type PaginationResult struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// PaginatedResult is a generic wrapper for paginated data with pagination metadata.
type PaginatedResult[T any] struct {
	Items      []T              `json:"items"`
	Pagination PaginationResult `json:"pagination"`
}

// PageResponse is a generic wrapper for paginated results (compatibility alias).
type PageResponse[T any] struct {
	Items      []T `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

// DomainEvent represents a significant event in the domain.
type DomainEvent interface {
	EventID() string
	OccurredAt() time.Time
	AggregateID() string
}

// BaseEvent provides common fields for domain events.
type BaseEvent struct {
	ID        string    `json:"event_id"`
	Timestamp time.Time `json:"occurred_at"`
	AggID     string    `json:"aggregate_id"`
}

func NewBaseEvent(aggID string) BaseEvent {
	return BaseEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now().UTC(),
		AggID:     aggID,
	}
}

func (e BaseEvent) EventID() string {
	return e.ID
}

func (e BaseEvent) OccurredAt() time.Time {
	return e.Timestamp
}

func (e BaseEvent) AggregateID() string {
	return e.AggID
}

// Validate checks if the ID is a valid UUID v4.
func (id ID) Validate() error {
	if id == "" {
		return fmt.Errorf("ID cannot be empty")
	}
	_, err := uuid.Parse(string(id))
	if err != nil {
		return fmt.Errorf("invalid ID format: %w", err)
	}
	return nil
}

// ToUnixMilli returns the timestamp in milliseconds since Unix epoch.
func (t Timestamp) ToUnixMilli() int64 {
	return time.Time(t).UnixMilli()
}

// FromUnixMilli converts milliseconds since Unix epoch to a Timestamp.
func FromUnixMilli(msec int64) Timestamp {
	return Timestamp(time.UnixMilli(msec).UTC())
}

// MarshalJSON implements json.Marshaler, using ISO 8601 format.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).Format(time.RFC3339Nano))
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		// Try fallback to RFC3339 if Nano fails
		parsed, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return err
		}
	}
	*t = Timestamp(parsed.UTC())
	return nil
}

// Validate checks if pagination parameters are within valid bounds.
func (p Pagination) Validate() error {
	if p.Page < 1 {
		return fmt.Errorf("page must be >= 1")
	}
	// PageSize defaults to 20 if 0, but if explictly checked here:
	// The requirement says: 1 <= PageSize <= 500.
	// Often 0 means default. But strict check:
	if p.PageSize < 1 || p.PageSize > 500 {
		// To align strictly with "PageSize 超出 [1, 500] 范围时 Validate() 返回 ErrInvalidPagination"
		// Assuming we define ErrInvalidPagination or return a specific error.
		// Since ErrInvalidPagination is in pkg/errors (sentinel), here we return generic error or specific message.
		return fmt.Errorf("page_size must be between 1 and 500")
	}
	return nil
}

// Offset returns the SQL OFFSET value.
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// Validate checks if the date range is valid.
func (dr DateRange) Validate() error {
	if time.Time(dr.From).After(time.Time(dr.To)) {
		return fmt.Errorf("invalid date range: 'from' must be before or equal to 'to'")
	}
	return nil
}

// NewID generates a new UUID v4.
func NewID() ID {
	return ID(uuid.New().String())
}

// GenerateID generates a unique ID with an optional prefix.
func GenerateID(prefix string) string {
	if prefix == "" {
		return uuid.New().String()
	}
	return fmt.Sprintf("%s-%s", prefix, uuid.New().String())
}

// NewTimestamp returns the current UTC time as a Timestamp.
func NewTimestamp() Timestamp {
	return Timestamp(time.Now().UTC())
}

// NewSuccessResponse creates a successful APIResponse.
func NewSuccessResponse[T any](data T) APIResponse[T] {
	return APIResponse[T]{
		Success:   true,
		Data:      data,
		Timestamp: NewTimestamp(),
	}
}

// NewErrorResponse creates an error APIResponse.
func NewErrorResponse(code string, message string) APIResponse[any] {
	return APIResponse[any]{
		Success: false,
		Error: &ErrorDetail{
			Code:    code,
			Message: message,
		},
		Timestamp: NewTimestamp(),
	}
}

// NewPaginatedResponse creates a successful paginated APIResponse.
func NewPaginatedResponse[T any](data T, pagination Pagination) APIResponse[T] {
	return APIResponse[T]{
		Success:    true,
		Data:       data,
		Pagination: &pagination,
		Timestamp:  NewTimestamp(),
	}
}

// Context keys for request context
type ContextKey string

const (
	// ContextKeyUserID is the context key for user ID.
	ContextKeyUserID ContextKey = "user_id"
	// ContextKeyTenantID is the context key for tenant ID.
	ContextKeyTenantID ContextKey = "tenant_id"
	// ContextKeyRequestID is the context key for request ID.
	ContextKeyRequestID ContextKey = "request_id"
)

// RiskLevel represents the risk assessment level.
type RiskLevel string

const (
	// RiskLow indicates low risk.
	RiskLow RiskLevel = "LOW"
	// RiskMedium indicates medium risk.
	RiskMedium RiskLevel = "MEDIUM"
	// RiskHigh indicates high risk.
	RiskHigh RiskLevel = "HIGH"
	// RiskCritical indicates critical risk.
	RiskCritical RiskLevel = "CRITICAL"
)

//Personal.AI order the ending
