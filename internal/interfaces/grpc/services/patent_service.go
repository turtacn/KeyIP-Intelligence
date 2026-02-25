package services

import (
	"context"
	"log"
	"regexp"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// PatentApplicationService 专利应用服务接口
type PatentApplicationService interface {
	GetByNumber(ctx context.Context, number string) (*Patent, error)
	Search(ctx context.Context, opts *PatentSearchOptions) (*PatentSearchResult, error)
	AnalyzeClaims(ctx context.Context, patentNumber string) (*ClaimAnalysis, error)
	GetFamily(ctx context.Context, patentNumber string) ([]*Patent, error)
	GetCitationNetwork(ctx context.Context, patentNumber string, depth int) (*CitationNetwork, error)
}

// FTOReportService FTO 报告服务接口
type FTOReportService interface {
	QuickCheck(ctx context.Context, smiles string, jurisdictions []string) (*FTOCheckResult, error)
}

// PatentServiceLogger 专利服务日志接口
type PatentServiceLogger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// defaultPatentLogger 默认日志实现
type defaultPatentLogger struct{}

func (l *defaultPatentLogger) Info(msg string, fields ...interface{}) {
	log.Printf("[INFO] %s %v", msg, fields)
}
func (l *defaultPatentLogger) Error(msg string, fields ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, fields)
}
func (l *defaultPatentLogger) Debug(msg string, fields ...interface{}) {
	log.Printf("[DEBUG] %s %v", msg, fields)
}

// PatentServiceServer 实现 gRPC PatentService 接口
type PatentServiceServer struct {
	UnimplementedPatentServiceServer
	patentApp  PatentApplicationService
	ftoService FTOReportService
	logger     PatentServiceLogger
}

// NewPatentServiceServer 创建专利服务
func NewPatentServiceServer(patentApp PatentApplicationService, ftoService FTOReportService, logger PatentServiceLogger) *PatentServiceServer {
	if logger == nil {
		logger = &defaultPatentLogger{}
	}
	return &PatentServiceServer{
		patentApp:  patentApp,
		ftoService: ftoService,
		logger:     logger,
	}
}

// PatentService 向后兼容的别名
type PatentService = PatentServiceServer

// NewPatentService 向后兼容的创建函数
func NewPatentService() *PatentService {
	return NewPatentServiceServer(nil, nil, nil)
}

// GetPatent 获取专利
func (s *PatentServiceServer) GetPatent(ctx context.Context, req *GetPatentRequest) (*GetPatentResponse, error) {
	if req == nil || req.GetNumber() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "patent_number is required")
	}

	// 专利号格式校验
	if !isValidPatentNumber(req.GetNumber()) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid patent number format")
	}

	ctx = s.extractPatentContext(ctx)
	s.logger.Debug("GetPatent called", "number", req.GetNumber())

	if s.patentApp == nil {
		// 模拟返回
		return &GetPatentResponse{
			Patent: &PatentProto{
				Number:       req.GetNumber(),
				Title:        "OLED Material Composition",
				Abstract:     "A novel organic light-emitting material...",
				FilingDate:   "2021-01-15",
				Status:       "Granted",
				Jurisdiction: extractJurisdiction(req.GetNumber()),
			},
		}, nil
	}

	patent, err := s.patentApp.GetByNumber(ctx, req.GetNumber())
	if err != nil {
		return nil, mapPatentError(err)
	}

	return &GetPatentResponse{
		Patent: patentDomainToProto(patent),
	}, nil
}

