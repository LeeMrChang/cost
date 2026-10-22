# 小程序本地联调

小程序已统一到仓库的 miniprogram/，微信开发者工具应重新导入这个目录。原来 private_front/cost-miniprogram 只保留为迁移前副本，不再是开发入口。

后端从仓库根目录运行 ./scripts/dev.sh，读取 backend/.env。已有本地数据库连接配置保留；微信登录还需要后端 WECHAT_APP_ID、WECHAT_APP_SECRET，AppID 与 miniprogram/project.config.json 一致。

小程序首先进入登录页。开发版显示调试地址设置，手机应填写电脑局域网 IP，不能填手机自己的 127.0.0.1。按微信开发者工具要求设置开发阶段的域名校验/真机调试。体验和正式版只使用 utils/config.js 的编译配置，发布前必须换成正式 HTTPS 地址。

首次微信登录后，只有持有 backend/.env 中初始化密钥的人能成为管理员。其他用户向管理员获取邀请码，在登录页加入。设置页显示自己的成员编号；管理员可生成邀请码、移除成员和调整预算。

目前不获取微信头像/昵称/手机号，也不自动绑定通讯录。账本共享给全部家庭成员；普通成员仅能修改自己的记录。

验证命令：

```sh
node miniprogram/tests/money.test.js
node miniprogram/tests/calendar.test.js
node miniprogram/tests/auth.test.js
node miniprogram/tests/validate.js
```
