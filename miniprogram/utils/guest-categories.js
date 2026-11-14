// 游客态分类表：名称与后端 DefaultCategories 保持一致，登录合并时按名称回填。
// id 使用 101~124 的本地段位，避免和服务端分类 id 混淆。
const CATEGORIES = [
	{ id: 101, name: '餐饮', kind: 'expense', icon: '🍜', sort_order: 10 },
	{ id: 102, name: '交通', kind: 'expense', icon: '🚗', sort_order: 20 },
	{ id: 103, name: '夜宵', kind: 'expense', icon: '🍗', sort_order: 30 },
	{ id: 104, name: '公交', kind: 'expense', icon: '🚌', sort_order: 40 },
	{ id: 105, name: '地铁', kind: 'expense', icon: '🚇', sort_order: 50 },
	{ id: 106, name: '早餐', kind: 'expense', icon: '🥐', sort_order: 60 },
	{ id: 107, name: '午餐', kind: 'expense', icon: '🍛', sort_order: 70 },
	{ id: 108, name: '晚餐', kind: 'expense', icon: '🍲', sort_order: 80 },
	{ id: 109, name: '消费', kind: 'expense', icon: '🛒', sort_order: 90 },
	{ id: 110, name: '饮料咖啡', kind: 'expense', icon: '☕', sort_order: 100 },
	{ id: 111, name: '住房', kind: 'expense', icon: '🏠', sort_order: 110 },
	{ id: 112, name: '其他', kind: 'expense', icon: '📦', sort_order: 120 },
	{ id: 113, name: '工资', kind: 'income', icon: '💼', sort_order: 10 },
	{ id: 114, name: '奖金', kind: 'income', icon: '🏆', sort_order: 20 },
	{ id: 115, name: '加班', kind: 'income', icon: '⏰', sort_order: 30 },
	{ id: 116, name: '福利', kind: 'income', icon: '🎫', sort_order: 40 },
	{ id: 117, name: '公积金', kind: 'income', icon: '🏦', sort_order: 50 },
	{ id: 118, name: '红包', kind: 'income', icon: '🧧', sort_order: 60 },
	{ id: 119, name: '兼职', kind: 'income', icon: '💻', sort_order: 70 },
	{ id: 120, name: '副业', kind: 'income', icon: '🌱', sort_order: 80 },
	{ id: 121, name: '退税', kind: 'income', icon: '💸', sort_order: 90 },
	{ id: 122, name: '投资', kind: 'income', icon: '📊', sort_order: 100 },
	{ id: 123, name: '意外收入', kind: 'income', icon: '🍀', sort_order: 110 },
	{ id: 124, name: '其他', kind: 'income', icon: '💰', sort_order: 120 },
];

function list() {
	return CATEGORIES.map(item => ({ ...item }));
}

function findByName(name, kind) {
	return CATEGORIES.find(item => item.name === name && item.kind === kind) || null;
}

function findLocal(id) {
	return CATEGORIES.find(item => item.id === Number(id)) || null;
}

// 合并时把游客分类映射到服务端分类：先同名同类型，再退回该类型的第一项。
function match(serverCategories, guestCategory) {
	const rows = (Array.isArray(serverCategories) ? serverCategories : [])
		.filter(item => item && item.kind === guestCategory.kind && !item.disabled);
	const sameName = rows.find(item => item.name === guestCategory.name);
	if (sameName) return sameName;
	return rows.length ? rows[0] : null;
}

module.exports = { list, findLocal, findByName, match, CATEGORIES };
