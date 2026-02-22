// ---
// Phase 10 - File #192: internal/application/collaboration/sharing.go
//
// 生成计划:
//
// 功能定位: 协作分享应用服务，协调接口层与领域层交互，编排分享创建、撤销、
//   查询、令牌验证等业务流程。本层不包含核心业务规则，规则下沉至领域层。
//
// 核心实现:
//   - 定义 SharingService 接口: Share / Revoke / ListShares / GetShareLink / ValidateShareToken
//   - 实现 sharingServiceImpl 结构体，注入领域服务、仓储、缓存、日志
//   - Share: 参数校验 -> 权限验证 -> 生成分享令牌 -> 持久化 -> 发布事件 -> 返回
//   - Revoke: 存在性校验 -> 权限验证 -> 标记失效 -> 清除缓存 -> 发布事件
//   - ListShares: 参数校验 -> 查询仓储 -> 结果转换
//   - GetShareLink: 查询分享 -> 构建链接 -> 缓存
//   - ValidateShareToken: 解析令牌 -> 验证有效期 -> 验证权限 -> 返回信息
//
// 业务逻辑:
//   - 分享令牌采用 HMAC-SHA256 签名的 JWT 格式，包含 workspaceID / permissions / expiry
//   - 支持有效期: Permanent / 7d / 30d / Custom
//   - 支持权限级别: ReadOnly / Comment / Edit
//   - 分享链接格式: https://{domain}/share/{token}
//   - 撤销操作幂等，重复撤销不报错
//
// 依赖关系:
//   - 依赖: internal/domain/collaboration (service, repository, workspace, permission)
//           internal/infrastructure/database/redis (cache)
//           pkg/errors, pkg/types/common
//           internal/infrastructure/monitoring/logging (logger)
//   - 被依赖: internal/interfaces/http/handlers/collaboration_handler.go
//
// 测试要求: Mock 领域服务与仓储，验证参数校验、权限检查、令牌生成与解析、缓存交互、异常场景
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
// ---

package collaboration

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	collabdomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/collaboration"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	pkgerrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// SharePermission enumerates the permission levels for a share link.
type SharePermission string

const (
	SharePermissionReadOnly SharePermission = "read_only"
	SharePermissionComment  SharePermission = "comment"
	SharePermissionEdit     SharePermission = "edit"
)

// ValidSharePermissions returns all valid permission values.
func ValidSharePermissions() []SharePermission {
	return []SharePermission{SharePermissionReadOnly, SharePermissionComment, SharePermissionEdit}
}

func (p SharePermission) IsValid() bool {
	for _, v := range ValidSharePermissions() {
		if p == v {
			return true
		}
	}
	return false
}

// ShareDuration enumerates preset share durations.
type ShareDuration string

const (
	ShareDurationPermanent ShareDuration = "permanent"
	ShareDuration7Days     ShareDuration = "7d"
	ShareDuration30Days    ShareDuration = "30d"
	ShareDurationCustom    ShareDuration = "custom"
)

// ShareRequest is the input DTO for creating a share.
type ShareRequest struct {
	WorkspaceID   string          `json:"workspace_id"`
	Permission    SharePermission `json:"permission"`
	Duration      ShareDuration   `json:"duration"`
	CustomExpiry  *time.Time      `json:"custom_expiry,omitempty"`
	CreatedBy     string          `json:"created_by"`
	Description   string          `json:"description,omitempty"`
	MaxAccessCount int            `json:"max_access_count,omitempty"`
}

