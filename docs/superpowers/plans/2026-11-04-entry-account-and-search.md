# 账单账户标签与搜索回归 Implementation Plan

> **For agentic workers:** 用户已确认方案并授权直接在功能分支实施；当前会话按 TDD 执行。Superpowers 技能本体在当前环境未安装，按 AGENTS.md 要求如实报告该缺口，以本计划与设计文档对齐流程，不声称完整执行技能脚本。

**Goal:** 记账可选账户标签并让明细展示账户，使账户余额真正随账单变动；为"备注或分类名"搜索补回归测试。
**Architecture:** 纯前端接线（复用既有 `/api/v1/accounts` 与 `account_id` 契约）+ 后端既有行为的回归测试。
**Tech Stack:** 原生小程序（JS/WXML/WXSS）、Go。
**Spec:** [设计](../specs/2026-11-04-entry-account-and-search-design.md)

## Global Constraints

- 不改 `/api/v1` 契约、无迁移；`account_id=0` 表示未指定账户。
- 不绑定银行卡或任何外部账户；账户仅为用户自建标签。
- 未指定账户的明细行不显示账户名，保持现状观感。

## Review Focus

- 编辑已停用账户的账单时，选择条仍可见原账户且带"（已停用）"标记。
- PATCH 显式传 0 时后端清除账户；未传时保留旧值（既有语义，前端始终显式传值）。
- 账本/搜索/日历三处列表均传入真实账户列表；分页追加时不丢账户映射。
- 记住上次账户只在可见（启用或编辑原账户）中命中才默认选中。

### Task 1: 设计与计划文档

Files: `docs/superpowers/specs/2026-11-04-entry-account-and-search-design.md`、`docs/superpowers/plans/2026-11-04-entry-account-and-search.md`。

- [x] 落盘设计说明与实施计划。

### Task 2: 后端搜索回归测试（红绿证据）

Files: `backend/internal/api/p0_test.go`（或新增测试文件）。

- [x] 写测试：按备注关键词命中、按分类名命中、两者均不命中为空；执行时发现既有 `p0_test.go` 已覆盖全部三条路径，未重复新增，改为强制重跑（`-count=1`）确认行为保持，如实记录为回归锁定而非 TDD 红灯。
- [x] `go test -race ./...`、`go vet ./...`、`go build ./cmd/api` 通过。

### Task 3: 前端账户选择与明细展示

Files: `miniprogram/pages/entry/index.{js,wxml,wxss}`、`miniprogram/pages/{ledger,search,calendar}/index.{js,wxml}`、`miniprogram/tests/account-entry.test.js`（新增）。

- [x] 写页面测试（红灯）：默认选中记忆账户、无记忆默认不指定、保存携带 `account_id`、编辑回填（含已停用原账户）、账本列表组数据带账户名。9/9 红灯确认。
- [x] 实现账户选择条、记住上次、编辑回填；账本/搜索/日历拉取并传入账户。
- [x] Node 全套通过（116/116）；更新 README 功能边界与验证记录。

### Task 4: 文档与验证记录

Files: `README.md`、`docs/ops/2026-11-04-entry-account-and-search-validation.md`。

- [x] README 功能边界同步；验证记录写明命令、环境、结果与未执行项（开发者工具、真机）。
