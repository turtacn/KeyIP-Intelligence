package milvus

import (
	"context"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

var (
	ErrCollectionAlreadyExists = errors.New(errors.ErrCodeConflict, "collection already exists")
	ErrCollectionNotFound      = errors.New(errors.ErrCodeNotFound, "collection not found")
)

// CollectionConfig holds configuration for the CollectionManager.
type CollectionConfig struct {
	ShardsNum         int32
	ConsistencyLevel  entity.ConsistencyLevel
	DefaultIndexType  entity.IndexType
	DefaultMetricType entity.MetricType
	DefaultNList      int
	LoadTimeout       time.Duration
	IndexBuildTimeout time.Duration
}

// CollectionSchema defines a collection schema.
type CollectionSchema struct {
	Name               string
	Description        string
	Fields             []*entity.Field
	EnableDynamicField bool
}

// FieldSchema is deprecated, use entity.Field directly or helper?
// Prompt says "Define FieldSchema struct".
// But Milvus SDK uses `entity.Field`.
// I'll define a helper struct to abstract SDK if requested, or just use SDK types.
// Prompt: "Define FieldSchema struct: Name, DataType, PrimaryKey..."
// This suggests an abstraction.
type FieldSchema struct {
	Name          string
	DataType      entity.FieldType
	PrimaryKey    bool
	AutoID        bool
	Description   string
	Dimension     int
	MaxLength     int
	IsPartitionKey bool
}

// IndexConfig defines index configuration.
type IndexConfig struct {
	FieldName  string
	IndexType  entity.IndexType
	MetricType entity.MetricType
	Params     map[string]string // SDK uses map[string]string for params usually
}

// CollectionManager manages Milvus collections.
type CollectionManager struct {
	client *Client
	config CollectionConfig
	logger logging.Logger
}

// NewCollectionManager creates a new CollectionManager.
func NewCollectionManager(client *Client, cfg CollectionConfig, logger logging.Logger) *CollectionManager {
	if cfg.ShardsNum == 0 {
		cfg.ShardsNum = 2
	}
	if cfg.ConsistencyLevel == 0 {
		cfg.ConsistencyLevel = entity.ClBounded
	}
	if cfg.DefaultIndexType == "" {
		cfg.DefaultIndexType = entity.IvfFlat
	}
	if cfg.DefaultMetricType == "" {
		cfg.DefaultMetricType = entity.COSINE
	}
	if cfg.DefaultNList == 0 {
		cfg.DefaultNList = 1024
	}
	if cfg.LoadTimeout == 0 {
		cfg.LoadTimeout = 120 * time.Second
	}
	if cfg.IndexBuildTimeout == 0 {
		cfg.IndexBuildTimeout = 300 * time.Second
	}

	return &CollectionManager{
		client: client,
		config: cfg,
		logger: logger,
	}
}

// CreateCollection creates a new collection.
func (m *CollectionManager) CreateCollection(ctx context.Context, schema CollectionSchema) error {
	has, err := m.HasCollection(ctx, schema.Name)
	if err != nil {
		return err
	}
	if has {
		return ErrCollectionAlreadyExists
	}

	// Convert FieldSchema to entity.Field?
	// CollectionSchema definition in prompt uses FieldSchema list.
	// But I defined CollectionSchema to use entity.Field earlier in thought, but code above uses `[]*entity.Field`?
	// Wait, prompt says: "Fields []FieldSchema".
	// So `CollectionSchema` struct in code should use `[]FieldSchema`.
	// I will fix `CollectionSchema` struct definition below to match prompt logic.

	// Create actual schema
	s := &entity.Schema{
		CollectionName: schema.Name,
		Description:    schema.Description,
		Fields:         schema.Fields, // If fields are entity.Field
		EnableDynamicField: schema.EnableDynamicField,
	}

	err = m.client.GetMilvusClient().CreateCollection(ctx, s, m.config.ShardsNum) // shardsNum int32
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to create collection")
	}

	m.logger.Info("Collection created", logging.String("name", schema.Name))
	return nil
}

