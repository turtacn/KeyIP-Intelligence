package citation

import (
	"time"

	"github.com/google/uuid"
)

type PatentNode struct {
	ID           uuid.UUID `json:"id"`
	PatentNumber string    `json:"patent_number"`
	Jurisdiction string    `json:"jurisdiction"`
	FilingDate   *time.Time `json:"filing_date,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type PatentNodeData struct {
	ID           uuid.UUID
	PatentNumber string
	Jurisdiction string
	FilingDate   *time.Time
}

type CitationEdge struct {
	FromPatentID uuid.UUID
	ToPatentID   uuid.UUID
	Type         string
	Metadata     map[string]any
}

type CitationPath struct {
	Nodes     []*PatentNode `json:"nodes"`
	Relations []string      `json:"relations"`
	Length    int           `json:"length"`
}

type CitationNetwork struct {
	Nodes []*PatentNode   `json:"nodes"`
	Edges []*CitationEdge `json:"edges"`
}

type CitationStats struct {
	ForwardCount  int64 `json:"forward_count"`
	BackwardCount int64 `json:"backward_count"`
	TotalCount    int64 `json:"total_count"`
}

type PatentWithCitationCount struct {
	Patent        *PatentNode `json:"patent"`
	CitationCount int64       `json:"citation_count"`
}

type CoCitationResult struct {
	Patent      *PatentNode `json:"patent"`
	CommonCount int64       `json:"common_count"`
}

type CouplingResult struct {
	Patent      *PatentNode `json:"patent"`
	CommonCount int64       `json:"common_count"`
}

type PatentWithPageRank struct {
	Patent   *PatentNode `json:"patent"`
	PageRank float64     `json:"pagerank"`
}

//Personal.AI order the ending
