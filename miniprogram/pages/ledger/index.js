const api = require('../../utils/api');
const { money } = require('../../utils/money');
const { emptyFilters, query, groups } = require('../../utils/ledger');

Page({
	data: {
		ledgerName: '我的账本', ledgerIcon: '📒',
		statusBarHeight: 20,
		month: new Date(Date.now() + 8 * 3600000).toISOString().slice(0, 7),
		ref: '', prevRef: '', nextRef: '',
		monthLabel: '',
		summary: {expense: '0.00', income: '0.00', balance: '0.00'},
		budgetSet: false, overspent: false,
		budget: {remaining: '', total: '', daily: '', ratio: 0, barClass: '', dailyShown: false, remainingDays: 0},
		groups: [], entries: [], categories: [],
		loading: true, loadingMore: false, error: '', total: 0, page: 0, hasMore: false,
		collapsed: {},
	},
	onLoad() {
		if (wx.getWindowInfo) {
			const info = wx.getWindowInfo();
			if (info.statusBarHeight) this.setData({statusBarHeight: info.statusBarHeight});
		}
	},
	onShow() { this.setData({groups: [], entries: []}); this.load(); },
	onHide() { this.loadVersion = (this.loadVersion || 0) + 1; },
	onUnload() { this.onHide(); },
	onReachBottom() { this.loadMore(); },
	async load() {
		const version = this.loadVersion = (this.loadVersion || 0) + 1;
		this.setData({loading: true, loadingMore: false, error: '', entries: [], groups: [], total: 0, page: 0, hasMore: false});
		try {
			const ref = this.data.ref || this.data.month + '-01';
			const [categories, stats, ledger, response] = await Promise.all([
				api.request('/api/v1/categories'),
				api.request('/api/v1/stats?range=month&ref=' + ref),
				api.request('/api/v1/ledgers/me').catch(() => null),
				api.request('/api/v1/entries?' + query(emptyFilters(), this.data.month, 1)),
			]);
			if (version !== this.loadVersion) return;
			const entries = response.items || [];
			const b = stats.budget || {};
			const s = stats.summary || {};
			const spent = s.expense_cent || 0;
			const budgetSet = b.total_cent !== null && b.total_cent !== undefined;
			const total = budgetSet ? b.total_cent : 0;
			const remaining = budgetSet ? (b.remaining_cent === null || b.remaining_cent === undefined ? 0 : b.remaining_cent) : -spent;
			const pct = budgetSet && total > 0 ? Math.round(((total - remaining) / total) * 100) : 0;
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
				groups: groups(entries, categories, []),
				total: response.total, page: 1,
				hasMore: entries.length < response.total,
				monthLabel: stats.label || (this.data.month.slice(0, 4) + '年' + Number(this.data.month.slice(5)) + '月'),
				prevRef: stats.prev_ref || '', nextRef: stats.next_ref || '',
				ledgerName, ledgerIcon,
				summary: {
					expense: money(spent),
					income: money(s.income_cent || 0),
					balance: money((s.balance_cent === null || s.balance_cent === undefined) ? 0 : s.balance_cent),
				},
				budgetSet,
				overspent: budgetSet && remaining < 0,
				budget: {
					remaining: money(Math.abs(remaining)),
					total: money(total),
					daily: money(rd === null || rd === undefined ? 0 : rd),
					ratio: Math.min(100, Math.max(0, pct)),
					barClass: pct >= 100 ? 'over' : pct >= 80 ? 'warn' : '',
					dailyShown,
					remainingDays,
				},
			});
		} catch (e) { if (version === this.loadVersion) this.setData({error: e.message}); }
		finally { if (version === this.loadVersion) this.setData({loading: false}); }
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
			this.setData({entries, groups: groups(entries, this.data.categories, []), total: response.total, page: next, hasMore: entries.length < response.total});
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
