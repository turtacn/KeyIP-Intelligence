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

type scanner interface {
	Scan(dest ...interface{}) error
}

type queryExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

type postgresLifecycleRepo struct {
	conn   *postgres.Connection
	log    logging.Logger
	executor queryExecutor
}

func NewPostgresLifecycleRepo(conn *postgres.Connection, log logging.Logger) lifecycle.LifecycleRepository {
	return &postgresLifecycleRepo{
		conn:   conn,
		log:    log,
		executor: conn.DB(),
	}
}

// Transaction support
type txLifecycleRepo struct {
	*postgresLifecycleRepo
	tx *sql.Tx
}

func (r *postgresLifecycleRepo) WithTx(ctx context.Context, fn func(lifecycle.LifecycleRepository) error) error {
	tx, err := r.conn.DB().BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to begin transaction")
	}

	txRepo := &txLifecycleRepo{
		postgresLifecycleRepo: &postgresLifecycleRepo{
			conn:     r.conn,
			log:      r.log,
			executor: tx,
		},
		tx: tx,
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

// Annuity Management

func (r *postgresLifecycleRepo) CreateAnnuity(ctx context.Context, annuity *lifecycle.Annuity) error {
	query := `
		INSERT INTO patent_annuities (
			patent_id, year_number, due_date, grace_deadline, status, amount, currency,
			paid_amount, paid_date, payment_reference, agent_name, agent_reference,
			notes, reminder_sent_at, reminder_count, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, created_at, updated_at
	`

	metaJSON, _ := json.Marshal(annuity.Metadata)

	err := r.executor.QueryRowContext(ctx, query,
		annuity.PatentID, annuity.YearNumber, annuity.DueDate, annuity.GraceDeadline, annuity.Status,
		annuity.Amount, annuity.Currency, annuity.PaidAmount, annuity.PaidDate,
		annuity.PaymentReference, annuity.AgentName, annuity.AgentReference,
		annuity.Notes, annuity.ReminderSentAt, annuity.ReminderCount, metaJSON,
	).Scan(&annuity.ID, &annuity.CreatedAt, &annuity.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return errors.Wrap(err, errors.ErrCodeConflict, "annuity already exists for this year")
		}
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create annuity")
	}
	return nil
}

func (r *postgresLifecycleRepo) GetAnnuity(ctx context.Context, id uuid.UUID) (*lifecycle.Annuity, error) {
	query := `SELECT * FROM patent_annuities WHERE id = $1`
	row := r.executor.QueryRowContext(ctx, query, id)
	return scanAnnuity(row)
}

