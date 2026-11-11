# Security and production-readiness audit

Date: 2026-11-11
Scope: `github.com/mitesh4000/pushti_sangar/server` (Go + Chi + SQLite backend)
Baseline: Stage 1 Templ/HTMX commit `2d8aad7`

This pass audited the existing backend before adding any further frontend work.
It preserves every existing JSON route and the existing SQLite database. The
changes are uncommitted in the working tree; Stage 1 remains at `2d8aad7`.

## 1. Executive summary

The original backend had critical authorization and integrity gaps: public
registration could inject roles, customer records and orders could be read or
modified by other customers, client-supplied prices/statuses were trusted, and
order placement was not transactional. Authentication failures also returned
HTTP 200, uploads trusted file extensions, and unexpected database errors were
returned to clients.

Those issues are fixed. The API contract is preserved, migrations are additive,
and the full test suite passes under the race detector.

| Area | Result |
|---|---|
| Critical authorization issues | Fixed |
| Order/inventory transaction safety | Fixed, concurrency-tested |
| Client price/status tampering | Fixed, server-authoritative |
| Upload security | Fixed, signature validation |
| Error information leakage | Fixed |
| Existing routes/data | Preserved |
| `go build ./...` | Pass |
| `go vet ./...` | Pass |
| `go test -race ./...` | Pass |
| Docker image | Not executed (Docker unavailable in the audit environment) |

## 2. Critical findings and fixes

### 2.1 Public registration allowed privilege injection — Critical

**Finding:** `POST /api/register` accepted `roles` and `active` from anonymous
callers, allowing anyone to create an admin account or a disabled account.

**Fix:** `RegisterUser` forces `role=user` and `active=true` for anonymous
callers. Only an authenticated administrator may supply roles or `active`.
Managers can create ordinary users but cannot mint administrators, and nobody
can change their own roles or deactivate their own account. Customer
registration likewise forces `active=1, deleted=0`.

### 2.2 Missing resource-level ownership checks — Critical

**Finding:** customer, cart, wishlist, and order handlers authorized the route
but not the record. Any authenticated customer could read or modify another
customer's data by changing an ID.

**Fix:** handlers now compare the authenticated customer ID from the token with
the target customer/order owner. Staff access is limited to the roles the
original API intended. Payment verification rejects another customer's order
without revealing whether it exists.

### 2.3 Client-controlled prices, totals, status, and payment state — Critical

**Finding:** order creation accepted client totals, status, payment status, and
customer IDs. A client could create a delivered/paid order for an arbitrary
price or another customer.

**Fix:** `service.Place` reads product prices from the database, applies coupons
server-side, computes subtotal/shipping/discount/total, forces `pending`/`unpaid`,
and derives the customer from the token when a customer is authenticated.
Client-supplied monetary and status fields are ignored.

### 2.4 Non-transactional checkout and overselling — Critical

**Finding:** order creation inserted rows and decremented stock without a
transaction, so partial orders and overselling were possible under concurrency.

**Fix:** checkout runs in one transaction: validate every line, insert the order,
insert immutable line snapshots, decrement stock, clear the cart, then commit.
`_txlock=immediate` plus `busy_timeout` serialises writers. Cancellation and
order deletion restock in the same transaction and are idempotent.

### 2.5 Weak/default JWT and token confusion — High

**Finding:** production could run with a documented default secret; tokens had no
scope, so a customer token could be used on user routes; query-string tokens were
accepted.

**Fix:** HS256 is enforced, expiry is required, and tokens carry `kind` and
`role` claims. Legacy tokens are still accepted by resolving their ID against
`User`/`Customer`. Query-string tokens are removed; Bearer, cookie, and
body/form tokens remain for compatibility. Production requires a 32+ character
non-default secret.

### 2.6 Authentication failures returned HTTP 200 — High

**Finding:** unauthenticated and forbidden requests returned HTTP 200 with
`success:false`, hiding failures from proxies and clients.

**Fix:** `AUTH_STATUS=strict` returns 401/403 and is the production default.
`AUTH_STATUS=legacy` preserves the original HTTP 200 behaviour for old clients.

