DROP INDEX IF EXISTS idx_learning_chapters_tenant;
DROP INDEX IF EXISTS idx_learning_subjects_tenant;

ALTER TABLE learning_chapters DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE learning_subjects DROP COLUMN IF EXISTS tenant_id;
