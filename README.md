# Go Easy Starter

基于 GoFrame 的通用 API 项目模板。带上日常开发需要的登录、文件、队列和部署基础，直接在示例模块上扩展自己的业务。

## 已包含

- 用户名密码注册、登录、资料修改、修改密码、退出。
- JWT Access Token + 轮换 Refresh Token、Redis 会话、单用户单会话、密码修改后撤销旧凭证。
- PostgreSQL、Redis JSON 缓存、分页、事务和用户数据隔离示例。
- 本地、UCloud、腾讯云 COS、阿里云 OSS、七牛、MinIO 六种上传驱动；附件记录、私有链接、本地分片续传。
- Asynq 即时/延迟任务、重试、优先级、独立消费者与定时调度、事务幂等示例。
- 可选 WebSocket：一次性票据、心跳、会话撤销检查、消息示例。
- 参数校验、统一 JSON 响应、CORS、限流、健康检查、开发环境 Swagger。
- GoFrame 代码生成、Makefile、Docker、Compose、systemd 与 GitHub Actions。

WebSocket 默认关闭；默认仅启用本地上传，其他驱动按配置启用。没有后台管理、支付、短信或特定业务模块。

## 快速开始

需要 **Git、Go 1.26.10+、Make**，以及可用的 PostgreSQL、Redis。下面以 macOS/Linux 终端为例；使用 Docker 方案时还需要安装并启动 **Docker 和 Docker Compose v2**。普通运行无需安装 GoFrame CLI。

先获取项目：

```sh
git clone https://github.com/reedtang666/go-easy-starter.git
cd go-easy-starter
```

后续命令均在项目根目录执行。

**使用 Docker 快速体验：**准备好上述工具后，按顺序执行以下命令即可使用默认配置启动。`--wait` 会等待数据库和 Redis 通过健康检查后再返回。

```sh
go mod download
make setup
docker compose up -d --wait postgres redis
make db-init
go run . all
```

另开终端访问 `http://localhost:8000/health/ready`，返回 HTTP 200 且状态为 `ready` 后，即可打开 `http://localhost:8000/swagger`，按照下文“第一次调用”注册、登录并上传文件。

下面是各步骤的说明；使用已有数据库/Redis 时，按第 2 步填写自己的连接信息。

### 1. 生成运行配置

```sh
go mod download
make setup
```

`make setup` 创建被 Git 忽略的 `manifest/config/config.yaml`，自动生成认证和本地文件签名所需的三把独立随机密钥。已有配置时跳过这一步，命令会拒绝覆盖已有文件。

生成内容是 JSON，也是合法 YAML，可以直接编辑。保留自动生成的密钥，按下一步填写连接信息。

### 2. 准备 PostgreSQL 和 Redis

以下两种方式任选一种。

**使用已有服务：**先创建一个供本项目使用的空 PostgreSQL 数据库。例如在 Navicat 的查询窗口执行：

```sql
CREATE DATABASE go_easy_starter;
```

数据库已存在且尚未建表时，直接使用该数据库。修改 `manifest/config/config.yaml` 中的对应字段：

| 字段 | 填写方式 |
| --- | --- |
| `database.url` | `postgres://用户名:密码@地址:5432/go_easy_starter?sslmode=disable`，替换成实际连接信息 |
| `redis.auth.address`、`redis.cache.address`、`redis.queue.address` | Redis 地址，例如 `127.0.0.1:6379` |
| 三处 Redis 的 `password` | 有密码就填写，没有密码保留空字符串 |
| 三处 Redis 的 `db` | 默认分别为 `1`、`2`、`3`，供会话、缓存和队列使用；为本项目预留这些库或改用空闲编号 |

三个 Redis 配置可以连接同一个实例。数据库 URL 中的用户名/密码含 `@`、`:`、`/`、`#` 等特殊字符时，需要进行 URL 编码。

**使用 Docker 新建本地依赖：**安装并启动 Docker 后执行：

```sh
make deps
docker compose ps
```

等 PostgreSQL 和 Redis 都显示 `healthy` 后再继续。Compose 会自动创建数据库，连接信息与 `make setup` 的默认配置一致：

| 服务 | 默认连接 |
| --- | --- |
| PostgreSQL | `127.0.0.1:5432`，数据库 `starter`，用户名 `starter`，密码 `starter` |
| Redis | `127.0.0.1:6379`，无密码，使用 DB `1`、`2`、`3` |

使用此方式时保留默认连接配置即可。Compose 的默认凭证仅用于本地开发。

### 3. 初始化数据库表（首次运行）

**已有数据库但没有表时，执行下面的命令即可自动建表：**

```sh
make db-init
```

等价命令是 `go run . db-init`。它读取 `config.yaml` 中的 `database.url`，在一个事务中按顺序执行仓库自带的两个 SQL 文件：

| SQL 文件 | 创建的表 |
| --- | --- |
| [`manifest/sql/001_init.sql`](manifest/sql/001_init.sql) | `users`（用户）、`attachments`（附件） |
| [`manifest/sql/002_example.sql`](manifest/sql/002_example.sql) | `example_items`（CRUD 示例）、`example_task_receipts`（任务幂等记录） |

