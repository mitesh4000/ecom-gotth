package handlers

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/models"
	"github.com/mitesh4000/pushti_sangar/server/internal/response"
	"github.com/mitesh4000/pushti_sangar/server/internal/upload"
)

func (a *API) serializeCategory(id string) map[string]any {
	var name, image, desc sql.NullString
	var created, updated sql.NullString
	var isDeleted, isActive int
	err := a.DB.QueryRow(`SELECT name, image, description, createdAt, updatedAt, isDeleted, isActive FROM "Category" WHERE id = ?`, id).
		Scan(&name, &image, &desc, &created, &updated, &isDeleted, &isActive)
	if err != nil {
		return nil
	}
	return withID(id, map[string]any{
		"name": nullStr(nullStringPtr(name)), "image": nullStr(nullStringPtr(image)),
		"description": nullStr(nullStringPtr(desc)),
		"createdAt":   scanTime(created), "updatedAt": scanTime(updated),
		"isDeleted": isDeleted != 0, "isActive": isActive != 0,
	})
}

func (a *API) CreateCategory(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	img, _ := upload.SaveImage(r, "image", filepath.Join(a.Cfg.UploadsDir, "Category"), "categoryImage")
	id := models.NewID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	imageVal := nullStr(strPtr(body, "image"))
	if img != "" {
		imageVal = img
	}
	_, err := a.DB.Exec(`INSERT INTO "Category" (id, name, image, description, createdAt, updatedAt, isDeleted, isActive)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?)`,
		id, nullStr(strPtr(body, "name")), imageVal, nullStr(strPtr(body, "description")), now, now, boolVal(body, "isActive", true))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "category": a.serializeCategory(id)})
}

func (a *API) GetAllCategories(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.DB.Query(`SELECT id FROM "Category" WHERE isDeleted = 0 ORDER BY createdAt DESC`)
	defer func() {
		if rows != nil {
			rows.Close()
		}
	}()
	var list []map[string]any
	if rows != nil {
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				if c := a.serializeCategory(id); c != nil {
					list = append(list, c)
				}
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"categories": list})
}

func (a *API) GetCategoryByID(w http.ResponseWriter, r *http.Request) {
	c := a.serializeCategory(chi.URLParam(r, "id"))
	if c == nil {
		response.Fail(w, "Category not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "category": c})
}

func (a *API) UpdateCategoryByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	img, _ := upload.SaveImage(r, "image", filepath.Join(a.Cfg.UploadsDir, "Category"), "categoryImage")
	sets := []string{"updatedAt = ?"}
	args := []any{time.Now().UTC().Format(time.RFC3339Nano)}
	if _, ok := body["name"]; ok {
		sets = append(sets, "name = ?")
		args = append(args, nullStr(strPtr(body, "name")))
	}
	if _, ok := body["description"]; ok {
		sets = append(sets, "description = ?")
		args = append(args, nullStr(strPtr(body, "description")))
	}
	if img != "" {
		sets = append(sets, "image = ?")
		args = append(args, img)
	}
	if _, ok := body["isActive"]; ok {
		sets = append(sets, "isActive = ?")
		args = append(args, boolVal(body, "isActive", true))
	}
	args = append(args, id)
	_, err := a.DB.Exec(`UPDATE "Category" SET `+joinComma(sets)+` WHERE id = ?`, args...)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "category": a.serializeCategory(id)})
}

