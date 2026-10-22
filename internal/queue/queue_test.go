package queue

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"testing"
	"time"
)

func TestEnqueueAndDelay(t *testing.T) {
	r := miniredis.RunT(t)
	c := config.Defaults()
	c.Redis.Queue.Address = r.Addr()
	client := NewClient(c)
	defer client.Close()
	info, e := client.Enqueue(context.Background(), Payload{Message: "test"}, time.Minute)
	if e != nil {
		t.Fatal(e)
	}
	if info.ID == "" {
		t.Fatal("missing ID")
	}
	ins := asynq.NewInspector(redisOption(c))
	defer ins.Close()
	task, e := ins.GetTaskInfo(c.Queue.Name, info.ID)
	if e != nil {
		t.Fatal(e)
	}
	if task.State != asynq.TaskStateScheduled {
		t.Fatal(task.State)
	}
	high, e := client.Enqueue(context.Background(), Payload{Message: "priority", Priority: "high"}, 0)
	if e != nil {
		t.Fatal(e)
	}
	if high.Queue != c.Queue.Name+":high" {
		t.Fatal(high.Queue)
	}
	if _, e = client.Enqueue(context.Background(), Payload{Message: "invalid", Priority: "other"}, 0); e == nil {
		t.Fatal("invalid priority accepted")
	}
	if _, e = client.Enqueue(context.Background(), Payload{}, 0); e == nil {
		t.Fatal("invalid payload accepted")
	}
}