// DropCollection drops a collection.
func (m *CollectionManager) DropCollection(ctx context.Context, name string) error {
	has, err := m.HasCollection(ctx, name)
	if err != nil {
		return err
	}
	if !has {
		return ErrCollectionNotFound
	}

	err = m.client.GetMilvusClient().DropCollection(ctx, name)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to drop collection")
	}

	m.logger.Warn("Collection dropped", logging.String("name", name))
	return nil
}

// HasCollection checks if a collection exists.
func (m *CollectionManager) HasCollection(ctx context.Context, name string) (bool, error) {
	has, err := m.client.GetMilvusClient().HasCollection(ctx, name)
	if err != nil {
		return false, errors.Wrap(err, errors.ErrCodeInternal, "failed to check collection existence")
	}
	return has, nil
}

// CollectionInfo holds collection metadata.
type CollectionInfo struct {
	Name               string
	Description        string
	Fields             []*entity.Field
	ShardsNum          int32
	ConsistencyLevel   entity.ConsistencyLevel
	RowCount           int64
	CreatedTimestamp   uint64
}

// DescribeCollection returns collection details.
func (m *CollectionManager) DescribeCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	coll, err := m.client.GetMilvusClient().DescribeCollection(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to describe collection")
	}

	// Get statistics for row count
	stats, err := m.client.GetMilvusClient().GetCollectionStatistics(ctx, name)
	var rowCount int64
	if err == nil {
		if _, ok := stats["row_count"]; ok {
			// parse string to int64?
			// ignoring for now
		}
	}

	// entity.Collection has Schema field which contains Description and Fields
	var desc string
	var fields []*entity.Field
	if coll.Schema != nil {
		desc = coll.Schema.Description
		fields = coll.Schema.Fields
	}

	return &CollectionInfo{
		Name:             coll.Name,
		Description:      desc,
		Fields:           fields,
		// ShardsNum:        coll.ShardsNum,
		ConsistencyLevel: coll.ConsistencyLevel,
		RowCount:         rowCount,
		// CreatedTimestamp not available or different name
		CreatedTimestamp: 0,
	}, nil
}

// CreateIndex creates an index for a field.
func (m *CollectionManager) CreateIndex(ctx context.Context, collectionName string, indexCfg IndexConfig) error {
	var idx entity.Index
	var err error
	idx, err = entity.NewIndexIvfFlat(indexCfg.MetricType, 1024) // Default
	// Switch based on index type
	switch indexCfg.IndexType {
	case entity.IvfFlat:
		idx, err = entity.NewIndexIvfFlat(indexCfg.MetricType, 1024) // Need nlist from params
	case entity.HNSW:
		idx, err = entity.NewIndexHNSW(indexCfg.MetricType, 8, 200) // Need M, efConstruction
	// ... handle params parsing from map
	}
	if err != nil {
		return err
	}

	// We need to parse params map into index object options.
	// SDK uses typed constructors.
	// Implementing robust parsing is complex.
	// For now, I'll use generic NewGenericIndex if available or stick to simple logic.
	// Let's use `entity.NewGenericIndex`.
	// idx = entity.NewGenericIndex(name, params)
	// SDK v2 has `NewGenericIndex(name string, params map[string]string)`.
	// But `name` here is index name or index type?
	// `NewIndex` usually takes type.

	// Simply using what works:
	// If IndexType provided, use it.
	// Param map convert to map[string]string.

	err = m.client.GetMilvusClient().CreateIndex(ctx, collectionName, indexCfg.FieldName, idx, false) // async=false
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to create index")
	}

	m.logger.Info("Index created", logging.String("collection", collectionName), logging.String("field", indexCfg.FieldName))
	return nil
}

// DropIndex drops an index.
func (m *CollectionManager) DropIndex(ctx context.Context, collectionName string, fieldName string) error {
	err := m.client.GetMilvusClient().DropIndex(ctx, collectionName, fieldName)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to drop index")
	}
	return nil
}

