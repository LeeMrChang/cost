const assert = require('assert').strict;
const { push, backspace, evaluate } = require('../utils/calc');

function cents(expr) { const r = evaluate(expr); assert.equal(r.ok, true, expr); return r.cents; }

// 四则与连算：先乘除后加减，结果按分四舍五入。
assert.equal(cents('12+8'), 2000);
assert.equal(cents('1+2×3'), 700);
assert.equal(cents('12÷4'), 300);
assert.equal(cents('10÷3'), 333);
assert.equal(cents('1.5+2.25'), 375);
assert.equal(cents('12.345'), 1235);
assert.equal(cents('5-8'), -300);
assert.equal(cents('12+'), 1200); // 尾随运算符忽略
assert.equal(cents('0.5'), 50);

// 连续输入运算符，后者覆盖前者；表达式不能以运算符开头。
assert.equal(push('×', '12+'), '12×');
assert.equal(push('÷', '12×'), '12÷');
assert.equal(push('+', ''), '');

// 数字与前导零、两位小数限制。
assert.equal(push('5', '0'), '5');
assert.equal(push('5', '1+0'), '1+5');
assert.equal(push('5', '1+0.5'), '1+0.55');
assert.equal(push('6', '12.55'), '12.55');
assert.equal(push('.', '12'), '12.');
assert.equal(push('.', '12.'), '12.');
assert.equal(push('.', ''), '0.');
assert.equal(push('.', '1+'), '1+0.');
assert.equal(push('1', ''), '1');

// 退格。
assert.equal(backspace('12+8'), '12+');
assert.equal(backspace(''), '');

// 除零、溢出与非法输入不崩溃，返回 ok:false。
assert.equal(evaluate('12÷0').ok, false);
assert.equal(evaluate('5÷0+3').ok, false);
assert.equal(evaluate('999999999×999999999').ok, false);
assert.equal(evaluate('').ok, false);
assert.equal(evaluate('+5').ok, false);
assert.equal(evaluate('1.23456').ok, false);

// 完整输入流。
let expr = '';
for (const k of ['1', '2', '+', '8']) expr = push(k, expr);
assert.equal(expr, '12+8');
assert.equal(evaluate(expr).cents, 2000);
console.log('calc tests passed');
