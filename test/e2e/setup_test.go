// Phase 18 - E2E Test Setup
// Global test environment initialization and cleanup for end-to-end tests.
// This file bootstraps the complete application stack and provides shared
// resources to all E2E test files.
package e2e_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/pkg/client"
	_ "github.com/lib/pq"
)

// testEnv holds all shared resources for E2E tests.
type testEnv struct {
	baseURL      string
	httpClient   *http.Client
	sdkClient    *client.Client
	adminToken   string
	analystToken string
	viewerToken  string
	db           *sql.DB
	cfg          *config.Config
	cleanupFuncs []func()
}

var env *testEnv

// TestMain is the entry point for all E2E tests in this package.
func TestMain(m *testing.M) {
	var err error
	env, err = setupTestEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "E2E test setup failed: %v\n", err)
		os.Exit(1)
	}

	// Run all tests
	exitCode := m.Run()

	// Cleanup
	cleanup()

	os.Exit(exitCode)
}

// setupTestEnv initializes the test environment.
func setupTestEnv() (*testEnv, error) {
	env := &testEnv{
		cleanupFuncs: make([]func(), 0),
	}

	// Load configuration
	cfg, err := loadE2EConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	env.cfg = cfg

	// Determine base URL
	mode := os.Getenv("KEYIP_E2E_MODE")
	if mode == "" {
		mode = "external"
	}

	if mode == "embedded" {
		// For embedded mode, we would start the server here
		// For now, fall back to external mode
		mode = "external"
	}

	if mode == "external" {
		baseURL := os.Getenv("KEYIP_E2E_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}
		env.baseURL = baseURL
	}

	// Wait for service health
	if err := waitForHealthy(env.baseURL, 30*time.Second); err != nil {
		fmt.Printf("Warning: service health check failed: %v\n", err)
		// Continue anyway for offline testing
	}

	// Setup HTTP client
	env.httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	// Connect to database for verification
	dbDSN := os.Getenv("KEYIP_E2E_DB_DSN")
	if dbDSN == "" {
		dbDSN = "postgres://keyip:keyip@localhost:5432/keyip_e2e?sslmode=disable"
	}
	db, err := sql.Open("postgres", dbDSN)
	if err == nil {
		env.db = db
		env.cleanupFuncs = append(env.cleanupFuncs, func() { db.Close() })
	}

	// Load seed data (if database available)
	if env.db != nil {
		if err := loadSeedData(env); err != nil {
			// Log warning but don't fail
			fmt.Printf("Warning: failed to load seed data: %v\n", err)
		}
	}

	// Obtain authentication tokens
	adminToken, err := obtainToken(env.baseURL, "admin", "admin123", "admin")
	if err != nil {
		// Use a default token for now
		adminToken = "test-admin-token"
	}
	env.adminToken = adminToken

	analystToken, err := obtainToken(env.baseURL, "analyst", "analyst123", "analyst")
	if err != nil {
		analystToken = "test-analyst-token"
	}
	env.analystToken = analystToken

	viewerToken, err := obtainToken(env.baseURL, "viewer", "viewer123", "viewer")
	if err != nil {
		viewerToken = "test-viewer-token"
	}
	env.viewerToken = viewerToken

	// Initialize SDK client
	sdkClient, err := client.NewClient(env.baseURL, env.adminToken)
	if err != nil {
		return nil, fmt.Errorf("create SDK client: %w", err)
	}
	env.sdkClient = sdkClient

	return env, nil
}

// loadE2EConfig loads the E2E test configuration.
func loadE2EConfig() (*config.Config, error) {
	configPath := os.Getenv("KEYIP_E2E_CONFIG")
	if configPath == "" {
		// Only try the dedicated e2e config file, not example files
		paths := []string{
			"configs/config.e2e.yaml",
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				configPath = p
				break
			}
		}
	}

	if configPath != "" {
		return config.Load(config.WithConfigPath(configPath))
	}

	// Use default config with minimal required values for test environment
	cfg := config.NewDefaultConfig()
	// Set minimal required values for e2e tests to avoid validation errors
	cfg.Database.Postgres.Host = "localhost"
	cfg.Database.Postgres.User = "keyip"
	cfg.Database.Postgres.Password = "keyip"
	cfg.Database.Postgres.DBName = "keyip_e2e"
	cfg.Database.Neo4j.URI = "bolt://localhost:7687"
	cfg.Database.Neo4j.User = "neo4j"
	cfg.Database.Neo4j.Password = "neo4j"
	cfg.Cache.Redis.Addr = "localhost:6379"
	cfg.Search.OpenSearch.Addresses = []string{"http://localhost:9200"}
	cfg.Search.Milvus.Address = "localhost:19530"
	cfg.Messaging.Kafka.Brokers = []string{"localhost:9092"}
	cfg.Messaging.Kafka.ConsumerGroup = "keyip-e2e"
	cfg.Storage.MinIO.Endpoint = "localhost:9000"
	cfg.Storage.MinIO.AccessKey = "minioadmin"
	cfg.Storage.MinIO.SecretKey = "minioadmin"
	cfg.Storage.MinIO.BucketName = "keyip-e2e"
	cfg.Auth.Keycloak.BaseURL = "http://localhost:8090"
	cfg.Auth.Keycloak.Realm = "keyip"
	cfg.Auth.Keycloak.ClientID = "keyip-client"
	cfg.Auth.Keycloak.ClientSecret = "test-secret"
	cfg.Auth.JWT.Secret = "test-jwt-secret-for-e2e-testing"
	cfg.Auth.JWT.Issuer = "keyip-e2e"
	cfg.Intelligence.ModelsDir = "/tmp/models"
	cfg.Intelligence.MolPatentGNN.ModelPath = "/tmp/models/molpatent_gnn"
	cfg.Intelligence.ClaimBERT.ModelPath = "/tmp/models/claim_bert"
	cfg.Intelligence.ClaimBERT.Device = "cpu"
	cfg.Intelligence.StrategyGPT.Endpoint = "http://localhost:8000/v1"
	cfg.Intelligence.StrategyGPT.APIKey = "test-api-key"
	cfg.Intelligence.StrategyGPT.ModelName = "gpt-4"
	cfg.Intelligence.ChemExtractor.OCREndpoint = "http://localhost:8001"
	cfg.Intelligence.ChemExtractor.NERModelPath = "/tmp/models/ner"
	cfg.Intelligence.InfringeNet.ModelPath = "/tmp/models/infringe_net"
	return cfg, nil
}

