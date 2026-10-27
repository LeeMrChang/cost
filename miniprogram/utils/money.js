function parseCent(value) {
 const text = String(value).trim();
 if (!/^\d{1,9}(\.\d{1,2})?$/.test(text)) throw new Error('请输入最多两位小数的金额');
 const [whole, fraction = ''] = text.split('.');
 const n = Number(whole) * 100 + Number(fraction.padEnd(2, '0'));
 if (!Number.isSafeInteger(n) || n > 100000000000) throw new Error('金额超出范围');
 return n;
}
function money(n) { return (n / 100).toFixed(2); }
function displayMoney(n) {
 const negative = n < 0;
 const [whole, fraction] = money(Math.abs(n)).split('.');
 return (negative ? '-' : '') + whole.replace(/\B(?=(\d{3})+(?!\d))/g, ',') + '.' + fraction;
}
// Allow incomplete decimal input while typing; reject invalid edits without changing the amount.
function acceptAmountInput(value, previous = '') {
 return /^(?:\d{1,9}(?:\.\d{0,2})?)?$/.test(value) ? value : previous;
}
function today() { const d = new Date(); return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0'); }
module.exports = { parseCent, money, displayMoney, today, acceptAmountInput };
