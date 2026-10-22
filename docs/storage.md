# 文件与存储

## 启用驱动

完整字段在 `manifest/config/config.example.yaml`。`make setup` 只创建可立即运行的本地配置；使用云存储时，将样例中对应配置块合并到自己的 `config.yaml`，填写凭证、设置 `enabled: true`，再修改 `storage.default`。

| driver | 关键字段 | 私有链接 |
| --- | --- | --- |
| `local` | `root`、独立 `signingKey` | 应用签名下载地址 |
| `oss` | `endpoint` 含协议、`bucket`、`accessKey`、`secretKey` | OSS SDK 签名 |
| `cos` | `endpoint` 完整桶域名含协议、桶名和密钥 | COS SDK 签名 |
| `qiniu` | 桶名、密钥、`publicUrl` 下载域名 | 七牛私有地址签名 |
| `minio` | `endpoint` 不含协议、`secure`、桶名和密钥 | S3 预签名 |
| `ucloud` | `endpoint`、`bucketHost`、`fileHost`、桶名和密钥 | UFile SDK 签名 |

`storage.drivers` 的键是稳定的存储名称，例如 `oss-main`；`driver` 才是实现类型。附件保存上传时的存储名称、驱动、桶、对象键、大小、摘要和归属。切换默认存储只影响新上传，已有附件仍访问原存储。因此迁移之前保留旧名称、桶及启用状态；不要把同一个名称指向另一个桶。

命名配置支持环境覆盖，例如：

```sh
export STARTER_STORAGE_OSS_MAIN_ACCESS_KEY='自己的 AccessKey'
export STARTER_STORAGE_OSS_MAIN_SECRET_KEY='自己的 SecretKey'
```

其他可覆盖字段为 `SIGNING_KEY`、`ENDPOINT`、`BUCKET`、`PUBLIC_URL`、`ROOT`，名称中的连字符转换为下划线。`enabled`、`secure` 等仍通过配置文件设置。

## 上传与访问

`POST /api/v1/files` 接收 multipart 表单 `file` 和 `private`，默认私有。大小默认最多 20 MiB，允许 MIME 类型在配置中指定；服务端读取内容识别类型、校验扩展名并计算 SHA-256。不做跨用户文件去重。

上传落对象后才写入附件元数据，写库失败会尝试删除对象。文件与数据库无法组成分布式事务；如果补偿删除也失败，需要按存储和附件记录核对孤儿对象。

- `GET /api/v1/files`：本人附件分页列表。
- `GET /api/v1/files/{id}`：本人元数据。
- `GET /api/v1/files/{id}/url`：按需获取地址；私有链接默认 15 分钟，不将过期签名永久写库。
- `DELETE /api/v1/files/{id}`：删除本人对象及记录；被头像引用时返回 409，先清除头像引用。

本地内容经 `/api/v1/files/{id}/content` 下载，不把私有目录挂为静态资源；目录使用 `os.OpenRoot` 限制路径。生产环境 `server.baseUrl` 必须是客户端能访问的 HTTPS API 地址。

云存储私密性取决于桶权限。`private: true` 用来生成私有地址，不能将已经公开的云桶变为私有。推荐默认私有桶；如需永久公开文件，使用独立的公开存储和对应访问策略。SDK 上传/删除及真实链接须用自己的账号验证。

## 本地分片续传

当前仅本地驱动支持此接口，云驱动明确返回 501 `STORAGE_CAPABILITY_UNSUPPORTED`。

1. `POST /api/v1/files/multipart`，传 `name`、`size`、完整文件 SHA-256 的 `digest`、`chunkSize`、`private`。
2. 保存返回的 `uploadId`。`chunkSize` 为 64 KiB 至 4 MiB，默认 1 MiB，总大小仍受 `storage.maxBytes` 限制。
3. `PUT /api/v1/files/multipart/{uploadId}/parts/{index}`，表单字段 `file`，索引从 0 开始，允许乱序。
4. `GET /api/v1/files/multipart/{uploadId}` 查询已收分片，恢复上传。
5. `POST /api/v1/files/multipart/{uploadId}/complete`。服务器按数字顺序合并、校验总大小和摘要、登记附件。

分片状态包含用户归属，文件锁协调同一共享目录上的并发完成；重复完成返回原附件。附件 `upload_id` 唯一约束覆盖进程重启后的重复入库。会话默认 24 小时过期，由队列清理任务删除临时目录。

多个 HTTP 实例使用本地驱动时需要共享可支持文件锁的持久目录，消费者清理任务也需要挂载同一目录；否则选择单实例本地存储或对象存储。

## MinIO 开发环境

可选示例在 `compose.storage.yaml`。MinIO 官方该版本说明要求自行从源码构建容器，镜像由使用者提供。参考 [MinIO 官方发布说明](https://github.com/minio/minio/releases/tag/RELEASE.2025-10-15T17-29-55Z)。

```sh
export STARTER_MINIO_IMAGE='你已构建或审核的镜像:固定版本'
docker compose -f compose.yaml -f compose.storage.yaml up -d minio
```

控制台为 `http://localhost:9001`，默认开发账号在 Compose 中，正式环境请替换。先创建 `starter` 私有桶、配置最小权限应用凭证，再启用 `minio-main`。本示例不会自动创建或修改桶权限。

## 验证

本地运行 `go test ./internal/library/storager/...`；真实流程按 [testing.md](testing.md) 上传后下载比较内容、删除后确认对象和附件都不存在。每个云驱动分别验证私有签名、过期、删除、权限错误；未配置真实账号时不算云联调通过。
