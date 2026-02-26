// Phase 13 - SDK Patents Sub-Client (300/349)
// File: pkg/client/patents.go
// Patent search, analysis, valuation, landscape and citation APIs.

package client

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// DateRange filters by a date field within [From, To].
type DateRange struct {
	Field string `json:"field,omitempty"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
}

// PatentSearchRequest describes a patent search query.
type PatentSearchRequest struct {
	Query         string     `json:"query"`
	QueryType     string     `json:"query_type"`
	Jurisdictions []string   `json:"jurisdictions,omitempty"`
	DateRange     *DateRange `json:"date_range,omitempty"`
	Applicants    []string   `json:"applicants,omitempty"`
	Inventors     []string   `json:"inventors,omitempty"`
	IPCCodes      []string   `json:"ipc_codes,omitempty"`
	CPCCodes      []string   `json:"cpc_codes,omitempty"`
	Status        []string   `json:"status,omitempty"`
	HasMolecule   bool       `json:"has_molecule,omitempty"`
	Page          int        `json:"page"`
	PageSize      int        `json:"page_size"`
	SortBy        string     `json:"sort_by,omitempty"`
	SortOrder     string     `json:"sort_order"`
}

// PatentSearchResult is the response envelope for patent search.
type PatentSearchResult struct {
	Patents    []Patent      `json:"patents"`
	Total      int64         `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	HasMore    bool          `json:"has_more"`
	SearchTime float64       `json:"search_time"`
	Facets     *SearchFacets `json:"facets,omitempty"`
}

// SearchFacets contains aggregation buckets.
type SearchFacets struct {
	Jurisdictions []FacetItem `json:"jurisdictions,omitempty"`
	Years         []FacetItem `json:"years,omitempty"`
	Applicants    []FacetItem `json:"applicants,omitempty"`
	IPCCodes      []FacetItem `json:"ipc_codes,omitempty"`
	Status        []FacetItem `json:"status,omitempty"`
}

// FacetItem is a single aggregation bucket.
type FacetItem struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// Patent is the summary representation returned in search results.
type Patent struct {
	ID              string   `json:"id"`
	PatentNumber    string   `json:"patent_number"`
	Title           string   `json:"title"`
	Abstract        string   `json:"abstract"`
	Applicants      []string `json:"applicants"`
	Inventors       []string `json:"inventors"`
	Assignees       []string `json:"assignees"`
	Jurisdiction    string   `json:"jurisdiction"`
	FilingDate      string   `json:"filing_date"`
	PublicationDate string   `json:"publication_date"`
	GrantDate       string   `json:"grant_date,omitempty"`
	PriorityDate    string   `json:"priority_date"`
	IPCCodes        []string `json:"ipc_codes"`
	CPCCodes        []string `json:"cpc_codes"`
	Status          string   `json:"status"`
	MoleculeCount   int      `json:"molecule_count"`
	CitationCount   int      `json:"citation_count"`
	RelevanceScore  float64  `json:"relevance_score,omitempty"`
	PatentValue     float64  `json:"patent_value,omitempty"`
}

// PatentDetail extends Patent with full-text content and relations.
type PatentDetail struct {
	Patent
	Claims        []Claim         `json:"claims"`
	Description   string          `json:"description"`
	Drawings      []Drawing       `json:"drawings"`
	FamilyID      string          `json:"family_id"`
	FamilyMembers []FamilyMember  `json:"family_members"`
	Citations     []Citation      `json:"citations"`
	CitedBy       []Citation      `json:"cited_by"`
	Molecules     []MoleculeBrief `json:"molecules"`
	LegalEvents   []LegalEvent    `json:"legal_events"`
	FullTextURL   string          `json:"full_text_url"`
}

// Claim represents a single patent claim.
type Claim struct {
	Number    int    `json:"number"`
	Text      string `json:"text"`
	Type      string `json:"type"`
	DependsOn []int  `json:"depends_on,omitempty"`
}

