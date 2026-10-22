# Docker 空账本部署 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 提供可审阅、无需 Git 提交即可上传的 Docker 空账本部署包和逐步操作文档。

**Architecture:** Compose 管理 api、mysql 和显式 migrate 任务。MySQL 内部访问，API 仅映射宿主机回环地址，通过 Mac SSH 隧道联调。

**Tech Stack:** Go 1.23+、MySQL 8.4、Docker Compose、Ubuntu linux/amd64、POSIX shell。

**Spec:** [Docker 空账本部署设计](../specs/2026-10-22-docker-empty-ledger-deployment-design.md)

## Global Constraints

- 云端从空账本开始，不迁移或覆盖 Mac 数据。
- 不 commit、不 push；保留全部既有工作区改动。
- API 宿主机仅 127.0.0.1:8080；MySQL 不映射端口。
- production、mysql、wechat；CA 证书齐全，微信 HTTPS 可出站。
- 数据卷持久化，部署包排除密钥、备份、Git 元数据和小程序代码。
- 迁移和运行业务数据库账号分离，根密码不进入 API。
- 专题文档 YYYY-MM-DD-english-topic.md；同步 docs/README.md。

## Review Focus

- 缺失或含特殊字符的密码：必填检查，使用 hex 随机密码并验证 DSN。
- 空数据库首次启动慢：数据库认证查询健康后再显式迁移。
- 现有卷不重跑初始化脚本：文档拒绝将已有卷当成新库，不提供清空真实卷的捷径。
- Docker 启动和持久化不等于可恢复：隔离环境进行重建及恢复验证。
- Mac 本地已有 8080、ARM64：隧道使用 18080，云镜像验证 amd64。

## Task 1: 可构建的最小后端镜像

**Files:** Create backend/Dockerfile, backend/.dockerignore。

**Interfaces:** Consumes backend/go.mod、cmd/api；Produces 镜像 cost-api:local，默认进程 /usr/local/bin/cost-api，接受 -migrate。

- [x] 明确当前环境的 Docker/构建能力；验证选定的官方 Go、Debian、MySQL 镜像标签，不假定可拉取。
- [x] 添加多阶段构建与 ignore；运行阶段非 root、含 CA，排除 .env、bin、.git。
- [x] 运行 docker build --platform linux/amd64 -t cost-api:local backend，检查镜像架构和非 root 用户；无法本机运行时保留云端验证步骤，不报通过。

## Task 2: Compose、数据库权限与启动顺序

**Files:** Create deploy/compose.cloud.yaml, deploy/.env.example, deploy/mysql/01-users.sh。

**Interfaces:** Consumes Task 1 镜像；Produces mysql、api 服务和 profile tools 下的 migrate 任务，项目名 cost-cloud。

- [x] 制作不含真实值的必填配置模板，使用独立 root、业务和迁移密码，云端 .env 不自动从 Mac 复制。
- [x] 实现空卷初始化账号权限、MySQL 认证查询健康检查、内部网络与命名卷。
- [x] 配置 api 回环端口、production/微信环境、非 root 镜像、日志轮转与 restart；migrate 仅手动执行。
- [x] 使用虚构配置运行 docker compose --env-file 测试配置 -f deploy/compose.cloud.yaml config --quiet；验证遗漏必填密码时失败。检查时不打印展开后的真实秘密。
- [x] 在隔离 Compose 项目和新测试卷上启动 mysql，运行迁移两次，查询 entries=0、默认分类存在、迁移版本齐全。
- [x] 验证业务账号 DML 成功而 CREATE TABLE 被拒绝，迁移账号可升级，根密码不在 API 环境中。
- [x] 启动 API，检查 /health 与未登录业务请求的 401；健康检查不被解释为真实微信联调通过。

## Task 3: 最小上传包和完整运维手册

**Files:** Create scripts/package-cloud.py, docs/ops/2026-10-22-docker-cloud-deployment.md; Modify docs/README.md。项目 README 只在相关段落追加入口，不覆盖已有未提交改动。

**Interfaces:** Consumes Dockerfile、Compose、迁移与账号初始化脚本；Produces 白名单打包的部署归档，输出到用户指定目录，包中只有 backend 构建源码、云部署文件和操作手册。

- [x] 实现明确白名单打包，默认输出到仓库外或用户指定路径，不包含 .env、bin、.git、备份、miniprogram。
- [x] 列出归档并检查无敏感文件，验证在临时目录解包后 Docker 构建所需路径齐全。
- [x] 手册写明服务器 mkdir/chmod、Mac scp、云端解包、生成密码、编辑配置、构建、数据库健康等待、显式迁移、API 启动、日志和停止命令，明确每条命令执行地点。
- [x] 加入 ssh -N -L 18080:127.0.0.1:8080 隧道、开发者工具设置和首次云端管理员绑定步骤。
- [x] 加入 mysqldump 备份、归档下载和独立测试库恢复验证；不提供 down -v 清空命令。
- [x] 用隔离测试数据验证重建后保留和备份恢复，完成后只清理本次明确创建的测试资源。

## Task 4: 审查、证据与交付

**Files:** Create docs/ops/2026-10-22-docker-cloud-validation.md。

- [x] 按 requesting-code-review 审查部署改动；没有独立审查工具/执行条件时明确记录缺口。
- [x] 对照设计逐项记录命令、退出码、跳过项；运行 git diff --check，检查文档链接及部署包不含真实密钥。
- [x] 确认未修改既有小程序代码和未覆盖用户文档更改；未执行 commit/push。
- [x] 交付本地部署文件及归档给用户审阅，再协助上传。真实服务器启动、微信联调与恢复结果仅在实际执行后记录通过。

配置/文档阶段不伪造 TDD 红灯；若实施需要修改 Go 行为，先按 TDD 写失败测试，再进行相应实现。

## 执行结果（2026-10-22）

Dockerfile、云端 Compose、账号初始化、最小打包及操作手册已制作；在云端完成独立测试环境的空卷迁移、权限、重建持久化和独立恢复后，正式空账本部署成功。域名/HTTPS 按用户要求不启用。用户禁止 commit/push，因此未创建 Git 提交，验收以 [执行记录](../../ops/2026-10-22-docker-cloud-validation.md) 为准；待用户过目后再决定提交。

执行方式补充：在用户指定的本地检出目录执行，保留现有工作内容；用户禁止提交，所以未创建新 Git 历史。用 Python 标准库白名单打包代替 shell 打包，以便拒绝符号链接和秘密文件。独立审查发现的 MySQL nounset 故障已在真实隔离新卷中复现并修复。
