package auth

import (
	"context"
	"sort"
	"sync"
	"time"
)

type session struct {
	user    int64
	expires time.Time
}
type invitation struct{ expires time.Time }
type Memory struct {
	mu          sync.Mutex
	next        int64
	users       map[string]int64
	members     map[int64]string
	sessions    map[string]session
	invites     map[string]invitation
	initialized bool
}

func NewMemory() *Memory {
	return &Memory{next: 1, users: map[string]int64{}, members: map[int64]string{}, sessions: map[string]session{}, invites: map[string]invitation{}}
}
func (m *Memory) Login(_ context.Context, openid, hash string, expires time.Time) (Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.users[openid]
	if id == 0 {
		id = m.next
		m.next++
		m.users[openid] = id
	}
	for k, s := range m.sessions {
		if !s.expires.After(time.Now()) {
			delete(m.sessions, k)
		}
	}
	m.sessions[hash] = session{id, expires}
	return Principal{id, m.members[id]}, nil
}
func (m *Memory) Resolve(_ context.Context, hash string, now time.Time) (Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[hash]
	if !ok || !s.expires.After(now) {
		return Principal{}, ErrUnauthorized
	}
	return Principal{s.user, m.members[s.user]}, nil
}
func (m *Memory) Logout(_ context.Context, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, hash)
	return nil
}
func (m *Memory) Bootstrap(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.initialized {
		return ErrConflict
	}
	m.initialized = true
	m.members[id] = "admin"
	return nil
}
func (m *Memory) Invite(_ context.Context, id int64, hash string, expires time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.members[id] != "admin" {
		return ErrForbidden
	}
	m.invites[hash] = invitation{expires}
	return nil
}
func (m *Memory) Join(_ context.Context, id int64, hash string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, ok := m.invites[hash]
	if !ok || !i.expires.After(now) {
		return ErrInvite
	}
	if m.members[id] != "" {
		return ErrConflict
	}
	delete(m.invites, hash)
	m.members[id] = "member"
	return nil
}
func (m *Memory) Members(context.Context) ([]Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Principal{}
	for id, role := range m.members {
		out = append(out, Principal{id, role})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (m *Memory) Remove(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.members[id] != "member" {
		return ErrForbidden
	}
	delete(m.members, id)
	return nil
}
