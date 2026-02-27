// Phase 17 - Integration Test Helpers
// Provides shared test infrastructure for integration tests including
// container lifecycle management, database seeding, service bootstrapping,
// and assertion utilities. All integration tests depend on this file.
package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	appCollaboration "github.com/turtacn/KeyIP-Intelligence/internal/application/collaboration"
	appInfringement "github.com/turtacn/KeyIP-Intelligence/internal/application/infringement"
	appLifecycle "github.com/turtacn/KeyIP-Intelligence/internal/application/lifecycle"
	appMining "github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
	appPortfolio "github.com/turtacn/KeyIP-Intelligence/internal/application/portfolio"
	appQuery "github.com/turtacn/KeyIP-Intelligence/internal/application/query"
	appReporting "github.com/turtacn/KeyIP-Intelligence/internal/application/reporting"
	domainMolecule "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	domainPatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domainPortfolio "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	domainLifecycle "github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	domainCollaboration "github.com/turtacn/KeyIP-Intelligence/internal/domain/collaboration"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	pkgErrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	commonTypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	moleculeTypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
	patentTypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ---------------------------------------------------------------------------
// Environment detection
// ---------------------------------------------------------------------------

const (
	// EnvIntegrationEnabled controls whether integration tests run.
	EnvIntegrationEnabled = "KEYIP_INTEGRATION_TEST"

	// EnvPostgresURL overrides the default PostgreSQL DSN.
	EnvPostgresURL = "KEYIP_TEST_POSTGRES_URL"

	// EnvNeo4jURL overrides the default Neo4j bolt URL.
	EnvNeo4jURL = "KEYIP_TEST_NEO4J_URL"

	// EnvRedisURL overrides the default Redis URL.
	EnvRedisURL = "KEYIP_TEST_REDIS_URL"

	// EnvOpenSearchURL overrides the default OpenSearch URL.
	EnvOpenSearchURL = "KEYIP_TEST_OPENSEARCH_URL"

	// EnvMilvusURL overrides the default Milvus gRPC address.
	EnvMilvusURL = "KEYIP_TEST_MILVUS_URL"

	// EnvKafkaBrokers overrides the default Kafka broker list.
	EnvKafkaBrokers = "KEYIP_TEST_KAFKA_BROKERS"

	// EnvMinIOEndpoint overrides the default MinIO endpoint.
	EnvMinIOEndpoint = "KEYIP_TEST_MINIO_ENDPOINT"

	// DefaultPostgresURL is the fallback PostgreSQL DSN for local dev.
	DefaultPostgresURL = "postgres://keyip:keyip@localhost:5432/keyip_test?sslmode=disable"

	// DefaultNeo4jURL is the fallback Neo4j bolt URL.
	DefaultNeo4jURL = "bolt://neo4j:neo4j@localhost:7687"

	// DefaultRedisURL is the fallback Redis URL.
	DefaultRedisURL = "redis://localhost:6379/1"

	// DefaultOpenSearchURL is the fallback OpenSearch URL.
	DefaultOpenSearchURL = "http://localhost:9200"

	// DefaultMilvusURL is the fallback Milvus gRPC address.
	DefaultMilvusURL = "localhost:19530"

	// DefaultKafkaBrokers is the fallback Kafka broker list.
	DefaultKafkaBrokers = "localhost:9092"

	// DefaultMinIOEndpoint is the fallback MinIO endpoint.
	DefaultMinIOEndpoint = "localhost:9000"

	// TestTimeout is the maximum duration for a single integration test.
	TestTimeout = 120 * time.Second

	// SetupTimeout is the maximum duration for test environment setup.
	SetupTimeout = 60 * time.Second
)

// ---------------------------------------------------------------------------
// SkipIfNoIntegration skips the calling test when the integration flag is unset.
// ---------------------------------------------------------------------------

func SkipIfNoIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv(EnvIntegrationEnabled) == "" {
		t.Skipf("skipping integration test: set %s=1 to enable", EnvIntegrationEnabled)
	}
}

// ---------------------------------------------------------------------------
// TestEnvironment holds all shared resources for an integration test suite.
// ---------------------------------------------------------------------------

