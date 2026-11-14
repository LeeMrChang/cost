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
  navigateBack: () => navigations.push('back'),
  switchTab: o => navigations.push(o.url),
  showModal: opts => opts.success({ confirm: true }),
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

test('a logged-in ledger merges pending guest records before refreshing', async () => {
 const storage = new Map([['cost_guest_entries', [guestEntry({})]]]);
 const posts = [];
 const api = {
  isAuthenticated: () => true,
  session: () => ({ token: 't', expires_at: new Date(Date.now() + 3600000).toISOString(), user: { nickname: '小夏' } }),
  me: async () => ({ id: 1, nickname: '小夏' }),
  request: async (url, method, data) => {
   if (url === '/api/v1/categories') return [{ id: 1, name: '餐饮', kind: 'expense' }];
   if (method === 'POST') { posts.push(data); return { id: 9 }; }
   if (url.startsWith('/api/v1/stats')) return { summary: {}, budget: {}, label: '2026年11月' };
   if (url.startsWith('/api/v1/entries')) return { items: [], total: 0 };
   return {};
  },
 };
 const { page } = loadPage('ledger', api, storage);
 page.data.month = '2026-11';
 await page.onShow();
 await new Promise(r => setImmediate(r));
 assert.equal(posts.length, 1);
 assert.equal(posts[0].client_request_id, 'guest-1-ab');
 assert.equal(storage.get('cost_guest_entries').length, 0);
});

test('guest statistics summarise local month data and refuse other ranges', async () => {
 const storage = new Map([['cost_guest_entries', [guestEntry({}), guestEntry({ local_id: 2, category_key: '工资', kind: 'income', amount_cent: 500000, occurred_at: '2026-11-13T09:00:00+08:00' })]]]);
 const api = { isAuthenticated: () => false, session: () => null, request: async () => { throw new Error('guest stats must stay local'); } };
 const { page, navigations } = loadPage('stats', api, storage);
 page.data.ref = '2026-11-14';
 await page.load();
 assert.equal(page.data.label, '2026年11月');
 assert.equal(page.data.summary.expense, '18.00');
 assert.equal(page.data.summary.income, '5,000.00');
 assert.equal(page.data.chart.length, 30);
 const food = page.data.categories.find(c => c.name === '餐饮');
 assert.equal(food.amount, '18.00');
 assert.equal(page.data.categories.length, 1);
 const before = page.data.range;
 page.setRange({ currentTarget: { dataset: { range: 'week' } } });
 assert.equal(page.data.range, before);
});

test('guest search filters local records by note and category name', async () => {
 const storage = new Map([['cost_guest_entries', [
  guestEntry({}),
  guestEntry({ local_id: 2, category_key: '交通', note: '打车', amount_cent: 2300 }),
 ]]]);
 const api = { isAuthenticated: () => false, session: () => null, request: async () => { throw new Error('guest search must stay local'); } };
 const { page, navigations } = loadPage('search', api, storage);
 page.data.keyword = '打车';
 await page.load();
 assert.equal(page.data.entries.length, 1);
 assert.equal(page.data.entries[0].id, 2);
 page.edit({ currentTarget: { dataset: { id: 2 } } });
 assert.equal(navigations[navigations.length - 1], '/pages/entry-detail/index?id=2&local=1');
});

test('guest calendar builds cells from local month data', async () => {
 const storage = new Map([['cost_guest_entries', [guestEntry({})]]]);
 const api = { isAuthenticated: () => false, session: () => null, request: async () => { throw new Error('guest calendar must stay local'); } };
 const { page } = loadPage('calendar', api, storage);
 page.data.month = '2026-11';
 await page.load();
 assert.equal(page.data.summary.expense, '18.00');
 const day = page.data.cells.find(c => c.key === '2026-11-14');
 assert.ok(day);
 await page.tapDay({ currentTarget: { dataset: { key: '2026-11-14' } } });
 assert.equal(page.data.dayEntries.length, 1);
});

test('guest settings stay local and guard management entries', async () => {
 const storage = new Map([['cost_guest_entries', [guestEntry({})]]]);
 const api = { isAuthenticated: () => false, session: () => null, request: async () => { throw new Error('guest settings must stay local'); } };
 const { page } = loadPage('settings', api, storage);
 await page.onShow();
 assert.equal(page.data.guest, true);
 assert.equal(page.data.guestPending, 1);
 assert.equal(page.data.user, null);
 page.openBudget();
});

test('guest detail renders and deletes a local record', async () => {
 const storage = new Map([['cost_guest_entries', [guestEntry({})]]]);
 const api = { isAuthenticated: () => false, session: () => null, request: async () => { throw new Error('guest detail must stay local'); } };
 const { page, navigations } = loadPage('entry-detail', api, storage);
 page.onLoad({ id: '1', local: '1' });
 await page.load();
 assert.equal(page.data.amount, '18.00');
 assert.equal(page.data.categoryName, '餐饮');
 page.remove();
 assert.equal(storage.get('cost_guest_entries').length, 0);
 assert.equal(navigations[navigations.length - 1], '/pages/ledger/index');
});
