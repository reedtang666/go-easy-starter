# 配置与部署

## 配置

默认读取 `manifest/config/config.yaml`；可用 `--config /absolute/path/config.yaml` 指定。`setup` 只生成配置，不连接数据库、Redis 或云平台。

支持的环境变量如下，存在时覆盖文件中的对应值，包括显式空字符串：

| 变量 | 对应配置 |
| --- | --- |
| `STARTER_SERVER_ADDRESS` | `server.address` |
| `STARTER_SERVER_BASE_URL` | `server.baseUrl` |
| `STARTER_DEVELOPMENT` | `server.development` |
| `STARTER_DATABASE_URL` | `database.url` |
| `STARTER_AUTH_ACCESS_KEY` / `STARTER_AUTH_REFRESH_KEY` | 两把独立的认证密钥 |
| `STARTER_AUTH_ACCESS_SECONDS` | Access Token 秒数 |
| `STARTER_REDIS_AUTH_ADDRESS/PASSWORD/DB/PREFIX` | 认证 Redis |
| `STARTER_REDIS_CACHE_ADDRESS/PASSWORD/DB/PREFIX` | 缓存 Redis |
| `STARTER_REDIS_QUEUE_ADDRESS/PASSWORD/DB` | 队列 Redis |
| `STARTER_STORAGE_<NAME>_<FIELD>` | 见 [storage.md](storage.md) |

其他配置通过文件设置，程序不会自动加载 `.env`。Compose 的 `.env` 仅供 Compose 插值，不自动注入本机运行的 Go 进程。

生产必须调整：`server.development: false`、真实 HTTPS `baseUrl`、允许的 `origins`、数据库/Redis 凭证、持久存储路径，以及是否允许公开注册 `auth.register`。认证密钥不足 32 字节或重复会拒绝启动。配置文件按 0600 权限保存，不加入 Git；可参考样例逐项核对。

认证、缓存、队列可以共用 Redis 实例但使用不同 DB；生产队列使用持久化和 `noeviction` 策略。只启用实际使用的存储配置。

## 构建与运行

```sh
make build
./bin/starter http --config manifest/config/config.yaml
./bin/starter queue --config manifest/config/config.yaml
./bin/starter scheduler --config manifest/config/config.yaml
```

上面是三个独立进程，使用不同终端或进程管理器运行。只开一个 Scheduler。开发可用 `./bin/starter all` 合并角色。

日志输出到标准输出，HTTP 日志包含方法、路径、状态、耗时和 traceId；不输出请求正文、Authorization 或 URL 查询参数。反向代理如记录完整 URL，应额外隐藏 WebSocket 票据和下载签名参数。

内置限流按真实 socket 对端 IP + 用户识别，不信任任意转发头。反向代理后公共登录请求可能共用代理 IP 的配额，应在入口配置合适的限流规则，并按业务调整 `limits`。

收到 SIGTERM/SIGINT 后停止调度、关闭连接并等待 HTTP/任务退出。长任务按 `queue.timeoutSeconds` 和部署的停止超时共同配置。

## Docker

```sh
make image
```

镜像使用 Go 1.26.10 构建，以 UID/GID **10001** 运行。挂载配置到 `/app/manifest/config/config.yaml`，持久化 `/app/storage`；配置须可被 UID 10001 读取，存储目录须可写，不要把配置改成公开可读来解决权限。

Linux 主机示例（假设已创建自己的专用部署目录）：

```sh
sudo install -d -o 10001 -g 10001 -m 0700 /srv/go-easy-starter/storage
sudo install -o 10001 -g 10001 -m 0600 manifest/config/config.yaml /srv/go-easy-starter/config.yaml
docker run --rm --name starter-http \
  --network your-backend-network \
  -p 127.0.0.1:8000:8000 \
  -v /srv/go-easy-starter/config.yaml:/app/manifest/config/config.yaml:ro \
  -v /srv/go-easy-starter/storage:/app/storage \
  go-easy-starter:local http
```

先把配置中的数据库/Redis 地址改成容器网络可达地址；容器内 `127.0.0.1` 指向容器自身。`your-backend-network` 是需要替换的实际网络名。消费者同样挂载存储，命令改为 `queue`；调度器使用 `scheduler`。初始化数据库可在相同镜像/配置/网络下显式运行 `db-init`。

`compose.yaml` 只提供本地依赖，默认开发密码和环回端口不能作为生产配置直接使用。不要执行 `docker compose down -v` 清理已有数据。

## systemd

模板：`manifest/deploy/starter@.service`。默认专用用户 `starter`，工作目录 `/opt/go-easy-starter`，配置 `/etc/go-easy-starter/config.yaml`，二进制 `/opt/go-easy-starter/bin/starter`。

先创建用户/目录、安装二进制和配置、设置存储目录归属，并完成数据库初始化；然后安装模板：

```sh
sudo install -m 0644 manifest/deploy/starter@.service /etc/systemd/system/starter@.service
sudo systemctl daemon-reload
sudo systemctl enable --now starter@http starter@queue starter@scheduler
sudo systemctl status starter@http starter@queue starter@scheduler
sudo journalctl -u starter@http -u starter@queue -u starter@scheduler -n 100 --no-pager
curl -fsS http://127.0.0.1:8000/health/ready
```

配置归属设为 `starter:starter`、权限 0600。模板只放开 `/opt/go-easy-starter/storage` 写权限；修改存储路径时同步修改 `ReadWritePaths`。

## 更新与回滚

更新前备份数据库、配置和本地文件，保留当前二进制/镜像版本。评估新编号 SQL 的兼容性后再显式执行迁移，替换程序并重启对应角色。确认 `active (running)`、日志无启动错误、`/health/ready` 返回 200，再验证登录和上传链路。

程序回滚到上一二进制/镜像；数据库迁移是否可回滚必须单独评估，不能用重置初始化脚本回滚。此仓库的 Docker/systemd 文件是部署模板，本地编译不代表已在 Linux 或生产环境部署成功。
