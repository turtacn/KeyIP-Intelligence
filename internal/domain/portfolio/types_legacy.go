package portfolio

// Legacy types for infrastructure compatibility

type Status = PortfolioStatus

type Valuation = PortfolioValuation

type OptimizationSuggestion struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Savings     float64 `json:"savings"`
}

type Summary = PortfolioSummary

type ExpiryTimelineEntry struct {
	Year        int     `json:"year"`
	Count       int     `json:"count"`
	Description string  `json:"description"`
}

type ComparisonResult struct {
	// Fields that legacy repo might use
}

// Ensure methods match if needed (e.g. Valuation might need extra fields)
type ValuationModel struct {
	// If repo expects specific fields not in PortfolioValuation
	// For now, assume mapping is close enough or repo usage is simple
}
