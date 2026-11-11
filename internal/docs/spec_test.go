package docs_test

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/config"
	"github.com/mitesh4000/pushti_sangar/server/internal/docs"
	"github.com/mitesh4000/pushti_sangar/server/internal/handlers"
)

var apiPrefixes = []string{
	"/api", "/category", "/subcategory", "/subsubcategory", "/product",
	"/customer", "/orders", "/dashboard", "/stocks", "/coupons", "/gst",
	"/dailyrates", "/material", "/season", "/color", "/size", "/banner",
	"/blog", "/blogcategory", "/content", "/gallery", "/gallerydetails",
	"/contact", "/subscribe",
}

func isAPIRoute(route string) bool {
	for _, prefix := range apiPrefixes {
		if route == prefix || strings.HasPrefix(route, prefix+"/") {
			return true
		}
	}
	return false
}

func TestEveryAPIRouteIsDocumented(t *testing.T) {
	spec := docs.Spec()
	documented := map[string]bool{}
	for path, raw := range spec["paths"].(map[string]any) {
		for method := range raw.(map[string]any) {
			documented[strings.ToUpper(method)+" "+path] = true
		}
	}

	api := &handlers.API{Cfg: &config.Config{}}
	router, ok := api.Routes().(chi.Routes)
	if !ok {
		t.Fatal("router does not implement chi.Routes")
	}
	var missing []string
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !isAPIRoute(route) {
			return nil
		}
		if !documented[method+" "+route] {
			missing = append(missing, method+" "+route)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("routes missing from OpenAPI spec:\n%s", strings.Join(missing, "\n"))
	}
}

func TestOperationIDsAreUnique(t *testing.T) {
	spec := docs.Spec()
	seen := map[string]string{}
	for path, raw := range spec["paths"].(map[string]any) {
		for method, rawOp := range raw.(map[string]any) {
			op := rawOp.(map[string]any)
			id, _ := op["operationId"].(string)
			if id == "" {
				t.Errorf("%s %s has no operationId", method, path)
				continue
			}
			if previous, ok := seen[id]; ok {
				t.Errorf("duplicate operationId %q used by %s and %s %s", id, previous, method, path)
			}
			seen[id] = method + " " + path
		}
	}
}

func TestSchemaReferencesResolve(t *testing.T) {
	spec := docs.Spec()
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)

	// Normalize to plain JSON types so the walk below is simple.
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}

	var walk func(value any)
	walk = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if r, ok := v["$ref"].(string); ok {
				const prefix = "#/components/schemas/"
				if !strings.HasPrefix(r, prefix) {
					t.Errorf("unsupported $ref %q", r)
				} else if _, ok := schemas[strings.TrimPrefix(r, prefix)]; !ok {
					t.Errorf("unresolved $ref %q", r)
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(generic)
}
