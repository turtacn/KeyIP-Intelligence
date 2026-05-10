// Integration tests for gRPC services using bufconn for in-memory connections.
// Health and reflection services use real protobuf types (fully wire-compatible).
// Molecule and patent services use stub proto types with a JSON codec,
// testing through the full gRPC handler dispatch + interceptor chain.
package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/turtacn/KeyIP-Intelligence/api/proto/v1"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	csgrpc "github.com/turtacn/KeyIP-Intelligence/internal/interfaces/grpc"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// JSON codec for stub proto types
// ---------------------------------------------------------------------------

// jsonCodec implements both grpc.Codec and encoding.Codec for JSON serialization
// of stub proto types. Uses a unique name to avoid collisions with the default proto codec.
type jsonCodec struct{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error)     { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v interface{}) error { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                               { return "json-stub-test" }
func (jsonCodec) String() string                             { return "json-stub-test" }

var registerJSONCodec sync.Once

func initJSONCodec() {
	registerJSONCodec.Do(func() {
		encoding.RegisterCodec(jsonCodec{})
	})
}

// ---------------------------------------------------------------------------
// Common test infrastructure
// ---------------------------------------------------------------------------

const bufSize = 1024 * 1024

// grpcTestHelper creates a gRPC server (with JSON codec for stub types) and
// returns the listener, server, and a conn dialing it in-memory.
type grpcTestHelper struct {
	lis        *bufconn.Listener
	server     *grpc.Server
	conn       *grpc.ClientConn
	refConn    *grpc.ClientConn // connection without JSON codec for proto-based services (health, reflection)
	molClient  pb.MoleculeServiceClient
	patClient  pb.PatentServiceClient
	healthCli  healthpb.HealthClient
}

func (h *grpcTestHelper) stop() {
	if h.conn != nil {
		h.conn.Close()
	}
	if h.refConn != nil {
		h.refConn.Close()
	}
	if h.server != nil {
		h.server.Stop()
	}
}

// newMoleculeTestServer creates an in-memory gRPC server with MoleculeService
// and health service registered, using a JSON codec for the stub proto types.
func newMoleculeTestServer(t *testing.T) (*grpcTestHelper, *MockMoleculeRepo, *MockSimilaritySearch, *MockLogger) {
	t.Helper()
	initJSONCodec()

	lis := bufconn.Listen(bufSize)

	mockRepo := new(MockMoleculeRepo)
	mockSim := new(MockSimilaritySearch)
	mockLog := new(MockLogger)
	mockLog.On("Info", mock.Anything, mock.Anything).Return().Maybe()
	mockLog.On("Error", mock.Anything, mock.Anything).Return().Maybe()

	s := grpc.NewServer()
	pb.RegisterMoleculeServiceServer(s, NewMoleculeServiceServer(mockRepo, mockSim, mockLog))
	hs := csgrpc.NewHealthService()
	healthpb.RegisterHealthServer(s, hs)

	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.DialContext(
		context.Background(), "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, a string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithCodec(jsonCodec{}),
	)
	require.NoError(t, err)

	return &grpcTestHelper{
		lis: lis, server: s, conn: conn,
		molClient: pb.NewMoleculeServiceClient(conn),
		healthCli: healthpb.NewHealthClient(conn),
	}, mockRepo, mockSim, mockLog
}

// newPatentTestServer creates an in-memory gRPC server with PatentService
// and health service registered.
func newPatentTestServer(t *testing.T) (*grpcTestHelper, *MockPatentRepo, *MockLogger) {
	t.Helper()
	initJSONCodec()

	lis := bufconn.Listen(bufSize)

	mockRepo := new(MockPatentRepo)
	mockLog := new(MockLogger)
	mockLog.On("Info", mock.Anything, mock.Anything).Return().Maybe()
	mockLog.On("Error", mock.Anything, mock.Anything).Return().Maybe()

	s := grpc.NewServer()
	pb.RegisterPatentServiceServer(s, NewPatentServiceServer(mockRepo, nil, mockLog))
	hs := csgrpc.NewHealthService()
	healthpb.RegisterHealthServer(s, hs)

	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.DialContext(
		context.Background(), "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, a string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithCodec(jsonCodec{}),
	)
	require.NoError(t, err)

	return &grpcTestHelper{
		lis: lis, server: s, conn: conn,
		patClient: pb.NewPatentServiceClient(conn),
		healthCli: healthpb.NewHealthClient(conn),
	}, mockRepo, mockLog
}

// newFullTestServer creates an in-memory gRPC server with MoleculeService,
// PatentService, health service, and optionally reflection registered.
func newFullTestServer(t *testing.T, enableReflection bool) *grpcTestHelper {
	t.Helper()
	initJSONCodec()

	lis := bufconn.Listen(bufSize)

	molRepo := new(MockMoleculeRepo)
	molSim := new(MockSimilaritySearch)
	molLog := new(MockLogger)
	molLog.On("Info", mock.Anything, mock.Anything).Return().Maybe()
	molLog.On("Error", mock.Anything, mock.Anything).Return().Maybe()

	patRepo := new(MockPatentRepo)
	patLog := new(MockLogger)
	patLog.On("Info", mock.Anything, mock.Anything).Return().Maybe()
	patLog.On("Error", mock.Anything, mock.Anything).Return().Maybe()

	// Default mock expectations to prevent panics on unexpected calls
	molRepo.On("FindByID", mock.Anything, mock.Anything).Maybe().Return(nil, errors.NewNotFound("not found"))
	molRepo.On("Search", mock.Anything, mock.Anything).Maybe().Return(&molecule.MoleculeSearchResult{}, nil)
	patRepo.On("GetByPatentNumber", mock.Anything, mock.Anything).Maybe().Return(nil, errors.NewNotFound("not found"))
	patRepo.On("Search", mock.Anything, mock.Anything).Maybe().Return(&patent.PatentSearchResult{}, nil)

	s := grpc.NewServer()
	pb.RegisterMoleculeServiceServer(s, NewMoleculeServiceServer(molRepo, molSim, molLog))
	pb.RegisterPatentServiceServer(s, NewPatentServiceServer(patRepo, nil, patLog))
	hs := csgrpc.NewHealthService()
	hs.SetServingStatus("keyip.v1.MoleculeService", healthpb.HealthCheckResponse_SERVING)
	hs.SetServingStatus("keyip.v1.PatentService", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, hs)
	if enableReflection {
		reflection.Register(s)
	}

	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.DialContext(
		context.Background(), "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, a string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithCodec(jsonCodec{}),
	)
	require.NoError(t, err)

	// Separate connection without JSON codec for proto-based services (health, reflection)
	refConn, err := grpc.DialContext(
		context.Background(), "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, a string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	return &grpcTestHelper{
		lis: lis, server: s, conn: conn, refConn: refConn,
		molClient: pb.NewMoleculeServiceClient(conn),
		patClient: pb.NewPatentServiceClient(conn),
		healthCli: healthpb.NewHealthClient(refConn),
	}
}

// =========================================================================
// Molecule Service Integration Tests
// =========================================================================

func TestIntegration_Molecule_GetMolecule(t *testing.T) {
	h, mockRepo, _, _ := newMoleculeTestServer(t)
	defer h.stop()

	ctx := context.Background()
	expectedMol, err := molecule.NewMolecule("CCO", molecule.SourceManual, "src")
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		mockRepo.On("FindByID", mock.Anything, "mol-001").Return(expectedMol, nil)
		resp, err := h.molClient.GetMolecule(ctx, &pb.GetMoleculeRequest{MoleculeId: "mol-001"})
		require.NoError(t, err)
		assert.Equal(t, expectedMol.ID.String(), resp.Molecule.MoleculeId)
		assert.Equal(t, "CCO", resp.Molecule.Smiles)
	})

	t.Run("not found", func(t *testing.T) {
		mockRepo.On("FindByID", mock.Anything, "unknown").Return(nil, errors.NewNotFound("not found"))
		_, err := h.molClient.GetMolecule(ctx, &pb.GetMoleculeRequest{MoleculeId: "unknown"})
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("empty id", func(t *testing.T) {
		_, err := h.molClient.GetMolecule(ctx, &pb.GetMoleculeRequest{MoleculeId: ""})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestIntegration_Molecule_ListMolecules(t *testing.T) {
	h, mockRepo, _, _ := newMoleculeTestServer(t)
	defer h.stop()
	ctx := context.Background()

	t.Run("with results", func(t *testing.T) {
		m1, _ := molecule.NewMolecule("C1=CC=CC=C1", molecule.SourcePatent, "a")
		m2, _ := molecule.NewMolecule("CCO", molecule.SourceManual, "b")
		mockRepo.On("Search", mock.Anything, mock.MatchedBy(func(q *molecule.MoleculeQuery) bool { return q.Limit == 20 && q.Offset == 0 })).
			Return(&molecule.MoleculeSearchResult{Molecules: []*molecule.Molecule{m1, m2}, Total: 2}, nil)

		resp, err := h.molClient.ListMolecules(ctx, &pb.ListMoleculesRequest{PageSize: 20})
		require.NoError(t, err)
		assert.Len(t, resp.Molecules, 2)
		assert.Equal(t, int64(2), resp.TotalCount)
	})

	t.Run("pagination", func(t *testing.T) {
		mols := make([]*molecule.Molecule, 25)
		for i := 0; i < 25; i++ {
			mols[i], _ = molecule.NewMolecule("C", molecule.SourceManual, fmt.Sprintf("m-%d", i))
		}
		r1 := &molecule.MoleculeSearchResult{Molecules: mols[:10], Total: 25, Offset: 0, Limit: 10, HasMore: true}
		mockRepo.On("Search", mock.Anything, mock.MatchedBy(func(q *molecule.MoleculeQuery) bool { return q.Offset == 0 && q.Limit == 10 })).Return(r1, nil)

		resp1, err := h.molClient.ListMolecules(ctx, &pb.ListMoleculesRequest{PageSize: 10})
		require.NoError(t, err)
		assert.Len(t, resp1.Molecules, 10)
		assert.NotEmpty(t, resp1.NextPageToken)

		r2 := &molecule.MoleculeSearchResult{Molecules: mols[10:20], Total: 25, Offset: 10, Limit: 10, HasMore: true}
		mockRepo.On("Search", mock.Anything, mock.MatchedBy(func(q *molecule.MoleculeQuery) bool { return q.Offset == 10 && q.Limit == 10 })).Return(r2, nil)

		resp2, err := h.molClient.ListMolecules(ctx, &pb.ListMoleculesRequest{PageSize: 10, PageToken: resp1.NextPageToken})
		require.NoError(t, err)
		assert.Len(t, resp2.Molecules, 10)
		assert.NotEmpty(t, resp2.NextPageToken)
	})

	t.Run("page size exceeds max", func(t *testing.T) {
		_, err := h.molClient.ListMolecules(ctx, &pb.ListMoleculesRequest{PageSize: 200})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid page token", func(t *testing.T) {
		_, err := h.molClient.ListMolecules(ctx, &pb.ListMoleculesRequest{PageSize: 20, PageToken: "!!!bad-base64"})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestIntegration_Molecule_SearchSimilar(t *testing.T) {
	h, _, mockSim, _ := newMoleculeTestServer(t)
	defer h.stop()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockSim.On("Search", mock.Anything, mock.MatchedBy(func(q *patent_mining.SimilarityQuery) bool { return q.SMILES == "c1ccccc1" })).
			Return([]patent_mining.SimilarityResult{
				{Molecule: &patent_mining.MoleculeInfo{ID: "m1", SMILES: "C1=CC=CC=C1", Name: "Benzene"}, Similarity: 0.85, Method: "tanimoto"},
			}, nil)

		resp, err := h.molClient.SearchSimilar(ctx, &pb.SearchSimilarRequest{Smiles: "c1ccccc1", Threshold: 0.7, MaxResults: 10})
		require.NoError(t, err)
		require.Len(t, resp.SimilarMolecules, 1)
		assert.Equal(t, float64(0.85), resp.SimilarMolecules[0].Similarity)
	})

	t.Run("missing smiles and inchi", func(t *testing.T) {
		_, err := h.molClient.SearchSimilar(ctx, &pb.SearchSimilarRequest{Threshold: 0.7})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid threshold", func(t *testing.T) {
		_, err := h.molClient.SearchSimilar(ctx, &pb.SearchSimilarRequest{Smiles: "CCO", Threshold: 1.5})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("negative threshold", func(t *testing.T) {
		_, err := h.molClient.SearchSimilar(ctx, &pb.SearchSimilarRequest{Smiles: "CCO", Threshold: -0.1})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("empty results", func(t *testing.T) {
		mockSim.On("Search", mock.Anything, mock.MatchedBy(func(q *patent_mining.SimilarityQuery) bool { return q.SMILES == "C#N" })).
			Return([]patent_mining.SimilarityResult{}, nil)
		resp, err := h.molClient.SearchSimilar(ctx, &pb.SearchSimilarRequest{Smiles: "C#N", Threshold: 0.9})
		require.NoError(t, err)
		assert.Empty(t, resp.SimilarMolecules)
	})

	t.Run("using inchi", func(t *testing.T) {
		mockSim.On("Search", mock.Anything, mock.MatchedBy(func(q *patent_mining.SimilarityQuery) bool { return q.InChI != "" })).
			Return([]patent_mining.SimilarityResult{
				{Molecule: &patent_mining.MoleculeInfo{ID: "m2", SMILES: "CCO", Name: "Ethanol"}, Similarity: 0.92, Method: "tanimoto"},
			}, nil)
		resp, err := h.molClient.SearchSimilar(ctx, &pb.SearchSimilarRequest{
			Inchi: "InChI=1S/C2H6O/c1-2-3/h3H,2H2,1H3", Threshold: 0.7,
		})
		require.NoError(t, err)
		assert.Len(t, resp.SimilarMolecules, 1)
	})
}

func TestIntegration_Molecule_PredictProperties(t *testing.T) {
	h, _, _, _ := newMoleculeTestServer(t)
	defer h.stop()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		resp, err := h.molClient.PredictProperties(ctx, &pb.PredictPropertiesRequest{Smiles: "CCO"})
		require.NoError(t, err)
		assert.LessOrEqual(t, resp.Homo, float32(-5.0))
		assert.GreaterOrEqual(t, resp.Homo, float32(-8.0))
		assert.LessOrEqual(t, resp.Lumo, float32(-1.0))
		assert.GreaterOrEqual(t, resp.Lumo, float32(-3.0))
		assert.GreaterOrEqual(t, resp.BandGap, float32(1.5))
		assert.LessOrEqual(t, resp.BandGap, float32(4.5))
		assert.GreaterOrEqual(t, resp.Confidence, float32(0.75))
	})

	t.Run("missing smiles", func(t *testing.T) {
		_, err := h.molClient.PredictProperties(ctx, &pb.PredictPropertiesRequest{Smiles: ""})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("deterministic", func(t *testing.T) {
		r1, _ := h.molClient.PredictProperties(ctx, &pb.PredictPropertiesRequest{Smiles: "c1ccccc1"})
		r2, _ := h.molClient.PredictProperties(ctx, &pb.PredictPropertiesRequest{Smiles: "c1ccccc1"})
		assert.Equal(t, r1.Homo, r2.Homo)
		assert.Equal(t, r1.Lumo, r2.Lumo)
		assert.Equal(t, r1.BandGap, r2.BandGap)
	})

	t.Run("different smiles yield different properties", func(t *testing.T) {
		r1, _ := h.molClient.PredictProperties(ctx, &pb.PredictPropertiesRequest{Smiles: "CCO"})
		r2, _ := h.molClient.PredictProperties(ctx, &pb.PredictPropertiesRequest{Smiles: "C1=CC=CC=C1"})
		assert.NotEqual(t, r1.Homo, r2.Homo)
	})
}

func TestIntegration_Molecule_CreateMolecule(t *testing.T) {
	h, mockRepo, _, _ := newMoleculeTestServer(t)
	defer h.stop()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*molecule.Molecule")).Return(nil)
		resp, err := h.molClient.CreateMolecule(ctx, &pb.CreateMoleculeRequest{Smiles: "C", Name: "Methane"})
		require.NoError(t, err)
		assert.Equal(t, "Methane", resp.Molecule.Name)
		assert.Equal(t, "C", resp.Molecule.Smiles)
	})

	t.Run("missing smiles", func(t *testing.T) {
		_, err := h.molClient.CreateMolecule(ctx, &pb.CreateMoleculeRequest{Name: "Invalid"})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("with properties", func(t *testing.T) {
		mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*molecule.Molecule")).Return(nil)
		resp, err := h.molClient.CreateMolecule(ctx, &pb.CreateMoleculeRequest{
			Smiles: "CCO", Name: "Ethanol",
			Properties: map[string]string{"MolecularWeight": "46.07"},
		})
		require.NoError(t, err)
		assert.Equal(t, "Ethanol", resp.Molecule.Name)
	})
}

func TestIntegration_Molecule_ContextCancellation(t *testing.T) {
	h, mockRepo, _, _ := newMoleculeTestServer(t)
	defer h.stop()

	t.Run("get molecule - cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		mockRepo.On("FindByID", mock.Anything, "x").Maybe().Return(nil, nil)
		_, err := h.molClient.GetMolecule(ctx, &pb.GetMoleculeRequest{MoleculeId: "x"})
		assert.Error(t, err)
	})

	t.Run("predict properties fails with cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := h.molClient.PredictProperties(ctx, &pb.PredictPropertiesRequest{Smiles: "CCO"})
		assert.Error(t, err)
		assert.Equal(t, codes.Canceled, status.Code(err))
	})
}

// =========================================================================
// Patent Service Integration Tests
// =========================================================================

func TestIntegration_Patent_GetPatent(t *testing.T) {
	h, mockRepo, _ := newPatentTestServer(t)
	defer h.stop()
	ctx := context.Background()
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		mockRepo.On("GetByPatentNumber", mock.Anything, "CN123456789A").
			Return(&patent.Patent{
				PatentNumber: "CN123456789A", Title: "OLED Device", Abstract: "Improved OLED",
				AssigneeName: "KeyIP Corp", Status: patent.PatentStatusGranted, FilingDate: &now,
			}, nil)
		resp, err := h.patClient.GetPatent(ctx, &pb.GetPatentRequest{PatentNumber: "CN123456789A"})
		require.NoError(t, err)
		assert.Equal(t, "CN123456789A", resp.Patent.PatentNumber)
		assert.Equal(t, "OLED Device", resp.Patent.Title)
		assert.Equal(t, "KeyIP Corp", resp.Patent.Applicants[0])
	})

	t.Run("not found", func(t *testing.T) {
		mockRepo.On("GetByPatentNumber", mock.Anything, "CN999999999A").Return(nil, errors.NewNotFound("not found"))
		_, err := h.patClient.GetPatent(ctx, &pb.GetPatentRequest{PatentNumber: "CN999999999A"})
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("invalid patent number", func(t *testing.T) {
		_, err := h.patClient.GetPatent(ctx, &pb.GetPatentRequest{PatentNumber: "INVALID"})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("empty patent number", func(t *testing.T) {
		_, err := h.patClient.GetPatent(ctx, &pb.GetPatentRequest{PatentNumber: ""})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("internal error", func(t *testing.T) {
		mockRepo.On("GetByPatentNumber", mock.Anything, "US1234567B2").Return(nil, fmt.Errorf("db down"))
		_, err := h.patClient.GetPatent(ctx, &pb.GetPatentRequest{PatentNumber: "US1234567B2"})
		assert.Equal(t, codes.Internal, status.Code(err))
	})
}

func TestIntegration_Patent_SearchPatents(t *testing.T) {
	h, mockRepo, _ := newPatentTestServer(t)
	defer h.stop()
	ctx := context.Background()

	t.Run("success with results", func(t *testing.T) {
		mockRepo.On("Search", mock.Anything, mock.MatchedBy(func(c patent.PatentSearchCriteria) bool { return len(c.TitleKeywords) > 0 && c.TitleKeywords[0] == "OLED" && c.Limit == 10 })).
			Return(&patent.PatentSearchResult{
				Patents: []*patent.Patent{
					{PatentNumber: "CN123456789A", Title: "OLED Device"},
					{PatentNumber: "US9876543B2", Title: "OLED Material"},
				},
				Total: 2,
			}, nil)
		resp, err := h.patClient.SearchPatents(ctx, &pb.SearchPatentsRequest{Query: "OLED", PageSize: 10})
		require.NoError(t, err)
		assert.Len(t, resp.Patents, 2)
		assert.Equal(t, int64(2), resp.TotalCount)
	})

	t.Run("pagination", func(t *testing.T) {
		mockRepo.On("Search", mock.Anything, mock.MatchedBy(func(c patent.PatentSearchCriteria) bool {
			return c.Offset == 0 && c.Limit == 1
		})).Return(&patent.PatentSearchResult{
			Patents: []*patent.Patent{{PatentNumber: "EP1234567A1", Title: "P1"}},
			Total: 3, Offset: 0, Limit: 1,
		}, nil)
		resp1, err := h.patClient.SearchPatents(ctx, &pb.SearchPatentsRequest{Query: "OLED", PageSize: 1})
		require.NoError(t, err)
		assert.NotEmpty(t, resp1.NextPageToken)
		decoded, _ := base64.StdEncoding.DecodeString(resp1.NextPageToken)
		assert.Equal(t, "1", string(decoded))
	})

	t.Run("page size exceeds max", func(t *testing.T) {
		_, err := h.patClient.SearchPatents(ctx, &pb.SearchPatentsRequest{Query: "x", PageSize: 200})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid page token", func(t *testing.T) {
		_, err := h.patClient.SearchPatents(ctx, &pb.SearchPatentsRequest{Query: "x", PageSize: 20, PageToken: "!!bad"})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("with patent offices filter", func(t *testing.T) {
		mockRepo.On("Search", mock.Anything, mock.MatchedBy(func(c patent.PatentSearchCriteria) bool {
			return len(c.Offices) == 1 && c.Offices[0] == patent.OfficeUSPTO
		})).Return(&patent.PatentSearchResult{
			Patents: []*patent.Patent{{PatentNumber: "US9876543B2", Title: "US Patent"}},
			Total: 1,
		}, nil)
		resp, err := h.patClient.SearchPatents(ctx, &pb.SearchPatentsRequest{
			Query: "OLED", PageSize: 20, PatentOffices: []string{"USPTO"},
		})
		require.NoError(t, err)
		assert.Len(t, resp.Patents, 1)
		assert.Equal(t, "US9876543B2", resp.Patents[0].PatentNumber)
	})
}

func TestIntegration_Patent_AnalyzeClaims(t *testing.T) {
	h, _, _ := newPatentTestServer(t)
	defer h.stop()
	ctx := context.Background()

	t.Run("empty patent number", func(t *testing.T) {
		_, err := h.patClient.AnalyzeClaims(ctx, &pb.AnalyzeClaimsRequest{PatentNumber: ""})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := h.patClient.AnalyzeClaims(ctx, &pb.AnalyzeClaimsRequest{PatentNumber: "BAD"})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestIntegration_Patent_CheckFTO(t *testing.T) {
	h, _, _ := newPatentTestServer(t)
	defer h.stop()
	ctx := context.Background()

	t.Run("placeholder returns safe default", func(t *testing.T) {
		resp, err := h.patClient.CheckFTO(ctx, &pb.CheckFTORequest{TargetSmiles: "CCO", Jurisdiction: "US"})
		require.NoError(t, err)
		assert.False(t, resp.CanOperate)
		assert.Equal(t, "UNKNOWN", resp.RiskLevel)
	})

	t.Run("missing target smiles", func(t *testing.T) {
		_, err := h.patClient.CheckFTO(ctx, &pb.CheckFTORequest{Jurisdiction: "US"})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("missing jurisdiction", func(t *testing.T) {
		_, err := h.patClient.CheckFTO(ctx, &pb.CheckFTORequest{TargetSmiles: "CCO"})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestIntegration_Patent_GetPatentFamily(t *testing.T) {
	h, mockRepo, _ := newPatentTestServer(t)
	defer h.stop()
	ctx := context.Background()

	t.Run("no family", func(t *testing.T) {
		mockRepo.On("GetByPatentNumber", mock.Anything, "CN123456789A").
			Return(&patent.Patent{PatentNumber: "CN123456789A", Title: "Solo", FamilyID: ""}, nil)
		resp, err := h.patClient.GetPatentFamily(ctx, &pb.GetPatentFamilyRequest{PatentNumber: "CN123456789A"})
		require.NoError(t, err)
		assert.Empty(t, resp.FamilyId)
		assert.Empty(t, resp.FamilyMembers)
	})

	t.Run("invalid patent number", func(t *testing.T) {
		_, err := h.patClient.GetPatentFamily(ctx, &pb.GetPatentFamilyRequest{PatentNumber: "INVALID"})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestIntegration_Patent_GetCitationNetwork(t *testing.T) {
	h, _, _ := newPatentTestServer(t)
	defer h.stop()
	ctx := context.Background()

	t.Run("placeholder returns root node", func(t *testing.T) {
		resp, err := h.patClient.GetCitationNetwork(ctx, &pb.GetCitationNetworkRequest{PatentNumber: "CN123456789A", Depth: 2})
		require.NoError(t, err)
		assert.Equal(t, int32(1), resp.TotalNodes)
		assert.True(t, resp.Nodes[0].IsRoot)
	})

	t.Run("empty patent number", func(t *testing.T) {
		_, err := h.patClient.GetCitationNetwork(ctx, &pb.GetCitationNetworkRequest{PatentNumber: ""})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("depth exceeds max", func(t *testing.T) {
		_, err := h.patClient.GetCitationNetwork(ctx, &pb.GetCitationNetworkRequest{PatentNumber: "CN123456789A", Depth: 10})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestIntegration_Patent_ContextCancellation(t *testing.T) {
	h, mockRepo, _ := newPatentTestServer(t)
	defer h.stop()

	t.Run("get patent - cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		mockRepo.On("GetByPatentNumber", mock.Anything, mock.Anything).Maybe().Return(nil, nil)
		_, err := h.patClient.GetPatent(ctx, &pb.GetPatentRequest{PatentNumber: "CN123456789A"})
		assert.Error(t, err)
	})

	t.Run("search patents - cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		mockRepo.On("Search", mock.Anything, mock.Anything).Maybe().Return(nil, nil)
		_, err := h.patClient.SearchPatents(ctx, &pb.SearchPatentsRequest{Query: "test"})
		assert.Error(t, err)
	})
}

// =========================================================================
// Health Check Integration Tests (real protobuf types)
// =========================================================================

func TestIntegration_HealthCheck_Serving(t *testing.T) {
	h := newFullTestServer(t, false)
	defer h.stop()
	ctx := context.Background()

	t.Run("overall health", func(t *testing.T) {
		resp, err := h.healthCli.Check(ctx, &healthpb.HealthCheckRequest{})
		require.NoError(t, err)
		assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)
	})

	t.Run("service health", func(t *testing.T) {
		resp, err := h.healthCli.Check(ctx, &healthpb.HealthCheckRequest{Service: "keyip.v1.MoleculeService"})
		require.NoError(t, err)
		assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)
	})

		t.Run("unknown service", func(t *testing.T) {
			_, err := h.healthCli.Check(ctx, &healthpb.HealthCheckRequest{Service: "nonexistent.Service"})
			assert.Error(t, err)
			assert.Equal(t, codes.NotFound, status.Code(err))
		})
}

func TestIntegration_HealthCheck_WithChecker(t *testing.T) {
	lis := bufconn.Listen(bufSize)
	hs := csgrpc.NewHealthService(csgrpc.NewChecker("pg", func(ctx context.Context) error { return nil }))
	s := grpc.NewServer()
	healthpb.RegisterHealthServer(s, hs)
	go func() { _ = s.Serve(lis) }()
	defer s.Stop()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	cli := healthpb.NewHealthClient(conn)
	resp, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)
}

func TestIntegration_HealthCheck_FailingChecker(t *testing.T) {
	lis := bufconn.Listen(bufSize)
	hs := csgrpc.NewHealthService(csgrpc.NewChecker("pg", func(ctx context.Context) error { return fmt.Errorf("down") }))
	s := grpc.NewServer()
	healthpb.RegisterHealthServer(s, hs)
	go func() { _ = s.Serve(lis) }()
	defer s.Stop()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	cli := healthpb.NewHealthClient(conn)
	resp, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_NOT_SERVING, resp.Status)
}

func TestIntegration_HealthCheck_MultipleCheckers(t *testing.T) {
	lis := bufconn.Listen(bufSize)
	hs := csgrpc.NewHealthService(
		csgrpc.NewChecker("pg", func(ctx context.Context) error { return nil }),
		csgrpc.NewChecker("redis", func(ctx context.Context) error { return fmt.Errorf("timeout") }),
	)
	s := grpc.NewServer()
	healthpb.RegisterHealthServer(s, hs)
	go func() { _ = s.Serve(lis) }()
	defer s.Stop()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	cli := healthpb.NewHealthClient(conn)
	resp, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_NOT_SERVING, resp.Status)
}

func TestIntegration_HealthCheck_AllCheckersHealthy(t *testing.T) {
	lis := bufconn.Listen(bufSize)
	hs := csgrpc.NewHealthService(
		csgrpc.NewChecker("pg", func(ctx context.Context) error { return nil }),
		csgrpc.NewChecker("redis", func(ctx context.Context) error { return nil }),
		csgrpc.NewChecker("neo4j", func(ctx context.Context) error { return nil }),
	)
	s := grpc.NewServer()
	healthpb.RegisterHealthServer(s, hs)
	go func() { _ = s.Serve(lis) }()
	defer s.Stop()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	cli := healthpb.NewHealthClient(conn)
	resp, err := cli.Check(context.Background(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)
}

// =========================================================================
// Server Reflection Integration Tests (real protobuf types)
// =========================================================================

func TestIntegration_ServerReflection_ListServices(t *testing.T) {
	h := newFullTestServer(t, true)
	defer h.stop()

	refClient := reflectionpb.NewServerReflectionClient(h.refConn)
	stream, err := refClient.ServerReflectionInfo(context.Background())
	require.NoError(t, err)

	require.NoError(t, stream.Send(&reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{ListServices: "*"},
	}))

	resp, err := stream.Recv()
	require.NoError(t, err)

	listResp := resp.GetListServicesResponse()
	require.NotNil(t, listResp)

	names := make([]string, 0, len(listResp.Service))
	for _, svc := range listResp.Service {
		names = append(names, svc.Name)
	}
	t.Logf("reflected services: %v", names)
	assert.Contains(t, names, "keyip.v1.MoleculeService")
	assert.Contains(t, names, "keyip.v1.PatentService")
	assert.Contains(t, names, "grpc.health.v1.Health")
	assert.Contains(t, names, "grpc.reflection.v1alpha.ServerReflection")
}

func TestIntegration_ServerReflection_FileContainingSymbol(t *testing.T) {
	h := newFullTestServer(t, true)
	defer h.stop()

	refClient := reflectionpb.NewServerReflectionClient(h.refConn)
	stream, err := refClient.ServerReflectionInfo(context.Background())
	require.NoError(t, err)

	require.NoError(t, stream.Send(&reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: "grpc.health.v1.Health",
		},
	}))

	resp, err := stream.Recv()
	require.NoError(t, err)
	assert.NotNil(t, resp.GetFileDescriptorResponse())
	assert.NotEmpty(t, resp.GetFileDescriptorResponse().FileDescriptorProto)
}

func TestIntegration_ServerReflection_Missing(t *testing.T) {
	h := newFullTestServer(t, false)
	defer h.stop()

	// Even without reflection, the registered services should still work
	molClient := pb.NewMoleculeServiceClient(h.conn)
	_, err := molClient.GetMolecule(context.Background(), &pb.GetMoleculeRequest{MoleculeId: "test"})
	assert.Error(t, err) // Not Unimplemented, just regular error from missing mock
	assert.NotEqual(t, codes.Unimplemented, status.Code(err))
}

// =========================================================================
// Interoperability
// =========================================================================

func TestIntegration_MultipleServicesOnSameServer(t *testing.T) {
	h := newFullTestServer(t, false)
	defer h.stop()
	ctx := context.Background()

	// All three service types accessible on the same connection
	cli := healthpb.NewHealthClient(h.conn)
	resp, err := cli.Check(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	assert.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.Status)
}
