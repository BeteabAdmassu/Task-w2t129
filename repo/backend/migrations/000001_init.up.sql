-- MedOps Offline Operations Console - Initial Schema

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Roles
CREATE TABLE roles (
    id VARCHAR(50) PRIMARY KEY,
    display_name VARCHAR(100) NOT NULL,
    permissions JSONB NOT NULL DEFAULT '{}'
);

INSERT INTO roles (id, display_name, permissions) VALUES
('system_admin', 'System Administrator', '{"admin": true, "users": true, "config": true, "backup": true, "rate_tables": true, "statements": true}'),
('inventory_pharmacist', 'Inventory Pharmacist / Materials Manager', '{"inventory": true, "skus": true, "stocktake": true}'),
('learning_coordinator', 'Learning Coordinator', '{"learning_write": true, "learning_read": true}'),
('front_desk', 'Front Desk', '{"members": true, "membership_tiers": true}'),
('maintenance_tech', 'Maintenance Supervisor / Technician', '{"work_orders_manage": true, "work_orders_read": true}');

-- Auth Users
CREATE TABLE auth_users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL REFERENCES roles(id),
    failed_attempts INT DEFAULT 0,
    locked_until TIMESTAMPTZ NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- SKUs
CREATE TABLE skus (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ndc VARCHAR(20),
    upc VARCHAR(20),
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    unit_of_measure VARCHAR(50) NOT NULL,
    low_stock_threshold INT DEFAULT 10,
    storage_location VARCHAR(255) DEFAULT '',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_skus_ndc ON skus(ndc);
CREATE INDEX idx_skus_upc ON skus(upc);
CREATE INDEX idx_skus_name ON skus(name);

-- Inventory Batches
CREATE TABLE inventory_batches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sku_id UUID NOT NULL REFERENCES skus(id),
    lot_number VARCHAR(100) NOT NULL,
    expiration_date DATE NOT NULL,
    quantity_on_hand INT NOT NULL CHECK (quantity_on_hand >= 0),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_batches_sku_id ON inventory_batches(sku_id);
CREATE INDEX idx_batches_expiration ON inventory_batches(expiration_date);

-- Stock Transactions
CREATE TABLE stock_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sku_id UUID NOT NULL REFERENCES skus(id),
    batch_id UUID NOT NULL REFERENCES inventory_batches(id),
    type VARCHAR(10) NOT NULL CHECK (type IN ('in', 'out')),
    quantity INT NOT NULL,
    reason_code VARCHAR(50) NOT NULL,
    prescription_id VARCHAR(100),
    performed_by UUID NOT NULL REFERENCES auth_users(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_stock_tx_sku ON stock_transactions(sku_id);
CREATE INDEX idx_stock_tx_created ON stock_transactions(created_at);

-- Stocktakes
CREATE TABLE stocktakes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'in_progress', 'completed')),
    created_by UUID NOT NULL REFERENCES auth_users(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE stocktake_lines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    stocktake_id UUID NOT NULL REFERENCES stocktakes(id) ON DELETE CASCADE,
    sku_id UUID NOT NULL REFERENCES skus(id),
    batch_id UUID NOT NULL REFERENCES inventory_batches(id),
    system_qty INT NOT NULL,
    counted_qty INT DEFAULT 0,
    variance INT DEFAULT 0,
    loss_reason TEXT
);

