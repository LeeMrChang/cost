# 微信头像与 CI 验证

2026-11-03，功能分支 `feat/wechat-user-nickname`，PR #13。用户要求添加头像和修复 CI；不合并或部署，不操作真实业务库。

## CI 原因与修复

从真实 Actions 日志定位到前缀测试硬编码旧结构 60 列，但新增 nickname 后已有 61 列。修正为至少包含原结构列且所有列有注释，不跳过任何测试。远端提交 `b7c5a1c9` 的 [Tests and checks](https://github.com/LeeMrChang/cost/actions/runs/41619774232/job/125862250258) 已成功，包含隔离 MySQL 8.4、Go/Node、部署故障检查和 workflow lint。PR 的生产部署 skipped 为预期行为。

该成功早于头像提交，不能作为 008 迁移已通过的证据。头像提交后的完整 CI 结果在下方另记。

## 头像实际验证

- TDD 后端红灯：`TestAvatarWorkflow` 在旧代码上传时返回 403 nickname_required；实现后支持缺昵称已登录用户上传。
- HTTP 测试覆盖 PNG/JPEG、规范化 256 JPEG、GET/HEAD 无需 Bearer、非 GET 不绕过认证、重登与昵称更新保留头像、替换旧地址失效、不同用户隔离、无会话/非法/超大/多文件/额外身份字段拒绝。
- 008 的真实 MySQL 共用流程与旧记录/部分 DDL/重复迁移测试已编写；本机仍无权访问 MySQL socket，交由本次 GitHub CI 执行。
- Node 红灯：新上传方法/头像状态不存在；实现后上传、缓存、迟到 me 与跨账号响应保护、取消、失败重试、昵称草稿保留和退出清空头像全部通过。
- 本机 `GOCACHE=/private/tmp/cost-go-cache go test -race -count=1 ./...`、`go vet ./...`、`go build -o /private/tmp/cost-avatar-api ./cmd/api` 通过；未配置 MySQL DSN，该次数据库测试跳过。
- 本机 Node 全套 102/102、语法/页面完整性与差异检查通过。新增 HEAD/JPEG 覆盖后重新执行 Go race 全套通过。
- 独立代码审查：未发现确定性重要问题；核对了图片限制、单文件、权限边界、昵称保留、迁移重入与跨会话缓存保护。

## 微信与发布待验证

开发者工具编译、真实 wx.login / chooseAvatar / uploadFile、真机显示和上传合法域名尚未实际验证。上线前配置 uploadFile 合法域名，在真机选择当前微信头像、刷新/重登确认显示、更换头像并验证旧地址失效；备份真实库并迁移至 008。只从 PR 合并后的正式发布流程部署，等待维护者 review。

## 最终检查

头像提交后的隔离 MySQL、008 迁移与完整检查由 [PR #13 的当前提交检查](https://github.com/LeeMrChang/cost/pull/13/checks) 验证；最终远端结果同步在 PR 说明中，不能用上文旧提交的成功替代当前提交验证。
