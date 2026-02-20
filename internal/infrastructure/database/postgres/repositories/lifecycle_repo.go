package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	appErrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// Domain entities — local mirrors of internal/domain/lifecycle
// ─────────────────────────────────────────────────────────────────────────────

// Deadline represents a single deadline entry stored inside the JSONB
// "deadlines" column of the lifecycles table.
type Deadline struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	DueDate     time.Time `json:"due_date"`
	Status      string    `json:"status"` // pending | completed | overdue
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	AssigneeID  string    `json:"assignee_id,omitempty"`
	Notes       string    `json:"notes,omitempty"`
}

// AnnuityRecord represents a single annuity payment entry stored inside the
// JSONB "annuity_schedule" column.
type AnnuityRecord struct {
	ID            string     `json:"id"`
	Year          int        `json:"year"`
	DueDate       time.Time  `json:"due_date"`
	Amount        float64    `json:"amount"`
	Currency      string     `json:"currency"`
	Status        string     `json:"status"` // unpaid | paid | grace | lapsed
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	PaymentRef    string     `json:"payment_ref,omitempty"`
	Jurisdiction  string     `json:"jurisdiction,omitempty"`
}

// LegalStatusEntry represents a legal status change event.
type LegalStatusEntry struct {
	Code        string    `json:"code"`
	Description string    `json:"description"`
	EffectiveAt time.Time `json:"effective_at"`
	Source      string    `json:"source,omitempty"`
}

