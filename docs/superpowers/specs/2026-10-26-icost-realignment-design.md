# Cost 重定位（iCost 对标）M1 设计

> 2026-11-11 更新：账单 CSV 导出已移除，以下相关内容保留为历史设计或验证记录；当前范围见[移除设计](2026-11-11-remove-bill-export-design.md)。

日期：2026-10-26
依据：Master 提供的《cost-原型与需求说明-iCost对标-v2.1.md》与《cost_可交互原型_v4.html》（位于 ~/Documents/cost_need/），以及 22 张 iCost 实拍截图。
状态：本设计替代此前以「家庭共享账本」为前提的设计（见 2026-10-22 monorepo-and-family-auth、2026-10-23 p0-ledger）。旧设计中的账户/转账（F03）保留为 P1，不再阻塞主线。

## 1. 目标与非目标

目标（M1，已由 Master 拍板）：

- 单账本、单用户：微信一键登录后直达明细页，无初始化密钥、邀请码、成员、角色。
- 全屏记账键盘：支出/收入切换、5 列分类宫格、四则运算计算器、保存再记、时间与备注、记住上次分类。
- 明细页改版：月份切换、剩余月预算大卡（超支红字 + 总预算 + 剩余日均）、按日分组可折叠、悬浮「+」、tabBar 两 tab（账本/统计）。
- 存量数据无损迁移（23 笔云端账单全部归属本人），迁移 005 重构预算表并 DROP 家庭协作三表。
- 统计骨架：周/月/年/全部/范围五档 + 支出/收入/结余/日均支出 4 指标 + 柱状图 + 分类排行。
- 月预算页（模型按周/月/年三粒度建全，UI 首版只做月）：总预算 + 分类预算 + 进度条 + 超支红字。
- 默认分类采用 Master 在 iCost 实际使用集（支出 12 个：餐饮/交通/夜宵/公交/地铁/早餐/午餐/晚餐/消费/饮料咖啡/住房/其他；收入 12 个：工资/奖金/加班/福利/公积金/红包/兼职/副业/退税/投资/意外收入/其他）。

非目标（M1 不做，后续迭代）：

- 收支日历页（FR-09）、搜索页（FR-10）、账单详情页与软删除/退款/报销/标签/模板（FR-11~15）、账户与转账 UI（FR-16）、周/年预算 UI、分享长图、图片、周期记账、多账本切换、语音/OCR/智能记账、密码保护、会员体系。

## 2. 数据模型（迁移 005）

| 表 | 动作 |
|---|---|
| t_users / t_sessions | 保留，唯一身份与会话 |
| t_ledgers | 加 `icon VARCHAR(32) NOT NULL DEFAULT '📒'`、`owner_id BIGINT NULL`；历史账本 id=1 改名「我的账本」并绑定 owner |
| t_categories | 加 `parent_id BIGINT NULL`（留二级能力，暂不用）；补种 iCost 分类集（按 name+kind 去重插入，不删除已有分类与引用） |
| t_entries | 加 `refund_of_id BIGINT NULL`、`reimbursable BOOLEAN NOT NULL DEFAULT FALSE`、`exclude_stats BOOLEAN NOT NULL DEFAULT FALSE`、`exclude_budget BOOLEAN NOT NULL DEFAULT FALSE`、`currency CHAR(3) NOT NULL DEFAULT 'CNY'`；历史 `created_by` 为空/0 归属 owner |
| t_budgets | 重构为 `(ledger_id, scope ENUM('week','month','year'), period_key VARCHAR(16), category_id BIGINT NOT NULL DEFAULT 0, amount_cent)`，主键 (ledger_id,scope,period_key,category_id)；category_id=0 表示总预算；旧 (ledger_id,month,amount_cent) 行转为 (scope='month',period_key=month,category_id=0) |
| t_accounts / t_transfers | 保留，P1 再接入 UI |
| t_family_setup / t_family_members / t_family_invites | DROP（执行迁移前须先备份真实库） |

预算 period_key：月 `2026-10`、周 `2026-W40`（周一为起点）、年 `2026`。

## 3. 身份与账本解析

- auth 只做微信身份与会话：`Principal{ID}`，无 Role。
- 登录时若用户没有账本，由 API 中间件调用 `EnsureLedger` 自动创建账本「我的账本」并种入默认分类（FR-01：登录直达明细页）。
- 所有业务查询从 `store.WithLedgerID` 注入的上下文取值，替换现有硬编码 `ledger_id=1`。

## 4. API

