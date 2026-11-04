# MySQL 表说明、字段字典与前缀注释设计

建立日期：2026-10-22。2026-11-04 补录表说明与字段字典，结构基准为仓库迁移 **001–008 全部成功后的状态**。本文前半部分用于查阅当前结构；后半部分保留 2026-10-22 的历史设计与数据合并范围。

本文根据 SQL 文件和 Go 迁移逻辑核对，不代表已检查某个本地或线上数据库。实际版本需查询该库的 `t_schema_migrations`。后续结构变化应同步更新本文。

## 表清单与用途

原来的 10 张表中，家庭协作三表已在 005 删除；004 新增账户和转账两表。因此迁移至 008 后，正常完成升级的数据库共有 **9 张表、63 个字段**。`t_budgets_new`、`t_budgets_old_005` 仅是 005 迁移中的临时表，不属于最终业务表。

| 原表名 | 带前缀表名 | 用途与主要关联 | 008 完成后的状态 |
|---|---|---|---|
| `ledgers` | `t_ledgers` | 用户的个人账本，保存名称、图标、币种、时区；`owner_id` 关联用户，一个用户对应一个账本 | 保留；005 起不再是固定 ID 为 1 的家庭账本 |
| `categories` | `t_categories` | 账本内的收入/支出分类，保存名称、图标、顺序、停用状态及父分类；供账单和分类预算使用 | 保留 |
| `entries` | `t_entries` | 收支账单，记录金额、发生时间、备注、分类、账户和创建者；支持软删除及扩展标记 | 保留 |
| `budgets` | `t_budgets` | 账本的周/月/年预算及分类预算，金额单位为分；分类编号 0 表示总预算 | 保留；005 已替换旧的仅月预算结构 |
| `users` | `t_users` | 微信用户身份及昵称、头像资料；通过 OpenID 识别用户，通过 `id` 关联账本、会话及创建记录 | 保留；007 新增昵称，008 新增头像 |
| `sessions` | `t_sessions` | 用户登录会话，保存令牌摘要、所属用户和到期时间；用于服务端认证 | 保留 |
| `family_setup` | `t_family_setup` | 历史家庭管理员初始化状态，记录是否已绑定管理员 | 005 已删除，不再使用 |
| `family_members` | `t_family_members` | 历史家庭成员及管理员/普通成员角色；005 升级时可据管理员确定旧账本归属 | 005 已删除，不再使用 |
| `family_invites` | `t_family_invites` | 历史一次性家庭邀请，记录邀请令牌摘要、创建者、有效期及使用者 | 005 已删除，不再使用 |
| `schema_migrations` | `t_schema_migrations` | 已成功执行的迁移版本与完成时间，供迁移器判断是否需要继续升级 | 保留 |
| 无；004 直接创建带前缀名称 | `t_accounts` | 账本内的现金、微信、支付宝、银行卡等资金账户，保存期初余额；供账单、转账关联 | 004 新增，当前保留 |
| 无；004 直接创建带前缀名称 | `t_transfers` | 同一账本两个账户之间的资金转移；影响账户余额，不作为收入或支出统计 | 004 新增，当前保留 |

## 当前字段字典（迁移至 008）

约定：所有表使用 InnoDB；默认字符集为 utf8mb4，特殊标识字段另行注明。金额字段以整数**分**保存；业务发生时间及会话时间按 UTC 写入，统计按账本时区计算。下表“必填”表示数据库 `NOT NULL`，不是说客户端必须逐项提交；服务端会生成或补齐部分值。`BOOLEAN` 在 MySQL 中对应 `TINYINT(1)`。

### t_ledgers：个人账本配置（6 个字段）

| 字段 | 类型 | 空值、默认值与说明 |
|---|---|---|
| `id` | `BIGINT` | 必填，主键；非自增，由服务端分配，不再固定为 1 |
| `name` | `VARCHAR(100)` | 必填；账本名称，创建时由服务端填“我的账本”，可在设置中修改 |
| `currency` | `CHAR(3)` | 必填，默认 `CNY`；账本币种代码 |
| `timezone` | `VARCHAR(64)` | 必填，默认 `Asia/Shanghai`；账本统计时区 |
| `icon` | `VARCHAR(32)` | 必填，默认 `📒`；账本图标，与用户头像是两份独立资料 |
| `owner_id` | `BIGINT` | 可空；账本所有者，外键关联 `t_users.id`；旧记录尚未归属时可为 NULL |

