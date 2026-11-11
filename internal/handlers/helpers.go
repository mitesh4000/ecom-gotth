package handlers

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mitesh4000/pushti_sangar/server/internal/config"
	"github.com/mitesh4000/pushti_sangar/server/internal/response"
)

type API struct {
	DB  *sql.DB
	Cfg *config.Config
}

func (a *API) decodeBody(r *http.Request) (map[string]any, error) {
	body := map[string]any{}
	if r.Body == nil {
		return body, nil
	}
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
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