// LifecycleEvent represents a generic lifecycle event.
type LifecycleEvent struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Description string                `json:"description"`
	OccurredAt  time.Time              `json:"occurred_at"`
	Actor       string                 `json:"actor,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// Lifecycle is the aggregate root for the lifecycle domain.  All sub-entities
// (deadlines, annuity schedule, legal status history, events) are stored as
// JSONB columns to keep the schema flexible while still allowing PostgreSQL
// jsonb_array_elements queries for filtering.
type Lifecycle struct {
	ID              common.ID
	TenantID        common.TenantID
	PatentID        common.ID
	Phase           string // filing | examination | granted | expired | abandoned
	Deadlines       []Deadline
	AnnuitySchedule []AnnuityRecord
	LegalStatus     []LegalStatusEntry
	Events          []LifecycleEvent
	Metadata        map[string]interface{}
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CreatedBy       common.UserID
	Version         int
}

// LifecycleSearchCriteria carries dynamic filter parameters.
type LifecycleSearchCriteria struct {
	PatentID string
	Phase    string
	Page     int
	PageSize int
}

// ─────────────────────────────────────────────────────────────────────────────
// LifecycleRepository
// ─────────────────────────────────────────────────────────────────────────────

// LifecycleRepository is the PostgreSQL implementation of the lifecycle
// domain's Repository interface.  Deadlines, annuity schedules, legal status
// history, and events are all persisted as JSONB columns, enabling rich
// server-side filtering via jsonb_array_elements without requiring separate
// normalised tables.
type LifecycleRepository struct {
	pool   *pgxpool.Pool
	logger Logger
}

// NewLifecycleRepository constructs a ready-to-use LifecycleRepository.
func NewLifecycleRepository(pool *pgxpool.Pool, logger Logger) *LifecycleRepository {
	return &LifecycleRepository{pool: pool, logger: logger}
}

// ─────────────────────────────────────────────────────────────────────────────
// Save
// ─────────────────────────────────────────────────────────────────────────────

func (r *LifecycleRepository) Save(ctx context.Context, lc *Lifecycle) error {
	r.logger.Debug("LifecycleRepository.Save", "lifecycle_id", lc.ID)

	deadlinesJSON, _ := json.Marshal(lc.Deadlines)
	annuityJSON, _ := json.Marshal(lc.AnnuitySchedule)
	legalJSON, _ := json.Marshal(lc.LegalStatus)
	eventsJSON, _ := json.Marshal(lc.Events)
	metaJSON, _ := json.Marshal(lc.Metadata)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO lifecycles (
			id, tenant_id, patent_id, phase,
			deadlines, annuity_schedule, legal_status, events,
			metadata, created_at, updated_at, created_by, version
		) VALUES (
			$1,$2,$3,$4,
			$5,$6,$7,$8,
			$9,$10,$11,$12,$13
		)`,
		lc.ID, lc.TenantID, lc.PatentID, lc.Phase,
		deadlinesJSON, annuityJSON, legalJSON, eventsJSON,
		metaJSON, lc.CreatedAt, lc.UpdatedAt, lc.CreatedBy, lc.Version,
	)
	if err != nil {
		r.logger.Error("LifecycleRepository.Save", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to insert lifecycle")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByID
// ─────────────────────────────────────────────────────────────────────────────

func (r *LifecycleRepository) FindByID(ctx context.Context, id common.ID) (*Lifecycle, error) {
	r.logger.Debug("LifecycleRepository.FindByID", "id", id)

	return r.scanLifecycle(r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, patent_id, phase,
		       deadlines, annuity_schedule, legal_status, events,
		       metadata, created_at, updated_at, created_by, version
		FROM lifecycles WHERE id = $1`, id))
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByPatentID
// ─────────────────────────────────────────────────────────────────────────────

func (r *LifecycleRepository) FindByPatentID(ctx context.Context, patentID common.ID) (*Lifecycle, error) {
	r.logger.Debug("LifecycleRepository.FindByPatentID", "patent_id", patentID)

	return r.scanLifecycle(r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, patent_id, phase,
		       deadlines, annuity_schedule, legal_status, events,
		       metadata, created_at, updated_at, created_by, version
		FROM lifecycles WHERE patent_id = $1`, patentID))
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByPhase
// ─────────────────────────────────────────────────────────────────────────────

func (r *LifecycleRepository) FindByPhase(ctx context.Context, phase string, page, pageSize int) ([]*Lifecycle, int64, error) {
	r.logger.Debug("LifecycleRepository.FindByPhase", "phase", phase)

	where := "WHERE phase = $1"

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM lifecycles %s", where), phase,
	).Scan(&total); err != nil {
		r.logger.Error("LifecycleRepository.FindByPhase: count", "error", err)
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
		SELECT id, tenant_id, patent_id, phase,
		       deadlines, annuity_schedule, legal_status, events,
		       metadata, created_at, updated_at, created_by, version
		FROM lifecycles %s
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3`, where), phase, pageSize, offset)
	if err != nil {
		r.logger.Error("LifecycleRepository.FindByPhase: query", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "query failed")
	}
	defer rows.Close()

	lifecycles, err := r.scanLifecycles(rows)
	return lifecycles, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// FindUpcomingDeadlines
//
// Uses jsonb_array_elements to unnest the JSONB deadlines array, then filters
// for entries whose due_date falls between NOW() and the supplied horizon, and
// whose status is still "pending".
//
// The query returns full Lifecycle rows that contain at least one matching
// deadline.  Client-side post-filtering extracts the specific deadline entries.
// ─────────────────────────────────────────────────────────────────────────────

func (r *LifecycleRepository) FindUpcomingDeadlines(
	ctx context.Context, before time.Time, page, pageSize int,
) ([]*Lifecycle, int64, error) {
	r.logger.Debug("LifecycleRepository.FindUpcomingDeadlines", "before", before)

	// Subquery: find lifecycle IDs that have at least one pending deadline
	// with due_date between now and the horizon.
	filterSQL := `
		SELECT DISTINCT l.id
		FROM lifecycles l,
		     jsonb_array_elements(l.deadlines) AS d
		WHERE (d->>'status') = 'pending'
		  AND (d->>'due_date')::timestamptz >= NOW()
		  AND (d->>'due_date')::timestamptz <= $1
	`

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM (%s) sub", filterSQL), before,
	).Scan(&total); err != nil {
		r.logger.Error("LifecycleRepository.FindUpcomingDeadlines: count", "error", err)
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
		SELECT l.id, l.tenant_id, l.patent_id, l.phase,
		       l.deadlines, l.annuity_schedule, l.legal_status, l.events,
		       l.metadata, l.created_at, l.updated_at, l.created_by, l.version
		FROM lifecycles l
		WHERE l.id IN (%s)
		ORDER BY l.updated_at DESC
		LIMIT $2 OFFSET $3`, filterSQL), before, pageSize, offset)
	if err != nil {
		r.logger.Error("LifecycleRepository.FindUpcomingDeadlines: query", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "query failed")
	}
	defer rows.Close()

	lifecycles, err := r.scanLifecycles(rows)
	return lifecycles, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// FindOverdueDeadlines
//
// Similar to FindUpcomingDeadlines but filters for deadlines whose due_date
// is in the past and whose status is NOT "completed".
// ─────────────────────────────────────────────────────────────────────────────

func (r *LifecycleRepository) FindOverdueDeadlines(
	ctx context.Context, page, pageSize int,
) ([]*Lifecycle, int64, error) {
	r.logger.Debug("LifecycleRepository.FindOverdueDeadlines")

	filterSQL := `
		SELECT DISTINCT l.id
		FROM lifecycles l,
		     jsonb_array_elements(l.deadlines) AS d
		WHERE (d->>'status') != 'completed'
		  AND (d->>'due_date')::timestamptz < NOW()
	`

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM (%s) sub", filterSQL),
	).Scan(&total); err != nil {
		r.logger.Error("LifecycleRepository.FindOverdueDeadlines: count", "error", err)
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
		SELECT l.id, l.tenant_id, l.patent_id, l.phase,
		       l.deadlines, l.annuity_schedule, l.legal_status, l.events,
		       l.metadata, l.created_at, l.updated_at, l.created_by, l.version
		FROM lifecycles l
		WHERE l.id IN (%s)
		ORDER BY l.updated_at DESC
		LIMIT $1 OFFSET $2`, filterSQL), pageSize, offset)
	if err != nil {
		r.logger.Error("LifecycleRepository.FindOverdueDeadlines: query", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "query failed")
	}
	defer rows.Close()

	lifecycles, err := r.scanLifecycles(rows)
	return lifecycles, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// FindUnpaidAnnuities
//
// Extracts annuity records from the JSONB annuity_schedule column where
// status = 'unpaid' and due_date falls before the supplied horizon.
// ─────────────────────────────────────────────────────────────────────────────

func (r *LifecycleRepository) FindUnpaidAnnuities(
	ctx context.Context, before time.Time, page, pageSize int,
) ([]*Lifecycle, int64, error) {
	r.logger.Debug("LifecycleRepository.FindUnpaidAnnuities", "before", before)

	filterSQL := `
		SELECT DISTINCT l.id
		FROM lifecycles l,
		     jsonb_array_elements(l.annuity_schedule) AS a
		WHERE (a->>'status') = 'unpaid'
		  AND (a->>'due_date')::timestamptz <= $1
	`

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM (%s) sub", filterSQL), before,
	).Scan(&total); err != nil {
		r.logger.Error("LifecycleRepository.FindUnpaidAnnuities: count", "error", err)
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
		SELECT l.id, l.tenant_id, l.patent_id, l.phase,
		       l.deadlines, l.annuity_schedule, l.legal_status, l.events,
		       l.metadata, l.created_at, l.updated_at, l.created_by, l.version
		FROM lifecycles l
		WHERE l.id IN (%s)
		ORDER BY l.updated_at DESC
		LIMIT $2 OFFSET $3`, filterSQL), before, pageSize, offset)
	if err != nil {
		r.logger.Error("LifecycleRepository.FindUnpaidAnnuities: query", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "query failed")
	}
	defer rows.Close()

	lifecycles, err := r.scanLifecycles(rows)
	return lifecycles, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// Search
