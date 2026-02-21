package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	Mode                         string        `mapstructure:"mode"`
	Addr                         string        `mapstructure:"addr"`
	MasterName                   string        `mapstructure:"master_name"`
	SentinelAddrs                []string      `mapstructure:"sentinel_addrs"`
	ClusterAddrs                 []string      `mapstructure:"cluster_addrs"`
	Password                     string        `mapstructure:"password"`
	Username                     string        `mapstructure:"username"`
	DB                           int           `mapstructure:"db"`
	PoolSize                     int           `mapstructure:"pool_size"`
	MinIdleConns                 int           `mapstructure:"min_idle_conns"`
	MaxIdleTime                  time.Duration `mapstructure:"max_idle_time"`
	PoolTimeout                  time.Duration `mapstructure:"pool_timeout"`
	DialTimeout                  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout                  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout                 time.Duration `mapstructure:"write_timeout"`
	TLSEnabled                   bool          `mapstructure:"tls_enabled"`
	TLSCertFile                  string        `mapstructure:"tls_cert_file"`
	TLSKeyFile                   string        `mapstructure:"tls_key_file"`
	TLSCAFile                    string        `mapstructure:"tls_ca_file"`
	TLSInsecure                  bool          `mapstructure:"tls_insecure"`
	MaxRetries                   int           `mapstructure:"max_retries"`
	MinRetryBackoff              time.Duration `mapstructure:"min_retry_backoff"`
	MaxRetryBackoff              time.Duration `mapstructure:"max_retry_backoff"`
}

type Client struct {
	rdb    redis.UniversalClient
	config *RedisConfig
	logger logging.Logger
	mu     sync.RWMutex
	closed bool
}

func NewClient(cfg *RedisConfig, log logging.Logger) (*Client, error) {
	applyDefaults(cfg)

	var rdb redis.UniversalClient
	var tlsConfig *tls.Config
	var err error

	if cfg.TLSEnabled {
		tlsConfig, err = buildTLSConfig(cfg)
		if err != nil {
			return nil, err
		}
	}

	switch cfg.Mode {
	case "cluster":
		rdb = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:           cfg.ClusterAddrs,
			Username:        cfg.Username,
			Password:        cfg.Password,
			PoolSize:        cfg.PoolSize,
			MinIdleConns:    cfg.MinIdleConns,
			ConnMaxIdleTime: cfg.MaxIdleTime,
			PoolTimeout:     cfg.PoolTimeout,
			DialTimeout:     cfg.DialTimeout,
			ReadTimeout:     cfg.ReadTimeout,
			WriteTimeout:    cfg.WriteTimeout,
			TLSConfig:       tlsConfig,
			MaxRetries:      cfg.MaxRetries,
			MinRetryBackoff: cfg.MinRetryBackoff,
			MaxRetryBackoff: cfg.MaxRetryBackoff,
		})
	case "sentinel":
		rdb = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:      cfg.MasterName,
			SentinelAddrs:   cfg.SentinelAddrs,
			Username:        cfg.Username,
			Password:        cfg.Password,
			DB:              cfg.DB,
			PoolSize:        cfg.PoolSize,
			MinIdleConns:    cfg.MinIdleConns,
			ConnMaxIdleTime: cfg.MaxIdleTime,
			PoolTimeout:     cfg.PoolTimeout,
			DialTimeout:     cfg.DialTimeout,
			ReadTimeout:     cfg.ReadTimeout,
			WriteTimeout:    cfg.WriteTimeout,
			TLSConfig:       tlsConfig,
			MaxRetries:      cfg.MaxRetries,
			MinRetryBackoff: cfg.MinRetryBackoff,
			MaxRetryBackoff: cfg.MaxRetryBackoff,
		})
	case "standalone":
		fallthrough
	default:
		if cfg.Mode != "standalone" && cfg.Mode != "" {
			log.Warn("Invalid Redis mode, falling back to standalone", logging.String("mode", cfg.Mode))
		}
		rdb = redis.NewClient(&redis.Options{
			Addr:            cfg.Addr,
			Username:        cfg.Username,
			Password:        cfg.Password,
			DB:              cfg.DB,
			PoolSize:        cfg.PoolSize,
			MinIdleConns:    cfg.MinIdleConns,
			ConnMaxIdleTime: cfg.MaxIdleTime,
			PoolTimeout:     cfg.PoolTimeout,
			DialTimeout:     cfg.DialTimeout,
			ReadTimeout:     cfg.ReadTimeout,
			WriteTimeout:    cfg.WriteTimeout,
			TLSConfig:       tlsConfig,
			MaxRetries:      cfg.MaxRetries,
			MinRetryBackoff: cfg.MinRetryBackoff,
			MaxRetryBackoff: cfg.MaxRetryBackoff,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to connect to redis")
	}

	log.Info("Redis client connected", logging.String("mode", cfg.Mode), logging.String("addr", cfg.Addr))

	return &Client{
		rdb:    rdb,
		config: cfg,
		logger: log,
	}, nil
}

