package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/config"
	"github.com/mitesh4000/pushti_sangar/server/internal/docs"
	"github.com/mitesh4000/pushti_sangar/server/internal/handlers"
)

func main() {
	spec := docs.Spec()
	paths := spec["paths"].(map[string]any)

	api := &handlers.API{Cfg: &config.Config{}}
	router := api.Routes().(chi.Routes)

	type row struct {
		path    string
		methods map[string]bool
	}
	prefixes := []string{
		"/api", "/category", "/subcategory", "/subsubcategory", "/product",
		"/customer", "/orders", "/dashboard", "/stocks", "/coupons", "/gst",
		"/dailyrates", "/material", "/season", "/color", "/size", "/banner",
		"/blog", "/blogcategory", "/content", "/gallery", "/gallerydetails",
		"/contact", "/subscribe",
	}
	isAPI := func(route string) bool {
		for _, p := range prefixes {
			if route == p || strings.HasPrefix(route, p+"/") {
				return true
			}
		}
		return false
	}

	rows := map[string]*row{}
	_ = chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.Contains(route, "*") || !isAPI(route) {
			return nil
		}
		r := rows[route]
		if r == nil {
			r = &row{path: route, methods: map[string]bool{}}
			rows[route] = r
		}
		r.methods[method] = true
		return nil
	})

	var list []*row
	for _, r := range rows {
		list = append(list, r)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].path < list[j].path })

	fmt.Println("| Endpoint | Before | After (canonical) | Operation | Risk |")
	fmt.Println("|---|---|---|---|---|")
	for _, r := range list {
		var after []string
		for _, m := range []string{"GET", "POST", "PATCH", "PUT", "DELETE"} {
			if r.methods[m] {
				after = append(after, m)
			}
		}
		summary := ""
		deprecated := false
		if raw, ok := paths[r.path].(map[string]any); ok {
			if op, ok := raw["post"].(map[string]any); ok {
				summary, _ = op["summary"].(string)
				deprecated, _ = op["deprecated"].(bool)
			}
			if summary == "" {
				for _, m := range []string{"get", "patch", "put", "delete"} {
					if op, ok := raw[m].(map[string]any); ok {
						summary, _ = op["summary"].(string)
						if summary != "" {
							break
						}
					}
				}
			}
		}
		before := "—"
		if r.methods["POST"] {
			before = "POST"
		}
		risk := "None (POST action/create retained)"
		switch {
		case r.methods["DELETE"]:
			risk = "Medium — delete/state change"
		case r.methods["PATCH"] || r.methods["PUT"]:
			risk = "Medium — update/state change"
		case r.methods["GET"]:
			risk = "Low — read-only GET added"
		}
		if deprecated {
			risk += "; legacy POST kept"
		}
		fmt.Printf("| `%s` | %s | %s | %s | %s |\n", r.path, before, strings.Join(after, ", "), summary, risk)
	}
}
