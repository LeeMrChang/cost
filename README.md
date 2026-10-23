# Cost 家庭账本

Cost 是供自己和家人使用的记账项目：原生微信小程序负责操作，Go 后端处理身份、权限和账单，MySQL 持久保存数据。当前一个部署服务一个家庭，家庭成员共享账本。

代码已整理到同一私有仓库，微信登录已完成开发者工具真实联调；后端与 MySQL 已通过 Docker 部署到腾讯云，小程序尚未正式发布。域名 ICP 接入备案、HTTPS 与真机多人验收仍待完成。历史验证见 [登录验证](docs/ops/2026-10-22-monorepo-wechat-auth-validation.md)，当前数据库状态见 [表前缀与数据迁移记录](docs/ops/2026-10-22-mysql-prefix-data-migration.md)。

## 功能与边界

- 账单新增、修改、删除、分类选择和按月查询。
- 月收入/支出、分类/每日汇总和月预算。
- 金额输入只接受数字和最多两位小数，账单及月预算统一为整数分；月份按 Asia/Shanghai 划分。
- 微信登录、自有 7 天会话、首次管理员初始化。
- 管理员生成一次性邀请码（24 小时有效），邀请或移除成员。
- 普通成员查看家庭账本，只修改自己的账单；管理员管理全部记录、预算和成员。
- 创建请求使用 client_request_id 去重；不支持离线同步。

当前不提供多家庭切换、个人私密账本、分类管理、图片、标签或 CSV 导出。历史账单保留，创建者为空的记录由管理员维护。

## 仓库结构

```text
cost/
├── backend/              Go 1.23+ 后端、数据库迁移和测试
│   ├── cmd/api/          启动入口
│   ├── internal/         auth、api、domain、store
│   └── .env.example      本地配置模板
├── miniprogram/          微信开发者工具导入这个目录
│   ├── pages/            登录、账本、统计、设置
│   ├── utils/            请求、会话、金额和接口地址配置
│   └── tests/            前端逻辑和文件检查
├── scripts/dev.sh        仓库根目录启动入口
├── deploy/               本地与云端 Docker Compose 配置
├── docs/                 文档索引、ADR、接口、运行与开发流程
├── cost.code-workspace   VS Code 前后端工作区
├── AGENTS.md             仓库开发约定
└── CLAUDE.md             Claude Code 入口
```

本机统一路径为 `/Users/silent/Documents/work/private/cost`。原 `private_front/cost-miniprogram` 保留为迁移前副本，不再是开发入口。其他机器克隆仓库后，使用自己的检出路径。

## 环境要求

| 工具 | 用途 |
|---|---|
| Go 1.23 或更新版本 | 编译和运行后端 |
| MySQL 8.4 | 持久保存账单、用户与会话 |
| 微信开发者工具、有效小程序 AppID、开发者账号 | 编译小程序和获取真实微信登录 code |
| 架构兼容的 Node.js | 运行前端测试；小程序运行本身无需 npm 构建 |
| Docker Compose（可选） | 没有现成 MySQL 时启动本地数据库 |

## 首次运行

以下命令除特别注明外，均在仓库根目录执行。

### 1. 准备数据库

已有本地 MySQL 和 cost 数据库时继续使用，不要再启动一个占用 3306 的数据库实例。新环境可选：

```sh
docker compose -f deploy/compose.local.yaml up -d mysql
```

此配置只把 MySQL 暴露到本机回环地址。默认开发数据库连接示例见配置模板，不能直接用作生产凭证。已有数据库在升级前先备份；不要执行会删除数据卷的 `docker compose down -v`。

### 2. 配置后端

仅在尚无本地配置时复制模板，避免覆盖已有密码和连接：

```sh
cp backend/.env.example backend/.env
```

编辑 `backend/.env`：

| 配置 | 本地使用要求 |
|---|---|
| COST_ADDR | 默认 `0.0.0.0:8080`，用于局域网真机联调 |
| COST_ENV | 本地为 `development` |
| COST_STORE | 真实账单使用 `mysql` |
| COST_MYSQL_DSN | 填写自己的数据库账号、密码和地址 |
| COST_AUTH_MODE | 使用 `wechat` |
| WECHAT_APP_ID | 与 miniprogram/project.config.json 的 appid 一致 |
| WECHAT_APP_SECRET | 对应小程序的服务端密钥 |
| COST_SETUP_SECRET | 首次绑定管理员的密钥，至少 32 字符 |

使用 `openssl rand -hex 32` 生成初始化密钥，并将结果保存在本机 `.env`。AppSecret 和初始化密钥不得提交 Git、写入小程序源码或输出到日志。

本机现有配置已保留在 backend/.env。启动脚本自动加载它；直接运行 `go run` 或二进制不会自动加载该文件。MySQL 账户必须有业务表权限，执行迁移时还需具备创建和修改表的权限。

### 3. 迁移和启动后端

备份确认后执行显式迁移；正常启动不会自动改表。当前数据库使用固定 `t_` 前缀，全部 10 张表及字段有中文注释；旧库由新版迁移命令安全改名，不需要手工删表或重建数据。升级前停止旧版后端，迁移后启动新版，旧版 SQL 不支持改名后的结构：

```sh
./scripts/dev.sh -migrate
./scripts/dev.sh
```

第二个命令持续运行，占用当前终端；在另一个终端检查：

```sh
curl http://127.0.0.1:8080/health
```

健康接口应显示 `status=ok`、`auth_mode=wechat`；它是进程存活检查，不代表数据库、微信登录或家庭权限全部验证成功。停止本地服务可在运行终端按 Ctrl+C。

