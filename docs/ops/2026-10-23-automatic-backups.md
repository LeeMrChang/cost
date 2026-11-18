# 自动备份与隔离恢复验证

> 公开文档脱敏说明（2026-11-18）：个人路径、部署标识及真实账本数据已省略或改为占位示例。历史验证结论保留，不代表使用占位示例重新执行。示例路径使用前须替换；部署脚本的内置默认路径以源码为准，本次未调整脚本。

对应 F04。代码提供自动运行能力；本次没有部署、读取或恢复真实业务库。首次部署前应在测试服务器验证完整 Compose 服务。

## 服务与默认策略

`deploy/compose.cloud.yaml` 包含两个常驻服务，复用部署所用的 MySQL 8.4 镜像：

- `backup`：启动后立即备份 `cost` 数据库，成功后等待 86400 秒再执行；成功文件保留 14 天。失败等待 300 秒重试。
- `backup-verify`：启动后恢复最新的成功备份，成功后每 604800 秒（7 天）再验证；没有备份或恢复失败时每 300 秒重试。

在服务器现有 `deploy/.env` 中可以设置：

```dotenv
COST_BACKUP_INTERVAL_SECONDS=86400
COST_BACKUP_RETENTION_DAYS=14
COST_BACKUP_VERIFY_INTERVAL_SECONDS=604800
COST_BACKUP_RETRY_SECONDS=300
```

参数必须为正整数。周期从上一次任务完成后计算，不绑定每天的某个时刻。

备份使用 `cost` 账号和现有 `COST_MYSQL_PASSWORD`，密码只通过子进程环境传递，不出现在命令行或脚本日志。现有账号已具有 `cost.*` 的 SELECT、INSERT、UPDATE、DELETE；当前全部业务表为 InnoDB，导出使用 `--single-transaction --no-tablespaces --set-gtid-purged=OFF --skip-triggers`，不需要增加 RELOAD、PROCESS、LOCK TABLES 或 TRIGGER 权限。当前没有数据库触发器、存储过程、事件和视图；未来引入这些对象时，必须同步扩展备份范围与授权。

## 文件、失败处理与健康状态

自动备份存放在 Compose 的 `backups` 命名卷（通常为 `cost-cloud_backups`），不在源码目录；文件名为 `cost-<UTC时间>-<随机后缀>.sql.gz`。目录权限 0700，文件权限 0600。备份含完整数据库，包含微信标识和会话哈希，应作为私密数据管理，不加入 Git。

每次先写临时目录，检查 mysqldump 成功、SQL 非空与 gzip 完整性，再在同一文件系统原子改名。新备份完成之后才清理超过保留天数的 `cost-*.sql.gz`。失败不替换旧文件、不执行过期删除；临时文件正常退出时会清理。SIGKILL 或宿主机掉电可能留下 `.backup.partial.*` 或恢复临时目录，需要确认没有任务运行后由运维清理。

`backup.status` 保存最近尝试的结果、Unix 时间和成功文件名。失败会写 `failed`，输出不含 SQL、客户数据或数据库密码的错误提示。健康检查要求最近状态为 `success` 且不超过备份间隔加 1 小时。恢复服务同样提供 `verify.status`，使用其验证间隔。

Compose 为两个服务配置了健康检查和日志轮转。健康检查只报告状态，不主动发送告警；生产环境应把 `unhealthy` 和磁盘容量接入已有监控。

## 隔离恢复的边界

`backup-verify` 使用 `network_mode: none`，不挂载生产 MySQL 数据卷、不接收生产凭证、不挂 Docker socket，备份卷以只读方式挂载。它在独立的 `backup_verify` 命名卷创建全新的随机数据目录，执行 MySQL 初始化、启动仅开放该目录内 Unix socket 的实例，再恢复一份备份副本。

脚本只允许 `COST_ISOLATED_RESTORE=yes`，不提供目标服务器参数；所有客户端都显式指定 `--no-defaults --protocol=socket` 以及新建目录内的 socket。完成或失败后均停止该实例并清理临时数据。不能将该服务改为挂载业务数据卷或网络服务。

恢复成功的判定包括：gzip 校验、全部 SQL 导入成功、`mysqlcheck` 通过，以及核心表查询成功。私有 `verify-report.tsv` 位于 `/verify/status/`，包含：

- 备份文件名、UTC 验证时间和已应用迁移版本；
- 账单总笔数、收入/支出笔数及各自总金额（整数分）；
- 家庭成员人数、账户数和转账数。