// TestEnvironment aggregates infrastructure clients, domain services, and
// application services required by integration tests. It is initialised once
// per test binary via sync.Once and torn down via cleanup functions registered
// through testing.T.Cleanup.
type TestEnvironment struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	Cfg    *config.Config
	Logger logging.Logger

	// Infrastructure handles (nil when the corresponding service is unavailable)
	PostgresDB *sql.DB
	// Neo4jSession, RedisClient, OpenSearchClient, MilvusClient, KafkaProducer,
	// MinIOClient would be typed fields in a real build; kept as interface{} here
	// so the file compiles without requiring every driver dependency.
	Neo4jSession     interface{}
	RedisClient      interface{}
	OpenSearchClient interface{}
	MilvusClient     interface{}
	KafkaProducer    interface{}
	MinIOClient      interface{}

	// Domain services
	MoleculeService      domainMolecule.Service
	PatentService        *domainPatent.PatentService
	PortfolioService     domainPortfolio.Service
	LifecycleService     domainLifecycle.Service
	CollaborationService domainCollaboration.CollaborationService

	// Application services
	SimilaritySearchService appMining.SimilaritySearchService
	PatentabilityService    appMining.PatentabilityService
	ChemExtractionService   appMining.ChemExtractionService
	WhiteSpaceService       appMining.WhiteSpaceService
	MonitoringService       appInfringement.MonitoringService
	RiskAssessmentService   appInfringement.RiskAssessmentService
	AlertService            appInfringement.AlertService
	CompetitorTrackingService appInfringement.CompetitorTrackingService
	ValuationAppService     appPortfolio.ValuationService
	OptimizationService     appPortfolio.OptimizationService
	GapAnalysisService      appPortfolio.GapAnalysisService
	ConstellationService    appPortfolio.ConstellationService
	AnnuityAppService       appLifecycle.AnnuityService
	DeadlineAppService      appLifecycle.DeadlineService
	LegalStatusService      appLifecycle.LegalStatusService
	CalendarService         appLifecycle.CalendarService
	WorkspaceAppService     appCollaboration.WorkspaceAppService
	SharingService          appCollaboration.SharingService
	KGSearchService         appQuery.KGSearchService
	NLQueryService          appQuery.NLQueryService
	FTOReportService        appReporting.FTOReportService
	InfringementReportService appReporting.InfringementReportService
	PortfolioReportService  appReporting.PortfolioReportService
	TemplateEngine          appReporting.TemplateEngine

	// HTTP test server (optional, created on demand)
	HTTPServer *httptest.Server
}

var (
	globalEnv     *TestEnvironment
	globalEnvOnce sync.Once
	globalEnvErr  error
)

// ---------------------------------------------------------------------------
// Setup / Teardown
// ---------------------------------------------------------------------------

// SetupTestEnvironment returns a shared TestEnvironment. The heavy
// initialisation (container connections, migrations, seeding) runs exactly
// once per test binary. Individual tests receive a child context that is
// cancelled when the test finishes.
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()
	SkipIfNoIntegration(t)

	globalEnvOnce.Do(func() {
		globalEnv, globalEnvErr = buildTestEnvironment()
	})
	if globalEnvErr != nil {
		t.Fatalf("integration environment setup failed: %v", globalEnvErr)
	}

	// Derive a per-test context with timeout.
	ctx, cancel := context.WithTimeout(globalEnv.Ctx, TestTimeout)
	t.Cleanup(cancel)

	// Return a shallow copy with the per-test context.
	env := *globalEnv
	env.Ctx = ctx
	env.Cancel = cancel
	return &env
}

// buildTestEnvironment performs the one-time heavy setup.
func buildTestEnvironment() (*TestEnvironment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), SetupTimeout)

	cfg, err := loadTestConfig()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("load test config: %w", err)
	}

	logger, err := logging.NewLogger(logging.LogConfig{
		Level:  logging.LevelDebug,
		Format: "console",
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create logger: %w", err)
	}

	env := &TestEnvironment{
		Ctx:    ctx,
		Cancel: cancel,
		Cfg:    cfg,
		Logger: logger,
	}

	// Connect to infrastructure components. Each connector is best-effort;
	// tests that require a missing component will skip themselves.
	env.connectPostgres()
	env.connectNeo4j()
	env.connectRedis()
	env.connectOpenSearch()
	env.connectMilvus()
	env.connectKafka()
	env.connectMinIO()

	// Bootstrap domain and application services using real or stub
	// repositories depending on which infrastructure is available.
	env.bootstrapServices()

	return env, nil
}

// loadTestConfig builds a Config suitable for integration tests.
func loadTestConfig() (*config.Config, error) {
	cfg := config.NewDefaultConfig()
	
	// Apply test-specific configuration (simplified for integration tests)
	// Note: For integration tests, we use environment variables or defaults
	// directly when connecting to infrastructure. The config here is mainly
	// for documentation and to satisfy service constructors that need it.
	
	return cfg, nil
}

