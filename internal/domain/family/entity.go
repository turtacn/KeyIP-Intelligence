package family

import (
	"time"
)

type FamilyMember struct {
	PatentID     string    `json:"patent_id"`
	PatentNumber string    `json:"patent_number"`
	Jurisdiction string    `json:"jurisdiction"`
	FilingDate   *time.Time `json:"filing_date,omitempty"`
	Role         string    `json:"role"`
	AddedAt      time.Time `json:"added_at"`
}

type FamilyAggregate struct {
	FamilyID   string          `json:"family_id"`
	FamilyType string          `json:"family_type"`
	Members    []FamilyMember  `json:"members"`
	Metadata   map[string]any  `json:"metadata,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type FamilyMemberInput struct {
	PatentID string
	Role     string
}

type PriorityLink struct {
	FromPatentID    string    `json:"from_patent_id"`
	ToPatentID      string    `json:"to_patent_id"`
	PriorityDate    time.Time `json:"priority_date"`
	PriorityCountry string    `json:"priority_country"`
}

type FamilyStats struct {
	TotalMembers            int64            `json:"total_members"`
	Jurisdictions           []string         `json:"jurisdictions"`
	JurisdictionCounts      map[string]int64 `json:"jurisdiction_counts"`
	EarliestFiling          *time.Time       `json:"earliest_filing,omitempty"`
	LatestExpiry            *time.Time       `json:"latest_expiry,omitempty"`
	StatusDistribution      map[string]int64 `json:"status_distribution"`
}

type RelatedFamily struct {
	FamilyID     string `json:"family_id"`
	FamilyType   string `json:"family_type"`
	OverlapCount int64  `json:"overlap_count"`
}

//Personal.AI order the ending