func applyDefaults(cfg *RedisConfig) {
	if cfg.PoolSize == 0 {
		cfg.PoolSize = 10 * 4 // Default GOMAXPROCS assumption
	}
	if cfg.MinIdleConns == 0 {
		cfg.MinIdleConns = 5
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 3 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 3 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.MinRetryBackoff == 0 {
		cfg.MinRetryBackoff = 8 * time.Millisecond
	}
	if cfg.MaxRetryBackoff == 0 {
		cfg.MaxRetryBackoff = 512 * time.Millisecond
	}
}

func buildTLSConfig(cfg *RedisConfig) (*tls.Config, error) {
	config := &tls.Config{
		InsecureSkipVerify: cfg.TLSInsecure,
	}

	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to load redis client cert")
		}
		config.Certificates = []tls.Certificate{cert}
	}

	if cfg.TLSCAFile != "" {
		caCert, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to read redis ca cert")
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		config.RootCAs = caCertPool
	}

	return config, nil
}

var ErrClientClosed = errors.New(errors.ErrCodeDatabaseError, "redis client is closed")

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	c.logger.Info("Closing Redis client")
	return c.rdb.Close()
}

func (c *Client) GetUnderlyingClient() redis.UniversalClient {
	return c.rdb
}

func (c *Client) checkClosed() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return ErrClientClosed
	}
	return nil
}

// Delegate methods

func (c *Client) Get(ctx context.Context, key string) *redis.StringCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewStringCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.Get(ctx, key)
}

func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewStatusCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.Set(ctx, key, value, expiration)
}

func (c *Client) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewIntCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.Del(ctx, keys...)
}

func (c *Client) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewIntCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.Exists(ctx, keys...)
}

func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewBoolCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.Expire(ctx, key, expiration)
}

func (c *Client) TTL(ctx context.Context, key string) *redis.DurationCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewDurationCmd(ctx, 0)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.TTL(ctx, key)
}

func (c *Client) Incr(ctx context.Context, key string) *redis.IntCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewIntCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.Incr(ctx, key)
}

func (c *Client) IncrBy(ctx context.Context, key string, value int64) *redis.IntCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewIntCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.IncrBy(ctx, key, value)
}

func (c *Client) Decr(ctx context.Context, key string) *redis.IntCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewIntCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.Decr(ctx, key)
}

func (c *Client) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewStringCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.HGet(ctx, key, field)
}

func (c *Client) HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewIntCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.HSet(ctx, key, values...)
}

func (c *Client) HGetAll(ctx context.Context, key string) *redis.MapStringStringCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewMapStringStringCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.HGetAll(ctx, key)
}

func (c *Client) HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewIntCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.HDel(ctx, key, fields...)
}

func (c *Client) ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewIntCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.ZAdd(ctx, key, members...)
}

func (c *Client) ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewStringSliceCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.ZRangeByScore(ctx, key, opt)
}

func (c *Client) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewZSliceCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.ZRevRangeWithScores(ctx, key, start, stop)
}

func (c *Client) ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewIntCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.ZRem(ctx, key, members...)
}

func (c *Client) ZScore(ctx context.Context, key, member string) *redis.FloatCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewFloatCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.ZScore(ctx, key, member)
}

func (c *Client) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewScanCmd(ctx, nil)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.Scan(ctx, cursor, match, count)
}

func (c *Client) Pipeline() redis.Pipeliner {
	return c.rdb.Pipeline()
}

func (c *Client) TxPipeline() redis.Pipeliner {
	return c.rdb.TxPipeline()
}

func (c *Client) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.Eval(ctx, script, keys, args...)
}

func (c *Client) EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) *redis.Cmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.EvalSha(ctx, sha1, keys, args...)
}

func (c *Client) ScriptLoad(ctx context.Context, script string) *redis.StringCmd {
	if err := c.checkClosed(); err != nil {
		cmd := redis.NewStringCmd(ctx)
		cmd.SetErr(err)
		return cmd
	}
	return c.rdb.ScriptLoad(ctx, script)
}

//Personal.AI order the ending
