package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	appErrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// Domain entities — local mirrors of internal/domain/collaboration
// ─────────────────────────────────────────────────────────────────────────────

// WorkspaceMember represents a single member entry stored inside the JSONB
// "members" column of the workspaces table.
type WorkspaceMember struct {
	UserID   string    `json:"user_id"`
	Role     string    `json:"role"` // owner | admin | editor | viewer
	JoinedAt time.Time `json:"joined_at"`
	InvitedBy string   `json:"invited_by,omitempty"`
}

// SharedResource represents a resource shared within a workspace, stored
// inside the JSONB "shared_resources" column.
type SharedResource struct {
	ResourceID   string    `json:"resource_id"`
	ResourceType string    `json:"resource_type"` // patent | portfolio | molecule | analysis
	SharedBy     string    `json:"shared_by"`
	SharedAt     time.Time `json:"shared_at"`
	Permissions  string    `json:"permissions"` // read | write | admin
	Notes        string    `json:"notes,omitempty"`
}

// Workspace is the aggregate root for the collaboration domain.
type Workspace struct {
	ID              common.ID
	TenantID        common.TenantID
	Name            string
	Description     string
	OwnerID         common.UserID
	Members         []WorkspaceMember
	SharedResources []SharedResource
	Status          string // active | archived | deleted
	Tags            []string
	Settings        map[string]interface{}
	Metadata        map[string]interface{}
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CreatedBy       common.UserID
	Version         int
}

// WorkspaceSearchCriteria carries dynamic filter parameters.
type WorkspaceSearchCriteria struct {
	Name     string
	OwnerID  string
	Status   string
	Tag      string
	Page     int
	PageSize int
}

// ─────────────────────────────────────────────────────────────────────────────
// CollaborationRepository
// ─────────────────────────────────────────────────────────────────────────────

// CollaborationRepository is the PostgreSQL implementation of the
// collaboration domain's Repository interface.  Members and shared resources
// are stored as JSONB columns, enabling rich containment queries.
type CollaborationRepository struct {
	pool   *pgxpool.Pool
	logger logging.Logger
}

// NewCollaborationRepository constructs a ready-to-use CollaborationRepository.
func NewCollaborationRepository(pool *pgxpool.Pool, logger logging.Logger) *CollaborationRepository {
	return &CollaborationRepository{pool: pool, logger: logger}
}