成功后终端输出 `database initialized`，刷新数据库即可看到 **4 张表**。初始化命令只连接 PostgreSQL，不需要先启动 API 或 Redis。

也可以使用 Navicat 手动导入：选择本项目的空数据库，依次执行 **001 → 002** 两个文件的全部 SQL。手动导入和 `make db-init` 任选一种。

注意：

- `make db-init` 创建表、索引和约束，数据库本身需要提前创建；Docker 方式已由 Compose 创建。
- 初始化只在首次执行。已有 `users` 表时会拒绝重复初始化，不会清空或重建已有数据。
- 程序启动时不会自动建表或迁移；后续表结构变更通过独立编号 SQL 显式执行。
- SQL 不预置登录账号，启动后通过注册接口创建用户。

### 4. 启动并确认连接正常

```sh
go run . all
```

`all` 会同时启动 HTTP API、队列消费者和定时调度器。以后启动直接执行此命令，保留已有配置和数据库表。

另开终端验证：

```sh
curl -i http://localhost:8000/health/ready
```

预期返回 HTTP 200、`code: 0`、`data.status: "ready"`，说明数据库与 Redis 连接正常。

启动成功后：

- 存活检查：`http://localhost:8000/health/live`
- 数据库、Redis 就绪检查：`http://localhost:8000/health/ready`
- 接口文档：`http://localhost:8000/swagger`
- OpenAPI：`http://localhost:8000/api.json`

健康检查应返回 HTTP 200、`code: 0`，存活状态为 `alive`，就绪状态为 `ready`。Swagger 和 OpenAPI 仅在 `server.development: true` 时开放。

## 第一次调用

没有预置账号，先注册再登录：

```sh
curl -sS http://localhost:8000/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"myuser","password":"change-me-123"}'

curl -sS http://localhost:8000/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"myuser","password":"change-me-123","deviceId":"local-dev"}'
```

将登录结果 `data.accessToken` 放入请求头，访问本人资料或上传文件：

```sh
export STARTER_ACCESS_TOKEN='替换为登录结果中的 accessToken'
curl -sS http://localhost:8000/api/v1/user/me \
  -H "Authorization: Bearer $STARTER_ACCESS_TOKEN"

printf 'hello starter\n' > /tmp/starter-upload.txt
curl -sS http://localhost:8000/api/v1/files \
  -H "Authorization: Bearer $STARTER_ACCESS_TOKEN" \
  -F 'file=@/tmp/starter-upload.txt' -F 'private=true'
```

业务响应统一为：

```json
{"code":0,"msg":"OK","data":{},"traceId":"..."}
```

错误同时使用实际 HTTP 状态码，例如 400、401、403、404、409、429、503。下载和 WebSocket 使用各自的原始协议。

认证默认 Access Token 15 分钟、Refresh Token 空闲 7 天且最长 30 天，并发刷新宽限 5 秒。客户端应串行刷新，成功后替换两个令牌。新登录会替换该用户的旧会话。

## 常用命令

| 命令 | 用途 |
| --- | --- |
| `make setup` | 首次生成本地配置和随机密钥，拒绝覆盖已有文件 |
| `make deps` | 使用 Docker Compose 启动本地 PostgreSQL 和 Redis |
| `make db-init` | 首次连接已创建的空数据库，执行两个 SQL 并创建 4 张表 |
| `go run . all` | 本地启动 HTTP、消费者、调度器 |
| `make http` / `make queue` / `make scheduler` | 分角色启动 |
| `make build` | 构建 `bin/starter` |
| `make check` | 单元测试、vet、构建 |
| `make test-integration` | 使用明确指定的测试数据库和 Redis 跑集成测试 |
| `make tools` | 安装项目内固定版本 `.tools/gf`，不替换全局工具 |
| `make dev` | 使用 GoFrame CLI 热重载 |
| `make ctrl` / `make service` / `make dao` | 生成控制器、服务接口、数据库模型 |

普通启动不需要安装 GoFrame CLI。生成代码/热重载前，先执行 `make tools`，将 `hack/config.example.yaml` 复制为 `hack/config.yaml` 并配置自己的开发数据库。

## 目录与文档

```text
api/                  请求、响应、路由与校验标签
internal/controller/  接口适配
internal/service/     生成的业务服务接口、运行资源入口
internal/logic/       用户、认证、文件、示例业务
internal/dao/         生成的 DAO、显式数据库 Store
internal/model/       DO、Entity、对外 View、分页模型
internal/library/     认证核心、存储、缓存、响应、上下文
internal/queue/       投递、消费者、周期调度
internal/websocket/   票据、连接与心跳
internal/bootstrap/   按角色组装与关闭资源
manifest/             配置样例、SQL、Docker、systemd
```

- [开发与代码生成](docs/development.md)
- [上传、六种驱动与分片](docs/storage.md)
- [队列、定时任务与 WebSocket](docs/queue-and-websocket.md)
- [配置与部署](docs/deployment.md)
- [测试方法与实际验证范围](docs/testing.md)

MIT License。真实云账号、桶权限、签名域名仍需在自己的环境联调，具体边界见测试文档。
