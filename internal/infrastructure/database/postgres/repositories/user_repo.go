package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/user"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type postgresUserRepo struct {
	conn     *postgres.Connection
	log      logging.Logger
	executor queryExecutor
}

func NewPostgresUserRepo(conn *postgres.Connection, log logging.Logger) user.UserRepository {
	return &postgresUserRepo{
		conn:     conn,
		log:      log,
		executor: conn.DB(),
	}
}

// WithTx implementation
func (r *postgresUserRepo) WithTx(ctx context.Context, fn func(user.UserRepository) error) error {
	tx, err := r.conn.DB().BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to begin transaction")
	}

	txRepo := &postgresUserRepo{
		conn:     r.conn,
		log:      r.log,
		executor: tx,
	}

	if err := fn(txRepo); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to commit transaction")
	}
	return nil
}

// User CRUD

func (r *postgresUserRepo) Create(ctx context.Context, u *user.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	query := `
		INSERT INTO users (
			id, email, username, display_name, password_hash, status, avatar_url, locale, timezone,
			mfa_enabled, mfa_secret, preferences, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING created_at, updated_at
	`
	prefJSON, _ := json.Marshal(u.Preferences)
	metaJSON, _ := json.Marshal(u.Metadata)

	err := r.executor.QueryRowContext(ctx, query,
		u.ID, u.Email, u.Username, u.DisplayName, u.PasswordHash, u.Status, u.AvatarURL, u.Locale, u.Timezone,
		u.MFAEnabled, u.MFASecret, prefJSON, metaJSON,
	).Scan(&u.CreatedAt, &u.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			if pqErr.Constraint == "users_email_key" {
				return errors.Wrap(err, errors.ErrCodeConflict, "email already exists")
			}
			if pqErr.Constraint == "users_username_key" {
				return errors.Wrap(err, errors.ErrCodeConflict, "username already exists")
			}
			return errors.Wrap(err, errors.ErrCodeConflict, "user already exists")
		}
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create user")
	}
	return nil
}

func (r *postgresUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	query := `SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL`
	row := r.executor.QueryRowContext(ctx, query, id)
	return scanUser(row, false)
}

func (r *postgresUserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	query := `SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL`
	row := r.executor.QueryRowContext(ctx, query, email)
	return scanUser(row, false)
}

func (r *postgresUserRepo) GetByUsername(ctx context.Context, username string) (*user.User, error) {
	query := `SELECT * FROM users WHERE username = $1 AND deleted_at IS NULL`
	row := r.executor.QueryRowContext(ctx, query, username)
	return scanUser(row, false)
}

func (r *postgresUserRepo) GetByEmailForAuth(ctx context.Context, email string) (*user.User, error) {
	query := `SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL`
	row := r.executor.QueryRowContext(ctx, query, email)
	return scanUser(row, true)
}

func (r *postgresUserRepo) Update(ctx context.Context, u *user.User) error {
	query := `
		UPDATE users SET
			email = $2, username = $3, display_name = $4, status = $5, avatar_url = $6,
			locale = $7, timezone = $8, preferences = $9, metadata = $10, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
	prefJSON, _ := json.Marshal(u.Preferences)
	metaJSON, _ := json.Marshal(u.Metadata)

	res, err := r.executor.ExecContext(ctx, query,
		u.ID, u.Email, u.Username, u.DisplayName, u.Status, u.AvatarURL,
		u.Locale, u.Timezone, prefJSON, metaJSON,
	)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update user")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.ErrNotFound("user", u.ID.String())
	}
	return nil
}

func (r *postgresUserRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET deleted_at = NOW(), status = 'inactive' WHERE id = $1 AND deleted_at IS NULL`
	res, err := r.executor.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to delete user")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.ErrNotFound("user", id.String())
	}
	return nil
}

func (r *postgresUserRepo) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	query := `UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.executor.ExecContext(ctx, query, id, passwordHash)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update password")
	}
	return nil
}

func (r *postgresUserRepo) UpdateLoginInfo(ctx context.Context, id uuid.UUID, ip string) error {
	query := `
		UPDATE users SET
			last_login_at = NOW(), last_login_ip = $2, login_count = login_count + 1, failed_login_count = 0, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.executor.ExecContext(ctx, query, id, ip)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update login info")
	}
	return nil
}

