package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/user"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type postgresUserRepo struct {
	conn *postgres.Connection
	tx   *sql.Tx
	log  logging.Logger
}

func NewPostgresUserRepo(conn *postgres.Connection, log logging.Logger) user.UserRepository {
	return &postgresUserRepo{
		conn: conn,
		log:  log,
	}
}

func (r *postgresUserRepo) executor() queryExecutor {
	if r.tx != nil {
		return r.tx
	}
	return r.conn.DB()
}

// User

func (r *postgresUserRepo) Create(ctx context.Context, u *user.User) error {
	query := `
		INSERT INTO users (
			email, username, display_name, password_hash, status, avatar_url, locale, timezone,
			mfa_enabled, preferences, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		) RETURNING id, created_at, updated_at
	`
	pref, _ := json.Marshal(u.Preferences)
	meta, _ := json.Marshal(u.Metadata)

	err := r.executor().QueryRowContext(ctx, query,
		u.Email, u.Username, u.DisplayName, u.PasswordHash, u.Status, u.AvatarURL, u.Locale, u.Timezone,
		u.MFAEnabled, pref, meta,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			if strings.Contains(pqErr.Message, "email") {
				return errors.New(errors.ErrCodeConflict, "email already exists")
			}
			return errors.New(errors.ErrCodeConflict, "username already exists")
		}
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create user")
	}
	return nil
}

func (r *postgresUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	// Not returning password_hash and mfa_secret
	query := `
		SELECT id, email, username, display_name, status, avatar_url, locale, timezone,
		       last_login_at, last_login_ip, login_count, failed_login_count, locked_until,
		       email_verified_at, mfa_enabled, preferences, metadata, created_at, updated_at, deleted_at
		FROM users WHERE id = $1 AND deleted_at IS NULL
	`
	row := r.executor().QueryRowContext(ctx, query, id)
	return scanUser(row)
}

func (r *postgresUserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	query := `
		SELECT id, email, username, display_name, status, avatar_url, locale, timezone,
		       last_login_at, last_login_ip, login_count, failed_login_count, locked_until,
		       email_verified_at, mfa_enabled, preferences, metadata, created_at, updated_at, deleted_at
		FROM users WHERE email = $1 AND deleted_at IS NULL
	`
	row := r.executor().QueryRowContext(ctx, query, email)
	return scanUser(row)
}

func (r *postgresUserRepo) GetByUsername(ctx context.Context, username string) (*user.User, error) {
	query := `
		SELECT id, email, username, display_name, status, avatar_url, locale, timezone,
		       last_login_at, last_login_ip, login_count, failed_login_count, locked_until,
		       email_verified_at, mfa_enabled, preferences, metadata, created_at, updated_at, deleted_at
		FROM users WHERE username = $1 AND deleted_at IS NULL
	`
	row := r.executor().QueryRowContext(ctx, query, username)
	return scanUser(row)
}

func (r *postgresUserRepo) GetByEmailForAuth(ctx context.Context, email string) (*user.User, error) {
	// Including password_hash and mfa_secret
	query := `SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL`
	row := r.executor().QueryRowContext(ctx, query, email)
	return scanUserForAuth(row)
}

func (r *postgresUserRepo) Update(ctx context.Context, u *user.User) error {
	query := `
		UPDATE users
		SET display_name = $1, status = $2, avatar_url = $3, locale = $4, timezone = $5,
		    preferences = $6, metadata = $7, updated_at = NOW()
		WHERE id = $8
	`
	pref, _ := json.Marshal(u.Preferences)
	meta, _ := json.Marshal(u.Metadata)

	res, err := r.executor().ExecContext(ctx, query,
		u.DisplayName, u.Status, u.AvatarURL, u.Locale, u.Timezone, pref, meta, u.ID,
	)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update user")
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "user not found")
	}
	return nil
}

