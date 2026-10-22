package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/hibiken/asynq"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"time"
)

const (
	TypeLog     = "example:log"
	TypeApply   = "example:apply"
	TypeCleanup = "storage:cleanup"
)

type Payload struct {
	Priority string `json:"priority,omitempty"`
	Message  string `json:"message,omitempty"`
	OwnerID  int64  `json:"ownerId,omitempty"`
	ItemID   int64  `json:"itemId,omitempty"`
	Key      string `json:"key,omitempty"`
}
type Client struct {
	client *asynq.Client
	cfg    config.Config
}

func redisOption(c config.Config) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{Addr: c.Redis.Queue.Address, Password: c.Redis.Queue.Password, DB: c.Redis.Queue.DB, DialTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
}
func NewClient(c config.Config) *Client {
	return &Client{client: asynq.NewClient(redisOption(c)), cfg: c}
}
func (c *Client) Close() error { return c.client.Close() }
func (c *Client) Enqueue(ctx context.Context, p Payload, delay time.Duration) (*asynq.TaskInfo, error) {
	if delay < 0 || delay > 7*24*time.Hour {
		return nil, response.Fail(400, "TASK_INVALID", "delay must be between zero and seven days")
	}
	name := c.cfg.Queue.Name
	switch p.Priority {
	case "", "default":
	case "high", "low":
		name += ":" + p.Priority
	default:
		return nil, response.Fail(400, "TASK_INVALID", "priority must be default, high or low")
	}
	typ := TypeLog
	if p.ItemID > 0 {
		if p.OwnerID <= 0 || p.Key == "" || len(p.Key) > 100 {
			return nil, response.Fail(400, "TASK_INVALID", "item tasks require ownerId and key (up to 100 bytes)")
		}
		p.Key = fmt.Sprintf("%d:%s", p.OwnerID, p.Key)
		typ = TypeApply
	} else if p.Message == "" || len(p.Message) > 500 {
		return nil, response.Fail(400, "TASK_INVALID", "log tasks require a message up to 500 bytes")
	}
	b, e := json.Marshal(p)
	if e != nil {
		return nil, e
	}
	return c.client.EnqueueContext(ctx, asynq.NewTask(typ, b), asynq.Queue(name), asynq.MaxRetry(c.cfg.Queue.MaxRetry), asynq.Timeout(time.Duration(c.cfg.Queue.TimeoutSeconds)*time.Second), asynq.ProcessIn(delay))
}
func NewWorker(c config.Config, apply func(context.Context, int64, int64, string) error, cleanup func(context.Context) error) (*asynq.Server, *asynq.ServeMux) {
	srv := asynq.NewServer(redisOption(c), asynq.Config{Concurrency: c.Queue.Concurrency, Queues: map[string]int{c.Queue.Name: 3, c.Queue.Name + ":high": 6, c.Queue.Name + ":low": 1}, ShutdownTimeout: time.Duration(c.Server.ShutdownSeconds) * time.Second})
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeLog, func(ctx context.Context, t *asynq.Task) error {
		var p Payload
		if e := json.Unmarshal(t.Payload(), &p); e != nil {
			return fmt.Errorf("invalid task payload: %w", asynq.SkipRetry)
		}
		if p.Message == "" {
			return fmt.Errorf("missing message: %w", asynq.SkipRetry)
		}
		id, _ := asynq.GetTaskID(ctx)
		attempt, _ := asynq.GetRetryCount(ctx)
		g.Log().Infof(ctx, "task completed id=%s type=%s attempt=%d", id, TypeLog, attempt)
		return nil
	})
	mux.HandleFunc(TypeApply, func(ctx context.Context, t *asynq.Task) error {
		var p Payload
		if e := json.Unmarshal(t.Payload(), &p); e != nil || p.OwnerID <= 0 || p.ItemID <= 0 || p.Key == "" {
			return fmt.Errorf("invalid item task: %w", asynq.SkipRetry)
		}
		if e := apply(ctx, p.OwnerID, p.ItemID, p.Key); e != nil {
			return e
		}
		id, _ := asynq.GetTaskID(ctx)
		g.Log().Infof(ctx, "task completed id=%s type=%s", id, TypeApply)
		return nil
	})
	mux.HandleFunc(TypeCleanup, func(ctx context.Context, t *asynq.Task) error { return cleanup(ctx) })
	return srv, mux
}
func NewScheduler(c config.Config) (*asynq.Scheduler, error) {
	zone, e := time.LoadLocation(c.Queue.Timezone)
	if e != nil {
		return nil, e
	}
	s := asynq.NewScheduler(redisOption(c), &asynq.SchedulerOpts{Location: zone})
	_, e = s.Register(c.Queue.CleanupCron, asynq.NewTask(TypeCleanup, nil), asynq.Queue(c.Queue.Name), asynq.MaxRetry(c.Queue.MaxRetry), asynq.Timeout(time.Duration(c.Queue.TimeoutSeconds)*time.Second))
	if e != nil {
		return nil, e
	}
	return s, nil
}
