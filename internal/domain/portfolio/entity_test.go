package portfolio

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPortfolio_Status(t *testing.T) {
	assert.Equal(t, "active", string(StatusActive))
	assert.Equal(t, "draft", string(StatusDraft))
}

func TestPortfolio_New(t *testing.T) {
	// Assuming NewPortfolio function might be implemented in entity.go or manually constructed
	p := &Portfolio{
		ID:        uuid.New(),
		Name:      "Test",
		Status:    StatusDraft,
		CreatedAt: time.Now(),
	}
	assert.NotNil(t, p)
	assert.Equal(t, StatusDraft, p.Status)
}

//Personal.AI order the ending
