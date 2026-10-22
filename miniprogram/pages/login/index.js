const api = require('../../utils/api');
Page({
 data: { busy: false, user: null, error: '', invite: '', secret: '', setup: false, baseURL: '', develop: false },
 async onShow() {
  this.setData({ develop: api.isDevelop(), baseURL: api.config().baseURL });
  if (api.session()) {
   try { const user = await api.me(); this.setData({ user }); this.enter(user); }
   catch (e) { this.setData({ error: e.message, user: null }); }
  }
 },
 input(e) { this.setData({ [e.currentTarget.dataset.field]: e.detail.value }); },
 enter(user) { if (user.role) wx.switchTab({ url: '/pages/ledger/index' }); },
 async signIn() {
  if (this.data.busy) return;
  this.setData({ busy: true, error: '' });
  try { const user = await api.login(); this.setData({ user }); this.enter(user); }
  catch (e) { this.setData({ error: e.message || '微信登录失败' }); }
  finally { this.setData({ busy: false }); }
 },
 async join() { await this.enroll('/api/v1/family/join', { token: this.data.invite.trim() }); },
 async bootstrap() { await this.enroll('/api/v1/family/bootstrap', { secret: this.data.secret }); },
 async enroll(path, body) {
  if (this.data.busy) return;
  this.setData({ busy: true, error: '' });
  try { await api.request(path, 'POST', body); this.setData({ invite: '', secret: '' }); const user = await api.me(); this.setData({ user }); this.enter(user); }
  catch (e) { this.setData({ error: e.message }); }
  finally { this.setData({ busy: false }); }
 },
 toggleSetup() { this.setData({ setup: !this.data.setup, secret: '' }); },
 async signOut() { try { await api.logout(); this.setData({ user: null, invite: '', secret: '', error: '' }); } catch (e) { this.setData({ error: e.message }); } },
 saveURL() {
  if (!api.isDevelop()) return;
  const value = this.data.baseURL.trim().replace(/\/+$/, '');
  if (!/^https?:\/\/[a-zA-Z0-9.-]+(?::\d+)?$/.test(value)) { this.setData({ error: '请输入正确的后端地址' }); return; }
  if (value !== api.config().baseURL) api.clearSession();
  wx.setStorageSync('cost_base_url', value);
  this.setData({ user: null, error: '' }); wx.showToast({ title: '调试地址已保存' });
 }
});
