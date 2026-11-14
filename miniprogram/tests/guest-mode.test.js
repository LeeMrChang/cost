const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

function loadPage(name, api, storage = new Map()) {
 let page; const requests = [], navigations = [];
 const file = path.resolve(__dirname, '../pages', name, 'index.js');
 const wxMock = {
  getStorageSync: key => storage.get(key),
  setStorageSync: (key, value) => storage.set(key, value),
  removeStorageSync: key => storage.delete(key),
  getWindowInfo: () => ({ statusBarHeight: 20, windowWidth: 375 }),
  getMenuButtonBoundingClientRect: () => ({ left: 300 }),
  navigateTo: o => navigations.push(o.url),
  reLaunch: o => navigations.push(o.url),
  showToast: () => {},
 };
 // 页面依赖的工具模块在宿主上下文执行，因此 wx 需要挂到 global。
 global.wx = wxMock;
 vm.runInNewContext(fs.readFileSync(file, 'utf8'), {
  Page: p => { page = p; },
  require: id => id.endsWith('/utils/api') ? api : require(path.resolve(path.dirname(file), id)),
  wx: wxMock,
  getApp: () => ({ globalData: {} }),
  getCurrentPages: () => [{ route: 'pages/ledger/index' }],
 }, { filename: file });
 page.data = JSON.parse(JSON.stringify(page.data));
 page.setData = v => Object.assign(page.data, v);
 return { page, requests, navigations, storage };
}

const guestEntry = over => Object.assign({
 local_id: 1, category_key: '餐饮', kind: 'expense', amount_cent: 1800,
 occurred_at: '2026-11-14T12:00:00+08:00', note: '午饭', client_request_id: 'guest-1-ab',
}, over);

test('the ledger renders local guest entries without calling the API', async () => {
 const storage = new Map([['cost_guest_entries', [guestEntry({}), guestEntry({ local_id: 2, kind: 'income', amount_cent: 500000, occurred_at: '2026-11-13T09:00:00+08:00', note: '工资' })]]]);
 const api = { isAuthenticated: () => false, session: () => null, request: async () => { throw new Error('must not call the API in guest mode'); } };
 const { page } = loadPage('ledger', api, storage);
 page.data.month = '2026-11';
 await page.load();
 assert.equal(page.data.guest, true);
 assert.equal(page.data.summary.expense, '18.00');
 assert.equal(page.data.summary.income, '5,000.00');
 assert.equal(page.data.groups.length, 2);
 assert.equal(page.data.groups.reduce((n, g) => n + g.items.length, 0), 2);
 assert.equal(page.data.budgetSet, false);
 assert.equal(page.data.entries.length, 2);
});

test('guest entries outside the current month are not listed', async () => {
 const storage = new Map([['cost_guest_entries', [guestEntry({ occurred_at: '2026-10-31T23:00:00+08:00' })]]]);
 const api = { isAuthenticated: () => false, session: () => null, request: async () => ({}) };
 const { page } = loadPage('ledger', api, storage);
 page.data.month = '2026-11';
 await page.load();
 assert.equal(page.data.entries.length, 0);
 assert.equal(page.data.summary.expense, '0.00');
});

test('the logged-in ledger keeps using the server API', async () => {
 const called = [];
 const api = {
  isAuthenticated: () => true,
  session: () => ({ token: 't', user: { nickname: '小夏' }, expires_at: new Date(Date.now() + 3600000).toISOString() }),
  me: async () => ({ id: 1, nickname: '小夏' }),
  request: async url => {
   called.push(url);
   if (url.startsWith('/api/v1/categories')) return [];
   if (url.startsWith('/api/v1/stats')) return { summary: {}, budget: {}, label: '2026年11月' };
   if (url.startsWith('/api/v1/entries')) return { items: [], total: 0 };
   return {};
  },
 };
 const { page } = loadPage('ledger', api);
 page.data.month = '2026-11';
 await page.load();
 assert.equal(page.data.guest, false);
 assert.ok(called.some(url => url.startsWith('/api/v1/entries')));
});

function loadEntry(api, storage, options = {}) {
 const entryLoad = loadPage('entry', api, storage);
 entryLoad.page.data.date = '2026-11-14';
 entryLoad.page.data.time = '12:30';
 return Object.assign(entryLoad, { options });
}

test('guest save writes a local entry and never calls the API', async () => {
 const storage = new Map();
 const api = { isAuthenticated: () => false, session: () => null, request: async () => { throw new Error('guest save must stay local'); } };
 const { page, navigations } = loadEntry(api, storage);
 await page.onShow();
 assert.equal(page.data.guest, true);
 page.setData({ resultCents: 1800, categoryID: page.data.categoryID || 101, note: '午饭' });
 await page.save();
 const rows = storage.get('cost_guest_entries');
 assert.equal(rows.length, 1);
 assert.equal(rows[0].amount_cent, 1800);
 assert.equal(rows[0].note, '午饭');
 assert.match(rows[0].client_request_id, /^guest-/);
 assert.equal(navigations[navigations.length - 1], '/pages/ledger/index');
});

test('guest save refuses a zero amount', async () => {
 const storage = new Map();
 const api = { isAuthenticated: () => false, session: () => null, request: async () => ({}) };
 const { page } = loadEntry(api, storage);
 await page.onShow();
 page.setData({ resultCents: 0, categoryID: 101 });
 await page.save();
 assert.equal((storage.get('cost_guest_entries') || []).length, 0);
});

test('guest edit updates the existing local entry instead of adding one', async () => {
 const storage = new Map([['cost_guest_entries', [guestEntry({ amount_cent: 1800 })]]]);
 const api = { isAuthenticated: () => false, session: () => null, request: async () => ({}) };
 const { page } = loadEntry(api, storage);
 page.onLoad({ localEdit: '1' });
 await page.onShow();
 assert.equal(page.data.localEdit, 1);
 assert.equal(page.data.resultCents, 1800);
 page.setData({ resultCents: 2500 });
 await page.save();
 const rows = storage.get('cost_guest_entries');
 assert.equal(rows.length, 1);
 assert.equal(rows[0].amount_cent, 2500);
 assert.equal(rows[0].client_request_id, 'guest-1-ab');
});
