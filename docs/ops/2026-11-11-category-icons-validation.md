# 分类图标接入与迁移验证

日期：2026-11-11。环境：macOS arm64、Node v26.13.0、Go 1.27.3、本机 MySQL socket。

## 内容

依据用户指定的 `cost分类图标候选对照.html`：采用绿框推荐值覆盖 10 个分类默认图标；14 个“保持不动”分类不变。其余候选全部加入分类编辑器，与原有选项去重后共 48 个。图标继续作为 Emoji 字符串保存，无外部资源和新增权限。

| 分类 | 新图标 |
|---|---|
| 福利（收入） | 🎫 |
| 奖金（收入） | 🏆 |
| 副业（收入） | 🌱 |
| 加班（收入） | ⏰ |
| 早餐（支出） | 🥐 |
| 午餐（支出） | 🍛 |
| 晚餐（支出） | 🍲 |
| 夜宵（支出） | 🍗 |
| 消费（支出） | 🛒 |
| 其他（支出） | 📦 |

设计与范围见[设计说明](../superpowers/specs/2026-11-11-category-icons-design.md)，步骤见[实施计划](../superpowers/plans/2026-11-11-category-icons.md)。

## 实际验证

实现前运行的失败证据：

- 前端 `node --test miniprogram/tests/p0-pages.test.js`：29 通过、1 失败，缺少候选 🎫。
- Go 默认分类测试：10 个分类图标与新预期不符，失败。
- MySQL 集成测试首次被沙箱阻止连接 socket，该环境失败不计作红灯；获得本机 MySQL 隔离测试授权后，修正测试预期中未覆盖原有默认分类的问题并重跑，在实际旧图标未更新的断言上失败。

实现后实际结果：

| 命令 | 结果 |
|---|---|
| `node --test 'miniprogram/tests/*.test.js'` | 122 通过，0 失败，退出 0 |
| `node miniprogram/tests/validate.js` | 语法、JSON、页面检查通过，退出 0 |
| `go test ./internal/api -run '^TestMySQLCategoryIconsMigration$' -count=1` | 隔离 MySQL 迁移测试通过，退出 0 |
| `go test -race ./...` | 全部包通过，包含隔离 MySQL 集成，退出 0 |
| `go vet ./...` | 退出 0 |
| `go build -o /private/tmp/cost-category-icons-api ./cmd/api` | 退出 0，产物在仓库外 |
| `git diff --check` | 通过 |

Go 命令在 backend/ 执行，使用 `GOCACHE=/private/tmp/cost-export-removal-go-cache`；MySQL 测试使用 `COST_TEST_MYSQL_ADMIN_DSN='root@unix(/tmp/mysql.sock)/'`，由测试创建和清理随机 cost_test_* 数据库，未修改真实账本库。未配置外部 Redis 测试连接，真实 Redis 服务集成不计作已验证。

迁移验证覆盖分类数量、ID、名称、排序、停用状态、父分类等字段保持不变，历史账单金额/备注/分类关联保留，同名收入“其他”不被支出规则覆盖，无关分类与其他账本不变；重复迁移保留用户后续改图标。

## 实际账本升级步骤（未执行）

1. 停止旧后端，备份真实数据库并确认备份成功。009 只更新账本 1 中这 10 类**仍为旧默认值**的图标：用户改过图标的行跳过，用户改过名的行仍按 `id` 命中并升级，用户自建的同名分类与其他账本不受影响。没有分类增删或表结构变化。
2. 使用具备迁移权限的配置在仓库根执行 `./scripts/dev.sh -migrate`，或使用部署环境既有显式迁移流程，将数据库升级至 009。不要仅把 SQL 文件手工执行而遗漏版本标记。
3. 启动新版后端，重新编译/发布小程序，刷新分类页和记账页。服务启动会创建新的缓存命名空间，避免读取旧分类缓存。

当前只修改代码与隔离测试库，未升级本地业务库或云端库，未提交、部署或发布。微信开发者工具、真实微信和真机显示未验证，Emoji 视觉随系统变化。

自查默认映射、迁移事务及版本标记、查询范围、候选去重和保存链路，未发现额外问题。未执行独立代码审查。当前环境缺少项目要求的 Superpowers 技能，不声称完整执行该技能流程；设计和计划在实现前建立，未伪造单独审批或独立审查证据。

## 追加修订（同日）：009 改为保护式更新

初版 009 按 `ledger_id + kind + name` 直接改写，语义是“覆盖匹配分类”。经审阅确认这会覆盖用户自定义的图标，且改名后的分类会静默漏改。本次改为保护式：

```sql
UPDATE t_categories SET icon='🍗'
WHERE ledger_id=1 AND kind='expense' AND (id=3 OR name='夜宵')
  AND (BINARY icon=BINARY '🍢');
```

- **双路身份匹配**：`id` 覆盖用户改名（分类管理支持改名，`p0_mysql.go:397` 同时写 `name`）；`name` 覆盖历史库经 005 以 `MAX(id)+1` 补种的分类（`icost.go` 第 3 步），这些行 id 不在 1~24。两路都靠旧值条件兜底，误伤面被旧图标限制住。
- **旧值条件**：只改仍是旧默认值的行，用户手动改过的图标一律跳过。
- **变体选择符**：消费的旧值 `🛍️` 含 U+FE0F，按 `BINARY` 精确比较会漏掉历史丢失变体符的写法，故并列 `'🛍️'` 与 `'🛍'` 两种。
- **保留**：`ledger_id=1` 范围、执行器版本标记与事务（`category_icons.go:17-23`）、迁移锁（`mysql.go:51`）。

测试同步改写为按 `id` 断言，覆盖六种情形：未改过图标的升级、改过图标的保留、改过名仍按 id 升级、用户自建同名分类不动、其他账本不动、历史账单关联不变；并保留“记录版本后重跑不覆盖后续修改”的断言。

验证命令与结果（backend/ 执行，`COST_TEST_MYSQL_ADMIN_DSN='root@unix(/tmp/mysql.sock)/'`，隔离 `cost_test_*` 库，未触碰真实账本库）：

| 命令 | 结果 |
|---|---|
| `go vet ./internal/api ./internal/store` | 退出 0 |
| `go test -run '^TestMySQLCategoryIconsMigration$' ./internal/api` | ok，0.77s |
| `go test ./...` | 全部包通过，退出 0 |

仍未提交、未部署、未升级真实账本；真机显示未验证。
