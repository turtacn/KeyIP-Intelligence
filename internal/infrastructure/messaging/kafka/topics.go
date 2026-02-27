package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// Topic Constants
const (
	TopicPatentIngested         = "patent.ingested"
	TopicPatentAnalyzed         = "patent.analyzed"
	TopicPatentUpdated          = "patent.updated"
	TopicPatentDeleted          = "patent.deleted"
	TopicPatentVectorized       = "patent.vectorized"
	TopicMoleculeIngested       = "molecule.ingested"
	TopicMoleculeAnalyzed       = "molecule.analyzed"
	TopicMoleculeVectorized     = "molecule.vectorized"
	TopicInfringementDetected   = "infringement.detected"
	TopicInfringementResolved   = "infringement.resolved"
	TopicInfringementEscalated  = "infringement.escalated"
	TopicCompetitiveIntelUpdate = "competitive_intel.update"
	TopicCompetitiveIntelAlert  = "competitive_intel.alert"
	TopicUserAction             = "user.action"
	TopicNotification           = "notification.send"
	TopicAuditLog               = "audit.log"
	TopicDeadLetterDefault      = "dead_letter.default"
	TopicDeadLetterPatent       = "dead_letter.patent"
	TopicDeadLetterInfringement = "dead_letter.infringement"
)

