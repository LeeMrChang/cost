# Redis 安装与记账返回首页耗时诊断

2026-11-04。[设计](../superpowers/specs/2026-11-04-redis-homepage-diagnostics-design.md) / [计划](../superpowers/plans/2026-11-04-redis-homepage-diagnostics.md)。用户实际反馈是新增账单后返回首页慢，本轮提供 Redis 安装工具和只读请求耗时报告，尚未在 Go 后端启用数据缓存。

## Redis 工具包操作

工具包仅包含 `deploy/redis/compose.yaml`、`scripts/setup-cost-redis.sh`、`scripts/profile-cost-home.py`，以及设计、计划、本说明共 6 份文件，不含 .env、密码、数据库或用户数据。通过服务器文件管理将工具包上传至 `/home/ubuntu/cost-redis-bootstrap-20261104.tar.gz`。

在服务器执行（如果解压目录已存在，先检查旧目录，不直接覆盖）：

```sh
mkdir -m 700 /home/ubuntu/cost-redis-tools-20261104
tar -xzf /home/ubuntu/cost-redis-bootstrap-20261104.tar.gz -C /home/ubuntu/cost-redis-tools-20261104
cd /home/ubuntu/cost-redis-tools-20261104
bash scripts/setup-cost-redis.sh
```

第一次安装默认使用官方 `redis:8.4.8-alpine`。服务器如需沿用现有腾讯云镜像源，在首次安装时改为：

```sh
COST_REDIS_IMAGE=mirror.ccs.tencentyun.com/library/redis:8.4.8-alpine bash scripts/setup-cost-redis.sh
```

腾讯云镜像源是否已同步该标签需以实际拉取为准。如果首次拉取失败，已生成的配置仍保存在 `/home/ubuntu/cost/redis/`；仅编辑 `.env` 中的 `COST_REDIS_IMAGE` 更换可用镜像源，保留其余值，然后重跑。脚本不会覆盖已有密码或自动升级不同的 Compose 配置。

安装要求：Ubuntu/Linux、Bash、Python 3、OpenSSL、flock、Docker Compose v2 和 Docker 权限；无普通用户 Docker 权限时尝试已有 `sudo -n docker`。前提是 Cost 已有 `cost-cloud_default` 网络；若网络名不同，先用 `docker network ls` 核对，在首次安装通过 `COST_REDIS_NETWORK=实际网络名` 指定。不要凭猜测连接其他项目的网络。

成功标志：脚本完成等待健康检查、认证 PING，并验证没有主机端口映射后打印 `Redis ready`。之后可只读检查：

```sh
docker ps --filter name=cost-cache-redis-1
docker port cost-cache-redis-1
docker stats --no-stream cost-cache-redis-1
```

`docker port` 应没有输出；内部别名为 `cost-redis:6379`，与现有 Cost 网络中的 API 可互通。密码只保存在服务器权限 600 的 `/home/ubuntu/cost/redis/.env`，不在终端输出，也不要把文件内容提交 Git 或贴到聊天。

Redis 使用非 root 用户、只读根文件系统、128 MiB 数据上限、256 MiB 容器上限、0.5 CPU 及 `allkeys-lru`。RDB/AOF 均关闭；它是可重建缓存，不代替 MySQL 持久存储。本工具不重启现有 API/MySQL/Resume，不修改业务环境变量。当前 Go API 尚未配置 Redis 客户端，因此容器启动不代表首页已经命中缓存。

需要停止时，仅操作这个项目：

```sh
docker compose --project-name cost-cache --env-file /home/ubuntu/cost/redis/.env -f /home/ubuntu/cost/redis/compose.yaml stop redis
```

## 采集新增账单后返回首页的耗时

在手机按正常流程新增账单并返回首页，复现后立即在上述工具包目录运行（只选本次复现的短时间窗，如 2 分钟）：

```sh
python3 scripts/profile-cost-home.py --since 2m
docker stats --no-stream --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}'
```

普通用户无 Docker 权限时，在 Python 命令前加 `sudo`。若 API 容器名不同，通过 `--container 实际容器名` 指定。报告只显示白名单接口的方法、路径及样本数、p50/p95/最大毫秒数，不显示原始日志、用户/账单数据、查询参数或令牌；单条账单 ID 被归一为 `{id}`。最多读取 10000 行，p95 使用最近秩分位数，小样本只适合定位明显瓶颈，不代表长期 SLA。

源码已确认的现象：

- 首页 `onShow` 先清空原列表，再刷新 `/auth/me`、`/categories`、`/stats`、`/ledgers/me`、`/accounts`、`/entries` 共 6 个请求；5 个业务请求通过 Promise.all 汇合，最慢的一个决定内容展示时间。
- 每个认证请求都做会话查询及 `LedgerForUser`，后者为已有账本也开事务并锁用户行；统计读取整个月的账单，账户余额做账单/转账汇总。
- 当前 request 日志的 duration 覆盖服务端认证与存储，不能覆盖手机 DNS/TLS、网络传输或 WXML 渲染。必须结合真机 Network 耗时，不能只靠服务端 p95 宣称根因已定位。

截图还显示 `cost-cloud-backup-1` 在重启（126）；这是单独的异常信号，需读取实际容器日志和资源占用，不能直接认定它导致首页慢。本轮不修改备份服务。

## 缓存是否合理

Redis 适合重复读取，但用户描述的“新增后返回”天然需要更新账单、统计、余额。若写入后全部正确失效，第一次返回通常仍是未命中，单独加缓存不保证这次变快。不能为了更快而显示旧的资金数据。

建议按实测排序：

1. 若后端均很快、手机仍慢，先减少请求往返和清空后整页等待，评估首页聚合接口、已有账本的快速只读查找。
2. 若统计或账户汇总明显慢，再优化查询/索引，并对重复结果缓存。分类/账本资料可缓存约 5 分钟；月统计及余额/首屏候选 TTL 为 10–30 秒，具体值待实测。
3. 缓存键必须包含用户、账本、月份/范围及日期；成功写入后及时失效，防止旧读并发回填。认证、会话原文、错误响应不进入业务结果缓存；Redis 失败用短超时回退 MySQL。任何接入均需单独测试跨用户隔离、记账/删除/恢复/转账/预算/资料更新后的新鲜度。

本轮没有修改业务接口或前端加载行为，后续接入方案仍需根据测量结果 review。

## 实际验证与未完成项

- 本机 `python3 -m unittest discover -s scripts/tests -v` 共 **43/43** 通过，包括 9 项安装器测试、6 项日志解析/CLI 测试和既有部署/备份/打包测试。安装测试中的 Docker 为边界替身；并发互斥测试使用真实双进程和文件锁。
- YAML 解析、无 ports/资源限制/只读参数及容器 shell 语法已检查；新增 CI `Redis container smoke test` 在隔离 GitHub runner 实际启动 Redis，检查认证、限额、缓存读写和重复安装。CI 结果以当前提交为准。
- 独立审查发现并修复：其他目录同 Compose 项目的接管风险、不同安装目录并发绕过目录锁，以及端口查询失败被误报成功；均有失败回归后通过。安装使用主机固定锁 `/tmp/cost-cache-install.lock`，持锁完成归属检查和启动。
- 当前直接 SSH 被执行环境拒绝，桌面会话工具不可用；本机无 Docker/Redis。**云端 Redis 创建、真实 PING/资源检查及首页耗时采集尚未执行**，不把替身测试或 GitHub runner 容器当成生产部署。
- CI/CD 的生产打包白名单不包含这些新工具；合并 PR 不会自动在服务器创建 Redis。服务器部署工具必须通过上面的独立步骤执行。
