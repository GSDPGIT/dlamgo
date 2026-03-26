package main

import (
	"sync"
	"time"
)

type CaptchaChallenge struct {
	ID        string
	Valid     bool
	ExpiresAt time.Time
}

type CaptchaStore struct {
	mu    sync.Mutex
	items map[string]CaptchaChallenge
}

func NewCaptchaStore() *CaptchaStore {
	return &CaptchaStore{items: map[string]CaptchaChallenge{}}
}

func (s *CaptchaStore) NewChallenge() CaptchaChallenge {
	challenge := CaptchaChallenge{
		ID:        randomToken(32),
		Valid:     true,
		ExpiresAt: time.Now().Add(2 * time.Minute),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[challenge.ID] = challenge
	return challenge
}

func (s *CaptchaStore) Verify(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, ok := s.items[id]
	if !ok {
		return false
	}
	delete(s.items, id)
	return challenge.Valid && time.Now().Before(challenge.ExpiresAt)
}
