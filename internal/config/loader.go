// Package config provides configuration loading, defaults, and validation for
// the KeyIP-Intelligence platform.
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// envPrefix is the environment variable prefix used by all platform settings.
const envPrefix = "KEYIP"

// newViper builds a pre-configured Viper instance with the platform's standard
// settings: YAML file type, KEYIP_ env prefix, automatic env binding, and a
// key replacer that maps "." → "_" so that nested keys like "database.host"
// resolve to "KEYIP_DATABASE_HOST".
func newViper() *viper.Viper {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	return v
}

// Load reads the YAML file at configPath, merges any KEYIP_* environment
// variable overrides, applies platform defaults for unset fields, and
// validates the result.  It returns a fully-populated *Config or a
// descriptive error.
func Load(configPath string) (*Config, error) {
	v := newViper()
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config: failed to read config file %q: %w", configPath, err)
	}

	return unmarshalAndFinalize(v)
}

// LoadFromEnv builds a Config entirely from KEYIP_* environment variables,
// with no config file required.  This is the preferred loading strategy for
// containerised (12-factor) deployments.
//
// Environment variable naming convention:
//
//	KEYIP_<SECTION>_<FIELD>   e.g.  KEYIP_DATABASE_HOST, KEYIP_REDIS_ADDR
func LoadFromEnv() (*Config, error) {
	v := newViper()
	// No config file — rely solely on env vars and defaults.
	return unmarshalAndFinalize(v)
}

// unmarshalAndFinalize unmarshals viper state into a Config struct, applies
// defaults, and validates the result.
func unmarshalAndFinalize(v *viper.Viper) (*Config, error) {
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("config: failed to unmarshal configuration: %w", err)
	}

	ApplyDefaults(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: validation failed: %w", err)
	}

	return cfg, nil
}

// Watch monitors configPath for changes and invokes onChange with the newly
// parsed Config whenever the file is modified on disk.  It is intended for
// hot-reloading non-critical settings such as log level and rate-limit
// thresholds; callers are responsible for applying only the safe subset of
// changes at runtime.
//
// Watch is non-blocking; it starts a background goroutine managed by viper.
// If the changed file fails to parse or validate, onChange is NOT called and
// the error is silently swallowed (viper behaviour) — add an OnConfigChange
// hook for custom error handling if needed.
func Watch(configPath string, onChange func(*Config)) {
	v := newViper()
	v.SetConfigFile(configPath)

	// Initial read — errors are ignored here; callers should call Load first.
	_ = v.ReadInConfig()

	v.WatchConfig()
	v.OnConfigChange(func(_ interface{}) {
		cfg, err := unmarshalAndFinalize(v)
		if err != nil {
			// Config change produced an invalid config; skip the callback to
			// prevent the application from entering a broken state.
			return
		}
		onChange(cfg)
	})
}

// MustLoad is a convenience wrapper around Load that panics on any error.
// It is intended for use in main() where a config-load failure is always fatal.
func MustLoad(configPath string) *Config {
	cfg, err := Load(configPath)
	if err != nil {
		panic(fmt.Sprintf("config: MustLoad failed: %v", err))
	}
	return cfg
}

//Personal.AI order the ending
