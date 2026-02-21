package lifecycle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewLifecycleRecord_Success(t *testing.T) {
	patentID := "pat1"
	jurisdiction := "CN"
	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	lr, err := NewLifecycleRecord(patentID, jurisdiction, filingDate)
	assert.NoError(t, err)
	assert.NotEmpty(t, lr.ID)
	assert.Equal(t, PhaseApplication, lr.CurrentPhase)
	assert.Equal(t, 20.0, lr.TotalLifeYears)
	assert.Equal(t, filingDate.AddDate(20, 0, 0), *lr.ExpirationDate)
	assert.Equal(t, 1, len(lr.Events))
	assert.Equal(t, "filed", lr.Events[0].EventType)
}

func TestNewLifecycleRecord_InvalidParams(t *testing.T) {
	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	_, err := NewLifecycleRecord("", "CN", filingDate)
	assert.Error(t, err)

	_, err = NewLifecycleRecord("pat1", "", filingDate)
	assert.Error(t, err)

	_, err = NewLifecycleRecord("pat1", "CN", time.Time{})
	assert.Error(t, err)
}

func TestLifecycleRecord_TransitionTo_Success(t *testing.T) {
	lr, _ := NewLifecycleRecord("pat1", "CN", time.Now())

	err := lr.TransitionTo(PhaseExamination, "Request exam", "user1")
	assert.NoError(t, err)
	assert.Equal(t, PhaseExamination, lr.CurrentPhase)
	assert.Equal(t, 1, len(lr.PhaseHistory))
	assert.Equal(t, PhaseApplication, lr.PhaseHistory[0].FromPhase)
	assert.Equal(t, PhaseExamination, lr.PhaseHistory[0].ToPhase)
}

func TestLifecycleRecord_TransitionTo_Invalid(t *testing.T) {
	lr, _ := NewLifecycleRecord("pat1", "CN", time.Now())

	// Skip Examination, go straight to Granted (invalid skip)
	err := lr.TransitionTo(PhaseGranted, "Grant", "user1")
	assert.Error(t, err)
}

func TestLifecycleRecord_MarkGranted_Success(t *testing.T) {
	lr, _ := NewLifecycleRecord("pat1", "CN", time.Now())
	_ = lr.TransitionTo(PhaseExamination, "Exam", "user1")

	grantDate := time.Now()
	err := lr.MarkGranted(grantDate)
	assert.NoError(t, err)
	assert.Equal(t, PhaseGranted, lr.CurrentPhase)
	assert.Equal(t, &grantDate, lr.GrantDate)
}

func TestLifecycleRecord_MarkAbandoned_Success(t *testing.T) {
	lr, _ := NewLifecycleRecord("pat1", "CN", time.Now())

	err := lr.MarkAbandoned("Cost cutting")
	assert.NoError(t, err)
	assert.Equal(t, PhaseAbandoned, lr.CurrentPhase)
	assert.NotNil(t, lr.AbandonmentDate)
}

func TestLifecycleRecord_CalculateRemainingLife(t *testing.T) {
	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lr, _ := NewLifecycleRecord("pat1", "CN", filingDate)

	// Expiration is 2040-01-01
	asOf := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	remaining := lr.CalculateRemainingLife(asOf)

	// Approximately 10 years. We use epsilon comparison for floats.
	assert.InEpsilon(t, 10.0, remaining, 0.01)

	_ = lr.MarkAbandoned("lost interest")
	assert.Equal(t, 0.0, lr.CalculateRemainingLife(asOf))
}

func TestLifecycleRecord_IsActive(t *testing.T) {
	lr, _ := NewLifecycleRecord("pat1", "CN", time.Now())
	assert.True(t, lr.IsActive())

	_ = lr.TransitionTo(PhaseExamination, "Exam", "user1")
	assert.True(t, lr.IsActive())

	_ = lr.MarkGranted(time.Now())
	assert.True(t, lr.IsActive())

	_ = lr.TransitionTo(PhaseMaintenance, "Paying annuities", "user1")
	assert.True(t, lr.IsActive())

	_ = lr.TransitionTo(PhaseExpired, "End of term", "user1")
	assert.False(t, lr.IsActive())
}

func TestLifecycleRecord_Validate(t *testing.T) {
	lr, _ := NewLifecycleRecord("pat1", "CN", time.Now())
	assert.NoError(t, lr.Validate())

	lr.ID = ""
	assert.Error(t, lr.Validate())
}

func TestLifecycleRecord_FullLifecycle(t *testing.T) {
	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lr, _ := NewLifecycleRecord("pat1", "CN", filingDate)

	assert.Equal(t, PhaseApplication, lr.CurrentPhase)

	assert.NoError(t, lr.TransitionTo(PhaseExamination, "Exam requested", "user1"))
	assert.NoError(t, lr.MarkGranted(filingDate.AddDate(3, 0, 0)))
	assert.NoError(t, lr.TransitionTo(PhaseMaintenance, "Annuities", "user1"))
	assert.NoError(t, lr.TransitionTo(PhaseExpired, "Term end", "system"))

	assert.Equal(t, PhaseExpired, lr.CurrentPhase)
	assert.Equal(t, 4, len(lr.PhaseHistory))
}

//Personal.AI order the ending
