# 开发与发布环境配置

2026-11-13 更新：仅开发者工具开发版默认本地，其余运行环境默认使用生产 HTTPS。是否允许手动调试覆盖，同时检查微信环境与设备平台；不能仅依据 `envVersion=develop` 就连接本地后端。

| 环境 | 默认接口 | 手动调试地址覆盖 |
|---|---|---|
| develop 且 platform=devtools | `http://127.0.0.1:8081` | 允许；需在登录页显式保存 |
| 手机开发预览、Android/iOS、桌面微信 | `https://api.xiachen.xyz` | 忽略；不展示调试表单 |
| trial 体验版、release 正式版 | `https://api.xiachen.xyz` | 忽略，即使在开发者工具中 |
| 无法识别微信环境或设备 | `https://api.xiachen.xyz` | 忽略 |

设备平台优先使用 `wx.getDeviceInfo()`；旧基础库没有该 API 时使用 `wx.getSystemInfoSync()`。无法确认平台则禁用调试覆盖。新的工具专用缓存键为 `cost_devtools_base_url`，旧 `cost_base_url` 不再读取，旧调试地址不会继续覆盖新版默认地址。开发者工具默认使用本机 8081，仅在端口不同或需要连接云端时手动覆盖。缓存不随上传包分发，不应要求审核人员手动设置。

地址集中维护在 `miniprogram/utils/config.js`。公开生产接口使用既有部署配置里的 `api.xiachen.xyz` 子域名；根域 `xiachen.xyz` 无须同时部署后端。

## 后端与服务器

- 本机 `backend/.env` 已是 `COST_ENV=development`、`COST_ADDR=0.0.0.0:8081`、MySQL 存储与微信认证，本轮保留原值和所有密钥。
- 云端 `deploy/compose.cloud.yaml` 已设置 `COST_ENV=production`，API 宿主机端口绑定 `127.0.0.1:8080`，MySQL 不暴露公网端口。
- 服务器私有 `deploy/.env` 设置 `COST_DOMAIN=api.xiachen.xyz`；`deploy/.env.example` 为无密钥模板。
- Caddy 使用该域名接收 HTTPS，再反向代理到容器内部 `api:8080`。后端监听地址不能填域名或 `https://`。

## 上线前衔接

本轮只修改项目配置，没有修改 DNS、服务器 `.env` 或开启 HTTPS，也没有部署。

备案通过并具备上线条件后，确认 DNS 的 `api` 记录指向服务器，开放所需入站 80/443；按 [云端部署文档](2026-10-22-docker-cloud-deployment.md) 显式启用 `https` profile。验证 `https://api.xiachen.xyz/health` 后，在微信后台配置对应 request 合法域名，再验收体验版与正式版。环境配置就绪不代表域名已经能访问。

## 验证

`environment.test.js` 断言各环境实际发出的 URL，覆盖开发工具默认本地、显式本地覆盖、真实客户端忽略新旧调试缓存、未知环境和设备识别失败。`login-environment.test.js` 执行实际登录页脚本，验证手机不展示调试入口、不能通过调用保存处理函数更改地址，开发工具仍可保存。

2026-11-13 的完整测试、网络验证和待验收项见 [审核登录修复验证](2026-11-13-review-production-backend-validation.md)。单元测试不代表微信编译、真实 code 交换或真机验收通过。
