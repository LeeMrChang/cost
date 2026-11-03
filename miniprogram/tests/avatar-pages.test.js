const test=require('node:test');const assert=require('node:assert/strict');
const fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const saved='/api/v1/avatars/'+ 'a'.repeat(64)+'.jpg';
function load(name){
 let page,fail=false;const uploads=[],toasts=[];
 const api={isDevelop:()=>false,config:()=>({baseURL:'https://api.example.test'}),session:()=>({token:'session'}),
  me:async()=>({id:1,nickname:name==='login'?'':'Silent',avatar_url:saved}),request:async()=>({id:1,name:'我的账本',icon:'📒'}),
  uploadAvatar:async file=>{uploads.push(file);if(fail)throw new Error('头像上传失败');return {id:1,nickname:name==='login'?'':'Silent',avatar_url:'/api/v1/avatars/'+'b'.repeat(64)+'.jpg'};},logout:async()=>{}
 };
 const wxStub={showToast:o=>toasts.push(o.title),reLaunch:()=>{}};global.wx=wxStub;
 const file=path.resolve(__dirname,'../pages',name,'index.js');
 vm.runInNewContext(fs.readFileSync(file,'utf8'),{Page:p=>{page=p;},require:id=>id.endsWith('/utils/api')?api:require(path.resolve(path.dirname(file),id)),wx:wxStub});
 page.data=JSON.parse(JSON.stringify(page.data));page.setData=v=>Object.assign(page.data,v);
 return {page,uploads,toasts,setFailure:value=>{fail=value;}};
}
for(const name of ['login','settings']){
 test(name+' loads saved avatar and uploads a replacement without losing nickname draft',async()=>{
  const {page,uploads,toasts}=load(name);if(name==='login')await page.onShow();else await page.load();
  assert.equal(page.data.avatarSrc,'https://api.example.test'+saved);
  page.setData({nickname:'正在填写'});
  assert.equal(typeof page.chooseAvatar,'function');
  await page.chooseAvatar({detail:{avatarUrl:'/tmp/wechat-chosen.png'}});
  assert.deepEqual(uploads,['/tmp/wechat-chosen.png']);assert.equal(page.data.nickname,'正在填写');
  assert.equal(page.data.avatarSrc,'https://api.example.test/api/v1/avatars/'+'b'.repeat(64)+'.jpg');assert.deepEqual(toasts,['头像已保存']);
  await page.signOut();assert.equal(page.data.avatarSrc,'');
 });
 test(name+' cancelled or failed avatar selection preserves original image and can retry',async()=>{
  const {page,uploads,setFailure}=load(name);if(name==='login')await page.onShow();else await page.load();
  assert.equal(typeof page.chooseAvatar,'function');
  await page.chooseAvatar({detail:{}});assert.equal(uploads.length,0);
  setFailure(true);await page.chooseAvatar({detail:{avatarUrl:'/tmp/avatar.png'}});
  assert.equal(page.data.avatarSrc,'https://api.example.test'+saved);assert.equal(page.data.error,'头像上传失败');assert.equal(page.data.busy,false);
  setFailure(false);await page.chooseAvatar({detail:{avatarUrl:'/tmp/avatar.png'}});assert.equal(page.data.error,'');assert.equal(uploads.length,2);
 });
}
