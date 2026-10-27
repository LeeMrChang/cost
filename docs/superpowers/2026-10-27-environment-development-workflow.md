# 环境与开发发布规范

本规范补充 [2026-10-22 开发流程](2026-10-22-development-workflow.md)，解决本地开发与线上发布分离后的环境、分支、配置、迁移和回滚问题。流程本身仍遵循 Superpowers；本文件只定义“代码怎样从本地走到线上”。

适用范围：仓库全部代码、文档、部署配置与数据库变更。用户明确指令和运行环境上级规则优先。

## 1. 环境分层

| 层 | 用途 | 后端 | 数据库 | 小程序 |
|---|---|---|---|---|
| 本地 development | 日常开发、联调 | `COST_ENV=development`，默认 `127.0.0.1:8080` | 本地 MySQL 或 memory | 开发者工具开发版，走本地地址 |
| CI | 每次 PR / 合入验证 | 临时进程 | 隔离 MySQL 服务 | Node 测试与页面结构检查 |
| 体验版 trial | 试用与验收 | `https://api.xiachen.xyz`（`COST_ENV=production`） | 生产库 | 体验版 |
| 正式版 release | 线上使用 | 同上 | 生产库 | 正式版 |

小程序按 `envVersion` 选择地址：`develop` 使用本地地址（允许登录页覆盖局域网 IP），`trial`、`release` 和无法识别的环境都使用生产地址。详见 [开发与发布环境配置](../ops/2026-10-27-environment-configuration.md)。

**当前事实：体验版与正式版共用同一套生产后端和数据库。** 体验版中的操作会写入真实账本。是否需要独立 staging 见第 9 节，尚未决策。

## 2. 分支与提交

- `main` 始终保持可部署；合入 `main` 视为发布动作，CI 的 `Tests and checks` 与 `Deploy production` 会连续执行。
- 功能分支从最新 `main` 建：`feat/<topic>`、`fix/<topic>`、`docs/<topic>`。同一分支只做一件事。
- 变更通过 PR 合入 `main`，不直接推送到 `main`。合并前确认 CI 通过，并在 PR 中填写验证证据。
- 提交信息说明“为什么”，一次提交保持一个目的；不混入无关格式化、生成产物或密钥文件。
- 提交时机由维护者决定；未经明确指示不推送、不合并。

### main 保护现状

GitHub 免费私有仓库不支持分支保护和规则集：对 `LeeMrChang/cost` 调用保护 API 返回 403 `Upgrade to GitHub Pro or make this repository public`。因此当前采用两层替代措施：

1. 本地 `.githooks/pre-push` 阻止直接推送 `main`。每个克隆需要执行一次：

   ```sh
   ./scripts/setup-git-hooks.sh
   ```

   钩子允许删除已合并的分支，拒绝 `main` 的新提交与删除。它可用 `git push --no-verify` 绕过，是防误操作，不是服务端强制。

2. PR 模板要求勾选 CI 通过；CI 中 `Tests and checks` 是唯一必需的检查名。

若仓库转为公开或升级 GitHub Pro，应补上真正的服务端保护：`main` 要求 PR、要求 `Tests and checks` 通过、禁止强推和删除。开启后本地钩子保留，不冲突。

## 3. 配置与密钥

后端配置全部来自环境变量，启动时先校验再连接任何依赖：

| 变量 | 默认 | 校验 |
|---|---|---|
| `COST_ENV` | `development` | 只能是 `development` 或 `production` |
| `COST_STORE` | `memory` | 只能是 `memory` 或 `mysql`；`production` 必须为 `mysql` |
| `COST_MYSQL_DSN` | 无 | `COST_STORE=mysql` 时必填 |
| `COST_AUTH_MODE` | `wechat` | 只能是 `wechat` 或 `development`；`production` 禁止 `development` |
| `WECHAT_APP_ID` / `WECHAT_APP_SECRET` | 无 | `wechat` 认证时必填 |
| `COST_ADDR` | `0.0.0.0:8080` | 监听地址，不填域名或 `https://` |
| `COST_DEV_TOKEN` | `cost-local-dev` | 仅 `development` 认证使用 |

`-migrate` 只操作存储，跳过认证相关校验，但仍要求 `COST_ENV`、`COST_STORE` 和 DSN 合法。

规则：

- `backend/.env`、`deploy/.env`、AppSecret、会话原文、数据库备份和调试凭证不得进入 Git、前端源码、日志或截图。
- 新增或修改环境变量时，同步更新 `backend/.env.example` / `deploy/.env.example`、本表和 [环境配置文档](../ops/2026-10-27-environment-configuration.md)。
- 前端不读取后端 `.env`；地址集中在 `miniprogram/utils/config.js`。本地地址覆盖只允许出现在开发版。
- 生产环境使用 `COST_ENV=production`，本地和 CI 不得伪装成生产去连接生产库。

## 4. 数据库迁移

正常启动不会自动改表，迁移只能显式执行。新增迁移必须：

