package claim_bert

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type TokenizerMockLogger struct {
	logging.Logger
}

func (m *TokenizerMockLogger) Warn(msg string, fields ...logging.Field) {}

func TestNewWordPieceTokenizer_Success(t *testing.T) {
	config := NewClaimBERTConfig()
	tokenizer, err := NewWordPieceTokenizer(config, &TokenizerMockLogger{})
	assert.NoError(t, err)
	assert.NotNil(t, tokenizer)
}

func TestLoadVocab_Success(t *testing.T) {
	// Create dummy vocab file
	file, err := os.CreateTemp("", "vocab.txt")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	file.WriteString("[UNK]\n[CLS]\n[SEP]\n[PAD]\n[MASK]\nhello\nworld\n##lo\n")
	file.Close()

	config := NewClaimBERTConfig()
	tokenizer, _ := NewWordPieceTokenizer(config, &TokenizerMockLogger{})
	err = tokenizer.LoadVocab(file.Name())
	assert.NoError(t, err)
	assert.Equal(t, 8, tokenizer.VocabSize())
}

func TestTokenize_Simple(t *testing.T) {
	config := NewClaimBERTConfig()
	tokenizer, _ := NewWordPieceTokenizer(config, &TokenizerMockLogger{})
	// Mock vocab manually
	tokenizer.vocab = map[string]int{
		"[UNK]": 0, "[CLS]": 1, "[SEP]": 2, "[PAD]": 3,
		"hello": 4, "world": 5,
	}

	output, err := tokenizer.Tokenize("hello world")
	assert.NoError(t, err)
	assert.Equal(t, []string{"hello", "world"}, output.Tokens)
}

func TestEncode_Padding(t *testing.T) {
	config := NewClaimBERTConfig()
	config.MaxSequenceLength = 10
	tokenizer, _ := NewWordPieceTokenizer(config, &TokenizerMockLogger{})
	tokenizer.vocab = map[string]int{
		"[UNK]": 0, "[CLS]": 1, "[SEP]": 2, "[PAD]": 3,
		"hello": 4,
	}

	encoded, err := tokenizer.Encode("hello")
	assert.NoError(t, err)
	assert.Equal(t, 10, len(encoded.InputIDs))
	assert.Equal(t, 1, encoded.InputIDs[0]) // CLS
	assert.Equal(t, 4, encoded.InputIDs[1]) // hello
	assert.Equal(t, 2, encoded.InputIDs[2]) // SEP
	assert.Equal(t, 3, encoded.InputIDs[3]) // PAD
}

func TestDecode_Simple(t *testing.T) {
	config := NewClaimBERTConfig()
	tokenizer, _ := NewWordPieceTokenizer(config, &TokenizerMockLogger{})
	tokenizer.vocab = map[string]int{
		"[UNK]": 0, "[CLS]": 1, "[SEP]": 2, "[PAD]": 3,
		"he": 4, "##llo": 5, "world": 6,
	}
	tokenizer.inverseVocab = map[int]string{
		0: "[UNK]", 1: "[CLS]", 2: "[SEP]", 3: "[PAD]",
		4: "he", 5: "##llo", 6: "world",
	}

	ids := []int{1, 4, 5, 6, 2, 3, 3}
	text, err := tokenizer.Decode(ids)
	assert.NoError(t, err)
	assert.Equal(t, "hello world", text)
}

//Personal.AI order the ending
