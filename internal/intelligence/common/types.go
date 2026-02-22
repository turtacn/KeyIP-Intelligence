package common

import (
	"context"
	"encoding/json"
	"fmt"
)

// ---------------------------------------------------------------------------
// InputFormat enum
// ---------------------------------------------------------------------------

type InputFormat int

const (
	FormatJSON     InputFormat = iota
	FormatProtobuf
	FormatNumpy
)

func (f InputFormat) String() string {
	switch f {
	case FormatJSON:
		return "JSON"
	case FormatProtobuf:
		return "Protobuf"
	case FormatNumpy:
		return "Numpy"
	default:
		return "Unknown"
	}
}

// ---------------------------------------------------------------------------
// ModelBackend interface
// ---------------------------------------------------------------------------

// ModelBackend defines the interface for invoking AI models (Triton, ONNX, etc).
type ModelBackend interface {
	Predict(ctx context.Context, req *PredictRequest) (*PredictResponse, error)
	PredictStream(ctx context.Context, req *PredictRequest) (<-chan *PredictResponse, error)
	Healthy(ctx context.Context) error
	Close() error
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
	ModelName    string            `json:"model_name"`
	ModelVersion string            `json:"model_version,omitempty"`
	InputName    string            `json:"input_name,omitempty"`
	InputData    []byte            `json:"input_data"`
	InputFormat  InputFormat       `json:"input_format"`
	OutputNames  []string          `json:"output_names,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Validate checks if the request is valid.
func (r *PredictRequest) Validate() error {
	if r == nil {
		return fmt.Errorf("%w: nil request", ErrInvalidInput)
	}
	if r.ModelName == "" {
		return fmt.Errorf("%w: model_name is required", ErrInvalidInput)
	}
	if len(r.InputData) == 0 {
		return fmt.Errorf("%w: input_data is required", ErrInvalidInput)
	}
	return nil
}

// PredictResponse carries the raw outputs from model inference.
type PredictResponse struct {
	ModelName       string            `json:"model_name"`
	ModelVersion    string            `json:"model_version"`
	Outputs         map[string][]byte `json:"outputs"`
	OutputFormat    InputFormat       `json:"output_format"`
	InferenceTimeMs int64             `json:"inference_time_ms"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

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

	// Case 2: []byte (JSON)
	if b, ok := input.([]byte); ok {
		var raw interface{}
		if err := json.Unmarshal(b, &raw); err != nil {
			return nil, fmt.Errorf("unmarshal json: %w", err)
		}
		return DecodeFloat64Matrix(raw)
	}

	// Case 3: Slice of interface{} (from JSON unmarshal)
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

// ---------------------------------------------------------------------------
// Model Descriptor types (for registration)
// ---------------------------------------------------------------------------

// SchemaField describes a single input/output field.
type SchemaField struct {
	Name        string `json:"name"`
	DataType    string `json:"data_type"`
	Shape       []int  `json:"shape"`
	Description string `json:"description,omitempty"`
}

// IOSchema describes the input or output schema of a model.
type IOSchema struct {
	Fields []SchemaField `json:"fields"`
}

// BackendType identifies the inference backend.
type BackendType string

const (
	BackendTriton     BackendType = "triton"
	BackendTorchServe BackendType = "torchserve"
	BackendONNX       BackendType = "onnx"
	BackendVLLM       BackendType = "vllm"
	BackendHTTP       BackendType = "http"
	BackendOpenAI     BackendType = "openai"
)

// ModelType identifies the kind of model.
type ModelType string

const (
	ModelTypeLLM  ModelType = "llm"
	ModelTypeGNN  ModelType = "gnn"
	ModelTypeBERT ModelType = "bert"
)

// ModelDescriptor is a rich description of a model for registration.
type ModelDescriptor struct {
	ModelID      string            `json:"model_id"`
	ModelVersion string            `json:"model_version"`
	ModelType    ModelType         `json:"model_type"`
	Framework    string            `json:"framework"`
	BackendType  BackendType       `json:"backend_type"`
	Endpoint     string            `json:"endpoint,omitempty"` // For HTTP/VLLM backends
	InputSchema  IOSchema          `json:"input_schema,omitempty"`
	OutputSchema IOSchema          `json:"output_schema,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}
