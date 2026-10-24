package auth

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrMemberNotFound 表示昵称修改目标不是当前家庭成员。
var ErrMemberNotFound = errors.New("member not found")

// trimNickname 与小程序 trim 一致：裁剪 Unicode 空白和首尾 BOM。
func trimNickname(value string) string {
	return strings.TrimFunc(value, func(r rune) bool { return unicode.IsSpace(r) || r == '\ufeff' })
}

// validNickname 按 Unicode 码点限制长度；控制字符和换行在裁剪空白前拒绝。
func validNickname(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return false
		}
	}
	return utf8.RuneCountInString(trimNickname(value)) <= 20
}

func (s *Service) nickname(w http.ResponseWriter, r *http.Request) {
	actor := Current(r.Context())
	if actor.Role != "admin" && actor.Role != "member" {
		failure(w, ErrForbidden)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		response(w, 400, map[string]string{"error": "invalid_member"})
		return
	}
	if actor.Role != "admin" && actor.ID != id {
		response(w, 403, map[string]string{"error": "nickname_permission_denied"})
		return
	}
	var input struct {
		Nickname *string `json:"nickname"`
	}
	if !body(w, r, &input) {
		return
	}
	if input.Nickname == nil || !validNickname(*input.Nickname) {
		response(w, 400, map[string]string{"error": "invalid_nickname"})
		return
	}
	value := trimNickname(*input.Nickname)
	if err = s.Repo.SetNickname(r.Context(), actor.ID, id, value); err != nil {
		if errors.Is(err, ErrMemberNotFound) {
			response(w, 404, map[string]string{"error": "member_not_found"})
			return
		}
		failure(w, err)
		return
	}
	response(w, 200, struct {
		ID       int64  `json:"id"`
		Nickname string `json:"nickname"`
	}{id, value})
}

// ProfileRepository 描述成员昵称持久化及历史成员资料读取能力。
// ids 由服务端已有账单创建人生成，不能直接使用客户端传入的任意用户列表。
type ProfileRepository interface {
	SetNickname(context.Context, int64, int64, string) error
	Profiles(context.Context, []int64) ([]Principal, error)
}
