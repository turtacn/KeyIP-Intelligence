// File: internal/application/portfolio/common_test.go
package portfolio

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	domainmol "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domainportfolio "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/molpatent_gnn"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// ... (Rest of file content with fixes)

// --- Patent Repository ---
type mockPatentRepo struct {
	patents     map[string]*patent.Patent
	byPortfolio map[string][]*patent.Patent
	byAssignee  map[string][]*patent.Patent
	byIDs       map[string]*patent.Patent
	err         error
}
func newMockPatentRepo() *mockPatentRepo {
	return &mockPatentRepo{
		patents:     make(map[string]*patent.Patent),
		byPortfolio: make(map[string][]*patent.Patent),
		byAssignee:  make(map[string][]*patent.Patent),
		byIDs:       make(map[string]*patent.Patent),
	}
}
func (m *mockPatentRepo) FindByID(ctx context.Context, id string) (*patent.Patent, error) {
	if m.err != nil { return nil, m.err }
	if p, ok := m.patents[id]; ok { return p, nil }
	if p, ok := m.byIDs[id]; ok { return p, nil }
	return nil, fmt.Errorf("patent %s not found", id)
}
func (m *mockPatentRepo) Save(ctx context.Context, p *patent.Patent) error {
	if m.err != nil { return m.err }
	m.patents[p.ID] = p
	m.byIDs[p.ID] = p
	return nil
}
func (m *mockPatentRepo) Update(ctx context.Context, p *patent.Patent) error { return m.err }
func (m *mockPatentRepo) Delete(ctx context.Context, id string) error        { return m.err }
func (m *mockPatentRepo) FindByIDs(ctx context.Context, ids []string) ([]*patent.Patent, error) {
	if m.err != nil { return nil, m.err }
	var result []*patent.Patent
	for _, id := range ids {
		if p, ok := m.patents[id]; ok { result = append(result, p) } else if p, ok := m.byIDs[id]; ok { result = append(result, p) }
	}
	return result, nil
}
func (m *mockPatentRepo) AssociateMolecule(ctx context.Context, patentID, moleculeID string) error { return m.err }
func (m *mockPatentRepo) ListByPortfolio(ctx context.Context, portfolioID string) ([]*patent.Patent, error) {
	if m.err != nil { return nil, m.err }
	if patents, ok := m.byPortfolio[portfolioID]; ok { return patents, nil }
	return nil, nil
}
func (m *mockPatentRepo) BatchCreate(ctx context.Context, patents []*patent.Patent) (int, error) {
	if m.err != nil { return 0, m.err }
	return len(patents), nil
}
func (m *mockPatentRepo) BatchCreateClaims(ctx context.Context, claims []*patent.Claim) error { return m.err }
func (m *mockPatentRepo) BatchUpdateStatus(ctx context.Context, ids []uuid.UUID, status patent.PatentStatus) (int64, error) {
	if m.err != nil { return 0, m.err }
	return int64(len(ids)), nil
}
func (m *mockPatentRepo) Create(ctx context.Context, p *patent.Patent) error { return m.err }
func (m *mockPatentRepo) GetByID(ctx context.Context, id uuid.UUID) (*patent.Patent, error) { return m.FindByID(ctx, id.String()) }
func (m *mockPatentRepo) GetByPatentNumber(ctx context.Context, number string) (*patent.Patent, error) {
	if m.err != nil { return nil, m.err }
	for _, p := range m.patents { if p.PatentNumber == number { return p, nil } }
	return nil, fmt.Errorf("patent not found")
}
func (m *mockPatentRepo) FindByPatentNumber(ctx context.Context, number string) (*patent.Patent, error) { return m.GetByPatentNumber(ctx, number) }
func (m *mockPatentRepo) SoftDelete(ctx context.Context, id uuid.UUID) error  { return m.err }
func (m *mockPatentRepo) Restore(ctx context.Context, id uuid.UUID) error     { return m.err }
func (m *mockPatentRepo) HardDelete(ctx context.Context, id uuid.UUID) error  { return m.err }
func (m *mockPatentRepo) Exists(ctx context.Context, patentNumber string) (bool, error) { return false, m.err }
func (m *mockPatentRepo) SaveBatch(ctx context.Context, patents []*patent.Patent) error { return m.err }
func (m *mockPatentRepo) FindByPatentNumbers(ctx context.Context, numbers []string) ([]*patent.Patent, error) { return nil, m.err }
func (m *mockPatentRepo) Search(ctx context.Context, query patent.PatentSearchCriteria) (*patent.PatentSearchResult, error) {
	if len(query.ApplicantNames) > 0 {
		name := query.ApplicantNames[0]
		if patents, ok := m.byAssignee[name]; ok { return &patent.PatentSearchResult{Patents: patents, Total: int64(len(patents))}, nil }
	}
	return &patent.PatentSearchResult{}, m.err
}
func (m *mockPatentRepo) SearchBySimilarity(ctx context.Context, req *patent.SimilaritySearchRequest) ([]*patent.PatentSearchResultWithSimilarity, error) { return nil, m.err }
func (m *mockPatentRepo) GetByFamilyID(ctx context.Context, familyID string) ([]*patent.Patent, error) { return nil, m.err }
func (m *mockPatentRepo) FindByFamilyID(ctx context.Context, familyID string) ([]*patent.Patent, error) { return nil, m.err }
func (m *mockPatentRepo) GetByAssignee(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*patent.Patent, int64, error) {
	if m.err != nil { return nil, 0, m.err }
	if patents, ok := m.byAssignee[assigneeID.String()]; ok { return patents, int64(len(patents)), nil }
	return nil, 0, nil
}
func (m *mockPatentRepo) GetByJurisdiction(ctx context.Context, jurisdiction string, limit, offset int) ([]*patent.Patent, int64, error) { return nil, 0, m.err }
func (m *mockPatentRepo) GetExpiringPatents(ctx context.Context, daysAhead int, limit, offset int) ([]*patent.Patent, int64, error) { return nil, 0, m.err }
func (m *mockPatentRepo) FindExpiringBefore(ctx context.Context, date time.Time) ([]*patent.Patent, error) { return nil, m.err }
func (m *mockPatentRepo) FindDuplicates(ctx context.Context, fullTextHash string) ([]*patent.Patent, error) { return nil, m.err }
func (m *mockPatentRepo) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*patent.Patent, error) { return nil, m.err }
func (m *mockPatentRepo) FindByIPCCode(ctx context.Context, ipcCode string) ([]*patent.Patent, error) { return nil, m.err }
func (m *mockPatentRepo) FindByApplicant(ctx context.Context, applicantName string) ([]*patent.Patent, error) { return nil, m.err }
func (m *mockPatentRepo) FindCitedBy(ctx context.Context, patentNumber string) ([]*patent.Patent, error) { return nil, m.err }
func (m *mockPatentRepo) FindCiting(ctx context.Context, patentNumber string) ([]*patent.Patent, error) { return nil, m.err }
func (m *mockPatentRepo) FindActiveByIPCCode(ctx context.Context, ipcCode string) ([]*patent.Patent, error) { return nil, m.err }
func (m *mockPatentRepo) FindWithMarkushStructures(ctx context.Context, offset, limit int) ([]*patent.Patent, error) { return nil, m.err }
func (m *mockPatentRepo) CreateClaim(ctx context.Context, claim *patent.Claim) error { return m.err }
func (m *mockPatentRepo) GetClaimsByPatent(ctx context.Context, patentID uuid.UUID) ([]*patent.Claim, error) { return nil, m.err }
func (m *mockPatentRepo) UpdateClaim(ctx context.Context, claim *patent.Claim) error { return m.err }
func (m *mockPatentRepo) DeleteClaimsByPatent(ctx context.Context, patentID uuid.UUID) error { return m.err }
func (m *mockPatentRepo) GetIndependentClaims(ctx context.Context, patentID uuid.UUID) ([]*patent.Claim, error) { return nil, m.err }
func (m *mockPatentRepo) SetInventors(ctx context.Context, patentID uuid.UUID, inventors []*patent.Inventor) error { return m.err }
func (m *mockPatentRepo) GetInventors(ctx context.Context, patentID uuid.UUID) ([]*patent.Inventor, error) { return nil, m.err }
func (m *mockPatentRepo) SearchByInventor(ctx context.Context, inventorName string, limit, offset int) ([]*patent.Patent, int64, error) { return nil, 0, m.err }
func (m *mockPatentRepo) SearchByAssigneeName(ctx context.Context, assigneeName string, limit, offset int) ([]*patent.Patent, int64, error) {
	if m.err != nil { return nil, 0, m.err }
	if patents, ok := m.byAssignee[assigneeName]; ok { return patents, int64(len(patents)), nil }
	return nil, 0, nil
}
func (m *mockPatentRepo) CountByStatus(ctx context.Context) (map[patent.PatentStatus]int64, error) { return nil, m.err }
func (m *mockPatentRepo) CountByJurisdiction(ctx context.Context) (map[string]int64, error) { return nil, m.err }
func (m *mockPatentRepo) CountByOffice(ctx context.Context) (map[patent.PatentOffice]int64, error) { return nil, m.err }
func (m *mockPatentRepo) CountByYear(ctx context.Context, field string) (map[int]int64, error) { return nil, m.err }
func (m *mockPatentRepo) GetIPCDistribution(ctx context.Context, level int) (map[string]int64, error) { return nil, m.err }
func (m *mockPatentRepo) CountByIPCSection(ctx context.Context) (map[string]int64, error) { return nil, m.err }
func (m *mockPatentRepo) WithTx(ctx context.Context, fn func(patent.PatentRepository) error) error { return fn(m) }