func (r *postgresUserRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET deleted_at = NOW() WHERE id = $1`
	_, err := r.executor().ExecContext(ctx, query, id)
	return err
}

func (r *postgresUserRepo) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	query := `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.executor().ExecContext(ctx, query, passwordHash, id)
	return err
}

func (r *postgresUserRepo) UpdateLoginInfo(ctx context.Context, id uuid.UUID, ip string) error {
	query := `
		UPDATE users
		SET last_login_at = NOW(), last_login_ip = $1, login_count = login_count + 1, failed_login_count = 0
		WHERE id = $2
	`
	_, err := r.executor().ExecContext(ctx, query, ip, id)
	return err
}

func (r *postgresUserRepo) IncrementFailedLogin(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE users
		SET failed_login_count = failed_login_count + 1,
		    locked_until = CASE WHEN failed_login_count + 1 >= 5 THEN NOW() + INTERVAL '30 minutes' ELSE locked_until END
		WHERE id = $1
	`
	_, err := r.executor().ExecContext(ctx, query, id)
	return err
}

func (r *postgresUserRepo) UpdateMFA(ctx context.Context, id uuid.UUID, enabled bool, secret string) error {
	query := `UPDATE users SET mfa_enabled = $1, mfa_secret = $2, updated_at = NOW() WHERE id = $3`
	// secret might be empty string which becomes '' in DB, or NULL if I use sql.NullString.
	// Prompt says mfa_secret VARCHAR(256), nullable.
	// If secret is empty, pass nil?
	var sec interface{} = secret
	if !enabled { sec = nil }

	_, err := r.executor().ExecContext(ctx, query, enabled, sec, id)
	return err
}

func (r *postgresUserRepo) VerifyEmail(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET email_verified_at = NOW(), status = 'active', updated_at = NOW() WHERE id = $1`
	_, err := r.executor().ExecContext(ctx, query, id)
	return err
}

func (r *postgresUserRepo) List(ctx context.Context, filter user.ListFilter) ([]*user.User, int64, error) {
	baseQuery := `FROM users WHERE deleted_at IS NULL`
	args := []interface{}{}
	argIdx := 1

	if filter.Status != "" {
		baseQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.Email != "" {
		baseQuery += fmt.Sprintf(` AND email ILIKE $%d`, argIdx)
		args = append(args, "%"+filter.Email+"%")
		argIdx++
	}

	limit := filter.Limit
	if limit <= 0 { limit = 20 }
	if limit > 100 { limit = 100 }
	offset := filter.Offset
	if offset < 0 { offset = 0 }

	var total int64
	err := r.executor().QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count users")
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, email, username, display_name, status, avatar_url, locale, timezone,
		       last_login_at, last_login_ip, login_count, failed_login_count, locked_until,
		       email_verified_at, mfa_enabled, preferences, metadata, created_at, updated_at, deleted_at
		%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.executor().QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to list users")
	}
	defer rows.Close()

	var users []*user.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil { return nil, 0, err }
		users = append(users, u)
	}
	return users, total, nil
}

// Organization
func scanOrganization(row scanner) (*user.Organization, error) {
	o := &user.Organization{}
	var settings []byte

	err := row.Scan(
		&o.ID, &o.Name, &o.Slug, &o.Description, &o.LogoURL,
		&o.Plan, &o.MaxMembers, &o.MaxPatents,
		&settings, &o.CreatedAt, &o.UpdatedAt, &o.DeletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "organization not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan organization")
	}
	if len(settings) > 0 { _ = json.Unmarshal(settings, &o.Settings) }
	return o, nil
}

func (r *postgresUserRepo) CreateOrganization(ctx context.Context, org *user.Organization) error {
	query := `
		INSERT INTO organizations (name, slug, description, logo_url, plan, max_members, max_patents, settings)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`
	settings, _ := json.Marshal(org.Settings)
	err := r.executor().QueryRowContext(ctx, query,
		org.Name, org.Slug, org.Description, org.LogoURL,
		org.Plan, org.MaxMembers, org.MaxPatents, settings,
	).Scan(&org.ID, &org.CreatedAt, &org.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return errors.New(errors.ErrCodeConflict, "organization slug already exists")
		}
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create organization")
	}
	return nil
}

