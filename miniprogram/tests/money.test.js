const assert = require('assert').strict;
const { parseCent, money } = require('../utils/money');
assert.equal(parseCent('18.50'),1850);assert.equal(parseCent('0.01'),1);assert.equal(parseCent('7'),700);assert.equal(parseCent('1.2'),120);assert.equal(money(-500),'-5.00');
for(const v of ['1.234','-1','1e2','','NaN'])assert.throws(()=>parseCent(v));
console.log('money tests passed');