// --- Portfolio Repository ---
type mockPortfolioRepo struct {
	portfolios map[string]*domainportfolio.Portfolio
	patents    map[string][]*patent.Patent
	err        error
}
func newMockPortfolioRepo() *mockPortfolioRepo {
	return &mockPortfolioRepo{
		portfolios: make(map[string]*domainportfolio.Portfolio),
		patents:    make(map[string][]*patent.Patent),
	}
}
func (m *mockPortfolioRepo) FindByID(ctx context.Context, id string) (*domainportfolio.Portfolio, error) {
	if m.err != nil { return nil, m.err }
	p, ok := m.portfolios[id]
	if !ok { return nil, fmt.Errorf("portfolio %s not found", id) }
	return p, nil
}
func (m *mockPortfolioRepo) Save(ctx context.Context, p *domainportfolio.Portfolio) error { return m.err }
func (m *mockPortfolioRepo) Update(ctx context.Context, p *domainportfolio.Portfolio) error { return m.err }
func (m *mockPortfolioRepo) Delete(ctx context.Context, id string) error { return m.err }
func (m *mockPortfolioRepo) FindByOwner(ctx context.Context, ownerID string) ([]*domainportfolio.Portfolio, error) { return nil, nil }
func (m *mockPortfolioRepo) Create(ctx context.Context, p *domainportfolio.Portfolio) error { return m.err }
func (m *mockPortfolioRepo) GetByID(ctx context.Context, id uuid.UUID) (*domainportfolio.Portfolio, error) { return m.FindByID(ctx, id.String()) }
func (m *mockPortfolioRepo) SoftDelete(ctx context.Context, id uuid.UUID) error { return m.err }
func (m *mockPortfolioRepo) List(ctx context.Context, ownerID uuid.UUID, status *domainportfolio.Status, limit, offset int) ([]*domainportfolio.Portfolio, int64, error) { return nil, 0, m.err }
func (m *mockPortfolioRepo) GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*domainportfolio.Portfolio, error) { return m.FindByOwner(ctx, ownerID.String()) }
func (m *mockPortfolioRepo) AddPatent(ctx context.Context, portfolioID, patentID uuid.UUID, role string, addedBy uuid.UUID) error { return m.err }
func (m *mockPortfolioRepo) RemovePatent(ctx context.Context, portfolioID, patentID uuid.UUID) error { return m.err }
func (m *mockPortfolioRepo) GetPatents(ctx context.Context, portfolioID uuid.UUID, role *string, limit, offset int) ([]*patent.Patent, int64, error) {
	if m.err != nil { return nil, 0, m.err }
	if patents, ok := m.patents[portfolioID.String()]; ok { return patents, int64(len(patents)), nil }
	return []*patent.Patent{}, 0, nil
}
func (m *mockPortfolioRepo) IsPatentInPortfolio(ctx context.Context, portfolioID, patentID uuid.UUID) (bool, error) { return false, m.err }
func (m *mockPortfolioRepo) BatchAddPatents(ctx context.Context, portfolioID uuid.UUID, patentIDs []uuid.UUID, role string, addedBy uuid.UUID) error { return m.err }
func (m *mockPortfolioRepo) GetPortfoliosByPatent(ctx context.Context, patentID uuid.UUID) ([]*domainportfolio.Portfolio, error) { return nil, m.err }
func (m *mockPortfolioRepo) CreateValuation(ctx context.Context, v *domainportfolio.Valuation) error { return m.err }
func (m *mockPortfolioRepo) GetLatestValuation(ctx context.Context, patentID uuid.UUID) (*domainportfolio.Valuation, error) { return nil, m.err }
func (m *mockPortfolioRepo) GetValuationHistory(ctx context.Context, patentID uuid.UUID, limit int) ([]*domainportfolio.Valuation, error) { return nil, m.err }
func (m *mockPortfolioRepo) BatchCreateValuations(ctx context.Context, valuations []*domainportfolio.Valuation) error { return m.err }
func (m *mockPortfolioRepo) GetValuationsByPortfolio(ctx context.Context, portfolioID uuid.UUID) ([]*domainportfolio.Valuation, error) { return nil, m.err }
func (m *mockPortfolioRepo) GetValuationDistribution(ctx context.Context, portfolioID uuid.UUID) (map[domainportfolio.ValuationTier]int64, error) { return nil, m.err }
func (m *mockPortfolioRepo) CreateHealthScore(ctx context.Context, score *domainportfolio.HealthScore) error { return m.err }
func (m *mockPortfolioRepo) GetLatestHealthScore(ctx context.Context, portfolioID uuid.UUID) (*domainportfolio.HealthScore, error) { return nil, m.err }
func (m *mockPortfolioRepo) GetHealthScoreHistory(ctx context.Context, portfolioID uuid.UUID, limit int) ([]*domainportfolio.HealthScore, error) { return nil, m.err }
func (m *mockPortfolioRepo) GetHealthScoreTrend(ctx context.Context, portfolioID uuid.UUID, startDate, endDate time.Time) ([]*domainportfolio.HealthScore, error) { return nil, m.err }
func (m *mockPortfolioRepo) CreateSuggestion(ctx context.Context, s *domainportfolio.OptimizationSuggestion) error { return m.err }
func (m *mockPortfolioRepo) GetSuggestions(ctx context.Context, portfolioID uuid.UUID, status *string, limit, offset int) ([]*domainportfolio.OptimizationSuggestion, int64, error) { return nil, 0, m.err }
func (m *mockPortfolioRepo) UpdateSuggestionStatus(ctx context.Context, id uuid.UUID, status string, resolvedBy uuid.UUID) error { return m.err }
func (m *mockPortfolioRepo) GetPendingSuggestionCount(ctx context.Context, portfolioID uuid.UUID) (int64, error) { return 0, m.err }
func (m *mockPortfolioRepo) GetPortfolioSummary(ctx context.Context, portfolioID uuid.UUID) (*domainportfolio.Summary, error) { return nil, m.err }
func (m *mockPortfolioRepo) GetJurisdictionCoverage(ctx context.Context, portfolioID uuid.UUID) (map[string]int64, error) { return nil, m.err }
func (m *mockPortfolioRepo) GetTechDomainCoverage(ctx context.Context, portfolioID uuid.UUID) (map[string]int64, error) { return nil, m.err }
func (m *mockPortfolioRepo) ComparePortfolios(ctx context.Context, portfolioIDs []uuid.UUID) ([]*domainportfolio.ComparisonResult, error) { return nil, m.err }
func (m *mockPortfolioRepo) GetExpiryTimeline(ctx context.Context, portfolioID uuid.UUID) ([]*domainportfolio.ExpiryTimelineEntry, error) { return nil, m.err }
func (m *mockPortfolioRepo) WithTx(ctx context.Context, fn func(domainportfolio.PortfolioRepository) error) error { return fn(m) }

