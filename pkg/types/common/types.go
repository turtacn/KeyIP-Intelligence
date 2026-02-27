package common

import (
	"context"
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

// EventType identifies the type of a domain event.
type EventType string

// DomainEvent represents a significant event in the domain.
type DomainEvent interface {
	EventID() string
	EventType() EventType
	OccurredAt() time.Time
	AggregateID() string
	Version() int
}

// BaseEvent provides common fields for domain events.
type BaseEvent struct {
	ID        string    `json:"event_id"`
	Type      EventType `json:"event_type"`
	Timestamp time.Time `json:"occurred_at"`
	AggID     string    `json:"aggregate_id"`
	AggVersion int       `json:"version"`
}

func NewBaseEvent(aggID string) BaseEvent {
	return BaseEvent{
		ID:        uuid.New().String(),
		Timestamp: time.Now().UTC(),
		AggID:     aggID,
		AggVersion: 1, // Default version if not specified
	}
}

func NewBaseEventWithVersion(eventType EventType, aggID string, version int) BaseEvent {
	return BaseEvent{
		ID:         uuid.New().String(),
		Type:       eventType,
		Timestamp:  time.Now().UTC(),
		AggID:      aggID,
		AggVersion: version,
	}
}

func (e BaseEvent) EventID() string {
	return e.ID
}

func (e BaseEvent) EventType() EventType {
	return e.Type
}

func (e BaseEvent) OccurredAt() time.Time {
	return e.Timestamp
}

func (e BaseEvent) AggregateID() string {
	return e.AggID
}

func (e BaseEvent) Version() int {
	return e.AggVersion
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

// Timestamp is a time.Time alias with custom JSON serialization.
type Timestamp time.Time

// Time is an alias for time.Time for backward compatibility.
type Time = time.Time

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

// Pagination parameters for list requests and responses.
type Pagination struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total,omitempty"`
}

// PageRequest is an alias for Pagination for backward compatibility.
type PageRequest = Pagination

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

// PageResponse is a generic wrapper for paginated results.
type PageResponse[T any] struct {
	Items      []T `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

// Validate checks if pagination parameters are within valid bounds.
func (p Pagination) Validate() error {
	if p.Page < 1 {
		return fmt.Errorf("page must be >= 1")
	}
	if p.PageSize < 1 || p.PageSize > 500 {
		return fmt.Errorf("page_size must be between 1 and 500")
	}
	return nil
}

// Offset returns the SQL OFFSET value.
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
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

// Validate checks if the date range is valid.
func (dr DateRange) Validate() error {
	if time.Time(dr.From).After(time.Time(dr.To)) {
		return fmt.Errorf("invalid date range: 'from' must be before or equal to 'to'")
	}
	return nil
}

// ErrorDetail provides structured error information for API responses.
type ErrorDetail struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// APIResponse is the generic wrapper for all API responses.
type APIResponse[T any] struct {
	Success   bool         `json:"success"`
	Data      T            `json:"data,omitempty"`
	Error     *ErrorDetail `json:"error,omitempty"`
	Pagination *Pagination  `json:"pagination,omitempty"`
	RequestID string       `json:"request_id"`
	Timestamp Timestamp    `json:"timestamp"`
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
	Message string        `json:"message,omitempty"`
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

// NewPageResponse constructs a PageResponse.
func NewPageResponse[T any](items []T, total int64, req PageRequest) PageResponse[T] {
	ps := req.PageSize
	if ps <= 0 {
		ps = 20
	}
	totalPages := 0
	if ps > 0 && total > 0 {
		totalPages = int((total + int64(ps) - 1) / int64(ps))
	}
	return PageResponse[T]{
		Items:      items,
		Total:      total,
		Page:       req.Page,
		PageSize:   ps,
		TotalPages: totalPages,
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

// CachePort defines the interface for caching.
type CachePort interface {
	Get(ctx context.Context, key string, value interface{}) error
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Delete(ctx context.Context, key string) error
}

// Logger defines the interface for logging.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

// --- Search Engine Types ---

// SearchRequest defines a search query.
type SearchRequest struct {
	IndexName      string                 `json:"index_name"`
	Query          *Query                 `json:"query"`
	Filters        []Filter               `json:"filters,omitempty"`
	Sort           []SortField            `json:"sort,omitempty"`
	Pagination     *Pagination            `json:"pagination,omitempty"`
	Highlight      *HighlightConfig       `json:"highlight,omitempty"`
	Aggregations   map[string]Aggregation `json:"aggregations,omitempty"`
	SourceIncludes []string               `json:"source_includes,omitempty"`
	SourceExcludes []string               `json:"source_excludes,omitempty"`
}

// Query defines a search query structure.
type Query struct {
	QueryType          string      `json:"query_type"`
	Field              string      `json:"field,omitempty"`
	Fields             []string    `json:"fields,omitempty"`
	Value              interface{} `json:"value,omitempty"`
	Boost              float64     `json:"boost,omitempty"`
	Must               []Query     `json:"must,omitempty"`
	Should             []Query     `json:"should,omitempty"`
	MustNot            []Query     `json:"must_not,omitempty"`
	MinimumShouldMatch string      `json:"minimum_should_match,omitempty"`
}

// Filter defines a filter condition.
type Filter struct {
	Field      string      `json:"field"`
	FilterType string      `json:"filter_type"`
	Value      interface{} `json:"value,omitempty"`
	RangeFrom  interface{} `json:"range_from,omitempty"`
	RangeTo    interface{} `json:"range_to,omitempty"`
}

// HighlightConfig defines highlighting settings.
type HighlightConfig struct {
	Fields            []string `json:"fields"`
	PreTag            string   `json:"pre_tag,omitempty"`
	PostTag           string   `json:"post_tag,omitempty"`
	FragmentSize      int      `json:"fragment_size,omitempty"`
	NumberOfFragments int      `json:"number_of_fragments,omitempty"`
}

// Aggregation defines an aggregation.
type Aggregation struct {
	AggType         string                 `json:"agg_type"`
	Field           string                 `json:"field,omitempty"`
	Size            int                    `json:"size,omitempty"`
	Interval        string                 `json:"interval,omitempty"`
	Ranges          []AggRange             `json:"ranges,omitempty"`
	SubAggregations map[string]Aggregation `json:"sub_aggregations,omitempty"`
}

// AggRange defines a range for range aggregation.
type AggRange struct {
	Key  string      `json:"key"`
	From interface{} `json:"from,omitempty"`
	To   interface{} `json:"to,omitempty"`
}

// SearchResult holds the search response.
type SearchResult struct {
	Total        int64                        `json:"total"`
	MaxScore     float64                      `json:"max_score"`
	Hits         []SearchHit                  `json:"hits"`
	Aggregations map[string]AggregationResult `json:"aggregations,omitempty"`
	TookMs       int64                        `json:"took_ms"`
}

// SearchHit represents a single search hit.
type SearchHit struct {
	ID         string              `json:"id"`
	Score      float64             `json:"score"`
	Source     json.RawMessage     `json:"source"`
	Highlights map[string][]string `json:"highlights,omitempty"`
	Sort       []interface{}       `json:"sort,omitempty"`
}

// AggregationResult holds the result of an aggregation.
type AggregationResult struct {
	Buckets []AggBucket `json:"buckets"`
	Value   *float64    `json:"value,omitempty"`
}

// AggBucket represents a bucket in an aggregation result.
type AggBucket struct {
	Key             interface{}                  `json:"key"`
	KeyAsString     string                       `json:"key_as_string,omitempty"`
	DocCount        int64                        `json:"doc_count"`
	SubAggregations map[string]AggregationResult `json:"sub_aggregations,omitempty"`
}

// IndexMapping defines settings and mappings for an index.
type IndexMapping struct {
	Settings map[string]interface{} `json:"settings,omitempty"`
	Mappings map[string]interface{} `json:"mappings,omitempty"`
}

// BulkResult summarizes the result of a bulk operation.
type BulkResult struct {
	Succeeded int             `json:"succeeded"`
	Failed    int             `json:"failed"`
	Errors    []BulkItemError `json:"errors,omitempty"`
}

// BulkItemError details an error for a specific document in a bulk operation.
type BulkItemError struct {
	DocID     string `json:"doc_id"`
	ErrorType string `json:"error_type"`
	Reason    string `json:"reason"`
}

// SearchEngine defines the interface for full-text search operations.
type SearchEngine interface {
	// Indexing
	CreateIndex(ctx context.Context, indexName string, mapping IndexMapping) error
	DeleteIndex(ctx context.Context, indexName string) error
	IndexExists(ctx context.Context, indexName string) (bool, error)
	IndexDocument(ctx context.Context, indexName string, docID string, document interface{}) error
	DeleteDocument(ctx context.Context, indexName string, docID string) error
	BulkIndex(ctx context.Context, indexName string, documents map[string]interface{}) (*BulkResult, error)
	UpdateMapping(ctx context.Context, indexName string, mapping map[string]interface{}) error

	// Searching
	Search(ctx context.Context, req SearchRequest) (*SearchResult, error)
	Count(ctx context.Context, indexName string, query *Query, filters []Filter) (int64, error)
	ScrollSearch(ctx context.Context, req SearchRequest, batchHandler func(hits []SearchHit) error) error
	MultiSearch(ctx context.Context, requests []SearchRequest) ([]*SearchResult, error)
	Suggest(ctx context.Context, indexName string, field string, text string, size int) ([]string, error)
}

// --- Vector Store Types ---

// VectorSearchRequest defines a vector search query.
type VectorSearchRequest struct {
	CollectionName     string                 `json:"collection_name"`
	VectorFieldName    string                 `json:"vector_field_name"`
	Vectors            [][]float32            `json:"vectors"`
	TopK               int                    `json:"top_k"`
	MetricType         string                 `json:"metric_type,omitempty"` // simplified from entity.MetricType
	Filters            string                 `json:"filters,omitempty"`
	OutputFields       []string               `json:"output_fields,omitempty"`
	SearchParams       map[string]interface{} `json:"search_params,omitempty"`
	GuaranteeTimestamp uint64                 `json:"guarantee_timestamp,omitempty"`
}

// VectorHit represents a single search hit.
type VectorHit struct {
	ID       int64                  `json:"id"`
	Score    float32                `json:"score"`
	Distance float32                `json:"distance,omitempty"`
	Fields   map[string]interface{} `json:"fields,omitempty"`
}

// VectorSearchResult holds the search response.
type VectorSearchResult struct {
	Results [][]VectorHit `json:"results"`
	TookMs  int64         `json:"took_ms"`
}

// InsertRequest defines data to insert.
type InsertRequest struct {
	CollectionName string                   `json:"collection_name"`
	Data           []map[string]interface{} `json:"data"`
}

// InsertResult holds the insertion result.
type InsertResult struct {
	InsertedCount int64   `json:"inserted_count"`
	IDs           []int64 `json:"ids"`
}

// CollectionSchema defines a collection schema (simplified for interface).
type CollectionSchema struct {
	Name               string        `json:"name"`
	Description        string        `json:"description"`
	Fields             []interface{} `json:"fields"` // Abstraction over SDK specific fields
	EnableDynamicField bool          `json:"enable_dynamic_field"`
}

// IndexConfig defines index configuration.
type IndexConfig struct {
	FieldName  string            `json:"field_name"`
	IndexType  string            `json:"index_type"` // simplified from entity.IndexType
	MetricType string            `json:"metric_type"`
	Params     map[string]string `json:"params,omitempty"`
}

// VectorStore defines the interface for vector database operations.
type VectorStore interface {
	// Schema Management
	CreateCollection(ctx context.Context, schema CollectionSchema) error
	DropCollection(ctx context.Context, name string) error
	HasCollection(ctx context.Context, name string) (bool, error)
	// DescribeCollection? SDK specific types often returned. We skip deep introspection for interface.
	CreateIndex(ctx context.Context, collectionName string, indexCfg IndexConfig) error
	DropIndex(ctx context.Context, collectionName string, fieldName string) error
	LoadCollection(ctx context.Context, name string) error
	ReleaseCollection(ctx context.Context, name string) error
	GetLoadState(ctx context.Context, name string) (string, error)
	EnsureCollection(ctx context.Context, schema CollectionSchema, indexConfigs []IndexConfig) error

	// Data Operations
	Insert(ctx context.Context, req InsertRequest) (*InsertResult, error)
	Upsert(ctx context.Context, req InsertRequest) (*InsertResult, error)
	Delete(ctx context.Context, collectionName string, ids []int64) error

	// Search
	Search(ctx context.Context, req VectorSearchRequest) (*VectorSearchResult, error)
	SearchByID(ctx context.Context, collectionName string, vectorFieldName string, id int64, topK int, filters string, outputFields []string) ([]VectorHit, error)
	BatchSearch(ctx context.Context, requests []VectorSearchRequest) ([]*VectorSearchResult, error)
	// HybridSearch? Dependency on Reranker interface which might be implementation specific.
	// We can define it generically.
	// HybridSearch(ctx context.Context, collectionName string, requests []VectorSearchRequest, reranker Reranker, topK int) (*VectorSearchResult, error)

	// Entity Retrieval
	GetEntityByIDs(ctx context.Context, collectionName string, ids []int64, outputFields []string) ([]map[string]interface{}, error)
	GetEntityCount(ctx context.Context, collectionName string) (int64, error)
}

// --- Messaging Types ---

// ProducerMessage represents a message to be produced.
type ProducerMessage struct {
	Topic     string            `json:"topic"`
	Key       []byte            `json:"key"`
	Value     []byte            `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
	Partition int               `json:"partition"`
	Timestamp time.Time         `json:"timestamp"`
}

// Message represents a consumed message.
type Message struct {
	Topic     string            `json:"topic"`
	Partition int               `json:"partition"`
	Offset    int64             `json:"offset"`
	Key       []byte            `json:"key"`
	Value     []byte            `json:"value"`
	Headers   map[string]string `json:"headers"`
	Timestamp time.Time         `json:"timestamp"`
}

// MessageHandler handles a consumed message.
type MessageHandler func(ctx context.Context, msg *Message) error

// TopicConfig defines configuration for a topic.
type TopicConfig struct {
	Name              string            `json:"name"`
	NumPartitions     int               `json:"num_partitions"`
	ReplicationFactor int               `json:"replication_factor"`
	RetentionMs       int64             `json:"retention_ms"`
	CleanupPolicy     string            `json:"cleanup_policy"`
	MaxMessageBytes   int               `json:"max_message_bytes"`
	MinInsyncReplicas int               `json:"min_insync_replicas"`
	Configs           map[string]string `json:"configs,omitempty"`
}

// BatchPublishResult summarizes batch publish.
type BatchPublishResult struct {
	Succeeded int              `json:"succeeded"`
	Failed    int              `json:"failed"`
	Errors    []BatchItemError `json:"errors,omitempty"`
}

// BatchItemError details an error for a specific message in a batch.
type BatchItemError struct {
	Index int    `json:"index"`
	Topic string `json:"topic"`
	Error error  `json:"error"` // Note: error is interface, not JSON friendly, but fine for Go code
}

// MessageProducer defines the interface for producing messages.
type MessageProducer interface {
	Publish(ctx context.Context, msg *ProducerMessage) error
	PublishBatch(ctx context.Context, msgs []*ProducerMessage) (*BatchPublishResult, error)
	PublishAsync(ctx context.Context, msg *ProducerMessage)
	Close() error
}

// MessageConsumer defines the interface for consuming messages.
type MessageConsumer interface {
	Subscribe(topic string, handler MessageHandler) error
	Unsubscribe(topic string) error
	Start(ctx context.Context) error
	Close() error
}
