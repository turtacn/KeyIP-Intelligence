package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

func TestAllTopics(t *testing.T) {
	expectedTopics := []string{
		"patent.new",
		"patent.status_changed",
		"molecule.indexed",
		"alert.trigger",
		"deadline.approaching",
		"report.generate",
		"infrastructure.health",
	}
	require.Len(t, allTopics, len(expectedTopics))
	for _, topic := range expectedTopics {
		assert.Contains(t, allTopics, topic)
	}
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "configs/config.yaml", defaultWorkerConfigPath)
	assert.Equal(t, 8081, defaultHealthPort)
	assert.Equal(t, 5*time.Minute, defaultHandlerTimeout)
	assert.Equal(t, 3, maxRetries)
}

// --- buildHandlerRegistry Tests ---

func TestBuildHandlerRegistry_ReturnsAllTopics(t *testing.T) {
	logger := logging.NewNopLogger()
	handlers := buildHandlerRegistry(nil, nil, nil, nil, logger)

	require.Len(t, handlers, len(allTopics))
	for _, topic := range allTopics {
		h, ok := handlers[topic]
		assert.True(t, ok, "missing handler for topic: %s", topic)
		assert.Equal(t, topic, h.Topic(),
			"handler topic should match map key")
	}
}

// testHandler is a simple MessageHandler implementation for testing.
type testHandler struct {
	topic string
}

func (h *testHandler) Topic() string { return h.topic }

func (h *testHandler) Handle(ctx context.Context, msg *common.Message) error {
	return nil
}

// --- workerLoop Tests ---

func TestWorkerLoop_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Pre-cancel the context

	err := workerLoop(ctx, 0, make(chan *common.Message), nil, nil, logging.NewNopLogger())
	assert.NoError(t, err, "should exit cleanly when context is cancelled")
}

func TestWorkerLoop_ChannelClosed(t *testing.T) {
	ch := make(chan *common.Message)
	close(ch)

	err := workerLoop(context.Background(), 0, ch, nil, nil, logging.NewNopLogger())
	assert.NoError(t, err, "should exit cleanly when message channel is closed")
}

func TestWorkerLoop_NilHandlersMap(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *common.Message, 1)
	ch <- &common.Message{Topic: "unknown.topic"}
	// Close channel after sending so the loop will exit
	close(ch)
	cancel() // Also cancel to ensure clean exit

	err := workerLoop(ctx, 0, ch, nil, nil, logging.NewNopLogger())
	assert.NoError(t, err, "should handle nil handlers map gracefully")
}

func TestWorkerLoop_ProcessesMessageSuccessfully(t *testing.T) {
	logger := logging.NewNopLogger()
	handlers := map[string]MessageHandler{
		"test.topic": &testHandler{topic: "test.topic"},
	}
	msgChan := make(chan *common.Message, 1)

	msgChan <- &common.Message{
		Topic:     "test.topic",
		Partition: 0,
		Offset:    100,
		Key:       []byte("test-key"),
		Value:     []byte("test-value"),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- workerLoop(context.Background(), 1, msgChan, handlers, nil, logger)
	}()

	// Give time for the message to be processed
	time.Sleep(50 * time.Millisecond)
	close(msgChan)

	select {
	case err := <-errCh:
		assert.NoError(t, err, "workerLoop should complete cleanly")
	case <-time.After(2 * time.Second):
		t.Fatal("workerLoop did not exit within timeout after channel close")
	}
}

func TestWorkerLoop_MultipleMessages(t *testing.T) {
	logger := logging.NewNopLogger()
	handlers := make(map[string]MessageHandler)
	for _, topic := range allTopics {
		handlers[topic] = &testHandler{topic: topic}
	}
	msgChan := make(chan *common.Message, len(allTopics))

	// Send one message per topic
	for i, topic := range allTopics {
		msgChan <- &common.Message{
			Topic:     topic,
			Partition: i,
			Offset:    int64(i * 100),
		}
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- workerLoop(context.Background(), 1, msgChan, handlers, nil, logger)
	}()

	time.Sleep(100 * time.Millisecond)
	close(msgChan)

	select {
	case err := <-errCh:
		assert.NoError(t, err, "workerLoop should process all messages cleanly")
	case <-time.After(2 * time.Second):
		t.Fatal("workerLoop did not exit within timeout")
	}
}

func TestWorkerLoop_RespectsContextCancellationWhileIdle(t *testing.T) {
	logger := logging.NewNopLogger()
	ctx, cancel := context.WithCancel(context.Background())
	msgChan := make(chan *common.Message) // unbuffered, will block

	errCh := make(chan error, 1)
	go func() {
		errCh <- workerLoop(ctx, 0, msgChan, nil, nil, logger)
	}()

	// Cancel while worker is waiting for messages
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		assert.NoError(t, err, "should exit cleanly when context is cancelled while idle")
	case <-time.After(2 * time.Second):
		t.Fatal("workerLoop did not respond to cancellation within timeout")
	}
}

// --- processMessage Tests ---

func TestProcessMessage_UnknownTopic(t *testing.T) {
	logger := logging.NewNopLogger()
	msg := &common.Message{
		Topic:     "nonexistent.topic",
		Partition: 0,
		Offset:    1,
	}

	// Should not panic with nil handlers
	processMessage(context.Background(), 0, msg, nil, nil, logger)

	// Should not panic with empty handlers
	processMessage(context.Background(), 0, msg, map[string]MessageHandler{}, nil, logger)
}

func TestProcessMessage_SuccessfulHandler(t *testing.T) {
	logger := logging.NewNopLogger()
	msg := &common.Message{
		Topic:     "test.topic",
		Partition: 0,
		Offset:    42,
	}
	handlers := map[string]MessageHandler{
		"test.topic": &testHandler{topic: "test.topic"},
	}

	// Should handle message without error
	processMessage(context.Background(), 0, msg, handlers, nil, logger)
}

func TestProcessMessage_WithCancelledContext(t *testing.T) {
	logger := logging.NewNopLogger()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := &common.Message{
		Topic:     "test.topic",
		Partition: 0,
		Offset:    42,
	}
	handlers := map[string]MessageHandler{
		"test.topic": &testHandler{topic: "test.topic"},
	}

	processMessage(ctx, 0, msg, handlers, nil, logger)
}

// --- workerInfrastructure Tests ---

func TestWorkerInfrastructure_CloseNilSafe(t *testing.T) {
	// Verify that Close() handles nil fields gracefully
	infra := &workerInfrastructure{}
	infra.Close() // Should not panic
}

func TestWorkerInfrastructure_CloseMultipleTimes(t *testing.T) {
	infra := &workerInfrastructure{}
	infra.Close()
	infra.Close() // Second close should not panic
}
