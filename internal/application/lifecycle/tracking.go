// Package lifecycle provides lifecycle tracking application services.
package lifecycle

import (
	"context"
	"time"
)

// TrackingService provides patent lifecycle tracking operations.
// This interface is used by HTTP handlers for lifecycle management.
type TrackingService interface {
	GetLifecycle(ctx context.Context, patentID string) (*Lifecycle, error)
	AdvancePhase(ctx context.Context, input *AdvancePhaseInput) (*Lifecycle, error)
	AddMilestone(ctx context.Context, input *AddMilestoneInput) (*Milestone, error)
	ListMilestones(ctx context.Context, patentID string) (*MilestoneList, error)
	RecordFee(ctx context.Context, input *RecordFeeInput) (*Fee, error)
	ListFees(ctx context.Context, patentID string) (*FeeList, error)
	GetTimeline(ctx context.Context, patentID string) (*Timeline, error)
	GetUpcomingDeadlines(ctx context.Context, input *UpcomingDeadlinesInput) ([]*DeadlineInfo, error)
}

// Lifecycle represents a patent lifecycle.
type Lifecycle struct {
	PatentID    string    `json:"patent_id"`
	Phase       string    `json:"phase"`
	Status      string    `json:"status"`
	StartDate   time.Time `json:"start_date"`
	CurrentDate time.Time `json:"current_date"`
	NextDeadline *time.Time `json:"next_deadline,omitempty"`
}

// AdvancePhaseInput contains input for advancing lifecycle phase.
type AdvancePhaseInput struct {
	PatentID string
	NewPhase string
	Date     time.Time
	Notes    string
	UserID   string
}

// Milestone represents a lifecycle milestone.
type Milestone struct {
	ID        string    `json:"id"`
	PatentID  string    `json:"patent_id"`
	Type      string    `json:"type"`
	Date      time.Time `json:"date"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// AddMilestoneInput contains input for adding a milestone.
type AddMilestoneInput struct {
	PatentID string
	Type     string
	Date     time.Time
	Notes    string
	UserID   string
}

// MilestoneList represents a list of milestones.
type MilestoneList struct {
	Milestones []*Milestone `json:"milestones"`
	Total      int          `json:"total"`
}

// Fee represents a lifecycle fee.
type Fee struct {
	ID        string    `json:"id"`
	PatentID  string    `json:"patent_id"`
	Type      string    `json:"type"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	DueDate   time.Time `json:"due_date"`
	PaidDate  *time.Time `json:"paid_date,omitempty"`
	Status    string    `json:"status"`
}

// RecordFeeInput contains input for recording a fee.
type RecordFeeInput struct {
	PatentID string
	Type     string
	Amount   float64
	Currency string
	DueDate  time.Time
	UserID   string
}

// FeeList represents a list of fees.
type FeeList struct {
	Fees  []*Fee `json:"fees"`
	Total int    `json:"total"`
}

// Timeline represents a patent timeline.
type Timeline struct {
	PatentID string           `json:"patent_id"`
	Events   []*TimelineEvent `json:"events"`
}

// TimelineEvent represents a timeline event.
type TimelineEvent struct {
	Date        time.Time `json:"date"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
}

// DeadlineInfo represents an upcoming deadline for the tracking service.
type DeadlineInfo struct {
	PatentID    string    `json:"patent_id"`
	Type        string    `json:"type"`
	DueDate     time.Time `json:"due_date"`
	Description string    `json:"description"`
	Priority    string    `json:"priority"`
}

// DeadlineQueryInput contains input for querying deadlines.
type DeadlineQueryInput struct {
	Days         int
	Jurisdiction string
	PatentIDs    []string
}

// UpcomingDeadlinesInput contains input for getting upcoming deadlines.
type UpcomingDeadlinesInput struct {
	Days         int      `json:"days"`
	Jurisdiction string   `json:"jurisdiction,omitempty"`
	PatentIDs    []string `json:"patent_ids,omitempty"`
	Types        []string `json:"types,omitempty"`
}

//Personal.AI order the ending