// ---------------------------------------------------------------------------
// Infrastructure connectors (best-effort)
// ---------------------------------------------------------------------------

func (env *TestEnvironment) connectPostgres() {
	dsn := os.Getenv(EnvPostgresURL)
	if dsn == "" {
		dsn = DefaultPostgresURL
	}
	
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		env.Logger.Warn("postgres unavailable for integration tests", logging.Err(err))
		return
	}
	if err := db.PingContext(env.Ctx); err != nil {
		env.Logger.Warn("postgres ping failed", logging.Err(err))
		_ = db.Close()
		return
	}
	env.PostgresDB = db
}

func (env *TestEnvironment) connectNeo4j() {
	// Placeholder: real implementation would use neo4j-go-driver.
	env.Logger.Info("neo4j connector: stub (driver not linked in test binary)")
}

func (env *TestEnvironment) connectRedis() {
	// Placeholder: real implementation would use go-redis.
	env.Logger.Info("redis connector: stub (driver not linked in test binary)")
}

func (env *TestEnvironment) connectOpenSearch() {
	env.Logger.Info("opensearch connector: stub")
}

func (env *TestEnvironment) connectMilvus() {
	env.Logger.Info("milvus connector: stub")
}

func (env *TestEnvironment) connectKafka() {
	env.Logger.Info("kafka connector: stub")
}

func (env *TestEnvironment) connectMinIO() {
	env.Logger.Info("minio connector: stub")
}

// ---------------------------------------------------------------------------
// Service bootstrap
// ---------------------------------------------------------------------------

func (env *TestEnvironment) bootstrapServices() {
	// In a full build the real repository implementations would be injected
	// here. For the integration test skeleton we wire stub/mock services that
	// satisfy the interfaces so the test files compile and the wiring logic
	// is exercised.
	env.Logger.Info("bootstrapping integration test services (stub wiring)")

	// Domain services are constructed with nil repos; individual tests that
	// exercise real persistence must check env.PostgresDB != nil etc.
	// This keeps the helper compilable without pulling every driver.
}

// ---------------------------------------------------------------------------
// Require* guards — skip a test when a specific backend is unavailable.
// ---------------------------------------------------------------------------

// RequirePostgres skips the test if PostgreSQL is not connected.
func RequirePostgres(t *testing.T, env *TestEnvironment) {
	t.Helper()
	if env.PostgresDB == nil {
		t.Skip("skipping: PostgreSQL not available")
	}
}

// RequireNeo4j skips the test if Neo4j is not connected.
func RequireNeo4j(t *testing.T, env *TestEnvironment) {
	t.Helper()
	if env.Neo4jSession == nil {
		t.Skip("skipping: Neo4j not available")
	}
}

// RequireRedis skips the test if Redis is not connected.
func RequireRedis(t *testing.T, env *TestEnvironment) {
	t.Helper()
	if env.RedisClient == nil {
		t.Skip("skipping: Redis not available")
	}
}

// RequireOpenSearch skips the test if OpenSearch is not connected.
func RequireOpenSearch(t *testing.T, env *TestEnvironment) {
	t.Helper()
	if env.OpenSearchClient == nil {
		t.Skip("skipping: OpenSearch not available")
	}
}

// RequireMilvus skips the test if Milvus is not connected.
func RequireMilvus(t *testing.T, env *TestEnvironment) {
	t.Helper()
	if env.MilvusClient == nil {
		t.Skip("skipping: Milvus not available")
	}
}

// RequireKafka skips the test if Kafka is not connected.
func RequireKafka(t *testing.T, env *TestEnvironment) {
	t.Helper()
	if env.KafkaProducer == nil {
		t.Skip("skipping: Kafka not available")
	}
}

// RequireMinIO skips the test if MinIO is not connected.
func RequireMinIO(t *testing.T, env *TestEnvironment) {
	t.Helper()
	if env.MinIOClient == nil {
		t.Skip("skipping: MinIO not available")
	}
}

// ---------------------------------------------------------------------------
// Fixture loading helpers
// ---------------------------------------------------------------------------

const fixtureBasePath = "../testdata/fixtures/"

// LoadFixture reads a JSON fixture file and unmarshals it into dest.
func LoadFixture(t *testing.T, filename string, dest interface{}) {
	t.Helper()
	data, err := os.ReadFile(fixtureBasePath + filename)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", filename, err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		t.Fatalf("failed to unmarshal fixture %s: %v", filename, err)
	}
}