// --- Other Mocks ---
type mockMoleculeRepo struct {
	molecules map[string]*domainmol.Molecule
	findErr   error
}
func newMockMoleculeRepo() *mockMoleculeRepo { return &mockMoleculeRepo{molecules: make(map[string]*domainmol.Molecule)} }
func (m *mockMoleculeRepo) FindByIDs(ctx context.Context, ids []string) ([]*domainmol.Molecule, error) {
	if m.findErr != nil { return nil, m.findErr }
	result := make([]*domainmol.Molecule, 0, len(ids))
	for _, id := range ids {
		if mol, ok := m.molecules[id]; ok { result = append(result, mol) }
	}
	return result, nil
}
func (m *mockMoleculeRepo) Save(ctx context.Context, mol *domainmol.Molecule) error { return nil }
func (m *mockMoleculeRepo) Update(ctx context.Context, molecule *domainmol.Molecule) error { return nil }
func (m *mockMoleculeRepo) Delete(ctx context.Context, id string) error { return nil }
func (m *mockMoleculeRepo) BatchSave(ctx context.Context, molecules []*domainmol.Molecule) (int, error) { return len(molecules), nil }
func (m *mockMoleculeRepo) FindByID(ctx context.Context, id string) (*domainmol.Molecule, error) {
	if mol, ok := m.molecules[id]; ok { return mol, nil }
	return nil, fmt.Errorf("not found")
}
func (m *mockMoleculeRepo) FindByInChIKey(ctx context.Context, inchiKey string) (*domainmol.Molecule, error) { return nil, nil }
func (m *mockMoleculeRepo) FindBySMILES(ctx context.Context, smiles string) ([]*domainmol.Molecule, error) { return nil, nil }
func (m *mockMoleculeRepo) Exists(ctx context.Context, id string) (bool, error) { _, ok := m.molecules[id]; return ok, nil }
func (m *mockMoleculeRepo) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) { return false, nil }
func (m *mockMoleculeRepo) Search(ctx context.Context, query *domainmol.MoleculeQuery) (*domainmol.MoleculeSearchResult, error) { return nil, nil }
func (m *mockMoleculeRepo) Count(ctx context.Context, query *domainmol.MoleculeQuery) (int64, error) { return 0, nil }
func (m *mockMoleculeRepo) FindBySource(ctx context.Context, source domainmol.MoleculeSource, offset, limit int) ([]*domainmol.Molecule, error) { return nil, nil }
func (m *mockMoleculeRepo) FindByStatus(ctx context.Context, status domainmol.MoleculeStatus, offset, limit int) ([]*domainmol.Molecule, error) { return nil, nil }
func (m *mockMoleculeRepo) FindByTags(ctx context.Context, tags []string, offset, limit int) ([]*domainmol.Molecule, error) { return nil, nil }
func (m *mockMoleculeRepo) FindByMolecularWeightRange(ctx context.Context, minWeight, maxWeight float64, offset, limit int) ([]*domainmol.Molecule, error) { return nil, nil }
func (m *mockMoleculeRepo) FindWithFingerprint(ctx context.Context, fpType domainmol.FingerprintType, offset, limit int) ([]*domainmol.Molecule, error) { return nil, nil }
func (m *mockMoleculeRepo) FindWithoutFingerprint(ctx context.Context, fpType domainmol.FingerprintType, offset, limit int) ([]*domainmol.Molecule, error) { return nil, nil }

