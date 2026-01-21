package auth

import (
	"sync"
	"time"
)

// SessionCache provides an in-memory cache for session validation
// This significantly improves login performance by reducing database queries
type SessionCache struct {
	mu       sync.RWMutex
	sessions map[string]*cachedSession
	ttl      time.Duration
}

type cachedSession struct {
	session   *Session
	expiresAt time.Time
	cachedAt  time.Time
}

// NewSessionCache creates a new session cache with the specified TTL
func NewSessionCache(ttl time.Duration) *SessionCache {
	if ttl == 0 {
		ttl = 5 * time.Minute // Default: 5 minutes cache
	}

	cache := &SessionCache{
		sessions: make(map[string]*cachedSession),
		ttl:      ttl,
	}

	// Start background cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Get retrieves a session from the cache
func (c *SessionCache) Get(token string) (*Session, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, exists := c.sessions[token]
	if !exists {
		return nil, false
	}

	// Check if cache entry has expired
	if time.Now().After(cached.cachedAt.Add(c.ttl)) {
		return nil, false
	}

	// Check if session itself has expired
	if time.Now().After(cached.expiresAt) {
		return nil, false
	}

	return cached.session, true
}

// Set stores a session in the cache
func (c *SessionCache) Set(session *Session) {
	if session == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.sessions[session.Token] = &cachedSession{
		session:   session,
		expiresAt: session.ExpiresAt,
		cachedAt:  time.Now(),
	}
}

// Delete removes a session from the cache
func (c *SessionCache) Delete(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.sessions, token)
}

// DeleteByUserID removes all sessions for a specific user
func (c *SessionCache) DeleteByUserID(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for token, cached := range c.sessions {
		if cached.session.UserID == userID {
			delete(c.sessions, token)
		}
	}
}

// Clear removes all sessions from the cache
func (c *SessionCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.sessions = make(map[string]*cachedSession)
}

// Size returns the number of cached sessions
func (c *SessionCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.sessions)
}

// cleanupLoop periodically removes expired entries from the cache
func (c *SessionCache) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired entries
func (c *SessionCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for token, cached := range c.sessions {
		// Remove if cache entry expired or session expired
		if now.After(cached.cachedAt.Add(c.ttl)) || now.After(cached.expiresAt) {
			delete(c.sessions, token)
		}
	}
}
