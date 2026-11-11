package middleware

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/mitesh4000/pushti_sangar/server/internal/auth"
	"github.com/mitesh4000/pushti_sangar/server/internal/response"
)

type ctxKey string

const (
	UserIDKey     ctxKey = "userID"
	CustomerIDKey ctxKey = "customerID"
	RolesKey      ctxKey = "roles"
	TokenKindKey  ctxKey = "tokenKind"
	RequestIDKey  ctxKey = "requestID"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 128 {
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", RequestIDFromContext(r.Context()),
			"ip", ClientIP(r),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return h.Hijack()
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered",
					"error", fmt.Sprint(rec),
					"path", r.URL.Path,
					"request_id", RequestIDFromContext(r.Context()),
					"stack", string(debug.Stack()),
				)
				response.JSON(w, http.StatusInternalServerError, map[string]any{
					"success": false,
					"msg":     "Internal Server Error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS only reflects configured origins. An empty allow-list means no
// cross-origin browser access, never a wildcard with credentials.
func CORS(allowed []string) func(http.Handler) http.Handler {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o != "" {
			set[o] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimRight(strings.TrimSpace(r.Header.Get("Origin")), "/")
			w.Header().Add("Vary", "Origin")
			if origin != "" {
				if _, ok := set[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Request-ID, X-CSRF-Token, Idempotency-Key")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
					w.Header().Set("Access-Control-Max-Age", "600")
				} else if r.Method == http.MethodOptions {
					http.Error(w, "origin not allowed", http.StatusForbidden)
					return
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CSRF protects state-changing requests that authenticate with the token
// cookie. Bearer-token requests are not vulnerable to browser CSRF and are
// skipped. In "same-origin" mode (the default) cross-origin browser requests
// are rejected, and requests from an explicitly allowed cross-origin frontend
// must also carry the double-submit token. In "strict" mode every unsafe
// cookie-authenticated request must carry it.
func CSRF(allowedOrigins []string, mode string) func(http.Handler) http.Handler {
	if mode == "" {
		mode = "same-origin"
	}
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = normalizeOrigin(o)
		if o != "" {
			allowed[o] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if mode == "off" {
				next.ServeHTTP(w, r)
				return
			}
			// Explicit bearer tokens are immune to CSRF because a browser does
			// not attach them automatically.
			if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
				next.ServeHTTP(w, r)
				return
			}
			if _, err := r.Cookie("token"); err != nil {
				// Unauthenticated requests are rejected by the auth middleware.
				next.ServeHTTP(w, r)
				return
			}

			origin := requestOrigin(r)
			crossOrigin := origin != "" && !sameOrigin(r, origin)
			if crossOrigin {
				if _, ok := allowed[origin]; !ok {
					response.FailStatus(w, http.StatusForbidden, "Cross-site request rejected")
					return
				}
			}
			if mode == "strict" || crossOrigin {
				cookie, err := r.Cookie("csrf_token")
				header := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
				if err != nil || cookie.Value == "" || header == "" ||
					subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
					response.FailStatus(w, http.StatusForbidden, "Invalid or missing CSRF token")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requestOrigin(r *http.Request) string {
	if origin := normalizeOrigin(r.Header.Get("Origin")); origin != "" {
		return origin
	}
	referer := strings.TrimSpace(r.Header.Get("Referer"))
	if referer == "" {
		return ""
	}
	u, err := url.Parse(referer)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return normalizeOrigin(u.Scheme + "://" + u.Host)
}

func normalizeOrigin(origin string) string {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" || origin == "null" {
		return ""
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Scheme + "://" + u.Host)
}

func sameOrigin(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// Protect is retained for callers that only need kind-aware token validation.
// New routes should use ProtectUser, ProtectCustomer, or ProtectAny so legacy
// tokens can be resolved against the database.
func Protect(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				response.FailStatus(w, http.StatusUnauthorized, "Not authorized, please login")
				return
			}
			claims, err := auth.ParseToken(secret, token)
			if err != nil {
				response.FailStatus(w, http.StatusUnauthorized, "Not authorized, please login")
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, claims.ID)
			ctx = context.WithValue(ctx, TokenKindKey, claims.Kind)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ProtectUser authenticates an admin-panel user. Tokens issued before the
// kind claim existed are accepted when their ID resolves to a User row.
func ProtectUser(db *sql.DB, secret string, strict bool) func(http.Handler) http.Handler {
	return protect(db, secret, strict, auth.KindUser)
}

// ProtectCustomer authenticates a storefront customer.
func ProtectCustomer(db *sql.DB, secret string, strict bool) func(http.Handler) http.Handler {
	return protect(db, secret, strict, auth.KindCustomer)
}

// ProtectAny accepts either a user or a customer token. Handlers that support
// both must still perform resource-level ownership checks.
func ProtectAny(db *sql.DB, secret string, strict bool) func(http.Handler) http.Handler {
	return protect(db, secret, strict, "")
}

func protect(db *sql.DB, secret string, strict bool, requiredKind string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				authFailure(w, strict, "Not authorized, please login")
				return
			}
			claims, err := auth.ParseToken(secret, token)
			if err != nil {
				authFailure(w, strict, "Not authorized, please login")
				return
			}
			kind := claims.Kind
			if kind == "" && db != nil {
				kind = resolveKind(db, claims.ID)
			}
			if requiredKind != "" && kind != requiredKind {
				authFailure(w, strict, "Not authorized, please login")
				return
			}
			if kind != auth.KindUser && kind != auth.KindCustomer {
				authFailure(w, strict, "Not authorized, please login")
				return
			}

			ctx := context.WithValue(r.Context(), TokenKindKey, kind)
			if kind == auth.KindCustomer {
				ctx = context.WithValue(ctx, CustomerIDKey, claims.ID)
			} else {
				ctx = context.WithValue(ctx, UserIDKey, claims.ID)
				if db != nil {
					ctx = context.WithValue(ctx, RolesKey, loadRoles(db, claims.ID))
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func resolveKind(db *sql.DB, id string) string {
	var exists int
	if err := db.QueryRow(`SELECT 1 FROM "User" WHERE id = ?`, id).Scan(&exists); err == nil {
		return auth.KindUser
	}
	if err := db.QueryRow(`SELECT 1 FROM "Customer" WHERE id = ?`, id).Scan(&exists); err == nil {
		return auth.KindCustomer
	}
	return ""
}

func loadRoles(db *sql.DB, userID string) []string {
	rows, err := db.Query(`
		SELECT r.role FROM "Role" r
		JOIN "UserRole" ur ON ur.roleId = r.id
		WHERE ur.userId = ?`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var role string
		if rows.Scan(&role) == nil {
			roles = append(roles, role)
		}
	}
	return roles
}

// RequireRoles requires at least one of the supplied roles. It must run after
// ProtectUser or ProtectAny so RolesKey is populated.
func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[strings.ToLower(strings.TrimSpace(role))] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, role := range RolesFromContext(r.Context()) {
				if _, ok := allowed[strings.ToLower(role)]; ok {
					next.ServeHTTP(w, r)
					return
				}
			}
			response.FailStatus(w, http.StatusForbidden, "Forbidden")
		})
	}
}

// TokenFromRequest extracts a token from the Authorization header or the
// token cookie without touching the request body. Public handlers that need an
// optional actor use this after decoding a body token separately.
func TokenFromRequest(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		parts := strings.SplitN(h, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	if c, err := r.Cookie("token"); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}

func extractToken(r *http.Request) string {
	if token := TokenFromRequest(r); token != "" {
		return token
	}
	// Legacy clients may post the token in a JSON or form body. Buffer and
	// restore the body so the handler can decode it normally.
	if r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
		ct := r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
			body, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err != nil {
				return ""
			}
			if strings.HasPrefix(ct, "application/json") {
				var payload map[string]any
				if json.Unmarshal(body, &payload) == nil {
					if token, ok := payload["token"].(string); ok {
						return strings.TrimSpace(token)
					}
				}
				return ""
			}
			if values, err := url.ParseQuery(string(body)); err == nil {
				return strings.TrimSpace(values.Get("token"))
			}
		}
	}
	return ""
}

func authFailure(w http.ResponseWriter, strict bool, message string) {
	if strict {
		response.FailStatus(w, http.StatusUnauthorized, message)
		return
	}
	// Compatibility mode for the original React clients, which expect HTTP
	// 200 with success:false. Production defaults to strict mode.
	response.OK(w, map[string]any{"success": false, "Error": message, "msg": message})
}

func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

func CustomerIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(CustomerIDKey).(string); ok {
		return v
	}
	return ""
}

func TokenKindFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(TokenKindKey).(string); ok {
		return v
	}
	return ""
}

func RolesFromContext(ctx context.Context) []string {
	if v, ok := ctx.Value(RolesKey).([]string); ok {
		return v
	}
	return nil
}

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-site")
		next.ServeHTTP(w, r)
	})
}

func LimitBody(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if max > 0 && r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, max)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter is a small in-process sliding-window limiter. It is sufficient
// for a single instance; deployments with multiple replicas should use a
// shared store or an edge rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests map[string][]time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, requests: make(map[string][]time.Time)}
}

func (l *RateLimiter) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(ClientIP(r)) {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(l.window.Seconds())))
				response.FailStatus(w, http.StatusTooManyRequests, "Too many requests, please try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (l *RateLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-l.window)
	entries := l.requests[key]
	kept := entries[:0]
	for _, t := range entries {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.limit {
		l.requests[key] = kept
		return false
	}
	l.requests[key] = append(kept, now)
	if len(l.requests) > 10000 {
		for k, v := range l.requests {
			if len(v) == 0 || v[len(v)-1].Before(cutoff) {
				delete(l.requests, k)
			}
		}
	}
	return true
}
