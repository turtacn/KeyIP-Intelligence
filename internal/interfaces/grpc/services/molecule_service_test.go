package services

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockMoleculeApp 模拟分子应用服务
type mockMoleculeApp struct {
	molecules map[string]*Molecule
	err       error
}

func newMockMoleculeApp() *mockMoleculeApp {
	return &mockMoleculeApp{
		molecules: map[string]*Molecule{
			"mol-123": {
				ID:              "mol-123",
				SMILES:          "C1=CC=CC=C1",
				InChIKey:        "UHOVQNZJYSORNB-UHFFFAOYSA-N",
				MolecularWeight: 78.11,
				Formula:         "C6H6",
				Name:            "Benzene",
			},
		},
	}
}

func (m *mockMoleculeApp) GetByID(ctx context.Context, id string) (*Molecule, error) {
	if m.err != nil {
		return nil, m.err
	}
	mol, ok := m.molecules[id]
	if !ok {
		return nil, &ErrNotFound{Msg: "molecule not found"}
	}
	return mol, nil
}

func (m *mockMoleculeApp) Create(ctx context.Context, cmd *CreateMoleculeCommand) (*Molecule, error) {
	if m.err != nil {
		return nil, m.err
	}
	mol := &Molecule{
		ID:     "mol-new",
		SMILES: cmd.SMILES,
		Name:   cmd.Name,
	}
	m.molecules[mol.ID] = mol
	return mol, nil
}

func (m *mockMoleculeApp) Update(ctx context.Context, cmd *UpdateMoleculeCommand) (*Molecule, error) {
	if m.err != nil {
		return nil, m.err
	}
	mol, ok := m.molecules[cmd.ID]
	if !ok {
		return nil, &ErrNotFound{Msg: "molecule not found"}
	}
	if cmd.Name != "" {
		mol.Name = cmd.Name
	}
	return mol, nil
}

func (m *mockMoleculeApp) Delete(ctx context.Context, id string) error {
	if m.err != nil {
		return m.err
	}
	if _, ok := m.molecules[id]; !ok {
		return &ErrNotFound{Msg: "molecule not found"}
	}
	delete(m.molecules, id)
	return nil
}

func (m *mockMoleculeApp) List(ctx context.Context, opts *ListMoleculesOptions) (*MoleculeList, error) {
	if m.err != nil {
		return nil, m.err
	}
	var mols []*Molecule
	for _, mol := range m.molecules {
		mols = append(mols, mol)
	}
	return &MoleculeList{
		Molecules:  mols,
		TotalCount: len(mols),
	}, nil
}

func (m *mockMoleculeApp) PredictProperties(ctx context.Context, smiles string) (*MoleculeProperties, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &MoleculeProperties{
		HOMO:               -5.2,
		LUMO:               -2.1,
		BandGap:            3.1,
		EmissionWavelength: 460.0,
		QuantumYield:       0.85,
	}, nil
}

// mockSimilaritySearch 模拟相似度搜索服务
type mockSimilaritySearch struct {
	results []*SimilarMolecule
	err     error
}

