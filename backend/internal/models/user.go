package models

import (
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents an application user.
type User struct {
	ID           int64      `json:"id,omitempty"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLogin    *time.Time `json:"last_login,omitempty"`
}

// CreateUser inserts a new user with a bcrypt-hashed password and returns
// the row id.
func CreateUser(db *sql.DB, username, password string) (int64, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("create user: hash password: %w", err)
	}

	res, err := db.Exec(
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, string(hash), time.Now().UTC(),
	)
	if err != nil {
		return 0, fmt.Errorf("create user: %w", err)
	}
	return res.LastInsertId()
}

// GetUserByUsername retrieves a user by their unique username. Returns nil
// when no matching row is found.
func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	u := &User{}
	var lastLogin sql.NullTime
	err := db.QueryRow(
		`SELECT id, username, password_hash, created_at, last_login FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt, &lastLogin)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	if lastLogin.Valid {
		u.LastLogin = &lastLogin.Time
	}
	return u, nil
}

// CheckPassword compares a plain-text password against the user's stored hash.
func (u *User) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}

// UpdateLastLogin sets the last_login timestamp to the current time.
func UpdateLastLogin(db *sql.DB, userID int64) error {
	_, err := db.Exec(
		`UPDATE users SET last_login = ? WHERE id = ?`,
		time.Now().UTC(), userID,
	)
	if err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	return nil
}

// UpdatePassword replaces the user's password hash with a new bcrypt hash.
func UpdatePassword(db *sql.DB, userID int64, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("update password: hash: %w", err)
	}
	_, err = db.Exec(
		`UPDATE users SET password_hash = ? WHERE id = ?`,
		string(hash), userID,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}
