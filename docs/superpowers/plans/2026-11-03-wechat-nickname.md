# 微信昵称记录 Implementation Plan

> **For agentic workers:** 使用 executing-plans 的逐任务执行思路；用户已明确要求直接切分支改，不再设置计划确认关卡。此次在当前会话实施，完成后请求独立代码审查。

**Goal:** 用户昵称存入 t_users，缺昵称必须补填后进入业务。
**Architecture:** auth 仓储保存和读取昵称，API 会话权限检查阻止未补填账号访问业务，原生表单提交资料。
**Tech Stack:** Go 标准库、MySQL、原生微信小程序 JavaScript/WXML/WXSS。
**Spec:** [设计](../specs/2026-11-03-wechat-nickname-design.md)

## Global Constraints

- 昵称 1–64 个 Unicode 码点；数据库 utf8mb4；空缺历史数据不猜测回填。
- 身份由微信 code / Bearer 会话确定，不信任客户端身份字段。
- 真实数据不在本任务修改；不部署、不推送主分支。

## Review Focus

- 微信昵称自动填充和失焦时序：表单提交读取最终值。
- 老会话缺昵称：403 保留会话并进入补填。
- 旧客户端省略昵称：保留已有昵称，不清空。
- emoji 与边界长度：按 Unicode 码点校验并在 MySQL 往返。
- DDL 与迁移版本写入间中断：重复执行恢复且旧数据不变。

### Task 1: 数据迁移、认证与权限

**Files:** `backend/internal/auth/{auth,memory,mysql}.go`、`backend/internal/store/{mysql,user_nickname}.go`、`backend/internal/store/migrations/007_user_nickname.sql`、`backend/internal/api/{api,auth_test,nickname_test}.go`。
**Interfaces:** Login(ctx, openid, nickname, hash, expires) 返回含昵称 Principal；UpdateNickname(ctx, id, nickname) 只更新指定已认证用户。

- [x] 写 HTTP 内存/MySQL 共用流程及旧用户迁移测试，运行观察昵称请求被旧代码拒绝的红灯。
- [x] 实现事务登录昵称保存、认证读取、当前用户 PATCH、业务访问拦截和重入安全迁移。
- [ ] 内存流程通过；真实 MySQL 测试因连接权限及临时实例初始化崩溃未完成。

### Task 2: 小程序昵称收集与展示

**Files:** `miniprogram/utils/api.js`、`miniprogram/pages/{login,settings}/index.{js,wxml,wxss}`、`miniprogram/tests/{auth,nickname}.test.js`。
**Interfaces:** api.login(nickname)、api.updateNickname(nickname)；用户对象包含 nickname；403 nickname_required 回登录保留会话。

- [x] 先测试登录阻止空昵称、老会话补填、失败可重试、成功缓存和设置页更新，运行红灯。
- [x] 实现昵称表单和会话同步；名字显示于设置页。
- [x] 执行 Node 全套测试和页面检查。

### Task 3: 交付验证与审查

- [x] Go race（MySQL 跳过）、vet、构建；Node 全套、页面检查；检查差异和密钥。
- [x] 更新 README、API 文档、验证记录与索引。
- [x] 独立审查并处理重要问题，交付分支和未执行的微信平台验证事项。

## 执行记录

- 分支：feat/wechat-user-nickname。原有改动仅为上一轮本任务的设计与索引。
- Pre-flight: Task 1 产生含 nickname 的用户响应，由 Task 2 消费；字段一致。Task 3 验证前两项，无接口冲突。
- 审查：独立 reviewer 发现退出/切后端残留昵称（P2），已通过失败保存→退出回归测试及清空状态修复；额外指出资料缓存竞态，已用乱序响应和跨 token 测试 RED→GREEN 修复。
- 执行方式：按用户“直接切分支改”在当前会话完成，没有逐任务提交/脚本工作区；保留实际红绿与最终验证记录，不声称完整执行 Superpowers 的所有脚本步骤。