type mockAssessmentRepo struct {
	records    map[string]*AssessmentRecord
	byPatent   map[string][]*AssessmentRecord
	byPortfolio map[string][]*AssessmentRecord
	err        error
}
func newMockAssessmentRepo() *mockAssessmentRepo {
	return &mockAssessmentRepo{
		records:     make(map[string]*AssessmentRecord),
		byPatent:    make(map[string][]*AssessmentRecord),
		byPortfolio: make(map[string][]*AssessmentRecord),
	}
}
func (m *mockAssessmentRepo) Save(ctx context.Context, record *AssessmentRecord) error {
	if m.err != nil { return m.err }
	m.records[record.ID] = record
	m.byPatent[record.PatentID] = append(m.byPatent[record.PatentID], record)
	if record.PortfolioID != "" {
		m.byPortfolio[record.PortfolioID] = append(m.byPortfolio[record.PortfolioID], record)
	}
	return nil
}
func (m *mockAssessmentRepo) FindByID(ctx context.Context, id string) (*AssessmentRecord, error) {
	if m.err != nil { return nil, m.err }
	r, ok := m.records[id]
	if !ok { return nil, fmt.Errorf("assessment %s not found", id) }
	return r, nil
}
func (m *mockAssessmentRepo) FindByPatentID(ctx context.Context, patentID string, limit, offset int) ([]*AssessmentRecord, error) {
	if m.err != nil { return nil, m.err }
	recs := m.byPatent[patentID]
	if offset >= len(recs) { return nil, nil }
	end := offset + limit
	if end > len(recs) { end = len(recs) }
	return recs[offset:end], nil
}
func (m *mockAssessmentRepo) FindByPortfolioID(ctx context.Context, portfolioID string) ([]*AssessmentRecord, error) {
	if m.err != nil { return nil, m.err }
	return m.byPortfolio[portfolioID], nil
}
func (m *mockAssessmentRepo) FindByIDs(ctx context.Context, ids []string) ([]*AssessmentRecord, error) {
	if m.err != nil { return nil, m.err }
	var result []*AssessmentRecord
	for _, id := range ids {
		if r, ok := m.records[id]; ok { result = append(result, r) }
	}
	return result, nil
}

