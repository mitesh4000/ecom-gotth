// Package session manages the storefront's guest cookie and CSRF token. It
// deliberately reuses the existing JWT customer token instead of introducing a
// second authentication system.
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/mitesh4000/pushti_sangar/server/internal/auth"
	"github.com/mitesh4000/pushti_sangar/server/internal/middleware"
	"github.com/mitesh4000/pushti_sangar/server/internal/service"
)

const (
	// GuestCookieName is the opaque guest session cookie. Its value is never
	// used as a database key directly; only its SHA-256 hash is stored.
	GuestCookieName = "storefront_session"
	// CSRFCookieName is the same cookie name used by the existing JSON API
	// middleware, so a browser only needs one double-submit cookie.
	CSRFCookieName = "csrf_token"

	defaultGuestTTL = 30 * 24 * time.Hour
	defaultCSRFTTL  = 24 * time.Hour
	maxCookieValue  = 128
)

type ctxKey string

const dataKey ctxKey = "storefront-session-data"

// Data is the request-scoped storefront session state. Nothing in it is taken
// from a query parameter or request body.
type Data struct {
	GuestKey   string
	CSRFToken  string
	CustomerID string
	CartCount  int
}

// SignedIn reports whether the request carries a valid customer token.
func (d Data) SignedIn() bool { return d.CustomerID != "" }

// Owner returns the cart owner for this request. Signed-in customers keep
// using the existing CartItem table; everyone else gets a hashed guest key.
func (d Data) Owner() service.CartOwner {
	if d.CustomerID != "" {
		return service.CartOwner{CustomerID: d.CustomerID}
	}
	return service.CartOwner{GuestKey: d.GuestKey}
}

// FromContext returns the session data stored by Manager.Session.
func FromContext(ctx context.Context) Data {
	if v, ok := ctx.Value(dataKey).(Data); ok {
		return v
	}
	return Data{}
}

// WithData stores session data on a context. It is exported for tests.
func WithData(ctx context.Context, data Data) context.Context {
	return context.WithValue(ctx, dataKey, data)
}

// GuestKeyFromContext returns the hashed guest cart key.
func GuestKeyFromContext(ctx context.Context) string { return FromContext(ctx).GuestKey }

// CSRFTokenFromContext returns the double-submit token for this response.
func CSRFTokenFromContext(ctx context.Context) string { return FromContext(ctx).CSRFToken }

// CustomerIDFromContext returns the authenticated customer id, if any.
func CustomerIDFromContext(ctx context.Context) string { return FromContext(ctx).CustomerID }

// CartCountFromContext returns the number of items in the current cart.
func CartCountFromContext(ctx context.Context) int { return FromContext(ctx).CartCount }

// HashToken derives the stored guest key. SHA-256 keeps a database leak from
// exposing the bearer cookie while allowing an indexed lookup.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

// Manager issues and reads storefront cookies and enforces CSRF for HTML
// forms. It is intentionally separate from the admin/customer JWT middleware.
type Manager struct {
	DB        *sql.DB
	Cart      *service.CartService
	JWTSecret string
	Secure    bool
	GuestTTL  time.Duration
	CSRFTTL   time.Duration
}

// New creates a Manager. A nil cart uses a service backed by db.
func New(db *sql.DB, jwtSecret string, secure bool) *Manager {
	return &Manager{
		DB:        db,
		Cart:      service.NewCart(db),
		JWTSecret: jwtSecret,
		Secure:    secure,
		GuestTTL:  defaultGuestTTL,
		CSRFTTL:   defaultCSRFTTL,
	}
}

// Session resolves the guest session, CSRF token, optional customer and cart
// count, then stores the result on the request context.
func (m *Manager) Session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := m.prepare(w, r)
		next.ServeHTTP(w, r.WithContext(WithData(r.Context(), data)))
	})
}

func (m *Manager) prepare(w http.ResponseWriter, r *http.Request) Data {
	data := Data{}

	guest, _ := m.ensureCookie(w, r, GuestCookieName, m.guestTTL(), true)
	if guest != "" {
		data.GuestKey = HashToken(guest)
	}
	csrf, _ := m.ensureCookie(w, r, CSRFCookieName, m.csrfTTL(), false)
	data.CSRFToken = csrf

	data.CustomerID = m.customerFromRequest(r)
	if m.Cart != nil {
		if owner := data.Owner(); owner.Valid() {
			if count, err := m.Cart.Count(r.Context(), owner); err == nil {
				data.CartCount = count
			}
		}
	}
	return data
}

func (m *Manager) guestTTL() time.Duration {
	if m.GuestTTL > 0 {
		return m.GuestTTL
	}
	return defaultGuestTTL
}

func (m *Manager) csrfTTL() time.Duration {
	if m.CSRFTTL > 0 {
		return m.CSRFTTL
	}
	return defaultCSRFTTL
}

func (m *Manager) customerFromRequest(r *http.Request) string {
	token := middleware.TokenFromRequest(r)
	if token == "" || strings.TrimSpace(m.JWTSecret) == "" {
		return ""
	}
	claims, err := auth.ParseToken(m.JWTSecret, token)
	if err != nil {
		return ""
	}
	if claims.Kind != "" && claims.Kind != auth.KindCustomer {
		return ""
	}
	if m.DB == nil {
		if claims.Kind == auth.KindCustomer {
			return claims.ID
		}
		return ""
	}
	var active, deleted int
	err = m.DB.QueryRowContext(r.Context(),
		`SELECT active, deleted FROM "Customer" WHERE id = ?`, claims.ID).Scan(&active, &deleted)
	if err != nil || active == 0 || deleted != 0 {
		return ""
	}
	return claims.ID
}

func (m *Manager) ensureCookie(w http.ResponseWriter, r *http.Request, name string, ttl time.Duration, httpOnly bool) (string, bool) {
	if c, err := r.Cookie(name); err == nil {
		value := strings.TrimSpace(c.Value)
		if len(value) >= 16 && len(value) <= maxCookieValue {
			return value, false
		}
	}
	value, err := randomToken()
	if err != nil {
		return "", false
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		Expires:  time.Now().Add(ttl),
		HttpOnly: httpOnly,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	return value, true
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// ProtectCSRF requires a valid double-submit token on unsafe methods. HTMX
// requests use the X-CSRF-Token header; ordinary forms may post the same token
// in a csrf_token field. HX-Request is never treated as a security signal.
func (m *Manager) ProtectCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		expected := CSRFTokenFromContext(r.Context())
		provided := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
		if provided == "" {
			provided = formCSRFToken(r)
		}
		if expected == "" || provided == "" ||
			subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
			http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func formCSRFToken(r *http.Request) string {
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	switch {
	case strings.HasPrefix(ct, "application/x-www-form-urlencoded"):
		if err := r.ParseForm(); err == nil {
			return strings.TrimSpace(r.PostForm.Get("csrf_token"))
		}
	case strings.HasPrefix(ct, "multipart/form-data"):
		if err := r.ParseMultipartForm(1 << 20); err == nil {
			return strings.TrimSpace(r.FormValue("csrf_token"))
		}
	}
	return ""
}
