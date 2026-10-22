# 开发流程：Superpowers

本项目使用 Superpowers 的设计、计划、测试、审查和验证流程。仓库执行入口为 [AGENTS.md](../AGENTS.md)，Claude Code 入口为 [CLAUDE.md](../CLAUDE.md)。

本说明于 2026-10-22 对照本机已安装的 Superpowers 7.0.0 整理。具体工作时读取当前版本技能；此文档是项目约束与操作索引，不代替技能全文。不同客户端的技能安装和启用状态彼此独立。

## 新功能与行为变更

1. 使用 `using-superpowers` 识别适用技能，说明本次使用的流程。
2. 使用 `brainstorming` 明确目标、非目标和验收标准；已有代码优先复用，按技能要求完成设计讨论和确认。
3. 将设计保存到 `docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md`。至少覆盖数据结构、接口、身份和权限边界、迁移、错误处理与验证方式。
4. 使用 `writing-plans` 编写 `docs/superpowers/plans/YYYY-MM-DD-<topic>.md`。引用设计，拆解有明确文件、接口和验收命令的任务，遵循技能的计划头和勾选项格式。
5. 按用户已经指定或确认的执行方式，使用 `executing-plans` 或 `subagent-driven-development`。不重复询问已经决定的执行方式；遵守运行环境的代理调用约束。
6. 使用 `test-driven-development` 完成红—绿—重构，保留实际测试证据。测试失败应来自缺少预期行为，环境错误或语法错误不能充当红灯证据。
7. 使用 `requesting-code-review` 审查实现是否符合设计及质量要求，用 `receiving-code-review` 核实和处理反馈。
8. 使用 `verification-before-completion` 运行本次所需检查，再按 `finishing-a-development-branch` 和用户授权完成提交、PR 或合并。

若已授权进入实施，不自行重开需求讨论；只有会改变设计或缺少必要决策时再明确提出。纯文档修改不伪造红绿测试；配置等例外按实际技能和已有用户授权处理。

## 问题修复

先使用 `systematic-debugging`，记录现象、复现步骤和证据，区分微信登录失败、会话过期、家庭成员权限、初始化密钥以及邀请码错误。定位原因后写可复现的失败测试，再做最小修复。不要仅凭错误提示猜测并连续修改。

## 项目验证命令

在 `backend/`：

```sh
go test -race ./...
go vet ./...
go build -o bin/cost-api ./cmd/api
```

认证、SQL 或迁移变更还需隔离数据库集成测试。以下为本机无密码 root socket 的示例；换环境时使用自己的测试管理连接，不要写入真实凭证或操作业务库：

```sh
COST_TEST_MYSQL_ADMIN_DSN='root@unix(/tmp/mysql.sock)/' go test -race ./...
```

测试创建随机名称的数据库并清理。未配置环境变量时 MySQL 测试会跳过；普通测试通过不能表述为“真实 MySQL 已验证”。

在仓库根目录：

```sh
node miniprogram/tests/money.test.js
node miniprogram/tests/auth.test.js
node miniprogram/tests/validate.js
```

Node 必须与本机架构兼容。2026-10-22 的测试使用 Codex 随附 Node；本机默认 `/usr/local/bin/node` 当时存在架构不兼容问题。遇到环境问题应记录实际使用的运行时，不把未执行的测试记为通过。

微信验证分开记录：

- 语法和页面完整性检查：不能证明平台编译或真实登录。
- 微信开发者工具编译：不能证明后端换取身份成功。
- 真实 `wx.login → 后端 → jscode2session`：证明当前账号的登录链路。
- 手机真机、另一微信账号、加入/移除家庭：分别证明设备环境和多人权限；未执行则保留待验证状态。

## 文档与证据

| 文件 | 职责 |
|---|---|
| README.md | 目录、启动和功能边界 |
| docs/AUTH.md | 登录、会话、家庭权限、接口与数据库升级 |
| docs/MINIPROGRAM.md | 开发者工具导入和本地联调 |
| docs/VALIDATION.md | 按日期记录实际验证、环境与未覆盖项 |
| docs/superpowers/specs/ | 事前设计或明确标注的事后补录 |
| docs/superpowers/plans/ | 实施前的任务计划和执行进度 |

每次验证记录说明日期、命令、运行环境、结果和跳过项。截图中的“已登录”与“已加入家庭”是不同状态；邀请码错误不能作为微信登录失败的证据。

## 本次改造的流程缺口

2026-10-22 的统一仓库与微信登录改造补充了专题文档和测试，但没有事前 Superpowers 设计/计划，也没有完整红灯记录和独立代码审查记录。本次补录不改变这一事实；不能宣称该改造完整遵循了 Superpowers。

已实现行为见 [补录设计说明](superpowers/specs/2026-10-22-monorepo-wechat-auth-design.md)。该文档用于维护和后续复核，不能代替事前设计确认。后续开发必须按上述流程留下相应证据；如需对既有实现补做独立审查，应单独记录实际审查结果。
