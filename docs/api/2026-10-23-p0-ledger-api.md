# 搜索、分类、账户、转账与导出 API

## 共同规则

使用现有 `Authorization: Bearer <token>`，要求家庭成员身份。管理员管理分类和账户；成员可以查看家庭数据、创建自己的收支与转账、导出所能查看的家庭账单；仅创建者或管理员能修改/删除记录。

金额为整数分，人民币；日期按 Asia/Shanghai。旧账单 `account_id=0` 表示未指定账户，不计入账户余额。所有字段和端点的最终行为由自动测试及实现验证。

## 账单搜索

`GET /api/v1/entries`

| 参数 | 说明 |
|---|---|
| month | YYYY-MM；未指定日期区间时默认当前月 |
| start_date / end_date | YYYY-MM-DD，起止日期包含当天；优先于 month |
| q | 备注包含匹配，关键词中的 `%` / `_` 按字面含义处理 |
| kind | income / expense |
| category_id | 分类 ID，包括停用的历史分类 |
| created_by | 记录人 ID；0 为历史未指定 |
| account_id | 账户 ID；0 为未指定账户 |
| min_amount_cent / max_amount_cent | 金额闭区间 |
| page / page_size | 从1开始，默认50条，最大100条 |

返回 `items`、`total`、`page`、`page_size` 和 `summary`；summary 含 `income_cent`、`expense_cent`、`balance_cent`，按全部匹配结果统计，不限于当前页。

`GET /api/v1/entry-authors` 返回可用于筛选的历史记录人 `{id}` 数组。返回记录人编号，不暴露微信 OpenID 或会话。

账单创建和修改沿用原端点，增加可选 `account_id`。更新时保留原停用分类/账户允许，切换到另一个停用项不允许。

## 分类

`GET /api/v1/categories` 返回全部分类，包括已停用分类。

- `POST /api/v1/categories`：`{name,kind,icon,sort_order,disabled}`。
- `PATCH /api/v1/categories/{id}`：更新名称、图标、排序及停用状态，不能改变收支类型。
- 返回字段：`id,name,kind,icon,sort_order,disabled`。

采用停用代替删除，历史账单展示最新名称；不提供二级分类、合并或批量迁移。

## 账户

`GET /api/v1/accounts` 返回共享账户数组，含 `id,name,type,opening_balance_cent,balance_cent,disabled`。

- `POST /api/v1/accounts`：`{name,type,opening_balance_cent}`。
- `PATCH /api/v1/accounts/{id}`：更新名称、类型、初始余额、停用状态。
- 类型：`cash`、`wechat`、`alipay`、`bank`、`other`。

初始余额表示纳入本账本之前的余额；余额允许为负数。当前余额=初始余额+关联收入-关联支出+转入-转出。账单更新、删除后余额同步变化。停用账户保留历史和余额。

## 转账

独立于收支记录，任何转账不进入月收支、消费统计或预算消耗。

- `POST /api/v1/transfers`：`{from_account_id,to_account_id,amount_cent,occurred_at,note,client_request_id}`。
- `GET /api/v1/transfers?page=1&page_size=50`：返回 `items,total,page,page_size`。
- `DELETE /api/v1/transfers/{id}`：仅创建者或管理员；同步撤销两侧余额影响。

金额必须为正，两个账户不同且启用。创建者由服务端确定。相同请求 ID、相同内容重试返回已有记录；不同内容冲突，不重复计入余额。需要修改时删除后重建。

## CSV 导出

`GET /api/v1/entries/export` 使用同样的筛选参数，忽略分页。导出上限50000条，超出返回400 `export_too_large`，不得静默截断。

包含账单时间、收支类型、分类、金额、备注、记录人和账户。UTF-8 BOM，CSV标准引用，文本中的公式前缀经过处理。只导出收支；转账在账户页查看。

小程序保存CSV到本地临时文件后由用户主动选择分享/保存；不会自动发送给其他人。CSV文件包含家庭消费信息，导出入口应明确说明范围。

## 迁移与验证

004 迁移在旧库基础上增加分类状态/排序、账户、账单账户引用和转账。执行前备份；使用正常迁移命令，先迁移再启动新版服务。测试使用独立随机命名数据库，禁止使用真实账本库。

见 [设计](../superpowers/specs/2026-10-23-p0-ledger-design.md)、[计划](../superpowers/plans/2026-10-23-p0-ledger.md)、[备份操作](../ops/2026-10-23-automatic-backups.md)。
