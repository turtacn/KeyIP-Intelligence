package lifecycle

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	domainLifecycle "github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	domainPatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domainPortfolio "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Calendar DTOs
// ---------------------------------------------------------------------------

// CalendarEventType classifies lifecycle calendar events.
type CalendarEventType string

const (
	EventTypeAnnuityDue      CalendarEventType = "annuity_due"
	EventTypeExamDeadline    CalendarEventType = "examination_deadline"
	EventTypeResponseDue     CalendarEventType = "response_due"
	EventTypeRenewalWindow   CalendarEventType = "renewal_window"
	EventTypeGracePeriodEnd  CalendarEventType = "grace_period_end"
	EventTypeCustomMilestone CalendarEventType = "custom_milestone"
	EventTypePCTDeadline     CalendarEventType = "pct_deadline"
	EventTypeParisConvention CalendarEventType = "paris_convention"
)

// CalendarEvent represents a single event on the patent lifecycle calendar.
type CalendarEvent struct {
	ID           string                      `json:"id"`
	PatentID     string                      `json:"patent_id"`
	PatentNumber string                      `json:"patent_number"`
	Title        string                      `json:"title"`
	Description  string                      `json:"description"`
	EventType    CalendarEventType           `json:"event_type"`
	Jurisdiction domainLifecycle.Jurisdiction `json:"jurisdiction"`
	EventDate    time.Time                   `json:"event_date"`
	DueDate      time.Time                   `json:"due_date"`
	Timezone     string                      `json:"timezone"`
	Priority     EventPriority               `json:"priority"`
	Status       EventStatus                 `json:"status"`
	Reminders    []ReminderConfig            `json:"reminders,omitempty"`
	Metadata     map[string]string           `json:"metadata,omitempty"`
	CreatedAt    time.Time                   `json:"created_at"`
	UpdatedAt    time.Time                   `json:"updated_at"`
}

// EventPriority indicates urgency.
type EventPriority string

const (
	PriorityCritical EventPriority = "critical"
	PriorityHigh     EventPriority = "high"
	PriorityMedium   EventPriority = "medium"
	PriorityLow      EventPriority = "low"
)

// EventStatus tracks completion state.
type EventStatus string

const (
	EventStatusUpcoming  EventStatus = "upcoming"
	EventStatusDueSoon   EventStatus = "due_soon"
	EventStatusOverdue   EventStatus = "overdue"
	EventStatusCompleted EventStatus = "completed"
	EventStatusCancelled EventStatus = "cancelled"
)

// ReminderConfig defines when a reminder fires relative to the due date.
type ReminderConfig struct {
	DaysBefore int    `json:"days_before"`
	Channel    string `json:"channel"` // email, sms, webhook, in_app
	Enabled    bool   `json:"enabled"`
}

// CalendarViewRequest defines the query for a calendar view.
type CalendarViewRequest struct {
	PortfolioID   string                        `json:"portfolio_id,omitempty"`
	PatentIDs     []string                      `json:"patent_ids,omitempty"`
	Jurisdictions []domainLifecycle.Jurisdiction `json:"jurisdictions,omitempty"`
	EventTypes    []CalendarEventType           `json:"event_types,omitempty"`
	StartDate     time.Time                     `json:"start_date" validate:"required"`
	EndDate       time.Time                     `json:"end_date" validate:"required"`
	Timezone      string                        `json:"timezone,omitempty"`
	IncludeCompleted bool                       `json:"include_completed,omitempty"`
}

// CalendarView is the response for a calendar query.
type CalendarView struct {
	Events       []CalendarEvent          `json:"events"`
	TotalCount   int                      `json:"total_count"`
	Period       DateRange                `json:"period"`
	ByMonth      map[string]int           `json:"by_month"`
	ByType       map[CalendarEventType]int `json:"by_type"`
	ByPriority   map[EventPriority]int    `json:"by_priority"`
}

