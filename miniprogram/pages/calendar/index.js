const api = require('../../utils/api');
const { money } = require('../../utils/money');
const { query, groups } = require('../../utils/ledger');

const WEEKDAYS = ['日', '一', '二', '三', '四', '五', '六'];

function todayKey() { return new Date(Date.now() + 8 * 3600000).toISOString().slice(0, 10); }
function shortMoney(n) { const s = money(n); return s.endsWith('.00') ? s.slice(0, -3) : s; }
function buildGrid(month, series) {
	const [y, m] = month.split('-').map(Number);
	const offset = new Date(Date.UTC(y, m - 1, 1)).getUTCDay();
	const daysInMonth = new Date(Date.UTC(y, m, 0)).getUTCDate();
	const cells = [];
	for (let i = 0; i < offset; i++) cells.push({day: 0});
	for (let d = 1; d <= daysInMonth; d++) {
		const key = month + '-' + String(d).padStart(2, '0');
		const p = series[key];
		cells.push({
			day: d, key, today: key === todayKey(),
			hasExpense: !!p && p.expense_cent > 0,
			hasIncome: !!p && p.income_cent > 0,
			expense: p && p.expense_cent > 0 ? shortMoney(p.expense_cent) : '',
			income: p && p.income_cent > 0 ? '+' + shortMoney(p.income_cent) : '',
		});
	}
	while (cells.length % 7) cells.push({day: 0});
	return cells;
}

Page({
	data: {
		month: '', label: '',
		prevRef: '', nextRef: '',
		weekdays: WEEKDAYS,
		cells: [],
		summary: {expense: '0.00', income: '0.00'},
		categories: [], dayTotal:0, dayEntries:[],
		selected: '', selectedDisplay: '',
		dayGroups: [], dayLoading: false,
		loading: true, error: '',
	},
	onShow() {
  if (!this.shown) {this.shown=true;return;}
  return this.refresh();
 },
 async refresh() {const selected=this.data.selected;await this.load();if(selected) await this.tapDay({currentTarget:{dataset:{key:selected}}});},
 onHide() {this.loadVersion=(this.loadVersion||0)+1;this.dayVersion=(this.dayVersion||0)+1;},
 onUnload() {this.onHide();},
 openEntry(e) {wx.navigateTo({url:'/pages/entry-detail/index?id='+e.currentTarget.dataset.id});},
 onLoad(options) { this.setData({month: (options && options.month) || todayKey().slice(0, 7)}); return this.load(); },
	async load() {
		const version = this.loadVersion = (this.loadVersion || 0) + 1;
		this.setData({loading: true, error: '', selected: '', selectedDisplay: '', dayGroups: []});
		try {
			const ref = this.data.month + '-01';
			const [categories, stats] = await Promise.all([
				api.request('/api/v1/categories'),
				api.request('/api/v1/stats?range=month&ref=' + ref),
			]);
			if (version !== this.loadVersion) return;
			const series = {};
			(stats.series || []).forEach(p => {series[p.key] = p;});
			const label = stats.label || this.data.month;
			const s = stats.summary || {};
			if (wx.setNavigationBarTitle) wx.setNavigationBarTitle({title: label});
			this.setData({
				categories,
				label,
				summary: {expense: money(s.expense_cent || 0), income: money(s.income_cent || 0)},
				prevRef: stats.prev_ref || '', nextRef: stats.next_ref || '',
				cells: buildGrid(this.data.month, series),
			});
		} catch (e) { if (version === this.loadVersion) this.setData({error: e.message}); }
		finally { if (version === this.loadVersion) this.setData({loading: false}); }
	},
	prevMonth() { if (this.data.prevRef) this.setData({month: this.data.prevRef.slice(0, 7)}); this.load(); },
	nextMonth() { if (this.data.nextRef) this.setData({month: this.data.nextRef.slice(0, 7)}); this.load(); },
	async tapDay(e) {
		const key = e.currentTarget.dataset.key;
		if (!key) return;
		const version = this.loadVersion, dayVersion=this.dayVersion=(this.dayVersion||0)+1;
		this.setData({selected: key, dayGroups: [], dayLoading: true});
		const d = new Date(key + 'T00:00:00Z');
		const display = (d.getUTCMonth() + 1) + '月' + d.getUTCDate() + '日 星期' + WEEKDAYS[d.getUTCDay()];
		try {
			let response = await api.request('/api/v1/entries?' + query({start_date: key, end_date: key}, '', 1));
			if (version !== this.loadVersion || dayVersion !== this.dayVersion) return;
   let items=response.items||[];
   for(let page=2;items.length<response.total;page++) {
    const next=await api.request('/api/v1/entries?'+query({start_date:key,end_date:key},'',page));
    if(version!==this.loadVersion || dayVersion!==this.dayVersion)return;
    if(!next.items || !next.items.length)break;
    items=items.concat(next.items);
   }
   response={...response,items};
			this.setData({selectedDisplay: display, dayEntries:response.items||[],dayTotal:response.total||0,dayGroups: groups(response.items || [], this.data.categories, [])});
		} catch (e) { if (version === this.loadVersion && dayVersion === this.dayVersion) this.setData({error: e.message}); }
		finally { if (version === this.loadVersion && dayVersion === this.dayVersion) this.setData({dayLoading: false}); }
	},
	goStats() { if (wx.switchTab) wx.switchTab({url: '/pages/stats/index'}); },
});
