# 账本资料编辑 Implementation Plan

> **For agentic workers:** 用户已要求直接在功能分支改；当前会话按 TDD 实施，完成后独立审查。

**Goal:** 登录复用当前微信身份已保存昵称，设置页支持账本重命名及图标编辑。
**Architecture:** PATCH 当前账本接口配合上下文账本隔离；设置页内联编辑；登录先识别身份再判断补填。
**Tech Stack:** Go、MySQL、原生小程序。
**Spec:** [设计](../specs/2026-11-03-ledger-profile-editing-design.md)

## Global Constraints

- 仅改当前会话账本；name 1–64、icon 1–32 Unicode 码点。
- 不跨账号预填昵称；昵称缺失仍由后端拦截业务请求。
- 使用已有数据列，不迁移或部署真实环境。

## Review Focus

- 部分更新不覆盖另一个字段；伪造账本 ID 被拒绝。
- 多账号账本隔离及中文 emoji 边界。
- 编辑失败保留草稿，取消不发送请求。
- 已退出的老账号重登可复用服务端昵称。
- 新用户无昵称只能补填，不提前进入业务页。

### Task 1: 账本编辑接口

Files: `backend/internal/{api,store,domain}/ledger_profile*.go`、store.Store 与 API 路由。
Interface: `PatchLedger(ctx, domain.LedgerPatch) (domain.Ledger,error)`。

- [x] 写内存/MySQL 共用 HTTP 测试，观察旧路由 405 红灯。
- [x] 实现校验、当前账本部分更新、API 及隔离。
- [x] Go race/vet/build 验证；MySQL 未执行，已记录限制。

### Task 2: 页面编辑与登录

Files: `miniprogram/pages/{settings,login}/index.*`、页面测试。

- [x] 写编辑、取消、失败重试及昵称复用测试，运行红灯。
- [x] 实现内联编辑及两阶段登录；首页 onShow 保持读取最新资料。
- [x] Node 全套与页面检查；更新文档及独立审查。

## 审查与执行记录

独立审查未发现必须修复的重要问题，发现补填时会话过期会清空昵称草稿（P3）。本次补充同身份和不同身份重新认证测试，前者观察红灯后修复，后者验证不跨身份复用；Node 最终 93/93 通过。按用户明确的直接修改方式在当前会话执行，未使用逐任务脚本，不声称完整执行所有 Superpowers 脚本步骤。真实 MySQL 与微信平台验证仍待执行。
