const api = require('../../utils/api');
const guestStore = require('../../utils/guest-store');
const guestCategories = require('../../utils/guest-categories');
const { displayMoney } = require('../../utils/money');
const { emptyFilters, query, groups, stamp } = require('../../utils/ledger');
const { avatarView } = require('../../utils/profile');

// 兼容测试替身：只注入 session 的 api mock 也能判断登录态；
// 两者都没声明时按已登录处理（旧测试替身语义）。
function isLoggedIn() {
	if (typeof api.isAuthenticated === 'function') return api.isAuthenticated();
	if (typeof api.session === 'function') return Boolean(api.session());
	return true;
}

Page({
	data: {
		ledgerName: '我的账本', ledgerIcon: '📒', avatarSrc: '',
		statusBarHeight: 20, navRight: 0,
		month: new Date(Date.now() + 8 * 3600000).toISOString().slice(0, 7),
		ref: '', prevRef: '', nextRef: '',
		monthLabel: '',
		summary: {expense: '0.00', income: '0.00', balance: '0.00'},
		budgetSet: false, overspent: false,
		budget: {remaining: '', total: '', daily: '', ratio: 0, barClass: '', dailyShown: false, remainingDays: 0},
		groups: [], entries: [], categories: [],
		loading: true, loadingMore: false, error: '', total: 0, page: 0, hasMore: false,
		collapsed: {}, accounts: [],
		guest: false, guestPending: 0,
	},
	onLoad() {
		const data = {};
		const info = wx.getWindowInfo ? wx.getWindowInfo() : null;
		if (info && info.statusBarHeight) data.statusBarHeight = info.statusBarHeight;
		if (info && info.windowWidth && wx.getMenuButtonBoundingClientRect) {
			const rect = wx.getMenuButtonBoundingClientRect();
			if (rect && rect.left > 0) data.navRight = info.windowWidth - rect.left + 8;
		}
		this.setData(data);
	},
	onShow() { this.setData({groups: [], entries: []}); this.load(); },
	onHide() { this.loadVersion = (this.loadVersion || 0) + 1; },
	onUnload() { this.onHide(); },
	onReachBottom() { this.loadMore(); },
	async load() {
		// 游客模式：未登录时只读本机记录，不触达任何接口。
		if (!isLoggedIn()) { this.setData({ guest: true }); return this.loadGuest(); }
		this.setData({ guest: false });
		const version = this.loadVersion = (this.loadVersion || 0) + 1;
		const current = api.session();
		const cachedAvatarSrc = avatarView(current && current.user, api).avatarSrc;
		this.setData({avatarSrc: cachedAvatarSrc, loading: true, loadingMore: false, error: '', entries: [], groups: [], total: 0, page: 0, hasMore: false});
		try {
			// Avatar refresh must not delay the ledger content or restore a hidden page's image.
			api.me().then(user => {
				if (version === this.loadVersion) this.setData({avatarSrc: avatarView(user, api).avatarSrc});
			}).catch(() => {});
			const ref = this.data.ref || this.data.month + '-01';
			const [categories, stats, ledger, accounts, response] = await Promise.all([
				api.request('/api/v1/categories'),
				api.request('/api/v1/stats?range=month&ref=' + ref),
				api.request('/api/v1/ledgers/me').catch(() => null),
				api.request('/api/v1/accounts').catch(() => []),
				api.request('/api/v1/entries?' + query(emptyFilters(), this.data.month, 1)),
			]);
			const accountList = Array.isArray(accounts) ? accounts : [];
			if (version !== this.loadVersion) return;
			const entries = response.items || [];
			const b = stats.budget || {};
			const s = stats.summary || {};
			const spent = s.expense_cent || 0;
			const budgetSet = b.total_cent !== null && b.total_cent !== undefined;
			const total = budgetSet ? b.total_cent : 0;
			const remaining = budgetSet ? (b.remaining_cent === null || b.remaining_cent === undefined ? 0 : b.remaining_cent) : -spent;
			const pct = budgetSet && total > 0 ? Math.round(((total - remaining) / total) * 100) : 0;
			const balanceCent = (s.balance_cent === null || s.balance_cent === undefined) ? 0 : s.balance_cent;
			const rd = b.remaining_daily_cent;
			const dailyShown = budgetSet && rd !== null && rd !== undefined;
			let remainingDays = 0;
			if (dailyShown) {
				const [y, m] = this.data.month.split('-').map(Number);
				const dim = new Date(y, m, 0).getDate();
				const now = new Date(Date.now() + 8 * 3600000);
				const todayDay = now.getUTCFullYear() === y && now.getUTCMonth() + 1 === m ? now.getUTCDate() : 1;
				remainingDays = dim - todayDay + 1;
			}
			const ledgerName = (ledger && ledger.name) || this.data.ledgerName;
			const ledgerIcon = (ledger && ledger.icon) || this.data.ledgerIcon;
			this.setData({
				categories, entries,
				accounts: accountList,
				groups: groups(entries, categories, accountList),
				total: response.total, page: 1,
				hasMore: entries.length < response.total,
				monthLabel: stats.label || (this.data.month.slice(0, 4) + '年' + Number(this.data.month.slice(5)) + '月'),
				prevRef: stats.prev_ref || '', nextRef: stats.next_ref || '',
				ledgerName, ledgerIcon,
				summary: {
					expense: displayMoney(spent),
					income: displayMoney(s.income_cent || 0),
					balance: displayMoney(Math.abs(balanceCent)),
					balanceNegative: balanceCent < 0,
				},
				budgetSet,
				overspent: budgetSet && remaining < 0,
				budget: {
					remaining: displayMoney(Math.abs(remaining)),
					total: displayMoney(total),
					daily: displayMoney(rd === null || rd === undefined ? 0 : rd),
					ratio: Math.min(100, Math.max(0, pct)),
					barClass: pct >= 100 ? 'over' : pct >= 80 ? 'warn' : '',
					dailyShown,
					remainingDays,
				},
			});
		} catch (e) { if (version === this.loadVersion) this.setData({error: e.message}); }
		finally { if (version === this.loadVersion) this.setData({loading: false}); }
	},
	// 游客态：本地记录按当前月份过滤，预算与账户在后端才有，这里不展示。
	loadGuest() {
		const version = this.loadVersion = (this.loadVersion || 0) + 1;
		const month = this.data.month;
		const categories = guestCategories.list();
		const entries = guestStore.read()
			.filter(item => stamp(item.occurred_at).date.slice(0, 7) === month)
			.sort((a, b) => b.occurred_at.localeCompare(a.occurred_at) || b.local_id - a.local_id);
		let expense = 0, income = 0;
		entries.forEach(item => { if (item.kind === 'income') income += item.amount_cent; else expense += item.amount_cent; });
		const [year, mon] = month.split('-').map(Number);
		const prev = mon === 1 ? (year - 1) + '-12' : year + '-' + String(mon - 1).padStart(2, '0');
		const next = mon === 12 ? (year + 1) + '-01' : year + '-' + String(mon + 1).padStart(2, '0');
		if (version !== this.loadVersion) return;
		this.setData({
			guest: true, guestPending: guestStore.pending(),
			entries, categories, accounts: [],
			groups: groups(entries, categories, []),
			total: entries.length, page: 1, hasMore: false,
			monthLabel: year + '年' + mon + '月',
			prevRef: prev + '-01', nextRef: next + '-01',
			summary: {
				expense: displayMoney(expense),
				income: displayMoney(income),
				balance: displayMoney(Math.abs(income - expense)),
				balanceNegative: income - expense < 0,
			},
			budgetSet: false, overspent: false,
			avatarSrc: '', ledgerName: '我的账本', ledgerIcon: '📒',
			error: '', loading: false,
		});
	},
	goLogin() { wx.navigateTo({ url: '/pages/login/index' }); },
	onAvatarError(event) {
		if (event.currentTarget.dataset.src === this.data.avatarSrc) this.setData({avatarSrc: ''});
	},
	async loadMore() {
		if (this.data.loading || this.data.loadingMore || !this.data.hasMore) return;
		const version = this.loadVersion, next = this.data.page + 1;
		this.setData({loadingMore: true, error: ''});
		try {
			const response = await api.request('/api/v1/entries?' + query(emptyFilters(), this.data.month, next));
			if (version !== this.loadVersion) return;
			const map = {}; this.data.entries.concat(response.items || []).forEach(e => {map[e.id] = e;});
			const entries = Object.values(map).sort((a, b) => b.occurred_at.localeCompare(a.occurred_at) || b.id - a.id);
			this.setData({entries, groups: groups(entries, this.data.categories, this.data.accounts), total: response.total, page: next, hasMore: entries.length < response.total});
		} catch (e) { if (version === this.loadVersion) this.setData({error: e.message}); }
		finally { if (version === this.loadVersion) this.setData({loadingMore: false}); }
	},
	shiftMonth(e) {
		const ref = e.currentTarget.dataset.ref;
		if (!ref) return;
		this.setData({ref, month: ref.slice(0, 7)});
		this.load();
	},
	toggleGroup(e) {
		const date = e.currentTarget.dataset.date;
		this.setData({collapsed: {...this.data.collapsed, [date]: !this.data.collapsed[date]}});
	},
	openSearch() { wx.navigateTo({url: '/pages/search/index'}); },
	openCalendar() { wx.navigateTo({url: '/pages/calendar/index?month=' + this.data.month}); },
	openBudget() { wx.navigateTo({url: '/pages/budget/index?scope=month&period=' + this.data.month}); },
	edit(e) {
		const id = Number(e.currentTarget.dataset.id);
		wx.navigateTo({url: '/pages/entry-detail/index?id=' + id});
	},
	add() { getApp().globalData.pendingEdit = null; wx.navigateTo({url: '/pages/entry/index'}); },
	goSettings() { wx.navigateTo({url: '/pages/settings/index'}); },
});
