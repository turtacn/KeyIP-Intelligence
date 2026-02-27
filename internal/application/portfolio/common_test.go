// internal/application/portfolio/common_test.go
package portfolio

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	domainmol "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	domainpatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domainportfolio "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/molpatent_gnn"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/testutil"
)

// -----------------------------------------------------------------------
// Mock: Logger
// -----------------------------------------------------------------------

type mockLoggerConstellation struct{}

func (m *mockLoggerConstellation) Debug(msg string, fields ...logging.Field) {}
func (m *mockLoggerConstellation) Info(msg string, fields ...logging.Field)  {}
func (m *mockLoggerConstellation) Warn(msg string, fields ...logging.Field)  {}
func (m *mockLoggerConstellation) Error(msg string, fields ...logging.Field) {}
func (m *mockLoggerConstellation) Fatal(msg string, fields ...logging.Field) {}
func (m *mockLoggerConstellation) With(fields ...logging.Field) logging.Logger { return m }
func (m *mockLoggerConstellation) WithContext(ctx context.Context) logging.Logger { return m }
func (m *mockLoggerConstellation) WithError(err error) logging.Logger { return m }
func (m *mockLoggerConstellation) Sync() error { return nil }

var _ logging.Logger = (*mockLoggerConstellation)(nil)

// Alias for generic mock logger use
type mockLogger = mockLoggerConstellation

// -----------------------------------------------------------------------
// Mock: ConstellationCache
// -----------------------------------------------------------------------

type mockConstellationCache struct {
	store    map[string]interface{}
	getErr   error
	setErr   error
	getCalls int
	setCalls int
}

func newMockConstellationCache() *mockConstellationCache {
	return &mockConstellationCache{store: make(map[string]interface{})}
}

func (m *mockConstellationCache) Get(ctx context.Context, key string, dest interface{}) error {
	m.getCalls++
	if m.getErr != nil {
		return m.getErr
	}
	if _, ok := m.store[key]; !ok {
		return fmt.Errorf("cache miss")
	}
	return nil
}

func (m *mockConstellationCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.setCalls++
	if m.setErr != nil {
		return m.setErr
	}
	m.store[key] = value
	return nil
}

func (m *mockConstellationCache) Delete(ctx context.Context, key string) error {
	delete(m.store, key)
	return nil
}

// -----------------------------------------------------------------------
// Mock: Portfolio Domain Service
// -----------------------------------------------------------------------

type mockPortfolioService struct {
	portfolio   *domainportfolio.Portfolio
	getByIDErr  error
}

// Implement all methods from PortfolioService interface
func (m *mockPortfolioService) CreatePortfolio(ctx context.Context, name, ownerID string, techDomains []string) (*domainportfolio.Portfolio, error) {
	return nil, nil
}

func (m *mockPortfolioService) AddPatentsToPortfolio(ctx context.Context, portfolioID string, patentIDs []string) error {
	return nil
}

func (m *mockPortfolioService) RemovePatentsFromPortfolio(ctx context.Context, portfolioID string, patentIDs []string) error {
	return nil
}

func (m *mockPortfolioService) ActivatePortfolio(ctx context.Context, portfolioID string) error {
	return nil
}

func (m *mockPortfolioService) ArchivePortfolio(ctx context.Context, portfolioID string) error {
	return nil
}

func (m *mockPortfolioService) CalculateHealthScore(ctx context.Context, portfolioID string) (*domainportfolio.HealthScore, error) {
	return nil, nil
}

func (m *mockPortfolioService) ComparePortfolios(ctx context.Context, portfolioIDs []string) ([]*domainportfolio.PortfolioComparison, error) {
	return nil, nil
}

func (m *mockPortfolioService) IdentifyGaps(ctx context.Context, portfolioID string, targetDomains []string) ([]*domainportfolio.GapInfo, error) {
	return nil, nil
}

func (m *mockPortfolioService) GetOverlapAnalysis(ctx context.Context, portfolioID1, portfolioID2 string) (*domainportfolio.OverlapResult, error) {
	return nil, nil
}

var _ domainportfolio.Service = (*mockPortfolioService)(nil)

// -----------------------------------------------------------------------
// Mock: Portfolio entity (for test data creation)
// -----------------------------------------------------------------------

type mockPortfolio struct {
	id   string
	name string
}

