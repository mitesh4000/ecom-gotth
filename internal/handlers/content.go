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

// --- Banner ---

func (a *API) GetBanners(w http.ResponseWriter, r *http.Request) {
	a.listBanners(w, `isDeleted = 0 AND isActive = 1`)
}

func (a *API) GetAllIncDelBanners(w http.ResponseWriter, r *http.Request) {
	a.listBanners(w, "1=1")
}

func (a *API) listBanners(w http.ResponseWriter, where string) {
	rows, err := a.DB.Query(`SELECT id, bannerTitle, image, description, bannerType, bannerText, createdAt, updatedAt, isDeleted, isActive
		FROM "Banner" WHERE ` + where)
	if err != nil {
		response.OK(w, map[string]any{"banners": []any{}})
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var id string
		var title, image, desc, btype, btext sql.NullString
		var created, updated sql.NullString
		var del, active int
		if rows.Scan(&id, &title, &image, &desc, &btype, &btext, &created, &updated, &del, &active) == nil {
			list = append(list, withID(id, map[string]any{
				"bannerTitle": nullStr(nullStringPtr(title)), "image": nullStr(nullStringPtr(image)),
				"description": nullStr(nullStringPtr(desc)), "bannerType": nullStr(nullStringPtr(btype)),
				"bannerText": nullStr(nullStringPtr(btext)),
				"createdAt":  scanTime(created), "updatedAt": scanTime(updated),
				"isDeleted": del != 0, "isActive": active != 0,
			}))
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"banners": list})
}

func (a *API) AddBanner(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	img, _ := upload.SaveImage(r, "image", filepath.Join(a.Cfg.UploadsDir, "Banner"), "banner")
	id := models.NewID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := a.DB.Exec(`INSERT INTO "Banner" (id, bannerTitle, image, description, bannerType, bannerText, createdAt, updatedAt, isDeleted, isActive)
		VALUES (?,?,?,?,?,?,?,?,0,1)`,
		id, nullStr(strPtr(body, "bannerTitle")), img, nullStr(strPtr(body, "description")),
		nullStr(strPtr(body, "bannerType")), nullStr(strPtr(body, "bannerText")), now, now)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "Banner added"})
}

func (a *API) GetBannerByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rows, _ := a.DB.Query(`SELECT id, bannerTitle, image, description, bannerType, bannerText, createdAt, updatedAt, isDeleted, isActive
		FROM "Banner" WHERE id = ?`, id)
	if rows == nil {
		response.Fail(w, "not found")
		return
	}
	defer rows.Close()
	if !rows.Next() {
		response.Fail(w, "not found")
		return
	}
	var title, image, desc, btype, btext sql.NullString
	var created, updated sql.NullString
	var del, active int
	_ = rows.Scan(&id, &title, &image, &desc, &btype, &btext, &created, &updated, &del, &active)
	response.OK(w, map[string]any{"success": true, "banner": withID(id, map[string]any{
		"bannerTitle": nullStr(nullStringPtr(title)), "image": nullStr(nullStringPtr(image)),
		"description": nullStr(nullStringPtr(desc)), "bannerType": nullStr(nullStringPtr(btype)),
		"bannerText": nullStr(nullStringPtr(btext)),
		"createdAt":  scanTime(created), "updatedAt": scanTime(updated),
		"isDeleted": del != 0, "isActive": active != 0,
	})})
}

func (a *API) UpdateBanner(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	img, _ := upload.SaveImage(r, "image", filepath.Join(a.Cfg.UploadsDir, "Banner"), "banner")
	sets := []string{"updatedAt = ?"}
	args := []any{time.Now().UTC().Format(time.RFC3339Nano)}
	if _, ok := body["bannerTitle"]; ok {
		sets = append(sets, "bannerTitle = ?")
		args = append(args, nullStr(strPtr(body, "bannerTitle")))
	}
	if img != "" {
		sets = append(sets, "image = ?")
		args = append(args, img)
	}
	args = append(args, id)
	_, _ = a.DB.Exec(`UPDATE "Banner" SET `+joinComma(sets)+` WHERE id = ?`, args...)
	response.OK(w, map[string]any{"success": true, "msg": "updated"})
}

