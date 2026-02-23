package services

import "testing"

func TestNewMoleculeServiceServer(t *testing.T) {
	svc := NewMoleculeServiceServer()
	if svc == nil {
		t.Error("service should not be nil")
	}
}

//Personal.AI order the ending
