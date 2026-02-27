// ---
// 继续输出 212 `internal/application/lifecycle/legal_status.go` 要实现专利法律状态管理应用服务。
//
// 实现要求:
//
// * 功能定位：专利法律状态全生命周期管理的业务编排层，协调多法域专利局数据同步、状态变更追踪、异常检测与通知触发
// * 核心实现：
//   - 定义 LegalStatusService 接口（SyncStatus / BatchSync / GetCurrentStatus / GetStatusHistory / DetectAnomalies / SubscribeStatusChange / UnsubscribeStatusChange / GetStatusSummary / ReconcileStatus）
//   - 实现 legalStatusServiceImpl 结构体，注入 LifecycleDomainService / LifecycleRepository / PatentRepository / EventPublisher / Cache / Logger / Metrics
//   - 定义全部请求/响应 DTO 与枚举（LegalStatusCode / AnomalyType / SeverityLevel / NotificationChannel）
//   - 方法实现流程：参数校验 -> 领域服务调用 -> 持久化 -> 事件发布 -> 缓存管理 -> 指标记录
// * 业务逻辑：
//   - 多法域状态码统一映射（CNIPA/USPTO/EPO/JPO/KIPO -> 内部枚举）
//   - 增量同步为主，Force=true 全量同步
//   - BatchSync 信号量并发控制（默认10）
//   - 缓存策略：当前状态TTL=1h，汇总TTL=15min，变更时主动失效
//   - 异常检测规则：UnexpectedLapse / MissedDeadline / StatusConflict / SyncFailure
//   - 健康度评分：1.0 - (critical*0.3 + high*0.15 + medium*0.05) / total，下限0
//   - 通知去重：同一专利同一变更24h内不重复
// * 依赖关系：
//   - 依赖：domain/lifecycle (service, repository, entity), domain/patent (repository), infrastructure/messaging/kafka (producer), infrastructure/database/redis (cache), infrastructure/monitoring (logger, metrics), pkg/errors, pkg/types/common
//   - 被依赖：interfaces/http/handlers/lifecycle_handler, interfaces/cli/lifecycle, application/reporting/fto_report
// * 测试要求：Mock全部注入依赖，覆盖同步/批量/缓存/异常检测/健康度/通知去重/对账全路径
// * 强制约束：文件最后一行必须为 `//Personal.AI order the ending`
// ---
package lifecycle

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

// LegalStatusCode represents the unified internal legal status of a patent,
// mapped from heterogeneous patent office status codes (CNIPA, USPTO, EPO, JPO, KIPO).
type LegalStatusCode string

const (
	StatusFiled           LegalStatusCode = "FILED"
	StatusPublished       LegalStatusCode = "PUBLISHED"
	StatusUnderExam       LegalStatusCode = "UNDER_EXAMINATION"
	StatusGranted         LegalStatusCode = "GRANTED"
	StatusLapsed          LegalStatusCode = "LAPSED"
	StatusWithdrawn       LegalStatusCode = "WITHDRAWN"
	StatusRejected        LegalStatusCode = "REJECTED"
	StatusExpired         LegalStatusCode = "EXPIRED"
	StatusRevoked         LegalStatusCode = "REVOKED"
	StatusUnderAppeal     LegalStatusCode = "UNDER_APPEAL"
	StatusTransferred     LegalStatusCode = "TRANSFERRED"
	StatusLicenseRecorded LegalStatusCode = "LICENSE_RECORDED"
)

// String returns the string representation of LegalStatusCode.
func (c LegalStatusCode) String() string { return string(c) }

// IsTerminal returns true if the status represents a terminal state where no
// further prosecution activity is expected.
func (c LegalStatusCode) IsTerminal() bool {
	switch c {
	case StatusLapsed, StatusWithdrawn, StatusRejected, StatusExpired, StatusRevoked:
		return true
	default:
		return false
	}
}

// IsActive returns true if the patent is in an active prosecution or maintenance state.
func (c LegalStatusCode) IsActive() bool {
	switch c {
	case StatusFiled, StatusPublished, StatusUnderExam, StatusGranted, StatusUnderAppeal, StatusTransferred, StatusLicenseRecorded:
		return true
	default:
		return false
	}
}

// ValidLegalStatusCodes is the canonical set of all valid status codes.
var ValidLegalStatusCodes = []LegalStatusCode{
	StatusFiled, StatusPublished, StatusUnderExam, StatusGranted,
	StatusLapsed, StatusWithdrawn, StatusRejected, StatusExpired,
	StatusRevoked, StatusUnderAppeal, StatusTransferred, StatusLicenseRecorded,
}

// AnomalyType classifies the kind of anomaly detected during status monitoring.
type AnomalyType string

const (
	AnomalyUnexpectedLapse  AnomalyType = "UNEXPECTED_LAPSE"
	AnomalyMissedDeadline   AnomalyType = "MISSED_DEADLINE"
	AnomalyStatusConflict   AnomalyType = "STATUS_CONFLICT"
	AnomalySyncFailure      AnomalyType = "SYNC_FAILURE"
	AnomalyUnauthorizedChg  AnomalyType = "UNAUTHORIZED_CHANGE"
)

func (a AnomalyType) String() string { return string(a) }

// SeverityLevel indicates the urgency of an anomaly or notification.
type SeverityLevel string

const (
	SeverityCritical SeverityLevel = "CRITICAL"
	SeverityHigh     SeverityLevel = "HIGH"
	SeverityMedium   SeverityLevel = "MEDIUM"
	SeverityLow      SeverityLevel = "LOW"
	SeverityInfo     SeverityLevel = "INFO"
)

func (s SeverityLevel) String() string { return string(s) }

// SeverityWeight returns the numeric weight used in health-score calculation.
func (s SeverityLevel) SeverityWeight() float64 {
	switch s {
	case SeverityCritical:
		return 0.30
	case SeverityHigh:
		return 0.15
	case SeverityMedium:
		return 0.05
	default:
		return 0.0
	}
}

// NotificationChannel enumerates supported alert delivery channels.
type NotificationChannel string

