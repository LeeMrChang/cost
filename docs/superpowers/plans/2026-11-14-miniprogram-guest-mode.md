# 实施计划：小程序游客模式（A+B）

设计：`docs/superpowers/specs/2026-11-14-miniprogram-guest-mode-design.md`

## 约束

- 先写失败测试再实现（TDD），每步跑 `node --test miniprogram/tests/*.test.js`。
- 只改小程序前端；后端、数据库、部署不动。
- 不删除现有登录页与既有测试；行为变化用新测试锁定。
- 提交前跑 `node miniprogram/tests/validate.js` 与完整测试。

## 任务

### 1. 游客数据层（纯逻辑，无 UI）

文件：`miniprogram/utils/guest-store.js`（新增）

- `list()` / `read()`：读 `cost_guest_entries`，返回数组。
- `add(entry)`：写入本地，生成 `local_id` 与 `client_request_id`。
- `update(local_id, patch)` / `remove(local_id)`。
- `clear()`。
- `pending()`：返回待合并数量。
- 存储异常（JSON 损坏）时回退为空数组，不抛错。

测试：`miniprogram/tests/guest-store.test.js`
- 增删改查与持久化；
- `client_request_id` 唯一且稳定；
- 损坏数据回退空数组。

### 2. 游客分类表

文件：`miniprogram/utils/guest-categories.js`（新增）

- 导出 `list()`，支出 8 项、收入 4 项，字段与接口分类一致：`id`（负数）、`name`、`kind`、`icon`、`sort_order`。
- 导出 `match(serverCategories, guestCategory)`：按名称匹配，失败回退同 kind 首项。

测试：`miniprogram/tests/guest-categories.test.js`
- 数量与 kind 分布；
- 名称匹配成功/失败回退。

### 3. 移除强制登录

文件：`miniprogram/utils/api.js`、`miniprogram/app.json`

- `app.json`：`pages` 顺序调整为账本页第一。
- `api.js`：
  - 新增 `isAuthenticated()`。
  - `request()` 默认不再 `goLogin()`；401 清会话并抛 `Error`（`code='unauthorized'`）；403 `nickname_required` 同理。
  - 保留 `goLogin()` 供主动跳转。
- 登录：`login(nickname)` 未传昵称时提交「微信用户」。

测试：`miniprogram/tests/guest-mode.test.js`
- 无会话时 `request` 不触发 `reLaunch`；
- 401 清会话但不跳转；
- `login()` 无昵称时提交默认值。

### 4. 账本页游客态

文件：`pages/ledger/index.js`、`pages/ledger/index.wxml`

- `onShow` 判断 `api.isAuthenticated()`：未登录走 `loadGuest()`。
- `loadGuest()`：读本地记录 → 用现有 `groups()` 渲染；月份过滤与摘要本地计算；预算卡隐藏；顶部提示条。
- 登录态保持原逻辑；`onShow` 若发现游客数据且已登录，调用合并。
- 顶部提示点击 → 登录页。

测试：`guest-mode.test.js`
- 游客态不调用 `api.request`；
- 本地记录渲染成 groups；
- 登录态仍调用接口（回归）。

### 5. 记账页游客态

文件：`pages/entry/index.js`、`pages/entry/index.wxml`

- 未登录：分类取 `guest-categories`，隐藏账户条，保存写 `guest-store`，编辑/删除走本地。
- 保存成功后返回账本并刷新。
- 登录态逻辑不变。

测试：`guest-mode.test.js`
- 游客保存写入 storage 且不调接口；
- 游客编辑本地记录；
- 缺失金额/分类仍拦截。

### 6. 合并逻辑

文件：`miniprogram/utils/guest-merge.js`（新增）

- `merge(api)`：逐条 POST，成功移除，失败保留并返回 `{ synced, failed }`。
- 分类映射用 `guest-categories.match`。
- 合并期间不阻塞 UI；异常不外抛。

测试：`guest-merge.test.js`
- 全部成功清空；
- 部分失败保留失败项；
- 重复请求使用同一 `client_request_id`（幂等前提）。

### 7. 统计 / 设置 / 其他入口

- `pages/stats/index.js`：未登录用本地记录算当月收支、分类排行；范围切到非月提示登录。
- `pages/settings/index.js`：未登录只显示登录入口与游客说明；预算、账户、分类管理入口跳登录引导。
- `pages/search/index.js`、`pages/calendar/index.js`：未登录只搜本地。

测试：`guest-mode.test.js` 覆盖统计摘要与设置页不调接口。

### 8. 文档与验收

- 更新 `README.md` 功能说明（游客可体验、登录后同步）。
- 更新或新增 `docs/ops/2026-11-14-miniprogram-guest-mode-validation.md`，记录自动化结果与待人工验收项。
- 记录技能缺口：本环境未安装 Superpowers 技能，设计/计划/测试按 AGENTS.md 约定手工执行。

## 验证命令

```sh
node --test miniprogram/tests/*.test.js
node miniprogram/tests/validate.js
```

## 验收

按设计文档「验收标准」1–8；微信开发者工具、真机、审核提交由用户完成，不在此计划内。

## 风险控制

- 每完成 1–2 个任务就提交一次，便于回退。
- 不修改后端；如发现必须改后端（如昵称默认值），先停下来单独设计。
