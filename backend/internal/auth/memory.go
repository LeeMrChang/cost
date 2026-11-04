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
	mu           sync.Mutex
	next         int64
	users        map[string]int64
	nicknames    map[int64]string
	avatarURLs   map[int64]string
	avatarImages map[string][]byte
	sessions     map[string]session
}

func NewMemory() *Memory {
	return &Memory{next: 1, users: map[string]int64{}, nicknames: map[int64]string{}, avatarURLs: map[int64]string{}, avatarImages: map[string][]byte{}, sessions: map[string]session{}}
}
func (m *Memory) Login(_ context.Context, openid, nickname, hash string, expires time.Time) (Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.users[openid]
	if id == 0 {
		id = m.next
		m.next++
		m.users[openid] = id
		m.nicknames[id] = ""
	}
	if nickname != "" {
		m.nicknames[id] = nickname
	}
	for k, s := range m.sessions {
		if !s.expires.After(time.Now()) {
			delete(m.sessions, k)
		}
	}
	m.sessions[hash] = session{id, expires}
	return m.profile(id), nil
}
func (m *Memory) Resolve(_ context.Context, hash string, now time.Time) (Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[hash]
	if !ok || !s.expires.After(now) {
		return Principal{}, ErrUnauthorized
	}
	return m.profile(s.user), nil
}
func (m *Memory) UpdateNickname(_ context.Context, id int64, nickname string) (Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.nicknames[id]; !ok {
		return Principal{}, ErrUnauthorized
	}
	m.nicknames[id] = nickname
	return m.profile(id), nil
}

// profile is called while holding m.mu.
func (m *Memory) profile(id int64) Principal {
	return Principal{ID: id, Nickname: m.nicknames[id], AvatarURL: m.avatarURLs[id]}
}
func (m *Memory) UpdateAvatar(_ context.Context, id int64, url string, data []byte) (Principal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.nicknames[id]; !ok {
		return Principal{}, ErrUnauthorized
	}
	delete(m.avatarImages, m.avatarURLs[id])
	m.avatarURLs[id] = url
	m.avatarImages[url] = append([]byte(nil), data...)
	return m.profile(id), nil
}
func (m *Memory) Avatar(_ context.Context, url string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.avatarImages[url]
	if !ok {
		return nil, ErrAvatarNotFound
	}
	return append([]byte(nil), data...), nil
}
func (m *Memory) Logout(_ context.Context, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, hash)
	return nil
}
