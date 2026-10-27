// 小程序没有运行时 .env；api.js 根据微信 envVersion 选择公开接口地址。
// 体验版/正式版固定使用 HTTPS，开发版可在登录页覆盖为电脑的局域网地址。
// 这里只能放公开配置，不能放 AppSecret、数据库密码或会话令牌。
module.exports = {
 baseURL: 'https://api.xiachen.xyz',
 developmentBaseURL: 'http://127.0.0.1:8080',
};
