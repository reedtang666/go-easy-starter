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

## 新增一个模块

1. 在 `manifest/sql/` 增加编号 SQL，先对自己的开发数据库执行。
2. `make tools`，复制 `hack/config.example.yaml` 为 `hack/config.yaml`，修改 `gfcli.gen.dao` 数据库连接与表名。
3. `make dao` 生成 DAO、DO、Entity。
4. 参照 `api/example/v1/example.go` 定义请求/响应，然后 `make ctrl`。
5. 在 `internal/logic/<module>` 实现 `sModule` 接收者的方法，再执行 `make service`。工具通过 `s` 前缀识别业务实现；构造函数不进入业务接口。
6. 在 `internal/bootstrap/app.go` 创建、注册服务，在 `internal/router/routes.go` 的合适路由组绑定控制器。
7. `make check`；涉及数据库/会话/上传边界时运行对应集成测试。

控制器生成会保留已存在方法实现；生成前后仍需检查 diff。不要手改 `internal/dao/internal`、DO、Entity 或生成的接口文件来实现业务规则。

GoFrame 运行库与项目 CLI 固定为 **v2.7.2**，Go 最低 **1.26.10**。使用项目 `.tools/gf`，不覆盖全局 GoFrame CLI。

## 数据库约定

SQL 文件使用事务，初始化需要显式执行 `make db-init`。后续版本 SQL由部署流程执行，程序启动不自动迁移。

PostgreSQL 18 新增的 NOT NULL 约束元数据会影响旧版 GoFrame 驱动的主键推断。因此单行新增通过 `dao.Store.InsertID` 的参数化 `INSERT ... RETURNING id` 获取主键；不要改回依赖旧元数据推断的 `InsertAndGetId`。表名/字段名来自代码，值绑定参数。

每个用户的资源查询都要包含 `owner_id`。不存在与非本人数据统一返回 404。附件属于上传人，头像引用也检查归属；数据库组合外键提供最终约束。

示例 `example_items` 包含完整分页 CRUD；`example_task_receipts` 演示任务幂等。把自己的事务写在 Logic 中，参照 `internal/logic/example/example.go`。

## 缓存与请求上下文

通过 `service.Current().Cache` 使用 JSON 缓存的 `Get(ctx,key,&value)`、`Set(ctx,key,value,ttl)`、`Remove(ctx,key)`，键自动加 `redis.cache.prefix`，TTL 至少 1 毫秒。缓存缺失返回 `false,nil`，Redis/反序列化错误单独返回。

认证身份通过 `internal/library/contexts.Get(ctx)` 获取。不要接收客户端提交的用户 ID 作为本人操作的身份；不要缓存账号状态替代认证时数据库检查。

## 移除示例

示例独立位于 `api/example`、`controller/example`、`logic/example`、`service/example.go`、两张 `example_*` 表、队列 `example:*` 处理器。删除时同步移除 Bootstrap 注册、路由绑定和集成测试；保留 `storage:cleanup`。消费端当前依赖数据库是因为内置幂等示例处理器，按自己的任务需求调整资源初始化。

## 验证

```sh
make check
make race
```

预期测试全部通过、vet 无诊断、`bin/starter` 生成。具体真实服务验证见 [testing.md](testing.md)。
