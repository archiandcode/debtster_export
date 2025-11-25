package clients

import (
	"context"
	"os"
	"time"

	"debtster-export/pkg/cache/redis"
)

type RedisConfig struct {
	Addr        string
	Password    string
	DB          int
	MaxRetries  int
	DialTimeout time.Duration
	Timeout     time.Duration

	Prefix string
}

type RedisClient struct {
	raw    *redis.Client
	prefix string
}

func NewRedisClient(cfg RedisConfig) (*RedisClient, error) {
	rdb, err := redis.NewRedisConnection(redis.ConnectionInfo{
		Addr:        cfg.Addr,
		Password:    cfg.Password,
		DB:          cfg.DB,
		MaxRetries:  cfg.MaxRetries,
		DialTimeout: cfg.DialTimeout,
		Timeout:     cfg.Timeout,
	})
	if err != nil {
		return nil, err
	}

	prefix := cfg.Prefix
	if prefix == "" {
		if envPrefix := os.Getenv("REDIS_PREFIX"); envPrefix != "" {
			prefix = envPrefix
		} else {
			prefix = "debtster_database_"
		}
	}

	return &RedisClient{
		raw:    rdb,
		prefix: prefix,
	}, nil
}

func (c *RedisClient) Close() {
	if c.raw == nil {
		return
	}
	redis.Close(c.raw)
}

func (c *RedisClient) withPrefix(key string) string {
	return c.prefix + key
}

func (c *RedisClient) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return c.raw.Set(ctx, c.withPrefix(key), value, ttl).Err()
}

func (c *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return c.raw.Get(ctx, c.withPrefix(key)).Result()
}

func (c *RedisClient) SAdd(ctx context.Context, key string, members ...any) error {
	return c.raw.SAdd(ctx, c.withPrefix(key), members...).Err()
}

func (c *RedisClient) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.raw.SMembers(ctx, c.withPrefix(key)).Result()
}