func (a *API) DeleteBanner(w http.ResponseWriter, r *http.Request) {
	_, _ = a.DB.Exec(`UPDATE "Banner" SET isDeleted = 1 WHERE id = ?`, chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

// --- Blog ---

func (a *API) GetBlogs(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.DB.Query(`SELECT id FROM "Blog" WHERE deleted = 0 AND active = 1 ORDER BY date DESC`)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				list = append(list, a.serializeBlog(id))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"blogs": list})
}

func (a *API) serializeBlog(id string) map[string]any {
	var feed, title, image, blog string
	var desc, catID sql.NullString
	var date, created, updated, deletedAt sql.NullString
	var del, active int
	err := a.DB.QueryRow(`SELECT blogFeed, blogTitle, imagePath, description, blogCategoryId, date, blog, deleted, active, createdAt, updatedAt, deletedAt
		FROM "Blog" WHERE id = ?`, id).
		Scan(&feed, &title, &image, &desc, &catID, &date, &blog, &del, &active, &created, &updated, &deletedAt)
	if err != nil {
		return nil
	}
	return withID(id, map[string]any{
		"blogFeed": feed, "blogTitle": title, "imagePath": image,
		"description": nullStr(nullStringPtr(desc)), "blogCategory": nullStr(nullStringPtr(catID)),
		"blogCategoryId": nullStr(nullStringPtr(catID)), "date": scanTime(date), "blog": blog,
		"deleted": del != 0, "active": active != 0,
		"createdAt": scanTime(created), "updatedAt": scanNullTime(updated), "deletedAt": scanNullTime(deletedAt),
	})
}

func (a *API) AddBlog(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	img, _ := upload.SaveImage(r, "imagePath", filepath.Join(a.Cfg.UploadsDir, "Blog"), "blog")
	id := models.NewID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := a.DB.Exec(`INSERT INTO "Blog" (id, blogFeed, blogTitle, imagePath, description, blogCategoryId, date, blog, deleted, active, createdAt)
		VALUES (?,?,?,?,?,?,?,?,0,1,?)`,
		id, str(body, "blogFeed"), str(body, "blogTitle"), img, nullStr(strPtr(body, "description")),
		nullStr(strPtr(body, "blogCategoryId")), now, str(body, "blog"), now)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "blog": a.serializeBlog(id)})
}

func (a *API) GetBlogByID(w http.ResponseWriter, r *http.Request) {
	b := a.serializeBlog(chi.URLParam(r, "id"))
	if b == nil {
		response.Fail(w, "not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "blog": b})
}

func (a *API) GetBlogByCategory(w http.ResponseWriter, r *http.Request) {
	cat := chi.URLParam(r, "id")
	rows, _ := a.DB.Query(`SELECT id FROM "Blog" WHERE blogCategoryId = ? AND deleted = 0`, cat)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				list = append(list, a.serializeBlog(id))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"blogs": list})
}

func (a *API) UpdateBlog(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	img, _ := upload.SaveImage(r, "imagePath", filepath.Join(a.Cfg.UploadsDir, "Blog"), "blog")
	sets := []string{"updatedAt = ?"}
	args := []any{time.Now().UTC().Format(time.RFC3339Nano)}
	if _, ok := body["blogTitle"]; ok {
		sets = append(sets, "blogTitle = ?")
		args = append(args, str(body, "blogTitle"))
	}
	if _, ok := body["blog"]; ok {
		sets = append(sets, "blog = ?")
		args = append(args, str(body, "blog"))
	}
	if img != "" {
		sets = append(sets, "imagePath = ?")
		args = append(args, img)
	}
	args = append(args, id)
	_, _ = a.DB.Exec(`UPDATE "Blog" SET `+joinComma(sets)+` WHERE id = ?`, args...)
	response.OK(w, map[string]any{"success": true, "blog": a.serializeBlog(id)})
}

