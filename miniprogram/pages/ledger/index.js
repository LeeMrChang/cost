const api = require('../../utils/api');
const { parseCent, money, acceptAmountInput } = require('../../utils/money');
const { emptyFilters, query, stamp, allowed, groups, requestID } = require('../../utils/ledger');
Page({
 data: {month: stamp().date.slice(0, 7), groups: [], entries: [], categories: [], accounts: [], authors: [], visibleCategories: [], visibleAccounts: [], summary: {}, error: '', loading: false, loadingMore: false, total: 0, page: 0, hasMore: false, filters: emptyFilters(), draft: emptyFilters(), search: '', filtered: false, filterOpen: false, filterError: '', exporting: false, editor: false, kind: 'expense', categoryID: 0, accountID: 0, accountIndex: 0, amount: '', note: '', date: stamp().date, time: '12:00', editID: null, saving: false, dateOpen: false, monthOpen: false, filterCategories: [], filterAccounts: [], filterAuthors: [], categoryFilterIndex: 0, accountFilterIndex: 0, authorFilterIndex: 0, kindFilterIndex: 0, kinds: [{label: '全部类型', value: ''}, {label: '支出', value: 'expense'}, {label: '收入', value: 'income'}]},
 onShow() { this.setData({groups: [], entries: [], summary: {}, editor: false, amount: '', note: ''}); this.load(); },
 onHide() { this.loadVersion = (this.loadVersion || 0) + 1; },
 onUnload() { this.onHide(); },
 onReachBottom() { this.loadMore(); },
 async load() {
  const version = this.loadVersion = (this.loadVersion || 0) + 1;
  this.setData({loading: true, loadingMore: false, error: '', entries: [], groups: [], total: 0, page: 0, hasMore: false, summary: {}});
  try {
   const q = query(this.data.filters, this.data.month, 1);
   const filtered = Object.keys(this.data.filters).some(k => this.data.filters[k] !== '');
   const [categories, accounts, authors, response, stats] = await Promise.all([api.request('/api/v1/categories'), api.request('/api/v1/accounts'), api.request('/api/v1/entry-authors'), api.request('/api/v1/entries?' + q), filtered ? Promise.resolve(null) : api.request('/api/v1/stats?month=' + this.data.month).catch(() => null)]);
   if (version !== this.loadVersion) return;
   const s = response.summary, entries = response.items || [];
   this.setData({categories, accounts, authors, entries, groups: groups(entries, categories, accounts), total: response.total, page: 1, hasMore: entries.length < response.total, filtered, summary: {expense: money(s.expense_cent), income: money(s.income_cent), balance: money(s.balance_cent), ...(!filtered ? {budget: !stats ? '读取失败' : stats.budget_cent === null ? '未设置' : '¥' + money(stats.budget_cent), remaining: !stats ? '读取失败' : stats.remaining_cent === null ? '未设置' : '¥' + money(stats.remaining_cent)} : {})}});
  } catch (e) { if (version === this.loadVersion) this.setData({error: e.message}); }
  finally { if (version === this.loadVersion) this.setData({loading: false}); }
 },
 async loadMore() {
  if (this.data.loading || this.data.loadingMore || !this.data.hasMore) return;
  const version = this.loadVersion, next = this.data.page + 1;
  this.setData({loadingMore: true, error: ''});
  try {
   const response = await api.request('/api/v1/entries?' + query(this.data.filters, this.data.month, next));
   if (version !== this.loadVersion) return;
   const map = {}; this.data.entries.concat(response.items || []).forEach(e => {map[e.id] = e;});
   const entries = Object.values(map).sort((a, b) => b.occurred_at.localeCompare(a.occurred_at) || b.id - a.id);
   this.setData({entries, groups: groups(entries, this.data.categories, this.data.accounts), total: response.total, summary: {...this.data.summary, income: money(response.summary.income_cent), expense: money(response.summary.expense_cent), balance: money(response.summary.balance_cent)}, page: next, hasMore: response.items.length > 0 && entries.length < response.total});
  } catch (e) { if (version === this.loadVersion) this.setData({error: e.message}); }
  finally { if (version === this.loadVersion) this.setData({loadingMore: false}); }
 },
 openMonth() { this.setData({monthOpen: true}); },
 closeMonth() { this.setData({monthOpen: false}); },
 onMonthConfirm(e) { this.setData({monthOpen: false, month: e.detail.month, filters: {...this.data.filters, start_date: '', end_date: ''}}); this.load(); },
 searchInput(e) { this.setData({search: e.detail.value}); },
 searchEntries() { this.setData({filters: {...this.data.filters, q: this.data.search.trim()}}); this.load(); },
 openFilters() {
  const draft = {...emptyFilters(), ...this.data.filters};
  const filterCategories = [{id: '', label: '全部分类'}, ...this.data.categories.map(c => ({...c, label: c.name + (c.kind === 'income' ? ' · 收入' : ' · 支出') + (c.disabled ? '（已停用）' : '')}))];
  const filterAccounts = [{id: '', label: '全部账户'}, {id: 0, label: '未指定账户'}, ...this.data.accounts.map(a => ({...a, label: a.name + (a.disabled ? '（已停用）' : '')}))];
  const filterAuthors = [{id: '', label: '全部成员'}, ...this.data.authors.map(a => ({id: a.id, label: a.id ? '成员 ' + a.id : '历史未指定成员'}))];
  const index = (items, value, key = 'id') => Math.max(0, items.findIndex(i => String(i[key]) === String(value)));
  this.setData({filterOpen: true, filterError: '', draft, filterCategories, filterAccounts, filterAuthors, categoryFilterIndex: index(filterCategories, draft.category_id), accountFilterIndex: index(filterAccounts, draft.account_id), authorFilterIndex: index(filterAuthors, draft.created_by), kindFilterIndex: index(this.data.kinds, draft.kind, 'value')});
 },
 closeFilters() { this.setData({filterOpen: false}); },
 filterInput(e) {
  const field = e.currentTarget.dataset.field;
  const value = ['min_amount', 'max_amount'].includes(field) ? acceptAmountInput(e.detail.value, this.data.draft[field]) : e.detail.value;
  this.setData({draft: {...this.data.draft, [field]: value}}); return value;
 },
 filterPick(e) {
  const field = e.currentTarget.dataset.field, i = Number(e.detail.value);
  const source = {kind: ['kinds', 'kindFilterIndex', 'value'], category_id: ['filterCategories', 'categoryFilterIndex', 'id'], account_id: ['filterAccounts', 'accountFilterIndex', 'id'], created_by: ['filterAuthors', 'authorFilterIndex', 'id']}[field];
  this.setData({draft: {...this.data.draft, [field]: this.data[source[0]][i][source[2]]}, [source[1]]: i});
 },
 clearDates() { this.setData({draft: {...this.data.draft, start_date: '', end_date: ''}}); },
 applyFilters() {
  try { query(this.data.draft, this.data.month); this.setData({filters: {...this.data.draft}, search: this.data.draft.q, filterOpen: false}); this.load(); }
  catch (e) { this.setData({filterError: e.message}); }
 },
 resetFilters() { this.setData({filters: emptyFilters(), search: '', filterOpen: false}); this.load(); },
 async exportEntries() {
  if (this.data.exporting) return;
  this.setData({exporting: true});
  try { await api.exportCSV(query(this.data.filters, this.data.month)); }
  catch (e) { wx.showToast({title: e.message, icon: 'none'}); }
  finally { this.setData({exporting: false}); }
 },
 openDate() { this.setData({dateOpen: true}); },
 closeDate() { this.setData({dateOpen: false}); },
 onDateConfirm(e) { this.setData({dateOpen: false, date: e.detail.date, time: e.detail.time}); },
 selection(kind, originalCategory, originalAccount) {
  const visibleCategories = allowed(this.data.categories.filter(c => c.kind === kind), originalCategory);
  const visibleAccounts = [{id: 0, name: '未指定账户', label: '未指定账户'}, ...allowed(this.data.accounts, originalAccount)];
  return {visibleCategories, visibleAccounts};
 },
 open() {
  if (this.data.loading || this.data.saving) return;
  this.requestID = requestID(); this.originalCategory = null; this.originalAccount = null;
  const selection = this.selection('expense');
  this.setData({...selection, editor: true, editID: null, kind: 'expense', categoryID: selection.visibleCategories.length ? selection.visibleCategories[0].id : 0, accountID: 0, accountIndex: 0, amount: '', note: '', ...stamp()});
 },
 edit(e) {
  if (this.data.saving) return;
  const item = this.data.groups.flatMap(g => g.items).find(x => x.id === Number(e.currentTarget.dataset.id)); if (!item) return;
  const current = api.session();
  if (!current || (current.user.role !== 'admin' && current.user.id !== item.created_by)) { wx.showToast({title: '只能修改自己的账单', icon: 'none'}); return; }
  this.originalCategory = item.category_id; this.originalAccount = item.account_id || 0;
  const selection = this.selection(item.kind, this.originalCategory, this.originalAccount);
  this.setData({...selection, editor: true, editID: item.id, kind: item.kind, categoryID: item.category_id, accountID: item.account_id || 0, accountIndex: Math.max(0, selection.visibleAccounts.findIndex(a => a.id === (item.account_id || 0))), amount: item.display, note: item.note, date: item.date, time: item.time});
 },
 close() { if (!this.data.saving) this.setData({editor: false}); },
 noop() {},
 kindChange(e) {
  const kind = e.currentTarget.dataset.kind, selection = this.selection(kind, this.originalCategory, this.originalAccount);
  this.setData({...selection, kind, categoryID: selection.visibleCategories.length ? selection.visibleCategories[0].id : 0});
 },
 categoryChange(e) { this.setData({categoryID: Number(e.currentTarget.dataset.id)}); },
 accountChange(e) { const i = Number(e.detail.value); this.setData({accountIndex: i, accountID: this.data.visibleAccounts[i].id}); },
 input(e) {
  const field = e.currentTarget.dataset.field, value = field === 'amount' ? acceptAmountInput(e.detail.value, this.data.amount) : e.detail.value;
  this.setData({[field]: value}); return value;
 },
 async save() {
  if (this.data.saving) return;
  try {
   const amount = parseCent(this.data.amount); if (amount <= 0) throw new Error('金额必须大于零');
   if (!this.data.categoryID) throw new Error('请先选择分类或联系管理员添加分类');
   const data = {category_id: this.data.categoryID, account_id: this.data.accountID, kind: this.data.kind, amount_cent: amount, occurred_at: this.data.date + 'T' + this.data.time + ':00+08:00', note: this.data.note, client_request_id: this.requestID || 'edit'};
   this.setData({saving: true});
   await api.request('/api/v1/entries' + (this.data.editID ? '/' + this.data.editID : ''), this.data.editID ? 'PATCH' : 'POST', data);
   this.setData({editor: false}); wx.showToast({title: '已保存'}); await this.load();
  } catch (e) { wx.showToast({title: e.message, icon: 'none'}); }
  finally { this.setData({saving: false}); }
 },
 remove() {
  if (this.data.saving) return;
  this.setData({saving: true});
  wx.showModal({title: '删除账单', content: '确定删除这笔账单？关联账户余额将同步重新计算。', success: async res => {
   if (!res.confirm) {this.setData({saving: false}); return;}
   try { await api.request('/api/v1/entries/' + this.data.editID, 'DELETE'); this.setData({editor: false}); await this.load(); }
   catch (e) {wx.showToast({title: e.message, icon: 'none'});} finally {this.setData({saving: false});}
  }, fail: () => this.setData({saving: false})});
 }
});
