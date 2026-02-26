package collaboration

import (
	"errors"
	"time"

	"github.com/google/uuid"
	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// WorkspaceStatus defines the status of a workspace.
type WorkspaceStatus string

const (
	WorkspaceStatusActive    WorkspaceStatus = "active"
	WorkspaceStatusSuspended WorkspaceStatus = "suspended"
	WorkspaceStatusArchived  WorkspaceStatus = "archived"
	WorkspaceStatusDeleted   WorkspaceStatus = "deleted"
)

// WorkspacePlan defines the subscription plan.
type WorkspacePlan string

const (
	PlanFree       WorkspacePlan = "free"
	PlanPro        WorkspacePlan = "pro"
	PlanEnterprise WorkspacePlan = "enterprise"
)

// WorkspaceSettings holds configuration for a workspace.
type WorkspaceSettings struct {
	DefaultCurrency          string   `json:"default_currency"`
	DefaultJurisdiction      string   `json:"default_jurisdiction"`
	AnnuityReminderDays      []int    `json:"annuity_reminder_days"`
	DeadlineReminderDays     []int    `json:"deadline_reminder_days"`
	EnableEmailNotifications bool     `json:"enable_email_notifications"`
	EnableInAppNotifications bool     `json:"enable_in_app_notifications"`
	MaxPortfolios            int      `json:"max_portfolios"` // -1 for unlimited
	MaxMembers               int      `json:"max_members"`    // -1 for unlimited
	AIFeaturesEnabled        bool     `json:"ai_features_enabled"`
	Language                 string   `json:"language"`
	Timezone                 string   `json:"timezone"`
}

// Workspace represents a collaboration environment.
type Workspace struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Slug         string            `json:"slug"`
	Description  string            `json:"description"`
	OwnerID      string            `json:"owner_id"`
	Status       WorkspaceStatus   `json:"status"`
	Plan         WorkspacePlan     `json:"plan"`
	Settings     WorkspaceSettings `json:"settings"`
	PortfolioIDs []string          `json:"portfolio_ids"`
	MemberCount  int               `json:"member_count"`
	LogoURL      string            `json:"logo_url"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// NewWorkspace creates a new workspace.
func NewWorkspace(name, ownerID string) (*Workspace, error) {
	if name == "" || len(name) > 100 {
		return nil, errors.New("invalid name")
	}
	if ownerID == "" {
		return nil, errors.New("ownerID required")
	}

	now := time.Now().UTC()
	return &Workspace{
		ID:          uuid.New().String(),
		Name:        name,
		Slug:        "",
		OwnerID:     ownerID,
		Status:      WorkspaceStatusActive,
		Plan:        PlanFree,
		Settings:    DefaultWorkspaceSettings(),
		MemberCount: 1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func DefaultWorkspaceSettings() WorkspaceSettings {
	return WorkspaceSettings{
		DefaultCurrency:          "USD",
		DefaultJurisdiction:      "US",
		AnnuityReminderDays:      []int{60, 30, 7},
		DeadlineReminderDays:     []int{30, 14, 7},
		EnableEmailNotifications: true,
		EnableInAppNotifications: true,
		MaxPortfolios:            5,
		MaxMembers:               3,
		AIFeaturesEnabled:        false,
		Language:                 "en",
		Timezone:                 "UTC",
	}
}

func PlanLimits(plan WorkspacePlan) WorkspaceSettings {
	s := DefaultWorkspaceSettings()
	switch plan {
	case PlanFree:
		s.MaxPortfolios = 5
		s.MaxMembers = 3
		s.AIFeaturesEnabled = false
	case PlanPro:
		s.MaxPortfolios = 50
		s.MaxMembers = 20
		s.AIFeaturesEnabled = true
	case PlanEnterprise:
		s.MaxPortfolios = -1
		s.MaxMembers = -1
		s.AIFeaturesEnabled = true
	}
	return s
}

func (w *Workspace) UpdateName(name string) error {
	if name == "" || len(name) > 100 {
		return apperrors.NewValidation("invalid name")
	}
	w.Name = name
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (w *Workspace) UpdateDescription(description string) error {
	if len(description) > 500 {
		return apperrors.NewValidation("description too long")
	}
	w.Description = description
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (w *Workspace) UpdateSettings(settings WorkspaceSettings) error {
	// Preserve plan limits
	settings.MaxPortfolios = w.Settings.MaxPortfolios
	settings.MaxMembers = w.Settings.MaxMembers
	settings.AIFeaturesEnabled = w.Settings.AIFeaturesEnabled

	w.Settings = settings
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (w *Workspace) UpgradePlan(plan WorkspacePlan) error {
	if w.Plan == PlanEnterprise && plan != PlanEnterprise {
		return apperrors.NewValidation("cannot downgrade from Enterprise")
	}
	if w.Plan == PlanPro && plan == PlanFree {
		return apperrors.NewValidation("cannot downgrade from Pro to Free")
	}

	w.Plan = plan
	limits := PlanLimits(plan)
	w.Settings.MaxPortfolios = limits.MaxPortfolios
	w.Settings.MaxMembers = limits.MaxMembers
	w.Settings.AIFeaturesEnabled = limits.AIFeaturesEnabled
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (w *Workspace) AddPortfolio(portfolioID string) error {
	for _, id := range w.PortfolioIDs {
		if id == portfolioID {
			return apperrors.NewValidation("portfolio already added")
		}
	}
	if !w.CanAddPortfolio() {
		return apperrors.NewValidation("portfolio limit reached")
	}
	w.PortfolioIDs = append(w.PortfolioIDs, portfolioID)
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (w *Workspace) RemovePortfolio(portfolioID string) error {
	for i, id := range w.PortfolioIDs {
		if id == portfolioID {
			w.PortfolioIDs = append(w.PortfolioIDs[:i], w.PortfolioIDs[i+1:]...)
			w.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return apperrors.NewNotFound("portfolio not found in workspace")
}

func (w *Workspace) IncrementMemberCount() {
	w.MemberCount++
	w.UpdatedAt = time.Now().UTC()
}

func (w *Workspace) DecrementMemberCount() {
	if w.MemberCount > 1 {
		w.MemberCount--
		w.UpdatedAt = time.Now().UTC()
	}
}

func (w *Workspace) CanAddMember() bool {
	if w.Settings.MaxMembers == -1 {
		return true
	}
	return w.MemberCount < w.Settings.MaxMembers
}

func (w *Workspace) CanAddPortfolio() bool {
	if w.Settings.MaxPortfolios == -1 {
		return true
	}
	return len(w.PortfolioIDs) < w.Settings.MaxPortfolios
}

func (w *Workspace) Suspend(reason string) error {
	if w.Status != WorkspaceStatusActive {
		return apperrors.NewValidation("workspace not active")
	}
	w.Status = WorkspaceStatusSuspended
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (w *Workspace) Reactivate() error {
	if w.Status != WorkspaceStatusSuspended {
		return apperrors.NewValidation("workspace not suspended")
	}
	w.Status = WorkspaceStatusActive
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (w *Workspace) Archive() error {
	if w.Status != WorkspaceStatusActive && w.Status != WorkspaceStatusSuspended {
		return apperrors.NewValidation("workspace cannot be archived")
	}
	w.Status = WorkspaceStatusArchived
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (w *Workspace) MarkDeleted() error {
	w.Status = WorkspaceStatusDeleted
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (w *Workspace) IsActive() bool {
	return w.Status == WorkspaceStatusActive
}

func (w *Workspace) TransferOwnership(newOwnerID string) error {
	if newOwnerID == "" || newOwnerID == w.OwnerID {
		return apperrors.NewValidation("invalid new owner")
	}
	w.OwnerID = newOwnerID
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (w *Workspace) Validate() error {
	if w.ID == "" || w.Name == "" || w.OwnerID == "" {
		return errors.New("invalid workspace")
	}
	if w.MemberCount < 1 {
		return errors.New("member count must be >= 1")
	}
	return nil
}

//Personal.AI order the ending
