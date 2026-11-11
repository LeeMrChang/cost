const api = require('../../utils/api');
const { displayMoney, today } = require('../../utils/money');
const RANGES = ['week', 'month', 'year', 'all', 'custom'];
function chartLabel(key) {
 if (key.length === 4) return key;
 if (key.length === 7) return key.slice(5);
 return key.slice(8);
}
Page({
 data: {
  statusBarHeight: 20, navRight: 0,
  range: 'month', label: '', ref: today(), prevRef: '', nextRef: '',
  summary: {}, chart: [], categories: [], budget: null, dense: false,
  error: '', loading: false, showValues: false,
  customStart: '', customEnd: '', startOpen: false, endOpen: false,
 },
 onLoad() {
  const data = {};
  const info = wx.getWindowInfo ? wx.getWindowInfo() : null;
  if (info && info.statusBarHeight) data.statusBarHeight = info.statusBarHeight;
  if (info && info.windowWidth && wx.getMenuButtonBoundingClientRect) {
   const rect = wx.getMenuButtonBoundingClientRect();
   if (rect && rect.left > 0) data.navRight = info.windowWidth - rect.left + 8;
  }
  this.setData(data);
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
   const [s, categories] = await Promise.all([
    api.request('/api/v1/stats?' + q.join('&')),
    api.request('/api/v1/categories'),
   ]);
   if (version !== this.version) return;
   const catMap = {};
   categories.forEach(c => { catMap[c.id] = c; });
   const max = Math.max(1, ...s.series.map(p => p.expense_cent));
   const total = s.summary.expense_cent || 0;
   const count = s.series.length;
   const labelStep = count > 14 ? 10 : 1;
   let budgetView = null;
   if (s.budget) {
    if (s.budget.total_cent === null) budgetView = { unset: true };
    else budgetView = {
     over: s.budget.remaining_cent < 0,
     remaining: displayMoney(Math.abs(s.budget.remaining_cent)),
     total: displayMoney(s.budget.total_cent),
     daily: s.budget.remaining_daily_cent === null ? '' : displayMoney(s.budget.remaining_daily_cent),
    };
   }
   this.setData({
    label: s.label, prevRef: s.prev_ref || '', nextRef: s.next_ref || '',
    dense: count > 14,
    showValues: this.data.range !== 'month' && this.data.range !== 'custom' && count <= 7,
    summary: {
     expense: displayMoney(s.summary.expense_cent), income: displayMoney(s.summary.income_cent),
     balance: displayMoney(s.summary.balance_cent), daily: displayMoney(s.summary.daily_avg_expense_cent),
    },
    chart: s.series.map((p, i) => ({
     key: p.key, label: chartLabel(p.key),
     showLabel: labelStep === 1 || i % labelStep === 0 || i === count - 1,
     value: displayMoney(p.expense_cent),
     height: Math.round(p.expense_cent / max * 100), hot: p.expense_cent > 0 && p.expense_cent === max,
    })),
    categories: s.categories.map(c => ({
     ...c, name: catMap[c.category_id].name, icon: catMap[c.category_id].icon, icon_key: catMap[c.category_id].icon_key || '',
     amount: displayMoney(c.amount_cent),
     percent: total ? (c.amount_cent / total * 100).toFixed(1) : '0.0',
     width: total ? Math.min(100, c.amount_cent / total * 100) : 0,
    })),
    budget: budgetView,
   });
  } catch (e) { if (version === this.version) this.setData({ error: e.message }); }
  finally { if (version === this.version) this.setData({ loading: false }); }
 },
});
