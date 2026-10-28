# 测试与验证范围

## 本地检查

```sh
make check
make race
git diff --check
```

`make check` 包含 `go test ./...`、`go vet ./...`、构建 `bin/starter`。普通测试不需要真实 PostgreSQL/Redis；会话、缓存、队列、票据测试使用测试内的独立 Redis 模拟服务，文件测试使用临时目录。

## 真实 PostgreSQL / Redis 集成测试

必须使用专用 `*_test` 数据库和独立测试 Redis，测试会写入用户/附件/任务数据。禁止填写生产连接。测试本身不会清空或删除整个数据库。

已有本地 `make deps` 依赖时，可以显式创建测试库并导入表：

```sh
docker compose exec -T postgres createdb -U starter starter_test
docker compose exec -T postgres psql -U starter -d starter_test -v ON_ERROR_STOP=1 < manifest/sql/001_init.sql
docker compose exec -T postgres psql -U starter -d starter_test -v ON_ERROR_STOP=1 < manifest/sql/002_example.sql
export STARTER_TEST_DATABASE_URL='postgres://starter:starter@127.0.0.1:5432/starter_test?sslmode=disable'
export STARTER_TEST_REDIS_ADDRESS='127.0.0.1:6379'
make test-integration
```

已有初始化的测试库时跳过建库/导表步骤。测试固定使用 Redis DB 11/12/13，认证前缀和队列名随机；环境变量未配置时直接失败，不将跳过伪装成成功。

覆盖真实 HTTP、数据库与消费者之间的注册/重复注册、参数校验、登录刷新、用户隔离、CRUD、任务幂等、本地上传下载、头像引用约束、本地分片、WebSocket 票据及密码修改后的撤销。独立 Scheduler 测试验证不初始化数据库、HTTP 或认证。

同一命令还会验证数据库迁移和生成模块：迁移测试使用临时 schema，生成模块测试创建独立随机 `*_test` 数据库，完成后自动删除。因此测试账号需要 `CREATEDB` 权限；上面的 Compose 默认账号满足要求。

生成模块验收会复制不含本地配置的源码，在项目外调用 `module --root`，然后实际执行 `make setup`、`db-init`、`db-status`、`db-new`、`db-up`。验证首次只应用 001/002、后续迁移按序执行、再次升级无操作，以及生成接口的 CRUD、搜索/筛选/排序/分页和跨用户隔离。运行这些测试需要 Go 和 Make。

需要同时验证 GoFrame 重新生成代码的兼容性时，先安装固定版本工具并明确指定：

```sh
make tools
export STARTER_TEST_GF="$PWD/.tools/gf"
make test-integration
```

未设置 `STARTER_TEST_GF` 时仍会验证生成模块的编译与接口，但不会执行 GoFrame CLI；GitHub Actions 已设置此变量。

## 手工验收

1. `/health/live` 和 `/health/ready` 都为 200，响应 `code:0`。
2. 注册、登录、查询本人资料；重复用户名返回 409，未登录查询返回 401。
3. 两个用户各建记录，互相查询对方 ID 返回 404。
4. 上传文件后按返回 URL 下载，内容一致；私有本地内容无签名访问返回 403；删除后无法访问。
5. 同一个幂等任务投递两次，消费者有任务 ID 日志，`taskRuns` 只增加一次。
6. 刷新、退出、重新登录、修改密码后，旧凭证按会话规则失效。
7. 开启 WebSocket 后，票据只能用一次，密码修改后旧连接关闭。

## 尚需环境验证

- 五家对象存储 SDK 的真实账号、桶权限、上传删除、域名和签名过期；本地测试不证明云端联调。
- Docker 镜像实际构建运行、Linux systemd 生命周期、反向代理、共享卷文件锁、正式部署和备份恢复。
- 吞吐、长时间运行及故障恢复；不宣称已有压力测试或完整 E2E 覆盖率。
- GitHub Actions 已包含单元、race、编译、迁移和生成模块集成工作流；本次修改提交推送后，仍需确认对应提交的远端执行结果。

