const { parseDate, parseMonth, shiftMonth, formatMonth, buildGrid } = require('../../utils/calendar');

Component({
 properties: {
  visible: { type: Boolean, value: false },
  mode: { type: String, value: 'date' },
  date: { type: String, value: '' },
  time: { type: String, value: '' },
  month: { type: String, value: '' }
 },
 data: {
  weekdays: ['周日', '周一', '周二', '周三', '周四', '周五', '周六'],
  title: '',
  viewYear: 0,
  viewMonth: 0,
  cells: [],
  monthCells: [],
  draftDate: '',
  draftTime: '',
  draftMonth: ''
 },
 observers: {
  visible(visible) { if (visible) this.reset(); }
 },
 methods: {
  reset() {
   if (this.data.mode === 'month') {
    const { year, month } = parseMonth(this.data.month);
    this.setData({ viewYear: year, title: year + '年', monthCells: this.yearMonths(year), draftMonth: formatMonth(year, month) });
    return;
   }
   const { year, month } = parseDate(this.data.date);
   this.setData({ viewYear: year, viewMonth: month, title: year + '年' + month + '月', cells: buildGrid(year, month), draftDate: this.data.date, draftTime: this.data.time });
  },
  yearMonths(year) {
   const list = [];
   for (let month = 1; month <= 12; month++) list.push({ value: formatMonth(year, month), label: month + '月' });
   return list;
  },
  prev() { this.step(-1); },
  next() { this.step(1); },
  step(delta) {
   if (this.data.mode === 'month') {
    const year = this.data.viewYear + delta;
    this.setData({ viewYear: year, title: year + '年', monthCells: this.yearMonths(year) });
    return;
   }
   const { year, month } = shiftMonth(this.data.viewYear, this.data.viewMonth, delta);
   this.setData({ viewYear: year, viewMonth: month, title: year + '年' + month + '月', cells: buildGrid(year, month) });
  },
  pickDay(e) { const date = e.currentTarget.dataset.date; if (date) this.setData({ draftDate: date }); },
  pickMonth(e) { this.setData({ draftMonth: e.currentTarget.dataset.value }); },
  pickTime(e) { this.setData({ draftTime: e.detail.value }); },
  confirm() {
   if (this.data.mode === 'month') this.triggerEvent('confirm', { month: this.data.draftMonth });
   else this.triggerEvent('confirm', { date: this.data.draftDate, time: this.data.draftTime });
  },
  cancel() { this.triggerEvent('cancel'); },
  noop() {}
 }
});
