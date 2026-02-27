// File: internal/interfaces/grpc/services/patent_service.go
package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
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

	pat, err := s.patentRepo.GetByPatentNumber(ctx, req.PatentNumber)
	if err != nil {
		s.logger.Error("failed to get patent",
			logging.Err(err),
			logging.String("patent_number", req.PatentNumber))
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

	// Build search criteria
	var offices []patent.PatentOffice
	for _, o := range req.PatentOffices {
		offices = append(offices, patent.PatentOffice(o))
	}

	criteria := patent.PatentSearchCriteria{
		TitleKeywords:    []string{req.Query},
		AbstractKeywords: []string{req.Query},
		IPCCodes:         req.IpcClasses,
		Offices:          offices,
		FilingDateStart:  parseDate(req.ApplicationDateFrom),
		FilingDateEnd:    parseDate(req.ApplicationDateTo),
		Offset:           int(offset),
		Limit:            int(pageSize),
		SortBy:           req.SortBy,
	}

	// Execute search
	result, err := s.patentRepo.Search(ctx, criteria)
	if err != nil {
		s.logger.Error("failed to search patents",
			logging.Err(err),
			logging.String("query", req.Query))
		return nil, mapPatentError(err)
	}

	// Convert patents to proto
	pbPatents := make([]*pb.Patent, len(result.Patents))
	for i, pat := range result.Patents {
		pbPatents[i] = patentDomainToProto(pat)
	}

	// Generate next page token
	var nextPageToken string
	if int64(len(result.Patents)) == int64(pageSize) {
		nextOffset := offset + int64(len(result.Patents))
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", nextOffset)))
	}

	return &pb.SearchPatentsResponse{
		Patents:       pbPatents,
		NextPageToken: nextPageToken,
		TotalCount:    result.Total,
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
	pat, err := s.patentRepo.GetByPatentNumber(ctx, req.PatentNumber)
	if err != nil {
		s.logger.Error("failed to get patent for claims analysis",
			logging.Err(err),
			logging.String("patent_number", req.PatentNumber))
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
	_, cancel := context.WithTimeout(ctx, ftoCheckTimeout)
	defer cancel()

	// Note: FTO QuickCheck is not yet implemented in FTOReportService
	// This is a placeholder implementation returning a safe default response
	s.logger.Info("FTO check requested (placeholder implementation)",
		logging.String("smiles", req.TargetSmiles),
		logging.String("jurisdiction", req.Jurisdiction))

	return &pb.CheckFTOResponse{
		CanOperate:      false, // Conservative: assume cannot operate until analyzed
		RiskLevel:       "UNKNOWN",
		Confidence:      0.0,
		BlockingPatents: []*pb.BlockingPatent{},
		Recommendation:  "Please use the full FTO report generation for comprehensive analysis",
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

	// Get patent to find its FamilyID
	pat, err := s.patentRepo.GetByPatentNumber(ctx, req.PatentNumber)
	if err != nil {
		s.logger.Error("failed to get patent for family lookup",
			logging.Err(err),
			logging.String("patent_number", req.PatentNumber))
		return nil, mapPatentError(err)
	}

	// If patent has no family ID, return just this patent
	if pat.FamilyID == "" {
		return &pb.GetPatentFamilyResponse{
			FamilyId:      "",
			FamilyMembers: []*pb.FamilyMember{},
			TotalMembers:  0,
		}, nil
	}

	// Get family members by FamilyID
	familyPatents, err := s.patentRepo.GetByFamilyID(ctx, pat.FamilyID)
	if err != nil {
		s.logger.Error("failed to get patent family members",
			logging.Err(err),
			logging.String("family_id", pat.FamilyID))
		return nil, mapPatentError(err)
	}

	// Convert to family members
	familyMembers := make([]*pb.FamilyMember, len(familyPatents))
	for i, member := range familyPatents {
		var appDate, pubDate int64
		if member.FilingDate != nil {
			appDate = member.FilingDate.Unix()
		}
		if member.PublicationDate != nil {
			pubDate = member.PublicationDate.Unix()
		}
		familyMembers[i] = &pb.FamilyMember{
			PatentNumber:     member.PatentNumber,
			PatentOffice:     member.Jurisdiction,
			ApplicationDate:  appDate,
			PublicationDate:  pubDate,
			LegalStatus:      string(member.Status),
			IsRepresentative: member.PatentNumber == req.PatentNumber,
		}
	}

	return &pb.GetPatentFamilyResponse{
		FamilyId:      pat.FamilyID,
		FamilyMembers: familyMembers,
		TotalMembers:  int32(len(familyMembers)),
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

	// Note: Citation network functionality requires Neo4j graph repository integration
	// This is a placeholder implementation
	s.logger.Info("citation network requested (placeholder implementation)",
		logging.String("patent_number", req.PatentNumber),
		logging.Int("depth", int(depth)))

	// Return root node only for now
	rootNode := &pb.CitationNode{
		PatentNumber:  req.PatentNumber,
		Title:         "",
		CitationLevel: 0,
		IsRoot:        true,
	}

	return &pb.GetCitationNetworkResponse{
		Nodes:       []*pb.CitationNode{rootNode},
		Edges:       []*pb.CitationEdge{},
		TotalNodes:  1,
		TotalEdges:  0,
		IsTruncated: false,
	}, nil
}

// patentDomainToProto converts domain patent to protobuf message
func patentDomainToProto(pat *patent.Patent) *pb.Patent {
	if pat == nil {
		return nil
	}

	// Extract inventor names
	inventorNames := make([]string, len(pat.Inventors))
	for i, inv := range pat.Inventors {
		inventorNames[i] = inv.Name
	}

	// Handle nullable dates
	var appDate, pubDate, grantDate, prioDate int64
	if pat.FilingDate != nil {
		appDate = pat.FilingDate.Unix()
	}
	if pat.PublicationDate != nil {
		pubDate = pat.PublicationDate.Unix()
	}
	if pat.GrantDate != nil {
		grantDate = pat.GrantDate.Unix()
	}
	if pat.PriorityDate != nil {
		prioDate = pat.PriorityDate.Unix()
	}

	return &pb.Patent{
		PatentNumber:    pat.PatentNumber,
		Title:           pat.Title,
		Abstract:        pat.Abstract,
		Applicants:      []string{pat.AssigneeName},
		Inventors:       inventorNames,
		IpcClasses:      pat.IPCCodes,
		CpcClasses:      pat.CPCCodes,
		PatentOffice:    pat.Jurisdiction,
		ApplicationDate: appDate,
		PublicationDate: pubDate,
		GrantDate:       grantDate,
		LegalStatus:     string(pat.Status),
		ClaimsCount:     int32(len(pat.Claims)),
		CitationsCount:  int32(len(pat.Cites)),
		FamilyId:        pat.FamilyID,
		PriorityNumber:  pat.ApplicationNumber,
		PriorityDate:    prioDate,
	}
}

// claimTreeToProto converts domain claim tree to protobuf message
func claimTreeToProto(tree *patent.ClaimTree) *pb.ClaimTree {
	if tree == nil {
		return nil
	}

	// Convert root claims
	rootClaims := make([]*pb.Claim, len(tree.Roots))
	for i, node := range tree.Roots {
		rootClaims[i] = claimNodeToProto(node)
	}

	return &pb.ClaimTree{
		IndependentClaims: rootClaims,
		TotalClaims:       int32(tree.TotalCount()),
		MaxDepth:          int32(tree.MaxDepth()),
	}
}

// claimNodeToProto converts a claim node to protobuf message
func claimNodeToProto(node *patent.ClaimNode) *pb.Claim {
	if node == nil || node.Claim == nil {
		return nil
	}

	claim := node.Claim

	// Convert children recursively
	children := make([]*pb.Claim, len(node.Children))
	for i, child := range node.Children {
		children[i] = claimNodeToProto(child)
	}

	return &pb.Claim{
		ClaimNumber:       int32(claim.Number),
		ClaimText:         claim.Text,
		ClaimType:         claim.Type.String(),
		IsIndependent:     claim.Type == patent.ClaimTypeIndependent,
		DependsOn:         int32SliceFromIntSlice(claim.DependsOn),
		DependentClaims:   children,
		TechnicalFeatures: []string{}, // Placeholder - would extract from claim.Elements
	}
}

// int32SliceFromIntSlice converts []int to []int32
func int32SliceFromIntSlice(ints []int) []int32 {
	result := make([]int32, len(ints))
	for i, v := range ints {
		result[i] = int32(v)
	}
	return result
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
