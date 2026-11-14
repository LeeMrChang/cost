const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');
const categories = [{id: 1, name: '餐饮', kind: 'expense', icon: '🍚', sort_order: 10, disabled: false}, {id: 2, name: '旧分类', kind: 'expense', icon: '📦', sort_order: 20, disabled: true}, {id: 3, name: '工资', kind: 'income', icon: '💰', sort_order: 10, disabled: false}];
const statsResponse = {range: 'month', label: '2026年10月', start: '2026-10-01', end: '2026-10-31', prev_ref: '2026-09-01', next_ref: '2026-11-01', summary: {income_cent: 20000, expense_cent: 1234, balance_cent: 18766, daily_avg_expense_cent: 45}, budget: {total_cent: 30000, remaining_cent: 28766, remaining_daily_cent: 1000}, series: [{key: '2026-10-01', expense_cent: 500, income_cent: 0}, {key: '2026-10-02', expense_cent: 1000, income_cent: 100}], categories: [{category_id: 1, amount_cent: 617, count: 2}]};
const today = () => new Date().getFullYear() + '-' + String(new Date().getMonth() + 1).padStart(2, '0') + '-' + String(new Date().getDate()).padStart(2, '0');
function load(name, handler, extra = {}) {
	const file = path.resolve(__dirname, '../pages', name, 'index.js');
	assert.ok(fs.existsSync(file), name + ' page must exist');
	let page; const requests = [], toasts = [], navigations = [];
	const api = {
		session: () => null, isAuthenticated: () => true, me: async () => ({id: 1, nickname: '小夏'}),
		login: async nickname => { requests.push(['login', nickname]); return {id: 1, nickname: nickname || '小夏'}; },
		logout: async () => { requests.push(['logout']); },
		isDevelop: () => true, config: () => ({baseURL: 'https://example.test'}), clearSession: () => {},
		request: async (...args) => {
			requests.push(args); if (handler) { const result = handler(...args); if (result !== undefined) return result; }
			if (args[0] === '/api/v1/categories') return categories;
			if (args[0] === '/api/v1/ledgers/me') return {id: 1, name: '我的账本', icon: '📒'};
			if (args[0].startsWith('/api/v1/stats?')) return statsResponse;
			return {};
		}, ...extra,
	};
	vm.runInNewContext(fs.readFileSync(file, 'utf8'), {Page: p => {page = p;}, require: id => id.endsWith('/utils/api') ? api : require(path.resolve(path.dirname(file), id)), wx: {showToast: v => toasts.push(v), showModal: opts => opts.success({confirm: true}), navigateTo: o => navigations.push(o.url), navigateBack: () => navigations.push('back'), reLaunch: o => navigations.push(o.url), switchTab: o => navigations.push(o.url), getStorageSync: () => '', setStorageSync: () => {}, removeStorageSync: () => {}, getAccountInfoSync: () => ({miniProgram: {envVersion: 'develop'}})}, getApp: () => ({globalData: {}}), getCurrentPages: () => [{route: 'pages/ledger/index'}]}, {filename: file});
	page.data = JSON.parse(JSON.stringify(page.data)); page.setData = v => Object.assign(page.data, v);
	return {page, requests, toasts, navigations};
}
const flush = () => new Promise(r => setImmediate(r));
const query = (requests, prefix) => new URL('https://example.test' + requests.filter(r => r[0].startsWith(prefix)).at(-1)[0]).searchParams;
test('stats month view renders metrics, chart, ranking and the budget strip', async () => {
	const {page, requests} = load('stats');
	await page.load(); await flush();
	const q = query(requests, '/api/v1/stats?');
	assert.equal(q.get('range'), 'month'); assert.equal(q.get('ref'), today());
	assert.equal(page.data.label, '2026年10月');
	assert.equal(page.data.summary.expense, '12.34'); assert.equal(page.data.summary.income, '200.00');
	assert.equal(page.data.summary.balance, '187.66'); assert.equal(page.data.summary.daily, '0.45');
	assert.equal(page.data.chart.length, 2); assert.equal(page.data.chart[0].height, 50); assert.equal(page.data.chart[0].hot, false);
	assert.equal(page.data.chart[1].height, 100); assert.equal(page.data.chart[1].hot, true); assert.equal(page.data.chart[1].label, '02');
	assert.equal(page.data.categories[0].percent, '50.0'); assert.equal(page.data.categories[0].width, 50); assert.equal(page.data.categories[0].amount, '6.17');
	assert.equal(page.data.categories[0].name, '餐饮'); assert.equal(page.data.categories[0].icon, '🍚');
	assert.equal(page.data.budget.remaining, '287.66'); assert.equal(page.data.budget.total, '300.00'); assert.equal(page.data.budget.daily, '10.00');
	assert.equal(page.data.budget.over, false);
});
test('stats chart thins axis labels for dense series and limits per-bar values', async () => {
	const daily = Array.from({length: 31}, (_, i) => ({key: '2026-10-' + String(i + 1).padStart(2, '0'), expense_cent: i === 15 ? 500 : 0, income_cent: 0}));
	const withSeries = series => load('stats', url => url.startsWith('/api/v1/stats?') ? {...statsResponse, series} : undefined);
	const month = withSeries(daily);
	await month.page.load(); await flush();
	assert.equal(month.page.data.dense, true); assert.equal(month.page.data.chart.length, 31);
	assert.deepEqual(month.page.data.chart.flatMap((p, i) => p.showLabel ? [i] : []), [0, 10, 20, 30]);
	const week = withSeries(daily.slice(0, 7));
	week.page.setRange({currentTarget: {dataset: {range: 'week'}}}); await flush();
	assert.equal(week.page.data.dense, false); assert.equal(week.page.data.showValues, true);
	assert.equal(week.page.data.chart.every(p => p.showLabel), true);
	const year = withSeries(Array.from({length: 12}, (_, i) => ({key: '2026-' + String(i + 1).padStart(2, '0'), expense_cent: i * 100, income_cent: 0})));
	year.page.setRange({currentTarget: {dataset: {range: 'year'}}}); await flush();
	assert.equal(year.page.data.dense, false); assert.equal(year.page.data.showValues, false);
	assert.equal(year.page.data.chart.every(p => p.showLabel), true);
});
test('stats overspent and unset budget views plus range switching via refs', async () => {
	const over = load('stats', url => url.startsWith('/api/v1/stats?') ? {...statsResponse, budget: {total_cent: 1000, remaining_cent: -234, remaining_daily_cent: null}} : undefined);
	await over.page.load(); await flush();
	assert.equal(over.page.data.budget.over, true); assert.equal(over.page.data.budget.remaining, '2.34'); assert.equal(over.page.data.budget.daily, '');
	const unset = load('stats', url => url.startsWith('/api/v1/stats?') ? {...statsResponse, budget: {total_cent: null, remaining_cent: null, remaining_daily_cent: null}} : undefined);
	await unset.page.load(); await flush();
	assert.equal(unset.page.data.budget.unset, true);
	const {page, requests} = load('stats');
	await page.load(); await flush();
	page.setRange({currentTarget: {dataset: {range: 'week'}}});
	await flush();
	assert.equal(query(requests, '/api/v1/stats?').get('range'), 'week');
	page.prev(); await flush();
	assert.equal(query(requests, '/api/v1/stats?').get('ref'), '2026-09-01');
	page.next(); await flush();
	assert.equal(query(requests, '/api/v1/stats?').get('ref'), '2026-11-01');
});
test('stats custom range auto-fills the current month and accepts picked dates', async () => {
	const {page, requests} = load('stats');
	await page.load(); await flush();
	page.setRange({currentTarget: {dataset: {range: 'custom'}}});
	await flush();
	assert.equal(page.data.customStart, today().slice(0, 7) + '-01'); assert.equal(page.data.customEnd, today());
	const q = query(requests, '/api/v1/stats?');
	assert.equal(q.get('range'), 'custom'); assert.equal(q.get('start_date'), today().slice(0, 7) + '-01'); assert.equal(q.get('end_date'), today());
	page.onStart({detail: {date: '2026-10-01'}}); await flush();
	assert.equal(query(requests, '/api/v1/stats?').get('start_date'), '2026-10-01');
	page.onEnd({detail: {date: '2026-10-31'}}); await flush();
	assert.equal(query(requests, '/api/v1/stats?').get('end_date'), '2026-10-31');
});
test('stats budget entry opens the budget page for the viewed month', async () => {
	const {page, navigations} = load('stats');
	await page.load(); await flush();
	page.openBudget();
	assert.deepEqual(navigations, ['/pages/budget/index?period=' + today().slice(0, 7)]);
});
const budgetsResponse = {scope: 'month', period: '2026-10', total_cent: 30000, categories: [{category_id: 1, name: '餐饮', icon: '🍚', amount_cent: 20000, spent_cent: 12000}]};
test('budget page loads total card and per-category rows for expense categories only', async () => {
	const {page, requests} = load('budget', url => url.startsWith('/api/v1/budgets?') ? budgetsResponse : undefined);
	page.onLoad({period: '2026-10'}); await page.load(); await flush();
	const q = query(requests, '/api/v1/budgets?');
	assert.equal(q.get('scope'), 'month'); assert.equal(q.get('period'), '2026-10');
	assert.equal(page.data.label, '2026年10月'); assert.equal(page.data.totalCent, 30000);
	assert.equal(page.data.totalDisplay, '300.00'); assert.equal(page.data.spent, '12.34');
	assert.equal(page.data.remaining, '287.66'); assert.equal(page.data.over, false); assert.equal(page.data.progress, 4);
	assert.equal(page.data.daily, '10.00');
	assert.equal(page.data.rows.length, 2);
	assert.equal(page.data.rows[0].id, 1); assert.equal(page.data.rows[0].set, true); assert.equal(page.data.rows[0].amount, '200.00'); assert.equal(page.data.rows[0].spent, '120.00'); assert.equal(page.data.rows[0].overSpent, false);
	assert.equal(page.data.rows[1].set, false);
});
test('budget saves and clears total and category amounts via PUT with amount 0 deletion', async () => {
	const {page, requests, toasts} = load('budget', url => url.startsWith('/api/v1/budgets?') ? budgetsResponse : undefined);
	page.onLoad({period: '2026-10'}); await page.load(); await flush();
	page.editTotal(); assert.equal(page.data.totalEditor, true); assert.equal(page.data.totalInput, '300.00');
	page.data.totalInput = '500'; await page.save(); await flush();
	let put = requests.filter(r => r[0] === '/api/v1/budgets' && r[1] === 'PUT').at(-1);
	assert.equal(put[2].scope, 'month'); assert.equal(put[2].period, '2026-10'); assert.equal(put[2].category_id, 0); assert.equal(put[2].amount_cent, 50000);
	assert.ok(toasts.some(t => t.title === '预算已保存'));
	page.editRow({currentTarget: {dataset: {id: 1}}}); assert.equal(page.data.editingID, 1); assert.equal(page.data.editInput, '200.00');
	page.data.editInput = '100'; await page.save(); await flush();
	put = requests.filter(r => r[0] === '/api/v1/budgets' && r[1] === 'PUT').at(-1);
	assert.equal(put[2].category_id, 1); assert.equal(put[2].amount_cent, 10000);
	page.editRow({currentTarget: {dataset: {id: 1}}}); await page.clear(); await flush();
	put = requests.filter(r => r[0] === '/api/v1/budgets' && r[1] === 'PUT').at(-1);
	assert.equal(put[2].category_id, 1); assert.equal(put[2].amount_cent, 0);
});
test('budget rejects empty amounts and marks overspent rows and months', async () => {
	const over = load('budget', url => url.startsWith('/api/v1/budgets?') ? {...budgetsResponse, total_cent: 1000, categories: [{category_id: 1, name: '餐饮', icon: '🍚', amount_cent: 1000, spent_cent: 1200}]} : undefined);
	over.page.onLoad({period: '2026-10'}); await over.page.load(); await flush();
	assert.equal(over.page.data.over, true); assert.equal(over.page.data.rows[0].overSpent, true);
	const {page, requests, toasts} = load('budget', url => url.startsWith('/api/v1/budgets?') ? budgetsResponse : undefined);
	page.onLoad({period: '2026-10'}); await page.load(); await flush();
	page.editTotal(); page.data.totalInput = ''; await page.save();
	assert.ok(toasts.some(t => /金额/.test(t.title)));
	assert.equal(requests.filter(r => r[1] === 'PUT').length, 0);
});
test('budget shifts months and defaults to the current month without options', async () => {
	const {page} = load('budget', url => url.startsWith('/api/v1/budgets?') ? {...budgetsResponse, period: new URL('https://x.test' + url).searchParams.get('period')} : undefined);
	page.onLoad({period: '2026-10'}); await page.load(); await flush();
	page.prev(); await flush();
	assert.equal(page.data.period, '2026-09'); assert.equal(page.data.label, '2026年9月');
	page.next(); await flush();
	assert.equal(page.data.period, '2026-10');
	const current = load('budget'); current.page.onLoad({}); await current.page.load(); await flush();
	assert.equal(current.page.data.period, today().slice(0, 7));
});
test('login reuses a known account nickname and enters the ledger directly', async () => {
	const {page, requests, navigations} = load('login');
	await page.signIn(); await flush();
	assert.deepEqual(navigations, ['/pages/ledger/index']);
	assert.ok(requests.some(r => r[0] === 'login' && r[1] === undefined));
	assert.equal(page.data.invite, undefined); assert.equal(page.data.secret, undefined);
});
test('login restores a session and skips the login button', async () => {
	const {page, navigations} = load('login', undefined, {session: () => ({user: {id: 1}})});
	await page.onShow(); await flush();
	assert.deepEqual(navigations, ['/pages/ledger/index']);
});
test('settings shows my ledger without bill export and signs out', async () => {
	const {page, requests} = load('settings');
	await page.load(); await flush();
	assert.equal(page.data.user.id, 1); assert.equal(page.data.ledger.name, '我的账本'); assert.equal(page.data.ledger.icon, '📒');
	assert.equal(page.exportCSV, undefined);
	await page.signOut();
	assert.equal(page.data.user, null); assert.equal(page.data.ledger, null);
	assert.ok(requests.some(r => r[0] === 'logout'));
});

test('budget unset categories still show their real spending', async () => {
 const {page}=load('budget',url=>url.startsWith('/api/v1/budgets?')?{...budgetsResponse,categories:[{category_id:1,amount_cent:0,spent_cent:8000,is_set:false}]}:undefined);
 page.onLoad({period:'2026-10'});await page.load();
 assert.equal(page.data.rows[0].set,false);
 assert.equal(page.data.rows[0].spent,'80.00');
});
