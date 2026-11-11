# REST method audit and migration guide

**Date:** 2026-11-11
**Scope:** every JSON API route registered by `internal/handlers.API.Routes()`.
**Goal:** make HTTP methods match the operation, keep every existing path and
response shape, and keep the original POST registrations working while clients
migrate.

## Summary

- Before this change every API endpoint was registered as `POST` (only
  `/healthz` and `/readyz` were `GET`). Endpoint names did not reliably describe
  the operation, so each handler was inspected before choosing a method.
- Reads are now `GET`, partial updates `PATCH`, complete replacements `PUT`
  (products, categories, customers), and deletions `DELETE`.
- Create, authentication, checkout, and payment-callback endpoints remain
  `POST`. They are actions rather than CRUD operations on an addressable
  resource, and forcing them into CRUD would be misleading.
- **No path was removed.** 108 original POST registrations are retained on the
  same path and are marked `deprecated: true` in the OpenAPI document.
- Every new method calls the existing handler and service code. There is no
  duplicated business logic and no change to request/response payloads.
- Authorization scopes are identical for the legacy and canonical methods.

## Method policy

| Operation | Method | Notes |
|---|---|---|
| Read / list / search | `GET` | Input comes from path parameters and the query string. `GET` never writes. |
| Create a resource | `POST` | Kept as an action; returns the created record. |
| Full replacement | `PUT` | Omitted fields are reset to their zero value. Implemented for products, categories, and customer profiles. |
| Partial update | `PATCH` | Only supplied fields change. Implemented by the existing update handlers. |
| Delete / remove | `DELETE` | Soft deletes remain soft deletes; cart/wishlist removal deletes the association. |
| Login, logout, register, password reset, payment callback, checkout | `POST` | Non-idempotent or action endpoints. Checkout and payment are protected by `Idempotency-Key`. |
| Static file mounts | `GET` (file server) | Not part of the JSON API inventory. |

### PUT vs PATCH

- `PATCH /product/updateproduct/{id}`, `PATCH /category/updatecategory/{id}`,
  and `PATCH /customer/updatecustomer/{id}` update only the fields present in
  the body.
- `PUT` on the same paths treats the body as the complete representation.
  Fields omitted from a product or category body are cleared (`NULL`, `false`,
  or `[]`); a customer `PUT` requires `username` and clears omitted optional
  profile fields. `email` remains immutable.
- Other resources intentionally expose `PATCH` only; they have no documented
  full-replacement contract.

### DELETE input

- Path id is canonical: `DELETE /product/deleteproduct/{id}`.
- The original body-id contract still works on the legacy `POST` form.
- For compatibility, the id may also be supplied as `?id=...` on paths without
  an `{id}` segment (for example `DELETE /api/deleteuser?id=...`).

## Route inventory

The table below is generated from the live chi router and the OpenAPI spec.
"Before" is the method that existed before this change. "After" lists every
method now registered on that path.

