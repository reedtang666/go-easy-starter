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
- GitHub Actions 仅提供工作流，提交后才能确认远端执行结果。

当前执行结果记录在文末；若某项未完成，会明确注明，后续可按上面命令继续。

## 本次执行结果（2026-10-22）

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
