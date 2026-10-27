const { parseCent, money } = require('./money');
function emptyFilters() { return { q: '', start_date: '', end_date: '', kind: '', category_id: '', created_by: '', account_id: '', min_amount: '', max_amount: '' }; }
function query(filters, month, page) {
 const f = { ...emptyFilters(), ...filters }, values = {};
 if (f.start_date || f.end_date) {
  if (!f.start_date || !f.end_date || !/^\d{4}-\d{2}-\d{2}$/.test(f.start_date) || !/^\d{4}-\d{2}-\d{2}$/.test(f.end_date) || f.start_date > f.end_date) throw new Error('请选择完整且起始不晚于结束的日期范围');
  values.start_date = f.start_date; values.end_date = f.end_date;
 } else values.month = month;
 for (const key of ['q', 'kind', 'category_id', 'created_by', 'account_id']) {
  if (f[key] !== '' && f[key] !== null && f[key] !== undefined) values[key] = String(f[key]).trim();
 }
 for (const key of ['min_amount', 'max_amount']) if (String(f[key]).trim() !== '') values[key + '_cent'] = parseCent(f[key]);
 if (values.min_amount_cent !== undefined && values.max_amount_cent !== undefined && values.min_amount_cent > values.max_amount_cent) throw new Error('最低金额不能大于最高金额');
 if (page) { values.page = page; values.page_size = 50; }
 return Object.keys(values).map(k => encodeURIComponent(k) + '=' + encodeURIComponent(values[k])).join('&');
}
function stamp(value = new Date()) {
 const iso = new Date(new Date(value).getTime() + 8 * 3600000).toISOString();
 return {date: iso.slice(0, 10), time: iso.slice(11, 16)};
}
function allowed(items, originalID) { return items.filter(i => !i.disabled || i.id === originalID).map(i => ({...i, label: i.name + (i.disabled ? '（已停用）' : '')})); }
function groups(entries, categories, accounts) {
	const cm = {}, am = {}, days = {};
	categories.forEach(c => { cm[c.id] = c; }); accounts.forEach(a => { am[a.id] = a; });
	entries.forEach(e => {
		const s = stamp(e.occurred_at);
		if (!days[s.date]) days[s.date] = {date: s.date, expense: 0, income: 0, items: []};
		if (e.kind === 'expense') days[s.date].expense += e.amount_cent;
		else days[s.date].income += e.amount_cent;
		days[s.date].items.push({...e, ...s, category: cm[e.category_id] || {name: '历史分类', icon: '📦'}, accountName: am[e.account_id] ? am[e.account_id].name : '未指定账户', authorName: e.created_by ? '用户 ' + e.created_by : '历史记录', display: money(e.amount_cent)});
	});
	return Object.keys(days).sort().reverse().map(k => {
		// k 已是北京时间的日历日期，按 UTC 解析仅用于计算星期，不受设备时区影响。
		const weekday = WEEKDAYS[new Date(k + 'T00:00:00Z').getUTCDay()];
		const d = days[k];
		return {...d, total: money(d.expense), income: money(d.income), hasIncome: d.income > 0, weekday, display: k.slice(5).replace('-', '/') + ' ' + weekday};
	});
}
const WEEKDAYS = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六'];
function requestID() { return 'wx-' + Date.now() + '-' + Math.random().toString(16).slice(2); }
module.exports = { emptyFilters, query, stamp, allowed, groups, requestID };