| Endpoint | Before | After (canonical) | Operation | Risk |
|---|---|---|---|---|
| `/api/create-order` | POST | POST | Create a payment order placeholder | None (POST action/create retained) |
| `/api/createrole` | POST | POST | Create a role | None (POST action/create retained) |
| `/api/deleterole` | POST | POST, DELETE | Delete a role | Medium — delete/state change; legacy POST kept |
| `/api/deleterole/{id}` | — | DELETE | Delete a role | Medium — delete/state change |
| `/api/deleteuser` | POST | POST, DELETE | Delete a user | Medium — delete/state change; legacy POST kept |
| `/api/deleteuser/{id}` | — | DELETE | Delete a user | Medium — delete/state change |
| `/api/forgotpassword` | POST | POST | Request a user password reset | None (POST action/create retained) |
| `/api/getloggedinuser` | POST | GET, POST | Get the logged-in user | Low — read-only GET added; legacy POST kept |
| `/api/getroles` | POST | GET, POST | List roles | Low — read-only GET added; legacy POST kept |
| `/api/getspecificuser` | POST | GET, POST | Get a user by id | Low — read-only GET added; legacy POST kept |
| `/api/getspecificuser/{id}` | — | GET | Get a user by id | Low — read-only GET added |
| `/api/getusers` | POST | GET, POST | List users | Low — read-only GET added; legacy POST kept |
| `/api/loggedin` | POST | GET, POST | Check user login status | Low — read-only GET added; legacy POST kept |
| `/api/login` | POST | POST | Log in a user | None (POST action/create retained) |
| `/api/logout` | POST | POST | Log out a user | None (POST action/create retained) |
| `/api/payment` | POST | POST | Payment gateway placeholder | None (POST action/create retained) |
| `/api/register` | POST | POST | Register a user | None (POST action/create retained) |
| `/api/resetpassword/{resetToken}` | POST | POST | Reset a user password with a token | None (POST action/create retained) |
| `/api/updatepassword` | POST | POST | Change the user password | None (POST action/create retained) |
| `/api/updateuser` | POST | POST, PATCH | Update a user | Medium — update/state change; legacy POST kept |
| `/api/updateuser/{id}` | — | PATCH | Update a user | Medium — update/state change |
| `/banner/add-new-banner` | POST | POST | Create a banner | None (POST action/create retained) |
| `/banner/all` | POST | GET, POST | List all banners including deleted ones | Low — read-only GET added; legacy POST kept |
| `/banner/delete-banner/{id}` | POST | POST, DELETE | Delete a banner | Medium — delete/state change; legacy POST kept |
| `/banner/get-banner` | POST | GET, POST | List active banners | Low — read-only GET added; legacy POST kept |
| `/banner/get-banner-by-id/{id}` | POST | GET, POST | Get a banner | Low — read-only GET added; legacy POST kept |
| `/banner/update-banner/{id}` | POST | POST, PATCH | Update a banner | Medium — update/state change; legacy POST kept |
| `/blog/add-blog` | POST | POST | Create a blog post | None (POST action/create retained) |
| `/blog/delete-blog/{id}` | POST | POST, DELETE | Delete a blog post | Medium — delete/state change; legacy POST kept |
| `/blog/get-blog` | POST | GET, POST | List active blogs | Low — read-only GET added; legacy POST kept |
| `/blog/get-blog/{id}` | POST | GET, POST | Get a blog post | Low — read-only GET added; legacy POST kept |
| `/blog/getblogbycategory/{id}` | POST | GET, POST | List blogs by category | Low — read-only GET added; legacy POST kept |
| `/blog/update-blog/{id}` | POST | POST, PATCH | Update a blog post | Medium — update/state change; legacy POST kept |
| `/blogcategory/addblogcategory` | POST | POST | Create a blog category | None (POST action/create retained) |
| `/blogcategory/deletecategory/{id}` | POST | POST, DELETE | Delete a blog category | Medium — delete/state change; legacy POST kept |
| `/blogcategory/getblogcategory` | POST | GET, POST | List blog categories | Low — read-only GET added; legacy POST kept |
| `/blogcategory/getspecificcategory/{id}` | POST | GET, POST | Get a blog category | Low — read-only GET added; legacy POST kept |
| `/blogcategory/updateblogcategory/{id}` | POST | POST, PATCH | Update a blog category | Medium — update/state change; legacy POST kept |
| `/category/addcategory` | POST | POST | Create a category | None (POST action/create retained) |
| `/category/deletecategory/{id}` | POST | POST, DELETE | Delete a category | Medium — delete/state change; legacy POST kept |
| `/category/getcategories` | POST | GET, POST | List categories | Low — read-only GET added; legacy POST kept |
| `/category/getspecificcategory/{id}` | POST | GET, POST | Get a category | Low — read-only GET added; legacy POST kept |
| `/category/getsubandsubsub/{id}` | POST | GET, POST | Get subcategories and sub-subcategories for a category | Low — read-only GET added; legacy POST kept |
| `/category/updatecategory/{id}` | POST | POST, PATCH, PUT | Update a category | Medium — update/state change; legacy POST kept |
| `/color/addcolor` | POST | POST | Create a color | None (POST action/create retained) |
| `/color/deletecolor/{id}` | POST | POST, DELETE | Delete a color | Medium — delete/state change; legacy POST kept |
| `/color/getcolors` | POST | GET, POST | List colors | Low — read-only GET added; legacy POST kept |
| `/color/getspecificcolor/{id}` | POST | GET, POST | Get a color | Low — read-only GET added; legacy POST kept |
| `/color/updatecolor/{id}` | POST | POST, PATCH | Update a color | Medium — update/state change; legacy POST kept |
| `/contact/addcontact` | POST | POST | Submit a contact message | None (POST action/create retained) |
| `/contact/deletecontact/{id}` | POST | POST, DELETE | Delete a contact message | Medium — delete/state change; legacy POST kept |
| `/contact/getcontact/{id}` | POST | GET, POST | Get a contact message | Low — read-only GET added; legacy POST kept |
| `/contact/getcontacts` | POST | GET, POST | List contact messages | Low — read-only GET added; legacy POST kept |
| `/content/createcontent` | POST | POST | Create a content block | None (POST action/create retained) |
| `/content/deletecontent/{id}` | POST | POST, DELETE | Delete a content block | Medium — delete/state change; legacy POST kept |
| `/content/getcontent` | POST | GET, POST | List content blocks | Low — read-only GET added; legacy POST kept |
| `/content/getspecificcontent/{id}` | POST | GET, POST | Get a content block | Low — read-only GET added; legacy POST kept |
| `/content/updatecontent/{id}` | POST | POST, PATCH | Update a content block | Medium — update/state change; legacy POST kept |
| `/coupons/createcoupon` | POST | POST | Create a coupon | None (POST action/create retained) |
| `/coupons/deletecoupon/{id}` | POST | POST, DELETE | Delete a coupon | Medium — delete/state change; legacy POST kept |
| `/coupons/getcoupons` | POST | GET, POST | List coupons | Low — read-only GET added; legacy POST kept |
| `/coupons/updatecoupon/{id}` | POST | POST, PATCH | Update a coupon | Medium — update/state change; legacy POST kept |
| `/customer/addtocart/{id}` | POST | POST | Add a product to the cart | None (POST action/create retained) |
| `/customer/addtowishlist/{id}` | POST | POST | Add a product to the wishlist | None (POST action/create retained) |
| `/customer/deletecustomer` | POST | POST, DELETE | Delete a customer | Medium — delete/state change; legacy POST kept |
| `/customer/deletecustomer/{id}` | — | DELETE | Delete a customer | Medium — delete/state change |
| `/customer/forgotpassword` | POST | POST | Request a customer password reset | None (POST action/create retained) |
| `/customer/getcart/{id}` | POST | GET, POST | Get a customer's cart | Low — read-only GET added; legacy POST kept |
| `/customer/getcustomers` | POST | GET, POST | List customers | Low — read-only GET added; legacy POST kept |
| `/customer/getloggedincustomer` | POST | GET, POST | Get the logged-in customer | Low — read-only GET added; legacy POST kept |
| `/customer/getspecificcustomer/{id}` | POST | GET, POST | Get a customer | Low — read-only GET added; legacy POST kept |
| `/customer/loggedin` | POST | GET, POST | Check customer login status | Low — read-only GET added; legacy POST kept |
| `/customer/login` | POST | POST | Log in a customer | None (POST action/create retained) |
| `/customer/logout` | POST | POST | Log out a customer | None (POST action/create retained) |
| `/customer/register` | POST | POST | Register a customer | None (POST action/create retained) |
| `/customer/removefromcart/{id}` | POST | POST, DELETE | Remove a product from the cart | Medium — delete/state change; legacy POST kept |
| `/customer/removefromwishlist/{id}` | POST | POST, DELETE | Remove a product from the wishlist | Medium — delete/state change; legacy POST kept |
| `/customer/resetpassword/{resetToken}` | POST | POST | Reset a customer password with a token | None (POST action/create retained) |
| `/customer/updatecustomer/{id}` | POST | POST, PATCH, PUT | Update a customer | Medium — update/state change; legacy POST kept |
| `/customer/updatepassword/{id}` | POST | POST | Change the customer password | None (POST action/create retained) |
| `/dailyrates/createrate` | POST | POST | Create a daily rate | None (POST action/create retained) |
| `/dailyrates/deleterate/{id}` | POST | POST, DELETE | Delete a daily rate | Medium — delete/state change; legacy POST kept |
| `/dailyrates/getrates` | POST | GET, POST | List daily rates | Low — read-only GET added; legacy POST kept |
| `/dailyrates/updaterate/{id}` | POST | POST, PATCH | Update a daily rate | Medium — update/state change; legacy POST kept |
| `/dashboard/getdashboard` | POST | GET, POST | Get dashboard statistics | Low — read-only GET added; legacy POST kept |
| `/dashboard/stats` | POST | GET, POST | Get dashboard statistics | Low — read-only GET added; legacy POST kept |
| `/gallery/creategallery` | POST | POST | Create a gallery category | None (POST action/create retained) |
| `/gallery/deletegallerycat/{id}` | POST | POST, DELETE | Delete a gallery category | Medium — delete/state change; legacy POST kept |
| `/gallery/getallgallerycat` | POST | GET, POST | List all gallery categories | Low — read-only GET added; legacy POST kept |
| `/gallery/getgallerycat` | POST | GET, POST | List active gallery categories | Low — read-only GET added; legacy POST kept |
| `/gallery/getspecificgallery/{id}` | POST | GET, POST | Get a gallery category | Low — read-only GET added; legacy POST kept |
| `/gallery/updategallery/{id}` | POST | POST, PATCH | Update a gallery category | Medium — update/state change; legacy POST kept |
| `/gallerydetails/creategalleryitem` | POST | POST | Create a gallery item | None (POST action/create retained) |
| `/gallerydetails/deletegalleryitem/{id}` | POST | POST, DELETE | Delete a gallery item | Medium — delete/state change; legacy POST kept |
| `/gallerydetails/getallgalleries` | POST | GET, POST | List all gallery items | Low — read-only GET added; legacy POST kept |
| `/gallerydetails/getgalleries` | POST | GET, POST | List active gallery items | Low — read-only GET added; legacy POST kept |
| `/gallerydetails/getspecificgalleryitem/{id}` | POST | GET, POST | Get a gallery item | Low — read-only GET added; legacy POST kept |
| `/gallerydetails/updategalleryitem/{id}` | POST | POST, PATCH | Update a gallery item | Medium — update/state change; legacy POST kept |
| `/gst/creategst` | POST | POST | Create a GST rate | None (POST action/create retained) |
| `/gst/deletegst/{id}` | POST | POST, DELETE | Delete a GST rate | Medium — delete/state change; legacy POST kept |
| `/gst/getgst` | POST | GET, POST | List GST rates | Low — read-only GET added; legacy POST kept |
| `/gst/updategst/{id}` | POST | POST, PATCH | Update a GST rate | Medium — update/state change; legacy POST kept |
| `/material/creatematerial` | POST | POST | Create a material | None (POST action/create retained) |
| `/material/getmaterial` | POST | GET, POST | List materials | Low — read-only GET added; legacy POST kept |
| `/orders/addorder` | POST | POST | Create an order | None (POST action/create retained) |
| `/orders/deleteorder/{id}` | POST | POST, DELETE | Delete an order | Medium — delete/state change; legacy POST kept |
| `/orders/getallorders` | POST | GET, POST | List orders | Low — read-only GET added; legacy POST kept |
| `/orders/gethighcustomerdata` | POST | GET, POST | List high-value customers | Low — read-only GET added; legacy POST kept |
| `/orders/getmedcustomerdata` | POST | GET, POST | List medium-value customers | Low — read-only GET added; legacy POST kept |
| `/orders/getspecificorder/{id}` | POST | GET, POST | Get an order | Low — read-only GET added; legacy POST kept |
| `/orders/gettopsellproducts` | POST | GET, POST | List top-selling products | Low — read-only GET added; legacy POST kept |
| `/orders/updateorder/{id}` | POST | POST, PATCH | Update an order | Medium — update/state change; legacy POST kept |
| `/product/addproduct` | POST | POST | Create a product | None (POST action/create retained) |
| `/product/addvar` | POST | POST | Create a product variant | None (POST action/create retained) |
| `/product/deleteproduct/{id}` | POST | POST, DELETE | Delete a product | Medium — delete/state change; legacy POST kept |
| `/product/get-products-for-table` | POST | GET, POST | List all products for the admin table | Low — read-only GET added; legacy POST kept |
| `/product/getallproducts` | POST | GET, POST | List active products | Low — read-only GET added; legacy POST kept |
| `/product/getproductbytags` | POST | GET, POST | List products by tag | Low — read-only GET added; legacy POST kept |
| `/product/getproductsbycategoryid/{id}` | POST | GET, POST | List products by category | Low — read-only GET added; legacy POST kept |
| `/product/getproductsbysubcategories/{id}` | POST | GET, POST | List products by subcategory | Low — read-only GET added; legacy POST kept |
| `/product/getproductsbysubsubcategories/{id}` | POST | GET, POST | List products by sub-subcategory | Low — read-only GET added; legacy POST kept |
| `/product/getspecificproduct/{id}` | POST | GET, POST | Get a product | Low — read-only GET added; legacy POST kept |
| `/product/getvarproduct` | POST | GET, POST | List all product variants | Low — read-only GET added; legacy POST kept |
| `/product/getvarproduct/{id}` | POST | GET, POST | List variants of a product | Low — read-only GET added; legacy POST kept |
| `/product/updateproduct/{id}` | POST | POST, PATCH, PUT | Update a product | Medium — update/state change; legacy POST kept |
| `/season/createseason` | POST | POST | Create a season | None (POST action/create retained) |
| `/season/getseasons` | POST | GET, POST | List seasons | Low — read-only GET added; legacy POST kept |
| `/size/addsize` | POST | POST | Create a size | None (POST action/create retained) |
| `/size/deletesize/{id}` | POST | POST, DELETE | Delete a size | Medium — delete/state change; legacy POST kept |
| `/size/getsizes` | POST | GET, POST | List sizes | Low — read-only GET added; legacy POST kept |
| `/size/getspecificsize/{id}` | POST | GET, POST | Get a size | Low — read-only GET added; legacy POST kept |
| `/size/updatesize/{id}` | POST | POST, PATCH | Update a size | Medium — update/state change; legacy POST kept |
| `/stocks/createstock` | POST | POST | Create a stock record | None (POST action/create retained) |
| `/stocks/deletestocks/{id}` | POST | POST, DELETE | Delete a stock record | Medium — delete/state change; legacy POST kept |
| `/stocks/getspecificstock/{id}` | POST | GET, POST | Get a stock record | Low — read-only GET added; legacy POST kept |
| `/stocks/getstocks` | POST | GET, POST | List stock records | Low — read-only GET added; legacy POST kept |
| `/stocks/getstocksbycategory/{id}` | POST | GET, POST | List stock by category | Low — read-only GET added; legacy POST kept |
| `/stocks/getstocksbyweighttype/{id}` | POST | GET, POST | List stock by weight type | Low — read-only GET added; legacy POST kept |
| `/stocks/updatestock/{id}` | POST | POST, PATCH | Update a stock record | Medium — update/state change; legacy POST kept |
| `/subcategory/addsubcategory` | POST | POST | Create a subcategory | None (POST action/create retained) |
| `/subcategory/deletesubcategory/{id}` | POST | POST, DELETE | Delete a subcategory | Medium — delete/state change; legacy POST kept |
| `/subcategory/getsubcategories` | POST | GET, POST | List subcategories | Low — read-only GET added; legacy POST kept |
| `/subcategory/getsubspecificcategory/{id}` | POST | GET, POST | Get a subcategory | Low — read-only GET added; legacy POST kept |
| `/subcategory/updatesubcategory/{id}` | POST | POST, PATCH | Update a subcategory | Medium — update/state change; legacy POST kept |
| `/subscribe/addsubscribe` | POST | POST | Subscribe an email address | None (POST action/create retained) |
| `/subscribe/deletesubscribe/{id}` | POST | POST, DELETE | Delete a subscriber | Medium — delete/state change; legacy POST kept |
| `/subscribe/getsubscribe` | POST | GET, POST | List subscribers | Low — read-only GET added; legacy POST kept |
| `/subsubcategory/addsubsubcategory` | POST | POST | Create a sub-subcategory | None (POST action/create retained) |
| `/subsubcategory/deletesubsubcategory/{id}` | POST | POST, DELETE | Delete a sub-subcategory | Medium — delete/state change; legacy POST kept |
| `/subsubcategory/getsubsubcategories` | POST | GET, POST | List sub-subcategories | Low — read-only GET added; legacy POST kept |
| `/subsubcategory/getsubsubspecificcategory/{id}` | POST | GET, POST | Get a sub-subcategory | Low — read-only GET added; legacy POST kept |
| `/subsubcategory/updatesubsubcategory/{id}` | POST | POST, PATCH | Update a sub-subcategory | Medium — update/state change; legacy POST kept |

