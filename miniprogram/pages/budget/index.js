const api = require('../../utils/api');
const { parseMonth, formatMonth, shiftMonth } = require('../../utils/calendar');
const { money, displayMoney, today, parseCent, acceptAmountInput } = require('../../utils/money');
Page({
 data: {
  period: '', label: '', monthOpen: false,
  totalCent: null, totalDisplay: '', spent: '0.00', remaining: '', daily: '', over: false, progress: 0,
  totalEditor: false, totalInput: '',
  rows: [], editingID: 0, editInput: '', saving: false, error: '', loading: false,
 },
 onLoad(options) { this.setData({ period: options.period || today().slice(0, 7) }); },
 onShow() { this.load(); },
 openMonth() { this.setData({ monthOpen: true }); },
 closeMonth() { this.setData({ monthOpen: false }); },
 onMonthConfirm(e) { this.setData({ monthOpen: false, period: e.detail.month }); this.load(); },
 prev() { this.step(-1); },
 next() { this.step(1); },
 step(delta) {
  const { year, month } = parseMonth(this.data.period);
  const s = shiftMonth(year, month, delta);
  this.setData({ period: formatMonth(s.year, s.month) }); this.load();
 },
 editTotal() { if (this.data.saving) return; this.setData({ totalEditor: true, totalInput: this.data.totalCent === null ? '' : money(this.data.totalCent), editingID: 0 }); },
 editRow(e) {
  if (this.data.saving) return;
  const id = Number(e.currentTarget.dataset.id);
  const row = this.data.rows.find(r => r.id === id); if (!row) return;
  this.setData({ totalEditor: false, editingID: id, editInput: row.set ? row.amount : '' });
 },
 closeEditor() { if (!this.data.saving) this.setData({ totalEditor: false, editingID: 0 }); },
 input(e) {
  const field = e.currentTarget.dataset.field;
  const value = acceptAmountInput(e.detail.value, this.data[field]);
  this.setData({ [field]: value });
  return value;
 },
 async save() {
  if (this.data.saving) return;
  const total = this.data.totalEditor;
  try {
   const amount = parseCent(this.data[total ? 'totalInput' : 'editInput']);
   this.setData({ saving: true });
   await api.request('/api/v1/budgets', 'PUT', { scope: 'month', period: this.data.period, category_id: total ? 0 : this.data.editingID, amount_cent: amount });
   this.setData({ totalEditor: false, editingID: 0 });
   wx.showToast({ title: '预算已保存' });
   await this.load();
  } catch (e) { wx.showToast({ title: e.message, icon: 'none' }); }
  finally { this.setData({ saving: false }); }
 },
 async clear() {
  if (this.data.saving) return;
  const total = this.data.totalEditor;
  try {
   this.setData({ saving: true });
   await api.request('/api/v1/budgets', 'PUT', { scope: 'month', period: this.data.period, category_id: total ? 0 : this.data.editingID, amount_cent: 0 });
   this.setData({ totalEditor: false, editingID: 0 });
   wx.showToast({ title: '预算已清除' });
   await this.load();
  } catch (e) { wx.showToast({ title: e.message, icon: 'none' }); }
  finally { this.setData({ saving: false }); }
 },
 async load() {
  const version = this.version = (this.version || 0) + 1;
  this.setData({ loading: true, error: '', totalEditor: false, editingID: 0 });
  try {
   const [b, s, cats] = await Promise.all([
    api.request('/api/v1/budgets?scope=month&period=' + this.data.period),
    api.request('/api/v1/stats?range=month&ref=' + this.data.period + '-01'),
    api.request('/api/v1/categories'),
   ]);
   if (version !== this.version) return;
   const { year, month } = parseMonth(this.data.period);
   const rows = cats.filter(c => c.kind === 'expense').sort((a, b) => a.sort_order - b.sort_order || a.id - b.id).map(c => {
    const row = (b.categories || []).find(r => r.category_id === c.id);
    return { id: c.id, name: c.name, icon: c.icon, icon_key: c.icon_key || '', set: !!row && row.is_set !== false, amount: row && row.is_set !== false ? money(row.amount_cent) : '', amountDisplay: row && row.is_set !== false ? displayMoney(row.amount_cent) : '', spent: row && row.spent_cent ? displayMoney(row.spent_cent) : '', progress: row && row.amount_cent>0?Math.min(100,Math.round(row.spent_cent/row.amount_cent*100)):0, warn:!!row && row.amount_cent>0 && row.spent_cent/row.amount_cent>0.8, overSpent: !!row && row.is_set!==false && row.spent_cent > row.amount_cent };
   });
   const data = { label: year + '年' + month + '月', rows, totalCent: b.total_cent === null || b.total_cent === undefined ? null : b.total_cent };
   if (data.totalCent !== null) {
    const remaining = data.totalCent - s.summary.expense_cent;
    data.totalDisplay = displayMoney(data.totalCent);
    data.spent = displayMoney(s.summary.expense_cent);
    data.remaining = displayMoney(Math.abs(remaining));
    data.over = remaining < 0;
    data.warn = data.totalCent>0 && s.summary.expense_cent/data.totalCent>0.8;
    data.progress = data.totalCent > 0 ? Math.min(100, Math.round(s.summary.expense_cent / data.totalCent * 100)) : 0;
    data.daily = s.budget && s.budget.remaining_daily_cent !== null && s.budget.remaining_daily_cent !== undefined ? displayMoney(s.budget.remaining_daily_cent) : '';
   }
   this.setData(data);
  } catch (e) { if (version === this.version) this.setData({ error: e.message }); }
  finally { if (version === this.version) this.setData({ loading: false }); }
 },
});