func (r *postgresUserRepo) IncrementFailedLogin(ctx context.Context, id uuid.UUID) error {
	// Simplified: just increment. Application logic decides locking.
	// But requirement says: "若达到阈值设置 locked_until".
	// Since repository doesn't know threshold, I'll just increment.
	// OR: I can set locked_until if failed_login_count > 5 in SQL.
	query := `
		UPDATE users SET
			failed_login_count = failed_login_count + 1,
			locked_until = CASE WHEN failed_login_count + 1 >= 5 THEN NOW() + INTERVAL '30 minutes' ELSE locked_until END,
			updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.executor.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to increment failed login")
	}
	return nil
}

func (r *postgresUserRepo) UpdateMFA(ctx context.Context, id uuid.UUID, enabled bool, secret string) error {
	query := `UPDATE users SET mfa_enabled = $2, mfa_secret = $3, updated_at = NOW() WHERE id = $1`
	var sec *string
	if secret != "" { sec = &secret }
	if !enabled { sec = nil }

	_, err := r.executor.ExecContext(ctx, query, id, enabled, sec)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update MFA")
	}
	return nil
}

func (r *postgresUserRepo) VerifyEmail(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET email_verified_at = NOW(), status = 'active', updated_at = NOW() WHERE id = $1`
	_, err := r.executor.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to verify email")
	}
	return nil
}

func (r *postgresUserRepo) List(ctx context.Context, filter user.ListFilter) ([]*user.User, int64, error) {
	where := "WHERE deleted_at IS NULL"
	args := []interface{}{}

	if filter.Status != "" {
		where += " AND status = $1"
		args = append(args, filter.Status)
	}
	if filter.Email != "" {
		where += fmt.Sprintf(" AND email ILIKE $%d", len(args)+1)
		args = append(args, "%"+filter.Email+"%")
	}

	countQuery := "SELECT COUNT(*) FROM users " + where
	var total int64
	if err := r.executor.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT * FROM users ` + where + ` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`
	query = fmt.Sprintf(query, len(args)+1, len(args)+2)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*user.User
	for rows.Next() {
		u, err := scanUser(rows, false)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}

// Organization

func (r *postgresUserRepo) CreateOrganization(ctx context.Context, org *user.Organization) error {
	if org.ID == uuid.Nil {
		org.ID = uuid.New()
	}
	query := `
		INSERT INTO organizations (
			id, name, slug, description, logo_url, plan, max_members, max_patents, settings
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`
	settingsJSON, _ := json.Marshal(org.Settings)

	err := r.executor.QueryRowContext(ctx, query,
		org.ID, org.Name, org.Slug, org.Description, org.LogoURL, org.Plan, org.MaxMembers, org.MaxPatents, settingsJSON,
	).Scan(&org.CreatedAt, &org.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create organization")
	}
	return nil
}

func (r *postgresUserRepo) GetOrganization(ctx context.Context, id uuid.UUID) (*user.Organization, error) {
	query := `SELECT * FROM organizations WHERE id = $1 AND deleted_at IS NULL`
	row := r.executor.QueryRowContext(ctx, query, id)
	return scanOrganization(row)
}

func (r *postgresUserRepo) GetOrganizationBySlug(ctx context.Context, slug string) (*user.Organization, error) {
	query := `SELECT * FROM organizations WHERE slug = $1 AND deleted_at IS NULL`
	row := r.executor.QueryRowContext(ctx, query, slug)
	return scanOrganization(row)
}

func (r *postgresUserRepo) UpdateOrganization(ctx context.Context, org *user.Organization) error {
	query := `
		UPDATE organizations SET
			name = $2, description = $3, logo_url = $4, plan = $5, max_members = $6, max_patents = $7, settings = $8, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
	settingsJSON, _ := json.Marshal(org.Settings)
	_, err := r.executor.ExecContext(ctx, query,
		org.ID, org.Name, org.Description, org.LogoURL, org.Plan, org.MaxMembers, org.MaxPatents, settingsJSON,
	)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update organization")
	}
	return nil
}

func (r *postgresUserRepo) AddMember(ctx context.Context, orgID, userID uuid.UUID, role string, invitedBy uuid.UUID) error {
	query := `INSERT INTO organization_members (organization_id, user_id, role, invited_by) VALUES ($1, $2, $3, $4)`
	_, err := r.executor.ExecContext(ctx, query, orgID, userID, role, invitedBy)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return errors.Wrap(err, errors.ErrCodeConflict, "user already in organization")
		}
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to add member")
	}
	return nil
}

func (r *postgresUserRepo) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	query := `DELETE FROM organization_members WHERE organization_id = $1 AND user_id = $2`
	_, err := r.executor.ExecContext(ctx, query, orgID, userID)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to remove member")
	}
	return nil
}