// waitForHealthy polls the health endpoint until it returns OK.
func waitForHealthy(baseURL string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{Timeout: 5 * time.Second}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	healthURL := baseURL + "/healthz"
	var lastErr error

	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("health check timeout: %w", lastErr)
			}
			return fmt.Errorf("health check timeout")

		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
			if err != nil {
				lastErr = err
				continue
			}

			resp, err := client.Do(req)
			if err != nil {
				lastErr = err
				continue
			}

			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
					if status, ok := result["status"].(string); ok && status == "ok" {
						resp.Body.Close()
						return nil
					}
				}
			}
			resp.Body.Close()
			lastErr = fmt.Errorf("unhealthy response: status=%d", resp.StatusCode)
		}
	}
}

// obtainToken retrieves an authentication token for the given user.
func obtainToken(baseURL, username, password, role string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	
	reqBody := map[string]string{
		"grant_type": "password",
		"username":   username,
		"password":   password,
	}
	
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	resp, err := client.Post(baseURL+"/api/v1/auth/token", "application/json", 
		bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth failed: status=%d", resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.AccessToken == "" {
		return "", fmt.Errorf("empty access token")
	}

	return result.AccessToken, nil
}

// loadSeedData injects fixture data into the test database.
func loadSeedData(env *testEnv) error {
	// Clean first
	_ = cleanDatabase(env)

	// Load molecule fixtures
	if err := loadMoleculeSeeds(env); err != nil {
		return fmt.Errorf("load molecule seeds: %w", err)
	}

	// Load patent fixtures
	if err := loadPatentSeeds(env); err != nil {
		return fmt.Errorf("load patent seeds: %w", err)
	}

	// Load portfolio fixtures
	if err := loadPortfolioSeeds(env); err != nil{
		return fmt.Errorf("load portfolio seeds: %w", err)
	}

	return nil
}

// loadMoleculeSeeds loads molecule fixture data.
func loadMoleculeSeeds(env *testEnv) error {
	fixturePath := "test/testdata/fixtures/molecule_fixtures.json"
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		// File may not exist, skip
		return nil
	}

	var molecules []map[string]interface{}
	if err := json.Unmarshal(data, &molecules); err != nil {
		return err
	}

	// Insert into database (simplified)
	for _, m := range molecules {
		// Use SDK client or direct DB insertion
		_ = m // TODO: implement actual insertion
	}

	return nil
}

// loadPatentSeeds loads patent fixture data.
func loadPatentSeeds(env *testEnv) error {
	fixturePath := "test/testdata/fixtures/patent_fixtures.json"
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		return nil
	}

	var patents []map[string]interface{}
	if err := json.Unmarshal(data, &patents); err != nil {
		return err
	}

	for _, p := range patents {
		_ = p // TODO: implement actual insertion
	}

	return nil
}

// loadPortfolioSeeds loads portfolio fixture data.
func loadPortfolioSeeds(env *testEnv) error {
	fixturePath := "test/testdata/fixtures/portfolio_fixtures.json"
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		return nil
	}

	var portfolios []map[string]interface{}
	if err := json.Unmarshal(data, &portfolios); err != nil {
		return err
	}

	for _, p := range portfolios {
		_ = p // TODO: implement actual insertion
	}

	return nil
}

// cleanDatabase removes all test data from the database.
func cleanDatabase(env *testEnv) error {
	if env.db == nil {
		return nil
	}

	tables := []string{
		"shares",
		"workspace_members",
		"workspaces",
		"portfolio_patents",
		"portfolios",
		"lifecycle_events",
		"claims",
		"patents",
		"molecules",
		"users",
	}

	for _, table := range tables {
		_, err := env.db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			// Table may not exist, continue
			continue
		}
	}

	return nil
}

// registerCleanup adds a cleanup function to be called at test teardown.
func registerCleanup(fn func()) {
	if env != nil {
		env.cleanupFuncs = append(env.cleanupFuncs, fn)
	}
}

// cleanup executes all registered cleanup functions.
func cleanup() {
	if env == nil {
		return
	}

	// Clean database
	_ = cleanDatabase(env)

	// Execute cleanup functions in reverse order
	for i := len(env.cleanupFuncs) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Cleanup panic: %v\n", r)
				}
			}()
			env.cleanupFuncs[i]()
		}()
	}
}

//Personal.AI order the ending

