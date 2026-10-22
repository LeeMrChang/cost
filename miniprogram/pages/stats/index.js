const { request } = require('../../utils/api');
const { money, today } = require('../../utils/money');
Page({
 data: { month: today().slice(0,7), error: '', summary: {}, rows: [], days: [] },
 onShow() { this.setData({ summary: {}, rows: [], days: [] }); this.load(); },
 monthChange(e) { this.setData({ month:e.detail.value }); this.load(); },
 async load() {
  const month = this.data.month; this.setData({error:''});
  try {
   const [s,categories] = await Promise.all([request('/api/v1/stats?month=' + month),request('/api/v1/categories')]);
   if (month !== this.data.month) return;
   const map = {};categories.forEach(c=>{map[c.id]=c;});
   const max = Math.max(1,...s.days.map(d=>d.expense_cent));
   this.setData({summary:{expense:money(s.expense_cent),income:money(s.income_cent),balance:money(s.balance_cent)},rows:s.categories.map(c=>({...c,name:map[c.category_id].name,icon:map[c.category_id].icon,amount:money(c.amount_cent),percent:s.expense_cent ? (c.amount_cent/s.expense_cent*100).toFixed(1) : '0.0'})),days:s.days.map(d=>({...d,date:d.date.slice(5),amount:money(d.expense_cent),width:d.expense_cent/max*100}))});
  } catch(e) {this.setData({error:e.message});}
 }
});
