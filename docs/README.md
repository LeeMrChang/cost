# Cost 文档索引

项目概况和首次运行从 [项目 README](../README.md) 开始。

## 目录约定

| 目录 | 内容 |
|---|---|
| adr/ | 架构与技术决策：背景、选择、取舍和影响 |
| api/ | 接口、身份与权限约定 |
| ops/ | 本地运行、联调、迁移、验证与后续运维 |
| superpowers/ | 开发流程，以及 specs/ 设计、plans/ 实施计划 |

专题文档统一使用 `YYYY-MM-DD-english-topic.md`，英文小写，以连字符分隔。日期为文档建立日期，不因小幅修订而更名；历史过程和验证记录在正文注明实际发生日期。入口文件保留 `README.md`，仓库工具入口保留 `AGENTS.md`、`CLAUDE.md`。目录索引随着新增文档更新。

## 架构决策

- [2026-10-22：统一仓库与家庭认证](adr/2026-10-22-monorepo-and-family-auth.md)（实现后补录）
- [ADR 索引及编写规则](adr/README.md)

## 接口

- [2026-10-22：微信登录与家庭 API](api/2026-10-22-wechat-auth-and-family-api.md)

## 运行与验证

- [2026-10-22：微信小程序本地开发](ops/2026-10-22-miniprogram-local-development.md)
- [2026-10-22：统一仓库与登录改造验证记录](ops/2026-10-22-monorepo-wechat-auth-validation.md)

## Superpowers

- [2026-10-22：开发流程规范](superpowers/2026-10-22-development-workflow.md)
- [2026-10-22：统一仓库与微信登录设计补录](superpowers/specs/2026-10-22-monorepo-wechat-auth-design.md)

原统一仓库与登录改造没有事前实施计划，不补造计划文件或审批记录。后续计划写入 `superpowers/plans/`，并在此添加链接。尚未形成研究或代码地图文档，不创建只有名字的空目录。

## Docker 云端部署

- [2026-10-22：Docker 空账本部署设计](superpowers/specs/2026-10-22-docker-empty-ledger-deployment-design.md)
- [2026-10-22：Docker 部署实施计划](superpowers/plans/2026-10-22-docker-empty-ledger-deployment.md)

容器及空账本已部署，域名/HTTPS 暂缓，实际执行与未覆盖项见验证记录。

- [2026-10-22：Docker 云端部署与运维](ops/2026-10-22-docker-cloud-deployment.md)

- [2026-10-22：Docker 云端部署验证](ops/2026-10-22-docker-cloud-validation.md)

## 数据库表前缀与数据迁移

- [2026-10-22：MySQL 前缀与注释设计](superpowers/specs/2026-10-22-mysql-table-prefix-comments-design.md)
- [2026-10-22：MySQL 前缀与迁移实施计划](superpowers/plans/2026-10-22-mysql-table-prefix-comments.md)
- [2026-10-22：两端升级及本地数据合并云端记录](ops/2026-10-22-mysql-prefix-data-migration.md)
