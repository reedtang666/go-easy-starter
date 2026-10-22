package bootstrap

import (
	"context"
	"errors"
	"fmt"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/net/goai"
	"github.com/hibiken/asynq"
	"github.com/reedtang666/go-easy-starter/internal/config"
	"github.com/reedtang666/go-easy-starter/internal/dao"
	core "github.com/reedtang666/go-easy-starter/internal/library/auth"
	"github.com/reedtang666/go-easy-starter/internal/library/cache"
	"github.com/reedtang666/go-easy-starter/internal/library/response"
	"github.com/reedtang666/go-easy-starter/internal/library/storager"
	authlogic "github.com/reedtang666/go-easy-starter/internal/logic/auth"
	examplelogic "github.com/reedtang666/go-easy-starter/internal/logic/example"
	filelogic "github.com/reedtang666/go-easy-starter/internal/logic/file"
	userlogic "github.com/reedtang666/go-easy-starter/internal/logic/user"
	"github.com/reedtang666/go-easy-starter/internal/queue"
	"github.com/reedtang666/go-easy-starter/internal/router"
	"github.com/reedtang666/go-easy-starter/internal/service"
	ws "github.com/reedtang666/go-easy-starter/internal/websocket"
	"net/url"
	"strings"
	"sync"
	"time"
)

type App struct {
	Config    config.Config
	Role      string
	HTTP      *ghttp.Server
	DB        gdb.DB
	Redis     map[string]*gredis.Redis
	Tasks     *queue.Client
	Worker    *asynq.Server
	Mux       *asynq.ServeMux
	Scheduler *asynq.Scheduler
	Hub       *ws.Hub
	files     service.IFile
	once      sync.Once
}

