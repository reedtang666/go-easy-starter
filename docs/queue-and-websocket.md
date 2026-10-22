# 队列、定时任务与 WebSocket

## 进程角色

| 角色 | 资源 | 行为 |
| --- | --- | --- |
| `http` | PostgreSQL、认证/缓存/队列 Redis、启用的存储 | 接口服务、任务生产者、可选 WebSocket |
| `queue` | PostgreSQL、队列 Redis、本地清理目录 | 消费任务；不启动 HTTP、认证和云存储客户端 |
| `scheduler` | 仅队列 Redis | 按配置投递周期任务，不执行业务处理 |
| `all` | 上述资源 | 本地一条命令启动全部角色 |

生产环境通常分别运行 `bin/starter http`、`bin/starter queue`、`bin/starter scheduler`。同一部署只运行一个调度器，避免重复周期投递；不要同时跑多个 `all` 扩容。

默认认证/缓存/队列使用 Redis DB 1/2/3。认证和缓存有独立键前缀，Asynq 使用自己的键结构，`redis.queue.prefix` 不改变 Asynq 前缀。不同项目的队列用不同 Redis DB/实例；只修改队列名称不足以隔离调度元数据。当前连接针对普通 Redis，不包含 Sentinel/Cluster。

## 投递与消费

`POST /api/v1/example/tasks` 为登录后的示例入口：

```json
{"message":"hello","delaySeconds":0,"priority":"default"}
```

返回 `taskId` 和实际队列名，消费者日志打印任务 ID、类型和执行结果。`priority` 支持 `high/default/low`，消费者权重为 6/3/1，不保证严格顺序。延迟范围 0 至 7 天；重试次数、处理超时、并发量分别由 `queue.maxRetry`、`timeoutSeconds`、`concurrency` 控制。

幂等事务示例：先创建 `POST /api/v1/example/items`，再投递：

```json
{"itemId":1,"key":"my-operation-001","delaySeconds":0,"priority":"high"}
```

`itemId` 必须属于当前用户。`key` 在该用户范围内唯一，同一操作重复投递时只增加一次 `taskRuns`。队列提供至少一次处理语义，业务通过数据库事务和 `example_task_receipts` 唯一键保证幂等；不要假设每个任务只执行一次。

处理器位于 `internal/queue/queue.go`，数据库变更在 `internal/logic/example/example.go`。新增任务：定义类型/负载，在消费者注册处理器；失败返回错误触发重试，确定无法执行的格式错误使用 `asynq.SkipRetry`。

## 定时任务

默认 `queue.cleanupCron: "@every 1h"`，时区 `Asia/Shanghai`，投递 `storage:cleanup` 清除过期本地分片会话。调度器只投递，必须有消费者才能执行。本地目录需要与 HTTP 进程共享；定时任务示例会实际执行文件清理。

## WebSocket

将 `websocket.enabled` 设为 `true` 后启用。浏览器连接流程：

1. 带 `Authorization: Bearer ...` 调用 `POST /api/v1/ws/ticket`。
2. 用返回票据连接 `ws://localhost:8000/ws?ticket=...`，生产使用 `wss://`。
3. 发送 `{"type":"echo","data":"hello"}` 收到回显。

票据默认 60 秒有效且只能消费一次。连接使用 `server.origins` 检查浏览器来源，原生客户端可不带 Origin。票据与 Access/Refresh Token 不应写入代理查询日志。

连接限制、消息大小、心跳时间可配置。每个连接单独写协程，心跳与消息处理核对会话；退出、替换登录、密码变更后关闭失效连接。`Hub.SendUser` 只推送本进程连接；多个 HTTP 实例之间没有自动广播，跨实例通知需另行实现。

## 验证

```sh
go test ./internal/queue/... ./internal/websocket/...
```

实际启动消费者后投递日志任务，应出现对应 `taskId` 日志；相同用户、相同幂等键投递两次，`taskRuns` 只增加一次。WebSocket 重用同一票据应失败，修改密码后旧连接应在心跳窗口关闭。集成流程见 [testing.md](testing.md)。
