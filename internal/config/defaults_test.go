package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyDefaults_EmptyConfig(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	assert.Equal(t, DefaultHTTPHost, cfg.Server.HTTP.Host)
	assert.Equal(t, DefaultHTTPPort, cfg.Server.HTTP.Port)
}

func TestApplyDefaults_PreserveExistingValues(t *testing.T) {
	cfg := &Config{}
	cfg.Server.HTTP.Port = 9999
	ApplyDefaults(cfg)

	assert.Equal(t, 9999, cfg.Server.HTTP.Port)
}

// //Personal.AI order the ending
