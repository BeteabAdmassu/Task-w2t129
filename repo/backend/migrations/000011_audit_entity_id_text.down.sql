DROP INDEX IF EXISTS idx_audit_entity;
ALTER TABLE audit_log ALTER COLUMN entity_id TYPE UUID USING entity_id::UUID;
CREATE INDEX idx_audit_entity ON audit_log(entity_type, entity_id);