// Helper to convert mockPortfolio to actual Portfolio struct for testing
func (m *mockPortfolio) toPortfolio() *domainportfolio.Portfolio {
	return &domainportfolio.Portfolio{
		ID:   m.id,
		Name: m.name,
	}
}

// -----------------------------------------------------------------------
// Mock: Portfolio Repository
// -----------------------------------------------------------------------

type mockPortfolioRepoConstellation struct {
	portfolios map[string]*domainportfolio.Portfolio
	err        error
}

func newMockPortfolioRepoConstellation() *mockPortfolioRepoConstellation {
	return &mockPortfolioRepoConstellation{portfolios: make(map[string]*domainportfolio.Portfolio)}
}

func (m *mockPortfolioRepoConstellation) GetByID(ctx context.Context, id string) (*domainportfolio.Portfolio, error) {
	if m.err != nil {
		return nil, m.err
	}
	p, ok := m.portfolios[id]
	if !ok {
		return nil, fmt.Errorf("portfolio %s not found", id)
	}
	return p, nil
}

// Stub remaining interface methods
func (m *mockPortfolioRepoConstellation) Create(ctx context.Context, p *domainportfolio.Portfolio) error {
	return m.err
}

func (m *mockPortfolioRepoConstellation) Update(ctx context.Context, p *domainportfolio.Portfolio) error {
	return m.err
}

func (m *mockPortfolioRepoConstellation) SoftDelete(ctx context.Context, id string) error {
	return m.err
}

func (m *mockPortfolioRepoConstellation) List(ctx context.Context, ownerID string, opts ...domainportfolio.PortfolioQueryOption) ([]*domainportfolio.Portfolio, int64, error) {
	return nil, 0, m.err
}

func (m *mockPortfolioRepoConstellation) GetByOwner(ctx context.Context, ownerID string) ([]*domainportfolio.Portfolio, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) AddPatent(ctx context.Context, portfolioID, patentID string, role string, addedBy string) error {
	return m.err
}

func (m *mockPortfolioRepoConstellation) RemovePatent(ctx context.Context, portfolioID, patentID string) error {
	return m.err
}

func (m *mockPortfolioRepoConstellation) GetPatents(ctx context.Context, portfolioID string, role *string, limit, offset int) ([]*domainpatent.Patent, int64, error) {
	return nil, 0, m.err
}

func (m *mockPortfolioRepoConstellation) IsPatentInPortfolio(ctx context.Context, portfolioID, patentID string) (bool, error) {
	return false, m.err
}

func (m *mockPortfolioRepoConstellation) BatchAddPatents(ctx context.Context, portfolioID string, patentIDs []string, role string, addedBy string) error {
	return m.err
}

func (m *mockPortfolioRepoConstellation) GetPortfoliosByPatent(ctx context.Context, patentID string) ([]*domainportfolio.Portfolio, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) CreateValuation(ctx context.Context, v *domainportfolio.Valuation) error {
	return m.err
}

func (m *mockPortfolioRepoConstellation) GetLatestValuation(ctx context.Context, patentID string) (*domainportfolio.Valuation, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) GetValuationHistory(ctx context.Context, patentID string, limit int) ([]*domainportfolio.Valuation, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) GetValuationsByPortfolio(ctx context.Context, portfolioID string) ([]*domainportfolio.Valuation, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) GetValuationDistribution(ctx context.Context, portfolioID string) (map[domainportfolio.ValuationTier]int64, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) BatchCreateValuations(ctx context.Context, valuations []*domainportfolio.Valuation) error {
	return m.err
}

func (m *mockPortfolioRepoConstellation) CreateHealthScore(ctx context.Context, score *domainportfolio.HealthScore) error {
	return m.err
}

func (m *mockPortfolioRepoConstellation) GetLatestHealthScore(ctx context.Context, portfolioID string) (*domainportfolio.HealthScore, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) GetHealthScoreHistory(ctx context.Context, portfolioID string, limit int) ([]*domainportfolio.HealthScore, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) GetHealthScoreTrend(ctx context.Context, portfolioID string, startDate, endDate time.Time) ([]*domainportfolio.HealthScore, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) CreateSuggestion(ctx context.Context, s *domainportfolio.OptimizationSuggestion) error {
	return m.err
}

func (m *mockPortfolioRepoConstellation) GetSuggestions(ctx context.Context, portfolioID string, status *string, limit, offset int) ([]*domainportfolio.OptimizationSuggestion, int64, error) {
	return nil, 0, m.err
}

