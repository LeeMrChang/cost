# iCost 重定位：单人账本、统计与预算 API

日期：2026-10-26。取代 [微信登录与家庭 API](2026-10-22-wechat-auth-and-family-api.md) 中的家庭部分；登录与账单基础规则仍以原文为准。设计依据见 [重定位设计](../superpowers/specs/2026-10-26-icost-realignment-design.md)，实施计划见 [M1 计划](../superpowers/plans/2026-10-26-icost-realignment.md)。

## 身份与账本模型

- 微信登录后用户自动持有自己的账本：认证通过后中间件执行 `LedgerForUser`，无账本则自动创建（名称「我的账本」）并种入 iCost 默认分类（支出 12 / 收入 12）。
- 单用户单账本、单人全权：无角色、无家庭表、无邀请码。`/api/v1/auth/me` 只返回 `{id}`；`GET /api/v1/ledgers/me` 返回 `{id,name,icon,currency,timezone}`。
- 每个请求按会话绑定账本，跨用户账本完全隔离；旧的 `ledger_id=1` 概念废止。
- 创建者权限仍保留语义：账单、转账只允许创建者修改/删除；单人账本下即本人全权。

## 统计

`GET /api/v1/stats?range=week|month|year|all|custom&ref=YYYY-MM-DD&start_date=&end_date=`

- 五档口径：周（周一为起点）、月、年、全部、自定义日期范围。`ref` 为锚点日期，缺省为今天；`start_date`/`end_date` 仅 custom 使用（含尾日）。
- 返回：

| 字段 | 说明 |
|---|---|
| range / label | 口径与展示名，如「2026年10月」 |
| start / end | 区间端点日期；all 无 start |
| prev_ref / next_ref | 上一期/下一期锚点，供 ‹ › 切期；custom/all 为空 |
| summary | `income_cent`、`expense_cent`、`balance_cent`、`daily_avg_expense_cent` |
| series | 时间序列点 `{key,expense_cent,income_cent}`；天 `YYYY-MM-DD`、月 `YYYY-MM`、年 `YYYY` |
| categories | 支出分类汇总 `{category_id,amount_cent,count}`，按金额降序 |
| budget | 仅月档返回：`total_cent`（nil 未设置）、`remaining_cent`（超支为负）、`remaining_daily_cent`（剩余÷本月剩余天数；期间已过或剩余≤0 为 nil） |

- 日期边界按 Asia/Shanghai；周与年锚点切期不越界，不产生「2025-W53」这类伪周。
- 转账不计入任何统计。参数非法返回 400 `invalid_range`。

## 预算

- `GET /api/v1/budgets?scope=&period=` 返回 `{scope,period,total_cent,categories:[{category_id,name,icon,amount_cent,spent_cent,is_set}]}`；`total_cent` 为 nil 表示未设置总预算，`categories` 包含所有支出分类，`is_set` 表示是否设置分类预算，`spent_cent` 为所选期间的实际支出，未设预算也返回。
- `PUT /api/v1/budgets` 请求 `{scope,period,category_id,amount_cent}`：`category_id=0` 为总预算；`amount_cent=0` 删除该条预算。整行覆盖，无局部 PATCH。
- scope 支持 `week`（period `YYYY-Www`，ISO 周）、`month`（`YYYY-MM`）、`year`（`YYYY`）。模型三种粒度已建全，小程序首版 UI 只提供月预算。
- 校验失败返回 400 `invalid_budget`；期间与格式错误不落库。

## 账单字段扩展

`entries` 新增 `refund_of_id`、`reimbursable`、`exclude_stats`、`exclude_budget`、`currency`（默认 CNY）字段，接口透传保存；M1 UI 尚未提供对应操作，后续版本使用。其余账单、分类、账户、转账、导出接口行为与 [P0 接口说明](2026-10-23-p0-ledger-api.md) 一致，仅去掉成员/管理员权限表述。

## 迁移 005

`005_icost_realignment.sql` 及 Go 应用层迁移（幂等）：

- `t_ledgers` 增加 `icon`、`owner_id` 及唯一键，现有账本归属迁移时的已登录用户；
- `t_categories` 增加 `parent_id`；补种 iCost 默认分类，不改动旧分类；
- `t_entries` 增加上述五个新列；
- `t_budgets` 重建为 `(ledger_id, scope, period_key, category_id, amount_cent)`，旧月预算转为 `month` 行；
- DROP `t_family_setup`、`t_family_members`、`t_family_invites` 三表。

**真实库执行前必须备份**（见 [备份操作](2026-10-23-automatic-backups.md)）；迁移只通过 `./scripts/dev.sh -migrate` 显式执行，正常启动不改表。

## 主流程修复补充（006）

- `GET /api/v1/entries/:id`：读取当前账本的正常账单；删除或不属于当前账本返回 404。
- `PATCH /api/v1/entries/:id`：原有编辑必填字段不变；`account_id`、`refund_of_id`、`reimbursable`、`exclude_stats`、`exclude_budget`、`currency` 缺省时保留，显式传值时修改。解除账户传 `account_id:0`。
- `DELETE /api/v1/entries/:id`：写入 UTC 软删除时间，返回 204；账单即从查询、收支统计和账户余额中移除；保留原幂等键，重复 POST 不会复活账单。
- `POST /api/v1/entries/:id/restore`：删除后 5 秒内恢复并返回账单；超过期限返回 409 `request_conflict`；记录不存在、跨账本或未删除返回 404。
- 新增 `006_entry_soft_delete.sql`，需显式迁移后运行新版服务。软删除历史不会被合并工具作为正常账单重新导入：来源库含删除记录时拒绝合并，改用整库恢复。

## 错误码

`unauthorized`、`invalid_json`、`invalid_entry`、`not_found`、`request_conflict`（幂等冲突）、`invalid_range`、`invalid_budget`、`export_too_large`、`wechat_not_configured`、`wechat_unavailable`、`invalid_wechat_code`、`too_many_login_attempts`、`authentication_unavailable`、`internal_error`。
