package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	pb "github.com/turtacn/KeyIP-Intelligence/api/proto/v1"
)

// MoleculeServiceServer implements the gRPC MoleculeService
type MoleculeServiceServer struct {
	pb.UnimplementedMoleculeServiceServer
	moleculeRepo     molecule.MoleculeRepository
	similaritySearch patent_mining.SimilaritySearchService
	logger           logging.Logger
}

// NewMoleculeServiceServer creates a new MoleculeServiceServer instance
func NewMoleculeServiceServer(
	moleculeRepo molecule.MoleculeRepository,
	similaritySearch patent_mining.SimilaritySearchService,
	logger logging.Logger,
) *MoleculeServiceServer {
	return &MoleculeServiceServer{
		moleculeRepo:     moleculeRepo,
		similaritySearch: similaritySearch,
		logger:           logger,
	}
}

// GetMolecule retrieves a molecule by ID
func (s *MoleculeServiceServer) GetMolecule(
	ctx context.Context,
	req *pb.GetMoleculeRequest,
) (*pb.GetMoleculeResponse, error) {
	if req.MoleculeId == "" {
		return nil, status.Error(codes.InvalidArgument, "molecule_id is required")
	}

	mol, err := s.moleculeRepo.FindByID(ctx, req.MoleculeId)
	if err != nil {
		s.logger.Error("failed to get molecule",
			logging.Err(err),
			logging.String("molecule_id", req.MoleculeId))
		return nil, mapDomainError(err)
	}

	return &pb.GetMoleculeResponse{
		Molecule: domainToProto(mol),
	}, nil
}

// CreateMolecule creates a new molecule
func (s *MoleculeServiceServer) CreateMolecule(
	ctx context.Context,
	req *pb.CreateMoleculeRequest,
) (*pb.CreateMoleculeResponse, error) {
	if req.Smiles == "" {
		return nil, status.Error(codes.InvalidArgument, "smiles is required")
	}

	// Determine molecule source from request metadata or default to manual
	source := molecule.SourceManual
	if req.OledLayer != "" {
		// Use OledLayer as source reference
	}

	// Create domain entity
	// Note: NewMolecule 3rd argument is SourceReference, not Name
	mol, err := molecule.NewMolecule(req.Smiles, source, "")
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	mol.Name = req.Name

	// Set additional fields via SetStructureIdentifiers if InChI is provided
	if req.Inchi != "" {
		_ = mol.SetStructureIdentifiers("", req.Inchi, "", "", 0)
	}

	// Set properties via AddProperty if Properties map is provided
	if req.Properties != nil {
		for k, v := range req.Properties {
			floatVal, err := strconv.ParseFloat(v, 64)
			if err == nil {
				_ = mol.AddProperty(&molecule.MolecularProperty{Name: k, Value: floatVal, Confidence: 1.0})
			}
		}
	}
	if req.Metadata != nil {
		mol.Metadata = make(map[string]any)
		for k, v := range req.Metadata {
			mol.Metadata[k] = v
		}
	}

	// Save to repository
	if err := s.moleculeRepo.Save(ctx, mol); err != nil {
		s.logger.Error("failed to create molecule",
			logging.Err(err),
			logging.String("smiles", req.Smiles))
		return nil, mapDomainError(err)
	}

	return &pb.CreateMoleculeResponse{
		Molecule: domainToProto(mol),
	}, nil
}

// UpdateMolecule updates an existing molecule
func (s *MoleculeServiceServer) UpdateMolecule(
	ctx context.Context,
	req *pb.UpdateMoleculeRequest,
) (*pb.UpdateMoleculeResponse, error) {
	if req.MoleculeId == "" {
		return nil, status.Error(codes.InvalidArgument, "molecule_id is required")
	}

	// Fetch existing molecule
	mol, err := s.moleculeRepo.FindByID(ctx, req.MoleculeId)
	if err != nil {
		s.logger.Error("failed to find molecule for update",
			logging.Err(err),
			logging.String("molecule_id", req.MoleculeId))
		return nil, mapDomainError(err)
	}

	// Update fields
	if req.Name != "" {
		mol.Name = req.Name
	}
	if req.Properties != nil {
		for k, v := range req.Properties {
			floatVal, err := strconv.ParseFloat(v, 64)
			if err == nil {
				_ = mol.AddProperty(&molecule.MolecularProperty{Name: k, Value: floatVal, Confidence: 1.0})
			}
		}
	}
	if req.Metadata != nil {
		mol.Metadata = make(map[string]any)
		for k, v := range req.Metadata {
			mol.Metadata[k] = v
		}
	}

	// Save changes
	if err := s.moleculeRepo.Update(ctx, mol); err != nil {
		s.logger.Error("failed to update molecule",
			logging.Err(err),
			logging.String("molecule_id", req.MoleculeId))
		return nil, mapDomainError(err)
	}

	return &pb.UpdateMoleculeResponse{
		Molecule: domainToProto(mol),
	}, nil
}