主键为 `id`；唯一索引 `uk_ledger_owner(owner_id)` 限制同一用户只拥有一个账本。`fk_ledger_owner` 为数据库外键。首次访问业务接口时，服务端按当前会话查找或创建用户自己的账本，不接收客户端任意指定账本归属。

### t_categories：收支分类（8 个字段）

| 字段 | 类型 | 空值、默认值与说明 |
|---|---|---|
| `id` | `BIGINT` | 必填，主键；非自增，由服务端分配全局分类编号 |
| `ledger_id` | `BIGINT` | 必填；所属账本，外键关联 `t_ledgers.id` |
| `name` | `VARCHAR(100)` | 必填；分类名称，如餐饮、工资 |
| `kind` | `VARCHAR(10)` | 必填；业务值为 `income`（收入）或 `expense`（支出） |
| `icon` | `VARCHAR(32)` | 必填；分类图标 |
| `sort_order` | `INT` | 必填，默认 0；显示顺序 |
| `disabled` | `BOOLEAN` | 必填，默认 FALSE；停用后不能用于新增账单，历史引用仍保留 |
| `parent_id` | `BIGINT` | 可空；父分类编号，NULL 或 0 表示一级分类；当前分类编辑 UI 暂未开放层级操作 |

`parent_id` 是预留的逻辑关联，当前没有数据库自关联外键；不能把它视为已由数据库自动校验的层级关系。`kind` 由应用层校验，当前分类表没有对应的 CHECK 约束。

### t_entries：收支账单（18 个字段）

| 字段 | 类型 | 空值、默认值与说明 |
|---|---|---|
| `id` | `BIGINT` | 必填，自增主键；账单编号 |
| `ledger_id` | `BIGINT` | 必填；所属账本，外键关联 `t_ledgers.id` |
| `category_id` | `BIGINT` | 必填；收支分类，外键关联 `t_categories.id` |
| `kind` | `VARCHAR(10)` | 必填；`income` 收入或 `expense` 支出，数据库 CHECK 限制取值 |
| `amount_cent` | `BIGINT` | 必填；金额分，数据库 CHECK 要求大于 0，收支方向由 `kind` 表达 |
| `occurred_at` | `DATETIME(6)` | 必填；业务发生时间，UTC；月度归属等按此字段统计 |
| `note` | `VARCHAR(500)` | 必填，默认空字符串；账单备注 |
| `client_request_id` | `VARCHAR(64)` | 必填，utf8mb4_bin 排序规则；客户端幂等键，区分大小写 |
| `created_at` | `DATETIME(6)` | 必填，默认当前时间；记录创建时间 |
| `updated_at` | `DATETIME(6)` | 必填，默认当前时间，更新时自动刷新；记录最后修改时间 |
| `created_by` | `BIGINT` | 可空；创建用户，外键关联 `t_users.id`；兼容旧记录无创建者 |
| `account_id` | `BIGINT` | 可空；收支所在资金账户，外键关联 `t_accounts.id`；NULL 表示未指定 |
| `refund_of_id` | `BIGINT` | 可空；预留的退款原账单编号，逻辑关联 `t_entries.id`，没有自关联外键 |
| `reimbursable` | `BOOLEAN` | 必填，默认 FALSE；可报销标记，UI 暂未开放 |
| `exclude_stats` | `BOOLEAN` | 必填，默认 FALSE；不计统计标记，UI 暂未开放 |
| `exclude_budget` | `BOOLEAN` | 必填，默认 FALSE；不计预算标记，UI 暂未开放 |
| `currency` | `CHAR(3)` | 必填，默认 `CNY`；账单币种代码，当前没有汇率换算流程 |
| `deleted_at` | `DATETIME(6)` | 可空；UTC 软删除时间，NULL 表示正常账单；006 新增 |

索引：`uk_request(ledger_id, client_request_id)` 防止同一账本重复提交；`idx_ledger_time(ledger_id, occurred_at, id)` 支持按时间查询；`idx_entry_account_time(ledger_id, account_id, occurred_at)` 支持账户账单查询。

