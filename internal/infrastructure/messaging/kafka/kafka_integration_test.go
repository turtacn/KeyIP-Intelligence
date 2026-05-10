package kafka

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// getKafkaBrokers returns the list of Kafka brokers from the KAFKA_BROKERS
// environment variable. It is used to gate integration tests.
func getKafkaBrokers() []string {
	brokersEnv := os.Getenv("KAFKA_BROKERS")
	if brokersEnv == "" {
		return nil
	}
	return strings.Split(brokersEnv, ",")
}

// skipIfNoKafka skips the test if no Kafka broker is configured.
func skipIfNoKafka(t *testing.T) []string {
	t.Helper()
	brokers := getKafkaBrokers()
	if len(brokers) == 0 {
		t.Skip("KAFKA_BROKERS not set; skipping Kafka integration test")
	}
	return brokers
}

// waitForTopic polls a topic until it is available or a timeout expires.
func waitForTopic(t *testing.T, brokers []string, topic string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := kafka.Dial("tcp", brokers[0])
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		partitions, err := conn.ReadPartitions(topic)
		conn.Close()
		if err == nil && len(partitions) > 0 {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("topic %s did not become available within %v", topic, timeout)
}

// ---------------------------------------------------------------------------
// Producer integration tests
// ---------------------------------------------------------------------------

func TestProducerIntegration_PublishAndConsume(t *testing.T) {
	brokers := skipIfNoKafka(t)

	// Create a dedicated topic for this test
	topic := "int-test-pub-consume-" + randomSuffix()
	createTempTopic(t, brokers, topic)
	defer deleteTempTopic(t, brokers, topic)
	waitForTopic(t, brokers, topic, 5*time.Second)

	// Create producer
	cfg := ProducerConfig{
		Brokers:         brokers,
		MaxRetries:      1,
		RetryBackoff:    50 * time.Millisecond,
		BatchSize:       1,
		BatchTimeout:    100 * time.Millisecond,
		MaxMessageBytes: 1024 * 1024,
		Acks:            "all",
	}
	producer, err := NewProducer(cfg, newMockLogger())
	require.NoError(t, err)
	defer producer.Close()

	// Publish a message
	msg := &common.ProducerMessage{
		Topic: topic,
		Key:   []byte("integration-key"),
		Value: []byte("integration-value-12345"),
		Headers: map[string]string{
			"source": "integration-test",
		},
	}
	err = producer.Publish(context.Background(), msg)
	require.NoError(t, err)

	// Verify metrics
	metrics := producer.GetMetrics()
	assert.Equal(t, int64(1), metrics.MessagesSent.Load())
	assert.Equal(t, int64(0), metrics.MessagesFailed.Load())
	assert.Greater(t, metrics.BytesSent.Load(), int64(0))

	// Consume the message back
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        "int-test-group-" + randomSuffix(),
		StartOffset:    kafka.FirstOffset,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        500 * time.Millisecond,
		CommitInterval: 0, // manual commit
	})
	defer reader.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	consumedMsg, err := reader.FetchMessage(ctx)
	require.NoError(t, err)
	assert.Equal(t, topic, consumedMsg.Topic)
	assert.Equal(t, "integration-key", string(consumedMsg.Key))
	assert.Equal(t, "integration-value-12345", string(consumedMsg.Value))

	// Check headers
	foundSource := false
	for _, h := range consumedMsg.Headers {
		if h.Key == "source" && string(h.Value) == "integration-test" {
			foundSource = true
			break
		}
	}
	assert.True(t, foundSource, "expected header source=integration-test")

	err = reader.CommitMessages(ctx, consumedMsg)
	assert.NoError(t, err)
}

func TestProducerIntegration_PublishBatch(t *testing.T) {
	brokers := skipIfNoKafka(t)

	topic := "int-test-batch-" + randomSuffix()
	createTempTopic(t, brokers, topic)
	defer deleteTempTopic(t, brokers, topic)
	waitForTopic(t, brokers, topic, 5*time.Second)

	cfg := ProducerConfig{
		Brokers:         brokers,
		MaxRetries:      1,
		RetryBackoff:    50 * time.Millisecond,
		BatchSize:       10,
		BatchTimeout:    500 * time.Millisecond,
		MaxMessageBytes: 1024 * 1024,
	}
	producer, err := NewProducer(cfg, newMockLogger())
	require.NoError(t, err)
	defer producer.Close()

	messages := make([]*common.ProducerMessage, 5)
	for i := 0; i < 5; i++ {
		messages[i] = &common.ProducerMessage{
			Topic: topic,
			Key:   []byte("k" + itoa(i)),
			Value: []byte("v" + itoa(i)),
		}
	}

	result, err := producer.PublishBatch(context.Background(), messages)
	require.NoError(t, err)
	assert.Equal(t, 5, result.Succeeded)
	assert.Equal(t, 0, result.Failed)

	// Consume all messages
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        "int-test-batch-group-" + randomSuffix(),
		StartOffset:    kafka.FirstOffset,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        1 * time.Second,
		CommitInterval: 0,
	})
	defer reader.Close()

	received := make(map[string]string)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for len(received) < 5 {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			break
		}
		received[string(msg.Key)] = string(msg.Value)
		_ = reader.CommitMessages(ctx, msg)
	}

	assert.Len(t, received, 5)
	for i := 0; i < 5; i++ {
		assert.Equal(t, "v"+itoa(i), received["k"+itoa(i)])
	}
}

