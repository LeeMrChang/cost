const { request, session } = require('../../utils/api');
const { parseCent, money, today } = require('../../utils/money');
Page({
 data: { month: today().slice(0, 7), groups: [], categories: [], visibleCategories: [], summary: {}, error: '', loading: false, editor: false, kind: 'expense', categoryID: 2, amount: '', note: '', date: today(), time: '12:00', editID: null, saving: false },
 onShow() { this.setData({ groups: [], summary: {}, editor: false, note: '', amount: '' }); this.load(); },
 async load() {
  this.setData({ loading: true, error: '' });
  const month = this.data.month;
  try {
   const [categories, response, s] = await Promise.all([request('/api/v1/categories'), request('/api/v1/entries?month=' + month), request('/api/v1/stats?month=' + month)]);
   if (this.data.month !== month) return;
   const map = {}; categories.forEach(c => { map[c.id] = c; });
   const grouped = {};
   response.items.forEach(e => {
    // Format ledger times in Asia/Shanghai regardless of phone timezone.
    const d = new Date(new Date(e.occurred_at).getTime() + 8 * 3600000).toISOString();
    const date = d.slice(0, 10);
    if (!grouped[date]) grouped[date] = { date, expense: 0, items: [] };
    if (e.kind === 'expense') grouped[date].expense += e.amount_cent;
    grouped[date].items.push({ ...e, category: map[e.category_id], date, time: d.slice(11, 16), display: money(e.amount_cent) });
   });
   this.setData({ categories, visibleCategories: categories.filter(c => c.kind === this.data.kind), groups: Object.keys(grouped).sort().reverse().map(k => ({ ...grouped[k], total: money(grouped[k].expense) })), summary: { expense: money(s.expense_cent), income: money(s.income_cent), budget: s.budget_cent === null ? '未设置' : '¥' + money(s.budget_cent), remaining: s.remaining_cent === null ? '未设置' : '¥' + money(s.remaining_cent) } });
  } catch (e) { this.setData({ error: e.message }); } finally { this.setData({ loading: false }); }
 },
 monthChange(e) { this.setData({ month: e.detail.value }); this.load(); },
 open() { const now = new Date(); this.requestID = 'wx-' + Date.now() + '-' + Math.random().toString(16).slice(2); this.setData({ editor: true, editID: null, kind: 'expense', visibleCategories: this.data.categories.filter(c => c.kind === 'expense'), categoryID: 2, amount: '', note: '', date: today(), time: String(now.getHours()).padStart(2,'0') + ':' + String(now.getMinutes()).padStart(2,'0') }); },
 edit(e) { const id = Number(e.currentTarget.dataset.id); const item = this.data.groups.flatMap(g => g.items).find(x => x.id === id); if (!item) return; const current = session(); if (!current || (current.user.role !== 'admin' && current.user.id !== item.created_by)) { wx.showToast({ title: '只能修改自己的账单', icon: 'none' }); return; } this.setData({ editor: true, editID: id, kind: item.kind, visibleCategories: this.data.categories.filter(c => c.kind === item.kind), categoryID: item.category_id, amount: item.display, note: item.note, date: item.date, time: item.time }); },
 close() { if (!this.data.saving) this.setData({ editor: false }); },
 noop() {},
 kindChange(e) { const kind = e.currentTarget.dataset.kind; const visible = this.data.categories.filter(c => c.kind === kind); this.setData({ kind, visibleCategories: visible, categoryID: visible.length ? visible[0].id : 0 }); },
 categoryChange(e) { this.setData({ categoryID: Number(e.currentTarget.dataset.id) }); },
 input(e) { this.setData({ [e.currentTarget.dataset.field]: e.detail.value }); },
 async save() {
  if (this.data.saving) return;
  try {
   const amount = parseCent(this.data.amount); if (amount <= 0) throw new Error('金额必须大于零');
   const data = { category_id: this.data.categoryID, kind: this.data.kind, amount_cent: amount, occurred_at: this.data.date + 'T' + this.data.time + ':00+08:00', note: this.data.note, client_request_id: this.requestID || 'edit' };
   this.setData({ saving: true });
   await request('/api/v1/entries' + (this.data.editID ? '/' + this.data.editID : ''), this.data.editID ? 'PATCH' : 'POST', data);
   this.setData({ editor: false }); wx.showToast({ title: '已保存' }); await this.load();
  } catch (e) { wx.showToast({ title: e.message, icon: 'none' }); } finally { this.setData({ saving: false }); }
 },
 remove() {
  if (this.data.saving) return;
  wx.showModal({ title: '删除账单', content: '确定删除这笔账单？', success: async res => {
   if (!res.confirm) return;
   this.setData({saving:true});
   try { await request('/api/v1/entries/' + this.data.editID, 'DELETE'); this.setData({ editor: false }); await this.load(); } catch(e) { wx.showToast({ title:e.message, icon:'none' }); } finally { this.setData({saving:false}); }
  }});
 }
});
