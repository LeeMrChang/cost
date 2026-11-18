# Redis 资料缓存启用与验收

> 公开文档脱敏说明（2026-11-18）：个人路径、部署标识及真实账本数据已省略或改为占位示例。历史验证结论保留，不代表使用占位示例重新执行。示例路径使用前须替换；部署脚本的内置默认路径以源码为准，本次未调整脚本。

2026-11-04。[设计](../superpowers/specs/2026-11-04-redis-reference-cache-design.md) / [计划](../superpowers/plans/2026-11-04-redis-reference-cache.md)。用户明确不再以卡顿为前提，目标是让 Cost 实际使用 Redis。

## 当前状态与缓存范围

用户在服务器执行安装后提供的截图显示 `cost-cache-redis-1 Healthy`、`Redis ready`，使用腾讯云镜像源的 Redis 8.4.8，内部地址 `cost-redis:6379`；安装脚本的认证与无端口校验已完成。这个生产状态依据用户截图，不是本会话直接 SSH 检查。

本次后端代码只缓存分类列表和账本名称/图标等资料（300 秒），分类/账本修改成功后立即逻辑失效。认证仍走原校验，金额、账单、统计和余额不缓存；Redis 不可达时短超时回退 MySQL，health 不依赖 Redis。HTTP 响应结构保持一致，无需发布小程序版本，无 SQL 迁移。

## 上线顺序：先配置，再 review/合并后端

Redis 已创建，不需要再次运行安装器。本次新增 `scripts/configure-cost-redis.py`，请上传该脚本到服务器 `/path/to/server-user/configure-cost-redis.py`。它只修改现有 API 环境文件，不重启服务，也不输出密码：

```sh
python3 /path/to/server-user/configure-cost-redis.py
```

路径示例（实际读取位置以脚本配置为准）：从 `/path/to/server-user/cost/redis/.env` 读取，向 `/path/to/server-user/cost/deploy/.env` 写入：

- `COST_REDIS_ADDR=cost-redis:6379`
- `COST_REDIS_PASSWORD=服务器已有的随机密码`

脚本原子写入并设权限 600，保留数据库/微信配置。源或目标不存在、是符号链接、源格式不合法、目标包含重复 Redis 设置时拒绝修改。支持单行赋值的空白、引号和 `export`；多行引号或无法安全识别的语法会原样拒绝，避免损坏其他凭证。不要把 `.env` 内容贴到聊天、提交 Git，或将密码写在 shell 命令参数里。非默认目录时使用 `--redis-env` / `--api-env`；非默认 Docker 网络时须自行核对 API 和 Redis 可互通。

配置完成后 review PR #15，合并后由现有 CI/CD 构建发布后端。旧 API 不会因为新增两项环境文件内容而立即使用 Redis；必须完成这次新版 API 部署。Compose 未将 Redis 设为 depends_on，不会重启其独立容器。

## 验收真实命中

新版发布后，手机打开首页两次，在微信开发者工具或真机调试 Network 中查看 `/api/v1/categories` 与 `/api/v1/ledgers/me` 的响应头 `X-Cost-Cache`：

| 值 | 含义 |
|---|---|
| MISS | 读取数据库并尝试填充缓存；首次、过期或修改后的第一次读取正常如此 |
| HIT | 本次资料结果来自 Redis |
| BYPASS | 未启用缓存，或 Redis 读取/填充失败；请求仍从数据库正常返回 |

重启 API 会更换命名空间，因此首次 MISS 正常。修改分类或账本名称/图标后再读取应直接看到新资料；不是等待五分钟刷新。头部只用于诊断，无用户/凭证信息。

服务器如需确认出现了缓存键，在手机打开首页后运行下面命令，只输出键数量，不展示缓存内容或密码：

```sh
docker exec cost-cache-redis-1 sh -ec 'REDISCLI_AUTH="$REDIS_PASSWORD" redis-cli --scan --pattern "cost:reference:v1:*" | wc -l'
```

数量大于 0 说明至少成功填充；以第二次请求的 HIT 作为命中证据。旧代次键保留到 TTL 自动清理，不使用全库清空。

## 失效和运行边界

当前部署单 API 进程：每次启动随机命名空间 + 原子代次 + 账本 ID + 类型。成功资料修改在返回前推进代次；内部错误或提交结果不确定也保守失效，确定的验证/冲突错误保留缓存。旧读只能回填旧键；Redis 断线期间修改也推进代次，因此重连不复活旧值。不缓存业务错误；损坏 JSON 读取回源。

多个 API 同时写同一数据库时，需要先设计共享失效。本轮不支持横向扩容下的即时一致性。手工 SQL 或离线工具修改这两种资料后重启 API 可立即生效，否则最多五分钟。其他资金查询不受资料缓存影响。

禁用：在服务器 API `.env` 中把 `COST_REDIS_ADDR` 和 `COST_REDIS_PASSWORD` **同时清空**，再重新创建 API 容器或走正常后端部署。只执行 `docker restart` 不会重新读取 Compose 环境文件。无需删除 Redis 或修改数据库。

## 验证记录

- API 新增测试先因缺少缓存选项失败，再实现后 API 全套通过；验证命中/到期、资料写后失效、失败写保留缓存、跨账本/登录失效、并发旧读、断线修改、损坏/超时回源及启动隔离。
- 配置和安全写文件脚本先运行失败测试；脚本全套本机 50 项通过、1 项真实 Docker smoke 按环境跳过。
- 本机最终 `go test -race -count=1 ./...`、`go vet ./...`、`go build ./...` 均退出 0。MySQL 集成未配置时跳过；显式尝试本机 MySQL Unix socket 被拒绝（operation not permitted）。Redis 超时用例也因 loopback bind 被拒绝而本机跳过，真实 Redis 用例无服务时跳过；这些边界由当前提交 GitHub CI 的独立 MySQL 与 Redis 容器验证，结果记录在 PR。
- 独立审查的两项重要问题和损坏 null 边界均先复现 RED 再修复 GREEN：dotenv 多行/重复设置保护；提交后回读失败仍失效（覆盖账本及分类的不确定提交）；带空白 null 不能作为 HIT。
- Redis 真实测试只在 GitHub 临时容器运行，未使用生产密码或数据库。当前会话仍无服务器执行通道；线上 API 配置、新版发布和手机 HIT 验证待用户执行。
