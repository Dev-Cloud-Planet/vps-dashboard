package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Context key for authenticated user claims.
// ---------------------------------------------------------------------------

type contextKey string

const claimsKey contextKey = "claims"

// Claims is the JWT payload embedded in every token.
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// ClaimsFromContext extracts the JWT claims from the request context.
func ClaimsFromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey).(*Claims)
	return c
}

// ---------------------------------------------------------------------------
// CORS middleware.
// ---------------------------------------------------------------------------

// CORSMiddleware returns a middleware that sets CORS headers. It allows the
// given origins (pass "*" to allow everything), supports pre-flight requests,
// and permits WebSocket upgrade headers.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"

	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[strings.ToLower(o)] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				origin = "*"
			}

			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if _, ok := allowed[strings.ToLower(origin)]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-Requested-With")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")

			// Handle pre-flight.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ---------------------------------------------------------------------------
// JWT auth middleware.
// ---------------------------------------------------------------------------

// JWTAuth returns a middleware that validates a JWT bearer token. The token is
// read from the Authorization header ("Bearer <token>") or, as a fallback for
// WebSocket connections, from the "token" query parameter.
func JWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractToken(r)
			if tokenStr == "" {
				respondError(w, http.StatusUnauthorized, "missing or malformed token")
				return
			}

			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				respondError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			// Store claims in context for downstream handlers.
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractToken tries the Authorization header first, then the "token" query
// parameter (used by WebSocket clients that cannot set headers).
func extractToken(r *http.Request) string {
	// Authorization: Bearer <token>
	if auth := r.Header.Get("Authorization"); auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	// Fallback: ?token=<token>
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return ""
}

// GenerateToken creates a signed JWT for the given user.
func GenerateToken(secret string, userID int64, username string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   fmt.Sprintf("%d", userID),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ---------------------------------------------------------------------------
// JSON response helpers.
// ---------------------------------------------------------------------------

// respondJSON writes a JSON-encoded response with the given status code.
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			log.Printf("[api] json encode error: %v", err)
		}
	}
}

// respondError writes a JSON error response.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// ---------------------------------------------------------------------------
// Pagination helpers.
// ---------------------------------------------------------------------------

// Pagination holds parsed pagination parameters.
type Pagination struct {
	Page    int
	PerPage int
}

// ParsePagination extracts page and per_page from query parameters with
// sensible defaults and upper bounds.
func ParsePagination(r *http.Request) Pagination {
	p := Pagination{Page: 1, PerPage: 50}
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.Page = n
		}
	}
	if v := r.URL.Query().Get("per_page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.PerPage = n
		}
	}
	if p.PerPage > 200 {
		p.PerPage = 200
	}
	return p
}

// TotalPages computes the number of pages given total items and per-page size.
func TotalPages(total, perPage int) int {
	if perPage <= 0 {
		return 0
	}
	pages := total / perPage
	if total%perPage > 0 {
		pages++
	}
	return pages
}

// ---------------------------------------------------------------------------
// Request-logging middleware (optional, lightweight).
// ---------------------------------------------------------------------------

// RequestLogger logs every request with method, path, status, and duration.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("[http] %s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}
