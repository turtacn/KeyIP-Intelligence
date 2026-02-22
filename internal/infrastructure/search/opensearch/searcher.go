package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// SearcherConfig holds configuration for the Searcher.
type SearcherConfig struct {
	DefaultPageSize        int
	MaxPageSize            int
	DefaultHighlightPreTag string
	DefaultHighlightPostTag string
	SearchTimeout          time.Duration
	ScrollKeepAlive        time.Duration
	MaxScrollSize          int
}

// SearchRequest defines a search query.
type SearchRequest struct {
	IndexName      string
	Query          *Query
	Filters        []Filter
	Sort           []SortField
	Pagination     *Pagination
	Highlight      *HighlightConfig
	Aggregations   map[string]Aggregation
	SourceIncludes []string
	SourceExcludes []string
}

// Query defines a search query structure.
type Query struct {
	QueryType          string
	Field              string
	Fields             []string
	Value              interface{}
	Boost              float64
	Must               []Query
	Should             []Query
	MustNot            []Query
	MinimumShouldMatch string
}

// Filter defines a filter condition.
type Filter struct {
	Field     string
	FilterType string
	Value     interface{}
	RangeFrom interface{}
	RangeTo   interface{}
}

// SortField defines sorting criteria.
type SortField struct {
	Field string
	Order string
}

// Pagination defines pagination parameters.
type Pagination struct {
	Offset int
	Limit  int
}

// HighlightConfig defines highlighting settings.
type HighlightConfig struct {
	Fields            []string
	PreTag            string
	PostTag           string
	FragmentSize      int
	NumberOfFragments int
}

// Aggregation defines an aggregation.
type Aggregation struct {
	AggType         string
	Field           string
	Size            int
	Interval        string
	Ranges          []AggRange
	SubAggregations map[string]Aggregation
}

// AggRange defines a range for range aggregation.
type AggRange struct {
	Key  string
	From interface{}
	To   interface{}
}

// SearchResult holds the search response.
type SearchResult struct {
	Total        int64
	MaxScore     float64
	Hits         []SearchHit
	Aggregations map[string]AggregationResult
	TookMs       int64
}

// SearchHit represents a single search hit.
type SearchHit struct {
	ID         string
	Score      float64
	Source     json.RawMessage
	Highlights map[string][]string
	Sort       []interface{}
}

// AggregationResult holds the result of an aggregation.
type AggregationResult struct {
	Buckets []AggBucket
	Value   *float64
}

// AggBucket represents a bucket in an aggregation result.
type AggBucket struct {
	Key             interface{}
	KeyAsString     string
	DocCount        int64
	SubAggregations map[string]AggregationResult
}

// Searcher performs search operations.
type Searcher struct {
	client *Client
	config SearcherConfig
	logger logging.Logger
}

// NewSearcher creates a new Searcher.
func NewSearcher(client *Client, cfg SearcherConfig, logger logging.Logger) *Searcher {
	if cfg.DefaultPageSize == 0 {
		cfg.DefaultPageSize = 20
	}
	if cfg.MaxPageSize == 0 {
		cfg.MaxPageSize = 100
	}
	if cfg.DefaultHighlightPreTag == "" {
		cfg.DefaultHighlightPreTag = "<em>"
	}
	if cfg.DefaultHighlightPostTag == "" {
		cfg.DefaultHighlightPostTag = "</em>"
	}
	if cfg.SearchTimeout == 0 {
		cfg.SearchTimeout = 10 * time.Second
	}
	if cfg.ScrollKeepAlive == 0 {
		cfg.ScrollKeepAlive = 5 * time.Minute
	}
	if cfg.MaxScrollSize == 0 {
		cfg.MaxScrollSize = 1000
	}

	return &Searcher{
		client: client,
		config: cfg,
		logger: logger,
	}
}