// EventEnvelope standardizes event messages.
type EventEnvelope struct {
	EventID       string            `json:"event_id"`
	EventType     string            `json:"event_type"`
	Source        string            `json:"source"`
	Timestamp     time.Time         `json:"timestamp"`
	SchemaVersion string            `json:"schema_version"`
	TraceID       string            `json:"trace_id,omitempty"`
	Payload       json.RawMessage   `json:"payload"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// Payload structs
type PatentIngestedPayload struct {
	PatentID     string    `json:"patent_id"`
	PatentNumber string    `json:"patent_number"`
	Title        string    `json:"title"`
	Source       string    `json:"source"`
	IngestedAt   time.Time `json:"ingested_at"`
}

type PatentAnalyzedPayload struct {
	PatentID      string    `json:"patent_id"`
	PatentNumber  string    `json:"patent_number"`
	AnalysisType  string    `json:"analysis_type"`
	ResultSummary string    `json:"result_summary"`
	AnalyzedAt    time.Time `json:"analyzed_at"`
}

type PatentVectorizedPayload struct {
	PatentID     string    `json:"patent_id"`
	PatentNumber string    `json:"patent_number"`
	VectorFields []string  `json:"vector_fields"`
	ModelName    string    `json:"model_name"`
	ModelVersion string    `json:"model_version"`
	VectorizedAt time.Time `json:"vectorized_at"`
}

type InfringementDetectedPayload struct {
	InfringementID  string    `json:"infringement_id"`
	SourcePatentID  string    `json:"source_patent_id"`
	TargetPatentID  string    `json:"target_patent_id"`
	SimilarityScore float64   `json:"similarity_score"`
	RiskLevel       string    `json:"risk_level"`
	DetectedAt      time.Time `json:"detected_at"`
}

type CompetitiveIntelUpdatePayload struct {
	CompetitorID   string    `json:"competitor_id"`
	CompetitorName string    `json:"competitor_name"`
	UpdateType     string    `json:"update_type"`
	Summary        string    `json:"summary"`
	DetectedAt     time.Time `json:"detected_at"`
}

type NotificationPayload struct {
	RecipientID string `json:"recipient_id"`
	Channel     string `json:"channel"`
	Subject     string `json:"subject"`
	Body        string `json:"body"`
	Priority    string `json:"priority"`
}

// Helper functions for EventEnvelope

func NewEventEnvelope(eventType string, source string, payload interface{}) (*EventEnvelope, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeSerialization, "failed to marshal payload")
	}
	return &EventEnvelope{
		EventID:       uuid.New().String(),
		EventType:     eventType,
		Source:        source,
		Timestamp:     time.Now().UTC(),
		SchemaVersion: "v1",
		Payload:       data,
	}, nil
}

func (e *EventEnvelope) DecodePayload(target interface{}) error {
	if len(e.Payload) == 0 || string(e.Payload) == "null" {
		return nil // or error if payload required?
	}
	return json.Unmarshal(e.Payload, target)
}

func (e *EventEnvelope) ToMessage(topic string) (*common.ProducerMessage, error) {
	val, err := json.Marshal(e)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeSerialization, "failed to marshal envelope")
	}
	headers := map[string]string{
		"event_type":     e.EventType,
		"source_service": e.Source,
		"schema_version": e.SchemaVersion,
	}
	if e.TraceID != "" {
		headers["trace_id"] = e.TraceID
	}
	return &common.ProducerMessage{
		Topic:     topic,
		Value:     val,
		Headers:   headers,
		Timestamp: e.Timestamp,
	}, nil
}

func MessageToEventEnvelope(msg *common.Message) (*EventEnvelope, error) {
	if len(msg.Value) == 0 {
		return nil, errors.New(errors.ErrCodeValidation, "empty message value")
	}
	var env EventEnvelope
	if err := json.Unmarshal(msg.Value, &env); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeSerialization, "failed to unmarshal envelope")
	}
	return &env, nil
}

// ConnInterface abstracts kafka.Conn for testing.
type ConnInterface interface {
	CreateTopics(topics ...kafka.TopicConfig) error
	DeleteTopics(topics ...string) error
	ReadPartitions(topics ...string) ([]kafka.Partition, error)
	Close() error
}

// TopicManager manages Kafka topics.
type TopicManager struct {
	conn   ConnInterface
	logger logging.Logger
}

func NewTopicManager(brokers []string, logger logging.Logger) (*TopicManager, error) {
	if len(brokers) == 0 {
		return nil, errors.New(errors.ErrCodeValidation, "brokers required")
	}
	// Connect to first broker (controller or any)
	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to dial kafka")
	}
	return &TopicManager{
		conn:   conn,
		logger: logger,
	}, nil
}

func (m *TopicManager) CreateTopic(ctx context.Context, cfg common.TopicConfig) error {
	if cfg.Name == "" {
		return errors.New(errors.ErrCodeValidation, "topic name required")
	}
	if cfg.NumPartitions <= 0 {
		return errors.New(errors.ErrCodeValidation, "NumPartitions must be > 0")
	}
	if cfg.ReplicationFactor <= 0 {
		return errors.New(errors.ErrCodeValidation, "ReplicationFactor must be > 0")
	}

	kCfg := kafka.TopicConfig{
		Topic:             cfg.Name,
		NumPartitions:     cfg.NumPartitions,
		ReplicationFactor: cfg.ReplicationFactor,
		ConfigEntries:     make([]kafka.ConfigEntry, 0),
	}

	if cfg.RetentionMs > 0 {
		kCfg.ConfigEntries = append(kCfg.ConfigEntries, kafka.ConfigEntry{ConfigName: "retention.ms", ConfigValue: fmt.Sprintf("%d", cfg.RetentionMs)})
	}
	if cfg.CleanupPolicy != "" {
		kCfg.ConfigEntries = append(kCfg.ConfigEntries, kafka.ConfigEntry{ConfigName: "cleanup.policy", ConfigValue: cfg.CleanupPolicy})
	}
	if cfg.MaxMessageBytes > 0 {
		kCfg.ConfigEntries = append(kCfg.ConfigEntries, kafka.ConfigEntry{ConfigName: "max.message.bytes", ConfigValue: fmt.Sprintf("%d", cfg.MaxMessageBytes)})
	}
	for k, v := range cfg.Configs {
		kCfg.ConfigEntries = append(kCfg.ConfigEntries, kafka.ConfigEntry{ConfigName: k, ConfigValue: v})
	}

	err := m.conn.CreateTopics(kCfg)
	if err != nil {
		if err.Error() == "topic already exists" {
			return nil
		}
		exists, _ := m.TopicExists(ctx, cfg.Name)
		if exists {
			return nil
		}
		return err
	}
	m.logger.Info("Topic created", logging.String("topic", cfg.Name))
	return nil
}

func (m *TopicManager) DeleteTopic(ctx context.Context, name string) error {
	err := m.conn.DeleteTopics(name)
	if err != nil {
		return nil
	}
	m.logger.Warn("Topic deleted", logging.String("topic", name))
	return nil
}

func (m *TopicManager) TopicExists(ctx context.Context, name string) (bool, error) {
	partitions, err := m.conn.ReadPartitions(name)
	if err != nil {
		return false, nil
	}
	return len(partitions) > 0, nil
}

func (m *TopicManager) ListTopics(ctx context.Context) ([]string, error) {
	partitions, err := m.conn.ReadPartitions()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var topics []string
	for _, p := range partitions {
		if !seen[p.Topic] {
			seen[p.Topic] = true
			topics = append(topics, p.Topic)
		}
	}
	return topics, nil
}

func (m *TopicManager) EnsureTopics(ctx context.Context, topics []common.TopicConfig) error {
	for _, topic := range topics {
		if err := m.CreateTopic(ctx, topic); err != nil {
			return err
		}
	}
	return nil
}

func (m *TopicManager) EnsureDefaultTopics(ctx context.Context) error {
	return m.EnsureTopics(ctx, DefaultTopics())
}

func (m *TopicManager) Close() error {
	return m.conn.Close()
}

func DefaultTopics() []common.TopicConfig {
	return []common.TopicConfig{
		{Name: TopicPatentIngested, NumPartitions: 12, ReplicationFactor: 3, RetentionMs: 7 * 24 * 3600 * 1000},
		{Name: TopicPatentAnalyzed, NumPartitions: 6, ReplicationFactor: 3, RetentionMs: 7 * 24 * 3600 * 1000},
		{Name: TopicPatentUpdated, NumPartitions: 6, ReplicationFactor: 3, RetentionMs: 7 * 24 * 3600 * 1000},
		{Name: TopicPatentDeleted, NumPartitions: 3, ReplicationFactor: 3, RetentionMs: 30 * 24 * 3600 * 1000},
		{Name: TopicPatentVectorized, NumPartitions: 6, ReplicationFactor: 3, RetentionMs: 3 * 24 * 3600 * 1000},
		{Name: TopicMoleculeIngested, NumPartitions: 6, ReplicationFactor: 3, RetentionMs: 7 * 24 * 3600 * 1000},
		{Name: TopicMoleculeAnalyzed, NumPartitions: 6, ReplicationFactor: 3, RetentionMs: 7 * 24 * 3600 * 1000},
		{Name: TopicMoleculeVectorized, NumPartitions: 6, ReplicationFactor: 3, RetentionMs: 3 * 24 * 3600 * 1000},
		{Name: TopicInfringementDetected, NumPartitions: 6, ReplicationFactor: 3, RetentionMs: 90 * 24 * 3600 * 1000},
		{Name: TopicInfringementResolved, NumPartitions: 3, ReplicationFactor: 3, RetentionMs: 90 * 24 * 3600 * 1000},
		{Name: TopicInfringementEscalated, NumPartitions: 3, ReplicationFactor: 3, RetentionMs: 90 * 24 * 3600 * 1000},
		{Name: TopicCompetitiveIntelUpdate, NumPartitions: 6, ReplicationFactor: 3, RetentionMs: 30 * 24 * 3600 * 1000},
		{Name: TopicCompetitiveIntelAlert, NumPartitions: 3, ReplicationFactor: 3, RetentionMs: 30 * 24 * 3600 * 1000},
		{Name: TopicUserAction, NumPartitions: 6, ReplicationFactor: 3, RetentionMs: 30 * 24 * 3600 * 1000},
		{Name: TopicNotification, NumPartitions: 6, ReplicationFactor: 3, RetentionMs: 3 * 24 * 3600 * 1000},
		{Name: TopicAuditLog, NumPartitions: 3, ReplicationFactor: 3, RetentionMs: 365 * 24 * 3600 * 1000},
		{Name: TopicDeadLetterDefault, NumPartitions: 3, ReplicationFactor: 3, RetentionMs: 30 * 24 * 3600 * 1000},
		{Name: TopicDeadLetterPatent, NumPartitions: 3, ReplicationFactor: 3, RetentionMs: 30 * 24 * 3600 * 1000},
		{Name: TopicDeadLetterInfringement, NumPartitions: 3, ReplicationFactor: 3, RetentionMs: 30 * 24 * 3600 * 1000},
	}
}