func (r *postgresUserRepo) GetOrganization(ctx context.Context, id uuid.UUID) (*user.Organization, error) {
	query := `SELECT * FROM organizations WHERE id = $1 AND deleted_at IS NULL`
	row := r.executor().QueryRowContext(ctx, query, id)
	return scanOrganization(row)
}

func (r *postgresUserRepo) GetOrganizationBySlug(ctx context.Context, slug string) (*user.Organization, error) {
	query := `SELECT * FROM organizations WHERE slug = $1 AND deleted_at IS NULL`
	row := r.executor().QueryRowContext(ctx, query, slug)
	return scanOrganization(row)
}

func (r *postgresUserRepo) UpdateOrganization(ctx context.Context, org *user.Organization) error {
	query := `
		UPDATE organizations
		SET name = $1, description = $2, logo_url = $3, plan = $4, max_members = $5, max_patents = $6, settings = $7, updated_at = NOW()
		WHERE id = $8
	`
	settings, _ := json.Marshal(org.Settings)
	res, err := r.executor().ExecContext(ctx, query,
		org.Name, org.Description, org.LogoURL, org.Plan, org.MaxMembers, org.MaxPatents, settings, org.ID,
	)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update organization")
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "organization not found")
	}
	return nil
}

func (r *postgresUserRepo) AddMember(ctx context.Context, orgID, userID uuid.UUID, role string, invitedBy uuid.UUID) error {
	query := `
		INSERT INTO organization_members (organization_id, user_id, role, invited_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (organization_id, user_id) DO UPDATE SET role = $3
	`
	_, err := r.executor().ExecContext(ctx, query, orgID, userID, role, invitedBy)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to add member")
	}
	return nil
}

func (r *postgresUserRepo) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	query := `DELETE FROM organization_members WHERE organization_id = $1 AND user_id = $2`
	_, err := r.executor().ExecContext(ctx, query, orgID, userID)
	return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to remove member")
}

func (r *postgresUserRepo) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	query := `UPDATE organization_members SET role = $1 WHERE organization_id = $2 AND user_id = $3`
	res, err := r.executor().ExecContext(ctx, query, role, orgID, userID)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update member role")
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "member not found")
	}
	return nil
}

func (r *postgresUserRepo) GetMembers(ctx context.Context, orgID uuid.UUID) ([]*user.OrganizationMember, error) {
	query := `SELECT organization_id, user_id, role, joined_at, invited_by FROM organization_members WHERE organization_id = $1 ORDER BY joined_at ASC`
	rows, err := r.executor().QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get members")
	}
	defer rows.Close()

	var members []*user.OrganizationMember
	for rows.Next() {
		m := &user.OrganizationMember{}
		var invitedBy uuid.NullUUID
		if err := rows.Scan(&m.OrganizationID, &m.UserID, &m.Role, &m.JoinedAt, &invitedBy); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan member")
		}
		if invitedBy.Valid {
			m.InvitedBy = &invitedBy.UUID
		}
		members = append(members, m)
	}
	return members, nil
}

func (r *postgresUserRepo) GetUserOrganizations(ctx context.Context, userID uuid.UUID) ([]*user.Organization, error) {
	query := `
		SELECT o.* FROM organizations o
		JOIN organization_members om ON o.id = om.organization_id
		WHERE om.user_id = $1 AND o.deleted_at IS NULL
		ORDER BY o.created_at DESC
	`
	rows, err := r.executor().QueryContext(ctx, query, userID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get user organizations")
	}
	defer rows.Close()

	var orgs []*user.Organization
	for rows.Next() {
		o, err := scanOrganization(rows)
		if err != nil { return nil, err }
		orgs = append(orgs, o)
	}
	return orgs, nil
}

