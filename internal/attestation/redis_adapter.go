package attestation

import (
	"context"
	"time"

	attestredis "github.com/kacy/device-attestation/redis"
	"github.com/redis/go-redis/v9"
)

// redisAdapter wraps a go-redis client to satisfy the attestredis.Cmdable interface.
type redisAdapter struct {
	client *redis.Client
}

func newRedisAdapter(client *redis.Client) *redisAdapter {
	return &redisAdapter{client: client}
}

func (r *redisAdapter) Get(ctx context.Context, key string) attestredis.StringCmd {
	return r.client.Get(ctx, key)
}

func (r *redisAdapter) Set(ctx context.Context, key string, value any, expiration time.Duration) attestredis.StatusCmd {
	return r.client.Set(ctx, key, value, expiration)
}

func (r *redisAdapter) SetNX(ctx context.Context, key string, value any, expiration time.Duration) attestredis.BoolCmd {
	return r.client.SetNX(ctx, key, value, expiration)
}

func (r *redisAdapter) Del(ctx context.Context, keys ...string) attestredis.IntCmd {
	return r.client.Del(ctx, keys...)
}

func (r *redisAdapter) Incr(ctx context.Context, key string) attestredis.IntCmd {
	return r.client.Incr(ctx, key)
}

func (r *redisAdapter) HSet(ctx context.Context, key string, values ...any) attestredis.IntCmd {
	return r.client.HSet(ctx, key, values...)
}

func (r *redisAdapter) HGet(ctx context.Context, key, field string) attestredis.StringCmd {
	return r.client.HGet(ctx, key, field)
}

func (r *redisAdapter) HGetAll(ctx context.Context, key string) attestredis.MapStringStringCmd {
	return r.client.HGetAll(ctx, key)
}

func (r *redisAdapter) HIncrBy(ctx context.Context, key, field string, incr int64) attestredis.IntCmd {
	return r.client.HIncrBy(ctx, key, field, incr)
}

func (r *redisAdapter) Expire(ctx context.Context, key string, expiration time.Duration) attestredis.BoolCmd {
	return r.client.Expire(ctx, key, expiration)
}