func New(ctx context.Context, c config.Config, role string) (_ *App, err error) {
	if e := c.Validate(role); e != nil {
		return nil, e
	}
	a := &App{Config: c, Role: role, Redis: map[string]*gredis.Redis{}}
	defer func() {
		if err != nil {
			a.Close()
		}
	}()
	connect := func(name string, v config.Redis) error {
		cl, e := gredis.New(&gredis.Config{Address: v.Address, Pass: v.Password, Db: v.DB, DialTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second})
		if e != nil {
			return fmt.Errorf("invalid Redis configuration: %s", name)
		}
		a.Redis[name] = cl
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, e = cl.Do(pingCtx, "PING"); e != nil {
			return fmt.Errorf("Redis unavailable: %s", name)
		}
		return nil
	}
	if err = connect("queue", c.Redis.Queue); err != nil {
		return nil, err
	}
	if role != "scheduler" {
		databaseURL, e := url.Parse(c.Database.URL)
		if e != nil {
			return nil, errors.New("invalid database configuration")
		}
		a.DB, err = gdb.New(gdb.ConfigNode{Type: "pgsql", Name: strings.TrimPrefix(databaseURL.Path, "/"), Namespace: "public", Link: c.Database.URL, QueryTimeout: 5 * time.Second, ExecTimeout: 5 * time.Second, TranTimeout: 30 * time.Second, MaxOpenConnCount: 20, MaxIdleConnCount: 5})
		if err != nil {
			return nil, errors.New("invalid database configuration")
		}
		if err = a.DB.PingMaster(); err != nil {
			return nil, errors.New("PostgreSQL unavailable")
		}
	}
	a.Tasks = queue.NewClient(c)
	if role == "queue" || role == "all" {
		ex := examplelogic.New(dao.New(a.DB))
		cleanup := func(ctx context.Context) error {
			for _, storage := range c.Storage.Drivers {
				if storage.Enabled && storage.Driver == "local" {
					if e := storager.CleanupLocal(ctx, storage.Root, c.Storage.MaxBytes, time.Duration(c.Storage.MultipartSeconds)*time.Second); e != nil {
						return e
					}
				}
			}
			return nil
		}
		a.Worker, a.Mux = queue.NewWorker(c, ex.ApplyTask, cleanup)
	}
	if role == "scheduler" || role == "all" {
		a.Scheduler, err = queue.NewScheduler(c)
		if err != nil {
			return nil, err
		}
	}
	if role == "http" || role == "all" {
		if err = connect("auth", c.Redis.Auth); err != nil {
			return nil, err
		}
		if err = connect("cache", c.Redis.Cache); err != nil {
			return nil, err
		}
		store := dao.New(a.DB)
		users := userlogic.New(store)
		sessionStore, e := core.NewRedisStore(a.Redis["auth"], c.Redis.Auth.Prefix)
		if e != nil {
			return nil, e
		}
		tokens, e := core.NewManager(c.Auth, sessionStore, nil)
		if e != nil {
			return nil, e
		}
		auth := authlogic.New(users, tokens)
		files, e := filelogic.New(store, c)
		if e != nil {
			return nil, e
		}
		a.files = files
		service.RegisterUser(users)
		service.RegisterAuth(auth)
		service.RegisterFile(files)
		service.RegisterExample(examplelogic.New(store))
		tickets := ws.NewTickets(a.Redis["auth"], c.Redis.Auth.Prefix, time.Duration(c.WebSocket.TicketSeconds)*time.Second)
		if c.WebSocket.Enabled {
			a.Hub = ws.NewHub(c, tickets, auth.ValidateSession)
		}
		service.Configure(&service.Runtime{Cache: cache.New(a.Redis["cache"], c.Redis.Cache.Prefix), Config: c, Tasks: a.Tasks, Hub: a.Hub, Tickets: tickets, Ready: a.Ready})
		s := ghttp.GetServer(fmt.Sprintf("starter-%d", time.Now().UnixNano()))
		a.HTTP = s
		if err = s.SetConfigWithMap(g.Map{"address": c.Server.Address, "logStdout": false, "accessLogEnabled": false, "errorLogEnabled": false, "errorStack": false, "dumpRouterMap": false, "sessionCookieOutput": false, "clientMaxBodySize": c.Server.MaxBodyBytes, "maxHeaderBytes": 16384, "readTimeout": 30 * time.Second, "writeTimeout": 60 * time.Second, "idleTimeout": 60 * time.Second, "graceful": false, "gracefulShutdownTimeout": c.Server.ShutdownSeconds}); err != nil {
			return nil, err
		}
		if c.Server.Development {
			s.SetOpenApiPath("/api.json")
			s.SetSwaggerPath("/swagger")
			doc := s.GetOpenApi()
			doc.Info.Title = "Go Easy Starter API"
			doc.Config.CommonResponse = response.Envelope{}
			doc.Config.CommonResponseDataField = "Data"
			doc.Components = goai.Components{SecuritySchemes: goai.SecuritySchemes{"BearerToken": goai.SecuritySchemeRef{Value: &goai.SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT"}}}}
		} else {
			s.SetOpenApiPath("")
			s.SetSwaggerPath("")
		}
		router.Register(s, c, a.Redis["auth"])
	}
	return a, nil
}
func (a *App) Ready(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if a.DB != nil {
		if _, e := a.DB.GetValue(ctx, "SELECT 1"); e != nil {
			return errors.New("database unavailable")
		}
	}
	for name, r := range a.Redis {
		if _, e := r.Do(ctx, "PING"); e != nil {
			return fmt.Errorf("Redis unavailable: %s", name)
		}
	}
	return nil
}
func (a *App) Start() error {
	if a.Worker != nil {
		if e := a.Worker.Start(a.Mux); e != nil {
			return e
		}
	}
	if a.Scheduler != nil {
		if e := a.Scheduler.Start(); e != nil {
			return e
		}
	}
	if a.HTTP != nil {
		if e := a.HTTP.Start(); e != nil {
			return e
		}
	}
	return nil
}
func (a *App) Close() {
	a.once.Do(func() {
		if a.Scheduler != nil {
			a.Scheduler.Shutdown()
		}
		if a.Hub != nil {
			a.Hub.Close()
		}
		if a.HTTP != nil {
			a.HTTP.Shutdown()
		}
		if a.Worker != nil {
			a.Worker.Shutdown()
		}
		if a.files != nil {
			a.files.Close()
		}
		if a.Tasks != nil {
			a.Tasks.Close()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, r := range a.Redis {
			r.Close(ctx)
		}
		if a.DB != nil {
			a.DB.Close(ctx)
		}
	})
}
