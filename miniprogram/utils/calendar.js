// Calendar helpers shared by the ledger editor and the month filters.
function pad(n) { return String(n).padStart(2, '0'); }

function parseDate(date) {
 const [year, month, day] = String(date).split('-').map(Number);
 return { year, month, day };
}

function parseMonth(month) {
 const [year, m] = String(month).split('-').map(Number);
 return { year, month: m };
}

function formatDate(year, month, day) { return year + '-' + pad(month) + '-' + pad(day); }

function formatMonth(year, month) { return year + '-' + pad(month); }

function shiftMonth(year, month, delta) {
 const index = year * 12 + (month - 1) + delta;
 return { year: Math.floor(index / 12), month: index % 12 + 1 };
}

function daysInMonth(year, month) { return new Date(year, month, 0).getDate(); }

// 0 = Sunday, matching the 周日-first grid used by the picker.
function firstWeekday(year, month) { return new Date(year, month - 1, 1).getDay(); }

// One cell per grid slot; leading slots before day 1 have an empty date.
function buildGrid(year, month) {
 const cells = [];
 for (let i = 0; i < firstWeekday(year, month); i++) cells.push({ key: 'blank-' + i, date: '', day: 0 });
 for (let day = 1; day <= daysInMonth(year, month); day++) {
  const date = formatDate(year, month, day);
  cells.push({ key: date, date, day });
 }
 return cells;
}

module.exports = { parseDate, parseMonth, formatDate, formatMonth, shiftMonth, daysInMonth, firstWeekday, buildGrid };