func (m *mockPortfolioRepoConstellation) UpdateSuggestionStatus(ctx context.Context, id string, status string, resolvedBy string) error {
	return m.err
}

func (m *mockPortfolioRepoConstellation) GetPendingSuggestionCount(ctx context.Context, portfolioID string) (int64, error) {
	return 0, m.err
}

func (m *mockPortfolioRepoConstellation) GetPortfolioSummary(ctx context.Context, portfolioID string) (*domainportfolio.Summary, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) GetJurisdictionCoverage(ctx context.Context, portfolioID string) (map[string]int64, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) GetTechDomainCoverage(ctx context.Context, portfolioID string) (map[string]int64, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) GetExpiryTimeline(ctx context.Context, portfolioID string) ([]*domainportfolio.ExpiryTimelineEntry, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) ComparePortfolios(ctx context.Context, portfolioIDs []string) ([]*domainportfolio.ComparisonResult, error) {
	return nil, m.err
}

func (m *mockPortfolioRepoConstellation) WithTx(ctx context.Context, fn func(domainportfolio.PortfolioRepository) error) error {
	return fn(m)
}

var _ domainportfolio.PortfolioRepository = (*mockPortfolioRepoConstellation)(nil)

// -----------------------------------------------------------------------
// Mock: Patent Repository
// -----------------------------------------------------------------------

type mockPatentRepoConstellation struct {
	testutil.BasePatentRepoMock
	byPortfolio map[string][]*domainpatent.Patent
	byAssignee  map[string][]*domainpatent.Patent
	byIDs       map[string]*domainpatent.Patent
	findErr     error
}

func newMockPatentRepoConstellation() *mockPatentRepoConstellation {
	return &mockPatentRepoConstellation{
		byPortfolio: make(map[string][]*domainpatent.Patent),
		byAssignee:  make(map[string][]*domainpatent.Patent),
		byIDs:       make(map[string]*domainpatent.Patent),
	}
}

// Simplified mock methods for testing - not implementing full PatentRepository interface
func (m *mockPatentRepoConstellation) ListByPortfolio(ctx context.Context, portfolioID string) ([]*domainpatent.Patent, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.byPortfolio[portfolioID], nil
}

// Stub minimal methods to satisfy compilation (not implementing full interface)
func (m *mockPatentRepoConstellation) Create(ctx context.Context, p *domainpatent.Patent) error {
	return nil
}

func (m *mockPatentRepoConstellation) GetByID(ctx context.Context, id uuid.UUID) (*domainpatent.Patent, error) {
	idStr := id.String()
	if p, ok := m.byIDs[idStr]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockPatentRepoConstellation) FindByID(ctx context.Context, id string) (*domainpatent.Patent, error) {
	if p, ok := m.byIDs[id]; ok {
		return p, nil
	}
	// Fallback to searching in byAssignee/byPortfolio if not directly in byIDs
	for _, patents := range m.byPortfolio {
		for _, p := range patents {
			if p.ID.String() == id {
				return p, nil
			}
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockPatentRepoConstellation) GetByPatentNumber(ctx context.Context, number string) (*domainpatent.Patent, error) {
	return nil, nil
}

func (m *mockPatentRepoConstellation) Update(ctx context.Context, p *domainpatent.Patent) error {
	return nil
}

func (m *mockPatentRepoConstellation) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockPatentRepoConstellation) Restore(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockPatentRepoConstellation) HardDelete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockPatentRepoConstellation) Search(ctx context.Context, criteria domainpatent.PatentSearchCriteria) (*domainpatent.PatentSearchResult, error) {
	return nil, nil
}

func (m *mockPatentRepoConstellation) GetByFamilyID(ctx context.Context, familyID string) ([]*domainpatent.Patent, error) {
	return nil, nil
}

func (m *mockPatentRepoConstellation) GetByAssignee(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*domainpatent.Patent, int64, error) {
	return nil, 0, nil
}

func (m *mockPatentRepoConstellation) SearchByAssigneeName(ctx context.Context, assigneeName string, limit, offset int) ([]*domainpatent.Patent, int64, error) {
	if m.findErr != nil {
		return nil, 0, m.findErr
	}
	patents := m.byAssignee[assigneeName]
	return patents, int64(len(patents)), nil
}

func (m *mockPatentRepoConstellation) WithTx(ctx context.Context, fn func(domainpatent.PatentRepository) error) error {
	return nil
}

