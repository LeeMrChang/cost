# Cost 重定位（iCost 对标）M1 实施计划

日期：2026-10-26
设计依据：[设计文档](../specs/2026-10-26-icost-realignment-design.md)
需求依据：~/Documents/cost_need/《cost-原型与需求说明-iCost对标-v2.1.md》《cost_可交互原型_v4.html》与 22 张 iCost 实拍截图。

## 0. 已由 Master 拍板的决策

| 编号 | 决策 | 结论 |
|---|---|---|
| D 范围 | 本次范围 | M1 核心闭环（FR-01~06 + 月预算 UI），其余 P1/P2 后续 |
| D 预算卡 | 明细页预算卡 | 文档版：剩余月预算大字 + 总预算 + 剩余日均 |
| D 分类 | 默认分类 | iCost 实际使用集（支出 12 / 收入 12），种入不删改旧分类 |
| D 预算粒度 | 预算模型/UI | 模型周/月/年建全，UI 首版只做月 |
| D4 | 家庭三表 | 迁移 005 直接 DROP（迁移前备份） |

## 1. 任务顺序（每步先写失败测试，最小实现通过）

### A. 后端模型与迁移

1. `backend/internal/domain/domain.go`：Entry 新增 refund_of_id/reimbursable/exclude_stats/exclude_budget/currency；Budget 行结构（scope/period/category）；Stats/Range 结构与计算（周一起点、日均、剩余日均）；iCost 默认分类集（替换 DefaultCategories 内容，Memory 顺序分配 ID）。
   - 测试：`domain_test.go` 补 range 边界、日均/剩余日均、预算视图计算。
2. `backend/internal/store/migrations/005_icost_realignment.sql` + `applyICostRealignment`（mysql.go 或独立文件）：
   - 预算表重构（新表+校验计数+RENAME 回滚安全）、entries/categories/ledgers 加列（information_schema 幂等检查）、账本 owner 绑定与改名、created_by 归属、分类补种、DROP 家庭三表。
   - 测试：`mysql_integration_test.go` 构造 004 形态数据 → 应用 005 → 断言转换/归属/删除/种子。
3. `backend/internal/store/`：账本上下文（WithLedgerID/LedgerID）、`LedgerForUser`（不存在则建账本+种分类）、`Ledger`、`Budgets`/`SetBudget`（scope 模型）、`Stats`（五档 SQL 聚合，week/month/year/all/custom）。
   - Memory store 同步实现；测试相应更新。

### B. 后端 auth 与 API

4. `backend/internal/auth/`：删 Role、SetupSecret、family 端点与 Repository 方法；登录仅 upsert 用户+会话。
5. `backend/internal/api/`：中间件改为 认证 → LedgerForUser → 注入 ledger_id；删除 adminOnly/canEdit 角色逻辑（单人全权）；`GET /stats` 五档、`GET/PUT /budgets` scope 模型、`GET /ledgers/me`；entries 字段透传。
   - 更新 `api_test.go`/`auth_test.go`/`p0_test.go`/`p0_edges_test.go`/`mysql_integration_test.go`。

### C. 小程序

6. `miniprogram/utils/calc.js` + `tests/calc.test.js`：表达式引擎（连算、运算符覆盖、除零保持、分四舍五入）。
7. `miniprogram/pages/entry/`：全屏记账键盘（json/wxml/wxss/js）；POST /entries；保存再记；记住上次分类（storage）。
8. `miniprogram/pages/ledger/`：重写（自定义头、预算卡、折叠分组、悬浮+）；`app.json` 两 tab + 页面注册；`app.wxss` 视觉 token。
9. `miniprogram/pages/stats/`：五档 + 4 指标 + 柱状图 + 排行 + 预算入口。
10. `miniprogram/pages/budget/`：月预算页。
11. `miniprogram/pages/login/`、`pages/settings/`、`pages/categories/`：去家庭化、我的账本、分类宫格微调；`utils/api.js` 消息表更新。
    - 前端测试：calc、money、auth、p0-pages、validate 全部更新通过。

### D. 验证与文档

12. 全量验证：`go test -race ./...`、`go vet`、`go build`；隔离 MySQL 集成测试；Node 测试；微信原生 wcc/wcsc 编译（如可用）；`bash -n` 部署脚本。
13. 文档同步：README.md、AGENTS.md、requirements.md、docs/README.md、api 文档（新 stats/budgets/ledgers）、ops 验证记录。
14. 提交（按用户指示，不自动推送）。

## 2. 验收核对方式

按「iCost 截图 S01~S22 ↔ 原型 v4 屏 ①~⑯ ↔ 实现」三列对照；本阶段核对 S03/S05/S06/S09/S10（M1 范围屏），逐屏记录于验证文档。

## 3. 环境限制

- 本机无 Superpowers 技能与微信开发者工具交互能力：以等价流程执行（失败测试先行、独立审查或明确标注未审查）。
- 迁移 005 对真实库仅在用户显式执行 `./scripts/dev.sh -migrate` 后生效，执行前必须备份。
- 真机、真实微信登录、小程序发布不在本计划内。
