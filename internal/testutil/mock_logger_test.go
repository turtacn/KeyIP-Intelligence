package testutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/testutil"
)

func TestMockLogger(t *testing.T) {
	logger := testutil.NewMockLogger()

	logger.Info("test info", logging.String("key", "value"))

	messages := logger.GetMessages()
	assert.Len(t, messages, 1)
	assert.Equal(t, "info", messages[0].Level)
	assert.Equal(t, "test info", messages[0].Message)

	logger.Clear()
	assert.Len(t, logger.GetMessages(), 0)

	logger.Error("test error")
	assert.True(t, logger.HasMessage("error", "test error"))
	assert.False(t, logger.HasMessage("info", "test info"))
}

func TestNopLogger(t *testing.T) {
	logger := testutil.NewNopLogger()

	// Ensure it implements the interface and doesn't panic
	logger.Info("test info")
	logger.Error("test error")

	// No assertion needed other than no panic
	assert.NotNil(t, logger)
}
