package auth

import (
	"context"
	"sort"
)

func (m *Memory) SetNickname(_ context.Context, actor, id int64, nickname string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	role := m.members[actor]
	if role == "" || (role != "admin" && actor != id) {
		return ErrForbidden
	}
	if m.members[id] == "" {
		return ErrMemberNotFound
	}
	m.nicknames[id] = nickname
	return nil
}
func (m *Memory) Profiles(_ context.Context, ids []int64) ([]Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	wanted := map[int64]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	out := []Principal{}
	for _, id := range m.users {
		if wanted[id] || m.members[id] != "" {
			out = append(out, Principal{ID: id, Role: m.members[id], Nickname: m.nicknames[id]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
