package family

import (
	"context"
)

type FamilyRepository interface {
	EnsureFamilyNode(ctx context.Context, familyID string, familyType string, metadata map[string]interface{}) error
	GetFamily(ctx context.Context, familyID string) (*FamilyAggregate, error)
	DeleteFamily(ctx context.Context, familyID string) error
	ListFamilies(ctx context.Context, familyType *string, limit, offset int) ([]*FamilyAggregate, int64, error)

	AddMember(ctx context.Context, familyID string, patentID string, memberRole string) error
	RemoveMember(ctx context.Context, familyID string, patentID string) error
	GetMembers(ctx context.Context, familyID string, memberRole *string) ([]*FamilyMember, error)
	GetFamilyByPatent(ctx context.Context, patentID string) ([]*FamilyAggregate, error)
	BatchAddMembers(ctx context.Context, familyID string, members []*FamilyMemberInput) error

	CreatePriorityLink(ctx context.Context, fromPatentID, toPatentID string, priorityDate string, priorityCountry string) error
	GetPriorityChain(ctx context.Context, patentID string) ([]*PriorityLink, error)
	GetDerivedPatents(ctx context.Context, patentID string) ([]*PriorityLink, error)

	GetFamilyStats(ctx context.Context, familyID string) (*FamilyStats, error)
	GetFamilyCoverage(ctx context.Context, familyID string) (map[string]int64, error)
	FindRelatedFamilies(ctx context.Context, familyID string, maxDistance int) ([]*RelatedFamily, error)
}

//Personal.AI order the ending
