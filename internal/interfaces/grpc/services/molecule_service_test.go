package services

import (
	"context"
	"testing"
)

func TestNewMoleculeService(t *testing.T) {
	svc := NewMoleculeService()
	if svc == nil {
		t.Fatal("NewMoleculeService should not return nil")
	}
}

func TestMoleculeService_GetMolecule(t *testing.T) {
	svc := NewMoleculeService()
	ctx := context.Background()

	req := &GetMoleculeRequest{Id: "mol-123"}
	resp, err := svc.GetMolecule(ctx, req)

	if err != nil {
		t.Fatalf("GetMolecule failed: %v", err)
	}

	if resp.Id != "mol-123" {
		t.Errorf("expected Id=mol-123, got %s", resp.Id)
	}

	if resp.Smiles == "" {
		t.Error("expected non-empty SMILES")
	}
}

func TestMoleculeService_SearchMolecules(t *testing.T) {
	svc := NewMoleculeService()
	ctx := context.Background()

	req := &SearchMoleculesRequest{Query: "benzene"}
	resp, err := svc.SearchMolecules(ctx, req)

	if err != nil {
		t.Fatalf("SearchMolecules failed: %v", err)
	}

	if resp.TotalCount == 0 {
		t.Error("expected at least one result")
	}

	if len(resp.Results) == 0 {
		t.Error("expected non-empty results")
	}
}

func TestMoleculeService_CalculateSimilarity(t *testing.T) {
	svc := NewMoleculeService()
	ctx := context.Background()

	req := &SimilarityRequest{
		Mol1: "C1=CC=CC=C1",
		Mol2: "C1=CC=CC=C1C",
	}
	resp, err := svc.CalculateSimilarity(ctx, req)

	if err != nil {
		t.Fatalf("CalculateSimilarity failed: %v", err)
	}

	if resp.Similarity < 0 || resp.Similarity > 1 {
		t.Errorf("similarity should be in [0,1], got %f", resp.Similarity)
	}

	if resp.Method == "" {
		t.Error("expected non-empty method")
	}
}

//Personal.AI order the ending
