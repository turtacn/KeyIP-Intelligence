package services

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockPatentApp 模拟专利应用服务
type mockPatentApp struct {
	patents map[string]*Patent
	err     error
}

func newMockPatentApp() *mockPatentApp {
	return &mockPatentApp{
		patents: map[string]*Patent{
			"CN202110123456": {
				Number:       "CN202110123456",
				Title:        "OLED Material Composition",
				Abstract:     "A novel organic light-emitting material...",
				FilingDate:   "2021-01-15",
				Status:       "Granted",
				Jurisdiction: "CN",
				Applicant:    "Acme Corp",
				Inventor:     "John Doe",
				IPCCodes:     []string{"H10K85/60"},
				CPCCodes:     []string{"H10K2101/10"},
			},
		},
	}
}

func (m *mockPatentApp) GetByNumber(ctx context.Context, number string) (*Patent, error) {
	if m.err != nil {
		return nil, m.err
	}
	patent, ok := m.patents[number]
	if !ok {
		return nil, errors.New("patent not found")
	}
	return patent, nil
}

func (m *mockPatentApp) Search(ctx context.Context, opts *PatentSearchOptions) (*PatentSearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	var patents []*Patent
	for _, p := range m.patents {
		patents = append(patents, p)
	}
	return &PatentSearchResult{
		Patents:    patents,
		TotalCount: len(patents),
	}, nil
}

func (m *mockPatentApp) AnalyzeClaims(ctx context.Context, patentNumber string) (*ClaimAnalysis, error) {
	if m.err != nil {
		return nil, m.err
	}
	if _, ok := m.patents[patentNumber]; !ok {
		return nil, errors.New("patent not found")
	}
	return &ClaimAnalysis{
		IndependentClaims: []*Claim{
			{Number: 1, Text: "A compound...", Type: "independent"},
		},
		DependentClaims: []*Claim{
			{Number: 2, Text: "The compound of claim 1...", Type: "dependent", DependsOn: 1},
		},
		TotalClaims: 2,
	}, nil
}

func (m *mockPatentApp) GetFamily(ctx context.Context, patentNumber string) ([]*Patent, error) {
	if m.err != nil {
		return nil, m.err
	}
	if _, ok := m.patents[patentNumber]; !ok {
		return nil, errors.New("patent not found")
	}
	return []*Patent{
		{Number: patentNumber, Jurisdiction: "CN"},
		{Number: "US20210123456", Jurisdiction: "US"},
	}, nil
}

func (m *mockPatentApp) GetCitationNetwork(ctx context.Context, patentNumber string, depth int) (*CitationNetwork, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &CitationNetwork{
		Nodes: []*CitationNode{
			{PatentNumber: patentNumber, Depth: 0},
			{PatentNumber: "US20200123456", Depth: 1, CitationType: "backward"},
		},
		Edges: []*CitationEdge{
			{From: patentNumber, To: "US20200123456", Type: "backward"},
		},
		TotalNodes: 2,
	}, nil
}

// mockFTOService 模拟 FTO 服务
type mockFTOService struct {
	result *FTOCheckResult
	err    error
}

func (m *mockFTOService) QuickCheck(ctx context.Context, smiles string, jurisdictions []string) (*FTOCheckResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &FTOCheckResult{
		RiskLevel:       "LOW",
		Confidence:      0.85,
		BlockingPatents: []*Patent{},
		Summary:         "No risks found",
	}, nil
}

// mockPatentLogger 模拟日志
type mockPatentLogger struct {
	messages []string
}

func (l *mockPatentLogger) Info(msg string, fields ...interface{})  { l.messages = append(l.messages, msg) }
func (l *mockPatentLogger) Error(msg string, fields ...interface{}) { l.messages = append(l.messages, msg) }
func (l *mockPatentLogger) Debug(msg string, fields ...interface{}) { l.messages = append(l.messages, msg) }

// TestNewPatentService 测试创建服务
func TestNewPatentService(t *testing.T) {
	svc := NewPatentService()
	if svc == nil {
		t.Fatal("NewPatentService should not return nil")
	}
}

// TestNewPatentServiceServer 测试创建完整服务
func TestNewPatentServiceServer(t *testing.T) {
	app := newMockPatentApp()
	fto := &mockFTOService{}
	logger := &mockPatentLogger{}

	svc := NewPatentServiceServer(app, fto, logger)
	if svc == nil {
		t.Fatal("NewPatentServiceServer should not return nil")
	}
	if svc.patentApp != app {
		t.Error("patentApp not set correctly")
	}
}

