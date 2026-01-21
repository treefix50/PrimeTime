package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/treefix50/primetime/internal/auth"
)

// handleAuthLogin handles user login
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "POST, OPTIONS") {
		return
	}

	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	if s.authManager == nil {
		s.writeError(w, "authentication not available", http.StatusNotImplemented)
		return
	}

	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.writeError(w, "bad request", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(payload.Username) == "" || strings.TrimSpace(payload.Password) == "" {
		s.writeError(w, "username and password are required", http.StatusBadRequest)
		return
	}

	session, err := s.authManager.Login(payload.Username, payload.Password)
	if err != nil {
		if err == auth.ErrInvalidCredentials {
			s.writeError(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}

	writeJSON(w, r, session)
}

// handleAuthLogout handles user logout
func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "POST, OPTIONS") {
		return
	}

	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	if s.authManager == nil {
		s.writeError(w, "authentication not available", http.StatusNotImplemented)
		return
	}

	token := extractToken(r)
	if token == "" {
		s.writeError(w, "missing authorization token", http.StatusUnauthorized)
		return
	}

	if err := s.authManager.Logout(token); err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}

	writeJSON(w, r, map[string]string{"status": "ok"})
}

// handleAuthSession validates the current session
func (s *Server) handleAuthSession(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}

	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	if s.authManager == nil {
		s.writeError(w, "authentication not available", http.StatusNotImplemented)
		return
	}

	token := extractToken(r)
	if token == "" {
		s.writeError(w, "missing authorization token", http.StatusUnauthorized)
		return
	}

	session, err := s.authManager.ValidateSession(token)
	if err != nil {
		if err == auth.ErrInvalidToken || err == auth.ErrTokenExpired {
			s.writeError(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}

	writeJSON(w, r, session)
}

// handleAuthUsers handles user management (admin only)
func (s *Server) handleAuthUsers(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, POST, OPTIONS") {
		return
	}

	if s.authManager == nil {
		s.writeError(w, "authentication not available", http.StatusNotImplemented)
		return
	}

	// Require authentication
	session, err := s.requireAuth(r)
	if err != nil {
		s.writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Admin only
		if !session.IsAdmin {
			s.writeError(w, "admin access required", http.StatusForbidden)
			return
		}

		users, err := s.authManager.ListUsers()
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}

		// Remove password hashes from response
		safeUsers := make([]map[string]interface{}, len(users))
		for i, user := range users {
			safeUsers[i] = map[string]interface{}{
				"id":        user.ID,
				"username":  user.Username,
				"isAdmin":   user.IsAdmin,
				"createdAt": user.CreatedAt,
				"lastLogin": user.LastLogin,
			}
		}

		writeJSON(w, r, safeUsers)

	case http.MethodPost:
		// Admin only
		if !session.IsAdmin {
			s.writeError(w, "admin access required", http.StatusForbidden)
			return
		}

		var payload struct {
			Username string `json:"username"`
			Password string `json:"password"`
			IsAdmin  bool   `json:"isAdmin"`
		}

		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeError(w, "bad request", http.StatusBadRequest)
			return
		}

		if strings.TrimSpace(payload.Username) == "" || strings.TrimSpace(payload.Password) == "" {
			s.writeError(w, "username and password are required", http.StatusBadRequest)
			return
		}

		user, err := s.authManager.CreateUser(payload.Username, payload.Password, payload.IsAdmin)
		if err != nil {
			if err == auth.ErrUserExists {
				s.writeError(w, "user already exists", http.StatusConflict)
				return
			}
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}

		// Return safe user data
		writeJSON(w, r, map[string]interface{}{
			"id":        user.ID,
			"username":  user.Username,
			"isAdmin":   user.IsAdmin,
			"createdAt": user.CreatedAt,
		})

	default:
		s.methodNotAllowed(w)
	}
}

// handleAuthUserPassword handles password changes
func (s *Server) handleAuthUserPassword(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "POST, OPTIONS") {
		return
	}

	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}

	if s.authManager == nil {
		s.writeError(w, "authentication not available", http.StatusNotImplemented)
		return
	}

	// Require authentication
	session, err := s.requireAuth(r)
	if err != nil {
		s.writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/auth/users/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" {
		s.writeError(w, errNotFound, http.StatusNotFound)
		return
	}

	userID := parts[0]

	var payload struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.writeError(w, "bad request", http.StatusBadRequest)
		return
	}

	// Users can only change their own password unless they're admin
	if userID != session.UserID && !session.IsAdmin {
		s.writeError(w, "forbidden", http.StatusForbidden)
		return
	}

	// Admin can reset any password without old password
	if session.IsAdmin && userID != session.UserID {
		if strings.TrimSpace(payload.NewPassword) == "" {
			s.writeError(w, "new password is required", http.StatusBadRequest)
			return
		}

		if err := s.authManager.ResetPassword(userID, payload.NewPassword); err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
	} else {
		// Regular password change requires old password
		if strings.TrimSpace(payload.OldPassword) == "" || strings.TrimSpace(payload.NewPassword) == "" {
			s.writeError(w, "old and new passwords are required", http.StatusBadRequest)
			return
		}

		if err := s.authManager.ChangePassword(userID, payload.OldPassword, payload.NewPassword); err != nil {
			if err == auth.ErrInvalidCredentials {
				s.writeError(w, "invalid old password", http.StatusUnauthorized)
				return
			}
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, r, map[string]string{"status": "ok"})
}

// handleAuthUserDelete handles user deletion (admin only)
func (s *Server) handleAuthUserDelete(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "DELETE, OPTIONS") {
		return
	}

	if r.Method != http.MethodDelete {
		s.methodNotAllowed(w)
		return
	}

	if s.authManager == nil {
		s.writeError(w, "authentication not available", http.StatusNotImplemented)
		return
	}

	// Require authentication
	session, err := s.requireAuth(r)
	if err != nil {
		s.writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Admin only
	if !session.IsAdmin {
		s.writeError(w, "admin access required", http.StatusForbidden)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/auth/users/")
	userID := strings.TrimSuffix(path, "/")

	if userID == "" {
		s.writeError(w, errNotFound, http.StatusNotFound)
		return
	}

	// Prevent deleting yourself
	if userID == session.UserID {
		s.writeError(w, "cannot delete your own account", http.StatusBadRequest)
		return
	}

	if err := s.authManager.DeleteUser(userID); err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}

	writeJSON(w, r, map[string]string{"status": "ok"})
}

// requireAuth validates the session and returns it
func (s *Server) requireAuth(r *http.Request) (*auth.Session, error) {
	if s.authManager == nil {
		return nil, auth.ErrInvalidToken
	}

	token := extractToken(r)
	if token == "" {
		return nil, auth.ErrInvalidToken
	}

	return s.authManager.ValidateSession(token)
}

// extractToken extracts the bearer token from the Authorization header
func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}

	return parts[1]
}