// Search executes a search request.
func (s *Searcher) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	if req.IndexName == "" {
		return nil, errors.New(errors.ErrCodeValidation, "IndexName is required")
	}

	// Validate and adjust pagination
	if req.Pagination == nil {
		req.Pagination = &Pagination{Offset: 0, Limit: s.config.DefaultPageSize}
	}
	if req.Pagination.Limit > s.config.MaxPageSize {
		req.Pagination.Limit = s.config.MaxPageSize
	}
	if req.Pagination.Offset < 0 {
		req.Pagination.Offset = 0
	}

	dsl, err := s.buildQueryDSL(req)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(dsl)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeSerialization, "failed to marshal query DSL")
	}

	osReq := opensearchapi.SearchRequest{
		Index: []string{req.IndexName},
		Body:  bytes.NewReader(body),
	}

	// Set timeout? opensearchapi usually takes context
	// We can wrap context with timeout if needed, but ctx is passed.

	start := time.Now()
	resp, err := osReq.Do(ctx, s.client.GetClient())
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, errors.New(errors.ErrCodeTimeout, "search request timed out")
		}
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "search request failed")
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, s.handleErrorResponse(resp)
	}

	result, err := s.parseSearchResponse(resp.Body)
	if err != nil {
		return nil, err
	}

	s.logger.Debug("Search executed",
		logging.String("index", req.IndexName),
		logging.Int64("took_ms", time.Since(start).Milliseconds()),
		logging.Int64("hits", result.Total))

	return result, nil
}

