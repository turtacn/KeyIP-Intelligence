// ---
// Phase 10 - File #193: internal/application/collaboration/sharing_test.go
//
// 生成计划:
//
// 功能定位: SharingService 接口所有方法的单元测试
//
// 测试用例:
//   - TestShareRequest_Validate: 请求参数校验（正常/缺字段/无效权限/无效时长/过期自定义时间）
//   - TestShare_Success: 正常分享流程
//   - TestShare_NilRequest: nil 请求
//   - TestShare_WorkspaceNotFound: 工作空间不存在
//   - TestShare_PermissionDenied: 无分享权限
//   - TestShare_PersistError: 持久化失败
//   - TestRevoke_Success: 正常撤销
//   - TestRevoke_AlreadyRevoked: 幂等撤销
//   - TestRevoke_NotFound: 分享不存在
//   - TestRevoke_PermissionDenied: 无撤销权限
//   - TestListShares_Success: 正常列表
//   - TestListShares_EmptyWorkspaceID: 空工作空间ID
//   - TestListShares_WithOptions: 分页与过滤选项
//   - TestGetShareLink_Success: 获取链接
//   - TestGetShareLink_CacheHit: 缓存命中
//   - TestGetShareLink_Revoked: 已撤销
//   - TestGetShareLink_Expired: 已过期
//   - TestValidateShareToken_Valid: 有效令牌
//   - TestValidateShareToken_Expired: 过期令牌
//   - TestValidateShareToken_Revoked: 已撤销令牌
//   - TestValidateShareToken_InvalidFormat: 格式错误
//   - TestValidateShareToken_AccessLimitReached: 访问次数超限
//   - TestTokenGenerateAndVerify: 令牌生成与验证往返
//
// Mock 依赖: mockCollaborationDomainService, mockWorkspaceRepository, mockShareRepository, mockCache, mockLogger
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
// ---

package collaboration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	collabdomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/collaboration"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// --- Mock implementations ---

type mockCollabDomainService struct {
	checkMemberAccessFn func(ctx context.Context, workspaceID, userID string, resource collabdomain.ResourceType, action collabdomain.Action) (bool, string, error)
}

func (m *mockCollabDomainService) CheckMemberAccess(ctx context.Context, workspaceID, userID string, resource collabdomain.ResourceType, action collabdomain.Action) (bool, string, error) {
	if m.checkMemberAccessFn != nil {
		return m.checkMemberAccessFn(ctx, workspaceID, userID, resource, action)
	}
	return true, "", nil
}

// Stubs for other methods of CollaborationService
func (m *mockCollabDomainService) CreateWorkspace(ctx context.Context, name, ownerID string) (*collabdomain.Workspace, error) {
	return &collabdomain.Workspace{ID: "ws-1", Name: name, OwnerID: ownerID}, nil
}
func (m *mockCollabDomainService) InviteMember(ctx context.Context, workspaceID, inviterID, inviteeUserID string, role collabdomain.Role) (*collabdomain.MemberPermission, error) {
	return nil, nil
}
func (m *mockCollabDomainService) AcceptInvitation(ctx context.Context, memberID, userID string) error {
	return nil
}
func (m *mockCollabDomainService) RemoveMember(ctx context.Context, workspaceID, removerID, targetUserID string) error {
	return nil
}
func (m *mockCollabDomainService) ChangeMemberRole(ctx context.Context, workspaceID, changerID, targetUserID string, newRole collabdomain.Role) error {
	return nil
}
func (m *mockCollabDomainService) GetWorkspaceMembers(ctx context.Context, workspaceID, requesterID string) ([]*collabdomain.MemberPermission, error) {
	return nil, nil
}
func (m *mockCollabDomainService) GetUserWorkspaces(ctx context.Context, userID string) ([]*collabdomain.WorkspaceSummary, error) {
	return nil, nil
}
func (m *mockCollabDomainService) TransferOwnership(ctx context.Context, workspaceID, currentOwnerID, newOwnerID string) error {
	return nil
}
func (m *mockCollabDomainService) UpdateWorkspace(ctx context.Context, workspaceID, requesterID, name, description string) error {
	return nil
}
func (m *mockCollabDomainService) DeleteWorkspace(ctx context.Context, workspaceID, requesterID string) error {
	return nil
}
func (m *mockCollabDomainService) GrantCustomPermission(ctx context.Context, workspaceID, granterID, targetUserID string, perm *collabdomain.Permission) error {
	return nil
}
func (m *mockCollabDomainService) RevokeCustomPermission(ctx context.Context, workspaceID, revokerID, targetUserID string, resource collabdomain.ResourceType, action collabdomain.Action) error {
	return nil
}
func (m *mockCollabDomainService) GetWorkspaceActivity(ctx context.Context, workspaceID string, limit int) ([]*collabdomain.ActivityRecord, error) {
	return nil, nil
}

