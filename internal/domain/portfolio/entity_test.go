package portfolio

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

func TestPortfolio_Status(t *testing.T) {
	assert.Equal(t, "active", string(StatusActive))
	assert.Equal(t, "draft", string(StatusDraft))
	assert.Equal(t, "archived", string(StatusArchived))
}

func TestPortfolio_New(t *testing.T) {
	p, err := NewPortfolio("Test", "owner-1", []string{"OLED"})
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, StatusDraft, p.Status)
	assert.Equal(t, "Test", p.Name)
	assert.Equal(t, "owner-1", p.OwnerID)
}

func TestPortfolio_Validate(t *testing.T) {
	tests := []struct {
		name    string
		p       *Portfolio
		wantErr bool
	}{
		{
			name: "Valid Portfolio",
			p: &Portfolio{
				ID:      string(common.NewID()),
				Name:    "Valid Portfolio",
				OwnerID: "owner-1",
				Status:  StatusDraft,
			},
			wantErr: false,
		},
		{
			name: "Missing ID",
			p: &Portfolio{
				Name:    "Valid Portfolio",
				OwnerID: "owner-1",
				Status:  StatusDraft,
			},
			wantErr: true,
		},
		{
			name: "Missing Name",
			p: &Portfolio{
				ID:      string(common.NewID()),
				OwnerID: "owner-1",
				Status:  StatusDraft,
			},
			wantErr: true,
		},
		{
			name: "Name Too Long",
			p: &Portfolio{
				ID:      string(common.NewID()),
				Name:    strings.Repeat("a", 257),
				OwnerID: "owner-1",
				Status:  StatusDraft,
			},
			wantErr: true,
		},
		{
			name: "Missing OwnerID",
			p: &Portfolio{
				ID:     string(common.NewID()),
				Name:   "Valid Portfolio",
				Status: StatusDraft,
			},
			wantErr: true,
		},
		{
			name: "Invalid Status",
			p: &Portfolio{
				ID:      string(common.NewID()),
				Name:    "Valid Portfolio",
				OwnerID: "owner-1",
				Status:  "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPortfolio_Transitions(t *testing.T) {
	p, _ := NewPortfolio("Test", "owner-1", nil)

	// Draft -> Active
	err := p.Activate()
	assert.NoError(t, err)
	assert.Equal(t, StatusActive, p.Status)

	// Active -> Archived
	err = p.Archive()
	assert.NoError(t, err)
	assert.Equal(t, StatusArchived, p.Status)

	// Archived -> Active (Invalid)
	err = p.Activate()
	assert.Error(t, err)
	assert.True(t, errors.IsValidation(err))

	// Reset to Draft manually to test invalid jump
	p.Status = StatusDraft
	// Draft -> Archived (Invalid)
	err = p.Archive()
	assert.Error(t, err)
	assert.True(t, errors.IsValidation(err))
}

func TestPortfolio_Timestamps(t *testing.T) {
	// Use time.Time directly and allow for small differences due to execution time
	start := time.Now().UTC()
	p, _ := NewPortfolio("Test", "owner-1", nil)

	// Check CreatedAt is close to start
	diff := p.CreatedAt.Sub(start)
	if diff < 0 {
		diff = -diff
	}
	assert.Less(t, diff, time.Second)

	// Check UpdatedAt is close to start
	diff = p.UpdatedAt.Sub(start)
	if diff < 0 {
		diff = -diff
	}
	assert.Less(t, diff, time.Second)

	time.Sleep(10 * time.Millisecond)
	prevUpdated := p.UpdatedAt

	err := p.Activate()
	assert.NoError(t, err)
	assert.True(t, p.UpdatedAt.After(prevUpdated))
}

//Personal.AI order the ending
