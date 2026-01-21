package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/treefix50/primetime/internal/auth"
)

// CreateUser creates a new user
func (s *Store) CreateUser(user auth.User) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	_, err := s.db.Exec(`
		INSERT INTO auth_users (id, username, password_hash, is_admin, created_at, last_login)
		VALUES (?, ?, ?, ?, ?, ?)
	`, user.ID, user.Username, user.PasswordHash, user.IsAdmin, user.CreatedAt.Unix(), nullInt64FromTime(user.LastLogin))
	return err
}

// GetUser retrieves a user by ID
func (s *Store) GetUser(id string) (*auth.User, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	var user auth.User
	var createdAt, lastLogin sql.NullInt64
	var isAdmin int

	err := s.db.QueryRow(`
		SELECT id, username, password_hash, is_admin, created_at, last_login
		FROM auth_users
		WHERE id = ?
	`, id).Scan(&user.ID, &user.Username, &user.PasswordHash, &isAdmin, &createdAt, &lastLogin)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, auth.ErrUserNotFound
		}
		return nil, err
	}

	user.IsAdmin = isAdmin == 1
	if createdAt.Valid {
		user.CreatedAt = time.Unix(createdAt.Int64, 0)
	}
	if lastLogin.Valid {
		user.LastLogin = time.Unix(lastLogin.Int64, 0)
	}

	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (s *Store) GetUserByUsername(username string) (*auth.User, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	var user auth.User
	var createdAt, lastLogin sql.NullInt64
	var isAdmin int

	err := s.db.QueryRow(`
		SELECT id, username, password_hash, is_admin, created_at, last_login
		FROM auth_users
		WHERE username = ?
	`, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &isAdmin, &createdAt, &lastLogin)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, auth.ErrUserNotFound
		}
		return nil, err
	}

	user.IsAdmin = isAdmin == 1
	if createdAt.Valid {
		user.CreatedAt = time.Unix(createdAt.Int64, 0)
	}
	if lastLogin.Valid {
		user.LastLogin = time.Unix(lastLogin.Int64, 0)
	}

	return &user, nil
}

// UpdateUser updates a user
func (s *Store) UpdateUser(user auth.User) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	_, err := s.db.Exec(`
		UPDATE auth_users
		SET username = ?, password_hash = ?, is_admin = ?, last_login = ?
		WHERE id = ?
	`, user.Username, user.PasswordHash, user.IsAdmin, nullInt64FromTime(user.LastLogin), user.ID)
	return err
}

// DeleteUser deletes a user
func (s *Store) DeleteUser(id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	_, err := s.db.Exec(`DELETE FROM auth_users WHERE id = ?`, id)
	return err
}

// ListUsers lists all users
func (s *Store) ListUsers() ([]auth.User, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	rows, err := s.db.Query(`
		SELECT id, username, password_hash, is_admin, created_at, last_login
		FROM auth_users
		ORDER BY username
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []auth.User
	for rows.Next() {
		var user auth.User
		var createdAt, lastLogin sql.NullInt64
		var isAdmin int

		if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &isAdmin, &createdAt, &lastLogin); err != nil {
			return nil, err
		}

		user.IsAdmin = isAdmin == 1
		if createdAt.Valid {
			user.CreatedAt = time.Unix(createdAt.Int64, 0)
		}
		if lastLogin.Valid {
			user.LastLogin = time.Unix(lastLogin.Int64, 0)
		}

		users = append(users, user)
	}

	return users, rows.Err()
}

// CountUsers returns the number of users
func (s *Store) CountUsers() (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("storage: missing database connection")
	}

	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM auth_users`).Scan(&count)
	return count, err
}

// CreateSession creates a new session
func (s *Store) CreateSession(session auth.Session) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	_, err := s.db.Exec(`
		INSERT INTO auth_sessions (token, user_id, username, is_admin, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, session.Token, session.UserID, session.Username, session.IsAdmin, session.CreatedAt.Unix(), session.ExpiresAt.Unix())
	return err
}

// GetSession retrieves a session by token
func (s *Store) GetSession(token string) (*auth.Session, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	var session auth.Session
	var createdAt, expiresAt int64
	var isAdmin int

	err := s.db.QueryRow(`
		SELECT token, user_id, username, is_admin, created_at, expires_at
		FROM auth_sessions
		WHERE token = ?
	`, token).Scan(&session.Token, &session.UserID, &session.Username, &isAdmin, &createdAt, &expiresAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, auth.ErrInvalidToken
		}
		return nil, err
	}

	session.IsAdmin = isAdmin == 1
	session.CreatedAt = time.Unix(createdAt, 0)
	session.ExpiresAt = time.Unix(expiresAt, 0)

	return &session, nil
}

// DeleteSession deletes a session
func (s *Store) DeleteSession(token string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	_, err := s.db.Exec(`DELETE FROM auth_sessions WHERE token = ?`, token)
	return err
}

// DeleteUserSessions deletes all sessions for a user
func (s *Store) DeleteUserSessions(userID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	_, err := s.db.Exec(`DELETE FROM auth_sessions WHERE user_id = ?`, userID)
	return err
}

// CleanExpiredSessions removes expired sessions
func (s *Store) CleanExpiredSessions() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	_, err := s.db.Exec(`DELETE FROM auth_sessions WHERE expires_at < ?`, time.Now().Unix())
	return err
}

// Helper function for nullable int64 from time
func nullInt64FromTime(t time.Time) sql.NullInt64 {
	if t.IsZero() {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: t.Unix(), Valid: true}
}