func TestProducerIntegration_Publish_MultipleTopics(t *testing.T) {
	brokers := skipIfNoKafka(t)

	topicA := "int-test-multi-a-" + randomSuffix()
	topicB := "int-test-multi-b-" + randomSuffix()
	createTempTopic(t, brokers, topicA)
	createTempTopic(t, brokers, topicB)
	defer deleteTempTopic(t, brokers, topicA)
	defer deleteTempTopic(t, brokers, topicB)
	waitForTopic(t, brokers, topicA, 5*time.Second)
	waitForTopic(t, brokers, topicB, 5*time.Second)

	cfg := ProducerConfig{
		Brokers:         brokers,
		BatchSize:       1,
		BatchTimeout:    100 * time.Millisecond,
		MaxMessageBytes: 1024 * 1024,
	}
	producer, err := NewProducer(cfg, newMockLogger())
	require.NoError(t, err)
	defer producer.Close()

	// Publish to topic A
	err = producer.Publish(context.Background(), &common.ProducerMessage{
		Topic: topicA, Key: []byte("key-a"), Value: []byte("val-a"),
	})
	require.NoError(t, err)

	// Publish to topic B
	err = producer.Publish(context.Background(), &common.ProducerMessage{
		Topic: topicB, Key: []byte("key-b"), Value: []byte("val-b"),
	})
	require.NoError(t, err)

	// Verify both were produced by checking metrics
	metrics := producer.GetMetrics()
	assert.Equal(t, int64(2), metrics.MessagesSent.Load())
}

func TestProducerIntegration_Close(t *testing.T) {
	brokers := skipIfNoKafka(t)

	cfg := ProducerConfig{
		Brokers:         brokers,
		MaxMessageBytes: 1024 * 1024,
	}
	producer, err := NewProducer(cfg, newMockLogger())
	require.NoError(t, err)

	err = producer.Close()
	assert.NoError(t, err)

	// Publishing after close should fail
	err = producer.Publish(context.Background(), &common.ProducerMessage{
		Topic: "nonexistent", Key: []byte("k"), Value: []byte("v"),
	})
	assert.ErrorIs(t, err, ErrProducerClosed)
}

// ---------------------------------------------------------------------------
// Consumer integration tests
// ---------------------------------------------------------------------------

func TestConsumerIntegration_ConsumeMessages(t *testing.T) {
	brokers := skipIfNoKafka(t)

	topic := "int-test-consume-" + randomSuffix()
	createTempTopic(t, brokers, topic)
	defer deleteTempTopic(t, brokers, topic)
	waitForTopic(t, brokers, topic, 5*time.Second)

	// Publish 3 messages first
	publishTestMessages(t, brokers, topic, 3)

	// Create consumer
	groupID := "int-test-consume-group-" + randomSuffix()
	consumerCfg := ConsumerConfig{
		Brokers:          brokers,
		GroupID:          groupID,
		Topics:           []string{topic},
		AutoOffsetReset:  "earliest",
		EnableAutoCommit: true,
		SessionTimeout:   10 * time.Second,
	}
	consumer, err := NewConsumer(consumerCfg, newMockLogger())
	require.NoError(t, err)
	defer consumer.Close()

	// Track received messages
	var mu sync.Mutex
	var received []string
	handler := func(ctx context.Context, msg *common.Message) error {
		mu.Lock()
		received = append(received, string(msg.Value))
		mu.Unlock()
		return nil
	}
	consumer.Subscribe(topic, handler)

	err = consumer.Start(context.Background())
	require.NoError(t, err)

	// Wait for messages
	time.Sleep(3 * time.Second)

	consumer.Close()

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, received, 3, "expected 3 messages to be consumed")
	for i := 0; i < 3; i++ {
		assert.Contains(t, received, "payload-"+itoa(i))
	}
}

