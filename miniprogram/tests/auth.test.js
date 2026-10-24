const assert = require('node:assert/strict');
const storage = new Map();
let handler, redirects = 0, requests = [];
global.getCurrentPages = () => [];
global.wx = {
 getAccountInfoSync: () => ({ miniProgram: { envVersion: 'develop' } }),
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
  options.success({ statusCode: 200, data: { token: 'opaque-session', expires_at: new Date(Date.now()+3600000).toISOString(), user: { id: 1, role: 'admin' } } });
 };
 await api.login();
 handler = options => { assert.equal(options.header.Authorization, 'Bearer opaque-session'); assert.equal(options.header['X-Dev-Token'], undefined); options.success({ statusCode: 200, data: [] }); };
 await api.request('/api/v1/entries');
 wx.setStorageSync('cost_base_url', 'http://192.168.1.2:8080');
 assert.equal(api.config().baseURL, 'http://192.168.1.2:8080');
 wx.getAccountInfoSync = () => ({ miniProgram: { envVersion: 'release' } });
 assert.equal(api.config().baseURL, 'http://127.0.0.1:8080');
 handler = options => options.success({ statusCode: 401, data: { error: 'unauthorized' } });
 await assert.rejects(api.request('/api/v1/entries'), /登录已过期/); assert.equal(api.session(), null);
 wx.setStorageSync('cost_session', { token: 'expired', expires_at: '2020-01-01' });
 assert.equal(api.session(), null);
 wx.setStorageSync('cost_session', { token: 'session', expires_at: new Date(Date.now()+3600000).toISOString() });
 handler = options => { assert.ok(options.url.endsWith('/api/v1/auth/logout')); options.success({ statusCode: 204 }); };
 await api.logout(); assert.equal(api.session(), null);
 // 旧账号延迟返回的昵称或401不能覆盖/清除新账号会话。
 const sessionA = {token: 'account-a', expires_at: new Date(Date.now()+3600000).toISOString(), user: {id: 1, nickname: '甲'}};
 const sessionB = {token: 'account-b', expires_at: sessionA.expires_at, user: {id: 2, nickname: '乙'}};
 for (const statusCode of [200, 401, 403]) {
  wx.setStorageSync('cost_session', sessionA);
  let pending; handler = options => {pending = options;};
  const first = api.me();
  wx.setStorageSync('cost_session', sessionB);
  pending.success({statusCode, data: statusCode === 200 ? sessionA.user : {error: statusCode === 401 ? 'unauthorized' : 'family_access_required'}});
  await assert.rejects(first, /登录状态已变化/);
  assert.equal(api.session().token, 'account-b'); assert.equal(api.session().user.nickname, '乙');
 }
 console.log('auth request/session tests passed');
})().catch(error => { console.error(error); process.exitCode = 1; });