func (r *postgresUserRepo) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	query := `UPDATE organization_members SET role = $3 WHERE organization_id = $1 AND user_id = $2`
	_, err := r.executor.ExecContext(ctx, query, orgID, userID, role)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update member role")
	}
	return nil
}

func (r *postgresUserRepo) GetMembers(ctx context.Context, orgID uuid.UUID) ([]*user.OrganizationMember, error) {
	query := `SELECT * FROM organization_members WHERE organization_id = $1 ORDER BY joined_at DESC`
	rows, err := r.executor.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []*user.OrganizationMember
	for rows.Next() {
		m, err := scanOrgMember(rows)
		if err != nil { return nil, err }
		members = append(members, m)
	}
	return members, nil
}

func (r *postgresUserRepo) GetUserOrganizations(ctx context.Context, userID uuid.UUID) ([]*user.Organization, error) {
	query := `
		SELECT o.* FROM organizations o
		JOIN organization_members m ON o.id = m.organization_id
		WHERE m.user_id = $1 AND o.deleted_at IS NULL
	`
	rows, err := r.executor.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
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
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM organization_members WHERE organization_id = $1 AND user_id = $2)`
	err := r.executor.QueryRowContext(ctx, query, orgID, userID).Scan(&exists)
	return exists, err
}

func (r *postgresUserRepo) GetMemberRole(ctx context.Context, orgID, userID uuid.UUID) (string, error) {
	query := `SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`
	var role string
	err := r.executor.QueryRowContext(ctx, query, orgID, userID).Scan(&role)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New(errors.ErrCodeNotFound, "member not found")
		}
		return "", err
	}
	return role, nil
}

// Role & Permission

func (r *postgresUserRepo) GetRole(ctx context.Context, id uuid.UUID) (*user.Role, error) {
	query := `SELECT * FROM roles WHERE id = $1`
	row := r.executor.QueryRowContext(ctx, query, id)
	return scanRole(row)
}

func (r *postgresUserRepo) GetRoleByName(ctx context.Context, name string) (*user.Role, error) {
	query := `SELECT * FROM roles WHERE name = $1`
	row := r.executor.QueryRowContext(ctx, query, name)
	return scanRole(row)
}

func (r *postgresUserRepo) AssignRole(ctx context.Context, userID, roleID uuid.UUID, orgID *uuid.UUID, grantedBy uuid.UUID) error {
	query := `INSERT INTO user_roles (user_id, role_id, organization_id, granted_by) VALUES ($1, $2, $3, $4)`
	_, err := r.executor.ExecContext(ctx, query, userID, roleID, orgID, grantedBy)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to assign role")
	}
	return nil
}

func (r *postgresUserRepo) RevokeRole(ctx context.Context, userID, roleID uuid.UUID, orgID *uuid.UUID) error {
	query := `DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2 AND organization_id IS NOT DISTINCT FROM $3`
	_, err := r.executor.ExecContext(ctx, query, userID, roleID, orgID)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to revoke role")
	}
	return nil
}

func (r *postgresUserRepo) GetUserRoles(ctx context.Context, userID uuid.UUID, orgID *uuid.UUID) ([]*user.Role, error) {
	query := `
		SELECT r.* FROM roles r
		JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1 AND ur.organization_id IS NOT DISTINCT FROM $2
	`
	rows, err := r.executor.QueryContext(ctx, query, userID, orgID)
	if err != nil {
		return nil, err
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
	// Aggregate permissions from all roles (including global roles if orgID is not nil?)
	// Usually permissions are cumulative.
	// Here we check roles assigned for this org + global roles (org_id IS NULL) if we want to include system roles.
	// But requirement says: "GetUserPermissions... 聚合用户所有角色的权限"

	query := `
		SELECT r.permissions
		FROM roles r
		JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1 AND (ur.organization_id = $2 OR ur.organization_id IS NULL)
	`
	rows, err := r.executor.QueryContext(ctx, query, userID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permMap := make(map[string]bool)
	for rows.Next() {
		var permJSON []byte
		if err := rows.Scan(&permJSON); err != nil {
			return nil, err
		}
		var perms []string
		_ = json.Unmarshal(permJSON, &perms)
		for _, p := range perms {
			permMap[p] = true
		}
	}

	result := make([]string, 0, len(permMap))
	for p := range permMap {
		result = append(result, p)
	}
	return result, nil
}

func (r *postgresUserRepo) HasPermission(ctx context.Context, userID uuid.UUID, orgID *uuid.UUID, permission string) (bool, error) {
	perms, err := r.GetUserPermissions(ctx, userID, orgID)
	if err != nil {
		return false, err
	}
	for _, p := range perms {
		if p == "*" {
			return true, nil
		}
		if p == permission {
			return true, nil
		}
		// Prefix wildcard: "patent:*" matches "patent:read"
		if len(p) > 2 && p[len(p)-2:] == ":*" {
			prefix := p[:len(p)-2]
			if len(permission) > len(prefix) && permission[:len(prefix)] == prefix {
				return true, nil
			}
		}
	}
	return false, nil
}

// API Key

func (r *postgresUserRepo) CreateAPIKey(ctx context.Context, key *user.APIKey) error {
	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}
	query := `
		INSERT INTO api_keys (
			id, user_id, organization_id, name, key_hash, key_prefix, scopes, rate_limit, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`
	err := r.executor.QueryRowContext(ctx, query,
		key.ID, key.UserID, key.OrganizationID, key.Name, key.KeyHash, key.KeyPrefix, pq.Array(key.Scopes), key.RateLimit, key.ExpiresAt,
	).Scan(&key.CreatedAt, &key.UpdatedAt)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create api key")
	}
	return nil
}

func (r *postgresUserRepo) GetAPIKeyByHash(ctx context.Context, keyHash string) (*user.APIKey, error) {
	query := `SELECT * FROM api_keys WHERE key_hash = $1 AND is_active = TRUE`
	row := r.executor.QueryRowContext(ctx, query, keyHash)
	return scanAPIKey(row)
}

func (r *postgresUserRepo) GetAPIKeysByUser(ctx context.Context, userID uuid.UUID) ([]*user.APIKey, error) {
	query := `SELECT * FROM api_keys WHERE user_id = $1`
	rows, err := r.executor.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
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
	query := `UPDATE api_keys SET last_used_at = NOW(), last_used_ip = $2 WHERE id = $1`
	_, err := r.executor.ExecContext(ctx, query, id, ip)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update api key usage")
	}
	return nil
}

func (r *postgresUserRepo) DeactivateAPIKey(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE api_keys SET is_active = FALSE, updated_at = NOW() WHERE id = $1`
	_, err := r.executor.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to deactivate api key")
	}
	return nil
}