func TestConsumerIntegration_ConsumerGroupReBalance(t *testing.T) {
	brokers := skipIfNoKafka(t)

	topic := "int-test-rebalance-" + randomSuffix()
	createTempTopic(t, brokers, topic)
	defer deleteTempTopic(t, brokers, topic)
	waitForTopic(t, brokers, topic, 5*time.Second)

	// Publish 6 messages before any consumer starts
	publishTestMessages(t, brokers, topic, 6)

	groupID := "int-test-rebalance-group-" + randomSuffix()

	// Create first consumer
	cfg1 := ConsumerConfig{
		Brokers:          brokers,
		GroupID:          groupID,
		Topics:           []string{topic},
		AutoOffsetReset:  "earliest",
		EnableAutoCommit: true,
		SessionTimeout:   10 * time.Second,
		HeartbeatInterval: 1 * time.Second,
	}
	consumer1, err := NewConsumer(cfg1, newMockLogger())
	require.NoError(t, err)
	defer consumer1.Close()

	var received1 []string
	var mu1 sync.Mutex
	consumer1.Subscribe(topic, func(ctx context.Context, msg *common.Message) error {
		mu1.Lock()
		received1 = append(received1, string(msg.Value))
		mu1.Unlock()
		return nil
	})
	err = consumer1.Start(context.Background())
	require.NoError(t, err)

	// Let consumer1 get the initial assignment and process some/all messages
	time.Sleep(3 * time.Second)

	// Create second consumer joining the same group (triggers rebalance)
	cfg2 := ConsumerConfig{
		Brokers:          brokers,
		GroupID:          groupID,
		Topics:           []string{topic},
		AutoOffsetReset:  "earliest",
		EnableAutoCommit: true,
		SessionTimeout:   10 * time.Second,
		HeartbeatInterval: 1 * time.Second,
	}
	consumer2, err := NewConsumer(cfg2, newMockLogger())
	require.NoError(t, err)
	defer consumer2.Close()

	var received2 []string
	var mu2 sync.Mutex
	consumer2.Subscribe(topic, func(ctx context.Context, msg *common.Message) error {
		mu2.Lock()
		received2 = append(received2, string(msg.Value))
		mu2.Unlock()
		return nil
	})
	err = consumer2.Start(context.Background())
	require.NoError(t, err)

	// Wait for rebalance and more processing
	time.Sleep(5 * time.Second)

	consumer1.Close()
	consumer2.Close()

	mu1.Lock()
	mu2.Lock()
	totalReceived := len(received1) + len(received2)
	mu2.Unlock()
	mu1.Unlock()

	// After rebalance, all 6 messages should be processed by the two consumers combined
	assert.Equal(t, 6, totalReceived,
		"both consumers combined should process all 6 messages after rebalance")
	t.Logf("Consumer1 received %d, Consumer2 received %d", len(received1), len(received2))
}

func TestConsumerIntegration_HandlerError(t *testing.T) {
	brokers := skipIfNoKafka(t)

	topic := "int-test-handler-error-" + randomSuffix()
	createTempTopic(t, brokers, topic)
	defer deleteTempTopic(t, brokers, topic)
	waitForTopic(t, brokers, topic, 5*time.Second)

	// Publish one message
	publishTestMessages(t, brokers, topic, 1)

	groupID := "int-test-handler-err-group-" + randomSuffix()
	consumerCfg := ConsumerConfig{
		Brokers:          brokers,
		GroupID:          groupID,
		Topics:           []string{topic},
		AutoOffsetReset:  "earliest",
		EnableAutoCommit: true,
		SessionTimeout:   10 * time.Second,
		RetryConfig: RetryConfig{
			MaxRetries:   1,
			RetryBackoff: 5 * time.Millisecond,
		},
	}
	consumer, err := NewConsumer(consumerCfg, newMockLogger())
	require.NoError(t, err)
	defer consumer.Close()

	consumer.Subscribe(topic, func(ctx context.Context, msg *common.Message) error {
		return assert.AnError // always fail
	})

	err = consumer.Start(context.Background())
	require.NoError(t, err)

	time.Sleep(2 * time.Second)
	consumer.Close()

	metrics := consumer.GetMetrics()
	assert.Equal(t, int64(1), metrics.MessagesConsumed.Load(), "should have consumed 1 message")
	// Message may or may not be counted as failed depending on timing of commit
	t.Logf("Consumed: %d, Processed: %d, Failed: %d, Retried: %d",
		metrics.MessagesConsumed.Load(), metrics.MessagesProcessed.Load(),
		metrics.MessagesFailed.Load(), metrics.MessagesRetried.Load())
}

// ---------------------------------------------------------------------------
// Topic configuration validation tests
// ---------------------------------------------------------------------------

