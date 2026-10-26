const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
test('undo restores the deleted ID and refreshes its owning page',async()=>{
 let c;const writes=[],events=[],app={globalData:{undoEntry:{id:9,deadline:Date.now()+5000}}};
 const file=path.resolve(__dirname,'../components/undo-banner/index.js');
 vm.runInNewContext(fs.readFileSync(file,'utf8'),{Component:x=>c=x,require:()=>({request:async(...a)=>writes.push(a)}),getApp:()=>app,setTimeout:()=>1,clearTimeout(){},wx:{showToast(){}}});
 Object.assign(c,c.methods);c.data={...c.data};c.setData=x=>Object.assign(c.data,x);c.triggerEvent=e=>events.push(e);
 c.sync();assert.equal(c.data.visible,true);await c.undo();
 assert.equal(writes[0][0],'/api/v1/entries/9/restore');assert.equal(writes[0][1],'POST');
 assert.equal(c.data.visible,false);assert.equal(app.globalData.undoEntry,null);assert.deepEqual(events,['restored']);
});
