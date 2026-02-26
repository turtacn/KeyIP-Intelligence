// Phase 13 - SDK Lifecycle Management Sub-Client (294/349)
// File: pkg/client/lifecycle.go
// Patent lifecycle management: deadlines, annuities, legal status, reminders.

package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// ---------------------------------------------------------------------------
// DTOs — request / response
// ---------------------------------------------------------------------------

// DeadlineQuery describes filters for deadline retrieval.
type DeadlineQuery struct {
	PatentNumbers []string `json:"patent_numbers,omitempty"`
	Jurisdictions []string `json:"jurisdictions,omitempty"`
	DaysAhead     int      `json:"days_ahead,omitempty"`
}

// Deadline represents a single patent-related deadline.
type Deadline struct {
	ID            string `json:"id"`
	PatentNumber  string `json:"patent_number"`
	Jurisdiction  string `json:"jurisdiction"`
	DeadlineType  string `json:"deadline_type"`
	DueDate       string `json:"due_date"`
	DaysRemaining int    `json:"days_remaining"`
	Status        string `json:"status"`
	Priority      string `json:"priority"`
	Description   string `json:"description"`
}

// AnnuityRecord represents a single annuity fee record.
type AnnuityRecord struct {
	ID            string  `json:"id"`
	PatentNumber  string  `json:"patent_number"`
	Jurisdiction  string  `json:"jurisdiction"`
	Year          int     `json:"year"`
	DueDate       string  `json:"due_date"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Status        string  `json:"status"`
	PaidDate      string  `json:"paid_date,omitempty"`
	Receipt       string  `json:"receipt,omitempty"`
}

// AnnuityPayRequest describes a payment recording request.
type AnnuityPayRequest struct {
	AnnuityID string  `json:"annuity_id"`
	PaidDate  string  `json:"paid_date"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	Receipt   string  `json:"receipt,omitempty"`
	Notes     string  `json:"notes,omitempty"`
}

// LegalStatusRecord holds the current and historical legal status of a patent.
type LegalStatusRecord struct {
	PatentNumber  string              `json:"patent_number"`
	Jurisdiction  string              `json:"jurisdiction"`
	CurrentStatus string              `json:"current_status"`
	StatusDate    string              `json:"status_date"`
	History       []LegalStatusChange `json:"history"`
}

// LegalStatusChange represents a single legal status transition.
type LegalStatusChange struct {
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
	ChangeDate string `json:"change_date"`
	Reason     string `json:"reason"`
	Source     string `json:"source"`
}

// ReminderConfig describes a configured reminder for a patent deadline.
type ReminderConfig struct {
	ID           string   `json:"id"`
	PatentNumber string   `json:"patent_number"`
	DeadlineType string   `json:"deadline_type"`
	ReminderDays []int    `json:"reminder_days"`
	Channels     []string `json:"channels"`
	Recipients   []string `json:"recipients"`
	Enabled      bool     `json:"enabled"`
}

// ReminderConfigRequest describes a request to create or update a reminder.
type ReminderConfigRequest struct {
	PatentNumber string   `json:"patent_number"`
	DeadlineType string   `json:"deadline_type"`
	ReminderDays []int    `json:"reminder_days"`
	Channels     []string `json:"channels"`
	Recipients   []string `json:"recipients"`
}

// AnnuitySummary provides an aggregate view of annuity obligations.
type AnnuitySummary struct {
	TotalDue         int                                   `json:"total_due"`
	TotalPaid        int                                   `json:"total_paid"`
	TotalOverdue     int                                   `json:"total_overdue"`
	TotalAmountDue   float64                               `json:"total_amount_due"`
	TotalAmountPaid  float64                               `json:"total_amount_paid"`
	Currency         string                                `json:"currency"`
	ByJurisdiction   map[string]AnnuityJurisdictionSummary `json:"by_jurisdiction"`
}

// AnnuityJurisdictionSummary provides per-jurisdiction annuity stats.
type AnnuityJurisdictionSummary struct {
	Jurisdiction string  `json:"jurisdiction"`
	Due          int     `json:"due"`
	Paid         int     `json:"paid"`
	Overdue      int     `json:"overdue"`
	AmountDue    float64 `json:"amount_due"`
	AmountPaid   float64 `json:"amount_paid"`
	Currency     string  `json:"currency"`
}

// ---------------------------------------------------------------------------
// LifecycleClient
// ---------------------------------------------------------------------------

// LifecycleClient provides access to patent lifecycle management endpoints.
type LifecycleClient struct {
	client *Client
}

