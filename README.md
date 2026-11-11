# Pushti Shangar server (Go + Chi)

Go backend for the Pushti Shangar storefront and admin panel. The existing Chi
JSON API remains the contract used by the React `client` and `adminpanel`; the
Templ/HTMX presentation layer added in Stage 1 is additive. The hardening pass
described in [`AUDIT.md`](AUDIT.md) keeps every existing route and preserves
existing SQLite data.

## Stack

- **HTTP**: `net/http` + [chi](https://github.com/go-chi/chi)
- **DB**: `database/sql` + [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)
- **Auth**: JWT HS256 (`golang-jwt/jwt/v5`) + bcrypt
- **HTML**: [Templ](https://templ.guide/) (`v0.2.793`)
- **Interactions**: [HTMX](https://htmx.org/) (`2.0.11`, served locally)
- **CSS**: [Tailwind CSS](https://tailwindcss.com/) (`v4.3.5`, standalone CLI — no Node.js/npm)
- **Config**: env vars + optional `.env` (`joho/godotenv`)

Templ `v0.2.793` is the newest release compatible with this repository's Go
`1.22.2` directive. Tailwind is built with its standalone binary, so no
JavaScript toolchain is required.

## Layout

```
cmd/server/main.go              # process entry, validation, graceful shutdown, health probe
cmd/seed/main.go                # idempotent seed (roles, staff, dev customers)
internal/
  config/                       # env loading + production validation
  database/                     # SQLite pool, pragmas, migration runner
  models/                       # domain types + ID generator
  auth/                         # JWT claims + password hashing
  middleware/                   # request ID, logging, recovery, CORS, auth, roles, limits
  upload/                       # signature-validated image saves
  response/                     # JSON helpers + `_id`/`id` dual fields
  service/                      # transactional checkout, cancellation, payments
  handlers/                     # JSON API handlers + route registration
  docs/                         # OpenAPI 3 spec + embedded Swagger UI
  web/                          # Stage 1 HTML/HTMX layer
migrations/
  001_init.sql                  # original schema
  002_hardening.sql             # additive hardening (idempotency, soft delete, snapshots)
  embed.go                      # embeds migrations for single-binary deploys
styles/input.css                # Tailwind source
static/                         # generated CSS, local HTMX, embedded images
uploads/                        # persistent user uploads (not embedded)
```

## Quick start

```bash
cp .env.example .env
make build
./bin/seed
./bin/server
# App:     http://localhost:5000
# Liveness: http://localhost:5000/healthz
# Readiness:http://localhost:5000/readyz
# API docs: http://localhost:5000/swagger
```

`make build` installs the pinned Templ/Tailwind tools into `.tools/`, generates
Templ Go files, builds minified CSS, and compiles `bin/server` and `bin/seed`.
Migrations run automatically at startup; production refuses to start if they
fail, development logs a warning.

For a fast edit loop:

```bash
make dev
```

This runs Templ watch, Tailwind watch, and [air](https://github.com/air-verse/air)
for Go live reload. Air and the CLI tools are development-only and are never
linked into the production binary.

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
| `make test-race` | Run the full suite with the race detector |
| `make fmt` / `make vet` | Format / vet the Go code |

## Configuration

Configuration is loaded from the environment, with `.env` as a development
convenience. See [`.env.example`](.env.example) for every supported variable.

### Production requirements

`config.Validate()` fails closed when the process runs with `ENV=production` and:

- `JWT_SECRET` is missing, shorter than 32 characters, or the development default;
- `CORS_ORIGINS` is empty, contains `*`, or contains a localhost origin;
- `COOKIE_SECURE` is false;
- `PAYMENT_PROVIDER` and `PAYMENT_SECRET` are set without each other.

Generate a secret with `openssl rand -hex 32`. Never commit `.env`; it is
ignored by Git and only `.env.example` (placeholders) is tracked.

### Auth status mode

`AUTH_STATUS` controls how authentication failures are reported:

| Value | Behaviour | Use |
|---|---|---|
| `legacy` | HTTP 200 + `{"success":false,"msg":...}` for missing/invalid auth | Default in development, preserves old clients |
| `strict` | HTTP 401 (not authenticated) / 403 (forbidden) | Default in production |

Both modes enforce the same authorization checks. Set `AUTH_STATUS=strict` in
staging before switching clients to status-code handling.

### Payment verification

`POST /api/payment` and `POST /api/create-order` verify a provider callback. The
provider signs the string `orderId|providerReference|amount` with HMAC-SHA256
using `PAYMENT_SECRET`; the amount is always read from the order row, never from
the client. When `PAYMENT_PROVIDER`/`PAYMENT_SECRET` are unset the endpoint fails
closed with HTTP 503. Client-supplied payment status is never trusted.

## Security model

- **Token scope**: JWT claims include `kind` (`user`/`customer`) and the user's
  primary `role`. Legacy tokens without `kind` are resolved against the `User`
  and `Customer` tables, so existing sessions keep working.
- **Route scopes**: routes are grouped into public, customer, staff
  (`user`/`manager`/`admin`), manager, and admin-only middleware scopes. A route
  cannot silently become public by omission.
- **Resource ownership**: customer, cart, wishlist, order, and payment handlers
  check that the authenticated customer owns the record. Staff roles may read
  broader data where the original API allowed it.
- **Privilege changes**: public registration always creates an active `user`.
  Only an authenticated administrator can assign roles or change `active`.
  Managers can create ordinary users but cannot mint or promote administrators.
  Users cannot change their own roles, and cannot change another user's password.
- **Server-controlled money**: order line prices, discounts, shipping, totals,
  status, and payment status are computed server-side. Client values are ignored.
- **Inventory**: checkout, cancellation, and order deletion run in transactions.
  Stock is decremented only after validation, and `_txlock=immediate` serialises
  writers so concurrent checkouts cannot oversell. `Idempotency-Key` (header or
  body) makes retried checkouts return the original order.
- **Uploads**: only JPEG, PNG, GIF, and WebP are accepted, verified by content
  signature with `http.DetectContentType`. SVG/BMP and disguised text files are
  rejected, names are randomised, and files are written via a temp file + rename.
  The per-image limit is 8 MB.
- **Errors**: unexpected failures are logged server-side and returned as a
  generic `500 Internal Server Error`; database errors are not leaked.
- **Transport**: security headers, strict CORS allow-list, request IDs, request
  body limits, login rate limiting, and optional trusted-proxy handling.

### Soft deletes

- Products: `isActive=0` (historical order items keep their snapshot).
- Customers: `deleted=1, active=0` (orders and reports keep the reference).
- Orders: `deletedAt` is set and any un-cancelled stock is restored.

## Database

Migrations are embedded and applied in filename order, recorded in
`schema_migrations`. `001_init.sql` adopts the original schema idempotently and
`002_hardening.sql` only adds columns/tables/indexes — no existing table is
recreated or dropped.

SQLite runs in WAL mode with a 5 second busy timeout and immediate write
transactions. This is reliable for a single server instance and moderate
traffic. For multi-instance or high-write production deployments, move to
PostgreSQL:

1. Replace the `modernc.org/sqlite` driver with `github.com/jackc/pgx/v5/stdlib`.
2. Change `database.Open` to accept a `postgres://` DSN and remove the SQLite
   `_pragma`/`_txlock` query parameters.
3. Translate the two migrations to PostgreSQL syntax (types are close; the
   partial unique index on `Order.idempotencyKey` is already valid).
4. Use `SELECT ... FOR UPDATE` (or `UPDATE ... WHERE stock >= ?`) in
   `service.Place` instead of relying on SQLite's database-wide write lock.

The handler and service layers do not depend on SQLite-specific SQL beyond the
`ON CONFLICT` upsert in invoice generation, which PostgreSQL also supports.

## Seeded logins

Development passwords are shown below. Override them with
`SEED_ADMIN_PASSWORD`, `SEED_MANAGER_PASSWORD`, `SEED_USER_PASSWORD`, and
`SEED_CUSTOMER_PASSWORD`. In production the seed command refuses to run unless
the staff passwords are supplied, and it does not create demo customers.

| Role     | Email                     | Development password |
|----------|---------------------------|----------------------|
| Admin    | admin@pushtishangar.com   | `Admin@12345`        |
| Manager  | manager@pushtishangar.com | `Manager@12345`      |
| Support  | user@pushtishangar.com    | `User@12345`         |
| Customer | aarohi@example.com        | `Customer@12345`     |

## API compatibility

Route prefixes and POST-based endpoints match the original Express app so the
React clients can point at this server with the same `REACT_APP_BASE_URL`.
Responses keep both `_id` and `id` on records.

Auth accepts:

- `Authorization: Bearer <token>` (preferred);
- the `token` cookie set by login;
- a `token` field in a JSON or form body for legacy clients.

Query-string tokens are intentionally no longer accepted. New tokens include
`kind`/`role` claims; old tokens remain valid until they expire.

### Main route groups

| Prefix | Purpose |
|--------|---------|
| `/api/*` | Users, roles, login, registration, password reset, payment |
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
| `/healthz`, `/readyz` | Liveness / readiness probes |

## Server-rendered web layer

`internal/handlers.API.Routes()` owns all existing API routes and calls
`web.RegisterWebRoutes` once, which adds the Stage 1 HTML pages, HTMX fragment,
and embedded static assets. `internal/web/render` provides page, fragment,
validation-error, error, and redirect helpers. `HX-Request` is a transport hint,
**not** an authentication signal; protected handlers still run the auth
middleware.

## API documentation (Swagger)

Start the server, then open:

- Swagger UI: <http://localhost:5000/swagger>
- OpenAPI 3 JSON: <http://localhost:5000/swagger/openapi.json>

The UI assets are embedded, so the docs work offline. The spec lives in
`internal/docs/spec.go` and `internal/docs/schemas.go`. Add a route to the
`operations` table and the matching schema to keep docs in sync.

## Docker

Build and run with Compose:

```bash
cp .env.example .env
# Set ENV=production, a strong JWT_SECRET, CORS_ORIGINS and COOKIE_SECURE=true.
docker compose up --build -d
```

The image is multi-stage: pinned Templ/Tailwind tools and Go modules are used to
generate assets and compile static binaries, then only the binaries and empty
writable data/upload directories are copied into
`gcr.io/distroless/static-debian12:nonroot`.

Compose persists SQLite and uploads in named volumes (`app_data`, `app_uploads`).
The runtime defaults are `DATABASE_URL=/data/app.db`, `UPLOADS_DIR=/uploads` and
`TMPDIR=/data/tmp`. The container healthcheck runs `/app/server healthcheck`,
which probes `/healthz` (the distroless image has no shell or curl).

The seed binary is included in the image:

```bash
docker compose run --rm --entrypoint /app/seed app
```

## Tests

```bash
make test-race
```

The suite uses temporary SQLite databases and real migrations. It covers:

- password hashing, token expiry, algorithm confusion (`none`), wrong secrets,
  and legacy tokens without `kind`;
- upload signature validation, path traversal, and rejected SVG/text payloads;
- transactional checkout, server-side totals, line-item snapshots, coupons,
  idempotency, concurrent oversell prevention, cancellation restock, and soft
  delete restock;
- HMAC payment verification, invalid signatures, unconfigured providers,
  cross-customer access, and payment idempotency;
- HTTP integration: auth status modes, registration privilege injection,
  self/other ownership, role checks, cart stock limits, order ownership and
  totals, admin cancellation, soft delete, health probes, 404s, and error
  message sanitisation.

`go build ./...`, `go vet ./...`, and `go test -race ./...` pass in this
repository. Docker was not built in the audit environment because Docker is not
installed there.

## Notes and limitations

- SQLite is a single-node database. Use PostgreSQL before scaling horizontally.
- Rate limiting is in-process; use a shared store (Redis) for multiple instances.
- JWTs are stateless: there is no server-side revocation list beyond disabling
  the account or changing its password. Keep `TokenTTL` short.
- Password reset requires an email provider; without `EMAIL_*` settings the
  reset email is not delivered (development logs it).
- Payment verification is a generic HMAC callback contract, not a full Stripe or
  Razorpay adapter. Add provider-specific signature parsing behind
  `service.VerifyPayment`.
- Upload validation checks file signatures; it does not re-encode images. Put
  uploads behind a CDN or image proxy if you need server-side re-encoding.
- Banner/color static mounts use `/banner-images` and `/color-images` because
  chi cannot mount a file server on the same path as an API route group. Files
  are also available under `/uploads/Banner` and `/uploads/Color`.
- The `.env` file is intentionally untracked. Keep real secrets out of version
  control and set `JWT_SECRET` explicitly in production.
