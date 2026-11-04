# 首页头像展示 Implementation Plan

> **For agentic workers:** 用户要求直接改并通过 PR review；在当前分支按 TDD 实施，最后独立只读审查。

**Goal:** 首页头像优先，无头像回退现有账本图标。
**Architecture:** 当前会话缓存即时渲染，me 刷新资料，复用 avatarView 解析 API 地址；不改后端。
**Tech Stack:** 原生微信小程序 JS/WXML/WXSS、Node 测试。
**Spec:** [设计](../specs/2026-11-04-ledger-header-avatar-design.md)

## Review Focus

当前会话身份、资料读取失败、设置返回后的头像刷新、旧图片错误迟到。

### Task 1: 首页头像

Files: `miniprogram/pages/ledger/index.{js,wxml,wxss}`、`miniprogram/tests/p0-pages.test.js`。

- [x] 写有/无/刷新/错误事件测试并观察失败。
- [x] 复用 profile.avatarView，新增头像图片与失败回退，保留点击设置。
- [x] Node 全套与页面检查，独立审查；记录真机未验证。
- [x] 实施完成后上传 PR #13，等待用户 review，不合并或部署。

## 执行记录

按用户既定方式在当前分支实施，没有运行逐任务脚本。独立审查无重要问题；其慢资料请求建议已补充红灯回归测试并拆分为独立异步刷新。Node 最终 107/107、页面完整性与差异检查通过。远端完整 CI 随 PR 执行，以当前提交检查为准。