// SearchPatents 搜索专利
func (s *PatentServiceServer) SearchPatents(ctx context.Context, req *SearchPatentsProtoRequest) (*SearchPatentsProtoResponse, error) {
	if req == nil || req.Query == "" {
		return nil, status.Errorf(codes.InvalidArgument, "query is required")
	}

	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		return nil, status.Errorf(codes.InvalidArgument, "page_size must be between 1 and 100")
	}

	ctx = s.extractPatentContext(ctx)
	s.logger.Debug("SearchPatents called", "query", req.Query)

	if s.patentApp == nil {
		// 模拟返回
		return &SearchPatentsProtoResponse{
			Results: []*PatentProto{
				{
					Number:       "CN202110123456",
					Title:        "OLED Material Composition",
					Abstract:     "A novel organic light-emitting material...",
					FilingDate:   "2021-01-15",
					Status:       "Granted",
					Jurisdiction: "CN",
				},
			},
			TotalCount: 1,
		}, nil
	}

	opts := &PatentSearchOptions{
		Query:         req.Query,
		PageSize:      pageSize,
		IPCCodes:      req.IpcCodes,
		CPCCodes:      req.CpcCodes,
		Jurisdictions: req.Jurisdictions,
		DateFrom:      req.DateFrom,
		DateTo:        req.DateTo,
		SortBy:        req.SortBy,
	}

	result, err := s.patentApp.Search(ctx, opts)
	if err != nil {
		return nil, mapPatentError(err)
	}

	patents := make([]*PatentProto, 0, len(result.Patents))
	for _, p := range result.Patents {
		patents = append(patents, patentDomainToProto(p))
	}

	return &SearchPatentsProtoResponse{
		Results:       patents,
		TotalCount:   int32(result.TotalCount),
		NextPageToken: result.NextPageToken,
	}, nil
}

// AnalyzeClaims 权利要求解析
func (s *PatentServiceServer) AnalyzeClaims(ctx context.Context, req *AnalyzeClaimsRequest) (*AnalyzeClaimsResponse, error) {
	if req == nil || req.PatentNumber == "" {
		return nil, status.Errorf(codes.InvalidArgument, "patent_number is required")
	}

	ctx = s.extractPatentContext(ctx)
	s.logger.Debug("AnalyzeClaims called", "number", req.PatentNumber)

	if s.patentApp == nil {
		// 模拟返回
		return &AnalyzeClaimsResponse{
			ClaimTree: &ClaimTreeProto{
				IndependentClaims: []*ClaimProto{
					{
						Number:   1,
						Text:     "A compound having the formula...",
						Type:     "independent",
						Features: []string{"organic molecule", "OLED material"},
					},
				},
				TotalClaims: 10,
			},
		}, nil
	}

	analysis, err := s.patentApp.AnalyzeClaims(ctx, req.PatentNumber)
	if err != nil {
		return nil, mapPatentError(err)
	}

	return &AnalyzeClaimsResponse{
		ClaimTree: claimAnalysisToProto(analysis),
	}, nil
}

// CheckFTO FTO 检查
func (s *PatentServiceServer) CheckFTO(ctx context.Context, req *CheckFTORequest) (*CheckFTOResponse, error) {
	if req == nil || req.Smiles == "" {
		return nil, status.Errorf(codes.InvalidArgument, "SMILES is required")
	}

	if !isValidSMILES(req.Smiles) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid SMILES format")
	}

	// 30 秒超时
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	s.logger.Debug("CheckFTO called", "smiles", req.Smiles)

	if s.ftoService == nil {
		// 模拟返回
		return &CheckFTOResponse{
			RiskLevel:    "LOW",
			Confidence:   0.85,
			BlockingPatents: []*PatentProto{},
			Summary:      "No significant FTO risks identified",
		}, nil
	}

	result, err := s.ftoService.QuickCheck(ctx, req.Smiles, req.Jurisdictions)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, status.Errorf(codes.DeadlineExceeded, "FTO check timed out")
		}
		return nil, mapPatentError(err)
	}

	blocking := make([]*PatentProto, 0, len(result.BlockingPatents))
	for _, p := range result.BlockingPatents {
		blocking = append(blocking, patentDomainToProto(p))
	}

	return &CheckFTOResponse{
		RiskLevel:       result.RiskLevel,
		Confidence:      result.Confidence,
		BlockingPatents: blocking,
		Summary:         result.Summary,
	}, nil
}