正常列表、统计及账户余额查询排除 `deleted_at IS NOT NULL` 的记录；删除不会释放原幂等键。当前恢复接口仅允许删除后 5 秒内撤销。退款、报销、排除标记属于模型/API 预留，不应仅因字段存在就认为已提供完整产品流程。分类/账户必须属于当前账本的规则还由应用层检查，单列外键本身不能保证跨表的 `ledger_id` 一致。

### t_budgets：总预算与分类预算（5 个字段）

| 字段 | 类型 | 空值、默认值与说明 |
|---|---|---|
| `ledger_id` | `BIGINT` | 必填；所属账本，外键关联 `t_ledgers.id` |
| `scope` | `ENUM('week','month','year')` | 必填；预算粒度：周、月、年 |
| `period_key` | `VARCHAR(16)` | 必填；周如 `2026-W40`，月如 `2026-10`，年如 `2026` |
| `category_id` | `BIGINT` | 必填，默认 0；0 表示总预算，非 0 表示对应支出分类预算 |
| `amount_cent` | `BIGINT` | 必填；预算金额分，数据库 CHECK 允许大于或等于 0 |

联合主键为 `(ledger_id, scope, period_key, category_id)`，同一账本、粒度、期间、分类只保存一条预算。分类编号是应用层逻辑关联，没有数据库外键，因为 0 是总预算哨兵值。API 写入金额为 0 时删除该预算行；无行表示未设置。当前小程序 UI 提供月预算，数据库模型保留周、年粒度。

005 将旧 `(ledger_id, month, amount_cent)` 结构转换为新结构，旧月预算变为 `scope='month'`、`period_key=原 month`、`category_id=0`。当前表已没有 `month` 字段。

### t_users：微信用户及个人资料（6 个字段）

| 字段 | 类型 | 空值、默认值与说明 |
|---|---|---|
| `id` | `BIGINT` | 必填，自增主键；应用内部用户编号，不作为页面昵称展示 |
| `openid` | `VARCHAR(128)` | 必填，ASCII 字符集、ascii_bin 排序规则、唯一；同一小程序内的微信身份标识，区分大小写 |
| `nickname` | `VARCHAR(64)` | 必填，utf8mb4，默认空字符串；007 新增，空字符串表示待补填，可由用户选择微信昵称或手填 |
| `created_at` | `DATETIME(6)` | 必填，默认当前时间；用户首次创建时间 |
| `avatar_url` | `VARCHAR(128)` | 可空，默认 NULL，ASCII/ascii_bin；008 新增，保存随机相对读取路径，不是微信临时文件地址 |
| `avatar_data` | `MEDIUMBLOB` | 可空，默认 NULL；008 新增，保存规范化后的 256×256 JPEG 图片字节 |

除 OpenID 唯一约束外，`uk_user_avatar_url(avatar_url)` 保证已设置的头像路径唯一；MySQL 允许多行 NULL。头像路径示例结构为 `/api/v1/avatars/<64位随机十六进制串>.jpg`，由小程序拼接 API 域名展示。新头像覆盖该用户的旧图片及路径，旧链接随后失效；图片随数据库备份，不依赖容器文件卷。

昵称和头像都需要用户主动选择或填写，`wx.login` / OpenID 不能直接带出新资料；再次登录可复用已保存的值。未设置头像时字段为 NULL，用户 JSON 的 `avatar_url` 返回空字符串；`avatar_data` 不通过用户 JSON 返回。头像上传与读取见 [头像 API](../../api/2026-11-03-wechat-avatar-api.md)，昵称规则见 [昵称 API](../../api/2026-11-03-wechat-nickname-api.md)。

### t_sessions：登录会话（3 个字段）

| 字段 | 类型 | 空值、默认值与说明 |
|---|---|---|
| `token_hash` | `CHAR(64)` | 必填，主键；会话令牌的 SHA-256 十六进制摘要，不是原始 Bearer 令牌 |
| `user_id` | `BIGINT` | 必填；登录用户，外键关联 `t_users.id`；同一用户可有多条会话 |
| `expires_at` | `DATETIME(6)` | 必填；UTC 到期时间，超时即不能继续认证 |

