const api = require('../../utils/api');
const { displayMoney } = require('../../utils/money');
const { emptyFilters, query, groups } = require('../../utils/ledger');
const { avatarView } = require('../../utils/profile');

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