type mockWorkspaceRepo struct {
	findByIDFn func(ctx context.Context, id string) (*collabdomain.Workspace, error)
}

func (m *mockWorkspaceRepo) FindByID(ctx context.Context, id string) (*collabdomain.Workspace, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return &collabdomain.Workspace{ID: id, Name: "test-ws"}, nil
}

func (m *mockWorkspaceRepo) Save(ctx context.Context, ws *collabdomain.Workspace) error { return nil }
func (m *mockWorkspaceRepo) Delete(ctx context.Context, id string) error                { return nil }
func (m *mockWorkspaceRepo) FindByOwnerID(ctx context.Context, ownerID string) ([]*collabdomain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceRepo) FindByMemberID(ctx context.Context, userID string) ([]*collabdomain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceRepo) FindBySlug(ctx context.Context, slug string) (*collabdomain.Workspace, error) {
	return nil, nil
}
func (m *mockWorkspaceRepo) Count(ctx context.Context, ownerID string) (int64, error) { return 0, nil }

type mockShareRepo struct {
	createFn              func(ctx context.Context, record *ShareRecord) error
	getByIDFn             func(ctx context.Context, shareID string) (*ShareRecord, error)
	getByTokenFn          func(ctx context.Context, token string) (*ShareRecord, error)
	listByWorkspaceFn     func(ctx context.Context, workspaceID string, includeRevoked bool, p commontypes.Pagination) ([]*ShareRecord, int, error)
	updateFn              func(ctx context.Context, record *ShareRecord) error
	incrementAccessCountFn func(ctx context.Context, shareID string) error
}

func (m *mockShareRepo) Create(ctx context.Context, record *ShareRecord) error {
	if m.createFn != nil {
		return m.createFn(ctx, record)
	}
	return nil
}

func (m *mockShareRepo) GetByID(ctx context.Context, shareID string) (*ShareRecord, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, shareID)
	}
	return nil, errors.New("not found")
}

func (m *mockShareRepo) GetByToken(ctx context.Context, token string) (*ShareRecord, error) {
	if m.getByTokenFn != nil {
		return m.getByTokenFn(ctx, token)
	}
	return nil, errors.New("not found")
}

func (m *mockShareRepo) ListByWorkspace(ctx context.Context, workspaceID string, includeRevoked bool, p commontypes.Pagination) ([]*ShareRecord, int, error) {
	if m.listByWorkspaceFn != nil {
		return m.listByWorkspaceFn(ctx, workspaceID, includeRevoked, p)
	}
	return nil, 0, nil
}

func (m *mockShareRepo) Update(ctx context.Context, record *ShareRecord) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, record)
	}
	return nil
}

func (m *mockShareRepo) IncrementAccessCount(ctx context.Context, shareID string) error {
	if m.incrementAccessCountFn != nil {
		return m.incrementAccessCountFn(ctx, shareID)
	}
	return nil
}

