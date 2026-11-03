const test=require('node:test');
const assert=require('node:assert/strict');
function setup(){
 const storage=new Map([['cost_session',{token:'first',expires_at:new Date(Date.now()+3600000).toISOString(),user:{id:1,nickname:'Silent',avatar_url:'/old'}}]]);
 const uploads=[],reads=[];let redirects=0;
 global.getCurrentPages=()=>[];
 global.wx={
  getAccountInfoSync:()=>({miniProgram:{envVersion:'release'}}),getStorageSync:key=>storage.get(key),setStorageSync:(key,value)=>storage.set(key,value),removeStorageSync:key=>storage.delete(key),
  request:options=>reads.push(options),uploadFile:options=>uploads.push(options),reLaunch:options=>{redirects++;options.complete();}
 };
 delete require.cache[require.resolve('../utils/api')];
 return {api:require('../utils/api'),storage,uploads,reads,redirects:()=>redirects};
}
test('avatar upload authenticates, parses uploadFile JSON and protects latest cached profile',async()=>{
 const {api,uploads,reads}=setup();
 assert.equal(typeof api.uploadAvatar,'function');
 const oldRead=api.me();const saving=api.uploadAvatar('/tmp/wechat-avatar.png');
 assert.equal(uploads[0].url,'https://api.xiachen.xyz/api/v1/auth/me/avatar');assert.equal(uploads[0].name,'avatar');assert.equal(uploads[0].filePath,'/tmp/wechat-avatar.png');assert.equal(uploads[0].header.Authorization,'Bearer first');
 const user={id:1,nickname:'Silent',avatar_url:'/api/v1/avatars/'+ 'a'.repeat(64)+'.jpg'};
 uploads[0].success({statusCode:200,data:JSON.stringify(user)});
 assert.deepEqual(await saving,user);assert.equal(api.session().user.avatar_url,user.avatar_url);
 reads[0].success({statusCode:200,data:{id:1,nickname:'Silent',avatar_url:'/old'}});
 assert.equal((await oldRead).avatar_url,user.avatar_url);assert.equal(api.session().user.nickname,'Silent');
});
test('avatar failure preserves old profile and reports a useful error',async()=>{
 const {api,uploads}=setup();assert.equal(typeof api.uploadAvatar,'function');
 const saving=api.uploadAvatar('/tmp/avatar.png');const denied=assert.rejects(saving,/2 MB/);
 uploads[0].success({statusCode:413,data:'{"error":"avatar_too_large"}'});await denied;
 assert.equal(api.session().user.avatar_url,'/old');
 const retry=api.uploadAvatar('/tmp/avatar.png');const network=assert.rejects(retry,/上传/);
 uploads[1].fail({errMsg:'network failure'});await network;assert.equal(api.session().user.avatar_url,'/old');
});
for(const statusCode of [200,401]){
 test('late avatar '+statusCode+' response does not modify another login',async()=>{
  const {api,storage,uploads,redirects}=setup();assert.equal(typeof api.uploadAvatar,'function');
  const saving=api.uploadAvatar('/tmp/avatar.png');const denied=assert.rejects(saving,/登录状态/);
  storage.set('cost_session',{token:'second',expires_at:new Date(Date.now()+3600000).toISOString(),user:{id:2,nickname:'Other'}});
  uploads[0].success({statusCode,data:JSON.stringify(statusCode===200?{id:1,nickname:'Silent',avatar_url:'/old'}:{error:'unauthorized'})});
  await denied;assert.equal(api.session().user.id,2);assert.equal(redirects(),0);
 });
}
test('avatar upload without a session goes to login without uploading',async()=>{
 const {api,storage,uploads,redirects}=setup();assert.equal(typeof api.uploadAvatar,'function');storage.clear();
 await assert.rejects(api.uploadAvatar('/tmp/avatar.png'),/登录/);assert.equal(uploads.length,0);assert.equal(redirects(),1);
});