// Drawing represents a patent drawing.
type Drawing struct {
	Number      int    `json:"number"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// FamilyMember represents a member of a patent family.
type FamilyMember struct {
	PatentNumber    string `json:"patent_number"`
	Jurisdiction    string `json:"jurisdiction"`
	Status          string `json:"status"`
	FilingDate      string `json:"filing_date"`
	PublicationDate string `json:"publication_date"`
}

// Citation represents a patent citation relationship.
type Citation struct {
	PatentNumber string `json:"patent_number"`
	Title        string `json:"title"`
	CitationType string `json:"citation_type"`
	Relevance    string `json:"relevance"`
}

// MoleculeBrief is a compact molecule representation within patent context.
type MoleculeBrief struct {
	ID               string  `json:"id"`
	SMILES           string  `json:"smiles"`
	MolecularFormula string  `json:"molecular_formula"`
	MolecularWeight  float64 `json:"molecular_weight"`
	Relevance        float64 `json:"relevance"`
}

// LegalEvent represents a legal status change event.
type LegalEvent struct {
	Date        string `json:"date"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

// PatentValueRequest describes a batch patent valuation request.
type PatentValueRequest struct {
	PatentNumbers []string `json:"patent_numbers"`
}

// PatentValueResult is the response for patent valuation.
type PatentValueResult struct {
	Evaluations []PatentValuation `json:"evaluations"`
}

// PatentValuation holds scores for a single patent.
type PatentValuation struct {
	PatentNumber   string            `json:"patent_number"`
	OverallScore   float64           `json:"overall_score"`
	TechnicalScore float64           `json:"technical_score"`
	LegalScore     float64           `json:"legal_score"`
	MarketScore    float64           `json:"market_score"`
	CitationScore  float64           `json:"citation_score"`
	FamilyScore    float64           `json:"family_score"`
	RemainingLife  float64           `json:"remaining_life"`
	Factors        []ValuationFactor `json:"factors"`
}

// ValuationFactor is a single scoring dimension.
type ValuationFactor struct {
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
}

// PatentLandscapeRequest describes a landscape analysis request.
type PatentLandscapeRequest struct {
	Query         string     `json:"query"`
	Jurisdictions []string   `json:"jurisdictions,omitempty"`
	DateRange     *DateRange `json:"date_range,omitempty"`
	TopApplicants int        `json:"top_applicants"`
	TopIPCCodes   int        `json:"top_ipc_codes"`
}

// PatentLandscape is the response for landscape analysis.
type PatentLandscape struct {
	TotalPatents             int64              `json:"total_patents"`
	YearlyTrend              []YearCount        `json:"yearly_trend"`
	TopApplicants            []ApplicantStats   `json:"top_applicants"`
	TopIPCCodes              []IPCStats         `json:"top_ipc_codes"`
	JurisdictionDistribution []FacetItem        `json:"jurisdiction_distribution"`
	StatusDistribution       []FacetItem        `json:"status_distribution"`
	TechnologyClusters       []TechnologyCluster `json:"technology_clusters"`
}

// YearCount is a year-count pair.
type YearCount struct {
	Year  int   `json:"year"`
	Count int64 `json:"count"`
}

// ApplicantStats holds filing statistics for an applicant.
type ApplicantStats struct {
	Name        string   `json:"name"`
	PatentCount int64    `json:"patent_count"`
	FirstFiling string   `json:"first_filing"`
	LastFiling  string   `json:"last_filing"`
	TopIPCCodes []string `json:"top_ipc_codes"`
}

// IPCStats holds statistics for an IPC code.
type IPCStats struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Count       int64  `json:"count"`
}

// TechnologyCluster represents a cluster of related patents.
type TechnologyCluster struct {
	ID            string   `json:"id"`
	Label         string   `json:"label"`
	PatentCount   int64    `json:"patent_count"`
	Keywords      []string `json:"keywords"`
	CentralPatent string   `json:"central_patent"`
}

// ---------------------------------------------------------------------------
// Internal response wrappers
// ---------------------------------------------------------------------------

type patentDetailResp struct {
	Data PatentDetail `json:"data"`
}

type familyMemberListResp struct {
	Data []FamilyMember `json:"data"`
}

type citationListResp struct {
	Data []Citation `json:"data"`
}

