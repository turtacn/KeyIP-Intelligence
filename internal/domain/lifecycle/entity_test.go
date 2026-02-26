package lifecycle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewLifecycleRecord_Success(t *testing.T) {
	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lr, err := NewLifecycleRecord("p1", "CN", filingDate)
	assert.NoError(t, err)
	assert.Equal(t, PhaseApplication, lr.CurrentPhase)
	assert.Equal(t, 1, len(lr.Events)) // "filed" event
	assert.NotNil(t, lr.ExpirationDate)
}

func TestNewLifecycleRecord_Validation(t *testing.T) {
	_, err := NewLifecycleRecord("", "CN", time.Now())
	assert.Error(t, err)

	_, err = NewLifecycleRecord("p1", "", time.Now())
	assert.Error(t, err)

	_, err = NewLifecycleRecord("p1", "CN", time.Time{})
	assert.Error(t, err)
}

func TestLifecycleRecord_TransitionTo_Success(t *testing.T) {
	lr, _ := NewLifecycleRecord("p1", "CN", time.Now())

	// Application -> Examination
	err := lr.TransitionTo(PhaseExamination, "Request filed", "user")
	assert.NoError(t, err)
	assert.Equal(t, PhaseExamination, lr.CurrentPhase)
	assert.Equal(t, 1, len(lr.PhaseHistory))

	// Examination -> Granted
	err = lr.TransitionTo(PhaseGranted, "Granted", "system")
	assert.NoError(t, err)
	assert.Equal(t, PhaseGranted, lr.CurrentPhase)
}

func TestLifecycleRecord_TransitionTo_Invalid(t *testing.T) {
	lr, _ := NewLifecycleRecord("p1", "CN", time.Now())

	// Application -> Granted (skip Examination) -> Invalid
	err := lr.TransitionTo(PhaseGranted, "Skip", "user")
	assert.Error(t, err)
}

func TestLifecycleRecord_MarkGranted(t *testing.T) {
	lr, _ := NewLifecycleRecord("p1", "CN", time.Now())
	lr.TransitionTo(PhaseExamination, "Request", "user")

	grantDate := time.Now()
	err := lr.MarkGranted(grantDate)
	assert.NoError(t, err)
	assert.Equal(t, PhaseGranted, lr.CurrentPhase)
	assert.NotNil(t, lr.GrantDate)
}

func TestLifecycleRecord_MarkAbandoned(t *testing.T) {
	lr, _ := NewLifecycleRecord("p1", "CN", time.Now())

	err := lr.MarkAbandoned("No money")
	assert.NoError(t, err)
	assert.Equal(t, PhaseAbandoned, lr.CurrentPhase)
	assert.NotNil(t, lr.AbandonmentDate)
}

func TestLifecycleRecord_CalculateRemainingLife(t *testing.T) {
	filingDate := time.Now().AddDate(-5, 0, 0)
	lr, _ := NewLifecycleRecord("p1", "CN", filingDate)
	// 20 years total. 5 years passed. 15 remaining.

	rem := lr.CalculateRemainingLife(time.Now())
	assert.InDelta(t, 15.0, rem, 0.1)

	// Abandoned -> 0
	lr.MarkAbandoned("Test")
	rem = lr.CalculateRemainingLife(time.Now())
	assert.Equal(t, 0.0, rem)
}

func TestLifecycleRecord_Validate(t *testing.T) {
	lr, _ := NewLifecycleRecord("p1", "CN", time.Now())
	assert.NoError(t, lr.Validate())

	lr.PatentID = ""
	assert.Error(t, lr.Validate())

	// Inconsistent history
	lr, _ = NewLifecycleRecord("p1", "CN", time.Now())
	lr.TransitionTo(PhaseExamination, "reason", "user")
	lr.CurrentPhase = PhaseApplication // Manually mess up
	assert.Error(t, lr.Validate())
}

//Personal.AI order the ending
