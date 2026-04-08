-- F-003: Change entity_id from UUID to TEXT so non-UUID values (e.g. "backup",
-- config keys, version strings) no longer cause silent audit log insert failures.
DROP INDEX IF EXISTS idx_audit_entity;
ALTER TABLE audit_log ALTER COLUMN entity_id TYPE TEXT USING entity_id::TEXT;
CREATE INDEX idx_audit_entity ON audit_log(entity_type, entity_id);
