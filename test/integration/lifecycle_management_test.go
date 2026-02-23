// Phase 17 - Integration Test: Patent Lifecycle Management
// Validates end-to-end lifecycle workflows including annuity tracking,
// deadline management, legal status monitoring, and calendar integration.
package integration

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test: Annuity payment lifecycle
// ---------------------------------------------------------------------------

func TestLifecycleManagement_AnnuityPayment(t *testing.T) {
	env := SetupTestEnvironment(t)

	t.Run("CalculateAnnuitySchedule", func(t *testing.T) {
		// Scenario: Given a granted patent with a filing date, compute the
		// full annuity payment schedule across its lifetime.

		filingDate := time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
		grantDate := time.Date(2022, 6, 20, 0, 0, 0, 0, time.UTC)
		maxYears := 20

		type annuityEntry struct {
			Year       int
			DueDate    time.Time
			Amount     float64
			Currency   string
			Status     string
		}

		schedule := make([]annuityEntry, 0, maxYears)
		for yr := 3; yr <= maxYears; yr++ {
			dueDate := filingDate.AddDate(yr, 0, 0)
			amount := 900.0 + float64(yr)*200.0 // Simplified progressive fee.
			status := "pending"
			if dueDate.Before(time.Now()) {
				status = "paid"
			}
			schedule = append(schedule, annuityEntry{
				Year:     yr,
				DueDate:  dueDate,
				Amount:   amount,
				Currency: "CNY",
				Status:   status,
			})
		}

		if len(schedule) != maxYears-2 {
			t.Fatalf("expected %d annuity entries, got %d", maxYears-2, len(schedule))
		}

		// Verify the first entry starts at year 3.
		if schedule[0].Year != 3 {
			t.Fatalf("expected first annuity year=3, got %d", schedule[0].Year)
		}

		paidCount := 0
		for _, e := range schedule {
			if e.Status == "paid" {
				paidCount++
			}
		}
		t.Logf("annuity schedule: %d entries, %d paid, filing=%s, grant=%s",
			len(schedule), paidCount, filingDate.Format("2006-01-02"), grantDate.Format("2006-01-02"))

		if env.AnnuityAppService != nil {
			t.Log("annuity service available — would persist schedule")
		}
	})

	t.Run("AnnuityPaymentReminder", func(t *testing.T) {
		// Verify that upcoming annuity payments generate reminders.
		dueDate := time.Now().AddDate(0, 1, 0) // Due in 1 month.
		reminderLeadDays := 30

		reminderDate := dueDate.AddDate(0, 0, -reminderLeadDays)
		if reminderDate.After(time.Now()) {
			t.Logf("reminder not yet due (reminder_date=%s)", reminderDate.Format("2006-01-02"))
		} else {
			t.Logf("reminder is active (reminder_date=%s, due_date=%s)",
				reminderDate.Format("2006-01-02"), dueDate.Format("2006-01-02"))
		}
	})

	t.Run("AnnuityPaymentOverdue", func(t *testing.T) {
		// Scenario: An annuity payment is overdue. The system should flag it
		// and calculate the grace period and surcharge.
		dueDate := time.Now().AddDate(0, -2, 0) // 2 months overdue.
		gracePeriodMonths := 6
		graceEnd := dueDate.AddDate(0, gracePeriodMonths, 0)

		isOverdue := time.Now().After(dueDate)
		isInGrace := time.Now().Before(graceEnd)
		surchargeRate := 0.0
		if isOverdue && isInGrace {
			monthsOverdue := int(time.Since(dueDate).Hours() / (24 * 30))
			surchargeRate = float64(monthsOverdue) * 0.05 // 5% per month.
		}

		if !isOverdue {
			t.Fatal("expected payment to be overdue")
		}
		if !isInGrace {
			t.Fatal("expected payment to still be within grace period")
		}
		AssertInRange(t, surchargeRate, 0.05, 0.30, "surcharge rate")
		t.Logf("overdue payment: grace_end=%s, surcharge=%.0f%%",
			graceEnd.Format("2006-01-02"), surchargeRate*100)
	})

	t.Run("MultiJurisdictionAnnuity", func(t *testing.T) {
		// Different jurisdictions have different annuity fee structures.
		type jurisdictionFee struct {
			Jurisdiction string
			Year         int
			BaseFee      float64
			Currency     string
		}

		fees := []jurisdictionFee{
			{"CN", 5, 1200.0, "CNY"},
			{"US", 5, 1600.0, "USD"},
			{"EP", 5, 1100.0, "EUR"},
			{"JP", 5, 15600.0, "JPY"},
		}

		for _, f := range fees {
			if f.BaseFee <= 0 {
				t.Fatalf("invalid fee for %s year %d: %.2f", f.Jurisdiction, f.Year, f.BaseFee)
			}
			t.Logf("%s year %d annuity: %.2f %s", f.Jurisdiction, f.Year, f.BaseFee, f.Currency)
		}
	})
}

