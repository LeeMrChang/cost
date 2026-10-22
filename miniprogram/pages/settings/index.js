const api = require('../../utils/api');
const { parseCent, today } = require('../../utils/money');
Page({
 data: { user: null, members: [], invite: '', expires: '', budget: '', month: today().slice(0,7), error: '', busy: false },
 onShow() { this.load(); },
 input(e) { this.setData({ [e.currentTarget.dataset.field]: e.detail.value }); },
 async load() {
  // Clear data from a previously logged-in account before fetching new permissions.
  this.setData({ user: null, members: [], invite: '', expires: '', error: '' });
  try { const user = await api.me(); this.setData({ user }); if (user.role === 'admin') this.setData({ members: await api.request('/api/v1/family/members') }); }
  catch (e) { this.setData({ error: e.message }); }
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
 async signOut() { try { await api.logout(); this.setData({ user: null, members: [], invite: '', budget: '' }); } catch (e) { this.setData({ error: e.message }); } },
 async saveBudget() {
  try { const amount = parseCent(this.data.budget); await api.request('/api/v1/budgets/' + this.data.month, 'PUT', { amount_cent: amount }); wx.showToast({ title: '预算已保存' }); }
  catch (e) { wx.showToast({ title: e.message, icon: 'none' }); }
 }
});
