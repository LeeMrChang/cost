# F05 家庭成员维度 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** 完成昵称维护、成员快捷筛选、历史身份展示和 CSV 姓名。

**Architecture:** 认证仓储维护用户昵称和当前成员权限；业务仓储提供历史创建人编号，API 合并目录。前端共用成员展示函数，筛选仍使用 ID。迁移 005 保留原数据并可重试。

**Tech Stack:** Go 标准库 HTTP、MySQL、原生 JavaScript/WXML/WXSS、Node tests。

设计：[已确认设计](../specs/2026-10-24-family-member-dimension-design.md)。执行方式：当前会话逐项执行，完成后独立代理审查。用户已确认执行，不再次询问实施方式；保持现有检出目录。

## Task 1：昵称接口与历史目录

文件：新建 backend/internal/auth/profile.go、profile_mysql.go、profile_memory.go，backend/internal/api/member_profiles.go、member_profiles_test.go；修改 auth/auth.go、mysql.go、memory.go，api/api.go、p0.go，domain/domain.go。

- [x] 写并运行 HTTP 工作流失败测试：PATCH 本人昵称返回200，普通成员改他人403，非法/缺失/null/控制字符400；登录及 me 返回最新昵称；目录含无账单当前成员；成员移除后保留昵称和历史编号，重新加入恢复 active；CSV 姓名公式转义。
- [x] `GOCACHE=/private/tmp/cost-go-cache go test ./internal/api -run TestMemberProfiles -count=1` 确认因接口未实现失败。
- [x] 实现 `SetNickname(ctx, actorID, targetID int64, nickname string)`，在仓储锁/事务中重新检查真实角色和目标成员存在性；`Profiles(ctx, ids []int64)` 返回全部当前成员与指定历史成员的并集。HTTP 只返回展示资料，不暴露 openid。
- [x] API 合并 `store.Authors` 与认证目录并排序；CSV 追加名称、状态两列，沿用 csvText。
- [x] 重跑定向测试至通过。

## Task 2：迁移与隔离 MySQL

文件：新建 store/migrations/005_member_nicknames.sql、store/member_profiles.go；修改 store/mysql.go、api/mysql_integration_test.go、prefix_integration_test.go；新增 api/member_profiles_mysql_test.go；更新 deploy/backup/verify.sh 与 scripts/tests/test_backup.py。

- [x] 增加 MySQL 同一 HTTP 工作流测试；在既有数据库模拟 004 状态，写入旧用户和账单，运行迁移检查金额/成员关系/昵称默认值；删除005版本记录后重跑验证保留昵称。
- [x] 隔离数据库运行失败测试，确认缺 nickname 或接口失败。
- [x] 005 对 t_users 增加 nickname VARCHAR(80) NOT NULL DEFAULT '' 中文注释；在迁移锁内检查列存在、执行 DDL、最后记录版本。
- [x] 恢复脚本要求005版本及 nickname 列可查询；先补失败回归，再修改脚本。
- [x] 运行隔离 MySQL 全套，验证迁移5个版本及旧数据保留。

## Task 3：小程序

文件：新建 utils/members.js、tests/members.test.js；修改 utils/ledger.js、api.js，pages/ledger/index.js/wxml/wxss，pages/settings/index.js/wxml/wxss。

- [x] 先写失败测试：昵称与历史标识、重名编号、Unicode校验；“我的”保留其他筛选且重置页码；“全家”仅清成员条件；高级筛选同步；保存昵称重复提交保护、失败保留输入、权限拒绝、刷新资料、账号切换清理。
- [x] `node --test miniprogram/tests/members.test.js` 确认失败。
- [x] 以共用 `memberName` / `memberLabel` 生成显示；昵称校验去空白、20码点、禁控制字符。设置页加入编辑与清除；账本加入快捷筛选并将目录用于分页行装饰。
- [x] 转账列表原本显示创建人编号，也接入统一昵称展示；目录包含仅有转账引用的历史成员，不新增转账筛选。
- [x] 执行前端全部测试和 validate.js，检查 WXML/WXSS。

## Task 4：验收、审查、文档

- [x] 执行 `go test -race -count=1 ./...`（配置隔离 MySQL）、`go vet ./...`、`go build -o /private/tmp/cost-f05-api ./cmd/api`。
- [x] 执行全部 miniprogram/tests/*.js、`python3 -m unittest discover -s scripts/tests -v`、Shell语法检查及 git diff --check。
- [x] 按 requesting-code-review 请求独立审查，核实并修复重要问题，新增失败回归后修复。
- [x] 更新 requirements.md、README.md、docs/README.md，新增 docs/api/2026-10-24-family-member-api.md、docs/ops/2026-10-24-family-member-validation.md；写明实际红绿证据、运行结果和真机/部署未执行项。
- [x] 核对差异与敏感数据；交付本地实现和验证状态。
