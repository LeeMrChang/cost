# iCost 重定位 M1 验证记录

> 公开文档脱敏说明（2026-11-18）：个人路径、部署标识及真实账本数据已省略或改为占位示例。历史验证结论保留，不代表使用占位示例重新执行。示例路径使用前须替换；部署脚本的内置默认路径以源码为准，本次未调整脚本。

日期：2026-10-26
实施计划：[2026-10-26-icost-realignment](../superpowers/plans/2026-10-26-icost-realignment.md)
接口约定：[2026-10-26-icost-realignment-api](../api/2026-10-26-icost-realignment-api.md)

## 1. 验证环境

| 项 | 值 |
|---|---|
| 主机 | macOS 27.0 (arm64) |
| Go | 1.27.2（`go vet` / `go build` / `go test` / `go test -race`） |
| MySQL | 8.4.11，unix socket `/tmp/mysql.sock`，隔离集成测试经 `COST_TEST_MYSQL_ADMIN_DSN='root@unix(/tmp/mysql.sock)/'` |
| Node | v22.23.2（`node --test 'tests/*.test.js'`） |
| Python | 3（`unittest discover -s scripts/tests`） |
| 微信开发者工具 | 本环境不可用，UI 仅做代码级核对，未做模拟器/真机视觉验证 |

## 2. 全量验证结果（计划第 12 步）

| 套件 | 命令 | 结果 |
|---|---|---|
| Go 静态检查 | `go vet ./...` | 通过 |
| Go 构建 | `go build ./...` | 通过 |
| Go 单元 + MySQL 集成 | `go test -count=1 ./...` | 通过（api/auth/domain/store 全绿） |
| Go 竞态 | `go test -race -count=1 ./...` | 通过 |
| 小程序前端 | `node --test 'tests/*.test.js'` | 46/46 通过 |
| 部署脚本 | `python3 -m unittest discover -s scripts/tests` | 28/28 通过 |
| Shell 语法 | `bash -n deploy/backup/{backup,common,verify}.sh` | 通过 |

## 3. 验证中发现并修复的问题

1. **005 分类补种 SQL 聚合空集缺陷（严重，影响真实老库迁移）**。原语句
   `INSERT ... SELECT COALESCE(MAX(id),0)+1 ... FROM t_categories WHERE NOT EXISTS(...)`
   在分类名已存在时 NOT EXISTS 为假、外层无行，但无 GROUP BY 的聚合对空输入仍输出一行 NULL，
   `COALESCE(NULL,0)+1` 得到 1，导致误插 `id=1`：
   - 真实老库（必带旧默认分类集）补种到第一个重名分类即报
     `Duplicate entry '1' for key 't_categories.PRIMARY'`，迁移 005 必然中断；
   - 只有一个重名分类的测试夹具不报错，但会静默插入一行 id=1 的重复分类（如 `午餐` 出现在 id 1 与 2）。
   修复：改从派生表取号
   `FROM (SELECT COALESCE(MAX(id),0)+1 AS id FROM t_categories) v`，
   NOT EXISTS 为假时零行不插。回归覆盖：
   `TestMySQLPrefixPreservesLegacy`、`TestMySQLPrefixResumeAfterRename`、
   `TestMySQLFeaturesMigrationResume`、`TestMySQLMergeLedger` 及全量套件。
2. **合并工具与 005 后模型脱节**。`MergeLedger` 身份校验引用已 DROP 的 `t_family_members`（1146），
   预算按旧 `month` 列校验与插入。更新为：
   - 身份校验：源 `created_by` 经 openid 映射到目标 `t_users`，无家庭成员条件；
   - 预算：按 `(ledger_id,scope,period_key,category_id)` 校验冲突与插入；
   - 新增护栏：源含扩展账单设置（`refund_of_id` 非空、`reimbursable`/`exclude_stats`/`exclude_budget`
     置位、非 CNY 币种）时停止合并，提示改用整库恢复——与账户/转账、扩展分类设置护栏一致。
   测试：fixtures 去掉家庭表语句、源分类与新默认集对齐，并新增
   `divergent legacy categories must stop` 用例（旧分类集与全新库 id 语义冲突时停止而非错挂）。
3. **003 迁移文件事后修订**。`003_table_comments.sql` 删除 3 个家庭表 ALTER 块：这些表由 005 DROP，
   语句在新装与老库快照路径上不可达且会 1146。003 未在真实库执行过，随本次发布一并生效；
   对已执行 003 的库无影响（版本标记跳过）。
