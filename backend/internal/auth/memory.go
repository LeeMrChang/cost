package auth

import (
	"context"
	"sync"
	"time"
)

type session struct {
	user    int64
	expires time.Time
}
type Memory struct {
	mu       sync.Mutex
	next     int64
	users    map[string]int64
	sessions map[string]session
}

func NewMemory() *Memory {
	return &Memory{next: 1, users: map[string]int64{}, sessions: map[string]session{}}
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
	return Principal{ID: id}, nil
}
func (m *Memory) Resolve(_ context.Context, hash string, now time.Time) (Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[hash]
	if !ok || !s.expires.After(now) {
		return Principal{}, ErrUnauthorized
	}
	return Principal{ID: s.user}, nil
}
func (m *Memory) Logout(_ context.Context, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, hash)
	return nil
}
