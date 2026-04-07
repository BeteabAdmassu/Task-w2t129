DROP INDEX IF EXISTS idx_stock_transactions_tenant;
DROP INDEX IF EXISTS idx_stocktakes_tenant;

ALTER TABLE stock_transactions DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE stocktakes         DROP COLUMN IF EXISTS tenant_id;