// GetPatentFamily 获取同族专利
func (s *PatentServiceServer) GetPatentFamily(ctx context.Context, req *GetPatentFamilyRequest) (*GetPatentFamilyResponse, error) {
	if req == nil || req.PatentNumber == "" {
		return nil, status.Errorf(codes.InvalidArgument, "patent_number is required")
	}

	ctx = s.extractPatentContext(ctx)
	s.logger.Debug("GetPatentFamily called", "number", req.PatentNumber)

	if s.patentApp == nil {
		// 模拟返回
		return &GetPatentFamilyResponse{
			FamilyMembers: []*PatentProto{
				{Number: req.PatentNumber, Jurisdiction: extractJurisdiction(req.PatentNumber)},
				{Number: "US20210123456", Jurisdiction: "US"},
				{Number: "EP3456789", Jurisdiction: "EP"},
			},
			FamilyId: "INPADOC-12345",
		}, nil
	}

	family, err := s.patentApp.GetFamily(ctx, req.PatentNumber)
	if err != nil {
		return nil, mapPatentError(err)
	}

	members := make([]*PatentProto, 0, len(family))
	for _, p := range family {
		members = append(members, patentDomainToProto(p))
	}

	return &GetPatentFamilyResponse{
		FamilyMembers: members,
		FamilyId:      "INPADOC-" + req.PatentNumber,
	}, nil
}

// GetCitationNetwork 获取引用网络
func (s *PatentServiceServer) GetCitationNetwork(ctx context.Context, req *GetCitationNetworkRequest) (*GetCitationNetworkResponse, error) {
	if req == nil || req.PatentNumber == "" {
		return nil, status.Errorf(codes.InvalidArgument, "patent_number is required")
	}

	depth := int(req.Depth)
	if depth <= 0 {
		depth = 2 // 默认深度
	}
	if depth > 5 {
		return nil, status.Errorf(codes.InvalidArgument, "depth must be between 1 and 5")
	}

	ctx = s.extractPatentContext(ctx)
	s.logger.Debug("GetCitationNetwork called", "number", req.PatentNumber, "depth", depth)

	if s.patentApp == nil {
		// 模拟返回
		return &GetCitationNetworkResponse{
			Nodes: []*CitationNodeProto{
				{PatentNumber: req.PatentNumber, Depth: 0},
				{PatentNumber: "US20200123456", Depth: 1, CitationType: "backward"},
				{PatentNumber: "CN202210123456", Depth: 1, CitationType: "forward"},
			},
			Edges: []*CitationEdgeProto{
				{From: req.PatentNumber, To: "US20200123456", Type: "backward"},
				{From: "CN202210123456", To: req.PatentNumber, Type: "forward"},
			},
			TotalNodes:  3,
			IsTruncated: false,
		}, nil
	}

	network, err := s.patentApp.GetCitationNetwork(ctx, req.PatentNumber, depth)
	if err != nil {
		return nil, mapPatentError(err)
	}

	nodes := make([]*CitationNodeProto, 0, len(network.Nodes))
	for _, n := range network.Nodes {
		nodes = append(nodes, &CitationNodeProto{
			PatentNumber: n.PatentNumber,
			Depth:        int32(n.Depth),
			CitationType: n.CitationType,
		})
	}

	edges := make([]*CitationEdgeProto, 0, len(network.Edges))
	for _, e := range network.Edges {
		edges = append(edges, &CitationEdgeProto{
			From: e.From,
			To:   e.To,
			Type: e.Type,
		})
	}

	// 大型网络截断（超过 1000 节点）
	isTruncated := len(nodes) > 1000
	if isTruncated {
		nodes = nodes[:1000]
	}

	return &GetCitationNetworkResponse{
		Nodes:       nodes,
		Edges:       edges,
		TotalNodes:  int32(network.TotalNodes),
		IsTruncated: isTruncated,
	}, nil
}

// 向后兼容方法
func (s *PatentServiceServer) GetPatentLegacy(ctx context.Context, req *GetPatentRequestLegacy) (*PatentResponseLegacy, error) {
	resp, err := s.GetPatent(ctx, &GetPatentRequest{Number: req.GetNumber()})
	if err != nil {
		return nil, err
	}
	return &PatentResponseLegacy{
		Number:       resp.Patent.Number,
		Title:        resp.Patent.Title,
		Abstract:     resp.Patent.Abstract,
		FilingDate:   resp.Patent.FilingDate,
		Status:       resp.Patent.Status,
		Jurisdiction: resp.Patent.Jurisdiction,
	}, nil
}

