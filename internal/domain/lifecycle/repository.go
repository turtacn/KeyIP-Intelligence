package lifecycle

import (
	"context"
	"time"

	"github.com/google/uuid"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// LifecycleRepository defines the persistence contract for lifecycle domain.
type LifecycleRepository interface {
	// Annuity
	CreateAnnuity(ctx context.Context, annuity *Annuity) error
	GetAnnuity(ctx context.Context, id uuid.UUID) (*Annuity, error)
	GetAnnuitiesByPatent(ctx context.Context, patentID uuid.UUID) ([]*Annuity, error)
	GetUpcomingAnnuities(ctx context.Context, daysAhead int, limit, offset int) ([]*Annuity, int64, error)
	GetOverdueAnnuities(ctx context.Context, limit, offset int) ([]*Annuity, int64, error)
	UpdateAnnuityStatus(ctx context.Context, id uuid.UUID, status AnnuityStatus, paidAmount int64, paidDate *time.Time, paymentRef string) error
	BatchCreateAnnuities(ctx context.Context, annuities []*Annuity) error
	UpdateReminderSent(ctx context.Context, id uuid.UUID) error

	// Deadline
	CreateDeadline(ctx context.Context, deadline *Deadline) error
	GetDeadline(ctx context.Context, id uuid.UUID) (*Deadline, error)
	GetDeadlinesByPatent(ctx context.Context, patentID uuid.UUID, statusFilter []DeadlineStatus) ([]*Deadline, error)
	GetActiveDeadlines(ctx context.Context, userID *uuid.UUID, daysAhead int, limit, offset int) ([]*Deadline, int64, error)
	UpdateDeadlineStatus(ctx context.Context, id uuid.UUID, status DeadlineStatus, completedBy *uuid.UUID) error
	ExtendDeadline(ctx context.Context, id uuid.UUID, newDueDate time.Time, reason string) error
	GetCriticalDeadlines(ctx context.Context, limit int) ([]*Deadline, error)

	// Event
	CreateEvent(ctx context.Context, event *LifecycleEvent) error
	GetEventsByPatent(ctx context.Context, patentID uuid.UUID, eventTypes []EventType, limit, offset int) ([]*LifecycleEvent, int64, error)
	GetEventTimeline(ctx context.Context, patentID uuid.UUID) ([]*LifecycleEvent, error)
	GetRecentEvents(ctx context.Context, orgID uuid.UUID, limit int) ([]*LifecycleEvent, error)

	// Cost
	CreateCostRecord(ctx context.Context, record *CostRecord) error
	GetCostsByPatent(ctx context.Context, patentID uuid.UUID) ([]*CostRecord, error)
	GetCostSummary(ctx context.Context, patentID uuid.UUID) (*CostSummary, error)
	GetPortfolioCostSummary(ctx context.Context, portfolioID uuid.UUID, startDate, endDate time.Time) (*PortfolioCostSummary, error)

	// Dashboard
	GetLifecycleDashboard(ctx context.Context, orgID uuid.UUID) (*DashboardStats, error)

	// Payment (Added for AnnuityService)
	SavePayment(ctx context.Context, payment *PaymentRecord) (*PaymentRecord, error)
	QueryPayments(ctx context.Context, query *PaymentQuery) ([]PaymentRecord, int64, error)

	// Legal Status (Added for LegalStatusService)
	GetByPatentID(ctx context.Context, patentID string) (*LegalStatusEntity, error)
	UpdateStatus(ctx context.Context, patentID string, status string, effectiveDate time.Time) error
	SaveSubscription(ctx context.Context, sub *SubscriptionEntity) error
	DeactivateSubscription(ctx context.Context, id string) error
	GetStatusHistory(ctx context.Context, patentID string, pagination *commontypes.Pagination, from, to *time.Time) ([]*StatusHistoryEntity, error)

	// Custom Event (Added for CalendarService)
	SaveCustomEvent(ctx context.Context, event *CustomEvent) error
	GetCustomEvents(ctx context.Context, patentIDs []string, start, end time.Time) ([]CustomEvent, error)
	UpdateEventStatus(ctx context.Context, eventID string, status string) error
	DeleteEvent(ctx context.Context, eventID string) error

	// Transaction
	WithTx(ctx context.Context, fn func(LifecycleRepository) error) error
}

// Service defines the domain service interface.
type Service interface {
	CalculateAnnuityFee(ctx context.Context, patentID string, jurisdiction Jurisdiction, asOf time.Time) (*AnnuityCalcResult, error)
	GetAnnuitySchedule(ctx context.Context, patentID string, jurisdiction Jurisdiction, start, end time.Time) ([]ScheduleEntry, error)
	FetchRemoteStatus(ctx context.Context, patentID string) (*RemoteStatusResult, error)
}

//Personal.AI order the ending
