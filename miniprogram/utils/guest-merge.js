// 登录后把游客期间本机记录合并进正式账本。
// 依赖后端按 client_request_id 幂等：重复合并不会产生重复账单。
const store = require('./guest-store');
const guestCategories = require('./guest-categories');

async function merge(api) {
	const rows = store.read();
	if (!rows.length) return { synced: 0, failed: 0 };
	let categories;
	try { categories = await api.request('/api/v1/categories'); }
	catch (_) { return { synced: 0, failed: rows.length }; }
	let synced = 0, failed = 0;
	for (const row of rows) {
		const target = guestCategories.match(categories, { name: row.category_key, kind: row.kind });
		if (!target) { failed++; continue; }
		try {
			await api.request('/api/v1/entries', 'POST', {
				category_id: target.id,
				kind: row.kind,
				amount_cent: row.amount_cent,
				account_id: 0,
				occurred_at: row.occurred_at,
				note: row.note || '',
				client_request_id: row.client_request_id,
			});
			store.remove(row.local_id);
			synced++;
		} catch (_) { failed++; }
	}
	return { synced, failed };
}

module.exports = { merge };
