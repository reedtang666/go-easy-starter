# 开发与代码生成

## 分层

调用方向是 `api → controller → service → logic → dao → PostgreSQL`。

- `api/<module>/v1`：路由、HTTP 方法、校验、Swagger 标签、响应类型。
- `internal/controller/<module>`：读取当前身份、调用 Service、组装响应。
- `internal/logic/<module>`：业务规则和事务；构造函数接收依赖。
- `internal/service`：GoFrame 生成的接口和注册入口。资源在 `internal/bootstrap/app.go` 显式创建和注册，没有启动即连接外部服务的业务 `init()`。
- `internal/dao`、`internal/model/do`、`internal/model/entity`：生成代码。`dao/store.go` 接收显式 `gdb.DB`，避免进程/测试隐式共用默认数据库。
- `internal/model/view`：对外模型。用户密码哈希、凭证版本不序列化；不要把生成的用户 Entity 直接作为 HTTP 响应。

公共组件不反向依赖业务逻辑。新增配置统一写入 `internal/config/config.go`，新增外部资源在 Bootstrap 创建和关闭。

## 创建第一个业务模块

在项目根目录执行：

```sh
make module NAME=article
# 等价命令：go run . module --name article
make check
```

命令只操作项目文件，不读取运行配置，不连接 PostgreSQL 或 Redis，也不需要 GoFrame CLI。模块名使用小写 snake_case，例如 `article`、`article_comment`；已有模块路径或包级声明冲突时拒绝生成，不覆盖原有业务代码。

以 `article` 为例，会生成：

| 路径 | 用途 |
| --- | --- |
| `api/article/`、`api/article/v1/` | 接口定义、请求校验与响应 |
| `internal/controller/article/` | 列表、创建、详情、更新、删除接口 |
| `internal/logic/article/` | 本人数据隔离、分页筛选、CRUD 逻辑 |
| `internal/service/article.go` | 服务接口与注册入口 |
| `internal/model/view/article.go` | 对外响应模型 |
| `manifest/sql/NNN_create_article_items.sql` | `article_items` 表及索引，编号自动分配 |

同时在 `internal/bootstrap/modules.go` 注册服务，在 `internal/router/modules.go` 绑定到已有鉴权路由组。生成后即可编译；调用接口前，需要对目标开发数据库应用生成的 SQL：

```sh
make db-status
make db-up
go run . all
```

首次创建空库先执行 `make db-init`；以前手动导入过初始 SQL 的旧库先按[迁移说明](migrations.md)执行 `make db-baseline`。数据库命令只需 PostgreSQL；启动 API 仍需要 Redis。

