const api = require('../../utils/api');
Page({
 data: {user: null, categories: [], visible: [], kind: 'expense', loading: false, error: '', editor: false, editID: null, name: '', icon: '📦', order: '0', disabled: false, saving: false, icons: ['🍚', '🛒', '🚗', '🏠', '🎮', '🏥', '🐱', '📚', '🎁', '✈️', '💰', '📦']},
 onShow() { this.load(); },
 onUnload() { this.version = (this.version || 0) + 1; },
 async load() {
  const version = this.version = (this.version || 0) + 1;
  this.setData({loading: true, error: '', user: null, categories: [], visible: []});
  try { const [user, categories] = await Promise.all([api.me(), api.request('/api/v1/categories')]); if (version !== this.version) return; this.setData({user, categories}); this.refresh(); }
  catch (e) {if (version === this.version) this.setData({error: e.message});}
  finally {if (version === this.version) this.setData({loading: false});}
 },
 refresh() { this.setData({visible: this.data.categories.filter(c => c.kind === this.data.kind).sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)}); },
 changeKind(e) { if (this.data.saving) return; this.setData({kind: e.currentTarget.dataset.kind}); this.refresh(); },
 canManage() { if (!this.data.user) {wx.showToast({title: '账本数据尚未加载', icon: 'none'}); return false;} return true; },
 open() {
  if (!this.canManage() || this.data.saving) return;
  const order = this.data.visible.reduce((max, c) => Math.max(max, c.sort_order), 0) + 10;
  this.setData({editor: true, editID: null, name: '', icon: '📦', order: String(order), disabled: false});
 },
 edit(e) {
  if (!this.canManage() || this.data.saving) return;
  const c = this.data.categories.find(c => c.id === Number(e.currentTarget.dataset.id)); if (!c) return;
  this.setData({editor: true, editID: c.id, kind: c.kind, name: c.name, icon: c.icon, order: String(c.sort_order), disabled: c.disabled});
 },
 close() {if (!this.data.saving) this.setData({editor: false});},
 noop() {},
 input(e) {const field = e.currentTarget.dataset.field; this.setData({[field]: e.detail.value});},
 pickIcon(e) {this.setData({icon: e.currentTarget.dataset.icon});},
 async save() {
  if (this.data.saving || !this.canManage()) return;
  try {
   const name = this.data.name.trim(), icon = this.data.icon.trim(), sort = Number(this.data.order);
   if (!name || name.length > 50) throw new Error('分类名称需为 1–50 个字符');
   if (!icon || Array.from(icon).length > 16) throw new Error('请填写简短图标（最多 16 个字符）');
   if (!/^\d+$/.test(this.data.order) || !Number.isSafeInteger(sort) || sort > 1000000) throw new Error('排序请输入 0–1000000 的整数');
   const data = {name, icon, kind: this.data.kind, sort_order: sort};
   this.setData({saving: true}); await api.request('/api/v1/categories' + (this.data.editID ? '/' + this.data.editID : ''), this.data.editID ? 'PATCH' : 'POST', data);
   this.setData({editor: false}); wx.showToast({title: '分类已保存'}); await this.load();
  } catch (e) {wx.showToast({title: e.message, icon: 'none'});} finally {this.setData({saving: false});}
 },
 toggle(e) {
  if (this.data.saving || !this.canManage()) return;
  const c = this.data.categories.find(c => c.id === Number(e.currentTarget.dataset.id)); if (!c) return;
  this.setData({saving: true});
  wx.showModal({title: c.disabled ? '启用分类' : '停用分类', content: c.disabled ? '启用后可继续用于新账单。' : '停用后新账单不可选，历史账单仍会保留。', success: async res => {
   if (!res.confirm) {this.setData({saving: false}); return;}
   try {await api.request('/api/v1/categories/' + c.id, 'PATCH', {disabled: !c.disabled}); await this.load();}
   catch (e) {wx.showToast({title: e.message, icon: 'none'});} finally {this.setData({saving: false});}
  }, fail: () => this.setData({saving: false})});
 }
});
