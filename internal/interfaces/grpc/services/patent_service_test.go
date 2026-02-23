package services

import "testing"

func TestNewPatentServiceServer(t *testing.T) {
	svc := NewPatentServiceServer()
	if svc == nil {
		t.Error("service should not be nil")
	}
}

//Personal.AI order the ending
