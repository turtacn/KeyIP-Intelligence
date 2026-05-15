package collaboration

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// minimalSharingService is an in-memory implementation of SharingService.
type minimalSharingService struct {
	logger logging.Logger
	shares map[string]*SharedDocument
}

func NewMinimalSharingService(logger logging.Logger) SharingService {
	return &minimalSharingService{
		logger: logger,
		shares: map[string]*SharedDocument{},
	}
}

func (s *minimalSharingService) Share(ctx context.Context, req *ShareRequest) (*ShareResponse, error) {
	id := uuid.New().String()
	token := uuid.New().String()
	var expiry *time.Time
	if req.CustomExpiry != nil { expiry = req.CustomExpiry }
	return &ShareResponse{
		ShareID: id, Token: token, Link: "https://keyip.io/share/" + token,
		ExpiresAt: expiry, Permission: req.Permission, CreatedAt: time.Now(),
	}, nil
}

func (s *minimalSharingService) Revoke(ctx context.Context, shareID, revokedBy string) error {
	return nil
}

func (s *minimalSharingService) ListShares(ctx context.Context, workspaceID string, opts ...ListSharesOption) ([]*ShareRecord, int, error) {
	return []*ShareRecord{}, 0, nil
}

func (s *minimalSharingService) GetShareLink(ctx context.Context, shareID string) (string, error) {
	return "https://keyip.io/share/" + shareID, nil
}

func (s *minimalSharingService) ValidateShareToken(ctx context.Context, token string) (*ShareInfo, error) {
	return &ShareInfo{
		ShareID: token, IsRevoked: false,
		Permission: SharePermissionReadOnly, CreatedBy: "system",
	}, nil
}

func (s *minimalSharingService) ShareDocument(ctx context.Context, input *ShareDocumentInput) (*SharedDocument, error) {
	id := uuid.New().String()
	var expiry *time.Time
	if input.ExpiresInHours > 0 {
		t := time.Now().Add(time.Duration(input.ExpiresInHours) * time.Hour)
		expiry = &t
	}
	doc := &SharedDocument{
		ID: id, WorkspaceID: input.WorkspaceID, DocumentID: input.DocumentID,
		SharedByUserID: input.SharedByUserID, EnableWatermark: input.EnableWatermark,
		MaxDownloads: input.MaxDownloads, ExpiresAt: expiry, CreatedAt: time.Now(),
	}
	s.shares[id] = doc
	return doc, nil
}

func (s *minimalSharingService) ListDocuments(ctx context.Context, input *ListSharedDocumentsInput) (*ListSharedDocumentsResult, error) {
	var docs []*SharedDocument
	for _, d := range s.shares { docs = append(docs, d) }
	if docs == nil { docs = []*SharedDocument{} }
	return &ListSharedDocumentsResult{Documents: docs, Total: len(docs)}, nil
}

var _ SharingService = (*minimalSharingService)(nil)