func (a *API) DeleteBlog(w http.ResponseWriter, r *http.Request) {
	_, _ = a.DB.Exec(`UPDATE "Blog" SET deleted = 1, deletedAt = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

// --- Blog Category ---

func (a *API) GetBlogCategories(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.DB.Query(`SELECT id, name, active, createdAt FROM "BlogCategory" WHERE deletedAt IS NULL`)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			var name sql.NullString
			var active int
			var created sql.NullString
			if rows.Scan(&id, &name, &active, &created) == nil {
				list = append(list, withID(id, map[string]any{
					"name": nullStr(nullStringPtr(name)), "active": active != 0, "createdAt": scanTime(created),
				}))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"categories": list})
}

func (a *API) CreateBlogCategory(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	_, err := a.DB.Exec(`INSERT INTO "BlogCategory" (id, name, active, createdAt) VALUES (?, ?, 1, ?)`,
		id, nullStr(strPtr(body, "name")), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "created"})
}

func (a *API) GetBlogCategoryByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var name sql.NullString
	var active int
	var created sql.NullString
	if err := a.DB.QueryRow(`SELECT name, active, createdAt FROM "BlogCategory" WHERE id = ?`, id).Scan(&name, &active, &created); err != nil {
		response.Fail(w, "not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "category": withID(id, map[string]any{
		"name": nullStr(nullStringPtr(name)), "active": active != 0, "createdAt": scanTime(created),
	})})
}

func (a *API) UpdateBlogCategory(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	_, _ = a.DB.Exec(`UPDATE "BlogCategory" SET name = COALESCE(?, name), active = COALESCE(?, active), updatedAt = ? WHERE id = ?`,
		nullStr(strPtr(body, "name")), boolVal(body, "active", true), time.Now().UTC().Format(time.RFC3339Nano), chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "updated"})
}

func (a *API) DeleteBlogCategory(w http.ResponseWriter, r *http.Request) {
	_, _ = a.DB.Exec(`UPDATE "BlogCategory" SET deletedAt = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

// --- Content ---

func (a *API) GetContents(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.DB.Query(`SELECT id, contentType, content, deleted, active, createdAt FROM "Content" WHERE deleted = 0`)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, ctype, content string
			var del, active int
			var created sql.NullString
			if rows.Scan(&id, &ctype, &content, &del, &active, &created) == nil {
				list = append(list, withID(id, map[string]any{
					"contentType": ctype, "content": content, "deleted": del != 0, "active": active != 0,
					"createdAt": scanTime(created),
				}))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"contents": list})
}

func (a *API) AddContent(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	_, err := a.DB.Exec(`INSERT INTO "Content" (id, contentType, content, deleted, active, createdAt) VALUES (?,?,?,0,1,?)`,
		id, str(body, "contentType"), str(body, "content"), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "created"})
}

func (a *API) GetContentByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var ctype, content string
	var del, active int
	var created sql.NullString
	if err := a.DB.QueryRow(`SELECT contentType, content, deleted, active, createdAt FROM "Content" WHERE id = ?`, id).
		Scan(&ctype, &content, &del, &active, &created); err != nil {
		response.Fail(w, "not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "content": withID(id, map[string]any{
		"contentType": ctype, "content": content, "deleted": del != 0, "active": active != 0, "createdAt": scanTime(created),
	})})
}

func (a *API) UpdateContent(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	_, _ = a.DB.Exec(`UPDATE "Content" SET contentType = COALESCE(?, contentType), content = COALESCE(?, content), updatedAt = ? WHERE id = ?`,
		nullStr(strPtr(body, "contentType")), nullStr(strPtr(body, "content")),
		time.Now().UTC().Format(time.RFC3339Nano), chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "updated"})
}