// ─────────────────────────────────────────────────────────────────────────────
// Save
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) Save(ctx context.Context, ws *Workspace) error {
	r.logger.Debug("CollaborationRepository.Save", logging.String("workspace_id", string(ws.ID)))

	membersJSON, _ := json.Marshal(ws.Members)
	resourcesJSON, _ := json.Marshal(ws.SharedResources)
	settingsJSON, _ := json.Marshal(ws.Settings)
	metaJSON, _ := json.Marshal(ws.Metadata)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO workspaces (
			id, tenant_id, name, description, owner_id,
			members, shared_resources, status, tags,
			settings, metadata,
			created_at, updated_at, created_by, version
		) VALUES (
			$1,$2,$3,$4,$5,
			$6,$7,$8,$9,
			$10,$11,
			$12,$13,$14,$15
		)`,
		ws.ID, ws.TenantID, ws.Name, ws.Description, ws.OwnerID,
		membersJSON, resourcesJSON, ws.Status, ws.Tags,
		settingsJSON, metaJSON,
		ws.CreatedAt, ws.UpdatedAt, ws.CreatedBy, ws.Version,
	)
	if err != nil {
		r.logger.Error("CollaborationRepository.Save", logging.Err(err))
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to insert workspace")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByID
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) FindByID(ctx context.Context, id common.ID) (*Workspace, error) {
	r.logger.Debug("CollaborationRepository.FindByID", logging.String("id", string(id)))

	return r.scanWorkspace(r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, description, owner_id,
		       members, shared_resources, status, tags,
		       settings, metadata,
		       created_at, updated_at, created_by, version
		FROM workspaces WHERE id = $1`, id))
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByOwner
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) FindByOwner(ctx context.Context, ownerID common.UserID, page, pageSize int) ([]*Workspace, int64, error) {
	r.logger.Debug("CollaborationRepository.FindByOwner", logging.String("owner_id", string(ownerID)))

	where := "WHERE owner_id = $1"

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM workspaces %s", where), ownerID,
	).Scan(&total); err != nil {
		r.logger.Error("CollaborationRepository.FindByOwner: count", logging.Err(err))
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "count failed")
	}

	if pageSize <= 0 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, name, description, owner_id,
		       members, shared_resources, status, tags,
		       settings, metadata,
		       created_at, updated_at, created_by, version
		FROM workspaces %s
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3`, where), ownerID, pageSize, offset)
	if err != nil {
		r.logger.Error("CollaborationRepository.FindByOwner: query", logging.Err(err))
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "query failed")
	}
	defer rows.Close()

	workspaces, err := r.scanWorkspaces(rows)
	return workspaces, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// FindWorkspacesByUser
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) FindWorkspacesByUser(
	ctx context.Context, userID common.UserID, page, pageSize int,
) ([]*Workspace, int64, error) {
	r.logger.Debug("CollaborationRepository.FindWorkspacesByUser", logging.String("user_id", string(userID)))

	// Build the JSONB containment pattern.
	pattern := fmt.Sprintf(`[{"user_id": "%s"}]`, string(userID))

	where := "WHERE members @> $1::jsonb"

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM workspaces %s", where), pattern,
	).Scan(&total); err != nil {
		r.logger.Error("CollaborationRepository.FindWorkspacesByUser: count", logging.Err(err))
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "count failed")
	}

	if pageSize <= 0 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, name, description, owner_id,
		       members, shared_resources, status, tags,
		       settings, metadata,
		       created_at, updated_at, created_by, version
		FROM workspaces %s
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3`, where), pattern, pageSize, offset)
	if err != nil {
		r.logger.Error("CollaborationRepository.FindWorkspacesByUser: query", logging.Err(err))
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "query failed")
	}
	defer rows.Close()

	workspaces, err := r.scanWorkspaces(rows)
	return workspaces, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// FindWorkspacesByResource
