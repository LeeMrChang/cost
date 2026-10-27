const api = require('../../utils/api');
const { displayMoney } = require('../../utils/money');
const { stamp } = require('../../utils/ledger');
Page({
 data: {id:0, entry:null, amount:'', categoryName:'', icon:'', accountName:'无', ledgerName:'', date:'', time:'', loading:true, saving:false, error:''},
 onLoad(options) {this.setData({id:Number(options.id)});},
 onShow() {return this.load();},
 onHide() {this.version=(this.version||0)+1;},
 onUnload() {this.onHide();},
 async load() {
  const version=this.version=(this.version||0)+1;
  this.setData({loading:true,error:''});
  try {
   if(!Number.isSafeInteger(this.data.id)||this.data.id<=0) throw new Error('账单编号无效');
   const [entry,categories,accounts,ledger]=await Promise.all([
    api.request('/api/v1/entries/'+this.data.id),api.request('/api/v1/categories'),api.request('/api/v1/accounts'),api.request('/api/v1/ledgers/me')
   ]);
   if(version!==this.version)return;
   const category=categories.find(c=>c.id===entry.category_id)||{};
   const account=accounts.find(a=>a.id===entry.account_id);
   this.setData({entry,amount:displayMoney(entry.amount_cent),categoryName:category.name||'历史分类',icon:category.icon||'📦',accountName:account?account.name:'无',ledgerName:ledger.name,...stamp(entry.occurred_at)});
  } catch(e) {if(version===this.version)this.setData({entry:null,error:e.message});}
  finally {if(version===this.version)this.setData({loading:false});}
 },
 edit() {
  if(!this.data.entry||this.data.saving)return;
  getApp().globalData.pendingEdit=this.data.entry;
  wx.navigateTo({url:'/pages/entry/index?editID='+this.data.id});
 },
 later() {wx.showToast({title:'后续版本支持',icon:'none'});},
 remove() {
  if(this.data.saving||!this.data.entry)return;
  this.setData({saving:true});
  wx.showModal({title:'删除账单',content:'删除后收支、预算和账户余额同步更新，5 秒内可以撤销。',success:async result=>{
   if(!result.confirm){this.setData({saving:false});return;}
   const deadline=Date.now()+5000;
   try {
    await api.request('/api/v1/entries/'+this.data.id,'DELETE');
    getApp().globalData.undoEntry={id:this.data.id,deadline};
    if(getCurrentPages().length>1)wx.navigateBack();else wx.switchTab({url:'/pages/ledger/index'});
   } catch(e) {wx.showToast({title:e.message,icon:'none'});}
   finally {this.setData({saving:false});}
  },fail:()=>this.setData({saving:false})});
 }
});
