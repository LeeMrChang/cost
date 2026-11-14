const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

function setup(initial) {
 const storage = new Map();
 if (initial !== undefined) storage.set('cost_guest_entries', initial);
 global.wx = {
  getStorageSync: key => storage.get(key),
  setStorageSync: (key, value) => storage.set(key, value),
  removeStorageSync: key => storage.delete(key),
 };
 delete require.cache[require.resolve('../utils/guest-store')];
 return { store: require('../utils/guest-store'), storage };
}

const sample = { category_key: 'food', kind: 'expense', amount_cent: 1800, occurred_at: '2026-11-14T12:00:00+08:00', note: '午饭' };

test('add stores a record with a local id and a guest request id', () => {
 const { store, storage } = setup();
 const one = store.add(sample);
 assert.equal(one.local_id, 1);
 assert.match(one.client_request_id, /^guest-\d+-[0-9a-f]+$/);
 assert.equal(storage.get('cost_guest_entries').length, 1);
 assert.equal(storage.get('cost_guest_entries')[0].note, '午饭');
});

test('local ids keep increasing across records', () => {
 const { store } = setup();
 const one = store.add(sample);
 const two = store.add({ ...sample, note: '晚饭' });
 assert.notEqual(one.local_id, two.local_id);
 assert.ok(two.local_id > one.local_id);
 assert.equal(store.read().length, 2);
});

test('update patches mutable fields and keeps the request id', () => {
 const { store } = setup();
 const one = store.add(sample);
 const updated = store.update(one.local_id, { amount_cent: 2500, note: '改过' });
 assert.equal(updated.amount_cent, 2500);
 assert.equal(updated.note, '改过');
 assert.equal(updated.client_request_id, one.client_request_id);
 assert.equal(store.read()[0].amount_cent, 2500);
});

test('update and remove ignore unknown ids', () => {
 const { store } = setup();
 store.add(sample);
 assert.equal(store.update(999, { note: 'x' }), null);
 assert.equal(store.remove(999), false);
 assert.equal(store.read().length, 1);
});

test('remove deletes one record and clear removes all', () => {
 const { store } = setup();
 const one = store.add(sample);
 store.add(sample);
 assert.equal(store.remove(one.local_id), true);
 assert.equal(store.read().length, 1);
 store.clear();
 assert.deepEqual(store.read(), []);
});

test('a corrupted storage value reads as an empty list instead of throwing', () => {
 assert.deepEqual(setup('{not json').store.read(), []);
 assert.deepEqual(setup({ nope: true }).store.read(), []);
});

test('pending counts records waiting to be merged', () => {
 const { store } = setup();
 assert.equal(store.pending(), 0);
 store.add(sample);
 store.add(sample);
 assert.equal(store.pending(), 2);
});
