package patent

import (
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

type PatentFiledEvent struct {
	common.BaseEvent
	PatentNumber string       `json:"patent_number"`
	Office       PatentOffice `json:"office"`
	FilingDate   *time.Time   `json:"filing_date"`
	Title        string       `json:"title"`
	Version      int          `json:"version"`
}

func NewPatentFiledEvent(p *Patent) *PatentFiledEvent {
	return &PatentFiledEvent{
		BaseEvent:    common.NewBaseEvent(p.ID.String()),
		PatentNumber: p.PatentNumber,
		Office:       p.Office,
		FilingDate:   p.Dates.FilingDate,
		Title:        p.Title,
		Version:      p.Version,
	}
}

type PatentPublishedEvent struct {
	common.BaseEvent
	PatentNumber    string     `json:"patent_number"`
	PublicationDate *time.Time `json:"publication_date"`
	Version         int        `json:"version"`
}

func NewPatentPublishedEvent(p *Patent) *PatentPublishedEvent {
	return &PatentPublishedEvent{
		BaseEvent:       common.NewBaseEvent(p.ID.String()),
		PatentNumber:    p.PatentNumber,
		PublicationDate: p.Dates.PublicationDate,
		Version:         p.Version,
	}
}

type PatentGrantedEvent struct {
	common.BaseEvent
	PatentNumber string     `json:"patent_number"`
	GrantDate    *time.Time `json:"grant_date"`
	ExpiryDate   *time.Time `json:"expiry_date"`
	Version      int        `json:"version"`
}

func NewPatentGrantedEvent(p *Patent) *PatentGrantedEvent {
	return &PatentGrantedEvent{
		BaseEvent:    common.NewBaseEvent(p.ID.String()),
		PatentNumber: p.PatentNumber,
		GrantDate:    p.Dates.GrantDate,
		ExpiryDate:   p.Dates.ExpiryDate,
		Version:      p.Version,
	}
}

type ClaimsUpdatedEvent struct {
	common.BaseEvent
	PatentNumber string `json:"patent_number"`
	ClaimCount   int    `json:"claim_count"`
	Version      int    `json:"version"`
}

func NewClaimsUpdatedEvent(p *Patent) *ClaimsUpdatedEvent {
	return &ClaimsUpdatedEvent{
		BaseEvent:    common.NewBaseEvent(p.ID.String()),
		PatentNumber: p.PatentNumber,
		ClaimCount:   p.ClaimCount(),
		Version:      p.Version,
	}
}

type MoleculeAssociatedEvent struct {
	common.BaseEvent
	PatentNumber string `json:"patent_number"`
	MoleculeID   string `json:"molecule_id"`
	Version      int    `json:"version"`
}

func NewMoleculeAssociatedEvent(p *Patent, moleculeID string) *MoleculeAssociatedEvent {
	return &MoleculeAssociatedEvent{
		BaseEvent:    common.NewBaseEvent(p.ID.String()),
		PatentNumber: p.PatentNumber,
		MoleculeID:   moleculeID,
		Version:      p.Version,
	}
}

type CitationAddedEvent struct {
	common.BaseEvent
	PatentNumber      string `json:"patent_number"`
	CitedPatentNumber string `json:"cited_patent_number"`
	Version           int    `json:"version"`
}

func NewCitationAddedEvent(p *Patent, citedPatentNumber string) *CitationAddedEvent {
	return &CitationAddedEvent{
		BaseEvent:         common.NewBaseEvent(p.ID.String()),
		PatentNumber:      p.PatentNumber,
		CitedPatentNumber: citedPatentNumber,
		Version:           p.Version,
	}
}

//Personal.AI order the ending
