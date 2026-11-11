package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/auth"
	"github.com/mitesh4000/pushti_sangar/server/internal/middleware"
	"github.com/mitesh4000/pushti_sangar/server/internal/models"
	"github.com/mitesh4000/pushti_sangar/server/internal/response"
	"github.com/mitesh4000/pushti_sangar/server/internal/upload"
)

func (a *API) serializeUser(id string) map[string]any {
	var name, email, photo string
	var active int
	var created, updated sql.NullString
	err := a.DB.QueryRow(`SELECT name, email, photo, active, createdAt, updatedAt FROM "User" WHERE id = ?`, id).
		Scan(&name, &email, &photo, &active, &created, &updated)
	if err != nil {
		return nil
	}
	roles := a.userRoles(id)
	return withID(id, map[string]any{
		"name":      name,
		"email":     email,
		"photo":     photo,
		"active":    active != 0,
		"createdAt": scanTime(created),
		"updatedAt": scanTime(updated),
		"roles":     roles,
	})
}

func (a *API) userRoles(userID string) []string {
	rows, err := a.DB.Query(`
		SELECT r.role FROM "Role" r
		JOIN "UserRole" ur ON ur.roleId = r.id
		WHERE ur.userId = ?`, userID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err == nil {
			roles = append(roles, role)
		}
	}
	if roles == nil {
		roles = []string{}
	}
	return roles
}

func (a *API) syncUserRoles(userID string, roleNames []string) error {
	_, _ = a.DB.Exec(`DELETE FROM "UserRole" WHERE userId = ?`, userID)
	seen := map[string]struct{}{}
	for _, name := range roleNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		var roleID string
		err := a.DB.QueryRow(`SELECT id FROM "Role" WHERE role = ?`, name).Scan(&roleID)
		if err == sql.ErrNoRows {
			roleID = models.NewID()
			_, err = a.DB.Exec(`INSERT INTO "Role" (id, role, permissions) VALUES (?, ?, '[]')`, roleID, name)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		_, err = a.DB.Exec(`INSERT INTO "UserRole" (id, userId, roleId) VALUES (?, ?, ?)`, models.NewID(), userID, roleID)
		if err != nil {
			return err
		}
	}
	return nil
}

func normalizeRoles(v any) []string {
	switch t := v.(type) {
	case []any:
		var out []string
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
		return out
	case []string:
		return t
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		if strings.HasPrefix(s, "[") {
			var arr []string
			if jsonUnmarshal(s, &arr) == nil {
				return arr
			}
		}
		parts := strings.Split(s, ",")
		var out []string
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	default:
		return nil
	}
}

func jsonUnmarshal(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}

func (a *API) RegisterUser(w http.ResponseWriter, r *http.Request) {
	body, err := a.decodeBody(r)
	if err != nil {
		response.Fail(w, "invalid body")
		return
	}
	password := str(body, "password")
	if password != str(body, "confirmPassword") {
		response.Fail(w, "Passwords do not match")
		return
	}
	photo, _ := upload.SaveImage(r, "photo", filepath.Join(a.Cfg.UploadsDir, "Users"), "userphoto")
	hash, err := auth.HashPassword(password)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	id := models.NewID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = a.DB.Exec(`INSERT INTO "User" (id, name, email, password, photo, active, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, str(body, "name"), str(body, "email"), hash, photo, boolVal(body, "active", true), now, now)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	_ = a.syncUserRoles(id, normalizeRoles(body["roles"]))
	response.Success(w, "User added successfully", map[string]any{"user": a.serializeUser(id)})
}

func (a *API) LoginUser(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	email, password := str(body, "email"), str(body, "password")
	if email == "" || password == "" {
		response.Fail(w, "Please fill all required fields")
		return
	}
	var id, hash string
	err := a.DB.QueryRow(`SELECT id, password FROM "User" WHERE email = ?`, email).Scan(&id, &hash)
	if err == sql.ErrNoRows {
		response.Fail(w, "User not found")
		return
	}
	if err != nil || !auth.CheckPassword(hash, password) {
		response.Fail(w, "Invalid credentials")
		return
	}
	token, err := auth.GenerateToken(a.Cfg.JWTSecret, id, a.Cfg.TokenTTL)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
		Expires:  time.Now().Add(a.Cfg.TokenTTL),
	})
	user := a.serializeUser(id)
	roles, _ := user["roles"].([]string)
	var role any
	if len(roles) > 0 {
		role = roles[0]
	}
	response.OK(w, map[string]any{
		"success": true,
		"msg":     "Successfully LoggedIn",
		"token":   token,
		"user":    user,
		"role":    role,
	})
}

func (a *API) LogoutUser(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "token", Value: "", Path: "/", HttpOnly: true, Expires: time.Unix(0, 0)})
	response.OK(w, map[string]any{"message": "Successfully logged out"})
}

func (a *API) GetLoggedInUser(w http.ResponseWriter, r *http.Request) {
	id := middleware.UserIDFromContext(r.Context())
	if id == "" {
		body, _ := a.decodeBody(r)
		if t := tokenFromBody(body); t != "" {
			if c, err := auth.ParseToken(a.Cfg.JWTSecret, t); err == nil {
				id = c.ID
			}
		}
	}
	if id == "" {
		response.Fail(w, "Not authorized")
		return
	}
	user := a.serializeUser(id)
	if user == nil {
		response.Fail(w, "User not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "user": user})
}

func (a *API) GetSpecificUser(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := str(body, "id")
	if id == "" {
		id = chi.URLParam(r, "id")
	}
	user := a.serializeUser(id)
	if user == nil {
		response.Fail(w, "User not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "user": user})
}

func (a *API) LoginStatus(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	token := tokenFromBody(body)
	if token == "" {
		response.OK(w, map[string]any{"success": false})
		return
	}
	_, err := auth.ParseToken(a.Cfg.JWTSecret, token)
	response.OK(w, map[string]any{"success": err == nil})
}

func (a *API) GetUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`SELECT id FROM "User" ORDER BY createdAt DESC`)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	defer rows.Close()
	var users []map[string]any
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			if u := a.serializeUser(id); u != nil {
				users = append(users, u)
			}
		}
	}
	if users == nil {
		users = []map[string]any{}
	}
	response.OK(w, map[string]any{"users": users})
}

func (a *API) UpdateUser(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := str(body, "id")
	if id == "" {
		id = middleware.UserIDFromContext(r.Context())
	}
	if id == "" {
		response.Fail(w, "User id required")
		return
	}
	photo, err := upload.SaveImage(r, "photo", filepath.Join(a.Cfg.UploadsDir, "Users"), "userphoto")
	sets := []string{}
	args := []any{}
	if name := str(body, "name"); name != "" {
		sets = append(sets, "name = ?")
		args = append(args, name)
	}
	if email := str(body, "email"); email != "" {
		sets = append(sets, "email = ?")
		args = append(args, email)
	}
	if err == nil && photo != "" {
		sets = append(sets, "photo = ?")
		args = append(args, photo)
	}
	if _, ok := body["active"]; ok {
		sets = append(sets, "active = ?")
		args = append(args, boolVal(body, "active", true))
	}
	sets = append(sets, "updatedAt = ?")
	args = append(args, time.Now().UTC().Format(time.RFC3339Nano))
	args = append(args, id)
	if len(sets) > 1 {
		_, err = a.DB.Exec(`UPDATE "User" SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
		if err != nil {
			response.FailErr(w, err)
			return
		}
	}
	if roles := normalizeRoles(body["roles"]); roles != nil {
		_ = a.syncUserRoles(id, roles)
	}
	response.Success(w, "User updated successfully", map[string]any{"user": a.serializeUser(id)})
}

