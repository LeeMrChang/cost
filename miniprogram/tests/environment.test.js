const assert = require('node:assert/strict');
const test = require('node:test');
const api = require('../utils/api');

for (const {name, version, override, expected} of [
 {name:'development defaults to local backend',version:'develop',expected:'http://127.0.0.1:8080'},
 {name:'development allows a LAN backend',version:'develop',override:'http://192.168.1.2:8080',expected:'http://192.168.1.2:8080'},
 {name:'trial uses HTTPS despite a saved local override',version:'trial',override:'http://127.0.0.1:8080',expected:'https://api.xiachen.xyz'},
 {name:'release uses HTTPS despite a saved LAN override',version:'release',override:'http://192.168.1.2:8080',expected:'https://api.xiachen.xyz'},
 {name:'unknown environment cannot use a local override',version:'unknown',override:'http://127.0.0.1:8080',expected:'https://api.xiachen.xyz'},
 {name:'missing environment metadata defaults to production',override:'http://127.0.0.1:8080',expected:'https://api.xiachen.xyz'},
]) {
 test(name,async()=>{
  let sent;
  global.wx={
   getAccountInfoSync:()=>{if(!version)throw new Error('metadata unavailable');return {miniProgram:{envVersion:version}};},
   getStorageSync:key=>key==='cost_base_url'?override:null,
   request:options=>{sent=options;options.success({statusCode:200,data:{status:'ok'}});},
  };
  assert.equal(api.config().baseURL,expected);
  await api.request('/health','GET',undefined,true);
  assert.equal(sent.url,expected+'/health');
 });
}
