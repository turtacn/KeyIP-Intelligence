package infringe_net

import (
	"context"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"golang.org/x/sync/errgroup"
)

// RiskLevel defines the level of infringement risk.
type RiskLevel string

const (
	RiskCritical RiskLevel = "Critical"
	RiskHigh     RiskLevel = "High"
	RiskMedium   RiskLevel = "Medium"
	RiskLow      RiskLevel = "Low"
	RiskNone     RiskLevel = "None"
)

// AssessmentRequest represents a request for infringement assessment.
type AssessmentRequest struct {
	MoleculeSMILES string
	TargetClaims   []*ClaimInput
	Options        *AssessmentOptions
}

// AssessmentOptions defines options for assessment.
type AssessmentOptions struct {
	IncludeEquivalents bool
	IncludeEstoppel    bool
	ConfidenceThreshold float64
}

// AssessmentResult represents the result of infringement assessment.
type AssessmentResult struct {
	RequestID         string
	OverallRiskLevel  RiskLevel
	OverallScore      float64
	LiteralAnalysis   *LiteralPredictionResult
	EquivalentsAnalysis *EquivalentsResult
	EstoppelCheck     *EstoppelResult
	MatchedClaims     []string
	Confidence        float64
	ProcessingTimeMs  int64
}

// InfringementAssessor defines the interface for infringement assessment.
type InfringementAssessor interface {
	Assess(ctx context.Context, req *AssessmentRequest) (*AssessmentResult, error)
	BatchAssess(ctx context.Context, reqs []*AssessmentRequest) ([]*AssessmentResult, error)
}

// infringementAssessor implements InfringementAssessor.
type infringementAssessor struct {
	model       InfringeModel
	equivalents EquivalentsAnalyzer
	mapper      ClaimElementMapper
	batch       common.BatchProcessor[*AssessmentRequest, *AssessmentResult]
	metrics     common.IntelligenceMetrics
	logger      logging.Logger
}

// NewInfringementAssessor creates a new InfringementAssessor.
func NewInfringementAssessor(
	model InfringeModel,
	equivalents EquivalentsAnalyzer,
	mapper ClaimElementMapper,
	batch common.BatchProcessor[*AssessmentRequest, *AssessmentResult],
	metrics common.IntelligenceMetrics,
	logger logging.Logger,
) InfringementAssessor {
	return &infringementAssessor{
		model:       model,
		equivalents: equivalents,
		mapper:      mapper,
		batch:       batch,
		metrics:     metrics,
		logger:      logger,
	}
}

func (a *infringementAssessor) Assess(ctx context.Context, req *AssessmentRequest) (*AssessmentResult, error) {
	start := time.Now()

	// 1. Map Elements
	mappedClaims, err := a.mapper.MapElements(ctx, req.TargetClaims)
	if err != nil {
		return nil, err
	}

	molElements, err := a.mapper.MapMoleculeToElements(ctx, req.MoleculeSMILES)
	if err != nil {
		return nil, err
	}

	// 2. Parallel Execution
	g, ctx := errgroup.WithContext(ctx)

	var literalRes *LiteralPredictionResult
	var equivRes *EquivalentsResult
	var estoppelRes *EstoppelResult

	// Literal Infringement
	g.Go(func() error {
		// Flatten mapped claims to features
		var features []*ClaimElementFeature
		for _, mc := range mappedClaims {
			for _, el := range mc.Elements {
				// Mock conversion from ClaimElement to ClaimElementFeature
				features = append(features, &ClaimElementFeature{ElementID: el.ElementID})
			}
		}

		res, err := a.model.PredictLiteralInfringement(ctx, &LiteralPredictionRequest{
			MoleculeSMILES: req.MoleculeSMILES,
			ClaimElements:  features,
			PredictionMode: "Strict",
		})
		if err != nil {
			return err
		}
		literalRes = res
		return nil
	})

	// Equivalents
	if req.Options != nil && req.Options.IncludeEquivalents {
		g.Go(func() error {
			// Flatten claims for equivalents
			var claimEls []*StructuralElement
			for _, mc := range mappedClaims {
				for _, el := range mc.Elements {
					claimEls = append(claimEls, &StructuralElement{
						ElementID:   el.ElementID,
						Description: el.Description,
						ElementType: el.ElementType,
					})
				}
			}

			res, err := a.equivalents.Analyze(ctx, &EquivalentsRequest{
				QueryMolecule: molElements,
				ClaimElements: claimEls,
			})
			if err != nil {
				return err
			}
			equivRes = res
			return nil
		})
	}

	// Estoppel
	if req.Options != nil && req.Options.IncludeEstoppel {
		g.Go(func() error {
			// Need alignment first. Doing separate alignment here or reusing?
			// Mapper needs claim elements.
			// Simplified: assume alignment logic is fast
			// In real code, alignment should be shared or executed once.
			// But checkEstoppel needs alignment.
			// Let's call AlignElements inside this goroutine for now.

			var allClaimEls []*ClaimElement
			for _, mc := range mappedClaims {
				allClaimEls = append(allClaimEls, mc.Elements...)
			}

			align, err := a.mapper.AlignElements(ctx, molElements, allClaimEls)
			if err != nil {
				return err
			}

			// Mock history: Need patent ID to fetch history.
			// Assuming history passed or fetched internally.
			// Here passed as nil, so estoppel check returns false.
			res, err := a.mapper.CheckEstoppel(ctx, align, nil)
			if err != nil {
				return err
			}
			estoppelRes = res
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// 3. Synthesize Result
	score := literalRes.OverallScore
	if equivRes != nil {
		// Weighted sum: 0.7 literal + 0.3 equivalents (if literal < 1.0)
		if score < 0.99 {
			score = score*0.7 + equivRes.OverallEquivalenceScore*0.3
		}
	}
	if estoppelRes != nil && estoppelRes.HasEstoppel {
		score -= estoppelRes.EstoppelPenalty * 0.2 // Penalty
	}
	if score < 0 { score = 0 }

	level := RiskLow
	if score >= 0.85 {
		level = RiskCritical
	} else if score >= 0.70 {
		level = RiskHigh
	} else if score >= 0.50 {
		level = RiskMedium
	}

	// Record metrics
	if a.metrics != nil {
		a.metrics.RecordRiskAssessment(ctx, string(level), float64(time.Since(start).Milliseconds()))
	}

	return &AssessmentResult{
		OverallRiskLevel: level,
		OverallScore:     score,
		LiteralAnalysis:  literalRes,
		EquivalentsAnalysis: equivRes,
		EstoppelCheck:    estoppelRes,
		ProcessingTimeMs: time.Since(start).Milliseconds(),
	}, nil
}

func (a *infringementAssessor) BatchAssess(ctx context.Context, reqs []*AssessmentRequest) ([]*AssessmentResult, error) {
	if a.batch == nil {
		// Fallback to sequential
		var results []*AssessmentResult
		for _, req := range reqs {
			res, err := a.Assess(ctx, req)
			if err != nil {
				return nil, err
			}
			results = append(results, res)
		}
		return results, nil
	}

	res, err := a.batch.Process(ctx, reqs, func(c context.Context, item *AssessmentRequest) (*AssessmentResult, error) {
		return a.Assess(c, item)
	})
	if err != nil {
		return nil, err
	}

	var outcomes []*AssessmentResult
	for _, r := range res.Results {
		if r.Error != nil {
			// Log error or return partial?
			// For now return nil for failed items
			outcomes = append(outcomes, nil)
		} else {
			outcomes = append(outcomes, r.Result)
		}
	}
	return outcomes, nil
}

//Personal.AI order the ending