func (s *PatentServiceServer) SearchPatentsLegacy(ctx context.Context, req *SearchPatentsRequestLegacy) (*SearchPatentsResponseLegacy, error) {
	resp, err := s.SearchPatents(ctx, &SearchPatentsProtoRequest{Query: req.Query})
	if err != nil {
		return nil, err
	}

	results := make([]*PatentResponseLegacy, 0, len(resp.Results))
	for _, p := range resp.Results {
		results = append(results, &PatentResponseLegacy{
			Number:       p.Number,
			Title:        p.Title,
			Abstract:     p.Abstract,
			FilingDate:   p.FilingDate,
			Status:       p.Status,
			Jurisdiction: p.Jurisdiction,
		})
	}

	return &SearchPatentsResponseLegacy{
		Results:    results,
		TotalCount: resp.TotalCount,
	}, nil
}

func (s *PatentServiceServer) AnalyzeInfringement(ctx context.Context, req *InfringementRequestLegacy) (*InfringementResponseLegacy, error) {
	// 模拟侵权分析
	return &InfringementResponseLegacy{
		RiskLevel:  "MEDIUM",
		Confidence: 0.75,
		Details:    "Potential overlap in claim 1...",
	}, nil
}

// extractPatentContext 从 gRPC metadata 提取上下文
func (s *PatentServiceServer) extractPatentContext(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	if tenantIDs := md.Get("x-tenant-id"); len(tenantIDs) > 0 {
		ctx = context.WithValue(ctx, "tenant_id", tenantIDs[0])
	}
	if userIDs := md.Get("x-user-id"); len(userIDs) > 0 {
		ctx = context.WithValue(ctx, "user_id", userIDs[0])
	}

	return ctx
}

// patentDomainToProto 将专利领域模型转换为 Protobuf
func patentDomainToProto(patent *Patent) *PatentProto {
	if patent == nil {
		return nil
	}
	return &PatentProto{
		Number:       patent.Number,
		Title:        patent.Title,
		Abstract:     patent.Abstract,
		FilingDate:   patent.FilingDate,
		Status:       patent.Status,
		Jurisdiction: patent.Jurisdiction,
		Applicant:    patent.Applicant,
		Inventor:     patent.Inventor,
		IPCCodes:     patent.IPCCodes,
		CPCCodes:     patent.CPCCodes,
	}
}

// claimAnalysisToProto 将权利要求分析转换为 Protobuf
func claimAnalysisToProto(analysis *ClaimAnalysis) *ClaimTreeProto {
	if analysis == nil {
		return nil
	}

	independentClaims := make([]*ClaimProto, 0, len(analysis.IndependentClaims))
	for _, c := range analysis.IndependentClaims {
		independentClaims = append(independentClaims, &ClaimProto{
			Number:      int32(c.Number),
			Text:        c.Text,
			Type:        c.Type,
			Features:    c.Features,
			DependsOn:   int32(c.DependsOn),
		})
	}

	dependentClaims := make([]*ClaimProto, 0, len(analysis.DependentClaims))
	for _, c := range analysis.DependentClaims {
		dependentClaims = append(dependentClaims, &ClaimProto{
			Number:      int32(c.Number),
			Text:        c.Text,
			Type:        c.Type,
			Features:    c.Features,
			DependsOn:   int32(c.DependsOn),
		})
	}

	return &ClaimTreeProto{
		IndependentClaims: independentClaims,
		DependentClaims:   dependentClaims,
		TotalClaims:       int32(analysis.TotalClaims),
	}
}

// mapPatentError 将领域错误映射为 gRPC 状态码
func mapPatentError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, "not found") {
		return status.Errorf(codes.NotFound, errMsg)
	}
	if strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "duplicate") {
		return status.Errorf(codes.AlreadyExists, errMsg)
	}
	if strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "validation") {
		return status.Errorf(codes.InvalidArgument, errMsg)
	}
	if strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "permission") {
		return status.Errorf(codes.PermissionDenied, errMsg)
	}

	return status.Errorf(codes.Internal, "internal error: %v", err)
}

// isValidPatentNumber 验证专利号格式
func isValidPatentNumber(number string) bool {
	if number == "" {
		return false
	}
	// 支持 CN/US/EP/JP/KR/WO 前缀格式
	validPattern := regexp.MustCompile(`^(CN|US|EP|JP|KR|WO)[0-9A-Z]+$`)
	return validPattern.MatchString(strings.ToUpper(number))
}

// extractJurisdiction 从专利号提取法域
func extractJurisdiction(number string) string {
	upper := strings.ToUpper(number)
	for _, prefix := range []string{"CN", "US", "EP", "JP", "KR", "WO"} {
		if strings.HasPrefix(upper, prefix) {
			return prefix
		}
	}
	return "UNKNOWN"
}

