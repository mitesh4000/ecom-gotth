-- Storefront support. Additive only: existing catalogue and cart tables keep
-- working, and the JSON API continues to use CartItem for signed-in customers.

-- Human-readable product URLs. Existing rows fall back to their id so every
-- product has a stable, unique address immediately after the migration.
ALTER TABLE "Product" ADD COLUMN "slug" TEXT;
UPDATE "Product" SET "slug" = "id" WHERE "slug" IS NULL OR "slug" = '';
CREATE UNIQUE INDEX IF NOT EXISTS "Product_slug_key" ON "Product"("slug");

-- Catalogue browsing index used by the storefront listing and home sections.
CREATE INDEX IF NOT EXISTS "Product_isVariant_isActive_idx" ON "Product"("isVariant", "isActive");
CREATE INDEX IF NOT EXISTS "Product_isProductPopular_idx" ON "Product"("isProductPopular");
CREATE INDEX IF NOT EXISTS "Product_isProductNew_idx" ON "Product"("isProductNew");

-- Guest carts are server-side and keyed by a hash of an opaque, HttpOnly
-- session cookie. The browser never supplies prices, totals or an owner id.
-- Signed-in customers continue to use the existing CartItem table.
CREATE TABLE IF NOT EXISTS "GuestCartItem" (
    "id" TEXT NOT NULL PRIMARY KEY,
    "sessionId" TEXT NOT NULL,
    "productId" TEXT NOT NULL,
    "quantity" INTEGER NOT NULL DEFAULT 1,
    "tax" REAL,
    "createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "GuestCartItem_productId_fkey" FOREIGN KEY ("productId") REFERENCES "Product" ("id") ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS "GuestCartItem_sessionId_productId_key"
    ON "GuestCartItem"("sessionId", "productId");
CREATE INDEX IF NOT EXISTS "GuestCartItem_sessionId_idx" ON "GuestCartItem"("sessionId");
CREATE INDEX IF NOT EXISTS "GuestCartItem_productId_idx" ON "GuestCartItem"("productId");
