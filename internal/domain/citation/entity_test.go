package citation

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPatentNode(t *testing.T) {
	id := uuid.New()
	now := time.Now().UTC()
	node := &PatentNode{
		ID:           id,
		PatentNumber: "US123456A",
		Jurisdiction: "US",
		FilingDate:   &now,
		CreatedAt:    now,
	}

	if node.ID != id {
		t.Errorf("expected ID %v, got %v", id, node.ID)
	}
	if node.PatentNumber != "US123456A" {
		t.Errorf("expected PatentNumber US123456A, got %s", node.PatentNumber)
	}
}

func TestCitationStats(t *testing.T) {
	stats := &CitationStats{
		ForwardCount:  10,
		BackwardCount: 5,
		TotalCount:    15,
	}

	if stats.ForwardCount != 10 {
		t.Errorf("expected ForwardCount 10, got %d", stats.ForwardCount)
	}
	if stats.TotalCount != 15 {
		t.Errorf("expected TotalCount 15, got %d", stats.TotalCount)
	}
}

func TestCitationPath(t *testing.T) {
	path := &CitationPath{
		Nodes:     []*PatentNode{{PatentNumber: "A"}, {PatentNumber: "B"}},
		Relations: []string{"CITED_BY"},
		Length:    1,
	}

	if len(path.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(path.Nodes))
	}
	if path.Length != 1 {
		t.Errorf("expected Length 1, got %d", path.Length)
	}
}
