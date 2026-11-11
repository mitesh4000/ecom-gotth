package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mitesh4000/pushti_sangar/server/internal/auth"
	"github.com/mitesh4000/pushti_sangar/server/internal/config"
	"github.com/mitesh4000/pushti_sangar/server/internal/middleware"
	"github.com/mitesh4000/pushti_sangar/server/internal/response"
)

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

const (
	minPasswordLength = 8
	maxPasswordLength = 72
)

type API struct {
	DB  *sql.DB
	Cfg *config.Config
}

// requestInput returns the decoded body for write methods and URL query
// parameters for GET/HEAD. Read handlers are shared between the legacy POST
// routes and the canonical GET routes, so they must not require a request body.
// DELETE requests merge query parameters into the body map without overriding
// explicit body fields, so an id can be supplied as ?id=... as well as in the
// path or a legacy body.
func (a *API) requestInput(r *http.Request) (map[string]any, error) {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return queryValues(r), nil
	}
	body, err := a.decodeBody(r)
	if r.Method == http.MethodDelete {
		for k, v := range queryValues(r) {
			if _, exists := body[k]; !exists {
				body[k] = v
			}
		}
	}
	return body, err
}

func queryValues(r *http.Request) map[string]any {
	out := map[string]any{}
	for k, vals := range r.URL.Query() {
		if len(vals) == 1 {
			out[k] = vals[0]
		} else {
			out[k] = vals
		}
	}
	return out
}

func (a *API) decodeBody(r *http.Request) (map[string]any, error) {
	body := map[string]any{}
	if r.Body == nil {
		return body, nil
	}
	maxBody := int64(32 << 20)
	if a.Cfg != nil && a.Cfg.MaxBodyBytes > 0 {
		maxBody = a.Cfg.MaxBodyBytes
	}
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxBody); err != nil {
			return body, err
		}
		for k, vals := range r.MultipartForm.Value {
			if len(vals) == 1 {
				body[k] = vals[0]
			} else {
				body[k] = vals
			}
		}
		return body, nil
	}
	if strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return body, err
		}
		for k, vals := range r.Form {
			if len(vals) == 1 {
				body[k] = vals[0]
			} else {
				body[k] = vals
			}
		}
		return body, nil
	}
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return body, err
	}
	if len(b) == 0 {
		return body, nil
	}
	if err := json.Unmarshal(b, &body); err != nil {
		return body, err
	}
	return body, nil
}

func str(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func strPtr(m map[string]any, key string) *string {
	s := str(m, key)
	if s == "" {
		return nil
	}
	return &s
}

func boolVal(m map[string]any, key string, def bool) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return def
	}
	return response.ToBool(v)
}

func floatPtr(m map[string]any, key string) *float64 {
	v, ok := m[key]
	if !ok || v == nil || v == "" {
		return nil
	}
	switch t := v.(type) {
	case float64:
		return &t
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return nil
		}
		return &f
	default:
		return nil
	}
}

func intPtr(m map[string]any, key string) *int {
	v, ok := m[key]
	if !ok || v == nil || v == "" {
		return nil
	}
	switch t := v.(type) {
	case float64:
		i := int(t)
		return &i
	case string:
		i, err := strconv.Atoi(t)
		if err != nil {
			return nil
		}
		return &i
	default:
		return nil
	}
}

func intVal(m map[string]any, key string, def int) int {
	if p := intPtr(m, key); p != nil {
		return *p
	}
	return def
}

func nullStr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func nullFloat(f *float64) any {
	if f == nil {
		return nil
	}
	return *f
}

func nullInt(i *int) any {
	if i == nil {
		return nil
	}
	return *i
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func scanTime(ns sql.NullString) time.Time {
	if !ns.Valid || ns.String == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, ns.String); err == nil {
			return t
		}
	}
	return time.Time{}
}

func scanNullTime(ns sql.NullString) *time.Time {
	t := scanTime(ns)
	if t.IsZero() {
		return nil
	}
	return &t
}

func withID(id string, m map[string]any) map[string]any {
	return response.WithID(id, m)
}

func tokenFromBody(m map[string]any) string {
	return str(m, "token")
}

