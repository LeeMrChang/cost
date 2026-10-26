const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

function loadPage(name) {
 const file = path.resolve(__dirname, '../pages', name, 'index.js');
 let page;
 const requests = [], toasts = [];
 const api = { request: (...args) => { requests.push(args); return Promise.resolve({}); }, session: () => null };
 vm.runInNewContext(fs.readFileSync(file, 'utf8'), {
  Page: value => { page = value; },
  require: id => id.endsWith('/utils/api') ? api : require(path.resolve(path.dirname(file), id)),
  wx: { showToast: value => toasts.push(value) }
 }, { filename: file });
 page.data = JSON.parse(JSON.stringify(page.data));
 page.setData = values => Object.assign(page.data, values);
 return { page, requests, toasts };
}

function input(page, field, value) {
 return page.input({ currentTarget: { dataset: { field } }, detail: { value } });
}

// 记账金额统一走 utils/money.acceptAmountInput 校验，覆盖转账金额与预算编辑两处输入框。
for (const [name, field, save, prepare] of [
 ['accounts', 'amount', 'saveTransfer', () => {}],
 ['budget', 'totalInput', 'save', page => page.editTotal()]
]) {
 test(`${name}: rejects typed or pasted text and returns the previous display value`, () => {
  const { page } = loadPage(name);
  for (const previous of ['', '12.34']) {
   page.data[field] = previous;
   for (const value of ['记得那几年大吉', 'abc', '12元', '1a2', '￥12.34', '１２', ' 12', '12 ', '12\n', '12\r']) {
    assert.equal(input(page, field, value), previous, value);
    assert.equal(page.data[field], previous, value);
   }
  }
 });

 test(`${name}: supports decimal typing, editing and clearing`, () => {
  const { page } = loadPage(name);
  for (const value of ['1', '12', '12.', '12.3', '12.34', '12.3', '12.', '12', '', '0', '0.', '0.01', '999999999.99']) {
   assert.equal(input(page, field, value), value);
   assert.equal(page.data[field], value);
  }
 });

 test(`${name}: rejects excess precision, signs, exponent notation and oversized input`, () => {
  const { page } = loadPage(name);
  page.data[field] = '12.34';
  for (const value of ['12.345', '1.2.3', '-1', '+1', '1e2', '1000000000', '.', 'NaN', 'Infinity']) {
   assert.equal(input(page, field, value), '12.34', value);
   assert.equal(page.data[field], '12.34', value);
  }
 });

 test(`${name}: invalid stored values are still rejected before making a request`, async () => {
  const { page, requests, toasts } = loadPage(name);
  prepare(page);
  for (const value of ['中文', '1a2', '1.234', '']) {
   page.data[field] = value;
   await page[save]();
   assert.match(toasts.at(-1).title, /请输入最多两位小数的金额/);
  }
  assert.equal(requests.length, 0);
 });
}

test('accounts: notes still accept Chinese text', () => {
 const { page } = loadPage('accounts');
 input(page, 'note', '午餐和朋友一起吃饭');
 assert.equal(page.data.note, '午餐和朋友一起吃饭');
});
