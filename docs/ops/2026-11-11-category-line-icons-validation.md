# 分类线性图标与统一入口验证

> 当前本地状态（21:35 更新）：经用户授权，已备份本地账本、升级至010并重启8081服务，健康检查200；云端未操作。此前“未迁移真实数据库”描述为各次验证当时的状态，最新执行见文末。

日期：2026-11-11。依据 Notion《分类图标规范》最新 §9，用户已明确授权直接动代码。环境：macOS arm64、Node v26.13.0、Go 1.27.3、本机隔离 MySQL。

## 改动与边界

- 48 个线性图标：24 默认映射＋24 扩展。午餐为方形便当盒，早餐可颂、餐饮交叉刀叉、晚餐深碗；钱袋、退税、宠物、医疗、学习、运动、保险等轮廓齐备。
- 源素材在 `design/category-icons/`；Lucide 1.8.0 ISC 许可随源及小程序资源保存，部分图形由项目绘制。源节点、SVG、展开轮廓、清单和生成脚本一并保留。临时构建环境依赖见该目录 README，不增加 Go/小程序运行依赖。
- 新增共享 category-icon 组件，在分类、记账、账单列表/详情、搜索、日历、统计和预算展示使用。iconfont 本地读取为 base64 后由 loadFontFace 加载；WOFF2 失败尝试 TTF；等待或失败时继续显示 Emoji，未知 key 与空 Emoji 有安全占位。没有为了字体加载阻塞账单内容。
- 编辑器支持内置和自建分类选择线稿或 Emoji；Emoji 更新显式清 key；图标 PATCH 缺省与 null/空串分别处理。
- 只读 system_icon_key 区分默认角色与自建分类。顶部 banner 仅统计内置角色中的 Emoji，可按用户/账本/版本关闭；二次确认后才发批量请求，成功失效分类缓存。事务内只统一内置角色，保留 Emoji 回退，自建和其他账本不动。
- 010 显式迁移新增两个可空字段并补默认角色和图标 key，保留自定义混排、分类 ID、历史账单和状态。先前移除 CSV 与底部导航图标的未提交改动保留。

设计见[设计说明](../superpowers/specs/2026-11-11-category-line-icons-design.md)，接口与实际升级步骤见[API 与迁移](../api/2026-11-11-category-line-icons-api.md)，计划见[实施计划](../superpowers/plans/2026-11-11-category-line-icons.md)。

## 红绿证据与排障

实现前：设置页测试输出新分类线稿选择处理器缺失的断言失败；`go test ./internal/api -run '^TestCategoryLineIconsMemory$' -count=1` 退出 1，默认分类缺少 icon_key/system_icon_key。实现后两者通过。

旧库兼容检查中新增 `TestMySQLLineIconLegacyIdentity`，先复现历史 ID 2 的午餐被误认成交通的失败（退出1）；随后增加对“名称明确属于另一默认分类”的 ID 冲突保护，再通过全量隔离测试。此处没有改动既定 009 SQL，保护应用于新的 010 角色识别。

临时 Python 依赖安装首次被沙箱阻断网络，获授权后装入 /private/tmp/cost-icons-venv；未改变项目运行环境。期间有根目录误跑 Go 命令、默认 Python 未安装 fontTools、字体生成 API 调整等环境/工具问题，均不算业务红灯证据。

## 最终验证

| 检查 | 实际结果 |
|---|---|
| 根目录 `node --test 'miniprogram/tests/*.test.js'` | 131 项通过，0 失败，退出 0 |
| `node miniprogram/tests/validate.js` | JS、JSON、页面完整性通过，退出 0 |
| backend 中 `go test -race ./...` | 所有包通过，包含隔离 MySQL 集成 |
| backend 中 `go vet ./...` | 退出 0 |
| backend 中 `go build -o /private/tmp/cost-category-line-icons-api ./cmd/api` | 退出 0，产物在仓库外 |
| 素材校验 | 48 个唯一 key 与码点、TTF/WOFF2 各 48 个非空字形、有效边界，前后端清单一致 |
| 生成与预览 | 从生成字体绘制全库状态/20px预览并实际查看；重复生成产物哈希一致；HTML 包含 20/32/44px 字号对照 |
| WXML 检查 | 本轮八个页面标签配对通过；不是微信编译器验收 |
| 差异检查 | `git diff --check` 通过；文档本地链接通过；未新增凭证或仓库内后端构建产物 |

