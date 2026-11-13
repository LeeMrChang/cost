const assert = require('node:assert/strict');
const storage = new Map();
let handler, redirects = 0, requests = [];
global.getCurrentPages = () => [];
global.wx = {
 getAccountInfoSync: () => ({ miniProgram: { envVersion: 'develop' } }),
 getDeviceInfo: () => ({ platform: 'devtools' }),
 getStorageSync: key => storage.get(key), setStorageSync: (key, value) => storage.set(key, value), removeStorageSync: key => storage.delete(key),
 reLaunch: options => { redirects++; options.complete(); },
 login: options => options.success({ code: 'one-time-code' }),
 request: options => { requests.push(options); handler(options); }
};
const api = require('../utils/api');
(async () => {
 await assert.rejects(api.request('/api/v1/entries'), /请先微信登录/);
 assert.equal(redirects, 1); assert.equal(requests.length, 0);
 handler = options => {
  assert.equal(options.data.code, 'one-time-code'); assert.equal(options.header.Authorization, undefined);
  assert.equal(options.data.nickname, '小夏🐱');
  options.success({ statusCode: 200, data: { token: 'opaque-session', expires_at: new Date(Date.now()+3600000).toISOString(), user: { id: 1, nickname: '小夏🐱' } } });
 };
 await api.login('小夏🐱');
 handler = options => {
  assert.ok(options.url.endsWith('/api/v1/auth/me')); assert.equal(options.method, 'PATCH');
  assert.equal(options.header.Authorization, 'Bearer opaque-session'); assert.equal(options.data.nickname, '新昵称');
  options.success({statusCode: 200, data: {id:1, nickname:'新昵称'}});
 };
 await api.updateNickname('新昵称');
 assert.equal(api.session().user.nickname, '新昵称'); assert.equal(api.session().token, 'opaque-session');
 handler = options => { assert.equal(options.header.Authorization, 'Bearer opaque-session'); assert.equal(options.header['X-Dev-Token'], undefined); options.success({ statusCode: 200, data: [] }); };
 await api.request('/api/v1/entries');
 handler = options => options.success({statusCode:403,data:{error:'nickname_required'}});
 const before = redirects;
 await assert.rejects(api.request('/api/v1/entries'), /昵称/);
 assert.equal(redirects, before+1); assert.equal(api.session().token, 'opaque-session');
 wx.setStorageSync('cost_devtools_base_url', 'http://192.168.1.2:8080');
 assert.equal(api.config().baseURL, 'http://192.168.1.2:8080');
 wx.getAccountInfoSync = () => ({ miniProgram: { envVersion: 'release' } });
 assert.equal(api.config().baseURL, 'https://api.xiachen.xyz');
 handler = options => options.success({ statusCode: 401, data: { error: 'unauthorized' } });
 await assert.rejects(api.request('/api/v1/entries'), /登录已过期/); assert.equal(api.session(), null);
 wx.setStorageSync('cost_session', { token: 'expired', expires_at: '2020-01-01' });
 assert.equal(api.session(), null);
 wx.setStorageSync('cost_session', { token: 'session', expires_at: new Date(Date.now()+3600000).toISOString() });
 handler = options => { assert.ok(options.url.endsWith('/api/v1/auth/logout')); options.success({ statusCode: 204 }); };
 await api.logout(); assert.equal(api.session(), null);
 console.log('auth request/session tests passed');
})().catch(error => { console.error(error); process.exitCode = 1; });