1. 先写设计或说明，明确目的、影响表、数据是否可逆。
2. 按 [迁移模板](templates/migration-template.md) 在 `backend/internal/store/migrations/` 新建 `NNN_<topic>.sql`。
3. 在 `backend/internal/store/` 增加对应的 `apply<Name>` 函数，检查 `t_schema_migrations` 版本、按需检查列/表是否存在，执行 SQL 后写入版本；并在 `Migrate()` 中调用。
4. 在隔离测试库跑通，验证旧数据保留和重复执行幂等。
5. **真实数据前先备份**：确认 [自动备份](../ops/2026-10-23-automatic-backups.md) 最近一次成功，或手动备份到指定路径并记录文件名与校验结果。没有可用备份不执行迁移。
6. 迁移后启动新版本并做本地联调；旧版本 SQL 可能不兼容新结构，不并行运行。

回滚优先“恢复备份 + 回退镜像”，不依赖反向 DDL。破坏性变更（删表、删列、改类型）必须在模板中写明不可逆部分。

## 5. 验收门禁

按改动执行，实际读取输出和退出码：

```sh
# backend/
go test -race ./...
go vet ./...
go build ./...

# 仓库根目录
node --test 'miniprogram/tests/*.test.js'
node miniprogram/tests/validate.js
python3 -m unittest discover -s scripts/tests -v
bash -n scripts/deploy-cloud.sh scripts/setup-cicd.sh
```

- 认证、SQL、迁移改动：补充隔离 MySQL 集成测试；未配置测试库时跳过，不能表述为“真实 MySQL 已验证”。
- 前端行为改动：除单测外必须本地联调——迁移后的数据库、实际后端、真实页面操作，并核对关键数字与数据库一致。测试全绿不等于功能可用。
- 微信能力分开记录：语法/页面检查、开发者工具编译、真实 `wx.login` 换取身份、手机真机，各自状态不得互相替代。
- PR 中写日期、命令、环境、结果和跳过项；未执行的检查写“未执行 + 原因”。

## 6. 需求与变更流程

沿用 [开发流程规范](2026-10-22-development-workflow.md)：`brainstorming` → 设计（`specs/`）→ 计划（`plans/`）→ TDD → 审查 → `verification-before-completion`。故障先用 `systematic-debugging` 复现和定位，再写失败测试。

界面变更额外遵守：

- `~/Documents/cost_need/` 的原型是交互基准，iCost 实拍是视觉基准，需求文档定稿优先；文案逐字照抄。
- 动 UI 前重读最新原型和文档，不依据旧截图或记忆。
- 纯文档修正只核对内容与链接，不补造测试或审查记录。

## 7. 回滚

| 对象 | 回滚方式 | 注意 |
|---|---|---|
| 小程序 | 微信后台回退体验版/正式版到上一版本 | 线上版本回退不经过 Git；代码同时用 revert 修正 |
| 后端镜像 | 重新部署上一 `COST_IMAGE_TAG` 或 revert 后走 CI | 数据库结构必须与旧镜像兼容 |
| 数据库 | 停止服务，从备份恢复 | 优先恢复备份，不执行临时反向 SQL |
| 配置/密钥 | 恢复上一份服务器 `.env` 并重启 | 密钥轮换需同步微信后台 |

回滚同样走 PR 与 CI；紧急直改生产时记录原因、时间、影响范围，事后补提交和验证记录。演练或真实回滚后检查 `/health` 和账本读取。

## 8. 记录与文档

- 专题文档使用 `YYYY-MM-DD-english-topic.md`，新增后更新 [文档索引](../README.md)。
- 设计与计划放 `specs/`、`plans/`；运行、迁移、验证放 `ops/`；本规范与模板放 `superpowers/`。
- PR 使用 [模板](../../.github/pull_request_template.md)，四项：改动、验证证据、风险与回滚、未完成项。
- 不伪造设计批准、失败测试、审查或验证证据；补录内容明确标注。

## 9. 待决策：体验版独立 staging

staging 指一套与生产隔离的“预发布环境”：独立后端实例、独立数据库和独立地址，体验版指向它，正式版仍指向生产。它解决当前“体验版操作写入真实账本、迁移先在生产生效”的问题，但会增加运维与费用。

| 方案 | 内容 | 优点 | 代价 |
|---|---|---|---|
| A. 维持现状 | 体验版 = 正式版 = 生产后端 | 零额外成本 | 试用改真实数据；上线前只能在生产验证迁移 |
| B. 独立 staging | 第二套后端 + 数据库 + 域名；`trial` 地址指向 staging | 可自由试用、迁移先演练、故障隔离 | 服务器/数据库成本；需同步配置、备份、CI 部署与数据初始化策略 |
| C. 体验版只读或测试账本 | 生产后端加开关，体验版只读或写入测试账本 | 成本低、风险小 | 不能完整验收写流程；需要额外的环境判断与服务端约束 |

选择 B 需要新增环境地址分支、部署流水线和迁移顺序（staging → 生产），并扩展 `COST_ENV` 允许值；这些都必须单独立设计和计划。当前未选择，代码与环境保持 A。

## 生效状态

- 已生效：启动环境校验、PR 模板、迁移模板、本地 pre-push 钩子、本规范。
- 未生效：GitHub 服务端分支保护（受免费私有仓库限制）。
- 未决策：独立 staging（第 9 节）。