func (r *postgresUserRepo) IsMember(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM organization_members WHERE organization_id = $1 AND user_id = $2)`
	var exists bool
	err := r.executor().QueryRowContext(ctx, query, orgID, userID).Scan(&exists)
	return exists, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to check membership")
}

func (r *postgresUserRepo) GetMemberRole(ctx context.Context, orgID, userID uuid.UUID) (string, error) {
	query := `SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`
	var role string
	err := r.executor().QueryRowContext(ctx, query, orgID, userID).Scan(&role)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New(errors.ErrCodeNotFound, "member not found")
		}
		return "", errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get member role")
	}
	return role, nil
}

// Role & Permission
func scanRole(row scanner) (*user.Role, error) {
	r := &user.Role{}
	var permissions []byte
	err := row.Scan(
		&r.ID, &r.Name, &r.DisplayName, &r.Description, &r.IsSystem,
		&permissions, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "role not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan role")
	}
	if len(permissions) > 0 { _ = json.Unmarshal(permissions, &r.Permissions) }
	return r, nil
}

func (r *postgresUserRepo) GetRole(ctx context.Context, id uuid.UUID) (*user.Role, error) {
	query := `SELECT * FROM roles WHERE id = $1`
	row := r.executor().QueryRowContext(ctx, query, id)
	return scanRole(row)
}

func (r *postgresUserRepo) GetRoleByName(ctx context.Context, name string) (*user.Role, error) {
	query := `SELECT * FROM roles WHERE name = $1`
	row := r.executor().QueryRowContext(ctx, query, name)
	return scanRole(row)
}

func (r *postgresUserRepo) AssignRole(ctx context.Context, userID, roleID uuid.UUID, orgID *uuid.UUID, grantedBy uuid.UUID) error {
	query := `
		INSERT INTO user_roles (user_id, role_id, organization_id, granted_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, role_id) WHERE organization_id IS NULL
		DO UPDATE SET granted_by = $4, granted_at = NOW()
	`
	_, err := r.executor().ExecContext(ctx, query, userID, roleID, orgID, grantedBy)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to assign role")
	}
	return nil
}

func (r *postgresUserRepo) RevokeRole(ctx context.Context, userID, roleID uuid.UUID, orgID *uuid.UUID) error {
	query := `DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2 AND (organization_id = $3 OR ($3 IS NULL AND organization_id IS NULL))`
	res, err := r.executor().ExecContext(ctx, query, userID, roleID, orgID)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to revoke role")
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "role assignment not found")
	}
	return nil
}

func (r *postgresUserRepo) GetUserRoles(ctx context.Context, userID uuid.UUID, orgID *uuid.UUID) ([]*user.Role, error) {
	query := `
		SELECT r.* FROM roles r
		JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1 AND (ur.organization_id = $2 OR ($2 IS NULL AND ur.organization_id IS NULL))
		ORDER BY r.name ASC
	`
	rows, err := r.executor().QueryContext(ctx, query, userID, orgID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get user roles")
	}
	defer rows.Close()

	var roles []*user.Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil { return nil, err }
		roles = append(roles, role)
	}
	return roles, nil
}

func (r *postgresUserRepo) GetUserPermissions(ctx context.Context, userID uuid.UUID, orgID *uuid.UUID) ([]string, error) {
	// Get all permissions from roles assigned to this user (global + org scope)
	query := `
		SELECT DISTINCT jsonb_array_elements_text(r.permissions) AS permission
		FROM roles r
		JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1 AND (ur.organization_id = $2 OR ($2 IS NULL AND ur.organization_id IS NULL))
		ORDER BY permission
	`
	rows, err := r.executor().QueryContext(ctx, query, userID, orgID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get user permissions")
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan permission")
		}
		permissions = append(permissions, perm)
	}
	return permissions, nil
}

func (r *postgresUserRepo) HasPermission(ctx context.Context, userID uuid.UUID, orgID *uuid.UUID, permission string) (bool, error) {
	// Check if any assigned role has the permission (including wildcard "*")
	query := `
		SELECT EXISTS(
			SELECT 1 FROM user_roles ur
			JOIN roles r ON ur.role_id = r.id
			WHERE ur.user_id = $1
			  AND (ur.organization_id = $2 OR ($2 IS NULL AND ur.organization_id IS NULL))
			  AND (r.permissions @> $3::jsonb OR r.permissions @> '["*"]'::jsonb)
		)
	`
	permBytes, _ := json.Marshal([]string{permission})
	var exists bool
	err := r.executor().QueryRowContext(ctx, query, userID, orgID, string(permBytes)).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to check permission")
	}
	return exists, nil
}

// API Key
func scanAPIKey(row scanner) (*user.APIKey, error) {
	k := &user.APIKey{}
	var orgID uuid.NullUUID
	var expiresAt, lastUsedAt sql.NullTime

	err := row.Scan(
		&k.ID, &k.UserID, &orgID, &k.Name, &k.KeyHash, &k.KeyPrefix,
		pq.Array(&k.Scopes), &k.RateLimit,
		&expiresAt, &lastUsedAt, &k.LastUsedIP,
		&k.IsActive, &k.CreatedAt, &k.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "API key not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan API key")
	}
	if orgID.Valid {
		k.OrganizationID = &orgID.UUID
	}
	if expiresAt.Valid { k.ExpiresAt = &expiresAt.Time }
	if lastUsedAt.Valid { k.LastUsedAt = &lastUsedAt.Time }
	return k, nil
}

func (r *postgresUserRepo) CreateAPIKey(ctx context.Context, key *user.APIKey) error {
	query := `
		INSERT INTO api_keys (user_id, organization_id, name, key_hash, key_prefix, scopes, rate_limit, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`
	err := r.executor().QueryRowContext(ctx, query,
		key.UserID, key.OrganizationID, key.Name, key.KeyHash, key.KeyPrefix,
		pq.Array(key.Scopes), key.RateLimit, key.ExpiresAt, key.IsActive,
	).Scan(&key.ID, &key.CreatedAt, &key.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create API key")
	}
	return nil
}

func (r *postgresUserRepo) GetAPIKeyByHash(ctx context.Context, keyHash string) (*user.APIKey, error) {
	query := `SELECT * FROM api_keys WHERE key_hash = $1`
	row := r.executor().QueryRowContext(ctx, query, keyHash)
	return scanAPIKey(row)
}

func (r *postgresUserRepo) GetAPIKeysByUser(ctx context.Context, userID uuid.UUID) ([]*user.APIKey, error) {
	query := `SELECT * FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.executor().QueryContext(ctx, query, userID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get API keys by user")
	}
	defer rows.Close()

	var keys []*user.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil { return nil, err }
		keys = append(keys, k)
	}
	return keys, nil
}

