# 微信用户头像 Implementation Plan

> **For agentic workers:** 用户已指定直接修改并通过 PR review；当前会话按 TDD 实施，最后独立审查。

**Goal:** 用户主动选择微信头像后长期保存并在登录/设置展示。
**Architecture:** 头像上传、规范化和读取归 auth；MySQL 用户表存 URL 与小尺寸图片，原子更新；原生选择头像与上传组件接入现有会话缓存。
**Tech Stack:** Go 标准库 image/jpeg/png、MySQL、原生微信小程序。
**Spec:** [设计](../specs/2026-11-03-wechat-avatar-design.md)

## Global Constraints

- JPEG/PNG <=2 MiB，尺寸 <=2048×2048，保存 256×256 JPEG。
- 当前会话决定上传者，用户不能设置头像 URL 或其他用户 ID；读取仅放开随机头像 URL。
- 不新增凭证、环境变量、磁盘持久卷；不部署，用户 review 后才合并。

## Review Focus

- 更改昵称、重登保留头像，头像更换保留昵称。
- 超大/伪造/多文件请求和尺寸限制，未完成昵称用户可选头像。
- 更换头像后旧 URL 失效，随机地址不能当成业务权限绕过。
- 前端失败保持原头像及跨账号请求保护。
- 008 可重入且旧用户身份/创建时间不变。

### Task 1: 后端与迁移

Files: auth/avatar*.go、auth Repository/Memory/MySQL、api 中间件、store 008 迁移与测试。
Interfaces: UpdateAvatar(ctx,id,url,data) Principal；Avatar(ctx,url) []byte；Principal.avatar_url。

- [x] 先写 HTTP/迁移测试，运行看到旧路由拒绝红灯。
- [x] 实现图片校验与规范化、持久化、随机 URL 读取、只更新当前用户。
- [x] 本机 Go race/vet/build；本机 MySQL 跳过，新增隔离测试随提交在 GitHub CI 执行，最终结果同步 PR。

### Task 2: 客户端

Files: utils/api.js、utils/profile.js、login/settings 页面和测试、公共样式。
Interfaces: api.uploadAvatar(filePath) 返回含 avatar_url 用户，缓存版本保护。

- [x] 写上传、展示、保留昵称草稿、失败重试和跨会话测试并观察红灯。
- [x] 实现原生 chooseAvatar/uploadFile、渲染和缓存同步。
- [x] Node 全套与页面检查，文档、独立审查；实现完成后上传 PR #13 供用户 review。

## 执行记录

采用用户指定的现有分支修改方式，没有使用逐任务脚本；HTTP 红灯为缺昵称上传返回403，Node 红灯为上传方法/头像数据不存在。修复后本机 Go race/vet/build、102 项前端测试和页面检查通过。独立审查未发现重要问题；真实微信能力尚未验证，CI 最新结果在 PR 说明中补充。
