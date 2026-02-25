// File: internal/interfaces/grpc/services/patent_service.go
package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/reporting"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	pb "github.com/turtacn/KeyIP-Intelligence/api/proto/v1"
)

const (
	maxCitationDepth     = 5
	defaultCitationDepth = 2
	ftoCheckTimeout      = 30 * time.Second
	maxNetworkNodes      = 1000
)

// PatentServiceServer implements the gRPC PatentService
type PatentServiceServer struct {
	pb.UnimplementedPatentServiceServer
	patentRepo  patent.PatentRepository
	ftoService  reporting.FTOReportService
	logger      logging.Logger
}

// NewPatentServiceServer creates a new PatentServiceServer instance
func NewPatentServiceServer(
	patentRepo patent.PatentRepository,
	ftoService reporting.FTOReportService,
	logger logging.Logger,
) *PatentServiceServer {
	return &PatentServiceServer{
		patentRepo: patentRepo,
		ftoService: ftoService,
		logger:     logger,
	}
}

// GetPatent retrieves a patent by patent number
func (s *PatentServiceServer) GetPatent(
	ctx context.Context,
	req *pb.GetPatentRequest,
) (*pb.GetPatentResponse, error) {
	if req.PatentNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "patent_number is required")
	}

	// Validate patent number format
	if !isValidPatentNumber(req.PatentNumber) {
		return nil, status.Error(codes.InvalidArgument, "invalid patent number format")
	}

	pat, err := s.patentRepo.FindByNumber(ctx, req.PatentNumber)
	if err != nil {
		s.logger.Error("failed to get patent", "error", err, "patent_number", req.PatentNumber)
		return nil, mapPatentError(err)
	}

	return &pb.GetPatentResponse{
		Patent: patentDomainToProto(pat),
	}, nil
}

// SearchPatents searches patents with filters and pagination
func (s *PatentServiceServer) SearchPatents(
	ctx context.Context,
	req *pb.SearchPatentsRequest,
) (*pb.SearchPatentsResponse, error) {
	// Validate page size
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		return nil, status.Error(codes.InvalidArgument, "page_size must be between 1 and 100")
	}

	// Decode page token
	var offset int64
	if req.PageToken != "" {
		decoded, err := base64.StdEncoding.DecodeString(req.PageToken)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid page_token")
		}
		offset, _ = strconv.ParseInt(string(decoded), 10, 64)
	}

	// Build search filter
	filter := &patent.SearchFilter{
		Query:          req.Query,
		IpcClasses:     req.IpcClasses,
		CpcClasses:     req.CpcClasses,
		PatentOffices:  req.PatentOffices,
		ApplicationDateFrom: parseDate(req.ApplicationDateFrom),
		ApplicationDateTo:   parseDate(req.ApplicationDateTo),
		PublicationDateFrom: parseDate(req.PublicationDateFrom),
		PublicationDateTo:   parseDate(req.PublicationDateTo),
		Offset:         offset,
		Limit:          int64(pageSize),
		SortBy:         req.SortBy,
		SortOrder:      req.SortOrder,
	}

	// Execute search
	patents, total, err := s.patentRepo.Search(ctx, filter)
	if err != nil {
		s.logger.Error("failed to search patents", "error", err, "query", req.Query)
		return nil, mapPatentError(err)
	}

	// Convert patents to proto
	pbPatents := make([]*pb.Patent, len(patents))
	for i, pat := range patents {
		pbPatents[i] = patentDomainToProto(pat)
	}

	// Generate next page token
	var nextPageToken string
	if int64(len(patents)) == int64(pageSize) {
		nextOffset := offset + int64(len(patents))
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", nextOffset)))
	}

	return &pb.SearchPatentsResponse{
		Patents:       pbPatents,
		NextPageToken: nextPageToken,
		TotalCount:    total,
	}, nil
}

// AnalyzeClaims analyzes patent claims structure
func (s *PatentServiceServer) AnalyzeClaims(
	ctx context.Context,
	req *pb.AnalyzeClaimsRequest,
) (*pb.AnalyzeClaimsResponse, error) {
	if req.PatentNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "patent_number is required")
	}

	// Validate patent number format
	if !isValidPatentNumber(req.PatentNumber) {
		return nil, status.Error(codes.InvalidArgument, "invalid patent number format")
	}

	// Fetch patent
	pat, err := s.patentRepo.FindByNumber(ctx, req.PatentNumber)
	if err != nil {
		s.logger.Error("failed to get patent for claims analysis", "error", err, "patent_number", req.PatentNumber)
		return nil, mapPatentError(err)
	}

	// Analyze claims structure
	claimTree := pat.AnalyzeClaims()

	return &pb.AnalyzeClaimsResponse{
		PatentNumber:       req.PatentNumber,
		ClaimTree:          claimTreeToProto(claimTree),
		IndependentCount:   int32(claimTree.IndependentCount()),
		DependentCount:     int32(claimTree.DependentCount()),
		TotalClaims:        int32(claimTree.TotalCount()),
		MaxDependencyDepth: int32(claimTree.MaxDepth()),
	}, nil
}

