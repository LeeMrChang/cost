const assert = require('node:assert/strict');
const test = require('node:test');
const api = require('../utils/api');
test('CSV export authenticates, writes CSV and deletes the local file after sharing', async () => {
 let sent, written, shared, removed;
 global.wx = {getAccountInfoSync: () => ({miniProgram: {envVersion: 'release'}}), getStorageSync: () => ({token: 'secret', expires_at: '2099-01-01'}), env: {USER_DATA_PATH: '/private-user'}, request: opts => {sent = opts; opts.success({statusCode: 200, data: '\ufeff分类,金额\r\n餐饮,12.34\r\n'});}, getFileSystemManager: () => ({writeFile: opts => {written = opts; opts.success();}, unlink: opts => {removed = opts.filePath; opts.success();}}), shareFileMessage: opts => {shared = opts; opts.success();}};
 assert.equal(typeof api.exportCSV, 'function'); await api.exportCSV('month=2026-10');
 assert.equal(sent.header.Authorization, 'Bearer secret'); assert.ok(sent.url.endsWith('/api/v1/entries/export?month=2026-10'));
 assert.match(written.data, /^\ufeff分类/); assert.equal(shared.filePath, written.filePath); assert.equal(removed, written.filePath);
});
test('CSV export errors create no files and preserve actionable server errors', async () => {
 let wrote = false;
 global.wx.request = opts => opts.success({statusCode: 400, data: '{"error":"export_too_large"}'});
 global.wx.getFileSystemManager = () => ({writeFile: () => {wrote = true;}});
 assert.equal(typeof api.exportCSV, 'function'); await assert.rejects(api.exportCSV(''), /导出.*范围|范围.*导出/); assert.equal(wrote, false);
});
