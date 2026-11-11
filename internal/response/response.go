package response

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// JSON writes a JSON response (matches Express res.send style used by frontends).
func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func OK(w http.ResponseWriter, payload any) {
	JSON(w, http.StatusOK, payload)
}

func Success(w http.ResponseWriter, msg string, extra map[string]any) {
	body := map[string]any{"success": true, "msg": msg}
	for k, v := range extra {
		body[k] = v
	}
	OK(w, body)
}

func Fail(w http.ResponseWriter, msg string) {
	OK(w, map[string]any{"success": false, "msg": msg})
}

// FailErr logs the internal error and returns a generic message. Database and
// filesystem details must never reach an API client.
func FailErr(w http.ResponseWriter, err error) {
	slog.Error("request failed", "error", err)
	JSON(w, http.StatusInternalServerError, map[string]any{
		"success": false,
		"msg":     "Internal Server Error",
	})
}

// FailConflict reports a duplicate/unique-constraint style error without
// leaking the database message.
func FailConflict(w http.ResponseWriter, msg string) {
	JSON(w, http.StatusConflict, map[string]any{"success": false, "msg": msg})
}

func FailStatus(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]any{"success": false, "msg": msg})
}

// WithID adds both _id and id fields expected by the React frontends.
func WithID(id string, m map[string]any) map[string]any {
	if m == nil {
		m = map[string]any{}
	}
	m["_id"] = id
	m["id"] = id
	return m
}

func ParseJSONField(s string, fallback any) any {
	if s == "" {
		return fallback
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return fallback
	}
	return v
}

func ToJSONString(v any) string {
	if v == nil {
		return "[]"
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return "[]"
		}
		return string(b)
	}
}

func ToBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	case float64:
		return t != 0
	case int:
		return t != 0
	default:
		return false
	}
}