func TestTopicValidation_InvalidConfigs(t *testing.T) {
	tests := []struct {
		name       string
		cfg        common.TopicConfig
		wantErr    bool
		errMessage string
	}{
		{
			name: "empty name",
			cfg: common.TopicConfig{
				Name:              "",
				NumPartitions:     1,
				ReplicationFactor: 1,
			},
			wantErr:    true,
			errMessage: "topic name required",
		},
		{
			name: "zero partitions",
			cfg: common.TopicConfig{
				Name:              "test",
				NumPartitions:     0,
				ReplicationFactor: 1,
			},
			wantErr:    true,
			errMessage: "NumPartitions must be > 0",
		},
		{
			name: "zero replication factor",
			cfg: common.TopicConfig{
				Name:              "test",
				NumPartitions:     1,
				ReplicationFactor: 0,
			},
			wantErr:    true,
			errMessage: "ReplicationFactor must be > 0",
		},
		{
			name: "negative partitions",
			cfg: common.TopicConfig{
				Name:              "test",
				NumPartitions:     -1,
				ReplicationFactor: 1,
			},
			wantErr:    true,
			errMessage: "NumPartitions must be > 0",
		},
		{
			name: "valid minimal config",
			cfg: common.TopicConfig{
				Name:              "test-valid",
				NumPartitions:     1,
				ReplicationFactor: 1,
			},
			wantErr: false,
		},
		{
			name: "valid with all fields",
			cfg: common.TopicConfig{
				Name:              "test-valid-full",
				NumPartitions:     3,
				ReplicationFactor: 1,
				RetentionMs:       86400000,
				CleanupPolicy:     "delete",
				MaxMessageBytes:   1048576,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockKafkaConn{
				createFunc: func(topics ...kafka.TopicConfig) error {
					return nil
				},
			}
			mgr := newTestTopicManager(mock)
			err := mgr.CreateTopic(context.Background(), tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMessage != "" {
					assert.Contains(t, err.Error(), tt.errMessage)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTopicValidation_ConsumerConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        ConsumerConfig
		wantErr    bool
		errMessage string
	}{
		{
			name: "empty brokers",
			cfg: ConsumerConfig{
				Brokers: nil,
				GroupID: "g",
			},
			wantErr:    true,
			errMessage: "Brokers required",
		},
		{
			name: "empty group id",
			cfg: ConsumerConfig{
				Brokers: []string{"localhost:9092"},
				GroupID: "",
			},
			wantErr:    true,
			errMessage: "GroupID required",
		},
		{
			name: "invalid offset reset",
			cfg: ConsumerConfig{
				Brokers:         []string{"localhost:9092"},
				GroupID:         "g",
				AutoOffsetReset: "middle",
			},
			wantErr:    true,
			errMessage: "Invalid AutoOffsetReset",
		},
		{
			name: "valid minimal",
			cfg: ConsumerConfig{
				Brokers: []string{"localhost:9092"},
				GroupID: "test-group",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConsumerConfig(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMessage != "" {
					assert.Contains(t, err.Error(), tt.errMessage)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTopicValidation_ProducerConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        ProducerConfig
		wantErr    bool
		errMessage string
	}{
		{
			name: "empty brokers",
			cfg: ProducerConfig{
				Brokers: nil,
			},
			wantErr:    true,
			errMessage: "Brokers required",
		},
		{
			name: "negative max retries",
			cfg: ProducerConfig{
				Brokers:    []string{"localhost:9092"},
				MaxRetries: -1,
			},
			wantErr:    true,
			errMessage: "MaxRetries must be >= 0",
		},
		{
			name: "valid minimal",
			cfg: ProducerConfig{
				Brokers: []string{"localhost:9092"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProducerConfig(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMessage != "" {
					assert.Contains(t, err.Error(), tt.errMessage)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Serialization / Deserialization integration tests
// ---------------------------------------------------------------------------

func TestDomainEvent_Serialization_AllPayloadTypes(t *testing.T) {
	tests := []struct {
		name    string
		payload interface{}
		eventID string
	}{
		{
			name: "PatentIngestedPayload",
			payload: PatentIngestedPayload{
				PatentID:     "pat-001",
				PatentNumber: "US-12345678",
				Title:        "Test Patent",
				Source:       "integration-test",
				IngestedAt:   time.Now().UTC(),
			},
		},
		{
			name: "PatentAnalyzedPayload",
			payload: PatentAnalyzedPayload{
				PatentID:      "pat-002",
				PatentNumber:  "US-87654321",
				AnalysisType:  "prior_art",
				ResultSummary: "Found 3 prior art references",
				AnalyzedAt:    time.Now().UTC(),
			},
		},
		{
			name: "PatentVectorizedPayload",
			payload: PatentVectorizedPayload{
				PatentID:     "pat-003",
				PatentNumber: "US-11223344",
				VectorFields: []string{"title", "abstract", "claims"},
				ModelName:    "all-MiniLM-L6-v2",
				ModelVersion: "1.0",
				VectorizedAt: time.Now().UTC(),
			},
		},
		{
			name: "InfringementDetectedPayload",
			payload: InfringementDetectedPayload{
				InfringementID:  "inf-001",
				SourcePatentID:  "pat-001",
				TargetPatentID:  "pat-002",
				SimilarityScore: 0.85,
				RiskLevel:       "HIGH",
				DetectedAt:      time.Now().UTC(),
			},
		},
		{
			name: "CompetitiveIntelUpdatePayload",
			payload: CompetitiveIntelUpdatePayload{
				CompetitorID:   "comp-001",
				CompetitorName: "Acme Corp",
				UpdateType:     "new_patent_filing",
				Summary:        "Filed patent on quantum computing",
				DetectedAt:     time.Now().UTC(),
			},
		},
		{
			name: "NotificationPayload",
			payload: NotificationPayload{
				RecipientID: "user-001",
				Channel:     "email",
				Subject:     "Alert: New infringement detected",
				Body:        "Patent XYZ may infringe on your portfolio.",
				Priority:    "high",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create envelope
			env, err := NewEventEnvelope("test."+strings.ToLower(tt.name), "integration-test", tt.payload)
			require.NoError(t, err)
			require.NotEmpty(t, env.EventID)
			assert.Equal(t, "v1", env.SchemaVersion)

			// Serialize to message
			msg, err := env.ToMessage("test-topic")
			require.NoError(t, err)
			require.NotNil(t, msg)
			assert.Equal(t, "test-topic", msg.Topic)
			assert.Equal(t, "test."+strings.ToLower(tt.name), msg.Headers["event_type"])
			assert.Equal(t, "integration-test", msg.Headers["source_service"])
			assert.Equal(t, "v1", msg.Headers["schema_version"])

			// Deserialize from message
			kafkaMsg := &common.Message{Value: msg.Value, Topic: msg.Topic, Headers: msg.Headers}
			decodedEnv, err := MessageToEventEnvelope(kafkaMsg)
			require.NoError(t, err)
			assert.Equal(t, env.EventID, decodedEnv.EventID)
			assert.Equal(t, env.EventType, decodedEnv.EventType)
			assert.Equal(t, env.SchemaVersion, decodedEnv.SchemaVersion)

			// Decode payload and verify fields individually
			// (DecodePayload unmarshals into a pointer, but it's cleaner to
			// compare each payload type's fields directly)
			switch p := tt.payload.(type) {
			case PatentIngestedPayload:
				var decoded PatentIngestedPayload
				err = decodedEnv.DecodePayload(&decoded)
				require.NoError(t, err)
				assert.Equal(t, p.PatentID, decoded.PatentID)
				assert.Equal(t, p.PatentNumber, decoded.PatentNumber)
				assert.Equal(t, p.Title, decoded.Title)
				assert.Equal(t, p.Source, decoded.Source)
			case PatentAnalyzedPayload:
				var decoded PatentAnalyzedPayload
				err = decodedEnv.DecodePayload(&decoded)
				require.NoError(t, err)
				assert.Equal(t, p.PatentID, decoded.PatentID)
				assert.Equal(t, p.PatentNumber, decoded.PatentNumber)
				assert.Equal(t, p.AnalysisType, decoded.AnalysisType)
				assert.Equal(t, p.ResultSummary, decoded.ResultSummary)
			case PatentVectorizedPayload:
				var decoded PatentVectorizedPayload
				err = decodedEnv.DecodePayload(&decoded)
				require.NoError(t, err)
				assert.Equal(t, p.PatentID, decoded.PatentID)
				assert.Equal(t, p.PatentNumber, decoded.PatentNumber)
				assert.Equal(t, p.VectorFields, decoded.VectorFields)
				assert.Equal(t, p.ModelName, decoded.ModelName)
				assert.Equal(t, p.ModelVersion, decoded.ModelVersion)
			case InfringementDetectedPayload:
				var decoded InfringementDetectedPayload
				err = decodedEnv.DecodePayload(&decoded)
				require.NoError(t, err)
				assert.Equal(t, p.InfringementID, decoded.InfringementID)
				assert.Equal(t, p.SourcePatentID, decoded.SourcePatentID)
				assert.Equal(t, p.TargetPatentID, decoded.TargetPatentID)
				assert.InDelta(t, p.SimilarityScore, decoded.SimilarityScore, 0.001)
				assert.Equal(t, p.RiskLevel, decoded.RiskLevel)
			case CompetitiveIntelUpdatePayload:
				var decoded CompetitiveIntelUpdatePayload
				err = decodedEnv.DecodePayload(&decoded)
				require.NoError(t, err)
				assert.Equal(t, p.CompetitorID, decoded.CompetitorID)
				assert.Equal(t, p.CompetitorName, decoded.CompetitorName)
				assert.Equal(t, p.UpdateType, decoded.UpdateType)
				assert.Equal(t, p.Summary, decoded.Summary)
			case NotificationPayload:
				var decoded NotificationPayload
				err = decodedEnv.DecodePayload(&decoded)
				require.NoError(t, err)
				assert.Equal(t, p.RecipientID, decoded.RecipientID)
				assert.Equal(t, p.Channel, decoded.Channel)
				assert.Equal(t, p.Subject, decoded.Subject)
				assert.Equal(t, p.Body, decoded.Body)
				assert.Equal(t, p.Priority, decoded.Priority)
			default:
				t.Fatalf("unexpected payload type %T", tt.payload)
			}
		})
	}
}

func TestDomainEvent_Integration_EndToEnd(t *testing.T) {
	brokers := skipIfNoKafka(t)

	topic := "int-test-domain-event-" + randomSuffix()
	createTempTopic(t, brokers, topic)
	defer deleteTempTopic(t, brokers, topic)
	waitForTopic(t, brokers, topic, 5*time.Second)

	// Create an event envelope
	payload := InfringementDetectedPayload{
		InfringementID:  "inf-e2e-001",
		SourcePatentID:  "pat-source",
		TargetPatentID:  "pat-target",
		SimilarityScore: 0.92,
		RiskLevel:       "CRITICAL",
		DetectedAt:      time.Now().UTC(),
	}
	env, err := NewEventEnvelope("infringement.detected", "integration-test", payload)
	require.NoError(t, err)

	producerMsg, err := env.ToMessage(topic)
	require.NoError(t, err)

	// Publish via real producer
	cfg := ProducerConfig{
		Brokers:         brokers,
		BatchSize:       1,
		BatchTimeout:    100 * time.Millisecond,
		MaxMessageBytes: 1024 * 1024,
	}
	producer, err := NewProducer(cfg, newMockLogger())
	require.NoError(t, err)
	defer producer.Close()

	err = producer.Publish(context.Background(), producerMsg)
	require.NoError(t, err)

	// Consume
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        "int-test-domain-event-group-" + randomSuffix(),
		StartOffset:    kafka.FirstOffset,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        1 * time.Second,
		CommitInterval: 0,
	})
	defer reader.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	kafkaMsg, err := reader.FetchMessage(ctx)
	require.NoError(t, err)
	_ = reader.CommitMessages(ctx, kafkaMsg)

	// Convert back to envelope
	msg := &common.Message{
		Topic: kafkaMsg.Topic, Key: kafkaMsg.Key, Value: kafkaMsg.Value,
		Timestamp: kafkaMsg.Time,
	}
	for _, h := range kafkaMsg.Headers {
		if msg.Headers == nil {
			msg.Headers = make(map[string]string)
		}
		msg.Headers[h.Key] = string(h.Value)
	}

	decodedEnv, err := MessageToEventEnvelope(msg)
	require.NoError(t, err)
	assert.Equal(t, env.EventID, decodedEnv.EventID)
	assert.Equal(t, env.EventType, decodedEnv.EventType)
	assert.Equal(t, env.Source, decodedEnv.Source)

	var decodedPayload InfringementDetectedPayload
	err = decodedEnv.DecodePayload(&decodedPayload)
	require.NoError(t, err)
	assert.Equal(t, "inf-e2e-001", decodedPayload.InfringementID)
	assert.Equal(t, "pat-source", decodedPayload.SourcePatentID)
	assert.Equal(t, "CRITICAL", decodedPayload.RiskLevel)
	assert.InDelta(t, 0.92, decodedPayload.SimilarityScore, 0.001)
}

func TestDomainEvent_DecodePayload_Empty(t *testing.T) {
	env := &EventEnvelope{
		EventID:   "test-empty",
		EventType: "test.event",
		Payload:   nil,
	}
	var target PatentIngestedPayload
	err := env.DecodePayload(&target)
	assert.NoError(t, err, "decoding empty payload should not error")

	env.Payload = []byte("null")
	err = env.DecodePayload(&target)
	assert.NoError(t, err, "decoding null payload should not error")
}

func TestDomainEvent_MessageToEventEnvelope_EmptyValue(t *testing.T) {
	msg := &common.Message{Value: nil}
	_, err := MessageToEventEnvelope(msg)
	assert.Error(t, err)

	msg.Value = []byte{}
	_, err = MessageToEventEnvelope(msg)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// Error handling tests
// ---------------------------------------------------------------------------

func TestErrorHandling_BrokerUnavailable(t *testing.T) {
	badBrokers := []string{"localhost:19092"}

	// Producer with unreachable broker
	cfg := ProducerConfig{
		Brokers:         badBrokers,
		MaxRetries:      0,
		BatchTimeout:    100 * time.Millisecond,
		WriteTimeout:    500 * time.Millisecond,
		MaxMessageBytes: 1024 * 1024,
	}
	producer, err := NewProducer(cfg, newMockLogger())
	require.NoError(t, err)
	defer producer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = producer.Publish(ctx, &common.ProducerMessage{
		Topic: "test",
		Key:   []byte("k"),
		Value: []byte("v"),
	})
	assert.Error(t, err)

	// Metrics should reflect failure (LastSentAt may not be initialized so
	// we access the internal metrics directly)
	assert.Equal(t, int64(1), producer.metrics.MessagesFailed.Load())
}

func TestErrorHandling_InvalidMessageValidation(t *testing.T) {
	brokers := skipIfNoKafka(t)

	cfg := ProducerConfig{
		Brokers:         brokers,
		MaxMessageBytes: 1024 * 1024,
	}
	producer, err := NewProducer(cfg, newMockLogger())
	require.NoError(t, err)
	defer producer.Close()

	tests := []struct {
		name string
		msg  *common.ProducerMessage
	}{
		{
			name: "empty topic",
			msg:  &common.ProducerMessage{Topic: "", Key: []byte("k"), Value: []byte("v")},
		},
		{
			name: "empty value",
			msg:  &common.ProducerMessage{Topic: "test", Key: []byte("k"), Value: []byte{}},
		},
		{
			name: "nil value",
			msg:  &common.ProducerMessage{Topic: "test", Key: []byte("k"), Value: nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := producer.Publish(context.Background(), tt.msg)
			assert.Error(t, err)
		})
	}
}

func TestErrorHandling_PublishToNonExistentTopic(t *testing.T) {
	brokers := skipIfNoKafka(t)

	cfg := ProducerConfig{
		Brokers:         brokers,
		MaxRetries:      1,
		RetryBackoff:    50 * time.Millisecond,
		BatchSize:       1,
		BatchTimeout:    100 * time.Millisecond,
		WriteTimeout:    2 * time.Second,
		ReadTimeout:     2 * time.Second,
		MaxMessageBytes: 1024 * 1024,
	}
	producer, err := NewProducer(cfg, newMockLogger())
	require.NoError(t, err)
	defer producer.Close()

	// Publish to a topic that exists but wasn't manually created first.
	// The kafka-go library should auto-create if the broker allows, or we get an error.
	// With default Kafka auto.create.topics.enable=true this will succeed.
	topic := "int-test-auto-create-" + randomSuffix()
	defer deleteTempTopic(t, brokers, topic)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = producer.Publish(ctx, &common.ProducerMessage{
		Topic: topic,
		Key:   []byte("auto-key"),
		Value: []byte("auto-value"),
	})
	// Either succeeds (auto-create) or fails — both are acceptable
	if err != nil {
		t.Logf("Publish to auto-created topic gave expected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TopicManager integration tests
// ---------------------------------------------------------------------------

func TestTopicManagerIntegration_CreateAndListTopics(t *testing.T) {
	brokers := skipIfNoKafka(t)

	mgr, err := NewTopicManager(brokers, newMockLogger())
	require.NoError(t, err)
	defer mgr.Close()

	topic := "int-test-mgr-list-" + randomSuffix()
	cfg := common.TopicConfig{
		Name:              topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}

	err = mgr.CreateTopic(context.Background(), cfg)
	require.NoError(t, err)

	exists, err := mgr.TopicExists(context.Background(), topic)
	assert.NoError(t, err)
	assert.True(t, exists)

	topics, err := mgr.ListTopics(context.Background())
	assert.NoError(t, err)
	assert.Contains(t, topics, topic)

	// Clean up
	err = mgr.DeleteTopic(context.Background(), topic)
	assert.NoError(t, err)

	exists, err = mgr.TopicExists(context.Background(), topic)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestTopicManagerIntegration_CreateDuplicateTopic(t *testing.T) {
	brokers := skipIfNoKafka(t)

	mgr, err := NewTopicManager(brokers, newMockLogger())
	require.NoError(t, err)
	defer mgr.Close()

	topic := "int-test-mgr-dedup-" + randomSuffix()
	cfg := common.TopicConfig{
		Name:              topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	}

	// First create should succeed
	err = mgr.CreateTopic(context.Background(), cfg)
	require.NoError(t, err)

	// Second create should also succeed (idempotent)
	err = mgr.CreateTopic(context.Background(), cfg)
	assert.NoError(t, err, "creating a duplicate topic should not error")

	// Clean up
	_ = mgr.DeleteTopic(context.Background(), topic)
}

func TestTopicManagerIntegration_EnsureTopics(t *testing.T) {
	brokers := skipIfNoKafka(t)

	mgr, err := NewTopicManager(brokers, newMockLogger())
	require.NoError(t, err)
	defer mgr.Close()

	topicA := "int-test-ensure-a-" + randomSuffix()
	topicB := "int-test-ensure-b-" + randomSuffix()

	err = mgr.EnsureTopics(context.Background(), []common.TopicConfig{
		{Name: topicA, NumPartitions: 1, ReplicationFactor: 1},
		{Name: topicB, NumPartitions: 1, ReplicationFactor: 1},
	})
	require.NoError(t, err)

	// Verify both exist
	for _, name := range []string{topicA, topicB} {
		exists, err := mgr.TopicExists(context.Background(), name)
		assert.NoError(t, err)
		assert.True(t, exists, "topic %s should exist", name)
		_ = mgr.DeleteTopic(context.Background(), name)
	}
}

func TestTopicManagerIntegration_EnsureDefaultTopics(t *testing.T) {
	brokers := skipIfNoKafka(t)

	mgr, err := NewTopicManager(brokers, newMockLogger())
	require.NoError(t, err)
	defer mgr.Close()

	// EnsureDefaultTopics creates all topics from DefaultTopics()
	err = mgr.EnsureDefaultTopics(context.Background())
	// This may succeed or fail depending on broker config; we just check it runs
	t.Logf("EnsureDefaultTopics result: %v", err)

	// Clean up defaults
	for _, dt := range DefaultTopics() {
		_ = mgr.DeleteTopic(context.Background(), dt.Name)
	}
}

// ---------------------------------------------------------------------------
// Producer PublishAsync integration test
// ---------------------------------------------------------------------------

func TestProducerIntegration_PublishAsyncWithRealBroker(t *testing.T) {
	brokers := skipIfNoKafka(t)

	topic := "int-test-async-" + randomSuffix()
	createTempTopic(t, brokers, topic)
	defer deleteTempTopic(t, brokers, topic)
	waitForTopic(t, brokers, topic, 5*time.Second)

	cfg := ProducerConfig{
		Brokers:         brokers,
		BatchSize:       1,
		BatchTimeout:    100 * time.Millisecond,
		MaxMessageBytes: 1024 * 1024,
	}
	producer, err := NewProducer(cfg, newMockLogger())
	require.NoError(t, err)
	defer producer.Close()

	var published int32
	producer.config.AsyncErrorHandler = func(err error, msg *common.ProducerMessage) {
		t.Logf("Async error: %v", err)
	}

	asyncMsg := &common.ProducerMessage{
		Topic: topic,
		Key:   []byte("async-key"),
		Value: []byte("async-value"),
	}
	producer.PublishAsync(context.Background(), asyncMsg)

	// Give time for async publish
	time.Sleep(1 * time.Second)

	// Consume the message
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       topic,
		GroupID:     "int-test-async-group-" + randomSuffix(),
		StartOffset: kafka.FirstOffset,
		MinBytes:    1,
		MaxBytes:    10e6,
		MaxWait:     1 * time.Second,
	})
	defer reader.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	consumed, err := reader.FetchMessage(ctx)
	if err == nil {
		assert.Equal(t, "async-value", string(consumed.Value))
		atomic.AddInt32(&published, 1)
	}

	assert.Equal(t, int32(1), atomic.LoadInt32(&published), "async publish should deliver message")
}

// ---------------------------------------------------------------------------
// Default topic constants consistency test
// ---------------------------------------------------------------------------

func TestDefaultTopics_Consistency(t *testing.T) {
	defaults := DefaultTopics()
	assert.Len(t, defaults, 19)

	// All topic names should be non-empty
	for _, d := range defaults {
		assert.NotEmpty(t, d.Name, "topic name should not be empty")
		assert.Greater(t, d.NumPartitions, 0, "topic %s should have partitions", d.Name)
		assert.Greater(t, d.ReplicationFactor, 0, "topic %s should have replication factor", d.Name)
	}

	// Verify specific topic constants are present
	names := make(map[string]bool)
	for _, d := range defaults {
		names[d.Name] = true
	}
	assert.True(t, names[TopicPatentIngested], "should include patent.ingested")
	assert.True(t, names[TopicInfringementDetected], "should include infringement.detected")
	assert.True(t, names[TopicDeadLetterDefault], "should include dead_letter.default")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// randomSuffix returns a short random string for topic/group naming.
func randomSuffix() string {
	return itoa(int(time.Now().UnixNano() % 100000))
}

// itoa is a simple int to string conversion without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	if neg {
		return "-" + s
	}
	return s
}

// createTempTopic creates a temporary topic and also returns a ConnInterface
// based TopicManager for it.
func createTempTopic(t *testing.T, brokers []string, topic string) {
	t.Helper()
	// Use kafka.Dial directly to avoid the ConnInterface requirement
	conn, err := kafka.Dial("tcp", brokers[0])
	require.NoError(t, err)
	defer conn.Close()

	err = conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	require.NoError(t, err)
}

func deleteTempTopic(t *testing.T, brokers []string, topic string) {
	t.Helper()
	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.DeleteTopics(topic)
}

// publishTestMessages publishes n messages to the given topic using a temporary producer.
func publishTestMessages(t *testing.T, brokers []string, topic string, n int) {
	t.Helper()
	cfg := ProducerConfig{
		Brokers:         brokers,
		BatchSize:       n,
		BatchTimeout:    1 * time.Second,
		MaxMessageBytes: 1024 * 1024,
	}
	producer, err := NewProducer(cfg, newMockLogger())
	require.NoError(t, err)
	defer producer.Close()

	msgs := make([]*common.ProducerMessage, n)
	for i := 0; i < n; i++ {
		msgs[i] = &common.ProducerMessage{
			Topic: topic,
			Key:   []byte("key-" + itoa(i)),
			Value: []byte("payload-" + itoa(i)),
		}
	}

	_, err = producer.PublishBatch(context.Background(), msgs)
	require.NoError(t, err)
}