### 4. 打开小程序

微信开发者工具导入 **miniprogram/**，不是 backend/ 或仓库根目录。AppID 需与后端一致，登录的微信需有相应开发者权限。

- 模拟器：后端地址默认 `http://127.0.0.1:8080`。
- 手机：在登录页“开发环境连接”填电脑的局域网 IP，例如 `http://192.168.1.10:8080`。手机的 127.0.0.1 指向手机自身。
- 本地 HTTP 联调按工具要求使用开发调试设置，确保电脑和手机网络互通、防火墙允许 8080。
- 体验/正式版忽略本地保存的地址，只读取 miniprogram/utils/config.js；发布前需配置正式 HTTPS 地址和合法域名。

### 5. 首次管理员和邀请家人

1. 点击“微信登录”。显示“已登录 · 成员编号”代表身份登录成功，此时不一定有账本权限。
2. 你是第一次创建家庭时，展开“首次使用：初始化家庭”。
3. 将 backend/.env 的 **COST_SETUP_SECRET** 填入“输入服务端初始化密钥”，点击“成为家庭管理员”。
4. 成功后移除服务端初始化密钥配置并重启后端；数据库中的管理员绑定会保留。
5. 管理员在“设置”生成邀请码，发给家人。
6. 家人用自己的微信登录，将邀请码填入“粘贴家庭邀请码”，再点“加入家庭账本”。

**初始化密钥与邀请码不同。** 初始化密钥用于首次绑定管理员；邀请码由管理员生成，供其他成员加入。管理员不能再次初始化，也不能通过普通成员移除接口删除。

## 常见问题

| 现象 | 处理 |
|---|---|
| 邀请码无效、已过期或已使用 | 向管理员获取新邀请码；首次创建家庭应走初始化入口，不能把初始化密钥填进邀请码框 |
| 已登录但不能查看账单 | 尚未加入家庭或已被移除；登录成功不等于成员资格 |
| 家庭已初始化，或已经是成员 | 当前数据库不能重复初始化；确认使用的账号及数据库，不要删除真实数据重试 |
| 服务端尚未配置微信登录 | 检查服务端 AppID/AppSecret，保存后重启服务 |
| 无法连接后端 | 确认服务运行、端口、接口地址、网络和微信开发调试设置 |
| 8080 已占用 | 先确认是否已有 Cost 服务运行，避免重复启动 |
| 重启后账单消失 | 检查是否误用 memory 存储；真实数据必须使用 mysql |
| Node 报 bad CPU type | 换用与本机架构兼容的 Node；本机首次验证使用 Codex 随附运行时 |

## 验证

在 backend/ 中执行：

```sh
go test -race ./...
go vet ./...
go build -o bin/cost-api ./cmd/api
```

涉及认证持久化或数据库升级时，还需运行隔离 MySQL 集成测试。以下是本机 root socket 的示例，应替换为具备创建/删除测试库权限的测试管理连接：

```sh
COST_TEST_MYSQL_ADMIN_DSN='root@unix(/tmp/mysql.sock)/' go test -race ./...
```

测试自动创建随机命名数据库并清理，不使用业务 cost 库。未设置该变量时 MySQL 集成测试跳过，普通测试通过不能视为 MySQL 已验证。

在仓库根目录执行：

```sh
node miniprogram/tests/money.test.js
node miniprogram/tests/calendar.test.js
node miniprogram/tests/auth.test.js
node miniprogram/tests/validate.js
```

微信能力还需要分别验证平台编译、真实微信登录、手机真机和另一个账号的家庭权限；自动测试不能代替这些操作。实际结果与剩余项记录在 [验证文档](docs/ops/2026-10-22-monorepo-wechat-auth-validation.md)。

## 开发流程与文档

开发按 Superpowers：需求与设计 → 实施计划 → TDD → 审查 → 验证。具体规范见 [AGENTS.md](AGENTS.md) 和 [开发流程](docs/superpowers/2026-10-22-development-workflow.md)。

专题文档采用 `YYYY-MM-DD-english-topic.md`，按 ADR、API、运行和 Superpowers 分类；README.md 作为各目录索引保留固定名称。从 [docs/README.md](docs/README.md) 查阅全部文档。

本次已有实现的设计为事后补录，未保留完整事前计划、TDD 红灯和独立审查记录；文档明确说明该缺口，不把测试通过等同于流程全部完成。

## 发布状态

GitHub Actions 自动 CI/CD 配置见 [接入与操作说明](docs/ops/2026-10-23-automatic-cicd.md)：PR 运行检查，main push 检查成功后自动部署后端。首次需要配置服务器专用 SSH 密钥和仓库 Secrets；是否已启用及最近部署结果以 GitHub Actions 为准。小程序仍需单独上传审核发布。

后端和 MySQL 已部署云端，手动备份恢复已验证；本地及云端数据库均完成 `t_` 前缀和注释升级，本地 21 笔账目与月预算已合并至云端，云端原有 2 笔保留。操作与回退边界见 [迁移记录](docs/ops/2026-10-22-mysql-prefix-data-migration.md)。HTTPS/域名、自动备份、入口限流、真机多人验证以及小程序备案、隐私说明和审核尚待完成，不能据此确认正式上线就绪。

后端默认微信认证；显式的 development 令牌模式只供本地后端测试，新小程序不使用该令牌。设置 COST_ENV=production 后，程序禁止开发令牌认证及内存存储，但这些检查不代替部署和发布验收。