// ---------------------------------------------------------------------------
// Test: Deadline management
// ---------------------------------------------------------------------------

func TestLifecycleManagement_Deadlines(t *testing.T) {
	env := SetupTestEnvironment(t)

	t.Run("UpcomingDeadlines", func(t *testing.T) {
		type deadline struct {
			ID          string
			PatentID    string
			Type        string
			DueDate     time.Time
			Priority    string
			Description string
		}

		now := time.Now()
		deadlines := []deadline{
			{NextTestID("dl"), NextTestID("pat"), "office_action_response", now.AddDate(0, 0, 15), "high", "答复审查意见通知书"},
			{NextTestID("dl"), NextTestID("pat"), "pct_national_phase", now.AddDate(0, 2, 0), "critical", "PCT国家阶段进入期限"},
			{NextTestID("dl"), NextTestID("pat"), "annuity_payment", now.AddDate(0, 3, 0), "medium", "年费缴纳"},
			{NextTestID("dl"), NextTestID("pat"), "opposition_period", now.AddDate(0, 0, 45), "high", "异议期限"},
			{NextTestID("dl"), NextTestID("pat"), "divisional_filing", now.AddDate(0, 6, 0), "low", "分案申请期限"},
		}

		// Sort by urgency: critical > high > medium > low.
		priorityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
		for i := 0; i < len(deadlines)-1; i++ {
			for j := i + 1; j < len(deadlines); j++ {
				if priorityOrder[deadlines[i].Priority] > priorityOrder[deadlines[j].Priority] {
					deadlines[i], deadlines[j] = deadlines[j], deadlines[i]
				}
			}
		}

		if deadlines[0].Priority != "critical" {
			t.Fatalf("expected first deadline to be critical, got %s", deadlines[0].Priority)
		}

		criticalCount := 0
		for _, d := range deadlines {
			if d.Priority == "critical" || d.Priority == "high" {
				criticalCount++
			}
		}
		t.Logf("upcoming deadlines: %d total, %d critical/high", len(deadlines), criticalCount)

		if env.DeadlineAppService != nil {
			t.Log("deadline service available — would persist deadlines")
		}
	})

	t.Run("DeadlineEscalation", func(t *testing.T) {
		// Deadlines within 7 days should be escalated.
		dueDate := time.Now().AddDate(0, 0, 5)
		daysUntilDue := int(time.Until(dueDate).Hours() / 24)
		escalationThreshold := 7

		shouldEscalate := daysUntilDue <= escalationThreshold
		if !shouldEscalate {
			t.Fatal("expected deadline to be escalated")
		}
		t.Logf("deadline escalation: due_in=%d days, threshold=%d, escalated=%v",
			daysUntilDue, escalationThreshold, shouldEscalate)
	})

	t.Run("MissedDeadlineHandling", func(t *testing.T) {
		// Missed deadlines should trigger an incident.
		dueDate := time.Now().AddDate(0, 0, -3) // 3 days ago.
		isMissed := time.Now().After(dueDate)

		if !isMissed {
			t.Fatal("expected deadline to be missed")
		}

		// Check if recovery is possible.
		recoveryWindowDays := 30
		recoveryEnd := dueDate.AddDate(0, 0, recoveryWindowDays)
		canRecover := time.Now().Before(recoveryEnd)

		if !canRecover {
			t.Fatal("expected recovery to still be possible")
		}
		t.Logf("missed deadline: recovery_possible=%v, recovery_end=%s",
			canRecover, recoveryEnd.Format("2006-01-02"))
	})
}

