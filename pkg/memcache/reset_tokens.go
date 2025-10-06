// pkg/mem/reset_tokens.go
package mem

import (
	"sync"
	"time"
)

type ResetTokenStore interface {
	Set(token string, accountEmail string, ttl time.Duration)

	// Consume returns the accountID for token if not expired,
	// and removes the token (single-use). Returns "" if missing/expired.
	Consume(token string) string

	// Optional: Peek reads without consuming (not used below).
	Peek(token string) (string, bool)
}

type entry struct {
	email     string
	expiresAt time.Time
}

type ResetTokens struct {
	mu   sync.RWMutex
	data map[string]entry
	// optional: a background janitor could be added if you want
}

func NewResetTokens() *ResetTokens {
	return &ResetTokens{
		data: make(map[string]entry),
	}
}

func (s *ResetTokens) Set(token string, accountEmail string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[token] = entry{
		email:     accountEmail,
		expiresAt: time.Now().Add(ttl),
	}
}

func (s *ResetTokens) Consume(token string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[token]
	if !ok {
		return ""
	}
	if time.Now().After(e.expiresAt) {
		delete(s.data, token) // cleanup expired
		return ""
	}
	delete(s.data, token) // single-use
	return e.email
}

func (s *ResetTokens) Peek(token string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.data[token]
	if !ok || time.Now().After(e.expiresAt) {
		return "", false
	}
	return e.email, true
}
