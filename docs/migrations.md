# 数据库迁移

数据库结构通过 `manifest/sql/` 下的编号 SQL 管理。程序启动不自动建表或升级；数据库命令只连接 PostgreSQL，不要求 Redis 或 API 正在运行。模块生成与开发流程见[开发说明](development.md)。

## 首次建表

先创建空 PostgreSQL 数据库，在 `manifest/config/config.yaml` 填写 `database.url`，然后执行：

```sh
make db-init
make db-status
```

`db-init` 只执行原始 `001_init.sql`、`002_example.sql`，逐条事务提交并记录 SHA256。成功输出 `database initialized`，库中共有 4 张业务表：`users`、`attachments`、`example_items`、`example_task_receipts`，以及迁移账本 `schema_migrations`。

它不会创建数据库本身，也不会重复初始化已有库。即使目录中有 `003` 及后续 SQL，首次初始化也只执行前两条；继续运行 `make db-up` 应用其余迁移。

## 变更表结构

```sh
make db-new NAME=add_article_status
# 等价：go run . db-new --name add_article_status
```

命令自动分配下一个编号，输出例如 `manifest/sql/004_add_article_status.sql`；实际路径以命令输出为准。此操作不读取配置、不连接数据库，也不覆盖已有文件。

打开新文件，写入 SQL。例如已通过 `make module NAME=article` 创建过文章模块，增加状态字段：

```sql
ALTER TABLE article_items
ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'draft';
```

生成的文件只有注释，需要填写可执行 SQL 才能应用。不要添加 `BEGIN`、`COMMIT` 等事务命令。确认目标配置后执行：

```sh
make db-status
make db-up
make db-status
```

成功输出 `database up to date; applied N migration(s)`。再次执行没有待办时，`N` 为 0，不重复执行。模块生成器已经附带完整建表 SQL，不需要为同一个模块再创建一份建表迁移。

`db-status` 为只读检查，会显示版本、文件、状态与应用时间：

| 状态 | 含义与处理 |
| --- | --- |
| `pending` | 尚未执行，确认 SQL 后运行 `db-up` |
| `applied` | 已执行，保持原文件不变 |
| `modified` | 已应用 SQL 的内容变化，恢复原文件 |
| `renamed` | 已应用 SQL 被改名，恢复原文件名 |
| `missing` | 已应用 SQL 被删除，恢复原文件 |

后面三种情况会阻止升级；低于已应用版本的新文件同样不能补执行。变更已经上线的结构时，创建更高编号的新迁移。

## 接管以前已建表的数据库

如果你之前手动导入过初始 SQL，或使用旧版 `db-init` 创建过 4 张业务表，数据库中还没有迁移账本：

```sh
make db-status
make db-baseline
make db-status
make db-up
```

`db-up` 会拒绝直接接管这种旧库，并提示先执行 `db-baseline`。Baseline 会核对原始 001、002 文件的校验值，以及 4 张表的核心字段类型、非空、默认值、主键、唯一约束、外键和检查约束；通过后在一个事务中记录前两条已应用，不重跑建表 SQL，不修改原有业务表或数据。

可以保留额外业务列，但核心结构必须符合初始模板。这个命令只接管未修改的原始 001、002，不提供通用的强制版本标记。校验不通过时，先检查提示和实际表结构；不要删除旧数据或篡改账本来绕过。已有迁移记录的库无需再次 Baseline。

## 指定环境、目录与超时

Makefile 默认使用 `manifest/config/config.yaml` 和 `manifest/sql`：

```sh
make db-status CONFIG=manifest/config/staging.yaml MIGRATIONS=manifest/sql
make db-up CONFIG=manifest/config/staging.yaml MIGRATIONS=manifest/sql
```

直接运行 CLI 时，数据库命令支持 `--config`、`--dir`、`--timeout`。默认总超时为 30 秒，包括连接、等待迁移锁和执行所有待办：

```sh
go run . db-up --config manifest/config/staging.yaml --dir manifest/sql --timeout=2m
go run . db-status --help
```

`db-new` 只支持 `--name`、`--dir`；`module` 不读取数据库配置。所有命令在项目根目录执行，路径相对于当前目录。

## 事务、并发和失败处理

每个文件在独立事务中执行，SQL 与它的账本记录一起成功或回滚。多个迁移进程通过 PostgreSQL advisory lock 串行运行；命令不会自动清空数据库或回滚已完成版本。

如果第 4 条迁移失败，第 1～3 条已提交的结果会保留，失败文件的 SQL 与记录回滚。此时先运行 `db-status`，确认失败版本仍为 `pending`，修正这条尚未应用的 SQL 后重新 `db-up`。数据库初始化的两条 SQL 也分别提交；中途失败时按实际状态通过 `db-up` 补完，不反复运行 `db-init`。

已应用文件不可修改、重命名或删除。仓库初始文件允许保留原有最外层 `BEGIN/COMMIT`，迁移器接管其事务；新文件直接写 SQL。脚本不能自行控制事务，因此不支持 `CREATE INDEX CONCURRENTLY` 等要求事务外执行的语句。当前不提供自动 `down`；需要撤销结构变更时，评估已有数据并创建新的补偿迁移。生产升级前应备份，并先在测试数据库执行相同迁移。
