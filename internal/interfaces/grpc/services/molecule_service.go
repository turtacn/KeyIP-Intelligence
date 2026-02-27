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
	if req.Smiles == "" {
		return nil, status.Error(codes.InvalidArgument, "smiles is required")
	}

	// Fetch molecules by SMILES
	molecules, err := s.moleculeRepo.FindBySMILES(ctx, req.Smiles)
	if err != nil && !errors.IsNotFound(err) {
		return nil, mapDomainError(err)
	}

	var mol *molecule.Molecule
	if len(molecules) > 0 {
		mol = molecules[0]
	} else {
		// Create temporary molecule for prediction
		mol, err = molecule.NewMolecule(req.Smiles, molecule.SourcePrediction, "")
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	// Placeholder: use mol in future AI prediction
	_ = mol

	// Return placeholder predicted properties (actual prediction would use AI models)
	// This is a stub implementation - real prediction would come from intelligence layer
	return &pb.PredictPropertiesResponse{
		Homo:               -5.5,  // eV - typical value
		Lumo:               -2.0,  // eV - typical value
		BandGap:            3.5,   // eV - typical value for OLED materials
		EmissionWavelength: 450.0, // nm - blue emission
		QuantumYield:       0.8,   // typical for good emitter
		Stability:          0.9,   // high stability
		Confidence:         0.5,   // moderate confidence for placeholder
	}, nil
}

// domainToProto converts domain molecule to protobuf message
func domainToProto(mol *molecule.Molecule) *pb.Molecule {
	if mol == nil {
		return nil
	}

	// Convert properties to map[string]string
	propsMap := make(map[string]string)
	for _, prop := range mol.Properties {
		propsMap[prop.Name] = fmt.Sprintf("%v", prop.Value)
	}

	// Convert metadata to map[string]string
	metaMap := make(map[string]string)
	if mol.Metadata != nil {
		for k, v := range mol.Metadata {
			metaMap[k] = fmt.Sprintf("%v", v)
		}
	}

	return &pb.Molecule{
		MoleculeId:   mol.ID.String(),
		Smiles:       mol.SMILES,
		Inchi:        mol.InChI,
		Name:         mol.Name,
		MoleculeType: string(mol.Source),
		OledLayer:    "",
		Properties:   propsMap,
		Metadata:     metaMap,
		CreatedAt:    mol.CreatedAt.Unix(),
		UpdatedAt:    mol.UpdatedAt.Unix(),
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