type mockCache struct {
	store    map[string]string
	getFn    func(ctx context.Context, key string) (string, error)
	setFn    func(ctx context.Context, key string, value string, ttl time.Duration) error
	deleteFn func(ctx context.Context, key string) error
}

func newMockCache() *mockCache {
	return &mockCache{store: make(map[string]string)}
}

func (m *mockCache) Get(ctx context.Context, key string) (string, error) {
	if m.getFn != nil {
		return m.getFn(ctx, key)
	}
	v, ok := m.store[key]
	if !ok {
		return "", errors.New("cache miss")
	}
	return v, nil
}

func (m *mockCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if m.setFn != nil {
		return m.setFn(ctx, key, value, ttl)
	}
	m.store[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, key)
	}
	delete(m.store, key)
	return nil
}

type mockSharingLogger struct{}

func (m *mockSharingLogger) Debug(msg string, fields ...logging.Field) {}
func (m *mockSharingLogger) Info(msg string, fields ...logging.Field)  {}
func (m *mockSharingLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *mockSharingLogger) Error(msg string, fields ...logging.Field) {}
func (m *mockSharingLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *mockSharingLogger) With(fields ...logging.Field) logging.Logger { return m }
func (m *mockSharingLogger) WithContext(ctx context.Context) logging.Logger { return m }
func (m *mockSharingLogger) WithError(err error) logging.Logger { return m }
func (m *mockSharingLogger) Sync() error { return nil }

func newTestSharingService(
	domainSvc *mockCollabDomainService,
	wsRepo *mockWorkspaceRepo,
	shareRepo *mockShareRepo,
	cache *mockCache,
) SharingService {
	if domainSvc == nil {
		domainSvc = &mockCollabDomainService{}
	}
	if wsRepo == nil {
		wsRepo = &mockWorkspaceRepo{}
	}
	if shareRepo == nil {
		shareRepo = &mockShareRepo{}
	}
	if cache == nil {
		cache = newMockCache()
	}
	return NewSharingService(
		domainSvc,
		wsRepo,
		shareRepo,
		cache,
		&mockSharingLogger{},
		SharingServiceConfig{
			BaseDomain:  "https://test.keyip.io",
			TokenSecret: []byte("test-secret-key-32bytes-long!!!!"),
			CacheTTL:    5 * time.Minute,
		},
	)
}

// --- ShareRequest.Validate tests ---

