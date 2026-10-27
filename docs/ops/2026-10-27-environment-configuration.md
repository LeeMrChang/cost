# 开发与发布环境配置

小程序按微信 `wx.getAccountInfoSync().miniProgram.envVersion` 自动选择接口地址，不读取 Node 的 `process.env`，也不把后端 `.env` 打包到客户端。

| 环境 | 默认接口 | 本地调试地址覆盖 |
|---|---|---|
| develop 开发版 | `http://127.0.0.1:8080` | 允许；登录页可填电脑局域网 IP |
| trial 体验版 | `https://api.xiachen.xyz` | 忽略 |
| release 正式版 | `https://api.xiachen.xyz` | 忽略 |
| 无法识别微信环境 | `https://api.xiachen.xyz` | 忽略 |

地址集中维护在 `miniprogram/utils/config.js`。公开生产接口使用既有部署配置里的 `api.xiachen.xyz` 子域名；根域 `xiachen.xyz` 无须同时部署后端。

## 后端与服务器

- 本机 `backend/.env` 已是 `COST_ENV=development`、`COST_ADDR=0.0.0.0:8080`、MySQL 存储与微信认证，本轮保留原值和所有密钥。
- 云端 `deploy/compose.cloud.yaml` 已设置 `COST_ENV=production`，API 宿主机端口绑定 `127.0.0.1:8080`，MySQL 不暴露公网端口。
- 服务器私有 `deploy/.env` 设置 `COST_DOMAIN=api.xiachen.xyz`；`deploy/.env.example` 为无密钥模板。
- Caddy 使用该域名接收 HTTPS，再反向代理到容器内部 `api:8080`。后端监听地址不能填域名或 `https://`。

## 上线前衔接

本轮只修改项目配置，没有修改 DNS、服务器 `.env` 或开启 HTTPS，也没有部署。

备案通过并具备上线条件后，确认 DNS 的 `api` 记录指向服务器，开放所需入站 80/443；按 [云端部署文档](2026-10-22-docker-cloud-deployment.md) 显式启用 `https` profile。验证 `https://api.xiachen.xyz/health` 后，在微信后台配置对应 request 合法域名，再验收体验版与正式版。环境配置就绪不代表域名已经能访问。

## 验证

`environment.test.js` 校验开发默认/局域网覆盖、体验/正式版不采用本地地址，以及未知环境回退生产地址；断言实际发出的请求 URL。相关会话测试继续验证认证行为。