var _ domainpatent.Repository = (*mockPatentRepoConstellation)(nil)

// -----------------------------------------------------------------------
// Mock: Patent entity (for test data creation)
// -----------------------------------------------------------------------

type mockPatent struct {
	id          string
	number      string
	techDomain  string
	legalStatus string
	assignee    string
	filingDate  time.Time
	valueScore  float64
	moleculeIDs []string
}

// Helper to convert mockPatent to actual Patent struct for testing
func (m *mockPatent) toPatent() *domainpatent.Patent {
	status := domainpatent.PatentStatusGranted
	if m.legalStatus == "pending" {
		status = domainpatent.PatentStatusFiled
	}

	// Generate valid UUID from string ID for testing
	var patentID uuid.UUID
	if parsedID, err := uuid.Parse(m.id); err == nil {
		patentID = parsedID
	} else {
		// If not a valid UUID, generate a new one (deterministic for testing)
		patentID = uuid.New()
	}

	filingDate := m.filingDate

	// Set metadata with value_score for GetValueScore() method
	metadata := make(map[string]interface{})
	if m.valueScore > 0 {
		metadata["value_score"] = m.valueScore
	}

	return &domainpatent.Patent{
		ID:               patentID,
		PatentNumber:     m.number,
		Title:            "Test Patent",
		Office:           domainpatent.OfficeUSPTO,
		Status:           status,
		AssigneeName:     m.assignee,
		FilingDate:       &filingDate,
		IPCCodes:         []string{m.techDomain},
		KeyIPTechCodes:   []string{m.techDomain},
		MoleculeIDs:      m.moleculeIDs,
		Metadata:         metadata,
	}
}

// -----------------------------------------------------------------------
// Mock: Molecule Repository
// -----------------------------------------------------------------------

type mockMoleculeRepo struct {
	molecules map[string]*domainmol.Molecule
	findErr   error
}

func newMockMoleculeRepo() *mockMoleculeRepo {
	return &mockMoleculeRepo{molecules: make(map[string]*domainmol.Molecule)}
}

func (m *mockMoleculeRepo) FindByIDs(ctx context.Context, ids []string) ([]*domainmol.Molecule, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	result := make([]*domainmol.Molecule, 0, len(ids))
	for _, id := range ids {
		if mol, ok := m.molecules[id]; ok {
			result = append(result, mol)
		}
	}
	return result, nil
}

// Stub remaining interface methods
func (m *mockMoleculeRepo) Save(ctx context.Context, mol *domainmol.Molecule) error {
	return nil
}

func (m *mockMoleculeRepo) Update(ctx context.Context, molecule *domainmol.Molecule) error {
	return nil
}

func (m *mockMoleculeRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockMoleculeRepo) BatchSave(ctx context.Context, molecules []*domainmol.Molecule) (int, error) {
	return len(molecules), nil
}

func (m *mockMoleculeRepo) FindByID(ctx context.Context, id string) (*domainmol.Molecule, error) {
	if mol, ok := m.molecules[id]; ok {
		return mol, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockMoleculeRepo) FindByInChIKey(ctx context.Context, inchiKey string) (*domainmol.Molecule, error) {
	return nil, nil
}

func (m *mockMoleculeRepo) FindBySMILES(ctx context.Context, smiles string) ([]*domainmol.Molecule, error) {
	return nil, nil
}

func (m *mockMoleculeRepo) Exists(ctx context.Context, id string) (bool, error) {
	_, ok := m.molecules[id]
	return ok, nil
}

func (m *mockMoleculeRepo) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) {
	return false, nil
}

func (m *mockMoleculeRepo) Search(ctx context.Context, query *domainmol.MoleculeQuery) (*domainmol.MoleculeSearchResult, error) {
	return nil, nil
}

func (m *mockMoleculeRepo) Count(ctx context.Context, query *domainmol.MoleculeQuery) (int64, error) {
	return 0, nil
}

func (m *mockMoleculeRepo) FindBySource(ctx context.Context, source domainmol.MoleculeSource, offset, limit int) ([]*domainmol.Molecule, error) {
	return nil, nil
}

func (m *mockMoleculeRepo) FindByStatus(ctx context.Context, status domainmol.MoleculeStatus, offset, limit int) ([]*domainmol.Molecule, error) {
	return nil, nil
}

func (m *mockMoleculeRepo) FindByTags(ctx context.Context, tags []string, offset, limit int) ([]*domainmol.Molecule, error) {
	return nil, nil
}

