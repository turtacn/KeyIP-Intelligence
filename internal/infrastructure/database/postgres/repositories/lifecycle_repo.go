package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/lib/pq"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Shared query executor
type queryExecutor interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type scanner interface {
	Scan(dest ...interface{}) error
}

type baseRepo struct {
	conn *postgres.Connection
	tx   *sql.Tx
	log  logging.Logger
}

func (r *baseRepo) executor() queryExecutor {
	if r.tx != nil {
		return r.tx
	}
	return r.conn.DB()
}

// --- LifecycleRepo ---

type postgresLifecycleRepo struct {
	baseRepo
}

func NewPostgresLifecycleRepo(conn *postgres.Connection, log logging.Logger) lifecycle.LifecycleRepository {
	return &postgresLifecycleRepo{
		baseRepo: baseRepo{conn: conn, log: log},
	}
}

func (r *postgresLifecycleRepo) Save(ctx context.Context, record *lifecycle.LifecycleRecord) error {
	// TODO: Implement persistence
	return nil
}

func (r *postgresLifecycleRepo) FindByID(ctx context.Context, id string) (*lifecycle.LifecycleRecord, error) {
	return nil, errors.New(errors.ErrCodeNotFound, "lifecycle record not found")
}

func (r *postgresLifecycleRepo) FindByPatentID(ctx context.Context, patentID string) (*lifecycle.LifecycleRecord, error) {
	return nil, errors.New(errors.ErrCodeNotFound, "lifecycle record not found")
}

func (r *postgresLifecycleRepo) FindByPhase(ctx context.Context, phase lifecycle.LifecyclePhase, opts ...lifecycle.LifecycleQueryOption) ([]*lifecycle.LifecycleRecord, error) {
	return []*lifecycle.LifecycleRecord{}, nil
}

func (r *postgresLifecycleRepo) FindExpiring(ctx context.Context, withinDays int) ([]*lifecycle.LifecycleRecord, error) {
	return []*lifecycle.LifecycleRecord{}, nil
}

func (r *postgresLifecycleRepo) FindByJurisdiction(ctx context.Context, jurisdictionCode string, opts ...lifecycle.LifecycleQueryOption) ([]*lifecycle.LifecycleRecord, error) {
	return []*lifecycle.LifecycleRecord{}, nil
}