func (r *postgresLifecycleRepo) GetAnnuitiesByPatent(ctx context.Context, patentID uuid.UUID) ([]*lifecycle.Annuity, error) {
	query := `SELECT * FROM patent_annuities WHERE patent_id = $1 ORDER BY year_number ASC`
	rows, err := r.executor.QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get annuities")
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
	targetDate := time.Now().AddDate(0, 0, daysAhead)

	// Count query
	countQuery := `
		SELECT COUNT(*) FROM patent_annuities
		WHERE status IN ('upcoming', 'due', 'overdue', 'grace_period')
		AND due_date <= $1
	`
	var total int64
	if err := r.executor.QueryRowContext(ctx, countQuery, targetDate).Scan(&total); err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count upcoming annuities")
	}

	// Data query
	query := `
		SELECT * FROM patent_annuities
		WHERE status IN ('upcoming', 'due', 'overdue', 'grace_period')
		AND due_date <= $1
		ORDER BY due_date ASC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.executor.QueryContext(ctx, query, targetDate, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get upcoming annuities")
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
	now := time.Now()

	countQuery := `
		SELECT COUNT(*) FROM patent_annuities
		WHERE status IN ('due', 'overdue')
		AND due_date < $1
	`
	var total int64
	if err := r.executor.QueryRowContext(ctx, countQuery, now).Scan(&total); err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count overdue annuities")
	}

	query := `
		SELECT * FROM patent_annuities
		WHERE status IN ('due', 'overdue')
		AND due_date < $1
		ORDER BY due_date ASC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.executor.QueryContext(ctx, query, now, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get overdue annuities")
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

func (r *postgresLifecycleRepo) UpdateAnnuityStatus(ctx context.Context, id uuid.UUID, status lifecycle.AnnuityStatus, paidAmount int64, paidDate *time.Time, paymentRef string) error {
	query := `
		UPDATE patent_annuities
		SET status = $1, paid_amount = $2, paid_date = $3, payment_reference = $4, updated_at = NOW()
		WHERE id = $5
	`
	res, err := r.executor.ExecContext(ctx, query, status, paidAmount, paidDate, paymentRef, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update annuity status")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.ErrNotFound("annuity", id.String())
	}
	return nil
}

func (r *postgresLifecycleRepo) BatchCreateAnnuities(ctx context.Context, annuities []*lifecycle.Annuity) error {
	if len(annuities) == 0 {
		return nil
	}

	// Implementing as loop for simplicity, can use COPY or multi-value insert for performance
	// Or use transaction
	return r.WithTx(ctx, func(txRepo lifecycle.LifecycleRepository) error {
		for _, a := range annuities {
			if err := txRepo.CreateAnnuity(ctx, a); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *postgresLifecycleRepo) UpdateReminderSent(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE patent_annuities
		SET reminder_sent_at = NOW(), reminder_count = reminder_count + 1, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.executor.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update reminder")
	}
	return nil
}

// Deadline Management

func (r *postgresLifecycleRepo) CreateDeadline(ctx context.Context, deadline *lifecycle.Deadline) error {
	query := `
		INSERT INTO patent_deadlines (
			patent_id, deadline_type, title, description, due_date, original_due_date,
			status, priority, assignee_id, reminder_config, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	metaJSON, _ := json.Marshal(deadline.Metadata)
	reminderJSON, _ := json.Marshal(deadline.ReminderConfig)

	err := r.executor.QueryRowContext(ctx, query,
		deadline.PatentID, deadline.DeadlineType, deadline.Title, deadline.Description,
		deadline.DueDate, deadline.OriginalDueDate, deadline.Status, deadline.Priority,
		deadline.AssigneeID, reminderJSON, metaJSON,
	).Scan(&deadline.ID, &deadline.CreatedAt, &deadline.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create deadline")
	}
	return nil
}

func (r *postgresLifecycleRepo) GetDeadline(ctx context.Context, id uuid.UUID) (*lifecycle.Deadline, error) {
	query := `SELECT * FROM patent_deadlines WHERE id = $1`
	row := r.executor.QueryRowContext(ctx, query, id)
	return scanDeadline(row)
}

func (r *postgresLifecycleRepo) GetDeadlinesByPatent(ctx context.Context, patentID uuid.UUID, statusFilter []lifecycle.DeadlineStatus) ([]*lifecycle.Deadline, error) {
	query := `SELECT * FROM patent_deadlines WHERE patent_id = $1`
	args := []interface{}{patentID}

	if len(statusFilter) > 0 {
		query += ` AND status = ANY($2)`
		// Convert slice to pq array
		statuses := make([]string, len(statusFilter))
		for i, s := range statusFilter {
			statuses[i] = string(s)
		}
		args = append(args, pq.Array(statuses))
	}
	query += ` ORDER BY due_date ASC`

	rows, err := r.executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get deadlines")
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

func (r *postgresLifecycleRepo) GetActiveDeadlines(ctx context.Context, userID *uuid.UUID, daysAhead int, limit, offset int) ([]*lifecycle.Deadline, int64, error) {
	targetDate := time.Now().AddDate(0, 0, daysAhead)
	args := []interface{}{targetDate}

	whereClause := `WHERE status = 'active' AND due_date <= $1`
	if userID != nil {
		whereClause += ` AND assignee_id = $2`
		args = append(args, userID)
	}

	countQuery := `SELECT COUNT(*) FROM patent_deadlines ` + whereClause
	var total int64
	if err := r.executor.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count active deadlines")
	}

	query := `SELECT * FROM patent_deadlines ` + whereClause + ` ORDER BY due_date ASC LIMIT $%d OFFSET $%d`
	// Adjust placeholders for LIMIT/OFFSET
	query = fmt.Sprintf(query, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get active deadlines")
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

func (r *postgresLifecycleRepo) UpdateDeadlineStatus(ctx context.Context, id uuid.UUID, status lifecycle.DeadlineStatus, completedBy *uuid.UUID) error {
	query := `
		UPDATE patent_deadlines
		SET status = $1, completed_by = $2, completed_at = CASE WHEN $1 = 'completed' THEN NOW() ELSE NULL END, updated_at = NOW()
		WHERE id = $3
	`
	res, err := r.executor.ExecContext(ctx, query, status, completedBy, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update deadline status")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.ErrNotFound("deadline", id.String())
	}
	return nil
}

func (r *postgresLifecycleRepo) ExtendDeadline(ctx context.Context, id uuid.UUID, newDueDate time.Time, reason string) error {
	// First check if deadline exists and can be extended
	d, err := r.GetDeadline(ctx, id)
	if err != nil {
		return err
	}
	if d.Status == lifecycle.DeadlineStatusCompleted {
		return errors.New(errors.ErrCodeValidation, "cannot extend completed deadline")
	}

	history := d.ExtensionHistory
	history = append(history, map[string]interface{}{
		"from": d.DueDate,
		"to": newDueDate,
		"reason": reason,
		"at": time.Now(),
	})

	historyJSON, _ := json.Marshal(history)

	query := `
		UPDATE patent_deadlines
		SET due_date = $1, extension_count = extension_count + 1, extension_history = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err = r.executor.ExecContext(ctx, query, newDueDate, historyJSON, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to extend deadline")
	}
	return nil
}

func (r *postgresLifecycleRepo) GetCriticalDeadlines(ctx context.Context, limit int) ([]*lifecycle.Deadline, error) {
	query := `
		SELECT * FROM patent_deadlines
		WHERE status = 'active' AND priority = 'critical'
		ORDER BY due_date ASC
		LIMIT $1
	`
	rows, err := r.executor.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get critical deadlines")
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

// Event Tracking

func (r *postgresLifecycleRepo) CreateEvent(ctx context.Context, event *lifecycle.LifecycleEvent) error {
	query := `
		INSERT INTO patent_lifecycle_events (
			patent_id, event_type, event_date, title, description,
			actor_id, actor_name, related_deadline_id, related_annuity_id,
			before_state, after_state, source, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at
	`

	metaJSON, _ := json.Marshal(event.Metadata)
	beforeJSON, _ := json.Marshal(event.BeforeState)
	afterJSON, _ := json.Marshal(event.AfterState)

	err := r.executor.QueryRowContext(ctx, query,
		event.PatentID, event.EventType, event.EventDate, event.Title, event.Description,
		event.ActorID, event.ActorName, event.RelatedDeadlineID, event.RelatedAnnuityID,
		beforeJSON, afterJSON, event.Source, metaJSON,
	).Scan(&event.ID, &event.CreatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create event")
	}
	return nil
}

func (r *postgresLifecycleRepo) GetEventsByPatent(ctx context.Context, patentID uuid.UUID, eventTypes []lifecycle.EventType, limit, offset int) ([]*lifecycle.LifecycleEvent, int64, error) {
	query := `SELECT * FROM patent_lifecycle_events WHERE patent_id = $1`
	args := []interface{}{patentID}

	if len(eventTypes) > 0 {
		query += ` AND event_type = ANY($2)`
		types := make([]string, len(eventTypes))
		for i, t := range eventTypes {
			types[i] = string(t)
		}
		args = append(args, pq.Array(types))
	}

	// Count first
	countQuery := "SELECT COUNT(*) " + query[len("SELECT * "):]
	var total int64
	if err := r.executor.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count events")
	}

	query += ` ORDER BY event_date DESC LIMIT $%d OFFSET $%d`
	query = fmt.Sprintf(query, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get events")
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

func (r *postgresLifecycleRepo) GetEventTimeline(ctx context.Context, patentID uuid.UUID) ([]*lifecycle.LifecycleEvent, error) {
	query := `SELECT * FROM patent_lifecycle_events WHERE patent_id = $1 ORDER BY event_date ASC`
	rows, err := r.executor.QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get timeline")
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

func (r *postgresLifecycleRepo) GetRecentEvents(ctx context.Context, orgID uuid.UUID, limit int) ([]*lifecycle.LifecycleEvent, error) {
	// JOIN with patents to filter by organization (via owner/assignee? or portfolios?)
	// Assuming orgID filtering is done via patents table join, but users schema connects org to user.
	// For simplicity, this query might need to join multiple tables.
	// As per schema, patents have `assignee_id` (User) and `patents` can be in `portfolios` (owned by User).
	// Users belong to Organizations.

	// Implementing a simplified version: get recent events for patents owned by users in the org.
	query := `
		SELECT e.* FROM patent_lifecycle_events e
		JOIN patents p ON e.patent_id = p.id
		JOIN users u ON p.assignee_id = u.id
		JOIN organization_members om ON u.id = om.user_id
		WHERE om.organization_id = $1
		ORDER BY e.event_date DESC
		LIMIT $2
	`
	rows, err := r.executor.QueryContext(ctx, query, orgID, limit)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get recent events")
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

// Cost Management

func (r *postgresLifecycleRepo) CreateCostRecord(ctx context.Context, record *lifecycle.CostRecord) error {
	query := `
		INSERT INTO patent_cost_records (
			patent_id, cost_type, amount, currency, amount_usd, exchange_rate,
			incurred_date, description, invoice_reference, related_annuity_id, related_event_id, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at
	`
	metaJSON, _ := json.Marshal(record.Metadata)

	err := r.executor.QueryRowContext(ctx, query,
		record.PatentID, record.CostType, record.Amount, record.Currency,
		record.AmountUSD, record.ExchangeRate, record.IncurredDate,
		record.Description, record.InvoiceReference,
		record.RelatedAnnuityID, record.RelatedEventID, metaJSON,
	).Scan(&record.ID, &record.CreatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create cost record")
	}
	return nil
}

func (r *postgresLifecycleRepo) GetCostsByPatent(ctx context.Context, patentID uuid.UUID) ([]*lifecycle.CostRecord, error) {
	query := `SELECT * FROM patent_cost_records WHERE patent_id = $1 ORDER BY incurred_date DESC`
	rows, err := r.executor.QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get costs")
	}
	defer rows.Close()

	var costs []*lifecycle.CostRecord
	for rows.Next() {
		c, err := scanCost(rows)
		if err != nil {
			return nil, err
		}
		costs = append(costs, c)
	}
	return costs, nil
}

func (r *postgresLifecycleRepo) GetCostSummary(ctx context.Context, patentID uuid.UUID) (*lifecycle.CostSummary, error) {
	query := `
		SELECT cost_type, SUM(amount)
		FROM patent_cost_records
		WHERE patent_id = $1
		GROUP BY cost_type
	`
	rows, err := r.executor.QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get cost summary")
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

func (r *postgresLifecycleRepo) GetPortfolioCostSummary(ctx context.Context, portfolioID uuid.UUID, startDate, endDate time.Time) (*lifecycle.PortfolioCostSummary, error) {
	query := `
		SELECT c.cost_type, SUM(c.amount)
		FROM patent_cost_records c
		JOIN portfolio_patents pp ON c.patent_id = pp.patent_id
		WHERE pp.portfolio_id = $1 AND c.incurred_date BETWEEN $2 AND $3
		GROUP BY c.cost_type
	`
	rows, err := r.executor.QueryContext(ctx, query, portfolioID, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get portfolio cost summary")
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

func (r *postgresLifecycleRepo) GetLifecycleDashboard(ctx context.Context, orgID uuid.UUID) (*lifecycle.DashboardStats, error) {
	// Complex aggregation query
	stats := &lifecycle.DashboardStats{}

	// Implementation would involve multiple queries or a CTE
	// For now, returning empty stats to satisfy interface
	return stats, nil
}

// NotImplemented methods for extra interface compliance

func (r *postgresLifecycleRepo) SavePayment(ctx context.Context, payment *lifecycle.PaymentRecord) (*lifecycle.PaymentRecord, error) {
	return nil, errors.New(errors.ErrCodeNotImplemented, "not implemented")
}
func (r *postgresLifecycleRepo) QueryPayments(ctx context.Context, query *lifecycle.PaymentQuery) ([]lifecycle.PaymentRecord, int64, error) {
	return nil, 0, errors.New(errors.ErrCodeNotImplemented, "not implemented")
}
func (r *postgresLifecycleRepo) GetByPatentID(ctx context.Context, patentID string) (*lifecycle.LegalStatusEntity, error) {
	return nil, errors.New(errors.ErrCodeNotImplemented, "not implemented")
}
func (r *postgresLifecycleRepo) UpdateStatus(ctx context.Context, patentID string, status string, effectiveDate time.Time) error {
	return errors.New(errors.ErrCodeNotImplemented, "not implemented")
}
func (r *postgresLifecycleRepo) SaveSubscription(ctx context.Context, sub *lifecycle.SubscriptionEntity) error {
	return errors.New(errors.ErrCodeNotImplemented, "not implemented")
}
func (r *postgresLifecycleRepo) DeactivateSubscription(ctx context.Context, id string) error {
	return errors.New(errors.ErrCodeNotImplemented, "not implemented")
}
func (r *postgresLifecycleRepo) GetStatusHistory(ctx context.Context, patentID string, pagination *commontypes.Pagination, from, to *time.Time) ([]*lifecycle.StatusHistoryEntity, error) {
	return nil, errors.New(errors.ErrCodeNotImplemented, "not implemented")
}
func (r *postgresLifecycleRepo) SaveCustomEvent(ctx context.Context, event *lifecycle.CustomEvent) error {
	return errors.New(errors.ErrCodeNotImplemented, "not implemented")
}
func (r *postgresLifecycleRepo) GetCustomEvents(ctx context.Context, patentIDs []string, start, end time.Time) ([]lifecycle.CustomEvent, error) {
	return nil, errors.New(errors.ErrCodeNotImplemented, "not implemented")
}
func (r *postgresLifecycleRepo) UpdateEventStatus(ctx context.Context, eventID string, status string) error {
	return errors.New(errors.ErrCodeNotImplemented, "not implemented")
}
func (r *postgresLifecycleRepo) DeleteEvent(ctx context.Context, eventID string) error {
	return errors.New(errors.ErrCodeNotImplemented, "not implemented")
}

// Scanners

func scanAnnuity(s scanner) (*lifecycle.Annuity, error) {
	var a lifecycle.Annuity
	var metaJSON []byte
	err := s.Scan(
		&a.ID, &a.PatentID, &a.YearNumber, &a.DueDate, &a.GraceDeadline,
		&a.Status, &a.Amount, &a.Currency, &a.PaidAmount, &a.PaidDate,
		&a.PaymentReference, &a.AgentName, &a.AgentReference, &a.Notes,
		&a.ReminderSentAt, &a.ReminderCount, &metaJSON,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "annuity not found")
		}
		return nil, err
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &a.Metadata)
	}
	return &a, nil
}

func scanDeadline(s scanner) (*lifecycle.Deadline, error) {
	var d lifecycle.Deadline
	var metaJSON, historyJSON, configJSON []byte
	err := s.Scan(
		&d.ID, &d.PatentID, &d.DeadlineType, &d.Title, &d.Description,
		&d.DueDate, &d.OriginalDueDate, &d.Status, &d.Priority,
		&d.AssigneeID, &d.CompletedAt, &d.CompletedBy,
		&d.ExtensionCount, &historyJSON, &configJSON, &d.LastReminderAt,
		&metaJSON, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "deadline not found")
		}
		return nil, err
	}
	if len(metaJSON) > 0 { _ = json.Unmarshal(metaJSON, &d.Metadata) }
	if len(historyJSON) > 0 { _ = json.Unmarshal(historyJSON, &d.ExtensionHistory) }
	if len(configJSON) > 0 { _ = json.Unmarshal(configJSON, &d.ReminderConfig) }
	return &d, nil
}

func scanEvent(s scanner) (*lifecycle.LifecycleEvent, error) {
	var e lifecycle.LifecycleEvent
	var metaJSON, beforeJSON, afterJSON, attachJSON []byte
	err := s.Scan(
		&e.ID, &e.PatentID, &e.EventType, &e.EventDate, &e.Title, &e.Description,
		&e.ActorID, &e.ActorName, &e.RelatedDeadlineID, &e.RelatedAnnuityID,
		&beforeJSON, &afterJSON, &attachJSON, &e.Source, &metaJSON,
		&e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(metaJSON) > 0 { _ = json.Unmarshal(metaJSON, &e.Metadata) }
	if len(beforeJSON) > 0 { _ = json.Unmarshal(beforeJSON, &e.BeforeState) }
	if len(afterJSON) > 0 { _ = json.Unmarshal(afterJSON, &e.AfterState) }
	// Attachments unmarshal skipped for brevity or add if needed
	return &e, nil
}

func scanCost(s scanner) (*lifecycle.CostRecord, error) {
	var c lifecycle.CostRecord
	var metaJSON []byte
	err := s.Scan(
		&c.ID, &c.PatentID, &c.CostType, &c.Amount, &c.Currency,
		&c.AmountUSD, &c.ExchangeRate, &c.IncurredDate,
		&c.Description, &c.InvoiceReference,
		&c.RelatedAnnuityID, &c.RelatedEventID, &metaJSON,
		&c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(metaJSON) > 0 { _ = json.Unmarshal(metaJSON, &c.Metadata) }
	return &c, nil
}
