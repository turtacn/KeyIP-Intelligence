package patent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

func newTestPatentForEvents() *Patent {
	now := time.Now().UTC()
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, now)
	return p
}

func TestEventType_Constants(t *testing.T) {
	assert.Equal(t, common.EventType("patent.created"), EventPatentCreated)
	assert.Equal(t, common.EventType("patent.granted"), EventPatentGranted)
	// Add more if needed, basically just ensuring they are defined
}

func TestPatentCreatedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	event := NewPatentCreatedEvent(p)

	assert.Equal(t, EventPatentCreated, event.EventType())
	assert.Equal(t, p.ID.String(), event.AggregateID())
	assert.Equal(t, p.Version, event.Version())
	assert.Equal(t, p.PatentNumber, event.PatentNumber)
	assert.Equal(t, p.Title, event.Title)
	assert.Equal(t, p.Office, event.Office)
	assert.Equal(t, p.Dates.FilingDate, event.FilingDate)
	assert.False(t, event.OccurredAt().IsZero())
}

func TestPatentPublishedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	now := time.Now().UTC()
	p.Publish(now)
	event := NewPatentPublishedEvent(p)

	assert.Equal(t, EventPatentPublished, event.EventType())
	assert.Equal(t, p.PatentNumber, event.PatentNumber)
	assert.Equal(t, p.Dates.PublicationDate, event.PublicationDate)
}

func TestPatentGrantedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	p.Publish(time.Now())
	p.EnterExamination()
	grantDate := time.Now().UTC()
	expiryDate := grantDate.AddDate(20, 0, 0)
	p.Grant(grantDate, expiryDate)
	event := NewPatentGrantedEvent(p)

	assert.Equal(t, EventPatentGranted, event.EventType())
	assert.Equal(t, p.PatentNumber, event.PatentNumber)
	assert.Equal(t, p.Dates.GrantDate, event.GrantDate)
	assert.Equal(t, p.Dates.ExpiryDate, event.ExpiryDate)
	assert.Equal(t, 0, event.ClaimCount)
}

func TestPatentRejectedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	event := NewPatentRejectedEvent(p, "Prior Art")

	assert.Equal(t, EventPatentRejected, event.EventType())
	assert.Equal(t, p.PatentNumber, event.PatentNumber)
	assert.Equal(t, "Prior Art", event.Reason)
}

func TestPatentWithdrawnEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	event := NewPatentWithdrawnEvent(p, PatentStatusFiled)

	assert.Equal(t, EventPatentWithdrawn, event.EventType())
	assert.Equal(t, p.PatentNumber, event.PatentNumber)
	assert.Equal(t, PatentStatusFiled, event.PreviousStatus)
}

func TestPatentExpiredEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	expiryDate := time.Now().UTC()
	p.Dates.ExpiryDate = &expiryDate
	event := NewPatentExpiredEvent(p)

	assert.Equal(t, EventPatentExpired, event.EventType())
	assert.Equal(t, p.PatentNumber, event.PatentNumber)
	assert.Equal(t, p.Dates.ExpiryDate, event.ExpiryDate)
}

func TestPatentInvalidatedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	event := NewPatentInvalidatedEvent(p, "Lack of Novelty")

	assert.Equal(t, EventPatentInvalidated, event.EventType())
	assert.Equal(t, p.PatentNumber, event.PatentNumber)
	assert.Equal(t, "Lack of Novelty", event.InvalidationReason)
}

func TestPatentLapsedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	event := NewPatentLapsedEvent(p)

	assert.Equal(t, EventPatentLapsed, event.EventType())
	assert.Equal(t, p.PatentNumber, event.PatentNumber)
}

func TestPatentClaimsUpdatedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	p.Claims = ClaimSet{
		{Number: 1, Type: ClaimTypeIndependent},
		{Number: 2, Type: ClaimTypeDependent, DependsOn: []int{1}},
	}
	event := NewPatentClaimsUpdatedEvent(p)

	assert.Equal(t, EventPatentClaimsUpdated, event.EventType())
	assert.Equal(t, p.PatentNumber, event.PatentNumber)
	assert.Equal(t, 2, event.ClaimCount)
	assert.Equal(t, 1, event.IndependentClaimCount)
	assert.False(t, event.HasMarkush)
}

func TestPatentMoleculeLinkedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	p.AddMolecule("MOL1")
	event := NewPatentMoleculeLinkedEvent(p, "MOL1")

	assert.Equal(t, EventPatentMoleculeLinked, event.EventType())
	assert.Equal(t, p.PatentNumber, event.PatentNumber)
	assert.Equal(t, "MOL1", event.MoleculeID)
	assert.Equal(t, 1, event.TotalLinkedMolecules)
}

func TestPatentMoleculeUnlinkedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	// Simulate unlink logic (RemoveMolecule implementation)
	event := NewPatentMoleculeUnlinkedEvent(p, "MOL1")

	assert.Equal(t, EventPatentMoleculeUnlinked, event.EventType())
	assert.Equal(t, p.PatentNumber, event.PatentNumber)
	assert.Equal(t, "MOL1", event.MoleculeID)
}

func TestPatentCitationAddedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	event := NewPatentCitationAddedEvent(p, "US999", "forward")

	assert.Equal(t, EventPatentCitationAdded, event.EventType())
	assert.Equal(t, p.PatentNumber, event.PatentNumber)
	assert.Equal(t, "US999", event.CitedPatentNumber)
	assert.Equal(t, "forward", event.Direction)
}

func TestPatentAnalysisCompletedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	event := NewPatentAnalysisCompletedEvent(p, "infringement", "High Risk")

	assert.Equal(t, EventPatentAnalysisCompleted, event.EventType())
	assert.Equal(t, p.PatentNumber, event.PatentNumber)
	assert.Equal(t, "infringement", event.AnalysisType)
	assert.Equal(t, "High Risk", event.ResultSummary)
}

//Personal.AI order the ending