func (r *postgresLifecycleRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func (r *postgresLifecycleRepo) Count(ctx context.Context, phase lifecycle.LifecyclePhase) (int64, error) {
	return 0, nil
}

// --- AnnuityRepo ---

type postgresAnnuityRepo struct {
	baseRepo
}

func NewPostgresAnnuityRepo(conn *postgres.Connection, log logging.Logger) lifecycle.AnnuityRepository {
	return &postgresAnnuityRepo{
		baseRepo: baseRepo{conn: conn, log: log},
	}
}

func (r *postgresAnnuityRepo) Save(ctx context.Context, annuity *lifecycle.AnnuityRecord) error {
	query := `
		INSERT INTO patent_annuities (
			patent_id, year_number, due_date, grace_deadline, status, amount, currency,
			paid_amount, paid_date, payment_reference, agent_name, agent_reference,
			notes, reminder_sent_at, reminder_count, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
		ON CONFLICT (patent_id, year_number) DO UPDATE SET
			due_date = EXCLUDED.due_date,
			grace_deadline = EXCLUDED.grace_deadline,
			status = EXCLUDED.status,
			amount = EXCLUDED.amount,
			currency = EXCLUDED.currency,
			paid_amount = EXCLUDED.paid_amount,
			paid_date = EXCLUDED.paid_date,
			payment_reference = EXCLUDED.payment_reference,
			notes = EXCLUDED.notes,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	metaJSON, _ := json.Marshal(annuity.Metadata)

	paidAmt := int64(0)
	if annuity.PaidAmount != nil {
		paidAmt = annuity.PaidAmount.Amount
	}

	err := r.executor().QueryRowContext(ctx, query,
		annuity.PatentID, annuity.YearNumber, annuity.DueDate, annuity.GraceDeadline, annuity.Status,
		annuity.Amount.Amount, annuity.Currency, paidAmt, annuity.PaidDate, annuity.PaymentReference,
		annuity.AgentName, annuity.AgentReference, annuity.Notes, annuity.ReminderSentAt, annuity.ReminderCount, metaJSON,
	).Scan(&annuity.ID, &annuity.CreatedAt, &annuity.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to save annuity")
	}
	return nil
}

func (r *postgresAnnuityRepo) FindByID(ctx context.Context, id string) (*lifecycle.AnnuityRecord, error) {
	query := `SELECT * FROM patent_annuities WHERE id = $1`
	row := r.executor().QueryRowContext(ctx, query, id)
	return scanAnnuity(row)
}

func (r *postgresAnnuityRepo) FindByPatentID(ctx context.Context, patentID string) ([]*lifecycle.AnnuityRecord, error) {
	query := `SELECT * FROM patent_annuities WHERE patent_id = $1 ORDER BY year_number ASC`
	rows, err := r.executor().QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query annuities")
	}
	defer rows.Close()

	var annuities []*lifecycle.AnnuityRecord
	for rows.Next() {
		a, err := scanAnnuity(rows)
		if err != nil {
			return nil, err
		}
		annuities = append(annuities, a)
	}
	return annuities, nil
}

func (r *postgresAnnuityRepo) FindByStatus(ctx context.Context, status lifecycle.AnnuityStatus) ([]*lifecycle.AnnuityRecord, error) {
	query := `SELECT * FROM patent_annuities WHERE status = $1`
	rows, err := r.executor().QueryContext(ctx, query, status)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query annuities by status")
	}
	defer rows.Close()

	var annuities []*lifecycle.AnnuityRecord
	for rows.Next() {
		a, err := scanAnnuity(rows)
		if err != nil {
			return nil, err
		}
		annuities = append(annuities, a)
	}
	return annuities, nil
}

func (r *postgresAnnuityRepo) FindPending(ctx context.Context, beforeDate time.Time) ([]*lifecycle.AnnuityRecord, error) {
	query := `
		SELECT * FROM patent_annuities
		WHERE status IN ('pending', 'upcoming', 'due', 'grace_period')
		AND due_date <= $1
	`
	rows, err := r.executor().QueryContext(ctx, query, beforeDate)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query pending annuities")
	}
	defer rows.Close()

	var annuities []*lifecycle.AnnuityRecord
	for rows.Next() {
		a, err := scanAnnuity(rows)
		if err != nil {
			return nil, err
		}
		annuities = append(annuities, a)
	}
	return annuities, nil
}

func (r *postgresAnnuityRepo) FindOverdue(ctx context.Context, asOfDate time.Time) ([]*lifecycle.AnnuityRecord, error) {
	query := `
		SELECT * FROM patent_annuities
		WHERE (status = 'overdue') OR (status IN ('pending', 'upcoming', 'due') AND due_date < $1)
	`
	rows, err := r.executor().QueryContext(ctx, query, asOfDate)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query overdue annuities")
	}
	defer rows.Close()

	var annuities []*lifecycle.AnnuityRecord
	for rows.Next() {
		a, err := scanAnnuity(rows)
		if err != nil {
			return nil, err
		}
		annuities = append(annuities, a)
	}
	return annuities, nil
}

func (r *postgresAnnuityRepo) SumByPortfolio(ctx context.Context, portfolioID string, fromDate, toDate time.Time) (int64, string, error) {
	return 0, "USD", nil
}

func (r *postgresAnnuityRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM patent_annuities WHERE id = $1`
	_, err := r.executor().ExecContext(ctx, query, id)
	return err
}

func (r *postgresAnnuityRepo) SaveBatch(ctx context.Context, annuities []*lifecycle.AnnuityRecord) error {
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
		_, err = stmt.ExecContext(ctx, a.PatentID, a.YearNumber, a.DueDate, a.GraceDeadline, a.Status, a.Amount.Amount, a.Currency, metaJSON, time.Now(), time.Now())
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

// --- DeadlineRepo ---

type postgresDeadlineRepo struct {
	baseRepo
}

func NewPostgresDeadlineRepo(conn *postgres.Connection, log logging.Logger) lifecycle.DeadlineRepository {
	return &postgresDeadlineRepo{
		baseRepo: baseRepo{conn: conn, log: log},
	}
}

func (r *postgresDeadlineRepo) Save(ctx context.Context, deadline *lifecycle.Deadline) error {
	query := `
		INSERT INTO patent_deadlines (
			patent_id, deadline_type, title, description, due_date, original_due_date,
			status, priority, assignee_id, reminder_config, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			due_date = EXCLUDED.due_date,
			status = EXCLUDED.status,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`
	remConfig, _ := json.Marshal(deadline.ReminderConfig)
	metaJSON, _ := json.Marshal(deadline.Metadata)

	err := r.executor().QueryRowContext(ctx, query,
		deadline.PatentID, deadline.Type, deadline.Title, deadline.Description,
		deadline.DueDate, deadline.OriginalDueDate, deadline.Status, deadline.Priority,
		deadline.AssigneeID, remConfig, metaJSON,
	).Scan(&deadline.ID, &deadline.CreatedAt, &deadline.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to save deadline")
	}
	return nil
}

func (r *postgresDeadlineRepo) FindByID(ctx context.Context, id string) (*lifecycle.Deadline, error) {
	query := `SELECT * FROM patent_deadlines WHERE id = $1`
	row := r.executor().QueryRowContext(ctx, query, id)
	return scanDeadline(row)
}

func (r *postgresDeadlineRepo) FindByPatentID(ctx context.Context, patentID string) ([]*lifecycle.Deadline, error) {
	query := `SELECT * FROM patent_deadlines WHERE patent_id = $1`
	rows, err := r.executor().QueryContext(ctx, query, patentID)
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

func (r *postgresDeadlineRepo) FindByOwnerID(ctx context.Context, ownerID string, from, to time.Time) ([]*lifecycle.Deadline, error) {
	query := `
		SELECT * FROM patent_deadlines
		WHERE assignee_id = $1 AND due_date BETWEEN $2 AND $3
	`
	rows, err := r.executor().QueryContext(ctx, query, ownerID, from, to)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query deadlines by owner")
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

func (r *postgresDeadlineRepo) FindOverdue(ctx context.Context, ownerID string, asOf time.Time) ([]*lifecycle.Deadline, error) {
	query := `
		SELECT * FROM patent_deadlines
		WHERE assignee_id = $1 AND status != 'completed' AND due_date < $2
	`
	rows, err := r.executor().QueryContext(ctx, query, ownerID, asOf)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query overdue deadlines")
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

func (r *postgresDeadlineRepo) FindUpcoming(ctx context.Context, ownerID string, withinDays int) ([]*lifecycle.Deadline, error) {
	limitDate := time.Now().AddDate(0, 0, withinDays)
	query := `
		SELECT * FROM patent_deadlines
		WHERE assignee_id = $1 AND status != 'completed' AND due_date <= $2 AND due_date >= NOW()
	`
	rows, err := r.executor().QueryContext(ctx, query, ownerID, limitDate)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query upcoming deadlines")
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

func (r *postgresDeadlineRepo) FindByType(ctx context.Context, deadlineType lifecycle.DeadlineType) ([]*lifecycle.Deadline, error) {
	query := `SELECT * FROM patent_deadlines WHERE deadline_type = $1`
	rows, err := r.executor().QueryContext(ctx, query, deadlineType)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query deadlines by type")
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

func (r *postgresDeadlineRepo) FindPendingReminders(ctx context.Context, reminderDate time.Time) ([]*lifecycle.Deadline, error) {
	return []*lifecycle.Deadline{}, nil
}

func (r *postgresDeadlineRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM patent_deadlines WHERE id = $1`
	_, err := r.executor().ExecContext(ctx, query, id)
	return err
}

func (r *postgresDeadlineRepo) CountByUrgency(ctx context.Context, ownerID string) (map[lifecycle.DeadlineUrgency]int64, error) {
	return make(map[lifecycle.DeadlineUrgency]int64), nil
}

// Helpers
func scanAnnuity(row scanner) (*lifecycle.AnnuityRecord, error) {
	a := &lifecycle.AnnuityRecord{}
	var metaJSON []byte
	var amt int64
	var paidAmt sql.NullInt64

	err := row.Scan(
		&a.ID, &a.PatentID, &a.YearNumber, &a.DueDate, &a.GraceDeadline, &a.Status,
		&amt, &a.Currency, &paidAmt, &a.PaidDate, &a.PaymentReference,
		&a.AgentName, &a.AgentReference, &a.Notes, &a.ReminderSentAt, &a.ReminderCount, &metaJSON,
		&a.CreatedAt, &a.UpdatedAt,
	)

	a.Amount = lifecycle.NewMoney(amt, a.Currency)
	if paidAmt.Valid {
		m := lifecycle.NewMoney(paidAmt.Int64, a.Currency)
		a.PaidAmount = &m
	}

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
		&d.ID, &d.PatentID, &d.Type, &d.Title, &d.Description,
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

//Personal.AI order the ending
