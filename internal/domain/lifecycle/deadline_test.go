package lifecycle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

func TestNewDeadline(t *testing.T) {
	due := time.Now().AddDate(0, 1, 0)
	d, err := NewDeadline("pat-1", "Office Action Response", due)
	assert.NoError(t, err)
	assert.NotNil(t, d)
	assert.Equal(t, "pat-1", d.PatentID)
	assert.Equal(t, "Office Action Response", d.Title)
	assert.Equal(t, DeadlineStatusActive, d.Status)
}

func TestDeadline_Urgency(t *testing.T) {
	now := time.Now().UTC()

	// Critical (<= 7 days)
	d1, _ := NewDeadline("p1", "t1", now.AddDate(0, 0, 5))
	assert.Equal(t, UrgencyCritical, d1.CheckUrgency())

	// High (<= 30 days)
	d2, _ := NewDeadline("p2", "t2", now.AddDate(0, 0, 20))
	assert.Equal(t, UrgencyHigh, d2.CheckUrgency())

	// Normal (> 30 days)
	d3, _ := NewDeadline("p3", "t3", now.AddDate(0, 2, 0))
	assert.Equal(t, UrgencyNormal, d3.CheckUrgency())

	// Overdue
	d4, _ := NewDeadline("p4", "t4", now.AddDate(0, 0, -1))
	assert.Equal(t, UrgencyOverdue, d4.CheckUrgency())
}

func TestDeadline_Complete(t *testing.T) {
	due := time.Now().AddDate(0, 1, 0)
	d, _ := NewDeadline("pat-1", "Task", due)

	err := d.Complete("user-1")
	assert.NoError(t, err)
	assert.Equal(t, DeadlineStatusCompleted, d.Status)
	assert.NotNil(t, d.CompletedAt)
	assert.Equal(t, "user-1", *d.CompletedBy)

	// Already completed
	err = d.Complete("user-2")
	assert.Error(t, err)
	assert.True(t, errors.IsValidation(err))
}

func TestDeadline_Extend(t *testing.T) {
	due := time.Now().AddDate(0, 1, 0)
	d, _ := NewDeadline("pat-1", "Task", due)

	newDate := due.AddDate(0, 1, 0)
	err := d.Extend(newDate, "Need more time")
	assert.NoError(t, err)
	assert.Equal(t, DeadlineStatusExtended, d.Status)
	assert.Equal(t, newDate, d.DueDate)
	assert.Equal(t, 1, d.ExtensionCount)
	assert.Len(t, d.ExtensionHistory, 1)

	// Invalid extension (earlier date)
	err = d.Extend(due, "Wait no") // earlier than newDate
	assert.Error(t, err)
}

func TestDeadline_DaysUntilDue(t *testing.T) {
	due := time.Now().Add(24 * time.Hour * 10) // 10 days
	d, _ := NewDeadline("pat-1", "Task", due)

	// Check roughly
	days := d.DaysUntilDue()
	assert.GreaterOrEqual(t, days, 9)
	assert.LessOrEqual(t, days, 10)
}

//Personal.AI order the ending