// CheckFTO performs quick FTO (Freedom to Operate) check
func (s *PatentServiceServer) CheckFTO(
	ctx context.Context,
	req *pb.CheckFTORequest,
) (*pb.CheckFTOResponse, error) {
	if req.TargetSmiles == "" {
		return nil, status.Error(codes.InvalidArgument, "target_smiles is required")
	}
	if req.Jurisdiction == "" {
		return nil, status.Error(codes.InvalidArgument, "jurisdiction is required")
	}

	// Set timeout for FTO check
	checkCtx, cancel := context.WithTimeout(ctx, ftoCheckTimeout)
	defer cancel()

	// Build FTO request
	ftoReq := &reporting.FTOQuickCheckRequest{
		TargetMolecule: req.TargetSmiles,
		Jurisdiction:   req.Jurisdiction,
		IncludeExpired: req.IncludeExpired,
	}

	// Perform FTO check
	result, err := s.ftoService.QuickCheck(checkCtx, ftoReq)
	if err != nil {
		s.logger.Error("failed to perform FTO check", "error", err, "smiles", req.TargetSmiles)
		if checkCtx.Err() == context.DeadlineExceeded {
			return nil, status.Error(codes.DeadlineExceeded, "FTO check timeout")
		}
		return nil, mapPatentError(err)
	}

	// Convert blocking patents
	blockingPatents := make([]*pb.BlockingPatent, len(result.BlockingPatents))
	for i, bp := range result.BlockingPatents {
		blockingPatents[i] = &pb.BlockingPatent{
			PatentNumber:    bp.PatentNumber,
			Title:           bp.Title,
			RiskLevel:       bp.RiskLevel,
			Similarity:      bp.Similarity,
			ExpiryDate:      bp.ExpiryDate.Unix(),
			LegalStatus:     bp.LegalStatus,
			MatchedClaims:   bp.MatchedClaims,
		}
	}

	return &pb.CheckFTOResponse{
		CanOperate:      result.CanOperate,
		RiskLevel:       result.RiskLevel,
		Confidence:      result.Confidence,
		BlockingPatents: blockingPatents,
		Recommendation:  result.Recommendation,
		CheckedAt:       time.Now().Unix(),
	}, nil
}

// GetPatentFamily retrieves patent family members
func (s *PatentServiceServer) GetPatentFamily(
	ctx context.Context,
	req *pb.GetPatentFamilyRequest,
) (*pb.GetPatentFamilyResponse, error) {
	if req.PatentNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "patent_number is required")
	}

	// Validate patent number format
	if !isValidPatentNumber(req.PatentNumber) {
		return nil, status.Error(codes.InvalidArgument, "invalid patent number format")
	}

	// Get patent family
	family, err := s.patentRepo.GetFamily(ctx, req.PatentNumber)
	if err != nil {
		s.logger.Error("failed to get patent family", "error", err, "patent_number", req.PatentNumber)
		return nil, mapPatentError(err)
	}

	// Convert family members
	familyMembers := make([]*pb.FamilyMember, len(family.Members))
	for i, member := range family.Members {
		familyMembers[i] = &pb.FamilyMember{
			PatentNumber:      member.PatentNumber,
			PatentOffice:      member.PatentOffice,
			ApplicationDate:   member.ApplicationDate.Unix(),
			PublicationDate:   member.PublicationDate.Unix(),
			LegalStatus:       member.LegalStatus,
			IsRepresentative:  member.IsRepresentative,
		}
	}

	return &pb.GetPatentFamilyResponse{
		FamilyId:      family.FamilyID,
		FamilyMembers: familyMembers,
		TotalMembers:  int32(len(family.Members)),
	}, nil
}

// GetCitationNetwork retrieves citation network for a patent
func (s *PatentServiceServer) GetCitationNetwork(
	ctx context.Context,
	req *pb.GetCitationNetworkRequest,
) (*pb.GetCitationNetworkResponse, error) {
	if req.PatentNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "patent_number is required")
	}

	// Validate patent number format
	if !isValidPatentNumber(req.PatentNumber) {
		return nil, status.Error(codes.InvalidArgument, "invalid patent number format")
	}

	// Validate depth
	depth := req.Depth
	if depth <= 0 {
		depth = defaultCitationDepth
	}
	if depth > maxCitationDepth {
		return nil, status.Errorf(codes.InvalidArgument, "depth must be between 1 and %d", maxCitationDepth)
	}

	// Build network query
	query := &patent.CitationNetworkQuery{
		PatentNumber:     req.PatentNumber,
		Depth:            int(depth),
		IncludeCiting:    req.IncludeCiting,
		IncludeCited:     req.IncludeCited,
		MaxNodes:         maxNetworkNodes,
	}

	// Get citation network
	network, err := s.patentRepo.GetCitationNetwork(ctx, query)
	if err != nil {
		s.logger.Error("failed to get citation network", "error", err, "patent_number", req.PatentNumber)
		return nil, mapPatentError(err)
	}

	// Convert nodes
	nodes := make([]*pb.CitationNode, len(network.Nodes))
	for i, node := range network.Nodes {
		nodes[i] = &pb.CitationNode{
			PatentNumber:    node.PatentNumber,
			Title:           node.Title,
			PublicationDate: node.PublicationDate.Unix(),
			CitationLevel:   int32(node.Level),
			IsRoot:          node.IsRoot,
		}
	}

	// Convert edges
	edges := make([]*pb.CitationEdge, len(network.Edges))
	for i, edge := range network.Edges {
		edges[i] = &pb.CitationEdge{
			FromPatent: edge.FromPatent,
			ToPatent:   edge.ToPatent,
			EdgeType:   edge.EdgeType,
		}
	}

	return &pb.GetCitationNetworkResponse{
		Nodes:       nodes,
		Edges:       edges,
		TotalNodes:  int32(len(network.Nodes)),
		TotalEdges:  int32(len(network.Edges)),
		IsTruncated: network.IsTruncated,
	}, nil
}

