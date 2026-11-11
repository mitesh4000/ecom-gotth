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

func (a *API) serializeProduct(id string) map[string]any {
	row := a.DB.QueryRow(`
		SELECT name, description, categoryId, subCategoryId, subSubCategoryId, tags,
			pricesOriginal, pricesDiscounted, pricesCalculatedPrice, imageGallery,
			stockQuantity, hsnCode, size, shippingCharge, material, color, season, gstId, sku,
			calculationOnWeight, weightTypeId, weight, laborCost, discountOnLaborCost,
			isActive, isProductPopular, isProductNew, filters, isVariant, mainProductId,
			productColor, productSize, otherVariations, createdAt, updatedAt
		FROM "Product" WHERE id = ?`, id)

	var name, desc, catID, subID, subSubID, tags, gallery, hsn, size, material, color, season, gstID, sku sql.NullString
	var wtID, mainID, pColor, pSize, filters, otherVar sql.NullString
	var pOrig, pDisc, pCalc, ship, weight, labor, discLabor sql.NullFloat64
	var stock sql.NullInt64
	var calcW, active, popular, isNew, isVar int
	var created, updated sql.NullString

	err := row.Scan(&name, &desc, &catID, &subID, &subSubID, &tags,
		&pOrig, &pDisc, &pCalc, &gallery,
		&stock, &hsn, &size, &ship, &material, &color, &season, &gstID, &sku,
		&calcW, &wtID, &weight, &labor, &discLabor,
		&active, &popular, &isNew, &filters, &isVar, &mainID,
		&pColor, &pSize, &otherVar, &created, &updated)
	if err != nil {
		return nil
	}

	m := withID(id, map[string]any{
		"name":                  nullStr(nullStringPtr(name)),
		"description":           nullStr(nullStringPtr(desc)),
		"categoryId":            nullStr(nullStringPtr(catID)),
		"Category":              nullStr(nullStringPtr(catID)),
		"subCategoryId":         nullStr(nullStringPtr(subID)),
		"SubCategory":           nullStr(nullStringPtr(subID)),
		"subSubCategoryId":      nullStr(nullStringPtr(subSubID)),
		"SubSubCategory":        nullStr(nullStringPtr(subSubID)),
		"tags":                  response.ParseJSONField(tags.String, []any{}),
		"prices": map[string]any{
			"original":        nullFloat(nullFloatPtr(pOrig)),
			"discounted":      nullFloat(nullFloatPtr(pDisc)),
			"calculatedPrice": nullFloat(nullFloatPtr(pCalc)),
		},
		"pricesOriginal":        nullFloat(nullFloatPtr(pOrig)),
		"pricesDiscounted":      nullFloat(nullFloatPtr(pDisc)),
		"pricesCalculatedPrice": nullFloat(nullFloatPtr(pCalc)),
		"imageGallery":          response.ParseJSONField(gallery.String, []any{}),
		"stockQuantity":         nullInt(nullIntPtr(stock)),
		"hsnCode":               nullStr(nullStringPtr(hsn)),
		"size":                  nullStr(nullStringPtr(size)),
		"shippingCharge":        nullFloat(nullFloatPtr(ship)),
		"material":              nullStr(nullStringPtr(material)),
		"color":                 nullStr(nullStringPtr(color)),
		"season":                nullStr(nullStringPtr(season)),
		"gstId":                 nullStr(nullStringPtr(gstID)),
		"sku":                   nullStr(nullStringPtr(sku)),
		"calculationOnWeight":   calcW != 0,
		"weightTypeId":          nullStr(nullStringPtr(wtID)),
		"weight":                nullFloat(nullFloatPtr(weight)),
		"laborCost":             nullFloat(nullFloatPtr(labor)),
		"discountOnLaborCost":   nullFloat(nullFloatPtr(discLabor)),
		"isActive":              active != 0,
		"isProductPopular":      popular != 0,
		"isProductNew":          isNew != 0,
		"filters":               response.ParseJSONField(nullStrVal(filters), []any{}),
		"isVariant":             isVar != 0,
		"mainProductId":         nullStr(nullStringPtr(mainID)),
		"productColor":          nullStr(nullStringPtr(pColor)),
		"productSize":           nullStr(nullStringPtr(pSize)),
		"otherVariations":       response.ParseJSONField(nullStrVal(otherVar), []any{}),
		"createdAt":             scanTime(created),
		"updatedAt":             scanTime(updated),
	})
	return m
}

