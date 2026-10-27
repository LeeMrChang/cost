const defaults = require('./config');
let redirecting = false;
function isDevelop() {
 try { return wx.getAccountInfoSync().miniProgram.envVersion === 'develop'; } catch (_) { return false; }
}
function config() {
 const baseURL = isDevelop()
  ? wx.getStorageSync('cost_base_url') || defaults.developmentBaseURL
  : defaults.baseURL;
 return { baseURL };
}
function clearSession() { wx.removeStorageSync('cost_session'); }
function session() { const value = wx.getStorageSync('cost_session'); return value && value.token && Date.parse(value.expires_at) > Date.now() ? value : null; }
function goLogin() {
 if (redirecting) return;
 const pages = getCurrentPages();
 if (pages.length && pages[pages.length - 1].route === 'pages/login/index') return;
 redirecting = true;
 wx.reLaunch({ url: '/pages/login/index', complete() { redirecting = false; } });
}
const messages = {
 unauthorized: '登录已过期，请重新登录',
 wechat_not_configured: '服务端尚未配置微信登录', wechat_unavailable: '微信登录服务暂不可用，请稍后重试',
 invalid_wechat_code: '登录凭证已失效，请重新点击微信登录',
 invalid_json: '请求格式有误，请重试', invalid_entry: '记账内容有误，请检查后重试',
 not_found: '内容不存在或已被删除', request_conflict: '操作冲突，请刷新后重试',
 invalid_range: '日期范围无效，请重新选择', invalid_budget: '预算参数无效，请检查后重试',
 entry_permission_denied: '只能修改或删除自己记录的账单', export_too_large: '导出数据过多，请缩小日期或筛选范围', transfer_permission_denied: '只能删除自己记录的转账', category_disabled: '该分类已停用，请重新选择', account_disabled: '该账户已停用，请重新选择',
 too_many_login_attempts: '操作太频繁，请稍后重试', authentication_unavailable: '登录验证暂不可用，请稍后重试'
};
function request(path, method = 'GET', data, publicRequest = false, options = {}) {
 const current = session();
 if (!publicRequest && !current) { clearSession(); goLogin(); return Promise.reject(new Error('请先微信登录')); }
 const header = { 'Content-Type': 'application/json' };
 if (current && !publicRequest) header.Authorization = 'Bearer ' + current.token;
 return new Promise((resolve, reject) => wx.request({
  url: config().baseURL + path, method, data, header, timeout: options.timeout || 12000, dataType: options.dataType || 'json',
  success(res) {
   if (res.statusCode >= 200 && res.statusCode < 300) { resolve(res.data); return; }
   let body = res.data;
   if (typeof body === 'string') { try { body = JSON.parse(body); } catch (_) { body = {}; } }
   const code = body && body.error;
   if (!publicRequest && res.statusCode === 401) { clearSession(); goLogin(); }
   reject(new Error(messages[code] || code || '请求失败'));
  },
  fail() { reject(new Error('无法连接服务，请检查网络和后端地址')); }
 }));
}
async function login() {
 const result = await new Promise((resolve, reject) => wx.login({ success: resolve, fail: reject }));
 if (!result.code) throw new Error('未获取到微信登录凭证');
 const value = await request('/api/v1/auth/wechat', 'POST', { code: result.code }, true);
 wx.setStorageSync('cost_session', value);
 return value.user;
}
async function me() { const user = await request('/api/v1/auth/me'); const s = session(); if (s) wx.setStorageSync('cost_session', { ...s, user }); return user; }
async function logout() { if (session()) await request('/api/v1/auth/logout', 'POST'); clearSession(); goLogin(); }
async function exportCSV(query) {
 if (typeof wx.shareFileMessage !== 'function') throw new Error('请升级微信后使用 CSV 导出分享');
 const csv = await request('/api/v1/entries/export?' + query, 'GET', undefined, false, {dataType: 'text', timeout: 60000});
 if (typeof csv !== 'string') throw new Error('导出内容格式异常，请重试');
 const fs = wx.getFileSystemManager();
 const filePath = wx.env.USER_DATA_PATH + '/cost-ledger-export.csv';
 try {
  await new Promise((resolve, reject) => fs.writeFile({filePath, data: csv, encoding: 'utf8', success: resolve, fail: () => reject(new Error('保存导出文件失败，请检查剩余空间'))}));
  await new Promise((resolve, reject) => wx.shareFileMessage({filePath, fileName: 'Cost账单.csv', success: resolve, fail: e => reject(new Error(/cancel/.test(e.errMsg || '') ? '已取消导出分享' : '文件分享失败，请重试'))}));
 } finally {
  await new Promise(resolve => fs.unlink({filePath, success: resolve, fail: resolve}));
 }
}
module.exports = { exportCSV, request, config, session, clearSession, login, logout, me, isDevelop, goLogin };