func TestShareRequest_Validate_Success(t *testing.T) {
	req := &ShareRequest{
		WorkspaceID: "ws-1",
		Permission:  SharePermissionReadOnly,
		Duration:    ShareDuration7Days,
		CreatedBy:   "user-1",
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestShareRequest_Validate_MissingWorkspaceID(t *testing.T) {
	req := &ShareRequest{
		Permission: SharePermissionReadOnly,
		Duration:   ShareDuration7Days,
		CreatedBy:  "user-1",
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected validation error for missing workspace_id")
	}
}

func TestShareRequest_Validate_MissingCreatedBy(t *testing.T) {
	req := &ShareRequest{
		WorkspaceID: "ws-1",
		Permission:  SharePermissionReadOnly,
		Duration:    ShareDuration7Days,
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected validation error for missing created_by")
	}
}

func TestShareRequest_Validate_InvalidPermission(t *testing.T) {
	req := &ShareRequest{
		WorkspaceID: "ws-1",
		Permission:  "admin",
		Duration:    ShareDuration7Days,
		CreatedBy:   "user-1",
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected validation error for invalid permission")
	}
}

func TestShareRequest_Validate_InvalidDuration(t *testing.T) {
	req := &ShareRequest{
		WorkspaceID: "ws-1",
		Permission:  SharePermissionEdit,
		Duration:    "1y",
		CreatedBy:   "user-1",
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected validation error for invalid duration")
	}
}

func TestShareRequest_Validate_CustomDuration_NoExpiry(t *testing.T) {
	req := &ShareRequest{
		WorkspaceID: "ws-1",
		Permission:  SharePermissionEdit,
		Duration:    ShareDurationCustom,
		CreatedBy:   "user-1",
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected validation error for custom duration without expiry")
	}
}

func TestShareRequest_Validate_CustomDuration_PastExpiry(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	req := &ShareRequest{
		WorkspaceID:  "ws-1",
		Permission:   SharePermissionEdit,
		Duration:     ShareDurationCustom,
		CustomExpiry: &past,
		CreatedBy:    "user-1",
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected validation error for past custom expiry")
	}
}

func TestShareRequest_Validate_CustomDuration_Success(t *testing.T) {
	future := time.Now().Add(48 * time.Hour)
	req := &ShareRequest{
		WorkspaceID:  "ws-1",
		Permission:   SharePermissionComment,
		Duration:     ShareDurationCustom,
		CustomExpiry: &future,
		CreatedBy:    "user-1",
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestShareRequest_Validate_NegativeMaxAccess(t *testing.T) {
	req := &ShareRequest{
		WorkspaceID:    "ws-1",
		Permission:     SharePermissionReadOnly,
		Duration:       ShareDurationPermanent,
		CreatedBy:      "user-1",
		MaxAccessCount: -1,
	}
	if err := req.Validate(); err == nil {
		t.Fatal("expected validation error for negative max_access_count")
	}
}

// --- Share tests ---

func TestShare_Success(t *testing.T) {
	svc := newTestSharingService(nil, nil, nil, nil)
	resp, err := svc.Share(context.Background(), &ShareRequest{
		WorkspaceID: "ws-1",
		Permission:  SharePermissionReadOnly,
		Duration:    ShareDuration7Days,
		CreatedBy:   "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ShareID == "" {
		t.Fatal("expected non-empty share_id")
	}
	if resp.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if resp.Link == "" {
		t.Fatal("expected non-empty link")
	}
	if resp.ExpiresAt == nil {
		t.Fatal("expected non-nil expires_at for 7d duration")
	}
	if resp.Permission != SharePermissionReadOnly {
		t.Fatalf("expected read_only permission, got %s", resp.Permission)
	}
}

func TestShare_PermanentDuration(t *testing.T) {
	svc := newTestSharingService(nil, nil, nil, nil)
	resp, err := svc.Share(context.Background(), &ShareRequest{
		WorkspaceID: "ws-1",
		Permission:  SharePermissionEdit,
		Duration:    ShareDurationPermanent,
		CreatedBy:   "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ExpiresAt != nil {
		t.Fatal("expected nil expires_at for permanent duration")
	}
}

func TestShare_NilRequest(t *testing.T) {
	svc := newTestSharingService(nil, nil, nil, nil)
	_, err := svc.Share(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestShare_WorkspaceNotFound(t *testing.T) {
	wsRepo := &mockWorkspaceRepo{
		findByIDFn: func(ctx context.Context, id string) (*collabdomain.Workspace, error) {
			return nil, errors.New("not found")
		},
	}
	svc := newTestSharingService(nil, wsRepo, nil, nil)
	_, err := svc.Share(context.Background(), &ShareRequest{
		WorkspaceID: "ws-missing",
		Permission:  SharePermissionReadOnly,
		Duration:    ShareDuration7Days,
		CreatedBy:   "user-1",
	})
	if err == nil {
		t.Fatal("expected error for missing workspace")
	}
}

func TestShare_PermissionDenied(t *testing.T) {
	domainSvc := &mockCollabDomainService{
		checkMemberAccessFn: func(ctx context.Context, workspaceID, userID string, resource collabdomain.ResourceType, action collabdomain.Action) (bool, string, error) {
			return false, "denied", nil
		},
	}
	svc := newTestSharingService(domainSvc, nil, nil, nil)
	_, err := svc.Share(context.Background(), &ShareRequest{
		WorkspaceID: "ws-1",
		Permission:  SharePermissionEdit,
		Duration:    ShareDuration30Days,
		CreatedBy:   "user-no-perm",
	})
	if err == nil {
		t.Fatal("expected permission denied error")
	}
}

func TestShare_PersistError(t *testing.T) {
	shareRepo := &mockShareRepo{
		createFn: func(ctx context.Context, record *ShareRecord) error {
			return errors.New("db error")
		},
	}
	svc := newTestSharingService(nil, nil, shareRepo, nil)
	_, err := svc.Share(context.Background(), &ShareRequest{
		WorkspaceID: "ws-1",
		Permission:  SharePermissionReadOnly,
		Duration:    ShareDuration7Days,
		CreatedBy:   "user-1",
	})
	if err == nil {
		t.Fatal("expected error for persist failure")
	}
}

// --- Revoke tests ---

func TestRevoke_Success(t *testing.T) {
	shareRepo := &mockShareRepo{
		getByIDFn: func(ctx context.Context, shareID string) (*ShareRecord, error) {
			return &ShareRecord{
				ID:          shareID,
				WorkspaceID: "ws-1",
				Token:       "tok",
				Revoked:     false,
			}, nil
		},
	}
	svc := newTestSharingService(nil, nil, shareRepo, nil)
	err := svc.Revoke(context.Background(), "share-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRevoke_AlreadyRevoked(t *testing.T) {
	shareRepo := &mockShareRepo{
		getByIDFn: func(ctx context.Context, shareID string) (*ShareRecord, error) {
			return &ShareRecord{
				ID:      shareID,
				Revoked: true,
			}, nil
		},
	}
	svc := newTestSharingService(nil, nil, shareRepo, nil)
	err := svc.Revoke(context.Background(), "share-1", "user-1")
	if err != nil {
		t.Fatalf("expected idempotent success, got %v", err)
	}
}

func TestRevoke_NotFound(t *testing.T) {
	svc := newTestSharingService(nil, nil, &mockShareRepo{}, nil)
	err := svc.Revoke(context.Background(), "share-missing", "user-1")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestRevoke_EmptyShareID(t *testing.T) {
	svc := newTestSharingService(nil, nil, nil, nil)
	err := svc.Revoke(context.Background(), "", "user-1")
	if err == nil {
		t.Fatal("expected validation error for empty share_id")
	}
}

func TestRevoke_EmptyRevokedBy(t *testing.T) {
	svc := newTestSharingService(nil, nil, nil, nil)
	err := svc.Revoke(context.Background(), "share-1", "")
	if err == nil {
		t.Fatal("expected validation error for empty revoked_by")
	}
}

func TestRevoke_PermissionDenied(t *testing.T) {
	shareRepo := &mockShareRepo{
		getByIDFn: func(ctx context.Context, shareID string) (*ShareRecord, error) {
			return &ShareRecord{
				ID:          shareID,
				WorkspaceID: "ws-1",
				Revoked:     false,
			}, nil
		},
	}
	domainSvc := &mockCollabDomainService{
		checkMemberAccessFn: func(ctx context.Context, workspaceID, userID string, resource collabdomain.ResourceType, action collabdomain.Action) (bool, string, error) {
			return false, "denied", nil
		},
	}
	svc := newTestSharingService(domainSvc, nil, shareRepo, nil)
	err := svc.Revoke(context.Background(), "share-1", "user-no-perm")
	if err == nil {
		t.Fatal("expected permission denied error")
	}
}

// --- ListShares tests ---

func TestListShares_Success(t *testing.T) {
	now := time.Now()
	shareRepo := &mockShareRepo{
		listByWorkspaceFn: func(ctx context.Context, workspaceID string, includeRevoked bool, p commontypes.Pagination) ([]*ShareRecord, int, error) {
			return []*ShareRecord{
				{ID: "s1", WorkspaceID: workspaceID, CreatedAt: now},
				{ID: "s2", WorkspaceID: workspaceID, CreatedAt: now},
			}, 2, nil
		},
	}
	svc := newTestSharingService(nil, nil, shareRepo, nil)
	records, total, err := svc.ListShares(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestListShares_EmptyWorkspaceID(t *testing.T) {
	svc := newTestSharingService(nil, nil, nil, nil)
	_, _, err := svc.ListShares(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error for empty workspace_id")
	}
}

func TestListShares_WithOptions(t *testing.T) {
	var capturedIncludeRevoked bool
	var capturedPagination commontypes.Pagination
	shareRepo := &mockShareRepo{
		listByWorkspaceFn: func(ctx context.Context, workspaceID string, includeRevoked bool, p commontypes.Pagination) ([]*ShareRecord, int, error) {
			capturedIncludeRevoked = includeRevoked
			capturedPagination = p
			return nil, 0, nil
		},
	}
	svc := newTestSharingService(nil, nil, shareRepo, nil)
	_, _, _ = svc.ListShares(context.Background(), "ws-1",
		WithIncludeRevoked(true),
		WithPagination(commontypes.Pagination{Page: 3, PageSize: 50}),
	)
	if !capturedIncludeRevoked {
		t.Fatal("expected includeRevoked to be true")
	}
	if capturedPagination.Page != 3 || capturedPagination.PageSize != 50 {
		t.Fatalf("expected page=3 pageSize=50, got page=%d pageSize=%d", capturedPagination.Page, capturedPagination.PageSize)
	}
}

// --- GetShareLink tests ---

func TestGetShareLink_Success(t *testing.T) {
	shareRepo := &mockShareRepo{
		getByIDFn: func(ctx context.Context, shareID string) (*ShareRecord, error) {
			return &ShareRecord{
				ID:    shareID,
				Token: "abc123",
			}, nil
		},
	}
	svc := newTestSharingService(nil, nil, shareRepo, nil)
	link, err := svc.GetShareLink(context.Background(), "share-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link == "" {
		t.Fatal("expected non-empty link")
	}
}

func TestGetShareLink_CacheHit(t *testing.T) {
	cache := newMockCache()
	svc := newTestSharingService(nil, nil, nil, cache)

	// Pre-populate cache — we need to know the cache key format
	// The service uses shareLinkCacheKey which is "share:link:{shareID}"
	cache.store["share:link:share-cached"] = "https://test.keyip.io/share/cached-token"

	link, err := svc.GetShareLink(context.Background(), "share-cached")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link != "https://test.keyip.io/share/cached-token" {
		t.Fatalf("expected cached link, got %s", link)
	}
}

func TestGetShareLink_Revoked(t *testing.T) {
	shareRepo := &mockShareRepo{
		getByIDFn: func(ctx context.Context, shareID string) (*ShareRecord, error) {
			return &ShareRecord{
				ID:      shareID,
				Revoked: true,
			}, nil
		},
	}
	svc := newTestSharingService(nil, nil, shareRepo, nil)
	_, err := svc.GetShareLink(context.Background(), "share-revoked")
	if err == nil {
		t.Fatal("expected error for revoked share")
	}
}

func TestGetShareLink_Expired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	shareRepo := &mockShareRepo{
		getByIDFn: func(ctx context.Context, shareID string) (*ShareRecord, error) {
			return &ShareRecord{
				ID:        shareID,
				ExpiresAt: &past,
			}, nil
		},
	}
	svc := newTestSharingService(nil, nil, shareRepo, nil)
	_, err := svc.GetShareLink(context.Background(), "share-expired")
	if err == nil {
		t.Fatal("expected error for expired share")
	}
}

func TestGetShareLink_EmptyID(t *testing.T) {
	svc := newTestSharingService(nil, nil, nil, nil)
	_, err := svc.GetShareLink(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error for empty share_id")
	}
}

// --- ValidateShareToken tests ---

func TestValidateShareToken_Valid(t *testing.T) {
	// First create a share to get a valid token
	var capturedRecord *ShareRecord
	shareRepo := &mockShareRepo{
		createFn: func(ctx context.Context, record *ShareRecord) error {
			capturedRecord = record
			return nil
		},
	}
	svc := newTestSharingService(nil, nil, shareRepo, nil)

	resp, err := svc.Share(context.Background(), &ShareRequest{
		WorkspaceID: "ws-1",
		Permission:  SharePermissionReadOnly,
		Duration:    ShareDuration30Days,
		CreatedBy:   "user-1",
	})
	if err != nil {
		t.Fatalf("failed to create share: %v", err)
	}

	// Now set up the repo to return the record by token
	shareRepo.getByTokenFn = func(ctx context.Context, token string) (*ShareRecord, error) {
		if capturedRecord != nil && capturedRecord.Token == token {
			return capturedRecord, nil
		}
		return nil, errors.New("not found")
	}

	info, err := svc.ValidateShareToken(context.Background(), resp.Token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.WorkspaceID != "ws-1" {
		t.Fatalf("expected workspace ws-1, got %s", info.WorkspaceID)
	}
	if info.Permission != SharePermissionReadOnly {
		t.Fatalf("expected read_only, got %s", info.Permission)
	}
}

func TestValidateShareToken_EmptyToken(t *testing.T) {
	svc := newTestSharingService(nil, nil, nil, nil)
	_, err := svc.ValidateShareToken(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error for empty token")
	}
}

func TestValidateShareToken_InvalidFormat(t *testing.T) {
	svc := newTestSharingService(nil, nil, nil, nil)
	_, err := svc.ValidateShareToken(context.Background(), "not-a-valid-token")
	if err == nil {
		t.Fatal("expected error for invalid token format")
	}
}

func TestValidateShareToken_Revoked(t *testing.T) {
	var capturedRecord *ShareRecord
	shareRepo := &mockShareRepo{
		createFn: func(ctx context.Context, record *ShareRecord) error {
			capturedRecord = record
			return nil
		},
	}
	svc := newTestSharingService(nil, nil, shareRepo, nil)

	resp, _ := svc.Share(context.Background(), &ShareRequest{
		WorkspaceID: "ws-1",
		Permission:  SharePermissionReadOnly,
		Duration:    ShareDuration7Days,
		CreatedBy:   "user-1",
	})

	shareRepo.getByTokenFn = func(ctx context.Context, token string) (*ShareRecord, error) {
		r := *capturedRecord
		r.Revoked = true
		return &r, nil
	}

	_, err := svc.ValidateShareToken(context.Background(), resp.Token)
	if err == nil {
		t.Fatal("expected error for revoked token")
	}
}

func TestValidateShareToken_Expired(t *testing.T) {
	var capturedRecord *ShareRecord
	shareRepo := &mockShareRepo{
		createFn: func(ctx context.Context, record *ShareRecord) error {
			capturedRecord = record
			return nil
		},
	}
	svc := newTestSharingService(nil, nil, shareRepo, nil)

	resp, _ := svc.Share(context.Background(), &ShareRequest{
		WorkspaceID: "ws-1",
		Permission:  SharePermissionReadOnly,
		Duration:    ShareDuration7Days,
		CreatedBy:   "user-1",
	})

	shareRepo.getByTokenFn = func(ctx context.Context, token string) (*ShareRecord, error) {
		r := *capturedRecord
		past := time.Now().Add(-1 * time.Hour)
		r.ExpiresAt = &past
		return &r, nil
	}

	_, err := svc.ValidateShareToken(context.Background(), resp.Token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateShareToken_AccessLimitReached(t *testing.T) {
	var capturedRecord *ShareRecord
	shareRepo := &mockShareRepo{
		createFn: func(ctx context.Context, record *ShareRecord) error {
			capturedRecord = record
			return nil
		},
	}
	svc := newTestSharingService(nil, nil, shareRepo, nil)

	resp, _ := svc.Share(context.Background(), &ShareRequest{
		WorkspaceID:    "ws-1",
		Permission:     SharePermissionReadOnly,
		Duration:       ShareDuration7Days,
		CreatedBy:      "user-1",
		MaxAccessCount: 5,
	})

	shareRepo.getByTokenFn = func(ctx context.Context, token string) (*ShareRecord, error) {
		r := *capturedRecord
		r.AccessCount = 5
		return &r, nil
	}

	_, err := svc.ValidateShareToken(context.Background(), resp.Token)
	if err == nil {
		t.Fatal("expected error for access limit reached")
	}
}

func TestValidateShareToken_CacheHit(t *testing.T) {
	cache := newMockCache()

	// We need a valid token to pass signature verification, so we create one first
	var capturedRecord *ShareRecord
	shareRepo := &mockShareRepo{
		createFn: func(ctx context.Context, record *ShareRecord) error {
			capturedRecord = record
			return nil
		},
	}
	svc2 := newTestSharingService(nil, nil, shareRepo, cache)
	resp, _ := svc2.Share(context.Background(), &ShareRequest{
		WorkspaceID: "ws-1",
		Permission:  SharePermissionReadOnly,
		Duration:    ShareDuration7Days,
		CreatedBy:   "user-1",
	})

	// Manually populate cache with valid info
	info := &ShareInfo{
		ShareID:     capturedRecord.ID,
		WorkspaceID: "ws-1",
		Permission:  SharePermissionReadOnly,
		CreatedBy:   "user-1",
		IsRevoked:   false,
	}
	infoBytes, _ := json.Marshal(info)

	// Use the same service to get the cache key
	impl := svc2.(*sharingServiceImpl)
	tokenKey := impl.tokenCacheKey(resp.Token)
	cache.store[tokenKey] = string(infoBytes)

	result, err := svc2.ValidateShareToken(context.Background(), resp.Token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WorkspaceID != "ws-1" {
		t.Fatalf("expected ws-1, got %s", result.WorkspaceID)
	}
}

// --- Token round-trip test ---

func TestTokenGenerateAndVerify(t *testing.T) {
	impl := &sharingServiceImpl{
		config: SharingServiceConfig{
			TokenSecret: []byte("round-trip-test-secret-key!!!!!!"),
		},
	}

	future := time.Now().Add(24 * time.Hour)
	token, err := impl.generateToken("share-rt", "ws-rt", SharePermissionComment, &future)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	payload, err := impl.verifyToken(token)
	if err != nil {
		t.Fatalf("failed to verify token: %v", err)
	}

	if payload.ShareID != "share-rt" {
		t.Fatalf("expected share-rt, got %s", payload.ShareID)
	}
	if payload.WorkspaceID != "ws-rt" {
		t.Fatalf("expected ws-rt, got %s", payload.WorkspaceID)
	}
	if payload.Permission != SharePermissionComment {
		t.Fatalf("expected comment, got %s", payload.Permission)
	}
}

func TestTokenVerify_TamperedSignature(t *testing.T) {
	impl := &sharingServiceImpl{
		config: SharingServiceConfig{
			TokenSecret: []byte("tamper-test-secret-key!!!!!!!!!!"),
		},
	}

	token, _ := impl.generateToken("share-t", "ws-t", SharePermissionEdit, nil)
	// Tamper with the signature
	parts := make([]byte, len(token))
	copy(parts, token)
	parts[len(parts)-1] = 'X'

	_, err := impl.verifyToken(string(parts))
	if err == nil {
		t.Fatal("expected error for tampered signature")
	}
}

func TestSharePermission_IsValid(t *testing.T) {
	tests := []struct {
		perm  SharePermission
		valid bool
	}{
		{SharePermissionReadOnly, true},
		{SharePermissionComment, true},
		{SharePermissionEdit, true},
		{"admin", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := tt.perm.IsValid(); got != tt.valid {
			t.Errorf("SharePermission(%q).IsValid() = %v, want %v", tt.perm, got, tt.valid)
		}
	}
}

//Personal.AI order the ending
