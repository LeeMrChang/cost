const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');
const ledger = require('../utils/ledger');
const authors = [{id: 0, nickname: '', status: 'unspecified'}, {id: 1, nickname: '爸爸', status: 'active'}, {id: 2, nickname: '妈妈🍀', status: 'removed'}, {id: 3, nickname: '爸爸', status: 'active'}];
const entry = {id: 1, created_by: 2, category_id: 1, account_id: 0, amount_cent: 100, kind: 'expense', occurred_at: '2026-10-24T01:00:00Z'};
function load(name, overrides = {}) {
 let page; let user = {id: 1, role: 'admin', nickname: '爸爸'}; const calls = [], toasts = [];
 const api = {session: () => ({user}), me: async () => user, logout: async () => {user = null;}, ...overrides};
 api.request = async (...args) => {
  calls.push(args);
  if (overrides.request) {const value = overrides.request(...args); if (value !== undefined) return value;}
  if (args[0] === '/api/v1/entry-authors') return authors;
  if (args[0] === '/api/v1/family/members') return [{id: 1, role: 'admin', nickname: '爸爸'}, {id: 3, role: 'member', nickname: '爸爸'}];
  if (args[0] === '/api/v1/categories') return [{id: 1, name: '午餐', kind: 'expense'}];
  if (args[0] === '/api/v1/accounts') return [];
  if (args[0].startsWith('/api/v1/transfers?')) return {items: [{id: 1, created_by: 2, occurred_at: entry.occurred_at, amount_cent: 100}], total: 1};
  if (args[0].startsWith('/api/v1/entries?')) return {items: [entry], total: 1, summary: {expense_cent: 100, income_cent: 0, balance_cent: -100}};
  if (args[0].startsWith('/api/v1/stats?')) return {budget_cent: null, remaining_cent: null};
  if (args[1] === 'PATCH') {if (args[0].includes('/1/')) user = {...user, nickname: args[2].nickname}; return {id: 1, nickname: args[2].nickname};}
  return {};
 };
 const file = path.resolve(__dirname, '../pages', name, 'index.js');
 vm.runInNewContext(fs.readFileSync(file, 'utf8'), {Page: p => {page = p;}, require: id => id.endsWith('/utils/api') ? api : require(path.resolve(path.dirname(file), id)), wx: {showToast: p => toasts.push(p), showModal: o => o.success({confirm: true})}});
 page.data = JSON.parse(JSON.stringify(page.data)); page.setData = v => Object.assign(page.data, v);
 return {page, calls, toasts};
}
test('ledger names distinguish removed, unspecified and unnamed members', () => {
 const rows = ledger.groups([entry, {...entry, id: 2, created_by: 0}, {...entry, id: 3, created_by: 9}], [], [], authors)[0].items;
 assert.equal(rows[0].authorName, '妈妈🍀（已移除）'); assert.equal(rows[1].authorName, '历史未指定成员'); assert.equal(rows[2].authorName, '成员 9');
});
test('ledger directory identifies duplicate names by ID and decorates entries', async () => {
 const {page} = load('ledger'); await page.load(); page.openFilters();
 assert.match(page.data.filterAuthors.find(a => a.id === 1).label, /爸爸.*1/);
 assert.match(page.data.filterAuthors.find(a => a.id === 3).label, /爸爸.*3/);
 assert.equal(page.data.groups[0].items[0].authorName, '妈妈🍀（已移除）');
});
test('mine and all preserve other filters and restart pagination', async () => {
 const {page, calls} = load('ledger'); page.data.filters = {...ledger.emptyFilters(), q: '空调', start_date: '2026-01-01', end_date: '2026-10-24', min_amount: '0', created_by: 2}; page.data.page = 3;
 assert.equal(typeof page.memberScope, 'function');
 await page.memberScope({currentTarget: {dataset: {scope: 'mine'}}});
 let url = new URL('https://example.test' + calls.filter(c => c[0].startsWith('/api/v1/entries?')).at(-1)[0]);
 assert.equal(url.searchParams.get('created_by'), '1'); assert.equal(url.searchParams.get('page'), '1'); assert.equal(url.searchParams.get('q'), '空调'); assert.equal(page.data.memberScopeValue, 'mine');
 page.openFilters(); assert.equal(String(page.data.draft.created_by), '1');
 await page.memberScope({currentTarget: {dataset: {scope: 'all'}}});
 url = new URL('https://example.test' + calls.filter(c => c[0].startsWith('/api/v1/entries?')).at(-1)[0]);
 assert.equal(url.searchParams.has('created_by'), false); assert.equal(url.searchParams.get('start_date'), '2026-01-01'); assert.equal(url.searchParams.get('min_amount_cent'), '0'); assert.equal(page.data.memberScopeValue, 'all');
});
test('fresh failed load clears cached names and member picker', async () => {
 let fail = false; const {page} = load('ledger', {request: () => fail ? Promise.reject(new Error('已移除')) : undefined});
 await page.load();page.openFilters();fail = true;await page.load();
 assert.equal(page.data.authors.length, 0);assert.equal(page.data.filterAuthors.length, 0);assert.equal(page.data.filterOpen, false);
});
test('nickname saves self, refreshes session and validates Unicode', async () => {
 const {page, calls} = load('settings'); await page.load();
 assert.equal(typeof page.editNickname, 'function');page.editNickname({currentTarget: {dataset: {id: 1}}});
 page.data.nickname = '  家长🍀  ';await page.saveNickname();
 assert.equal(calls.find(c => c[1] === 'PATCH')[2].nickname, '家长🍀');assert.equal(page.data.user.nickname, '家长🍀');assert.equal(page.data.nicknameEditor, false);
 for (const value of ['名'.repeat(21), '坏\n名字', '\t名字', '名字\u2028换行']) {page.editNickname({currentTarget: {dataset: {id: 1}}}); page.data.nickname = value;const count = calls.length;await page.saveNickname();assert.equal(calls.length, count);assert.ok(page.data.nicknameError);}
 page.data.nickname = '🍀'.repeat(20);await page.saveNickname();assert.equal(page.data.user.nickname, '🍀'.repeat(20));
 page.editNickname({currentTarget: {dataset: {id: 1}}});page.clearNickname();await page.saveNickname();assert.equal(page.data.user.nickname, '');
});
test('nickname editor rejects other members, blocks duplicates and preserves failed input', async () => {
 let reject;const {page, calls} = load('settings', {me: async () => ({id: 1, role: 'member', nickname: ''}), request: (url,method) => method === 'PATCH' ? new Promise((_,r) => {reject = r;}) : undefined});
 await page.load();assert.equal(typeof page.editNickname, 'function');page.editNickname({currentTarget: {dataset: {id: 3}}});assert.equal(page.data.nicknameEditor, false);
 page.editNickname({currentTarget: {dataset: {id: 1}}});page.data.nickname = '保留输入';const first = page.saveNickname();await page.saveNickname();assert.equal(calls.filter(c => c[1] === 'PATCH').length, 1);
 reject(new Error('网络错误'));await first;assert.equal(page.data.nickname, '保留输入');assert.equal(page.data.nicknameEditor, true);assert.match(page.data.nicknameError, /网络错误/);assert.equal(page.data.nicknameSaving, false);
});
test('settings failure and logout clear member editor and profiles', async () => {
 let fail = false;const {page} = load('settings', {me: async () => {if (fail) throw new Error('过期'); return {id: 1, role: 'admin', nickname: '爸爸'};}});
 await page.load();assert.equal(typeof page.editNickname, 'function');page.editNickname({currentTarget: {dataset: {id: 1}}});page.data.nickname = '私有草稿';fail = true;await page.load();
 assert.equal(page.data.user, null);assert.equal(page.data.members.length, 0);assert.equal(page.data.nickname, '');assert.equal(page.data.nicknameEditor, false);
});
test('transfer rows use the same historical name presentation', async () => {
 const {page} = load('accounts');await page.load();assert.equal(page.data.transfers[0].authorName, '妈妈🍀（已移除）');
});

test('nickname trims BOM consistently with ordinary surrounding whitespace', () => {
 const {nicknameValue} = require('../utils/members');
 assert.equal(nicknameValue('\ufeff' + '名'.repeat(20) + '\ufeff'), '名'.repeat(20));
 assert.equal(nicknameValue('\ufeff'), '');
});