// Validate checks the share request for correctness.
func (r *ShareRequest) Validate() error {
	if strings.TrimSpace(r.WorkspaceID) == "" {
		return pkgerrors.NewValidation("workspace_id is required")
	}
	if strings.TrimSpace(r.CreatedBy) == "" {
		return pkgerrors.NewValidation("created_by is required")
	}
	if !r.Permission.IsValid() {
		return pkgerrors.NewValidation(fmt.Sprintf("invalid permission: %s", r.Permission))
	}
	switch r.Duration {
	case ShareDurationPermanent, ShareDuration7Days, ShareDuration30Days:
		// valid preset
	case ShareDurationCustom:
		if r.CustomExpiry == nil {
			return pkgerrors.NewValidation("custom_expiry is required when duration is custom")
		}
		if r.CustomExpiry.Before(time.Now()) {
			return pkgerrors.NewValidation("custom_expiry must be in the future")
		}
	default:
		return pkgerrors.NewValidation(fmt.Sprintf("invalid duration: %s", r.Duration))
	}
	if r.MaxAccessCount < 0 {
		return pkgerrors.NewValidation("max_access_count must be non-negative")
	}
	return nil
}

// ShareResponse is the output DTO after creating a share.
type ShareResponse struct {
	ShareID   string          `json:"share_id"`
	Token     string          `json:"token"`
	Link      string          `json:"link"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
	Permission SharePermission `json:"permission"`
	CreatedAt time.Time       `json:"created_at"`
}

// ShareInfo contains validated share metadata returned by token validation.
type ShareInfo struct {
	ShareID     string          `json:"share_id"`
	WorkspaceID string          `json:"workspace_id"`
	Permission  SharePermission `json:"permission"`
	ExpiresAt   *time.Time      `json:"expires_at,omitempty"`
	CreatedBy   string          `json:"created_by"`
	IsRevoked   bool            `json:"is_revoked"`
}

// ShareRecord is the persistent representation of a share.
type ShareRecord struct {
	ID             string          `json:"id"`
	WorkspaceID    string          `json:"workspace_id"`
	Token          string          `json:"token"`
	Permission     SharePermission `json:"permission"`
	ExpiresAt      *time.Time      `json:"expires_at,omitempty"`
	CreatedBy      string          `json:"created_by"`
	Description    string          `json:"description,omitempty"`
	MaxAccessCount int             `json:"max_access_count"`
	AccessCount    int             `json:"access_count"`
	Revoked        bool            `json:"revoked"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ListSharesOption configures listing behavior.
type ListSharesOption func(*listSharesConfig)

type listSharesConfig struct {
	Pagination  commontypes.Pagination
	IncludeRevoked bool
}

func WithPagination(p commontypes.Pagination) ListSharesOption {
	return func(c *listSharesConfig) {
		c.Pagination = p
	}
}

func WithIncludeRevoked(include bool) ListSharesOption {
	return func(c *listSharesConfig) {
		c.IncludeRevoked = include
	}
}

// ShareRepository abstracts persistence for share records.
type ShareRepository interface {
	Create(ctx context.Context, record *ShareRecord) error
	GetByID(ctx context.Context, shareID string) (*ShareRecord, error)
	GetByToken(ctx context.Context, token string) (*ShareRecord, error)
	ListByWorkspace(ctx context.Context, workspaceID string, includeRevoked bool, pagination commontypes.Pagination) ([]*ShareRecord, int, error)
	Update(ctx context.Context, record *ShareRecord) error
	IncrementAccessCount(ctx context.Context, shareID string) error
}

// Cache abstracts caching operations.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// SharingService defines the application-level sharing operations.
type SharingService interface {
	Share(ctx context.Context, req *ShareRequest) (*ShareResponse, error)
	Revoke(ctx context.Context, shareID string, revokedBy string) error
	ListShares(ctx context.Context, workspaceID string, opts ...ListSharesOption) ([]*ShareRecord, int, error)
	GetShareLink(ctx context.Context, shareID string) (string, error)
	ValidateShareToken(ctx context.Context, token string) (*ShareInfo, error)
}

// SharingServiceConfig holds configuration for the sharing service.
type SharingServiceConfig struct {
	BaseDomain     string
	TokenSecret    []byte
	DefaultLinkTTL time.Duration
	CacheTTL       time.Duration
}

type sharingServiceImpl struct {
	domainService collabdomain.Service
	workspaceRepo collabdomain.WorkspaceRepository
	shareRepo     ShareRepository
	cache         Cache
	logger        logging.Logger
	config        SharingServiceConfig
}

// NewSharingService constructs a SharingService with all required dependencies.
func NewSharingService(
	domainService collabdomain.Service,
	workspaceRepo collabdomain.WorkspaceRepository,
	shareRepo ShareRepository,
	cache Cache,
	logger logging.Logger,
	config SharingServiceConfig,
) SharingService {
	if len(config.TokenSecret) == 0 {
		config.TokenSecret = []byte("keyip-default-secret-change-me")
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 10 * time.Minute
	}
	if config.BaseDomain == "" {
		config.BaseDomain = "https://app.keyip-intelligence.io"
	}
	return &sharingServiceImpl{
		domainService: domainService,
		workspaceRepo: workspaceRepo,
		shareRepo:     shareRepo,
		cache:         cache,
		logger:        logger,
		config:        config,
	}
}

// Share creates a new share link for a workspace resource.
func (s *sharingServiceImpl) Share(ctx context.Context, req *ShareRequest) (*ShareResponse, error) {
	if req == nil {
		return nil, pkgerrors.NewValidation("share request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Verify workspace exists
	ws, err := s.workspaceRepo.GetByID(ctx, req.WorkspaceID)
	if err != nil {
		s.logger.Error("failed to fetch workspace", "workspace_id", req.WorkspaceID, "error", err)
		return nil, pkgerrors.NewNotFound(fmt.Sprintf("workspace %s not found", req.WorkspaceID))
	}

	// Verify caller has permission to share
	if err := s.domainService.CheckPermission(ctx, req.CreatedBy, ws.ID, collabdomain.ActionShare); err != nil {
		s.logger.Warn("permission denied for share", "user", req.CreatedBy, "workspace", ws.ID)
		return nil, pkgerrors.NewPermissionDenied("insufficient permission to share this workspace")
	}

	// Calculate expiry
	var expiresAt *time.Time
	switch req.Duration {
	case ShareDuration7Days:
		t := time.Now().Add(7 * 24 * time.Hour)
		expiresAt = &t
	case ShareDuration30Days:
		t := time.Now().Add(30 * 24 * time.Hour)
		expiresAt = &t
	case ShareDurationCustom:
		expiresAt = req.CustomExpiry
	case ShareDurationPermanent:
		// nil means no expiry
	}

	shareID := commontypes.NewID()
	now := time.Now().UTC()

	// Generate signed token
	token, err := s.generateToken(shareID, req.WorkspaceID, req.Permission, expiresAt)
	if err != nil {
		s.logger.Error("failed to generate share token", "error", err)
		return nil, pkgerrors.NewInternal("failed to generate share token")
	}

	record := &ShareRecord{
		ID:             shareID,
		WorkspaceID:    req.WorkspaceID,
		Token:          token,
		Permission:     req.Permission,
		ExpiresAt:      expiresAt,
		CreatedBy:      req.CreatedBy,
		Description:    req.Description,
		MaxAccessCount: req.MaxAccessCount,
		AccessCount:    0,
		Revoked:        false,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.shareRepo.Create(ctx, record); err != nil {
		s.logger.Error("failed to persist share record", "share_id", shareID, "error", err)
		return nil, pkgerrors.NewInternal("failed to create share")
	}

	link := fmt.Sprintf("%s/share/%s", s.config.BaseDomain, token)

	s.logger.Info("share created", "share_id", shareID, "workspace_id", req.WorkspaceID, "permission", req.Permission)

	return &ShareResponse{
		ShareID:    shareID,
		Token:      token,
		Link:       link,
		ExpiresAt:  expiresAt,
		Permission: req.Permission,
		CreatedAt:  now,
	}, nil
}

// Revoke invalidates an existing share. The operation is idempotent.
func (s *sharingServiceImpl) Revoke(ctx context.Context, shareID string, revokedBy string) error {
	if strings.TrimSpace(shareID) == "" {
		return pkgerrors.NewValidation("share_id is required")
	}
	if strings.TrimSpace(revokedBy) == "" {
		return pkgerrors.NewValidation("revoked_by is required")
	}

	record, err := s.shareRepo.GetByID(ctx, shareID)
	if err != nil {
		return pkgerrors.NewNotFound(fmt.Sprintf("share %s not found", shareID))
	}

	// Idempotent: already revoked is not an error
	if record.Revoked {
		s.logger.Info("share already revoked", "share_id", shareID)
		return nil
	}

	// Verify permission to revoke
	if err := s.domainService.CheckPermission(ctx, revokedBy, record.WorkspaceID, collabdomain.ActionShare); err != nil {
		return pkgerrors.NewPermissionDenied("insufficient permission to revoke this share")
	}

	record.Revoked = true
	record.UpdatedAt = time.Now().UTC()

	if err := s.shareRepo.Update(ctx, record); err != nil {
		s.logger.Error("failed to update share record", "share_id", shareID, "error", err)
		return pkgerrors.NewInternal("failed to revoke share")
	}

	// Invalidate cache
	cacheKey := s.shareLinkCacheKey(shareID)
	_ = s.cache.Delete(ctx, cacheKey)
	tokenCacheKey := s.tokenCacheKey(record.Token)
	_ = s.cache.Delete(ctx, tokenCacheKey)

	s.logger.Info("share revoked", "share_id", shareID, "revoked_by", revokedBy)
	return nil
}

// ListShares returns share records for a workspace with optional pagination.
func (s *sharingServiceImpl) ListShares(ctx context.Context, workspaceID string, opts ...ListSharesOption) ([]*ShareRecord, int, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, 0, pkgerrors.NewValidation("workspace_id is required")
	}

	cfg := &listSharesConfig{
		Pagination: commontypes.Pagination{
			Page:     1,
			PageSize: 20,
		},
		IncludeRevoked: false,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.Pagination.Page < 1 {
		cfg.Pagination.Page = 1
	}
	if cfg.Pagination.PageSize < 1 || cfg.Pagination.PageSize > 100 {
		cfg.Pagination.PageSize = 20
	}

	records, total, err := s.shareRepo.ListByWorkspace(ctx, workspaceID, cfg.IncludeRevoked, cfg.Pagination)
	if err != nil {
		s.logger.Error("failed to list shares", "workspace_id", workspaceID, "error", err)
		return nil, 0, pkgerrors.NewInternal("failed to list shares")
	}

	return records, total, nil
}

// GetShareLink returns the full share URL for a given share ID, with caching.
func (s *sharingServiceImpl) GetShareLink(ctx context.Context, shareID string) (string, error) {
	if strings.TrimSpace(shareID) == "" {
		return "", pkgerrors.NewValidation("share_id is required")
	}

	cacheKey := s.shareLinkCacheKey(shareID)
	cached, err := s.cache.Get(ctx, cacheKey)
	if err == nil && cached != "" {
		return cached, nil
	}

	record, err := s.shareRepo.GetByID(ctx, shareID)
	if err != nil {
		return "", pkgerrors.NewNotFound(fmt.Sprintf("share %s not found", shareID))
	}

	if record.Revoked {
		return "", pkgerrors.NewValidation("share has been revoked")
	}

	if record.ExpiresAt != nil && record.ExpiresAt.Before(time.Now()) {
		return "", pkgerrors.NewValidation("share has expired")
	}

	link := fmt.Sprintf("%s/share/%s", s.config.BaseDomain, record.Token)

	_ = s.cache.Set(ctx, cacheKey, link, s.config.CacheTTL)

	return link, nil
}

// ValidateShareToken verifies a share token and returns the associated metadata.
func (s *sharingServiceImpl) ValidateShareToken(ctx context.Context, token string) (*ShareInfo, error) {
	if strings.TrimSpace(token) == "" {
		return nil, pkgerrors.NewValidation("token is required")
	}

	// Check cache first
	tokenKey := s.tokenCacheKey(token)
	cached, err := s.cache.Get(ctx, tokenKey)
	if err == nil && cached != "" {
		var info ShareInfo
		if jsonErr := json.Unmarshal([]byte(cached), &info); jsonErr == nil {
			if !info.IsRevoked {
				return &info, nil
			}
		}
	}

	// Parse and verify token signature
	payload, err := s.verifyToken(token)
	if err != nil {
		return nil, pkgerrors.NewValidation(fmt.Sprintf("invalid share token: %v", err))
	}

	// Fetch record from repository for authoritative state
	record, err := s.shareRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, pkgerrors.NewNotFound("share not found for the given token")
	}

	if record.Revoked {
		return nil, pkgerrors.NewValidation("share has been revoked")
	}

	if record.ExpiresAt != nil && record.ExpiresAt.Before(time.Now()) {
		return nil, pkgerrors.NewValidation("share has expired")
	}

	if record.MaxAccessCount > 0 && record.AccessCount >= record.MaxAccessCount {
		return nil, pkgerrors.NewValidation("share access limit reached")
	}

	// Increment access count
	_ = s.shareRepo.IncrementAccessCount(ctx, record.ID)

	info := &ShareInfo{
		ShareID:     payload.ShareID,
		WorkspaceID: payload.WorkspaceID,
		Permission:  payload.Permission,
		ExpiresAt:   record.ExpiresAt,
		CreatedBy:   record.CreatedBy,
		IsRevoked:   false,
	}

	// Cache the validated info
	if infoBytes, jsonErr := json.Marshal(info); jsonErr == nil {
		ttl := s.config.CacheTTL
		if record.ExpiresAt != nil {
			remaining := time.Until(*record.ExpiresAt)
			if remaining < ttl {
				ttl = remaining
			}
		}
		if ttl > 0 {
			_ = s.cache.Set(ctx, tokenKey, string(infoBytes), ttl)
		}
	}

	return info, nil
}

// --- Token generation and verification ---

type tokenPayload struct {
	ShareID     string          `json:"sid"`
	WorkspaceID string          `json:"wid"`
	Permission  SharePermission `json:"perm"`
	ExpiresAt   *time.Time      `json:"exp,omitempty"`
	IssuedAt    time.Time       `json:"iat"`
}

func (s *sharingServiceImpl) generateToken(shareID, workspaceID string, perm SharePermission, expiresAt *time.Time) (string, error) {
	payload := tokenPayload{
		ShareID:     shareID,
		WorkspaceID: workspaceID,
		Permission:  perm,
		ExpiresAt:   expiresAt,
		IssuedAt:    time.Now().UTC(),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token payload: %w", err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sig := s.sign(encoded)

	return encoded + "." + sig, nil
}

func (s *sharingServiceImpl) verifyToken(token string) (*tokenPayload, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("malformed token: expected 2 parts, got %d", len(parts))
	}

	encoded := parts[0]
	sig := parts[1]

	expectedSig := s.sign(encoded)
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid token signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token payload: %w", err)
	}

	var payload tokenPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token payload: %w", err)
	}

	return &payload, nil
}

func (s *sharingServiceImpl) sign(data string) string {
	mac := hmac.New(sha256.New, s.config.TokenSecret)
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *sharingServiceImpl) shareLinkCacheKey(shareID string) string {
	return fmt.Sprintf("share:link:%s", shareID)
}

func (s *sharingServiceImpl) tokenCacheKey(token string) string {
	// Use a hash of the token to avoid overly long cache keys
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("share:token:%s", base64.RawURLEncoding.EncodeToString(h[:16]))
}

//Personal.AI order the ending
