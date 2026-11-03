package selfdestruct

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var redisContainer testcontainers.Container

func TestMemoryQueue(t *testing.T) {
	testQueue(t, NewMemoryQueue())
}

func TestRedisQueue(t *testing.T) {
	ctx := context.Background()
	endpoint, err := redisContainer.Endpoint(ctx, "")
	require.NoError(t, err)
	queue := ConnectToRedis(ctx, &redis.Options{Addr: endpoint})
	t.Cleanup(func() {
		queue.rdb.Del(ctx, redisScheduleKey, redisTasksKey)
		assert.NoError(t, queue.Close())
	})
	queue.rdb.Del(ctx, redisScheduleKey, redisTasksKey)

	testQueue(t, queue)
}

func TestNewRedisQueue_SharedClient(t *testing.T) {
	ctx := context.Background()
	endpoint, err := redisContainer.Endpoint(ctx, "")
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: endpoint})
	t.Cleanup(func() { assert.NoError(t, rdb.Close()) })

	assert.NoError(t, NewRedisQueue(rdb).Close())
	assert.NoError(t, rdb.Ping(ctx).Err(), "the shared client must stay open")
}

// testQueue checks the contract of the Queue interface.
func testQueue(t *testing.T, queue Queue) {
	ctx := context.Background()
	now := time.Now()
	first := Task{Kind: KindDelete, ChatID: 1, MessageID: 1, ReplyToMessageID: 10}
	second := Task{Kind: KindReplaceInline, InlineMessageID: "inline", Text: "·"}
	later := Task{Kind: KindDelete, ChatID: 1, MessageID: 3}

	require.NoError(t, queue.Add(ctx, second, now.Add(-time.Second)))
	require.NoError(t, queue.Add(ctx, first, now.Add(-2*time.Second)))
	require.NoError(t, queue.Add(ctx, later, now.Add(time.Hour)))
	n, err := queue.Len(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	due, err := queue.TakeDue(ctx, now, 1)
	require.NoError(t, err)
	assert.Equal(t, []Task{first}, due, "the earliest task, within the limit")

	due, err = queue.TakeDue(ctx, now, 10)
	require.NoError(t, err)
	assert.Equal(t, []Task{second}, due, "the taken tasks are not returned again, the later ones are not due")

	task, ok, err := queue.Take(ctx, later.ID())
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, later, task)

	_, ok, err = queue.Take(ctx, later.ID())
	require.NoError(t, err)
	assert.False(t, ok, "a task is taken only once")

	// a task for the same message replaces the previous one
	require.NoError(t, queue.Add(ctx, first, now.Add(time.Hour)))
	require.NoError(t, queue.Add(ctx, first, now.Add(-time.Second)))
	due, err = queue.TakeDue(ctx, now, 10)
	require.NoError(t, err)
	assert.Equal(t, []Task{first}, due)
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	var err error
	redisContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         "goSadTgBot-Redis-selfdestruct",
			Image:        "redis:latest",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections"),
		},
		Started: true,
		Reuse:   true,
	})
	if err != nil {
		panic(err)
	}
	code := m.Run()
	if err := redisContainer.Terminate(ctx); err != nil {
		fmt.Println("Failed to terminate the container:", err)
	}
	os.Exit(code)
}
