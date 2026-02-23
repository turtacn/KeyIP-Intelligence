package services

import (
	"context"
	"testing"
)

func TestNewPatentService(t *testing.T) {
	svc := NewPatentService()
	if svc == nil {
		t.Fatal("NewPatentService should not return nil")
	}
}

func TestPatentService_GetPatent(t *testing.T) {
	svc := NewPatentService()
	ctx := context.Background()

	req := &GetPatentRequest{Number: "CN202110123456"}
	resp, err := svc.GetPatent(ctx, req)

	if err != nil {
		t.Fatalf("GetPatent failed: %v", err)
	}

	if resp.Number != "CN202110123456" {
		t.Errorf("expected Number=CN202110123456, got %s", resp.Number)
	}

	if resp.Title == "" {
		t.Error("expected non-empty title")
	}
}

func TestPatentService_SearchPatents(t *testing.T) {
	svc := NewPatentService()
	ctx := context.Background()

	req := &SearchPatentsRequest{Query: "OLED"}
	resp, err := svc.SearchPatents(ctx, req)

	if err != nil {
		t.Fatalf("SearchPatents failed: %v", err)
	}

	if resp.TotalCount == 0 {
		t.Error("expected at least one result")
	}

	if len(resp.Results) == 0 {
		t.Error("expected non-empty results")
	}
}

func TestPatentService_AnalyzeInfringement(t *testing.T) {
	svc := NewPatentService()
	ctx := context.Background()

	req := &InfringementRequest{
		PatentNumber: "CN202110123456",
		MoleculeId:   "mol-123",
	}
	resp, err := svc.AnalyzeInfringement(ctx, req)

	if err != nil {
		t.Fatalf("AnalyzeInfringement failed: %v", err)
	}

	if resp.RiskLevel == "" {
		t.Error("expected non-empty risk level")
	}

	if resp.Confidence < 0 || resp.Confidence > 1 {
		t.Errorf("confidence should be in [0,1], got %f", resp.Confidence)
	}
}

//Personal.AI order the ending
