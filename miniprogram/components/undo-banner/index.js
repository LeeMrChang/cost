const api=require('../../utils/api');
Component({
 data:{visible:false,restoring:false},
 lifetimes:{attached(){this.sync();},detached(){clearTimeout(this.timer);}},
 pageLifetimes:{show(){this.sync();},hide(){clearTimeout(this.timer);}},
 methods:{
  sync(){
   clearTimeout(this.timer);
   const undo=getApp().globalData.undoEntry;
   if(!undo||undo.deadline<=Date.now()){this.setData({visible:false});return;}
   this.setData({visible:true});
   this.timer=setTimeout(()=>this.setData({visible:false}),undo.deadline-Date.now());
  },
  async undo(){
   const pending=getApp().globalData.undoEntry;
   if(this.data.restoring||!pending)return;
   if(pending.deadline<=Date.now()){this.sync();return;}
   this.setData({restoring:true});
   try{
    await api.request('/api/v1/entries/'+pending.id+'/restore','POST');
    if(getApp().globalData.undoEntry===pending)getApp().globalData.undoEntry=null;
    clearTimeout(this.timer);this.setData({visible:false});
    wx.showToast({title:'已撤销删除'});this.triggerEvent('restored');
   }catch(e){wx.showToast({title:e.message,icon:'none'});this.sync();}
   finally{this.setData({restoring:false});}
  }
 }
});