func (r *postgresUserRepo) DeleteAPIKey(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM api_keys WHERE id = $1`
	_, err := r.executor.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to delete api key")
	}
	return nil
}

// Audit Log

func (r *postgresUserRepo) CreateAuditLog(ctx context.Context, log *user.AuditLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	query := `
		INSERT INTO audit_logs (
			id, user_id, organization_id, action, resource_type, resource_id,
			ip_address, user_agent, request_id, before_state, after_state, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at
	`
	beforeJSON, _ := json.Marshal(log.BeforeState)
	afterJSON, _ := json.Marshal(log.AfterState)
	metaJSON, _ := json.Marshal(log.Metadata)

	err := r.executor.QueryRowContext(ctx, query,
		log.ID, log.UserID, log.OrganizationID, log.Action, log.ResourceType, log.ResourceID,
		log.IPAddress, log.UserAgent, log.RequestID, beforeJSON, afterJSON, metaJSON,
	).Scan(&log.CreatedAt)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create audit log")
	}
	return nil
}

func (r *postgresUserRepo) GetAuditLogs(ctx context.Context, filter user.AuditLogFilter) ([]*user.AuditLog, int64, error) {
	where := "WHERE 1=1"
	args := []interface{}{}

	if filter.UserID != nil {
		where += fmt.Sprintf(" AND user_id = $%d", len(args)+1)
		args = append(args, filter.UserID)
	}
	if filter.Action != "" {
		where += fmt.Sprintf(" AND action = $%d", len(args)+1)
		args = append(args, filter.Action)
	}

	countQuery := "SELECT COUNT(*) FROM audit_logs " + where
	var total int64
	r.executor.QueryRowContext(ctx, countQuery, args...).Scan(&total)

	query := `SELECT * FROM audit_logs ` + where + ` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`
	query = fmt.Sprintf(query, len(args)+1, len(args)+2)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
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
	query := `SELECT * FROM audit_logs WHERE resource_type = $1 AND resource_id = $2 ORDER BY created_at DESC LIMIT $3`
	rows, err := r.executor.QueryContext(ctx, query, resourceType, resourceID, limit)
	if err != nil {
		return nil, err
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
	query := `SELECT action, COUNT(*) FROM audit_logs WHERE user_id = $1 AND created_at BETWEEN $2 AND $3 GROUP BY action`
	rows, err := r.executor.QueryContext(ctx, query, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	summary := &user.ActivitySummary{
		UserID: userID,
		ActionCounts: make(map[string]int64),
	}
	for rows.Next() {
		var action string
		var count int64
		rows.Scan(&action, &count)
		summary.ActionCounts[action] = count
	}
	return summary, nil
}

func (r *postgresUserRepo) PurgeAuditLogs(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `DELETE FROM audit_logs WHERE created_at < $1`
	res, err := r.executor.ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Helpers

func scanUser(row scanner, withPassword bool) (*user.User, error) {
	var u user.User
	var prefJSON, metaJSON []byte
	var pwdHash sql.NullString
	var mfaSecret sql.NullString

	err := row.Scan(
		&u.ID, &u.Email, &u.Username, &u.DisplayName, &pwdHash, &u.Status, &u.AvatarURL,
		&u.Locale, &u.Timezone, &u.LastLoginAt, &u.LastLoginIP, &u.LoginCount, &u.FailedLoginCount,
		&u.LockedUntil, &u.EmailVerifiedAt, &u.MFAEnabled, &mfaSecret, &prefJSON, &metaJSON,
		&u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "user not found")
		}
		return nil, err
	}
	if withPassword && pwdHash.Valid {
		u.PasswordHash = pwdHash.String
	}
	if mfaSecret.Valid {
		u.MFASecret = mfaSecret.String
	}
	if len(prefJSON) > 0 { _ = json.Unmarshal(prefJSON, &u.Preferences) }
	if len(metaJSON) > 0 { _ = json.Unmarshal(metaJSON, &u.Metadata) }
	return &u, nil
}

func scanOrganization(row scanner) (*user.Organization, error) {
	var o user.Organization
	var settingsJSON []byte
	err := row.Scan(
		&o.ID, &o.Name, &o.Slug, &o.Description, &o.LogoURL, &o.Plan, &o.MaxMembers, &o.MaxPatents, &settingsJSON,
		&o.CreatedAt, &o.UpdatedAt, &o.DeletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "organization not found")
		}
		return nil, err
	}
	if len(settingsJSON) > 0 { _ = json.Unmarshal(settingsJSON, &o.Settings) }
	return &o, nil
}

func scanOrgMember(row scanner) (*user.OrganizationMember, error) {
	var m user.OrganizationMember
	err := row.Scan(
		&m.OrganizationID, &m.UserID, &m.Role, &m.JoinedAt, &m.InvitedBy,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func scanRole(row scanner) (*user.Role, error) {
	var r user.Role
	var permJSON []byte
	err := row.Scan(
		&r.ID, &r.Name, &r.DisplayName, &r.Description, &r.IsSystem, &permJSON,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "role not found")
		}
		return nil, err
	}
	if len(permJSON) > 0 { _ = json.Unmarshal(permJSON, &r.Permissions) }
	return &r, nil
}

func scanAPIKey(row scanner) (*user.APIKey, error) {
	var k user.APIKey
	var scopes []string
	err := row.Scan(
		&k.ID, &k.UserID, &k.OrganizationID, &k.Name, &k.KeyHash, &k.KeyPrefix, pq.Array(&scopes),
		&k.RateLimit, &k.ExpiresAt, &k.LastUsedAt, &k.LastUsedIP, &k.IsActive,
		&k.CreatedAt, &k.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "api key not found")
		}
		return nil, err
	}
	k.Scopes = scopes
	return &k, nil
}

func scanAuditLog(row scanner) (*user.AuditLog, error) {
	var l user.AuditLog
	var beforeJSON, afterJSON, metaJSON []byte
	err := row.Scan(
		&l.ID, &l.UserID, &l.OrganizationID, &l.Action, &l.ResourceType, &l.ResourceID,
		&l.IPAddress, &l.UserAgent, &l.RequestID, &beforeJSON, &afterJSON, &metaJSON,
		&l.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(beforeJSON) > 0 { _ = json.Unmarshal(beforeJSON, &l.BeforeState) }
	if len(afterJSON) > 0 { _ = json.Unmarshal(afterJSON, &l.AfterState) }
	if len(metaJSON) > 0 { _ = json.Unmarshal(metaJSON, &l.Metadata) }
	return &l, nil
}
