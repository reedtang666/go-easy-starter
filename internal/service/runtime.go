package service

import (
	"context"
	"sync/atomic"

	"github.com/reedtang666/go-easy-starter/internal/config"
	"github.com/reedtang666/go-easy-starter/internal/library/cache"
	"github.com/reedtang666/go-easy-starter/internal/queue"
	ws "github.com/reedtang666/go-easy-starter/internal/websocket"
)

type Runtime struct {
	Cache   *cache.Cache
	Config  config.Config
	Tasks   *queue.Client
	Hub     *ws.Hub
	Tickets *ws.Tickets
	Ready   func(context.Context) error
}

var current atomic.Pointer[Runtime]

func Configure(r *Runtime) { current.Store(r) }
func Current() *Runtime {
	r := current.Load()
	if r == nil {
		panic("runtime not initialized")
	}
	return r
}