func (r *postgresUserRepo) UpdateAPIKeyLastUsed(ctx context.Context, id uuid.UUID, ip string) error {
	query := `UPDATE api_keys SET last_used_at = NOW(), last_used_ip = $1 WHERE id = $2`
	_, err := r.executor().ExecContext(ctx, query, ip, id)
	return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update API key last used")
}

func (r *postgresUserRepo) DeactivateAPIKey(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE api_keys SET is_active = FALSE, updated_at = NOW() WHERE id = $1`
	res, err := r.executor().ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to deactivate API key")
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "API key not found")
	}
	return nil
}

func (r *postgresUserRepo) DeleteAPIKey(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM api_keys WHERE id = $1`
	res, err := r.executor().ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to delete API key")
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "API key not found")
	}
	return nil
}

// Audit Log
func scanAuditLog(row scanner) (*user.AuditLog, error) {
	l := &user.AuditLog{}
	var userID, orgID uuid.NullUUID
	var beforeState, afterState, metadata []byte

	err := row.Scan(
		&l.ID, &userID, &orgID, &l.Action, &l.ResourceType, &l.ResourceID,
		&l.IPAddress, &l.UserAgent, &l.RequestID,
		&beforeState, &afterState, &metadata,
		&l.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "audit log not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan audit log")
	}
	if userID.Valid { l.UserID = &userID.UUID }
	if orgID.Valid { l.OrganizationID = &orgID.UUID }
	if len(beforeState) > 0 { _ = json.Unmarshal(beforeState, &l.BeforeState) }
	if len(afterState) > 0 { _ = json.Unmarshal(afterState, &l.AfterState) }
	if len(metadata) > 0 { _ = json.Unmarshal(metadata, &l.Metadata) }
	return l, nil
}

