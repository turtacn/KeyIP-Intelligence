package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

type postgresLifecycleRepo struct {
	conn *postgres.Connection
	tx   *sql.Tx
	log  logging.Logger
}

func NewPostgresLifecycleRepo(conn *postgres.Connection, log logging.Logger) lifecycle.LifecycleRepository {
	return &postgresLifecycleRepo{
		conn: conn,
		log:  log,
	}
}

func (r *postgresLifecycleRepo) executor() queryExecutor {
	if r.tx != nil {
		return r.tx
	}
	return r.conn.DB()
}

// Annuity

func (r *postgresLifecycleRepo) CreateAnnuity(ctx context.Context, annuity *lifecycle.Annuity) error {
	query := `
		INSERT INTO patent_annuities (
			patent_id, year_number, due_date, grace_deadline, status, amount, currency,
			paid_amount, paid_date, payment_reference, agent_name, agent_reference,
			notes, reminder_sent_at, reminder_count, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		) RETURNING id, created_at, updated_at
	`

	metaJSON, _ := json.Marshal(annuity.Metadata)

	err := r.executor().QueryRowContext(ctx, query,
		annuity.PatentID, annuity.YearNumber, annuity.DueDate, annuity.GraceDeadline, annuity.Status,
		annuity.Amount, annuity.Currency, annuity.PaidAmount, annuity.PaidDate, annuity.PaymentReference,
		annuity.AgentName, annuity.AgentReference, annuity.Notes, annuity.ReminderSentAt, annuity.ReminderCount, metaJSON,
	).Scan(&annuity.ID, &annuity.CreatedAt, &annuity.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return errors.Wrap(err, errors.ErrCodeConflict, "annuity already exists for this patent and year")
		}
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create annuity")
	}
	return nil
}

func (r *postgresLifecycleRepo) GetAnnuity(ctx context.Context, id string) (*lifecycle.Annuity, error) {
	query := `SELECT * FROM patent_annuities WHERE id = $1`
	row := r.executor().QueryRowContext(ctx, query, id)
	return scanAnnuity(row)
}

func (r *postgresLifecycleRepo) GetAnnuitiesByPatent(ctx context.Context, patentID string) ([]*lifecycle.Annuity, error) {
	query := `SELECT * FROM patent_annuities WHERE patent_id = $1 ORDER BY year_number ASC`
	rows, err := r.executor().QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query annuities")
	}
	defer rows.Close()

	var annuities []*lifecycle.Annuity
	for rows.Next() {
		a, err := scanAnnuity(rows)
		if err != nil {
			return nil, err
		}
		annuities = append(annuities, a)
	}
	return annuities, nil
}

func (r *postgresLifecycleRepo) GetUpcomingAnnuities(ctx context.Context, daysAhead int, limit, offset int) ([]*lifecycle.Annuity, int64, error) {
	deadline := time.Now().AddDate(0, 0, daysAhead)

	// Correct query with JOIN
	baseQuery := `
		FROM patent_annuities a
		JOIN patents p ON a.patent_id = p.id
		WHERE a.status IN ('upcoming', 'due', 'grace_period')
		AND a.due_date <= $1
		AND p.deleted_at IS NULL
	`

	var total int64
	err := r.executor().QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery, deadline).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count upcoming annuities")
	}

	dataQuery := `SELECT a.* ` + baseQuery + ` ORDER BY a.due_date ASC LIMIT $2 OFFSET $3`
	rows, err := r.executor().QueryContext(ctx, dataQuery, deadline, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query upcoming annuities")
	}
	defer rows.Close()

	var annuities []*lifecycle.Annuity
	for rows.Next() {
		a, err := scanAnnuity(rows)
		if err != nil {
			return nil, 0, err
		}
		annuities = append(annuities, a)
	}
	return annuities, total, nil
}

