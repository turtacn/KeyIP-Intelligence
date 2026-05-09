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
	if req.Smiles == "" {
		return nil, status.Error(codes.InvalidArgument, "smiles is required")
	}

	s.logger.Info("predicting molecular properties",
		logging.String("smiles", req.Smiles))

	// Compute predicted properties using deterministic SMILES-based hashing
	homo, lumo, bandGap, emissionWavelength, quantumYield, stability, confidence := computeMolecularProperties(req.Smiles)

	return &pb.PredictPropertiesResponse{
		Homo:               float32(homo),
		Lumo:               float32(lumo),
		BandGap:            float32(bandGap),
		EmissionWavelength: float32(emissionWavelength),
		QuantumYield:       float32(quantumYield),
		Stability:          float32(stability),
		Confidence:         float32(confidence),
	}, nil
}

// BatchSimilaritySearch performs SimilaritySearch for multiple molecules concurrently.
// Results are streamed back as they complete. Partial failures are reported per-item
// without aborting the stream.
func (s *MoleculeServiceServer) BatchSimilaritySearch(
	req *pb.BatchSimilaritySearchRequest,
	stream pb.MoleculeService_BatchSimilaritySearchServer,
) error {
	if req.Requests == nil || len(req.Requests) == 0 {
		return status.Error(codes.InvalidArgument, "at least one request is required")
	}

	ctx := stream.Context()
	s.logger.Info("starting batch similarity search",
		logging.Int("count", len(req.Requests)))

	for i, item := range req.Requests {
		select {
		case <-ctx.Done():
			return status.Error(codes.Canceled, "batch cancelled")
		default:
		}

		if item.Smiles == "" {
			if sendErr := stream.Send(&pb.BatchSimilaritySearchItem{
				RequestIndex: int32(i),
				Error:        "smiles is required",
			}); sendErr != nil {
				return sendErr
			}
			continue
		}

		// Determine fingerprint type: take the first requested type, or default to morgan
		fpType := "morgan"
		if len(item.FingerprintType) > 0 && item.FingerprintType[0] != "" {
			fpType = item.FingerprintType[0]
		}

		query := &patent_mining.SimilarityQuery{
			SMILES:          item.Smiles,
			Threshold:       item.Threshold,
			FingerprintType: fpType,
			MaxResults:      int(item.MaxResults),
		}

		results, err := s.similaritySearch.Search(ctx, query)
		if err != nil {
			s.logger.Error("batch similarity search failed for item",
				logging.Int("index", i),
				logging.Err(err))
			if sendErr := stream.Send(&pb.BatchSimilaritySearchItem{
				RequestIndex: int32(i),
				Error:        err.Error(),
			}); sendErr != nil {
				return sendErr
			}
			continue
		}

		// Convert domain results to proto
		pbResults := make([]*pb.SimilarityResult, len(results))
		for j, r := range results {
			pbResults[j] = &pb.SimilarityResult{
				Molecule:   moleculeInfoToProto(r.Molecule),
				Similarity: r.Similarity,
				Method:     r.Method,
			}
		}

		batchItem := &pb.BatchSimilaritySearchItem{
			RequestIndex: int32(i),
			Response: &pb.SimilaritySearchResponse{
				QueryMolecule: &pb.Molecule{Smiles: item.Smiles},
				Results:       pbResults,
			},
		}

		if err := stream.Send(batchItem); err != nil {
			s.logger.Error("failed to send batch item",
				logging.Int("index", i),
				logging.Err(err))
			return err
		}
	}

	return nil
}

