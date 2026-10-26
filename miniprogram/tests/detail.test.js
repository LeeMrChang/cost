const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const entry={id:7,category_id:1,account_id:4,kind:'expense',amount_cent:1234,occurred_at:'2026-10-26T01:12:00Z',note:'Breakfast',currency:'CNY'};
function load(){
 let page;const writes=[],navigation=[],app={globalData:{}};
 const file=path.resolve(__dirname,'../pages/entry-detail/index.js');
 const api={request:async(url,method)=>{writes.push([url,method]);if(url==='/api/v1/categories')return [{id:1,name:'Food',icon:'F'}];if(url==='/api/v1/accounts')return [{id:4,name:'Cash'}];if(url==='/api/v1/ledgers/me')return {name:'My ledger'};return entry;}};
 vm.runInNewContext(fs.readFileSync(file,'utf8'),{Page:p=>page=p,require:id=>id.endsWith('/utils/api')?api:require(path.resolve(path.dirname(file),id)),getApp:()=>app,getCurrentPages:()=>[{},{}],wx:{showToast(){},showModal:o=>o.success({confirm:true}),navigateTo:o=>navigation.push(o.url),navigateBack:()=>navigation.push('back'),switchTab:o=>navigation.push(o.url)}});
 page.data=JSON.parse(JSON.stringify(page.data));page.setData=x=>Object.assign(page.data,x);return {page,writes,navigation,app};
}
test('details load by ID and editing hands over the complete historical entry',async()=>{
 const {page,app,navigation}=load();page.onLoad({id:'7'});await page.load();
 assert.equal(page.data.amount,'12.34');assert.equal(page.data.accountName,'Cash');
 page.edit();assert.equal(app.globalData.pendingEdit.account_id,4);assert.equal(navigation[0],'/pages/entry/index?editID=7');
});
test('delete returns to its source with a five second undo opportunity',async()=>{
 const {page,app,writes,navigation}=load();page.onLoad({id:'7'});await page.load();
 page.remove();await new Promise(setImmediate);
 assert.ok(writes.some(x=>x[0]==='/api/v1/entries/7'&&x[1]==='DELETE'));
 assert.equal(navigation.at(-1),'back');assert.equal(app.globalData.undoEntry.id,7);
 assert.ok(app.globalData.undoEntry.deadline>Date.now());
});
