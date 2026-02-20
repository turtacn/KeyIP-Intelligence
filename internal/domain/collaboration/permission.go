// Package collaboration implements permission modeling for the workspace domain.
package collaboration

// ─────────────────────────────────────────────────────────────────────────────
// Permission — atomic authorization unit
// ─────────────────────────────────────────────────────────────────────────────

// Permission represents a single authorization grant: the ability to perform
// an Action on a Resource, optionally constrained by Conditions.
type Permission struct {
	// Action is the operation being permitted (e.g., "read", "write", "delete").
	Action string `json:"action"`

	// Resource is the entity type being acted upon (e.g., "patent", "portfolio").
	Resource string `json:"resource"`

	// Conditions is an optional map of attribute-based access control (ABAC)
	// constraints.  Example: {"owner": "self"} means the permission applies
	// only when the user is the resource owner.
	Conditions map[string]string `json:"conditions,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// PermissionSet — collection of permissions
// ─────────────────────────────────────────────────────────────────────────────

// PermissionSet is an ordered collection of Permission grants.
// It supports set operations like Contains and Merge.
type PermissionSet struct {
	Permissions []Permission `json:"permissions"`
}

// Contains returns true if this set includes a permission matching the given
// action and resource (ignoring conditions).
func (ps *PermissionSet) Contains(action, resource string) bool {
	for _, p := range ps.Permissions {
		if p.Action == action && p.Resource == resource {
			return true
		}
	}
	return false
}

// Merge combines this permission set with another, returning a new set that
// contains the union of both.  Duplicate permissions are retained (caller
// can deduplicate if needed).
func (ps *PermissionSet) Merge(other PermissionSet) PermissionSet {
	merged := append([]Permission{}, ps.Permissions...)
	merged = append(merged, other.Permissions...)
	return PermissionSet{Permissions: merged}
}

// ─────────────────────────────────────────────────────────────────────────────
// Predefined permission constants
// ─────────────────────────────────────────────────────────────────────────────

var (
	// PermPatentRead grants read access to patent details and documents.
	PermPatentRead = Permission{Action: "read", Resource: "patent"}

	// PermPatentWrite grants the ability to annotate or update patent metadata.
	PermPatentWrite = Permission{Action: "write", Resource: "patent"}

	// PermPatentDelete grants the ability to remove a patent from the workspace.
	PermPatentDelete = Permission{Action: "delete", Resource: "patent"}

	// PermPortfolioRead grants read access to portfolio analysis results.
	PermPortfolioRead = Permission{Action: "read", Resource: "portfolio"}

	// PermPortfolioWrite grants the ability to create or modify portfolios.
	PermPortfolioWrite = Permission{Action: "write", Resource: "portfolio"}

	// PermReportGenerate grants the ability to generate IP intelligence reports.
	PermReportGenerate = Permission{Action: "generate", Resource: "report"}

	// PermWorkspaceManage grants the ability to update workspace settings
	// (name, description, archiving).
	PermWorkspaceManage = Permission{Action: "manage", Resource: "workspace"}

	// PermMemberInvite grants the ability to invite new members to the workspace.
	PermMemberInvite = Permission{Action: "invite", Resource: "member"}

	// PermMemberRemove grants the ability to remove members from the workspace.
	PermMemberRemove = Permission{Action: "remove", Resource: "member"}
)

// ─────────────────────────────────────────────────────────────────────────────
// RolePermissions — RBAC mapping from roles to permission sets
// ─────────────────────────────────────────────────────────────────────────────

// RolePermissions maps each MemberRole to its granted PermissionSet.
// This implements a simple role-based access control (RBAC) model:
//
//   - Owner: all permissions
//   - Admin: all except delete
//   - Editor: read + write
//   - Viewer: read only
var RolePermissions = map[MemberRole]PermissionSet{
	RoleOwner: {
		Permissions: []Permission{
			PermPatentRead,
			PermPatentWrite,
			PermPatentDelete,
			PermPortfolioRead,
			PermPortfolioWrite,
			PermReportGenerate,
			PermWorkspaceManage,
			PermMemberInvite,
			PermMemberRemove,
		},
	},
	RoleAdmin: {
		Permissions: []Permission{
			PermPatentRead,
			PermPatentWrite,
			// Notably missing: PermPatentDelete
			PermPortfolioRead,
			PermPortfolioWrite,
			PermReportGenerate,
			PermWorkspaceManage,
			PermMemberInvite,
			PermMemberRemove,
		},
	},
	RoleEditor: {
		Permissions: []Permission{
			PermPatentRead,
			PermPatentWrite,
			PermPortfolioRead,
			PermPortfolioWrite,
			PermReportGenerate,
		},
	},
	RoleViewer: {
		Permissions: []Permission{
			PermPatentRead,
			PermPortfolioRead,
		},
	},
}

// ─────────────────────────────────────────────────────────────────────────────
// Authorization helper function
// ─────────────────────────────────────────────────────────────────────────────

// HasPermission checks whether a given role grants permission to perform
// the specified action on the specified resource.
// Returns true if the role's permission set contains a matching entry.
func HasPermission(role MemberRole, action, resource string) bool {
	ps, exists := RolePermissions[role]
	if !exists {
		return false
	}
	return ps.Contains(action, resource)
}

//Personal.AI order the ending
