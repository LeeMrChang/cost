const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
function load(name, api) {
 let page; const navigations = [], toasts = [];
 const file = path.resolve(__dirname, '../pages', name, 'index.js');
 vm.runInNewContext(fs.readFileSync(file, 'utf8'), {
  Page: p => { page = p; }, require: id => id.endsWith('/utils/api') ? api : require(path.resolve(path.dirname(file), id)),
  wx: { reLaunch: o => navigations.push(o.url), showToast: o => toasts.push(o.title) }
 });
 page.data = JSON.parse(JSON.stringify(page.data));
 page.setData = v => Object.assign(page.data, v);
 return {page, navigations, toasts};
}
const defaults = {isDevelop: () => false, config: () => ({baseURL: 'https://example.test'}), session: () => null};
test('returning account reuses the server nickname after WeChat login', async () => {
 const calls = [];
 const {page, navigations} = load('login', {...defaults, login: async value => {calls.push(value); return {id: 1, nickname: '服务端昵称'};}});
 await page.signIn();
 assert.deepEqual(calls, [undefined]);assert.equal(page.data.user.nickname,'服务端昵称');
 assert.deepEqual(navigations, ['/pages/ledger/index']);
});
test('new account logs in first and must submit the final nickname form value', async () => {
 let signedIn = false;const calls = [];
 const {page,navigations} = load('login',{...defaults,session:()=>signedIn?{token:'new'}:null,
  login:async()=>{signedIn=true;return {id:1,nickname:''};},
  updateNickname:async value=>{calls.push(value);return {id:1,nickname:value};}
 });
 await page.signIn();assert.equal(navigations.length,0);assert.equal(page.data.user.id,1);
 await page.signIn({detail:{value:{nickname:'   '}}});assert.equal(calls.length,0);assert.match(page.data.error,/昵称/);
 page.setData({nickname:'旧输入'});
 await page.signIn({detail:{value:{nickname:' 小夏🐱 '}}});
 assert.deepEqual(calls,['小夏🐱']);assert.deepEqual(navigations,['/pages/ledger/index']);
});
for (const sameAccount of [true, false]) {
 test('expired nickname form reauthenticates and '+(sameAccount?'preserves same account input':'discards another account input'), async () => {
  let session=null;const calls=[];
  const {page,navigations}=load('login',{...defaults,session:()=>session,
   login:async()=>{session={token:'renewed'};return {id:sameAccount?1:2,nickname:''};},
   updateNickname:async value=>{calls.push(value);return {id:1,nickname:value};}
  });
  page.setData({user:{id:1,nickname:''}});
  await page.signIn({detail:{value:{nickname:'已填好的名字'}}});
  if(sameAccount){assert.deepEqual(calls,['已填好的名字']);assert.deepEqual(navigations,['/pages/ledger/index']);}
  else {assert.equal(calls.length,0);assert.equal(page.data.nickname,'');assert.equal(page.data.user.id,2);assert.equal(navigations.length,0);}
 });
}
test('legacy session stays on profile form and can retry a failed save', async () => {
 let attempt = 0; const calls = [];
 const {page, navigations} = load('login', {...defaults,
  session: () => ({token:'existing'}), me: async () => ({id: 1, nickname: ''}),
  login: async () => {throw new Error('must not re-login');},
  updateNickname: async name => { calls.push(name); if (++attempt === 1) throw new Error('网络错误'); return {id: 1,nickname:name}; }
 });
 await page.onShow(); assert.equal(navigations.length, 0); assert.equal(page.data.user.id, 1);
 await page.signIn({detail: {value: {nickname: '补填名字'}}});
 assert.equal(page.data.nickname, '补填名字'); assert.equal(page.data.busy, false); assert.equal(page.data.error, '网络错误');
 await page.signIn({detail: {value: {nickname: '补填名字'}}});
 assert.deepEqual(calls, ['补填名字','补填名字']); assert.deepEqual(navigations, ['/pages/ledger/index']);
});
test('existing named session enters without another prompt', async () => {
 const {page, navigations} = load('login', {...defaults, session: () => ({}), me: async () => ({id:1,nickname:'已有昵称'})});
 await page.onShow(); assert.deepEqual(navigations, ['/pages/ledger/index']);
});
test('settings loads and saves nickname with failure retry', async () => {
 let fail = true;
 const {page, toasts} = load('settings', {
  me: async () => ({id:1,nickname:'原昵称'}), request: async () => ({id:1,name:'我的账本'}),
  updateNickname: async nickname => {if(fail) throw new Error('保存失败');return {id:1,nickname};}
 });
 await page.load(); assert.equal(page.data.nickname, '原昵称');
 await page.saveNickname({detail:{value:{nickname:' 新昵称 '}}});
 assert.equal(page.data.nickname,'新昵称'); assert.equal(page.data.user.nickname,'原昵称');assert.equal(page.data.error,'保存失败');
 fail=false;await page.saveNickname({detail:{value:{nickname:' 新昵称 '}}});
 assert.equal(page.data.user.nickname,'新昵称'); assert.deepEqual(toasts,['昵称已保存']);
});
test('logout clears the previous nickname before a different account logs in', async () => {
 const {page} = load('login', {...defaults, session:()=>({token:'old'}), me:async()=>({id:1,nickname:''}),
  updateNickname:async()=>{throw new Error('保存失败');}, logout: async () => {}});
 await page.onShow();
 await page.signIn({detail:{value:{nickname:'上个用户'}}});
 assert.equal(page.data.error,'保存失败');assert.equal(page.data.nickname,'上个用户');
 await page.signOut();
 assert.equal(page.data.user,null); assert.equal(page.data.nickname,'');
 const settings = load('settings', {logout:async()=>{}}).page;
 settings.setData({user:{id:1,nickname:'上个用户'},nickname:'上个用户'});
 await settings.signOut();
 assert.equal(settings.data.nickname,'');
});
test('nickname form rejects too long values and controls without sending a request', async () => {
 const {page, navigations} = load('login', {...defaults,session:()=>({token:'test'}),updateNickname:async()=>{throw new Error('should not send');}});
 page.setData({user:{id:1,nickname:''}});
 for (const value of ['猫'.repeat(65),'bad\nname','\t小夏']) {
  await page.signIn({detail:{value:{nickname:value}}});
  assert.match(page.data.error,/昵称/); assert.notEqual(page.data.error,'should not send');
 }
 assert.equal(navigations.length,0);
});