func (r *postgresUserRepo) CreateAuditLog(ctx context.Context, al *user.AuditLog) error {
	query := `
		INSERT INTO audit_logs (user_id, organization_id, action, resource_type, resource_id,
		                        ip_address, user_agent, request_id, before_state, after_state, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at
	`
	beforeState, _ := json.Marshal(al.BeforeState)
	afterState, _ := json.Marshal(al.AfterState)
	meta, _ := json.Marshal(al.Metadata)

	err := r.executor().QueryRowContext(ctx, query,
		al.UserID, al.OrganizationID, al.Action, al.ResourceType, al.ResourceID,
		al.IPAddress, al.UserAgent, al.RequestID,
		beforeState, afterState, meta,
	).Scan(&al.ID, &al.CreatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create audit log")
	}
	return nil
}

func (r *postgresUserRepo) GetAuditLogs(ctx context.Context, filter user.AuditLogFilter) ([]*user.AuditLog, int64, error) {
	baseQuery := `FROM audit_logs WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if filter.UserID != nil {
		baseQuery += fmt.Sprintf(` AND user_id = $%d`, argIdx)
		args = append(args, *filter.UserID)
		argIdx++
	}
	if filter.OrganizationID != nil {
		baseQuery += fmt.Sprintf(` AND organization_id = $%d`, argIdx)
		args = append(args, *filter.OrganizationID)
		argIdx++
	}
	if filter.Action != "" {
		baseQuery += fmt.Sprintf(` AND action = $%d`, argIdx)
		args = append(args, filter.Action)
		argIdx++
	}
	if filter.ResourceType != "" {
		baseQuery += fmt.Sprintf(` AND resource_type = $%d`, argIdx)
		args = append(args, filter.ResourceType)
		argIdx++
	}
	if filter.ResourceID != "" {
		baseQuery += fmt.Sprintf(` AND resource_id = $%d`, argIdx)
		args = append(args, filter.ResourceID)
		argIdx++
	}
	if filter.StartDate != nil {
		baseQuery += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
		args = append(args, *filter.StartDate)
		argIdx++
	}
	if filter.EndDate != nil {
		baseQuery += fmt.Sprintf(` AND created_at <= $%d`, argIdx)
		args = append(args, *filter.EndDate)
		argIdx++
	}

	var total int64
	err := r.executor().QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count audit logs")
	}

	limit := filter.Limit
	if limit <= 0 { limit = 50 }
	offset := filter.Offset
	if offset < 0 { offset = 0 }

	query := fmt.Sprintf("SELECT * %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", baseQuery, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.executor().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get audit logs")
	}
	defer rows.Close()

	var logs []*user.AuditLog
	for rows.Next() {
		l, err := scanAuditLog(rows)
		if err != nil { return nil, 0, err }
		logs = append(logs, l)
	}
	return logs, total, nil
}

func (r *postgresUserRepo) GetAuditLogsByResource(ctx context.Context, resourceType string, resourceID string, limit int) ([]*user.AuditLog, error) {
	if limit <= 0 { limit = 50 }

	query := `SELECT * FROM audit_logs WHERE resource_type = $1 AND resource_id = $2 ORDER BY created_at DESC LIMIT $3`
	rows, err := r.executor().QueryContext(ctx, query, resourceType, resourceID, limit)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get audit logs by resource")
	}
	defer rows.Close()

	var logs []*user.AuditLog
	for rows.Next() {
		l, err := scanAuditLog(rows)
		if err != nil { return nil, err }
		logs = append(logs, l)
	}
	return logs, nil
}

func (r *postgresUserRepo) GetUserActivitySummary(ctx context.Context, userID uuid.UUID, startDate, endDate time.Time) (*user.ActivitySummary, error) {
	query := `
		SELECT action, COUNT(*) AS count
		FROM audit_logs
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
		GROUP BY action
		ORDER BY count DESC
	`
	rows, err := r.executor().QueryContext(ctx, query, userID, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get user activity summary")
	}
	defer rows.Close()

	summary := &user.ActivitySummary{
		UserID:       userID,
		ActionCounts: make(map[string]int64),
	}
	for rows.Next() {
		var action string
		var count int64
		if err := rows.Scan(&action, &count); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan activity summary")
		}
		summary.ActionCounts[action] = count
	}
	return summary, nil
}

func (r *postgresUserRepo) PurgeAuditLogs(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `DELETE FROM audit_logs WHERE created_at < $1`
	res, err := r.executor().ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to purge audit logs")
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

// Transaction
func (r *postgresUserRepo) WithTx(ctx context.Context, fn func(user.UserRepository) error) error {
	tx, err := r.conn.DB().BeginTx(ctx, nil)
	if err != nil { return err }
	txRepo := &postgresUserRepo{conn: r.conn, tx: tx, log: r.log}
	if err := fn(txRepo); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Scanners
func scanUser(row scanner) (*user.User, error) {
	u := &user.User{}
	var pref, meta []byte
	err := row.Scan(
		&u.ID, &u.Email, &u.Username, &u.DisplayName, &u.Status, &u.AvatarURL, &u.Locale, &u.Timezone,
		&u.LastLoginAt, &u.LastLoginIP, &u.LoginCount, &u.FailedLoginCount, &u.LockedUntil,
		&u.EmailVerifiedAt, &u.MFAEnabled, &pref, &meta, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "user not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan user")
	}
	if len(pref) > 0 { _ = json.Unmarshal(pref, &u.Preferences) }
	if len(meta) > 0 { _ = json.Unmarshal(meta, &u.Metadata) }
	return u, nil
}

func scanUserForAuth(row scanner) (*user.User, error) {
	// Same as scanUser but includes password_hash and mfa_secret
	u := &user.User{}
	// Columns: id, email, username, display_name, password_hash, status, avatar_url, locale, timezone,
	// last_login_at, last_login_ip, login_count, failed_login_count, locked_until,
	// email_verified_at, mfa_enabled, mfa_secret, preferences, metadata, created_at, updated_at, deleted_at
	var pref, meta []byte
	var pwd, mfaSecret sql.NullString

	err := row.Scan(
		&u.ID, &u.Email, &u.Username, &u.DisplayName, &pwd, &u.Status, &u.AvatarURL, &u.Locale, &u.Timezone,
		&u.LastLoginAt, &u.LastLoginIP, &u.LoginCount, &u.FailedLoginCount, &u.LockedUntil,
		&u.EmailVerifiedAt, &u.MFAEnabled, &mfaSecret, &pref, &meta, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "user not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan user")
	}
	if pwd.Valid { u.PasswordHash = pwd.String }
	if mfaSecret.Valid { u.MFASecret = mfaSecret.String }
	if len(pref) > 0 { _ = json.Unmarshal(pref, &u.Preferences) }
	if len(meta) > 0 { _ = json.Unmarshal(meta, &u.Metadata) }
	return u, nil
}

//Personal.AI order the ending
