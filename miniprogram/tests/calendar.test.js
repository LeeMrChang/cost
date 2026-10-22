const assert = require('assert').strict;
const { parseDate, parseMonth, formatDate, formatMonth, shiftMonth, daysInMonth, firstWeekday, buildGrid } = require('../utils/calendar');

assert.deepEqual(parseDate('2026-10-22'), { year: 2026, month: 10, day: 22 });
assert.deepEqual(parseMonth('2026-10'), { year: 2026, month: 10 });
assert.equal(formatDate(2026, 1, 5), '2026-01-05');
assert.equal(formatMonth(2026, 1), '2026-01');

assert.deepEqual(shiftMonth(2026, 12, 1), { year: 2027, month: 1 });
assert.deepEqual(shiftMonth(2026, 1, -1), { year: 2025, month: 12 });
assert.deepEqual(shiftMonth(2026, 10, -13), { year: 2025, month: 9 });

assert.equal(daysInMonth(2026, 2), 28);
assert.equal(daysInMonth(2024, 2), 29);
assert.equal(daysInMonth(2026, 10), 31);

// 2026-10-01 is a Thursday; week starts on Sunday.
assert.equal(firstWeekday(2026, 10), 4);
assert.equal(firstWeekday(2026, 11), 0);

const grid = buildGrid(2026, 10);
assert.equal(grid.length, 35);
assert.deepEqual(grid.slice(0, 4).map(c => c.date), ['', '', '', '']);
assert.deepEqual(grid.slice(4, 7).map(c => c.date), ['2026-10-01', '2026-10-02', '2026-10-03']);
assert.equal(grid[34].date, '2026-10-31');
assert.equal(grid.filter(c => c.date).length, 31);

console.log('calendar tests passed');