4. **icostOwner 兼容未改名快照**。家庭管理员查询对 `t_family_members` / `family_members` 双前缀回退，
   避免在"002 后未改名"的老库快照上 1146。
5. **前缀断言更新**。`assertPrefixed` 期望值 57 列 → 59 列（005 后共 9 张带注释前缀表、59 列，
   经隔离库 information_schema 实测确认）。

## 4. M1 屏对照（代码级核对，视觉未验证）

按计划第 2 节，本阶段核对 S03/S05/S06/S09/S10。

| iCost 截图 | 要求要点 | 实现位置 | 状态 |
|---|---|---|---|
| S03 明细页 | 标题=账本名（自定义导航栏：左上账本名 ▾ 进设置，右上账本图标头像 + ⋯ 菜单）、灰色搜索条「搜索备注 / 分类名」、月份切换（`2026年10月` 格式，**点标题或右侧「收支日历」双入口**进日历）、月度汇总条（支出红/收入绿/结余）、白卡「剩余月预算」+超支红标+**红字大金额（超支时）**+进度条（<80% 蓝 / ≥80% 黄 / ≥100% 红）+总预算/剩余日均（本月剩 N 天）、按日分组（10/26 星期一 + 支出: ¥x + 折叠）、账单行（圆图标、**分类名 + 空格 + 备注**、时间单独一行、**支出红 #E5484D**/收入绿 #17A34A）、**蓝色悬浮 + #3B82F6**、页脚「— 本月共 N 笔 —」 | `miniprogram/pages/ledger/`（2026-10-27 按原型 v4/文档 v2.1 对齐）、`miniprogram/pages/calendar/`（S20 月历）、`miniprogram/pages/search/`（搜索页：防抖、10 筛选 chips、汇总、点单进账单详情） | 已实现（代码级，真实数据模拟输出与截图同构） |
| S05 记账键盘（支出） | 全屏弹层、支出/收入切换、5 列分类宫格、报销/优惠/图片/标签 chips、计算器键盘、保存再记 | `miniprogram/pages/entry/` | 已实现（代码级）；chips 中报销/优惠/图片/标签为占位（`later`），字段未落库 |
| S06 记账键盘（收入） | 收入分类宫格、绿色主题 | `miniprogram/pages/entry/`（kind 切换 + 绿色 token） | 已实现（代码级） |
| S09 标签选择 sheet | 搜索/创建标签、标签管理入口 | 未实现（M1 不含标签管理） | 待 P1 |
| S10 统计页 | 周/月/年/全部/范围五档、指标卡、支出统计图、分类排行 | `miniprogram/pages/stats/`（五档 + 4 指标 + 柱状图 + 排行 + 月预算入口）；`GET /api/v1/stats` | 骨架已实现（代码级）；iCost 的 8 指标与报销统计 4 指标未做（P1） |
| S15~S18 预算 | 周/月/年三粒度 | 模型三粒度已建全；UI 首版仅月（`miniprogram/pages/budget/`） | 已实现（按 D 预算粒度决策） |

## 5. 本地开发库迁移与联调（2026-10-26 16:58 补做）

初次验证未覆盖「本地开发库升 005 后起服务」，导致前端联调时后端全部接口报
`Error 1054: Unknown column 'owner_id'`、前端明细页显示 internal_error。本次补做：

1. **备份**：`mysqldump` 导出 `cost` 库至 `backups/cost-pre005-20261026-165655.sql.gz`
   （12 张表；真实账单、用户、预算和分类数量省略；仓库 `backups/` 已 gitignore）。
2. **迁移**：`./scripts/dev.sh -migrate` 以应用账号 `cost_app` 执行时报
   `DROP command denied`——本地未建服务端同款 `cost_migrator` 账号。改用
   `COST_STORE=mysql COST_MYSQL_DSN='root@unix(/tmp/mysql.sock)/cost?...' go run ./cmd/api -migrate`
   执行成功。**本地执行迁移的常规命令应记入 README/运维文档**（当前 dev.sh 只能以
   具备 DDL 权限的账号跑）。
3. **迁移结果**：005 落库；家庭三表删除；`t_ledgers` 增加 `icon`/`owner_id`（uk 唯一）；
   `t_budgets` 重建为 `(ledger_id, scope, period_key, category_id, amount_cent)`，
   旧月预算金额不变并转为对应月份的 month 总预算；分类补种为 13 支出 + 12 收入；账单无损（真实金额和数量省略）。
