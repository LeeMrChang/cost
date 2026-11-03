function avatarView(user, api) {
 return { user, avatarSrc: user && user.avatar_url ? api.config().baseURL + user.avatar_url : '' };
}
// Preserve the nickname draft while an avatar upload returns current profile data.
async function chooseAvatar(page, event, api) {
 const filePath = event && event.detail && event.detail.avatarUrl;
 if (!filePath || page.data.busy || !page.data.user) return;
 page.setData({ busy: true, error: '' });
 try {
  const user = await api.uploadAvatar(filePath);
  page.setData(avatarView(user, api));
  wx.showToast({ title: '头像已保存' });
 } catch (e) { page.setData({ error: e.message }); }
 finally { page.setData({ busy: false }); }
}
module.exports = { avatarView, chooseAvatar };
