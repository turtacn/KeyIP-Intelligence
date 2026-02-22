package claim_bert

import (
	"fmt"
	"math/bits"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Task Head Name Constants
// ---------------------------------------------------------------------------

const (
	// TaskClaimClassification classifies claim type:
	// Independent / Dependent / Method / Product / Use → 5 classes.
	TaskClaimClassification = "ClaimClassification"

	// TaskFeatureExtraction identifies technical feature boundaries
	// via BIO sequence labeling (B-COMPOUND, I-COMPOUND, B-CONDITION,
	// I-CONDITION, B-PROPERTY, I-PROPERTY, B-MARKUSH, I-MARKUSH, O).
	TaskFeatureExtraction = "FeatureExtraction"

	// TaskScopeAnalysis evaluates claim scope breadth as a regression
	// score in [0, 1] where 1 = broadest possible scope.
	TaskScopeAnalysis = "ScopeAnalysis"

	// TaskDependencyParsing resolves claim dependency pointers
	// (which parent claim does a dependent claim refer to).
	TaskDependencyParsing = "DependencyParsing"
)

// Claim classification output dimension (5 classes).
const claimClassificationOutputDim = 5

// BIO label count for feature extraction:
// B-COMPOUND, I-COMPOUND, B-CONDITION, I-CONDITION,
// B-PROPERTY, I-PROPERTY, B-MARKUSH, I-MARKUSH, O = 9.
const featureExtractionOutputDim = 9

// Scope analysis is a single regression score.
const scopeAnalysisOutputDim = 1

// Dependency parsing: pointer logits over max sequence positions.
// At inference time the actual dim equals the number of claims in the
// document, but the head is configured with a fixed upper-bound.
const dependencyParsingOutputDim = 128

// ---------------------------------------------------------------------------
// Pooling strategies
// ---------------------------------------------------------------------------

const (
	PoolingCLS  = "CLS"
	PoolingMean = "Mean"
	PoolingMax  = "Max"
)

// validPoolingStrategies is the set of accepted pooling strategy names.
var validPoolingStrategies = map[string]bool{
	PoolingCLS:  true,
	PoolingMean: true,
	PoolingMax:  true,
}

// ---------------------------------------------------------------------------
// Activation types
// ---------------------------------------------------------------------------

const (
	ActivationSoftmax = "Softmax"
	ActivationSigmoid = "Sigmoid"
	ActivationLinear  = "Linear"
)

// ---------------------------------------------------------------------------
// BackendType alias (re-exported from common for convenience)
// ---------------------------------------------------------------------------

// BackendType identifies the inference backend (e.g. TorchServe, Triton, ONNX).
type BackendType = common.BackendType

// ---------------------------------------------------------------------------
// TaskHeadConfig
// ---------------------------------------------------------------------------

// TaskHeadConfig describes a single task-specific output head that sits on
// top of the shared BERT encoder. Multi-task learning allows the encoder to
// capture richer representations while each head specialises in one aspect
// of patent claim understanding.
type TaskHeadConfig struct {
	// TaskName uniquely identifies the task.
	// One of: ClaimClassification, FeatureExtraction, ScopeAnalysis, DependencyParsing.
	TaskName string `json:"task_name" yaml:"task_name"`

	// OutputDim is the dimensionality of the head's output layer.
	OutputDim int `json:"output_dim" yaml:"output_dim"`

	// ActivationType is the final activation applied to the head output.
	// Softmax for classification, Sigmoid for multi-label, Linear for regression.
	ActivationType string `json:"activation_type" yaml:"activation_type"`

	// Enabled controls whether this head is active during inference.
	// Disabling unused heads saves compute.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// LossWeight is the relative weight of this head's loss during training.
	// Ignored at inference time but stored for reproducibility.
	LossWeight float64 `json:"loss_weight,omitempty" yaml:"loss_weight,omitempty"`

	// Description is a human-readable summary of the task.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

// ---------------------------------------------------------------------------
// ClaimBERTConfig
// ---------------------------------------------------------------------------

// ClaimBERTConfig holds every tuneable parameter of the ClaimBERT model.
//
// ClaimBERT is a BERT-based model fine-tuned on chemical patent claim text.
// It understands:
//   - Claim hierarchy (independent vs. dependent)
//   - Technical feature boundaries
//   - Markush structure descriptions ("selected from the group consisting of …")
//   - Legal scope language ("comprising", "consisting of", "consisting essentially of")
//   - Multi-language claims (zh / en / ja / ko / de)
//
// The extended vocabulary (default 31 000 tokens) covers standard BERT
// word-pieces plus domain-specific additions for chemical nomenclature
// (alkyl, aryl, heteroaryl …), Markush keywords, and patent-law terms.
type ClaimBERTConfig struct {
	// ModelID uniquely identifies this model instance.
	// Convention: claim-bert-v{major}.{minor}.{patch}
	ModelID string `json:"model_id" yaml:"model_id"`

	// ModelPath is the filesystem or object-store path to the serialised weights.
	ModelPath string `json:"model_path" yaml:"model_path"`

	// BackendType selects the serving infrastructure (Triton, TorchServe, ONNX …).
	BackendType BackendType `json:"backend_type" yaml:"backend_type"`

	// MaxSequenceLength is the maximum number of tokens the encoder accepts.
	// Must be a power of 2 and ≤ 2048. Default 512.
	MaxSequenceLength int `json:"max_sequence_length" yaml:"max_sequence_length"`

	// HiddenDim is the width of every hidden layer in the Transformer.
	// Must be divisible by NumAttentionHeads. Default 768.
	HiddenDim int `json:"hidden_dim" yaml:"hidden_dim"`

	// NumAttentionHeads is the number of parallel attention heads. Default 12.
	NumAttentionHeads int `json:"num_attention_heads" yaml:"num_attention_heads"`

	// NumLayers is the number of stacked Transformer encoder layers. Default 12.
	NumLayers int `json:"num_layers" yaml:"num_layers"`

	// VocabSize is the total vocabulary size including domain-specific extensions.
	// Default 31 000 (≈ 30 522 standard BERT + ~478 chemical-patent tokens).
	VocabSize int `json:"vocab_size" yaml:"vocab_size"`

	// PoolingStrategy determines how token-level representations are aggregated
	// into a single sentence-level vector. One of CLS / Mean / Max. Default CLS.
	PoolingStrategy string `json:"pooling_strategy" yaml:"pooling_strategy"`

	// TaskHeads lists the multi-task output heads. At least one must be enabled.
	TaskHeads []TaskHeadConfig `json:"task_heads" yaml:"task_heads"`

	// TimeoutMs is the per-request inference timeout in milliseconds. Default 3000.
	TimeoutMs int `json:"timeout_ms" yaml:"timeout_ms"`

	// MaxBatchSize is the maximum number of claims processed in one batch. Default 32.
	MaxBatchSize int `json:"max_batch_size" yaml:"max_batch_size"`

	// Labels are arbitrary key-value metadata attached to the model.
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

// NewClaimBERTConfig creates a ClaimBERTConfig populated with sensible defaults.
// Any non-zero field in the supplied overrides takes precedence.
func NewClaimBERTConfig(overrides *ClaimBERTConfig) *ClaimBERTConfig {
	cfg := &ClaimBERTConfig{
		ModelID:           "claim-bert-v1.0.0",
		ModelPath:         "",
		BackendType:       common.BackendTriton,
		MaxSequenceLength: 512,
		HiddenDim:         768,
		NumAttentionHeads: 12,
		NumLayers:         12,
		VocabSize:         31000,
		PoolingStrategy:   PoolingCLS,
		TaskHeads:         DefaultTaskHeads(),
		TimeoutMs:         3000,
		MaxBatchSize:      32,
	}

	if overrides == nil {
		return cfg
	}

	// Apply non-zero overrides.
	if overrides.ModelID != "" {
		cfg.ModelID = overrides.ModelID
	}
	if overrides.ModelPath != "" {
		cfg.ModelPath = overrides.ModelPath
	}
	if overrides.BackendType != "" {
		cfg.BackendType = overrides.BackendType
	}
	if overrides.MaxSequenceLength > 0 {
		cfg.MaxSequenceLength = overrides.MaxSequenceLength
	}
	if overrides.HiddenDim > 0 {
		cfg.HiddenDim = overrides.HiddenDim
	}
	if overrides.NumAttentionHeads > 0 {
		cfg.NumAttentionHeads = overrides.NumAttentionHeads
	}
	if overrides.NumLayers > 0 {
		cfg.NumLayers = overrides.NumLayers
	}
	if overrides.VocabSize > 0 {
		cfg.VocabSize = overrides.VocabSize
	}
	if overrides.PoolingStrategy != "" {
		cfg.PoolingStrategy = overrides.PoolingStrategy
	}
	if len(overrides.TaskHeads) > 0 {
		cfg.TaskHeads = overrides.TaskHeads
	}
	if overrides.TimeoutMs > 0 {
		cfg.TimeoutMs = overrides.TimeoutMs
	}
	if overrides.MaxBatchSize > 0 {
		cfg.MaxBatchSize = overrides.MaxBatchSize
	}
	if len(overrides.Labels) > 0 {
		cfg.Labels = overrides.Labels
	}

	return cfg
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// Validate checks every field for consistency and returns the first error found.
func (c *ClaimBERTConfig) Validate() error {
	if c.ModelID == "" {
		return errors.NewInvalidInputError("model_id is required")
	}

	// MaxSequenceLength must be a power of 2 and ≤ 2048.
	if c.MaxSequenceLength <= 0 {
		return errors.NewInvalidInputError("max_sequence_length must be positive")
	}
	if !isPowerOfTwo(c.MaxSequenceLength) {
		return errors.NewInvalidInputError(
			fmt.Sprintf("max_sequence_length must be a power of 2, got %d", c.MaxSequenceLength))
	}
	if c.MaxSequenceLength > 2048 {
		return errors.NewInvalidInputError(
			fmt.Sprintf("max_sequence_length must be <= 2048, got %d", c.MaxSequenceLength))
	}

	// HiddenDim must be divisible by NumAttentionHeads.
	if c.HiddenDim <= 0 {
		return errors.NewInvalidInputError("hidden_dim must be positive")
	}
	if c.NumAttentionHeads <= 0 {
		return errors.NewInvalidInputError("num_attention_heads must be positive")
	}
	if c.HiddenDim%c.NumAttentionHeads != 0 {
		return errors.NewInvalidInputError(
			fmt.Sprintf("hidden_dim (%d) must be divisible by num_attention_heads (%d)",
				c.HiddenDim, c.NumAttentionHeads))
	}

	if c.NumLayers <= 0 {
		return errors.NewInvalidInputError("num_layers must be positive")
	}

	// VocabSize > 0.
	if c.VocabSize <= 0 {
		return errors.NewInvalidInputError("vocab_size must be positive")
	}

	// PoolingStrategy must be one of the known strategies.
	if !validPoolingStrategies[c.PoolingStrategy] {
		return errors.NewInvalidInputError(
			fmt.Sprintf("pooling_strategy must be one of [CLS, Mean, Max], got %q", c.PoolingStrategy))
	}

	// At least one TaskHead must be enabled.
	if len(c.TaskHeads) == 0 {
		return errors.NewInvalidInputError("at least one task head is required")
	}
	enabledCount := 0
	for i, th := range c.TaskHeads {
		if th.TaskName == "" {
			return errors.NewInvalidInputError(
				fmt.Sprintf("task_heads[%d].task_name is required", i))
		}
		if th.OutputDim <= 0 {
			return errors.NewInvalidInputError(
				fmt.Sprintf("task_heads[%d].output_dim must be positive", i))
		}
		if th.Enabled {
			enabledCount++
		}
	}
	if enabledCount == 0 {
		return errors.NewInvalidInputError("at least one task head must be enabled")
	}

	if c.TimeoutMs <= 0 {
		return errors.NewInvalidInputError("timeout_ms must be positive")
	}
	if c.MaxBatchSize <= 0 {
		return errors.NewInvalidInputError("max_batch_size must be positive")
	}

	return nil
}

// ---------------------------------------------------------------------------
// Default Task Heads
// ---------------------------------------------------------------------------

// DefaultTaskHeads returns the four pre-defined task heads with sensible defaults.
//
//   ClaimClassification  – 5-class softmax  (Independent / Dependent / Method / Product / Use)
//   FeatureExtraction    – 9-label BIO tagger (B/I for COMPOUND, CONDITION, PROPERTY, MARKUSH + O)
//   ScopeAnalysis        – scalar regression [0,1]
//   DependencyParsing    – pointer logits over up to 128 candidate claims
func DefaultTaskHeads() []TaskHeadConfig {
	return []TaskHeadConfig{
		{
			TaskName:       TaskClaimClassification,
			OutputDim:      claimClassificationOutputDim,
			ActivationType: ActivationSoftmax,
			Enabled:        true,
			LossWeight:     1.0,
			Description:    "Classifies claim type: Independent / Dependent / Method / Product / Use",
		},
		{
			TaskName:       TaskFeatureExtraction,
			OutputDim:      featureExtractionOutputDim,
			ActivationType: ActivationSoftmax,
			Enabled:        true,
			LossWeight:     1.5,
			Description:    "BIO sequence labeling for technical feature boundary identification",
		},
		{
			TaskName:       TaskScopeAnalysis,
			OutputDim:      scopeAnalysisOutputDim,
			ActivationType: ActivationLinear,
			Enabled:        true,
			LossWeight:     0.8,
			Description:    "Regression score [0,1] estimating claim scope breadth",
		},
		{
			TaskName:       TaskDependencyParsing,
			OutputDim:      dependencyParsingOutputDim,
			ActivationType: ActivationSoftmax,
			Enabled:        true,
			LossWeight:     1.0,
			Description:    "Pointer network resolving dependent claim references",
		},
	}
}

// ---------------------------------------------------------------------------
// Registry integration
// ---------------------------------------------------------------------------

// ModelDescriptor returns a structured description of the model suitable for
// registration in a model registry.
func (c *ClaimBERTConfig) ModelDescriptor() common.ModelDescriptor {
	// Build input schema.
	inputSchema := common.IOSchema{
		Fields: []common.SchemaField{
			{Name: "input_ids", DataType: "int64", Shape: []int{-1, c.MaxSequenceLength}, Description: "Token IDs"},
			{Name: "attention_mask", DataType: "int64", Shape: []int{-1, c.MaxSequenceLength}, Description: "Attention mask"},
			{Name: "token_type_ids", DataType: "int64", Shape: []int{-1, c.MaxSequenceLength}, Description: "Segment IDs"},
		},
	}

	// Build output schema from enabled task heads.
	var outputFields []common.SchemaField
	for _, th := range c.TaskHeads {
		if !th.Enabled {
			continue
		}
		shape := []int{-1, th.OutputDim}
		if th.TaskName == TaskFeatureExtraction {
			// Sequence-level output: (batch, seq_len, num_labels).
			shape = []int{-1, c.MaxSequenceLength, th.OutputDim}
		}
		outputFields = append(outputFields, common.SchemaField{
			Name:        strings.ToLower(th.TaskName),
			DataType:    "float32",
			Shape:       shape,
			Description: th.Description,
		})
	}
	outputSchema := common.IOSchema{Fields: outputFields}

	// Extract version from ModelID (claim-bert-v1.0.0 → 1.0.0).
	version := c.ModelID
	if idx := strings.Index(c.ModelID, "-v"); idx >= 0 && idx+2 < len(c.ModelID) {
		version = c.ModelID[idx+2:]
	}

	return common.ModelDescriptor{
		ModelID:      c.ModelID,
		ModelVersion: version,
		ModelType:    "claim-bert",
		Framework:    "pytorch",
		BackendType:  c.BackendType,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Metadata: map[string]string{
			"max_sequence_length": fmt.Sprintf("%d", c.MaxSequenceLength),
			"hidden_dim":          fmt.Sprintf("%d", c.HiddenDim),
			"num_layers":          fmt.Sprintf("%d", c.NumLayers),
			"vocab_size":          fmt.Sprintf("%d", c.VocabSize),
			"pooling_strategy":    c.PoolingStrategy,
			"domain":              "chemical-patent-claims",
			"languages":           "zh,en,ja,ko,de",
		},
	}
}

// RegisterToRegistry validates the config and registers the model descriptor
// with the supplied registry.
func (c *ClaimBERTConfig) RegisterToRegistry(registry common.ModelRegistry) error {
	if registry == nil {
		return errors.NewInvalidInputError("registry is required")
	}
	if err := c.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	desc := c.ModelDescriptor()
	if err := registry.Register(desc); err != nil {
		return fmt.Errorf("registry registration failed: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Convenience accessors
// ---------------------------------------------------------------------------

// EnabledTaskHeads returns only the task heads that are currently enabled.
func (c *ClaimBERTConfig) EnabledTaskHeads() []TaskHeadConfig {
	var out []TaskHeadConfig
	for _, th := range c.TaskHeads {
		if th.Enabled {
			out = append(out, th)
		}
	}
	return out
}

// HasTask reports whether a task head with the given name exists and is enabled.
func (c *ClaimBERTConfig) HasTask(taskName string) bool {
	for _, th := range c.TaskHeads {
		if th.TaskName == taskName && th.Enabled {
			return true
		}
	}
	return false
}

// HeadDimPerAttention returns HiddenDim / NumAttentionHeads.
func (c *ClaimBERTConfig) HeadDimPerAttention() int {
	if c.NumAttentionHeads == 0 {
		return 0
	}
	return c.HiddenDim / c.NumAttentionHeads
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// isPowerOfTwo returns true if n is a positive power of two.
func isPowerOfTwo(n int) bool {
	if n <= 0 {
		return false
	}
	return bits.OnesCount(uint(n)) == 1
}

//Personal.AI order the ending
