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

// --- Roles ---

func (a *API) GetRoles(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`SELECT id, role, permissions FROM "Role"`)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var id, role, perms string
		if rows.Scan(&id, &role, &perms) == nil {
			list = append(list, withID(id, map[string]any{
				"role": role, "permissions": response.ParseJSONField(perms, []any{}),
			}))
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"roles": list})
}

func (a *API) CreateRole(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	_, err := a.DB.Exec(`INSERT INTO "Role" (id, role, permissions) VALUES (?, ?, ?)`,
		id, str(body, "role"), response.ToJSONString(body["permissions"]))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "role": withID(id, map[string]any{
		"role": str(body, "role"), "permissions": response.ParseJSONField(response.ToJSONString(body["permissions"]), []any{}),
	})})
}

func (a *API) DeleteRole(w http.ResponseWriter, r *http.Request) {
	body, _ := a.requestInput(r)
	id := str(body, "id")
	if id == "" {
		id = chi.URLParam(r, "id")
	}
	if id == "" {
		response.Fail(w, "Role id required")
		return
	}
	_, _ = a.DB.Exec(`DELETE FROM "Role" WHERE id = ?`, id)
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

// --- GST ---

func (a *API) GetGSTs(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.DB.Query(`SELECT id, gst, sgst, cgst, igst, updatedAt, isDeleted, isActive FROM "Gst" WHERE isDeleted = 0`)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			var gst float64
			var sgst, cgst, igst sql.NullFloat64
			var updated sql.NullString
			var del, active int
			if rows.Scan(&id, &gst, &sgst, &cgst, &igst, &updated, &del, &active) == nil {
				list = append(list, withID(id, map[string]any{
					"gst": gst, "sgst": nullFloat(nullFloatPtr(sgst)), "cgst": nullFloat(nullFloatPtr(cgst)),
					"igst": nullFloat(nullFloatPtr(igst)), "updatedAt": scanTime(updated),
					"isDeleted": del != 0, "isActive": active != 0,
				}))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"gsts": list})
}

func (a *API) CreateGST(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	g := floatPtr(body, "gst")
	var gv float64
	if g != nil {
		gv = *g
	}
	_, err := a.DB.Exec(`INSERT INTO "Gst" (id, gst, sgst, cgst, igst, updatedAt, isDeleted, isActive) VALUES (?,?,?,?,?,?,0,1)`,
		id, gv, nullFloat(floatPtr(body, "sgst")), nullFloat(floatPtr(body, "cgst")), nullFloat(floatPtr(body, "igst")), now)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "GST created"})
}

func (a *API) UpdateGST(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	_, _ = a.DB.Exec(`UPDATE "Gst" SET gst = COALESCE(?, gst), sgst = COALESCE(?, sgst), cgst = COALESCE(?, cgst),
		igst = COALESCE(?, igst), updatedAt = ? WHERE id = ?`,
		nullFloat(floatPtr(body, "gst")), nullFloat(floatPtr(body, "sgst")), nullFloat(floatPtr(body, "cgst")),
		nullFloat(floatPtr(body, "igst")), time.Now().UTC().Format(time.RFC3339Nano), id)
	response.OK(w, map[string]any{"success": true, "msg": "updated"})
}

func (a *API) DeleteGST(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, _ = a.DB.Exec(`UPDATE "Gst" SET isDeleted = 1 WHERE id = ?`, id)
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

// --- Coupons ---

func (a *API) GetCoupons(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.DB.Query(`SELECT id, name, type, description, start, expiry, discount, active, createdAt, updatedAt FROM "Coupon"`)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, name, typ string
			var desc sql.NullString
			var start, expiry, created, updated sql.NullString
			var disc sql.NullFloat64
			var active int
			if rows.Scan(&id, &name, &typ, &desc, &start, &expiry, &disc, &active, &created, &updated) == nil {
				list = append(list, withID(id, map[string]any{
					"name": name, "type": typ, "description": nullStr(nullStringPtr(desc)),
					"start": scanNullTime(start), "expiry": scanNullTime(expiry),
					"discount": nullFloat(nullFloatPtr(disc)), "active": active != 0,
					"createdAt": scanTime(created), "updatedAt": scanTime(updated),
				}))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"coupons": list})
}