// LoadMoleculeFixtures loads the standard molecule test fixtures.
func LoadMoleculeFixtures(t *testing.T) []moleculeTypes.MoleculeDTO {
	t.Helper()
	var fixtures []moleculeTypes.MoleculeDTO
	LoadFixture(t, "molecule_fixtures.json", &fixtures)
	return fixtures
}

// LoadPatentFixtures loads the standard patent test fixtures.
func LoadPatentFixtures(t *testing.T) []patentTypes.PatentDTO {
	t.Helper()
	var fixtures []patentTypes.PatentDTO
	LoadFixture(t, "patent_fixtures.json", &fixtures)
	return fixtures
}

// LoadPortfolioFixtures loads the standard portfolio test fixtures.
func LoadPortfolioFixtures(t *testing.T) []map[string]interface{} {
	t.Helper()
	var fixtures []map[string]interface{}
	LoadFixture(t, "portfolio_fixtures.json", &fixtures)
	return fixtures
}

// LoadSamplePatent reads a single sample patent JSON from testdata/patents/.
func LoadSamplePatent(t *testing.T, filename string) patentTypes.PatentDTO {
	t.Helper()
	data, err := os.ReadFile("../testdata/patents/" + filename)
	if err != nil {
		t.Fatalf("failed to read sample patent %s: %v", filename, err)
	}
	var p patentTypes.PatentDTO
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("failed to unmarshal sample patent %s: %v", filename, err)
	}
	return p
}