// TestGetPatent_Success 正常获取专利
func TestGetPatent_Success(t *testing.T) {
	app := newMockPatentApp()
	svc := NewPatentServiceServer(app, nil, nil)
	ctx := context.Background()

	resp, err := svc.GetPatent(ctx, &GetPatentRequest{Number: "CN202110123456"})
	if err != nil {
		t.Fatalf("GetPatent failed: %v", err)
	}

	if resp.Patent.Number != "CN202110123456" {
		t.Errorf("expected CN202110123456, got %s", resp.Patent.Number)
	}
	if resp.Patent.Title == "" {
		t.Error("expected non-empty title")
	}
}

// TestGetPatent_NotFound 专利不存在
func TestGetPatent_NotFound(t *testing.T) {
	app := newMockPatentApp()
	svc := NewPatentServiceServer(app, nil, nil)
	ctx := context.Background()

	_, err := svc.GetPatent(ctx, &GetPatentRequest{Number: "CN999999999999"})
	if err == nil {
		t.Fatal("expected error for nonexistent patent")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

// TestGetPatent_InvalidNumber 非法专利号格式
func TestGetPatent_InvalidNumber(t *testing.T) {
	svc := NewPatentService()
	ctx := context.Background()

	_, err := svc.GetPatent(ctx, &GetPatentRequest{Number: "INVALID"})
	if err == nil {
		t.Fatal("expected error for invalid patent number")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestSearchPatents_BasicQuery 基本全文搜索
func TestSearchPatents_BasicQuery(t *testing.T) {
	app := newMockPatentApp()
	svc := NewPatentServiceServer(app, nil, nil)
	ctx := context.Background()

	resp, err := svc.SearchPatents(ctx, &SearchPatentsProtoRequest{Query: "OLED"})
	if err != nil {
		t.Fatalf("SearchPatents failed: %v", err)
	}

	if resp.TotalCount == 0 {
		t.Error("expected at least one result")
	}
}

// TestSearchPatents_WithFilters 带 IPC/日期/专利局过滤
func TestSearchPatents_WithFilters(t *testing.T) {
	app := newMockPatentApp()
	svc := NewPatentServiceServer(app, nil, nil)
	ctx := context.Background()

	resp, err := svc.SearchPatents(ctx, &SearchPatentsProtoRequest{
		Query:         "OLED",
		IpcCodes:      []string{"H10K85/60"},
		Jurisdictions: []string{"CN", "US"},
		DateFrom:      "2020-01-01",
		DateTo:        "2022-12-31",
	})
	if err != nil {
		t.Fatalf("SearchPatents with filters failed: %v", err)
	}

	if resp == nil {
		t.Error("expected response")
	}
}

// TestSearchPatents_Pagination 分页查询
func TestSearchPatents_Pagination(t *testing.T) {
	svc := NewPatentService()
	ctx := context.Background()

	resp, err := svc.SearchPatents(ctx, &SearchPatentsProtoRequest{
		Query:    "OLED",
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("SearchPatents pagination failed: %v", err)
	}

	if resp == nil {
		t.Error("expected response")
	}
}

// TestSearchPatents_EmptyResults 无结果
func TestSearchPatents_EmptyResults(t *testing.T) {
	app := &mockPatentApp{patents: map[string]*Patent{}}
	svc := NewPatentServiceServer(app, nil, nil)
	ctx := context.Background()

	resp, err := svc.SearchPatents(ctx, &SearchPatentsProtoRequest{Query: "nonexistent"})
	if err != nil {
		t.Fatalf("SearchPatents failed: %v", err)
	}

	if resp.TotalCount != 0 {
		t.Errorf("expected 0 results, got %d", resp.TotalCount)
	}
}

// TestAnalyzeClaims_Success 权利要求解析成功
func TestAnalyzeClaims_Success(t *testing.T) {
	app := newMockPatentApp()
	svc := NewPatentServiceServer(app, nil, nil)
	ctx := context.Background()

	resp, err := svc.AnalyzeClaims(ctx, &AnalyzeClaimsRequest{PatentNumber: "CN202110123456"})
	if err != nil {
		t.Fatalf("AnalyzeClaims failed: %v", err)
	}

	if resp.ClaimTree == nil {
		t.Fatal("expected claim tree")
	}
	if len(resp.ClaimTree.IndependentClaims) == 0 {
		t.Error("expected independent claims")
	}
}

// TestAnalyzeClaims_NotFound 专利不存在
func TestAnalyzeClaims_NotFound(t *testing.T) {
	app := newMockPatentApp()
	svc := NewPatentServiceServer(app, nil, nil)
	ctx := context.Background()

	_, err := svc.AnalyzeClaims(ctx, &AnalyzeClaimsRequest{PatentNumber: "CN999999999999"})
	if err == nil {
		t.Fatal("expected error for nonexistent patent")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

// TestCheckFTO_NoRisk FTO 检查无风险
func TestCheckFTO_NoRisk(t *testing.T) {
	fto := &mockFTOService{
		result: &FTOCheckResult{
			RiskLevel:       "LOW",
			Confidence:      0.9,
			BlockingPatents: []*Patent{},
			Summary:         "No FTO risks",
		},
	}
	svc := NewPatentServiceServer(nil, fto, nil)
	ctx := context.Background()

	resp, err := svc.CheckFTO(ctx, &CheckFTORequest{Smiles: "C1=CC=CC=C1"})
	if err != nil {
		t.Fatalf("CheckFTO failed: %v", err)
	}

	if resp.RiskLevel != "LOW" {
		t.Errorf("expected LOW risk, got %s", resp.RiskLevel)
	}
}

// TestCheckFTO_HighRisk FTO 检查高风险
func TestCheckFTO_HighRisk(t *testing.T) {
	fto := &mockFTOService{
		result: &FTOCheckResult{
			RiskLevel:  "HIGH",
			Confidence: 0.95,
			BlockingPatents: []*Patent{
				{Number: "CN202110123456", Title: "Blocking patent"},
			},
			Summary: "High FTO risk detected",
		},
	}
	svc := NewPatentServiceServer(nil, fto, nil)
	ctx := context.Background()

	resp, err := svc.CheckFTO(ctx, &CheckFTORequest{Smiles: "C1=CC=CC=C1"})
	if err != nil {
		t.Fatalf("CheckFTO failed: %v", err)
	}

	if resp.RiskLevel != "HIGH" {
		t.Errorf("expected HIGH risk, got %s", resp.RiskLevel)
	}
	if len(resp.BlockingPatents) == 0 {
		t.Error("expected blocking patents")
	}
}

// TestCheckFTO_InvalidSMILES 非法 SMILES
func TestCheckFTO_InvalidSMILES(t *testing.T) {
	svc := NewPatentService()
	ctx := context.Background()

	_, err := svc.CheckFTO(ctx, &CheckFTORequest{Smiles: "!!!invalid!!!"})
	if err == nil {
		t.Fatal("expected error for invalid SMILES")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestGetPatentFamily_Success 同族专利查询成功
func TestGetPatentFamily_Success(t *testing.T) {
	app := newMockPatentApp()
	svc := NewPatentServiceServer(app, nil, nil)
	ctx := context.Background()

	resp, err := svc.GetPatentFamily(ctx, &GetPatentFamilyRequest{PatentNumber: "CN202110123456"})
	if err != nil {
		t.Fatalf("GetPatentFamily failed: %v", err)
	}

	if len(resp.FamilyMembers) == 0 {
		t.Error("expected family members")
	}
}

// TestGetCitationNetwork_DefaultDepth 默认深度引用网络
func TestGetCitationNetwork_DefaultDepth(t *testing.T) {
	app := newMockPatentApp()
	svc := NewPatentServiceServer(app, nil, nil)
	ctx := context.Background()

	resp, err := svc.GetCitationNetwork(ctx, &GetCitationNetworkRequest{PatentNumber: "CN202110123456"})
	if err != nil {
		t.Fatalf("GetCitationNetwork failed: %v", err)
	}

	if len(resp.Nodes) == 0 {
		t.Error("expected nodes")
	}
}

// TestGetCitationNetwork_ExceedDepthLimit 超出深度限制
func TestGetCitationNetwork_ExceedDepthLimit(t *testing.T) {
	svc := NewPatentService()
	ctx := context.Background()

	_, err := svc.GetCitationNetwork(ctx, &GetCitationNetworkRequest{
		PatentNumber: "CN202110123456",
		Depth:        10,
	})
	if err == nil {
		t.Fatal("expected error for exceeding depth limit")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestPatentDomainToProto_FullConversion 完整字段转换
func TestPatentDomainToProto_FullConversion(t *testing.T) {
	patent := &Patent{
		Number:       "CN202110123456",
		Title:        "OLED Material",
		Abstract:     "Novel material...",
		FilingDate:   "2021-01-15",
		Status:       "Granted",
		Jurisdiction: "CN",
		Applicant:    "Acme Corp",
		Inventor:     "John Doe",
		IPCCodes:     []string{"H10K85/60"},
		CPCCodes:     []string{"H10K2101/10"},
	}

	proto := patentDomainToProto(patent)

	if proto.Number != patent.Number {
		t.Errorf("Number mismatch")
	}
	if proto.Title != patent.Title {
		t.Errorf("Title mismatch")
	}
	if proto.Jurisdiction != patent.Jurisdiction {
		t.Errorf("Jurisdiction mismatch")
	}
}

// TestPatentDomainToProto_NilInput nil 输入处理
func TestPatentDomainToProto_NilInput(t *testing.T) {
	proto := patentDomainToProto(nil)
	if proto != nil {
		t.Error("expected nil for nil input")
	}
}

// TestMapPatentError_AllCodes 全部错误码映射
func TestMapPatentError_AllCodes(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected codes.Code
	}{
		{"NotFound", errors.New("patent not found"), codes.NotFound},
		{"AlreadyExists", errors.New("already exists"), codes.AlreadyExists},
		{"Invalid", errors.New("invalid input"), codes.InvalidArgument},
		{"Unauthorized", errors.New("unauthorized access"), codes.PermissionDenied},
		{"Generic", errors.New("unknown error"), codes.Internal},
		{"Nil", nil, codes.OK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := mapPatentError(tc.err)
			if tc.err == nil {
				if err != nil {
					t.Errorf("expected nil error, got %v", err)
				}
				return
			}

			st, ok := status.FromError(err)
			if !ok {
				t.Fatal("expected gRPC status error")
			}
			if st.Code() != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, st.Code())
			}
		})
	}
}

// TestIsValidPatentNumber 验证专利号格式
func TestIsValidPatentNumber(t *testing.T) {
	tests := []struct {
		number string
		valid  bool
	}{
		{"CN202110123456", true},
		{"US20210123456", true},
		{"EP3456789", true},
		{"JP2021123456", true},
		{"KR20210123456", true},
		{"WO2021123456", true},
		{"", false},
		{"INVALID", false},
		{"12345", false},
	}

	for _, tc := range tests {
		result := isValidPatentNumber(tc.number)
		if result != tc.valid {
			t.Errorf("isValidPatentNumber(%s) = %v, want %v", tc.number, result, tc.valid)
		}
	}
}

// TestExtractJurisdiction 从专利号提取法域
func TestExtractJurisdiction(t *testing.T) {
	tests := []struct {
		number       string
		jurisdiction string
	}{
		{"CN202110123456", "CN"},
		{"US20210123456", "US"},
		{"EP3456789", "EP"},
		{"jp2021123456", "JP"},
		{"UNKNOWN123", "UNKNOWN"},
	}

	for _, tc := range tests {
		result := extractJurisdiction(tc.number)
		if result != tc.jurisdiction {
			t.Errorf("extractJurisdiction(%s) = %s, want %s", tc.number, result, tc.jurisdiction)
		}
	}
}

// 向后兼容测试
func TestPatentService_GetPatent(t *testing.T) {
	svc := NewPatentService()
	ctx := context.Background()

	req := &GetPatentRequest{Number: "CN202110123456"}
	resp, err := svc.GetPatent(ctx, req)
	if err != nil {
		t.Fatalf("GetPatent failed: %v", err)
	}

	if resp.Patent.Number != "CN202110123456" {
		t.Errorf("expected Number=CN202110123456, got %s", resp.Patent.Number)
	}
}

func TestPatentService_SearchPatents(t *testing.T) {
	svc := NewPatentService()
	ctx := context.Background()

	req := &SearchPatentsProtoRequest{Query: "OLED"}
	resp, err := svc.SearchPatents(ctx, req)
	if err != nil {
		t.Fatalf("SearchPatents failed: %v", err)
	}

	if resp.TotalCount == 0 {
		t.Error("expected at least one result")
	}
}

func TestPatentService_AnalyzeInfringement(t *testing.T) {
	svc := NewPatentService()
	ctx := context.Background()

	req := &InfringementRequestLegacy{
		PatentNumber: "CN202110123456",
		MoleculeId:   "mol-123",
	}
	resp, err := svc.AnalyzeInfringement(ctx, req)
	if err != nil {
		t.Fatalf("AnalyzeInfringement failed: %v", err)
	}

	if resp.RiskLevel == "" {
		t.Error("expected non-empty risk level")
	}

	if resp.Confidence < 0 || resp.Confidence > 1 {
		t.Errorf("confidence should be in [0,1], got %f", resp.Confidence)
	}
}

//Personal.AI order the ending