func (m *mockMoleculeRepo) FindByMolecularWeightRange(ctx context.Context, minWeight, maxWeight float64, offset, limit int) ([]*domainmol.Molecule, error) {
	return nil, nil
}

func (m *mockMoleculeRepo) FindWithFingerprint(ctx context.Context, fpType domainmol.FingerprintType, offset, limit int) ([]*domainmol.Molecule, error) {
	return nil, nil
}

func (m *mockMoleculeRepo) FindWithoutFingerprint(ctx context.Context, fpType domainmol.FingerprintType, offset, limit int) ([]*domainmol.Molecule, error) {
	return nil, nil
}

var _ domainmol.Repository = (*mockMoleculeRepo)(nil)

// -----------------------------------------------------------------------
// Mock: Molecule entity (for test data creation)
// -----------------------------------------------------------------------

type mockMolecule struct {
	id     string
	smiles string
}

// Helper to convert mockMolecule to actual Molecule struct for testing
func (m *mockMolecule) toMolecule() *domainmol.Molecule {
	// Generate valid UUID from string ID for testing
	var moleculeID uuid.UUID
	if parsedID, err := uuid.Parse(m.id); err == nil {
		moleculeID = parsedID
	} else {
		// If not a valid UUID, generate a new one
		moleculeID = uuid.New()
	}

	return &domainmol.Molecule{
		ID:     moleculeID,
		SMILES: m.smiles,
	}
}

// -----------------------------------------------------------------------
// Mock: Molecule Domain Service
// -----------------------------------------------------------------------

type mockMoleculeService struct{}

func (m *mockMoleculeService) CreateFromSMILES(ctx context.Context, smiles string, metadata map[string]string) (*domainmol.Molecule, error) {
	return nil, nil
}

func (m *mockMoleculeService) RegisterMolecule(ctx context.Context, smiles string, source domainmol.MoleculeSource, sourceRef string) (*domainmol.Molecule, error) { return nil, nil }
func (m *mockMoleculeService) BatchRegisterMolecules(ctx context.Context, requests []domainmol.MoleculeRegistrationRequest) (*domainmol.BatchRegistrationResult, error) { return nil, nil }
func (m *mockMoleculeService) GetMolecule(ctx context.Context, id string) (*domainmol.Molecule, error) { return nil, nil }
func (m *mockMoleculeService) GetMoleculeByInChIKey(ctx context.Context, inchiKey string) (*domainmol.Molecule, error) { return nil, nil }
func (m *mockMoleculeService) SearchMolecules(ctx context.Context, query *domainmol.MoleculeQuery) (*domainmol.MoleculeSearchResult, error) { return nil, nil }
func (m *mockMoleculeService) CalculateFingerprints(ctx context.Context, moleculeID string, fpTypes []domainmol.FingerprintType) error { return nil }
func (m *mockMoleculeService) FindSimilarMolecules(ctx context.Context, targetSMILES string, fpType domainmol.FingerprintType, threshold float64, limit int) ([]*domainmol.SimilarityResult, error) { return nil, nil }
func (m *mockMoleculeService) CompareMolecules(ctx context.Context, smiles1, smiles2 string, fpTypes []domainmol.FingerprintType) (*domainmol.MoleculeComparisonResult, error) { return nil, nil }
func (m *mockMoleculeService) ArchiveMolecule(ctx context.Context, id string) error { return nil }
func (m *mockMoleculeService) DeleteMolecule(ctx context.Context, id string) error { return nil }
func (m *mockMoleculeService) AddMoleculeProperties(ctx context.Context, moleculeID string, properties []*domainmol.MolecularProperty) error { return nil }
func (m *mockMoleculeService) TagMolecule(ctx context.Context, moleculeID string, tags []string) error { return nil }
func (m *mockMoleculeService) Canonicalize(ctx context.Context, smiles string) (string, string, error) { return "", "", nil }
func (m *mockMoleculeService) CanonicalizeFromInChI(ctx context.Context, inchi string) (string, string, error) { return "", "", nil }

var _ domainmol.Service = (*mockMoleculeService)(nil)

// -----------------------------------------------------------------------
// Mock: GNN Inference Engine
// -----------------------------------------------------------------------

type mockGNNInference struct {
	embedding []float32
	embedErr  error
}

