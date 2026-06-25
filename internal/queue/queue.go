package queue

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrEmpty is returned by Pop when no job became available before the
// timeout elapsed.
var ErrEmpty = errors.New("queue: no job available")

// Queue is a simple FIFO job queue backed by a Redis list.
type Queue struct {
	client *redis.Client
	key    string
}

// New wraps an existing Redis client. key is the Redis list name to use.
func New(client *redis.Client, key string) *Queue {
	return &Queue{client: client, key: key}
}

// Push adds a job ID to the queue.
func (q *Queue) Push(ctx context.Context, jobID string) error {
	return q.client.LPush(ctx, q.key, jobID).Err()
}

// Pop blocks for up to timeout waiting for a job ID to become available.
// Returns ErrEmpty if nothing arrived within that time.
func (q *Queue) Pop(ctx context.Context, timeout time.Duration) (string, error) {
	result, err := q.client.BRPop(ctx, timeout, q.key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrEmpty
	}
	if err != nil {
		return "", err
	}
	return result[1], nil
}