## Deprecated endpoints

All of the following paths keep their original `POST` registration for backward
compatibility. The OpenAPI operation is marked `deprecated: true` and points at
the canonical method. They will be removed only in a separately planned breaking
release, after the React client and admin panel have migrated.

```
/api/deleterole
/api/deleteuser
/api/getloggedinuser
/api/getroles
/api/getspecificuser
/api/getusers
/api/loggedin
/api/updateuser
/banner/all
/banner/delete-banner/{id}
/banner/get-banner
/banner/get-banner-by-id/{id}
/banner/update-banner/{id}
/blog/delete-blog/{id}
/blog/get-blog
/blog/get-blog/{id}
/blog/getblogbycategory/{id}
/blog/update-blog/{id}
/blogcategory/deletecategory/{id}
/blogcategory/getblogcategory
/blogcategory/getspecificcategory/{id}
/blogcategory/updateblogcategory/{id}
/category/deletecategory/{id}
/category/getcategories
/category/getspecificcategory/{id}
/category/getsubandsubsub/{id}
/category/updatecategory/{id}
/color/deletecolor/{id}
/color/getcolors
/color/getspecificcolor/{id}
/color/updatecolor/{id}
/contact/deletecontact/{id}
/contact/getcontact/{id}
/contact/getcontacts
/content/deletecontent/{id}
/content/getcontent
/content/getspecificcontent/{id}
/content/updatecontent/{id}
/coupons/deletecoupon/{id}
/coupons/getcoupons
/coupons/updatecoupon/{id}
/customer/deletecustomer
/customer/getcart/{id}
/customer/getcustomers
/customer/getloggedincustomer
/customer/getspecificcustomer/{id}
/customer/loggedin
/customer/removefromcart/{id}
/customer/removefromwishlist/{id}
/customer/updatecustomer/{id}
/dailyrates/deleterate/{id}
/dailyrates/getrates
/dailyrates/updaterate/{id}
/dashboard/getdashboard
/dashboard/stats
/gallery/deletegallerycat/{id}
/gallery/getallgallerycat
/gallery/getgallerycat
/gallery/getspecificgallery/{id}
/gallery/updategallery/{id}
/gallerydetails/deletegalleryitem/{id}
/gallerydetails/getallgalleries
/gallerydetails/getgalleries
/gallerydetails/getspecificgalleryitem/{id}
/gallerydetails/updategalleryitem/{id}
/gst/deletegst/{id}
/gst/getgst
/gst/updategst/{id}
/material/getmaterial
/orders/deleteorder/{id}
/orders/getallorders
/orders/gethighcustomerdata
/orders/getmedcustomerdata
/orders/getspecificorder/{id}
/orders/gettopsellproducts
/orders/updateorder/{id}
/product/deleteproduct/{id}
/product/get-products-for-table
/product/getallproducts
/product/getproductbytags
/product/getproductsbycategoryid/{id}
/product/getproductsbysubcategories/{id}
/product/getproductsbysubsubcategories/{id}
/product/getspecificproduct/{id}
/product/getvarproduct
/product/getvarproduct/{id}
/product/updateproduct/{id}
/season/getseasons
/size/deletesize/{id}
/size/getsizes
/size/getspecificsize/{id}
/size/updatesize/{id}
/stocks/deletestocks/{id}
/stocks/getspecificstock/{id}
/stocks/getstocks
/stocks/getstocksbycategory/{id}
/stocks/getstocksbyweighttype/{id}
/stocks/updatestock/{id}
/subcategory/deletesubcategory/{id}
/subcategory/getsubcategories
/subcategory/getsubspecificcategory/{id}
/subcategory/updatesubcategory/{id}
/subscribe/deletesubscribe/{id}
/subscribe/getsubscribe
/subsubcategory/deletesubsubcategory/{id}
/subsubcategory/getsubsubcategories
/subsubcategory/getsubsubspecificcategory/{id}
/subsubcategory/updatesubsubcategory/{id}
```

