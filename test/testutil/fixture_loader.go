// Package testutil provides shared test utilities for unit, integration, and E2E tests.
//
// This file provides fixture loading functions and type definitions that mirror
// the JSON fixture file structures under test/testdata/fixtures/.
package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// =============================================================================
// Fixture envelope types (top-level file structure)
// =============================================================================

// PatentFixtureFile represents the top-level structure of patent_fixtures.json.
type PatentFixtureFile struct {
	Version     string          `json:"version"`
	GeneratedAt time.Time       `json:"generated_at"`
	Description string          `json:"description"`
	Patents     []PatentFixture `json:"patents"`
}

// MoleculeFixtureFile represents the top-level structure of molecule_fixtures.json.
type MoleculeFixtureFile struct {
	Version     string            `json:"version"`
	GeneratedAt time.Time         `json:"generated_at"`
	Description string            `json:"description"`
	Molecules   []MoleculeFixture `json:"molecules"`
}

// PortfolioFixtureFile represents the top-level structure of portfolio_fixtures.json.
type PortfolioFixtureFile struct {
	Version     string              `json:"version"`
	GeneratedAt time.Time           `json:"generated_at"`
	Description string              `json:"description"`
	Portfolios  []PortfolioFixture  `json:"portfolios"`
}

// =============================================================================
// Fixture domain types
// =============================================================================

// PatentFixture mirrors the patent entry in patent_fixtures.json.
type PatentFixture struct {
	ID                string                    `json:"id"`
	PatentNumber      string                    `json:"patent_number"`
	Title             string                    `json:"title"`
	Abstract          string                    `json:"abstract"`
	FilingDate        string                    `json:"filing_date"`
	PublicationDate   string                    `json:"publication_date"`
	GrantDate         string                    `json:"grant_date"`
	LegalStatus       string                    `json:"legal_status"`
	Jurisdiction      string                    `json:"jurisdiction"`
	Assignees         []AssigneeFixture         `json:"assignees"`
	Inventors         []InventorFixture         `json:"inventors"`
	IPCCodes          []string                  `json:"ipc_codes"`
	Claims            []ClaimFixture            `json:"claims"`
	Molecules         []PatentMolRefFixture     `json:"molecules"`
	MarkushStructures []MarkushStructureFixture `json:"markush_structures"`
	FamilyID          string                    `json:"family_id"`
	PriorityDate      string                    `json:"priority_date"`
	Metadata          map[string]any            `json:"metadata"`
	CreatedAt         string                    `json:"created_at"`
	UpdatedAt         string                    `json:"updated_at"`
}

// AssigneeFixture represents a patent assignee in fixture data.
type AssigneeFixture struct {
	Name    string `json:"name"`
	Country string `json:"country"`
	Type    string `json:"type"`
}

// InventorFixture represents a patent inventor in fixture data.
type InventorFixture struct {
	Name    string `json:"name"`
	Country string `json:"country"`
}

// ClaimFixture represents a single patent claim in fixture data.
type ClaimFixture struct {
	Number    int                  `json:"number"`
	Type      string               `json:"type"`
	DependsOn *int                 `json:"depends_on"`
	Text      string               `json:"text"`
	Category  string               `json:"category"`
	Elements  []ClaimElementFixture `json:"elements"`
}

// ClaimElementFixture represents a claim element/limitation in fixture data.
type ClaimElementFixture struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	IsNovel     bool   `json:"is_novel"`
}

// PatentMolRefFixture references a molecule associated with a patent in fixture data.
type PatentMolRefFixture struct {
	MoleculeID string `json:"molecule_id"`
	Location   string `json:"location"`
	Role       string `json:"role"`
}

// MarkushStructureFixture represents a Markush structure in patent fixture data.
type MarkushStructureFixture struct {
	ID                  string                    `json:"id"`
	CoreSMARTS          string                    `json:"core_smarts"`
	VariableGroups      []VariableGroupFixture    `json:"variable_groups"`
	EstimatedCoverage   int64                     `json:"estimated_coverage"`
	RelatedClaimNumbers []int                     `json:"related_claim_numbers"`
}

// VariableGroupFixture represents a variable group in a Markush structure.
type VariableGroupFixture struct {
	Position string   `json:"position"`
	Options  []string `json:"options"`
}

// MoleculeFixture mirrors the molecule entry in molecule_fixtures.json.
type MoleculeFixture struct {
	ID               string              `json:"id"`
	SMILES           string              `json:"smiles"`
	InChI            string              `json:"inchi"`
	InChIKey         string              `json:"inchi_key"`
	MolecularFormula string              `json:"molecular_formula"`
	MolecularWeight  float64             `json:"molecular_weight"`
	Name             string              `json:"name"`
	Status           string              `json:"status"`
	Fingerprints     map[string]string   `json:"fingerprints"`
	Properties       []MolPropertyFixture `json:"properties"`
	Metadata         map[string]any      `json:"metadata"`
	CreatedAt        string              `json:"created_at"`
	UpdatedAt        string              `json:"updated_at"`
}

