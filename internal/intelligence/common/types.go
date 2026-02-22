package common

import (
	"context"
	"encoding/json"
	"fmt"
)

// ---------------------------------------------------------------------------
// ModelBackend interface
// ---------------------------------------------------------------------------

// ModelBackend defines the interface for invoking AI models (Triton, ONNX, etc).
type ModelBackend interface {
	Predict(ctx context.Context, req *PredictRequest) (*PredictResponse, error)
}

// ---------------------------------------------------------------------------
// Logger interface
// ---------------------------------------------------------------------------

// Logger defines a structured logging interface compatible with zap or others.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// noopLogger implements Logger and does nothing.
type noopLogger struct{}

func (n *noopLogger) Info(string, ...interface{})  {}
func (n *noopLogger) Warn(string, ...interface{})  {}
func (n *noopLogger) Debug(string, ...interface{}) {}
func (n *noopLogger) Error(string, ...interface{}) {}

// NewNoopLogger returns a Logger that discards all logs.
func NewNoopLogger() Logger {
	return &noopLogger{}
}

// ---------------------------------------------------------------------------
// Predict types
// ---------------------------------------------------------------------------

// PredictRequest carries the input payload for model inference.
type PredictRequest struct {
	ModelName   string            `json:"model_name"`
	InputData   []byte            `json:"input_data"`
	InputFormat string            `json:"input_format"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// PredictResponse carries the raw outputs from model inference.
type PredictResponse struct {
	Outputs map[string]interface{} `json:"outputs"`
}

// Format constants.
const (
	FormatJSON = "JSON"
)

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

// EncodeTokenList encodes a list of tokens into a JSON byte slice.
func EncodeTokenList(tokens []string) []byte {
	b, _ := json.Marshal(tokens)
	return b
}

// DecodeTokenList decodes a JSON byte slice into a list of tokens.
func DecodeTokenList(data []byte) ([]string, error) {
	var tokens []string
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, err
	}
	return tokens, nil
}

// EncodeFloat64Matrix encodes a [][]float64 matrix into a JSON byte slice.
func EncodeFloat64Matrix(matrix [][]float64) ([]byte, error) {
	return json.Marshal(matrix)
}

// DecodeFloat64Matrix decodes a generic interface{} (usually from JSON unmarshal)
// into a [][]float64 matrix. It handles potential type mismatches robustly.
func DecodeFloat64Matrix(input interface{}) ([][]float64, error) {
	if input == nil {
		return nil, fmt.Errorf("input is nil")
	}

	// Case 1: Already [][]float64
	if mat, ok := input.([][]float64); ok {
		return mat, nil
	}

	// Case 2: Slice of interface{} (from JSON unmarshal)
	slice, ok := input.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected []interface{}, got %T", input)
	}

	result := make([][]float64, len(slice))
	for i, rowRaw := range slice {
		rowSlice, ok := rowRaw.([]interface{})
		if !ok {
			return nil, fmt.Errorf("row %d is not []interface{}, got %T", i, rowRaw)
		}
		row := make([]float64, len(rowSlice))
		for j, valRaw := range rowSlice {
			f, err := toFloat64(valRaw)
			if err != nil {
				return nil, fmt.Errorf("row %d col %d: %w", i, j, err)
			}
			row[j] = f
		}
		result[i] = row
	}

	return result, nil
}

func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case json.Number:
		return val.Float64()
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

//Personal.AI order the ending
