const api = require('../../utils/api');
const { today } = require('../../utils/money');
Page({
 data: { user: null, ledger: null, error: '', busy: false },
 onShow() { this.load(); },
 openCategories() { wx.navigateTo({ url: '/pages/categories/index' }); },
 openAccounts() { wx.navigateTo({ url: '/pages/accounts/index' }); },
 openBudget() { wx.navigateTo({ url: '/pages/budget/index' }); },
 async load() {
  // Clear data from a previously logged-in account before fetching new permissions.
  this.setData({ user: null, ledger: null, error: '' });
  try {
   const [user, ledger] = await Promise.all([api.me(), api.request('/api/v1/ledgers/me')]);
   this.setData({ user, ledger });
  } catch (e) { this.setData({ error: e.message }); }
 },
 async signOut() { try { await api.logout(); this.setData({ user: null, ledger: null }); } catch (e) { this.setData({ error: e.message }); } },
 async exportCSV() {
  if (this.data.busy) return;
  this.setData({ busy: true, error: '' });
  try { await api.exportCSV('month=' + today().slice(0, 7)); }
  catch (e) { this.setData({ error: e.message }); }
  finally { this.setData({ busy: false }); }
 }
});
