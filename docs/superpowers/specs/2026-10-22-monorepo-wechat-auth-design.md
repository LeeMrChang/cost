# 统一仓库与微信登录：实现设计补录

日期：2026-10-22。对应实现提交：`0518b049785da3b3d37ce91bf5ce07341a0e2590`。

**性质：实现后的补录，不是实施前设计，不代表曾完成 Superpowers 设计批准、计划审批、TDD 红灯过程或独立代码审查。**

## 用户目标与本次边界

将现有 Go 后端和原生微信小程序整理到本地 cost 根目录及同一个私有 GitHub 仓库，完善微信登录和家庭权限。保留本地数据库和已有账单。本阶段不购买云资源、不部署公网服务、不提交微信发布审核。

## 目录与责任

- backend/cmd/api：组装存储、身份服务和 HTTP 服务，读取环境配置。
- backend/internal/domain：账单、金额、月份和汇总。
- backend/internal/store：账单持久化、显式版本迁移。
- backend/internal/auth：微信 code 交换、自有会话、家庭初始化与邀请；内存和 MySQL 实现。
- backend/internal/api：账单 API、请求身份校验和创建者权限。
- miniprogram/utils：请求、会话与金额工具；pages/login、ledger、stats、settings 为四类页面。
- docs、deploy、scripts：专题说明、本地数据库配置和启动入口。

## 身份与权限

用户主动点击微信登录，客户端提交一次性 code，服务端使用 AppID/AppSecret 向微信换取 OpenID。服务端签发 32 字节随机令牌，有效期 7 天；会话原文仅交给客户端，数据库存 SHA-256 摘要。微信 session_key 不保存或下发。

登录与成员资格分离。未加入者可查询自身身份、退出、初始化或加入家庭，不能读取账单。首次管理员绑定要求至少 32 字符初始化密钥，并以数据库事务保证只能初始化一次。管理员生成 24 小时有效、只能使用一次的邀请码；加入操作事务性消费邀请码。

所有家庭成员可读取同一个账本。普通成员仅编辑自己创建的记录；管理员维护全部记录、预算和普通成员。每次请求重新读取成员身份，移除后现有会话失去账本权限，但记录保留。

## 数据与迁移

当前一个部署对应一个家庭，保留 ledger_id=1；没有多家庭切换或个人私密账本。

保留原有 ledgers、categories、entries、budgets。002 增加 users、sessions、family_setup、family_members、family_invites，以及 entries.created_by。旧账单创建者为空，交由管理员维护。金额单位为分，月份按 Asia/Shanghai。

schema_migrations 记录迁移版本；命名锁避免同库迁移并发。MySQL DDL 不支持整个升级事务回滚，因此步骤可重入，完成后才记版本。真实库升级前备份，并对比原账单业务字段；测试使用随机隔离数据库。

## 配置与故障

后端默认微信认证。开发令牌需显式启用，production 环境禁止开发令牌和内存存储。AppSecret、初始化密钥和数据库连接只在被忽略的本地 .env 或服务端环境配置中。

没有 AppSecret 时明确返回未配置错误；微信无效 code 与服务不可用分别处理。会话无效返回 401，未加入家庭或越权返回 403。邀请码无效/过期/已使用与管理员初始化是不同流程。

登录、初始化和加入接口使用进程内直连 IP 限流；未来代理或多实例部署需要重新设计入口限流。客户端开发地址仅在开发版生效；体验/正式版使用编译配置，尚未配置云端 HTTPS 地址。

## 验收映射与证据边界

| 验收项 | 实现与证据位置 |
|---|---|
| code 交换、微信错误映射、不下发身份密钥 | backend/internal/auth/auth_test.go |
| 未加入拒绝读取、初始化、一次性加入、越权拒绝、移除及退出 | backend/internal/api/auth_test.go |
| 会话失效、邀请码过期/并发消费 | backend/internal/auth/auth_test.go 和 api/auth_test.go |
| 旧结构升级与数据保留、MySQL 权限流程 | backend/internal/api/mysql_integration_test.go |
| 请求携带会话、401 清理、开发地址隔离 | miniprogram/tests/auth.test.js |
| 金额转换与页面文件检查 | miniprogram/tests/money.test.js、validate.js |
| 本机迁移、平台编译、真实微信登录 | docs/ops/2026-10-22-monorepo-wechat-auth-validation.md 中 2026-10-22 实际记录 |

原验证记录中的测试结果属于当时执行证据，不等于后续每次变更自动通过。真机多人流程和独立审查未在初次交付记录中完成；不能据此宣称生产发布就绪。

## 后续工作

首次管理员由用户本人初始化。云端部署、备份恢复演练、正式 HTTPS 配置、手机多人验证、小程序备案与审核为后续阶段；不在本次补录中假定完成。
