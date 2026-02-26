// Package testutil provides common test utilities for KeyIP-Intelligence.
package testutil

import (
	"context"
	"sync"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// MockLogger implements logging.Logger for testing purposes.
// It records log messages and can be used to verify logging behavior.
type MockLogger struct {
	mu       sync.Mutex
	Messages []LogMessage
}

// LogMessage represents a single log entry captured by MockLogger.
type LogMessage struct {
	Level   string
	Message string
	Fields  []logging.Field
}

// NewMockLogger creates a new MockLogger instance.
func NewMockLogger() *MockLogger {
	return &MockLogger{
		Messages: make([]LogMessage, 0),
	}
}

func (m *MockLogger) log(level, msg string, fields []logging.Field) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = append(m.Messages, LogMessage{
		Level:   level,
		Message: msg,
		Fields:  fields,
	})
}

func (m *MockLogger) Debug(msg string, fields ...logging.Field) {
	m.log("debug", msg, fields)
}

func (m *MockLogger) Info(msg string, fields ...logging.Field) {
	m.log("info", msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...logging.Field) {
	m.log("warn", msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...logging.Field) {
	m.log("error", msg, fields)
}

func (m *MockLogger) Fatal(msg string, fields ...logging.Field) {
	m.log("fatal", msg, fields)
}

func (m *MockLogger) With(fields ...logging.Field) logging.Logger {
	return m
}

func (m *MockLogger) WithContext(ctx context.Context) logging.Logger {
	return m
}

func (m *MockLogger) WithError(err error) logging.Logger {
	return m
}

func (m *MockLogger) Sync() error {
	return nil
}

// GetMessages returns a copy of all logged messages.
func (m *MockLogger) GetMessages() []LogMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]LogMessage, len(m.Messages))
	copy(result, m.Messages)
	return result
}

// Clear removes all logged messages.
func (m *MockLogger) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = m.Messages[:0]
}

// HasMessage checks if a message with the given level and content was logged.
func (m *MockLogger) HasMessage(level, msg string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, logged := range m.Messages {
		if logged.Level == level && logged.Message == msg {
			return true
		}
	}
	return false
}

// NopLogger is a logger that discards all output, useful for tests that don't need logging verification.
type NopLogger struct{}

func NewNopLogger() *NopLogger                                   { return &NopLogger{} }
func (n *NopLogger) Debug(msg string, fields ...logging.Field)   {}
func (n *NopLogger) Info(msg string, fields ...logging.Field)    {}
func (n *NopLogger) Warn(msg string, fields ...logging.Field)    {}
func (n *NopLogger) Error(msg string, fields ...logging.Field)   {}
func (n *NopLogger) Fatal(msg string, fields ...logging.Field)   {}
func (n *NopLogger) With(fields ...logging.Field) logging.Logger { return n }
func (n *NopLogger) WithContext(ctx context.Context) logging.Logger { return n }
func (n *NopLogger) WithError(err error) logging.Logger          { return n }
func (n *NopLogger) Sync() error                                 { return nil }

//Personal.AI order the ending