先按 [README](../README.md#第一次调用)注册、登录并设置 `STARTER_ACCESS_TOKEN`，然后调用：

```sh
curl -sS http://localhost:8000/api/v1/article/items \
  -H "Authorization: Bearer $STARTER_ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"第一篇文章","content":"从这里开始写业务"}'

curl -sS 'http://localhost:8000/api/v1/article/items?page=1&pageSize=20&sortBy=created_at&sortOrder=desc' \
  -H "Authorization: Bearer $STARTER_ACCESS_TOKEN"
```

创建返回 `data.item`，列表返回 `data.list`、`total`、`page`、`pageSize`。详情、更新、删除使用 `/api/v1/article/items/{id}`，对应 GET、PUT、DELETE。不存在和不属于当前用户的记录统一返回 404。Swagger 会展示这些接口。

生成的模块使用 `dao.Store` 的显式数据库连接，可以直接继续开发。新增业务字段时，在新迁移中变更表结构，并同步修改 API、Logic、View；生成命令不会根据自定义字段自动补业务规则。

## 常用列表参数

内置示例和新生成模块共用 `internal/model/list_query.go`、`internal/library/query`：

| 参数 | 含义 |
| --- | --- |
| `page`、`pageSize` | 默认第 1 页、每页 20 条，每页最多 100 条 |
| `keyword` | 标题包含关键词，不区分大小写；`%`、`_`、`!` 按字面匹配 |
| `createdFrom`、`createdTo` | 创建时间范围，RFC3339，例如 `2026-10-22T00:00:00Z`；可只填一端，两端均包含 |
| `sortBy` | `id`、`title`、`created_at`、`updated_at`，默认 `id` |
| `sortOrder` | `asc`、`desc`，默认 `desc` |

非主键排序会追加同方向的 `id` 排序，避免相同标题或时间下分页顺序不确定。Count 和 List 复用相同筛选条件，`total` 为筛选后的数量。时间格式错误、结束早于开始、非法排序返回 400。

关键词、时间等查询值绑定参数，排序字段从白名单中选择。扩展自己的列表时保留 `owner_id` 条件；修改排序字段应同步修改校验与白名单。URL 参数中的 `+`、`%` 等字符需要编码，curl 可使用 `--get --data-urlencode`。

## 继续使用 GoFrame 代码生成

模块骨架与 GoFrame 生成流程兼容。需要根据数据库生成 DAO，或新增接口、Service 方法时，再安装项目工具：

```sh
make tools
# 没有本地工具配置时执行；已有文件则编辑自己的连接配置
cp hack/config.example.yaml hack/config.yaml
```

修改 `hack/config.yaml` 中的开发数据库连接。运行配置 `manifest/config/config.yaml` 和 CLI 配置 `hack/config.yaml` 分别管理，不要提交真实连接信息。

```sh
# 已应用 article_items 迁移后，只生成该表的 DAO、DO、Entity
.tools/gf gen dao --tables=article_items

# 新增或修改 api/<module>/v1 请求/响应后
make ctrl

# 新增或修改 internal/logic/<module> 的业务方法后
make service
```

`--tables` 指定本次生成目标，无需改动 `hack/config.yaml` 原有表清单。`make dao` 则使用该文件中配置的表清单。

业务实现使用 `sModule` 接收者，CLI 通过 `s` 前缀识别实现；构造函数不进入业务接口。资源仍在 Bootstrap 中显式创建和注册，新模块自动进入专门的 `modules.go` 注册文件。

控制器生成会保留已存在的方法实现，生成前后仍需检查 diff。不要手改 `internal/dao/internal`、DO、Entity 或生成的 Service 接口来实现业务规则。GoFrame 运行库与项目 CLI 固定为 **v2.7.2**，Go 最低 **1.26.10**；使用项目 `.tools/gf`，不覆盖全局工具。

## 数据库约定

初始化显式执行 `make db-init`，后续表结构变更使用 `make db-new NAME=...` 创建迁移、填写 SQL，再用 `make db-status`、`make db-up` 查看和应用。每条 SQL 和对应迁移记录在同一个事务中提交，程序启动不自动迁移。旧数据库接管、文件不可修改规则和失败恢复见[数据库迁移](migrations.md)。

PostgreSQL 18 新增的 NOT NULL 约束元数据会影响旧版 GoFrame 驱动的主键推断。因此单行新增通过 `dao.Store.InsertID` 的参数化 `INSERT ... RETURNING id` 获取主键；不要改回依赖旧元数据推断的 `InsertAndGetId`。表名/字段名来自代码，值绑定参数。

每个用户的资源查询都要包含 `owner_id`。不存在与非本人数据统一返回 404。附件属于上传人，头像引用也检查归属；数据库组合外键提供最终约束。

示例 `example_items` 包含完整分页 CRUD；`example_task_receipts` 演示任务幂等。把自己的事务写在 Logic 中，参照 `internal/logic/example/example.go`。

## 缓存与请求上下文

通过 `service.Current().Cache` 使用 JSON 缓存的 `Get(ctx,key,&value)`、`Set(ctx,key,value,ttl)`、`Remove(ctx,key)`，键自动加 `redis.cache.prefix`，TTL 至少 1 毫秒。缓存缺失返回 `false,nil`，Redis/反序列化错误单独返回。

认证身份通过 `internal/library/contexts.Get(ctx)` 获取。不要接收客户端提交的用户 ID 作为本人操作的身份；不要缓存账号状态替代认证时数据库检查。

## 移除示例

示例独立位于 `api/example`、`internal/controller/example`、`internal/logic/example`、`internal/service/example.go`、两张 `example_*` 表、队列 `example:*` 处理器。删除时同步移除 Bootstrap 注册、路由绑定和集成测试；保留 `storage:cleanup`。已经应用的初始 SQL 仍保留，需要删示例表时新增迁移，不修改或删除原来的 `002_example.sql`。消费端当前依赖数据库是因为内置幂等示例处理器，按自己的任务需求调整资源初始化。

## 验证

```sh
make check
make race
```

预期测试全部通过、vet 无诊断、`bin/starter` 生成。具体真实服务验证见 [testing.md](testing.md)。
