const test = require('node:test');
const assert = require('node:assert/strict');
function setup() {
 const storage = new Map([['cost_session', {token:'first',expires_at:new Date(Date.now()+3600000).toISOString(),user:{id:1,nickname:'原昵称'}}]]);
 const pending = [];
 global.wx = {
  getAccountInfoSync:()=>({miniProgram:{envVersion:'release'}}),
  getStorageSync:key=>storage.get(key), setStorageSync:(key,value)=>storage.set(key,value), removeStorageSync:key=>storage.delete(key),
  request:options=>pending.push(options)
 };
 delete require.cache[require.resolve('../utils/api')];
 return {api:require('../utils/api'),storage,pending};
}
test('late profile read cannot overwrite a nickname saved while it was in flight', async () => {
 const {api,pending}=setup();
 const read=api.me(); const write=api.updateNickname('新昵称');
 pending[1].success({statusCode:200,data:{id:1,nickname:'新昵称'}});
 await write;
 pending[0].success({statusCode:200,data:{id:1,nickname:'原昵称'}});
 const user=await read;
 assert.equal(api.session().user.nickname,'新昵称');assert.equal(user.nickname,'新昵称');
});
for(const method of ['me','updateNickname']) {
 test('late '+method+' response does not put another account profile into the new session', async () => {
  const {api,storage,pending}=setup();
  const request=api[method]('旧账号新昵称');
  const denied=assert.rejects(request,/登录状态/);
  storage.set('cost_session',{token:'second',expires_at:new Date(Date.now()+3600000).toISOString(),user:{id:2,nickname:'另一个人'}});
  pending[0].success({statusCode:200,data:{id:1,nickname:'旧账号'}});
  await denied;
  assert.equal(api.session().user.id,2);assert.equal(api.session().user.nickname,'另一个人');
 });
}
