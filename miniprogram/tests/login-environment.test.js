const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const test = require('node:test');

function load(platform) {
 const writes=[],toasts=[];
 global.wx={
  getAccountInfoSync:()=>({miniProgram:{envVersion:'develop'}}),
  getDeviceInfo:()=>({platform}),
  getStorageSync:key=>key==='cost_base_url'?'http://127.0.0.1:8081':null,
  setStorageSync:(...args)=>writes.push(args),removeStorageSync:()=>{},
  showToast:value=>toasts.push(value),
 };
 const file=path.resolve(__dirname,'../pages/login/index.js');
 let page;
 vm.runInNewContext(fs.readFileSync(file,'utf8'),{
  Page:value=>{page=value;},wx:global.wx,
  require:id=>require(path.resolve(path.dirname(file),id)),
 },{filename:file});
 page.data={...page.data};page.setData=value=>Object.assign(page.data,value);
 return {page,writes,toasts};
}

for(const platform of ['android','ios','mac','unknown']) {
 test(`login page hides debug UI and rejects address changes on ${platform}`,async()=>{
  const {page,writes,toasts}=load(platform);
  await page.onShow();
  assert.equal(page.data.develop,false);
  assert.equal(page.data.baseURL,'https://api.xiachen.xyz');
  page.data.baseURL='http://192.168.1.2:8080';
  page.saveURL();
  assert.equal(writes.length,0);
  assert.equal(toasts.length,0);
 });
}

test('devtools developer can explicitly save a debugging address',async()=>{
 const {page,writes,toasts}=load('devtools');
 await page.onShow();
 assert.equal(page.data.develop,true);
 page.data.baseURL='http://127.0.0.1:8080/';
 page.saveURL();
 assert.deepEqual(writes,[['cost_devtools_base_url','http://127.0.0.1:8080']]);
 assert.equal(toasts.length,1);
});

test('fresh devtools login page displays the local default without manual setup',async()=>{
 const {page}=load('devtools');
 wx.getStorageSync=()=>null;
 await page.onShow();
 assert.equal(page.data.develop,true);
 assert.equal(page.data.baseURL,'http://127.0.0.1:8081');
});