func (a *API) DeleteContent(w http.ResponseWriter, r *http.Request) {
	_, _ = a.DB.Exec(`UPDATE "Content" SET deleted = 1, deletedAt = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

// --- Gallery ---

func (a *API) GetGalleryCategories(w http.ResponseWriter, r *http.Request) {
	a.listGalleryCats(w, `deleted = 0 AND active = 1`)
}

func (a *API) GetAllGalleryCategories(w http.ResponseWriter, r *http.Request) {
	a.listGalleryCats(w, "1=1")
}

func (a *API) listGalleryCats(w http.ResponseWriter, where string) {
	rows, _ := a.DB.Query(`SELECT id, gallaryCategoryTitle, description, imagePath, deleted, active, createdAt FROM "GalleryCategory" WHERE ` + where)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, title, desc, img string
			var del, active int
			var created sql.NullString
			if rows.Scan(&id, &title, &desc, &img, &del, &active, &created) == nil {
				list = append(list, withID(id, map[string]any{
					"gallaryCategoryTitle": title, "description": desc, "imagePath": img,
					"deleted": del != 0, "active": active != 0, "createdAt": scanTime(created),
				}))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"categories": list})
}

func (a *API) AddGalleryCategory(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	img, _ := upload.SaveImage(r, "imagePath", filepath.Join(a.Cfg.UploadsDir, "Gallery"), "gallery")
	id := models.NewID()
	_, err := a.DB.Exec(`INSERT INTO "GalleryCategory" (id, gallaryCategoryTitle, description, imagePath, deleted, active, createdAt)
		VALUES (?,?,?,?,0,1,?)`, id, str(body, "gallaryCategoryTitle"), str(body, "description"), img, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "created"})
}

func (a *API) GetGalleryCategoryByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var title, desc, img string
	var del, active int
	var created sql.NullString
	if err := a.DB.QueryRow(`SELECT gallaryCategoryTitle, description, imagePath, deleted, active, createdAt FROM "GalleryCategory" WHERE id = ?`, id).
		Scan(&title, &desc, &img, &del, &active, &created); err != nil {
		response.Fail(w, "not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "category": withID(id, map[string]any{
		"gallaryCategoryTitle": title, "description": desc, "imagePath": img,
		"deleted": del != 0, "active": active != 0, "createdAt": scanTime(created),
	})})
}

func (a *API) UpdateGalleryCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	img, _ := upload.SaveImage(r, "imagePath", filepath.Join(a.Cfg.UploadsDir, "Gallery"), "gallery")
	sets := []string{"updatedAt = ?"}
	args := []any{time.Now().UTC().Format(time.RFC3339Nano)}
	if _, ok := body["gallaryCategoryTitle"]; ok {
		sets = append(sets, "gallaryCategoryTitle = ?")
		args = append(args, str(body, "gallaryCategoryTitle"))
	}
	if img != "" {
		sets = append(sets, "imagePath = ?")
		args = append(args, img)
	}
	args = append(args, id)
	_, _ = a.DB.Exec(`UPDATE "GalleryCategory" SET `+joinComma(sets)+` WHERE id = ?`, args...)
	response.OK(w, map[string]any{"success": true, "msg": "updated"})
}

func (a *API) DeleteGalleryCategory(w http.ResponseWriter, r *http.Request) {
	_, _ = a.DB.Exec(`UPDATE "GalleryCategory" SET deleted = 1, deletedAt = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

func (a *API) GetGalleryItems(w http.ResponseWriter, r *http.Request) {
	a.listGalleryItems(w, `deleted = 0 AND active = 1`)
}

func (a *API) GetAllGalleryItems(w http.ResponseWriter, r *http.Request) {
	a.listGalleryItems(w, "1=1")
}

func (a *API) listGalleryItems(w http.ResponseWriter, where string) {
	rows, _ := a.DB.Query(`SELECT id, imageTitle, description, imagePath, galleryCategoryId, deleted, active, createdAt FROM "GalleryDetails" WHERE ` + where)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, title, desc, img string
			var catID sql.NullString
			var del, active int
			var created sql.NullString
			if rows.Scan(&id, &title, &desc, &img, &catID, &del, &active, &created) == nil {
				list = append(list, withID(id, map[string]any{
					"imageTitle": title, "description": desc, "imagePath": img,
					"galleryCategory": nullStr(nullStringPtr(catID)), "galleryCategoryId": nullStr(nullStringPtr(catID)),
					"deleted": del != 0, "active": active != 0, "createdAt": scanTime(created),
				}))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"items": list})
}

func (a *API) AddGalleryItem(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	img, _ := upload.SaveImage(r, "imagePath", filepath.Join(a.Cfg.UploadsDir, "Gallery"), "galleryitem")
	id := models.NewID()
	_, err := a.DB.Exec(`INSERT INTO "GalleryDetails" (id, imageTitle, description, imagePath, galleryCategoryId, deleted, active, createdAt)
		VALUES (?,?,?,?,?,0,1,?)`,
		id, str(body, "imageTitle"), str(body, "description"), img, nullStr(strPtr(body, "galleryCategoryId")),
		time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "created"})
}

func (a *API) GetGalleryItemByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var title, desc, img string
	var catID sql.NullString
	var del, active int
	var created sql.NullString
	if err := a.DB.QueryRow(`SELECT imageTitle, description, imagePath, galleryCategoryId, deleted, active, createdAt FROM "GalleryDetails" WHERE id = ?`, id).
		Scan(&title, &desc, &img, &catID, &del, &active, &created); err != nil {
		response.Fail(w, "not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "item": withID(id, map[string]any{
		"imageTitle": title, "description": desc, "imagePath": img,
		"galleryCategory": nullStr(nullStringPtr(catID)), "deleted": del != 0, "active": active != 0,
		"createdAt": scanTime(created),
	})})
}

