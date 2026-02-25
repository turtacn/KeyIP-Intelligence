package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// MoleculeApplicationService 分子应用服务接口
type MoleculeApplicationService interface {
	GetByID(ctx context.Context, id string) (*Molecule, error)
	Create(ctx context.Context, cmd *CreateMoleculeCommand) (*Molecule, error)
	Update(ctx context.Context, cmd *UpdateMoleculeCommand) (*Molecule, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, opts *ListMoleculesOptions) (*MoleculeList, error)
	PredictProperties(ctx context.Context, smiles string) (*MoleculeProperties, error)
}

// SimilaritySearchService 相似度搜索服务接口
type SimilaritySearchService interface {
	Search(ctx context.Context, query string, threshold float64, fingerprintType string, limit int) ([]*SimilarMolecule, error)
}

// Logger 日志接口
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// defaultServiceLogger 默认日志实现
type defaultServiceLogger struct{}

func (l *defaultServiceLogger) Info(msg string, fields ...interface{}) {
	log.Printf("[INFO] %s %v", msg, fields)
}
func (l *defaultServiceLogger) Error(msg string, fields ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, fields)
}
func (l *defaultServiceLogger) Debug(msg string, fields ...interface{}) {
	log.Printf("[DEBUG] %s %v", msg, fields)
}

// MoleculeServiceServer 实现 gRPC MoleculeService 接口
type MoleculeServiceServer struct {
	UnimplementedMoleculeServiceServer
	moleculeApp      MoleculeApplicationService
	similaritySearch SimilaritySearchService
	logger           Logger
}

// NewMoleculeServiceServer 创建分子服务
func NewMoleculeServiceServer(moleculeApp MoleculeApplicationService, similaritySearch SimilaritySearchService, logger Logger) *MoleculeServiceServer {
	if logger == nil {
		logger = &defaultServiceLogger{}
	}
	return &MoleculeServiceServer{
		moleculeApp:      moleculeApp,
		similaritySearch: similaritySearch,
		logger:           logger,
	}
}

// MoleculeService 向后兼容的别名
type MoleculeService = MoleculeServiceServer

// NewMoleculeService 向后兼容的创建函数
func NewMoleculeService() *MoleculeService {
	return NewMoleculeServiceServer(nil, nil, nil)
}

// GetMolecule 获取分子数据
func (s *MoleculeServiceServer) GetMolecule(ctx context.Context, req *GetMoleculeRequest) (*GetMoleculeResponse, error) {
	// 参数校验
	if req == nil || req.GetId() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "molecule_id is required")
	}

	// 从 metadata 提取上下文信息
	ctx = s.extractContext(ctx)

	s.logger.Debug("GetMolecule called", "id", req.GetId())

	// 调用应用服务
	if s.moleculeApp == nil {
		// 模拟返回
		return &GetMoleculeResponse{
			Molecule: &MoleculeProto{
				Id:              req.GetId(),
				Smiles:          "C1=CC=CC=C1",
				InchiKey:        "UHOVQNZJYSORNB-UHFFFAOYSA-N",
				MolecularWeight: 78.11,
				Formula:         "C6H6",
				Name:            "Benzene",
			},
		}, nil
	}

	mol, err := s.moleculeApp.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &GetMoleculeResponse{
		Molecule: domainToProto(mol),
	}, nil
}

// CreateMolecule 创建分子
func (s *MoleculeServiceServer) CreateMolecule(ctx context.Context, req *CreateMoleculeRequest) (*CreateMoleculeResponse, error) {
	// 参数校验
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "request is required")
	}
	if req.Smiles == "" {
		return nil, status.Errorf(codes.InvalidArgument, "SMILES is required")
	}
	if !isValidSMILES(req.Smiles) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid SMILES format")
	}

	ctx = s.extractContext(ctx)
	s.logger.Debug("CreateMolecule called", "smiles", req.Smiles)

	if s.moleculeApp == nil {
		// 模拟返回
		return &CreateMoleculeResponse{
			Molecule: &MoleculeProto{
				Id:     "mol-" + generateID(),
				Smiles: req.Smiles,
				Name:   req.Name,
			},
		}, nil
	}

	cmd := protoToCreateCommand(req)
	mol, err := s.moleculeApp.Create(ctx, cmd)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &CreateMoleculeResponse{
		Molecule: domainToProto(mol),
	}, nil
}