type mockAIScorer struct {
	scores map[AssessmentDimension]map[string]float64
	err    error
	calls  int32
}
func newMockAIScorer() *mockAIScorer {
	return &mockAIScorer{scores: make(map[AssessmentDimension]map[string]float64)}
}
func (m *mockAIScorer) ScorePatent(ctx context.Context, pat *patent.Patent, dim AssessmentDimension) (map[string]float64, error) {
	atomic.AddInt32(&m.calls, 1)
	if m.err != nil { return nil, m.err }
	if s, ok := m.scores[dim]; ok { return s, nil }
	return nil, fmt.Errorf("no AI scores configured for dimension %s", dim)
}

type mockCitationRepo struct {
	forwardCounts map[string]int
	maxInDomain   map[string]int
	err           error
}
func newMockCitationRepo() *mockCitationRepo {
	return &mockCitationRepo{forwardCounts: make(map[string]int), maxInDomain: make(map[string]int)}
}
func (m *mockCitationRepo) CountForwardCitations(ctx context.Context, patentID string) (int, error) {
	if m.err != nil { return 0, m.err }
	return m.forwardCounts[patentID], nil
}
func (m *mockCitationRepo) CountBackwardCitations(ctx context.Context, patentID string) (int, error) { return 0, nil }
func (m *mockCitationRepo) MaxForwardCitationsInDomain(ctx context.Context, domain string) (int, error) {
	if m.err != nil { return 0, m.err }
	return m.maxInDomain[domain], nil
}

