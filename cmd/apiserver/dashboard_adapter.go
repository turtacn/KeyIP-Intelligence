// dashboard_adapter.go — Minimal management dashboard with competitive radar.
// Provides competitive landscape analysis and NL query delegation for the
// management dashboard use case.
package main

import (
	"context"
	"fmt"
	"time"
)

// CompetitiveRadarPoint represents a single competitor on the radar chart.
type CompetitiveRadarPoint struct {
	CompetitorName    string  `json:"competitor_name"`
	PatentCount       int     `json:"patent_count"`
	AvgQualityScore   float64 `json:"avg_quality_score"`
	TechCoveragePct   float64 `json:"tech_coverage_pct"`
	LitigationCount   int     `json:"litigation_count"`
	MarketSharePct    float64 `json:"market_share_pct"`
	InnovationIndex   float64 `json:"innovation_index"`
}

// DashboardSummary provides top-level management KPIs.
type DashboardSummary struct {
	TotalPatents      int                        `json:"total_patents"`
	ActiveMolecules   int                        `json:"active_molecules"`
	PendingReports    int                        `json:"pending_reports"`
	HighRiskAlerts    int                        `json:"high_risk_alerts"`
	CompetitiveRadar  []CompetitiveRadarPoint    `json:"competitive_radar"`
	RecentActivity    []DashboardActivity        `json:"recent_activity"`
	GeneratedAt       time.Time                  `json:"generated_at"`
}

// DashboardActivity records a recent event in the system.
type DashboardActivity struct {
	Timestamp   time.Time `json:"timestamp"`
	EventType   string    `json:"event_type"`
	Description string    `json:"description"`
	EntityID    string    `json:"entity_id"`
}

// DashboardService provides management dashboard data.
// In production this aggregates data from PostgreSQL, Redis, and the
// competitive intelligence engines. This minimal adapter returns
// static example data that demonstrates the dashboard contract.
type DashboardService struct {
	// Ready for real data sources: patentRepo, moleculeRepo, portfolioSvc, etc.
}

// NewDashboardService creates a minimal dashboard service.
func NewDashboardService() *DashboardService {
	return &DashboardService{}
}

// GetSummary returns the dashboard summary with radar data.
func (d *DashboardService) GetSummary(ctx context.Context) (*DashboardSummary, error) {
	now := time.Now()
	return &DashboardSummary{
		TotalPatents:    1423,
		ActiveMolecules: 87,
		PendingReports:  3,
		HighRiskAlerts:  12,
		CompetitiveRadar: []CompetitiveRadarPoint{
			{CompetitorName: "Pfizer", PatentCount: 4521, AvgQualityScore: 78.5, TechCoveragePct: 92.0, LitigationCount: 34, MarketSharePct: 28.0, InnovationIndex: 85.3},
			{CompetitorName: "Novartis", PatentCount: 3890, AvgQualityScore: 82.1, TechCoveragePct: 88.0, LitigationCount: 28, MarketSharePct: 24.0, InnovationIndex: 88.7},
			{CompetitorName: "Roche", PatentCount: 3120, AvgQualityScore: 76.3, TechCoveragePct: 85.0, LitigationCount: 22, MarketSharePct: 19.0, InnovationIndex: 79.1},
			{CompetitorName: "Merck", PatentCount: 2840, AvgQualityScore: 80.7, TechCoveragePct: 82.0, LitigationCount: 19, MarketSharePct: 17.0, InnovationIndex: 82.4},
			{CompetitorName: "Bayer", PatentCount: 2150, AvgQualityScore: 72.4, TechCoveragePct: 75.0, LitigationCount: 15, MarketSharePct: 12.0, InnovationIndex: 71.8},
		},
		RecentActivity: []DashboardActivity{
			{Timestamp: now.Add(-10 * time.Minute), EventType: "report_completed", Description: "FTO Report for Ibuprofen completed", EntityID: "rpt-001"},
			{Timestamp: now.Add(-35 * time.Minute), EventType: "patent_imported", Description: "5 new patents imported from USPTO", EntityID: "imp-042"},
			{Timestamp: now.Add(-1 * time.Hour), EventType: "risk_alert", Description: "High infringement risk detected for EP1234567", EntityID: "alert-099"},
		},
		GeneratedAt: now,
	}, nil
}

// QueryNL performs a natural language query against the dashboard data.
// In production, this delegates to query.NLQueryService.
func (d *DashboardService) QueryNL(ctx context.Context, question string) (string, error) {
	// Placeholder: in production, delegates to the NLQueryService + LLM.
	if question == "" {
		return "", fmt.Errorf("question is required")
	}
	return fmt.Sprintf("Dashboard NL query received: \"%s\". NL query engine is ready for backend wiring.", question), nil
}

//Personal.AI order the ending