func (a *API) CreateCoupon(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := a.DB.Exec(`INSERT INTO "Coupon" (id, name, type, description, start, expiry, discount, active, createdAt, updatedAt)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		id, str(body, "name"), strDefault(body, "type", "%"), nullStr(strPtr(body, "description")),
		nullStr(strPtr(body, "start")), nullStr(strPtr(body, "expiry")), nullFloat(floatPtr(body, "discount")),
		boolVal(body, "active", true), now, now)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "Coupon created"})
}

func (a *API) UpdateCoupon(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	_, _ = a.DB.Exec(`UPDATE "Coupon" SET name = COALESCE(?, name), type = COALESCE(?, type), description = COALESCE(?, description),
		discount = COALESCE(?, discount), active = COALESCE(?, active), updatedAt = ? WHERE id = ?`,
		nullStr(strPtr(body, "name")), nullStr(strPtr(body, "type")), nullStr(strPtr(body, "description")),
		nullFloat(floatPtr(body, "discount")), boolVal(body, "active", true),
		time.Now().UTC().Format(time.RFC3339Nano), id)
	response.OK(w, map[string]any{"success": true, "msg": "updated"})
}

func (a *API) DeleteCoupon(w http.ResponseWriter, r *http.Request) {
	_, _ = a.DB.Exec(`DELETE FROM "Coupon" WHERE id = ?`, chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

// --- Stock ---

func (a *API) serializeStock(id string) map[string]any {
	var pid, name, color, size sql.NullString
	var qty int
	var price float64
	var date sql.NullString
	err := a.DB.QueryRow(`SELECT productId, name, color, size, quantity, currentPricePerUnit, date FROM "Stock" WHERE id = ?`, id).
		Scan(&pid, &name, &color, &size, &qty, &price, &date)
	if err != nil {
		return nil
	}
	return withID(id, map[string]any{
		"ProductId": nullStr(nullStringPtr(pid)), "productId": nullStr(nullStringPtr(pid)),
		"name": nullStr(nullStringPtr(name)), "color": nullStr(nullStringPtr(color)), "size": nullStr(nullStringPtr(size)),
		"quantity": qty, "currentPricePerUnit": price, "date": scanNullTime(date),
	})
}

func (a *API) GetAllStock(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.DB.Query(`SELECT id FROM "Stock"`)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				if s := a.serializeStock(id); s != nil {
					list = append(list, s)
				}
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"stocks": list})
}

func (a *API) CreateStock(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	qty := intVal(body, "quantity", 0)
	price := 0.0
	if p := floatPtr(body, "currentPricePerUnit"); p != nil {
		price = *p
	}
	_, err := a.DB.Exec(`INSERT INTO "Stock" (id, productId, name, color, size, quantity, currentPricePerUnit, date)
		VALUES (?,?,?,?,?,?,?,?)`,
		id, nullStr(strPtr(body, "productId")), nullStr(strPtr(body, "name")), nullStr(strPtr(body, "color")),
		nullStr(strPtr(body, "size")), qty, price, nullStr(strPtr(body, "date")))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "stock": a.serializeStock(id)})
}

func (a *API) GetStockByID(w http.ResponseWriter, r *http.Request) {
	s := a.serializeStock(chi.URLParam(r, "id"))
	if s == nil {
		response.Fail(w, "not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "stock": s})
}

func (a *API) UpdateStockByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	_, _ = a.DB.Exec(`UPDATE "Stock" SET quantity = COALESCE(?, quantity), currentPricePerUnit = COALESCE(?, currentPricePerUnit),
		name = COALESCE(?, name), color = COALESCE(?, color), size = COALESCE(?, size) WHERE id = ?`,
		nullInt(intPtr(body, "quantity")), nullFloat(floatPtr(body, "currentPricePerUnit")),
		nullStr(strPtr(body, "name")), nullStr(strPtr(body, "color")), nullStr(strPtr(body, "size")), id)
	response.OK(w, map[string]any{"success": true, "stock": a.serializeStock(id)})
}

func (a *API) DeleteStockByID(w http.ResponseWriter, r *http.Request) {
	_, _ = a.DB.Exec(`DELETE FROM "Stock" WHERE id = ?`, chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

func (a *API) GetStockByCategory(w http.ResponseWriter, r *http.Request) {
	catID := chi.URLParam(r, "id")
	rows, _ := a.DB.Query(`SELECT s.id FROM "Stock" s JOIN "Product" p ON p.id = s.productId WHERE p.categoryId = ?`, catID)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				list = append(list, a.serializeStock(id))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"stocks": list})
}

func (a *API) GetStockByPriceUpdate(w http.ResponseWriter, r *http.Request) {
	wtID := chi.URLParam(r, "id")
	rows, _ := a.DB.Query(`SELECT s.id FROM "Stock" s JOIN "Product" p ON p.id = s.productId WHERE p.weightTypeId = ?`, wtID)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				list = append(list, a.serializeStock(id))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"stocks": list})
}

// --- Simple masters: Color, Material, Season, Size, PriceUpdate ---

func (a *API) GetColors(w http.ResponseWriter, r *http.Request) {
	a.listSimple(w, `SELECT id, name, createdAt FROM "Color"`, "colors", true)
}

func (a *API) AddColor(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	_, _ = upload.SaveImage(r, "photo", filepath.Join(a.Cfg.UploadsDir, "Color"), "color")
	id := models.NewID()
	_, err := a.DB.Exec(`INSERT INTO "Color" (id, name, createdAt) VALUES (?, ?, ?)`,
		id, nullStr(strPtr(body, "name")), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "Color added"})
}

func (a *API) GetColorByID(w http.ResponseWriter, r *http.Request) {
	var id, name string
	var created sql.NullString
	id = chi.URLParam(r, "id")
	err := a.DB.QueryRow(`SELECT name, createdAt FROM "Color" WHERE id = ?`, id).Scan(&name, &created)
	if err != nil {
		response.Fail(w, "not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "color": withID(id, map[string]any{"name": name, "createdAt": scanTime(created)})})
}

func (a *API) UpdateColor(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	_, _ = a.DB.Exec(`UPDATE "Color" SET name = ? WHERE id = ?`, nullStr(strPtr(body, "name")), chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "updated"})
}

func (a *API) DeleteColor(w http.ResponseWriter, r *http.Request) {
	_, _ = a.DB.Exec(`DELETE FROM "Color" WHERE id = ?`, chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

func (a *API) GetMaterials(w http.ResponseWriter, r *http.Request) {
	a.listSimple(w, `SELECT id, name, createdAt FROM "Material"`, "materials", true)
}

func (a *API) AddMaterial(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	_, err := a.DB.Exec(`INSERT INTO "Material" (id, name, createdAt) VALUES (?, ?, ?)`,
		id, nullStr(strPtr(body, "name")), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "Material added"})
}

func (a *API) GetSeasons(w http.ResponseWriter, r *http.Request) {
	a.listSimple(w, `SELECT id, name, createdAt FROM "Season"`, "seasons", true)
}

func (a *API) AddSeason(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	_, err := a.DB.Exec(`INSERT INTO "Season" (id, name, createdAt) VALUES (?, ?, ?)`,
		id, nullStr(strPtr(body, "name")), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "Season added"})
}

func (a *API) GetSizes(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.DB.Query(`SELECT id, size FROM "Size"`)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			var size sql.NullInt64
			if rows.Scan(&id, &size) == nil {
				list = append(list, withID(id, map[string]any{"size": nullInt(nullIntPtr(size))}))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"sizes": list})
}

func (a *API) AddSize(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	_, err := a.DB.Exec(`INSERT INTO "Size" (id, size) VALUES (?, ?)`, id, nullInt(intPtr(body, "size")))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "Size added"})
}

func (a *API) GetSizeByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var size sql.NullInt64
	if err := a.DB.QueryRow(`SELECT size FROM "Size" WHERE id = ?`, id).Scan(&size); err != nil {
		response.Fail(w, "not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "size": withID(id, map[string]any{"size": nullInt(nullIntPtr(size))})})
}

func (a *API) UpdateSize(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	_, _ = a.DB.Exec(`UPDATE "Size" SET size = ? WHERE id = ?`, nullInt(intPtr(body, "size")), chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "updated"})
}

func (a *API) DeleteSize(w http.ResponseWriter, r *http.Request) {
	_, _ = a.DB.Exec(`DELETE FROM "Size" WHERE id = ?`, chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

func (a *API) GetDailyRates(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.DB.Query(`SELECT id, productType, price FROM "PriceUpdate"`)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			var pt sql.NullString
			var price sql.NullFloat64
			if rows.Scan(&id, &pt, &price) == nil {
				list = append(list, withID(id, map[string]any{
					"productType": nullStr(nullStringPtr(pt)), "price": nullFloat(nullFloatPtr(price)),
				}))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"rates": list})
}

func (a *API) CreateDailyRate(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	_, err := a.DB.Exec(`INSERT INTO "PriceUpdate" (id, productType, price) VALUES (?, ?, ?)`,
		id, nullStr(strPtr(body, "productType")), nullFloat(floatPtr(body, "price")))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "created"})
}

func (a *API) UpdateDailyRate(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	_, _ = a.DB.Exec(`UPDATE "PriceUpdate" SET productType = COALESCE(?, productType), price = COALESCE(?, price) WHERE id = ?`,
		nullStr(strPtr(body, "productType")), nullFloat(floatPtr(body, "price")), chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "updated"})
}

func (a *API) DeleteDailyRate(w http.ResponseWriter, r *http.Request) {
	_, _ = a.DB.Exec(`DELETE FROM "PriceUpdate" WHERE id = ?`, chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

func (a *API) listSimple(w http.ResponseWriter, query, key string, hasCreated bool) {
	rows, err := a.DB.Query(query)
	if err != nil {
		response.OK(w, map[string]any{key: []any{}})
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var id string
		var name sql.NullString
		var created sql.NullString
		var err error
		if hasCreated {
			err = rows.Scan(&id, &name, &created)
		} else {
			err = rows.Scan(&id, &name)
		}
		if err == nil {
			m := withID(id, map[string]any{"name": nullStr(nullStringPtr(name))})
			if hasCreated {
				m["createdAt"] = scanTime(created)
			}
			list = append(list, m)
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{key: list})
}