索引 `idx_session_expiry(expires_at)` 用于到期处理。当前服务签发 7 天有效会话；登录时清理过期会话，退出登录删除当前会话。数据库不保存会话原文，不应将令牌、OpenID 或备份内容复制进文档。

### t_accounts：资金账户（6 个字段）

| 字段 | 类型 | 空值、默认值与说明 |
|---|---|---|
| `id` | `BIGINT` | 必填，自增主键；账户编号 |
| `ledger_id` | `BIGINT` | 必填；所属账本，外键关联 `t_ledgers.id` |
| `name` | `VARCHAR(100)` | 必填；用户命名的资金账户 |
| `type` | `VARCHAR(16)` | 必填；应用层允许 `cash`、`wechat`、`alipay`、`bank`、`other` |
| `opening_balance_cent` | `BIGINT` | 必填，默认 0；期初余额分，可为负数 |
| `disabled` | `BOOLEAN` | 必填，默认 FALSE；停用状态，保留原账单与转账引用 |

余额不是本表中的单独存储列，而是查询计算：期初余额 + 正常账单收入 − 正常账单支出 + 转入 − 转出。这里的微信、支付宝账户是记账分类，不代表项目已接入支付或读取真实支付账户余额。

### t_transfers：账户间转账（9 个字段）

| 字段 | 类型 | 空值、默认值与说明 |
|---|---|---|
| `id` | `BIGINT` | 必填，自增主键；转账编号 |
| `ledger_id` | `BIGINT` | 必填；所属账本，外键关联 `t_ledgers.id` |
| `from_account_id` | `BIGINT` | 必填；转出账户，外键关联 `t_accounts.id` |
| `to_account_id` | `BIGINT` | 必填；转入账户，外键关联 `t_accounts.id` |
| `amount_cent` | `BIGINT` | 必填；转账金额分，数据库 CHECK 要求 `0 < amount_cent <= 100000000000` |
| `occurred_at` | `DATETIME(6)` | 必填；UTC 转账发生时间 |
| `note` | `VARCHAR(500)` | 必填，默认空字符串；转账备注 |
| `client_request_id` | `VARCHAR(64)` | 必填，utf8mb4_bin 排序规则；账本内唯一且区分大小写的幂等键 |
| `created_by` | `BIGINT` | 可空；创建用户，外键关联 `t_users.id` |

唯一索引 `uk_transfer_request(ledger_id, client_request_id)` 防止重复转账；`idx_transfer_time(ledger_id, occurred_at, id)` 支持查询。数据库 CHECK 禁止转出和转入账户相同；应用层另外检查两个账户均属于当前账本且可用。转账只改变账户余额，不计入收支汇总或预算支出；此表没有账单的 `deleted_at` 软删除字段。

### t_schema_migrations：迁移版本（2 个字段）

| 字段 | 类型 | 空值、默认值与说明 |
|---|---|---|
| `version` | `VARCHAR(64)` | 必填，主键；已完成的迁移文件名，如 `008_user_avatar.sql` |
| `applied_at` | `DATETIME(6)` | 必填，默认当前时间；该版本完成登记的数据库时间 |

迁移器持有数据库级命名锁，成功完成某版本的全部步骤后才登记版本。MySQL DDL 会隐式提交，部分执行失败时可能已有结构变化；需由可重入迁移检查状态后继续，不能只手工插入版本行跳过 DDL。

## 已删除的家庭协作表（历史说明）

以下用于理解截图和旧备份，**不是当前运行依赖**。005 在确定旧账本归属后删除带前缀及无前缀的家庭协作三表。

| 历史表 | 历史字段与含义 | 历史约束与替代方式 |
|---|---|---|
| `t_family_setup` / `family_setup` | `id TINYINT`：初始化槽位，主键，固定记录 1；`owner_id BIGINT NULL`：初始化时绑定的管理员用户 | `owner_id` 外键关联用户；当前按 `t_ledgers.owner_id` 保存个人账本归属，不再有全局家庭初始化 |
| `t_family_members` / `family_members` | `user_id BIGINT`：成员用户，主键；`role VARCHAR(16)`：`admin` 或 `member` | 用户外键及角色 CHECK；当前不维护成员角色，账本仅由所属用户管理 |
| `t_family_invites` / `family_invites` | `token_hash CHAR(64)`：邀请令牌摘要，主键；`created_by BIGINT`：创建人；`expires_at DATETIME(6)`：有效期；`used_by BIGINT NULL`：使用者 | 创建人、使用者外键关联用户；当前没有家庭邀请或共享账本流程 |

