# 账户接入与搜索回归验证记录

2026-11-04。改动范围：`miniprogram/`（记账页账户选择条、账本/搜索/日历明细行账户名）与后端零代码变更（搜索回归锁定）。设计与计划见 [设计](../superpowers/specs/2026-11-04-entry-account-and-search-design.md)、[计划](../superpowers/plans/2026-11-04-entry-account-and-search.md)。

## 环境

- macOS（darwin/arm64）；Go 1.27.2；Node v22.22.2。
- 分支 `feat/entry-account-and-category-search`（自 `docs/mysql-table-reference` HEAD 创建）。

## 已执行验证

| 命令 | 结果 |
|---|---|
| `node --test miniprogram/tests/account-entry.test.js`（实现前） | 9/9 失败（红灯，均为缺少预期行为） |
| `node --test miniprogram/tests/account-entry.test.js`（实现后） | 9/9 通过 |
| `node --test 'miniprogram/tests/*.test.js'` | 116/116 通过（原 107 + 新增 9） |
| `go test -race -count=1 ./internal/... ./cmd/...` | 全部 ok（api/auth/domain/store/cmd） |
| `go vet ./...` | 通过 |
| `go build ./cmd/api` | 通过 |

后端搜索说明：按备注与按分类名搜索的行为已由既有 `internal/api/p0_test.go` 覆盖（`q=%25_` 命中备注、`q=家庭` 命中分类"家庭用品"、未命中词为空），本次未修改查询代码，重跑确认行为保持。

## 新增测试覆盖

`miniprogram/tests/account-entry.test.js`：记账页默认选中记忆账户、无记忆回落"不指定"、保存携带 `account_id` 并记忆、编辑回填原账户且 PATCH 携带、已停用原账户保持可见（"（已停用）"标记）、切换选择；账本/搜索/日历明细组数据携带账户名。

## 未执行 / 待验证

- 微信开发者工具编译、真实微信登录链路、手机真机：本轮未执行，待用户在开发者工具验证（重点：记账页账户选择条渲染、保存后账户页余额变动）。
- MySQL 集成测试：本次无 SQL/迁移变更，未单独运行；环境配置 `COST_TEST_MYSQL_ADMIN_DSN` 后可随全量运行。
- 线上部署版本确认：分类名搜索依赖 `internal/store/p0_mysql.go` Query 中的 `LOCATE ... OR category_id IN (...)` 子查询，需确认 api.xiachen.xyz 当前部署包含该逻辑（该逻辑已存在于仓库主线历史提交中）。

## 合入 docs/mysql-table-reference 时的补录验证（2026-11-04）

按用户明确要求，将 `feat/entry-account-and-category-search` 的 `154b879` 快进合入 `docs/mysql-table-reference`，并通过既有 PR #14 提供 review。原工作区五份功能文档与该功能提交逐字一致，先暂存备份后合并，确认内容全部保留；未跟踪的 `.workbuddy/` 未加入提交。

独立只读审查复现并修复三项账户状态问题：刷新会将编辑中的新选择恢复成原账户（新建时的“不指定”也会被记忆值覆盖）；编辑停用账户后“保存再记”仍保留停用账户；无编辑 handoff 时，账户先返回会遗漏后到的停用原账户。修复只初始化一次默认值、保留原始账户列表供编辑完成后重算，并在转为新建时清除原账户例外。

- TDD：先新增 5 条回归，旧实现 9 项通过、5 项失败，分别暴露账户 ID 被覆盖及停用原账户不可见；复审再补账户列表延迟/失败时保存再记的 2 条回归，修复前分别因 `9 != 0` 失败。修复后前端全套 **123/123** 通过。
- `node miniprogram/tests/validate.js`、`git diff --check` 和合并后的文档相对链接检查通过。
- `GOCACHE=/private/tmp/cost-go-cache go test -race -count=1 ./...`、`go vet ./...`、构建 `/private/tmp/cost-account-merge-api` 均退出 0。本机未配置隔离 MySQL 管理连接，数据库集成测试跳过；本次没有后端或迁移变化。
- 开发者工具、真实微信与真机仍未针对账户功能验收，不以自动测试替代；远端完整 CI 及复审结果以 PR #14 的最新提交检查和说明为准。本次不合并 main、不部署。