type mockCache struct {
	store  map[string][]byte
	getErr error
	setErr error
	err    error
	getCalls int
	setCalls int
}
func newMockCache() *mockCache { return &mockCache{store: make(map[string][]byte)} }
func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) {
	m.getCalls++
	if m.getErr != nil { return nil, m.getErr }
	if m.err != nil { return nil, m.err }
	v, ok := m.store[key]
	if !ok { return nil, fmt.Errorf("cache miss") }
	return v, nil
}
func (m *mockCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.setCalls++
	if m.setErr != nil { return m.setErr }
	if m.err != nil { return m.err }
	m.store[key] = value
	return nil
}
func (m *mockCache) Delete(ctx context.Context, key string) error { delete(m.store, key); return nil }
func (m *mockCache) GetInterface(ctx context.Context, key string, dest interface{}) error {
	_, err := m.Get(ctx, key)
	return err
}
func (m *mockCache) SetInterface(ctx context.Context, key string, value interface{}, ttl time.Duration) error { return m.Set(ctx, key, nil, ttl) }

type mockLogger struct{}
func (mockLogger) Debug(msg string, fields ...logging.Field) {}
func (mockLogger) Info(msg string, fields ...logging.Field)  {}
func (mockLogger) Warn(msg string, fields ...logging.Field)  {}
func (mockLogger) Error(msg string, fields ...logging.Field) {}
func (mockLogger) Fatal(msg string, fields ...logging.Field) {}
func (mockLogger) With(fields ...logging.Field) logging.Logger { return mockLogger{} }
func (mockLogger) WithContext(ctx context.Context) logging.Logger { return mockLogger{} }
func (mockLogger) WithError(err error) logging.Logger { return mockLogger{} }
func (mockLogger) Sync() error { return nil }

