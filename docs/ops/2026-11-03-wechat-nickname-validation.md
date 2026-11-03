# 微信登录昵称验证与升级

2026-11-03。分支 `feat/wechat-user-nickname`，本次只修改代码，不部署、不修改真实用户数据。

本记录对应提交 `791e8cd`；同日追加的账本编辑和“先登录、复用已有昵称”见 [后续验证](2026-11-03-ledger-profile-validation.md)。

## 实际验证

- 后端 TDD 红灯：`go test ./internal/api -run '^TestNicknameWorkflow$' -count=1`，新增带昵称登录被旧代码以 400 invalid_json 拒绝；实现后内存 HTTP 流程通过。
- 前端红灯：新增登录表单空值阻止、旧会话补填、设置保存及请求/缓存测试，旧代码未提交 nickname、空昵称直接进入，测试失败；实现后通过。
- 换账号红灯：退出后残留上一用户昵称；补充测试复现，修复清空昵称。
- `GOCACHE=/private/tmp/cost-go-cache go test -race -count=1 ./...` 通过。该次未设置 MySQL DSN，数据库测试跳过，不能视为真实 MySQL 已验证。
- `GOCACHE=/private/tmp/cost-go-cache go vet ./...`、`go build -o /private/tmp/cost-nickname-api ./cmd/api` 通过。
- Node 全套、页面完整性与最终差异检查：见下方最终记录。

基线检查发现测试月份漂移：前端日历跳转断言固定 2026-10；后端隔离测试写入 10 月账单但用当前月份查询（初次默认缓存掩盖了问题，禁用缓存后出现）。仅调整这两处测试为明确月份/页面月份，没有修改业务时间逻辑。

## 验证限制

真实 MySQL 未完成：配置独立测试库管理 DSN 连接 `/tmp/mysql.sock` 报 operation not permitted；提权请求被自动审批拒绝（环境禁止 sandbox_approval）。另尝试在 `/private/tmp/cost-nickname-mysql` 初始化独立 MySQL 8.4.11，mysqld 在初始化时 SIGSEGV，尚未启动服务。未操作业务库。已编写数据库测试，覆盖数据库昵称流程、64 个 emoji、旧用户数据保留、重复迁移及 DDL 成功但未写版本的恢复，需在有权限的环境执行。

微信开发者工具编译、真实 wx.login 换取身份、原生昵称快速填写及内容安全/输入时序、手机真机均未执行。本次 Node 测试验证的是页面与请求逻辑，不能替代微信平台验证。平台隐私声明应与昵称收集用途保持一致。

## 发布前执行

1. 在有权限的开发环境运行隔离测试（测试只创建并清理随机 cost_test_* 数据库）：

   ```sh
   cd backend
   COST_TEST_MYSQL_ADMIN_DSN='root@unix(/tmp/mysql.sock)/' go test -race -count=1 ./...
   ```

2. 用开发者工具和真机验证：新用户选择微信昵称后登录；历史会话缺昵称补填；保存失败可重试；退出后切账号不残留昵称；设置修改后数据库可见。
3. 备份目标数据库并记录旧用户行数及 id/openid/created_at，使用有 DDL 权限的迁移账号执行 `go run ./cmd/api -migrate`，确认 007 已登记、旧字段与行数保留。
4. 配套更新后端和小程序；新版后端会要求旧客户端用户补填昵称，旧客户端没有此表单，因此应安排协调发布或提示用户更新小程序。未迁移 007 的库不能运行新版认证 SQL。

## 最终记录

- `node --test miniprogram/tests/*.test.js`：86/86 通过。
- `node miniprogram/tests/validate.js`：JavaScript 语法、JSON 与页面文件检查通过；`git diff --check` 通过。
- 独立审查：1 项 P2（退出/切调试地址保留上一用户昵称），已修复并补充失败保存→退出场景。审查还发现 GET me 迟到可覆盖更新后的昵称或跨账号资料，追加 3 个测试复现失败后修复；旧会话响应不能写入新会话或被页面误用。
- 本次采用当前会话实施及最后独立审查；未运行 Superpowers 逐任务脚本，未伪造逐任务提交记录。真实 MySQL 与微信平台验证仍为上文待执行项。
