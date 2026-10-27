// 计算器表达式引擎：数字、小数点与 + − × ÷；按运算符从左到右即时计算，每步按分四舍五入。
const OPS = ['+', '−', '×', '÷'];
const SCALE = 10000;

function isOp(ch) { return ch === '-' || OPS.indexOf(ch) >= 0; }

function lastNumber(expr) {
	let i = expr.length;
	while (i > 0 && !isOp(expr[i - 1])) i--;
	return expr.slice(i);
}

// push 返回按下 key 后的新表达式；非法输入原样返回（不崩溃）。
function push(key, expr) {
	if (key === '-') key = '−';
	if (/^\d$/.test(key)) {
		const num = lastNumber(expr), head = expr.slice(0, expr.length - num.length);
		const dot = num.indexOf('.');
		if (dot >= 0 && num.length - dot - 1 >= 2) return expr; // 金额最多两位小数
		return head + (num === '0' && dot < 0 ? key : num + key);
	}
	if (key === '.') {
		const num = lastNumber(expr);
		if (num.indexOf('.') >= 0) return expr;
		return expr + (num === '' ? '0.' : '.');
	}
	if (isOp(key)) {
		if (expr === '') return '';
		if (isOp(expr[expr.length - 1])) return expr.slice(0, -1) + key; // 运算符覆盖前者
		return expr + key;
	}
	return expr;
}

function backspace(expr) { return expr.slice(0, -1); }

function parseUnits(raw) {
	if (!/^\d{1,9}(\.\d{0,4})?$/.test(raw)) return null;
	const [whole, fraction = ''] = raw.split('.');
	const n = Number(whole) * SCALE + Number(fraction.padEnd(4, '0'));
	return Number.isSafeInteger(n) ? n : null;
}

function parseCents(raw) {
	const units = parseUnits(raw);
	return units === null ? null : Math.round(units / 100);
}

function tokenize(expr) {
	const tokens = [];
	let i = 0;
	while (i < expr.length) {
		if (isOp(expr[i])) { tokens.push(expr[i] === '-' ? '−' : expr[i]); i++; continue; }
		let j = i;
		while (j < expr.length && !isOp(expr[j])) j++;
		const n = parseCents(expr.slice(i, j));
		if (n === null) return null;
		tokens.push(n); i = j;
	}
	return tokens;
}

// step 对一步运算即时求值；除零、溢出与非法操作数返回 null。
function step(op, left, right) {
	if (op === '÷' && right === 0) return null;
	if (op === '×' || op === '÷') {
		const product = op === '×' ? left * right : left * 100;
		if (!Number.isSafeInteger(product)) return null;
		const value = Math.round(product / (op === '×' ? 100 : right));
		return Number.isSafeInteger(value) ? value : null;
	}
	const value = op === '+' ? left + right : left - right;
	return Number.isSafeInteger(value) ? value : null;
}

// evaluate 返回 {ok:true, cents} 或 {ok:false}；尾随运算符被忽略，除零/溢出/非法输入返回 ok:false。
function evaluate(expr) {
	let tokens = tokenize(expr);
	if (!tokens || tokens.length === 0) return {ok: false};
	if (typeof tokens[tokens.length - 1] === 'string') tokens = tokens.slice(0, -1);
	if (tokens.length === 0 || typeof tokens[0] === 'string') return {ok: false};
	let acc = tokens[0];
	for (let i = 1; i < tokens.length; i += 2) {
		if (typeof tokens[i] !== 'string' || typeof tokens[i + 1] !== 'number') return {ok: false};
		acc = step(tokens[i], acc, tokens[i + 1]);
		if (acc === null) return {ok: false};
	}
	return {ok: true, cents: acc};
}

module.exports = { push, backspace, evaluate };