// 领域模型类型
type Patent struct {
	Number       string
	Title        string
	Abstract     string
	FilingDate   string
	Status       string
	Jurisdiction string
	Applicant    string
	Inventor     string
	IPCCodes     []string
	CPCCodes     []string
}

type PatentSearchOptions struct {
	Query         string
	PageSize      int
	PageToken     string
	IPCCodes      []string
	CPCCodes      []string
	Jurisdictions []string
	DateFrom      string
	DateTo        string
	SortBy        string
}

type PatentSearchResult struct {
	Patents       []*Patent
	TotalCount    int
	NextPageToken string
}

type Claim struct {
	Number    int
	Text      string
	Type      string
	Features  []string
	DependsOn int
}

type ClaimAnalysis struct {
	IndependentClaims []*Claim
	DependentClaims   []*Claim
	TotalClaims       int
}

type CitationNode struct {
	PatentNumber string
	Depth        int
	CitationType string
}

type CitationEdge struct {
	From string
	To   string
	Type string
}

type CitationNetwork struct {
	Nodes      []*CitationNode
	Edges      []*CitationEdge
	TotalNodes int
}

type FTOCheckResult struct {
	RiskLevel       string
	Confidence      float64
	BlockingPatents []*Patent
	Summary         string
}

// Protobuf 消息类型
type UnimplementedPatentServiceServer struct{}

type GetPatentRequest struct{ Number string }

func (req *GetPatentRequest) GetNumber() string { return req.Number }

type GetPatentResponse struct{ Patent *PatentProto }

type PatentProto struct {
	Number       string
	Title        string
	Abstract     string
	FilingDate   string
	Status       string
	Jurisdiction string
	Applicant    string
	Inventor     string
	IPCCodes     []string
	CPCCodes     []string
}

type SearchPatentsProtoRequest struct {
	Query         string
	PageSize      int32
	PageToken     string
	IpcCodes      []string
	CpcCodes      []string
	Jurisdictions []string
	DateFrom      string
	DateTo        string
	SortBy        string
}

type SearchPatentsProtoResponse struct {
	Results       []*PatentProto
	TotalCount    int32
	NextPageToken string
}

type AnalyzeClaimsRequest struct{ PatentNumber string }
type AnalyzeClaimsResponse struct{ ClaimTree *ClaimTreeProto }

type ClaimTreeProto struct {
	IndependentClaims []*ClaimProto
	DependentClaims   []*ClaimProto
	TotalClaims       int32
}

type ClaimProto struct {
	Number    int32
	Text      string
	Type      string
	Features  []string
	DependsOn int32
}

type CheckFTORequest struct {
	Smiles        string
	Jurisdictions []string
}

type CheckFTOResponse struct {
	RiskLevel       string
	Confidence      float64
	BlockingPatents []*PatentProto
	Summary         string
}

type GetPatentFamilyRequest struct{ PatentNumber string }
type GetPatentFamilyResponse struct {
	FamilyMembers []*PatentProto
	FamilyId      string
}

type GetCitationNetworkRequest struct {
	PatentNumber string
	Depth        int32
}

type GetCitationNetworkResponse struct {
	Nodes       []*CitationNodeProto
	Edges       []*CitationEdgeProto
	TotalNodes  int32
	IsTruncated bool
}

type CitationNodeProto struct {
	PatentNumber string
	Depth        int32
	CitationType string
}

type CitationEdgeProto struct {
	From string
	To   string
	Type string
}

// 向后兼容的旧类型
type GetPatentRequestLegacy struct{ Number string }

func (req *GetPatentRequestLegacy) GetNumber() string { return req.Number }

type PatentResponseLegacy struct {
	Number       string
	Title        string
	Abstract     string
	FilingDate   string
	Status       string
	Jurisdiction string
}

type SearchPatentsRequestLegacy struct{ Query string }
type SearchPatentsResponseLegacy struct {
	Results    []*PatentResponseLegacy
	TotalCount int32
}

type InfringementRequestLegacy struct{ PatentNumber, MoleculeId string }
type InfringementResponseLegacy struct {
	RiskLevel  string
	Confidence float64
	Details    string
}

//Personal.AI order the ending
