# MySQL 表前缀、注释及本地数据合并云端记录

日期：2026-10-22。用户已确认统一 t_、补齐注释，并授权将 Mac 数据迁至云端；本次未 commit、push。

## 最终结构

两端 cost 库均为 t_ledgers、t_categories、t_entries、t_budgets、t_users、t_sessions、t_family_setup、t_family_members、t_family_invites、t_schema_migrations，共 10 张表、39 个字段。表和字段均有中文 COMMENT。Navicat 刷新“表”节点后查看；旧查询请将 entries 等更新为 t_entries。

金额字段 amount_cent 单位为分；occurred_at 和 expires_at 按 UTC 解释；预算月份 YYYY-MM 按账本时区统计；token_hash 为摘要，不能用其代替原始令牌登录。

## 实现及升级机制

- 保留 001/002 SQL 历史；迁移命令在数据库锁内检查前缀状态，执行一次多表 RENAME，MySQL 更新外键引用。
- 003_table_comments.sql 用显式列定义补充注释，不丢失自增、默认值、索引、CHECK、外键和区分大小写排序。
- 迁移历史最终放在 t_schema_migrations，当前 3 个版本。已完成改名但注释/版本未完成时可重新执行。
- 新旧表并存或不完整新前缀结构拒绝，不合并或覆盖冲突表。普通启动不执行 DDL；迁移超时为 5 分钟。
- 账目和认证 SQL 使用固定 t_。不支持旧版后端继续读取新结构，不提供无前缀视图或任意动态前缀。

## 两端执行结果

| 检查 | Mac | 腾讯云 |
|---|---|---|
| 升级前账目 | 21 | 2 |
| 升级后/合并后账目 | 21 | 23 |
| 月预算 | 1 条 | 导入 1 条 |
| 带注释表 | 10 | 10 |
| 带注释字段 | 39 | 39 |
| 迁移版本 | 3 | 3 |
| 后端 | 新版本地进程已启动 | cost-api:prefix-20261022，healthy |

升级前暂停两端旧 API，完整备份恢复至独立 cost_verify_prefix_backup 库。Mac 恢复库与真实库逐表 JSON 数据指纹一致；改名后 9 张业务/身份表行数及全部字段指纹保持，迁移表增加 003 记录。云端所有原有记录逐字段与备份恢复库对比无差异；原有账号、成员、管理员、会话均保持。

Mac 完整备份与云端升级前/升级后备份保存在私有任务工作目录；云端备份另保存在 /home/ubuntu/cost/backups/。文件权限 600，不在 Git 或公开制品中。真实数据库未删表或覆盖恢复。临时恢复库、导入 staging 库及临时运维二进制已清理。

## Mac 数据合并规则

运维入口 backend/cmd/merge-ledger，方法 store.MySQL.MergeLedger。将 Mac 备份恢复为同实例的独立 staging 库，再显式升级该库；环境 COST_MYSQL_DSN 指向目标 cost，COST_IMPORT_SOURCE_SCHEMA 指向 staging。执行账号只在运维期间使用，必须能读 source 且对 target 有必要写权限；不把 staging 权限授予常驻 API。

- 保留云端账本与身份，不复制 Mac 的会话、邀请、权限及迁移记录。
- 分类按 ID 核对名称/类型/图标一致后复用；新分类可新增，冲突停止。
- 创建者按相同 openid 映射云端现有家庭成员，未知用户停止；历史空创建者仍保留为空。
- 按 ledger_id + client_request_id 去重；同标识但金额/分类/类型/时间/备注/创建者不同则停止。
- 月预算相同月份金额一致跳过，不同金额停止，不静默覆盖。
- 全部预检和插入在同一事务；失败回滚。目标账目使用新自增 ID，原本地 ID 不用于覆盖云端主键。保留原发生、创建及更新时间。

真实执行首次新增 21 笔，第二次新增 0 笔；逐笔业务字段与来源对比匹配 21 笔，原云端 2 笔全部保持。合并后对 23 笔重新做完整备份。

## 日后升级与回退

Mac：停止旧 Cost API，先备份，再在仓库根运行 ./scripts/dev.sh -migrate，然后 ./scripts/dev.sh 启动新版。不要把旧编译二进制再连接新结构。

云端：在 deploy/ 执行 docker compose -f compose.cloud.yaml stop api；备份并确认可恢复；构建新镜像；docker compose -f compose.cloud.yaml --profile tools run --rm -T migrate；成功后 docker compose -f compose.cloud.yaml up -d api。迁移使用既有迁移账号，业务账号 DML 权限不变。

回退不能仅切回旧镜像：旧 SQL 使用无前缀名，须先停止写入并设计反向改名及迁移记录同步。已有新写入时不能以旧备份覆盖。旧镜像 cost-api:local 与改动前源代码备份暂时保留，供人工恢复方案使用。

## 验证证据与边界

- 前缀测试真实红灯：新库无 t_ 注释表；实现后新库、旧 001/002 库保留、重复迁移、改名中断恢复、新旧冲突拒绝全部通过。
- 合并测试：不同用户 ID 的身份映射、保留目标已有账单、重复导入 0 新增、账单/预算/分类冲突停止、未知身份拒绝、非法或相同来源库拒绝均通过。
- COST_TEST_MYSQL_ADMIN_DSN 指向本地 root socket，仅自动创建/清理随机隔离测试库；完整 go test -race ./...、go vet ./...、本地与 linux/amd64 编译通过。
- 独立只读审查：无 Critical/Important 问题。
- 两端 /health 为 wechat/ok；云端未认证账目请求为 401，MySQL 无公网端口，API 仍绑定服务器回环地址。
- 升级后生产没有额外创建测试账单，账目/家庭增改查删通过隔离数据库 API 工作流验证。新一轮真实微信登录、手机与多人验证仍需开发者工具/真机验证，不把数据库核验或健康接口称为这些项目已通过。
- 本次未改小程序接口地址、未配置域名/HTTPS、未配置自动部署和定时备份。