// UpdateMolecule 更新分子
func (s *MoleculeServiceServer) UpdateMolecule(ctx context.Context, req *UpdateMoleculeRequest) (*UpdateMoleculeResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "molecule_id is required")
	}

	ctx = s.extractContext(ctx)
	s.logger.Debug("UpdateMolecule called", "id", req.Id)

	if s.moleculeApp == nil {
		return &UpdateMoleculeResponse{
			Molecule: &MoleculeProto{
				Id:     req.Id,
				Smiles: req.Smiles,
				Name:   req.Name,
			},
		}, nil
	}

	cmd := &UpdateMoleculeCommand{
		ID:     req.Id,
		Name:   req.Name,
		Smiles: req.Smiles,
	}
	mol, err := s.moleculeApp.Update(ctx, cmd)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &UpdateMoleculeResponse{
		Molecule: domainToProto(mol),
	}, nil
}

// DeleteMolecule 删除分子（软删除）
func (s *MoleculeServiceServer) DeleteMolecule(ctx context.Context, req *DeleteMoleculeRequest) (*DeleteMoleculeResponse, error) {
	if req == nil || req.Id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "molecule_id is required")
	}

	ctx = s.extractContext(ctx)
	s.logger.Debug("DeleteMolecule called", "id", req.Id)

	if s.moleculeApp == nil {
		return &DeleteMoleculeResponse{Success: true}, nil
	}

	err := s.moleculeApp.Delete(ctx, req.Id)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &DeleteMoleculeResponse{Success: true}, nil
}

// ListMolecules 列出分子
func (s *MoleculeServiceServer) ListMolecules(ctx context.Context, req *ListMoleculesRequest) (*ListMoleculesResponse, error) {
	// 分页参数校验
	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		return nil, status.Errorf(codes.InvalidArgument, "page_size must be between 1 and 100")
	}

	ctx = s.extractContext(ctx)
	s.logger.Debug("ListMolecules called", "page_size", pageSize)

	if s.moleculeApp == nil {
		// 模拟返回
		return &ListMoleculesResponse{
			Molecules: []*MoleculeProto{
				{Id: "mol-001", Smiles: "C1=CC=CC=C1", Name: "Benzene"},
				{Id: "mol-002", Smiles: "CC(=O)O", Name: "Acetic acid"},
			},
			TotalCount:    2,
			NextPageToken: "",
		}, nil
	}

	// 解析分页 token
	cursor := ""
	if req.PageToken != "" {
		decoded, err := decodePageToken(req.PageToken)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid page_token")
		}
		cursor = decoded
	}

	opts := &ListMoleculesOptions{
		PageSize:        pageSize,
		Cursor:          cursor,
		MoleculeType:    req.MoleculeType,
		OLEDLayer:       req.OledLayer,
		SortBy:          req.SortBy,
		SortDescending:  req.SortDescending,
	}

	list, err := s.moleculeApp.List(ctx, opts)
	if err != nil {
		return nil, mapDomainError(err)
	}

	// 转换结果
	molecules := make([]*MoleculeProto, 0, len(list.Molecules))
	for _, mol := range list.Molecules {
		molecules = append(molecules, domainToProto(mol))
	}

	// 生成下一页 token
	nextPageToken := ""
	if list.NextCursor != "" {
		nextPageToken = encodePageToken(list.NextCursor)
	}

	return &ListMoleculesResponse{
		Molecules:     molecules,
		TotalCount:    int32(list.TotalCount),
		NextPageToken: nextPageToken,
	}, nil
}

