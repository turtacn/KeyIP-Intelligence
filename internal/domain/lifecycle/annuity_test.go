package lifecycle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

func TestNewAnnuity(t *testing.T) {
	due := time.Now().AddDate(1, 0, 0)
	a, err := NewAnnuity("pat-1", 1, due, "USD", 1000)
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.Equal(t, "pat-1", a.PatentID)
	assert.Equal(t, int64(1000), *a.Amount)
	assert.Equal(t, AnnuityStatusUpcoming, a.Status)
	assert.Equal(t, "USD", a.Currency)
	assert.NotNil(t, a.GraceDeadline)
}

func TestNewAnnuity_Invalid(t *testing.T) {
	due := time.Now()
	_, err := NewAnnuity("", 1, due, "USD", 100)
	assert.Error(t, err)
	assert.True(t, errors.IsValidation(err))

	_, err = NewAnnuity("pat-1", 0, due, "USD", 100)
	assert.Error(t, err)

	_, err = NewAnnuity("pat-1", 1, due, "", 100)
	assert.Error(t, err)

	_, err = NewAnnuity("pat-1", 1, due, "USD", -10)
	assert.Error(t, err)
}

func TestAnnuity_Pay(t *testing.T) {
	due := time.Now()
	a, _ := NewAnnuity("pat-1", 1, due, "USD", 1000)

	now := time.Now()
	err := a.Pay(1000, "USD", now, "REF123")
	assert.NoError(t, err)
	assert.Equal(t, AnnuityStatusPaid, a.Status)
	assert.Equal(t, int64(1000), *a.PaidAmount)

	// Invalid Currency
	err = a.Pay(1000, "EUR", now, "REF456")
	assert.Error(t, err)

	// Invalid Amount
	err = a.Pay(-100, "USD", now, "REF789")
	assert.Error(t, err)
}

func TestAnnuity_CheckStatus(t *testing.T) {
	now := time.Now().UTC()

	// Upcoming (far future)
	a1, _ := NewAnnuity("p1", 1, now.AddDate(1, 0, 0), "USD", 100)
	a1.CheckStatus(now)
	assert.Equal(t, AnnuityStatusUpcoming, a1.Status)

	// Due (within 3 months)
	a2, _ := NewAnnuity("p2", 1, now.AddDate(0, 1, 0), "USD", 100)
	a2.CheckStatus(now)
	assert.Equal(t, AnnuityStatusDue, a2.Status)

	// Grace Period (past due but before grace)
	a3, _ := NewAnnuity("p3", 1, now.AddDate(0, -1, 0), "USD", 100)
	a3.CheckStatus(now)
	assert.Equal(t, AnnuityStatusGracePeriod, a3.Status)

	// Overdue (past grace)
	a4, _ := NewAnnuity("p4", 1, now.AddDate(0, -7, 0), "USD", 100)
	a4.CheckStatus(now)
	assert.Equal(t, AnnuityStatusOverdue, a4.Status)
}

//Personal.AI order the ending
