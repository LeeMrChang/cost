# Cost 家庭账本

Go + MySQL 后端与原生微信小程序的统一仓库。一个部署服务一个家庭；家庭成员共享账本，管理员邀请成员，成员只编辑自己的账单。

## 目录

```text
backend/       Go 1.23+ API、持久化、迁移、测试
miniprogram/   原生 JavaScript / WXML / WXSS 微信小程序
scripts/       从仓库根目录启动和检查
deploy/       本地数据库配置
docs/         登录、联调和验证说明
```

已有功能：账单新增/修改/删除、月明细、分类/月收支/每日汇总、月预算、创建请求去重。金额单位为分，月份按 Asia/Shanghai 计算。

新增：微信 code 登录、7 天会话、家庭管理员初始化、一次性邀请码（24 小时有效）、成员移除、账单创建者权限。数据库只存会话和邀请码的 SHA-256 摘要，不保存微信 session_key，不向前端返回 OpenID 或 AppSecret。

## 本地启动

1. 安装 Go 1.23+ 与 MySQL 8.4；已有数据库可继续使用。新环境可使用 `docker compose -fdeploy/compose.local.yaml up -d`。
2. 复制 `backend/.env.example` 为 `backend/.env`（已有本地配置请勿覆盖），填写数据库连接、`WECHAT_APP_ID`、`WECHAT_APP_SECRET`，生成 `COST_SETUP_SECRET`：`openssl rand -hex 32`。AppID 必须与小程序一致。
3. 修改结构前先备份数据库，然后执行 `./scripts/dev.sh -migrate`。这是显式操作；正常启动不会自动改表。
4. 执行 `./scripts/dev.sh` 启动 API。
5. 微信开发者工具导入 **miniprogram/**，登录自己的微信开发者账号；也可以用 `cost.code-workspace` 打开前后端开发工作区。
6. 本地真机联调时，在登录页“开发环境连接”填写电脑局域网地址（如 `http://192.168.1.10:8080`），按开发者工具要求开启调试并关闭开发阶段域名校验；手机和电脑需网络互通。体验/正式版忽略本地调试地址，只使用 `miniprogram/utils/config.js`。
7. 微信登录后首次使用，在“初始化家庭”中输入 `COST_SETUP_SECRET`，成为唯一管理员。初始化成功后从服务环境中移除该密钥并重启。随后在“家庭设置”生成邀请码给家人。

当前本地 `.env` 保留在 backend/，不提交 Git。不要将 AppSecret 或初始化密钥粘贴到小程序配置、GitHub 或日志中。

## 权限规则

- 微信登录成功不等于加入家庭。未加入者只能访问自己的登录信息、退出、管理员初始化或邀请码加入接口。
- 家庭成员可查看整个家庭账本；普通成员只修改/删除自己的记录。
- 管理员可管理全部记录、预算、邀请码和普通成员。管理员不能被成员移除接口删除。
- 历史账单的创建者为空，原有账单/分类/预算不删除；管理员可维护历史账单。
- 移除成员后，已有会话每次访问都会重新检查成员身份，立即失去账本访问权限。账单和创建者信息保留。
- 当前仅一个家庭账本（ledger_id=1），没有个人私密账本或多家庭切换功能。

## 验证

```sh
cd backend
go test -race ./...
go vet ./...
go build -o bin/cost-api ./cmd/api
```

在仓库根目录执行 `node miniprogram/tests/money.test.js`、`node miniprogram/tests/auth.test.js` 和 `node miniprogram/tests/validate.js`。

可选 MySQL 集成测试：在 backend/ 设置 `COST_TEST_MYSQL_ADMIN_DSN='root@unix(/tmp/mysql.sock)/'` 后运行 `go test -race ./...`。测试自动创建随机名称的临时数据库并清理；不会使用本地 cost 数据库。测试账号需要创建/删除测试数据库权限。

## 开发模式与发布

默认 `COST_AUTH_MODE=wechat`。没有微信密钥时，登录返回明确的“尚未配置”错误，不会使用模拟身份替代。已有 API 的开发令牌模式需显式设置 `COST_AUTH_MODE=development`，只供本地后端测试；新小程序仅使用微信会话。

`COST_ENV=production` 禁止开发令牌和内存存储。生产 HTTPS、域名、小程序备案/隐私说明/审核、云端备份及部署属于后续阶段，尚未上线。发布前还需要配置小程序正式 HTTPS API 地址并启用域名校验；当前项目保留本地联调配置。

更多说明：[微信登录与 API](docs/AUTH.md)、[小程序联调](docs/MINIPROGRAM.md)、[验证记录](docs/VALIDATION.md)。

## 开发规范

开发遵循 [Superpowers 流程](docs/DEVELOPMENT.md)，仓库约定见 [AGENTS.md](AGENTS.md)。本次登录改造的 [设计补录](docs/superpowers/specs/2026-10-22-monorepo-wechat-auth-design.md) 明确记录既有流程缺口，不作为事前设计或 TDD 证据。