// ─────────────────────────────────────────────────────────────────────────────

func (r *LifecycleRepository) Search(ctx context.Context, criteria LifecycleSearchCriteria) ([]*Lifecycle, int64, error) {
	r.logger.Debug("LifecycleRepository.Search", "criteria", criteria)

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

	if criteria.PatentID != "" {
		ph := nextArg(criteria.PatentID)
		conditions = append(conditions, fmt.Sprintf("patent_id = %s", ph))
	}
	if criteria.Phase != "" {
		ph := nextArg(criteria.Phase)
		conditions = append(conditions, fmt.Sprintf("phase = %s", ph))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM lifecycles %s", whereClause), args...,
	).Scan(&total); err != nil {
		r.logger.Error("LifecycleRepository.Search: count", "error", err)
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
		SELECT id, tenant_id, patent_id, phase,
		       deadlines, annuity_schedule, legal_status, events,
		       metadata, created_at, updated_at, created_by, version
		FROM lifecycles %s
		ORDER BY updated_at DESC
		LIMIT %s OFFSET %s`, whereClause, phLimit, phOffset), args...)
	if err != nil {
		r.logger.Error("LifecycleRepository.Search: query", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "search query failed")
	}
	defer rows.Close()

	lifecycles, err := r.scanLifecycles(rows)
	return lifecycles, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// Update — optimistic locking
// ─────────────────────────────────────────────────────────────────────────────

func (r *LifecycleRepository) Update(ctx context.Context, lc *Lifecycle) error {
	r.logger.Debug("LifecycleRepository.Update", "lifecycle_id", lc.ID, "version", lc.Version)

	deadlinesJSON, _ := json.Marshal(lc.Deadlines)
	annuityJSON, _ := json.Marshal(lc.AnnuitySchedule)
	legalJSON, _ := json.Marshal(lc.LegalStatus)
	eventsJSON, _ := json.Marshal(lc.Events)
	metaJSON, _ := json.Marshal(lc.Metadata)
	newVersion := lc.Version + 1

	tag, err := r.pool.Exec(ctx, `
		UPDATE lifecycles SET
			phase=$1,
			deadlines=$2, annuity_schedule=$3, legal_status=$4, events=$5,
			metadata=$6, updated_at=$7, version=$8
		WHERE id=$9 AND version=$10`,
		lc.Phase,
		deadlinesJSON, annuityJSON, legalJSON, eventsJSON,
		metaJSON, time.Now().UTC(), newVersion,
		lc.ID, lc.Version,
	)
	if err != nil {
		r.logger.Error("LifecycleRepository.Update", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to update lifecycle")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeConflict, "optimistic lock conflict: lifecycle version mismatch")
	}
	lc.Version = newVersion
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Delete
// ─────────────────────────────────────────────────────────────────────────────

func (r *LifecycleRepository) Delete(ctx context.Context, id common.ID) error {
	r.logger.Debug("LifecycleRepository.Delete", "id", id)

	tag, err := r.pool.Exec(ctx, `DELETE FROM lifecycles WHERE id = $1`, id)
	if err != nil {
		r.logger.Error("LifecycleRepository.Delete", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to delete lifecycle")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeNotFound, "lifecycle not found")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Count
// ─────────────────────────────────────────────────────────────────────────────

func (r *LifecycleRepository) Count(ctx context.Context) (int64, error) {
	r.logger.Debug("LifecycleRepository.Count")

	var count int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM lifecycles`).Scan(&count); err != nil {
		r.logger.Error("LifecycleRepository.Count", "error", err)
		return 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to count lifecycles")
	}
	return count, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// AddDeadline — atomic JSONB array append
// ─────────────────────────────────────────────────────────────────────────────

// AddDeadline appends a single deadline to the JSONB deadlines array of the
// specified lifecycle, using an atomic jsonb_set + || operation.
func (r *LifecycleRepository) AddDeadline(ctx context.Context, lifecycleID common.ID, d Deadline) error {
	r.logger.Debug("LifecycleRepository.AddDeadline", "lifecycle_id", lifecycleID, "deadline_id", d.ID)

	dJSON, _ := json.Marshal(d)

	tag, err := r.pool.Exec(ctx, `
		UPDATE lifecycles
		SET deadlines = COALESCE(deadlines, '[]'::jsonb) || $1::jsonb,
		    updated_at = NOW()
		WHERE id = $2`, string(dJSON), lifecycleID)
	if err != nil {
		r.logger.Error("LifecycleRepository.AddDeadline", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to add deadline")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeNotFound, "lifecycle not found")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// CompleteDeadline — atomic JSONB element update
// ─────────────────────────────────────────────────────────────────────────────

// CompleteDeadline marks a specific deadline as completed by iterating the
// JSONB array server-side.  This uses a CTE to rebuild the array with the
// target element's status changed to "completed".
func (r *LifecycleRepository) CompleteDeadline(ctx context.Context, lifecycleID common.ID, deadlineID string) error {
	r.logger.Debug("LifecycleRepository.CompleteDeadline", "lifecycle_id", lifecycleID, "deadline_id", deadlineID)

	now := time.Now().UTC().Format(time.RFC3339)

	// Rebuild the JSONB array, updating the matching element in-place.
	tag, err := r.pool.Exec(ctx, `
		UPDATE lifecycles
		SET deadlines = (
			SELECT jsonb_agg(
				CASE
					WHEN elem->>'id' = $1
					THEN elem || jsonb_build_object('status', 'completed', 'completed_at', $2::text)
					ELSE elem
				END
			)
			FROM jsonb_array_elements(deadlines) AS elem
		),
		updated_at = NOW()
		WHERE id = $3`, deadlineID, now, lifecycleID)
	if err != nil {
		r.logger.Error("LifecycleRepository.CompleteDeadline", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to complete deadline")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeNotFound, "lifecycle not found")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// RecordAnnuityPayment — atomic JSONB element update
// ─────────────────────────────────────────────────────────────────────────────

// RecordAnnuityPayment marks a specific annuity record as paid.
func (r *LifecycleRepository) RecordAnnuityPayment(
	ctx context.Context, lifecycleID common.ID, annuityID, paymentRef string,
) error {
	r.logger.Debug("LifecycleRepository.RecordAnnuityPayment",
		"lifecycle_id", lifecycleID, "annuity_id", annuityID)

	now := time.Now().UTC().Format(time.RFC3339)

	tag, err := r.pool.Exec(ctx, `
		UPDATE lifecycles
		SET annuity_schedule = (
			SELECT jsonb_agg(
				CASE
					WHEN elem->>'id' = $1
					THEN elem || jsonb_build_object('status', 'paid', 'paid_at', $2::text, 'payment_ref', $3)
					ELSE elem
				END
			)
			FROM jsonb_array_elements(annuity_schedule) AS elem
		),
		updated_at = NOW()
		WHERE id = $4`, annuityID, now, paymentRef, lifecycleID)
	if err != nil {
		r.logger.Error("LifecycleRepository.RecordAnnuityPayment", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to record annuity payment")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeNotFound, "lifecycle not found")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// AddEvent — atomic JSONB array append
// ─────────────────────────────────────────────────────────────────────────────

func (r *LifecycleRepository) AddEvent(ctx context.Context, lifecycleID common.ID, evt LifecycleEvent) error {
	r.logger.Debug("LifecycleRepository.AddEvent", "lifecycle_id", lifecycleID, "event_id", evt.ID)

	evtJSON, _ := json.Marshal(evt)

	tag, err := r.pool.Exec(ctx, `
		UPDATE lifecycles
		SET events = COALESCE(events, '[]'::jsonb) || $1::jsonb,
		    updated_at = NOW()
		WHERE id = $2`, string(evtJSON), lifecycleID)
	if err != nil {
		r.logger.Error("LifecycleRepository.AddEvent", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to add event")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeNotFound, "lifecycle not found")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal scanners
// ─────────────────────────────────────────────────────────────────────────────

func (r *LifecycleRepository) scanLifecycle(row pgx.Row) (*Lifecycle, error) {
	var lc Lifecycle
	var deadlinesJSON, annuityJSON, legalJSON, eventsJSON, metaJSON []byte

	err := row.Scan(
		&lc.ID, &lc.TenantID, &lc.PatentID, &lc.Phase,
		&deadlinesJSON, &annuityJSON, &legalJSON, &eventsJSON,
		&metaJSON, &lc.CreatedAt, &lc.UpdatedAt, &lc.CreatedBy, &lc.Version,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, appErrors.New(appErrors.CodeNotFound, "lifecycle not found")
		}
		r.logger.Error("scanLifecycle", "error", err)
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to scan lifecycle")
	}

	if len(deadlinesJSON) > 0 {
		_ = json.Unmarshal(deadlinesJSON, &lc.Deadlines)
	}
	if len(annuityJSON) > 0 {
		_ = json.Unmarshal(annuityJSON, &lc.AnnuitySchedule)
	}
	if len(legalJSON) > 0 {
		_ = json.Unmarshal(legalJSON, &lc.LegalStatus)
	}
	if len(eventsJSON) > 0 {
		_ = json.Unmarshal(eventsJSON, &lc.Events)
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &lc.Metadata)
	}
	return &lc, nil
}

func (r *LifecycleRepository) scanLifecycles(rows pgx.Rows) ([]*Lifecycle, error) {
	var lifecycles []*Lifecycle
	for rows.Next() {
		var lc Lifecycle
		var deadlinesJSON, annuityJSON, legalJSON, eventsJSON, metaJSON []byte

		err := rows.Scan(
			&lc.ID, &lc.TenantID, &lc.PatentID, &lc.Phase,
			&deadlinesJSON, &annuityJSON, &legalJSON, &eventsJSON,
			&metaJSON, &lc.CreatedAt, &lc.UpdatedAt, &lc.CreatedBy, &lc.Version,
		)
		if err != nil {
			r.logger.Error("scanLifecycles", "error", err)
			return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to scan lifecycle row")
		}

		if len(deadlinesJSON) > 0 {
			_ = json.Unmarshal(deadlinesJSON, &lc.Deadlines)
		}
		if len(annuityJSON) > 0 {
			_ = json.Unmarshal(annuityJSON, &lc.AnnuitySchedule)
		}
		if len(legalJSON) > 0 {
			_ = json.Unmarshal(legalJSON, &lc.LegalStatus)
		}
		if len(eventsJSON) > 0 {
			_ = json.Unmarshal(eventsJSON, &lc.Events)
		}
		if len(metaJSON) > 0 {
			_ = json.Unmarshal(metaJSON, &lc.Metadata)
		}
		lifecycles = append(lifecycles, &lc)
	}
	if err := rows.Err(); err != nil {
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "row iteration error")
	}
	return lifecycles, nil
}

