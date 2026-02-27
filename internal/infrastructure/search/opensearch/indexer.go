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
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

var (
	ErrIndexAlreadyExists  = errors.New(errors.ErrCodeConflict, "index already exists")
	ErrIndexNotFound       = errors.New(errors.ErrCodeNotFound, "index not found")
	ErrIndexCreationFailed = errors.New(errors.ErrCodeInternal, "index creation failed")
	ErrDocumentIndexFailed = errors.New(errors.ErrCodeInternal, "document index failed")
	ErrDocumentNotFound    = errors.New(errors.ErrCodeNotFound, "document not found")
	ErrMappingConflict     = errors.New(errors.ErrCodeConflict, "mapping conflict")
)

// IndexerConfig holds configuration for the Indexer.
type IndexerConfig struct {
	BulkBatchSize     int
	BulkFlushInterval time.Duration
	BulkFlushBytes    int
	BulkWorkers       int
	RefreshPolicy     string
}

// Indexer manages index operations and document ingestion.
type Indexer struct {
	client *Client
	config IndexerConfig
	logger logging.Logger
}

// NewIndexer creates a new Indexer.
func NewIndexer(client *Client, cfg IndexerConfig, logger logging.Logger) *Indexer {
	if cfg.BulkBatchSize == 0 {
		cfg.BulkBatchSize = 500
	}
	if cfg.BulkFlushInterval == 0 {
		cfg.BulkFlushInterval = 5 * time.Second
	}
	if cfg.BulkFlushBytes == 0 {
		cfg.BulkFlushBytes = 5 * 1024 * 1024 // 5MB
	}
	if cfg.BulkWorkers == 0 {
		cfg.BulkWorkers = 2
	}
	if cfg.RefreshPolicy == "" {
		cfg.RefreshPolicy = "false"
	}

	return &Indexer{
		client: client,
		config: cfg,
		logger: logger,
	}
}

// CreateIndex creates a new index with the given mapping.
func (i *Indexer) CreateIndex(ctx context.Context, indexName string, mapping common.IndexMapping) error {
	// Check if index exists
	exists, err := i.IndexExists(ctx, indexName)
	if err != nil {
		return err
	}
	if exists {
		return ErrIndexAlreadyExists
	}

	body, err := json.Marshal(mapping)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeSerialization, "failed to marshal index mapping")
	}

	req := opensearchapi.IndicesCreateRequest{
		Index: indexName,
		Body:  bytes.NewReader(body),
	}

	resp, err := req.Do(ctx, i.client.GetClient())
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to create index request")
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return i.handleErrorResponse(resp, ErrIndexCreationFailed)
	}

	i.logger.Info("Index created", logging.String("index", indexName))
	return nil
}

// DeleteIndex deletes an index.
func (i *Indexer) DeleteIndex(ctx context.Context, indexName string) error {
	req := opensearchapi.IndicesDeleteRequest{
		Index: []string{indexName},
	}

	resp, err := req.Do(ctx, i.client.GetClient())
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to delete index request")
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return ErrIndexNotFound
	}

	if resp.IsError() {
		return i.handleErrorResponse(resp, errors.New(errors.ErrCodeInternal, "delete index failed"))
	}

	i.logger.Warn("Index deleted", logging.String("index", indexName))
	return nil
}

// IndexExists checks if an index exists.
func (i *Indexer) IndexExists(ctx context.Context, indexName string) (bool, error) {
	req := opensearchapi.IndicesExistsRequest{
		Index: []string{indexName},
	}

	resp, err := req.Do(ctx, i.client.GetClient())
	if err != nil {
		return false, errors.Wrap(err, errors.ErrCodeInternal, "failed to check index existence")
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return true, nil
	}
	if resp.StatusCode == 404 {
		return false, nil
	}

	return false, i.handleErrorResponse(resp, errors.New(errors.ErrCodeInternal, "check index existence failed"))
}

// IndexDocument indexes a single document.
func (i *Indexer) IndexDocument(ctx context.Context, indexName string, docID string, document interface{}) error {
	body, err := json.Marshal(document)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeSerialization, "failed to marshal document")
	}

	req := opensearchapi.IndexRequest{
		Index:      indexName,
		DocumentID: docID,
		Body:       bytes.NewReader(body),
		Refresh:    i.config.RefreshPolicy,
	}

	resp, err := req.Do(ctx, i.client.GetClient())
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to index document request")
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return i.handleErrorResponse(resp, ErrDocumentIndexFailed)
	}

	return nil
}