// Count returns the number of documents matching the query.
func (s *Searcher) Count(ctx context.Context, indexName string, query *Query, filters []Filter) (int64, error) {
	// Build partial request just for query/filter
	req := SearchRequest{
		IndexName: indexName,
		Query:     query,
		Filters:   filters,
	}
	dsl, err := s.buildQueryDSL(req)
	if err != nil {
		return 0, err
	}

	// DSL might contain from/size/sort which Count API doesn't like?
	// Usually Count API takes query body.
	// We should strip non-query parts.
	queryDSL := map[string]interface{}{}
	if q, ok := dsl["query"]; ok {
		queryDSL["query"] = q
	}

	body, err := json.Marshal(queryDSL)
	if err != nil {
		return 0, errors.Wrap(err, errors.ErrCodeSerialization, "failed to marshal count query")
	}

	osReq := opensearchapi.CountRequest{
		Index: []string{indexName},
		Body:  bytes.NewReader(body),
	}

	resp, err := osReq.Do(ctx, s.client.GetClient())
	if err != nil {
		return 0, errors.Wrap(err, errors.ErrCodeInternal, "count request failed")
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return 0, s.handleErrorResponse(resp)
	}

	var countResp struct {
		Count int64 `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&countResp); err != nil {
		return 0, errors.Wrap(err, errors.ErrCodeSerialization, "failed to decode count response")
	}

	return countResp.Count, nil
}

// ScrollSearch performs a scroll search.
func (s *Searcher) ScrollSearch(ctx context.Context, req SearchRequest, batchHandler func(hits []SearchHit) error) error {
	// Start scroll
	dsl, err := s.buildQueryDSL(req)
	if err != nil {
		return err
	}
	// Remove from/size from DSL as scroll manages it? No, size sets batch size.
	// Default size if not set
	if req.Pagination == nil {
		dsl["size"] = s.config.MaxScrollSize
	} else {
		dsl["size"] = req.Pagination.Limit // User defined batch size
	}
	delete(dsl, "from") // Scroll ignores from, always next batch

	body, err := json.Marshal(dsl)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeSerialization, "failed to marshal scroll query")
	}

	osReq := opensearchapi.SearchRequest{
		Index:  []string{req.IndexName},
		Body:   bytes.NewReader(body),
		Scroll: s.config.ScrollKeepAlive,
	}

	resp, err := osReq.Do(ctx, s.client.GetClient())
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "initial scroll request failed")
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return s.handleErrorResponse(resp)
	}

	// Let's decode into generic map to get scroll_id and hits
	var rawResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
		return errors.Wrap(err, errors.ErrCodeSerialization, "failed to decode scroll response")
	}

	scrollID, _ := rawResp["_scroll_id"].(string)
	hitsRaw, _ := rawResp["hits"].(map[string]interface{})
	hitsList, _ := hitsRaw["hits"].([]interface{})

	// Process first batch
	if len(hitsList) > 0 {
		// We need to parse hitsList into []SearchHit
		// Re-encode and use parse logic or manual map
		// This is inefficient. Ideally parseSearchResponse should support ScrollID or return generic.
		// For now, let's implement a parser for hits.
		hits, err := s.parseHits(hitsList)
		if err != nil {
			s.clearScroll(ctx, scrollID)
			return err
		}
		if err := batchHandler(hits); err != nil {
			s.clearScroll(ctx, scrollID)
			return err
		}
	} else {
		// No hits, done
		s.clearScroll(ctx, scrollID)
		return nil
	}

	// Loop
	for {
		scrollReq := opensearchapi.ScrollRequest{
			ScrollID: scrollID,
			Scroll:   s.config.ScrollKeepAlive,
		}

		resp, err := scrollReq.Do(ctx, s.client.GetClient())
		if err != nil {
			s.clearScroll(ctx, scrollID)
			return errors.Wrap(err, errors.ErrCodeInternal, "scroll request failed")
		}
		defer resp.Body.Close()

		if resp.IsError() {
			s.clearScroll(ctx, scrollID)
			return s.handleErrorResponse(resp)
		}

		if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
			s.clearScroll(ctx, scrollID)
			return errors.Wrap(err, errors.ErrCodeSerialization, "failed to decode scroll response")
		}

		// Update scrollID if changed (usually same)
		if newID, ok := rawResp["_scroll_id"].(string); ok && newID != "" {
			scrollID = newID
		}

		hitsRaw, _ = rawResp["hits"].(map[string]interface{})
		hitsList, _ = hitsRaw["hits"].([]interface{})

		if len(hitsList) == 0 {
			break // Done
		}

		hits, err := s.parseHits(hitsList)
		if err != nil {
			s.clearScroll(ctx, scrollID)
			return err
		}

		if err := batchHandler(hits); err != nil {
			s.clearScroll(ctx, scrollID)
			return err
		}
	}

	return s.clearScroll(ctx, scrollID)
}

func (s *Searcher) clearScroll(ctx context.Context, scrollID string) error {
	if scrollID == "" {
		return nil
	}
	req := opensearchapi.ClearScrollRequest{
		ScrollID: []string{scrollID},
	}
	_, err := req.Do(ctx, s.client.GetClient())
	return err
}

// MultiSearch performs multiple searches in a single request.
func (s *Searcher) MultiSearch(ctx context.Context, requests []SearchRequest) ([]*SearchResult, error) {
	var buf bytes.Buffer
	for _, req := range requests {
		// Header: {"index": "name"}
		meta := fmt.Sprintf(`{"index": "%s"}`, req.IndexName)
		buf.WriteString(meta + "\n")

		// Body: Query DSL
		dsl, err := s.buildQueryDSL(req)
		if err != nil {
			return nil, err
		}
		body, err := json.Marshal(dsl)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeSerialization, "failed to marshal msearch body")
		}
		buf.Write(body)
		buf.WriteString("\n")
	}

	osReq := opensearchapi.MsearchRequest{
		Body: bytes.NewReader(buf.Bytes()),
	}

	resp, err := osReq.Do(ctx, s.client.GetClient())
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "msearch request failed")
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, s.handleErrorResponse(resp)
	}

	var msearchResp struct {
		Responses []json.RawMessage `json:"responses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&msearchResp); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeSerialization, "failed to decode msearch response")
	}

	results := make([]*SearchResult, len(requests))
	for i, raw := range msearchResp.Responses {
		// Check for error in individual response
		// We need to parse it partially to see if it's an error
		// Reuse parseSearchResponse but handle raw json
		// Or wrap raw in a reader
		r, err := s.parseSearchResponse(bytes.NewReader(raw))
		if err != nil {
			// Check if it's an error response
			// Log warn and set nil
			s.logger.Warn("msearch sub-request failed", logging.Error(err))
			results[i] = nil
		} else {
			results[i] = r
		}
	}

	return results, nil
}

