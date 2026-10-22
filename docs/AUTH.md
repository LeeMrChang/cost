# 微信登录与家庭权限

## 登录流程

小程序在用户点击登录时调用 `wx.login()`，把临时 code 发给后端 `POST /api/v1/auth/wechat`。后端通过 HTTPS 调用微信 `jscode2session` 校验身份，生成 32 字节随机会话令牌，有效期 7 天。客户端存原始令牌，服务端仅存 SHA-256 摘要。

微信 AppSecret 只放在 backend/.env（部署时放服务端密钥配置），不在小程序、源码、日志或接口响应中出现。微信 session_key 不保存或下发。每次请求校验会话过期时间并读取当前成员身份；退出登录撤销当前令牌。不同设备可各自登录。

参考微信官方接口：
- https://developers.weixin.qq.com/miniprogram/dev/api/open-api/login/wx.login.html
- https://developers.weixin.qq.com/miniprogram/dev/OpenApiDoc/user-login/code2Session.html

## 接口

除健康检查、微信 code 登录外，均使用 `Authorization: Bearer <token>`。业务接口要求家庭成员。

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | /api/v1/auth/wechat | `{"code":"微信临时code"}`，返回 token、expires_at 和 user |
| GET | /api/v1/auth/me | 返回当前用户 id 和 role，未加入家庭时 role 为空 |
| POST | /api/v1/auth/logout | 撤销当前会话 |
| POST | /api/v1/family/bootstrap | `{"secret":"初始化密钥"}`；仅可成功一次 |
| POST | /api/v1/family/invites | 管理员生成 24 小时、一次性邀请码 |
| POST | /api/v1/family/join | `{"token":"邀请码"}`，加入家庭 |
| GET | /api/v1/family/members | 管理员查看成员列表 |
| DELETE | /api/v1/family/members/{id} | 管理员移除普通成员 |

初始化密钥至少 32 字符。只有持有密钥的已登录用户可以初始化，不能由首次微信登录者自动占有家庭。数据库事务确保只能有一个管理员初始化成功。初始化完成后删除服务端该配置并重启；数据库中的初始化状态会保留。

登录、初始化和加入家庭接口有单实例、按直连 IP 计算的每分钟 30 次请求限制；不信任客户端提供的 X-Forwarded-For。未来部署代理/多实例时需要重新配置入口限流，代理后的用户可能共享此限制。

## 账单与数据库升级

保留原来的 ledger_id=1，服务一个家庭。账单新增的 created_by 为服务器判定的用户 ID，忽略客户端传入值；更新不改变创建者。成员不能编辑他人的记录，预算只允许管理员修改。

001 保留原有四张表与数据；002 添加 users、sessions、family_setup、family_members、family_invites 和 entries.created_by。使用 schema_migrations 记录版本；迁移持有数据库级命名锁。MySQL DDL 不支持整体事务回滚，因此迁移采用可重入步骤，只有全部成功才记版本。迁移前备份，失败后修复原因再重试。旧记录创建者保留 NULL，由管理员维护。

不要把同一数据库用于不同 AppID；OpenID 仅在所属小程序内标识用户。变更 AppID 需要专门迁移身份。

## 本地真实微信验证

1. 填写后端 WECHAT_APP_SECRET，并确认 AppID 与小程序一致。
2. 运行数据库迁移与后端，微信开发者工具导入 miniprogram/。
3. 设置电脑局域网地址，使用真实开发者微信登录。
4. 初始化管理员，生成邀请码；用另一个开发者/体验成员微信测试加入。
5. 验证各自记账、权限拒绝、退出/重登、移除后拒绝访问。

自动测试使用可注入的微信服务替身检查 code 交换和权限，不替代微信平台的真实凭证联调。
