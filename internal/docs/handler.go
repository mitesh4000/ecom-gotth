package docs

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"

	swaggerFiles "github.com/swaggo/files/v2"
)

// Handler serves the Swagger UI, its static assets, and the OpenAPI document.
//
// Routes expected by the handler:
//
//	/swagger            -> redirect to /swagger/
//	/swagger/           -> Swagger UI
//	/swagger/openapi.json -> OpenAPI 3 document
//	/swagger/*          -> embedded Swagger UI assets
func Handler() http.Handler {
	assets := http.StripPrefix("/swagger/", http.FileServer(http.FS(swaggerFiles.FS)))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/swagger":
			http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
		case "/swagger/", "/swagger/index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, indexHTML)
		case "/swagger/openapi.json", "/swagger/doc.json":
			serveSpec(w)
		default:
			assets.ServeHTTP(w, r)
		}
	})
}

var (
	specOnce sync.Once
	specJSON []byte
	specErr  error
)

func serveSpec(w http.ResponseWriter) {
	specOnce.Do(func() {
		specJSON, specErr = json.MarshalIndent(Spec(), "", "  ")
	})
	if specErr != nil {
		http.Error(w, "failed to build OpenAPI document", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(specJSON)
}

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Pusti Shangar API - Swagger UI</title>
  <link rel="stylesheet" type="text/css" href="./swagger-ui.css">
  <link rel="icon" type="image/png" href="./favicon-32x32.png" sizes="32x32">
  <link rel="icon" type="image/png" href="./favicon-16x16.png" sizes="16x16">
  <style>
    html { box-sizing: border-box; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin: 0; background: #fafafa; }
  </style>
</head>
<body>
<div id="swagger-ui"></div>
<script src="./swagger-ui-bundle.js"></script>
<script src="./swagger-ui-standalone-preset.js"></script>
<script>
  window.onload = function () {
    window.ui = SwaggerUIBundle({
      url: "./openapi.json",
      dom_id: "#swagger-ui",
      deepLinking: true,
      persistAuthorization: true,
      tryItOutEnabled: true,
      validatorUrl: null,
      presets: [
        SwaggerUIBundle.presets.apis,
        SwaggerUIStandalonePreset
      ],
      plugins: [
        SwaggerUIBundle.plugins.DownloadUrl
      ],
      layout: "StandaloneLayout"
    });
  };
</script>
</body>
</html>
`