Go 检查使用 `GOCACHE=/private/tmp/cost-export-removal-go-cache`。MySQL 使用 `COST_TEST_MYSQL_ADMIN_DSN='root@unix(/tmp/mysql.sock)/'`，测试仅创建/清理随机 cost_test_* 数据库，没有读取或更改真实账本。未配置真实 Redis 集成测试连接，相应外部 Redis 验证不计作完成。

测试覆盖双轨编辑（缺省/null/空串/旧客户端）、未知 key、系统身份伪造防护、字体失败与 API 不支持时回退、共享字体只加载一次、banner 计数/忽略版本/二次确认/防重复请求，以及 SQL 范围、事务失败全部回滚、DDL 中断重试、默认与自定义旧数据保留。

009 已复核为 Notion §9 的 id OR name＋kind＋账本1＋BINARY旧值条件；补验福利 ⭐、副业 ⭐ 保留、消费丢失 FE0F 的旧值、历史同名种子更新、其他账本、自建同名、旧账单关联、版本重跑及手工 SQL 重跑保护。不会通过删除旧值条件来满足原覆盖式期望。

## 实测资源

| 资源 | 大小 |
|---|---|
| 48 字形 WOFF2 | 10068 bytes |
| WOFF2 的 base64 字符串 | 13424 bytes |
| 兼容 TTF | 41652 bytes |

小程序同时携带两种格式供回退，清单与许可另计。这是本地产物大小，不是微信最终上传压缩包体实测；不再将旧的20KB估计套用于整个功能。

预览：`design/category-icons/preview.html` 为可直接打开的完整字号/状态对照；`/private/tmp/cost-category-icons-preview.png` 为从字体产物绘制的全库静态图。它们均不是真机截图。

## 交付与未验证项

尚未执行微信开发者工具编译、真实微信或 iOS/Android 真机显示；本地字体 Data URL 的实际平台兼容性、冷启动闪烁、缩放和滚动效果仍需检查，失败时将回落 Emoji。不能将自动化 mock 的成功等同于平台加载成功。

未执行独立代码审查。已自查角色边界、SQL范围/事务、缓存失效、跨页键传递、字体资源、许可和差异；当前未找到 using-superpowers 等项目所需技能，不声称完整执行了技能流程。设计与计划在本轮实现前建立，没有伪造独立审查或先前审批。

未提交、部署、发布小程序，未升级真实数据库。上线前须备份，再按显式迁移流程升级至010，重启新版后端并重新编译/发布小程序。**禁止在真实库删除009版本记录强制重跑**；原版本已覆盖过的用户选择只能从备份或手动设置恢复。本次只在隔离测试库重置迁移标记来制造旧状态。


## 修订：微信启动时 JSON 模块加载失败

用户开发者工具截图提供了实际平台证据：基础库 3.17.6 将 `require('../assets/category-icons/catalog.json')` 解析到 `catalog.json.js`，模块未定义，导致 app.js 启动失败。先前的 Node 检查支持 JSON require，未覆盖此差异；不能将先前自动化通过解释为微信编译通过。

新增 `miniprogram/tests/wechat-modules.test.js`，用只解析 JS 的模块加载器执行真实 app.js 启动依赖图；修改前退出 1，复现相同 catalog.json.js 未定义错误。随后生成器改为输出 CommonJS catalog.js、运行时引用不带扩展名的 catalog，移除过时的小程序 JSON 清单，后端 JSON 清单保持原格式。重新生成后，该回归测试通过；同时核对前后端清单内容一致。

本次 `node --test 'miniprogram/tests/*.test.js'` 129 项全部通过，退出 0；validate.js 与 diff 检查通过。只有运行时清单格式、引用、生成器、回归测试和相关说明变化，没有后端行为变更，因此未重复后端数据库测试。当前环境仍缺少项目要求的 Superpowers 技能，不声称完整执行该技能流程；未做独立代码审查。

尚未在微信开发者工具重新编译或在真机复验，需用户点击“编译”确认该启动错误消失。字体实际加载与真机显示仍维持待验证状态。未提交或部署。