func newLifecycleClient(c *Client) *LifecycleClient {
	return &LifecycleClient{client: c}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// buildDeadlineQueryParams encodes a DeadlineQuery into a URL query string.
func (lc *LifecycleClient) buildDeadlineQueryParams(query *DeadlineQuery) string {
	params := url.Values{}

	daysAhead := query.DaysAhead
	if daysAhead <= 0 {
		daysAhead = 90
	}
	params.Set("days_ahead", fmt.Sprintf("%d", daysAhead))

	if len(query.PatentNumbers) > 0 {
		params.Set("patent_numbers", strings.Join(query.PatentNumbers, ","))
	}
	if len(query.Jurisdictions) > 0 {
		params.Set("jurisdictions", strings.Join(query.Jurisdictions, ","))
	}

	return params.Encode()
}

// ---------------------------------------------------------------------------
// Public methods
// ---------------------------------------------------------------------------

// GetDeadlines retrieves upcoming deadlines matching the query.
// GET /api/v1/lifecycle/deadlines?...
func (lc *LifecycleClient) GetDeadlines(ctx context.Context, query *DeadlineQuery) ([]Deadline, error) {
	if query == nil {
		query = &DeadlineQuery{}
	}
	qs := lc.buildDeadlineQueryParams(query)
	path := "/api/v1/lifecycle/deadlines?" + qs

	var resp APIResponse[[]Deadline]
	if err := lc.client.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	if resp.Data == nil {
		resp.Data = []Deadline{}
	}
	return resp.Data, nil
}

// GetDeadline retrieves a single deadline by ID.
// GET /api/v1/lifecycle/deadlines/{deadlineID}
func (lc *LifecycleClient) GetDeadline(ctx context.Context, deadlineID string) (*Deadline, error) {
	if deadlineID == "" {
		return nil, invalidArg("deadlineID is required")
	}
	var resp APIResponse[Deadline]
	if err := lc.client.get(ctx, "/api/v1/lifecycle/deadlines/"+deadlineID, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetAnnuities retrieves annuity records for a patent.
// GET /api/v1/lifecycle/patents/{patentNumber}/annuities
func (lc *LifecycleClient) GetAnnuities(ctx context.Context, patentNumber string) ([]AnnuityRecord, error) {
	if patentNumber == "" {
		return nil, invalidArg("patentNumber is required")
	}
	var resp APIResponse[[]AnnuityRecord]
	if err := lc.client.get(ctx, "/api/v1/lifecycle/patents/"+url.PathEscape(patentNumber)+"/annuities", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetAnnuitySummary retrieves an aggregate annuity summary.
// GET /api/v1/lifecycle/annuities/summary
func (lc *LifecycleClient) GetAnnuitySummary(ctx context.Context) (*AnnuitySummary, error) {
	var resp APIResponse[AnnuitySummary]
	if err := lc.client.get(ctx, "/api/v1/lifecycle/annuities/summary", &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// RecordAnnuityPayment records a payment for an annuity.
// POST /api/v1/lifecycle/annuities/payments
func (lc *LifecycleClient) RecordAnnuityPayment(ctx context.Context, req *AnnuityPayRequest) (*AnnuityRecord, error) {
	if req == nil || req.AnnuityID == "" {
		return nil, invalidArg("annuity_id is required")
	}
	if req.PaidDate == "" {
		return nil, invalidArg("paid_date is required")
	}
	if req.Amount <= 0 {
		return nil, invalidArg("amount must be greater than 0")
	}
	var resp APIResponse[AnnuityRecord]
	if err := lc.client.post(ctx, "/api/v1/lifecycle/annuities/payments", req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetLegalStatus retrieves the legal status of a patent.
// GET /api/v1/lifecycle/patents/{patentNumber}/legal-status
func (lc *LifecycleClient) GetLegalStatus(ctx context.Context, patentNumber string) (*LegalStatusRecord, error) {
	if patentNumber == "" {
		return nil, invalidArg("patentNumber is required")
	}
	var resp APIResponse[LegalStatusRecord]
	if err := lc.client.get(ctx, "/api/v1/lifecycle/patents/"+url.PathEscape(patentNumber)+"/legal-status", &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// SyncLegalStatus triggers an asynchronous legal status sync from patent offices.
// POST /api/v1/lifecycle/legal-status/sync
// Returns nil on success (202 Accepted). The sync runs asynchronously.
func (lc *LifecycleClient) SyncLegalStatus(ctx context.Context, patentNumbers []string) error {
	if len(patentNumbers) == 0 {
		return invalidArg("patent_numbers is required")
	}
	body := map[string]interface{}{
		"patent_numbers": patentNumbers,
	}
	if err := lc.client.post(ctx, "/api/v1/lifecycle/legal-status/sync", body, nil); err != nil {
		return err
	}
	return nil
}

// GetReminders retrieves reminder configurations for a patent.
// GET /api/v1/lifecycle/patents/{patentNumber}/reminders
func (lc *LifecycleClient) GetReminders(ctx context.Context, patentNumber string) ([]ReminderConfig, error) {
	if patentNumber == "" {
		return nil, invalidArg("patentNumber is required")
	}
	var resp APIResponse[[]ReminderConfig]
	if err := lc.client.get(ctx, "/api/v1/lifecycle/patents/"+url.PathEscape(patentNumber)+"/reminders", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// SetReminder creates or updates a reminder configuration.
// POST /api/v1/lifecycle/reminders
func (lc *LifecycleClient) SetReminder(ctx context.Context, req *ReminderConfigRequest) (*ReminderConfig, error) {
	if req == nil || req.PatentNumber == "" {
		return nil, invalidArg("patent_number is required")
	}
	if len(req.ReminderDays) == 0 {
		return nil, invalidArg("reminder_days is required")
	}
	if len(req.Channels) == 0 {
		return nil, invalidArg("channels is required")
	}
	var resp APIResponse[ReminderConfig]
	if err := lc.client.post(ctx, "/api/v1/lifecycle/reminders", req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// DeleteReminder deletes a reminder configuration.
// DELETE /api/v1/lifecycle/reminders/{reminderID}
func (lc *LifecycleClient) DeleteReminder(ctx context.Context, reminderID string) error {
	if reminderID == "" {
		return invalidArg("reminderID is required")
	}
	if err := lc.client.delete(ctx, "/api/v1/lifecycle/reminders/"+reminderID); err != nil {
		return err
	}
	return nil
}

//Personal.AI order the ending
