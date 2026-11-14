# 小程序游客模式验证记录

日期：2026-11-14
设计：[游客模式设计](../superpowers/specs/2026-11-14-miniprogram-guest-mode-design.md)
计划：[实施计划](../superpowers/plans/2026-11-14-miniprogram-guest-mode.md)

## 背景

2026-11-14 微信审核驳回：打开即要求授权手机号、头像、昵称，未先提供体验。本次整改为「游客可体验 + 延迟登录」。

## 改动

| 文件 | 改动 |
|---|---|
| `miniprogram/app.json` | 启动页由登录页改为账本页 |
| `miniprogram/utils/api.js` | 移除无会话/401/403 的自动跳登录；新增 `isAuthenticated()`；登录未填昵称时提交「微信用户」 |
| `miniprogram/utils/nickname.js` | 新增 `optional()`，空昵称合法 |
| `miniprogram/pages/login/*` | 昵称可选，登录后直接进入；历史无昵称账号自动补默认昵称 |
| `miniprogram/utils/guest-store.js` | 新增：本地游客记录增删改查，`client_request_id` 稳定 |
| `miniprogram/utils/guest-categories.js` | 新增：与后端同名的 24 个游客分类，含按名称映射到服务端分类 |
| `miniprogram/utils/guest-merge.js` | 新增：登录后逐条合并，失败保留本地 |
| `miniprogram/pages/ledger/*` | 游客态读本地、隐藏预算卡、顶部登录提示；登录态进入时自动合并 |
| `miniprogram/pages/entry/*` | 游客态可记账/编辑，隐藏账户条，分类用游客表 |
| `miniprogram/pages/entry-detail/*` | 游客态读取、编辑、删除本地记录 |
| `miniprogram/pages/stats/*` | 游客态只算当月本地数据，其他范围提示登录 |
| `miniprogram/pages/search/*` | 游客态本地检索，跳转本地详情 |
| `miniprogram/pages/calendar/*` | 游客态本地日历与按日明细 |
| `miniprogram/pages/settings/*` | 游客态显示登录入口，管理入口提示登录 |

后端、数据库、部署配置未修改。

## 自动化验证

执行时间：2026-11-14，工作目录 `/Users/silent/Developer/personal/cost`。

```sh
node --test miniprogram/tests/*.test.js
node miniprogram/tests/validate.js
```

结果：

- `node --test`：**171 项通过，0 失败**（改动前 154 项）。
- `validate.js`：`JavaScript syntax, JSON and page files validated`。

新增测试覆盖：

- `guest-store.test.js`（7 项）：本地增删改查、id 递增、损坏数据回退、待合并计数。
- `guest-categories.test.js`（4 项）：24 项分类、按名称匹配、类型不混用、回退首项。
- `guest-merge.test.js`（5 项）：全部成功清空、失败保留、分类回退、空数据不发请求、分类接口失败不丢数据。
- `guest-mode.test.js`（12 项）：账本游客渲染不发接口、跨月过滤、登录态仍走接口、记账落本地、零金额拦截、本地编辑、登录后合并、统计本地汇总与范围限制、搜索本地过滤、日历本地数据、设置游客态、详情删除。
- 改写 `auth.test.js`、`nickname.test.js`：锁定「不再自动跳转」「昵称可选」的新行为。

## 未验证（必须人工完成）

1. 微信开发者工具：冷启动进入账本、游客记账、登录合并的实际界面与交互。
2. 真实微信登录（`wx.login`）与后端会话；本地只有 mock。
3. 真机（iOS/Android/桌面微信）表现。
4. 上传审核：需要用户在微信公众平台操作，本次不代为提交。

## 已知限制

- 游客数据只存 `wx.storage`，清缓存或卸载会丢失；设置页已提示。
- 游客统计只支持月范围；周/年/全部/自定义需登录。
- 游客记账户籍固定为「未指定账户」，登录合并后也不补账户。
- 分类名称若后端改名，合并会回退到同类型第一个分类，账单仍会入账但分类可能不准。
- 本环境未安装 Superpowers 技能，设计与计划按 `AGENTS.md` 约定手工执行，未做独立代码审查（`requesting-code-review` 不可用）。

## 提交方式

当次例外：本次改动按用户明确要求直接在 `main` 提交并推送，未走分支 + PR。默认流程已写入 `AGENTS.md`「分支与提交」：功能、修复与行为性重构先开分支，合并走 PR；只有用户当次明确同意才直接在 `main` 提交，且只对当次有效。

## 回退

- 代码：`git revert` 本次三次提交（`8df5e92`、`b5a6ad5`、`9a43de4`、`84a614e` 中相关部分）。
- 行为：恢复 `app.json` 首页为登录页、`api.request` 的 `goLogin()`，游客数据仍在本地，不影响已登录用户。
