# Pusti Shangar (Go + Templ + HTMX + Tailwind)

Stage 1 of the server-rendered migration. The existing Chi JSON API remains the
source of truth; the new presentation layer is additive.

## Stack

- **HTTP**: `net/http` + [chi](https://github.com/go-chi/chi)
- **DB**: `database/sql` + [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)
- **Auth**: JWT (`golang-jwt/jwt/v5`) + bcrypt
- **HTML**: [Templ](https://templ.guide/) (`v0.2.793`)
- **Interactions**: [HTMX](https://htmx.org/) (`2.0.11`, served locally)
- **CSS**: [Tailwind CSS](https://tailwindcss.com/) (`v4.3.5`, standalone CLI — no Node.js/npm)
- **Config**: env vars + optional `.env` (`joho/godotenv`)

Templ `v0.2.793` is the newest release compatible with this repository's Go
`1.22.2` directive. Newer Templ releases require Go 1.23+. Tailwind is built
with its standalone binary, so no JavaScript toolchain is required.

## Layout

```
cmd/server/main.go              # process entry, graceful shutdown
cmd/seed/main.go                # idempotent seed (admin/manager/user + sample customers)
internal/
  config/                       # env loading
  database/                     # SQLite open + migration runner
  models/                       # domain types + ID generator
  auth/                         # JWT + password hashing
  middleware/                   # CORS, recover, logger, protect
  upload/                       # multipart image saves
  response/                     # JSON helpers + `_id`/`id` dual fields
  handlers/                     # existing JSON API handlers + route registration
  docs/                         # OpenAPI 3 spec + embedded Swagger UI
  web/
    routes.go                   # RegisterWebRoutes: HTML + embedded static assets
    render/render.go            # shared page/fragment/redirect/error rendering
    handlers/home.go            # homepage, health and HTMX demo handlers
    views/
      layouts/base.templ        # document shell
      components/               # navigation + footer
      pages/                    # homepage, HTMX fragment, error page
migrations/
  001_init.sql                  # schema
  embed.go                      # embeds the schema for single-binary deploys
styles/input.css                # Tailwind source
static/
  css/app.css                   # generated, minified, embedded
  js/htmx.min.js                # local HTMX build, embedded
  images/logo.svg               # embedded image
  embed.go                      # go:embed for frontend assets
uploads/                        # persistent user uploads (not embedded)
```

## Quick start

```bash
cp .env.example .env
make build
./bin/seed
./bin/server
# App:    http://localhost:5000
# Health: http://localhost:5000/health
# API:    http://localhost:5000/api/...
```

`make build` installs the pinned Templ/Tailwind tools into `.tools/`, generates
Templ Go files, builds minified CSS, and compiles `bin/server` and `bin/seed`.

For a fast edit loop:

```bash
make dev
```

This runs Templ watch, Tailwind watch, and [air](https://github.com/air-verse/air)
for Go live reload. All watchers are stopped together on Ctrl-C. Air and the
CLI tools are development-only and are never linked into the production binary.

### Make targets

| Command | Responsibility |
|---|---|
| `make generate` | Generate Go code from `.templ` files |
| `make css` | Build minified production CSS |
| `make css-watch` | Watch Tailwind sources and rebuild CSS |
| `make templ-watch` | Watch `.templ` files and regenerate Go |
| `make build` | Generate Templ, build CSS, compile `bin/server` and `bin/seed` |
| `make run` | Build and start `bin/server` |
| `make dev` | Run Templ/Tailwind watchers + air |
| `make clean` | Remove `bin/` and `tmp/` build output |
| `make test` | Run `go test ./...` |
| `make fmt` / `make vet` | Format / vet the Go code |

## Server-rendered web layer

`internal/handlers.API.Routes()` still owns all existing API routes. It calls
`web.RegisterWebRoutes` once, which adds:

```
/                          Full HTML homepage
/health                    JSON liveness response
/fragments/server-message  HTMX HTML fragment (GET)
/static/*                  Embedded CSS, JS and images
```

`internal/web/render` provides the reusable rendering contract:

- `Renderer.Page` — full document with `text/html; charset=utf-8`
- `Renderer.Fragment` — fragment with `Cache-Control: no-store`
- `Renderer.ValidationError` — 4xx fragment for form errors
- `Renderer.Error` — user-safe error page; details stay in server logs
- `Renderer.Redirect` — `303 See Other`, or `HX-Redirect` for HTMX requests
- `IsHTMX` — checks `HX-Request: true` for transport behaviour only

`HX-Request` is **not** an authentication signal. Protected handlers must still
run the existing auth middleware.

The homepage's “Load server message” button demonstrates the full round trip:
HTMX sends a GET with `HX-Request: true`, Go returns a Templ fragment, and HTMX
swaps it into `#server-message`. A normal browser visit to the fragment URL is
redirected back to the homepage section instead of rendering a bare fragment.

## Tailwind CSS

`styles/input.css` imports Tailwind and scans `internal/web` for complete class
names in `.templ` and Go files. Production CSS is minified:

```bash
make css
```

The result is `static/css/app.css`. It is compiled and committed so `go build`
works immediately; Docker regenerates it from source during the image build.
Do not build class names dynamically (for example `"bg-" + color`); use complete
class strings or Tailwind's explicit source safelisting.

## Static assets and embedding

`static/embed.go` embeds `css/`, `js/` and `images/` into the binary. The Chi
router serves them under `/static/*`. The compiled server therefore does not
need the source tree or a Node/Tailwind installation at runtime.

User uploads are deliberately **not** embedded. They stay in `UPLOADS_DIR`
(`./uploads` in development, `/uploads` in Docker) and should be backed by
persistent storage.

The SQL schema is also embedded (`migrations/embed.go`), so the binary can
create or migrate its database without a `migrations/` directory at runtime.

## API documentation (Swagger)

Start the server, then open:

- Swagger UI: <http://localhost:5000/swagger>
- OpenAPI 3 JSON: <http://localhost:5000/swagger/openapi.json>

The UI assets are embedded in the binary (`github.com/swaggo/files/v2`), so the
docs work without internet access or a CDN. The spec lives in
`internal/docs/spec.go` (routes/tags) and `internal/docs/schemas.go`
(request/response models). Add a route to the `operations` table and the matching
schema to keep the docs in sync.

Authentication can be supplied with Swagger's **Authorize** button (Bearer JWT)
or by passing a `token` field in the request body, matching the server's
existing token handling.

## Seeded logins

| Role     | Email                     | Password      |
|----------|---------------------------|---------------|
| Admin    | admin@pushtishangar.com   | Admin@123     |
| Manager  | manager@pushtishangar.com | Manager@123   |
| Support  | user@pushtishangar.com    | User@123      |
| Customer | aarohi@example.com        | Customer@123  |

## API compatibility

Route prefixes and POST-based endpoints match the original Express app so the
React `client` and `adminpanel` can point at this server with the same
`REACT_APP_BASE_URL`.

Responses include both `_id` and `id` on records. Auth accepts Bearer header,
`token` cookie, or body field where the original did.

### Main route groups

| Prefix | Purpose |
|--------|---------|
| `/api/*` | Users, roles, payment stubs |
| `/category`, `/subcategory`, `/subsubcategory` | Catalog hierarchy |
| `/product` | Products + variants |
| `/customer` | Customers, cart, wishlist, auth |
| `/orders` | Orders + reports |
| `/stocks`, `/gst`, `/dailyrates` | Inventory / tax / rates |
| `/coupons`, `/material`, `/season`, `/color`, `/size` | Masters |
| `/banner`, `/blog`, `/blogcategory`, `/content` | CMS |
| `/gallery`, `/gallerydetails` | Gallery |
| `/contact`, `/subscribe` | Lead forms |
| `/dashboard/*` | Aggregates |
| `/uploads/*` | Static files |

## Docker

Build and run with Compose:

```bash
cp .env.example .env
# Set a strong JWT_SECRET in .env before deploying.
docker compose up --build -d
```

The image is multi-stage:

1. Install pinned Templ and Tailwind standalone binaries.
2. Download Go modules.
3. Run `templ generate`, build minified Tailwind CSS, and compile static Go binaries.
4. Copy only the binaries and empty writable data/upload directories into
   `gcr.io/distroless/static-debian12:nonroot`.

Compose persists SQLite and uploads in named volumes (`app_data`, `app_uploads`).
The runtime defaults are `DATABASE_URL=/data/app.db`, `UPLOADS_DIR=/uploads` and
`TMPDIR=/data/tmp`; override them with environment variables as needed.

The seed binary is included in the image:

```bash
docker compose run --rm --entrypoint /app/seed app
```

## Verification

```bash
make generate
make css
go fmt ./...
go vet ./...
go test ./...
go build ./cmd/server
```

Then, with the server running:

```bash
curl -i http://localhost:5000/                       # 200 HTML
curl -i http://localhost:5000/static/css/app.css     # 200 CSS
curl -i http://localhost:5000/static/js/htmx.min.js  # 200 JS
curl -i http://localhost:5000/health                 # 200 JSON
curl -i -H 'HX-Request: true' \
  http://localhost:5000/fragments/server-message     # 200 HTML fragment
curl -i -X POST http://localhost:5000/api/payment    # 200 JSON
```

## Notes

- Banner/color static mounts were moved to `/banner-images` and `/color-images`
  because chi cannot mount a file server on the same path as an API route group.
  Files are still available under `/uploads/Banner` and `/uploads/Color`.
- Payment gateway handlers are stubs; wire your provider keys in production.
- SQLite is fine for local/dev; for production traffic prefer PostgreSQL (swap
  driver + DSN in `database.Open`).
- The `.env` file is intentionally untracked. Keep real secrets out of version
  control and set `JWT_SECRET` explicitly in production.
