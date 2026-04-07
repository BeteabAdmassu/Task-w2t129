ALTER TABLE auth_users          DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE members             DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE skus                DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE work_orders         DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE knowledge_points    DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE member_transactions DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE rate_tables         DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE charge_statements   DROP COLUMN IF EXISTS tenant_id;