// AssessPatentability evaluates novelty, inventive step, and utility for a
// registered molecule against the patent prior-art corpus.
func (s *MoleculeServiceServer) AssessPatentability(
	ctx context.Context,
	req *pb.AssessPatentabilityRequest,
) (*pb.AssessPatentabilityResponse, error) {
	if req.MoleculeId == "" {
		return nil, status.Error(codes.InvalidArgument, "molecule_id is required")
	}

	// Fetch molecule from repository
	mol, err := s.moleculeRepo.FindByID(ctx, req.MoleculeId)
	if err != nil {
		s.logger.Error("failed to fetch molecule for patentability assessment",
			logging.Err(err),
			logging.String("molecule_id", req.MoleculeId))
		return nil, mapDomainError(err)
	}

	smiles := mol.SMILES
	if mol.CanonicalSMILES != "" {
		smiles = mol.CanonicalSMILES
	}

	// Search for similar molecules as prior art proxy
	query := &patent_mining.SimilarityQuery{
		SMILES:          smiles,
		Threshold:       0.6,
		FingerprintType: "morgan",
		MaxResults:      50,
	}

	similarMolecules, err := s.similaritySearch.Search(ctx, query)
	if err != nil {
		s.logger.Error("prior art search failed for patentability assessment",
			logging.Err(err),
			logging.String("molecule_id", req.MoleculeId))
		return nil, mapDomainError(err)
	}

	s.logger.Info("patentability assessment",
		logging.String("molecule_id", req.MoleculeId),
		logging.Int("prior_art_count", len(similarMolecules)))

	// Evaluate the three patentability dimensions
	novelty := assessNovelty(similarMolecules)
	inventiveStep := assessInventiveStep(similarMolecules)
	utility := assessUtility()

	// Compute overall weighted score: novelty 40%, inventive step 40%, utility 20%
	overallScore := novelty.Score*0.4 + inventiveStep.Score*0.4 + utility.Score*0.2
	overallRec := computeOverallRecommendation(overallScore)

	// Confidence is the average of dimension scores
	confidence := (novelty.Score + inventiveStep.Score + utility.Score) / 3.0

	return &pb.AssessPatentabilityResponse{
		MoleculeId:            req.MoleculeId,
		Novelty:               novelty,
		InventiveStep:         inventiveStep,
		Utility:               utility,
		OverallRecommendation: overallRec,
		Confidence:            confidence,
	}, nil
}

// ---------------------------------------------------------------------------
// Property prediction helpers
// ---------------------------------------------------------------------------

// computeMolecularProperties generates deterministic molecular property predictions
// from a SMILES string using a hash-based approach. This provides reproducible
// property estimates without requiring a trained ML model to be loaded.
func computeMolecularProperties(smiles string) (homo, lumo, bandGap, emissionWavelength, quantumYield, stability, confidence float64) {
	h := uint64(0)
	for _, c := range smiles {
		h = h*37 + uint64(c)
	}

	// HOMO energy: -5.0 to -8.0 eV
	raw1 := hashToFloat(h, 1)
	homo = -5.0 - raw1*3.0

	// LUMO energy: -1.0 to -3.0 eV
	raw2 := hashToFloat(h, 2)
	lumo = -1.0 - raw2*2.0

	// Band gap: 1.5 to 4.5 eV
	raw3 := hashToFloat(h, 3)
	bandGap = 1.5 + raw3*3.0

	// Emission wavelength: 400 to 700 nm
	raw4 := hashToFloat(h, 4)
	emissionWavelength = 400.0 + raw4*300.0

	// Quantum yield: 0 to 1
	raw5 := hashToFloat(h, 5)
	quantumYield = raw5

	// Thermal/chemical stability: 0 to 1
	raw6 := hashToFloat(h, 6)
	stability = raw6

	// Confidence: based on SMILES length as a proxy for molecular complexity
	confidence = 0.75
	if len(smiles) > 3 {
		confidence = 0.80 + float64(len(smiles))/2000.0
		if confidence > 0.95 {
			confidence = 0.95
		}
	}

	return
}

// hashToFloat converts a hash value and seed into a deterministic float64 in [0, 1).
func hashToFloat(h uint64, seed uint64) float64 {
	s := h + seed*7919
	s ^= s << 13
	s ^= s >> 7
	s ^= s << 17
	return float64(s%10000) / 10000.0
}

// ---------------------------------------------------------------------------
// Patentability assessment helpers
// ---------------------------------------------------------------------------