4. **接口实测**（development 鉴权实例，端口 8081，`X-Dev-Token`）：
   `ledgers/me`、`categories`、`entries`、`stats`（month/week/自定义 ref）、
   `budgets` GET/PUT 全部 200；周预算写入→读取→删除往返一致；`range=bad` 返回 400
   `invalid_range`、`period=2026-13` 返回 400 `invalid_budget`。
5. **微信模式路径**：`LedgerForUser` 的 `SELECT id FROM t_ledgers WHERE owner_id=?`
   已在新库直接验证返回账本 1；用户 8080 端口 wechat 模式服务无需重启，重试即可恢复。
6. **搜索联调（2026-10-26 17:30 补做）**：关键词 q 由「仅备注」扩展为「备注或分类名」
   （MySQL `LOCATE(BINARY ?,BINARY note)` OR 分类名子查询；内存店同规则）。开发鉴权实例对真实本地库
   实测覆盖全时间范围、分类关键词、备注关键词、收入类型组合及分页一致性；具体查询命中数量与个人收支关键词省略。
7. **头像与收支日历联调（2026-10-26 18:00 补做）**：明细页改自定义导航栏（右上账本图标头像 +
   ⋯ 菜单），「收支日历」替换原设置入口；新增 `pages/calendar`（S20：周日开头月历、红出绿入、
   今天描边、点日展开当日明细、月份切换）。真实本地库实测：`ledgers/me` 返回 `icon:📒`；
   `stats?range=month&ref=2026-10-01` series 含当月 31 天逐日收支；
   按日查询返回的明细与 S03 截图分组数据一致（具体日期、金额及笔数省略）。
8. **原型 v4 / 文档 v2.1 对齐（2026-10-27 上午补做）**：以 Master 更新的 `cost_可交互原型_v4.html`
   与 `cost-原型与需求说明-iCost对标-v2.1.md`（2026-10-26 17:19/17:22）为准回改三页——
   - 明细页：账本名左置（▾ 进设置）、月份标题与「收支日历」双入口（移除 calendar-popup 月历弹层）、
     新增月度汇总条（支出红/收入绿/结余）、预算卡进度条 + 「本月剩 N 天」（N = 当月天数 − 今日 + 1，
     与后端 `remaining_daily_cent` 口径一致；无预算显示「未设置」引导）、支出金额改红、悬浮 + 改蓝、
     备注与分类名以空格分隔、条目副行只留时间、页脚「本月共 N 笔」。
   - 搜索页：占位「搜索备注 / 分类名」、10 个筛选 chips（仅支出/仅收入可用，报销/退款/优惠/
     不计收支/不计预算/包含图片/不包含图片置灰待 P1/P2）、汇总改「找到 N 条结果 · 支出/收入」、
     结果副行显示日期+时间、金额补 ¥ 前缀、支出红。
   - 日历页：导航标题=月份、顶部「本月支出/收入」行、格子去 ¥ 前缀（收入 + 前缀、空档「—」、
     同日支出在上收入在下）、点日标题右侧显示当日支出/收入合计、记账键盘主按钮统一「保存」。
   - 迁移：本地开发库应用 `006_entry_soft_delete`（先备份 `backups/cost-pre006-20261027-093541.sql.gz`，
     9 张表，gzip 校验通过）。
   - 验证：`node --test 'tests/*.test.js'` 69/69 通过、`tests/validate.js` 通过；
     后端 `go test -count=1 ./...` 全绿；development 鉴权实例（8080，`X-Dev-Token`）对真实本地库实测：
     `ledgers/me`、月度统计、分类搜索、按日及按月查询全部返回 200；汇总、预算、剩余日均与明细核对一致（个人收支、预算和命中数量省略）。

## 6. 未覆盖与遗留

- **模拟器/真机视觉验证未做**：本环境无微信开发者工具，上述屏对照为代码级；wxml/wxss 渲染效果
  需在微信开发者工具中按原型 v4 逐屏人工核对。
- **真实库（服务器）迁移未执行**：005 对真实库仅在显式执行迁移后生效，
  执行前必须备份（计划第 3 节约定）。第 3 节第 1 条缺陷正是在老库形态数据上发现的，
  真实库迁移前务必以备份快照先行演练一次。
- **合并工具护栏扩展**：源含扩展账单设置时停止合并；此类数据须走整库备份恢复路径。
- **微信登录真实链路**：本地 wechat 模式服务已在运行，但真实 `wx.login → jscode2session`
  链路的实机验证仍需在微信开发者工具/真机完成。
