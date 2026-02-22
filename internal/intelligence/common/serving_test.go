package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGRPCServingClient(t *testing.T) {
	_, err := NewGRPCServingClient([]string{"localhost:50051"}, &MockLogger{})
	assert.NoError(t, err)
}

func TestNewHTTPServingClient(t *testing.T) {
	_, err := NewHTTPServingClient("http://localhost:8080", &MockLogger{})
	assert.NoError(t, err)
}

func TestMockServingClient(t *testing.T) {
	client := NewMockServingClient()
	client.PredictFunc = func(ctx context.Context, req *PredictRequest) (*PredictResponse, error) {
		return &PredictResponse{ModelName: "mocked"}, nil
	}

	res, err := client.Predict(context.Background(), &PredictRequest{ModelName: "test"})
	assert.NoError(t, err)
	assert.Equal(t, "mocked", res.ModelName)
}
//Personal.AI order the ending