func (m *mockSimilaritySearch) Search(ctx context.Context, query string, threshold float64, fingerprintType string, limit int) ([]*SimilarMolecule, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

// mockServiceLogger 模拟日志记录器
type mockServiceLogger struct {
	messages []string
}

func (l *mockServiceLogger) Info(msg string, fields ...interface{})  { l.messages = append(l.messages, msg) }
func (l *mockServiceLogger) Error(msg string, fields ...interface{}) { l.messages = append(l.messages, msg) }
func (l *mockServiceLogger) Debug(msg string, fields ...interface{}) { l.messages = append(l.messages, msg) }

// TestNewMoleculeService 测试创建服务
func TestNewMoleculeService(t *testing.T) {
	svc := NewMoleculeService()
	if svc == nil {
		t.Fatal("NewMoleculeService should not return nil")
	}
}

// TestNewMoleculeServiceServer 测试创建完整服务
func TestNewMoleculeServiceServer(t *testing.T) {
	app := newMockMoleculeApp()
	search := &mockSimilaritySearch{}
	logger := &mockServiceLogger{}

	svc := NewMoleculeServiceServer(app, search, logger)
	if svc == nil {
		t.Fatal("NewMoleculeServiceServer should not return nil")
	}
	if svc.moleculeApp != app {
		t.Error("moleculeApp not set correctly")
	}
}

// TestGetMolecule_Success 正常获取分子数据
func TestGetMolecule_Success(t *testing.T) {
	app := newMockMoleculeApp()
	svc := NewMoleculeServiceServer(app, nil, nil)
	ctx := context.Background()

	resp, err := svc.GetMolecule(ctx, &GetMoleculeRequest{Id: "mol-123"})
	if err != nil {
		t.Fatalf("GetMolecule failed: %v", err)
	}

	if resp.Molecule.Id != "mol-123" {
		t.Errorf("expected Id=mol-123, got %s", resp.Molecule.Id)
	}
	if resp.Molecule.Smiles != "C1=CC=CC=C1" {
		t.Errorf("expected Benzene SMILES, got %s", resp.Molecule.Smiles)
	}
}

// TestGetMolecule_NotFound 分子不存在返回 NotFound
func TestGetMolecule_NotFound(t *testing.T) {
	app := newMockMoleculeApp()
	svc := NewMoleculeServiceServer(app, nil, nil)
	ctx := context.Background()

	_, err := svc.GetMolecule(ctx, &GetMoleculeRequest{Id: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent molecule")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

// TestGetMolecule_EmptyID 空 ID 返回 InvalidArgument
func TestGetMolecule_EmptyID(t *testing.T) {
	svc := NewMoleculeService()
	ctx := context.Background()

	_, err := svc.GetMolecule(ctx, &GetMoleculeRequest{Id: ""})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestCreateMolecule_Success 正常创建分子
func TestCreateMolecule_Success(t *testing.T) {
	app := newMockMoleculeApp()
	svc := NewMoleculeServiceServer(app, nil, nil)
	ctx := context.Background()

	req := &CreateMoleculeRequest{
		Smiles: "CC(=O)O",
		Name:   "Acetic acid",
	}
	resp, err := svc.CreateMolecule(ctx, req)
	if err != nil {
		t.Fatalf("CreateMolecule failed: %v", err)
	}

	if resp.Molecule.Smiles != "CC(=O)O" {
		t.Errorf("expected SMILES CC(=O)O, got %s", resp.Molecule.Smiles)
	}
}

// TestCreateMolecule_InvalidSMILES 非法 SMILES 返回 InvalidArgument
func TestCreateMolecule_InvalidSMILES(t *testing.T) {
	svc := NewMoleculeService()
	ctx := context.Background()

	req := &CreateMoleculeRequest{
		Smiles: "!!!invalid!!!",
		Name:   "Invalid",
	}
	_, err := svc.CreateMolecule(ctx, req)
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

// TestCreateMolecule_Duplicate 重复创建返回 AlreadyExists
func TestCreateMolecule_Duplicate(t *testing.T) {
	app := newMockMoleculeApp()
	app.err = &ErrConflict{Msg: "molecule already exists"}
	svc := NewMoleculeServiceServer(app, nil, nil)
	ctx := context.Background()

	req := &CreateMoleculeRequest{
		Smiles: "C1=CC=CC=C1",
		Name:   "Benzene",
	}
	_, err := svc.CreateMolecule(ctx, req)
	if err == nil {
		t.Fatal("expected error for duplicate")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.AlreadyExists {
		t.Errorf("expected AlreadyExists, got %v", st.Code())
	}
}

// TestUpdateMolecule_Success 正常更新
func TestUpdateMolecule_Success(t *testing.T) {
	app := newMockMoleculeApp()
	svc := NewMoleculeServiceServer(app, nil, nil)
	ctx := context.Background()

	req := &UpdateMoleculeRequest{
		Id:   "mol-123",
		Name: "Updated Benzene",
	}
	resp, err := svc.UpdateMolecule(ctx, req)
	if err != nil {
		t.Fatalf("UpdateMolecule failed: %v", err)
	}

	if resp.Molecule.Name != "Updated Benzene" {
		t.Errorf("expected updated name, got %s", resp.Molecule.Name)
	}
}

// TestUpdateMolecule_NotFound 更新不存在的分子
func TestUpdateMolecule_NotFound(t *testing.T) {
	app := newMockMoleculeApp()
	svc := NewMoleculeServiceServer(app, nil, nil)
	ctx := context.Background()

	req := &UpdateMoleculeRequest{
		Id:   "nonexistent",
		Name: "Test",
	}
	_, err := svc.UpdateMolecule(ctx, req)
	if err == nil {
		t.Fatal("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

// TestDeleteMolecule_Success 正常删除
func TestDeleteMolecule_Success(t *testing.T) {
	app := newMockMoleculeApp()
	svc := NewMoleculeServiceServer(app, nil, nil)
	ctx := context.Background()

	resp, err := svc.DeleteMolecule(ctx, &DeleteMoleculeRequest{Id: "mol-123"})
	if err != nil {
		t.Fatalf("DeleteMolecule failed: %v", err)
	}

	if !resp.Success {
		t.Error("expected success=true")
	}
}

// TestDeleteMolecule_NotFound 删除不存在的分子
func TestDeleteMolecule_NotFound(t *testing.T) {
	app := newMockMoleculeApp()
	svc := NewMoleculeServiceServer(app, nil, nil)
	ctx := context.Background()

	_, err := svc.DeleteMolecule(ctx, &DeleteMoleculeRequest{Id: "nonexistent"})
	if err == nil {
		t.Fatal("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

// TestListMolecules_FirstPage 首页查询
func TestListMolecules_FirstPage(t *testing.T) {
	app := newMockMoleculeApp()
	svc := NewMoleculeServiceServer(app, nil, nil)
	ctx := context.Background()

	resp, err := svc.ListMolecules(ctx, &ListMoleculesRequest{PageSize: 10})
	if err != nil {
		t.Fatalf("ListMolecules failed: %v", err)
	}

	if resp.TotalCount == 0 {
		t.Error("expected at least one result")
	}
}

// TestListMolecules_WithPageToken 翻页查询
func TestListMolecules_WithPageToken(t *testing.T) {
	svc := NewMoleculeService()
	ctx := context.Background()

	token := encodePageToken("cursor-123")
	_, err := svc.ListMolecules(ctx, &ListMoleculesRequest{
		PageSize:  10,
		PageToken: token,
	})
	if err != nil {
		t.Fatalf("ListMolecules with token failed: %v", err)
	}
}

// TestListMolecules_InvalidPageSize 非法 page_size
func TestListMolecules_InvalidPageSize(t *testing.T) {
	svc := NewMoleculeService()
	ctx := context.Background()

	_, err := svc.ListMolecules(ctx, &ListMoleculesRequest{PageSize: 200})
	if err == nil {
		t.Fatal("expected error for invalid page_size")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestSearchSimilar_BySMILES SMILES 相似度搜索
func TestSearchSimilar_BySMILES(t *testing.T) {
	search := &mockSimilaritySearch{
		results: []*SimilarMolecule{
			{
				Molecule:   &Molecule{ID: "mol-001", SMILES: "C1=CC=CC=C1"},
				Similarity: 0.95,
			},
		},
	}
	svc := NewMoleculeServiceServer(nil, search, nil)
	ctx := context.Background()

	resp, err := svc.SearchSimilar(ctx, &SearchSimilarRequest{
		Smiles:    "C1=CC=CC=C1",
		Threshold: 0.7,
	})
	if err != nil {
		t.Fatalf("SearchSimilar failed: %v", err)
	}

	if resp.TotalCount == 0 {
		t.Error("expected at least one result")
	}
}

// TestSearchSimilar_ByInChI InChI 相似度搜索
func TestSearchSimilar_ByInChI(t *testing.T) {
	svc := NewMoleculeService()
	ctx := context.Background()

	resp, err := svc.SearchSimilar(ctx, &SearchSimilarRequest{
		Inchi:     "InChI=1S/C6H6/c1-2-4-6-5-3-1/h1-6H",
		Threshold: 0.7,
	})
	if err != nil {
		t.Fatalf("SearchSimilar by InChI failed: %v", err)
	}

	if resp == nil {
		t.Error("expected response")
	}
}

// TestSearchSimilar_InvalidThreshold 非法阈值
func TestSearchSimilar_InvalidThreshold(t *testing.T) {
	svc := NewMoleculeService()
	ctx := context.Background()

	_, err := svc.SearchSimilar(ctx, &SearchSimilarRequest{
		Smiles:    "C1=CC=CC=C1",
		Threshold: 1.5, // 超出范围
	})
	if err == nil {
		t.Fatal("expected error for invalid threshold")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestSearchSimilar_EmptyResults 无相似结果
func TestSearchSimilar_EmptyResults(t *testing.T) {
	search := &mockSimilaritySearch{results: []*SimilarMolecule{}}
	svc := NewMoleculeServiceServer(nil, search, nil)
	ctx := context.Background()

	resp, err := svc.SearchSimilar(ctx, &SearchSimilarRequest{
		Smiles:    "C1=CC=CC=C1",
		Threshold: 0.99,
	})
	if err != nil {
		t.Fatalf("SearchSimilar failed: %v", err)
	}

	if resp.TotalCount != 0 {
		t.Errorf("expected 0 results, got %d", resp.TotalCount)
	}
}

// TestPredictProperties_Success 属性预测成功
func TestPredictProperties_Success(t *testing.T) {
	app := newMockMoleculeApp()
	svc := NewMoleculeServiceServer(app, nil, nil)
	ctx := context.Background()

	resp, err := svc.PredictProperties(ctx, &PredictPropertiesRequest{Smiles: "C1=CC=CC=C1"})
	if err != nil {
		t.Fatalf("PredictProperties failed: %v", err)
	}

	if resp.Properties.Homo >= 0 {
		t.Error("expected negative HOMO")
	}
	if resp.Properties.BandGap <= 0 {
		t.Error("expected positive band gap")
	}
}

// TestPredictProperties_InvalidSMILES 非法 SMILES
func TestPredictProperties_InvalidSMILES(t *testing.T) {
	svc := NewMoleculeService()
	ctx := context.Background()

	_, err := svc.PredictProperties(ctx, &PredictPropertiesRequest{Smiles: "!!!invalid!!!"})
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

// TestDomainToProto_FullConversion 完整字段转换验证
func TestDomainToProto_FullConversion(t *testing.T) {
	mol := &Molecule{
		ID:              "mol-123",
		SMILES:          "C1=CC=CC=C1",
		InChIKey:        "UHOVQNZJYSORNB-UHFFFAOYSA-N",
		MolecularWeight: 78.11,
		Formula:         "C6H6",
		Name:            "Benzene",
		MoleculeType:    "aromatic",
		OLEDLayer:       "EML",
	}

	proto := domainToProto(mol)

	if proto.Id != mol.ID {
		t.Errorf("ID mismatch: %s != %s", proto.Id, mol.ID)
	}
	if proto.Smiles != mol.SMILES {
		t.Errorf("SMILES mismatch")
	}
	if proto.InchiKey != mol.InChIKey {
		t.Errorf("InChIKey mismatch")
	}
	if proto.MolecularWeight != mol.MolecularWeight {
		t.Errorf("MolecularWeight mismatch")
	}
	if proto.Formula != mol.Formula {
		t.Errorf("Formula mismatch")
	}
	if proto.Name != mol.Name {
		t.Errorf("Name mismatch")
	}
	if proto.MoleculeType != mol.MoleculeType {
		t.Errorf("MoleculeType mismatch")
	}
	if proto.OledLayer != mol.OLEDLayer {
		t.Errorf("OLEDLayer mismatch")
	}
}

// TestDomainToProto_NilInput nil 输入处理
func TestDomainToProto_NilInput(t *testing.T) {
	proto := domainToProto(nil)
	if proto != nil {
		t.Error("expected nil for nil input")
	}
}

// TestMapDomainError_AllCodes 全部错误码映射验证
func TestMapDomainError_AllCodes(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected codes.Code
	}{
		{"NotFound", &ErrNotFound{Msg: "not found"}, codes.NotFound},
		{"Validation", &ErrValidation{Msg: "invalid"}, codes.InvalidArgument},
		{"Conflict", &ErrConflict{Msg: "duplicate"}, codes.AlreadyExists},
		{"Unauthorized", &ErrUnauthorized{Msg: "denied"}, codes.PermissionDenied},
		{"Generic", errors.New("unknown error"), codes.Internal},
		{"Nil", nil, codes.OK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := mapDomainError(tc.err)
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

// TestPageTokenEncoding_RoundTrip 分页 token 编解码往返一致性
func TestPageTokenEncoding_RoundTrip(t *testing.T) {
	original := "cursor:mol-123:page-2"
	encoded := encodePageToken(original)
	decoded, err := decodePageToken(encoded)
	if err != nil {
		t.Fatalf("decodePageToken failed: %v", err)
	}

	if decoded != original {
		t.Errorf("round trip failed: %s != %s", decoded, original)
	}
}

// TestIsValidSMILES 验证 SMILES 格式检查
func TestIsValidSMILES(t *testing.T) {
	tests := []struct {
		smiles string
		valid  bool
	}{
		{"C1=CC=CC=C1", true},
		{"CC(=O)O", true},
		{"[Cu+2]", true},
		{"", false},
		{"!!!invalid!!!", false},
		{"hello world", false},
	}

	for _, tc := range tests {
		result := isValidSMILES(tc.smiles)
		if result != tc.valid {
			t.Errorf("isValidSMILES(%s) = %v, want %v", tc.smiles, result, tc.valid)
		}
	}
}

// 向后兼容测试
func TestMoleculeService_GetMolecule(t *testing.T) {
	svc := NewMoleculeService()
	ctx := context.Background()

	req := &GetMoleculeRequest{Id: "mol-123"}
	resp, err := svc.GetMolecule(ctx, req)
	if err != nil {
		t.Fatalf("GetMolecule failed: %v", err)
	}

	if resp.Molecule.Id != "mol-123" {
		t.Errorf("expected Id=mol-123, got %s", resp.Molecule.Id)
	}
}

func TestMoleculeService_SearchMolecules(t *testing.T) {
	svc := NewMoleculeService()
	ctx := context.Background()

	req := &SearchMoleculesRequestLegacy{Query: "benzene"}
	resp, err := svc.SearchMolecules(ctx, req)
	if err != nil {
		t.Fatalf("SearchMolecules failed: %v", err)
	}

	if resp.TotalCount == 0 {
		t.Error("expected at least one result")
	}
}

func TestMoleculeService_CalculateSimilarity(t *testing.T) {
	svc := NewMoleculeService()
	ctx := context.Background()

	req := &SimilarityRequestLegacy{
		Mol1: "C1=CC=CC=C1",
		Mol2: "C1=CC=CC=C1C",
	}
	resp, err := svc.CalculateSimilarity(ctx, req)
	if err != nil {
		t.Fatalf("CalculateSimilarity failed: %v", err)
	}

	if resp.Similarity < 0 || resp.Similarity > 1 {
		t.Errorf("similarity should be in [0,1], got %f", resp.Similarity)
	}

	if resp.Method == "" {
		t.Error("expected non-empty method")
	}
}

//Personal.AI order the ending