// SearchSimilar 相似度搜索
func (s *MoleculeServiceServer) SearchSimilar(ctx context.Context, req *SearchSimilarRequest) (*SearchSimilarResponse, error) {
	// 参数校验
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "request is required")
	}
	query := req.Smiles
	if query == "" {
		query = req.Inchi
	}
	if query == "" {
		return nil, status.Errorf(codes.InvalidArgument, "SMILES or InChI is required")
	}

	threshold := req.Threshold
	if threshold < 0.0 || threshold > 1.0 {
		return nil, status.Errorf(codes.InvalidArgument, "threshold must be between 0.0 and 1.0")
	}
	if threshold == 0 {
		threshold = 0.7 // 默认阈值
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 10
	}

	ctx = s.extractContext(ctx)
	s.logger.Debug("SearchSimilar called", "query", query, "threshold", threshold)

	if s.similaritySearch == nil {
		// 模拟返回
		return &SearchSimilarResponse{
			Results: []*SimilarMoleculeProto{
				{
					Molecule:   &MoleculeProto{Id: "mol-001", Smiles: "C1=CC=CC=C1"},
					Similarity: 0.95,
				},
			},
			TotalCount: 1,
		}, nil
	}

	results, err := s.similaritySearch.Search(ctx, query, threshold, req.FingerprintType, limit)
	if err != nil {
		return nil, mapDomainError(err)
	}

	protoResults := make([]*SimilarMoleculeProto, 0, len(results))
	for _, r := range results {
		protoResults = append(protoResults, &SimilarMoleculeProto{
			Molecule:   domainToProto(r.Molecule),
			Similarity: r.Similarity,
		})
	}

	return &SearchSimilarResponse{
		Results:    protoResults,
		TotalCount: int32(len(protoResults)),
	}, nil
}

// PredictProperties 预测分子属性
func (s *MoleculeServiceServer) PredictProperties(ctx context.Context, req *PredictPropertiesRequest) (*PredictPropertiesResponse, error) {
	if req == nil || req.Smiles == "" {
		return nil, status.Errorf(codes.InvalidArgument, "SMILES is required")
	}
	if !isValidSMILES(req.Smiles) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid SMILES format")
	}

	ctx = s.extractContext(ctx)
	s.logger.Debug("PredictProperties called", "smiles", req.Smiles)

	if s.moleculeApp == nil {
		// 模拟返回 OLED 材料相关属性
		return &PredictPropertiesResponse{
			Properties: &MoleculePropertiesProto{
				Homo:           -5.2,
				Lumo:           -2.1,
				BandGap:        3.1,
				EmissionWavelength: 460.0,
				QuantumYield:   0.85,
			},
		}, nil
	}

	props, err := s.moleculeApp.PredictProperties(ctx, req.Smiles)
	if err != nil {
		return nil, mapDomainError(err)
	}

	return &PredictPropertiesResponse{
		Properties: &MoleculePropertiesProto{
			Homo:               props.HOMO,
			Lumo:               props.LUMO,
			BandGap:            props.BandGap,
			EmissionWavelength: props.EmissionWavelength,
			QuantumYield:       props.QuantumYield,
		},
	}, nil
}

// 向后兼容的方法
func (s *MoleculeServiceServer) GetMoleculeLegacy(ctx context.Context, req *GetMoleculeRequestLegacy) (*MoleculeResponseLegacy, error) {
	resp, err := s.GetMolecule(ctx, &GetMoleculeRequest{Id: req.GetId()})
	if err != nil {
		return nil, err
	}
	return &MoleculeResponseLegacy{
		Id:              resp.Molecule.Id,
		Smiles:          resp.Molecule.Smiles,
		InchiKey:        resp.Molecule.InchiKey,
		MolecularWeight: resp.Molecule.MolecularWeight,
		Formula:         resp.Molecule.Formula,
	}, nil
}