### 2.7 Unsafe uploads — High

**Finding:** uploads trusted the client filename/extension and could store SVG or
non-image content, enabling stored XSS or path issues.

**Fix:** only JPEG/PNG/GIF/WebP signatures accepted via `http.DetectContentType`,
8 MB per-image limit, random filenames, temp-file + rename writes, no SVG/BMP.
Client filenames and paths are ignored.

### 2.8 Internal errors leaked to clients — Medium

**Finding:** `response.FailErr` returned raw database/error text.

**Fix:** details are logged with the request ID; clients receive a generic
`500 Internal Server Error`.

### 2.9 Destructive deletes broke history — Medium

**Finding:** deleting products/customers/orders removed or orphaned rows that
historical orders and reports referenced.

**Fix:** product delete is `isActive=0`, customer delete is
`deleted=1, active=0`, and order delete sets `deletedAt` plus restocks. Reads
filter deleted records.

### 2.10 Other issues fixed

- `AddVarProduct` decoded the request body twice and could lose data.
- Product creation accepted empty names and negative prices/stock.
- Product lists were unbounded; now capped at 500 rows.
- Cart quantities were unbounded and ignored stock; now capped and stock-aware.
- Password reset tokens were reusable; now single-use with `usedAt` and expiry.
- Users could attempt to change another user's password; the handler now always
  uses the authenticated user.
- Seed credentials were hard-coded and documented as production defaults; now
  environment-driven, with production refusing demo customers and missing
  passwords.
- Startup did not validate production configuration, log structurally, or shut
  down gracefully; all three are now implemented.
- SQLite ran without WAL/busy-timeout/foreign-key pragmas on every connection.

## 3. Database changes

`migrations/002_hardening.sql` is additive. It adds:

- `Order.paymentStatus`, `Order.idempotencyKey`, `Order.deletedAt`;
- a partial unique index on `idempotencyKey` and an index on `deletedAt`;
- `OrderItem.productName`, `sku`, `unitPrice`, `lineTotal`, `tax`, `stockSource`;
- `Token.usedAt`;
- a `Payment` audit table with idempotency and provider references.

`migrations/embed.go` applies embedded migrations in filename order and records
them in `schema_migrations`. `001_init.sql` is adopted idempotently so existing
databases are not recreated. The migration was applied to a copy of `data/dev.db`
and both versions were recorded successfully.

`internal/database.Open` now applies, per connection:

- `foreign_keys=ON`
- `busy_timeout=5000`
- `journal_mode=WAL`
- `synchronous=NORMAL`
- `temp_store=MEMORY`
- `_txlock=immediate`

The pool allows multiple connections so handlers that serialize a row can query
related rows without deadlocking; write transactions are still serialised by the
immediate lock.

## 4. Authentication and authorization

- Claims: `id`, `kind` (`user`/`customer`), `role`, standard expiry/issued-at.
- Route scopes: public, customer, staff, manager, admin.
- Middleware: `ProtectUser`, `ProtectCustomer`, `ProtectAny`, `RequireRoles`.
- Ownership helpers: `canAccessCustomer`, `canAccessUser`, order-owner lookup.
- Role loading is always from the database, not trusted from the token alone.
- Password changes always target the authenticated account.
- Password resets are single-use and expire after 30 minutes.
- Login, registration, forgot-password, and reset-password are rate-limited by
  client IP (60 requests/minute, in-process).

## 5. Business logic

- Checkout: server-side pricing, coupon validation, shipping, tax snapshot,
  idempotency, stock validation, stock decrement, cart clear, atomic commit.
- Cancellation: restocks only the inventory source used at checkout; repeated
  cancellation is a no-op.
- Deletion: soft delete + restock for un-cancelled orders.
- Payments: HMAC-SHA256 over `orderId|providerReference|amount`, amount read
  from the order, audit rows for success/failure, idempotency, fails closed when
  unconfigured.
- Invoice generation: transactional counter update and order update.

## 6. Tests added

