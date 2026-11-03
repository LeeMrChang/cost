# 微信用户头像 API

2026-11-03。头像由用户主动点击原生 `chooseAvatar` 选择；wx.login 不提供头像。客户端将微信返回的临时文件通过 wx.uploadFile 上传，服务端持久保存图片与稳定地址。头像可选，不影响昵称必填规则。

## 上传与读取

`POST /api/v1/auth/me/avatar`：Bearer 认证，multipart/form-data，恰好一个名为 `avatar`、带文件名的文件，不接受其他表单字段或用户 ID。JPEG/PNG 文件不超过 2 MiB，宽高均不超过 2048 像素。服务端解码并中心裁剪为 256×256、白底 JPEG 后保存，不保存原始图片/元数据，不接收客户端头像 URL。

返回用户 `{id,nickname,avatar_url}`。无效文件或多个文件返回 400 `invalid_avatar`，超限返回 413 `avatar_too_large`，未登录返回 401。尚未填写昵称的用户也允许上传。更新头像不覆盖昵称；更新昵称不清空头像。

`avatar_url` 为 `/api/v1/avatars/<64位随机hex>.jpg`。客户端按当前 API baseURL 拼接展示。GET/HEAD 此完整路径不要求 Bearer，供原生 image 使用随机链接读取；返回 Content-Type image/jpeg、nosniff、private max-age=3600。随机路径不存在或头像已被更换时返回 404。无需图片内容的 HEAD 返回同样响应头且没有响应体；其他业务路由仍需认证。

保存成功后，新登录与 GET me 均返回最新 avatar_url。替换头像会更换随机地址，旧地址不再关联服务端图片；本地先前缓存可能保留到客户端正常淘汰。不在请求日志输出完整随机读取地址。

## 数据与平台配置

显式迁移 `008_user_avatar.sql`：新增 `t_users.avatar_url VARCHAR(128) ASCII NULL DEFAULT NULL`、`avatar_data MEDIUMBLOB NULL` 和头像路径唯一索引。未设置头像时数据库为 NULL、用户响应为空字符串。头像字节随数据库备份，容器重建不丢失，不依赖新增文件卷或外部对象存储账号。

微信公众平台将当前 API HTTPS 域名加入 **uploadFile 合法域名**（与 request 域名配置独立），例如当前正式接口 `https://api.xiachen.xyz`。开发版可用现有本地调试地址；正式版仍使用固定 HTTPS 配置。更新隐私说明以覆盖用户主动提交的头像用途。无新增后端环境变量。

设置页“个人资料”和登录后的补填表单显示头像选择按钮。取消选择不上传；失败保留原头像与昵称草稿；保存成功更新会话缓存。退出或切调试后端清除页面头像，旧账号迟到上传响应不能写入新会话。

升级前备份并迁移到 008，再启动新版后端。回滚代码可保留新增列，不破坏性删列。[实际验证与未覆盖项](../ops/2026-11-03-wechat-avatar-validation.md)。