// AddEventRequest creates a custom milestone event.
type AddEventRequest struct {
	PatentID    string            `json:"patent_id" validate:"required"`
	Title       string            `json:"title" validate:"required"`
	Description string            `json:"description,omitempty"`
	EventType   CalendarEventType `json:"event_type,omitempty"`
	DueDate     time.Time         `json:"due_date" validate:"required"`
	Timezone    string            `json:"timezone,omitempty"`
	Priority    EventPriority     `json:"priority,omitempty"`
	Reminders   []ReminderConfig  `json:"reminders,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ICalExportRequest defines scope for iCal export.
type ICalExportRequest struct {
	PortfolioID string    `json:"portfolio_id,omitempty"`
	PatentIDs   []string  `json:"patent_ids,omitempty"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
}

// ---------------------------------------------------------------------------
// Service interface
// ---------------------------------------------------------------------------

// CalendarService defines the application-level contract for lifecycle calendar.
type CalendarService interface {
	// GetCalendarView returns events within a date range.
	GetCalendarView(ctx context.Context, req *CalendarViewRequest) (*CalendarView, error)

	// AddEvent creates a custom milestone event.
	AddEvent(ctx context.Context, req *AddEventRequest) (*CalendarEvent, error)

	// UpdateEventStatus marks an event as completed or cancelled.
	UpdateEventStatus(ctx context.Context, eventID string, status EventStatus) error

	// DeleteEvent removes a custom event.
	DeleteEvent(ctx context.Context, eventID string) error

	// ExportICal generates iCalendar format data.
	ExportICal(ctx context.Context, req *ICalExportRequest) ([]byte, error)

	// GetUpcomingDeadlines returns events due within N days.
	GetUpcomingDeadlines(ctx context.Context, portfolioID string, withinDays int) ([]CalendarEvent, error)
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

type calendarServiceImpl struct {
	lifecycleSvc  domainLifecycle.Service
	lifecycleRepo domainLifecycle.LifecycleRepository
	patentRepo    domainPatent.PatentRepository
	portfolioRepo domainPortfolio.PortfolioRepository
	cache         CachePort
	logger        Logger
	defaultTZ     string
}

// CalendarServiceConfig holds tunables.
type CalendarServiceConfig struct {
	DefaultTimezone string `yaml:"default_timezone"`
}

// NewCalendarService constructs a CalendarService.
func NewCalendarService(
	lifecycleSvc domainLifecycle.Service,
	lifecycleRepo domainLifecycle.LifecycleRepository,
	patentRepo domainPatent.PatentRepository,
	portfolioRepo domainPortfolio.PortfolioRepository,
	cache CachePort,
	logger Logger,
	cfg CalendarServiceConfig,
) CalendarService {
	tz := cfg.DefaultTimezone
	if tz == "" {
		tz = "Asia/Shanghai"
	}
	return &calendarServiceImpl{
		lifecycleSvc:  lifecycleSvc,
		lifecycleRepo: lifecycleRepo,
		patentRepo:    patentRepo,
		portfolioRepo: portfolioRepo,
		cache:         cache,
		logger:        logger,
		defaultTZ:     tz,
	}
}

// GetCalendarView returns events within a date range.
func (s *calendarServiceImpl) GetCalendarView(ctx context.Context, req *CalendarViewRequest) (*CalendarView, error) {
	if req == nil {
		return nil, errors.NewValidationOp("calendar.view", "request must not be nil")
	}
	if req.StartDate.IsZero() || req.EndDate.IsZero() {
		return nil, errors.NewValidationOp("calendar.view", "start_date and end_date are required")
	}
	if req.EndDate.Before(req.StartDate) {
		return nil, errors.NewValidationOp("calendar.view", "end_date must be after start_date")
	}

	tz := req.Timezone
	if tz == "" {
		tz = s.defaultTZ
	}

	patentIDs, err := s.resolvePatentIDs(ctx, req.PortfolioID, req.PatentIDs)
	if err != nil {
		return nil, err
	}

	var allEvents []CalendarEvent

	for _, pid := range patentIDs {
		// Verify valid UUID if needed, or just assume string ID.
		// Since patent.Patent.ID is string, but repo usually expects valid ID.
		// However, in our system, IDs are UUID strings.

		patent, fetchErr := s.patentRepo.FindByID(ctx, pid)
		if fetchErr != nil {
			s.logger.Warn("calendar: skipping patent", "patent_id", pid, "error", fetchErr)
			continue
		}

		jurisdiction := domainLifecycle.Jurisdiction(patent.Office)
		if !s.jurisdictionMatch(jurisdiction, req.Jurisdictions) {
			continue
		}

		events, genErr := s.generateEventsForPatent(ctx, patent, jurisdiction, req.StartDate, req.EndDate, tz)
		if genErr != nil {
			s.logger.Warn("calendar: event generation failed", "patent_id", pid, "error", genErr)
			continue
		}

		for _, ev := range events {
			if !s.eventTypeMatch(ev.EventType, req.EventTypes) {
				continue
			}
			if !req.IncludeCompleted && ev.Status == EventStatusCompleted {
				continue
			}
			allEvents = append(allEvents, ev)
		}
	}

	// Load custom events from repository
	customEvents, custErr := s.lifecycleRepo.GetCustomEvents(ctx, patentIDs, req.StartDate, req.EndDate)
	if custErr == nil {
		for _, ce := range customEvents {
			ev := mapDomainCustomEvent(ce, tz)
			if s.eventTypeMatch(ev.EventType, req.EventTypes) {
				if req.IncludeCompleted || ev.Status != EventStatusCompleted {
					allEvents = append(allEvents, ev)
				}
			}
		}
	}

	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].DueDate.Before(allEvents[j].DueDate)
	})

	view := &CalendarView{
		Events:     allEvents,
		TotalCount: len(allEvents),
		Period:     DateRange{Start: req.StartDate, End: req.EndDate},
		ByMonth:    make(map[string]int),
		ByType:     make(map[CalendarEventType]int),
		ByPriority: make(map[EventPriority]int),
	}
	for _, ev := range allEvents {
		monthKey := ev.DueDate.Format("2006-01")
		view.ByMonth[monthKey]++
		view.ByType[ev.EventType]++
		view.ByPriority[ev.Priority]++
	}

	s.logger.Info("calendar view generated",
		"events", len(allEvents),
		"start", req.StartDate.Format("2006-01-02"),
		"end", req.EndDate.Format("2006-01-02"),
	)

	return view, nil
}

