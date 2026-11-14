const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const guest = require('../utils/guest-categories');

test('guest categories mirror the server defaults by name', () => {
 const list = guest.list();
 assert.equal(list.length, 24);
 assert.equal(list.filter(c => c.kind === 'expense').length, 12);
 assert.equal(list.filter(c => c.kind === 'income').length, 12);
 assert.equal(new Set(list.map(c => c.id)).size, 24);
 assert.equal(new Set(list.map(c => c.kind + '/' + c.name)).size, 24);
 assert.ok(list.every(c => c.icon && c.sort_order >= 0));
});

test('match prefers the server category with the same name and kind', () => {
 const server = [
  { id: 7, name: '午餐', kind: 'expense', icon: '🍛' },
  { id: 13, name: '工资', kind: 'income', icon: '💼' },
 ];
 const matched = guest.match(server, { name: '工资', kind: 'income' });
 assert.equal(matched.id, 13);
 const expense = guest.match(server, { name: '午餐', kind: 'expense' });
 assert.equal(expense.id, 7);
});

test('match ignores a same-name category of the other kind', () => {
 const server = [{ id: 24, name: '其他', kind: 'income', icon: '💰' }];
 const matched = guest.match(server, { name: '其他', kind: 'expense' });
 assert.equal(matched, null);
});

test('match falls back to the first category of the same kind', () => {
 const server = [
  { id: 2, name: '交通', kind: 'expense', icon: '🚗' },
  { id: 1, name: '餐饮', kind: 'expense', icon: '🍜' },
 ];
 assert.equal(guest.match(server, { name: '不存在的分类', kind: 'expense' }).id, 2);
 assert.equal(guest.match([], { name: '餐饮', kind: 'expense' }), null);
});