func (a *API) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := middleware.UserIDFromContext(r.Context())
	if id == "" {
		id = str(body, "id")
	}
	var hash string
	if err := a.DB.QueryRow(`SELECT password FROM "User" WHERE id = ?`, id).Scan(&hash); err != nil {
		response.Fail(w, "User not found , Please signup")
		return
	}
	oldP, newP := str(body, "oldPassword"), str(body, "newPassword")
	if oldP == "" || newP == "" {
		response.Fail(w, "Please add old and new password")
		return
	}
	if !auth.CheckPassword(hash, oldP) {
		response.Fail(w, "Old password is incorrect")
		return
	}
	if oldP == newP {
		response.Fail(w, "New Password cannot be same as Old password")
		return
	}
	nh, _ := auth.HashPassword(newP)
	_, _ = a.DB.Exec(`UPDATE "User" SET password = ?, updatedAt = ? WHERE id = ?`, nh, time.Now().UTC().Format(time.RFC3339Nano), id)
	response.Success(w, "Password changed successfully", nil)
}

func (a *API) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	email := str(body, "email")
	var id, name string
	err := a.DB.QueryRow(`SELECT id, name FROM "User" WHERE email = ?`, email).Scan(&id, &name)
	if err != nil {
		response.Fail(w, "User not found")
		return
	}
	_, _ = a.DB.Exec(`DELETE FROM "Token" WHERE userId = ?`, id)
	raw := randomHex(32) + id
	h := sha256.Sum256([]byte(raw))
	hashed := hex.EncodeToString(h[:])
	now := time.Now().UTC()
	_, err = a.DB.Exec(`INSERT INTO "Token" (id, userId, token, createdAt, expiresAt) VALUES (?, ?, ?, ?, ?)`,
		models.NewID(), id, hashed, now.Format(time.RFC3339Nano), now.Add(30*time.Minute).Format(time.RFC3339Nano))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	resetURL := fmt.Sprintf("%s/resetpassword/%s", a.Cfg.AdminURL, raw)
	_ = a.sendEmail("Password reset request", fmt.Sprintf("<h2>Hello %s</h2><p><a href=%s>%s</a></p>", name, resetURL, resetURL), email)
	response.Success(w, "Reset Email Sent", nil)
}

func (a *API) ResetPassword(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	resetToken := chi.URLParam(r, "resetToken")
	h := sha256.Sum256([]byte(resetToken))
	hashed := hex.EncodeToString(h[:])
	var userID sql.NullString
	err := a.DB.QueryRow(`SELECT userId FROM "Token" WHERE token = ? AND expiresAt > ?`, hashed, time.Now().UTC().Format(time.RFC3339Nano)).Scan(&userID)
	if err != nil || !userID.Valid {
		response.OK(w, "Invalid or Expired Token")
		return
	}
	nh, _ := auth.HashPassword(str(body, "password"))
	_, _ = a.DB.Exec(`UPDATE "User" SET password = ?, updatedAt = ? WHERE id = ?`, nh, time.Now().UTC().Format(time.RFC3339Nano), userID.String)
	response.Success(w, "Password Reset Successful, Please Login", nil)
}

func (a *API) DeleteUser(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := str(body, "id")
	_, err := a.DB.Exec(`DELETE FROM "User" WHERE id = ?`, id)
	if err != nil {
		response.OK(w, map[string]any{"success": true, "message": "record not found"})
		return
	}
	response.OK(w, map[string]any{"success": true, "message": "Successfully Deleted"})
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
