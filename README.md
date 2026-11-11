# Pusti Shangar API (Go)

Idiomatic Go rewrite of the Express + Prisma + SQLite server.

## Stack

- **HTTP**: `net/http` + [chi](https://github.com/go-chi/chi)
- **DB**: `database/sql` + [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)
- **Auth**: JWT (`golang-jwt/jwt/v5`) + bcrypt
- **Config**: env vars + optional `.env` (`joho/godotenv`)

## Layout

```
cmd/server/main.go     # process entry, graceful shutdown
cmd/seed/main.go       # idempotent seed (admin/manager/user + sample customers)
internal/
  config/              # env loading
  database/            # SQLite open + migrations
  models/              # domain types + ID generator
  auth/                # JWT + password hashing
  middleware/          # CORS, recover, logger, protect
  upload/              # multipart image saves
  response/            # JSON helpers + `_id`/`id` dual fields
  handlers/            # all HTTP handlers (API-compatible routes)
  docs/                # OpenAPI 3 spec + embedded Swagger UI
migrations/001_init.sql
uploads/               # static image storage
```

## Quick start

```bash
cd server-go
cp .env.example .env
go mod tidy
go run ./cmd/seed
go run ./cmd/server
# API: http://localhost:5000
```

Or build binaries:

```bash
go build -o bin/server ./cmd/server
go build -o bin/seed ./cmd/seed
./bin/seed && ./bin/server
```

## API documentation (Swagger)

Start the server, then open:

- Swagger UI: <http://localhost:5000/swagger>
- OpenAPI 3 JSON: <http://localhost:5000/swagger/openapi.json>

The UI assets are embedded in the binary (`github.com/swaggo/files/v2`), so the docs work without
internet access or a CDN. The spec lives in `internal/docs/spec.go` (routes/tags) and
`internal/docs/schemas.go` (request/response models). Add a route to the `operations` table and the
matching schema to keep the docs in sync.

Authentication can be supplied with Swagger's **Authorize** button (Bearer JWT) or by passing a
`token` field in the request body, matching the server's existing token handling.

## Seeded logins

| Role     | Email                     | Password      |
|----------|---------------------------|---------------|
| Admin    | admin@pushtishangar.com   | Admin@123     |
| Manager  | manager@pushtishangar.com | Manager@123   |
| Support  | user@pushtishangar.com    | User@123      |
| Customer | aarohi@example.com        | Customer@123  |

## API compatibility

Route prefixes and POST-based endpoints match the original Express app so the React `client` and `adminpanel` can point at this server with the same `REACT_APP_BASE_URL`.

Responses include both `_id` and `id` on records. Auth accepts Bearer header, `token` cookie, or body field where the original did.

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

## Notes

- Banner/color static mounts were moved to `/banner-images` and `/color-images` because chi cannot mount a file server on the same path as an API route group. Files are still available under `/uploads/Banner` and `/uploads/Color`.
- Payment gateway handlers are stubs; wire your provider keys in production.
- SQLite is fine for local/dev; for production traffic prefer PostgreSQL (swap driver + DSN in `database.Open`).
# ecom-gotth
# ecom-gotth
# ecom-gotth
