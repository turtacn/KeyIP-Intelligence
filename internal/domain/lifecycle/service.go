// Package lifecycle provides the application service layer for patent lifecycle
// management operations.
package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// Service interface
// ─────────────────────────────────────────────────────────────────────────────

// Service defines the application-level operations for patent lifecycle management.
// It orchestrates domain logic and repository operations.
type Service interface {
	// CreateLifecycle creates a new patent lifecycle record.
	CreateLifecycle(ctx context.Context, cmd CreateLifecycleCommand) (*PatentLifecycle, error)

	// GetLifecycle retrieves a lifecycle by ID.
	GetLifecycle(ctx context.Context, id common.ID) (*PatentLifecycle, error)

	// GetLifecycleByPatentNumber retrieves a lifecycle by patent number.
	GetLifecycleByPatentNumber(ctx context.Context, patentNumber string) (*PatentLifecycle, error)

	// AddDeadline adds a new deadline to a lifecycle.
	AddDeadline(ctx context.Context, lifecycleID common.ID, cmd AddDeadlineCommand) error

	// CompleteDeadline marks a deadline as completed.
	CompleteDeadline(ctx context.Context, lifecycleID common.ID, deadlineID common.ID) error

	// ExtendDeadline extends a deadline by the specified number of days.
	ExtendDeadline(ctx context.Context, lifecycleID common.ID, deadlineID common.ID, days int) error

	// RecordAnnuityPayment records a payment for an annuity.
	RecordAnnuityPayment(ctx context.Context, lifecycleID common.ID, annuityID common.ID, amount float64) error

	// UpdateLegalStatus updates the legal status of a patent.
	UpdateLegalStatus(ctx context.Context, lifecycleID common.ID, newStatus, reason string) error

	// GetUpcomingDeadlines retrieves all lifecycles with upcoming deadlines.
	GetUpcomingDeadlines(ctx context.Context, withinDays int, tenantID *common.TenantID) ([]*PatentLifecycle, error)

	// GetOverdueDeadlines retrieves all lifecycles with overdue deadlines.
	GetOverdueDeadlines(ctx context.Context, tenantID *common.TenantID) ([]*PatentLifecycle, error)

	// GetUpcomingAnnuities retrieves all lifecycles with upcoming annuity payments.
	GetUpcomingAnnuities(ctx context.Context, withinDays int, tenantID *common.TenantID) ([]*PatentLifecycle, error)

	// ListLifecycles retrieves lifecycles with pagination.
	ListLifecycles(ctx context.Context, offset, limit int, tenantID *common.TenantID) ([]*PatentLifecycle, int64, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Command DTOs
// ─────────────────────────────────────────────────────────────────────────────

// CreateLifecycleCommand contains parameters for creating a new lifecycle.
type CreateLifecycleCommand struct {
	PatentID     common.ID                `json:"patent_id"`
	PatentNumber string                   `json:"patent_number"`
	Jurisdiction ptypes.JurisdictionCode  `json:"jurisdiction"`
	FilingDate   time.Time                `json:"filing_date"`
	GrantDate    *time.Time               `json:"grant_date,omitempty"`
}

// AddDeadlineCommand contains parameters for adding a deadline.
type AddDeadlineCommand struct {
	Type        DeadlineType     `json:"type"`
	DueDate     time.Time        `json:"due_date"`
	Priority    DeadlinePriority `json:"priority"`
	Description string           `json:"description"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Service implementation
// ─────────────────────────────────────────────────────────────────────────────

type service struct {
	repo Repository
}

// NewService creates a new lifecycle service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreateLifecycle(ctx context.Context, cmd CreateLifecycleCommand) (*PatentLifecycle, error) {
	// Validate command.
	if cmd.PatentID == "" {
		return nil, errors.InvalidParam("patent_id is required")
	}
	if cmd.PatentNumber == "" {
		return nil, errors.InvalidParam("patent_number is required")
	}
	if cmd.FilingDate.IsZero() {
		return nil, errors.InvalidParam("filing_date is required")
	}

	// Check if lifecycle already exists for this patent.
	existing, err := s.repo.FindByPatentID(ctx, cmd.PatentID)
	if err == nil && existing != nil {
		return nil, errors.Conflict(fmt.Sprintf("lifecycle already exists for patent %s", cmd.PatentID))
	}

	// Create new lifecycle aggregate.
	lc, err := NewPatentLifecycle(cmd.PatentID, cmd.PatentNumber, cmd.Jurisdiction, cmd.FilingDate)
	if err != nil {
		return nil, err
	}

	// If grant date is provided, update legal status and regenerate schedule.
	if cmd.GrantDate != nil {
		if err := lc.Grant(*cmd.GrantDate); err != nil {
			return nil, err
		}
	}

	// Persist.
	if err := s.repo.Save(ctx, lc); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to save lifecycle")
	}

	return lc, nil
}

func (s *service) GetLifecycle(ctx context.Context, id common.ID) (*PatentLifecycle, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *service) GetLifecycleByPatentNumber(ctx context.Context, patentNumber string) (*PatentLifecycle, error) {
	return s.repo.FindByPatentNumber(ctx, patentNumber)
}

func (s *service) AddDeadline(ctx context.Context, lifecycleID common.ID, cmd AddDeadlineCommand) error {
	lc, err := s.repo.FindByID(ctx, lifecycleID)
	if err != nil {
		return err
	}

	deadline, err := NewDeadline(cmd.Type, cmd.DueDate, cmd.Priority, cmd.Description)
	if err != nil {
		return err
	}

	if err := lc.AddDeadline(*deadline); err != nil {
		return err
	}

	return s.repo.Save(ctx, lc)
}

func (s *service) CompleteDeadline(ctx context.Context, lifecycleID common.ID, deadlineID common.ID) error {
	lc, err := s.repo.FindByID(ctx, lifecycleID)
	if err != nil {
		return err
	}

	if err := lc.MarkDeadlineCompleted(deadlineID); err != nil {
		return err
	}

	return s.repo.Save(ctx, lc)
}

func (s *service) ExtendDeadline(ctx context.Context, lifecycleID common.ID, deadlineID common.ID, days int) error {
	lc, err := s.repo.FindByID(ctx, lifecycleID)
	if err != nil {
		return err
	}

	// Find the deadline.
	var found *Deadline
	for i := range lc.Deadlines {
		if lc.Deadlines[i].ID == deadlineID {
			found = &lc.Deadlines[i]
			break
		}
	}
	if found == nil {
		return errors.NotFound(fmt.Sprintf("deadline %s not found", deadlineID))
	}

	if err := found.Extend(days); err != nil {
		return err
	}

	lc.Events = append(lc.Events, LifecycleEvent{
		Type:        "deadline_extended",
		Date:        time.Now().UTC(),
		Description: fmt.Sprintf("deadline %s extended by %d days", deadlineID, days),
		Handled:     false,
	})

	return s.repo.Save(ctx, lc)
}

func (s *service) RecordAnnuityPayment(ctx context.Context, lifecycleID common.ID, annuityID common.ID, amount float64) error {
	lc, err := s.repo.FindByID(ctx, lifecycleID)
	if err != nil {
		return err
	}

	if err := lc.RecordPayment(annuityID, amount, time.Now().UTC()); err != nil {
		return err
	}

	return s.repo.Save(ctx, lc)
}

func (s *service) UpdateLegalStatus(ctx context.Context, lifecycleID common.ID, newStatus, reason string) error {
	lc, err := s.repo.FindByID(ctx, lifecycleID)
	if err != nil {
		return err
	}

	if err := lc.UpdateLegalStatus(newStatus, reason); err != nil {
		return err
	}

	return s.repo.Save(ctx, lc)
}

func (s *service) GetUpcomingDeadlines(ctx context.Context, withinDays int, tenantID *common.TenantID) ([]*PatentLifecycle, error) {
	return s.repo.FindUpcomingDeadlines(ctx, withinDays, tenantID)
}

func (s *service) GetOverdueDeadlines(ctx context.Context, tenantID *common.TenantID) ([]*PatentLifecycle, error) {
	return s.repo.FindOverdueDeadlines(ctx, tenantID)
}

func (s *service) GetUpcomingAnnuities(ctx context.Context, withinDays int, tenantID *common.TenantID) ([]*PatentLifecycle, error) {
	return s.repo.FindUpcomingAnnuities(ctx, withinDays, tenantID)
}

func (s *service) ListLifecycles(ctx context.Context, offset, limit int, tenantID *common.TenantID) ([]*PatentLifecycle, int64, error) {
	return s.repo.List(ctx, offset, limit, tenantID)
}

//Personal.AI order the ending
