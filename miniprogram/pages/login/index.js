const api = require('../../utils/api');
const nickname = require('../../utils/nickname');
const profile = require('../../utils/profile');

const DEFAULT_NAME = api.DEFAULT_NICKNAME || '微信用户';

Page({
	data: { busy: false, user: null, nickname: '', avatarSrc: '', error: '', baseURL: '', develop: false },
	async onShow() {
		this.setData({ develop: api.isDevelop(), baseURL: api.config().baseURL });
		if (!api.session()) return;
		try {
			let user = await api.me();
			// 历史账号可能没有昵称：补默认昵称，不再要求用户先填表。
			if (!user.nickname) {
				try { user = await api.updateNickname(DEFAULT_NAME); } catch (_) { /* 进入账本后再补 */ }
			}
			this.setData({ ...profile.avatarView(user, api), nickname: user.nickname || DEFAULT_NAME });
			this.enter();
		} catch (e) { this.setData({ error: e.message, user: null, avatarSrc: '' }); }
	},
	input(e) { this.setData({ [e.currentTarget.dataset.field]: e.detail.value }); },
	chooseAvatar(event) { return profile.chooseAvatar(this, event, api); },
	enter() { wx.reLaunch({ url: '/pages/ledger/index' }); },
	async signIn(event) {
		if (this.data.busy) return;
		let typed = '';
		try { typed = nickname.optional(event); } catch (e) { this.setData({ error: e.message }); return; }
		this.setData({ busy: true, error: '', nickname: typed || this.data.nickname || '' });
		try {
			if (api.session()) {
				// 已登录：保存可选昵称后直接进入。
				const user = typed ? await api.updateNickname(typed) : this.data.user;
				this.setData({ ...profile.avatarView(user, api), nickname: (user && user.nickname) || DEFAULT_NAME });
				this.enter();
				return;
			}
			let user = await api.login(typed || undefined);
			if (!user.nickname) {
				try { user = await api.updateNickname(DEFAULT_NAME); } catch (_) { /* 下次登录再补 */ }
			}
			this.setData({ ...profile.avatarView(user, api), nickname: user.nickname || DEFAULT_NAME });
			this.enter();
		}
		catch (e) { this.setData({ error: e.message || '微信登录失败' }); }
		finally { this.setData({ busy: false }); }
	},
	async signOut() { try { await api.logout(); this.setData({ user: null, nickname: '', avatarSrc: '', error: '' }); } catch (e) { this.setData({ error: e.message }); } },
	saveURL() {
		if (!api.isDevelop()) return;
		const value = this.data.baseURL.trim().replace(/\/+$/, '');
		if (!/^https?:\/\/[a-zA-Z0-9.-]+(?::\d+)?$/.test(value)) { this.setData({ error: '请输入正确的后端地址' }); return; }
		if (value !== api.config().baseURL) api.clearSession();
		wx.setStorageSync('cost_devtools_base_url', value);
		this.setData({ user: null, nickname: '', avatarSrc: '', error: '' }); wx.showToast({ title: '调试地址已保存' });
	}
});
