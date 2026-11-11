const api = require('../../utils/api');
const { money } = require('../../utils/money');
const { push, backspace, evaluate } = require('../../utils/calc');
const { requestID, stamp, allowed } = require('../../utils/ledger');

// 账户是用户自建的记账标签；图标映射与账户页保持一致。
const ACCOUNT_TYPES = { cash: '💵', wechat: '💚', alipay: '💙', bank: '💳', other: '👛' };

Page({
	data: {
		ledgerName: '我的账本',
		kind: 'expense',
		expr: '', result: '0.00', resultCents: 0,
		categories: [], grid: [], categoryID: 0,
		accounts: [], accountID: 0,
		date: stamp().date, time: stamp().time,
		note: '', noteOpen: false,
		keys: [
 {v:'1'},{v:'2'},{v:'3'},{left:'+',right:'×',op:true},
 {v:'4'},{v:'5'},{v:'6'},{left:'−',right:'÷',op:true},
 {v:'7'},{v:'8'},{v:'9'},{action:'again'},
 {v:'.'},{v:'0'},{v:'⌫'},{action:'done'}
 ],
 statusBarHeight:20, loadingEntry:false, editError:'',

		saving: false, editID: null,
	},
	async onLoad(options) {
  this.setData({...stamp()});
  if(wx.getWindowInfo)this.setData({statusBarHeight:wx.getWindowInfo().statusBarHeight||20});
  if (!options || !options.editID)return;
  const id=Number(options.editID);
  const app=getApp();
  let pending=app.globalData && app.globalData.pendingEdit;
  if(app.globalData)app.globalData.pendingEdit=null;
  if(!Number.isSafeInteger(id)||id<=0){this.setData({editError:'Invalid entry ID'});return;}
  if(!pending || pending.id!==id) {
   this.setData({loadingEntry:true});
   try {pending=await api.request('/api/v1/entries/'+id);}
   catch(e){this.setData({loadingEntry:false,editError:e.message});return;}
  }
  this.originalCategoryID=pending.category_id;
  this.originalAccountID=pending.account_id||0;
  this.setData({editID:pending.id,kind:pending.kind,expr:money(pending.amount_cent),result:money(pending.amount_cent),resultCents:pending.amount_cent,categoryID:pending.category_id,accountID:this.originalAccountID,note:pending.note||'',loadingEntry:false,...stamp(pending.occurred_at)});
  this.accountSelectionInitialized=true;
  this.syncAccount();
  if(this.data.categories.length) this.buildGrid();
 },
	onShow() { this.loadCategories(); this.loadLedger(); this.loadAccounts(); },
	async loadLedger() {
		try {
			const l = await api.request('/api/v1/ledgers/me');
			if (l && l.name) this.setData({ledgerName: l.name});
		} catch (_) {}
	},
	async loadCategories() {
		try {
			const categories = await api.request('/api/v1/categories');
			const visible = categories.filter(c => c.kind === this.data.kind && (!c.disabled || c.id === this.originalCategoryID));
			let categoryID = this.data.categoryID;
			if (!visible.some(c => c.id === categoryID)) {
				const remembered = Number(wx.getStorageSync('cost_last_category_' + this.data.kind));
				categoryID = visible.some(c => c.id === remembered) ? remembered : (visible.length ? visible[0].id : 0);
			}
			this.setData({categories, categoryID});
			this.buildGrid();
		} catch (e) { wx.showToast({title: e.message, icon: 'none'}); }
	},
	async loadAccounts() {
		try {
			const rows = await api.request('/api/v1/accounts');
			this.accountRows = (Array.isArray(rows) ? rows : []).map(a => ({...a, icon: ACCOUNT_TYPES[a.type] || '👛'}));
			this.syncAccount();
		} catch (_) {}
	},
	syncAccount() {
		// Keep the complete list until late edit details reveal the original account.
		if (!this.accountRows || this.data.loadingEntry) return;
		const accounts = allowed(this.accountRows, this.originalAccountID || 0);
		let accountID = this.data.accountID || 0;
		// Zero is a deliberate choice after initialization, not a missing default.
		if (!this.accountSelectionInitialized && !this.data.editID && accountID === 0) {
			const remembered = Number(wx.getStorageSync('cost_last_account'));
			accountID = accounts.some(a => a.id === remembered) ? remembered : 0;
		}
		if (accountID && !accounts.some(a => a.id === accountID)) accountID = 0;
		this.accountSelectionInitialized = true;
		this.setData({accounts, accountID});
	},
	pickAccount(e) { this.accountSelectionInitialized = true; this.setData({accountID: Number(e.currentTarget.dataset.id) || 0}); },
	buildGrid() {
		const selected = this.data.categoryID;
		const grid = this.data.categories
			.filter(c => c.kind === this.data.kind && (!c.disabled || c.id === this.originalCategoryID))
			.sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)
			.map(c => ({id: c.id, name: c.name, icon: c.icon, icon_key: c.icon_key || '', selected: c.id === selected}));
		grid.push({id: -1, more: true});
		this.setData({grid});
	},
	switchKind(e) {
		const kind = e.currentTarget.dataset.kind;
		if (kind === this.data.kind) return;
		const visible = this.data.categories.filter(c => c.kind === kind && !c.disabled);
		const remembered = Number(wx.getStorageSync('cost_last_category_' + kind));
		const categoryID = visible.some(c => c.id === remembered) ? remembered : (visible.length ? visible[0].id : 0);
		this.setData({kind, categoryID});
		this.buildGrid();
	},
	pickCategory(e) {
		if (e.currentTarget.dataset.more) { wx.navigateTo({url: '/pages/categories/index'}); return; }
		this.setData({categoryID: Number(e.currentTarget.dataset.id)});
		this.buildGrid();
	},
	press(e) {
		if (this.data.saving) return;
		const key = e.currentTarget.dataset.v;
		const expr = key === '⌫' ? backspace(this.data.expr) : push(key, this.data.expr);
		const r = evaluate(expr);
		this.setData({expr, ...(expr === '' ? {result:'0.00',resultCents:0} : r.ok ? {result: money(r.cents), resultCents: r.cents} : {})});
	},
	toggleNote() { this.setData({noteOpen: !this.data.noteOpen}); },
	onNote(e) { this.setData({note: e.detail.value}); },
	onDate(e) { this.setData({date: e.detail.value}); },
	onTime(e) { this.setData({time: e.detail.value}); },
	later() { wx.showToast({title: '后续版本支持', icon: 'none'}); },
	close() {
		if (this.data.saving) return;
		if (getCurrentPages().length > 1) wx.navigateBack();
		else wx.reLaunch({url: '/pages/ledger/index'});
	},
	saveAgain() { return this.save(true); },
	async save(keep = false) {
		keep = keep === true;
		if (this.data.saving || this.data.loadingEntry || this.data.editError) return;
		if (this.data.resultCents <= 0) { wx.showToast({title: '请输入大于零的金额', icon: 'none'}); return; }
		if (!this.data.categoryID) { wx.showToast({title: '请选择分类', icon: 'none'}); return; }
		this.setData({saving: true});
		try {
			const data = {
				category_id: this.data.categoryID, kind: this.data.kind,
				amount_cent: this.data.resultCents,
				account_id: this.data.accountID || 0,
				occurred_at: this.data.date + 'T' + this.data.time + ':00+08:00',
				note: this.data.note,
				client_request_id: this.data.editID ? 'edit' : (this.requestID || (this.requestID = requestID())),
			};
			await api.request('/api/v1/entries' + (this.data.editID ? '/' + this.data.editID : ''), this.data.editID ? 'PATCH' : 'POST', data);
			wx.setStorageSync('cost_last_category_' + this.data.kind, this.data.categoryID);
			wx.setStorageSync('cost_last_account', this.data.accountID || 0);
			wx.showToast({title: '已保存'});
			if (keep) {
				this.requestID = requestID();
				this.originalCategoryID = null;
				this.originalAccountID = null;
				this.setData({editID: null, accountID: this.accountRows ? this.data.accountID : 0, expr: '', result: '0.00', resultCents: 0, note: '', noteOpen: false, ...stamp()});
				this.syncAccount();
				await this.loadCategories();
			} else {
				this.setData({saving: false});
				this.close();
			}
		} catch (e) { wx.showToast({title: e.message, icon: 'none'}); }
		finally { this.setData({saving: false}); }
	},
});