// AddEvent creates a custom milestone event.
func (s *calendarServiceImpl) AddEvent(ctx context.Context, req *AddEventRequest) (*CalendarEvent, error) {
	if req == nil {
		return nil, errors.NewValidationOp("calendar.add", "request must not be nil")
	}
	if req.PatentID == "" {
		return nil, errors.NewValidationOp("calendar.add", "patent_id is required")
	}
	if req.Title == "" {
		return nil, errors.NewValidationOp("calendar.add", "title is required")
	}
	if req.DueDate.IsZero() {
		return nil, errors.NewValidationOp("calendar.add", "due_date is required")
	}

	if _, err := uuid.Parse(req.PatentID); err != nil {
		return nil, errors.NewValidationOp("calendar.add", fmt.Sprintf("invalid patent_id: %s", req.PatentID))
	}

	patent, err := s.patentRepo.FindByID(ctx, req.PatentID)
	if err != nil {
		return nil, errors.NewNotFoundOp("calendar.add", fmt.Sprintf("patent %s not found", req.PatentID))
	}

	eventType := req.EventType
	if eventType == "" {
		eventType = EventTypeCustomMilestone
	}
	priority := req.Priority
	if priority == "" {
		priority = PriorityMedium
	}
	tz := req.Timezone
	if tz == "" {
		tz = s.defaultTZ
	}

	now := time.Now()
	event := &CalendarEvent{
		ID:           fmt.Sprintf("evt-%d", now.UnixNano()),
		PatentID:     req.PatentID,
		PatentNumber: patent.PatentNumber,
		Title:        req.Title,
		Description:  req.Description,
		EventType:    eventType,
		Jurisdiction: domainLifecycle.Jurisdiction(patent.Office),
		EventDate:    req.DueDate,
		DueDate:      req.DueDate,
		Timezone:     tz,
		Priority:     priority,
		Status:       resolveEventStatus(req.DueDate, now),
		Reminders:    req.Reminders,
		Metadata:     req.Metadata,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if event.Reminders == nil {
		event.Reminders = defaultReminders()
	}

	domainEvent := toDomainCustomEvent(event)
	if saveErr := s.lifecycleRepo.SaveCustomEvent(ctx, domainEvent); saveErr != nil {
		s.logger.Error("failed to save custom event", "error", saveErr)
		return nil, errors.NewInternalOp("calendar.add", fmt.Sprintf("save failed: %v", saveErr))
	}

	s.logger.Info("custom event created",
		"event_id", event.ID,
		"patent_id", req.PatentID,
		"due_date", req.DueDate.Format("2006-01-02"),
	)

	return event, nil
}

// UpdateEventStatus marks an event as completed or cancelled.
func (s *calendarServiceImpl) UpdateEventStatus(ctx context.Context, eventID string, status EventStatus) error {
	if eventID == "" {
		return errors.NewValidationOp("calendar.update_status", "event_id is required")
	}
	if !isValidEventStatus(status) {
		return errors.NewValidationOp("calendar.update_status", fmt.Sprintf("invalid status: %s", status))
	}

	if err := s.lifecycleRepo.UpdateEventStatus(ctx, eventID, string(status)); err != nil {
		s.logger.Error("failed to update event status", "event_id", eventID, "error", err)
		return errors.NewInternalOp("calendar.update_status", fmt.Sprintf("update failed: %v", err))
	}

	s.logger.Info("event status updated", "event_id", eventID, "status", status)
	return nil
}

// DeleteEvent removes a custom event.
func (s *calendarServiceImpl) DeleteEvent(ctx context.Context, eventID string) error {
	if eventID == "" {
		return errors.NewValidationOp("calendar.delete", "event_id is required")
	}

	if err := s.lifecycleRepo.DeleteEvent(ctx, eventID); err != nil {
		s.logger.Error("failed to delete event", "event_id", eventID, "error", err)
		return errors.NewInternalOp("calendar.delete", fmt.Sprintf("delete failed: %v", err))
	}

	s.logger.Info("event deleted", "event_id", eventID)
	return nil
}

// ExportICal generates iCalendar format data.
func (s *calendarServiceImpl) ExportICal(ctx context.Context, req *ICalExportRequest) ([]byte, error) {
	if req == nil {
		return nil, errors.NewValidationOp("calendar.ical", "request must not be nil")
	}

	startDate := req.StartDate
	endDate := req.EndDate
	if startDate.IsZero() {
		startDate = time.Now()
	}
	if endDate.IsZero() {
		endDate = startDate.AddDate(1, 0, 0)
	}

	viewReq := &CalendarViewRequest{
		PortfolioID:      req.PortfolioID,
		PatentIDs:        req.PatentIDs,
		StartDate:        startDate,
		EndDate:          endDate,
		IncludeCompleted: false,
	}
	view, err := s.GetCalendarView(ctx, viewReq)
	if err != nil {
		return nil, err
	}

	ical := buildICalData(view.Events)
	return ical, nil
}

// GetUpcomingDeadlines returns events due within N days.
func (s *calendarServiceImpl) GetUpcomingDeadlines(ctx context.Context, portfolioID string, withinDays int) ([]CalendarEvent, error) {
	if portfolioID == "" {
		return nil, errors.NewValidationOp("calendar.upcoming", "portfolio_id is required")
	}
	if withinDays <= 0 {
		withinDays = 30
	}

	now := time.Now()
	endDate := now.AddDate(0, 0, withinDays)

	viewReq := &CalendarViewRequest{
		PortfolioID:      portfolioID,
		StartDate:        now,
		EndDate:          endDate,
		IncludeCompleted: false,
	}
	view, err := s.GetCalendarView(ctx, viewReq)
	if err != nil {
		return nil, err
	}

	// Filter to only upcoming/due_soon/overdue
	var deadlines []CalendarEvent
	for _, ev := range view.Events {
		switch ev.Status {
		case EventStatusUpcoming, EventStatusDueSoon, EventStatusOverdue:
			deadlines = append(deadlines, ev)
		}
	}

	return deadlines, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (s *calendarServiceImpl) resolvePatentIDs(ctx context.Context, portfolioID string, explicitIDs []string) ([]string, error) {
	if len(explicitIDs) > 0 {
		return explicitIDs, nil
	}
	if portfolioID == "" {
		return nil, errors.NewValidationOp("calendar", "either portfolio_id or patent_ids is required")
	}

	portUUID, err := uuid.Parse(portfolioID)
	if err != nil {
		return nil, errors.NewValidationOp("calendar", fmt.Sprintf("invalid portfolio_id: %v", err))
	}

	patents, _, err := s.portfolioRepo.GetPatents(ctx, portUUID, nil, 10000, 0)
	if err != nil {
		return nil, errors.NewInternalOp("calendar", fmt.Sprintf("failed to list portfolio patents: %v", err))
	}

	ids := make([]string, 0, len(patents))
	for _, p := range patents {
		ids = append(ids, p.ID)
	}
	return ids, nil
}

func (s *calendarServiceImpl) jurisdictionMatch(j domainLifecycle.Jurisdiction, filter []domainLifecycle.Jurisdiction) bool {
	if len(filter) == 0 {
		return true
	}
	for _, f := range filter {
		if f == j {
			return true
		}
	}
	return false
}

func (s *calendarServiceImpl) eventTypeMatch(t CalendarEventType, filter []CalendarEventType) bool {
	if len(filter) == 0 {
		return true
	}
	for _, f := range filter {
		if f == t {
			return true
		}
	}
	return false
}

func (s *calendarServiceImpl) generateEventsForPatent(
	ctx context.Context,
	patent *domainPatent.Patent,
	jurisdiction domainLifecycle.Jurisdiction,
	start, end time.Time,
	tz string,
) ([]CalendarEvent, error) {
	var events []CalendarEvent
	now := time.Now()

	// Ensure filing date is available
	if patent.Dates.FilingDate == nil {
		return nil, errors.NewValidationOp("calendar", fmt.Sprintf("patent %s missing filing date", patent.PatentNumber))
	}
	filingDate := *patent.Dates.FilingDate

	// Generate annuity due events
	maxYears := jurisdictionMaxLife(jurisdiction)
	filingYear := filingDate.Year()

	for year := 1; year <= maxYears; year++ {
		dueDate := annuityDueDate(filingDate, year, jurisdiction)
		if dueDate.Before(start) || dueDate.After(end) {
			continue
		}

		gracePeriodEnd := dueDate.AddDate(0, 6, 0)
		priority := classifyDeadlinePriority(dueDate, now)

		events = append(events, CalendarEvent{
			ID:           fmt.Sprintf("ann-%s-%d", patent.ID, year),
			PatentID:     patent.ID,
			PatentNumber: patent.PatentNumber,
			Title:        fmt.Sprintf("Year %d Annuity Due - %s", year, patent.PatentNumber),
			Description:  fmt.Sprintf("Annual maintenance fee year %d for patent %s in %s", year, patent.PatentNumber, jurisdiction),
			EventType:    EventTypeAnnuityDue,
			Jurisdiction: jurisdiction,
			EventDate:    dueDate,
			DueDate:      dueDate,
			Timezone:     tz,
			Priority:     priority,
			Status:       resolveEventStatus(dueDate, now),
			Reminders:    defaultReminders(),
			Metadata: map[string]string{
				"year_number":      fmt.Sprintf("%d", year),
				"filing_year":      fmt.Sprintf("%d", filingYear),
				"grace_period_end": gracePeriodEnd.Format("2006-01-02"),
			},
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	// Generate PCT deadline if applicable
	if jurisdiction == domainLifecycle.JurisdictionCN {
		pctDeadline := filingDate.AddDate(0, 30, 0)
		if !pctDeadline.Before(start) && !pctDeadline.After(end) {
			events = append(events, CalendarEvent{
				ID:           fmt.Sprintf("pct-%s", patent.ID),
				PatentID:     patent.ID,
				PatentNumber: patent.PatentNumber,
				Title:        fmt.Sprintf("PCT National Phase Deadline - %s", patent.PatentNumber),
				Description:  "30-month deadline for PCT national phase entry",
				EventType:    EventTypePCTDeadline,
				Jurisdiction: jurisdiction,
				EventDate:    pctDeadline,
				DueDate:      pctDeadline,
				Timezone:     tz,
				Priority:     classifyDeadlinePriority(pctDeadline, now),
				Status:       resolveEventStatus(pctDeadline, now),
				Reminders:    defaultReminders(),
				CreatedAt:    now,
				UpdatedAt:    now,
			})
		}
	}

	// Paris Convention priority deadline (12 months from filing)
	parisDeadline := filingDate.AddDate(1, 0, 0)
	if !parisDeadline.Before(start) && !parisDeadline.After(end) {
		events = append(events, CalendarEvent{
			ID:           fmt.Sprintf("paris-%s", patent.ID),
			PatentID:     patent.ID,
			PatentNumber: patent.PatentNumber,
			Title:        fmt.Sprintf("Paris Convention Priority Deadline - %s", patent.PatentNumber),
			Description:  "12-month priority deadline under Paris Convention",
			EventType:    EventTypeParisConvention,
			Jurisdiction: jurisdiction,
			EventDate:    parisDeadline,
			DueDate:      parisDeadline,
			Timezone:     tz,
			Priority:     classifyDeadlinePriority(parisDeadline, now),
			Status:       resolveEventStatus(parisDeadline, now),
			Reminders:    defaultReminders(),
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	return events, nil
}

func resolveEventStatus(dueDate time.Time, now time.Time) EventStatus {
	daysUntil := int(dueDate.Sub(now).Hours() / 24)
	switch {
	case daysUntil < -30:
		return EventStatusOverdue
	case daysUntil < 0:
		return EventStatusOverdue
	case daysUntil <= 14:
		return EventStatusDueSoon
	default:
		return EventStatusUpcoming
	}
}

func classifyDeadlinePriority(dueDate time.Time, now time.Time) EventPriority {
	daysUntil := int(dueDate.Sub(now).Hours() / 24)
	switch {
	case daysUntil < 0:
		return PriorityCritical
	case daysUntil <= 7:
		return PriorityCritical
	case daysUntil <= 30:
		return PriorityHigh
	case daysUntil <= 90:
		return PriorityMedium
	default:
		return PriorityLow
	}
}

func defaultReminders() []ReminderConfig {
	return []ReminderConfig{
		{DaysBefore: 90, Channel: "email", Enabled: true},
		{DaysBefore: 30, Channel: "email", Enabled: true},
		{DaysBefore: 14, Channel: "in_app", Enabled: true},
		{DaysBefore: 7, Channel: "email", Enabled: true},
		{DaysBefore: 1, Channel: "email", Enabled: true},
	}
}

func isValidEventStatus(s EventStatus) bool {
	switch s {
	case EventStatusUpcoming, EventStatusDueSoon, EventStatusOverdue,
		EventStatusCompleted, EventStatusCancelled:
		return true
	}
	return false
}

func annuityDueDate(filingDate time.Time, year int, jurisdiction domainLifecycle.Jurisdiction) time.Time {
	// Most jurisdictions: annuity due on filing anniversary
	// CN: due from year 3 onward, on filing anniversary
	// US: due at 3.5, 7.5, 11.5 years (simplified to anniversary)
	return filingDate.AddDate(year, 0, 0)
}

func jurisdictionMaxLife(j domainLifecycle.Jurisdiction) int {
	switch j {
	case domainLifecycle.JurisdictionCN:
		return 20
	case domainLifecycle.JurisdictionUS:
		return 20
	case domainLifecycle.JurisdictionEP:
		return 20
	case domainLifecycle.JurisdictionJP:
		return 20
	case domainLifecycle.JurisdictionKR:
		return 20
	default:
		return 20
	}
}

func mapDomainCustomEvent(de domainLifecycle.CustomEvent, tz string) CalendarEvent {
	return CalendarEvent{
		ID:           de.ID,
		PatentID:     de.PatentID,
		PatentNumber: de.PatentNumber,
		Title:        de.Title,
		Description:  de.Description,
		EventType:    CalendarEventType(de.EventType),
		Jurisdiction: de.Jurisdiction,
		EventDate:    de.EventDate,
		DueDate:      de.DueDate,
		Timezone:     tz,
		Priority:     EventPriority(de.Priority),
		Status:       EventStatus(de.Status),
		Metadata:     de.Metadata,
		CreatedAt:    de.CreatedAt,
		UpdatedAt:    de.UpdatedAt,
	}
}

func toDomainCustomEvent(ev *CalendarEvent) *domainLifecycle.CustomEvent {
	return &domainLifecycle.CustomEvent{
		ID:           ev.ID,
		PatentID:     ev.PatentID,
		PatentNumber: ev.PatentNumber,
		Title:        ev.Title,
		Description:  ev.Description,
		EventType:    string(ev.EventType),
		Jurisdiction: ev.Jurisdiction,
		EventDate:    ev.EventDate,
		DueDate:      ev.DueDate,
		Priority:     string(ev.Priority),
		Status:       string(ev.Status),
		Metadata:     ev.Metadata,
		CreatedAt:    ev.CreatedAt,
		UpdatedAt:    ev.UpdatedAt,
	}
}

func buildICalData(events []CalendarEvent) []byte {
	var buf []byte
	buf = append(buf, "BEGIN:VCALENDAR\r\n"...)
	buf = append(buf, "VERSION:2.0\r\n"...)
	buf = append(buf, "PRODID:-//KeyIP-Intelligence//Patent Lifecycle Calendar//EN\r\n"...)
	buf = append(buf, "CALSCALE:GREGORIAN\r\n"...)
	buf = append(buf, "METHOD:PUBLISH\r\n"...)

	for _, ev := range events {
		buf = append(buf, "BEGIN:VEVENT\r\n"...)
		buf = append(buf, fmt.Sprintf("UID:%s@keyip-intelligence\r\n", ev.ID)...)
		buf = append(buf, fmt.Sprintf("DTSTART:%s\r\n", ev.DueDate.UTC().Format("20060102T150405Z"))...)
		buf = append(buf, fmt.Sprintf("DTEND:%s\r\n", ev.DueDate.Add(time.Hour).UTC().Format("20060102T150405Z"))...)
		buf = append(buf, fmt.Sprintf("SUMMARY:%s\r\n", ev.Title)...)
		if ev.Description != "" {
			buf = append(buf, fmt.Sprintf("DESCRIPTION:%s\r\n", ev.Description)...)
		}
		buf = append(buf, fmt.Sprintf("CATEGORIES:%s\r\n", ev.EventType)...)

		// Add VALARM for each reminder
		for _, r := range ev.Reminders {
			if r.Enabled {
				buf = append(buf, "BEGIN:VALARM\r\n"...)
				buf = append(buf, "ACTION:DISPLAY\r\n"...)
				buf = append(buf, fmt.Sprintf("TRIGGER:-P%dD\r\n", r.DaysBefore)...)
				buf = append(buf, fmt.Sprintf("DESCRIPTION:Reminder: %s\r\n", ev.Title)...)
				buf = append(buf, "END:VALARM\r\n"...)
			}
		}

		buf = append(buf, "END:VEVENT\r\n"...)
	}

	buf = append(buf, "END:VCALENDAR\r\n"...)
	return buf
}

//Personal.AI order the ending