// ─────────────────────────────────────────────────────────────────────
func (r *CollaborationRepository) FindWorkspacesByResource(
	ctx context.Context, resourceID common.ID, page, pageSize int,
) ([]*Workspace, int64, error) {
	r.logger.Debug("CollaborationRepository.FindWorkspacesByResource", logging.String("resource_id", string(resourceID)))

	// Build the JSONB containment pattern.
	pattern := fmt.Sprintf(`[{"resource_id": "%s"}]`, string(resourceID))

	where := "WHERE shared_resources @> $1::jsonb"

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM workspaces %s", where), pattern,
	).Scan(&total); err != nil {
		r.logger.Error("CollaborationRepository.FindWorkspacesByResource: count", logging.Err(err))
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "count failed")
	}

	if pageSize <= 0 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, name, description, owner_id,
		       members, shared_resources, status, tags,
		       settings, metadata,
		       created_at, updated_at, created_by, version
		FROM workspaces %s
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3`, where), pattern, pageSize, offset)
	if err != nil {
		r.logger.Error("CollaborationRepository.FindWorkspacesByResource: query", logging.Err(err))
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "query failed")
	}
	defer rows.Close()

	workspaces, err := r.scanWorkspaces(rows)
	return workspaces, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// Search
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) Search(ctx context.Context, criteria WorkspaceSearchCriteria) ([]*Workspace, int64, error) {
	// Cannot log criteria directly as it's a struct and logging package might not support Any or requires conversion
	r.logger.Debug("CollaborationRepository.Search")

	var (
		conditions []string
		args       []interface{}
		argIdx     int
	)

	nextArg := func(v interface{}) string {
		argIdx++
		args = append(args, v)
		return fmt.Sprintf("$%d", argIdx)
	}

	if criteria.Name != "" {
		ph := nextArg("%" + strings.ToLower(criteria.Name) + "%")
		conditions = append(conditions, fmt.Sprintf("LOWER(name) LIKE %s", ph))
	}
	if criteria.OwnerID != "" {
		ph := nextArg(criteria.OwnerID)
		conditions = append(conditions, fmt.Sprintf("owner_id = %s", ph))
	}
	if criteria.Status != "" {
		ph := nextArg(criteria.Status)
		conditions = append(conditions, fmt.Sprintf("status = %s", ph))
	}
	if criteria.Tag != "" {
		ph := nextArg(criteria.Tag)
		conditions = append(conditions, fmt.Sprintf("tags @> ARRAY[%s]::TEXT[]", ph))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM workspaces %s", whereClause), args...,
	).Scan(&total); err != nil {
		r.logger.Error("CollaborationRepository.Search: count", logging.Err(err))
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "count failed")
	}

	pageSize := criteria.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	page := criteria.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	phLimit := nextArg(pageSize)
	phOffset := nextArg(offset)

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, name, description, owner_id,
		       members, shared_resources, status, tags,
		       settings, metadata,
		       created_at, updated_at, created_by, version
		FROM workspaces %s
		ORDER BY updated_at DESC
		LIMIT %s OFFSET %s`, whereClause, phLimit, phOffset), args...)
	if err != nil {
		r.logger.Error("CollaborationRepository.Search: query", logging.Err(err))
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "search query failed")
	}
	defer rows.Close()

	workspaces, err := r.scanWorkspaces(rows)
	return workspaces, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// Update — optimistic locking
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) Update(ctx context.Context, ws *Workspace) error {
	r.logger.Debug("CollaborationRepository.Update",
		logging.String("workspace_id", string(ws.ID)),
		logging.Int("version", ws.Version),
	)

	membersJSON, _ := json.Marshal(ws.Members)
	resourcesJSON, _ := json.Marshal(ws.SharedResources)
	settingsJSON, _ := json.Marshal(ws.Settings)
	metaJSON, _ := json.Marshal(ws.Metadata)
	newVersion := ws.Version + 1

	tag, err := r.pool.Exec(ctx, `
		UPDATE workspaces SET
			name=$1, description=$2, owner_id=$3,
			members=$4, shared_resources=$5, status=$6, tags=$7,
			settings=$8, metadata=$9,
			updated_at=$10, version=$11
		WHERE id=$12 AND version=$13`,
		ws.Name, ws.Description, ws.OwnerID,
		membersJSON, resourcesJSON, ws.Status, ws.Tags,
		settingsJSON, metaJSON,
		time.Now().UTC(), newVersion,
		ws.ID, ws.Version,
	)
	if err != nil {
		r.logger.Error("CollaborationRepository.Update", logging.Err(err))
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to update workspace")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeConflict, "optimistic lock conflict: workspace version mismatch")
	}
	ws.Version = newVersion
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// AddMember
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) AddMember(ctx context.Context, workspaceID common.ID, member WorkspaceMember) error {
	r.logger.Debug("CollaborationRepository.AddMember",
		logging.String("workspace_id", string(workspaceID)),
		logging.String("user_id", member.UserID),
	)

	memberJSON, _ := json.Marshal(member)

	// First check if the user is already a member to avoid duplicates.
	pattern := fmt.Sprintf(`[{"user_id": "%s"}]`, member.UserID)
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM workspaces WHERE id = $1 AND members @> $2::jsonb)`,
		workspaceID, pattern,
	).Scan(&exists); err != nil {
		r.logger.Error("CollaborationRepository.AddMember: check", logging.Err(err))
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to check membership")
	}
	if exists {
		return appErrors.New(appErrors.CodeConflict, "user is already a member of this workspace")
	}

	tag, err := r.pool.Exec(ctx, `
		UPDATE workspaces
		SET members = COALESCE(members, '[]'::jsonb) || $1::jsonb,
		    updated_at = NOW()
		WHERE id = $2`, string(memberJSON), workspaceID)
	if err != nil {
		r.logger.Error("CollaborationRepository.AddMember", logging.Err(err))
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to add member")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeNotFound, "workspace not found")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// RemoveMember
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) RemoveMember(ctx context.Context, workspaceID common.ID, userID common.UserID) error {
	r.logger.Debug("CollaborationRepository.RemoveMember",
		logging.String("workspace_id", string(workspaceID)),
		logging.String("user_id", string(userID)),
	)

	// Rebuild the JSONB array excluding the target user.
	tag, err := r.pool.Exec(ctx, `
		UPDATE workspaces
		SET members = (
			SELECT COALESCE(jsonb_agg(elem), '[]'::jsonb)
			FROM jsonb_array_elements(members) AS elem
			WHERE elem->>'user_id' != $1
		),
		updated_at = NOW()
		WHERE id = $2`, string(userID), workspaceID)
	if err != nil {
		r.logger.Error("CollaborationRepository.RemoveMember", logging.Err(err))
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to remove member")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeNotFound, "workspace not found")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// UpdateMemberRole
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) UpdateMemberRole(
	ctx context.Context, workspaceID common.ID, userID common.UserID, newRole string,
) error {
	r.logger.Debug("CollaborationRepository.UpdateMemberRole",
		logging.String("workspace_id", string(workspaceID)),
		logging.String("user_id", string(userID)),
		logging.String("role", newRole),
	)

	tag, err := r.pool.Exec(ctx, `
		UPDATE workspaces
		SET members = (
			SELECT jsonb_agg(
				CASE
					WHEN elem->>'user_id' = $1
					THEN elem || jsonb_build_object('role', $2)
					ELSE elem
				END
			)
			FROM jsonb_array_elements(members) AS elem
		),
		updated_at = NOW()
		WHERE id = $3`, string(userID), newRole, workspaceID)
	if err != nil {
		r.logger.Error("CollaborationRepository.UpdateMemberRole", logging.Err(err))
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to update member role")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeNotFound, "workspace not found")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ShareResource
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) ShareResource(ctx context.Context, workspaceID common.ID, res SharedResource) error {
	r.logger.Debug("CollaborationRepository.ShareResource",
		logging.String("workspace_id", string(workspaceID)),
		logging.String("resource_id", res.ResourceID),
	)

	// Check for duplicate.
	pattern := fmt.Sprintf(`[{"resource_id": "%s"}]`, res.ResourceID)
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM workspaces WHERE id = $1 AND shared_resources @> $2::jsonb)`,
		workspaceID, pattern,
	).Scan(&exists); err != nil {
		r.logger.Error("CollaborationRepository.ShareResource: check", logging.Err(err))
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to check resource sharing")
	}
	if exists {
		return appErrors.New(appErrors.CodeConflict, "resource is already shared in this workspace")
	}

	resJSON, _ := json.Marshal(res)

	tag, err := r.pool.Exec(ctx, `
		UPDATE workspaces
		SET shared_resources = COALESCE(shared_resources, '[]'::jsonb) || $1::jsonb,
		    updated_at = NOW()
		WHERE id = $2`, string(resJSON), workspaceID)
	if err != nil {
		r.logger.Error("CollaborationRepository.ShareResource", logging.Err(err))
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to share resource")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeNotFound, "workspace not found")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// UnshareResource
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) UnshareResource(ctx context.Context, workspaceID common.ID, resourceID common.ID) error {
	r.logger.Debug("CollaborationRepository.UnshareResource",
		logging.String("workspace_id", string(workspaceID)),
		logging.String("resource_id", string(resourceID)),
	)

	tag, err := r.pool.Exec(ctx, `
		UPDATE workspaces
		SET shared_resources = (
			SELECT COALESCE(jsonb_agg(elem), '[]'::jsonb)
			FROM jsonb_array_elements(shared_resources) AS elem
			WHERE elem->>'resource_id' != $1
		),
		updated_at = NOW()
		WHERE id = $2`, string(resourceID), workspaceID)
	if err != nil {
		r.logger.Error("CollaborationRepository.UnshareResource", logging.Err(err))
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to unshare resource")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeNotFound, "workspace not found")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Delete
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) Delete(ctx context.Context, id common.ID) error {
	r.logger.Debug("CollaborationRepository.Delete", logging.String("id", string(id)))

	tag, err := r.pool.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, id)
	if err != nil {
		r.logger.Error("CollaborationRepository.Delete", logging.Err(err))
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to delete workspace")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeNotFound, "workspace not found")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Count
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) Count(ctx context.Context) (int64, error) {
	r.logger.Debug("CollaborationRepository.Count")

	var count int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM workspaces`).Scan(&count); err != nil {
		r.logger.Error("CollaborationRepository.Count", logging.Err(err))
		return 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to count workspaces")
	}
	return count, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal scanners
// ─────────────────────────────────────────────────────────────────────────────

func (r *CollaborationRepository) scanWorkspace(row pgx.Row) (*Workspace, error) {
	var ws Workspace
	var membersJSON, resourcesJSON, settingsJSON, metaJSON []byte

	err := row.Scan(
		&ws.ID, &ws.TenantID, &ws.Name, &ws.Description, &ws.OwnerID,
		&membersJSON, &resourcesJSON, &ws.Status, &ws.Tags,
		&settingsJSON, &metaJSON,
		&ws.CreatedAt, &ws.UpdatedAt, &ws.CreatedBy, &ws.Version,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, appErrors.New(appErrors.CodeNotFound, "workspace not found")
		}
		r.logger.Error("scanWorkspace", logging.Err(err))
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to scan workspace")
	}

	if len(membersJSON) > 0 {
		_ = json.Unmarshal(membersJSON, &ws.Members)
	}
	if len(resourcesJSON) > 0 {
		_ = json.Unmarshal(resourcesJSON, &ws.SharedResources)
	}
	if len(settingsJSON) > 0 {
		_ = json.Unmarshal(settingsJSON, &ws.Settings)
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &ws.Metadata)
	}
	return &ws, nil
}

func (r *CollaborationRepository) scanWorkspaces(rows pgx.Rows) ([]*Workspace, error) {
	var workspaces []*Workspace
	for rows.Next() {
		var ws Workspace
		var membersJSON, resourcesJSON, settingsJSON, metaJSON []byte

		err := rows.Scan(
			&ws.ID, &ws.TenantID, &ws.Name, &ws.Description, &ws.OwnerID,
			&membersJSON, &resourcesJSON, &ws.Status, &ws.Tags,
			&settingsJSON, &metaJSON,
			&ws.CreatedAt, &ws.UpdatedAt, &ws.CreatedBy, &ws.Version,
		)
		if err != nil {
			r.logger.Error("scanWorkspaces", logging.Err(err))
			return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to scan workspace row")
		}

		if len(membersJSON) > 0 {
			_ = json.Unmarshal(membersJSON, &ws.Members)
		}
		if len(resourcesJSON) > 0 {
			_ = json.Unmarshal(resourcesJSON, &ws.SharedResources)
		}
		if len(settingsJSON) > 0 {
			_ = json.Unmarshal(settingsJSON, &ws.Settings)
		}
		if len(metaJSON) > 0 {
			_ = json.Unmarshal(metaJSON, &ws.Metadata)
		}
		workspaces = append(workspaces, &ws)
	}
	if err := rows.Err(); err != nil {
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "row iteration error")
	}
	return workspaces, nil
}

//Personal.AI order the ending
