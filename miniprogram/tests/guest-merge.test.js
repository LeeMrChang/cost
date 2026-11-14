const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

function setup(rows, serverCategories) {
 const storage = new Map([['cost_guest_entries', rows]]);
 global.wx = {
  getStorageSync: key => storage.get(key),
  setStorageSync: (key, value) => storage.set(key, value),
  removeStorageSync: key => storage.delete(key),
 };
 delete require.cache[require.resolve('../utils/guest-merge')];
 delete require.cache[require.resolve('../utils/guest-store')];
 return { merge: require('../utils/guest-merge'), storage };
}

const rows = [
 { local_id: 1, category_key: '餐饮', kind: 'expense', amount_cent: 1800, occurred_at: '2026-11-14T12:00:00+08:00', note: '午饭', client_request_id: 'guest-1-a' },
 { local_id: 2, category_key: '不存在的分类', kind: 'expense', amount_cent: 500, occurred_at: '2026-11-13T09:00:00+08:00', note: '', client_request_id: 'guest-2-b' },
];
const categories = [
 { id: 1, name: '餐饮', kind: 'expense', disabled: false },
 { id: 2, name: '交通', kind: 'expense', disabled: false },
];

test('merge posts each local entry and clears the ones that succeeded', async () => {
 const { merge, storage } = setup([rows[0]], categories);
 const posted = [];
 const api = { request: async (url, method, data) => {
  if (url === '/api/v1/categories') return categories;
  posted.push(data);
  return { id: 10 };
 } };
 const result = await merge.merge(api);
 assert.deepEqual(result, { synced: 1, failed: 0 });
 assert.equal(posted[0].client_request_id, 'guest-1-a');
 assert.equal(posted[0].category_id, 1);
 assert.equal(posted[0].account_id, 0);
 assert.deepEqual(storage.get('cost_guest_entries'), []);
});

test('a category missing on the server falls back to the first of the same kind', async () => {
 const { merge } = setup([rows[1]], categories);
 const posted = [];
 const api = { request: async (url, method, data) => {
  if (url === '/api/v1/categories') return categories;
  posted.push(data);
  return { id: 11 };
 } };
 const result = await merge.merge(api);
 assert.equal(result.synced, 1);
 assert.equal(posted[0].category_id, 1);
});

test('a failed upload keeps its local row for the next attempt', async () => {
 const { merge, storage } = setup([rows[0]], categories);
 const api = { request: async url => {
  if (url === '/api/v1/categories') return categories;
  throw new Error('network');
 } };
 const result = await merge.merge(api);
 assert.deepEqual(result, { synced: 0, failed: 1 });
 assert.equal(storage.get('cost_guest_entries').length, 1);
});

test('nothing happens when there is no local data', async () => {
 const { merge } = setup([], categories);
 let calls = 0;
 const result = await merge.merge({ request: async () => { calls++; return []; } });
 assert.deepEqual(result, { synced: 0, failed: 0 });
 assert.equal(calls, 0);
});

test('an unreachable category list aborts without losing rows', async () => {
 const { merge, storage } = setup([rows[0]], categories);
 const api = { request: async () => { throw new Error('offline'); } };
 const result = await merge.merge(api);
 assert.deepEqual(result, { synced: 0, failed: 1 });
 assert.equal(storage.get('cost_guest_entries').length, 1);
});
