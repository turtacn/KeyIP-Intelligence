package patent

import (
	"testing"
	"time"
)

func TestPatentStatus(t *testing.T) {
	if PatentStatusDraft.String() != "draft" {
		t.Errorf("expected draft, got %s", PatentStatusDraft.String())
	}
	if !PatentStatusFiled.IsActive() {
		t.Error("expected filed to be active")
	}
	if PatentStatusExpired.IsActive() {
		t.Error("expected expired not to be active")
	}
}

func TestNewPatent(t *testing.T) {
	now := time.Now().UTC()
	p, err := NewPatent("US123", "Test Patent", OfficeUSPTO, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.PatentNumber != "US123" {
		t.Errorf("expected US123, got %s", p.PatentNumber)
	}
	if p.Status != PatentStatusFiled {
		t.Errorf("expected filed, got %s", p.Status)
	}
	if p.Dates.FilingDate == nil || !p.Dates.FilingDate.Equal(now) {
		t.Error("filing date mismatch")
	}
}

func TestPatentTransitions(t *testing.T) {
	now := time.Now().UTC()
	p, _ := NewPatent("US123", "Test", OfficeUSPTO, now)

	// Publish
	err := p.Publish(now)
	if err != nil {
		t.Fatalf("unexpected publish error: %v", err)
	}
	if p.Status != PatentStatusPublished {
		t.Errorf("expected published, got %s", p.Status)
	}

	// Exam
	err = p.EnterExamination()
	if err != nil {
		t.Fatalf("unexpected exam error: %v", err)
	}
	if p.Status != PatentStatusUnderExamination {
		t.Errorf("expected under exam, got %s", p.Status)
	}

	// Grant
	err = p.Grant(now, now.AddDate(20, 0, 0))
	if err != nil {
		t.Fatalf("unexpected grant error: %v", err)
	}
	if p.Status != PatentStatusGranted {
		t.Errorf("expected granted, got %s", p.Status)
	}

	// Expire
	err = p.Expire()
	if err != nil {
		t.Fatalf("unexpected expire error: %v", err)
	}
	if p.Status != PatentStatusExpired {
		t.Errorf("expected expired, got %s", p.Status)
	}
}

func TestPatentInvalidTransitions(t *testing.T) {
	now := time.Now().UTC()
	p, _ := NewPatent("US123", "Test", OfficeUSPTO, now)

	// Can't grant directly from Filed (must publish & exam first in this model logic)
	err := p.Grant(now, now)
	if err == nil {
		t.Error("expected error granting from filed status")
	}
}
