// Read the form's final value, including WeChat's nickname autofill.
// 游客/登录都不再强制昵称：空值返回 ''，由调用方决定默认昵称。
function optional(event) {
 const raw = event && event.detail && event.detail.value && event.detail.value.nickname;
 const value = typeof raw === 'string' ? raw.trim() : '';
 if (!value) return '';
 if (Array.from(value).length > 64 || /[\u0000-\u001f\u007f-\u009f\u2028\u2029]/.test(raw)) {
  throw new Error('昵称最多 64 个字，不能包含换行或控制字符');
 }
 return value;
}
function fromForm(event) {
 const value = optional(event);
 if (!value) throw new Error('请填写昵称，可选择使用微信昵称');
 return value;
}
module.exports = { fromForm, optional };