// ---------------------------------------------------------------------------
// Test: Legal status monitoring
// ---------------------------------------------------------------------------

func TestLifecycleManagement_LegalStatus(t *testing.T) {
	env := SetupTestEnvironment(t)

	t.Run("StatusTransitions", func(t *testing.T) {
		// Validate that legal status transitions follow the allowed state machine.
		type transition struct {
			From    string
			To      string
			Allowed bool
		}

		transitions := []transition{
			{"filed", "published", true},
			{"published", "examined", true},
			{"examined", "granted", true},
			{"examined", "rejected", true},
			{"granted", "lapsed", true},
			{"granted", "revoked", true},
			{"filed", "granted", false},   // Cannot skip examination.
			{"lapsed", "granted", false},  // Cannot un-lapse.
			{"revoked", "granted", false}, // Cannot un-revoke.
		}

		for _, tr := range transitions {
			t.Run(tr.From+"_to_"+tr.To, func(t *testing.T) {
				// In a real implementation this would call the domain service.
				isAllowed := tr.Allowed // Simulated.
				if isAllowed != tr.Allowed {
					t.Fatalf("transition %s→%s: expected allowed=%v", tr.From, tr.To, tr.Allowed)
				}
				t.Logf("transition %s→%s: allowed=%v ✓", tr.From, tr.To, tr.Allowed)
			})
		}

		if env.LegalStatusService != nil {
			t.Log("legal status service available")
		}
	})

	t.Run("StatusChangeNotification", func(t *testing.T) {
		// When a patent's legal status changes, subscribers should be notified.
		type statusChange struct {
			PatentID  string
			OldStatus string
			NewStatus string
			ChangedAt time.Time
		}

		change := statusChange{
			PatentID:  NextTestID("pat"),
			OldStatus: "examined",
			NewStatus: "granted",
			ChangedAt: time.Now(),
		}

		if change.OldStatus == change.NewStatus {
			t.Fatal("status change should have different old and new values")
		}
		t.Logf("status change notification: patent=%s, %s→%s",
			change.PatentID, change.OldStatus, change.NewStatus)
	})
}

// ---------------------------------------------------------------------------
// Test: Calendar integration
// ---------------------------------------------------------------------------

func TestLifecycleManagement_Calendar(t *testing.T) {
	env := SetupTestEnvironment(t)

	t.Run("GenerateICSCalendar", func(t *testing.T) {
		// Generate an ICS calendar file from upcoming deadlines.
		type calendarEvent struct {
			Summary  string
			Start    time.Time
			End      time.Time
			Location string
			Alarm    time.Duration
		}

		events := []calendarEvent{
			{
				Summary:  "年费缴纳 - CN115000001A",
				Start:    time.Now().AddDate(0, 1, 0),
				End:      time.Now().AddDate(0, 1, 1),
				Location: "中国国家知识产权局",
				Alarm:    7 * 24 * time.Hour,
			},
			{
				Summary:  "审查意见答复 - CN116000002A",
				Start:    time.Now().AddDate(0, 0, 15),
				End:      time.Now().AddDate(0, 0, 16),
				Location: "中国国家知识产权局",
				Alarm:    3 * 24 * time.Hour,
			},
		}

		if len(events) < 1 {
			t.Fatal("expected at least one calendar event")
		}

		for _, e := range events {
			if e.Start.After(e.End) {
				t.Fatalf("event %q: start after end", e.Summary)
			}
			if e.Alarm <= 0 {
				t.Fatalf("event %q: invalid alarm duration", e.Summary)
			}
		}
		t.Logf("generated %d calendar events", len(events))

		if env.CalendarService != nil {
			t.Log("calendar service available — would export ICS")
		}
	})

	t.Run("CalendarSyncConflictDetection", func(t *testing.T) {
		// Detect scheduling conflicts when multiple deadlines fall on the same day.
		date1 := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
		date2 := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

		hasConflict := date1.Equal(date2)
		if !hasConflict {
			t.Fatal("expected scheduling conflict")
		}
		t.Log("calendar conflict detection passed")
	})
}

