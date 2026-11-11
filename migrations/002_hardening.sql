-- Additive hardening migration. No existing table is recreated and no column is
-- dropped. Existing rows keep their data; new columns are nullable or have a
-- constant default.

-- Order lifecycle and idempotency.
ALTER TABLE "Order" ADD COLUMN "paymentStatus" TEXT NOT NULL DEFAULT 'unpaid';
ALTER TABLE "Order" ADD COLUMN "idempotencyKey" TEXT;
ALTER TABLE "Order" ADD COLUMN "deletedAt" DATETIME;

CREATE UNIQUE INDEX IF NOT EXISTS "Order_idempotencyKey_key"
    ON "Order"("idempotencyKey") WHERE "idempotencyKey" IS NOT NULL;
CREATE INDEX IF NOT EXISTS "Order_deletedAt_idx" ON "Order"("deletedAt");

-- Immutable line-item snapshot. Prices and product metadata are copied at
-- checkout so later catalogue edits cannot rewrite historical orders.
ALTER TABLE "OrderItem" ADD COLUMN "productName" TEXT;
ALTER TABLE "OrderItem" ADD COLUMN "sku" TEXT;
ALTER TABLE "OrderItem" ADD COLUMN "unitPrice" REAL;
ALTER TABLE "OrderItem" ADD COLUMN "lineTotal" REAL;
ALTER TABLE "OrderItem" ADD COLUMN "tax" REAL;
-- Records which inventory source was decremented at checkout so cancellation
-- restores exactly the same source. Values: stock, product, untracked.
ALTER TABLE "OrderItem" ADD COLUMN "stockSource" TEXT;

CREATE INDEX IF NOT EXISTS "OrderItem_orderId_productId_idx"
    ON "OrderItem"("orderId", "productId");

-- Password reset tokens become single-use.
ALTER TABLE "Token" ADD COLUMN "usedAt" DATETIME;

-- Payment attempts are recorded separately from the order. A client can never
-- set an order to paid by posting a status field.
CREATE TABLE IF NOT EXISTS "Payment" (
    "id" TEXT NOT NULL PRIMARY KEY,
    "orderId" TEXT NOT NULL,
    "customerId" TEXT NOT NULL,
    "amount" REAL NOT NULL,
    "currency" TEXT NOT NULL DEFAULT 'INR',
    "provider" TEXT,
    "providerReference" TEXT,
    "status" TEXT NOT NULL DEFAULT 'pending',
    "failureReason" TEXT,
    "idempotencyKey" TEXT,
    "createdAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT "Payment_orderId_fkey" FOREIGN KEY ("orderId") REFERENCES "Order" ("id") ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT "Payment_customerId_fkey" FOREIGN KEY ("customerId") REFERENCES "Customer" ("id") ON DELETE CASCADE ON UPDATE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS "Payment_idempotencyKey_key"
    ON "Payment"("idempotencyKey") WHERE "idempotencyKey" IS NOT NULL;
CREATE INDEX IF NOT EXISTS "Payment_orderId_idx" ON "Payment"("orderId");
CREATE INDEX IF NOT EXISTS "Payment_customerId_idx" ON "Payment"("customerId");
CREATE INDEX IF NOT EXISTS "Payment_status_idx" ON "Payment"("status");