> POST-only create and action endpoints are **not** deprecated. Only the rows in
> the inventory marked "legacy POST kept" appear in this list.

## Backward compatibility

- Paths, request bodies, response bodies, and status-code behaviour are
  unchanged for existing POST clients.
- Auth middleware, role checks, ownership checks, soft deletes, server-side
  pricing/stock, and payment verification are reused by the new methods.
- `AUTH_STATUS=legacy` still returns HTTP 200 with `{"success":false}` for auth
  failures; `strict` continues to return 401/403.
- The legacy read handlers now accept query parameters on `GET`. On `POST` they
  still read the JSON/form body exactly as before.
- DELETE handlers read the id from the path, the query string, or (on the
  legacy POST form) the body.
- `Idempotency-Key` (header or body `idempotencyKey`) is unchanged and still
  makes checkout retries return the original order. Payment callbacks remain
  idempotent in `service.VerifyPayment`.

## CSRF protection

Cookie-authenticated unsafe requests are protected by the new
`middleware.CSRF` layer:

- `CSRF_MODE=off` disables the check (not recommended in production).
- `CSRF_MODE=same-origin` (default) rejects cross-origin cookie-authenticated
  unsafe requests. An explicitly allowed `CORS_ORIGINS` frontend must also send
  the double-submit token.