// patentDomainToProto converts domain patent to protobuf message
func patentDomainToProto(pat *patent.Patent) *pb.Patent {
	if pat == nil {
		return nil
	}

	return &pb.Patent{
		PatentNumber:      pat.PatentNumber(),
		Title:             pat.Title(),
		Abstract:          pat.Abstract(),
		Applicants:        pat.Applicants(),
		Inventors:         pat.Inventors(),
		IpcClasses:        pat.IPCClasses(),
		CpcClasses:        pat.CPCClasses(),
		PatentOffice:      pat.PatentOffice(),
		ApplicationDate:   pat.ApplicationDate().Unix(),
		PublicationDate:   pat.PublicationDate().Unix(),
		GrantDate:         pat.GrantDate().Unix(),
		LegalStatus:       pat.LegalStatus(),
		ClaimsCount:       int32(pat.ClaimsCount()),
		CitationsCount:    int32(pat.CitationsCount()),
		FamilyId:          pat.FamilyID(),
		PriorityNumber:    pat.PriorityNumber(),
		PriorityDate:      pat.PriorityDate().Unix(),
	}
}

// claimTreeToProto converts domain claim tree to protobuf message
func claimTreeToProto(tree *patent.ClaimTree) *pb.ClaimTree {
	if tree == nil {
		return nil
	}

	// Convert independent claims
	independentClaims := make([]*pb.Claim, len(tree.IndependentClaims))
	for i, claim := range tree.IndependentClaims {
		independentClaims[i] = claimToProto(claim)
	}

	return &pb.ClaimTree{
		IndependentClaims: independentClaims,
		TotalClaims:       int32(tree.TotalCount()),
		MaxDepth:          int32(tree.MaxDepth()),
	}
}

// claimToProto converts domain claim to protobuf message
func claimToProto(claim *patent.Claim) *pb.Claim {
	if claim == nil {
		return nil
	}

	// Convert dependent claims recursively
	dependentClaims := make([]*pb.Claim, len(claim.DependentClaims))
	for i, dep := range claim.DependentClaims {
		dependentClaims[i] = claimToProto(dep)
	}

	return &pb.Claim{
		ClaimNumber:       claim.ClaimNumber,
		ClaimText:         claim.ClaimText,
		ClaimType:         claim.ClaimType,
		IsIndependent:     claim.IsIndependent,
		DependsOn:         claim.DependsOn,
		DependentClaims:   dependentClaims,
		TechnicalFeatures: claim.TechnicalFeatures,
	}
}

// isValidPatentNumber validates patent number format
func isValidPatentNumber(patentNumber string) bool {
	// Support common patent office prefixes: CN, US, EP, JP, KR, WO
	validFormats := []string{
		`^CN\d{9}[A-Z]?$`,                    // CN123456789A
		`^US\d{7,8}[A-Z]\d?$`,                // US1234567B2
		`^EP\d{7}[A-Z]\d?$`,                  // EP1234567A1
		`^JP\d{7,10}[A-Z]?$`,                 // JP2021123456A
		`^KR\d{10}[A-Z]\d?$`,                 // KR1020210001234B1
		`^WO\d{4}/\d{6}[A-Z]\d?$`,            // WO2021/123456A1
	}

	for _, pattern := range validFormats {
		matched, _ := regexp.MatchString(pattern, patentNumber)
		if matched {
			return true
		}
	}
	return false
}

// parseDate parses Unix timestamp to time.Time
func parseDate(timestamp int64) *time.Time {
	if timestamp <= 0 {
		return nil
	}
	t := time.Unix(timestamp, 0)
	return &t
}

// mapPatentError maps domain errors to gRPC status codes
func mapPatentError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.IsNotFound(err):
		return status.Error(codes.NotFound, err.Error())
	case errors.IsValidation(err):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.IsConflict(err):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.IsUnauthorized(err):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

//Personal.AI order the ending