func (a *API) DeleteCategoryByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, _ = a.DB.Exec(`UPDATE "Category" SET isDeleted = 1, updatedAt = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339Nano), id)
	response.OK(w, map[string]any{"success": true, "msg": "Category deleted"})
}

func (a *API) GetSubAndSubSubByCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	subRows, _ := a.DB.Query(`SELECT id, name FROM "SubCategory" WHERE categoryId = ? AND isDeleted = 0`, id)
	var subs []map[string]any
	if subRows != nil {
		defer subRows.Close()
		for subRows.Next() {
			var sid, name string
			if subRows.Scan(&sid, &name) == nil {
				subs = append(subs, withID(sid, map[string]any{"name": name, "Category": id}))
			}
		}
	}
	ssRows, _ := a.DB.Query(`SELECT id, name, subCategoryId FROM "SubSubCategory" WHERE categoryId = ? AND isDeleted = 0`, id)
	var ssubs []map[string]any
	if ssRows != nil {
		defer ssRows.Close()
		for ssRows.Next() {
			var sid, name string
			var subID sql.NullString
			if ssRows.Scan(&sid, &name, &subID) == nil {
				ssubs = append(ssubs, withID(sid, map[string]any{
					"name": name, "Category": id, "SubCategory": nullStr(nullStringPtr(subID)),
				}))
			}
		}
	}
	if subs == nil {
		subs = []map[string]any{}
	}
	if ssubs == nil {
		ssubs = []map[string]any{}
	}
	response.OK(w, map[string]any{"subCategories": subs, "subSubCategories": ssubs})
}

// --- SubCategory ---

func (a *API) serializeSubCategory(id string) map[string]any {
	var name string
	var catID sql.NullString
	var created, updated sql.NullString
	var isDeleted, isActive int
	err := a.DB.QueryRow(`SELECT name, categoryId, createdAt, updatedAt, isDeleted, isActive FROM "SubCategory" WHERE id = ?`, id).
		Scan(&name, &catID, &created, &updated, &isDeleted, &isActive)
	if err != nil {
		return nil
	}
	m := withID(id, map[string]any{
		"name": name, "Category": nullStr(nullStringPtr(catID)), "categoryId": nullStr(nullStringPtr(catID)),
		"createdAt": scanTime(created), "updatedAt": scanTime(updated),
		"isDeleted": isDeleted != 0, "isActive": isActive != 0,
	})
	if catID.Valid {
		var title sql.NullString
		_ = a.DB.QueryRow(`SELECT name FROM "Category" WHERE id = ?`, catID.String).Scan(&title)
		if title.Valid {
			m["CategoryTitle"] = title.String
		}
	}
	return m
}

func (a *API) AddSubCategory(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := a.DB.Exec(`INSERT INTO "SubCategory" (id, name, categoryId, createdAt, updatedAt, isDeleted, isActive)
		VALUES (?, ?, ?, ?, ?, 0, ?)`, id, str(body, "name"), nullStr(strPtr(body, "categoryId")), now, now, boolVal(body, "isActive", true))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "subCategory": a.serializeSubCategory(id)})
}

func (a *API) GetAllSubCategories(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.DB.Query(`SELECT id FROM "SubCategory" WHERE isDeleted = 0 ORDER BY createdAt DESC`)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				if s := a.serializeSubCategory(id); s != nil {
					list = append(list, s)
				}
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"subCategories": list})
}

func (a *API) GetSubCategoryByID(w http.ResponseWriter, r *http.Request) {
	s := a.serializeSubCategory(chi.URLParam(r, "id"))
	if s == nil {
		response.Fail(w, "not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "subCategory": s})
}

func (a *API) UpdateSubCategoryByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	_, err := a.DB.Exec(`UPDATE "SubCategory" SET name = COALESCE(?, name), categoryId = COALESCE(?, categoryId),
		isActive = COALESCE(?, isActive), updatedAt = ? WHERE id = ?`,
		nullStr(strPtr(body, "name")), nullStr(strPtr(body, "categoryId")),
		boolVal(body, "isActive", true), time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "subCategory": a.serializeSubCategory(id)})
}

func (a *API) DeleteSubCategoryByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, _ = a.DB.Exec(`UPDATE "SubCategory" SET isDeleted = 1, updatedAt = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339Nano), id)
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

// --- SubSubCategory ---

func (a *API) serializeSubSubCategory(id string) map[string]any {
	var name string
	var catID, subID sql.NullString
	var created, updated sql.NullString
	var isDeleted, isActive int
	err := a.DB.QueryRow(`SELECT name, categoryId, subCategoryId, createdAt, updatedAt, isDeleted, isActive FROM "SubSubCategory" WHERE id = ?`, id).
		Scan(&name, &catID, &subID, &created, &updated, &isDeleted, &isActive)
	if err != nil {
		return nil
	}
	m := withID(id, map[string]any{
		"name": name, "Category": nullStr(nullStringPtr(catID)), "SubCategory": nullStr(nullStringPtr(subID)),
		"categoryId": nullStr(nullStringPtr(catID)), "subCategoryId": nullStr(nullStringPtr(subID)),
		"createdAt": scanTime(created), "updatedAt": scanTime(updated),
		"isDeleted": isDeleted != 0, "isActive": isActive != 0,
	})
	return m
}

func (a *API) AddSubSubCategory(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := a.DB.Exec(`INSERT INTO "SubSubCategory" (id, name, categoryId, subCategoryId, createdAt, updatedAt, isDeleted, isActive)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?)`,
		id, str(body, "name"), nullStr(strPtr(body, "categoryId")), nullStr(strPtr(body, "subCategoryId")),
		now, now, boolVal(body, "isActive", true))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "subSubCategory": a.serializeSubSubCategory(id)})
}

func (a *API) GetAllSubSubCategories(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.DB.Query(`SELECT id FROM "SubSubCategory" WHERE isDeleted = 0 ORDER BY createdAt DESC`)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				if s := a.serializeSubSubCategory(id); s != nil {
					list = append(list, s)
				}
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"subSubCategories": list})
}

func (a *API) GetSubSubCategoryByID(w http.ResponseWriter, r *http.Request) {
	s := a.serializeSubSubCategory(chi.URLParam(r, "id"))
	if s == nil {
		response.Fail(w, "not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "subSubCategory": s})
}

func (a *API) UpdateSubSubCategoryByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	_, err := a.DB.Exec(`UPDATE "SubSubCategory" SET name = COALESCE(?, name), categoryId = COALESCE(?, categoryId),
		subCategoryId = COALESCE(?, subCategoryId), isActive = COALESCE(?, isActive), updatedAt = ? WHERE id = ?`,
		nullStr(strPtr(body, "name")), nullStr(strPtr(body, "categoryId")), nullStr(strPtr(body, "subCategoryId")),
		boolVal(body, "isActive", true), time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "subSubCategory": a.serializeSubSubCategory(id)})
}

func (a *API) DeleteSubSubCategoryByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, _ = a.DB.Exec(`UPDATE "SubSubCategory" SET isDeleted = 1, updatedAt = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339Nano), id)
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}