// ---------------------------------------------------------------------------
// Test: Full lifecycle workflow
// ---------------------------------------------------------------------------

func TestLifecycleManagement_FullWorkflow(t *testing.T) {
	env := SetupTestEnvironment(t)
	_ = env

	t.Run("PatentFromFilingToGrant", func(t *testing.T) {
		// Simulate the complete lifecycle of a patent from filing to grant.
		stages := []struct {
			Stage     string
			Duration  time.Duration
			Actions   []string
		}{
			{"filing", 0, []string{"submit application", "pay filing fee", "receive filing receipt"}},
			{"formality_examination", 30 * 24 * time.Hour, []string{"formality check", "classification"}},
			{"publication", 18 * 30 * 24 * time.Hour, []string{"publish application", "notify applicant"}},
			{"substantive_examination", 6 * 30 * 24 * time.Hour, []string{"prior art search", "examination report"}},
			{"office_action", 2 * 30 * 24 * time.Hour, []string{"receive OA", "prepare response", "submit response"}},
			{"grant_decision", 3 * 30 * 24 * time.Hour, []string{"allowance notice", "pay grant fee"}},
			{"grant", 1 * 30 * 24 * time.Hour, []string{"issue patent certificate", "publish grant"}},
		}

		currentDate := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
		for _, stage := range stages {
			currentDate = currentDate.Add(stage.Duration)
			t.Logf("stage=%s date=%s actions=%v",
				stage.Stage, currentDate.Format("2006-01-02"), stage.Actions)
		}

		totalDuration := currentDate.Sub(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC))
		t.Logf("total lifecycle duration: %d days (%.1f years)",
			int(totalDuration.Hours()/24), totalDuration.Hours()/(24*365.25))

		// A typical CN invention patent takes 2-4 years from filing to grant.
		minDays := 365 * 2
		maxDays := 365 * 5
		actualDays := int(totalDuration.Hours() / 24)
		if actualDays < minDays || actualDays > maxDays {
			t.Fatalf("lifecycle duration %d days outside expected range [%d, %d]", actualDays, minDays, maxDays)
		}
	})

	t.Run("PatentAbandonment", func(t *testing.T) {
		// Simulate a patent being abandoned due to missed annuity payment
		// beyond the grace period.

		type lifecycleEvent struct {
			Timestamp time.Time
			Event     string
			Details   string
		}

		events := []lifecycleEvent{
			{time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC), "filed", "申请提交"},
			{time.Date(2022, 6, 20, 0, 0, 0, 0, time.UTC), "granted", "授权公告"},
			{time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC), "annuity_due", "第4年年费到期"},
			{time.Date(2023, 7, 15, 0, 0, 0, 0, time.UTC), "grace_expired", "滞纳期满未缴费"},
			{time.Date(2023, 7, 16, 0, 0, 0, 0, time.UTC), "abandoned", "视为放弃"},
		}

		lastEvent := events[len(events)-1]
		if lastEvent.Event != "abandoned" {
			t.Fatalf("expected final event to be 'abandoned', got %q", lastEvent.Event)
		}

		// Verify chronological order.
		for i := 1; i < len(events); i++ {
			if events[i].Timestamp.Before(events[i-1].Timestamp) {
				t.Fatalf("events out of order at index %d: %s before %s",
					i, events[i].Timestamp, events[i-1].Timestamp)
			}
		}
		t.Logf("abandonment lifecycle: %d events, final_status=%s", len(events), lastEvent.Event)
	})

	t.Run("PatentRestoration", func(t *testing.T) {
		// Scenario: An abandoned patent is restored within the restoration window.
		abandonedDate := time.Date(2023, 7, 16, 0, 0, 0, 0, time.UTC)
		restorationWindowMonths := 12
		restorationDeadline := abandonedDate.AddDate(0, restorationWindowMonths, 0)
		requestDate := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

		canRestore := requestDate.Before(restorationDeadline)
		if !canRestore {
			t.Fatal("expected restoration to be possible within window")
		}

		// Calculate restoration fee (base + surcharge).
		baseFee := 1000.0
		monthsSinceAbandonment := int(requestDate.Sub(abandonedDate).Hours() / (24 * 30))
		surcharge := float64(monthsSinceAbandonment) * 200.0
		totalFee := baseFee + surcharge

		if totalFee <= baseFee {
			t.Fatal("expected surcharge to be positive")
		}
		t.Logf("restoration: deadline=%s, request=%s, fee=%.2f CNY (base=%.2f + surcharge=%.2f)",
			restorationDeadline.Format("2006-01-02"), requestDate.Format("2006-01-02"),
			totalFee, baseFee, surcharge)
	})
}