// Suggest provides search suggestions.
func (s *Searcher) Suggest(ctx context.Context, indexName string, field string, text string, size int) ([]string, error) {
	// Using completion suggester or just prefix match?
	// Prompt says "Suggest ... 构建 completion suggest 查询".
	// Assuming "completion" type field mapping.

	suggestName := "my-suggest"
	dsl := map[string]interface{}{
		"suggest": map[string]interface{}{
			suggestName: map[string]interface{}{
				"prefix": text,
				"completion": map[string]interface{}{
					"field": field,
					"size":  size,
				},
			},
		},
	}

	body, err := json.Marshal(dsl)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeSerialization, "failed to marshal suggest query")
	}

	osReq := opensearchapi.SearchRequest{
		Index: []string{indexName},
		Body:  bytes.NewReader(body),
	}

	resp, err := osReq.Do(ctx, s.client.GetClient())
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "suggest request failed")
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, s.handleErrorResponse(resp)
	}

	var suggestResp struct {
		Suggest map[string][]struct {
			Text    string `json:"text"`
			Options []struct {
				Text string `json:"text"`
			} `json:"options"`
		} `json:"suggest"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&suggestResp); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeSerialization, "failed to decode suggest response")
	}

	var suggestions []string
	if opts, ok := suggestResp.Suggest[suggestName]; ok {
		for _, entry := range opts {
			for _, option := range entry.Options {
				suggestions = append(suggestions, option.Text)
			}
		}
	}

	return suggestions, nil
}

// Private methods

func (s *Searcher) buildQueryDSL(req SearchRequest) (map[string]interface{}, error) {
	dsl := map[string]interface{}{}

	// Query
	var queryMap map[string]interface{}
	if req.Query != nil {
		queryMap = s.buildQuery(req.Query)
	}

	// Filters
	if len(req.Filters) > 0 {
		filterClauses := make([]map[string]interface{}, len(req.Filters))
		for i, f := range req.Filters {
			filterClauses[i] = s.buildFilter(f)
		}

		// Wrap query in bool/filter
		// If there is already a query, it goes to "must". Filters go to "filter".
		boolQuery := map[string]interface{}{
			"filter": filterClauses,
		}
		if queryMap != nil {
			boolQuery["must"] = queryMap
		} else {
			boolQuery["must"] = map[string]interface{}{"match_all": map[string]interface{}{}}
		}
		queryMap = map[string]interface{}{"bool": boolQuery}
	}

	if queryMap != nil {
		dsl["query"] = queryMap
	}

	// Pagination
	if req.Pagination != nil {
		dsl["from"] = req.Pagination.Offset
		dsl["size"] = req.Pagination.Limit
	}

	// Sort
	if len(req.Sort) > 0 {
		sortList := make([]map[string]interface{}, len(req.Sort))
		for i, sort := range req.Sort {
			sortList[i] = map[string]interface{}{
				sort.Field: map[string]interface{}{"order": sort.Order},
			}
		}
		dsl["sort"] = sortList
	}

	// Highlight
	if req.Highlight != nil {
		fields := map[string]interface{}{}
		for _, f := range req.Highlight.Fields {
			fields[f] = map[string]interface{}{}
		}
		dsl["highlight"] = map[string]interface{}{
			"fields":        fields,
			"pre_tags":      []string{req.Highlight.PreTag},
			"post_tags":     []string{req.Highlight.PostTag},
			"fragment_size": req.Highlight.FragmentSize,
			"number_of_fragments": req.Highlight.NumberOfFragments,
		}
	}

	// Aggregations
	if len(req.Aggregations) > 0 {
		dsl["aggs"] = s.buildAggregations(req.Aggregations)
	}

	// Source filtering
	if len(req.SourceIncludes) > 0 || len(req.SourceExcludes) > 0 {
		dsl["_source"] = map[string]interface{}{
			"includes": req.SourceIncludes,
			"excludes": req.SourceExcludes,
		}
	}

	return dsl, nil
}

func (s *Searcher) buildQuery(q *Query) map[string]interface{} {
	switch q.QueryType {
	case "match":
		return map[string]interface{}{
			"match": map[string]interface{}{
				q.Field: map[string]interface{}{
					"query": q.Value,
					"boost": q.Boost,
				},
			},
		}
	case "multi_match":
		return map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  q.Value,
				"fields": q.Fields,
			},
		}
	case "term":
		return map[string]interface{}{
			"term": map[string]interface{}{
				q.Field: q.Value,
			},
		}
	case "terms":
		return map[string]interface{}{
			"terms": map[string]interface{}{
				q.Field: q.Value,
			},
		}
	case "range":
		return map[string]interface{}{
			"range": map[string]interface{}{
				q.Field: q.Value,
			},
		}
	case "bool":
		boolQ := map[string]interface{}{}
		if len(q.Must) > 0 {
			clauses := make([]map[string]interface{}, len(q.Must))
			for i, sub := range q.Must {
				clauses[i] = s.buildQuery(&sub)
			}
			boolQ["must"] = clauses
		}
		if len(q.Should) > 0 {
			clauses := make([]map[string]interface{}, len(q.Should))
			for i, sub := range q.Should {
				clauses[i] = s.buildQuery(&sub)
			}
			boolQ["should"] = clauses
		}
		if len(q.MustNot) > 0 {
			clauses := make([]map[string]interface{}, len(q.MustNot))
			for i, sub := range q.MustNot {
				clauses[i] = s.buildQuery(&sub)
			}
			boolQ["must_not"] = clauses
		}
		if q.MinimumShouldMatch != "" {
			boolQ["minimum_should_match"] = q.MinimumShouldMatch
		}
		return map[string]interface{}{"bool": boolQ}
	case "match_phrase":
		return map[string]interface{}{
			"match_phrase": map[string]interface{}{
				q.Field: q.Value,
			},
		}
	case "wildcard":
		return map[string]interface{}{
			"wildcard": map[string]interface{}{
				q.Field: q.Value,
			},
		}
	case "exists":
		return map[string]interface{}{
			"exists": map[string]interface{}{
				"field": q.Field,
			},
		}
	}
	return nil
}

func (s *Searcher) buildFilter(f Filter) map[string]interface{} {
	switch f.FilterType {
	case "term":
		return map[string]interface{}{
			"term": map[string]interface{}{f.Field: f.Value},
		}
	case "terms":
		return map[string]interface{}{
			"terms": map[string]interface{}{f.Field: f.Value},
		}
	case "range":
		rangeMap := map[string]interface{}{}
		if f.RangeFrom != nil {
			rangeMap["gte"] = f.RangeFrom
		}
		if f.RangeTo != nil {
			rangeMap["lte"] = f.RangeTo
		}
		return map[string]interface{}{
			"range": map[string]interface{}{f.Field: rangeMap},
		}
	case "exists":
		return map[string]interface{}{
			"exists": map[string]interface{}{"field": f.Field},
		}
	}
	return nil
}

func (s *Searcher) buildAggregations(aggs map[string]Aggregation) map[string]interface{} {
	dsl := map[string]interface{}{}
	for name, agg := range aggs {
		aggDSL := map[string]interface{}{}

		switch agg.AggType {
		case "terms":
			aggDSL["terms"] = map[string]interface{}{
				"field": agg.Field,
				"size":  agg.Size,
			}
		case "date_histogram":
			aggDSL["date_histogram"] = map[string]interface{}{
				"field":    agg.Field,
				"interval": agg.Interval,
				"calendar_interval": agg.Interval,
			}
		case "range":
			ranges := make([]map[string]interface{}, len(agg.Ranges))
			for i, r := range agg.Ranges {
				ranges[i] = map[string]interface{}{
					"key":  r.Key,
					"from": r.From,
					"to":   r.To,
				}
			}
			aggDSL["range"] = map[string]interface{}{
				"field":  agg.Field,
				"ranges": ranges,
			}
		case "avg":
			aggDSL["avg"] = map[string]interface{}{"field": agg.Field}
		case "sum":
			aggDSL["sum"] = map[string]interface{}{"field": agg.Field}
		case "cardinality":
			aggDSL["cardinality"] = map[string]interface{}{"field": agg.Field}
		}

		if len(agg.SubAggregations) > 0 {
			aggDSL["aggs"] = s.buildAggregations(agg.SubAggregations)
		}

		dsl[name] = aggDSL
	}
	return dsl
}

func (s *Searcher) parseSearchResponse(body io.Reader) (*SearchResult, error) {
	var resp struct {
		Took int64 `json:"took"`
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			MaxScore float64 `json:"max_score"`
			Hits     []struct {
				ID        string                 `json:"_id"`
				Score     float64                `json:"_score"`
				Source    json.RawMessage        `json:"_source"`
				Highlight map[string][]string    `json:"highlight"`
				Sort      []interface{}          `json:"sort"`
			} `json:"hits"`
		} `json:"hits"`
		Aggregations map[string]json.RawMessage `json:"aggregations"`
	}

	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeSerialization, "failed to decode search response")
	}

	result := &SearchResult{
		Total:    resp.Hits.Total.Value,
		MaxScore: resp.Hits.MaxScore,
		TookMs:   resp.Took,
	}

	for _, h := range resp.Hits.Hits {
		result.Hits = append(result.Hits, SearchHit{
			ID:         h.ID,
			Score:      h.Score,
			Source:     h.Source,
			Highlights: h.Highlight,
			Sort:       h.Sort,
		})
	}

	if len(resp.Aggregations) > 0 {
		result.Aggregations = make(map[string]AggregationResult)
		for name, raw := range resp.Aggregations {
			result.Aggregations[name] = s.parseAggregationResult(raw)
		}
	}

	return result, nil
}

func (s *Searcher) parseHits(hitsList []interface{}) ([]SearchHit, error) {
	hits := make([]SearchHit, len(hitsList))
	for i, item := range hitsList {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid hit format")
		}

		h := SearchHit{}
		if id, ok := m["_id"].(string); ok { h.ID = id }
		if score, ok := m["_score"].(float64); ok { h.Score = score }
		if src, ok := m["_source"]; ok {
			b, _ := json.Marshal(src)
			h.Source = b
		}
		if hl, ok := m["highlight"].(map[string]interface{}); ok {
			h.Highlights = make(map[string][]string)
			for k, v := range hl {
				if strs, ok := v.([]interface{}); ok {
					var ss []string
					for _, s := range strs {
						ss = append(ss, fmt.Sprint(s))
					}
					h.Highlights[k] = ss
				}
			}
		}
		if srt, ok := m["sort"].([]interface{}); ok {
			h.Sort = srt
		}
		hits[i] = h
	}
	return hits, nil
}

func (s *Searcher) parseAggregationResult(raw json.RawMessage) AggregationResult {
	// Need to determine if it's bucket or metric agg
	var asMap map[string]interface{}
	json.Unmarshal(raw, &asMap)

	res := AggregationResult{}

	if val, ok := asMap["value"].(float64); ok {
		res.Value = &val
	}

	if buckets, ok := asMap["buckets"].([]interface{}); ok {
		for _, b := range buckets {
			bMap, ok := b.(map[string]interface{})
			if !ok { continue }

			bucket := AggBucket{}
			if key, ok := bMap["key"]; ok {
				bucket.Key = key
			}
			if keyS, ok := bMap["key_as_string"].(string); ok {
				bucket.KeyAsString = keyS
			} else {
				bucket.KeyAsString = fmt.Sprint(bucket.Key)
			}
			if docCount, ok := bMap["doc_count"].(float64); ok { // JSON numbers are float64
				bucket.DocCount = int64(docCount)
			}

			bucket.SubAggregations = make(map[string]AggregationResult)
			for k, v := range bMap {
				if k == "key" || k == "doc_count" || k == "key_as_string" { continue }

				if subRaw, err := json.Marshal(v); err == nil {
					// Check if it looks like an agg result
					var check map[string]interface{}
					if vMap, ok := v.(map[string]interface{}); ok {
						check = vMap
					} else {
						continue
					}

					if _, hasBuckets := check["buckets"]; hasBuckets {
						bucket.SubAggregations[k] = s.parseAggregationResult(subRaw)
					} else if _, hasValue := check["value"]; hasValue {
						bucket.SubAggregations[k] = s.parseAggregationResult(subRaw)
					}
				}
			}
			res.Buckets = append(res.Buckets, bucket)
		}
	}

	return res
}

func (s *Searcher) handleErrorResponse(resp *opensearchapi.Response) error {
	// Reusing logic from Indexer or duplicating
	bodyBytes, _ := io.ReadAll(resp.Body)
	var errResp struct {
		Error struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error"`
	}
	if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error.Reason != "" {
		return errors.Wrapf(errors.New(errors.ErrCodeInternal, "search error"), errors.ErrCodeInternal, "OpenSearch error: %s - %s", errResp.Error.Type, errResp.Error.Reason)
	}
	return errors.Wrapf(errors.New(errors.ErrCodeInternal, "search error"), errors.ErrCodeInternal, "OpenSearch error status: %d", resp.StatusCode)
}

//Personal.AI order the ending
