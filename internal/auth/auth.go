package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
)

// User represents a user in the system
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // Never expose password hash
	IsAdmin      bool      `json:"isAdmin"`
	CreatedAt    time.Time `json:"createdAt"`
	LastLogin    time.Time `json:"lastLogin,omitempty"`
}

// Session represents an active user session
type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"userId"`
	Username  string    `json:"username"`
	IsAdmin   bool      `json:"isAdmin"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Store defines the interface for authentication storage
type Store interface {
	// User management
	CreateUser(user User) error
	GetUser(id string) (*User, error)
	GetUserByUsername(username string) (*User, error)
	UpdateUser(user User) error
	DeleteUser(id string) error
	ListUsers() ([]User, error)
	CountUsers() (int, error)

	// Session management
	CreateSession(session Session) error
	GetSession(token string) (*Session, error)
	DeleteSession(token string) error
	DeleteUserSessions(userID string) error
	CleanExpiredSessions() error
}

// Manager handles authentication operations
type Manager struct {
	store           Store
	sessionDuration time.Duration
	sessionCache    *SessionCache
}

// NewManager creates a new authentication manager
func NewManager(store Store, sessionDuration time.Duration) *Manager {
	if sessionDuration == 0 {
		sessionDuration = 24 * time.Hour // Default: 24 hours
	}
	return &Manager{
		store:           store,
		sessionDuration: sessionDuration,
		sessionCache:    NewSessionCache(5 * time.Minute), // 5 minute cache TTL
	}
}

// GenerateAdminPassword generates a secure random password for the admin user
func GenerateAdminPassword() string {
	// Generate 16 random bytes
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based generation
		return fmt.Sprintf("admin-%d", time.Now().UnixNano())
	}
	return base64.URLEncoding.EncodeToString(bytes)[:22] // 22 characters
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword verifies a password against a hash
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken generates a secure random token
func GenerateToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback
		hash := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
		return hex.EncodeToString(hash[:])
	}
	return hex.EncodeToString(bytes)
}

// GenerateUserID generates a unique user ID
func GenerateUserID(username string) string {
	hash := sha256.Sum256([]byte(username + time.Now().String()))
	return "user_" + hex.EncodeToString(hash[:8])
}

// InitializeAdmin creates the admin user if no users exist
func (m *Manager) InitializeAdmin() (string, error) {
	count, err := m.store.CountUsers()
	if err != nil {
		return "", err
	}

	// Admin already exists
	if count > 0 {
		return "", nil
	}

	// Generate admin password
	password := GenerateAdminPassword()
	passwordHash, err := HashPassword(password)
	if err != nil {
		return "", err
	}

	// Create admin user
	admin := User{
		ID:           "admin",
		Username:     "admin",
		PasswordHash: passwordHash,
		IsAdmin:      true,
		CreatedAt:    time.Now(),
	}

	if err := m.store.CreateUser(admin); err != nil {
		return "", err
	}

	return password, nil
}

// Login authenticates a user and creates a session
func (m *Manager) Login(username, password string) (*Session, error) {
	user, err := m.store.GetUserByUsername(username)
	if err != nil {
		if err == ErrUserNotFound {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !VerifyPassword(password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	// Update last login
	user.LastLogin = time.Now()
	if err := m.store.UpdateUser(*user); err != nil {
		// Log error but don't fail login
	}

	// Create session
	session := Session{
		Token:     GenerateToken(),
		UserID:    user.ID,
		Username:  user.Username,
		IsAdmin:   user.IsAdmin,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(m.sessionDuration),
	}

	if err := m.store.CreateSession(session); err != nil {
		return nil, err
	}

	// Cache the session for faster validation
	if m.sessionCache != nil {
		m.sessionCache.Set(&session)
	}

	return &session, nil
}

// Logout invalidates a session
func (m *Manager) Logout(token string) error {
	// Remove from cache first
	if m.sessionCache != nil {
		m.sessionCache.Delete(token)
	}

	return m.store.DeleteSession(token)
}

// ValidateSession validates a session token with caching
func (m *Manager) ValidateSession(token string) (*Session, error) {
	// Try cache first for performance
	if m.sessionCache != nil {
		if session, found := m.sessionCache.Get(token); found {
			return session, nil
		}
	}

	// Cache miss - query database
	session, err := m.store.GetSession(token)
	if err != nil {
		return nil, err
	}

	if time.Now().After(session.ExpiresAt) {
		_ = m.store.DeleteSession(token)
		if m.sessionCache != nil {
			m.sessionCache.Delete(token)
		}
		return nil, ErrTokenExpired
	}

	// Cache the session for future requests
	if m.sessionCache != nil {
		m.sessionCache.Set(session)
	}

	return session, nil
}

// CreateUser creates a new user (admin only)
func (m *Manager) CreateUser(username, password string, isAdmin bool) (*User, error) {
	// Check if user exists
	existing, err := m.store.GetUserByUsername(username)
	if err == nil && existing != nil {
		return nil, ErrUserExists
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := User{
		ID:           GenerateUserID(username),
		Username:     username,
		PasswordHash: passwordHash,
		IsAdmin:      isAdmin,
		CreatedAt:    time.Now(),
	}

	if err := m.store.CreateUser(user); err != nil {
		return nil, err
	}

	return &user, nil
}

// ChangePassword changes a user's password
func (m *Manager) ChangePassword(userID, oldPassword, newPassword string) error {
	user, err := m.store.GetUser(userID)
	if err != nil {
		return err
	}

	if !VerifyPassword(oldPassword, user.PasswordHash) {
		return ErrInvalidCredentials
	}

	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = newHash
	if err := m.store.UpdateUser(*user); err != nil {
		return err
	}

	// Invalidate all sessions for this user
	return m.store.DeleteUserSessions(userID)
}

// ResetPassword resets a user's password (admin only)
func (m *Manager) ResetPassword(userID, newPassword string) error {
	user, err := m.store.GetUser(userID)
	if err != nil {
		return err
	}

	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = newHash
	if err := m.store.UpdateUser(*user); err != nil {
		return err
	}

	// Invalidate all sessions for this user
	return m.store.DeleteUserSessions(userID)
}

// DeleteUser deletes a user (admin only)
func (m *Manager) DeleteUser(userID string) error {
	// Delete all sessions first
	if err := m.store.DeleteUserSessions(userID); err != nil {
		return err
	}

	return m.store.DeleteUser(userID)
}

// ListUsers lists all users (admin only)
func (m *Manager) ListUsers() ([]User, error) {
	return m.store.ListUsers()
}

// CleanupExpiredSessions removes expired sessions
func (m *Manager) CleanupExpiredSessions() error {
	return m.store.CleanExpiredSessions()
}
