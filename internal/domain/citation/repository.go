package citation

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type CitationRepository interface {
	EnsurePatentNode(ctx context.Context, patentID uuid.UUID, patentNumber string, jurisdiction string, filingDate *time.Time) error
	BatchEnsurePatentNodes(ctx context.Context, patents []*PatentNodeData) error

	CreateCitation(ctx context.Context, fromPatentID, toPatentID uuid.UUID, citationType string, metadata map[string]interface{}) error
	DeleteCitation(ctx context.Context, fromPatentID, toPatentID uuid.UUID, citationType string) error
	BatchCreateCitations(ctx context.Context, citations []*CitationEdge) error

	GetForwardCitations(ctx context.Context, patentID uuid.UUID, depth int, limit int) ([]*CitationPath, error)
	GetBackwardCitations(ctx context.Context, patentID uuid.UUID, depth int, limit int) ([]*CitationPath, error)
	GetCitationNetwork(ctx context.Context, patentID uuid.UUID, depth int) (*CitationNetwork, error)
	GetCitationCount(ctx context.Context, patentID uuid.UUID) (*CitationStats, error)
	GetMostCitedPatents(ctx context.Context, jurisdiction *string, limit int) ([]*PatentWithCitationCount, error)

	GetCitationChain(ctx context.Context, fromPatentID, toPatentID uuid.UUID) ([]*CitationPath, error)
	GetCoCitationPatents(ctx context.Context, patentID uuid.UUID, minCommonCitations int, limit int) ([]*CoCitationResult, error)
	GetBibliographicCoupling(ctx context.Context, patentID uuid.UUID, minCommonReferences int, limit int) ([]*CouplingResult, error)

	CalculatePageRank(ctx context.Context, iterations int, dampingFactor float64) error
	GetTopPageRankPatents(ctx context.Context, limit int) ([]*PatentWithPageRank, error)
}

//Personal.AI order the ending
