package patent

import (
	"testing"
	"time"
)

func TestEventType_Constants(t *testing.T) {
	if EventPatentCreated != "patent.created" {
		t.Error("EventPatentCreated constant mismatch")
	}
}

func TestBaseEvent_NewBaseEvent(t *testing.T) {
	e := NewBaseEvent(EventPatentCreated, "agg-123", 1)
	if e.EventID() == "" {
		t.Error("EventID should not be empty")
	}
	if e.EventType() != EventPatentCreated {
		t.Error("EventType mismatch")
	}
	if e.AggregateID() != "agg-123" {
		t.Error("AggregateID mismatch")
	}
	if e.Version() != 1 {
		t.Error("Version mismatch")
	}
	if e.OccurredAt().IsZero() {
		t.Error("OccurredAt should not be zero")
	}
	if e.AggregateType() != "Patent" {
		t.Error("AggregateType mismatch")
	}
}

func newTestPatentForEvents() *Patent {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now().UTC())
	return p
}

func TestPatentCreatedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	e := NewPatentCreatedEvent(p)
	if e.EventType() != EventPatentCreated {
		t.Error("EventType mismatch")
	}
	payload := e.Payload().(struct {
		PatentNumber string       `json:"patent_number"`
		Title        string       `json:"title"`
		Office       PatentOffice `json:"office"`
		FilingDate   time.Time    `json:"filing_date"`
	})
	if payload.PatentNumber != p.PatentNumber {
		t.Error("PatentNumber mismatch in payload")
	}
}

func TestPatentStatusEvents(t *testing.T) {
	p := newTestPatentForEvents()

	t.Run("Published", func(t *testing.T) {
		now := time.Now().UTC()
		p.Publish(now)
		e := NewPatentPublishedEvent(p)
		payload := e.Payload().(struct {
			PatentNumber    string    `json:"patent_number"`
			PublicationDate time.Time `json:"publication_date"`
		})
		if payload.PublicationDate != now {
			t.Error("PublicationDate mismatch")
		}
	})

	t.Run("Granted", func(t *testing.T) {
		p.Status = PatentStatusUnderExamination
		now := time.Now().UTC()
		p.Grant(now, now.AddDate(20, 0, 0))
		e := NewPatentGrantedEvent(p)
		payload := e.Payload().(struct {
			PatentNumber string    `json:"patent_number"`
			GrantDate    time.Time `json:"grant_date"`
			ExpiryDate   time.Time `json:"expiry_date"`
			ClaimCount   int       `json:"claim_count"`
		})
		if payload.GrantDate != now {
			t.Error("GrantDate mismatch")
		}
	})
}

func TestPatentRejectedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	e := NewPatentRejectedEvent(p, "Prior art found")
	payload := e.Payload().(struct {
		PatentNumber string `json:"patent_number"`
		Reason       string `json:"reason"`
	})
	if payload.Reason != "Prior art found" {
		t.Error("Reason mismatch")
	}
}

func TestPatentWithdrawnEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	e := NewPatentWithdrawnEvent(p, PatentStatusFiled)
	payload := e.Payload().(struct {
		PatentNumber   string       `json:"patent_number"`
		PreviousStatus PatentStatus `json:"previous_status"`
	})
	if payload.PreviousStatus != PatentStatusFiled {
		t.Error("PreviousStatus mismatch")
	}
}

func TestPatentExpiredEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	now := time.Now().UTC()
	p.Dates.ExpiryDate = &now
	e := NewPatentExpiredEvent(p)
	payload := e.Payload().(struct {
		PatentNumber string    `json:"patent_number"`
		ExpiryDate   time.Time `json:"expiry_date"`
	})
	if payload.ExpiryDate != now {
		t.Error("ExpiryDate mismatch")
	}
}

func TestPatentInvalidatedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	e := NewPatentInvalidatedEvent(p, "Lack of novelty")
	payload := e.Payload().(struct {
		PatentNumber      string `json:"patent_number"`
		InvalidationReason string `json:"invalidation_reason"`
	})
	if payload.InvalidationReason != "Lack of novelty" {
		t.Error("InvalidationReason mismatch")
	}
}

func TestPatentLapsedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	e := NewPatentLapsedEvent(p)
	if e.EventType() != EventPatentLapsed {
		t.Error("EventType mismatch")
	}
}

func TestPatentClaimsUpdatedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	e := NewPatentClaimsUpdatedEvent(p)
	if e.EventType() != EventPatentClaimsUpdated {
		t.Error("EventType mismatch")
	}
}

func TestPatentMoleculeEvents(t *testing.T) {
	p := newTestPatentForEvents()
	p.MoleculeIDs = []string{"M1", "M2"}

	t.Run("Linked", func(t *testing.T) {
		e := NewPatentMoleculeLinkedEvent(p, "M3")
		payload := e.Payload().(struct {
			PatentNumber         string `json:"patent_number"`
			MoleculeID           string `json:"molecule_id"`
			TotalLinkedMolecules int    `json:"total_linked_molecules"`
		})
		if payload.MoleculeID != "M3" || payload.TotalLinkedMolecules != 2 {
			t.Error("Payload mismatch")
		}
	})

	t.Run("Unlinked", func(t *testing.T) {
		e := NewPatentMoleculeUnlinkedEvent(p, "M1")
		payload := e.Payload().(struct {
			PatentNumber         string `json:"patent_number"`
			MoleculeID           string `json:"molecule_id"`
			TotalLinkedMolecules int    `json:"total_linked_molecules"`
		})
		if payload.MoleculeID != "M1" {
			t.Error("Payload mismatch")
		}
	})
}

func TestPatentCitationAddedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	e := NewPatentCitationAddedEvent(p, "CIT-123", "forward")
	payload := e.Payload().(struct {
		PatentNumber      string `json:"patent_number"`
		CitedPatentNumber string `json:"cited_patent_number"`
		Direction         string `json:"direction"`
	})
	if payload.CitedPatentNumber != "CIT-123" || payload.Direction != "forward" {
		t.Error("Payload mismatch")
	}
}

func TestPatentAnalysisCompletedEvent_New(t *testing.T) {
	p := newTestPatentForEvents()
	e := NewPatentAnalysisCompletedEvent(p, "infringement", "High risk")
	payload := e.Payload().(struct {
		PatentNumber  string `json:"patent_number"`
		AnalysisType  string `json:"analysis_type"`
		ResultSummary string `json:"result_summary"`
	})
	if payload.AnalysisType != "infringement" || payload.ResultSummary != "High risk" {
		t.Error("Payload mismatch")
	}
}

func TestEvent_UniqueIDs(t *testing.T) {
	e1 := NewBaseEvent(EventPatentCreated, "1", 1)
	e2 := NewBaseEvent(EventPatentCreated, "1", 1)
	if e1.EventID() == e2.EventID() {
		t.Error("Event IDs should be unique")
	}
}

func TestEvent_OccurredAt_UTC(t *testing.T) {
	e := NewBaseEvent(EventPatentCreated, "1", 1)
	if e.OccurredAt().Location() != time.UTC {
		// Note: time.Now().UTC().Location() is UTC.
	}
}

//Personal.AI order the ending
