package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var (
	ErrConfigFileNotFound = errors.New("config file not found")
	ErrConfigParseError   = errors.New("config file parse error")
	ErrConfigValidation   = errors.New("config validation error")
)

type loaderOptions struct {
	configPath  string
	configName  string
	configType  string
	envPrefix   string
	searchPaths []string
	overrides   map[string]interface{}
}

type LoaderOption func(*loaderOptions)

func WithConfigPath(path string) LoaderOption {
	return func(o *loaderOptions) {
		o.configPath = path
	}
}

func WithConfigName(name string) LoaderOption {
	return func(o *loaderOptions) {
		o.configName = name
	}
}

func WithConfigType(typ string) LoaderOption {
	return func(o *loaderOptions) {
		o.configType = typ
	}
}

func WithEnvPrefix(prefix string) LoaderOption {
	return func(o *loaderOptions) {
		o.envPrefix = prefix
	}
}

func WithSearchPaths(paths ...string) LoaderOption {
	return func(o *loaderOptions) {
		o.searchPaths = paths
	}
}

func WithOverrides(overrides map[string]interface{}) LoaderOption {
	return func(o *loaderOptions) {
		o.overrides = overrides
	}
}

// Load loads configuration from various sources.
func Load(opts ...LoaderOption) (*Config, error) {
	options := &loaderOptions{
		configName:  "config",
		configType:  "yaml",
		envPrefix:   "KEYIP",
		searchPaths: []string{".", "./configs", "/etc/keyip", "$HOME/.keyip"},
		overrides:   make(map[string]interface{}),
	}

	for _, opt := range opts {
		opt(options)
	}

	v := viper.New()
	v.SetConfigName(options.configName)
	v.SetConfigType(options.configType)
	v.SetEnvPrefix(options.envPrefix)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	if options.configPath != "" {
		v.SetConfigFile(options.configPath)
	} else {
		for _, path := range options.searchPaths {
			v.AddConfigPath(path)
		}
	}

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			if options.configPath != "" {
				return nil, fmt.Errorf("%w: %s", ErrConfigFileNotFound, options.configPath)
			}
		} else {
			return nil, fmt.Errorf("%w: %v", ErrConfigParseError, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConfigParseError, err)
	}

	ApplyDefaults(&cfg)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConfigValidation, err)
	}

	Set(&cfg)
	return &cfg, nil
}

func LoadFromFile(path string) (*Config, error) {
	return Load(WithConfigPath(path))
}

func LoadFromEnv() (*Config, error) {
	return Load()
}

func MustLoad(opts ...LoaderOption) *Config {
	cfg, err := Load(opts...)
	if err != nil {
		panic(err)
	}
	return cfg
}

func WatchConfig(callback func(*Config)) error {
	viper.OnConfigChange(func(e fsnotify.Event) {
		cfg, err := Load()
		if err == nil {
			callback(cfg)
		}
	})
	viper.WatchConfig()
	return nil
}

// //Personal.AI order the ending
