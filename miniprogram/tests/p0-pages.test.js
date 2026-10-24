const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');
const categories = [{id: 1, name: '餐饮', kind: 'expense', icon: '🍚', sort_order: 10, disabled: false}, {id: 2, name: '旧分类', kind: 'expense', icon: '📦', sort_order: 20, disabled: true}, {id: 3, name: '工资', kind: 'income', icon: '💰', sort_order: 10, disabled: false}];
const accounts = [{id: 1, name: '现金', type: 'cash', opening_balance_cent: 0, balance_cent: 100, disabled: false}, {id: 2, name: '旧账户', type: 'bank', opening_balance_cent: -100, balance_cent: -100, disabled: true}, {id: 3, name: '微信', type: 'wechat', opening_balance_cent: 0, balance_cent: 0, disabled: false}];
const entry = {id: 1, category_id: 2, account_id: 2, kind: 'expense', amount_cent: 1234, note: '旧账', created_by: 1, occurred_at: '2026-10-23T02:00:00Z'};
const response = {items: [entry], total: 51, page: 1, page_size: 50, summary: {income_cent: 20000, expense_cent: 1234, balance_cent: 18766}};
function load(name, handler, role = 'admin') {
 const file = path.resolve(__dirname, '../pages', name, 'index.js');
 assert.ok(fs.existsSync(file), name + ' page must exist');
 let page; const requests = [], toasts = [];
 const api = {session: () => ({user: {id: 1, role}}), me: async () => ({id: 1, role}), exportCSV: async q => requests.push(['export', q]), request: async (...args) => {
  requests.push(args); if (handler) { const result = handler(...args); if (result !== undefined) return result; }
  if (args[0] === '/api/v1/categories') return categories;
  if (args[0] === '/api/v1/accounts') return accounts;
  if (args[0] === '/api/v1/entry-authors') return [{id: 0}, {id: 1}];
  if (args[0].startsWith('/api/v1/entries?')) return response;
  if (args[0].startsWith('/api/v1/transfers?')) return {items: [], total: 0, page: 1, page_size: 50};
  if (args[0].startsWith('/api/v1/stats?')) return {income_cent: 20000, expense_cent: 1234, budget_cent: null, remaining_cent: null};
  return {};
 }};
 vm.runInNewContext(fs.readFileSync(file, 'utf8'), {Page: p => {page = p;}, require: id => id.endsWith('/utils/api') ? api : require(path.resolve(path.dirname(file), id)), wx: {showToast: v => toasts.push(v), showModal: opts => opts.success({confirm: true})}}, {filename: file});
 page.data = JSON.parse(JSON.stringify(page.data)); page.setData = v => Object.assign(page.data, v);
 return {page, requests, toasts};
}
test('ledger sends combined filters with zero values and whole-result summary', async () => {
 const {page, requests} = load('ledger');
 page.data.filters = {q: '午餐 &%', start_date: '2026-01-01', end_date: '2026-10-23', kind: 'expense', category_id: '2', created_by: '0', account_id: '0', min_amount: '0', max_amount: '20.50'};
 await page.load(); const q = new URL('https://example.test' + requests.find(r => r[0].startsWith('/api/v1/entries?'))[0]).searchParams;
 assert.equal(q.get('q'), '午餐 &%'); assert.equal(q.get('start_date'), '2026-01-01'); assert.equal(q.get('end_date'), '2026-10-23'); assert.equal(q.has('month'), false);
 assert.equal(q.get('created_by'), '0'); assert.equal(q.get('account_id'), '0'); assert.equal(q.get('min_amount_cent'), '0'); assert.equal(q.get('max_amount_cent'), '2050');
 assert.equal(page.data.total, 51); assert.equal(page.data.summary.income, '200.00'); assert.equal(page.data.summary.balance, '187.66');
});
test('ledger rejects reversed date and amount ranges before fetching entries', async () => {
 const {page, requests} = load('ledger');
 page.data.filters = {start_date: '2026-10-23', end_date: '2026-10-22'}; await page.load();
 assert.match(page.data.error, /日期/); assert.equal(requests.filter(r => r[0].startsWith('/api/v1/entries?')).length, 0);
 page.data.filters = {min_amount: '2', max_amount: '1'}; await page.load(); assert.match(page.data.error, /金额/);
});
test('ledger skips blank optional amounts and paginates without duplicate entries', async () => {
 const {page, requests} = load('ledger', url => url.includes('page=2') ? {...response, page: 2, items: [{...entry, id: 2}]} : undefined);
 page.data.filters = {min_amount: '', max_amount: ''}; await page.load();
 assert.ok(!requests.find(r => r[0].startsWith('/api/v1/entries?'))[0].includes('amount_cent'));
 assert.equal(typeof page.loadMore, 'function'); await page.loadMore();
 assert.equal(page.data.groups.flatMap(g => g.items).length, 2); assert.equal(page.data.page, 2); assert.equal(page.data.total, 51);
});
test('ledger ignores old completed responses without clearing newer loading state', async () => {
 const pending = []; const {page} = load('ledger', url => url.startsWith('/api/v1/entries?') ? new Promise(resolve => pending.push(resolve)) : undefined);
 const first = page.load(); const second = page.load(); await Promise.resolve();
 pending[0]({...response, total: 99}); await first; assert.equal(page.data.loading, true);
 pending[1]({...response, total: 2}); await second; assert.equal(page.data.total, 2); assert.equal(page.data.loading, false);
});
test('ledger excludes disabled selections on create and preserves historical references when editing', async () => {
 const {page, requests} = load('ledger'); await page.load(); page.open();
 assert.deepEqual(Array.from(page.data.visibleCategories, c => c.id), [1]); assert.equal(page.data.categoryID, 1);
 assert.ok(!page.data.visibleAccounts.some(a => a.id === 2)); page.edit({currentTarget: {dataset: {id: 1}}});
 assert.ok(page.data.visibleCategories.some(c => c.id === 2)); assert.ok(page.data.visibleAccounts.some(a => a.id === 2));
 await page.save(); const write = requests.find(r => r[1] === 'PATCH'); assert.equal(write[2].account_id, 2);
});
test('ledger exports currently applied filters', async () => {
 const {page, requests} = load('ledger'); page.data.filters = {q: '奶茶'};
 assert.equal(typeof page.exportEntries, 'function'); await page.exportEntries();
 assert.equal(new URLSearchParams(requests.find(r => r[0] === 'export')[1]).get('q'), '奶茶');
});
test('categories validate names and allow create plus historical category rename', async () => {
 const {page, requests, toasts} = load('categories'); await page.load(); page.open(); page.data.name = ' '; await page.save();
 assert.ok(toasts.length); assert.equal(requests.filter(r => r[1] === 'POST').length, 0);
 page.data.name = '宠物'; page.data.icon = '🐱'; await page.save();
 const created = requests.find(r => r[1] === 'POST'); assert.equal(created[2].name, '宠物'); assert.equal(created[2].kind, 'expense');
 page.edit({currentTarget: {dataset: {id: 2}}}); page.data.name = '旧分类改名'; await page.save();
 assert.ok(requests.some(r => r[0] === '/api/v1/categories/2' && r[1] === 'PATCH'));
});
test('accounts allow signed opening balances and transfers between different enabled accounts', async () => {
 const {page, requests} = load('accounts'); await page.load(); page.openAccount(); page.data.name = '信用卡'; page.data.opening = '-12.34'; await page.saveAccount();
 assert.equal(requests.find(r => r[1] === 'POST')[2].opening_balance_cent, -1234);
 page.openTransfer(); page.data.fromID = 1; page.data.toID = 1; page.data.amount = '5'; await page.saveTransfer();
 assert.equal(requests.filter(r => r[0] === '/api/v1/transfers').length, 0); page.data.toID = 3; await page.saveTransfer();
 const transfer = requests.find(r => r[0] === '/api/v1/transfers'); assert.equal(transfer[2].amount_cent, 500); assert.ok(transfer[2].client_request_id);
 assert.equal(transfer[2].from_account_id, 1); assert.equal(transfer[2].to_account_id, 3);
});
test('members cannot manage accounts but can create transfers', async () => {
 const {page, requests} = load('accounts', undefined, 'member'); await page.load(); page.openAccount();
 assert.equal(page.data.accountEditor, false); page.data.name = 'test'; await page.saveAccount(); assert.equal(requests.filter(r => r[1] === 'POST').length, 0);
 page.openTransfer(); assert.equal(page.data.transferEditor, true);
});
test('ledger keeps monthly budget separate from filtered totals and refreshes summary on later pages', async () => {
 const {page} = load('ledger', url => url.startsWith('/api/v1/stats?') ? {income_cent: 20000, expense_cent: 1234, budget_cent: 30000, remaining_cent: 28766} : url.includes('page=2') ? {...response, page: 2, items: [{...entry, id: 2}], summary: {income_cent: 40000, expense_cent: 2468, balance_cent: 37532}} : undefined);
 await page.load(); assert.equal(page.data.summary.remaining, '¥287.66');
 await page.loadMore(); assert.equal(page.data.summary.income, '400.00'); assert.equal(page.data.summary.expense, '24.68');
 page.data.filters = {q: '旧账'}; await page.load(); assert.equal(page.data.summary.remaining, undefined);
});
test('failed fresh queries clear the prior result so old entries cannot appear under a new filter', async () => {
 let broken = false; const {page} = load('ledger', url => broken && url.startsWith('/api/v1/entries?') ? Promise.reject(new Error('offline')) : undefined);
 await page.load(); broken = true; page.data.filters = {q: 'different'}; await page.load();
 assert.equal(page.data.groups.length, 0); assert.equal(page.data.total, 0); assert.match(page.data.error, /offline/);
});
test('ledger save excludes duplicate submissions while the first request is pending', async () => {
 let resolve; const {page, requests} = load('ledger', (url, method) => method === 'POST' ? new Promise(r => {resolve = r;}) : undefined);
 await page.load(); page.open(); page.data.amount = '1.00'; page.data.accountID = 1;
 const first = page.save(); await page.save(); assert.equal(requests.filter(r => r[1] === 'POST').length, 1);
 resolve({}); await first; assert.equal(page.data.editor, false); assert.equal(page.data.saving, false);
});
test('management edits do not re-enable categories or accounts using stale disabled state', async () => {
 for (const [name, open, save] of [['categories', 'open', 'save'], ['accounts', 'openAccount', 'saveAccount']]) {
  const {page, requests} = load(name); await page.load(); page[open](); page.data.name = '新名称'; await page[save]();
  assert.equal(Object.hasOwn(requests.find(r => r[1] === 'POST')[2], 'disabled'), false);
 }
});
test('disable toggles submit only the state, preserving names and balances changed elsewhere', async () => {
 for (const [name, toggle, route] of [['categories', 'toggle', '/api/v1/categories/1'], ['accounts', 'toggleAccount', '/api/v1/accounts/1']]) {
  const {page, requests} = load(name); await page.load(); page[toggle]({currentTarget: {dataset: {id: 1}}});
  await new Promise(setImmediate);
  const write = requests.find(r => r[0] === route && r[1] === 'PATCH');
  assert.deepEqual(Object.keys(write[2]), ['disabled']); assert.equal(write[2].disabled, true);
 }
});
test('management pages clear previous family data when a fresh authenticated load fails', async () => {
 for (const name of ['accounts', 'categories']) {
  let fail = false; const {page} = load(name, url => fail ? Promise.reject(new Error('登录过期')) : undefined);
  await page.load(); fail = true; await page.load();
  assert.equal(page.data.user, null); assert.equal(page.data[name].length, 0);
 }
});