func nullFloatPtr(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	v := nf.Float64
	return &v
}

func nullIntPtr(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	v := int(ni.Int64)
	return &v
}

func nullStrVal(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

func (a *API) listProducts(where string, args ...any) []map[string]any {
	q := `SELECT id FROM "Product"`
	if where != "" {
		q += " WHERE " + where
	}
	q += " ORDER BY createdAt DESC"
	rows, err := a.DB.Query(q, args...)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			if p := a.serializeProduct(id); p != nil {
				list = append(list, p)
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	return list
}

func (a *API) AddProduct(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	paths, _ := upload.SaveImages(r, "imageGallery", filepath.Join(a.Cfg.UploadsDir, "Products"), "productimage", 10)
	gallery := response.ToJSONString(paths)
	if gallery == "[]" && str(body, "imageGallery") != "" {
		gallery = response.ToJSONString(body["imageGallery"])
	}
	id := models.NewID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := a.DB.Exec(`INSERT INTO "Product" (
		id, name, description, categoryId, subCategoryId, subSubCategoryId, tags,
		pricesOriginal, pricesDiscounted, pricesCalculatedPrice, imageGallery,
		stockQuantity, hsnCode, size, shippingCharge, material, color, season, gstId, sku,
		calculationOnWeight, weightTypeId, weight, laborCost, discountOnLaborCost,
		isActive, isProductPopular, isProductNew, filters, isVariant, mainProductId,
		productColor, productSize, otherVariations, createdAt, updatedAt
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, nullStr(strPtr(body, "name")), nullStr(strPtr(body, "description")),
		nullStr(strPtr(body, "categoryId")), nullStr(strPtr(body, "subCategoryId")), nullStr(strPtr(body, "subSubCategoryId")),
		response.ToJSONString(body["tags"]),
		nullFloat(floatPtr(body, "pricesOriginal")), nullFloat(floatPtr(body, "pricesDiscounted")), nullFloat(floatPtr(body, "pricesCalculatedPrice")),
		gallery,
		nullInt(intPtr(body, "stockQuantity")), nullStr(strPtr(body, "hsnCode")), nullStr(strPtr(body, "size")),
		nullFloat(floatPtr(body, "shippingCharge")), nullStr(strPtr(body, "material")), nullStr(strPtr(body, "color")),
		nullStr(strPtr(body, "season")), nullStr(strPtr(body, "gstId")), nullStr(strPtr(body, "sku")),
		boolVal(body, "calculationOnWeight", false), nullStr(strPtr(body, "weightTypeId")),
		nullFloat(floatPtr(body, "weight")), nullFloat(floatPtr(body, "laborCost")), nullFloat(floatPtr(body, "discountOnLaborCost")),
		boolVal(body, "isActive", true), boolVal(body, "isProductPopular", true), boolVal(body, "isProductNew", true),
		response.ToJSONString(body["filters"]), boolVal(body, "isVariant", false), nullStr(strPtr(body, "mainProductId")),
		nullStr(strPtr(body, "productColor")), nullStr(strPtr(body, "productSize")),
		response.ToJSONString(body["otherVariations"]), now, now,
	)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "Product added", "product": a.serializeProduct(id)})
}

func (a *API) GetAllProducts(w http.ResponseWriter, r *http.Request) {
	response.OK(w, map[string]any{"products": a.listProducts("isActive = 1 AND (isVariant = 0 OR isVariant IS NULL)")})
}

func (a *API) GetAllProductsForTable(w http.ResponseWriter, r *http.Request) {
	response.OK(w, map[string]any{"products": a.listProducts("")})
}

func (a *API) GetSpecificProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p := a.serializeProduct(id)
	if p == nil {
		response.Fail(w, "Product not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "product": p})
}

func (a *API) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	paths, _ := upload.SaveImages(r, "imageGallery", filepath.Join(a.Cfg.UploadsDir, "Products"), "productimage", 10)

	sets := []string{"updatedAt = ?"}
	args := []any{time.Now().UTC().Format(time.RFC3339Nano)}
	add := func(col string, val any) {
		sets = append(sets, col+" = ?")
		args = append(args, val)
	}
	if _, ok := body["name"]; ok {
		add("name", nullStr(strPtr(body, "name")))
	}
	if _, ok := body["description"]; ok {
		add("description", nullStr(strPtr(body, "description")))
	}
	if _, ok := body["categoryId"]; ok {
		add("categoryId", nullStr(strPtr(body, "categoryId")))
	}
	if _, ok := body["subCategoryId"]; ok {
		add("subCategoryId", nullStr(strPtr(body, "subCategoryId")))
	}
	if _, ok := body["subSubCategoryId"]; ok {
		add("subSubCategoryId", nullStr(strPtr(body, "subSubCategoryId")))
	}
	if _, ok := body["tags"]; ok {
		add("tags", response.ToJSONString(body["tags"]))
	}
	if _, ok := body["pricesOriginal"]; ok {
		add("pricesOriginal", nullFloat(floatPtr(body, "pricesOriginal")))
	}
	if _, ok := body["pricesDiscounted"]; ok {
		add("pricesDiscounted", nullFloat(floatPtr(body, "pricesDiscounted")))
	}
	if _, ok := body["pricesCalculatedPrice"]; ok {
		add("pricesCalculatedPrice", nullFloat(floatPtr(body, "pricesCalculatedPrice")))
	}
	if len(paths) > 0 {
		add("imageGallery", response.ToJSONString(paths))
	}
	if _, ok := body["stockQuantity"]; ok {
		add("stockQuantity", nullInt(intPtr(body, "stockQuantity")))
	}
	if _, ok := body["isActive"]; ok {
		add("isActive", boolVal(body, "isActive", true))
	}
	if _, ok := body["gstId"]; ok {
		add("gstId", nullStr(strPtr(body, "gstId")))
	}
	args = append(args, id)
	_, err := a.DB.Exec(`UPDATE "Product" SET `+joinComma(sets)+` WHERE id = ?`, args...)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "Product updated", "product": a.serializeProduct(id)})
}

func (a *API) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := a.DB.Exec(`DELETE FROM "Product" WHERE id = ?`, id)
	if err != nil {
		response.Fail(w, "Product not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "Product deleted"})
}

func (a *API) GetProductsByCategoryID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	response.OK(w, map[string]any{"products": a.listProducts("categoryId = ? AND isActive = 1", id)})
}

func (a *API) GetProductsBySubCategoryID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	response.OK(w, map[string]any{"products": a.listProducts("subCategoryId = ? AND isActive = 1", id)})
}

func (a *API) GetProductsBySubSubCategoryID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	response.OK(w, map[string]any{"products": a.listProducts("subSubCategoryId = ? AND isActive = 1", id)})
}

func (a *API) GetProductsByTag(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	tag := str(body, "tag")
	response.OK(w, map[string]any{"products": a.listProducts("tags LIKE ? AND isActive = 1", "%"+tag+"%")})
}

func (a *API) AddVarProduct(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	body["isVariant"] = true
	a.AddProduct(w, r)
}

func (a *API) GetVarProductByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	response.OK(w, map[string]any{"products": a.listProducts("mainProductId = ?", id)})
}

func (a *API) GetAllVarProducts(w http.ResponseWriter, r *http.Request) {
	response.OK(w, map[string]any{"products": a.listProducts("isVariant = 1")})
}