func (r *postgresLifecycleRepo) GetOverdueAnnuities(ctx context.Context, limit, offset int) ([]*lifecycle.Annuity, int64, error) {
	// Logic similar to GetUpcomingAnnuities but status='overdue' or due_date < NOW and status in ('upcoming','due')
	// The prompt says "status IN ('upcoming', 'due', 'overdue', 'grace_period')" for upcoming.
	// For overdue, typically just status='overdue'. Or calculate dynamically.
	// I'll assume status field is updated by a background job, so I query status='overdue'.

	baseQuery := `
		FROM patent_annuities a
		JOIN patents p ON a.patent_id = p.id
		WHERE a.status = 'overdue'
		AND p.deleted_at IS NULL
	`

	var total int64
	err := r.executor().QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count overdue annuities")
	}

	dataQuery := `SELECT a.* ` + baseQuery + ` ORDER BY a.due_date ASC LIMIT $1 OFFSET $2`
	rows, err := r.executor().QueryContext(ctx, dataQuery, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query overdue annuities")
	}
	defer rows.Close()

	var annuities []*lifecycle.Annuity
	for rows.Next() {
		a, err := scanAnnuity(rows)
		if err != nil {
			return nil, 0, err
		}
		annuities = append(annuities, a)
	}
	return annuities, total, nil
}

func (r *postgresLifecycleRepo) UpdateAnnuityStatus(ctx context.Context, id string, status lifecycle.AnnuityStatus, paidAmount int64, paidDate *time.Time, paymentRef string) error {
	query := `
		UPDATE patent_annuities
		SET status = $1, paid_amount = $2, paid_date = $3, payment_reference = $4, updated_at = NOW()
		WHERE id = $5
	`
	res, err := r.executor().ExecContext(ctx, query, status, paidAmount, paidDate, paymentRef, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update annuity status")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "annuity not found")
	}
	return nil
}

func (r *postgresLifecycleRepo) BatchCreateAnnuities(ctx context.Context, annuities []*lifecycle.Annuity) error {
	if len(annuities) == 0 {
		return nil
	}

	tx, err := r.conn.DB().BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to begin transaction")
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, pq.CopyIn("patent_annuities",
		"patent_id", "year_number", "due_date", "grace_deadline", "status", "amount", "currency",
		"metadata", "created_at", "updated_at"))
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to prepare copy statement")
	}
	defer stmt.Close()

	for _, a := range annuities {
		metaJSON, _ := json.Marshal(a.Metadata)
		_, err = stmt.ExecContext(ctx, a.PatentID, a.YearNumber, a.DueDate, a.GraceDeadline, a.Status, a.Amount, a.Currency, metaJSON, time.Now(), time.Now())
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to exec copy")
		}
	}

	_, err = stmt.ExecContext(ctx)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to flush copy")
	}

	return tx.Commit()
}