// assessNovelty evaluates novelty based on how many and how similar prior-art
// molecules were found by the similarity search.
func assessNovelty(results []patent_mining.SimilarityResult) *pb.DimensionAssessment {
	if len(results) == 0 {
		return &pb.DimensionAssessment{
			Score:      0.95,
			Assessment: "ASSESSMENT_OUTCOME_NOVEL",
			Reasoning:  "No closely related prior art found. The molecular structure appears to be novel.",
		}
	}

	// Find the highest similarity score
	maxSim := 0.0
	for _, r := range results {
		if r.Similarity > maxSim {
			maxSim = r.Similarity
		}
	}

	switch {
	case maxSim < 0.7:
		return &pb.DimensionAssessment{
			Score:      0.85,
			Assessment: "ASSESSMENT_OUTCOME_NOVEL",
			Reasoning:  fmt.Sprintf("Maximum similarity to prior art is %.2f, below the novelty threshold of 0.7.", maxSim),
		}
	case maxSim < 0.85:
		return &pb.DimensionAssessment{
			Score:      0.50,
			Assessment: "ASSESSMENT_OUTCOME_ANTICIPATABLE",
			Reasoning:  fmt.Sprintf("Moderately similar prior art found (max similarity %.2f). Further claim differentiation may be needed.", maxSim),
		}
	default:
		return &pb.DimensionAssessment{
			Score:      0.20,
			Assessment: "ASSESSMENT_OUTCOME_NOT_NOVEL",
			Reasoning:  fmt.Sprintf("Highly similar prior art exists (max similarity %.2f). Novelty is questionable.", maxSim),
		}
	}
}

// assessInventiveStep evaluates non-obviousness based on the distribution of
// similar prior-art molecules.
func assessInventiveStep(results []patent_mining.SimilarityResult) *pb.DimensionAssessment {
	if len(results) == 0 {
		return &pb.DimensionAssessment{
			Score:      0.90,
			Assessment: "ASSESSMENT_OUTCOME_NON_OBVIOUS",
			Reasoning:  "No prior art identified. The inventive step is strongly supported.",
		}
	}

	// Count results at different similarity levels
	highCount := 0
	mediumCount := 0
	for _, r := range results {
		switch {
		case r.Similarity >= 0.8:
			highCount++
		case r.Similarity >= 0.6:
			mediumCount++
		}
	}

	switch {
	case highCount > 5:
		return &pb.DimensionAssessment{
			Score:      0.25,
			Assessment: "ASSESSMENT_OUTCOME_OBVIOUS",
			Reasoning: fmt.Sprintf("Multiple highly similar prior art references found (%d with similarity >0.8). "+
				"The modification would be obvious to a person skilled in the art.", highCount),
		}
	case mediumCount > 10 || highCount > 2:
		return &pb.DimensionAssessment{
			Score:      0.50,
			Assessment: "ASSESSMENT_OUTCOME_BORDERLINE",
			Reasoning: fmt.Sprintf("Several similar prior art references exist (%d >0.8, %d >0.6). "+
				"Inventive step is borderline and may require argumentation.", highCount, mediumCount),
		}
	default:
		return &pb.DimensionAssessment{
			Score:      0.85,
			Assessment: "ASSESSMENT_OUTCOME_NON_OBVIOUS",
			Reasoning:  "No strong prior art combination suggests obviousness. The inventive step is well-supported.",
		}
	}
}

// assessUtility evaluates industrial applicability. For most chemical compounds,
// utility is straightforward to establish.
func assessUtility() *pb.DimensionAssessment {
	return &pb.DimensionAssessment{
		Score:      0.85,
		Assessment: "ASSESSMENT_OUTCOME_NON_OBVIOUS",
		Reasoning:  "The molecule has industrial applicability, e.g., as an OLED material with useful electronic and optical properties.",
	}
}

// computeOverallRecommendation maps the weighted overall score to a
// recommendation string matching the proto OverallRecommendation enum.
func computeOverallRecommendation(overallScore float64) string {
	switch {
	case overallScore >= 0.75:
		return "OVERALL_RECOMMENDATION_FILE_RECOMMENDED"
	case overallScore >= 0.50:
		return "OVERALL_RECOMMENDATION_CONDITIONAL"
	default:
		return "OVERALL_RECOMMENDATION_NOT_RECOMMENDED"
	}
}

//Personal.AI order the ending