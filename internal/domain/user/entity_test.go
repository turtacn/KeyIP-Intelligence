package user

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestUserStruct(t *testing.T) {
	id := uuid.New()
	now := time.Now().UTC()
	u := &User{
		ID:          id,
		Email:       "test@example.com",
		Username:    "testuser",
		CreatedAt:   now,
		Status:      "active",
		MFAEnabled:  false,
	}

	if u.ID != id {
		t.Errorf("expected ID %v, got %v", id, u.ID)
	}
	if u.Email != "test@example.com" {
		t.Errorf("expected Email test@example.com, got %s", u.Email)
	}
}

func TestOrganizationStruct(t *testing.T) {
	id := uuid.New()
	org := &Organization{
		ID:   id,
		Name: "Test Org",
		Plan: "Pro",
	}

	if org.ID != id {
		t.Errorf("expected ID %v, got %v", id, org.ID)
	}
	if org.Plan != "Pro" {
		t.Errorf("expected Plan Pro, got %s", org.Plan)
	}
}

func TestAPIKey(t *testing.T) {
	key := &APIKey{
		Name:      "test-key",
		IsActive:  true,
		KeyPrefix: "sk-test",
	}

	if !key.IsActive {
		t.Error("expected active key")
	}
	if key.KeyPrefix != "sk-test" {
		t.Errorf("expected prefix sk-test, got %s", key.KeyPrefix)
	}
}
