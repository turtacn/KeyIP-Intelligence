package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

var (
	ErrClientClosed   = errors.New(errors.ErrCodeInternal, "redis client is closed")
	ErrInvalidMode    = errors.New(errors.ErrCodeValidation, "invalid redis mode")
	ErrConnectionFailed = errors.New(errors.ErrCodeDatabaseError, "redis connection failed")
)

type RedisConfig struct {
	Mode            string        `mapstructure:"mode"` // standalone, sentinel, cluster
	Addr            string        `mapstructure:"addr"`
	MasterName      string        `mapstructure:"master_name"`
	SentinelAddrs   []string      `mapstructure:"sentinel_addrs"`
	ClusterAddrs    []string      `mapstructure:"cluster_addrs"`
	Password        string        `mapstructure:"password"`
	Username        string        `mapstructure:"username"`
	DB              int           `mapstructure:"db"`
	PoolSize        int           `mapstructure:"pool_size"`
	MinIdleConns    int           `mapstructure:"min_idle_conns"`
	MaxIdleTime     time.Duration `mapstructure:"max_idle_time"`
	PoolTimeout     time.Duration `mapstructure:"pool_timeout"`
	DialTimeout     time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	TLSEnabled      bool          `mapstructure:"tls_enabled"`
	TLSCertFile     string        `mapstructure:"tls_cert_file"`
	TLSKeyFile      string        `mapstructure:"tls_key_file"`
	TLSCAFile       string        `mapstructure:"tls_ca_file"`
	TLSInsecure     bool          `mapstructure:"tls_insecure"`
	MaxRetries      int           `mapstructure:"max_retries"`
	MinRetryBackoff time.Duration `mapstructure:"min_retry_backoff"`
	MaxRetryBackoff time.Duration `mapstructure:"max_retry_backoff"`
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
	var err error

	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, err
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
		if cfg.Mode != "" && cfg.Mode != "standalone" {
			log.Warn("Invalid redis mode, defaulting to standalone", logging.String("mode", cfg.Mode))
		}
		rdb = redis.NewClient(&redis.Options{
			Addr:            cfg.Addr,
			Username:        cfg.Username,
			Password:        cfg.Password,
			DB:              cfg.DB,
			PoolSize:        cfg.PoolSize,
			MinIdleConns:    cfg.MinIdleConns,
			ConnMaxIdleTime: cfg.MaxIdleTime,
			DialTimeout:     cfg.DialTimeout,
			ReadTimeout:     cfg.ReadTimeout,
			WriteTimeout:    cfg.WriteTimeout,
			TLSConfig:       tlsConfig,
			MaxRetries:      cfg.MaxRetries,
			MinRetryBackoff: cfg.MinRetryBackoff,
			MaxRetryBackoff: cfg.MaxRetryBackoff,
		})
	}

	client := &Client{
		rdb:    rdb,
		config: cfg,
		logger: log,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		rdb.Close()
		return nil, ErrConnectionFailed
	}

	log.Info("Redis client connected",
		logging.String("mode", cfg.Mode),
		logging.String("addr", cfg.Addr),
	)

	return client, nil
}

func applyDefaults(cfg *RedisConfig) {
	if cfg.PoolSize == 0 {
		cfg.PoolSize = 10 * runtime.GOMAXPROCS(0)
	}
	if cfg.MinIdleConns == 0 {
		cfg.MinIdleConns = 5
	}
	if cfg.MaxIdleTime == 0 {
		cfg.MaxIdleTime = 5 * time.Minute
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
	if !cfg.TLSEnabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.TLSInsecure,
	}

	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load tls keypair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if cfg.TLSCAFile != "" {
		caCert, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read ca cert: %w", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

func (c *Client) Ping(ctx context.Context) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrClientClosed
	}
	c.mu.RUnlock()
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	err := c.rdb.Close()
	if err == nil {
		c.logger.Info("Closed Redis client")
	} else {
		c.logger.Error("Failed to close Redis client", logging.Err(err))
	}
	return err
}

func (c *Client) GetUnderlyingClient() redis.UniversalClient {
	return c.rdb
}

func (c *Client) PoolStats() *redis.PoolStats {
	return c.rdb.PoolStats()
}

func (c *Client) IsCluster() bool {
	_, ok := c.rdb.(*redis.ClusterClient)
	return ok
}

func (c *Client) Pipeline() redis.Pipeliner {
	return c.rdb.Pipeline()
}

func (c *Client) TxPipeline() redis.Pipeliner {
	return c.rdb.TxPipeline()
}

// Commands

func (c *Client) Get(ctx context.Context, key string) *redis.StringCmd {
	if c.isClosed() {
		return errorStringCmd(ErrClientClosed)
	}
	return c.rdb.Get(ctx, key)
}

func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	if c.isClosed() {
		return errorStatusCmd(ErrClientClosed)
	}
	return c.rdb.Set(ctx, key, value, expiration)
}

func (c *Client) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	if c.isClosed() {
		return errorIntCmd(ErrClientClosed)
	}
	return c.rdb.Del(ctx, keys...)
}

func (c *Client) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	if c.isClosed() {
		return errorIntCmd(ErrClientClosed)
	}
	return c.rdb.Exists(ctx, keys...)
}