// BulkIndex indexes multiple documents in batches.
func (i *Indexer) BulkIndex(ctx context.Context, indexName string, documents map[string]interface{}) (*common.BulkResult, error) {
	result := &common.BulkResult{}
	if len(documents) == 0 {
		return result, nil
	}

	docIDs := make([]string, 0, len(documents))
	for id := range documents {
		docIDs = append(docIDs, id)
	}

	batchSize := i.config.BulkBatchSize
	totalDocs := len(docIDs)

	for start := 0; start < totalDocs; start += batchSize {
		end := start + batchSize
		if end > totalDocs {
			end = totalDocs
		}

		batchIDs := docIDs[start:end]
		var buf bytes.Buffer

		for _, id := range batchIDs {
			doc := documents[id]

			// Action and metadata
			meta := fmt.Sprintf(`{"index":{"_index":"%s","_id":"%s"}}`, indexName, id)
			buf.WriteString(meta + "\n")

			// Source
			docBytes, err := json.Marshal(doc)
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, common.BulkItemError{
					DocID:     id,
					ErrorType: "serialization_error",
					Reason:    err.Error(),
				})
				continue
			}
			buf.Write(docBytes)
			buf.WriteString("\n")
		}

		if buf.Len() == 0 {
			continue
		}

		req := opensearchapi.BulkRequest{
			Body:    bytes.NewReader(buf.Bytes()),
			Refresh: i.config.RefreshPolicy,
		}

		resp, err := req.Do(ctx, i.client.GetClient())
		if err != nil {
			// Network error counts as failure for the whole batch
			return result, errors.Wrap(err, errors.ErrCodeInternal, "bulk request failed")
		}
		defer resp.Body.Close()

		if resp.IsError() {
			result.Failed += len(batchIDs)
			err = i.handleErrorResponse(resp, errors.New(errors.ErrCodeInternal, "bulk batch failed"))
			result.Errors = append(result.Errors, common.BulkItemError{
				DocID:     "batch_error",
				ErrorType: "http_error",
				Reason:    err.Error(),
			})
			continue
		}

		// Parse bulk response to find individual failures
		var bulkResp struct {
			Errors bool `json:"errors"`
			Items  []map[string]struct {
				Index  string `json:"_index"`
				ID     string `json:"_id"`
				Status int    `json:"status"`
				Error  struct {
					Type   string `json:"type"`
					Reason string `json:"reason"`
				} `json:"error,omitempty"`
			} `json:"items"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&bulkResp); err != nil {
			return result, errors.Wrap(err, errors.ErrCodeSerialization, "failed to decode bulk response")
		}

		if !bulkResp.Errors {
			result.Succeeded += len(bulkResp.Items)
		} else {
			for _, item := range bulkResp.Items {
				// Each item is a map with key usually "index", "create", "update", "delete"
				var info struct {
					ID     string `json:"_id"`
					Status int    `json:"status"`
					Error  struct {
						Type   string `json:"type"`
						Reason string `json:"reason"`
					} `json:"error,omitempty"`
				}
				// Extract the inner object (index/create/update/delete)
				for _, v := range item {
					info.ID = v.ID
					info.Status = v.Status
					info.Error = v.Error
					break
				}

				if info.Status >= 200 && info.Status < 300 {
					result.Succeeded++
				} else {
					result.Failed++
					result.Errors = append(result.Errors, common.BulkItemError{
						DocID:     info.ID,
						ErrorType: info.Error.Type,
						Reason:    info.Error.Reason,
					})
				}
			}
		}
	}

	i.logger.Info("Bulk index completed",
		logging.Int("total", totalDocs),
		logging.Int("succeeded", result.Succeeded),
		logging.Int("failed", result.Failed))

	return result, nil
}

// DeleteDocument deletes a document.
func (i *Indexer) DeleteDocument(ctx context.Context, indexName string, docID string) error {
	req := opensearchapi.DeleteRequest{
		Index:      indexName,
		DocumentID: docID,
		Refresh:    i.config.RefreshPolicy,
	}

	resp, err := req.Do(ctx, i.client.GetClient())
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to delete document request")
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return ErrDocumentNotFound
	}

	if resp.IsError() {
		return i.handleErrorResponse(resp, errors.New(errors.ErrCodeInternal, "delete document failed"))
	}

	return nil
}

// UpdateMapping updates the index mapping.
func (i *Indexer) UpdateMapping(ctx context.Context, indexName string, mapping map[string]interface{}) error {
	body, err := json.Marshal(mapping)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeSerialization, "failed to marshal mapping")
	}

	req := opensearchapi.IndicesPutMappingRequest{
		Index: []string{indexName},
		Body:  bytes.NewReader(body),
	}

	resp, err := req.Do(ctx, i.client.GetClient())
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to update mapping request")
	}
	defer resp.Body.Close()

	if resp.StatusCode == 400 || resp.StatusCode == 409 {
		return i.handleErrorResponse(resp, ErrMappingConflict)
	}

	if resp.IsError() {
		return i.handleErrorResponse(resp, errors.New(errors.ErrCodeInternal, "update mapping failed"))
	}

	return nil
}

func (i *Indexer) handleErrorResponse(resp *opensearchapi.Response, defaultErr error) error {
	var errResp struct {
		Error struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error"`
	}
	// Try to decode error
	bodyBytes, _ := io.ReadAll(resp.Body)
	// Reset body for potential re-read? No, we consumed it.

	if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error.Reason != "" {
		return errors.Wrapf(defaultErr, errors.ErrCodeInternal, "OpenSearch error: %s - %s", errResp.Error.Type, errResp.Error.Reason)
	}

	return errors.Wrapf(defaultErr, errors.ErrCodeInternal, "OpenSearch error status: %d", resp.StatusCode)
}

