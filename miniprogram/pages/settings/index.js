const {memberLabel, nicknameValue} = require('../../utils/members');
const api = require('../../utils/api');
const { parseCent, today, acceptAmountInput } = require('../../utils/money');
Page({
 data: { nicknameEditor: false, nicknameSaving: false, nickname: '', nicknameError: '', nicknameID: null, user: null, members: [], invite: '', expires: '', budget: '', month: today().slice(0,7), monthOpen: false, error: '', busy: false },
 onShow() { this.load(); },
 onHide() { this.version = (this.version || 0) + 1; this.setData({nicknameEditor: false, nickname: '', nicknameError: '', nicknameSaving: false}); },
 onUnload() { this.onHide(); },
 openCategories() { wx.navigateTo({url: '/pages/categories/index'}); },
 openAccounts() { wx.navigateTo({url: '/pages/accounts/index'}); },
 openExport() { wx.switchTab({url: '/pages/ledger/index'}); wx.showToast({title: '在账本页筛选后点击导出 CSV', icon: 'none'}); },
 openMonth() { this.setData({ monthOpen: true }); },
 closeMonth() { this.setData({ monthOpen: false }); },
 onMonthConfirm(e) { this.setData({ monthOpen: false, month: e.detail.month }); },
 input(e) {
  const field = e.currentTarget.dataset.field;
  const value = field === 'budget' ? acceptAmountInput(e.detail.value, this.data.budget) : e.detail.value;
  this.setData({ [field]: value });
  return value;
 },
 async load() {
  const version = this.version = (this.version || 0) + 1;
  this.setData({ user: null, members: [], invite: '', expires: '', error: '', nicknameEditor: false, nickname: '', nicknameID: null, nicknameError: '', nicknameSaving: false });
  try {
   const user = await api.me();
   if (version !== this.version) return;
   if (!user || !['admin', 'member'].includes(user.role)) throw new Error('请先加入家庭账本');
   const members = user.role === 'admin' ? await api.request('/api/v1/family/members') : [];
   if (version !== this.version) return;
   this.setData({user: {...user, label: memberLabel(user)}, members: members.map(m => ({...m, label: memberLabel(m)}))});
  } catch (e) { if (version === this.version) this.setData({error: e.message}); }
 },
 editNickname(e) {
  if (this.data.nicknameSaving || !this.data.user) return;
  const id = Number(e.currentTarget.dataset.id), user = this.data.user;
  if (user.role !== 'admin' && id !== user.id) return;
  const member = id === user.id ? user : this.data.members.find(m => m.id === id);
  if (!member) return;
  this.setData({nicknameEditor: true, nicknameID: id, nickname: member.nickname || '', nicknameError: ''});
 },
 nicknameInput(e) { if (!this.data.nicknameSaving) this.setData({nickname: e.detail.value, nicknameError: ''}); },
 clearNickname() { if (!this.data.nicknameSaving) this.setData({nickname: '', nicknameError: ''}); },
 closeNickname() { if (!this.data.nicknameSaving) this.setData({nicknameEditor: false, nickname: '', nicknameID: null, nicknameError: ''}); },
 noop() {},
 async saveNickname() {
  if (this.data.nicknameSaving || !this.data.user || !this.data.nicknameEditor) return;
  const user = this.data.user, id = this.data.nicknameID, version = this.version;
  if (user.role !== 'admin' && user.id !== id) return;
  try {
   const nickname = nicknameValue(this.data.nickname);
   this.setData({nicknameSaving: true, nicknameError: ''});
   await api.request('/api/v1/family/members/' + id + '/nickname', 'PATCH', {nickname});
   if (version !== this.version) return;
   this.setData({nicknameEditor: false, nickname: ''});
   wx.showToast({title: '昵称已保存'});
   await this.load();
  } catch (e) { if (version === this.version) this.setData({nicknameError: e.message}); }
  finally { if (version === this.version) this.setData({nicknameSaving: false}); }
 },
 async generateInvite() {
  if (this.data.busy) return; this.setData({ busy: true, error: '' });
  try { const i = await api.request('/api/v1/family/invites', 'POST'); this.setData({ invite: i.token, expires: i.expires_at }); }
  catch (e) { this.setData({ error: e.message }); } finally { this.setData({ busy: false }); }
 },
 copyInvite() { if (this.data.invite) wx.setClipboardData({ data: this.data.invite }); },
 removeMember(e) {
  const id = e.currentTarget.dataset.id;
  wx.showModal({ title: '移除家庭成员', content: '移除后，该成员将无法访问账本；已有账单会保留。', success: async res => {
   if (!res.confirm) return;
   try { await api.request('/api/v1/family/members/' + id, 'DELETE'); await this.load(); }
   catch (err) { this.setData({ error: err.message }); }
  }});
 },
 async signOut() { try { await api.logout(); this.version = (this.version || 0) + 1; this.setData({ user: null, members: [], invite: '', budget: '', nicknameEditor: false, nickname: '', nicknameID: null, nicknameError: '', nicknameSaving: false }); } catch (e) { this.setData({ error: e.message }); } },
 async saveBudget() {
  try { const amount = parseCent(this.data.budget); await api.request('/api/v1/budgets/' + this.data.month, 'PUT', { amount_cent: amount }); wx.showToast({ title: '预算已保存' }); }
  catch (e) { wx.showToast({ title: e.message, icon: 'none' }); }
 }
});
