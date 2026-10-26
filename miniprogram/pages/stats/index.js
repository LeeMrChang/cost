const api = require('../../utils/api');
const { money, today } = require('../../utils/money');
const RANGES = ['week', 'month', 'year', 'all', 'custom'];
function chartLabel(key) {
 if (key.length === 4) return key;
 if (key.length === 7) return key.slice(5);
 return key.slice(8);
}
Page({
 data: {
  range: 'month', label: '', ref: today(), prevRef: '', nextRef: '',
  summary: {}, chart: [], categories: [], budget: null,
  error: '', loading: false, showValues: false,
  customStart: '', customEnd: '', startOpen: false, endOpen: false,
 },
 onShow() { this.load(); },
 setRange(e) {
  const range = e.currentTarget.dataset.range;
  if (range === this.data.range) return;
  const data = { range };
  if (range === 'custom' && !this.data.customStart) {
   const now = today();
   data.customStart = now.slice(0, 7) + '-01';
   data.customEnd = now;
  }
  this.setData(data); this.load();
 },
 prev() { if (!this.data.prevRef) return; this.setData({ ref: this.data.prevRef }); this.load(); },
 next() { if (!this.data.nextRef) return; this.setData({ ref: this.data.nextRef }); this.load(); },
 openStart() { this.setData({ startOpen: true }); },
 openEnd() { this.setData({ endOpen: true }); },
 closeStart() { this.setData({ startOpen: false }); },
 closeEnd() { this.setData({ endOpen: false }); },
 onStart(e) { this.setData({ startOpen: false, customStart: e.detail.date }); this.load(); },
 onEnd(e) { this.setData({ endOpen: false, customEnd: e.detail.date }); this.load(); },
 openBudget() {
  const period = this.data.range === 'month' ? this.data.ref.slice(0, 7) : today().slice(0, 7);
  wx.navigateTo({ url: '/pages/budget/index?period=' + period });
 },
 async load() {
  const version = this.version = (this.version || 0) + 1;
  this.setData({ loading: true, error: '' });
  try {
   const q = ['range=' + this.data.range];
   if (this.data.range === 'custom') q.push('start_date=' + this.data.customStart, 'end_date=' + this.data.customEnd);
   else q.push('ref=' + this.data.ref);
   const s = await api.request('/api/v1/stats?' + q.join('&'));
   if (version !== this.version) return;
   const max = Math.max(1, ...s.series.map(p => p.expense_cent));
   const total = s.summary.expense_cent || 0;
   let budgetView = null;
   if (s.budget) {
    if (s.budget.total_cent === null) budgetView = { unset: true };
    else budgetView = {
     over: s.budget.remaining_cent < 0,
     remaining: money(Math.abs(s.budget.remaining_cent)),
     total: money(s.budget.total_cent),
     daily: s.budget.remaining_daily_cent === null ? '' : money(s.budget.remaining_daily_cent),
    };
   }
   this.setData({
    label: s.label, prevRef: s.prev_ref || '', nextRef: s.next_ref || '',
    showValues: this.data.range !== 'month' && this.data.range !== 'custom',
    summary: {
     expense: money(s.summary.expense_cent), income: money(s.summary.income_cent),
     balance: money(s.summary.balance_cent), daily: money(s.summary.daily_avg_expense_cent),
    },
    chart: s.series.map(p => ({
     key: p.key, label: chartLabel(p.key), value: money(p.expense_cent),
     height: Math.round(p.expense_cent / max * 100), hot: p.expense_cent > 0 && p.expense_cent === max,
    })),
    categories: s.categories.map(c => ({
     ...c, amount: money(c.amount_cent),
     percent: total ? (c.amount_cent / total * 100).toFixed(1) : '0.0',
     width: total ? Math.min(100, c.amount_cent / total * 100) : 0,
    })),
    budget: budgetView,
   });
  } catch (e) { if (version === this.version) this.setData({ error: e.message }); }
  finally { if (version === this.version) this.setData({ loading: false }); }
 },
});