当前执行结果记录在文末；若某项未完成，会明确注明，后续可按上面命令继续。

## 基础版执行结果（2026-10-22）

- `go test ./...`：通过，配置、会话、缓存、本地存储/分片、队列、用户名密码、WebSocket 票据测试通过。
- `go test -race ./...`：通过，未发现数据竞争。
- `go vet ./...`：通过，无诊断。
- `go build -trimpath -o bin/starter .`：通过，生成本机 macOS arm64 二进制。
- PostgreSQL **18** + Redis **7.4.2** 隔离实例：`go test -tags=integration ./internal/bootstrap -count=1 -timeout=90s` 通过（4.861 秒）。覆盖实际 HTTP、队列消费者、本地文件和 WebSocket 链路。
- 二进制 `setup`、对新建空测试库执行 `db-init`：通过。
- 官方 GoFrame CLI **2.7.2** 的 `gen ctrl`、`gen service`、`gen dao`：已实际执行；生成文件留在仓库中。
- 配置/Compose/CI YAML 语法、安装脚本语法、文档本地链接和源码空白检查：通过。
- Go **1.26.10** 下 `govulncheck ./...`：未发现当前调用路径可达漏洞；工具另报告未调用路径的包/模块漏洞，不能据此宣称依赖完全无漏洞。
- Docker、systemd、五家真实对象存储及 GitHub Actions 远端运行：未执行，按上述步骤在对应环境验收。

测试实例只用于本项目验收，位于临时目录和独立端口，验收后已关闭。后续复测使用本文明确配置的独立测试环境。

## 开发工具收尾验收（2026-10-28）

环境：macOS arm64、Go 1.26.10、GoFrame CLI 2.7.2、临时 PostgreSQL 18.6、临时 Redis 7.4.2。数据库与 Redis 仅监听独立本地端口，使用新建测试数据；未连接项目已有运行配置中的服务。

| 检查 | 实际结果 |
| --- | --- |
| `make check`（单元测试、vet、构建） | 通过 |
| `go test -race ./... -count=1` | 通过，未发现数据竞争 |
| `go mod verify` | 通过 |
| `go test -tags=integration -race ./internal/bootstrap ./internal/migration ./internal/scaffold -count=1 -timeout=180s`（同时指定覆盖率输出和 `STARTER_TEST_GF`） | 三个包全部通过；含真实 HTTP/消费者、迁移和生成模块验收 |
| 数据库迁移边界 | 初始化、增量、重复升级、SQL 漂移拒绝、失败事务回滚、并发锁、锁等待取消、旧库 baseline 均通过 |
| CLI 凭证保护回归 | 4 种数据库命令 × 3 类错误连接串通过；解析错误不再包含密码 |
| GoFrame `gen ctrl` / `gen service` | 在生成模块副本中执行，随后编译与接口验收通过 |
| 干净源码首次使用 | 不复制已有配置或 `.tools`，在含空格的目录执行 README 命令、模块生成和增量迁移，通过 |
| `go run . all` 实际进程 | live/ready 返回 200，Swagger/OpenAPI、注册/登录/本人资料、文章模块、私有上传下载通过；SIGTERM 后监听关闭 |
| 重复初始化保护 | 已有配置未被覆盖，已有数据库拒绝再次初始化；重复 `db-up` 返回 `applied 0 migration(s)` |

本次覆盖率：查询工具单测 100.0%，脚手架 83.7%，迁移单测及集成测试合计 85.2%。这些是对应包的语句覆盖率，不代表整个项目达到相同比例。

干净源码验收使用独立 PostgreSQL/Redis，未执行 Docker Compose；本机无 Docker。Docker 镜像、Linux systemd、真实对象存储、负载/备份恢复以及本次提交的远端 CI，仍按上面的环境验证范围单独验收。
