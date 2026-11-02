package selfdestruct

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	redisScheduleKey = "selfdestruct.schedule" // sorted set: the identifiers of the tasks by their time
	redisTasksKey    = "selfdestruct.tasks"    // hash: the tasks by their identifiers
)

// RedisQueue is a [Queue] in Redis: the tasks survive a restart of the application.
// The struct holds a single reference, so it's passed by value.
type RedisQueue struct {
	rdb        *redis.Client
	ownsClient bool
}

// ConnectToRedis connects to Redis with a client of its own, which is closed by [RedisQueue.Close], and panics if
// the server is unavailable, like the state storage of the wizards does. Use [NewRedisQueue] to share a client.
func ConnectToRedis(ctx context.Context, options *redis.Options) RedisQueue {
	rdb := redis.NewClient(options)
	if err := rdb.Ping(ctx).Err(); err != nil {
		panic(err)
	}
	return RedisQueue{rdb: rdb, ownsClient: true}
}

// NewRedisQueue creates a [RedisQueue] using the client, which is a pool of connections that may be shared with other
// storages of the application. The client isn't closed by [RedisQueue.Close]: its owner closes it.
func NewRedisQueue(rdb *redis.Client) RedisQueue {
	return RedisQueue{rdb: rdb}
}

// Close closes the client if the queue has created it by itself in [ConnectToRedis].
func (q RedisQueue) Close() error {
	if !q.ownsClient {
		return nil
	}
	return q.rdb.Close()
}

func (q RedisQueue) Add(ctx context.Context, task Task, at time.Time) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	id := task.ID()
	_, err = q.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, redisTasksKey, id, data)
		pipe.ZAdd(ctx, redisScheduleKey, &redis.Z{Score: float64(at.UnixMilli()), Member: id})
		return nil
	})
	return err
}

// Take claims the task by removing it from the schedule first: only the one who has removed it gets the task, so it's
// never executed twice, even by several instances of the application.
func (q RedisQueue) Take(ctx context.Context, id string) (Task, bool, error) {
	removed, err := q.rdb.ZRem(ctx, redisScheduleKey, id).Result()
	if err != nil || removed == 0 {
		return Task{}, false, err
	}
	data, err := q.rdb.HGet(ctx, redisTasksKey, id).Bytes()
	q.rdb.HDel(ctx, redisTasksKey, id)
	if errors.Is(err, redis.Nil) {
		return Task{}, false, nil
	} else if err != nil {
		return Task{}, false, err
	}
	var task Task
	if err = json.Unmarshal(data, &task); err != nil {
		return Task{}, false, err
	}
	return task, true, nil
}

func (q RedisQueue) TakeDue(ctx context.Context, now time.Time, limit int) ([]Task, error) {
	ids, err := q.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     redisScheduleKey,
		Start:   "-inf",
		Stop:    now.UnixMilli(),
		ByScore: true,
		Count:   int64(limit),
	}).Result()
	if err != nil {
		return nil, err
	}
	tasks := make([]Task, 0, len(ids))
	for _, id := range ids {
		task, ok, err := q.Take(ctx, id)
		if err != nil {
			return tasks, err
		}
		if ok {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}