const (
	ChannelEmail      NotificationChannel = "EMAIL"
	ChannelWeChatWork NotificationChannel = "WECHAT_WORK"
	ChannelDingTalk   NotificationChannel = "DINGTALK"
	ChannelSMS        NotificationChannel = "SMS"
	ChannelInApp      NotificationChannel = "IN_APP"
	ChannelWebhook    NotificationChannel = "WEBHOOK"
)

func (n NotificationChannel) String() string { return string(n) }

// ---------------------------------------------------------------------------
// DTOs — Request / Response
// ---------------------------------------------------------------------------

// BatchSyncRequest carries parameters for a bulk legal-status synchronisation job.
type BatchSyncRequest struct {
	PatentIDs     []string `json:"patent_ids" validate:"required,min=1,max=500"`
	Jurisdictions []string `json:"jurisdictions,omitempty"`
	Force         bool     `json:"force"`
}

// Validate performs structural validation on the request.
func (r *BatchSyncRequest) Validate() error {
	if len(r.PatentIDs) == 0 {
		return errors.NewValidationOp("batch_sync", "patent_ids must not be empty")
	}
	if len(r.PatentIDs) > 500 {
		return errors.NewValidationOp("batch_sync", "patent_ids must not exceed 500 entries per batch")
	}
	seen := make(map[string]struct{}, len(r.PatentIDs))
	for _, id := range r.PatentIDs {
		if id == "" {
			return errors.NewValidationOp("batch_sync", "patent_ids must not contain empty strings")
		}
		if _, dup := seen[id]; dup {
			return errors.NewValidationOp("batch_sync", fmt.Sprintf("duplicate patent_id: %s", id))
		}
		seen[id] = struct{}{}
	}
	return nil
}

// SyncError records a per-patent synchronisation failure.
type SyncError struct {
	PatentID string `json:"patent_id"`
	Error    string `json:"error"`
}

// BatchSyncResult aggregates the outcome of a batch synchronisation.
type BatchSyncResult struct {
	Succeeded int           `json:"succeeded"`
	Failed    int           `json:"failed"`
	Errors    []SyncError   `json:"errors,omitempty"`
	Duration  time.Duration `json:"duration_ms"`
}

// SyncResult describes the outcome of synchronising a single patent's legal status.
type SyncResult struct {
	PatentID       string    `json:"patent_id"`
	PreviousStatus string    `json:"previous_status"`
	CurrentStatus  string    `json:"current_status"`
	Changed        bool      `json:"changed"`
	SyncedAt       time.Time `json:"synced_at"`
	Source         string    `json:"source"`
}

// LegalStatusDetail provides the full current legal status of a patent.
type LegalStatusDetail struct {
	PatentID      string                 `json:"patent_id"`
	Jurisdiction  string                 `json:"jurisdiction"`
	Status        LegalStatusCode        `json:"status"`
	StatusText    string                 `json:"status_text"`
	EffectiveDate time.Time              `json:"effective_date"`
	NextAction    string                 `json:"next_action,omitempty"`
	NextDeadline  *time.Time             `json:"next_deadline,omitempty"`
	RawData       map[string]interface{} `json:"raw_data,omitempty"`
}

// LegalStatusEvent represents a single state transition in a patent's legal history.
type LegalStatusEvent struct {
	EventID     string    `json:"event_id"`
	PatentID    string    `json:"patent_id"`
	FromStatus  string    `json:"from_status"`
	ToStatus    string    `json:"to_status"`
	EventDate   time.Time `json:"event_date"`
	Source      string    `json:"source"`
	Description string    `json:"description"`
}

// StatusAnomaly describes a detected irregularity in a patent's legal status.
type StatusAnomaly struct {
	PatentID        string        `json:"patent_id"`
	AnomalyType     AnomalyType   `json:"anomaly_type"`
	Severity        SeverityLevel `json:"severity"`
	Description     string        `json:"description"`
	DetectedAt      time.Time     `json:"detected_at"`
	SuggestedAction string        `json:"suggested_action"`
}

// SubscriptionRequest defines the parameters for subscribing to status-change notifications.
type SubscriptionRequest struct {
	PatentIDs     []string              `json:"patent_ids,omitempty"`
	PortfolioID   string                `json:"portfolio_id,omitempty"`
	StatusFilters []string              `json:"status_filters,omitempty"`
	Channels      []NotificationChannel `json:"channels" validate:"required,min=1"`
	Recipient     string                `json:"recipient" validate:"required"`
}

// Validate checks the subscription request for correctness.
func (r *SubscriptionRequest) Validate() error {
	if len(r.PatentIDs) == 0 && r.PortfolioID == "" {
		return errors.NewValidationOp("subscription", "either patent_ids or portfolio_id must be specified")
	}
	if len(r.Channels) == 0 {
		return errors.NewValidationOp("subscription", "at least one notification channel is required")
	}
	if r.Recipient == "" {
		return errors.NewValidationOp("subscription", "recipient must not be empty")
	}
	return nil
}

