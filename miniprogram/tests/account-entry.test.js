const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

const categories = [
	{id: 1, name: '餐饮', kind: 'expense', icon: '🍜', sort_order: 10, disabled: false},
	{id: 3, name: '工资', kind: 'income', icon: '💰', sort_order: 10, disabled: false},
];
const accounts = [
	{id: 1, name: '现金', type: 'cash', disabled: false},
	{id: 2, name: '微信', type: 'wechat', disabled: false},
	{id: 9, name: '旧卡', type: 'bank', disabled: true},
];
const statsResponse = {label: '2026年11月', prev_ref: '2026-10-01', next_ref: '2026-12-01', summary: {income_cent: 0, expense_cent: 800, balance_cent: -800}, budget: {total_cent: null}, series: [], categories: []};
const entriesResponse = {items: [
	{id: 11, category_id: 1, account_id: 2, kind: 'expense', amount_cent: 500, note: '午餐', created_by: 1, occurred_at: '2026-11-03T02:00:00Z'},
	{id: 12, category_id: 1, account_id: 0, kind: 'expense', amount_cent: 300, note: '现金', created_by: 1, occurred_at: '2026-11-03T01:00:00Z'},
], total: 2, page: 1, page_size: 50, summary: {income_cent: 0, expense_cent: 800, balance_cent: -800}};

const flush = () => new Promise(r => setImmediate(r));

function baseWx(storage) {
	return {
		showToast: () => {}, showModal: o => o.success({confirm: true}),
		getStorageSync: k => storage[k], setStorageSync: (k, v) => {storage[k] = v;}, removeStorageSync: () => {},
		navigateTo: () => {}, navigateBack: () => {}, reLaunch: () => {}, switchTab: () => {},
		getWindowInfo: () => ({statusBarHeight: 20, windowWidth: 375}), getMenuButtonBoundingClientRect: () => ({left: 300}),
	};
}

function loadPage(name, handler, options = {}) {
	const file = path.resolve(__dirname, '../pages', name, 'index.js');
	assert.ok(fs.existsSync(file), name + ' page must exist');
	let page; const requests = [];
	const storage = options.storage || {}; const app = {globalData: {pendingEdit: options.pendingEdit || null, undoEntry: null}};
	const api = {
		session: () => options.session === undefined ? {user: {id: 1}} : options.session,
		me: async () => ({id: 1, nickname: '小夏'}),
		config: () => ({baseURL: 'https://example.test'}),
		request: async (...args) => {
			requests.push(args);
			if (handler) { const result = handler(...args); if (result !== undefined) return result; }
			if (args[0] === '/api/v1/categories') return categories;
			if (args[0] === '/api/v1/accounts') return options.noAccounts ? {} : accounts;
			if (args[0] === '/api/v1/ledgers/me') return {id: 1, name: '我的账本', icon: '📒'};
			if (args[0].startsWith('/api/v1/entries?')) return entriesResponse;
			if (args[0].startsWith('/api/v1/stats?')) return statsResponse;
			return {};
		},
	};
	vm.runInNewContext(fs.readFileSync(file, 'utf8'), {
		Page: p => {page = p;}, Component: p => {page = p;},
		require: id => id.endsWith('/utils/api') ? api : require(path.resolve(path.dirname(file), id)),
		wx: baseWx(storage), getApp: () => app,
		getCurrentPages: () => [{route: 'pages/ledger/index'}, {route: 'pages/' + name + '/index'}],
		setTimeout, clearTimeout,
	}, {filename: file});
	page.data = JSON.parse(JSON.stringify(page.data)); page.setData = v => Object.assign(page.data, v);
	return {page, requests, storage, app, api};
}

// —— 记账页：账户选择 ——

test('new entry defaults to the remembered account when it is visible', async () => {
	const {page, storage} = loadPage('entry', null, {storage: {cost_last_account: '2'}});
	await page.onShow(); await flush();
	assert.equal(page.data.accountID, 2);
});

