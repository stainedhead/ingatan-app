// Package webui provides the Admin WebUI adapter for ingatan.
// It serves a localhost-only, startup-token-secured browser console for
// administrative operations: principal management, store management, and
// system health. It does NOT use JWT authentication.
package webui

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// sessionRecord holds a single session's expiry.
type sessionRecord struct {
	expiresAt time.Time
}

// SessionStore is an in-memory session store for WebUI admin sessions.
// Sessions are intentionally lost on server restart — operators must
// re-authenticate using the startup token printed at boot.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]sessionRecord
	ttl      time.Duration
	quit     chan struct{}
}

// NewSessionStore creates a SessionStore with the given session TTL.
// A background goroutine prunes expired sessions every 5 minutes.
// Call Close to stop the goroutine.
func NewSessionStore(ttl time.Duration) *SessionStore {
	s := &SessionStore{
		sessions: make(map[string]sessionRecord),
		ttl:      ttl,
		quit:     make(chan struct{}),
	}
	go s.cleanup()
	return s
}

// Create generates a new session, stores it, and returns the session ID.
// The session ID is a cryptographically random 64-character hex string.
func (s *SessionStore) Create() string {
	id := generateHex(32)
	s.mu.Lock()
	s.sessions[id] = sessionRecord{expiresAt: time.Now().Add(s.ttl)}
	s.mu.Unlock()
	return id
}

// Valid reports whether the given session ID is active and not expired.
func (s *SessionStore) Valid(id string) bool {
	s.mu.RLock()
	rec, ok := s.sessions[id]
	s.mu.RUnlock()
	return ok && time.Now().Before(rec.expiresAt)
}

// Delete removes a session. No-op if the session does not exist.
func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// Close stops the background cleanup goroutine.
func (s *SessionStore) Close() {
	close(s.quit)
}

// cleanup removes expired sessions every 5 minutes until Close is called.
func (s *SessionStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.prune()
		case <-s.quit:
			return
		}
	}
}

func (s *SessionStore) prune() {
	now := time.Now()
	s.mu.Lock()
	for id, rec := range s.sessions {
		if now.After(rec.expiresAt) {
			delete(s.sessions, id)
		}
	}
	s.mu.Unlock()
}

// generateHex returns a cryptographically random hex string of 2*n characters.
func generateHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is unrecoverable.
		panic("webui: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
