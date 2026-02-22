package family

import (
	"testing"
	"time"
)

func TestFamilyMember(t *testing.T) {
	now := time.Now().UTC()
	member := &FamilyMember{
		PatentID:     "123",
		PatentNumber: "US123",
		Jurisdiction: "US",
		Role:         "priority",
		AddedAt:      now,
	}

	if member.PatentID != "123" {
		t.Errorf("expected PatentID 123, got %s", member.PatentID)
	}
}

func TestFamilyAggregate(t *testing.T) {
	agg := &FamilyAggregate{
		FamilyID:   "fam-1",
		FamilyType: "simple",
		Members:    []FamilyMember{{PatentID: "1"}},
	}

	if agg.FamilyID != "fam-1" {
		t.Errorf("expected FamilyID fam-1, got %s", agg.FamilyID)
	}
	if len(agg.Members) != 1 {
		t.Errorf("expected 1 member, got %d", len(agg.Members))
	}
}

func TestFamilyStats(t *testing.T) {
	stats := &FamilyStats{
		TotalMembers:       5,
		JurisdictionCounts: map[string]int64{"US": 3, "EP": 2},
	}

	if stats.TotalMembers != 5 {
		t.Errorf("expected TotalMembers 5, got %d", stats.TotalMembers)
	}
	if stats.JurisdictionCounts["US"] != 3 {
		t.Errorf("expected 3 US patents, got %d", stats.JurisdictionCounts["US"])
	}
}
