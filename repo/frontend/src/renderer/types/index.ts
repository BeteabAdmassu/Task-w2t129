export interface User {
  id: string;
  username: string;
  role: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export interface ErrorResponse {
  error: string;
  code: number;
  details?: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface SKU {
  id: string;
  ndc?: string;
  upc?: string;
  name: string;
  description: string;
  unit_of_measure: string;
  low_stock_threshold: number;
  storage_location: string;
  is_active: boolean;
  created_at: string;
}

export interface InventoryBatch {
  id: string;
  sku_id: string;
  lot_number: string;
  expiration_date: string;
  quantity_on_hand: number;
  created_at: string;
}

export interface StockTransaction {
  id: string;
  sku_id: string;
  batch_id: string;
  type: 'in' | 'out';
  quantity: number;
  reason_code: string;
  prescription_id?: string;
  performed_by: string;
  created_at: string;
}

export interface Stocktake {
  id: string;
  period_start: string;
  period_end: string;
  status: 'draft' | 'in_progress' | 'completed';
  created_by: string;
  created_at: string;
  lines?: StocktakeLine[];
}

export interface StocktakeLine {
  id: string;
  stocktake_id: string;
  sku_id: string;
  batch_id: string;
  system_qty: number;
  counted_qty: number;
  variance: number;
  loss_reason?: string;
}

export interface LearningSubject {
  id: string;
  name: string;
  description: string;
  sort_order: number;
  created_at: string;
}

export interface LearningChapter {
  id: string;
  subject_id: string;
  name: string;
  sort_order: number;
}

export interface KnowledgePoint {
  id: string;
  chapter_id: string;
  title: string;
  content: string;
  tags: string[];
  classifications?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface WorkOrder {
  id: string;
  submitted_by: string;
  assigned_to?: string;
  trade: string;
  priority: 'urgent' | 'high' | 'normal';
  sla_deadline: string;
  status: string;
  description: string;
  location: string;
  parts_cost: number;
  labor_cost: number;
  rating?: number;
  closed_at?: string;
  created_at: string;
}

export interface Member {
  id: string;
  name: string;
  phone: string;
  tier_id: string;
  points_balance: number;
  stored_value: number;
  status: 'active' | 'frozen' | 'expired';
  frozen_at?: string;
  expires_at: string;
  created_at: string;
}

export interface MembershipTier {
  id: string;
  name: string;
  benefits: Record<string, unknown>;
  sort_order: number;
}

export interface SessionPackage {
  id: string;
  member_id: string;
  package_name: string;
  total_sessions: number;
  remaining_sessions: number;
  expires_at: string;
}

export interface MemberTransaction {
  id: string;
  member_id: string;
  type: string;
  amount: number;
  description: string;
  performed_by: string;
  created_at: string;
}

export interface RateTable {
  id: string;
  name: string;
  type: 'distance' | 'weight' | 'volume';
  tiers: Array<{ min: number; max: number; rate: number }>;
  fuel_surcharge_pct: number;
  taxable: boolean;
  effective_date: string;
}

/** Canonical statement lifecycle: pending → approved → paid */
export type StatementStatus = 'pending' | 'approved' | 'paid';

export interface ChargeStatement {
  id: string;
  period_start: string;
  period_end: string;
  total_amount: number;
  expected_total: number;
  status: StatementStatus;
  approved_by?: string;
  variance_notes?: string;
  paid_at?: string;
  created_at: string;
}

export interface ChargeLineItem {
  id: string;
  statement_id: string;
  description: string;
  quantity: number;
  unit_price: number;
  surcharge: number;
  tax: number;
  total: number;
}

export interface DraftCheckpoint {
  id: string;
  user_id: string;
  form_type: string;
  form_id?: string;
  state_json: Record<string, unknown>;
  saved_at: string;
}

export interface Role {
  id: string;
  display_name: string;
  permissions: Record<string, boolean>;
}
