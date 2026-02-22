package molpatent_gnn

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

type MockLogger struct {
	logging.Logger
}

func (m *MockLogger) Error(msg string, fields ...logging.Field) {}

func TestPreprocessSMILES_Methane(t *testing.T) {
	cfg := NewGNNModelConfig()
	p := NewGNNPreprocessor(cfg, &MockLogger{})

	graph, err := p.PreprocessSMILES(context.Background(), "C")
	assert.NoError(t, err)
	assert.Equal(t, 1, graph.NumAtoms)
	assert.Equal(t, 0, graph.NumBonds)
	assert.Equal(t, 1, len(graph.NodeFeatures))
	// C is index 0
	assert.Equal(t, float32(1.0), graph.NodeFeatures[0][0])
}

func TestPreprocessSMILES_Ethanol(t *testing.T) {
	cfg := NewGNNModelConfig()
	p := NewGNNPreprocessor(cfg, &MockLogger{})

	graph, err := p.PreprocessSMILES(context.Background(), "CCO")
	assert.NoError(t, err)
	assert.Equal(t, 3, graph.NumAtoms) // C, C, O
	assert.Equal(t, 2, graph.NumBonds) // C-C, C-O
	assert.Equal(t, 4, len(graph.EdgeIndex)) // Bidirectional
}

func TestPreprocessSMILES_Benzene(t *testing.T) {
	cfg := NewGNNModelConfig()
	p := NewGNNPreprocessor(cfg, &MockLogger{})

	graph, err := p.PreprocessSMILES(context.Background(), "c1ccccc1")
	assert.NoError(t, err)
	assert.Equal(t, 6, graph.NumAtoms)
	assert.Equal(t, 6, graph.NumBonds)
	// Check aromaticity feature at index 20
	assert.Equal(t, float32(1.0), graph.NodeFeatures[0][20])
}

func TestPreprocessSMILES_Branch(t *testing.T) {
	cfg := NewGNNModelConfig()
	p := NewGNNPreprocessor(cfg, &MockLogger{})

	// C(O)C -> 2-propanol isomer (propan-2-ol if implicit H)
	// C - O (branch)
	// |
	// C
	// Atom 0 (C), Atom 1 (O), Atom 2 (C).
	// Bonds: 0-1, 0-2.
	graph, err := p.PreprocessSMILES(context.Background(), "C(O)C")
	assert.NoError(t, err)
	assert.Equal(t, 3, graph.NumAtoms)
	assert.Equal(t, 2, graph.NumBonds)
}

func TestValidateSMILES(t *testing.T) {
	cfg := NewGNNModelConfig()
	p := NewGNNPreprocessor(cfg, &MockLogger{})

	assert.NoError(t, p.ValidateSMILES("C"))
	assert.ErrorIs(t, p.ValidateSMILES(""), ErrEmptySMILES)
	assert.ErrorIs(t, p.ValidateSMILES("C(C"), ErrUnbalancedParentheses)
	// Removing check for ring closure in ValidateSMILES as it is done in PreprocessSMILES

	// Check PreprocessSMILES catches ring closure error
	_, err := p.PreprocessSMILES(context.Background(), "C1C")
	assert.ErrorIs(t, err, ErrUnmatchedRingClosure)
}

func TestPreprocessBatch(t *testing.T) {
	cfg := NewGNNModelConfig()
	p := NewGNNPreprocessor(cfg, &MockLogger{})

	inputs := []molecule.MoleculeInput{
		{Format: molecule.FormatSMILES, Value: "C"},
		{Format: molecule.FormatSMILES, Value: "O"},
	}
	graphs, err := p.PreprocessBatch(context.Background(), inputs)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(graphs))
	assert.NotNil(t, graphs[0])
	assert.NotNil(t, graphs[1])
}
//Personal.AI order the ending