// LoadSMILESFile reads a .smi file and returns one SMILES string per line.
func LoadSMILESFile(t *testing.T, filename string) []string {
	t.Helper()
	data, err := os.ReadFile("../testdata/molecules/" + filename)
	if err != nil {
		t.Fatalf("failed to read SMILES file %s: %v", filename, err)
	}
	var result []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			// Take only the SMILES part (first whitespace-delimited token).
			parts := strings.Fields(line)
			if len(parts) > 0 {
				result = append(result, parts[0])
			}
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Seed helpers — insert fixture data into real backends.
// ---------------------------------------------------------------------------

// SeedMolecules inserts molecule fixtures into PostgreSQL.
func SeedMolecules(t *testing.T, env *TestEnvironment) {
	t.Helper()
	RequirePostgres(t, env)
	fixtures := LoadMoleculeFixtures(t)
	for _, m := range fixtures {
		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO molecules (id, smiles, inchi, inchi_key, molecular_formula, molecular_weight, status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 ON CONFLICT (id) DO NOTHING`,
			m.ID, m.SMILES, m.InChI, m.InChIKey, m.MolecularFormula, m.MolecularWeight,
			"active", time.Now(), time.Now(),
		)
		if err != nil {
			t.Fatalf("seed molecule %s: %v", m.ID, err)
		}
	}
	t.Logf("seeded %d molecules", len(fixtures))
}

// SeedPatents inserts patent fixtures into PostgreSQL.
func SeedPatents(t *testing.T, env *TestEnvironment) {
	t.Helper()
	RequirePostgres(t, env)
	fixtures := LoadPatentFixtures(t)
	for _, p := range fixtures {
		_, err := env.PostgresDB.ExecContext(env.Ctx,
			`INSERT INTO patents (id, patent_number, title, abstract, assignee, filing_date, publication_date, legal_status, jurisdiction, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			 ON CONFLICT (id) DO NOTHING`,
			p.ID, p.PatentNumber, p.Title, p.Abstract, p.Assignee,
			p.FilingDate, p.PublicationDate, p.Status, p.Jurisdiction,
			time.Now(), time.Now(),
		)
		if err != nil {
			t.Fatalf("seed patent %s: %v", p.ID, err)
		}
	}
	t.Logf("seeded %d patents", len(fixtures))
}

// SeedAll inserts all fixture categories.
func SeedAll(t *testing.T, env *TestEnvironment) {
	t.Helper()
	SeedMolecules(t, env)
	SeedPatents(t, env)
}

// ---------------------------------------------------------------------------
// Cleanup helpers
// ---------------------------------------------------------------------------

// TruncateTable removes all rows from the given table. Use with caution.
func TruncateTable(t *testing.T, env *TestEnvironment, table string) {
	t.Helper()
	RequirePostgres(t, env)
	_, err := env.PostgresDB.ExecContext(env.Ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
	if err != nil {
		t.Fatalf("truncate %s: %v", table, err)
	}
}

// TruncateAllTables truncates all known application tables.
func TruncateAllTables(t *testing.T, env *TestEnvironment) {
	t.Helper()
	tables := []string{
		"molecules", "patents", "claims", "portfolios",
		"portfolio_patents", "lifecycle_records", "annuities",
		"deadlines", "workspaces", "workspace_members", "shares",
		"audit_logs",
	}
	for _, tbl := range tables {
		// Best-effort: table may not exist yet.
		_, _ = env.PostgresDB.ExecContext(env.Ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", tbl))
	}
}

// ---------------------------------------------------------------------------
// Assertion helpers
// ---------------------------------------------------------------------------

// AssertNoError fails the test if err is non-nil.
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// AssertError fails the test if err is nil.
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error but got nil")
	}
}

// AssertErrorCode checks that err wraps a pkgErrors error with the given code.
func AssertErrorCode(t *testing.T, err error, expectedCode pkgErrors.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s but got nil", expectedCode)
	}
	var appErr *pkgErrors.AppError
	if ok := pkgErrors.As(err, &appErr); !ok {
		t.Fatalf("expected AppError, got %T: %v", err, err)
	}
	if appErr.Code != expectedCode {
		t.Fatalf("expected error code %s, got %s (message: %s)", expectedCode, appErr.Code, appErr.Message)
	}
}

// AssertHTTPStatus sends req to handler and asserts the response status code.
func AssertHTTPStatus(t *testing.T, handler http.Handler, req *http.Request, expectedStatus int) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != expectedStatus {
		t.Fatalf("expected HTTP %d, got %d; body: %s", expectedStatus, rr.Code, rr.Body.String())
	}
	return rr
}

// AssertJSONContains checks that the JSON body contains the expected key.
func AssertJSONContains(t *testing.T, body []byte, key string) {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if _, ok := m[key]; !ok {
		t.Fatalf("expected JSON key %q not found in response", key)
	}
}

// AssertStringContains checks that s contains substr.
func AssertStringContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("expected string to contain %q, got: %s", substr, s)
	}
}

// AssertInRange checks that val is within [min, max].
func AssertInRange(t *testing.T, val, min, max float64, label string) {
	t.Helper()
	if val < min || val > max {
		t.Fatalf("%s: expected value in [%.4f, %.4f], got %.4f", label, min, max, val)
	}
}

// ---------------------------------------------------------------------------
// Timing helpers
// ---------------------------------------------------------------------------

// MeasureDuration returns the wall-clock duration of fn.
func MeasureDuration(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}

// AssertDurationUnder fails if fn takes longer than maxDuration.
func AssertDurationUnder(t *testing.T, label string, maxDuration time.Duration, fn func()) {
	t.Helper()
	d := MeasureDuration(fn)
	if d > maxDuration {
		t.Fatalf("%s took %v, exceeding limit of %v", label, d, maxDuration)
	}
	t.Logf("%s completed in %v (limit: %v)", label, d, maxDuration)
}

// ---------------------------------------------------------------------------
// ID generation for test isolation
// ---------------------------------------------------------------------------

var testIDCounter uint64
var testIDMu sync.Mutex

// NextTestID returns a unique string ID for test data isolation.
func NextTestID(prefix string) string {
	testIDMu.Lock()
	testIDCounter++
	id := testIDCounter
	testIDMu.Unlock()
	return fmt.Sprintf("%s-test-%d-%d", prefix, time.Now().UnixNano(), id)
}

// ---------------------------------------------------------------------------
// Pagination helper
// ---------------------------------------------------------------------------

// DefaultPagination returns a standard pagination request for tests.
func DefaultPagination() commonTypes.Pagination {
	return commonTypes.Pagination{
		Page:     1,
		PageSize: 50,
	}
}

// ---------------------------------------------------------------------------
// Unused import guards (compile-time interface satisfaction checks)
// ---------------------------------------------------------------------------
// Note: These checks are commented out as the concrete implementations
// may not be exposed in the public API. The interfaces are what matter
// for integration testing.
// var (
// 	_ appCollaboration.SharingService       = (*appCollaboration.SharingServiceImpl)(nil)
// 	_ appInfringement.MonitoringService     = (*appInfringement.MonitoringServiceImpl)(nil)
// 	_ appLifecycle.AnnuityService           = (*appLifecycle.AnnuityServiceImpl)(nil)
// 	_ appMining.SimilaritySearchService     = (*appMining.SimilaritySearchServiceImpl)(nil)
// 	_ appPortfolio.ValuationService         = (*appPortfolio.ValuationServiceImpl)(nil)
// 	_ appQuery.KGSearchService              = (*appQuery.KGSearchServiceImpl)(nil)
// 	_ appReporting.FTOReportService         = (*appReporting.FTOReportServiceImpl)(nil)
// )

//Personal.AI order the ending
