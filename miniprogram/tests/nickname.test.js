const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

function load(name, api) {
 let page; const navigations = [], toasts = [];
 const file = path.resolve(__dirname, '../pages', name, 'index.js');
 vm.runInNewContext(fs.readFileSync(file, 'utf8'), {
  Page: p => { page = p; }, require: id => id.endsWith('/utils/api') ? api : require(path.resolve(path.dirname(file), id)),
  wx: { reLaunch: o => navigations.push(o.url), showToast: o => toasts.push(o.title) }
 });
 page.data = JSON.parse(JSON.stringify(page.data));
 page.setData = v => Object.assign(page.data, v);
 return { page, navigations, toasts };
}
const defaults = { DEFAULT_NICKNAME: '微信用户', isDevelop: () => false, config: () => ({ baseURL: 'https://example.test' }), session: () => null };

test('a returning account with a nickname enters the ledger immediately', async () => {
 const { page, navigations } = load('login', { ...defaults, session: () => ({ token: 't' }), me: async () => ({ id: 1, nickname: '小夏' }) });
 await page.onShow();
 assert.deepEqual(navigations, ['/pages/ledger/index']);
 assert.equal(page.data.nickname, '小夏');
});

test('an account without a nickname receives the default one instead of a blocking form', async () => {
 const calls = [];
 const { page, navigations } = load('login', { ...defaults, session: () => ({ token: 't' }), me: async () => ({ id: 1, nickname: '' }), updateNickname: async value => { calls.push(value); return { id: 1, nickname: value }; } });
 await page.onShow();
 assert.deepEqual(calls, ['微信用户']);
 assert.deepEqual(navigations, ['/pages/ledger/index']);
});

test('first login without a typed nickname logs in and enters with the default name', async () => {
 const calls = [];
 const { page, navigations } = load('login', {
  ...defaults,
  login: async value => { calls.push(value); return { id: 1, nickname: '' }; },
  updateNickname: async value => ({ id: 1, nickname: value }),
 });
 await page.signIn();
 assert.deepEqual(calls, [undefined]);
 assert.deepEqual(navigations, ['/pages/ledger/index']);
 assert.equal(page.data.error, '');
});

test('a typed nickname is submitted with the login', async () => {
 const calls = [];
 const { page, navigations } = load('login', {
  ...defaults,
  login: async value => { calls.push(value); return { id: 1, nickname: value }; },
 });
 await page.signIn({ detail: { value: { nickname: ' 小夏🐱 ' } } });
 assert.deepEqual(calls, ['小夏🐱']);
 assert.deepEqual(navigations, ['/pages/ledger/index']);
});

test('an over-long nickname is rejected before any login attempt', async () => {
 let attempts = 0;
 const { page, navigations } = load('login', { ...defaults, login: async () => { attempts++; return { id: 1, nickname: 'x' }; } });
 await page.signIn({ detail: { value: { nickname: '名'.repeat(65) } } });
 assert.equal(attempts, 0);
 assert.equal(navigations.length, 0);
 assert.match(page.data.error, /64/);
});