## 迁移来源与只读核查

结构不能只看某一个 SQL 文件：004/005 等迁移还在 Go 中执行增列、外键和索引步骤。阅读顺序及来源如下：

| 版本 | 主要结构变化 | 实现来源 |
|---|---|---|
| 001 | 账本、分类、账单、旧月预算 | [001 SQL](../../../backend/internal/store/migrations/001_init.sql) |
| 002 | 用户、会话、历史家庭协作表；账单创建者 | [002 SQL](../../../backend/internal/store/migrations/002_auth.sql)、[迁移入口及创建者外键](../../../backend/internal/store/mysql.go) |
| 003 | `t_` 前缀、表与字段中文注释 | [003 SQL](../../../backend/internal/store/migrations/003_table_comments.sql)、[前缀迁移](../../../backend/internal/store/prefix.go) |
| 004 | 账户、转账；分类排序/停用、账单账户关联 | [004 SQL](../../../backend/internal/store/migrations/004_ledger_features.sql)、[Go 扩展步骤](../../../backend/internal/store/p0_mysql.go) |
| 005 | 账本所有者及图标、父分类、账单扩展、三粒度预算；删除家庭协作三表 | [005 SQL](../../../backend/internal/store/migrations/005_icost_realignment.sql)、[Go 重构与归属迁移](../../../backend/internal/store/icost.go) |
| 006 | 账单软删除时间 | [006 SQL](../../../backend/internal/store/migrations/006_entry_soft_delete.sql)、[可重入步骤](../../../backend/internal/store/soft_delete.go) |
| 007 | 用户昵称 | [007 SQL](../../../backend/internal/store/migrations/007_user_nickname.sql)、[可重入步骤](../../../backend/internal/store/user_nickname.go) |
| 008 | 用户头像路径、图片字节及路径唯一索引 | [008 SQL](../../../backend/internal/store/migrations/008_user_avatar.sql)、[可重入步骤](../../../backend/internal/store/user_avatar.go) |

在需要核对的 `cost` 数据库连接中运行以下只读 SQL，即可核查实际版本、表和字段；不读取用户数据或图片内容：

```sql
SELECT version, applied_at FROM t_schema_migrations ORDER BY version;

SELECT TABLE_NAME, TABLE_COMMENT
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'
ORDER BY TABLE_NAME;

SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE,
       COLUMN_DEFAULT, COLUMN_KEY, EXTRA, COLUMN_COMMENT
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
ORDER BY TABLE_NAME, ORDINAL_POSITION;
```

升级仍须先备份并走显式迁移；不要把本说明作为手动建表/补列脚本。迁移执行方式见 [云端部署](../../ops/2026-10-22-docker-cloud-deployment.md) 和 [自动 CI/CD](../../ops/2026-10-23-automatic-cicd.md)。

## 历史设计（2026-10-22）

以下保留当时已确认的设计与授权范围。其中“10 张表”“家庭账本”等表述仅对应当时版本；当前表和字段以本文前半部分的 008 字典为准。

### 目标与范围

用户要求云端和 Mac 本地 Cost 数据库补充说明，表名增加 t_ 前缀，Go 后端适配。所有 10 张应用表及字段均添加中文 COMMENT；保留账目、分类、预算、微信用户、成员、会话、邀请以及迁移记录。接口 JSON 和小程序行为不变。本次不提交或推送 Git。

表名对照及当前状态已汇总到本文“表清单与用途”；三个家庭表的历史字段见“已删除的家庭协作表”。

### 技术决策（实施者选择）

固定前缀 t_，不引入动态任意前缀。正常业务 SQL 只访问新表，不建无前缀兼容视图，不双写。旧版 Go 程序不能在改表后继续服务，升级需短暂停止 API。兼容指新版迁移命令支持旧数据库升级，以及新版账目和认证 SQL 支持新表名。