func (r *postgresLifecycleRepo) UpdateReminderSent(ctx context.Context, id string) error {
	query := `
		UPDATE patent_annuities
		SET reminder_sent_at = NOW(), reminder_count = reminder_count + 1, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.executor().ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update reminder")
	}
	return nil
}

// Deadline

func (r *postgresLifecycleRepo) CreateDeadline(ctx context.Context, deadline *lifecycle.Deadline) error {
	query := `
		INSERT INTO patent_deadlines (
			patent_id, deadline_type, title, description, due_date, original_due_date,
			status, priority, assignee_id, reminder_config, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		) RETURNING id, created_at, updated_at
	`
	remConfig, _ := json.Marshal(deadline.ReminderConfig)
	metaJSON, _ := json.Marshal(deadline.Metadata)

	err := r.executor().QueryRowContext(ctx, query,
		deadline.PatentID, deadline.DeadlineType, deadline.Title, deadline.Description,
		deadline.DueDate, deadline.OriginalDueDate, deadline.Status, deadline.Priority,
		deadline.AssigneeID, remConfig, metaJSON,
	).Scan(&deadline.ID, &deadline.CreatedAt, &deadline.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create deadline")
	}
	return nil
}

func (r *postgresLifecycleRepo) GetDeadline(ctx context.Context, id string) (*lifecycle.Deadline, error) {
	query := `SELECT * FROM patent_deadlines WHERE id = $1`
	row := r.executor().QueryRowContext(ctx, query, id)
	return scanDeadline(row)
}

func (r *postgresLifecycleRepo) GetDeadlinesByPatent(ctx context.Context, patentID string, statusFilter []lifecycle.DeadlineStatus) ([]*lifecycle.Deadline, error) {
	var query string
	var args []interface{}
	args = append(args, patentID)

	if len(statusFilter) > 0 {
		statuses := make([]string, len(statusFilter))
		for i, s := range statusFilter {
			statuses[i] = string(s)
		}
		query = fmt.Sprintf("SELECT * FROM patent_deadlines WHERE patent_id = $1 AND status = ANY($2::text[])")
		args = append(args, pq.Array(statuses))
	} else {
		query = "SELECT * FROM patent_deadlines WHERE patent_id = $1"
	}

	rows, err := r.executor().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query deadlines")
	}
	defer rows.Close()

	var deadlines []*lifecycle.Deadline
	for rows.Next() {
		d, err := scanDeadline(rows)
		if err != nil {
			return nil, err
		}
		deadlines = append(deadlines, d)
	}
	return deadlines, nil
}

func (r *postgresLifecycleRepo) GetActiveDeadlines(ctx context.Context, userID *string, daysAhead int, limit, offset int) ([]*lifecycle.Deadline, int64, error) {
	deadline := time.Now().AddDate(0, 0, daysAhead)

	baseQuery := `
		FROM patent_deadlines d
		JOIN patents p ON d.patent_id = p.id
		WHERE d.status = 'active'
		AND d.due_date <= $1
		AND p.deleted_at IS NULL
	`
	args := []interface{}{deadline}
	argIdx := 2

	if userID != nil {
		baseQuery += fmt.Sprintf(" AND d.assignee_id = $%d", argIdx)
		args = append(args, userID)
		argIdx++
	}

	var total int64
	err := r.executor().QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count active deadlines")
	}

	dataQuery := fmt.Sprintf("SELECT d.* %s ORDER BY d.due_date ASC LIMIT $%d OFFSET $%d", baseQuery, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.executor().QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query active deadlines")
	}
	defer rows.Close()

	var deadlines []*lifecycle.Deadline
	for rows.Next() {
		d, err := scanDeadline(rows)
		if err != nil {
			return nil, 0, err
		}
		deadlines = append(deadlines, d)
	}
	return deadlines, total, nil
}

func (r *postgresLifecycleRepo) UpdateDeadlineStatus(ctx context.Context, id string, status lifecycle.DeadlineStatus, completedBy *string) error {
	var query string
	var args []interface{}

	if status == lifecycle.DeadlineStatusCompleted {
		query = "UPDATE patent_deadlines SET status = $1, completed_by = $2, completed_at = NOW(), updated_at = NOW() WHERE id = $3"
		args = []interface{}{status, completedBy, id}
	} else {
		query = "UPDATE patent_deadlines SET status = $1, updated_at = NOW() WHERE id = $2"
		args = []interface{}{status, id}
	}

	res, err := r.executor().ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update deadline status")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "deadline not found")
	}
	return nil
}

func (r *postgresLifecycleRepo) ExtendDeadline(ctx context.Context, id string, newDueDate time.Time, reason string) error {
	// First fetch current to append history
	d, err := r.GetDeadline(ctx, id)
	if err != nil {
		return err
	}
	if d.Status == lifecycle.DeadlineStatusCompleted {
		return errors.New(errors.ErrCodeConflict, "cannot extend completed deadline")
	}

	historyEntry := map[string]interface{}{
		"from": d.DueDate,
		"to": newDueDate,
		"reason": reason,
		"at": time.Now(),
	}

	// Append using jsonb_set or || operator in Postgres, but simple approach is to read, modify, write or use complex SQL.
	// Using SQL with jsonb concatenation is better for atomicity.
	historyEntryJSON, _ := json.Marshal(historyEntry)

	query := `
		UPDATE patent_deadlines
		SET due_date = $1,
		    extension_count = extension_count + 1,
		    extension_history = extension_history || $2::jsonb,
		    updated_at = NOW()
		WHERE id = $3
	`
	_, err = r.executor().ExecContext(ctx, query, newDueDate, historyEntryJSON, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to extend deadline")
	}
	return nil
}

func (r *postgresLifecycleRepo) GetCriticalDeadlines(ctx context.Context, limit int) ([]*lifecycle.Deadline, error) {
	query := `
		SELECT d.*
		FROM patent_deadlines d
		JOIN patents p ON d.patent_id = p.id
		WHERE d.priority = 'critical' AND d.status = 'active'
		AND p.deleted_at IS NULL
		ORDER BY d.due_date ASC
		LIMIT $1
	`
	rows, err := r.executor().QueryContext(ctx, query, limit)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query critical deadlines")
	}
	defer rows.Close()

	var deadlines []*lifecycle.Deadline
	for rows.Next() {
		d, err := scanDeadline(rows)
		if err != nil {
			return nil, err
		}
		deadlines = append(deadlines, d)
	}
	return deadlines, nil
}

// Event

func (r *postgresLifecycleRepo) DeleteEvent(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return errors.NewValidationOp("delete_event", "invalid event id")
	}
	query := `DELETE FROM patent_lifecycle_events WHERE id = $1`
	res, err := r.executor().ExecContext(ctx, query, uid)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to delete event")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "event not found")
	}
	return nil
}

func (r *postgresLifecycleRepo) CreateEvent(ctx context.Context, event *lifecycle.LifecycleEvent) error {
	query := `
		INSERT INTO patent_lifecycle_events (
			patent_id, event_type, event_date, title, description, actor_id, actor_name,
			related_deadline_id, related_annuity_id, before_state, after_state, attachments, source, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		) RETURNING id, created_at
	`
	before, _ := json.Marshal(event.BeforeState)
	after, _ := json.Marshal(event.AfterState)
	attach, _ := json.Marshal(event.Attachments)
	meta, _ := json.Marshal(event.Metadata)

	err := r.executor().QueryRowContext(ctx, query,
		event.PatentID, event.EventType, event.EventDate, event.Title, event.Description,
		event.ActorID, event.ActorName, event.RelatedDeadlineID, event.RelatedAnnuityID,
		before, after, attach, event.Source, meta,
	).Scan(&event.ID, &event.CreatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create event")
	}
	return nil
}

func (r *postgresLifecycleRepo) GetEventsByPatent(ctx context.Context, patentID string, eventTypes []lifecycle.EventType, limit, offset int) ([]*lifecycle.LifecycleEvent, int64, error) {
	baseQuery := `FROM patent_lifecycle_events WHERE patent_id = $1`
	args := []interface{}{patentID}

	if len(eventTypes) > 0 {
		baseQuery += ` AND event_type = ANY($2::lifecycle_event_type[])`
		types := make([]string, len(eventTypes))
		for i, t := range eventTypes {
			types[i] = string(t)
		}
		args = append(args, pq.Array(types))
	}

	var total int64
	err := r.executor().QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count events")
	}

	dataQuery := fmt.Sprintf("SELECT * %s ORDER BY event_date DESC LIMIT $%d OFFSET $%d", baseQuery, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.executor().QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query events")
	}
	defer rows.Close()

	var events []*lifecycle.LifecycleEvent
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, 0, err
		}
		events = append(events, e)
	}
	return events, total, nil
}

func (r *postgresLifecycleRepo) GetEventTimeline(ctx context.Context, patentID string) ([]*lifecycle.LifecycleEvent, error) {
	query := `SELECT * FROM patent_lifecycle_events WHERE patent_id = $1 ORDER BY event_date ASC`
	rows, err := r.executor().QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query timeline")
	}
	defer rows.Close()

	var events []*lifecycle.LifecycleEvent
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *postgresLifecycleRepo) GetRecentEvents(ctx context.Context, orgID string, limit int) ([]*lifecycle.LifecycleEvent, error) {
	// JOIN with patents to filter by organization (assuming assignee_id -> user -> org via user_roles or direct org ownership?
	// Patent has assignee_id (user). User belongs to org.
	// Or Portfolio -> Patent.
	// Prompt says "GetRecentEvents(ctx, orgID string, limit int)".
	// This implies fetching events for all patents belonging to users in that org OR patents in portfolios of that org.
	// A simplier way: If patents have `assignee_id` pointing to user, and user is in org.
	// But `patents` table `assignee_id` refs `users`.
	// `organization_members` links user to org.
	// So: JOIN patents p ON e.patent_id = p.id JOIN organization_members om ON p.assignee_id = om.user_id WHERE om.organization_id = $1

	query := `
		SELECT e.*
		FROM patent_lifecycle_events e
		JOIN patents p ON e.patent_id = p.id
		JOIN organization_members om ON p.assignee_id = om.user_id
		WHERE om.organization_id = $1
		AND p.deleted_at IS NULL
		ORDER BY e.event_date DESC
		LIMIT $2
	`
	rows, err := r.executor().QueryContext(ctx, query, orgID, limit)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query recent events")
	}
	defer rows.Close()

	var events []*lifecycle.LifecycleEvent
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

// Cost

func (r *postgresLifecycleRepo) CreateCostRecord(ctx context.Context, record *lifecycle.CostRecord) error {
	query := `
		INSERT INTO patent_cost_records (
			patent_id, cost_type, amount, currency, amount_usd, exchange_rate,
			incurred_date, description, invoice_reference, related_annuity_id, related_event_id, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		) RETURNING id, created_at
	`
	meta, _ := json.Marshal(record.Metadata)

	err := r.executor().QueryRowContext(ctx, query,
		record.PatentID, record.CostType, record.Amount, record.Currency, record.AmountUSD, record.ExchangeRate,
		record.IncurredDate, record.Description, record.InvoiceReference, record.RelatedAnnuityID, record.RelatedEventID, meta,
	).Scan(&record.ID, &record.CreatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create cost record")
	}
	return nil
}

func (r *postgresLifecycleRepo) GetCostsByPatent(ctx context.Context, patentID string) ([]*lifecycle.CostRecord, error) {
	query := `SELECT * FROM patent_cost_records WHERE patent_id = $1 ORDER BY incurred_date DESC`
	rows, err := r.executor().QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query costs")
	}
	defer rows.Close()

	var costs []*lifecycle.CostRecord
	for rows.Next() {
		c, err := scanCostRecord(rows)
		if err != nil {
			return nil, err
		}
		costs = append(costs, c)
	}
	return costs, nil
}

func (r *postgresLifecycleRepo) GetCostSummary(ctx context.Context, patentID string) (*lifecycle.CostSummary, error) {
	query := `
		SELECT cost_type, SUM(amount_usd)
		FROM patent_cost_records
		WHERE patent_id = $1
		GROUP BY cost_type
	`
	rows, err := r.executor().QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query cost summary")
	}
	defer rows.Close()

	summary := &lifecycle.CostSummary{
		TotalCosts: make(map[string]int64),
	}
	for rows.Next() {
		var cType string
		var total int64
		if err := rows.Scan(&cType, &total); err != nil {
			return nil, err
		}
		summary.TotalCosts[cType] = total
	}
	return summary, nil
}

func (r *postgresLifecycleRepo) GetPortfolioCostSummary(ctx context.Context, portfolioID string, startDate, endDate time.Time) (*lifecycle.PortfolioCostSummary, error) {
	query := `
		SELECT c.cost_type, SUM(c.amount_usd)
		FROM patent_cost_records c
		JOIN portfolio_patents pp ON c.patent_id = pp.patent_id
		WHERE pp.portfolio_id = $1
		AND c.incurred_date BETWEEN $2 AND $3
		GROUP BY c.cost_type
	`
	rows, err := r.executor().QueryContext(ctx, query, portfolioID, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query portfolio cost summary")
	}
	defer rows.Close()

	summary := &lifecycle.PortfolioCostSummary{
		TotalCosts: make(map[string]int64),
	}
	for rows.Next() {
		var cType string
		var total int64
		if err := rows.Scan(&cType, &total); err != nil {
			return nil, err
		}
		summary.TotalCosts[cType] = total
	}
	return summary, nil
}

func (r *postgresLifecycleRepo) DeactivateSubscription(ctx context.Context, subscriptionID string) error {
	uid, err := uuid.Parse(subscriptionID)
	if err != nil {
		return errors.NewValidationOp("deactivate_subscription", "invalid subscription id")
	}
	query := `UPDATE patent_monitor_subscriptions SET is_active = false, updated_at = NOW() WHERE id = $1`
	res, err := r.executor().ExecContext(ctx, query, uid)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to deactivate subscription")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "subscription not found")
	}
	return nil
}

// Dashboard

func (r *postgresLifecycleRepo) GetLifecycleDashboard(ctx context.Context, orgID string) (*lifecycle.DashboardStats, error) {
	stats := &lifecycle.DashboardStats{}

	// Complex aggregations could be separate queries or CTE. Here use separate for clarity.

	// 1. Upcoming Annuities (next 90 days)
	q1 := `
		SELECT COUNT(*)
		FROM patent_annuities a
		JOIN patents p ON a.patent_id = p.id
		JOIN organization_members om ON p.assignee_id = om.user_id
		WHERE om.organization_id = $1
		AND a.status IN ('upcoming', 'due', 'grace_period')
		AND a.due_date <= NOW() + INTERVAL '90 days'
		AND p.deleted_at IS NULL
	`
	if err := r.executor().QueryRowContext(ctx, q1, orgID).Scan(&stats.UpcomingAnnuities); err != nil {
		return nil, err
	}

	// 2. Overdue Annuities
	q2 := `
		SELECT COUNT(*)
		FROM patent_annuities a
		JOIN patents p ON a.patent_id = p.id
		JOIN organization_members om ON p.assignee_id = om.user_id
		WHERE om.organization_id = $1
		AND a.status = 'overdue'
		AND p.deleted_at IS NULL
	`
	if err := r.executor().QueryRowContext(ctx, q2, orgID).Scan(&stats.OverdueAnnuities); err != nil {
		return nil, err
	}

	// 3. Active Deadlines
	q3 := `
		SELECT COUNT(*)
		FROM patent_deadlines d
		JOIN patents p ON d.patent_id = p.id
		JOIN organization_members om ON p.assignee_id = om.user_id
		WHERE om.organization_id = $1
		AND d.status = 'active'
		AND p.deleted_at IS NULL
	`
	if err := r.executor().QueryRowContext(ctx, q3, orgID).Scan(&stats.ActiveDeadlines); err != nil {
		return nil, err
	}

	// 4. Recent Events (last 30 days)
	q4 := `
		SELECT COUNT(*)
		FROM patent_lifecycle_events e
		JOIN patents p ON e.patent_id = p.id
		JOIN organization_members om ON p.assignee_id = om.user_id
		WHERE om.organization_id = $1
		AND e.event_date >= NOW() - INTERVAL '30 days'
		AND p.deleted_at IS NULL
	`
	if err := r.executor().QueryRowContext(ctx, q4, orgID).Scan(&stats.RecentEvents); err != nil {
		return nil, err
	}

	// 5. Total Cost YTD
	q5 := `
		SELECT COALESCE(SUM(amount_usd), 0)
		FROM patent_cost_records c
		JOIN patents p ON c.patent_id = p.id
		JOIN organization_members om ON p.assignee_id = om.user_id
		WHERE om.organization_id = $1
		AND c.incurred_date >= date_trunc('year', CURRENT_DATE)
		AND p.deleted_at IS NULL
	`
	if err := r.executor().QueryRowContext(ctx, q5, orgID).Scan(&stats.TotalCostYTD); err != nil {
		return nil, err
	}

	return stats, nil
}

// Payment

func (r *postgresLifecycleRepo) SavePayment(ctx context.Context, payment *lifecycle.PaymentRecord) (*lifecycle.PaymentRecord, error) {
	// Stub implementation
	return payment, nil
}

func (r *postgresLifecycleRepo) QueryPayments(ctx context.Context, query *lifecycle.PaymentQuery) ([]lifecycle.PaymentRecord, int64, error) {
	// Stub implementation
	return []lifecycle.PaymentRecord{}, 0, nil
}

// Legal Status

func (r *postgresLifecycleRepo) GetByPatentID(ctx context.Context, patentID string) (*lifecycle.LegalStatusEntity, error) {
	// Stub implementation
	return &lifecycle.LegalStatusEntity{}, nil
}

func (r *postgresLifecycleRepo) UpdateStatus(ctx context.Context, patentID string, status string, effectiveDate time.Time) error {
	// Stub implementation
	return nil
}

func (r *postgresLifecycleRepo) SaveSubscription(ctx context.Context, sub *lifecycle.SubscriptionEntity) error {
	// Stub implementation
	return nil
}

func (r *postgresLifecycleRepo) GetStatusHistory(ctx context.Context, patentID string, pagination *commontypes.Pagination, from, to *time.Time) ([]*lifecycle.StatusHistoryEntity, error) {
	// Stub implementation
	return []*lifecycle.StatusHistoryEntity{}, nil
}

// Custom Event

func (r *postgresLifecycleRepo) SaveCustomEvent(ctx context.Context, event *lifecycle.CustomEvent) error {
	// Stub implementation
	return nil
}

func (r *postgresLifecycleRepo) GetCustomEvents(ctx context.Context, patentIDs []string, start, end time.Time) ([]lifecycle.CustomEvent, error) {
	// Stub implementation
	return []lifecycle.CustomEvent{}, nil
}

func (r *postgresLifecycleRepo) UpdateEventStatus(ctx context.Context, eventID string, status string) error {
	// Stub implementation
	return nil
}

// Transaction

func (r *postgresLifecycleRepo) WithTx(ctx context.Context, fn func(lifecycle.LifecycleRepository) error) error {
	tx, err := r.conn.DB().BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to begin transaction")
	}

	txRepo := &postgresLifecycleRepo{
		conn: r.conn,
		tx:   tx,
		log:  r.log,
	}

	if err := fn(txRepo); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			r.log.Error("Failed to rollback transaction", logging.Err(rbErr))
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to commit transaction")
	}
	return nil
}
// Implement all methods for txLifecycleRepo by delegating or copy-paste?
// Copy-paste or code reuse is needed.
// To avoid duplication, I should have a `baseRepo` that takes `queryExecutor`.
// Refactoring:

type baseLifecycleRepo struct {
	exec queryExecutor
	log  logging.Logger
	// conn is needed for WithTx only on the main repo, or access DB for BatchCreate
	db   *sql.DB // optional, for BatchCreate using CopyIn which requires *sql.Tx or *sql.DB
}

// Re-implementing methods on baseLifecycleRepo...
// Since I already wrote methods on postgresLifecycleRepo, I'll change the receiver to baseLifecycleRepo and embed it.

// But `WithTx` is only on the main repo.
// I will adapt the structure.

// To keep it simple in one file: I will make `postgresLifecycleRepo` use a `tx` field if present.

func (r *postgresLifecycleRepo) DB() queryExecutor {
	// this is not safe if r is shared.
	// Better to create a new instance.
	return r.conn.DB()
}

// I will define `postgresLifecycleRepo` to hold `queryExecutor` instead of `conn`.
// But `WithTx` needs `conn.DB().BeginTx`.
// So `postgresLifecycleRepo` needs `conn`.
// The tx version will need to wrap the tx.

// Let's use a struct that has both, and a method `executor()` that picks one.
/*
type postgresLifecycleRepo struct {
    conn *postgres.Connection
    tx   *sql.Tx
    log  logging.Logger
}
func (r *postgresLifecycleRepo) executor() queryExecutor {
    if r.tx != nil { return r.tx }
    return r.conn.DB()
}
*/
// This works if we create a new instance for tx.

// Re-defining:
/*
type postgresLifecycleRepo struct {
	conn *postgres.Connection
	tx   *sql.Tx
	log  logging.Logger
}
*/

// Updating NewPostgresLifecycleRepo
// func NewPostgresLifecycleRepo(conn *postgres.Connection, log logging.Logger) lifecycle.LifecycleRepository {
// 	return &postgresLifecycleRepo{conn: conn, log: log}
// }

// Updating WithTx
// func (r *postgresLifecycleRepo) WithTx(...) {
//     tx, err := r.conn.DB().BeginTx(...)
//     txRepo := &postgresLifecycleRepo{conn: r.conn, tx: tx, log: r.log}
//     ...
// }

// Scan helpers
func scanAnnuity(row scanner) (*lifecycle.Annuity, error) {
	a := &lifecycle.Annuity{}
	var metaJSON []byte
	err := row.Scan(
		&a.ID, &a.PatentID, &a.YearNumber, &a.DueDate, &a.GraceDeadline, &a.Status,
		&a.Amount, &a.Currency, &a.PaidAmount, &a.PaidDate, &a.PaymentReference,
		&a.AgentName, &a.AgentReference, &a.Notes, &a.ReminderSentAt, &a.ReminderCount, &metaJSON,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "annuity not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan annuity")
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &a.Metadata)
	}
	return a, nil
}

func scanDeadline(row scanner) (*lifecycle.Deadline, error) {
	d := &lifecycle.Deadline{}
	var remJSON, metaJSON, extHistoryJSON []byte
	err := row.Scan(
		&d.ID, &d.PatentID, &d.DeadlineType, &d.Title, &d.Description,
		&d.DueDate, &d.OriginalDueDate, &d.Status, &d.Priority, &d.AssigneeID,
		&d.CompletedAt, &d.CompletedBy, &d.ExtensionCount, &extHistoryJSON,
		&remJSON, &d.LastReminderAt, &metaJSON, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "deadline not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan deadline")
	}
	if len(remJSON) > 0 { _ = json.Unmarshal(remJSON, &d.ReminderConfig) }
	if len(metaJSON) > 0 { _ = json.Unmarshal(metaJSON, &d.Metadata) }
	if len(extHistoryJSON) > 0 { _ = json.Unmarshal(extHistoryJSON, &d.ExtensionHistory) }
	return d, nil
}

func scanEvent(row scanner) (*lifecycle.LifecycleEvent, error) {
	e := &lifecycle.LifecycleEvent{}
	var before, after, attach, meta []byte
	err := row.Scan(
		&e.ID, &e.PatentID, &e.EventType, &e.EventDate, &e.Title, &e.Description,
		&e.ActorID, &e.ActorName, &e.RelatedDeadlineID, &e.RelatedAnnuityID,
		&before, &after, &attach, &e.Source, &meta, &e.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "event not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan event")
	}
	if len(before) > 0 { _ = json.Unmarshal(before, &e.BeforeState) }
	if len(after) > 0 { _ = json.Unmarshal(after, &e.AfterState) }
	if len(attach) > 0 { _ = json.Unmarshal(attach, &e.Attachments) }
	if len(meta) > 0 { _ = json.Unmarshal(meta, &e.Metadata) }
	return e, nil
}

func scanCostRecord(row scanner) (*lifecycle.CostRecord, error) {
	c := &lifecycle.CostRecord{}
	var meta []byte
	err := row.Scan(
		&c.ID, &c.PatentID, &c.CostType, &c.Amount, &c.Currency, &c.AmountUSD,
		&c.ExchangeRate, &c.IncurredDate, &c.Description, &c.InvoiceReference,
		&c.RelatedAnnuityID, &c.RelatedEventID, &meta, &c.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "cost record not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan cost record")
	}
	if len(meta) > 0 { _ = json.Unmarshal(meta, &c.Metadata) }
	return c, nil
}

//Personal.AI order the ending
