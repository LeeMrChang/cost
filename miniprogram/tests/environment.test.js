const assert = require('node:assert/strict');
const test = require('node:test');
const api = require('../utils/api');
const production = 'https://api.xiachen.xyz';

for (const {name, version = 'develop', platform, override, legacyOverride, expected = production} of [
 {name:'development tools default to local backend',platform:'devtools',expected:'http://127.0.0.1:8081'},
 {name:'development tools ignore legacy cached debug addresses',platform:'devtools',legacyOverride:'http://127.0.0.1:8080',expected:'http://127.0.0.1:8081'},
 {name:'Android development preview defaults to production',platform:'android'},
 {name:'Android development ignores legacy cached localhost',platform:'android',legacyOverride:'http://127.0.0.1:8081'},
 {name:'Android development ignores stale localhost cache',platform:'android',override:'http://127.0.0.1:8081'},
 {name:'iOS development ignores stale LAN cache',platform:'ios',override:'http://192.168.1.2:8080'},
 {name:'desktop WeChat ignores local overrides',platform:'mac',override:'http://127.0.0.1:8080'},
 {name:'development tools allow an explicit local backend',platform:'devtools',override:'http://127.0.0.1:8081',expected:'http://127.0.0.1:8081'},
 {name:'development tools allow an explicit LAN backend',platform:'devtools',override:'http://192.168.1.2:8080',expected:'http://192.168.1.2:8080'},
 {name:'trial tools cannot use a saved local override',version:'trial',platform:'devtools',override:'http://127.0.0.1:8080'},
 {name:'release uses HTTPS despite saved LAN override',version:'release',platform:'android',override:'http://192.168.1.2:8080'},
 {name:'unknown account environment ignores local overrides',version:'unknown',platform:'devtools',override:'http://127.0.0.1:8080'},
 {name:'missing device platform ignores local overrides',override:'http://127.0.0.1:8080'},
]) {
 test(name,async()=>{
  let sent;
  global.wx={
   getAccountInfoSync:()=>({miniProgram:{envVersion:version}}),
   getDeviceInfo:()=>({platform}),
   getStorageSync:key=>key==='cost_devtools_base_url'?override:key==='cost_base_url'?legacyOverride:null,
   request:options=>{sent=options;options.success({statusCode:200,data:{status:'ok'}});},
  };
  assert.equal(api.config().baseURL,expected);
  await api.request('/health','GET',undefined,true);
  assert.equal(sent.url,expected+'/health');
 });
}

test('unavailable environment metadata defaults to production',()=>{
 global.wx={getAccountInfoSync:()=>{throw new Error('unavailable');},getDeviceInfo:()=>({platform:'devtools'}),getStorageSync:()=> 'http://127.0.0.1:8080'};
 assert.equal(api.config().baseURL,production);
});

test('device API failure cannot activate local overrides',()=>{
 global.wx={getAccountInfoSync:()=>({miniProgram:{envVersion:'develop'}}),getDeviceInfo:()=>{throw new Error('unavailable');},getStorageSync:()=> 'http://127.0.0.1:8080'};
 assert.equal(api.config().baseURL,production);
});

test('legacy device detection still allows explicit devtools debugging',()=>{
 global.wx={getAccountInfoSync:()=>({miniProgram:{envVersion:'develop'}}),getSystemInfoSync:()=>({platform:'devtools'}),getStorageSync:()=> 'http://127.0.0.1:8080'};
 assert.equal(api.config().baseURL,'http://127.0.0.1:8080');
});
