package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a Redis-based implementation of MappingStore
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
	prefix string
}

// NewRedisStore creates a new Redis-based mapping store
func NewRedisStore(address, password string, db int, ttl time.Duration) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStore{
		client: client,
		ttl:    ttl,
		prefix: "llm-secret:",
	}, nil
}

// Store saves a new secret-placeholder mapping
func (r *RedisStore) Store(placeholder, secret string) error {
	ctx := context.Background()

	// Store placeholder -> secret mapping
	key := r.prefix + "p:" + placeholder
	if err := r.client.Set(ctx, key, secret, r.ttl).Err(); err != nil {
		return err
	}

	// Store secret -> placeholder reverse mapping
	reverseKey := r.prefix + "s:" + secret
	if err := r.client.Set(ctx, reverseKey, placeholder, r.ttl).Err(); err != nil {
		return err
	}

	return nil
}

// Lookup retrieves a secret by its placeholder
func (r *RedisStore) Lookup(placeholder string) (string, bool) {
	ctx := context.Background()
	key := r.prefix + "p:" + placeholder

	secret, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false
	}
	if err != nil {
		return "", false
	}

	// Refresh TTL on access
	r.client.Expire(ctx, key, r.ttl)

	return secret, true
}

// LookupBySecret retrieves a placeholder by the secret value
func (r *RedisStore) LookupBySecret(secret string) (string, bool) {
	ctx := context.Background()
	key := r.prefix + "s:" + secret

	placeholder, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false
	}
	if err != nil {
		return "", false
	}

	// Refresh TTL on access
	r.client.Expire(ctx, key, r.ttl)

	return placeholder, true
}

// Touch updates the TTL for a mapping
func (r *RedisStore) Touch(placeholder string) error {
	ctx := context.Background()
	key := r.prefix + "p:" + placeholder
	return r.client.Expire(ctx, key, r.ttl).Err()
}

// Cleanup is a no-op for Redis as TTL handles expiration
func (r *RedisStore) Cleanup() error {
	// Redis handles expiration automatically via TTL
	return nil
}

// Size returns the approximate number of stored mappings
func (r *RedisStore) Size() int {
	ctx := context.Background()
	keys, err := r.client.Keys(ctx, r.prefix+"p:*").Result()
	if err != nil {
		return 0
	}
	return len(keys)
}

// Close closes the Redis connection
func (r *RedisStore) Close() error {
	return r.client.Close()
}
