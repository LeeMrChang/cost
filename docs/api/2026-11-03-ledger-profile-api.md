# 账本资料编辑 API

2026-11-03。设置页顶部卡片对应账本资料，用户昵称单独由 [auth/me 接口](2026-11-03-wechat-nickname-api.md) 管理。

`PATCH /api/v1/ledgers/me`，需有效 Bearer 会话及已补填昵称。请求示例：

```json
{"name":"旅行基金","icon":"✈️"}
```

name、icon 可只传一个；省略或 null 保留原字段，至少一个非 null 字段。名称去首尾空白后 1–64 个 Unicode 码点，图标 1–32 个，支持中文、emoji；拒绝空白、控制字符、行分隔字符和非法 UTF-8。无效字段值返回 400 `invalid_ledger`；未知字段（包括 id、owner_id、currency、timezone）、损坏 JSON 返回 400 `invalid_json`。未登录 401，缺昵称 403 `nickname_required`。

返回最新 `{id,name,icon,currency,timezone}`。账本 ID 由会话上下文确定，只更新所属账本的名称/图标，不改变账本归属、账单或预算。GET 接口行为不变。

设置页点击卡片上的“编辑”展开表单：原名称/图标自动带入，提供 8 个常用图标，未换图标时保留当前值；取消不写入，保存失败保留草稿。保存后卡片更新，返回账本首页时通过原有 onShow 读取最新资料。

使用已有 `t_ledgers.name` 和 `icon` 列，无新增迁移；部署整体版本仍须先完成之前的 007 昵称迁移。验证边界见 [验证记录](../ops/2026-11-03-ledger-profile-validation.md)。