- `CSRF_MODE=strict` requires the double-submit token on every unsafe
  cookie-authenticated request.

Login sets a readable `csrf_token` cookie alongside the HttpOnly `token`
cookie. Browser clients must echo it in `X-CSRF-Token`. Requests authenticated
with `Authorization: Bearer ...` are exempt because browsers do not attach that
header automatically. `GET`, `HEAD`, and `OPTIONS` are never blocked.

## Migration plan for clients

1. Switch reads from `POST` to `GET` and move filters/pagination from the body
   to the query string.
2. Switch partial updates from `POST` to `PATCH`; use `PUT` only when sending a
   complete replacement.
3. Switch deletes from `POST` with a body to `DELETE` with the id in the path.
4. For cookie sessions, read the `csrf_token` cookie after login and send
   `X-CSRF-Token` on every unsafe request.
5. Keep `POST` as a fallback until the server deprecation window ends.

Suggested deprecation window: at least one minor release after both clients
have shipped the canonical methods. Remove the legacy registrations only in a
major release with a changelog entry.

## Tests added

`internal/handlers/rest_methods_test.go` covers:

- canonical `GET` reads, including query-string pagination;
- `405 Method Not Allowed` for unsupported methods;
- `GET` does not mutate users or orders;
- `PATCH` partial-update semantics;
- `PUT` replacement and required-field validation;
- `DELETE` soft delete and `?id=` compatibility;
- authorization on new methods;
- legacy `POST` compatibility;
- checkout idempotency over HTTP;
- CSRF `strict` and `same-origin` modes.

`internal/docs/spec_test.go` continues to enforce that every API route is
documented and that operation IDs are unique.

## Verification

```
go test ./...
go vet ./...
go build ./cmd/server
```

All three pass. The race suite can be run with `make test-race` (allow at least
240 seconds; bcrypt is slow under the race detector).
