const api = require('../../utils/api');
const {parseCent, money, acceptAmountInput} = require('../../utils/money');
const {stamp, requestID} = require('../../utils/ledger');
const types = [{value: 'cash', label: '现金', icon: '💵'}, {value: 'wechat', label: '微信', icon: '💚'}, {value: 'alipay', label: '支付宝', icon: '💙'}, {value: 'bank', label: '银行卡', icon: '💳'}, {value: 'other', label: '其他', icon: '👛'}];
Page({
 data: {user: null, accounts: [], activeAccounts: [], transfers: [], types, loading: false, loadingMore: false, total: 0, page: 0, hasMore: false, error: '', saving: false, accountEditor: false, transferEditor: false, editID: null, name: '', opening: '0.00', typeIndex: 0, disabled: false, fromID: 0, toID: 0, fromIndex: 0, toIndex: 1, amount: '', note: '', date: stamp().date, time: stamp().time, dateOpen: false, balance: '0.00'},
 onShow() {this.load();},
 onUnload() {this.version = (this.version || 0) + 1;},
 onReachBottom() {this.loadMore();},
 decorateTransfers(items, accounts) {
  const map = {}; accounts.forEach(a => {map[a.id] = a.name;});
  return items.map(t => ({...t, fromName: map[t.from_account_id] || '历史账户', toName: map[t.to_account_id] || '历史账户', display: money(t.amount_cent), ...stamp(t.occurred_at), canDelete: true}));
 },
 async load() {
  const version = this.version = (this.version || 0) + 1; this.setData({loading: true, loadingMore: false, error: '', user: null, accounts: [], activeAccounts: [], transfers: [], total: 0, page: 0, hasMore: false, balance: '0.00'});
  try {
   const [user, rows, transfers] = await Promise.all([api.me(), api.request('/api/v1/accounts'), api.request('/api/v1/transfers?page=1&page_size=50')]); if (version !== this.version) return;
   const accounts = rows.map(a => {const type = types.find(t => t.value === a.type) || types[4]; return {...a, icon: type.icon, typeLabel: type.label, balance: money(a.balance_cent), opening: money(a.opening_balance_cent)};});
   this.setData({user, accounts, activeAccounts: accounts.filter(a => !a.disabled), balance: money(accounts.reduce((sum, a) => sum + a.balance_cent, 0)), transfers: this.decorateTransfers(transfers.items || [], accounts), total: transfers.total, page: 1, hasMore: transfers.items.length < transfers.total});
  } catch (e) {if (version === this.version) this.setData({error: e.message});} finally {if (version === this.version) this.setData({loading: false});}
 },
 async loadMore() {
  if (this.data.loading || this.data.loadingMore || !this.data.hasMore) return;
  const version = this.version, page = this.data.page + 1; this.setData({loadingMore: true, error: ''});
  try {
   const result = await api.request('/api/v1/transfers?page=' + page + '&page_size=50'); if (version !== this.version) return;
   const map = {}; this.data.transfers.concat(this.decorateTransfers(result.items, this.data.accounts)).forEach(t => {map[t.id] = t;});
   const transfers = Object.values(map).sort((a, b) => b.occurred_at.localeCompare(a.occurred_at) || b.id - a.id);
   this.setData({transfers, total: result.total, page, hasMore: result.items.length > 0 && transfers.length < result.total});
  } catch (e) {if (version === this.version) this.setData({error: e.message});} finally {if (version === this.version) this.setData({loadingMore: false});}
 },
 canManage() {if (!this.data.user) {wx.showToast({title: '账本数据尚未加载', icon: 'none'}); return false;} return true;},
 openAccount() {if (this.data.saving || !this.canManage()) return; this.setData({accountEditor: true, editID: null, name: '', opening: '0.00', typeIndex: 0, disabled: false});},
 editAccount(e) {
  if (this.data.saving || !this.canManage()) return; const a = this.data.accounts.find(a => a.id === Number(e.currentTarget.dataset.id)); if (!a) return;
  this.setData({accountEditor: true, editID: a.id, name: a.name, opening: a.opening, typeIndex: Math.max(0, types.findIndex(t => t.value === a.type)), disabled: a.disabled});
 },
 input(e) {
  const field = e.currentTarget.dataset.field; let value = e.detail.value;
  if (field === 'amount') value = acceptAmountInput(value, this.data.amount);
  if (field === 'opening' && !/^-?(?:\d{1,9}(?:\.\d{0,2})?)?$/.test(value)) value = this.data.opening;
  this.setData({[field]: value}); return value;
 },
 typeChange(e) {this.setData({typeIndex: Number(e.detail.value)});},
 async saveAccount() {
  if (this.data.saving || !this.canManage()) return;
  try {
   const name = this.data.name.trim(); if (!name || name.length > 50) throw new Error('账户名称需为 1–50 个字符');
   const text = this.data.opening.trim(), opening = parseCent(text.startsWith('-') ? text.slice(1) : text) * (text.startsWith('-') ? -1 : 1);
   const data = {name, type: types[this.data.typeIndex].value, opening_balance_cent: opening};
   this.setData({saving: true}); await api.request('/api/v1/accounts' + (this.data.editID ? '/' + this.data.editID : ''), this.data.editID ? 'PATCH' : 'POST', data);
   this.setData({accountEditor: false}); wx.showToast({title: '账户已保存'}); await this.load();
  } catch (e) {wx.showToast({title: e.message, icon: 'none'});} finally {this.setData({saving: false});}
 },
 toggleAccount(e) {
  if (this.data.saving || !this.canManage()) return; const a = this.data.accounts.find(a => a.id === Number(e.currentTarget.dataset.id)); if (!a) return;
  this.setData({saving: true});
  wx.showModal({title: a.disabled ? '启用账户' : '停用账户', content: a.disabled ? '启用后可用于记账和转账。' : '停用后新账单及转账不可选此账户，历史记录和余额仍保留。', success: async res => {
   if (!res.confirm) {this.setData({saving: false}); return;}
   try {await api.request('/api/v1/accounts/' + a.id, 'PATCH', {disabled: !a.disabled}); await this.load();}
   catch (e) {wx.showToast({title: e.message, icon: 'none'});} finally {this.setData({saving: false});}
  }, fail: () => this.setData({saving: false})});
 },
 openTransfer() {
  if (this.data.saving || this.data.loading) return;
  const a = this.data.activeAccounts; if (a.length < 2) {wx.showToast({title: '请先启用至少两个账户', icon: 'none'}); return;}
  this.transferRequestID = requestID(); this.setData({transferEditor: true, fromIndex: 0, toIndex: 1, fromID: a[0].id, toID: a[1].id, amount: '', note: '', ...stamp()});
 },
 transferAccountChange(e) {
  const field = e.currentTarget.dataset.field, index = Number(e.detail.value);
  this.setData({[field + 'Index']: index, [field + 'ID']: this.data.activeAccounts[index].id});
 },
 openDate() {this.setData({dateOpen: true});}, closeDate() {this.setData({dateOpen: false});},
 onDateConfirm(e) {this.setData({dateOpen: false, date: e.detail.date, time: e.detail.time});},
 async saveTransfer() {
  if (this.data.saving) return;
  try {
   const amount = parseCent(this.data.amount); if (amount <= 0) throw new Error('转账金额必须大于零');
   if (!this.data.fromID || !this.data.toID || this.data.fromID === this.data.toID) throw new Error('请选择不同的转出和转入账户');
   if (!this.data.activeAccounts.some(a => a.id === this.data.fromID) || !this.data.activeAccounts.some(a => a.id === this.data.toID)) throw new Error('请选择启用的账户');
   this.setData({saving: true});
   await api.request('/api/v1/transfers', 'POST', {from_account_id: this.data.fromID, to_account_id: this.data.toID, amount_cent: amount, occurred_at: this.data.date + 'T' + this.data.time + ':00+08:00', note: this.data.note, client_request_id: this.transferRequestID});
   this.setData({transferEditor: false}); wx.showToast({title: '转账已记录'}); await this.load();
  } catch (e) {wx.showToast({title: e.message, icon: 'none'});} finally {this.setData({saving: false});}
 },
 removeTransfer(e) {
  if (this.data.saving) return; const t = this.data.transfers.find(t => t.id === Number(e.currentTarget.dataset.id)); if (!t || !t.canDelete) return;
  this.setData({saving: true}); wx.showModal({title: '删除转账', content: '将撤销这笔转账对双方余额的影响。确定删除？', success: async res => {
   if (!res.confirm) {this.setData({saving: false}); return;}
   try {await api.request('/api/v1/transfers/' + t.id, 'DELETE'); await this.load();} catch (e) {wx.showToast({title: e.message, icon: 'none'});} finally {this.setData({saving: false});}
  }, fail: () => this.setData({saving: false})});
 },
 close() {if (!this.data.saving) this.setData({accountEditor: false, transferEditor: false});}, noop() {}
});
