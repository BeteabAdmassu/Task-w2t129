-- Add tenant_id directly to stocktakes and stock_transactions.
-- Prior to this migration these tables relied on JOIN-based scoping through
-- auth_users (stocktakes.created_by) and skus (stock_transactions.sku_id).
-- Direct columns allow simpler, safer WHERE predicates without fragile JOINs.

ALTER TABLE stocktakes
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE stock_transactions
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

CREATE INDEX IF NOT EXISTS idx_stocktakes_tenant        ON stocktakes(tenant_id);
CREATE INDEX IF NOT EXISTS idx_stock_transactions_tenant ON stock_transactions(tenant_id);