func strDefault(m map[string]any, key, def string) string {
	if s := str(m, key); s != "" {
		return s
	}
	return def
}

func validEmail(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) <= 254 && emailPattern.MatchString(value)
}

func validatePassword(password string) error {
	if len(password) < minPasswordLength {
		return errors.New("password must be at least 8 characters")
	}
	if len(password) > maxPasswordLength {
		return errors.New("password must be at most 72 characters")
	}
	if strings.TrimSpace(password) == "" {
		return errors.New("password must not be blank")
	}
	return nil
}

// setAuthCookie writes the JWT cookie with flags appropriate for the current
// environment. SameSite=None is only valid together with Secure.
func (a *API) setAuthCookie(w http.ResponseWriter, token string) {
	secure := a.Cfg != nil && a.Cfg.CookieSecure
	sameSite := http.SameSiteLaxMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}
	expires := time.Now().Add(a.Cfg.TokenTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
		Expires:  expires,
	})
	// Double-submit CSRF token. It is deliberately readable by JavaScript so
	// browser clients can echo it in the X-CSRF-Token header on unsafe
	// requests. See middleware.CSRF.
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    randomHex(32),
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: sameSite,
		Expires:  expires,
	})
}

func clearAuthCookie(w http.ResponseWriter) {
	for _, name := range []string{"token", "csrf_token"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: name == "token",
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
		})
	}
}

// userIDFromRequest resolves an optional user identity. It is used by public
// endpoints such as registration where an admin may supply elevated fields.
func (a *API) userIDFromRequest(r *http.Request, body map[string]any) string {
	if id := middleware.UserIDFromContext(r.Context()); id != "" {
		return id
	}
	token := tokenFromBody(body)
	if token == "" {
		token = middleware.TokenFromRequest(r)
	}
	if token == "" || a.Cfg == nil {
		return ""
	}
	claims, err := auth.ParseToken(a.Cfg.JWTSecret, token)
	if err != nil {
		return ""
	}
	if claims.Kind != "" && claims.Kind != auth.KindUser {
		return ""
	}
	return claims.ID
}

func (a *API) customerIDFromRequest(r *http.Request, body map[string]any) string {
	if id := middleware.CustomerIDFromContext(r.Context()); id != "" {
		return id
	}
	token := tokenFromBody(body)
	if token == "" {
		token = middleware.TokenFromRequest(r)
	}
	if token == "" || a.Cfg == nil {
		return ""
	}
	claims, err := auth.ParseToken(a.Cfg.JWTSecret, token)
	if err != nil {
		return ""
	}
	if claims.Kind != "" && claims.Kind != auth.KindCustomer {
		return ""
	}
	return claims.ID
}

func hasRole(roles []string, wanted ...string) bool {
	for _, role := range roles {
		for _, w := range wanted {
			if strings.EqualFold(strings.TrimSpace(role), w) {
				return true
			}
		}
	}
	return false
}

func (a *API) isStaff(r *http.Request) bool {
	return hasRole(middleware.RolesFromContext(r.Context()), "admin", "manager", "user")
}

func (a *API) isAdminOrManager(r *http.Request) bool {
	return hasRole(middleware.RolesFromContext(r.Context()), "admin", "manager")
}

func (a *API) isAdmin(r *http.Request) bool {
	return hasRole(middleware.RolesFromContext(r.Context()), "admin")
}

// canAccessUser reports whether the actor may read or modify the target user.
func (a *API) canAccessUser(r *http.Request, id string) bool {
	if id == "" {
		return false
	}
	if middleware.UserIDFromContext(r.Context()) == id {
		return true
	}
	return a.isAdminOrManager(r)
}

// canAccessCustomer reports whether the actor owns the customer record or is
// staff. Customers never receive staff roles.
func (a *API) canAccessCustomer(r *http.Request, id string) bool {
	if id == "" {
		return false
	}
	if middleware.CustomerIDFromContext(r.Context()) == id {
		return true
	}
	return a.isStaff(r)
}

func pagination(body map[string]any) (limit, offset int) {
	limit = intVal(body, "limit", 100)
	offset = intVal(body, "offset", 0)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