## 修订：空 key 警告与旧后端拒绝保存

用户开发者工具截图显示 iconKey:String 收到 null 的重复警告，以及 POST /api/v1/categories 返回400、提示“请求格式有误”。只读检查发现本机8081监听服务的二进制不包含 icon_key、system_icon_key 或 /api/v1/categories/system-icons；健康接口正常且 auth_mode=wechat，说明当前运行的仍是旧后端。重新编译微信代码不会重启 Go 服务。

先新增两个失败断言并运行（退出1）：旧数据的 null key 不能直接传给 String 组件属性；普通 Emoji 保存请求不应包含旧后端未知的 icon_key 字段。随后各分类组件绑定统一通过 `|| ''` 兜底；保存仅在选择线稿时发送 icon_key，Emoji 请求只发送 icon，新后端同样会按已有协议清空旧 key。

本次前端全量131项通过、语法/页面检查通过、diff检查通过，退出0。新增覆盖8个页面的组件key绑定与普通Emoji保存兼容；没有后端代码变化，未重复后端测试。项目要求的 Superpowers 技能仍缺失，未完成独立代码审查。

未操作真实账本、未执行迁移、未停止或重启8081服务，未在开发者工具复验。重新编译前端后可验证Emoji保存与警告消失；线性图标保存及统一接口仍需备份后升级010并启动新版后端，不能声称本次前端兼容修复已经使旧后端具备新功能。


## 本地账本实际升级（2026-11-11 21:35）

用户重启新版后端后，日志出现 MySQL 1054 `Unknown column 'icon_key' in 'field list'`，分类接口500。原因是正常启动不自动执行显式迁移，目标库仍缺010字段。

获得本轮本地备份、迁移与服务重启授权后，使用仓库外临时升级助手执行：

1. 校验连接目标为本机MySQL且已有008；通过进程工作目录确认8081监听者为本仓库后端，发送SIGTERM并等待停止。
2. 对旧数据取得计数与摘要；使用应用连接做完整single-transaction gzip备份，成功解压读取后才继续迁移。备份为 `backups/cost-pre010-20261111-213451.sql.gz`，4314 bytes，权限0600，已确认由Git忽略。没有打印凭证、会话原文或备份内容。
3. 因本机应用账号无DDL权限，仅迁移子进程使用本机root socket连接；保持同一库名，执行已构建的新版API `-migrate`。没有修改.env，也没有操作云端。
4. 迁移后确认009/010版本记录及 icon_key、system_icon_key 两列存在。8张业务表（不含迁移表）计数与内容摘要一致；分类仅对保留字段做比对，允许预期的图标与新增角色变化。账单及其关联、金额和备注等内容保留。
5. 使用 `./scripts/dev.sh` 启动新版8081后端，启动日志为 development/mysql/wechat；HTTP健康检查200，status=ok。

本轮为运行环境升级，没有新增业务代码或测试；以上是实际备份、迁移、数据保留和服务恢复证据，不以历史测试代替。用户需在微信开发者工具刷新/重新编译，分类页、字体实际加载和真机仍需复验；没有宣称健康接口能证明全部业务交互已通过。


## 云端自动部署迁移与打包修订

核对 `.github/workflows/ci-cd.yml`、`scripts/deploy-cloud.sh` 和 `deploy/compose.cloud.yaml`：main上的非PR部署在检查通过后执行发布脚本；脚本停止API、备份并在隔离MySQL恢复校验，随后以cost_migrator执行migrate，成功才启动新版API并做健康检查。单独重启API容器不执行迁移。

本次发现新的Go embed清单 category_icon_catalog.json 未进入打包和远端解包白名单，会使图标版本在构建前后缺少运行文件。先新增“清单必须在发布包中”和“远端允许该清单”的回归测试，两项按预期失败；随后只把这个明确路径加入两处白名单，不放开任意JSON文件。

验证：脚本全量53项测试中52项通过、1项Docker外部服务测试跳过，退出0；bash语法检查退出0。实际生成52文件的发布包，并从解包后的backend执行Go构建成功，证明embed清单随包且可编译。未运行Docker镜像构建或真实云端发布，未提交推送，因此不能声称云端已升级至010。