-- Learning
CREATE TABLE learning_subjects (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    sort_order INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE learning_chapters (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    subject_id UUID NOT NULL REFERENCES learning_subjects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    sort_order INT DEFAULT 0
);

CREATE TABLE knowledge_points (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    chapter_id UUID NOT NULL REFERENCES learning_chapters(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    content TEXT DEFAULT '',
    tags TEXT[] DEFAULT '{}',
    classifications JSONB DEFAULT '{}',
    search_vector TSVECTOR,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_kp_search_vector ON knowledge_points USING GIN(search_vector);
CREATE INDEX idx_kp_tags ON knowledge_points USING GIN(tags);

-- Trigger for search vector
CREATE OR REPLACE FUNCTION update_kp_search_vector() RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', COALESCE(NEW.title, '') || ' ' || COALESCE(NEW.content, '') || ' ' || COALESCE(array_to_string(NEW.tags, ' '), ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER kp_search_vector_update
    BEFORE INSERT OR UPDATE ON knowledge_points
    FOR EACH ROW EXECUTE FUNCTION update_kp_search_vector();

-- Work Orders
CREATE TABLE work_orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    submitted_by UUID NOT NULL REFERENCES auth_users(id),
    assigned_to UUID REFERENCES auth_users(id),
    trade VARCHAR(50) NOT NULL CHECK (trade IN ('electrical', 'plumbing', 'hvac', 'general')),
    priority VARCHAR(10) NOT NULL CHECK (priority IN ('urgent', 'high', 'normal')),
    sla_deadline TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted', 'dispatched', 'in_progress', 'completed', 'closed')),
    description TEXT NOT NULL,
    location VARCHAR(255) NOT NULL,
    parts_cost DECIMAL(10,2) DEFAULT 0,
    labor_cost DECIMAL(10,2) DEFAULT 0,
    rating INT CHECK (rating >= 1 AND rating <= 5),
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_wo_status ON work_orders(status);
CREATE INDEX idx_wo_assigned ON work_orders(assigned_to);
CREATE INDEX idx_wo_priority ON work_orders(priority);
CREATE INDEX idx_wo_created ON work_orders(created_at);

-- Work Order Photos
CREATE TABLE managed_files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sha256 VARCHAR(64) UNIQUE NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    storage_path VARCHAR(500) NOT NULL,
    uploaded_by UUID REFERENCES auth_users(id),
    retention_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_files_sha256 ON managed_files(sha256);

CREATE TABLE work_order_photos (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    work_order_id UUID NOT NULL REFERENCES work_orders(id) ON DELETE CASCADE,
    file_id UUID NOT NULL REFERENCES managed_files(id)
);

-- Membership
CREATE TABLE membership_tiers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    benefits JSONB DEFAULT '{}',
    sort_order INT DEFAULT 0
);

INSERT INTO membership_tiers (id, name, benefits, sort_order) VALUES
(uuid_generate_v4(), 'Bronze', '{"discount_pct": 5, "points_multiplier": 1}', 1),
(uuid_generate_v4(), 'Silver', '{"discount_pct": 10, "points_multiplier": 1.5}', 2),
(uuid_generate_v4(), 'Gold', '{"discount_pct": 15, "points_multiplier": 2}', 3),
(uuid_generate_v4(), 'Platinum', '{"discount_pct": 20, "points_multiplier": 3}', 4);

CREATE TABLE members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    id_number_encrypted BYTEA,
    phone VARCHAR(50) DEFAULT '',
    tier_id UUID NOT NULL REFERENCES membership_tiers(id),
    points_balance INT DEFAULT 0,
    stored_value DECIMAL(10,2) DEFAULT 0,
    stored_value_encrypted BYTEA,
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'frozen', 'expired')),
    frozen_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_members_status ON members(status);
CREATE INDEX idx_members_expires ON members(expires_at);

CREATE TABLE session_packages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    member_id UUID NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    package_name VARCHAR(255) NOT NULL,
    total_sessions INT NOT NULL,
    remaining_sessions INT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE member_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    member_id UUID NOT NULL REFERENCES members(id),
    type VARCHAR(30) NOT NULL CHECK (type IN ('points_earn', 'points_redeem', 'stored_value_add', 'stored_value_use', 'stored_value_refund', 'session_redeem')),
    amount DECIMAL(10,2) DEFAULT 0,
    description TEXT DEFAULT '',
    performed_by UUID NOT NULL REFERENCES auth_users(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Rate Tables & Charges
CREATE TABLE rate_tables (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    type VARCHAR(30) NOT NULL CHECK (type IN ('distance', 'weight', 'volume')),
    tiers JSONB NOT NULL DEFAULT '[]',
    fuel_surcharge_pct DECIMAL(5,2) DEFAULT 0,
    taxable BOOLEAN DEFAULT false,
    effective_date DATE NOT NULL
);

CREATE TABLE charge_statements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    total_amount DECIMAL(12,2) DEFAULT 0,
    status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'pending_approval', 'approved', 'exported')),
    approved_by_1 UUID REFERENCES auth_users(id),
    approved_by_2 UUID REFERENCES auth_users(id),
    variance_notes TEXT,
    exported_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE charge_line_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    statement_id UUID NOT NULL REFERENCES charge_statements(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    quantity DECIMAL(10,3) NOT NULL,
    unit_price DECIMAL(10,4) NOT NULL,
    surcharge DECIMAL(10,2) DEFAULT 0,
    tax DECIMAL(10,2) DEFAULT 0,
    total DECIMAL(12,2) NOT NULL
);

-- Draft Checkpoints
CREATE TABLE draft_checkpoints (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth_users(id),
    form_type VARCHAR(100) NOT NULL,
    form_id VARCHAR(100),
    state_json JSONB NOT NULL,
    saved_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_drafts_user_form ON draft_checkpoints(user_id, form_type, form_id);

-- Audit Log
CREATE TABLE audit_log (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES auth_users(id),
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID,
    details JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_audit_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_created ON audit_log(created_at);

-- System Config
CREATE TABLE system_config (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL
);

-- Seed default admin user (password: AdminPass1234)
INSERT INTO auth_users (id, username, password_hash, role) VALUES
(uuid_generate_v4(), 'admin', '$2b$10$5ATxtLrjEY3zY.R.qpO7TeCTfIrnE/uHIB9sEjqmj6cSBEiIMGbdq', 'system_admin');
