# Redis Reference Cache Implementation Plan

> **For agentic workers:** 按 `executing-plans` 在当前会话顺序实施；独立 reviewer 只读审查全量改动。用户已授权直接改分支并要求 PR review，不额外等待实现细节审批。

**Goal:** 已部署的 Redis 实际缓存分类和账本资料，写后立即更新且故障不阻塞记账。

**Architecture:** API 层可选类型化 cache-aside；单 API 的启动随机命名空间和原子代次隔离陈旧值；Go Redis 客户端独立包。

**Tech Stack:** Go 1.23+、go-redis/v9 v9.17.3、Python 3、Docker Compose。

**Spec:** [设计](../specs/2026-11-04-redis-reference-cache-design.md)。

## Global Constraints

- 只缓存两个成功的资料 GET，300 秒 TTL；认证与金额相关查询保持数据库行为。
- 缓存每个操作 80 ms，失败回退；GET 出错不继续 SET。
- 密码仅服务器保存，配置脚本无业务重启，不改 MySQL 或小程序。
- 单 API 进程；多进程共享数据库写入需要重新设计共享失效。

## Review Focus

- 写提交和并发读交错时，旧快照不能回填新键；专门用阻塞底层读取的测试验证。
- 断线期间完成修改后重连，旧缓存不得复活；测试恢复同一缓存后读取。
- 缓存开启不能绕过登录/会话失效或跨账本泄漏；走真实 HTTP/auth 服务测试。
- 配置错误不得泄漏密码或覆盖现有环境；使用隔离临时文件测试。
- Redis 可选；迁移、禁用、启动不可达、测试无 Redis 的跳过要有明确行为。

## Task 1：资料 cache-aside

文件：`backend/internal/api/reference_cache.go`、`reference_cache_test.go`、`api.go`、`p0.go`、`ledger_profile.go`。

接口：`ReferenceCache` 的 `Get(context.Context,string)([]byte,error)` / `Set(context.Context,string,[]byte,time.Duration) error`；构造参数 `WithReferenceCache` 作为可选 variadic Option。

- [x] 写缓存命中、隔离、写后失效、并发旧回填、故障/损坏、认证测试，确认 RED。
- [x] 最小实现 helper，保持原响应结构；修改成功推进原子代次。
- [x] 跑 API 全套，确认 GREEN。

## Task 2：Redis 客户端、运行配置及部署

文件：`backend/internal/cache/redis.go` 与测试、`backend/cmd/api/main.go` 与测试、`backend/go.mod/go.sum`、两份 `.env.example`、`deploy/compose.cloud.yaml`、`scripts/configure-cost-redis.py` 与测试、`scripts/tests/redis-smoke.sh`。

- [x] 配置与脚本先写失败测试；Redis 集成测试无服务时跳过，在现有 CI 真实容器执行。
- [x] 增加固定依赖、小池/无重试/80 ms 超时，构造可选缓存，不把 Redis 接入 health 或 migrate。
- [x] 安全配置脚本原子写入、保留其他环境值/权限/不输出凭证，Compose 注入可选变量。
- [ ] 跑 Go race、vet、build、脚本全套和 CI smoke。

## Task 3：交付

- [x] README、环境规范、索引、操作说明记录启用/回退/命中验证步骤，并补充服务器 Redis Healthy 的截图证据边界。
- [x] 独立审查并按失败回归修复重要发现。
- [ ] 提交并更新 PR #15 标题描述；等待当前提交 CI，保留用户 review，不自动合并。
- [ ] 线上配置/新版 API 发布与 HIT 验证尚需用户执行，明确区分代码完成和上线。

## 实际执行记录

- 核心资料缓存先运行缺少功能的失败测试，再实现并通过 API 全套。配置/脚本同样先失败后实现。
- 审查两项重要发现加 null 边界：先新增复现失败，再完成一次修复；最终 Go race 全套退出 0、vet/build 退出 0；脚本 50 项通过/1 项仅 CI smoke 跳过。无遗留审查问题。
- Go 本机无服务的 MySQL/Redis 集成，以及被禁止 bind 的 Redis 超时测试跳过；显式 MySQL socket 测试曾因权限拒绝失败，当前真实依赖验证交 CI。
- 旧写后读失败情形采用保守失效：不修改 SQL，验证/冲突错误仍保留缓存。这是对原“失败写保留”范围的明确细化，避免未知提交状态遮蔽数据库新值。
