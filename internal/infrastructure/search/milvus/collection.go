package milvus

import (
	"context"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
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
func (m *CollectionManager) CreateCollection(ctx context.Context, schema common.CollectionSchema) error {
	has, err := m.HasCollection(ctx, schema.Name)
	if err != nil {
		return err
	}
	if has {
		return ErrCollectionAlreadyExists
	}

	// Convert common.CollectionSchema to entity.Schema
	fields := make([]*entity.Field, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		if field, ok := f.(*entity.Field); ok {
			fields = append(fields, field)
		} else {
			return errors.New(errors.ErrCodeValidation, "invalid field type in schema")
		}
	}

	s := &entity.Schema{
		CollectionName:     schema.Name,
		Description:        schema.Description,
		Fields:             fields,
		EnableDynamicField: schema.EnableDynamicField,
	}

	err = m.client.GetMilvusClient().CreateCollection(ctx, s, m.config.ShardsNum)
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
// Matches common? No, common doesn't define CollectionInfo, it defines CollectionSchema.
// But we can return common.CollectionSchema?
// DescribeCollection in interface? No, interface doesn't have DescribeCollection.
// Wait, interface in common.VectorStore does NOT have DescribeCollection.
// It has: Create, Drop, Has, CreateIndex, DropIndex, Load, Release, GetLoadState, Ensure.
// So DescribeCollection is extra. I can keep it or remove it. I'll keep it for internal use or extended interface.
type CollectionInfo struct {
	Name             string
	Description      string
	Fields           []*entity.Field
	ShardsNum        int32
	ConsistencyLevel entity.ConsistencyLevel
	RowCount         int64
	CreatedTimestamp uint64
}

// DescribeCollection returns collection details.
func (m *CollectionManager) DescribeCollection(ctx context.Context, name string) (*CollectionInfo, error) {
	coll, err := m.client.GetMilvusClient().DescribeCollection(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to describe collection")
	}

	// stats, err := m.client.GetMilvusClient().GetCollectionStatistics(ctx, name)
	// var rowCount int64
	// parse stats...

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
		ConsistencyLevel: coll.ConsistencyLevel,
		RowCount:         0, // Placeholder
		CreatedTimestamp: 0,
	}, nil
}

// CreateIndex creates an index for a field.
func (m *CollectionManager) CreateIndex(ctx context.Context, collectionName string, indexCfg common.IndexConfig) error {
	var idx entity.Index
	var err error

	metricType := entity.MetricType(indexCfg.MetricType)
	if metricType == "" {
		metricType = m.config.DefaultMetricType
	}

	// Simple mapping for demo
	// In real world, use indexCfg.IndexType string to decide
	if indexCfg.IndexType == "" || indexCfg.IndexType == "IVF_FLAT" {
		idx, err = entity.NewIndexIvfFlat(metricType, 1024)
	} else if indexCfg.IndexType == "HNSW" {
		idx, err = entity.NewIndexHNSW(metricType, 8, 200)
	} else {
		// Default
		idx, err = entity.NewIndexIvfFlat(metricType, 1024)
	}

	if err != nil {
		return err
	}

	err = m.client.GetMilvusClient().CreateIndex(ctx, collectionName, indexCfg.FieldName, idx, false)
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
func (m *CollectionManager) EnsureCollection(ctx context.Context, schema common.CollectionSchema, indexConfigs []common.IndexConfig) error {
	exists, err := m.HasCollection(ctx, schema.Name)
	if err != nil {
		return err
	}

	if !exists {
		if err := m.CreateCollection(ctx, schema); err != nil {
			return err
		}
	}

	for _, idxCfg := range indexConfigs {
		if err := m.CreateIndex(ctx, schema.Name, idxCfg); err != nil {
			m.logger.Warn("CreateIndex failed (might exist)", logging.Error(err))
		}
	}

	if err := m.LoadCollection(ctx, schema.Name); err != nil {
		return err
	}

	return nil
}

// Predefined Schemas

func PatentVectorSchema() common.CollectionSchema {
	fields := []*entity.Field{
		{Name: "id", DataType: entity.FieldTypeInt64, PrimaryKey: true, AutoID: false},
		{Name: "patent_number", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "64"}},
		{Name: "title_vector", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": "768"}},
		{Name: "abstract_vector", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": "768"}},
		{Name: "claims_vector", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": "768"}},
		{Name: "tech_domain", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "128"}, IsPartitionKey: true},
		{Name: "filing_date", DataType: entity.FieldTypeInt64},
		{Name: "assignee", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "256"}},
	}
	ifaces := make([]interface{}, len(fields))
	for i, f := range fields {
		ifaces[i] = f
	}
	return common.CollectionSchema{
		Name:        "patents",
		Description: "Patent vectors",
		Fields:      ifaces,
	}
}

func MoleculeVectorSchema() common.CollectionSchema {
	fields := []*entity.Field{
		{Name: "id", DataType: entity.FieldTypeInt64, PrimaryKey: true, AutoID: false},
		{Name: "smiles", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "2048"}},
		{Name: "fingerprint_vector", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": "2048"}},
		{Name: "structure_vector", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": "512"}},
		{Name: "molecular_weight", DataType: entity.FieldTypeFloat},
		{Name: "source_patent", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "64"}},
	}
	ifaces := make([]interface{}, len(fields))
	for i, f := range fields {
		ifaces[i] = f
	}
	return common.CollectionSchema{
		Name:        "molecules",
		Description: "Molecule vectors",
		Fields:      ifaces,
	}
}