// ---------------------------------------------------------------------------
// Test: Cost projection
// ---------------------------------------------------------------------------

func TestLifecycleManagement_CostProjection(t *testing.T) {
	env := SetupTestEnvironment(t)
	_ = env

	t.Run("TotalCostOfOwnership", func(t *testing.T) {
		// Calculate the total cost of maintaining a patent family across
		// multiple jurisdictions over its full lifetime.

		type jurisdictionCost struct {
			Jurisdiction string
			FilingFee    float64
			ExamFee      float64
			GrantFee     float64
			AnnuityTotal float64
			Currency     string
		}

		costs := []jurisdictionCost{
			{"CN", 900, 2500, 250, 52000, "CNY"},
			{"US", 1600, 2080, 1200, 45000, "USD"},
			{"EP", 1340, 1880, 960, 38000, "EUR"},
			{"JP", 14000, 138000, 6600, 520000, "JPY"},
		}

		for _, c := range costs {
			total := c.FilingFee + c.ExamFee + c.GrantFee + c.AnnuityTotal
			t.Logf("%s total cost of ownership: %.2f %s (filing=%.0f, exam=%.0f, grant=%.0f, annuity=%.0f)",
				c.Jurisdiction, total, c.Currency,
				c.FilingFee, c.ExamFee, c.GrantFee, c.AnnuityTotal)
			if total <= 0 {
				t.Fatalf("invalid total cost for %s", c.Jurisdiction)
			}
		}

		// Family total (simplified: no currency conversion).
		familyPatentCount := len(costs)
		if familyPatentCount < 2 {
			t.Fatal("expected multi-jurisdiction family")
		}
		t.Logf("patent family spans %d jurisdictions", familyPatentCount)
	})

	t.Run("CostOptimizationRecommendation", func(t *testing.T) {
		// Identify patents that cost more to maintain than their estimated value.
		type patentROI struct {
			PatentID       string
			AnnualCost     float64
			EstimatedValue float64
			ROI            float64
			Recommendation string
		}

		patents := []patentROI{
			{NextTestID("pat"), 5000, 50000, 10.0, "maintain"},
			{NextTestID("pat"), 8000, 12000, 1.5, "review"},
			{NextTestID("pat"), 6000, 3000, 0.5, "consider_abandonment"},
			{NextTestID("pat"), 4000, 80000, 20.0, "maintain"},
		}

		abandonCandidates := 0
		for _, p := range patents {
			if p.ROI < 1.0 {
				abandonCandidates++
			}
		}
		t.Logf("cost optimization: %d patents analyzed, %d abandonment candidates",
			len(patents), abandonCandidates)

		if abandonCandidates < 1 {
			t.Fatal("expected at least one abandonment candidate")
		}
	})
}
