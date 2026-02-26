package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/models"
)

// AuthHandler holds dependencies for authentication endpoints.
type AuthHandler struct {
	DB        *sql.DB
	JWTSecret string
}

// loginRequest is the expected JSON body for POST /api/auth/login.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginResponse is the JSON response for a successful login.
type loginResponse struct {
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
	User      *models.User `json:"user"`
}

// changePasswordRequest is the expected JSON body for PUT /api/auth/password.
type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Login handles POST /api/auth/login.
// It validates credentials with bcrypt and returns a signed JWT (24h expiry).
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := models.GetUserByUsername(h.DB, req.Username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database error")
		return
	}
	if user == nil {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !user.CheckPassword(req.Password) {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Update last login timestamp.
	_ = models.UpdateLastLogin(h.DB, user.ID)

	expiry := 24 * time.Hour
	token, err := GenerateToken(h.JWTSecret, user.ID, user.Username, expiry)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not generate token")
		return
	}

	respondJSON(w, http.StatusOK, loginResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(expiry),
		User:      user,
	})
}

// ChangePassword handles PUT /api/auth/password.
// It requires the current password and sets a new one.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		respondError(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}

	if len(req.NewPassword) < 8 {
		respondError(w, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}

	user, err := models.GetUserByUsername(h.DB, claims.Username)
	if err != nil || user == nil {
		respondError(w, http.StatusInternalServerError, "could not fetch user")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		respondError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	if err := models.UpdatePassword(h.DB, user.ID, req.NewPassword); err != nil {
		respondError(w, http.StatusInternalServerError, "could not update password")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "password updated successfully"})
}
