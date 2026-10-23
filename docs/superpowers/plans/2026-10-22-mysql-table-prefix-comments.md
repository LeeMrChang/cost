# MySQL 前缀、注释与账目迁移实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Mac 与云端统一 t_ 和中文注释，保留两端数据并将 Mac 业务数据合并云端。
**Architecture:** 保留旧迁移，新增原子改名及可重试注释迁移；Go 业务 SQL 使用新名。私有 staging 库用于恢复校验和事务合并，不替换云端身份权限。
**Tech Stack:** Go 标准库、MySQL 8.4、Docker Compose。
**Spec:** ../specs/2026-10-22-mysql-table-prefix-comments-design.md

## Global Constraints

- 不 commit/push；不改 API JSON；一个服务一个家庭；金额整数分。
- 真实 cost 数据库改动前停止 API 写入、备份且恢复验证；独立随机测试库。
- 固定 t_；10 张表/全部列注释；旧数据/约束/会话保留。
- 云端已有 2 笔、Mac 21 笔，合并保留云端数据，不导入本地权限令牌。

## Review Focus

- 新旧表并存或混合结构必须拒绝，不能静默丢数据。
- 改名后但版本未落库再次迁移可恢复。
- MODIFY COLUMN 不能丢失自增、默认时间和二进制排序。
- 同一用户本地/云端 ID 不同必须按 openid 映射。
- 重复导入不重复，相同幂等键/分类/月份冲突拒绝并回滚。

### Task 1: 迁移与 Go 适配

Files: backend/internal/store/mysql.go、新增 prefix.go、migrations/003_table_comments.sql；backend/internal/auth/mysql.go；backend/cmd/api/main.go；backend/internal/api/mysql_integration_test.go。
Interfaces: MySQL.Migrate(ctx) 保留签名；业务仓库签名不变。
- [x] 添加集成测试：新库 10 张 t_、注释全非空；旧 002 库账目/身份保留；重复迁移；已改名未记录 003 恢复；冲突拒绝。
- [x] COST_TEST_MYSQL_ADMIN_DSN=root@unix(/tmp/mysql.sock)/ go test ./internal/api -run MySQL：观察缺少 t_ 的红灯。
- [x] 实现结构预检、原子 RENAME、可重复注释及 t_schema_migrations 跟踪，改账目/认证 SQL，迁移超时 5 分钟。
- [x] 相同集成命令全绿；运行 race、vet 和构建。

### Task 2: 导入与文档

Files: backend/internal/store/merge.go、backend/cmd/merge-ledger/main.go（校验 staging 数据库名并使用参数化查询/受限标识符）、隔离集成验证脚本、README.md、docs/README.md、docs/ops/2026-10-22-mysql-prefix-data-migration.md。
Interfaces: 新 t_ 结构；staging 恢复 Mac 数据，云端 cost 为目标。
- [x] 在隔离 source/target 库准备不同 user ID、重复请求、预算及分类冲突样本，验证合并策略测试先失败。
- [x] 实现事务合并和预检；不改目标已有记录、不覆盖身份；重复执行相同内容跳过。
- [x] 隔离合并测试全绿，记录命令与结果。

### Task 3: 备份、真实升级与合并

- [x] 识别 Mac API 进程和云端容器，停止 API 写入；备份两端并恢复独立库验证，记录数据摘要。
- [x] Mac 显式迁移，校验行数与定义，启动新二进制。
- [x] 新源码包上传云端，保留旧镜像，构建、显式迁移，校验原有数据。
- [x] 本地升级后数据导入云端私有 staging，事务合并；来源 21 笔及预算存在、原云端 2 笔不变，重复合并无新增。
- [x] 启动云 API，验证健康与授权业务读写、独立审查；清理临时凭据与 staging，备份私有保留。
- [x] docs 记录实际结果、未覆盖项；本次保持未提交。

## 实际执行与裁决

- 用户明确接受方案并授权迁移 Mac 数据，同会话直接实施；不改变当前检出目录，不 commit/push。
- 数据合并使用独立运维 Go 命令，源/目标位于同一 MySQL 实例的隔离 staging 与真实 cost 库。相较 SQL 文本替换更易做真实集成测试，数据库名限字母数字下划线。
- 使用本会话 workspace 外部私有工作目录保存执行日志与备份，不把含个人数据的文件放进项目。
- Mac 旧进程不允许沙箱终止，用户在 Mac 终端停止后才升级；已恢复新版后端。
- 隔离测试红灯：缺少带注释前缀表；导入入口缺失导致编译失败。实现后真实 MySQL 集成测试、race、vet、构建均通过。
- 独立只读代码审查无 Critical/Important 发现。实际两端备份、恢复演练、数据指纹与导入去重结果详见 ops 文档。
- 当前升级后真实写入验证由隔离数据库完整工作流承担；生产只做健康、未授权拒绝和数据查询核验，不额外写入测试账单。
