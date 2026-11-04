const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
function load() {
 let page, fail = false;
 const calls = [], toasts = [];
 let ledger = {id:1,name:'我的账本',icon:'📒',currency:'CNY',timezone:'Asia/Shanghai'};
 const api = {
  me:async()=>({id:1,nickname:'小夏'}),
  request:async(url,method,data)=>{
   if(method==='PATCH') {
    calls.push({url,method,data:JSON.parse(JSON.stringify(data))});
    if(fail)throw new Error('保存失败');
    ledger={...ledger,...data};
   }
   return {...ledger};
  }
 };
 const file=path.resolve(__dirname,'../pages/settings/index.js');
 vm.runInNewContext(fs.readFileSync(file,'utf8'),{
  Page:p=>{page=p;},require:id=>id.endsWith('/utils/api')?api:require(path.resolve(path.dirname(file),id)),
  wx:{showToast:o=>toasts.push(o.title)}
 });
 page.data=JSON.parse(JSON.stringify(page.data));page.setData=v=>Object.assign(page.data,v);
 return {page,calls,toasts,setFailure:value=>{fail=value;}};
}
test('ledger card editor prefills current data and cancel does not save',async()=>{
 const {page,calls}=load();await page.load();
 page.editLedger();
 assert.equal(page.data.editingLedger,true);assert.equal(page.data.ledgerDraftName,'我的账本');assert.equal(page.data.ledgerDraftIcon,'📒');
 page.inputLedgerName({detail:{value:'草稿'}});page.chooseLedgerIcon({currentTarget:{dataset:{icon:'✈️'}}});
 page.cancelLedgerEdit();assert.equal(page.data.editingLedger,false);assert.equal(page.data.ledger.name,'我的账本');assert.equal(calls.length,0);
 page.editLedger();assert.equal(page.data.ledgerDraftName,'我的账本');assert.equal(page.data.ledgerDraftIcon,'📒');
});
test('ledger editor keeps failed draft and retries with final form value',async()=>{
 const {page,calls,toasts,setFailure}=load();await page.load();page.editLedger();
 page.chooseLedgerIcon({currentTarget:{dataset:{icon:'✈️'}}});
 setFailure(true);await page.saveLedger({detail:{value:{name:'  旅行基金🐱  '}}});
 assert.equal(page.data.editingLedger,true);assert.equal(page.data.ledgerDraftName,'旅行基金🐱');assert.equal(page.data.ledgerDraftIcon,'✈️');
 assert.equal(page.data.ledger.name,'我的账本');assert.equal(page.data.ledgerError,'保存失败');assert.equal(page.data.busy,false);
 setFailure(false);await page.saveLedger({detail:{value:{name:'  旅行基金🐱  '}}});
 assert.deepEqual(calls.at(-1),{url:'/api/v1/ledgers/me',method:'PATCH',data:{name:'旅行基金🐱',icon:'✈️'}});
 assert.equal(page.data.ledger.name,'旅行基金🐱');assert.equal(page.data.ledger.icon,'✈️');assert.equal(page.data.editingLedger,false);assert.deepEqual(toasts,['账本已更新']);
});
test('ledger editor rejects blank, too long and multiline names without saving',async()=>{
 const {page,calls}=load();await page.load();page.editLedger();
 for(const value of ['   ','🐱'.repeat(65),'bad\nname']){
  await page.saveLedger({detail:{value:{name:value}}});
  assert.ok(page.data.ledgerError);assert.equal(calls.length,0);assert.equal(page.data.editingLedger,true);
 }
 const boundary='🐱'.repeat(64);
 await page.saveLedger({detail:{value:{name:boundary}}});assert.equal(page.data.ledger.name,boundary);
});
