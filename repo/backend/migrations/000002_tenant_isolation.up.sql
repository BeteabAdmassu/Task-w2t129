-- Tenant isolation: adds tenant_id to all primary business tables.
-- For single-clinic deployments tenant_id is always 'default'.
-- For multi-clinic rollouts each clinic gets its own tenant_id value,
-- enforced via the TENANT_ID environment variable at startup.

ALTER TABLE auth_users         ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE members            ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE skus               ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE work_orders        ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE knowledge_points   ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE member_transactions ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE rate_tables        ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE charge_statements  ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

CREATE INDEX IF NOT EXISTS idx_auth_users_tenant          ON auth_users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_members_tenant             ON members(tenant_id);
CREATE INDEX IF NOT EXISTS idx_skus_tenant                ON skus(tenant_id);
CREATE INDEX IF NOT EXISTS idx_work_orders_tenant         ON work_orders(tenant_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_points_tenant    ON knowledge_points(tenant_id);
CREATE INDEX IF NOT EXISTS idx_member_transactions_tenant ON member_transactions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_rate_tables_tenant         ON rate_tables(tenant_id);
CREATE INDEX IF NOT EXISTS idx_charge_statements_tenant   ON charge_statements(tenant_id);