| Package | Coverage |
|---|---|
| `internal/auth` | bcrypt, claim round-trip, expiry, wrong secret, `none` algorithm, legacy tokens |
| `internal/upload` | PNG/JPEG acceptance, path traversal, SVG rejection, disguised text rejection |
| `internal/service` | totals, snapshots, coupons, unavailable products, invalid quantities, insufficient stock rollback, Stock-table source, concurrent oversell prevention, idempotency, cancel/restock, soft delete, payment success/failure/idempotency/cross-customer |
| `internal/handlers` | strict vs legacy auth, login claims, disabled accounts, registration role injection, manager privilege escalation, admin role assignment/self-change denial, weak/mismatched passwords, role escalation, customer/order ownership, cart stock limits, server totals/status, admin cancel/restock, soft delete, health/readiness, 404, sanitised 500s, HTTP-level oversell prevention |

Verification actually run:

```text
go build ./...                 # pass
go vet ./...                   # pass
go test -race -timeout 240s ./...  # pass
```

Observed package results included `internal/auth`, `internal/upload`,
`internal/service`, `internal/handlers`, `internal/docs`, `internal/web`, and
`internal/web/render` all passing with the race detector.

HTTP smoke tests used copied databases on alternate ports:

- `/healthz` 200, `/readyz` 200;
- `/product/getallproducts` 200, `/category/getcategories` 200;
- unauthenticated protected route: 200 + `success:false` in legacy mode,
  401 in strict mode;
- both migrations recorded in `schema_migrations`.

## 7. Files changed

Modified:

```
.env.example
Dockerfile
Makefile
README.md
cmd/seed/main.go
cmd/server/main.go
docker-compose.yml
internal/auth/jwt.go
internal/config/config.go
internal/database/database.go
internal/handlers/content.go
internal/handlers/customer.go
internal/handlers/helpers.go
internal/handlers/order.go
internal/handlers/product.go
internal/handlers/router.go
internal/handlers/user.go
internal/middleware/middleware.go
internal/response/response.go
internal/upload/upload.go
migrations/embed.go
```

Added:

```
internal/auth/jwt_test.go
internal/handlers/health.go
internal/handlers/integration_test.go
internal/service/order.go
internal/service/payment.go
internal/service/helpers_test.go
internal/service/order_test.go
internal/service/payment_test.go
internal/upload/upload_test.go
migrations/002_hardening.sql
```

## 8. Compatibility notes

- All existing JSON paths and POST verbs are preserved.
- Records keep both `_id` and `id`.
- Auth still accepts Bearer, cookie, and body/form `token` values.
- Legacy JWTs without `kind` continue to work until they expire.
- `AUTH_STATUS=legacy` keeps the original HTTP 200 + `success:false` contract.
- Response shapes were extended (for example `paymentStatus`, line snapshots,
  `payment` details); existing fields were not removed.
- No existing table is recreated or dropped by the new migration.

## 9. Remaining limitations and recommendations

1. **SQLite is single-node.** Move to PostgreSQL before horizontal scaling. The
   README documents the driver and transaction changes required.
2. **Stateless JWTs cannot be revoked individually.** Keep TTLs short; account
   disabling and password changes take effect on the next request because roles
   and active state are read from the database.
3. **Rate limiting is in-process.** Use Redis or an edge limiter for multiple
   replicas.
4. **Email delivery is optional.** Password reset links are only delivered when
   `EMAIL_*` is configured.
5. **Payment verification is provider-agnostic HMAC.** Add provider-specific
   adapters for Stripe/Razorpay signature formats and replay windows.
6. **Uploads are validated but not re-encoded.** Consider image re-encoding or a
   CDN/image proxy for defence in depth.
7. **No refresh tokens, MFA, or email verification.** These are product
   decisions, not regressions from the original app.
8. **No CSP/HSTS yet.** Security headers currently cover nosniff, frame denial,
   referrer policy, permissions policy, and cross-origin resource policy. Add
   HSTS once TLS termination is confirmed and a CSP after auditing the React and
   HTMX asset sources.
9. **Docker was not executed.** Docker is unavailable in the audit environment;
   the Dockerfile/compose healthcheck should be validated in CI.
