ALTER TABLE work_order_photos
  DROP CONSTRAINT IF EXISTS work_order_photos_wo_file_unique;
ALTER TABLE work_order_photos DROP COLUMN IF EXISTS created_at;

ALTER TABLE charge_statements
  ADD COLUMN IF NOT EXISTS approved_by_1 UUID REFERENCES auth_users(id);
ALTER TABLE charge_statements
  ADD COLUMN IF NOT EXISTS approved_by_2 UUID REFERENCES auth_users(id);
ALTER TABLE charge_statements
  ADD COLUMN IF NOT EXISTS exported_at TIMESTAMPTZ;

UPDATE charge_statements SET approved_by_1 = approved_by WHERE approved_by IS NOT NULL;
UPDATE charge_statements SET exported_at   = paid_at     WHERE paid_at     IS NOT NULL;
UPDATE charge_statements SET status = 'draft'    WHERE status = 'pending';
UPDATE charge_statements SET status = 'exported' WHERE status = 'paid';

ALTER TABLE charge_statements
  DROP CONSTRAINT IF EXISTS charge_statements_status_check;
ALTER TABLE charge_statements
  ADD CONSTRAINT charge_statements_status_check
  CHECK (status IN ('draft', 'pending_approval', 'approved', 'exported'));
ALTER TABLE charge_statements
  ALTER COLUMN status SET DEFAULT 'draft';

ALTER TABLE charge_statements DROP COLUMN IF EXISTS approved_by;
ALTER TABLE charge_statements DROP COLUMN IF EXISTS paid_at;
ALTER TABLE charge_statements DROP COLUMN IF EXISTS expected_total;
