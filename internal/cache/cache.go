// Package cache is the Redis layer in front of Postgres. It holds three kinds
// of ephemeral state that make horizontal scaling cheap and bounded:
//
//   - result cache: options_hash -> scan id, so identical requests within the
//     TTL skip a re-scan;
//   - robots.txt cache: avoids re-fetching robots for bursts on the same target;
//   - throttle counters + progress: per-user / global in-flight caps and live
//     scan progress for SSE, shared across all stateless app instances.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	rdb *redis.Client
}

func New(ctx context.Context, redisURL string) (*Cache, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &Cache{rdb: rdb}, nil
}

// Client is shared with the asynq queue/scheduler.
func (c *Cache) Client() *redis.Client { return c.rdb }

func (c *Cache) Close() error { return c.rdb.Close() }

// --- Result cache (options_hash -> scan id) ---

func scanKey(hash string) string { return "cache:scan:" + hash }

func (c *Cache) PutScanID(ctx context.Context, optionsHash, scanID string, ttl time.Duration) error {
	return c.rdb.Set(ctx, scanKey(optionsHash), scanID, ttl).Err()
}

func (c *Cache) GetScanID(ctx context.Context, optionsHash string) (string, bool, error) {
	v, err := c.rdb.Get(ctx, scanKey(optionsHash)).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// --- robots.txt cache ---

func robotsKey(target string) string { return "cache:robots:" + target }

func (c *Cache) GetRobots(ctx context.Context, target string) ([]string, bool) {
	v, err := c.rdb.Get(ctx, robotsKey(target)).Bytes()
	if err != nil {
		return nil, false
	}
	var paths []string
	if json.Unmarshal(v, &paths) != nil {
		return nil, false
	}
	return paths, true
}

func (c *Cache) SetRobots(ctx context.Context, target string, paths []string, ttl time.Duration) {
	b, err := json.Marshal(paths)
	if err == nil {
		c.rdb.Set(ctx, robotsKey(target), b, ttl)
	}
}

// --- Progress (for SSE) ---

func progressKey(scanID string) string { return "progress:" + scanID }

// SetProgress publishes scan progress so any instance's SSE handler can read it.
func (c *Cache) SetProgress(ctx context.Context, scanID string, done, total int) {
	c.rdb.HSet(ctx, progressKey(scanID), "done", done, "total", total)
	c.rdb.Expire(ctx, progressKey(scanID), 15*time.Minute)
}

func (c *Cache) GetProgress(ctx context.Context, scanID string) (done, total int, ok bool) {
	m, err := c.rdb.HGetAll(ctx, progressKey(scanID)).Result()
	if err != nil || len(m) == 0 {
		return 0, 0, false
	}
	done, _ = strconv.Atoi(m["done"])
	total, _ = strconv.Atoi(m["total"])
	return done, total, true
}

// --- Throttle counters ---

const inflightTTL = 30 * time.Minute // safety net so a crashed job can't leak a slot forever

func userKey(userID string) string { return "inflight:user:" + userID }

const globalKey = "inflight:global"

// TryAcquire atomically reserves an in-flight slot for userID, enforcing both
// the per-user and global caps. It returns ok=false (with no reservation held)
// when either cap would be exceeded. A cap of 0 means unlimited. Release must be
// called exactly once for every ok=true.
func (c *Cache) TryAcquire(ctx context.Context, userID string, maxPerUser, maxGlobal int) (bool, error) {
	uCount, err := c.rdb.Incr(ctx, userKey(userID)).Result()
	if err != nil {
		return false, err
	}
	c.rdb.Expire(ctx, userKey(userID), inflightTTL)
	if maxPerUser > 0 && int(uCount) > maxPerUser {
		c.rdb.Decr(ctx, userKey(userID))
		return false, nil
	}

	gCount, err := c.rdb.Incr(ctx, globalKey).Result()
	if err != nil {
		c.rdb.Decr(ctx, userKey(userID))
		return false, err
	}
	c.rdb.Expire(ctx, globalKey, inflightTTL)
	if maxGlobal > 0 && int(gCount) > maxGlobal {
		c.rdb.Decr(ctx, globalKey)
		c.rdb.Decr(ctx, userKey(userID))
		return false, nil
	}
	return true, nil
}

// Release frees a previously acquired in-flight slot. Counters are floored at 0.
func (c *Cache) Release(ctx context.Context, userID string) {
	if n, err := c.rdb.Decr(ctx, userKey(userID)).Result(); err == nil && n < 0 {
		c.rdb.Set(ctx, userKey(userID), 0, inflightTTL)
	}
	if n, err := c.rdb.Decr(ctx, globalKey).Result(); err == nil && n < 0 {
		c.rdb.Set(ctx, globalKey, 0, inflightTTL)
	}
}
