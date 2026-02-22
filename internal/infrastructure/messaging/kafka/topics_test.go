package kafka

import (
	"context"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
)

type mockKafkaConn struct {
	createFunc func(topics ...kafka.TopicConfig) error
	deleteFunc func(topics ...string) error
	readFunc   func(topics ...string) ([]kafka.Partition, error)
	closeFunc  func() error
}

func (m *mockKafkaConn) CreateTopics(topics ...kafka.TopicConfig) error {
	if m.createFunc != nil {
		return m.createFunc(topics...)
	}
	return nil
}

func (m *mockKafkaConn) DeleteTopics(topics ...string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(topics...)
	}
	return nil
}

func (m *mockKafkaConn) ReadPartitions(topics ...string) ([]kafka.Partition, error) {
	if m.readFunc != nil {
		return m.readFunc(topics...)
	}
	return nil, nil
}

func (m *mockKafkaConn) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func newTestTopicManager(mock ConnInterface) *TopicManager {
	return &TopicManager{
		conn:   mock,
		logger: newMockLogger(),
	}
}

func TestTopicConstants(t *testing.T) {
	assert.Equal(t, "patent.ingested", TopicPatentIngested)
}

func TestDefaultTopics(t *testing.T) {
	defaults := DefaultTopics()
	assert.Len(t, defaults, 19)
}

func TestCreateTopic_Success(t *testing.T) {
	mock := &mockKafkaConn{
		createFunc: func(topics ...kafka.TopicConfig) error {
			assert.Len(t, topics, 1)
			assert.Equal(t, "test", topics[0].Topic)
			return nil
		},
	}
	m := newTestTopicManager(mock)
	err := m.CreateTopic(context.Background(), TopicConfig{Name: "test", NumPartitions: 1, ReplicationFactor: 1})
	assert.NoError(t, err)
}

func TestDeleteTopic_Success(t *testing.T) {
	mock := &mockKafkaConn{
		deleteFunc: func(topics ...string) error {
			assert.Equal(t, "test", topics[0])
			return nil
		},
	}
	m := newTestTopicManager(mock)
	err := m.DeleteTopic(context.Background(), "test")
	assert.NoError(t, err)
}

func TestEventEnvelope_RoundTrip(t *testing.T) {
	payload := PatentIngestedPayload{PatentID: "123"}
	env, err := NewEventEnvelope("type", "src", payload)
	assert.NoError(t, err)

	msg, err := env.ToMessage("topic")
	assert.NoError(t, err)

	kafkaMsg := &Message{Value: msg.Value}
	decodedEnv, err := MessageToEventEnvelope(kafkaMsg)
	assert.NoError(t, err)

	var decodedPayload PatentIngestedPayload
	err = decodedEnv.DecodePayload(&decodedPayload)
	assert.NoError(t, err)
	assert.Equal(t, "123", decodedPayload.PatentID)
}

//Personal.AI order the ending
