package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// AnnuityService defines the domain service for annuity calculations.
// It abstracts away the complex logic of fee schedules and rules.
type AnnuityService interface {
	CalculateFee(ctx context.Context, jurisdiction Jurisdiction, year int, filingDate, grantDate *time.Time) (*AnnuityCalcResult, error)
	GetSchedule(ctx context.Context, jurisdiction Jurisdiction, filingDate, grantDate *time.Time) ([]ScheduleEntry, error)
}

// DeadlineService defines the domain service for deadline calculations.
type DeadlineService interface {
	CalculateDeadlines(ctx context.Context, eventType EventType, eventDate time.Time, jurisdiction Jurisdiction) ([]*Deadline, error)
}

// Service defines the main lifecycle application service.
type Service interface {
	CalculateAnnuityFee(ctx context.Context, patentID string, jurisdiction Jurisdiction, asOf time.Time) (*AnnuityCalcResult, error)
	GetAnnuitySchedule(ctx context.Context, patentID string, jurisdiction Jurisdiction, start, end time.Time) ([]ScheduleEntry, error)
	FetchRemoteStatus(ctx context.Context, patentID string) (*RemoteStatusResult, error)
	ProcessDailyMaintenance(ctx context.Context) error
	CheckHealth(ctx context.Context, patentID string) (string, error)
}

// Service is an alias for Service for backward compatibility.
type LifecycleService = Service

type lifecycleServiceImpl struct {
	repo               LifecycleRepository
	annuityService     AnnuityService
	deadlineService    DeadlineService
	jurisdictionReg    JurisdictionRegistry
}

// NewService creates a new lifecycle Service.
func NewService(
	repo LifecycleRepository,
	annuityService AnnuityService,
	deadlineService DeadlineService,
	jurisdictionReg JurisdictionRegistry,
) Service {
	return &lifecycleServiceImpl{
		repo:            repo,
		annuityService:  annuityService,
		deadlineService: deadlineService,
		jurisdictionReg: jurisdictionReg,
	}
}

// CalculateAnnuityFee calculates the annuity fee for a patent.
func (s *lifecycleServiceImpl) CalculateAnnuityFee(ctx context.Context, patentID string, jurisdiction Jurisdiction, asOf time.Time) (*AnnuityCalcResult, error) {
	// In a real implementation, this would fetch patent details (dates) and call annuityService
	// For now, we return a mock result as placeholders
	return &AnnuityCalcResult{
		Fee:            100000, // 1000.00
		Currency:       "USD",
		YearNumber:     1,
		DueDate:        asOf,
		GracePeriodEnd: asOf.AddDate(0, 6, 0),
		Status:         string(AnnuityStatusUpcoming),
	}, nil
}

// GetAnnuitySchedule returns the annuity schedule for a patent.
func (s *lifecycleServiceImpl) GetAnnuitySchedule(ctx context.Context, patentID string, jurisdiction Jurisdiction, start, end time.Time) ([]ScheduleEntry, error) {
	// Placeholder implementation
	return []ScheduleEntry{}, nil
}

// FetchRemoteStatus fetches the remote status of a patent.
func (s *lifecycleServiceImpl) FetchRemoteStatus(ctx context.Context, patentID string) (*RemoteStatusResult, error) {
	// Placeholder implementation
	return &RemoteStatusResult{
		Status:        "unknown",
		EffectiveDate: time.Now(),
		NextAction:    "",
		Source:        "mock",
		Jurisdiction:  "US",
	}, nil
}

// ProcessDailyMaintenance runs daily checks for annuities and deadlines.
// It collects errors instead of failing fast.
func (s *lifecycleServiceImpl) ProcessDailyMaintenance(ctx context.Context) error {
	var errs error

	// 1. Check upcoming annuities (e.g., due in 90 days)
	annuities, _, err := s.repo.GetUpcomingAnnuities(ctx, 90, 100, 0)
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("failed to fetch upcoming annuities: %w", err))
	} else {
		for _, a := range annuities {
			a.CheckStatus(time.Now().UTC())
			if err := s.repo.UpdateAnnuityStatus(ctx, a.ID, a.Status, 0, nil, ""); err != nil {
				errs = errors.Join(errs, fmt.Errorf("failed to update annuity %s: %w", a.ID, err))
			}
		}
	}

	// 2. Check active deadlines
	// This would require a user ID context or system-wide fetch.
	// Assuming GetActiveDeadlines handles nil userID for "all active"
	deadlines, _, err := s.repo.GetActiveDeadlines(ctx, nil, 30, 100, 0)
	if err != nil {
		errs = errors.Join(errs, fmt.Errorf("failed to fetch active deadlines: %w", err))
	} else {
		for _, d := range deadlines {
			// Logic to check expiry/reminders would go here
			// For now, just logging or updating internal state if needed
			if d.CheckUrgency() == UrgencyOverdue {
				// Mark as missed or send alert
				// d.Status = DeadlineStatusMissed (if auto-miss logic exists)
			}
		}
	}

	return errs
}

// CheckHealth determines the health status of a patent lifecycle.
func (s *lifecycleServiceImpl) CheckHealth(ctx context.Context, patentID string) (string, error) {
	// Check for overdue annuities
	annuities, err := s.repo.GetAnnuitiesByPatent(ctx, patentID)
	if err != nil {
		return "unknown", err
	}
	for _, a := range annuities {
		if a.Status == AnnuityStatusOverdue {
			return "critical", nil
		}
	}

	// Check for critical deadlines
	deadlines, err := s.repo.GetDeadlinesByPatent(ctx, patentID, []DeadlineStatus{DeadlineStatusActive})
	if err != nil {
		return "unknown", err
	}
	for _, d := range deadlines {
		if d.CheckUrgency() == UrgencyCritical || d.CheckUrgency() == UrgencyOverdue {
			return "warning", nil
		}
	}

	return "healthy", nil
}

//Personal.AI order the ending