test('new entry falls back to no account without a remembered one', async () => {
	const {page} = loadPage('entry');
	await page.onShow(); await flush();
	assert.equal(page.data.accountID, 0);
});

test('save sends the selected account and remembers it', async () => {
	const {page, requests, storage} = loadPage('entry');
	await page.onShow(); await flush();
	page.setData({categoryID: 1, resultCents: 1200, accountID: 2});
	await page.save(); await flush();
	const post = requests.find(r => r[0] === '/api/v1/entries' && r[1] === 'POST');
	assert.ok(post, 'POST /api/v1/entries expected');
	assert.equal(post[2].account_id, 2);
	assert.equal(storage.cost_last_account, 2);
});

test('editing preselects the original account and PATCH carries it', async () => {
	const pending = {id: 7, category_id: 1, kind: 'expense', amount_cent: 1234, note: '', account_id: 2, occurred_at: '2026-11-03T02:00:00Z'};
	const {page, requests} = loadPage('entry', null, {pendingEdit: pending});
	await page.onLoad({editID: '7'}); await page.onShow(); await flush();
	assert.equal(page.data.accountID, 2);
	page.setData({resultCents: 1500});
	await page.save(); await flush();
	const patch = requests.find(r => r[0] === '/api/v1/entries/7' && r[1] === 'PATCH');
	assert.ok(patch, 'PATCH /api/v1/entries/7 expected');
	assert.equal(patch[2].account_id, 2);
});

test('a disabled original account stays selectable with a disabled label', async () => {
	const pending = {id: 7, category_id: 1, kind: 'expense', amount_cent: 1234, note: '', account_id: 9, occurred_at: '2026-11-03T02:00:00Z'};
	const {page} = loadPage('entry', null, {pendingEdit: pending});
	await page.onLoad({editID: '7'}); await page.onShow(); await flush();
	assert.equal(page.data.accountID, 9);
	const row = page.data.accounts.find(a => a.id === 9);
	assert.ok(row, 'disabled original account must stay visible');
	assert.match(row.label, /已停用/);
});

test('pickAccount switches the selection', async () => {
	const {page} = loadPage('entry');
	await page.onShow(); await flush();
	page.pickAccount({currentTarget: {dataset: {id: '1'}}});
	assert.equal(page.data.accountID, 1);
});

for (const selected of [0, 1]) {
	test('editing preserves selected account ' + selected + ' across refresh and saves that choice', async () => {
		const pending = {id: 7, category_id: 1, kind: 'expense', amount_cent: 1234, note: '', account_id: 2, occurred_at: '2026-11-03T02:00:00Z'};
		const {page, requests} = loadPage('entry', null, {pendingEdit: pending, storage: {cost_last_account: 2}});
		await page.onLoad({editID: '7'}); page.onShow(); await flush();
		page.pickAccount({currentTarget: {dataset: {id: String(selected)}}});
		await page.loadAccounts();
		assert.equal(page.data.accountID, selected);
		await page.save();
		assert.equal(requests.find(r => r[1] === 'PATCH')[2].account_id, selected);
	});
}

test('new entry keeps an explicit no-account choice when a delayed refresh finishes', async () => {
	let finish;
	const {page} = loadPage('entry', url => url === '/api/v1/accounts' && finish === null ? new Promise(resolve => {finish = resolve;}) : undefined, {storage: {cost_last_account: 2}});
	await page.loadAccounts();
	assert.equal(page.data.accountID, 2);
	finish = null;
	const refresh = page.loadAccounts();
	page.pickAccount({currentTarget: {dataset: {id: '0'}}});
	finish(accounts); await refresh;
	assert.equal(page.data.accountID, 0);
});

