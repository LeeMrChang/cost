# 验证记录

2026-10-22：统一仓库与微信登录改造。

- Go `go test -race ./...` 通过，包括旧有记账逻辑和新增登录/成员权限测试。
- 真实 MySQL 8.4 集成测试通过：独立临时库中新建/重复迁移、旧结构升级保留账单、微信会话持久化、邀请加入、创建者权限、成员移除和退出。测试库已自动清理。
- `go vet ./...`、`go build -o bin/cost-api ./cmd/api` 通过。
- Node 金额测试、会话请求测试通过；全部 JavaScript 语法、JSON 和页面文件完整性检查通过。
- 本机 cost 数据库升级前已备份；升级前后全部已有账单的原始业务字段逐行一致。
- 微信开发者工具已导入新的 miniprogram 目录，成功编译并显示登录页面。

Git 忽略本地 .env、二进制、project.private.config.json。提交前检查配置中的实际 AppSecret 和初始化密钥未出现在暂存内容中。

真实微信登录已通过：开发者工具当前微信账号成功调用 wx.login → Go 后端 → 微信 jscode2session，返回有效会话并显示未加入家庭状态。未替用户初始化管理员，待本人在首次使用时操作。

云端部署、体验版/真机多人测试、小程序发布审核不在本次阶段内。

## 流程记录说明

本次改造未保留事前 Superpowers 设计/实施计划、完整 TDD 红灯记录或独立代码审查记录；以上测试通过记录不等于流程全部完成。后续约定见 [开发规范](../superpowers/2026-10-22-development-workflow.md)，实现事实见 [设计补录](../superpowers/specs/2026-10-22-monorepo-wechat-auth-design.md)。
