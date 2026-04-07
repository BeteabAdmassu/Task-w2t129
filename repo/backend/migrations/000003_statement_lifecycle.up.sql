-- Migrate charge_statements to canonical 3-state lifecycle: pending → approved → paid
-- Also renames the two-approver columns to a single approved_by column,
-- renames exported_at → paid_at, and adds expected_total.
-- Adds UNIQUE constraint to work_order_photos for idempotent photo linkage.

-- ── charge_statements columns ─────────────────────────────────────────────────

ALTER TABLE charge_statements
  ADD COLUMN IF NOT EXISTS expected_total NUMERIC(12,2) NOT NULL DEFAULT 0;

ALTER TABLE charge_statements
  ADD COLUMN IF NOT EXISTS approved_by UUID REFERENCES auth_users(id);

ALTER TABLE charge_statements
  ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ;

-- Copy first-approver to the new single column where present.
UPDATE charge_statements SET approved_by = approved_by_1 WHERE approved_by_1 IS NOT NULL;

-- Remap status values before changing the CHECK constraint.
UPDATE charge_statements SET status = 'pending' WHERE status IN ('draft', 'pending_approval');
UPDATE charge_statements SET status = 'paid'    WHERE status = 'exported';
-- 'approved' stays 'approved'.

-- Copy exported_at → paid_at for any already-exported rows.
UPDATE charge_statements SET paid_at = exported_at WHERE exported_at IS NOT NULL;

ALTER TABLE charge_statements
  DROP CONSTRAINT IF EXISTS charge_statements_status_check;

ALTER TABLE charge_statements
  ADD CONSTRAINT charge_statements_status_check
  CHECK (status IN ('pending', 'approved', 'paid'));

ALTER TABLE charge_statements
  ALTER COLUMN status SET DEFAULT 'pending';

-- Drop the now-superseded columns.
ALTER TABLE charge_statements DROP COLUMN IF EXISTS approved_by_1;
ALTER TABLE charge_statements DROP COLUMN IF EXISTS approved_by_2;
ALTER TABLE charge_statements DROP COLUMN IF EXISTS exported_at;

-- ── work_order_photos unique constraint ───────────────────────────────────────

ALTER TABLE work_order_photos
  ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT NOW();

ALTER TABLE work_order_photos
  DROP CONSTRAINT IF EXISTS work_order_photos_wo_file_unique;

ALTER TABLE work_order_photos
  ADD CONSTRAINT work_order_photos_wo_file_unique UNIQUE (work_order_id, file_id);