func (s *MoleculeServiceServer) SearchMolecules(ctx context.Context, req *SearchMoleculesRequestLegacy) (*SearchMoleculesResponseLegacy, error) {
	resp, err := s.SearchSimilar(ctx, &SearchSimilarRequest{
		Smiles:    req.Query,
		Threshold: 0.7,
	})
	if err != nil {
		return nil, err
	}

	results := make([]*MoleculeResponseLegacy, 0, len(resp.Results))
	for _, r := range resp.Results {
		results = append(results, &MoleculeResponseLegacy{
			Id:              r.Molecule.Id,
			Smiles:          r.Molecule.Smiles,
			InchiKey:        r.Molecule.InchiKey,
			MolecularWeight: r.Molecule.MolecularWeight,
			Formula:         r.Molecule.Formula,
		})
	}

	return &SearchMoleculesResponseLegacy{
		Results:    results,
		TotalCount: resp.TotalCount,
	}, nil
}

func (s *MoleculeServiceServer) CalculateSimilarity(ctx context.Context, req *SimilarityRequestLegacy) (*SimilarityResponseLegacy, error) {
	// 模拟相似度计算
	return &SimilarityResponseLegacy{
		Similarity: 0.85,
		Method:     "Tanimoto",
	}, nil
}

// extractContext 从 gRPC metadata 提取上下文信息
func (s *MoleculeServiceServer) extractContext(ctx context.Context) context.Context {
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

// domainToProto 将领域模型转换为 Protobuf 消息
func domainToProto(mol *Molecule) *MoleculeProto {
	if mol == nil {
		return nil
	}
	return &MoleculeProto{
		Id:              mol.ID,
		Smiles:          mol.SMILES,
		InchiKey:        mol.InChIKey,
		MolecularWeight: mol.MolecularWeight,
		Formula:         mol.Formula,
		Name:            mol.Name,
		MoleculeType:    mol.MoleculeType,
		OledLayer:       mol.OLEDLayer,
	}
}

// protoToCreateCommand 将 Protobuf 消息转换为领域命令
func protoToCreateCommand(req *CreateMoleculeRequest) *CreateMoleculeCommand {
	return &CreateMoleculeCommand{
		SMILES:       req.Smiles,
		Name:         req.Name,
		MoleculeType: req.MoleculeType,
		OLEDLayer:    req.OledLayer,
		Properties:   req.Properties,
	}
}

// mapDomainError 将领域错误映射为 gRPC 状态码
func mapDomainError(err error) error {
	if err == nil {
		return nil
	}

	// 检查错误类型
	var errNotFound *ErrNotFound
	var errValidation *ErrValidation
	var errConflict *ErrConflict
	var errUnauthorized *ErrUnauthorized

	if errors.As(err, &errNotFound) {
		return status.Errorf(codes.NotFound, err.Error())
	}
	if errors.As(err, &errValidation) {
		return status.Errorf(codes.InvalidArgument, err.Error())
	}
	if errors.As(err, &errConflict) {
		return status.Errorf(codes.AlreadyExists, err.Error())
	}
	if errors.As(err, &errUnauthorized) {
		return status.Errorf(codes.PermissionDenied, err.Error())
	}

	// 检查错误消息
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

// encodePageToken 编码分页 token (base64)
func encodePageToken(cursor string) string {
	return base64.StdEncoding.EncodeToString([]byte(cursor))
}

// decodePageToken 解码分页 token
func decodePageToken(token string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// isValidSMILES 简单的 SMILES 格式验证
func isValidSMILES(smiles string) bool {
	if smiles == "" {
		return false
	}
	// 基本的 SMILES 字符验证
	validChars := regexp.MustCompile(`^[A-Za-z0-9@+\-\[\]\(\)\\/#=%\.\*]+$`)
	return validChars.MatchString(smiles)
}

// generateID 生成简单的 ID
func generateID() string {
	return fmt.Sprintf("%d", randomInt())
}

var idCounter int64 = 0

func randomInt() int64 {
	idCounter++
	return idCounter
}

// 领域模型类型定义
type Molecule struct {
	ID              string
	SMILES          string
	InChIKey        string
	MolecularWeight float64
	Formula         string
	Name            string
	MoleculeType    string
	OLEDLayer       string
}

type CreateMoleculeCommand struct {
	SMILES       string
	Name         string
	MoleculeType string
	OLEDLayer    string
	Properties   map[string]float64
}

type UpdateMoleculeCommand struct {
	ID     string
	Name   string
	Smiles string
}

type ListMoleculesOptions struct {
	PageSize       int
	Cursor         string
	MoleculeType   string
	OLEDLayer      string
	SortBy         string
	SortDescending bool
}

type MoleculeList struct {
	Molecules  []*Molecule
	TotalCount int
	NextCursor string
}

type MoleculeProperties struct {
	HOMO               float64
	LUMO               float64
	BandGap            float64
	EmissionWavelength float64
	QuantumYield       float64
}

type SimilarMolecule struct {
	Molecule   *Molecule
	Similarity float64
}

// 错误类型
type ErrNotFound struct{ Msg string }

func (e *ErrNotFound) Error() string { return e.Msg }

type ErrValidation struct{ Msg string }

func (e *ErrValidation) Error() string { return e.Msg }

type ErrConflict struct{ Msg string }

func (e *ErrConflict) Error() string { return e.Msg }

type ErrUnauthorized struct{ Msg string }

func (e *ErrUnauthorized) Error() string { return e.Msg }

// Protobuf 消息类型 (模拟)
type UnimplementedMoleculeServiceServer struct{}

type GetMoleculeRequest struct{ Id string }

func (req *GetMoleculeRequest) GetId() string { return req.Id }

type GetMoleculeResponse struct{ Molecule *MoleculeProto }

type MoleculeProto struct {
	Id              string
	Smiles          string
	InchiKey        string
	MolecularWeight float64
	Formula         string
	Name            string
	MoleculeType    string
	OledLayer       string
}

type CreateMoleculeRequest struct {
	Smiles       string
	Name         string
	MoleculeType string
	OledLayer    string
	Properties   map[string]float64
}

type CreateMoleculeResponse struct{ Molecule *MoleculeProto }

type UpdateMoleculeRequest struct {
	Id     string
	Name   string
	Smiles string
}

type UpdateMoleculeResponse struct{ Molecule *MoleculeProto }

type DeleteMoleculeRequest struct{ Id string }
type DeleteMoleculeResponse struct{ Success bool }

type ListMoleculesRequest struct {
	PageSize       int32
	PageToken      string
	MoleculeType   string
	OledLayer      string
	SortBy         string
	SortDescending bool
}

func (r *ListMoleculesRequest) GetPageSize() int32 { return r.PageSize }

type ListMoleculesResponse struct {
	Molecules     []*MoleculeProto
	TotalCount    int32
	NextPageToken string
}

type SearchSimilarRequest struct {
	Smiles          string
	Inchi           string
	Threshold       float64
	FingerprintType string
	Limit           int32
}

type SearchSimilarResponse struct {
	Results    []*SimilarMoleculeProto
	TotalCount int32
}

type SimilarMoleculeProto struct {
	Molecule   *MoleculeProto
	Similarity float64
}

type PredictPropertiesRequest struct{ Smiles string }
type PredictPropertiesResponse struct{ Properties *MoleculePropertiesProto }

type MoleculePropertiesProto struct {
	Homo               float64
	Lumo               float64
	BandGap            float64
	EmissionWavelength float64
	QuantumYield       float64
}

// 向后兼容的旧类型
type GetMoleculeRequestLegacy struct{ Id string }

func (req *GetMoleculeRequestLegacy) GetId() string { return req.Id }

type MoleculeResponseLegacy struct {
	Id              string
	Smiles          string
	InchiKey        string
	MolecularWeight float64
	Formula         string
}

type SearchMoleculesRequestLegacy struct{ Query string }
type SearchMoleculesResponseLegacy struct {
	Results    []*MoleculeResponseLegacy
	TotalCount int32
}

type SimilarityRequestLegacy struct{ Mol1, Mol2 string }
type SimilarityResponseLegacy struct {
	Similarity float64
	Method     string
}

//Personal.AI order the ending
