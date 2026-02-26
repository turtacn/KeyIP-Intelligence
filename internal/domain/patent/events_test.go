package patent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEventType_Constants(t *testing.T) {
	assert.Equal(t, EventType("patent.created"), EventPatentCreated)
	assert.Equal(t, EventType("patent.published"), EventPatentPublished)
	assert.Equal(t, EventType("patent.examination_started"), EventPatentExaminationStarted)
}

func TestBaseEvent_NewBaseEvent(t *testing.T) {
	be := NewBaseEvent(EventPatentCreated, "agg-id", 1)
	assert.NotEmpty(t, be.EventID())
	assert.Equal(t, EventPatentCreated, be.EventType())
	assert.Equal(t, "agg-id", be.AggregateID())
	assert.Equal(t, "Patent", be.AggregateType())
	assert.WithinDuration(t, time.Now().UTC(), be.OccurredAt(), time.Second)
	assert.Equal(t, 1, be.Version())
}

func TestPatentCreatedEvent_New(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	e := NewPatentCreatedEvent(p)

	assert.Equal(t, EventPatentCreated, e.EventType())
	assert.Equal(t, p.ID, e.AggregateID())
	assert.Equal(t, "CN123", e.PatentNumber)
	assert.Equal(t, "Title", e.Title)

	payload := e.Payload().(map[string]interface{})
	assert.Equal(t, "CN123", payload["patent_number"])
}

func TestPatentCreatedEvent_Implements_DomainEvent(t *testing.T) {
	var _ DomainEvent = (*PatentCreatedEvent)(nil)
}

func TestPatentPublishedEvent_New(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Publish(time.Now())
	// p.Publish creates event internally, but we test constructor here
	e := NewPatentPublishedEvent(p)
	assert.Equal(t, EventPatentPublished, e.EventType())
	assert.NotZero(t, e.PublicationDate)
}

func TestPatentExaminationStartedEvent_New(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	e := NewPatentExaminationStartedEvent(p)
	assert.Equal(t, EventPatentExaminationStarted, e.EventType())
	assert.Equal(t, "CN123", e.PatentNumber)
}

func TestPatentGrantedEvent_New(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Publish(time.Now())
	p.EnterExamination()
	p.Grant(time.Now(), time.Now().AddDate(20, 0, 0))

	e := NewPatentGrantedEvent(p)
	assert.Equal(t, EventPatentGranted, e.EventType())
	assert.NotZero(t, e.GrantDate)
	assert.NotZero(t, e.ExpiryDate)
}

func TestPatentRejectedEvent_New(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	e := NewPatentRejectedEvent(p, "Reason")
	assert.Equal(t, EventPatentRejected, e.EventType())
	assert.Equal(t, "Reason", e.Reason)
}

func TestPatentWithdrawnEvent_New(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	e := NewPatentWithdrawnEvent(p, PatentStatusFiled)
	assert.Equal(t, EventPatentWithdrawn, e.EventType())
	assert.Equal(t, PatentStatusFiled, e.PreviousStatus)
}

func TestPatentExpiredEvent_New(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	e := NewPatentExpiredEvent(p)
	assert.Equal(t, EventPatentExpired, e.EventType())
}

func TestPatentInvalidatedEvent_New(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	e := NewPatentInvalidatedEvent(p, "Reason")
	assert.Equal(t, EventPatentInvalidated, e.EventType())
	assert.Equal(t, "Reason", e.InvalidationReason)
}

func TestPatentLapsedEvent_New(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	e := NewPatentLapsedEvent(p)
	assert.Equal(t, EventPatentLapsed, e.EventType())
}

func TestPatentClaimsUpdatedEvent_New(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	c, err := NewClaim(1, "A generic text for claim", ClaimTypeIndependent, ClaimCategoryProduct)
	if err != nil {
		t.Fatalf("failed to create claim: %v", err)
	}
	p.SetClaims(ClaimSet{*c})

	e := NewPatentClaimsUpdatedEvent(p)
	assert.Equal(t, EventPatentClaimsUpdated, e.EventType())
	assert.Equal(t, 1, e.ClaimCount)
}

func TestPatentMoleculeLinkedEvent_New(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.AddMolecule("M1")

	e := NewPatentMoleculeLinkedEvent(p, "M1")
	assert.Equal(t, EventPatentMoleculeLinked, e.EventType())
	assert.Equal(t, "M1", e.MoleculeID)
	assert.Equal(t, 1, e.TotalLinkedMolecules)
}

func TestPatentMoleculeUnlinkedEvent_New(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.AddMolecule("M1")
	p.RemoveMolecule("M1")

	e := NewPatentMoleculeUnlinkedEvent(p, "M1")
	assert.Equal(t, EventPatentMoleculeUnlinked, e.EventType())
	assert.Equal(t, "M1", e.MoleculeID)
	assert.Equal(t, 0, e.TotalLinkedMolecules)
}

func TestPatentCitationAddedEvent_New_Forward(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	e := NewPatentCitationAddedEvent(p, "CN999", "forward")
	assert.Equal(t, EventPatentCitationAdded, e.EventType())
	assert.Equal(t, "forward", e.Direction)
}

func TestPatentCitationAddedEvent_New_Backward(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	e := NewPatentCitationAddedEvent(p, "CN999", "backward")
	assert.Equal(t, "backward", e.Direction)
}

func TestPatentAnalysisCompletedEvent_New(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	e := NewPatentAnalysisCompletedEvent(p, "fto", "safe")
	assert.Equal(t, EventPatentAnalysisCompleted, e.EventType())
	assert.Equal(t, "fto", e.AnalysisType)
}

func TestEvent_Immutability(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	e := NewPatentCreatedEvent(p)
	originalTitle := e.Title

	// Modify Patent
	p.Title = "New Title"

	// Event should keep original title
	assert.Equal(t, originalTitle, e.Title)
}

func TestPatent_EventCollection(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	assert.Empty(t, p.DomainEvents())

	p.Publish(time.Now())
	assert.Len(t, p.DomainEvents(), 1)
	assert.Equal(t, EventPatentPublished, p.DomainEvents()[0].EventType())

	p.EnterExamination()
	assert.Len(t, p.DomainEvents(), 2)
	assert.Equal(t, EventPatentExaminationStarted, p.DomainEvents()[1].EventType())

	p.ClearEvents()
	assert.Empty(t, p.DomainEvents())
}

//Personal.AI order the ending
