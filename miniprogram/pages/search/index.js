const api = require('../../utils/api');
const { displayMoney } = require('../../utils/money');
const { query, groups } = require('../../utils/ledger');

const ALL_TIME = { start_date: '2000-01-01', end_date: '9999-12-30' };
const CHIPS = [{label: '全部', value: ''}, {label: '仅支出', value: 'expense'}, {label: '仅收入', value: 'income'}];
CHIPS.push(...[{label:'报销',value:'reimbursement'},{label:'退款',value:'refund'},{label:'优惠',value:'discount'},{label:'不计收支',value:'exclude_stats'},{label:'不计预算',value:'exclude_budget'},{label:'包含图片',value:'image'},{label:'不包含图片',value:'no_image'}].map(c=>({...c,pending:true})));

Page({
	data: {
		keyword: '', kind: '',
		chips: CHIPS,
		categories: [], entries: [], groups: [],
		summary: {total: 0, income: '', expense: ''},
		focus: true, loading: false, loadingMore: false, error: '',
		total: 0, page: 0, hasMore: false, searched: false,
	},
	onLoad() { this.clearInput(); },
 onShow() { if (this.data.searched || this.resumeQuery) {this.resumeQuery=false;this.load();} },
 onHide() { this.resumeQuery=this.data.loading || !!this.timer; clearTimeout(this.timer);this.timer=null; this.loadVersion=(this.loadVersion||0)+1; },
 onUnload() { this.onHide(); },
 submit() { clearTimeout(this.timer); this.setData({focus:false}); if(wx.hideKeyboard) wx.hideKeyboard(); return this.load(); },
 focusInput() { this.setData({focus:true}); },
	onReachBottom() { this.loadMore(); },
	onInput(e) {
		this.loadVersion=(this.loadVersion||0)+1;
 this.setData({keyword: e.detail.value, loadingMore:false});
		clearTimeout(this.timer);
 if (!e.detail.value.trim()) {this.clearInput();return;}
		this.timer = setTimeout(() => {this.timer=null;this.load();}, 400);
	},
	clearInput() { clearTimeout(this.timer);this.timer=null;this.resumeQuery=false; this.loadVersion=(this.loadVersion||0)+1; this.setData({keyword:'', kind:'',focus:true,searched:false,loading:false,loadingMore:false,error:'',entries:[],groups:[],total:0,page:0,hasMore:false}); },
	setKind(e) { const value=e.currentTarget.dataset.value; if (!['','expense','income'].includes(value)) {wx.showToast({title:'\u540e\u7eed\u7248\u672c\u652f\u6301',icon:'none'});return;} this.setData({kind:value}); this.submit(); },
	async load() {
		const version = this.loadVersion = (this.loadVersion || 0) + 1;
		this.setData({loading: true, loadingMore:false, error: '', entries: [], groups: [], total: 0, page: 0, hasMore: false, searched: false});
		try {
			const filters = {q: this.data.keyword, kind: this.data.kind, ...ALL_TIME};
			const [categories, response] = await Promise.all([
				api.request('/api/v1/categories'),
				api.request('/api/v1/entries?' + query(filters, '', 1)),
			]);
			if (version !== this.loadVersion) return;
			const entries = response.items || [];
			this.setData({
				categories, entries,
				groups: groups(entries, categories, []),
				total: response.total, page: 1,
				hasMore: entries.length < response.total,
				searched: true,
				summary: {
					total: response.total,
					expense: displayMoney((response.summary && response.summary.expense_cent) || 0),
					income: displayMoney((response.summary && response.summary.income_cent) || 0),
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
			const filters = {q: this.data.keyword, kind: this.data.kind, ...ALL_TIME};
			const response = await api.request('/api/v1/entries?' + query(filters, '', next));
			if (version !== this.loadVersion) return;
			const map = {}; this.data.entries.concat(response.items || []).forEach(e => {map[e.id] = e;});
			const entries = Object.values(map).sort((a, b) => b.occurred_at.localeCompare(a.occurred_at) || b.id - a.id);
			this.setData({entries, groups: groups(entries, this.data.categories, []), total: response.total, page: next, hasMore: entries.length < response.total});
		} catch (e) { if (version === this.loadVersion) this.setData({error: e.message}); }
		finally { if (version === this.loadVersion) this.setData({loadingMore: false}); }
	},
	edit(e) {
		const id = Number(e.currentTarget.dataset.id);
		const item = this.data.entries.find(x => x.id === id);
		if (!item) return;
		wx.navigateTo({url: '/pages/entry-detail/index?id=' + id});
	},
	goBack() { wx.navigateBack(); },
});
