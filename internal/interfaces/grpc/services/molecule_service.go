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
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	pb "github.com/turtacn/KeyIP-Intelligence/api/proto/molecule/v1"
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
		s.logger.Error("failed to get molecule", "error", err, "molecule_id", req.MoleculeId)
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

	// Create domain entity
	mol, err := molecule.NewMolecule(
		req.Smiles,
		req.Inchi,
		req.Name,
		req.MoleculeType,
		req.OledLayer,
	)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Set properties
	if req.Properties != nil {
		mol.SetProperties(req.Properties)
	}
	if req.Metadata != nil {
		mol.SetMetadata(req.Metadata)
	}

	// Save to repository
	if err := s.moleculeRepo.Create(ctx, mol); err != nil {
		s.logger.Error("failed to create molecule", "error", err, "smiles", req.Smiles)
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
		s.logger.Error("failed to find molecule for update", "error", err, "molecule_id", req.MoleculeId)
		return nil, mapDomainError(err)
	}

	// Update fields
	if req.Name != "" {
		mol.SetName(req.Name)
	}
	if req.Properties != nil {
		mol.SetProperties(req.Properties)
	}
	if req.Metadata != nil {
		mol.SetMetadata(req.Metadata)
	}

	// Save changes
	if err := s.moleculeRepo.Update(ctx, mol); err != nil {
		s.logger.Error("failed to update molecule", "error", err, "molecule_id", req.MoleculeId)
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
		s.logger.Error("failed to delete molecule", "error", err, "molecule_id", req.MoleculeId)
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

	// Build filter
	filter := &molecule.ListFilter{
		MoleculeType: req.MoleculeType,
		OledLayer:    req.OledLayer,
		Offset:       offset,
		Limit:        int64(pageSize),
		SortBy:       req.SortBy,
	}

	// Query repository
	molecules, total, err := s.moleculeRepo.List(ctx, filter)
	if err != nil {
		s.logger.Error("failed to list molecules", "error", err)
		return nil, mapDomainError(err)
	}

	// Convert molecules to proto
	pbMolecules := make([]*pb.Molecule, len(molecules))
	for i, mol := range molecules {
		pbMolecules[i] = domainToProto(mol)
	}

	// Generate next page token
	var nextPageToken string
	if int64(len(molecules)) == int64(pageSize) {
		nextOffset := offset + int64(len(molecules))
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", nextOffset)))
	}

	return &pb.ListMoleculesResponse{
		Molecules:     pbMolecules,
		NextPageToken: nextPageToken,
		TotalCount:    total,
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
		s.logger.Error("failed to search similar molecules", "error", err)
		return nil, mapDomainError(err)
	}

	// Convert results to proto
	similarMolecules := make([]*pb.SimilarMolecule, len(results))
	for i, result := range results {
		similarMolecules[i] = &pb.SimilarMolecule{
			Molecule:   domainToProto(result.Molecule),
			Similarity: result.Similarity,
			Method:     result.Method,
		}
	}

	return &pb.SearchSimilarResponse{
		SimilarMolecules: similarMolecules,
	}, nil
}

// PredictProperties predicts molecular properties
func (s *MoleculeServiceServer) PredictProperties(
	ctx context.Context,
	req *pb.PredictPropertiesRequest,
) (*pb.PredictPropertiesResponse, error) {
	if req.Smiles == "" {
		return nil, status.Error(codes.InvalidArgument, "smiles is required")
	}

	// Fetch or create molecule
	mol, err := s.moleculeRepo.FindBySMILES(ctx, req.Smiles)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, mapDomainError(err)
		}
		// Create temporary molecule for prediction
		mol, err = molecule.NewMolecule(req.Smiles, "", "", "", "")
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	// Get predicted properties from molecule entity
	props := mol.PredictedProperties()

	return &pb.PredictPropertiesResponse{
		Homo:               float32(props.HOMO),
		Lumo:               float32(props.LUMO),
		BandGap:            float32(props.BandGap),
		EmissionWavelength: float32(props.EmissionWavelength),
		QuantumYield:       float32(props.QuantumYield),
		Stability:          float32(props.Stability),
		Confidence:         float32(props.Confidence),
	}, nil
}

// domainToProto converts domain molecule to protobuf message
func domainToProto(mol *molecule.Molecule) *pb.Molecule {
	if mol == nil {
		return nil
	}

	return &pb.Molecule{
		MoleculeId:   mol.ID(),
		Smiles:       mol.SMILES(),
		Inchi:        mol.InChI(),
		Name:         mol.Name(),
		MoleculeType: mol.Type(),
		OledLayer:    mol.OLEDLayer(),
		Properties:   mol.Properties(),
		Metadata:     mol.Metadata(),
		CreatedAt:    mol.CreatedAt().Unix(),
		UpdatedAt:    mol.UpdatedAt().Unix(),
	}
}

// mapDomainError maps domain errors to gRPC status codes
func mapDomainError(err error) error {
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