// Subscription represents an active status-change notification subscription.
type Subscription struct {
	ID        string    `json:"id"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	Filters   []string  `json:"filters,omitempty"`
}

// StatusSummary provides a portfolio-level aggregation of legal statuses.
type StatusSummary struct {
	PortfolioID    string         `json:"portfolio_id"`
	TotalPatents   int            `json:"total_patents"`
	ByStatus       map[string]int `json:"by_status"`
	ByJurisdiction map[string]int `json:"by_jurisdiction"`
	AnomalyCount   int            `json:"anomaly_count"`
	LastSyncAt     time.Time      `json:"last_sync_at"`
	HealthScore    float64        `json:"health_score"`
}

// Discrepancy records a single field-level mismatch between local and remote state.
type Discrepancy struct {
	Field      string `json:"field"`
	LocalValue string `json:"local_value"`
	RemoteValue string `json:"remote_value"`
	Resolution string `json:"resolution,omitempty"`
}

// ReconcileResult describes the outcome of reconciling local vs remote legal status.
type ReconcileResult struct {
	PatentID      string        `json:"patent_id"`
	Consistent    bool          `json:"consistent"`
	LocalStatus   string        `json:"local_status"`
	RemoteStatus  string        `json:"remote_status"`
	Discrepancies []Discrepancy `json:"discrepancies,omitempty"`
	ReconciledAt  time.Time     `json:"reconciled_at"`
}

// QueryOption is a functional option for parameterising history queries.
type QueryOption func(*queryOptions)

type queryOptions struct {
	Pagination *commontypes.Pagination
	From       *time.Time
	To         *time.Time
}

func defaultQueryOptions() *queryOptions {
	return &queryOptions{}
}

func applyQueryOptions(opts []QueryOption) *queryOptions {
	o := defaultQueryOptions()
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// WithPagination sets pagination parameters on a history query.
func WithPagination(p commontypes.Pagination) QueryOption {
	return func(o *queryOptions) { o.Pagination = &p }
}

// WithTimeRange restricts the history query to a specific time window.
func WithTimeRange(from, to time.Time) QueryOption {
	return func(o *queryOptions) { o.From = &from; o.To = &to }
}

// ---------------------------------------------------------------------------
// Port interfaces — abstractions over infrastructure dependencies
// ---------------------------------------------------------------------------

// EventPublisher abstracts asynchronous event emission (e.g. Kafka producer).
type EventPublisher interface {
	Publish(ctx context.Context, topic string, key string, payload interface{}) error
}

// Metrics abstracts metric recording (counters, histograms).
type Metrics interface {
	IncCounter(name string, labels map[string]string)
	ObserveHistogram(name string, value float64, labels map[string]string)
}

// ---------------------------------------------------------------------------
// Service interface
// ---------------------------------------------------------------------------

// LegalStatusService defines the application-layer contract for patent legal
// status management. It orchestrates domain services, repositories, caching,
// event publishing, and observability concerns.
type LegalStatusService interface {
	// SyncStatus synchronises the legal status of a single patent from the
	// authoritative patent office source and persists any detected changes.
	SyncStatus(ctx context.Context, patentID string) (*SyncResult, error)

	// BatchSync synchronises legal statuses for multiple patents concurrently,
	// respecting a configurable concurrency limit.
	BatchSync(ctx context.Context, req *BatchSyncRequest) (*BatchSyncResult, error)

	// GetCurrentStatus returns the most recent legal status for a patent,
	// served from cache when available.
	GetCurrentStatus(ctx context.Context, patentID string) (*LegalStatusDetail, error)

	// GetStatusHistory returns the chronological list of status transitions
	// for a patent, supporting pagination and time-range filtering.
	GetStatusHistory(ctx context.Context, patentID string, opts ...QueryOption) ([]*LegalStatusEvent, error)

	// DetectAnomalies scans all patents within a portfolio for status
	// irregularities and returns them sorted by severity (descending).
	DetectAnomalies(ctx context.Context, portfolioID string) ([]*StatusAnomaly, error)

	// SubscribeStatusChange creates a notification subscription for legal
	// status changes matching the given filters.
	SubscribeStatusChange(ctx context.Context, req *SubscriptionRequest) (*Subscription, error)

	// UnsubscribeStatusChange deactivates an existing notification subscription.
	UnsubscribeStatusChange(ctx context.Context, subscriptionID string) error

	// GetStatusSummary returns an aggregated view of legal statuses across
	// all patents in a portfolio, including a computed health score.
	GetStatusSummary(ctx context.Context, portfolioID string) (*StatusSummary, error)

	// ReconcileStatus compares the locally stored legal status against the
	// remote patent office record and reports any discrepancies.
	ReconcileStatus(ctx context.Context, patentID string) (*ReconcileResult, error)
}

// Service is an alias for LegalStatusService for backward compatibility with apiserver.
type Service = LegalStatusService

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

// LegalStatusConfig holds tuneable parameters for the legal status service.
type LegalStatusConfig struct {
	// MaxBatchConcurrency limits the number of concurrent sync operations
	// within a single BatchSync call. Default: 10.
	MaxBatchConcurrency int

	// StatusCacheTTL is the time-to-live for individual patent status cache
	// entries. Default: 1 hour.
	StatusCacheTTL time.Duration

	// SummaryCacheTTL is the time-to-live for portfolio summary cache entries.
	// Default: 15 minutes.
	SummaryCacheTTL time.Duration

	// NotificationDedupeWindow is the minimum interval between duplicate
	// notifications for the same patent + status change. Default: 24 hours.
	NotificationDedupeWindow time.Duration

	// SyncFailureThreshold is the number of consecutive sync failures before
	// a SyncFailure anomaly is raised. Default: 3.
	SyncFailureThreshold int
}

// DefaultLegalStatusConfig returns production-ready default configuration.
func DefaultLegalStatusConfig() LegalStatusConfig {
	return LegalStatusConfig{
		MaxBatchConcurrency:      10,
		StatusCacheTTL:           1 * time.Hour,
		SummaryCacheTTL:          15 * time.Minute,
		NotificationDedupeWindow: 24 * time.Hour,
		SyncFailureThreshold:     3,
	}
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

// legalStatusServiceImpl is the concrete implementation of LegalStatusService.
type legalStatusServiceImpl struct {
	lifecycleSvc  lifecycle.Service
	lifecycleRepo lifecycle.LifecycleRepository
	patentRepo    patent.PatentRepository
	publisher     EventPublisher
	cache         common.CachePort
	logger        common.Logger
	metrics       Metrics
	config        LegalStatusConfig
}

// NewLegalStatusService constructs a new LegalStatusService with all required
// dependencies. It returns an error if any mandatory dependency is nil.
func NewLegalStatusService(
	lifecycleSvc lifecycle.Service,
	lifecycleRepo lifecycle.LifecycleRepository,
	patentRepo patent.PatentRepository,
	publisher EventPublisher,
	cache common.CachePort,
	logger common.Logger,
	metrics Metrics,
	cfg *LegalStatusConfig,
) (LegalStatusService, error) {
	if lifecycleSvc == nil {
		return nil, errors.NewInternalOp("legal_status.new", "lifecycle domain service must not be nil")
	}
	if lifecycleRepo == nil {
		return nil, errors.NewInternalOp("legal_status.new", "lifecycle repository must not be nil")
	}
	if patentRepo == nil {
		return nil, errors.NewInternalOp("legal_status.new", "patent repository must not be nil")
	}
	if publisher == nil {
		return nil, errors.NewInternalOp("legal_status.new", "event publisher must not be nil")
	}
	if cache == nil {
		return nil, errors.NewInternalOp("legal_status.new", "cache must not be nil")
	}
	if logger == nil {
		return nil, errors.NewInternalOp("legal_status.new", "logger must not be nil")
	}
	if metrics == nil {
		return nil, errors.NewInternalOp("legal_status.new", "metrics must not be nil")
	}

	c := DefaultLegalStatusConfig()
	if cfg != nil {
		if cfg.MaxBatchConcurrency > 0 {
			c.MaxBatchConcurrency = cfg.MaxBatchConcurrency
		}
		if cfg.StatusCacheTTL > 0 {
			c.StatusCacheTTL = cfg.StatusCacheTTL
		}
		if cfg.SummaryCacheTTL > 0 {
			c.SummaryCacheTTL = cfg.SummaryCacheTTL
		}
		if cfg.NotificationDedupeWindow > 0 {
			c.NotificationDedupeWindow = cfg.NotificationDedupeWindow
		}
		if cfg.SyncFailureThreshold > 0 {
			c.SyncFailureThreshold = cfg.SyncFailureThreshold
		}
	}

	return &legalStatusServiceImpl{
		lifecycleSvc:  lifecycleSvc,
		lifecycleRepo: lifecycleRepo,
		patentRepo:    patentRepo,
		publisher:     publisher,
		cache:         cache,
		logger:        logger,
		metrics:       metrics,
		config:        c,
	}, nil
}

// ---------------------------------------------------------------------------
// Cache key helpers
// ---------------------------------------------------------------------------

func statusCacheKey(patentID string) string {
	return fmt.Sprintf("legal_status:current:%s", patentID)
}

func summaryCacheKey(portfolioID string) string {
	return fmt.Sprintf("legal_status:summary:%s", portfolioID)
}

func notificationDedupeKey(patentID, toStatus string) string {
	return fmt.Sprintf("legal_status:notify_dedupe:%s:%s", patentID, toStatus)
}

// ---------------------------------------------------------------------------
// Jurisdiction status code mapping
// ---------------------------------------------------------------------------

// jurisdictionStatusMap maps patent-office-specific status strings to the
// unified internal LegalStatusCode. Each top-level key is a jurisdiction code.
var jurisdictionStatusMap = map[string]map[string]LegalStatusCode{
	"CN": {
		"申请":   StatusFiled,
		"公开":   StatusPublished,
		"实质审查": StatusUnderExam,
		"授权":   StatusGranted,
		"失效":   StatusLapsed,
		"撤回":   StatusWithdrawn,
		"驳回":   StatusRejected,
		"届满":   StatusExpired,
		"无效":   StatusRevoked,
		"复审":   StatusUnderAppeal,
		"转让":   StatusTransferred,
		"许可备案": StatusLicenseRecorded,
	},
	"US": {
		"FILED":             StatusFiled,
		"PUBLISHED":         StatusPublished,
		"UNDER_EXAMINATION": StatusUnderExam,
		"PATENTED":          StatusGranted,
		"LAPSED":            StatusLapsed,
		"WITHDRAWN":         StatusWithdrawn,
		"ABANDONED":         StatusWithdrawn,
		"REJECTED":          StatusRejected,
		"EXPIRED":           StatusExpired,
		"REVOKED":           StatusRevoked,
		"ON_APPEAL":         StatusUnderAppeal,
		"REASSIGNED":        StatusTransferred,
		"LICENSE_RECORDED":  StatusLicenseRecorded,
	},
	"EP": {
		"FILING":        StatusFiled,
		"PUBLICATION":   StatusPublished,
		"EXAMINATION":   StatusUnderExam,
		"GRANT":         StatusGranted,
		"LAPSE":         StatusLapsed,
		"WITHDRAWAL":    StatusWithdrawn,
		"REFUSAL":       StatusRejected,
		"EXPIRY":        StatusExpired,
		"REVOCATION":    StatusRevoked,
		"APPEAL":        StatusUnderAppeal,
		"TRANSFER":      StatusTransferred,
		"LICENCE":       StatusLicenseRecorded,
	},
	"JP": {
		"出願":   StatusFiled,
		"公開":   StatusPublished,
		"審査中":  StatusUnderExam,
		"登録":   StatusGranted,
		"消滅":   StatusLapsed,
		"取下":   StatusWithdrawn,
		"拒絶":   StatusRejected,
		"満了":   StatusExpired,
		"無効":   StatusRevoked,
		"審判":   StatusUnderAppeal,
		"移転":   StatusTransferred,
		"実施権": StatusLicenseRecorded,
	},
	"KR": {
		"출원":   StatusFiled,
		"공개":   StatusPublished,
		"심사중":  StatusUnderExam,
		"등록":   StatusGranted,
		"소멸":   StatusLapsed,
		"취하":   StatusWithdrawn,
		"거절":   StatusRejected,
		"만료":   StatusExpired,
		"무효":   StatusRevoked,
		"심판":   StatusUnderAppeal,
		"이전":   StatusTransferred,
		"실시권": StatusLicenseRecorded,
	},
}

// MapJurisdictionStatus translates a jurisdiction-specific status string into
// the unified internal LegalStatusCode. Returns StatusFiled as fallback when
// no mapping is found, along with a boolean indicating whether the mapping
// was exact.
func MapJurisdictionStatus(jurisdiction, rawStatus string) (LegalStatusCode, bool) {
	if jMap, ok := jurisdictionStatusMap[jurisdiction]; ok {
		if code, found := jMap[rawStatus]; found {
			return code, true
		}
	}
	return StatusFiled, false
}

// ---------------------------------------------------------------------------
// SyncStatus
// ---------------------------------------------------------------------------

func (s *legalStatusServiceImpl) SyncStatus(ctx context.Context, patentID string) (*SyncResult, error) {
	if patentID == "" {
		return nil, errors.NewValidationOp("sync_status", "patent_id must not be empty")
	}

	start := time.Now()
	s.logger.Debug("sync_status started", "patent_id", patentID)

	// 1. Retrieve local current status from repository.
	// patentID here is UUID string from request
	localEntity, err := s.lifecycleRepo.GetByPatentID(ctx, patentID)
	if err != nil {
		s.logger.Error("failed to get local status", "patent_id", patentID, "error", err)
		s.metrics.IncCounter("legal_status_sync_errors_total", map[string]string{"patent_id": patentID, "stage": "local_fetch"})
		return nil, fmt.Errorf("fetch local status for %s: %w", patentID, err)
	}

	previousStatus := ""
	if localEntity != nil {
		previousStatus = localEntity.Status
	}

	// 2. Invoke domain service to fetch authoritative remote status.
	remoteStatus, err := s.lifecycleSvc.FetchRemoteStatus(ctx, patentID)
	if err != nil {
		s.logger.Error("failed to fetch remote status", "patent_id", patentID, "error", err)
		s.metrics.IncCounter("legal_status_sync_errors_total", map[string]string{"patent_id": patentID, "stage": "remote_fetch"})
		return nil, fmt.Errorf("fetch remote status for %s: %w", patentID, err)
	}

	currentStatus := remoteStatus.Status
	changed := previousStatus != currentStatus

	// 3. Persist changes when a transition is detected.
	if changed {
		if err := s.lifecycleRepo.UpdateStatus(ctx, patentID, currentStatus, remoteStatus.EffectiveDate); err != nil {
			s.logger.Error("failed to persist status change", "patent_id", patentID, "error", err)
			s.metrics.IncCounter("legal_status_sync_errors_total", map[string]string{"patent_id": patentID, "stage": "persist"})
			return nil, fmt.Errorf("persist status change for %s: %w", patentID, err)
		}

		// 4. Publish domain event.
		event := map[string]interface{}{
			"patent_id":       patentID,
			"previous_status": previousStatus,
			"current_status":  currentStatus,
			"changed_at":      time.Now().UTC(),
			"source":          remoteStatus.Source,
		}
		if pubErr := s.publisher.Publish(ctx, "legal_status.changed", patentID, event); pubErr != nil {
			// Non-fatal: log and continue.
			s.logger.Warn("failed to publish status change event", "patent_id", patentID, "error", pubErr)
		}

		// 5. Invalidate cache.
		_ = s.cache.Delete(ctx, statusCacheKey(patentID))

		s.metrics.IncCounter("legal_status_changes_total", map[string]string{
			"patent_id":   patentID,
			"from_status": previousStatus,
			"to_status":   currentStatus,
		})
	}

	elapsed := time.Since(start)
	s.metrics.ObserveHistogram("legal_status_sync_duration_seconds", elapsed.Seconds(), map[string]string{"patent_id": patentID})
	s.metrics.IncCounter("legal_status_syncs_total", map[string]string{"patent_id": patentID, "changed": fmt.Sprintf("%t", changed)})

	s.logger.Info("sync_status completed", "patent_id", patentID, "changed", changed, "duration", elapsed)

	return &SyncResult{
		PatentID:       patentID,
		PreviousStatus: previousStatus,
		CurrentStatus:  currentStatus,
		Changed:        changed,
		SyncedAt:       time.Now().UTC(),
		Source:         remoteStatus.Source,
	}, nil
}

// ---------------------------------------------------------------------------
// BatchSync
// ---------------------------------------------------------------------------

func (s *legalStatusServiceImpl) BatchSync(ctx context.Context, req *BatchSyncRequest) (*BatchSyncResult, error) {
	if req == nil {
		return nil, errors.NewValidationOp("batch_sync", "batch sync request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	start := time.Now()
	concurrency := s.config.MaxBatchConcurrency
	s.logger.Info("batch_sync started", "count", len(req.PatentIDs), "concurrency", concurrency, "force", req.Force)

	// Semaphore for concurrency control.
	sem := make(chan struct{}, concurrency)

	var (
		mu          sync.Mutex
		succeeded   int32
		failed      int32
		syncErrors  []SyncError
	)

	var wg sync.WaitGroup
	for _, pid := range req.PatentIDs {
		wg.Add(1)
		go func(patentID string) {
			defer wg.Done()

			// Acquire semaphore slot.
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				syncErrors = append(syncErrors, SyncError{PatentID: patentID, Error: ctx.Err().Error()})
				mu.Unlock()
				atomic.AddInt32(&failed, 1)
				return
			}

			_, err := s.SyncStatus(ctx, patentID)
			if err != nil {
				mu.Lock()
				syncErrors = append(syncErrors, SyncError{PatentID: patentID, Error: err.Error()})
				mu.Unlock()
				atomic.AddInt32(&failed, 1)
				return
			}
			atomic.AddInt32(&succeeded, 1)
		}(pid)
	}
	wg.Wait()

	elapsed := time.Since(start)
	result := &BatchSyncResult{
		Succeeded: int(atomic.LoadInt32(&succeeded)),
		Failed:    int(atomic.LoadInt32(&failed)),
		Errors:    syncErrors,
		Duration:  elapsed,
	}

	s.metrics.ObserveHistogram("legal_status_batch_sync_duration_seconds", elapsed.Seconds(), map[string]string{
		"total": fmt.Sprintf("%d", len(req.PatentIDs)),
	})
	s.logger.Info("batch_sync completed", "succeeded", result.Succeeded, "failed", result.Failed, "duration", elapsed)

	return result, nil
}

// ---------------------------------------------------------------------------
// GetCurrentStatus
// ---------------------------------------------------------------------------

func (s *legalStatusServiceImpl) GetCurrentStatus(ctx context.Context, patentID string) (*LegalStatusDetail, error) {
	if patentID == "" {
		return nil, errors.NewValidationOp("get_current_status", "patent_id must not be empty")
	}

	// 1. Try cache.
	cacheKey := statusCacheKey(patentID)
	var cached LegalStatusDetail
	if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
		s.metrics.IncCounter("legal_status_cache_hits_total", map[string]string{"patent_id": patentID})
		return &cached, nil
	}
	s.metrics.IncCounter("legal_status_cache_misses_total", map[string]string{"patent_id": patentID})

	// 2. Fetch from repository.
	entity, err := s.lifecycleRepo.GetByPatentID(ctx, patentID)
	if err != nil {
		return nil, fmt.Errorf("get current status for %s: %w", patentID, err)
	}
	if entity == nil {
		return nil, errors.NewNotFoundOp("get_current_status", fmt.Sprintf("legal status not found for patent %s", patentID))
	}

	mappedCode, _ := MapJurisdictionStatus(entity.Jurisdiction, entity.Status)

	detail := &LegalStatusDetail{
		PatentID:      entity.PatentID,
		Jurisdiction:  entity.Jurisdiction,
		Status:        mappedCode,
		StatusText:    entity.Status,
		EffectiveDate: entity.EffectiveDate,
		NextAction:    entity.NextAction,
		NextDeadline:  entity.NextDeadline,
		RawData:       entity.RawData,
	}

	// 3. Backfill cache.
	if setErr := s.cache.Set(ctx, cacheKey, detail, s.config.StatusCacheTTL); setErr != nil {
		s.logger.Warn("failed to set status cache", "patent_id", patentID, "error", setErr)
	}

	return detail, nil
}

// ---------------------------------------------------------------------------
// GetStatusHistory
// ---------------------------------------------------------------------------

func (s *legalStatusServiceImpl) GetStatusHistory(ctx context.Context, patentID string, opts ...QueryOption) ([]*LegalStatusEvent, error) {
	if patentID == "" {
		return nil, errors.NewValidationOp("get_status_history", "patent_id must not be empty")
	}

	qo := applyQueryOptions(opts)

	entities, err := s.lifecycleRepo.GetStatusHistory(ctx, patentID, qo.Pagination, qo.From, qo.To)
	if err != nil {
		return nil, fmt.Errorf("get status history for %s: %w", patentID, err)
	}

	events := make([]*LegalStatusEvent, 0, len(entities))
	for _, e := range entities {
		events = append(events, &LegalStatusEvent{
			EventID:     e.EventID,
			PatentID:    e.PatentID,
			FromStatus:  e.FromStatus,
			ToStatus:    e.ToStatus,
			EventDate:   e.EventDate,
			Source:      e.Source,
			Description: e.Description,
		})
	}

	return events, nil
}

// ---------------------------------------------------------------------------
// DetectAnomalies
// ---------------------------------------------------------------------------

func (s *legalStatusServiceImpl) DetectAnomalies(ctx context.Context, portfolioID string) ([]*StatusAnomaly, error) {
	if portfolioID == "" {
		return nil, errors.NewValidationOp("detect_anomalies", "portfolio_id must not be empty")
	}

	s.logger.Debug("detect_anomalies started", "portfolio_id", portfolioID)

	// Load all patents in the portfolio.
	patents, err := s.patentRepo.ListByPortfolio(ctx, portfolioID)
	if err != nil {
		return nil, fmt.Errorf("list patents for portfolio %s: %w", portfolioID, err)
	}

	now := time.Now().UTC()
	var anomalies []*StatusAnomaly

	for _, p := range patents {
		entity, err := s.lifecycleRepo.GetByPatentID(ctx, p.ID.String())
		if err != nil {
			s.logger.Warn("failed to get status for anomaly detection", "patent_id", p.ID.String(), "error", err)
			continue
		}
		if entity == nil {
			continue
		}

		mappedCode, _ := MapJurisdictionStatus(entity.Jurisdiction, entity.Status)

		// Rule 1: UnexpectedLapse — granted patent became lapsed without explicit abandonment.
		if mappedCode == StatusLapsed && entity.PreviousStatus != "" {
			prevCode, _ := MapJurisdictionStatus(entity.Jurisdiction, entity.PreviousStatus)
			if prevCode == StatusGranted {
				anomalies = append(anomalies, &StatusAnomaly{
					PatentID:        p.ID.String(),
					AnomalyType:     AnomalyUnexpectedLapse,
					Severity:        SeverityCritical,
					Description:     fmt.Sprintf("Patent %s lapsed unexpectedly from granted status", p.ID.String()),
					DetectedAt:      now,
					SuggestedAction: "Verify annuity payment status and contact patent office immediately",
				})
			}
		}

		// Rule 2: MissedDeadline — deadline within 7 days and no action recorded.
		if entity.NextDeadline != nil && !entity.NextDeadline.IsZero() {
			daysUntilDeadline := entity.NextDeadline.Sub(now).Hours() / 24
			if daysUntilDeadline <= 7 && daysUntilDeadline > 0 {
				anomalies = append(anomalies, &StatusAnomaly{
					PatentID:        p.ID.String(),
					AnomalyType:     AnomalyMissedDeadline,
					Severity:        SeverityHigh,
					Description:     fmt.Sprintf("Patent %s has a deadline in %.0f days (%s) with no action recorded", p.ID.String(), daysUntilDeadline, entity.NextAction),
					DetectedAt:      now,
					SuggestedAction: fmt.Sprintf("Take action on '%s' before %s", entity.NextAction, entity.NextDeadline.Format(time.RFC3339)),
				})
			} else if daysUntilDeadline <= 0 {
				anomalies = append(anomalies, &StatusAnomaly{
					PatentID:        p.ID.String(),
					AnomalyType:     AnomalyMissedDeadline,
					Severity:        SeverityCritical,
					Description:     fmt.Sprintf("Patent %s has a past-due deadline: %s", p.ID.String(), entity.NextDeadline.Format(time.RFC3339)),
					DetectedAt:      now,
					SuggestedAction: "Immediately assess whether late action or petition for revival is possible",
				})
			}
		}

		// Rule 3: StatusConflict — local vs remote inconsistency persisting > 24h.
		if entity.LastSyncAt != nil && entity.RemoteStatus != "" {
			if entity.Status != entity.RemoteStatus && now.Sub(*entity.LastSyncAt).Hours() > 24 {
				anomalies = append(anomalies, &StatusAnomaly{
					PatentID:        p.ID.String(),
					AnomalyType:     AnomalyStatusConflict,
					Severity:        SeverityHigh,
					Description:     fmt.Sprintf("Patent %s local status '%s' conflicts with remote '%s' for over 24 hours", p.ID.String(), entity.Status, entity.RemoteStatus),
					DetectedAt:      now,
					SuggestedAction: "Run reconciliation to resolve the conflict",
				})
			}
		}

		// Rule 4: SyncFailure — consecutive failures exceeding threshold.
		if entity.ConsecutiveSyncFailures >= s.config.SyncFailureThreshold {
			anomalies = append(anomalies, &StatusAnomaly{
				PatentID:        p.ID.String(),
				AnomalyType:     AnomalySyncFailure,
				Severity:        SeverityMedium,
				Description:     fmt.Sprintf("Patent %s has %d consecutive sync failures", p.ID.String(), entity.ConsecutiveSyncFailures),
				DetectedAt:      now,
				SuggestedAction: "Check network connectivity and patent office API availability",
			})
		}
	}

	// Sort by severity: Critical > High > Medium > Low > Info.
	sortAnomaliesBySeverity(anomalies)

	s.logger.Info("detect_anomalies completed", "portfolio_id", portfolioID, "anomaly_count", len(anomalies))
	s.metrics.IncCounter("legal_status_anomalies_detected_total", map[string]string{
		"portfolio_id": portfolioID,
		"count":        fmt.Sprintf("%d", len(anomalies)),
	})

	return anomalies, nil
}

// severityOrder maps severity levels to sort priority (lower = more severe).
var severityOrder = map[SeverityLevel]int{
	SeverityCritical: 0,
	SeverityHigh:     1,
	SeverityMedium:   2,
	SeverityLow:      3,
	SeverityInfo:     4,
}

// sortAnomaliesBySeverity sorts anomalies in-place by severity descending
// (critical first). Uses insertion sort for typically small slices.
func sortAnomaliesBySeverity(anomalies []*StatusAnomaly) {
	for i := 1; i < len(anomalies); i++ {
		key := anomalies[i]
		j := i - 1
		for j >= 0 && severityOrder[anomalies[j].Severity] > severityOrder[key.Severity] {
			anomalies[j+1] = anomalies[j]
			j--
		}
		anomalies[j+1] = key
	}
}

// ---------------------------------------------------------------------------
// SubscribeStatusChange
// ---------------------------------------------------------------------------

func (s *legalStatusServiceImpl) SubscribeStatusChange(ctx context.Context, req *SubscriptionRequest) (*Subscription, error) {
	if req == nil {
		return nil, errors.NewValidationOp("subscribe", "subscription request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	subID := fmt.Sprintf("sub_%d", time.Now().UnixNano())

	sub := &Subscription{
		ID:        subID,
		Active:    true,
		CreatedAt: time.Now().UTC(),
		Filters:   req.StatusFilters,
	}

	// Persist subscription via repository.
	if err := s.lifecycleRepo.SaveSubscription(ctx, &lifecycle.SubscriptionEntity{
		ID:            subID,
		PatentIDs:     req.PatentIDs,
		PortfolioID:   req.PortfolioID,
		StatusFilters: req.StatusFilters,
		Channels:      toStringChannels(req.Channels),
		Recipient:     req.Recipient,
		Active:        true,
		CreatedAt:     sub.CreatedAt,
	}); err != nil {
		return nil, fmt.Errorf("save subscription: %w", err)
	}

	s.logger.Info("subscription created", "subscription_id", subID, "recipient", req.Recipient)
	s.metrics.IncCounter("legal_status_subscriptions_created_total", map[string]string{"recipient": req.Recipient})

	return sub, nil
}

// toStringChannels converts NotificationChannel slice to string slice.
func toStringChannels(channels []NotificationChannel) []string {
	result := make([]string, len(channels))
	for i, ch := range channels {
		result[i] = ch.String()
	}
	return result
}

// ---------------------------------------------------------------------------
// UnsubscribeStatusChange
// ---------------------------------------------------------------------------

func (s *legalStatusServiceImpl) UnsubscribeStatusChange(ctx context.Context, subscriptionID string) error {
	if subscriptionID == "" {
		return errors.NewValidationOp("unsubscribe", "subscription_id must not be empty")
	}

	if err := s.lifecycleRepo.DeactivateSubscription(ctx, subscriptionID); err != nil {
		return fmt.Errorf("deactivate subscription %s: %w", subscriptionID, err)
	}

	s.logger.Info("subscription deactivated", "subscription_id", subscriptionID)
	s.metrics.IncCounter("legal_status_subscriptions_deactivated_total", map[string]string{"subscription_id": subscriptionID})

	return nil
}

// ---------------------------------------------------------------------------
// GetStatusSummary
// ---------------------------------------------------------------------------

func (s *legalStatusServiceImpl) GetStatusSummary(ctx context.Context, portfolioID string) (*StatusSummary, error) {
	if portfolioID == "" {
		return nil, errors.NewValidationOp("get_status_summary", "portfolio_id must not be empty")
	}

	// 1. Try cache.
	cacheKey := summaryCacheKey(portfolioID)
	var cached StatusSummary
	if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
		s.metrics.IncCounter("legal_status_summary_cache_hits_total", map[string]string{"portfolio_id": portfolioID})
		return &cached, nil
	}
	s.metrics.IncCounter("legal_status_summary_cache_misses_total", map[string]string{"portfolio_id": portfolioID})

	// 2. Load patents and aggregate.
	patents, err := s.patentRepo.ListByPortfolio(ctx, portfolioID)
	if err != nil {
		return nil, fmt.Errorf("list patents for summary %s: %w", portfolioID, err)
	}

	byStatus := make(map[string]int)
	byJurisdiction := make(map[string]int)
	var latestSync time.Time

	for _, p := range patents {
		entity, err := s.lifecycleRepo.GetByPatentID(ctx, p.ID.String())
		if err != nil || entity == nil {
			continue
		}

		mappedCode, _ := MapJurisdictionStatus(entity.Jurisdiction, entity.Status)
		byStatus[mappedCode.String()]++
		byJurisdiction[entity.Jurisdiction]++

		if entity.LastSyncAt != nil && entity.LastSyncAt.After(latestSync) {
			latestSync = *entity.LastSyncAt
		}
	}

	// 3. Detect anomalies for health score.
	anomalies, _ := s.DetectAnomalies(ctx, portfolioID)
	anomalyCount := len(anomalies)

	// 4. Compute health score.
	healthScore := computeHealthScore(anomalies, len(patents))

	summary := &StatusSummary{
		PortfolioID:    portfolioID,
		TotalPatents:   len(patents),
		ByStatus:       byStatus,
		ByJurisdiction: byJurisdiction,
		AnomalyCount:   anomalyCount,
		LastSyncAt:     latestSync,
		HealthScore:    healthScore,
	}

	// 5. Backfill cache.
	if setErr := s.cache.Set(ctx, cacheKey, summary, s.config.SummaryCacheTTL); setErr != nil {
		s.logger.Warn("failed to set summary cache", "portfolio_id", portfolioID, "error", setErr)
	}

	return summary, nil
}

// computeHealthScore calculates the portfolio health score based on anomaly
// severity distribution.
//
// Formula: HealthScore = 1.0 - (critical*0.3 + high*0.15 + medium*0.05) / total
// The result is clamped to [0.0, 1.0].
func computeHealthScore(anomalies []*StatusAnomaly, totalPatents int) float64 {
	if totalPatents == 0 {
		return 1.0
	}

	var criticalCount, highCount, mediumCount int
	for _, a := range anomalies {
		switch a.Severity {
		case SeverityCritical:
			criticalCount++
		case SeverityHigh:
			highCount++
		case SeverityMedium:
			mediumCount++
		}
	}

	penalty := (float64(criticalCount)*0.3 + float64(highCount)*0.15 + float64(mediumCount)*0.05) / float64(totalPatents)
	score := 1.0 - penalty

	return math.Max(0.0, math.Min(1.0, score))
}

// ---------------------------------------------------------------------------
// ReconcileStatus
// ---------------------------------------------------------------------------

func (s *legalStatusServiceImpl) ReconcileStatus(ctx context.Context, patentID string) (*ReconcileResult, error) {
	if patentID == "" {
		return nil, errors.NewValidationOp("reconcile_status", "patent_id must not be empty")
	}

	s.logger.Debug("reconcile_status started", "patent_id", patentID)

	// 1. Get local status.
	localEntity, err := s.lifecycleRepo.GetByPatentID(ctx, patentID)
	if err != nil {
		return nil, fmt.Errorf("get local status for reconciliation %s: %w", patentID, err)
	}
	if localEntity == nil {
		return nil, errors.NewNotFoundOp("reconcile_status", fmt.Sprintf("no local status found for patent %s", patentID))
	}

	// 2. Get remote status.
	remoteStatus, err := s.lifecycleSvc.FetchRemoteStatus(ctx, patentID)
	if err != nil {
		return nil, fmt.Errorf("fetch remote status for reconciliation %s: %w", patentID, err)
	}

	// 3. Field-by-field comparison.
	var discrepancies []Discrepancy

	if localEntity.Status != remoteStatus.Status {
		discrepancies = append(discrepancies, Discrepancy{
			Field:       "status",
			LocalValue:  localEntity.Status,
			RemoteValue: remoteStatus.Status,
			Resolution:  "update_local",
		})
	}

	if localEntity.Jurisdiction != remoteStatus.Jurisdiction {
		discrepancies = append(discrepancies, Discrepancy{
			Field:       "jurisdiction",
			LocalValue:  localEntity.Jurisdiction,
			RemoteValue: remoteStatus.Jurisdiction,
			Resolution:  "investigate",
		})
	}

	if !localEntity.EffectiveDate.Equal(remoteStatus.EffectiveDate) {
		discrepancies = append(discrepancies, Discrepancy{
			Field:       "effective_date",
			LocalValue:  localEntity.EffectiveDate.Format(time.RFC3339),
			RemoteValue: remoteStatus.EffectiveDate.Format(time.RFC3339),
			Resolution:  "update_local",
		})
	}

	localNextAction := localEntity.NextAction
	remoteNextAction := remoteStatus.NextAction
	if localNextAction != remoteNextAction {
		discrepancies = append(discrepancies, Discrepancy{
			Field:       "next_action",
			LocalValue:  localNextAction,
			RemoteValue: remoteNextAction,
			Resolution:  "update_local",
		})
	}

	consistent := len(discrepancies) == 0

	// 4. Auto-fix: if inconsistent, update local to match remote.
	if !consistent {
		if updateErr := s.lifecycleRepo.UpdateStatus(ctx, patentID, remoteStatus.Status, remoteStatus.EffectiveDate); updateErr != nil {
			s.logger.Warn("auto-reconciliation failed", "patent_id", patentID, "error", updateErr)
		} else {
			_ = s.cache.Delete(ctx, statusCacheKey(patentID))
			s.logger.Info("auto-reconciliation applied", "patent_id", patentID, "discrepancy_count", len(discrepancies))
		}
	}

	result := &ReconcileResult{
		PatentID:      patentID,
		Consistent:    consistent,
		LocalStatus:   localEntity.Status,
		RemoteStatus:		  remoteStatus.Status,
		Discrepancies: discrepancies,
		ReconciledAt:  time.Now().UTC(),
	}

	s.metrics.IncCounter("legal_status_reconciliations_total", map[string]string{
		"patent_id":  patentID,
		"consistent": fmt.Sprintf("%t", consistent),
	})
	s.logger.Info("reconcile_status completed", "patent_id", patentID, "consistent", consistent, "discrepancies", len(discrepancies))

	return result, nil
}

// ---------------------------------------------------------------------------
// Compile-time interface compliance check
// ---------------------------------------------------------------------------

var _ LegalStatusService = (*legalStatusServiceImpl)(nil)

//Personal.AI order the ending