func (a *API) UpdateGalleryItem(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	img, _ := upload.SaveImage(r, "imagePath", filepath.Join(a.Cfg.UploadsDir, "Gallery"), "galleryitem")
	sets := []string{"updatedAt = ?"}
	args := []any{time.Now().UTC().Format(time.RFC3339Nano)}
	if _, ok := body["imageTitle"]; ok {
		sets = append(sets, "imageTitle = ?")
		args = append(args, str(body, "imageTitle"))
	}
	if img != "" {
		sets = append(sets, "imagePath = ?")
		args = append(args, img)
	}
	args = append(args, id)
	_, _ = a.DB.Exec(`UPDATE "GalleryDetails" SET `+joinComma(sets)+` WHERE id = ?`, args...)
	response.OK(w, map[string]any{"success": true, "msg": "updated"})
}

func (a *API) DeleteGalleryItem(w http.ResponseWriter, r *http.Request) {
	_, _ = a.DB.Exec(`UPDATE "GalleryDetails" SET deleted = 1, deletedAt = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

// --- Contact / Subscribe ---

func (a *API) AddContact(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	_, err := a.DB.Exec(`INSERT INTO "Contact" (id, name, email, phone, subject, message, createdAt) VALUES (?,?,?,?,?,?,?)`,
		id, nullStr(strPtr(body, "name")), str(body, "email"), nullStr(strPtr(body, "phone")),
		nullStr(strPtr(body, "subject")), nullStr(strPtr(body, "message")), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "Contact submitted"})
}

func (a *API) GetContacts(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.DB.Query(`SELECT id, name, email, phone, subject, message, createdAt FROM "Contact" ORDER BY createdAt DESC`)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, email string
			var name, phone, subject, message sql.NullString
			var created sql.NullString
			if rows.Scan(&id, &name, &email, &phone, &subject, &message, &created) == nil {
				list = append(list, withID(id, map[string]any{
					"name": nullStr(nullStringPtr(name)), "email": email,
					"phone": nullStr(nullStringPtr(phone)), "subject": nullStr(nullStringPtr(subject)),
					"message": nullStr(nullStringPtr(message)), "createdAt": scanTime(created),
				}))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"contacts": list})
}

func (a *API) GetContactByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var email string
	var name, phone, subject, message sql.NullString
	var created sql.NullString
	if err := a.DB.QueryRow(`SELECT name, email, phone, subject, message, createdAt FROM "Contact" WHERE id = ?`, id).
		Scan(&name, &email, &phone, &subject, &message, &created); err != nil {
		response.Fail(w, "not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "contact": withID(id, map[string]any{
		"name": nullStr(nullStringPtr(name)), "email": email, "phone": nullStr(nullStringPtr(phone)),
		"subject": nullStr(nullStringPtr(subject)), "message": nullStr(nullStringPtr(message)),
		"createdAt": scanTime(created),
	})})
}

func (a *API) DeleteContact(w http.ResponseWriter, r *http.Request) {
	_, _ = a.DB.Exec(`DELETE FROM "Contact" WHERE id = ?`, chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

func (a *API) AddSubscribe(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	_, err := a.DB.Exec(`INSERT INTO "Subscribe" (id, email, createdAt) VALUES (?, ?, ?)`,
		id, str(body, "email"), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "Subscribed"})
}

func (a *API) GetSubscribes(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.DB.Query(`SELECT id, email, createdAt FROM "Subscribe" ORDER BY createdAt DESC`)
	var list []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, email string
			var created sql.NullString
			if rows.Scan(&id, &email, &created) == nil {
				list = append(list, withID(id, map[string]any{"email": email, "createdAt": scanTime(created)}))
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"subscribers": list})
}

func (a *API) DeleteSubscribe(w http.ResponseWriter, r *http.Request) {
	_, _ = a.DB.Exec(`DELETE FROM "Subscribe" WHERE id = ?`, chi.URLParam(r, "id"))
	response.OK(w, map[string]any{"success": true, "msg": "deleted"})
}

// Payment gateway placeholder (keeps route compatibility)
func (a *API) PaymentGateway(w http.ResponseWriter, r *http.Request) {
	response.OK(w, map[string]any{"success": true, "msg": "Payment gateway endpoint – configure provider keys in production"})
}
