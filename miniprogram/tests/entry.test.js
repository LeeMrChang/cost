const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');
const categories = [
	{id: 1, name: '餐饮', kind: 'expense', icon: '🍜', sort_order: 10, disabled: false},
	{id: 2, name: '交通', kind: 'expense', icon: '🚗', sort_order: 20, disabled: false},
	{id: 13, name: '工资', kind: 'income', icon: '💰', sort_order: 10, disabled: false},
	{id: 15, name: '红包', kind: 'income', icon: '🧧', sort_order: 20, disabled: false},
	{id: 14, name: '旧收入', kind: 'income', icon: '📦', sort_order: 30, disabled: true},
];
function load(options = {}, handler) {
	const file = path.resolve(__dirname, '../pages/entry/index.js');
	assert.ok(fs.existsSync(file), 'entry page must exist');
	let page; const requests = [], toasts = [], navigations = [];
	const storage = {}; const app = {globalData: {pendingEdit: options.pendingEdit || null}};
	const api = {request: async (...args) => {
		requests.push(args);
		if (handler) { const result = handler(...args); if (result !== undefined) return result; }
		if (args[0] === '/api/v1/categories') return categories;
		if (args[0] === '/api/v1/ledgers/me') return {id: 1, name: '我的账本', icon: '📒'};
		return {};
	}};
	vm.runInNewContext(fs.readFileSync(file, 'utf8'), {
		Page: p => {page = p;},
		require: id => id.endsWith('/utils/api') ? api : require(path.resolve(path.dirname(file), id)),
		wx: {
			showToast: v => toasts.push(v),
			getStorageSync: k => storage[k],
			setStorageSync: (k, v) => {storage[k] = v;},
			navigateTo: o => navigations.push(o.url),
			navigateBack: () => navigations.push('back'),
			reLaunch: o => navigations.push(o.url),
		},
		getApp: () => app,
		getCurrentPages: () => [{route: 'pages/ledger/index'}, {route: 'pages/entry/index'}],
	}, {filename: file});
	page.data = JSON.parse(JSON.stringify(page.data)); page.setData = v => Object.assign(page.data, v);
	return {page, requests, toasts, navigations, storage, app};
}
const flush = () => new Promise(r => setImmediate(r));
test('complete handles a real tap event by closing instead of saving again', async () => {
 const {page, navigations} = load();
 page.setData({categoryID: 1, resultCents: 1200});
 await page.save({type: 'tap', currentTarget: {dataset: {}}});
 assert.deepEqual(navigations, ['back']);
});
test('editing then saving again creates an independent entry on the next save', async () => {
 const {page, requests} = load();
 page.setData({editID: 88, categoryID: 1, resultCents: 1200});
 await page.saveAgain();
 page.setData({resultCents: 500});
 await page.saveAgain();
 const writes = requests.filter(r => ['POST','PATCH'].includes(r[1]));
 assert.equal(writes[0][1], 'PATCH');
 assert.equal(writes[1][1], 'POST');
 assert.equal(writes[1][0], '/api/v1/entries');
});
test('editing retains a disabled historical category', async () => {
 const {page} = load({pendingEdit: {id: 88, category_id: 14, kind: 'income', amount_cent: 500, occurred_at: '2026-10-26T12:00:00+08:00'}});
 page.onLoad({editID: '88'});
 await page.loadCategories();
 assert.equal(page.data.categoryID, 14);
 assert.ok(page.data.grid.some(c => c.id === 14 && c.selected));
});
test('keypad builds expressions, overrides operators and keeps the last result on divide by zero', () => {
	const {page} = load();
	for (const k of ['1', '2', '+', '8']) page.press({currentTarget: {dataset: {v: k}}});
	assert.equal(page.data.expr, '12+8'); assert.equal(page.data.result, '20.00'); assert.equal(page.data.resultCents, 2000);
	page.press({currentTarget: {dataset: {v: '×'}}});
	page.press({currentTarget: {dataset: {v: '−'}}});
	assert.equal(page.data.expr, '12+8−');
	page.press({currentTarget: {dataset: {v: '⌫'}}});
	page.press({currentTarget: {dataset: {v: '÷'}}});
	page.press({currentTarget: {dataset: {v: '0'}}});
	assert.equal(page.data.expr, '12+8÷0');
	assert.equal(page.data.result, '20.00'); // 除零不崩溃，保持上一结果
});
test('save requires a positive amount and a category before posting', async () => {
	const {page, requests, toasts} = load();
	page.onLoad({}); page.onShow(); await flush();
	await page.save();
	assert.equal(toasts.length, 1); assert.equal(requests.filter(r => r[1] === 'POST').length, 0);
	for (const k of ['1', '2']) page.press({currentTarget: {dataset: {v: k}}});
	await page.save();
	const write = requests.find(r => r[1] === 'POST');
	assert.equal(write[2].amount_cent, 1200); assert.equal(write[2].kind, 'expense'); assert.equal(write[2].category_id, 1);
	assert.match(write[2].occurred_at, /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:00\+08:00$/);
	assert.ok(write[2].client_request_id);
});
test('save remembers the category per kind and closes the editor', async () => {
	const {page, requests, toasts, storage, navigations} = load();
	page.onLoad({}); page.onShow(); await flush();
	for (const k of ['5']) page.press({currentTarget: {dataset: {v: k}}});
	await page.save();
	assert.equal(storage.cost_last_category_expense, 1);
	assert.deepEqual(navigations, ['back']);
	assert.equal(toasts[0].title, '已保存');
	assert.equal(requests.filter(r => r[1] === 'POST').length, 1);
});
test('save-and-continue keeps the editor open, clears the amount and reuses the category', async () => {
	const {page, requests} = load();
	page.onLoad({}); page.onShow(); await flush();
	for (const k of ['1', '2']) page.press({currentTarget: {dataset: {v: k}}});
	await page.saveAgain(); await flush();
	assert.equal(page.data.expr, ''); assert.equal(page.data.resultCents, 0); assert.equal(page.data.categoryID, 1);
	for (const k of ['5']) page.press({currentTarget: {dataset: {v: k}}});
	await page.saveAgain(); await flush();
	const posts = requests.filter(r => r[1] === 'POST');
	assert.equal(posts.length, 2);
	assert.notEqual(posts[0][2].client_request_id, posts[1][2].client_request_id);
	assert.equal(posts[1][2].amount_cent, 500);
});
test('kind switch uses the remembered category of each kind and falls back to the first', async () => {
	const {page, storage} = load();
	page.onLoad({}); page.onShow(); await flush();
	page.switchKind({currentTarget: {dataset: {kind: 'income'}}});
	assert.equal(page.data.categoryID, 13);
	storage.cost_last_category_income = 15;
	page.switchKind({currentTarget: {dataset: {kind: 'expense'}}});
	page.switchKind({currentTarget: {dataset: {kind: 'income'}}});
	assert.equal(page.data.categoryID, 15);
});
test('category grid marks the selection and routes the more cell to management', () => {
	const {page, navigations} = load();
	page.onLoad({});
	page.data.categories = categories;
	page.buildGrid();
	assert.equal(page.data.grid.length, 3); // 两个可用支出分类 + 更多
	assert.equal(page.data.grid[0].selected, false);
	page.pickCategory({currentTarget: {dataset: {id: 1}}});
	assert.equal(page.data.grid[0].selected, true);
	page.pickCategory({currentTarget: {dataset: {more: true, id: -1}}});
	assert.deepEqual(navigations, ['/pages/categories/index']);
});
test('non-M1 chips toast and the note chip toggles the note input', () => {
	const {page, toasts} = load();
	page.onLoad({});
	page.later();
	assert.equal(toasts[0].title, '后续版本支持');
	page.toggleNote();
	assert.equal(page.data.noteOpen, true);
});
test('edit mode prefills from the handoff and patches the existing entry', async () => {
	const pending = {id: 7, category_id: 2, kind: 'expense', amount_cent: 1850, note: '打车', occurred_at: '2026-10-23T02:00:00Z'};
	const {page, requests, app} = load({pendingEdit: pending});
	page.onLoad({editID: '7'});
	assert.equal(app.globalData.pendingEdit, null);
	assert.equal(page.data.expr, '18.50'); assert.equal(page.data.resultCents, 1850);
	assert.equal(page.data.categoryID, 2); assert.equal(page.data.note, '打车');
	assert.equal(page.data.date, '2026-10-23'); assert.equal(page.data.time, '10:00');
	page.onShow(); await flush();
	await page.save();
	const write = requests.find(r => r[1] === 'PATCH');
	assert.equal(write[0], '/api/v1/entries/7');
	assert.equal(write[2].amount_cent, 1850); assert.equal(write[2].category_id, 2);
});
test('deleting all digits cannot save the previous amount', async()=>{
 const {page,requests}=load();page.setData({categoryID:1});
 page.press({currentTarget:{dataset:{v:'5'}}});page.press({currentTarget:{dataset:{v:'⌫'}}});
 await page.save();assert.equal(requests.filter(r=>r[1]==='POST').length,0);assert.equal(page.data.resultCents,0);
});
test('opening an editor without a handoff loads the requested entry instead of creating a blank one',async()=>{
 const {page}=load({},url=>url==='/api/v1/entries/88'?{id:88,category_id:1,kind:'expense',amount_cent:4500,occurred_at:'2026-10-26T12:00:00+08:00'}:undefined);
 await page.onLoad({editID:'88'});
 assert.equal(page.data.editID,88);assert.equal(page.data.resultCents,4500);
});