type mockPortfolioService struct{
	portfolio   *domainportfolio.Portfolio
}
func (m *mockPortfolioService) CreatePortfolio(ctx context.Context, name, ownerID string, techDomains []string) (*domainportfolio.Portfolio, error) { return nil, nil }
func (m *mockPortfolioService) AddPatentsToPortfolio(ctx context.Context, portfolioID string, patentIDs []string) error { return nil }
func (m *mockPortfolioService) RemovePatentsFromPortfolio(ctx context.Context, portfolioID string, patentIDs []string) error { return nil }
func (m *mockPortfolioService) ActivatePortfolio(ctx context.Context, portfolioID string) error { return nil }
func (m *mockPortfolioService) ArchivePortfolio(ctx context.Context, portfolioID string) error { return nil }
func (m *mockPortfolioService) CalculateHealthScore(ctx context.Context, portfolioID string) (*domainportfolio.HealthScore, error) { return nil, nil }
func (m *mockPortfolioService) ComparePortfolios(ctx context.Context, portfolioIDs []string) ([]*domainportfolio.PortfolioComparison, error) { return nil, nil }
func (m *mockPortfolioService) IdentifyGaps(ctx context.Context, portfolioID string, targetDomains []string) ([]*domainportfolio.GapInfo, error) { return nil, nil }
func (m *mockPortfolioService) GetOverlapAnalysis(ctx context.Context, portfolioID1, portfolioID2 string) (*domainportfolio.OverlapResult, error) { return nil, nil }

// mockPortfolioDomainSvc is alias for mockPortfolioService as used in some tests
type mockPortfolioDomainSvc = mockPortfolioService

type mockValuationDomainSvc struct{}
func (mockValuationDomainSvc) CalculateHealthScore(ctx context.Context, portfolioID string) (*domainportfolio.HealthScore, error) { return nil, nil }

type mockMoleculeService struct{}
func (m *mockMoleculeService) CreateFromSMILES(ctx context.Context, smiles string, metadata map[string]string) (*domainmol.Molecule, error) { return nil, nil }

type mockGNNInference struct {
	embedding []float32
	embedErr  error
}
func (m *mockGNNInference) Embed(ctx context.Context, req *molpatent_gnn.EmbedRequest) (*molpatent_gnn.EmbedResponse, error) {
	if m.embedErr != nil { return nil, m.embedErr }
	vec := m.embedding
	if vec == nil { vec = []float32{0.1, 0.2, 0.3, 0.4} }
	return &molpatent_gnn.EmbedResponse{Embedding: vec, SMILES: req.SMILES, Confidence: 0.95}, nil
}
func (m *mockGNNInference) BatchEmbed(ctx context.Context, req *molpatent_gnn.BatchEmbedRequest) (*molpatent_gnn.BatchEmbedResponse, error) { return nil, nil }
func (m *mockGNNInference) ComputeSimilarity(ctx context.Context, req *molpatent_gnn.SimilarityRequest) (*molpatent_gnn.SimilarityResponse, error) { return nil, nil }
func (m *mockGNNInference) SearchSimilar(ctx context.Context, req *molpatent_gnn.SimilarSearchRequest) (*molpatent_gnn.SimilarSearchResponse, error) { return nil, nil }

