-- Add tenant_id to learning content tables so subjects and chapters are
-- scoped per tenant rather than shared globally.

ALTER TABLE learning_subjects
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

ALTER TABLE learning_chapters
    ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';

CREATE INDEX IF NOT EXISTS idx_learning_subjects_tenant ON learning_subjects(tenant_id);
CREATE INDEX IF NOT EXISTS idx_learning_chapters_tenant ON learning_chapters(tenant_id);
