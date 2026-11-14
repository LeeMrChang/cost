const api = require('../../utils/api');
const guestStore = require('../../utils/guest-store');
const nickname = require('../../utils/nickname');
const profile = require('../../utils/profile');
function isLoggedIn() {
 if (typeof api.isAuthenticated === 'function') return api.isAuthenticated();
 if (typeof api.session === 'function') return Boolean(api.session());
 return true;
}

Page({
 data: {
  user: null, nickname: '', avatarSrc: '', ledger: null, error: '', busy: false, guest: false, guestPending: 0,
  editingLedger: false, ledgerDraftName: '', ledgerDraftIcon: '', ledgerError: '',
  ledgerIcons: ['📒', '🏠', '✈️', '🍜', '💰', '🎯', '🎓', '🎁']
 },
 onShow() {
  if (!isLoggedIn()) {
   this.setData({ guest: true, guestPending: guestStore.pending(), user: null, ledger: null, nickname: '', avatarSrc: '', error: '' });
   return;
  }
  this.setData({ guest: false });
  this.load();
 },
 goLogin() { wx.navigateTo({ url: '/pages/login/index' }); },
 openCategories() { this.openProtected('/pages/categories/index'); },
 openAccounts() { this.openProtected('/pages/accounts/index'); },
 openBudget() { this.openProtected('/pages/budget/index'); },
 openProtected(url) {
  if (!isLoggedIn()) { wx.showToast({ title: '登录后可用', icon: 'none' }); return; }
  wx.navigateTo({ url });
 },
 async load() {
  // Clear data from a previously logged-in account before fetching new permissions.
  this.setData({ user: null, nickname: '', avatarSrc: '', ledger: null, error: '', editingLedger: false, ledgerDraftName: '', ledgerDraftIcon: '', ledgerError: '' });
  try {
   const [user, ledger] = await Promise.all([api.me(), api.request('/api/v1/ledgers/me')]);
   this.setData({ ...profile.avatarView(user, api), nickname: user.nickname || '', ledger });
  } catch (e) { this.setData({ error: e.message }); }
 },
 chooseAvatar(event) { return profile.chooseAvatar(this, event, api); },
 inputNickname(event) { this.setData({ nickname: event.detail.value }); },
 editLedger() {
  if (!this.data.ledger || this.data.busy || this.data.editingLedger) return;
  this.setData({ editingLedger: true, ledgerDraftName: this.data.ledger.name, ledgerDraftIcon: this.data.ledger.icon, ledgerError: '' });
 },
 inputLedgerName(event) { this.setData({ ledgerDraftName: event.detail.value }); },
 chooseLedgerIcon(event) {
  const icon = event.currentTarget.dataset.icon;
  if (!this.data.busy && this.data.ledgerIcons.includes(icon)) this.setData({ ledgerDraftIcon: icon });
 },
 cancelLedgerEdit() {
  if (this.data.busy) return;
  this.setData({ editingLedger: false, ledgerDraftName: '', ledgerDraftIcon: '', ledgerError: '' });
 },
 async saveLedger(event) {
  if (this.data.busy || !this.data.editingLedger) return;
  const raw = event && event.detail && event.detail.value && event.detail.value.name;
  const name = typeof raw === 'string' ? raw.trim() : '';
  if (!name || Array.from(name).length > 64 || /[\u0000-\u001f\u007f-\u009f\u2028\u2029]/.test(raw)) {
   this.setData({ ledgerError: '请输入 1–64 个字的账本名称，不能包含换行或控制字符' }); return;
  }
  this.setData({ busy: true, ledgerDraftName: name, ledgerError: '' });
  try {
   const ledger = await api.request('/api/v1/ledgers/me', 'PATCH', { name, icon: this.data.ledgerDraftIcon });
   this.setData({ ledger, editingLedger: false, ledgerDraftName: '', ledgerDraftIcon: '' });
   wx.showToast({ title: '账本已更新' });
  } catch (e) { this.setData({ ledgerError: e.message }); }
  finally { this.setData({ busy: false }); }
 },
 async saveNickname(event) {
  if (this.data.busy) return;
  let value;
  try { value = nickname.fromForm(event); }
  catch (e) { this.setData({ error: e.message }); return; }
  this.setData({ busy: true, error: '', nickname: value });
  try {
   const user = await api.updateNickname(value);
   this.setData({ ...profile.avatarView(user, api), nickname: user.nickname });
   wx.showToast({ title: '昵称已保存' });
  } catch (e) { this.setData({ error: e.message }); }
  finally { this.setData({ busy: false }); }
 },
 async signOut() { try { await api.logout(); this.setData({ user: null, nickname: '', avatarSrc: '', ledger: null }); } catch (e) { this.setData({ error: e.message }); } }
});