// --- Constellation Mocks ---
type mockConstellationCache struct {
	store    map[string]interface{}
	getErr   error
	setErr   error
	getCalls int
	setCalls int
}
func newMockConstellationCache() *mockConstellationCache { return &mockConstellationCache{store: make(map[string]interface{})} }
func (m *mockConstellationCache) Get(ctx context.Context, key string, dest interface{}) error {
	m.getCalls++
	if m.getErr != nil { return m.getErr }
	if _, ok := m.store[key]; !ok { return fmt.Errorf("cache miss") }
	return nil
}
func (m *mockConstellationCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.setCalls++
	if m.setErr != nil { return m.setErr }
	m.store[key] = value
	return nil
}
func (m *mockConstellationCache) Delete(ctx context.Context, key string) error { delete(m.store, key); return nil }

type mockPortfolio struct {
	id   string
	name string
}
func (m *mockPortfolio) toPortfolio() *domainportfolio.Portfolio {
	var portfolioID uuid.UUID
	if parsedID, err := uuid.Parse(m.id); err == nil { portfolioID = parsedID } else { portfolioID = uuid.New() }
	return &domainportfolio.Portfolio{ID: portfolioID, Name: m.name}
}

type mockMolecule struct {
	id     string
	smiles string
}
func (m *mockMolecule) toMolecule() *domainmol.Molecule {
	var moleculeID uuid.UUID
	if parsedID, err := uuid.Parse(m.id); err == nil { moleculeID = parsedID } else { moleculeID = uuid.New() }
	return &domainmol.Molecule{ID: moleculeID, SMILES: m.smiles}
}

// ---------------------------------------------------------------------------
// Test Helper Functions
// ---------------------------------------------------------------------------

func makeTestPatent(id, title, status string, claimCount int, ipcCount int, filingYearsAgo float64) *patent.Patent {
	claims := make([]patent.Claim, claimCount)
	for i := 0; i < claimCount; i++ {
		claims[i] = patent.Claim{Number: i + 1, Type: patent.ClaimTypeDependent}
	}
	ipcs := make([]patent.IPCClassification, ipcCount)
	for i := 0; i < ipcCount; i++ {
		ipcs[i] = patent.IPCClassification{Full: fmt.Sprintf("G06F%d/00", i), Section: "G"}
	}
	filingDate := time.Now().AddDate(0, 0, -int(filingYearsAgo*365.25))

	// Ensure Metadata is not nil
	return &patent.Patent{
		ID:           id,
		PatentNumber: id,
		Title:        title,
		Status:       patent.PatentStatusGranted,
		Claims:       claims,
		IPCCodes:     ipcs,
		Dates:        patent.PatentDate{FilingDate: &filingDate},
		Metadata:     make(map[string]interface{}),
	}
}

func createTestPatent(number, techDomain, assignee string, filingDate time.Time, valueScore float64) *patent.Patent {
	return createTestPatentWithMolecules("", number, techDomain, assignee, filingDate, valueScore, nil)
}

func createTestPatentWithMolecules(id, number, techDomain, assignee string, filingDate time.Time, valueScore float64, moleculeIDs []string) *patent.Patent {
	var patentID string
	if id != "" { patentID = id } else { patentID = uuid.New().String() }

	now := time.Now()
	expiry := filingDate.AddDate(20, 0, 0)
	return &patent.Patent{
		ID:           patentID,
		PatentNumber: number,
		Title:        "Test",
		Applicants:   []patent.Applicant{{Name: assignee}},
		Dates:        patent.PatentDate{FilingDate: &filingDate, GrantDate: &now, ExpiryDate: &expiry},
		Status:       patent.PatentStatusGranted,
		Office:       patent.OfficeUSPTO,
		IPCCodes:     []patent.IPCClassification{{Full: techDomain + " 1/1", Section: techDomain}},
		MoleculeIDs:  moleculeIDs,
		Metadata:     map[string]any{"value_score": valueScore},
	}
}

//Personal.AI order the ending