test('save-and-continue drops the disabled historical account before creating a new entry', async () => {
	const pending = {id: 7, category_id: 1, kind: 'expense', amount_cent: 1234, note: '', account_id: 9, occurred_at: '2026-11-03T02:00:00Z'};
	const {page, requests} = loadPage('entry', null, {pendingEdit: pending});
	await page.onLoad({editID: '7'}); page.onShow(); await flush();
	await page.saveAgain();
	assert.equal(page.data.editID, null);
	assert.equal(page.data.accountID, 0);
	assert.equal(page.data.accounts.some(a => a.id === 9), false);
	page.setData({resultCents: 500}); await page.save();
	assert.equal(requests.find(r => r[1] === 'PATCH')[2].account_id, 9);
	assert.equal(requests.find(r => r[1] === 'POST')[2].account_id, 0);
});

for (const unavailable of ['pending', 'failed']) {
	test('save-and-continue clears an unverified account when the account list is ' + unavailable, async () => {
		let finish;
		const pending = {id: 7, category_id: 1, kind: 'expense', amount_cent: 1234, note: '', account_id: 9, occurred_at: '2026-11-03T02:00:00Z'};
		const {page, requests} = loadPage('entry', url => {
			if (url !== '/api/v1/accounts') return undefined;
			if (unavailable === 'failed') throw new Error('账户暂不可用');
			return new Promise(resolve => {finish = resolve;});
		}, {pendingEdit: pending});
		await page.onLoad({editID: '7'}); page.onShow(); await flush();
		await page.saveAgain();
		assert.equal(page.data.accountID, 0);
		page.setData({resultCents: 500}); await page.save();
		assert.equal(requests.find(r => r[1] === 'PATCH')[2].account_id, 9);
		assert.equal(requests.find(r => r[1] === 'POST')[2].account_id, 0);
		if (finish) { finish(accounts); await flush(); }
		assert.equal(page.data.accountID, 0);
		assert.equal(page.data.accounts.some(a => a.id === 9), false);
	});
}

test('slow edit details still reveal and select a disabled historical account', async () => {
	let finish;
	const pending = {id: 7, category_id: 1, kind: 'expense', amount_cent: 1234, note: '', account_id: 9, occurred_at: '2026-11-03T02:00:00Z'};
	const {page, requests} = loadPage('entry', (url, method) => url === '/api/v1/entries/7' && !method ? new Promise(resolve => {finish = resolve;}) : undefined);
	const loading = page.onLoad({editID: '7'}); page.onShow(); await flush();
	finish(pending); await loading;
	assert.equal(page.data.accountID, 9);
	const original = page.data.accounts.find(a => a.id === 9);
	assert.ok(original, 'late historical account must be visible');
	assert.match(original.label, /已停用/);
	await page.save();
	assert.equal(requests.find(r => r[1] === 'PATCH')[2].account_id, 9);
});

// —— 列表页：明细行带账户名 ——

test('ledger groups carry account names from fetched accounts', async () => {
	const {page} = loadPage('ledger');
	await page.load(); await flush();
	const flat = page.data.groups.flatMap(g => g.items);
	assert.equal(flat[0].accountName, '微信');
	assert.equal(flat[1].accountName, '未指定账户');
});

test('search results carry account names too', async () => {
	const {page, requests} = loadPage('search');
	await page.load(); await flush();
	assert.ok(requests.some(r => r[0] === '/api/v1/accounts'), 'search must fetch accounts');
	const flat = page.data.groups.flatMap(g => g.items);
	assert.equal(flat[0].accountName, '微信');
});

test('calendar day list carries account names too', async () => {
	const {page} = loadPage('calendar');
	await page.onLoad({month: '2026-11'}); await flush();
	await page.tapDay({currentTarget: {dataset: {key: '2026-11-03'}}}); await flush();
	const flat = page.data.dayGroups.flatMap(g => g.items);
	assert.equal(flat[0].accountName, '微信');
	assert.equal(flat[1].accountName, '未指定账户');
});