// DeleteMolecule soft-deletes a molecule
func (s *MoleculeServiceServer) DeleteMolecule(
	ctx context.Context,
	req *pb.DeleteMoleculeRequest,
) (*pb.DeleteMoleculeResponse, error) {
	if req.MoleculeId == "" {
		return nil, status.Error(codes.InvalidArgument, "molecule_id is required")
	}

	if err := s.moleculeRepo.Delete(ctx, req.MoleculeId); err != nil {
		s.logger.Error("failed to delete molecule",
			logging.Err(err),
			logging.String("molecule_id", req.MoleculeId))
		return nil, mapDomainError(err)
	}

	return &pb.DeleteMoleculeResponse{
		Success: true,
	}, nil
}

// ListMolecules lists molecules with pagination and filters
func (s *MoleculeServiceServer) ListMolecules(
	ctx context.Context,
	req *pb.ListMoleculesRequest,
) (*pb.ListMoleculesResponse, error) {
	// Validate page size
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		return nil, status.Error(codes.InvalidArgument, "page_size must be between 1 and 100")
	}

	// Decode page token
	var offset int
	if req.PageToken != "" {
		decoded, err := base64.StdEncoding.DecodeString(req.PageToken)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid page_token")
		}
		parsedOffset, _ := strconv.ParseInt(string(decoded), 10, 64)
		offset = int(parsedOffset)
	}

	// Build query using MoleculeQuery
	query := &molecule.MoleculeQuery{
		Offset:    offset,
		Limit:     pageSize,
		SortBy:    req.SortBy,
		SortOrder: "desc",
	}

	// Query repository using Search
	result, err := s.moleculeRepo.Search(ctx, query)
	if err != nil {
		s.logger.Error("failed to list molecules", logging.Err(err))
		return nil, mapDomainError(err)
	}

	// Convert molecules to proto
	pbMolecules := make([]*pb.Molecule, len(result.Molecules))
	for i, mol := range result.Molecules {
		pbMolecules[i] = domainToProto(mol)
	}

	// Generate next page token
	var nextPageToken string
	if result.HasMore {
		nextOffset := offset + len(result.Molecules)
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", nextOffset)))
	}

	return &pb.ListMoleculesResponse{
		Molecules:     pbMolecules,
		NextPageToken: nextPageToken,
		TotalCount:    result.Total,
	}, nil
}

// SearchSimilar searches for similar molecules
func (s *MoleculeServiceServer) SearchSimilar(
	ctx context.Context,
	req *pb.SearchSimilarRequest,
) (*pb.SearchSimilarResponse, error) {
	if req.Smiles == "" && req.Inchi == "" {
		return nil, status.Error(codes.InvalidArgument, "either smiles or inchi is required")
	}

	// Validate threshold
	if req.Threshold < 0.0 || req.Threshold > 1.0 {
		return nil, status.Error(codes.InvalidArgument, "threshold must be between 0.0 and 1.0")
	}

	// Build search query
	query := &patent_mining.SimilarityQuery{
		SMILES:          req.Smiles,
		InChI:           req.Inchi,
		Threshold:       req.Threshold,
		FingerprintType: req.FingerprintType,
		MaxResults:      int(req.MaxResults),
	}

	// Perform search
	results, err := s.similaritySearch.Search(ctx, query)
	if err != nil {
		s.logger.Error("failed to search similar molecules", logging.Err(err))
		return nil, mapDomainError(err)
	}

	// Convert results to proto
	similarMolecules := make([]*pb.SimilarMolecule, len(results))
	for i, result := range results {
		similarMolecules[i] = &pb.SimilarMolecule{
			Molecule:   moleculeInfoToProto(result.Molecule),
			Similarity: result.Similarity,
			Method:     result.Method,
		}
	}

	return &pb.SearchSimilarResponse{
		SimilarMolecules: similarMolecules,
	}, nil
}

// moleculeInfoToProto converts MoleculeInfo to protobuf Molecule
func moleculeInfoToProto(info *patent_mining.MoleculeInfo) *pb.Molecule {
	if info == nil {
		return nil
	}
	return &pb.Molecule{
		MoleculeId:   info.ID,
		Smiles:       info.SMILES,
		Inchi:        info.InChI,
		Name:         info.Name,
		MoleculeType: info.Type,
		OledLayer:    info.OLEDLayer,
	}
}

// PredictProperties predicts molecular properties
func (s *MoleculeServiceServer) PredictProperties(
	ctx context.Context,
	req *pb.PredictPropertiesRequest,
) (*pb.PredictPropertiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "PredictProperties is not implemented")
}

// BatchSimilaritySearch performs SimilaritySearch for multiple molecules concurrently
func (s *MoleculeServiceServer) BatchSimilaritySearch(
	req *pb.BatchSimilaritySearchRequest,
	stream pb.MoleculeService_BatchSimilaritySearchServer,
) error {
	return status.Error(codes.Unimplemented, "BatchSimilaritySearch is not implemented")
}

// AssessPatentability evaluates novelty, inventive step, and utility
func (s *MoleculeServiceServer) AssessPatentability(
	ctx context.Context,
	req *pb.AssessPatentabilityRequest,
) (*pb.AssessPatentabilityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "AssessPatentability is not implemented")
}

//Personal.AI order the ending