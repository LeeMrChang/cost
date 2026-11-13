const api = require('../../utils/api');
const nickname = require('../../utils/nickname');
const profile = require('../../utils/profile');
Page({
 data: { busy: false, user: null, nickname: '', avatarSrc: '', error: '', baseURL: '', develop: false },
 async onShow() {
  this.setData({ develop: api.isDevelop(), baseURL: api.config().baseURL });
  if (api.session()) {
   try { const user = await api.me(); this.setData({ ...profile.avatarView(user, api), nickname: user.nickname || '' }); if (user.nickname) this.enter(); }
   catch (e) { this.setData({ error: e.message, user: null, avatarSrc: '' }); }
  }
 },
 input(e) { this.setData({ [e.currentTarget.dataset.field]: e.detail.value }); },
 chooseAvatar(event) { return profile.chooseAvatar(this, event, api); },
 enter() { wx.reLaunch({ url: '/pages/ledger/index' }); },
 async signIn(event) {
  if (this.data.busy) return;
  let value;
  const previousUser = this.data.user;
  if (previousUser) {
   try { value = nickname.fromForm(event); }
   catch (e) { this.setData({ error: e.message }); return; }
  }
  this.setData({ busy: true, error: '', nickname: value || '' });
  try {
   let user, savingProfile = false;
   if (previousUser && api.session()) {
    savingProfile = true;
    user = await api.updateNickname(value);
   } else {
    user = await api.login();
    const sameAccount = previousUser && user.id === previousUser.id;
    // Only reuse a draft after WeChat has confirmed the same account again.
    if (!sameAccount) value = '';
    this.setData({ ...profile.avatarView(user, api), nickname: user.nickname || value || '' });
    if (sameAccount && !user.nickname) {
     savingProfile = true;
     user = await api.updateNickname(value);
    }
   }
   this.setData({ ...profile.avatarView(user, api), nickname: user.nickname || value || '' });
   if (user.nickname) this.enter();
   else if (savingProfile) this.setData({ error: '昵称未保存，请重试' });
  }
  catch (e) { this.setData({ error: e.message || '微信登录失败' }); }
  finally { this.setData({ busy: false }); }
 },
 async signOut() { try { await api.logout(); this.setData({ user: null, nickname: '', avatarSrc: '', error: '' }); } catch (e) { this.setData({ error: e.message }); } },
 saveURL() {
  if (!api.isDevelop()) return;
  const value = this.data.baseURL.trim().replace(/\/+$/, '');
  if (!/^https?:\/\/[a-zA-Z0-9.-]+(?::\d+)?$/.test(value)) { this.setData({ error: '请输入正确的后端地址' }); return; }
  if (value !== api.config().baseURL) api.clearSession();
  wx.setStorageSync('cost_devtools_base_url', value);
  this.setData({ user: null, nickname: '', avatarSrc: '', error: '' }); wx.showToast({ title: '调试地址已保存' });
 }
});