func (c *Client) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	if c.isClosed() {
		return errorBoolCmd(ErrClientClosed)
	}
	return c.rdb.Expire(ctx, key, expiration)
}

func (c *Client) TTL(ctx context.Context, key string) *redis.DurationCmd {
	if c.isClosed() {
		return errorDurationCmd(ErrClientClosed)
	}
	return c.rdb.TTL(ctx, key)
}

func (c *Client) Incr(ctx context.Context, key string) *redis.IntCmd {
	if c.isClosed() {
		return errorIntCmd(ErrClientClosed)
	}
	return c.rdb.Incr(ctx, key)
}

func (c *Client) IncrBy(ctx context.Context, key string, value int64) *redis.IntCmd {
	if c.isClosed() {
		return errorIntCmd(ErrClientClosed)
	}
	return c.rdb.IncrBy(ctx, key, value)
}

func (c *Client) Decr(ctx context.Context, key string) *redis.IntCmd {
	if c.isClosed() {
		return errorIntCmd(ErrClientClosed)
	}
	return c.rdb.Decr(ctx, key)
}

func (c *Client) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	if c.isClosed() {
		return errorStringCmd(ErrClientClosed)
	}
	return c.rdb.HGet(ctx, key, field)
}

func (c *Client) HSet(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	if c.isClosed() {
		return errorIntCmd(ErrClientClosed)
	}
	return c.rdb.HSet(ctx, key, values...)
}

func (c *Client) HGetAll(ctx context.Context, key string) *redis.MapStringStringCmd {
	if c.isClosed() {
		// Need error helper for MapStringStringCmd
		cmd := redis.NewMapStringStringCmd(ctx)
		cmd.SetErr(ErrClientClosed)
		return cmd
	}
	return c.rdb.HGetAll(ctx, key)
}

func (c *Client) HDel(ctx context.Context, key string, fields ...string) *redis.IntCmd {
	if c.isClosed() {
		return errorIntCmd(ErrClientClosed)
	}
	return c.rdb.HDel(ctx, key, fields...)
}

func (c *Client) ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd {
	if c.isClosed() {
		return errorIntCmd(ErrClientClosed)
	}
	return c.rdb.ZAdd(ctx, key, members...)
}

func (c *Client) ZRangeByScore(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.StringSliceCmd {
	if c.isClosed() {
		cmd := redis.NewStringSliceCmd(ctx)
		cmd.SetErr(ErrClientClosed)
		return cmd
	}
	return c.rdb.ZRangeByScore(ctx, key, opt)
}

func (c *Client) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd {
	if c.isClosed() {
		cmd := redis.NewZSliceCmd(ctx)
		cmd.SetErr(ErrClientClosed)
		return cmd
	}
	return c.rdb.ZRevRangeWithScores(ctx, key, start, stop)
}

func (c *Client) ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	if c.isClosed() {
		return errorIntCmd(ErrClientClosed)
	}
	return c.rdb.ZRem(ctx, key, members...)
}

func (c *Client) ZScore(ctx context.Context, key, member string) *redis.FloatCmd {
	if c.isClosed() {
		cmd := redis.NewFloatCmd(ctx)
		cmd.SetErr(ErrClientClosed)
		return cmd
	}
	return c.rdb.ZScore(ctx, key, member)
}

func (c *Client) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	if c.isClosed() {
		cmd := redis.NewScanCmd(ctx, nil)
		cmd.SetErr(ErrClientClosed)
		return cmd
	}
	return c.rdb.Scan(ctx, cursor, match, count)
}

func (c *Client) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	if c.isClosed() {
		cmd := redis.NewCmd(ctx)
		cmd.SetErr(ErrClientClosed)
		return cmd
	}
	return c.rdb.Eval(ctx, script, keys, args...)
}

func (c *Client) EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) *redis.Cmd {
	if c.isClosed() {
		cmd := redis.NewCmd(ctx)
		cmd.SetErr(ErrClientClosed)
		return cmd
	}
	return c.rdb.EvalSha(ctx, sha1, keys, args...)
}

func (c *Client) ScriptLoad(ctx context.Context, script string) *redis.StringCmd {
	if c.isClosed() {
		return errorStringCmd(ErrClientClosed)
	}
	return c.rdb.ScriptLoad(ctx, script)
}

// Helper methods

func (c *Client) isClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closed
}

func errorStringCmd(err error) *redis.StringCmd {
	cmd := redis.NewStringCmd(context.Background())
	cmd.SetErr(err)
	return cmd
}

func errorStatusCmd(err error) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(context.Background())
	cmd.SetErr(err)
	return cmd
}

func errorIntCmd(err error) *redis.IntCmd {
	cmd := redis.NewIntCmd(context.Background())
	cmd.SetErr(err)
	return cmd
}

func errorBoolCmd(err error) *redis.BoolCmd {
	cmd := redis.NewBoolCmd(context.Background())
	cmd.SetErr(err)
	return cmd
}

func errorDurationCmd(err error) *redis.DurationCmd {
	cmd := redis.NewDurationCmd(context.Background(), 0)
	cmd.SetErr(err)
	return cmd
}
//Personal.AI order the ending