// Predefined Mappings

func PatentIndexMapping() common.IndexMapping {
	return common.IndexMapping{
		Settings: map[string]interface{}{
			"number_of_shards": 3,
			"number_of_replicas": 1,
			"analysis": map[string]interface{}{
				"analyzer": map[string]interface{}{
					"ik_max_word": map[string]interface{}{
						"type": "custom",
						"tokenizer": "ik_max_word", // Assuming IK plugin installed
					},
				},
			},
		},
		Mappings: map[string]interface{}{
			"properties": map[string]interface{}{
				"patent_number": map[string]interface{}{"type": "keyword"},
				"title": map[string]interface{}{
					"type": "text",
					"analyzer": "ik_max_word",
					"search_analyzer": "ik_max_word",
				},
				"abstract": map[string]interface{}{
					"type": "text",
					"analyzer": "ik_max_word",
					"search_analyzer": "ik_max_word",
				},
				"claims": map[string]interface{}{"type": "text"}, // Usually long text, maybe also analyzed
				"assignee": map[string]interface{}{"type": "keyword"},
				"inventors": map[string]interface{}{"type": "keyword"},
				"filing_date": map[string]interface{}{"type": "date"},
				"ipc_codes": map[string]interface{}{"type": "keyword"},
				"legal_status": map[string]interface{}{"type": "keyword"},
				"full_text": map[string]interface{}{"type": "text"},
				"tech_domain": map[string]interface{}{"type": "keyword"},
			},
		},
	}
}

func MoleculeIndexMapping() common.IndexMapping {
	return common.IndexMapping{
		Settings: map[string]interface{}{
			"number_of_shards": 3,
			"number_of_replicas": 1,
		},
		Mappings: map[string]interface{}{
			"properties": map[string]interface{}{
				"smiles": map[string]interface{}{"type": "keyword"},
				"inchi": map[string]interface{}{"type": "keyword"},
				"inchi_key": map[string]interface{}{"type": "keyword"},
				"molecular_formula": map[string]interface{}{"type": "keyword"},
				"molecular_weight": map[string]interface{}{"type": "float"},
				"name": map[string]interface{}{"type": "text"},
				"synonyms": map[string]interface{}{"type": "text"},
				"source_patents": map[string]interface{}{"type": "keyword"},
			},
		},
	}
}