// MolPropertyFixture represents a molecular property in fixture data.
type MolPropertyFixture struct {
	Type       string  `json:"type"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	Conditions string  `json:"conditions"`
}

// PortfolioFixture mirrors the portfolio entry in portfolio_fixtures.json.
type PortfolioFixture struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	OwnerID     string              `json:"owner_id"`
	PatentIDs   []string            `json:"patent_ids"`
	TechDomains []string            `json:"tech_domains"`
	Strategy    string              `json:"strategy"`
	HealthScore map[string]any      `json:"health_score"`
	Statistics  map[string]any      `json:"statistics"`
	CreatedAt   string              `json:"created_at"`
	UpdatedAt   string              `json:"updated_at"`
}

// UserFixture represents a user entry for test fixtures.
// There is no dedicated users_fixtures.json; this type exists for
// programmatic fixture creation and for future user fixture files.
type UserFixture struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	Password  string `json:"-"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// FixtureSet aggregates all fixture types loaded from fixture files.
type FixtureSet struct {
	Patents    []PatentFixture    `json:"patents"`
	Molecules  []MoleculeFixture  `json:"molecules"`
	Portfolios []PortfolioFixture `json:"portfolios"`
	Users      []UserFixture      `json:"users"`
}

// =============================================================================
// Loader functions
// =============================================================================

func fixturePath(dir, name string) string {
	return fmt.Sprintf("%s/%s", dir, name)
}

// LoadPatents reads a patent fixture JSON file and returns the parsed patents.
// The file is expected to have the structure defined by PatentFixtureFile.
func LoadPatents(path string) ([]PatentFixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read patent fixture file: %w", err)
	}

	var envelope PatentFixtureFile
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal patent fixture: %w", err)
	}

	return envelope.Patents, nil
}

// LoadMolecules reads a molecule fixture JSON file and returns the parsed molecules.
// The file is expected to have the structure defined by MoleculeFixtureFile.
func LoadMolecules(path string) ([]MoleculeFixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read molecule fixture file: %w", err)
	}

	var envelope MoleculeFixtureFile
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal molecule fixture: %w", err)
	}

	return envelope.Molecules, nil
}

// LoadPortfolios reads a portfolio fixture JSON file and returns the parsed portfolios.
// The file is expected to have the structure defined by PortfolioFixtureFile.
func LoadPortfolios(path string) ([]PortfolioFixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read portfolio fixture file: %w", err)
	}

	var envelope PortfolioFixtureFile
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal portfolio fixture: %w", err)
	}

	return envelope.Portfolios, nil
}

// LoadUsers reads a user fixture JSON file and returns the parsed users.
// The file is expected to have the structure:
//
//	{ "version": "...", "generated_at": "...", "description": "...", "users": [...] }
//
// If the file does not exist, an empty slice is returned instead of an error,
// allowing tests to run without a users fixture file.
func LoadUsers(path string) ([]UserFixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []UserFixture{}, nil
		}
		return nil, fmt.Errorf("read user fixture file: %w", err)
	}

	var envelope struct {
		Version     string        `json:"version"`
		GeneratedAt time.Time     `json:"generated_at"`
		Description string        `json:"description"`
		Users       []UserFixture `json:"users"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal user fixture: %w", err)
	}

	return envelope.Users, nil
}

// LoadAll loads all fixture files from the given directory and returns a
// FixtureSet. Each fixture filename is inferred by convention:
//   - patent_fixtures.json
//   - molecule_fixtures.json
//   - portfolio_fixtures.json
//   - users_fixtures.json (optional)
func LoadAll(dir string) (FixtureSet, error) {
	patents, err := LoadPatents(fixturePath(dir, "patent_fixtures.json"))
	if err != nil {
		return FixtureSet{}, fmt.Errorf("load patents: %w", err)
	}

	molecules, err := LoadMolecules(fixturePath(dir, "molecule_fixtures.json"))
	if err != nil {
		return FixtureSet{}, fmt.Errorf("load molecules: %w", err)
	}

	portfolios, err := LoadPortfolios(fixturePath(dir, "portfolio_fixtures.json"))
	if err != nil {
		return FixtureSet{}, fmt.Errorf("load portfolios: %w", err)
	}

	users, err := LoadUsers(fixturePath(dir, "users_fixtures.json"))
	if err != nil {
		return FixtureSet{}, fmt.Errorf("load users: %w", err)
	}

	return FixtureSet{
		Patents:    patents,
		Molecules:  molecules,
		Portfolios: portfolios,
		Users:      users,
	}, nil
}

// MustLoadAll is like LoadAll but panics on error. Useful for one-time setup
// in TestMain or Test suite initialization.
func MustLoadAll(dir string) FixtureSet {
	fs, err := LoadAll(dir)
	if err != nil {
		panic(fmt.Sprintf("MustLoadAll(%s): %v", dir, err))
	}
	return fs
}
