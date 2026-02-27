package testutil

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
)

type BasePatentRepoMock struct{}

func (BasePatentRepoMock) Create(ctx context.Context, p *patent.Patent) error { return nil }
func (BasePatentRepoMock) Update(ctx context.Context, p *patent.Patent) error { return nil }
func (BasePatentRepoMock) Delete(ctx context.Context, id string) error        { return nil }
func (BasePatentRepoMock) SoftDelete(ctx context.Context, id uuid.UUID) error { return nil }
func (BasePatentRepoMock) Restore(ctx context.Context, id uuid.UUID) error    { return nil }
func (BasePatentRepoMock) HardDelete(ctx context.Context, id uuid.UUID) error { return nil }

func (BasePatentRepoMock) Search(ctx context.Context, c patent.PatentSearchCriteria) (*patent.PatentSearchResult, error) {
	return &patent.PatentSearchResult{Patents: []*patent.Patent{}, Total: 0, Offset: c.Offset, Limit: c.Limit, HasMore: false}, nil
}

func (BasePatentRepoMock) FindByApplicant(ctx context.Context, applicantName string) ([]*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) SearchByAssigneeName(ctx context.Context, name string, limit, offset int) ([]*patent.Patent, int64, error) {
	return nil, 0, nil
}

func (BasePatentRepoMock) FindByFamilyID(ctx context.Context, familyID string) ([]*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) GetByFamilyID(ctx context.Context, familyID string) ([]*patent.Patent, error) {
	return nil, nil
}

func (BasePatentRepoMock) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) AssociateMolecule(ctx context.Context, patentID string, moleculeID string) error {
	return nil
}

func (BasePatentRepoMock) CountByIPCSection(ctx context.Context) (map[string]int64, error) {
	return map[string]int64{}, nil
}
func (BasePatentRepoMock) CountByJurisdiction(ctx context.Context) (map[string]int64, error) {
	return map[string]int64{}, nil
}
func (BasePatentRepoMock) CountByOffice(ctx context.Context) (map[patent.PatentOffice]int64, error) {
	return map[patent.PatentOffice]int64{}, nil
}
func (BasePatentRepoMock) CountByStatus(ctx context.Context) (map[patent.PatentStatus]int64, error) {
	return map[patent.PatentStatus]int64{}, nil
}
func (BasePatentRepoMock) CountByYear(ctx context.Context, field string) (map[int]int64, error) {
	return map[int]int64{}, nil
}
func (BasePatentRepoMock) GetIPCDistribution(ctx context.Context, level int) (map[string]int64, error) {
	return map[string]int64{}, nil
}

func (BasePatentRepoMock) Save(ctx context.Context, patent *patent.Patent) error { return nil }
func (BasePatentRepoMock) FindByID(ctx context.Context, id string) (*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) FindByPatentNumber(ctx context.Context, patentNumber string) (*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) Exists(ctx context.Context, patentNumber string) (bool, error) {
	return false, nil
}
func (BasePatentRepoMock) SaveBatch(ctx context.Context, patents []*patent.Patent) error { return nil }
func (BasePatentRepoMock) FindByIDs(ctx context.Context, ids []string) ([]*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) FindByPatentNumbers(ctx context.Context, numbers []string) ([]*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) FindByIPCCode(ctx context.Context, ipcCode string) ([]*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) FindCitedBy(ctx context.Context, patentNumber string) ([]*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) FindCiting(ctx context.Context, patentNumber string) ([]*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) FindExpiringBefore(ctx context.Context, date time.Time) ([]*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) FindActiveByIPCCode(ctx context.Context, ipcCode string) ([]*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) FindWithMarkushStructures(ctx context.Context, offset, limit int) ([]*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) GetByID(ctx context.Context, id uuid.UUID) (*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) GetByPatentNumber(ctx context.Context, number string) (*patent.Patent, error) {
	return nil, nil
}
func (BasePatentRepoMock) ListByPortfolio(ctx context.Context, portfolioID string) ([]*patent.Patent, error) {
	return nil, nil
}