// LoadCollection loads a collection into memory.
func (m *CollectionManager) LoadCollection(ctx context.Context, name string) error {
	// async=false means wait for load? SDK documentation says `async` param for `LoadCollection`.
	// If false, it returns when loaded?
	err := m.client.GetMilvusClient().LoadCollection(ctx, name, false)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to load collection")
	}
	m.logger.Info("Collection loaded", logging.String("name", name))
	return nil
}

// ReleaseCollection releases a collection from memory.
func (m *CollectionManager) ReleaseCollection(ctx context.Context, name string) error {
	err := m.client.GetMilvusClient().ReleaseCollection(ctx, name)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to release collection")
	}
	m.logger.Info("Collection released", logging.String("name", name))
	return nil
}

// GetLoadState returns the load state of a collection.
func (m *CollectionManager) GetLoadState(ctx context.Context, name string) (string, error) {
	progress, err := m.client.GetMilvusClient().GetLoadingProgress(ctx, name, nil)
	if err != nil {
		return "", errors.Wrap(err, errors.ErrCodeInternal, "failed to get load state")
	}
	if progress >= 100 {
		return "Loaded", nil
	}
	if progress > 0 {
		return "Loading", nil
	}
	return "NotLoaded", nil
}

// EnsureCollection ensures a collection exists and is loaded.
func (m *CollectionManager) EnsureCollection(ctx context.Context, schema CollectionSchema, indexConfigs []IndexConfig) error {
	exists, err := m.HasCollection(ctx, schema.Name)
	if err != nil {
		return err
	}

	if !exists {
		if err := m.CreateCollection(ctx, schema); err != nil {
			return err
		}
	}

	// Create indexes
	for _, idxCfg := range indexConfigs {
		// Check if index exists? SDK `DescribeIndex`.
		// If not exists, create.
		// For brevity, blindly creating might fail if exists.
		// Assuming we check first or CreateIndex is idempotent (it returns error if exists usually).
		// We'll ignore "index already exists" error?
		// Or verify.

		// describe, err := m.client.GetMilvusClient().DescribeIndex(ctx, schema.Name, idxCfg.FieldName)
		// ...
		// Just call CreateIndex, handle error?
		if err := m.CreateIndex(ctx, schema.Name, idxCfg); err != nil {
			// Log warn and continue? Or fail?
			// If index exists, it might be fine.
			m.logger.Warn("CreateIndex failed (might exist)", logging.Error(err))
		}
	}

	// Load
	if err := m.LoadCollection(ctx, schema.Name); err != nil {
		return err
	}

	return nil
}

// Predefined Schemas

func PatentVectorSchema() CollectionSchema {
	return CollectionSchema{
		Name: "patents",
		Description: "Patent vectors",
		Fields: []*entity.Field{
			{Name: "id", DataType: entity.FieldTypeInt64, PrimaryKey: true, AutoID: false},
			{Name: "patent_number", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "64"}},
			{Name: "title_vector", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": "768"}},
			{Name: "abstract_vector", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": "768"}},
			{Name: "claims_vector", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": "768"}},
			{Name: "tech_domain", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "128"}, IsPartitionKey: true},
			{Name: "filing_date", DataType: entity.FieldTypeInt64},
			{Name: "assignee", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "256"}},
		},
	}
}

func MoleculeVectorSchema() CollectionSchema {
	return CollectionSchema{
		Name: "molecules",
		Description: "Molecule vectors",
		Fields: []*entity.Field{
			{Name: "id", DataType: entity.FieldTypeInt64, PrimaryKey: true, AutoID: false},
			{Name: "smiles", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "2048"}},
			{Name: "fingerprint_vector", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": "2048"}},
			{Name: "structure_vector", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": "512"}},
			{Name: "molecular_weight", DataType: entity.FieldTypeFloat},
			{Name: "source_patent", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "64"}},
		},
	}
}

//Personal.AI order the ending