保留既有 001/002 迁移历史语义；增加显式前缀及注释迁移。迁移器在同一数据库锁内识别旧/新迁移记录表，改名后继续追踪，避免重建空迁移表导致旧迁移重跑。全新库执行既有初始化再升级，最终只留下带前缀的 10 张表。

旧表完整存在时使用一次多表 RENAME TABLE，MySQL 自动更新外键引用，不复制或清空数据，不禁用外键。若同一逻辑表的新旧名同时存在或出现非预期混合结构，明确报错停止，不覆盖、合并或删除已有表。全旧和全新结构都可再次执行迁移；注释更新失败后可在全新表名上重试，完成全部步骤后才记录版本。

注释通过明确 ALTER TABLE / MODIFY COLUMN 补充，保持各字段原类型、可空性、默认值、自增、字符集和排序规则，保持主键、索引、CHECK 与外键语义。字段说明明确 amount_cent 单位、UTC 存储时间、月份格式 YYYY-MM、角色枚举、请求幂等键和 token_hash 的摘要属性。注释不包含凭据或实际用户数据。

迁移命令的上下文超时需适配真实 ALTER TABLE 所需时间；业务启动不隐式执行 DDL。更新迁移器、账目 SQL、认证 SQL、集成测试和运维查询文档。README 同步描述当前 Docker 部署状态和升级入口。

### 验证

先添加并实际运行会失败的隔离 MySQL 测试，再最小实现通过。测试覆盖：全新库、001 旧库保留历史账单、002 旧库保留用户与会话/成员/邀请/预算、重复迁移、改名完成但注释/版本记录未完成的恢复、冲突拒绝且数据不变。断言 10 张新表存在、旧表不存在、全部表和字段注释非空、关键列定义与外键/索引保留；通过 API 家庭工作流和账目增改查删验证 SQL 适配。

运行 go test -race ./...、go vet ./... 和编译；未设置 MySQL 测试连接导致跳过不能视为通过。测试只创建隔离随机数据库，绝不将真实 cost 库作为测试库。

### 本地及云端实施

代码和隔离验证完成后，分别确认 Mac/云端实际连接与服务进程；仅更新各自 cost 数据库，不涉及同实例其他数据库。两端依次执行，先 Mac 再云端。

每端先暂停 API 写入，再用私有配置执行完整数据库备份；验证备份可恢复到隔离库，并记录升级前的表行数与关键业务数据校验值。确认备份后使用有 DDL 权限的迁移账号执行迁移，重新构建/启动新版 API，再对比数据与注释、验证健康和登录后业务查询。云端业务账号保持现有 DML 权限，迁移权限不授予 API。

失败时保持 API 停止并保留备份，优先用可重入迁移修复。必要时在确认无新写入后协调反向改名及恢复旧镜像，迁移记录必须与回退结构一致；不在真实库直接删表或自动覆盖恢复。已恢复服务后出现新写入时，不允许以旧备份直接覆盖真实库。

Navicat 保存的旧查询需更新新表名。现有微信账号及有效会话不因改名重建。升级备份不进入 Git 或公开交付目录。

### 验收与边界

Mac 和云端两端各自 10 张表统一 t_、表/字段有中文注释、既有数据保持、Go 微信认证和账目读写通过，记录实际命令结果与未覆盖项。此次不做自动部署、域名/HTTPS 或小程序提审。不擅自初始化或更换管理员。

### 用户新增范围：本地业务数据迁移到云端

保留云端现有账目及身份权限。只合并账本配置、分类、账单和预算；不复制本地会话、邀请令牌、迁移历史或管理员绑定。账单创建者按相同微信 openid 映射云端用户，未知用户或相同请求标识但内容不同则停止，不猜测归属。相同分类 ID 需语义一致，冲突停止；预算月份已有相同金额则跳过，不同金额停止交由用户决定。数据合并使用事务，失败回滚，重复执行不增加重复账单。源导出文件仅保存在私有目录，迁移后用行数、金额和业务字段校验验证来源记录均存在且原有云端记录保持。两端备份和恢复演练通过后才改真实库。
