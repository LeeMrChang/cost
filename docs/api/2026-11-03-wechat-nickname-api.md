# 微信登录昵称 API

2026-11-03。当前身份模型无 role，用户响应为 `{"id":1,"nickname":"小夏🐱","avatar_url":""}`，头像字段及上传见 [头像 API](2026-11-03-wechat-avatar-api.md)。昵称由用户主动填写或通过微信原生昵称输入框选择，属于展示资料；服务端仍只使用校验成功的 openid 和 Bearer 会话确定身份。

## 登录及资料

`POST /api/v1/auth/wechat`：`{"code":"微信临时code"}`，也接受可选 `nickname`。后端验证 code 后建立会话，提供昵称时在同一事务保存，返回 `token`、`expires_at`、`user`。省略 nickname 时复用该微信身份已保存的昵称，不清空已有值；尚无昵称的用户取得用于补填的会话，但业务接口返回 403 `nickname_required`。传入空白昵称返回 400 `invalid_nickname`。

`GET /api/v1/auth/me`：Bearer 认证，返回当前 id 和最新 nickname。历史用户尚未补填时 nickname 为空字符串。

`PATCH /api/v1/auth/me`：Bearer 认证，仅接受 `{"nickname":"新昵称"}`；返回更新后的用户，其他有效会话下次读取也可见。不能传入 id、openid 等身份字段。未登录返回 401；无效昵称返回 400 `invalid_nickname`，未知字段、损坏 JSON 或非法 UTF-8 返回 400 `invalid_json`。

`POST /api/v1/auth/logout`：行为不变，缺昵称用户也可退出。

昵称去除首尾空白后为 1–64 个 Unicode 码点；支持中文、emoji，拒绝控制字符与行分隔字符。昵称不是唯一键，不保证对应微信当前昵称，微信改名不会自动同步。昵称不会出现在请求日志中，响应不暴露 openid 或 session_key。

## 缺昵称账号

除健康检查、微信登录和随机头像地址读取外，缺昵称账号只允许通过 Bearer 会话查询/更新自身资料、上传头像和退出。其他业务接口返回 HTTP 403 `{"error":"nickname_required"}`，直到成功补填。小程序保留该会话并回到登录页显示补填表单。新登录先用微信身份取得会话，已有昵称直接复用，缺昵称才填写并 PATCH，保存后进入账本。设置页展示并允许修改昵称。

补填时会话过期会重新微信认证；仅在服务端返回相同用户 ID 时继续保存已填昵称，账号切换则清空草稿，避免把上一账号昵称带给新账号。

## 数据迁移

新增显式迁移 `007_user_nickname.sql`：`t_users.nickname VARCHAR(64) CHARACTER SET utf8mb4 NOT NULL DEFAULT ''`。历史 id、openid、created_at 不变，空昵称等待用户补填；已有 openid 不能反查昵称。迁移遵循命名锁与可重入步骤，DDL 成功而版本未落库时可以重试。

升级流程与验证边界见 [运行记录](../ops/2026-11-03-wechat-nickname-validation.md)。
