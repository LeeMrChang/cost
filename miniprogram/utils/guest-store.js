// 游客模式本地存储：未登录时的记账先落在本机，登录后按 client_request_id 合并。
// 任何存储异常都不能影响页面渲染，读失败一律当作空列表。
const KEY = 'cost_guest_entries';

function sanitize(value) {
	if (!Array.isArray(value)) return [];
	return value.filter(item => item && typeof item === 'object');
}

function read() {
	try { return sanitize(wx.getStorageSync(KEY)); } catch (_) { return []; }
}

function write(list) {
	try { wx.setStorageSync(KEY, list); } catch (_) { /* 存储不可用时保持内存态，不阻塞记账 */ }
}

function nextID(list) {
	return list.reduce((max, item) => Math.max(max, Number(item.local_id) || 0), 0) + 1;
}

function requestID() {
	return 'guest-' + Date.now() + '-' + Math.random().toString(16).slice(2, 10);
}

function add(entry) {
	const list = read();
	const record = Object.assign({}, entry, {
		local_id: nextID(list),
		client_request_id: requestID(),
	});
	list.push(record);
	write(list);
	return record;
}

function update(localID, patch) {
	const list = read();
	const index = list.findIndex(item => item.local_id === localID);
	if (index < 0) return null;
	// 本地身份字段不允许被补丁覆盖，避免合并时重复入账。
	const next = Object.assign({}, patch);
	delete next.local_id;
	delete next.client_request_id;
	const updated = Object.assign({}, list[index], next);
	list[index] = updated;
	write(list);
	return updated;
}

function remove(localID) {
	const list = read();
	const next = list.filter(item => item.local_id !== localID);
	if (next.length === list.length) return false;
	write(next);
	return true;
}

function clear() {
	write([]);
}

function pending() {
	return read().length;
}

module.exports = { read, add, update, remove, clear, pending, requestID };
