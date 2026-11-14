const api = require('../../utils/api');
const guestStore = require('../../utils/guest-store');
const guestCategories = require('../../utils/guest-categories');
const { displayMoney, today } = require('../../utils/money');
const { stamp } = require('../../utils/ledger');

function isLoggedIn() {
 if (typeof api.isAuthenticated === 'function') return api.isAuthenticated();
 if (typeof api.session === 'function') return Boolean(api.session());
 return true;
}
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
  // 游客态只有本地当月数据，其他区间需要登录后由服务端统计。
  if (!isLoggedIn() && range !== 'month') {
   wx.showToast({ title: '登录后可查看周/年/自定义统计', icon: 'none' });
   return;
  }
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
  if (!isLoggedIn()) return this.loadGuest();
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
   this.applyStats(s, categories);
  } catch (e) { if (version === this.version) this.setData({ error: e.message }); }
  finally { if (version === this.version) this.setData({ loading: false }); }
 },
 // 游客态：只统计本机当月记录，字段形状与服务端一致，复用同一套渲染。
 loadGuest() {
  const month = (this.data.ref || today()).slice(0, 7);
  const categories = guestCategories.list();
  const catMap = {};
  categories.forEach(c => { catMap[c.id] = c; });
  const days = {};
  const [year, mon] = month.split('-').map(Number);
  const dayCount = new Date(year, mon, 0).getDate();
  for (let d = 1; d <= dayCount; d++) {
   const key = month + '-' + String(d).padStart(2, '0');
   days[key] = { key, expense_cent: 0, income_cent: 0 };
  }
  const totals = {};
  let expense = 0, income = 0;
  guestStore.read().forEach(item => {
   const key = stamp(item.occurred_at).date;
   if (!days[key]) return;
   if (item.kind === 'income') { income += item.amount_cent; days[key].income_cent += item.amount_cent; return; }
   // 分类排行只统计支出，与服务端一致。
   const category = guestCategories.findByName(item.category_key, item.kind);
   const id = category ? category.id : 112;
   if (!totals[id]) totals[id] = { category_id: id, amount_cent: 0 };
   expense += item.amount_cent;
   days[key].expense_cent += item.amount_cent;
   totals[id].amount_cent += item.amount_cent;
  });
  const series = Object.values(days);
  const ranked = Object.values(totals).sort((a, b) => b.amount_cent - a.amount_cent);
  this.applyStats({
   label: year + '年' + mon + '月', prev_ref: '', next_ref: '',
   summary: { expense_cent: expense, income_cent: income, balance_cent: income - expense, daily_avg_expense_cent: Math.round(expense / dayCount) },
   series, categories: ranked, budget: null,
  }, categories);
 },
 applyStats(s, categories) {
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
    ...c, name: (catMap[c.category_id] || {}).name || '未分类', icon: (catMap[c.category_id] || {}).icon || '📦', icon_key: (catMap[c.category_id] || {}).icon_key || '',
    amount: displayMoney(c.amount_cent),
    percent: total ? (c.amount_cent / total * 100).toFixed(1) : '0.0',
    width: total ? Math.min(100, c.amount_cent / total * 100) : 0,
   })),
   budget: budgetView,
   loading: false, error: '',
  });
 },
});
