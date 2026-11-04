# Redis 部署与首页诊断 Implementation Plan

> 执行方式：当前会话实施并独立只读审查。用户已授权在服务器建立 Redis；当前无可用服务器执行通道，先完成可执行、可 review 的工具包，服务端执行保持待验收。

**Goal:** 提供不影响现有业务容器的 Redis 安装工具和安全的首页请求耗时报告。

**Architecture:** 独立 Compose 项目接入已有 Cost Docker 网络；缓存数据不持久化。Python 只读分析现有 Go request 日志，不修改业务接口。

**Tech Stack:** Docker Compose、Bash、Python 3；固定 `redis:8.4.8-alpine`。

**Spec:** [设计](../specs/2026-11-04-redis-homepage-diagnostics-design.md)。

## 约束与审查重点

- 不映射 6379、不触碰现有 API/MySQL/Resume，不覆盖密码或未知目录/容器。
- Redis 128 MiB 数据上限、256 MiB 容器上限、0.5 CPU、认证及只读文件系统。
- 诊断只输出聚合路径耗时，不转储日志或真实数据；无样本/读取失败明确报告。
- 初次安装失败仍保留已生成的配置以便重试，成功必须有认证 PING 与无主机端口证据。
- 无真实 Docker/服务器访问时明确标记未验证，不将替身测试写成线上部署成功。

## Task 1：Redis 安装工具

文件：`deploy/redis/compose.yaml`、`scripts/setup-cost-redis.sh`、`scripts/tests/test_redis_setup.py`。

- [x] 先写行为测试并运行，确认因缺少安装脚本失败。
- [x] 实现首次生成凭证、受管目录重跑、网络/容器归属检查、只启动 redis、认证 PING 与无端口校验。
- [x] 验证所有错误路径不会启动/停止无关服务，不输出密码；运行 Bash 与 Compose 内容检查。补充其他目录接管、端口查询失败及并发安装的失败回归后修复。

## Task 2：请求耗时报告

文件：`scripts/profile-cost-home.py`、`scripts/tests/test_homepage_profile.py`。

- [x] 先写固定日志测试，验证单位、分位数、接口归一化、敏感字段丢弃与无样本/错误行为。
- [x] 实现纯解析与 Docker 日志 CLI，验证测试通过。
- [x] 更新 `docs/ops/2026-11-04-redis-homepage-diagnostics.md` 和索引，生成包含部署配置、两个脚本及三份说明的工具包（6 个白名单文件）。

## Task 3：交付与实测

- [x] 运行 scripts Python 全套（43 项通过、1 项仅 GitHub runner 执行的 Docker smoke 跳过）、相关语法/链接/差异检查，独立审查并处理重要发现。增加 GitHub runner 真实 Redis smoke，不将其等同生产验收。
- [x] 提交新分支 `infra/redis-homepage-diagnostics` 及 [PR #15](https://github.com/LeeMrChang/cost/pull/15)；现有自动部署白名单不包含新工具，不把合并 PR 等同 Redis 已启动。
- [ ] 取得服务器执行通道后创建 Redis、检查健康与资源限制、复现写账返回慢并采集耗时；不能执行时明确保留为待完成。