全部12张业务/迁移表均需可查询，且报告必须含004迁移版本；缺失表或版本会使验证失败。报告权限为 0600，不写容器日志。它记录恢复后数据，方便人工核对；没有对运行中的源库做第二次查询比对，因为业务写入会使前后快照不同。失败保留上一份成功报告，须同时检查 `verify.status`，不能把旧报告当作最近验证成功。

## 部署与操作

自动部署打包程序会包含三个 worker 脚本。部署脚本完成原有上线前备份和恢复检查后，暂停定期备份/恢复服务，再执行迁移；API 恢复健康后按本次发布目录重新启动两个 worker。原有 `$root/backups/ci-*.sql` 上线前备份保持原策略，不受自动备份卷的 14 天清理影响。

迁移失败时 API 和这两个 worker 保持停止，先检查数据库和上线前备份再恢复服务。脚本成功结束表示服务已启动，首轮备份/恢复的实际结果由健康检查确认。

手动部署时，在选定的发布目录执行（沿用服务器上的真实 `.env` 路径）：

```sh
docker compose --project-name cost-cloud --env-file /path/to/server-user/cost/deploy/.env \
  -f deploy/compose.cloud.yaml up -d backup backup-verify
docker compose --project-name cost-cloud --env-file /path/to/server-user/cost/deploy/.env \
  -f deploy/compose.cloud.yaml ps backup backup-verify
docker compose --project-name cost-cloud --env-file /path/to/server-user/cost/deploy/.env \
  -f deploy/compose.cloud.yaml logs --tail 50 backup backup-verify
```

需要立即运行一轮时，先停止对应常驻 worker，避免同一状态文件和保留策略并发操作；运行结束后重新启动：

```sh
# 先执行一次备份；确认命令成功后再做恢复验证。
docker compose --project-name cost-cloud --env-file /path/to/server-user/cost/deploy/.env \
  -f deploy/compose.cloud.yaml stop backup
docker compose --project-name cost-cloud --env-file /path/to/server-user/cost/deploy/.env \
  -f deploy/compose.cloud.yaml run --rm --no-deps backup once
docker compose --project-name cost-cloud --env-file /path/to/server-user/cost/deploy/.env \
  -f deploy/compose.cloud.yaml up -d backup

docker compose --project-name cost-cloud --env-file /path/to/server-user/cost/deploy/.env \
  -f deploy/compose.cloud.yaml stop backup-verify
docker compose --project-name cost-cloud --env-file /path/to/server-user/cost/deploy/.env \
  -f deploy/compose.cloud.yaml run --rm --no-deps backup-verify once
docker compose --project-name cost-cloud --env-file /path/to/server-user/cost/deploy/.env \
  -f deploy/compose.cloud.yaml up -d backup-verify
```

正常启停使用 `docker compose stop/start` 或不带 `-v` 的 `down`；`down -v` 会删除数据库和备份卷。备份还应复制到独立受控存储；当前实现为同宿主机备份，不抵御整台服务器或磁盘丢失。真实故障恢复需要另行确认目标、停写并保留现状，此脚本不执行生产恢复。

## 本次验证记录

2026-10-23/24：先新增失败测试，验证 worker 实现缺失、部署未启动备份服务、发布包未包含 worker；实现后通过。随后新增私有核对报告和缺失核心表失败测试，观察失败后实现并通过。

- Python 行为测试覆盖成功/空备份/中途失败、保留策略、权限、过期健康状态、隔离开关、损坏备份、恢复失败、缺失核心表、私有报告、部署状态机和发布打包。外部 MySQL 进程使用受控替身；文件系统、gzip、状态及清理实际执行。
- `bash -n deploy/backup/*.sh scripts/deploy-cloud.sh scripts/setup-cicd.sh` 用于语法验证。
- 本机没有 Docker，尚未运行 Compose 容器。尝试使用本机 Homebrew MySQL 8.4.11 在 `/private/tmp` 新建独立实例进行实际恢复，但 `mysqld --no-defaults --initialize-insecure` 在初始化时 SIGSEGV；独立最小初始化命令亦复现相同崩溃，因此真实恢复未验证通过。未接触业务数据库。

上线前仍需在目标 MySQL 镜像上完成首次备份、失败重试、隔离恢复和报告核对，并确认磁盘容量与备份监控。
