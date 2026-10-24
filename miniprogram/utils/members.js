// 昵称仅用于展示，筛选和权限始终使用成员 ID。
function memberName(member = {}) {
 if (!member.id) return '历史未指定成员';
 const name = member.nickname || '成员 ' + member.id;
 return name + (member.status === 'removed' ? '（已移除）' : '');
}
function memberLabel(member) {
 return memberName(member) + (member.id && member.nickname ? ' · 编号 ' + member.id : '');
}
function nicknameValue(value) {
 if (typeof value !== 'string' || /[\u0000-\u001f\u007f-\u009f\u2028\u2029]/.test(value)) throw new Error('昵称不能包含换行或控制字符');
 const name = value.trim();
 if ([...name].length > 20) throw new Error('昵称最多 20 个字符');
 return name;
}
module.exports = {memberName, memberLabel, nicknameValue};