- 移除：`/api/v1/family/*` 全部端点、一切 role 字段与 admin 校验、开发令牌模式保留（仅测试用）。
- 保留：entries CRUD + 筛选/分页、categories、accounts、transfers、export（权限检查删除，单人全权）。
- 重写：
  - `GET /api/v1/stats?range=week|month|year|all|custom[&ref=YYYY-MM-DD][&start=YYYY-MM-DD&end=YYYY-MM-DD]`
    响应：`{range, start, end, prev_ref, next_ref, summary:{income_cent,expense_cent,balance_cent,daily_avg_expense_cent}, series:[{key,expense_cent,income_cent}], categories:[{category_id,amount_cent,count}], budget:{total_cent|null, remaining_cent|null, remaining_daily_cent|null}}`
    - week：周一起 7 天；month：自然月；year：自然年；all：全量；custom：起止日（含尾日）。
    - daily_avg_expense：期间含今天时按已过天数（至少 1 天），否则按期间总天数。
    - budget 仅 range=month 时返回（明细页预算卡数据源）。
  - `GET /api/v1/budgets?scope=&period=` → `{scope, period, total_cent, categories:[{category_id,name,icon,amount_cent,spent_cent}]}`（category_id=0 行是总预算；spent 为期间该分类支出）
  - `PUT /api/v1/budgets` `{scope, period, category_id, amount_cent}`（category_id=0 设总预算；amount=0 表示删除该预算行）
  - `GET /api/v1/ledgers/me` → `{id,name,icon,currency}`
- entries 响应新增字段：refund_of_id、reimbursable、exclude_stats、exclude_budget、currency（M1 不提供写入 UI，字段透传即可）。

## 5. 小程序页面（M1）

| 页面 | 说明 |
|---|---|
| pages/login | 微信一键登录 → reLaunch 明细页；保留开发环境地址配置；删除家庭初始化/邀请 UI |
| pages/ledger（账本 tab） | 自定义头（账本名→我的账本页；月份‹›；点月份标题弹月选择器）；预算大卡（文档版：剩余大字 + 总预算 + 剩余日均，超支红字+「超支」标记，无预算显示引导）；按日分组（组头 `MM/DD 星期X · 支出 ¥x` 可折叠）；行 = emoji 图标 + 分类名 + 备注 + HH:mm + 金额（支出红/收入绿）；悬浮黄色「+」 |
| pages/entry（全屏键盘） | 顶部 取消/账本名 + 支出|收入分段；金额大字 + 运算表达式；5 列分类宫格（末位「⚙更多」进分类管理）；chips：报销/优惠/图片/标签/备注（M1 仅备注、时间可用，其余 toast「后续版本支持」）；时间行（默认当前分钟，可改）；保存再记/保存 |
| pages/stats（统计 tab） | 五档 segmented + ‹ › 蓝色切期；4 指标卡；柱状图（最高柱红色，其余品牌蓝）；分类排行（图标/名称/占比/金额）；右上「预算」入口 |
| pages/budget | 月总预算卡（已用/剩余/进度条，超支红）+ 分类预算列表（每行可编辑金额）；只放月粒度 |
| pages/settings | 改为「我的账本」：账本名/图标展示、分类管理入口、导出 CSV、退出登录 |
| pages/categories | 现有能力保留（增删改/emoji/排序/停用） |

计算器规则（11.4 增补）：`+ − × ÷` 与小数点；连续输入运算符后者覆盖前者；除零/非法表达式不崩溃、保持上一结果；结果按分四舍五入（与整数分模型一致）；保存时金额必须 >0。

视觉 token（11.3 定稿）：支出红 #E5484D、收入绿 #17A34A、主色蓝 #3B82F6、底色 #F4F5F7、卡片 #FFFFFF、主文字 #1A1D21、次级 #8B9199、三级 #B6BBC2、分割线 #ECEEF1；悬浮 + 用黄色 #F5A524（文档 4.1 与 iCost 一致，原型为蓝色，此处以文档为准）。

## 6. 验收标准（M1 出口）

- 登录后无中间流程直达明细页；代码与接口无 admin/member/family 分支。
- 键盘最短 2 击记一笔（打开键盘 → 选默认分类 → 输金额 → 保存）；`12+8` 出 `20.00`；「保存再记」后键盘不关、金额清零、分类保留。
- 明细页：组头折叠生效；预算卡剩余日均 =（总预算 − 已支出）÷ 本月剩余天数；超支红字。
- 迁移 005 后：23 笔历史账单全部可见且金额分毫不差；旧月预算转成 month 总预算；家庭三表不存在。
- 统计：周/月/年/全部/范围切换正确，‹ › 切期正确，月视图 budget 字段正确。
- 现有自动化（go test -race/vet/build、隔离 MySQL 集成测试、Node 测试、页面检查）全部通过。

## 7. 风险与边界

- 005 迁移含 DROP 家庭三表：执行前必须备份真实库（云端已有自动备份），迁移仅在显式 `-migrate` 时执行。
- 新分类种入只增不改，历史分类与账单引用不动。
- 微信开发者工具模拟器交互、真机、真实微信登录无法在本环境完成，验证记录须如实标注。
- 本环境未安装 Superpowers 技能（brainstorming/writing-plans/TDD/审查），按 AGENTS.md 精神以等价的「设计→计划→失败测试先行→审查→验证」执行并如实标注缺口。