func (m *mockGNNInference) Embed(ctx context.Context, req *molpatent_gnn.EmbedRequest) (*molpatent_gnn.EmbedResponse, error) {
	if m.embedErr != nil {
		return nil, m.embedErr
	}
	vec := m.embedding
	if vec == nil {
		vec = []float32{0.1, 0.2, 0.3, 0.4}
	}
	return &molpatent_gnn.EmbedResponse{
		Embedding:    vec,
		SMILES:       req.SMILES,
		Confidence:   0.95,
		InferenceMs:  10,
		ModelVersion: "test-v1",
	}, nil
}

func (m *mockGNNInference) BatchEmbed(ctx context.Context, req *molpatent_gnn.BatchEmbedRequest) (*molpatent_gnn.BatchEmbedResponse, error) {
	return &molpatent_gnn.BatchEmbedResponse{Results: nil}, nil
}

func (m *mockGNNInference) ComputeSimilarity(ctx context.Context, req *molpatent_gnn.SimilarityRequest) (*molpatent_gnn.SimilarityResponse, error) {
	return &molpatent_gnn.SimilarityResponse{
		FusedScore:   0.85,
		Scores:       map[string]float64{"morgan": 0.85},
		InferenceMs:  10,
		ModelVersion: "test-v1",
	}, nil
}

func (m *mockGNNInference) SearchSimilar(ctx context.Context, req *molpatent_gnn.SimilarSearchRequest) (*molpatent_gnn.SimilarSearchResponse, error) {
	return &molpatent_gnn.SimilarSearchResponse{
		Matches:      nil,
		QuerySMILES:  req.SMILES,
		InferenceMs:  10,
	}, nil
}

// -----------------------------------------------------------------------
// Helper to create a test portfolio with specific ID
// -----------------------------------------------------------------------
func createTestPortfolioWithID(id, name string) *domainportfolio.Portfolio {
	now := time.Now()
	// Since we are using string IDs now, we keep it as string
	p := &domainportfolio.Portfolio{
		ID:           id,
		Name:         name,
		OwnerID:      uuid.New().String(),
		TechDomains:  []string{"C07D"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return p
}

// -----------------------------------------------------------------------
// Helper to create a test portfolio with random UUID (string)
// -----------------------------------------------------------------------
func createTestPortfolio(name string) *domainportfolio.Portfolio {
	return createTestPortfolioWithID(uuid.New().String(), name)
}

// -----------------------------------------------------------------------
// Helper to create a mock portfolio repository with predefined portfolios
// -----------------------------------------------------------------------
func newMockPortfolioRepoWithData(portfolios ...*domainportfolio.Portfolio) *mockPortfolioRepoConstellation {
	repo := newMockPortfolioRepoConstellation()
	for _, p := range portfolios {
		repo.portfolios[p.ID] = p
	}
	return repo
}

// -----------------------------------------------------------------------
// Helper to create test patent from mock data
// -----------------------------------------------------------------------
func createTestPatent(number, techDomain, assignee string, filingDate time.Time, valueScore float64) *domainpatent.Patent {
	return createTestPatentWithMolecules("", number, techDomain, assignee, filingDate, valueScore, nil)
}

// -----------------------------------------------------------------------
// Helper to create test patent with full fields
// -----------------------------------------------------------------------
func createTestPatentWithMolecules(id, number, techDomain, assignee string, filingDate time.Time, valueScore float64, moleculeIDs []string) *domainpatent.Patent {
	var patentID uuid.UUID
	if id != "" {
		if parsedID, err := uuid.Parse(id); err == nil {
			patentID = parsedID
		} else {
			patentID = uuid.New()
		}
	} else {
		patentID = uuid.New()
	}

	now := time.Now()
	expiryDate := filingDate.AddDate(20, 0, 0)
	return &domainpatent.Patent{
		ID:              patentID,
		PatentNumber:    number,
		Title:           "Test Patent",
		Abstract:        "Test abstract",
		AssigneeName:    assignee,
		FilingDate:      &filingDate,
		GrantDate:       &now,
		ExpiryDate:      &expiryDate,
		Status:          domainpatent.PatentStatusGranted,
		Office:          domainpatent.OfficeUSPTO,
		IPCCodes:        []string{techDomain},
		KeyIPTechCodes:  []string{techDomain},
		MoleculeIDs:     moleculeIDs,
		Metadata:        map[string]any{"value_score": valueScore},
		CreatedAt:       now,
		UpdatedAt:       now,
		Version:         1,
	}
}

//Personal.AI order the ending
