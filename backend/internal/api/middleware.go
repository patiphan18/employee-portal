package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/digithun/employee-portal/backend/internal/auth"
	"github.com/digithun/employee-portal/backend/internal/models"
)

type ctxKey string

const claimsKey ctxKey = "claims"

// ClaimsFrom returns the verified token claims placed by RequireAuth.
func ClaimsFrom(ctx context.Context) (*auth.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*auth.Claims)
	return c, ok
}

// userID is the only source of identity for a request: never a body or query value,
// which is what closes the IDOR hole described in the design doc.
func userID(r *http.Request) string {
	if c, ok := ClaimsFrom(r.Context()); ok {
		return c.Subject
	}
	return ""
}

// RequireAuth rejects requests without a valid bearer token (FR-1.4, FR-1.5).
func RequireAuth(issuer *auth.Issuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := auth.BearerToken(r.Header.Get("Authorization"))
			if !ok {
				writeError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			claims, err := issuer.Verify(token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin must be mounted behind RequireAuth.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := ClaimsFrom(r.Context())
		if !ok || c.Role != models.RoleAdmin {
			writeError(w, http.StatusForbidden, "admin role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireEmployee guards the self-service profile endpoints. An administrator is a
// configuration operator, not a staff member with a profile (FR-2.4), so the two
// roles get disjoint surfaces: /me* is employee-only and /admin/* is admin-only.
// Must be mounted behind RequireAuth.
func RequireEmployee(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := ClaimsFrom(r.Context())
		if !ok || c.Role == models.RoleAdmin {
			writeError(w, http.StatusForbidden, "administrators do not have an employee profile")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequestLogger emits one structured line per request (NFR-8).
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"user_id", userID(r),
		)
	})
}

// Recoverer turns a panic into a 500 instead of a dropped connection.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic", "error", rec, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// RateLimiter is a fixed-window per-IP limiter for the login route (FR-1.7). It is
// in-process, so the limit is per instance — see the design doc's known limitations.
type RateLimiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	limit   int
	window  time.Duration
	nowFunc func() time.Time
	// trustProxy mirrors config.TrustProxyHeaders; see ClientIP.
	trustProxy bool
}

func NewRateLimiter(limit int, window time.Duration, trustProxy bool) *RateLimiter {
	return &RateLimiter{
		hits:       map[string][]time.Time{},
		limit:      limit,
		window:     window,
		nowFunc:    time.Now,
		trustProxy: trustProxy,
	}
}

// Allow records an attempt and reports whether it is within the limit.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.nowFunc()
	cutoff := now.Add(-rl.window)

	kept := rl.hits[key][:0]
	for _, t := range rl.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.limit {
		rl.hits[key] = kept
		return false
	}
	rl.hits[key] = append(kept, now)
	return true
}

// Middleware applies the limiter keyed on the client IP.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(ClientIP(r, rl.trustProxy)) {
			writeError(w, http.StatusTooManyRequests, "too many attempts, try again later")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP resolves the address the rate limiter keys on.
//
// Behind the bundled nginx, every request arrives from the proxy's own address,
// so keying on RemoteAddr alone would give the whole deployment a single shared
// budget and lock out legitimate users. When trustProxy is set, the proxy's
// headers are used instead.
//
// Both headers are attacker-controlled unless a proxy overwrites them, which is
// why this is opt-in. nginx sets X-Real-IP to $remote_addr (overwriting anything
// the client sent) so it is preferred. X-Forwarded-For is built with
// proxy_add_x_forwarded_for, which *appends* the real peer to whatever the client
// supplied, so the trustworthy entry is the last one, never the first.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			parts := strings.Split(fwd, ",")
			for i := len(parts) - 1; i >= 0; i-- {
				if ip := strings.TrimSpace(parts[i]); ip != "" {
					return ip
				}
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
