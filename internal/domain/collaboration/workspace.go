package collaboration

import (
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// WorkspaceStatus defines the status of a workspace.
type WorkspaceStatus string

const (
	WorkspaceStatusActive    WorkspaceStatus = "active"
	WorkspaceStatusSuspended WorkspaceStatus = "suspended"
	WorkspaceStatusArchived  WorkspaceStatus = "archived"
	WorkspaceStatusDeleted   WorkspaceStatus = "deleted"
)

// WorkspacePlan defines the subscription plan of a workspace.
type WorkspacePlan string

const (
	PlanFree       WorkspacePlan = "free"
	PlanPro        WorkspacePlan = "pro"
	PlanEnterprise WorkspacePlan = "enterprise"
)

// WorkspaceSettings defines the settings of a workspace.
type WorkspaceSettings struct {
	DefaultCurrency          string   `json:"default_currency"`
	DefaultJurisdiction      string   `json:"default_jurisdiction"`
	AnnuityReminderDays      []int    `json:"annuity_reminder_days"`
	DeadlineReminderDays     []int    `json:"deadline_reminder_days"`
	EnableEmailNotifications bool     `json:"enable_email_notifications"`
	EnableInAppNotifications bool     `json:"enable_in_app_notifications"`
	MaxPortfolios            int      `json:"max_portfolios"`
	MaxMembers               int      `json:"max_members"`
	AIFeaturesEnabled        bool     `json:"ai_features_enabled"`
	Language                 string   `json:"language"`
	Timezone                 string   `json:"timezone"`
}

// Workspace is the aggregate root for a collaboration unit.
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

// DefaultWorkspaceSettings returns default settings for a free plan.
func DefaultWorkspaceSettings() WorkspaceSettings {
	return WorkspaceSettings{
		DefaultCurrency:          "USD",
		DefaultJurisdiction:      "US",
		AnnuityReminderDays:      []int{7, 30, 60},
		DeadlineReminderDays:     []int{7, 14, 30},
		EnableEmailNotifications: true,
		EnableInAppNotifications: true,
		MaxPortfolios:            5,
		MaxMembers:               3,
		AIFeaturesEnabled:        false,
		Language:                 "en",
		Timezone:                 "UTC",
	}
}

// PlanLimits returns settings for a specific plan.
func PlanLimits(plan WorkspacePlan) WorkspaceSettings {
	settings := DefaultWorkspaceSettings()
	switch plan {
	case PlanFree:
		settings.MaxPortfolios = 5
		settings.MaxMembers = 3
		settings.AIFeaturesEnabled = false
	case PlanPro:
		settings.MaxPortfolios = 50
		settings.MaxMembers = 20
		settings.AIFeaturesEnabled = true
	case PlanEnterprise:
		settings.MaxPortfolios = -1
		settings.MaxMembers = -1
		settings.AIFeaturesEnabled = true
	}
	return settings
}

// NewWorkspace creates a new workspace.
func NewWorkspace(name, ownerID string) (*Workspace, error) {
	if name == "" || len(name) > 100 {
		return nil, errors.InvalidParam("name must be between 1 and 100 characters")
	}
	if ownerID == "" {
		return nil, errors.InvalidParam("ownerID is required")
	}

	now := time.Now().UTC()
	return &Workspace{
		ID:          uuid.New().String(),
		Name:        name,
		OwnerID:     ownerID,
		Status:      WorkspaceStatusActive,
		Plan:        PlanFree,
		Settings:    DefaultWorkspaceSettings(),
		MemberCount: 1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// UpdateName updates the workspace name.
func (w *Workspace) UpdateName(name string) error {
	if name == "" || len(name) > 100 {
		return errors.InvalidParam("name must be between 1 and 100 characters")
	}
	w.Name = name
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateDescription updates the workspace description.
func (w *Workspace) UpdateDescription(description string) error {
	if len(description) > 500 {
		return errors.InvalidParam("description must be at most 500 characters")
	}
	w.Description = description
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateSettings updates the workspace settings.
func (w *Workspace) UpdateSettings(settings WorkspaceSettings) error {
	// Preserve plan-controlled limits
	settings.MaxPortfolios = w.Settings.MaxPortfolios
	settings.MaxMembers = w.Settings.MaxMembers
	settings.AIFeaturesEnabled = w.Settings.AIFeaturesEnabled

	w.Settings = settings
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// UpgradePlan upgrades the workspace plan.
func (w *Workspace) UpgradePlan(plan WorkspacePlan) error {
	planRank := map[WorkspacePlan]int{
		PlanFree:       0,
		PlanPro:        1,
		PlanEnterprise: 2,
	}

	currentRank, ok1 := planRank[w.Plan]
	newRank, ok2 := planRank[plan]

	if !ok2 {
		return errors.InvalidParam("invalid plan")
	}
	if !ok1 || newRank <= currentRank {
		return errors.InvalidParam("can only upgrade to a higher plan")
	}

	w.Plan = plan
	limits := PlanLimits(plan)
	w.Settings.MaxPortfolios = limits.MaxPortfolios
	w.Settings.MaxMembers = limits.MaxMembers
	w.Settings.AIFeaturesEnabled = limits.AIFeaturesEnabled
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// AddPortfolio adds a portfolio to the workspace.
func (w *Workspace) AddPortfolio(portfolioID string) error {
	if portfolioID == "" {
		return errors.InvalidParam("portfolioID is required")
	}
	for _, id := range w.PortfolioIDs {
		if id == portfolioID {
			return errors.Conflict("portfolio already added to workspace")
		}
	}
	if w.Settings.MaxPortfolios != -1 && len(w.PortfolioIDs) >= w.Settings.MaxPortfolios {
		return errors.Forbidden("maximum number of portfolios reached for current plan")
	}
	w.PortfolioIDs = append(w.PortfolioIDs, portfolioID)
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// RemovePortfolio removes a portfolio from the workspace.
func (w *Workspace) RemovePortfolio(portfolioID string) error {
	for i, id := range w.PortfolioIDs {
		if id == portfolioID {
			w.PortfolioIDs = append(w.PortfolioIDs[:i], w.PortfolioIDs[i+1:]...)
			w.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return errors.NotFound("portfolio not found in workspace")
}

// IncrementMemberCount increments the member count.
func (w *Workspace) IncrementMemberCount() {
	w.MemberCount++
	w.UpdatedAt = time.Now().UTC()
}

// DecrementMemberCount decrements the member count.
func (w *Workspace) DecrementMemberCount() {
	if w.MemberCount > 1 {
		w.MemberCount--
		w.UpdatedAt = time.Now().UTC()
	}
}

// CanAddMember checks if a member can be added to the workspace.
func (w *Workspace) CanAddMember() bool {
	if w.Settings.MaxMembers == -1 {
		return true
	}
	return w.MemberCount < w.Settings.MaxMembers
}

// CanAddPortfolio checks if a portfolio can be added to the workspace.
func (w *Workspace) CanAddPortfolio() bool {
	if w.Settings.MaxPortfolios == -1 {
		return true
	}
	return len(w.PortfolioIDs) < w.Settings.MaxPortfolios
}

// Suspend suspends the workspace.
func (w *Workspace) Suspend(reason string) error {
	if w.Status != WorkspaceStatusActive {
		return errors.InvalidState("only active workspaces can be suspended")
	}
	w.Status = WorkspaceStatusSuspended
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// Reactivate reactivates the workspace.
func (w *Workspace) Reactivate() error {
	if w.Status != WorkspaceStatusSuspended {
		return errors.InvalidState("only suspended workspaces can be reactivated")
	}
	w.Status = WorkspaceStatusActive
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// Archive archives the workspace.
func (w *Workspace) Archive() error {
	if w.Status != WorkspaceStatusActive && w.Status != WorkspaceStatusSuspended {
		return errors.InvalidState("only active or suspended workspaces can be archived")
	}
	w.Status = WorkspaceStatusArchived
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// MarkDeleted marks the workspace as deleted.
func (w *Workspace) MarkDeleted() error {
	w.Status = WorkspaceStatusDeleted
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// IsActive checks if the workspace is active.
func (w *Workspace) IsActive() bool {
	return w.Status == WorkspaceStatusActive
}

// TransferOwnership transfers the workspace ownership.
func (w *Workspace) TransferOwnership(newOwnerID string) error {
	if newOwnerID == "" {
		return errors.InvalidParam("newOwnerID is required")
	}
	if newOwnerID == w.OwnerID {
		return errors.InvalidParam("cannot transfer ownership to current owner")
	}
	w.OwnerID = newOwnerID
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// Validate validates the workspace.
func (w *Workspace) Validate() error {
	if w.ID == "" || w.Name == "" || w.Slug == "" || w.OwnerID == "" {
		return errors.InvalidParam("ID, Name, Slug, and OwnerID are required")
	}
	if w.MemberCount < 1 {
		return errors.InvalidParam("MemberCount must be at least 1")
	}
	validStatus := map[WorkspaceStatus]bool{
		WorkspaceStatusActive: true, WorkspaceStatusSuspended: true,
		WorkspaceStatusArchived: true, WorkspaceStatusDeleted: true,
	}
	if !validStatus[w.Status] {
		return errors.InvalidParam("invalid status")
	}
	validPlan := map[WorkspacePlan]bool{
		PlanFree: true, PlanPro: true, PlanEnterprise: true,
	}
	if !validPlan[w.Plan] {
		return errors.InvalidParam("invalid plan")
	}
	return nil
}

//Personal.AI order the ending
