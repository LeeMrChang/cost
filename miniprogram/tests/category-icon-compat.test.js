const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const test=require('node:test');

test('category icon bindings always pass strings for absent legacy icon keys',()=>{
 const root=path.resolve(__dirname,'../pages');let checked=0;
 for(const page of ['categories','entry','ledger','search','calendar','entry-detail','stats','budget']) {
  const text=fs.readFileSync(path.join(root,page,'index.wxml'),'utf8');
  for(const match of text.matchAll(/icon-key="\{\{(.*?)\}\}"/g)) {
   const value=vm.runInNewContext(match[1],{item:{key:'dining',icon_key:null,category:{icon_key:null}},iconKey:null});
   assert.equal(typeof value,'string',page+' passes a null/missing key to a String component property');checked++;
  }
 }
 assert.ok(checked>=8);
});
