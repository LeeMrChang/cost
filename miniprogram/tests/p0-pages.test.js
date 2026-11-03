const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');
const categories = [{id: 1, name: '餐饮', kind: 'expense', icon: '🍚', sort_order: 10, disabled: false}, {id: 2, name: '旧分类', kind: 'expense', icon: '📦', sort_order: 20, disabled: true}, {id: 3, name: '工资', kind: 'income', icon: '💰', sort_order: 10, disabled: false}];
const accounts = [{id: 1, name: '现金', type: 'cash', opening_balance_cent: 0, balance_cent: 100, disabled: false}, {id: 2, name: '旧账户', type: 'bank', opening_balance_cent: -100, balance_cent: -100, disabled: true}, {id: 3, name: '微信', type: 'wechat', opening_balance_cent: 0, balance_cent: 0, disabled: false}];
const entry = {id: 1, category_id: 2, account_id: 2, kind: 'expense', amount_cent: 1234, note: '旧账', created_by: 1, occurred_at: '2026-10-23T02:00:00Z'};
const response = {items: [entry], total: 51, page: 1, page_size: 50, summary: {income_cent: 20000, expense_cent: 1234, balance_cent: 18766}};
const statsResponse = {range: 'month', label: '2026年10月', start: '2026-10-01', end: '2026-10-31', prev_ref: '2026-09-01', next_ref: '2026-11-01', summary: {income_cent: 20000, expense_cent: 1234, balance_cent: 18766, daily_avg_expense_cent: 45}, budget: {total_cent: 30000, remaining_cent: 28766, remaining_daily_cent: 1000}, series: [], categories: []};
function load(name, handler, role = 'admin') {
	const file = path.resolve(__dirname, '../pages', name, 'index.js');
	assert.ok(fs.existsSync(file), name + ' page must exist');
	let page; const requests = [], toasts = [], navigations = [];
	const app = {globalData: {pendingEdit: null}};
	const api = {session: () => ({user: {id: 1, role}}), me: async () => ({id: 1, role}), exportCSV: async q => requests.push(['export', q]), request: async (...args) => {
		requests.push(args); if (handler) { const result = handler(...args); if (result !== undefined) return result; }
		if (args[0] === '/api/v1/categories') return categories;
		if (args[0] === '/api/v1/accounts') return accounts;
		if (args[0] === '/api/v1/entry-authors') return [{id: 0}, {id: 1}];
		if (args[0] === '/api/v1/ledgers/me') return {id: 1, name: '我的账本', icon: '📒'};
		if (args[0].startsWith('/api/v1/entries?')) return response;
		if (args[0].startsWith('/api/v1/transfers?')) return {items: [], total: 0, page: 1, page_size: 50};
		if (args[0].startsWith('/api/v1/stats?')) return statsResponse;
		return {};
	}};
	vm.runInNewContext(fs.readFileSync(file, 'utf8'), {Page: p => {page = p;}, require: id => id.endsWith('/utils/api') ? api : require(path.resolve(path.dirname(file), id)), wx: {showToast: v => toasts.push(v), showModal: opts => opts.success({confirm: true}), navigateTo: o => navigations.push(o.url), navigateBack: () => navigations.push('back'), reLaunch: o => navigations.push(o.url)}, getApp: () => app, getCurrentPages: () => [{route: 'pages/ledger/index'}], setTimeout, clearTimeout}, {filename: file});
	page.data = JSON.parse(JSON.stringify(page.data)); page.setData = v => Object.assign(page.data, v);
	return {page, requests, toasts, navigations, app};
}
const flush = () => new Promise(r => setImmediate(r));
test('ledger loads month groups and renders the documented budget card', async () => {
	const {page, requests} = load('ledger');
	await page.load(); await flush();
	const q = new URL('https://example.test' + requests.find(r => r[0].startsWith('/api/v1/entries?'))[0]).searchParams;
	assert.equal(q.get('month'), page.data.month); assert.equal(q.get('page'), '1'); assert.equal(q.get('page_size'), '50');
	assert.equal(page.data.budgetSet, true); assert.equal(page.data.overspent, false);
 assert.equal(page.data.summary.expense,'12.34');assert.equal(page.data.summary.income,'200.00');assert.equal(page.data.summary.balance,'187.66');
	assert.equal(page.data.budget.remaining, '287.66'); assert.equal(page.data.budget.total, '300.00'); assert.equal(page.data.budget.daily, '10.00');
	assert.equal(page.data.budget.ratio, 4); assert.equal(page.data.budget.barClass, ''); assert.equal(page.data.budget.dailyShown, true);
	assert.ok(page.data.budget.remainingDays >= 1 && page.data.budget.remainingDays <= 31);
	assert.equal(page.data.monthLabel, '2026年10月'); assert.equal(page.data.prevRef, '2026-09-01'); assert.equal(page.data.nextRef, '2026-11-01');
	assert.equal(page.data.groups.length, 1); assert.match(page.data.groups[0].display, /10\/23 星期五/);
	assert.equal(page.data.groups[0].total, '12.34');
});
test('ledger marks actual overspending and guides setup when the budget is unset', async () => {
	const overspent = load('ledger', url => url.startsWith('/api/v1/stats?') ? {...statsResponse, budget: {total_cent: 1000, remaining_cent: -234, remaining_daily_cent: null}} : undefined);
	await overspent.page.load(); await flush();
	assert.equal(overspent.page.data.overspent, true); assert.equal(overspent.page.data.budget.remaining, '2.34'); assert.equal(overspent.page.data.budget.total, '10.00'); assert.equal(overspent.page.data.budget.daily, '0.00');
	assert.equal(overspent.page.data.budget.ratio, 100); assert.equal(overspent.page.data.budget.barClass, 'over');
	const unset = load('ledger', url => url.startsWith('/api/v1/stats?') ? {...statsResponse, budget: {total_cent: null, remaining_cent: null, remaining_daily_cent: null}} : undefined);
	await unset.page.load(); await flush();
	assert.equal(unset.page.data.budgetSet, false);
	assert.equal(unset.page.data.overspent, false);
	assert.equal(unset.page.data.budget.remaining, '12.34'); assert.equal(unset.page.data.budget.total, '0.00'); assert.equal(unset.page.data.budget.daily, '0.00');
});
test('ledger shifts months via stats refs and opens the calendar from the month title', async () => {
	const {page, navigations} = load('ledger'); await page.load(); await flush();
	page.shiftMonth({currentTarget: {dataset: {ref: '2026-09-01'}}});
	assert.equal(page.data.month, '2026-09'); assert.equal(page.data.ref, '2026-09-01');
	page.openCalendar();
	assert.deepEqual(navigations[navigations.length - 1], '/pages/calendar/index?month=2026-09');
});
test('ledger collapses day groups and hands the entry to the keypad editor', async () => {
	const {page, navigations, app} = load('ledger'); await page.load(); await flush();
	const date = page.data.groups[0].date;
	page.toggleGroup({currentTarget: {dataset: {date}}});
	assert.equal(page.data.collapsed[date], true);
	page.toggleGroup({currentTarget: {dataset: {date}}});
	assert.equal(page.data.collapsed[date], false);
	page.edit({currentTarget: {dataset: {id: 1}}});
	assert.deepEqual(navigations, ['/pages/entry-detail/index?id=1']);
	page.add();
	assert.equal(app.globalData.pendingEdit, null);
	assert.deepEqual(navigations[navigations.length - 1], '/pages/entry/index');
});
test('ledger paginates without duplicate entries', async () => {
	const {page} = load('ledger', url => url.includes('page=2') ? {...response, page: 2, items: [{...entry, id: 2}]} : undefined);
	await page.load(); await flush();
	await page.loadMore(); await flush();
	assert.equal(page.data.groups.flatMap(g => g.items).length, 2);
	assert.equal(page.data.page, 2); assert.equal(page.data.total, 51);
});
test('ledger ignores old completed responses without clearing newer loading state', async () => {
	const pending = []; const {page} = load('ledger', url => url.startsWith('/api/v1/entries?') ? new Promise(resolve => pending.push(resolve)) : undefined);
	const first = page.load(); const second = page.load(); await Promise.resolve();
	pending[0]({...response, total: 99}); await first; assert.equal(page.data.loading, true);
	pending[1]({...response, total: 2}); await second; await flush();
	assert.equal(page.data.total, 2); assert.equal(page.data.loading, false);
});
test('failed fresh ledger queries clear the prior result', async () => {
	let broken = false; const {page} = load('ledger', url => broken && url.startsWith('/api/v1/entries?') ? Promise.reject(new Error('offline')) : undefined);
	await page.load(); await flush();
	broken = true; await page.load(); await flush();
	assert.equal(page.data.groups.length, 0); assert.equal(page.data.total, 0); assert.match(page.data.error, /offline/);
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
test('management pages clear previous data when a fresh authenticated load fails', async () => {
	for (const name of ['accounts', 'categories']) {
		let fail = false; const {page} = load(name, url => fail ? Promise.reject(new Error('登录过期')) : undefined);
		await page.load(); fail = true; await page.load();
		assert.equal(page.data.user, null); assert.equal(page.data[name].length, 0);
	}
});
test('ledger opens the search page from the search bar', async () => {
	const {page, navigations} = load('ledger'); await page.load(); await flush();
	page.openSearch();
	assert.deepEqual(navigations[navigations.length - 1], '/pages/search/index');
});
test('ledger shows the ledger icon avatar and opens the calendar', async () => {
	const {page, navigations} = load('ledger'); await page.load(); await flush();
	assert.equal(page.data.ledgerIcon, '📒'); assert.equal(page.data.ledgerName, '我的账本');
	page.openCalendar();
	assert.deepEqual(navigations[navigations.length - 1], '/pages/calendar/index?month=' + page.data.month);
});
test('calendar builds a sunday-start month grid with red expense and green income', async () => {
	const {page} = load('calendar', url => url.startsWith('/api/v1/stats?') ? {...statsResponse, series: [{key: '2026-10-23', expense_cent: 1234, income_cent: 500}]} : undefined);
	await page.onLoad({month: '2026-10'}); await flush();
	assert.equal(page.data.cells.length, 35);
	assert.equal(page.data.cells[0].day, 0);
	assert.equal(page.data.cells[4].day, 1);
	assert.equal(page.data.cells[34].day, 31);
	const cell = page.data.cells[26];
	assert.equal(cell.key, '2026-10-23'); assert.equal(cell.expense, '12.34'); assert.equal(cell.income, '+5');
	assert.equal(cell.hasExpense, true); assert.equal(cell.hasIncome, true);
	assert.equal(page.data.summary.expense, '12.34'); assert.equal(page.data.summary.income, '200.00');
});
test('calendar expands the tapped day with its entries and shifts months', async () => {
	const {page, requests} = load('calendar',url=>url.startsWith('/api/v1/entries?')?{...response,total:1}:undefined);
	await page.onLoad({month: '2026-10'}); await flush();
	page.tapDay({currentTarget: {dataset: {key: '2026-10-23'}}}); await flush(); await flush();
	const url = requests.find(r => r[0].startsWith('/api/v1/entries?start_date=2026-10-23'))[0];
	const q = new URL('https://example.test' + url).searchParams;
	assert.equal(q.get('start_date'), '2026-10-23'); assert.equal(q.get('end_date'), '2026-10-23');
	assert.equal(page.data.selected, '2026-10-23');
	assert.match(page.data.selectedDisplay, /10月23日/);
	assert.equal(page.data.dayGroups.length, 1); assert.equal(page.data.dayGroups[0].items.length, 1);
	page.prevMonth(); await flush();
	assert.equal(page.data.month, '2026-09');
	page.nextMonth(); await flush();
	assert.equal(page.data.month, '2026-11');
});
test('search opens empty and only queries all time after explicit search', async () => {
	const {page, requests} = load('search');
	await page.onLoad(); await flush();
 assert.equal(requests.length, 0);
 assert.equal(page.data.searched, false);
 assert.equal(page.data.focus, true);
 await page.load();
	const url = requests.find(r => r[0].startsWith('/api/v1/entries?'))[0];
	const q = new URL('https://example.test' + url).searchParams;
	assert.equal(q.get('start_date'), '2000-01-01'); assert.equal(q.get('end_date'), '9999-12-30');
	assert.equal(q.get('page'), '1'); assert.equal(q.get('page_size'), '50'); assert.equal(q.has('month'), false);
	assert.equal(page.data.summary.total, 51);
	assert.equal(page.data.summary.expense, '12.34'); assert.equal(page.data.summary.income, '200.00');
	assert.equal(page.data.groups.length, 1);
});
test('search debounces input and switches kind chips', async () => {
	const {page, requests} = load('search');
	await page.onLoad(); await flush();
	assert.equal(page.data.chips.length, 10);
	assert.equal(page.data.chips.filter(c => c.pending).length, 7);
	page.onInput({detail: {value: '咖'}});
	page.onInput({detail: {value: '咖啡'}});
	await new Promise(r => setTimeout(r, 450)); await flush();
	let url = requests.find(r => r[0].startsWith('/api/v1/entries?') && r[0].includes('q='))[0];
	assert.equal(new URL('https://example.test' + url).searchParams.get('q'), '咖啡');
	page.setKind({currentTarget: {dataset: {value: 'expense'}}}); await flush();
	url = requests.filter(r => r[0].startsWith('/api/v1/entries?') && r[0].includes('kind=expense')).at(-1)[0];
	assert.ok(url.includes('q=' + encodeURIComponent('咖啡')));
	page.setKind({currentTarget: {dataset: {value: 'refund'}}});
	assert.equal(page.data.kind, 'expense');
	page.clearInput(); await flush();
	assert.equal(page.data.keyword, '');
});
test('search opens details for a tapped entry', async () => {
	const {page, navigations, app} = load('search');
	await page.onLoad(); await page.load(); await flush();
	page.edit({currentTarget: {dataset: {id: 1}}});
	assert.deepEqual(navigations, ['/pages/entry-detail/index?id=1']);
	page.goBack();
	assert.equal(navigations[navigations.length - 1], 'back');
});

test('calendar ignores the old date request when a later selection finishes first', async () => {
 let first, second;
 const {page}=load('calendar',url=>url.startsWith('/api/v1/entries?')?new Promise(resolve=>{if(url.includes('2026-10-22')) first=resolve;else second=resolve;}):undefined);
 await page.onLoad({month:'2026-10'});
 const a=page.tapDay({currentTarget:{dataset:{key:'2026-10-22'}}});
 const b=page.tapDay({currentTarget:{dataset:{key:'2026-10-23'}}});
 second({items:[entry],total:1});await b;
 first({items:[],total:0});await a;
 assert.equal(page.data.selected,'2026-10-23');
 assert.equal(page.data.dayGroups.length,1);
});
test('clearing search invalidates an outstanding result', async () => {
 let resolve;
 const {page}=load('search',url=>url.startsWith('/api/v1/entries?')?new Promise(r=>resolve=r):undefined);
 page.onLoad();page.data.keyword='coffee';const pending=page.load();
 page.clearInput();resolve(response);await pending;
 assert.equal(page.data.searched,false);assert.equal(page.data.groups.length,0);
});
test('search refresh clears cancelled pagination and can resume loading',async()=>{
 const {page}=load('search');page.onLoad();await page.load();
 page.setData({loadingMore:true});page.onHide();page.onShow();await flush();
 assert.equal(page.data.loadingMore,false);assert.equal(page.data.hasMore,true);
});
test('search resumes a query interrupted by hide before the first result',async()=>{
 let first;let calls=0;
 const {page}=load('search',url=>url.startsWith('/api/v1/entries?')&&calls++===0?new Promise(r=>first=r):undefined);
 page.onLoad();page.data.keyword='coffee';const pending=page.load();
 page.onHide();page.onShow();await flush();
 first(response);await pending;
 assert.equal(page.data.searched,true);assert.equal(page.data.loading,false);
});

// The ledger page refreshes its identity card when returning from settings.
test('ledger onShow refreshes a renamed ledger and its icon', async () => {
 let ledger = {id: 1, name: '我的账本', icon: '📒'};
 const {page} = load('ledger', url => url === '/api/v1/ledgers/me' ? ledger : undefined);
 page.onShow(); await flush();
 assert.equal(page.data.ledgerName, '我的账本');
 ledger = {id: 1, name: '旅行基金', icon: '✈️'};
 page.onShow(); await flush();
 assert.equal(page.data.ledgerName, '旅行基金'); assert.equal(page.data.ledgerIcon, '✈️');
});