type moleculeBriefListResp struct {
	Data []MoleculeBrief `json:"data"`
	Meta *ResponseMeta   `json:"meta,omitempty"`
}

type legalEventListResp struct {
	Data []LegalEvent `json:"data"`
}

type claimListResp struct {
	Data []Claim `json:"data"`
}

type patentValueResp struct {
	Data PatentValueResult `json:"data"`
}

type patentLandscapeResp struct {
	Data PatentLandscape `json:"data"`
}

// ---------------------------------------------------------------------------
// PatentsClient
// ---------------------------------------------------------------------------

// PatentsClient provides access to patent search and analysis endpoints.
type PatentsClient struct {
	client *Client
}

func newPatentsClient(c *Client) *PatentsClient {
	return &PatentsClient{client: c}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func clampPageSize(v int) int {
	if v < 1 {
		return 1
	}
	if v > 100 {
		return 100
	}
	return v
}

// validateDateRange checks that From <= To when both are present.
func validateDateRange(dr *DateRange) error {
	if dr == nil {
		return nil
	}
	if dr.From == "" || dr.To == "" {
		return nil
	}
	const layout = "2006-01-02"
	from, errF := time.Parse(layout, dr.From)
	to, errT := time.Parse(layout, dr.To)
	if errF != nil || errT != nil {
		// If dates don't parse as date-only, try RFC3339.
		from, errF = time.Parse(time.RFC3339, dr.From)
		to, errT = time.Parse(time.RFC3339, dr.To)
		if errF != nil || errT != nil {
			return nil // let the server validate unparseable dates
		}
	}
	if from.After(to) {
		return invalidArg("date_range.from must not be after date_range.to")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Public methods
// ---------------------------------------------------------------------------

// Search performs a patent search.
// POST /api/v1/patents/search
func (pc *PatentsClient) Search(ctx context.Context, req *PatentSearchRequest) (*PatentSearchResult, error) {
	if req == nil || req.Query == "" {
		return nil, invalidArg("query is required")
	}
	if err := validateDateRange(req.DateRange); err != nil {
		return nil, err
	}
	// defaults
	if req.QueryType == "" {
		req.QueryType = "keyword"
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	req.PageSize = clampPageSize(req.PageSize)
	if req.SortOrder == "" {
		req.SortOrder = "desc"
	}

	var result PatentSearchResult
	if err := pc.client.post(ctx, "/api/v1/patents/search", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Get retrieves a patent by internal ID.
// GET /api/v1/patents/{patentID}
func (pc *PatentsClient) Get(ctx context.Context, patentID string) (*PatentDetail, error) {
	if patentID == "" {
		return nil, invalidArg("patentID is required")
	}
	var resp patentDetailResp
	if err := pc.client.get(ctx, "/api/v1/patents/"+patentID, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetByNumber retrieves a patent by its publication number.
// GET /api/v1/patents/by-number?number={patentNumber}
func (pc *PatentsClient) GetByNumber(ctx context.Context, patentNumber string) (*PatentDetail, error) {
	if patentNumber == "" {
		return nil, invalidArg("patentNumber is required")
	}
	path := "/api/v1/patents/by-number?number=" + url.QueryEscape(patentNumber)
	var resp patentDetailResp
	if err := pc.client.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetFamily retrieves the patent family members.
// GET /api/v1/patents/{patentNumber}/family
func (pc *PatentsClient) GetFamily(ctx context.Context, patentNumber string) ([]FamilyMember, error) {
	if patentNumber == "" {
		return nil, invalidArg("patentNumber is required")
	}
	var resp familyMemberListResp
	if err := pc.client.get(ctx, "/api/v1/patents/"+url.PathEscape(patentNumber)+"/family", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetCitations retrieves patents cited by the given patent.
// GET /api/v1/patents/{patentNumber}/citations
func (pc *PatentsClient) GetCitations(ctx context.Context, patentNumber string) ([]Citation, error) {
	if patentNumber == "" {
		return nil, invalidArg("patentNumber is required")
	}
	var resp citationListResp
	if err := pc.client.get(ctx, "/api/v1/patents/"+url.PathEscape(patentNumber)+"/citations", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetCitedBy retrieves patents that cite the given patent.
// GET /api/v1/patents/{patentNumber}/cited-by
func (pc *PatentsClient) GetCitedBy(ctx context.Context, patentNumber string) ([]Citation, error) {
	if patentNumber == "" {
		return nil, invalidArg("patentNumber is required")
	}
	var resp citationListResp
	if err := pc.client.get(ctx, "/api/v1/patents/"+url.PathEscape(patentNumber)+"/cited-by", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetMolecules retrieves molecules associated with a patent.
// GET /api/v1/patents/{patentNumber}/molecules?page=&page_size=
func (pc *PatentsClient) GetMolecules(ctx context.Context, patentNumber string, page, pageSize int) ([]MoleculeBrief, int64, error) {
	if patentNumber == "" {
		return nil, 0, invalidArg("patentNumber is required")
	}
	if page <= 0 {
		page = 1
	}
	pageSize = clampPageSize(pageSize)
	path := fmt.Sprintf("/api/v1/patents/%s/molecules?page=%d&page_size=%d",
		url.PathEscape(patentNumber), page, pageSize)
	var resp moleculeBriefListResp
	if err := pc.client.get(ctx, path, &resp); err != nil {
		return nil, 0, err
	}
	var total int64
	if resp.Meta != nil {
		total = int64(resp.Meta.Total)
	}
	return resp.Data, total, nil
}

// EvaluateValue performs batch patent valuation.
// POST /api/v1/patents/evaluate
func (pc *PatentsClient) EvaluateValue(ctx context.Context, req *PatentValueRequest) (*PatentValueResult, error) {
	if req == nil || len(req.PatentNumbers) == 0 {
		return nil, invalidArg("patent_numbers is required")
	}
	if len(req.PatentNumbers) > 100 {
		return nil, invalidArg("patent_numbers exceeds maximum of 100")
	}
	var resp patentValueResp
	if err := pc.client.post(ctx, "/api/v1/patents/evaluate", req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetLandscape performs a patent landscape analysis.
// POST /api/v1/patents/landscape
func (pc *PatentsClient) GetLandscape(ctx context.Context, req *PatentLandscapeRequest) (*PatentLandscape, error) {
	if req == nil || req.Query == "" {
		return nil, invalidArg("query is required")
	}
	if req.TopApplicants <= 0 {
		req.TopApplicants = 20
	}
	if req.TopIPCCodes <= 0 {
		req.TopIPCCodes = 20
	}
	var resp patentLandscapeResp
	if err := pc.client.post(ctx, "/api/v1/patents/landscape", req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetLegalEvents retrieves legal events for a patent.
// GET /api/v1/patents/{patentNumber}/legal-events
func (pc *PatentsClient) GetLegalEvents(ctx context.Context, patentNumber string) ([]LegalEvent, error) {
	if patentNumber == "" {
		return nil, invalidArg("patentNumber is required")
	}
	var resp legalEventListResp
	if err := pc.client.get(ctx, "/api/v1/patents/"+url.PathEscape(patentNumber)+"/legal-events", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetClaims retrieves claims for a patent.
// GET /api/v1/patents/{patentNumber}/claims
func (pc *PatentsClient) GetClaims(ctx context.Context, patentNumber string) ([]Claim, error) {
	if patentNumber == "" {
		return nil, invalidArg("patentNumber is required")
	}
	var resp claimListResp
	if err := pc.client.get(ctx, "/api/v1/patents/"+url.PathEscape(patentNumber)+"/claims", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// SemanticSearch is a convenience wrapper that performs a semantic patent search.
// POST /api/v1/patents/semantic-search
func (pc *PatentsClient) SemanticSearch(ctx context.Context, text string, page, pageSize int) (*PatentSearchResult, error) {
	if text == "" {
		return nil, invalidArg("text is required")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	pageSize = clampPageSize(pageSize)

	req := &PatentSearchRequest{
		Query:     text,
		QueryType: "semantic",
		Page:      page,
		PageSize:  pageSize,
		SortOrder: "desc",
	}
	var result PatentSearchResult
	if err := pc.client.post(ctx, "/api/v1/patents/semantic-search", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

//Personal.AI order the ending
