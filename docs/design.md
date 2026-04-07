# Design Document — MedOps Offline Operations Console

## Business Goal
Deliver an offline-first desktop workspace for community clinics and pharmacy-adjacent outpatient centers to manage regulated inventory, staff learning, memberships, and facilities work orders — eliminating reliance on internet connectivity while enforcing healthcare compliance rules.

## Core Requirements

### Desktop Shell & UX
1. Electron-based desktop app with React UI, embedding a local Go (Echo) backend service
2. Bundled local PostgreSQL instance; delivered as a Windows .msi installer
3. No internet dependencies — fully offline operation
4. Baseline resolution 1920x1080 with high-DPI scaling support
5. Multi-window parallel work (e.g., inventory receiving alongside work-order queue)
6. Right-click context menus on all tables (quick adjust, void, print, export)
7. Keyboard-first shortcuts: Ctrl+K global search, Ctrl+N new record, Ctrl+Enter save, F2 edit row
8. System tray mode with lock screen (without quitting), local reminders (membership expiry at 14/7/1 days, low-stock alerts), and one-click backup

### Authentication & Security
9. Local username/password auth with minimum 12-character passwords
10. Account lockout after 5 failed attempts for 15 minutes
11. Role-based access control (RBAC) for five defined roles
12. Sensitive fields (real-name verification status, deposits, violation notes) encrypted at rest using a locally stored key protected by OS credential storage
13. Sensitive fields masked on-screen by default

### Roles
14. System Administrator — local security, configuration, backups
15. Inventory Pharmacist / Materials Manager — drug/consumable receiving, dispensing deductions, counts
16. Learning Coordinator — content curation, competency review
17. Front Desk — membership validation, benefit redemption
18. Maintenance Supervisor / Technician — repair intake and closure

### Inventory Management
19. SKU model with NDC/UPC, batch/lot, expiration date, storage location, unit-of-measure
20. Stock in/out transactions require reason codes
21. Prescription-based deduction enforces "cannot dispense past expiration" and "cannot reduce below zero"
22. Configurable low-stock thresholds per SKU
23. Monthly stocktake workflow producing variance and loss records

### Learning Management
24. Subject → Chapter → Knowledge-Point hierarchy with tags and multi-dimensional classifications
25. Full-text search by title/keyword/tag
26. Import/export Markdown, HTML, and local files

### Work Orders (Facilities Maintenance)
27. Staff submit repair requests with photos, description, location text, and priority
28. Auto-dispatch based on trade and workload
29. SLA enforcement: Urgent 4 hours, High 24 hours, Normal 3 business days
30. Capture parts/labor cost and responsibility assignment
31. 1–5 rating at close
32. Archive for trend analytics

### Memberships
33. Tiered membership levels
34. Points, stored-value, and session packages
35. Freeze/unfreeze capability
36. Strict benefit validation: no redemption after expiration
37. Partial session redemption not allowed
38. Stored-value refunds only within 7 days if unused

### Charges & Settlement
39. Offline rate tables (distance in miles — manual or USB CSV import, weight/volume tiering, fuel surcharge %, taxable flags)
40. Generate statements by date range
41. Reconcile variances with mandatory notes for deltas over $25.00
42. Single-step approval: reconcile (`pending`→`approved`) carries `expected_total` and optional `variance_notes` (required when ABS(total−expected)>$25); export transitions `approved`→`paid`
43. Export signed CSV/JSON files for downstream systems

### File Operations
44. Deduplicate attachments by SHA-256 fingerprint
45. App-managed directory with configurable retention rules
46. Export audit-ready ZIP bundles

### Performance & Reliability
47. Cold-start in under 10 seconds on standard office PCs
48. Stable for 30-day uptime with explicit resource disposal
49. Automatic state recovery — draft checkpoints every 30 seconds for forms
50. Offline updates via imported packages with one-click rollback to previous version

## Main User Flow

### Inventory Receiving (Primary Happy Path)
1. User launches MedOps → app cold-starts in <10s → lock screen appears
2. User enters credentials → system validates locally → role-appropriate dashboard loads
3. Inventory Pharmacist clicks "New Receiving" (Ctrl+N) → receiving form opens in a new window
4. User scans or enters NDC/UPC → system looks up SKU, pre-fills details
5. User enters batch/lot, expiration date, quantity, storage location, reason code → form auto-saves draft every 30s
6. User submits (Ctrl+Enter) → system validates expiration > today, creates stock-in transaction
7. If SKU was below low-stock threshold, system clears low-stock alert in tray
8. User right-clicks transaction row → prints receiving slip
9. User minimizes to system tray (lock screen) → moves to next task

### Work Order Flow
1. Staff member submits repair request with photo, description, location, priority
2. System auto-dispatches to technician based on trade + workload
3. SLA timer starts (Urgent: 4h, High: 24h, Normal: 3 business days)
4. Technician opens work order → records parts/labor cost → closes order
5. Requester rates 1–5 → order archived for trend analytics

### Membership Redemption Flow
1. Front Desk opens member lookup (Ctrl+K) → searches by name/ID
2. System displays membership tier, points balance, stored-value, session packages, status
3. Front Desk selects benefit to redeem → system validates: not expired, not frozen, no partial sessions
4. Redemption recorded → balances updated

## Tech Stack
- **Desktop Shell**: Electron (latest LTS) — required by prompt
- **Frontend**: React 18 + TypeScript + Vite (bundled inside Electron renderer)
- **Backend**: Go 1.22 + Echo v4 (embedded local HTTP service)
- **Database**: PostgreSQL 16 (bundled with .msi installer, runs as local service)
- **ORM/Query**: sqlc or GORM for Go
- **Full-text Search**: PostgreSQL tsvector/tsquery (no external search engine — offline constraint)
- **Encryption**: Go `crypto/aes` + OS credential storage via `github.com/zalando/go-keyring` (Windows Credential Manager)
- **File Storage**: Local filesystem with SHA-256 dedup
- **Installer**: WiX Toolset / electron-builder for .msi packaging
- **Testing**: Go `testing` + testify (backend), Vitest + React Testing Library (frontend)

## Database Schema

### auth_users
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| username | VARCHAR(100) UNIQUE | |
| password_hash | VARCHAR(255) | bcrypt |
| role | VARCHAR(50) | FK to roles |
| failed_attempts | INT DEFAULT 0 | |
| locked_until | TIMESTAMPTZ NULL | |
| is_active | BOOLEAN DEFAULT true | |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |

### roles
| Column | Type | Notes |
|--------|------|-------|
| id | VARCHAR(50) PK | system_admin, inventory_pharmacist, learning_coordinator, front_desk, maintenance_tech |
| display_name | VARCHAR(100) | |
| permissions | JSONB | permission matrix |

### skus
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| ndc | VARCHAR(20) | nullable |
| upc | VARCHAR(20) | nullable |
| name | VARCHAR(255) | |
| description | TEXT | |
| unit_of_measure | VARCHAR(50) | |
| low_stock_threshold | INT | |
| storage_location | VARCHAR(255) | |
| is_active | BOOLEAN DEFAULT true | |
| created_at | TIMESTAMPTZ | |

**Indexes**: idx_skus_ndc, idx_skus_upc, idx_skus_name

### inventory_batches
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| sku_id | UUID FK → skus | |
| lot_number | VARCHAR(100) | |
| expiration_date | DATE | |
| quantity_on_hand | INT | CHECK >= 0 |
| created_at | TIMESTAMPTZ | |

**Indexes**: idx_batches_sku_id, idx_batches_expiration

### stock_transactions
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| sku_id | UUID FK → skus | |
| batch_id | UUID FK → inventory_batches | |
| type | VARCHAR(10) | 'in' or 'out' |
| quantity | INT | |
| reason_code | VARCHAR(50) | required |
| prescription_id | VARCHAR(100) | nullable |
| performed_by | UUID FK → auth_users | |
| created_at | TIMESTAMPTZ | |

**Indexes**: idx_stock_tx_sku, idx_stock_tx_created

### stocktakes
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| period_start | DATE | |
| period_end | DATE | |
| status | VARCHAR(20) | draft, in_progress, completed |
| created_by | UUID FK → auth_users | |
| created_at | TIMESTAMPTZ | |

### stocktake_lines
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| stocktake_id | UUID FK → stocktakes | |
| sku_id | UUID FK → skus | |
| batch_id | UUID FK → inventory_batches | |
| system_qty | INT | |
| counted_qty | INT | |
| variance | INT | computed |
| loss_reason | TEXT | nullable |

### learning_subjects
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | VARCHAR(255) | |
| description | TEXT | |
| sort_order | INT | |
| created_at | TIMESTAMPTZ | |

### learning_chapters
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| subject_id | UUID FK → learning_subjects | |
| name | VARCHAR(255) | |
| sort_order | INT | |

### knowledge_points
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| chapter_id | UUID FK → learning_chapters | |
| title | VARCHAR(255) | |
| content | TEXT | Markdown/HTML |
| tags | TEXT[] | array of tags |
| classifications | JSONB | multi-dimensional |
| search_vector | TSVECTOR | for full-text search |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |

**Indexes**: idx_kp_search_vector (GIN), idx_kp_tags (GIN)

### work_orders
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| submitted_by | UUID FK → auth_users | |
| assigned_to | UUID FK → auth_users | nullable |
| trade | VARCHAR(50) | electrical, plumbing, HVAC, general |
| priority | VARCHAR(10) | urgent, high, normal |
| sla_deadline | TIMESTAMPTZ | computed from priority |
| status | VARCHAR(20) | submitted, dispatched, in_progress, completed, closed |
| description | TEXT | |
| location | VARCHAR(255) | |
| parts_cost | DECIMAL(10,2) DEFAULT 0 | |
| labor_cost | DECIMAL(10,2) DEFAULT 0 | |
| rating | INT | 1-5, nullable |
| closed_at | TIMESTAMPTZ | nullable |
| created_at | TIMESTAMPTZ | |

**Indexes**: idx_wo_status, idx_wo_assigned, idx_wo_priority, idx_wo_created

### work_order_photos
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| work_order_id | UUID FK → work_orders | |
| file_id | UUID FK → managed_files | |

### members
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | VARCHAR(255) | |
| id_number_encrypted | BYTEA | encrypted at rest |
| phone | VARCHAR(50) | |
| tier_id | UUID FK → membership_tiers | |
| points_balance | INT DEFAULT 0 | |
| stored_value | DECIMAL(10,2) DEFAULT 0 | |
| stored_value_encrypted | BYTEA | encrypted balance mirror |
| status | VARCHAR(20) | active, frozen, expired |
| frozen_at | TIMESTAMPTZ | nullable |
| expires_at | TIMESTAMPTZ | |
| created_at | TIMESTAMPTZ | |

**Indexes**: idx_members_status, idx_members_expires

### membership_tiers
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | VARCHAR(100) | |
| benefits | JSONB | |
| sort_order | INT | |

### session_packages
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| member_id | UUID FK → members | |
| package_name | VARCHAR(255) | |
| total_sessions | INT | |
| remaining_sessions | INT | |
| expires_at | TIMESTAMPTZ | |

### member_transactions
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| member_id | UUID FK → members | |
| type | VARCHAR(30) | points_earn, points_redeem, stored_value_add, stored_value_use, stored_value_refund, session_redeem |
| amount | DECIMAL(10,2) | |
| description | TEXT | |
| performed_by | UUID FK → auth_users | |
| created_at | TIMESTAMPTZ | |

### rate_tables
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| name | VARCHAR(255) | |
| type | VARCHAR(30) | distance, weight, volume |
| tiers | JSONB | array of {min, max, rate} |
| fuel_surcharge_pct | DECIMAL(5,2) | |
| taxable | BOOLEAN | |
| effective_date | DATE | |

### charge_statements
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| period_start | DATE | |
| period_end | DATE | |
| total_amount | DECIMAL(12,2) | |
| status | VARCHAR(20) | pending, approved, paid — lifecycle: pending→approved→paid |
| expected_total | DECIMAL(12,2) | set during reconcile; variance = ABS(total_amount - expected_total) |
| approved_by | UUID FK → auth_users | nullable, set on reconcile/approve |
| variance_notes | TEXT | required if ABS(total_amount - expected_total) > $25 |
| paid_at | TIMESTAMPTZ | nullable, set when exported |
| created_at | TIMESTAMPTZ | |

### charge_line_items
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| statement_id | UUID FK → charge_statements | |
| description | TEXT | |
| quantity | DECIMAL(10,3) | |
| unit_price | DECIMAL(10,4) | |
| surcharge | DECIMAL(10,2) | |
| tax | DECIMAL(10,2) | |
| total | DECIMAL(12,2) | |

### managed_files
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| sha256 | VARCHAR(64) UNIQUE | dedup key |
| original_name | VARCHAR(255) | |
| mime_type | VARCHAR(100) | |
| size_bytes | BIGINT | |
| storage_path | VARCHAR(500) | relative to app data dir |
| retention_until | TIMESTAMPTZ | nullable |
| created_at | TIMESTAMPTZ | |

**Indexes**: idx_files_sha256

### draft_checkpoints
| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| user_id | UUID FK → auth_users | |
| form_type | VARCHAR(100) | |
| form_id | VARCHAR(100) | nullable (for edits) |
| state_json | JSONB | |
| saved_at | TIMESTAMPTZ | |

**Indexes**: idx_drafts_user_form (user_id, form_type, form_id)

### audit_log
| Column | Type | Notes |
|--------|------|-------|
| id | BIGSERIAL PK | |
| user_id | UUID FK → auth_users | |
| action | VARCHAR(100) | |
| entity_type | VARCHAR(50) | |
| entity_id | UUID | |
| details | JSONB | |
| created_at | TIMESTAMPTZ | |

**Indexes**: idx_audit_entity, idx_audit_created

## API Endpoints

All endpoints served locally at `http://localhost:<port>/api/v1`. Auth via session token stored in Electron.

### Authentication
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | /api/v1/auth/login | No | Login with username/password, returns session token |
| POST | /api/v1/auth/logout | Yes | Invalidate session |
| GET | /api/v1/auth/me | Yes | Current user profile and role |
| PUT | /api/v1/auth/password | Yes | Change own password |

### User Management (System Admin)
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /api/v1/users | Admin | List all users |
| POST | /api/v1/users | Admin | Create user with role |
| PUT | /api/v1/users/:id | Admin | Update user/role |
| DELETE | /api/v1/users/:id | Admin | Deactivate user |
| POST | /api/v1/users/:id/unlock | Admin | Manually unlock locked account |

### Inventory — SKUs
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /api/v1/skus | Inventory | List/search SKUs |
| POST | /api/v1/skus | Inventory | Create SKU |
| GET | /api/v1/skus/:id | Inventory | Get SKU detail with batches |
| PUT | /api/v1/skus/:id | Inventory | Update SKU |
| GET | /api/v1/skus/:id/batches | Inventory | List batches for SKU |
| GET | /api/v1/skus/low-stock | Inventory | List SKUs below threshold |

### Inventory — Transactions
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | /api/v1/inventory/receive | Inventory | Stock-in with batch, lot, reason code |
| POST | /api/v1/inventory/dispense | Inventory | Stock-out; enforces expiration + non-negative |
| GET | /api/v1/inventory/transactions | Inventory | List transactions with filters |
| POST | /api/v1/inventory/adjust | Inventory | Adjustment with reason code |

### Inventory — Stocktake
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | /api/v1/stocktakes | Inventory | Start new stocktake |
| GET | /api/v1/stocktakes/:id | Inventory | Get stocktake with lines |
| PUT | /api/v1/stocktakes/:id/lines | Inventory | Submit counted quantities |
| POST | /api/v1/stocktakes/:id/complete | Inventory | Finalize, compute variance/loss |

### Learning
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /api/v1/learning/subjects | Any | List subjects |
| POST | /api/v1/learning/subjects | Learning | Create subject |
| PUT | /api/v1/learning/subjects/:id | Learning | Update subject |
| GET | /api/v1/learning/chapters | Any | List chapters (filter by subject) |
| POST | /api/v1/learning/chapters | Learning | Create chapter |
| GET | /api/v1/learning/knowledge-points | Any | List/search knowledge points |
| POST | /api/v1/learning/knowledge-points | Learning | Create knowledge point |
| PUT | /api/v1/learning/knowledge-points/:id | Learning | Update knowledge point |
| GET | /api/v1/learning/search | Any | Full-text search across all content |
| POST | /api/v1/learning/import | Learning | Import Markdown/HTML/file — multipart fields: `file`, `category`, `title`, `chapter_id` all required |
| GET | /api/v1/learning/export/:id | Learning | Export knowledge point |

### Work Orders
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /api/v1/work-orders | Any | List work orders (filtered by role) |
| POST | /api/v1/work-orders | Any | Submit repair request |
| GET | /api/v1/work-orders/:id | Any | Get work order detail |
| PUT | /api/v1/work-orders/:id | Maintenance | Update status, cost, assignment |
| POST | /api/v1/work-orders/:id/close | Maintenance | Close with parts/labor cost |
| POST | /api/v1/work-orders/:id/rate | Any | Rate 1–5 after closure |
| GET | /api/v1/work-orders/analytics | Maintenance | Trend analytics data |

### Memberships
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /api/v1/members | FrontDesk | List/search members |
| POST | /api/v1/members | FrontDesk | Create member |
| GET | /api/v1/members/:id | FrontDesk | Get member detail |
| PUT | /api/v1/members/:id | FrontDesk | Update member info |
| POST | /api/v1/members/:id/freeze | FrontDesk | Freeze membership |
| POST | /api/v1/members/:id/unfreeze | FrontDesk | Unfreeze membership |
| POST | /api/v1/members/:id/redeem | FrontDesk | Redeem benefit (points/session/stored-value) |
| POST | /api/v1/members/:id/add-value | FrontDesk | Add stored value or points |
| POST | /api/v1/members/:id/refund | FrontDesk | Refund stored value (within 7 days, unused) |
| GET | /api/v1/members/:id/transactions | FrontDesk | Member transaction history |
| GET | /api/v1/membership-tiers | FrontDesk | List tiers |

### Charges & Settlement
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /api/v1/rate-tables | Admin | List rate tables |
| POST | /api/v1/rate-tables | Admin | Create/import rate table |
| PUT | /api/v1/rate-tables/:id | Admin | Update rate table |
| POST | /api/v1/rate-tables/import-csv | Admin | Import rate table from USB CSV |
| GET | /api/v1/statements | Admin | List statements |
| POST | /api/v1/statements/generate | Admin | Generate statement by date range |
| GET | /api/v1/statements/:id | Admin | Get statement with line items |
| POST | /api/v1/statements/:id/reconcile | Admin | Reconcile (`pending`→`approved`): body `{expected_total, variance_notes?}`; notes required when ABS(total−expected)>$25 |
| POST | /api/v1/statements/:id/approve | Admin | Approve alias (same `pending`→`approved` transition as reconcile) |
| POST | /api/v1/statements/:id/export | Admin | Export & mark paid (`approved`→`paid`); query `?format=csv\|json` |

### Files
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | /api/v1/files/upload | Yes | Upload file (deduped by SHA-256) |
| GET | /api/v1/files/:id | Yes | Download file |
| POST | /api/v1/files/export-zip | Yes | Export audit-ready ZIP bundle |

### System
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /api/v1/health | No | Health check |
| POST | /api/v1/system/backup | Admin | Trigger one-click backup |
| GET | /api/v1/system/backup/status | Admin | Backup status |
| POST | /api/v1/system/update | Admin | Import offline update package |
| POST | /api/v1/system/rollback | Admin | Rollback to previous version |
| GET | /api/v1/system/config | Admin | Get system configuration |
| PUT | /api/v1/system/config | Admin | Update system configuration |

### Drafts (State Recovery)
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| PUT | /api/v1/drafts/:formType | Yes | Save/update draft checkpoint |
| GET | /api/v1/drafts | Yes | List user's drafts |
| GET | /api/v1/drafts/:formType/:formId | Yes | Get specific draft |
| DELETE | /api/v1/drafts/:formType/:formId | Yes | Delete draft |

## Implied Requirements
- Error handling on all endpoints (400, 401, 403, 404, 409, 500) with structured error responses
- Input validation on all forms and API inputs (field lengths, formats, required fields)
- Loading/empty/error/success states on all frontend views
- Auth middleware on all protected routes with role checking
- Audit logging for all state-changing operations (inventory, membership, work orders, admin actions)
- Health check endpoint at /api/v1/health
- Structured logging (JSON format) to local log files with rotation
- Graceful shutdown handling for both Go service and PostgreSQL
- Database migrations managed via versioned migration files (golang-migrate or similar)
- Proper connection pooling for PostgreSQL
- Form dirty-state tracking (warn before navigating away from unsaved changes)
- Pagination on all list endpoints (cursor-based or offset/limit)
- Sortable/filterable table columns in the UI
- Backup includes both database dump and managed files directory
- Membership expiration reminders computed via a background timer in the Go service
- SLA deadline computation must account for business days (for Normal priority)
- Rate limiting is not needed (local-only, single-user desktop)
- CORS not needed (same-origin Electron)

## Scope Boundary
Do NOT build these:
- No internet/cloud connectivity or sync
- No email or SMS notifications (offline system — tray notifications only)
- No multi-site or multi-tenant support
- No payment gateway integration (charges are recorded offline)
- No barcode/QR scanning hardware integration (manual entry of NDC/UPC)
- No report generation beyond what's needed for export (CSV/JSON/ZIP)
- No real-time collaboration or multi-user conflict resolution (single desktop)
- No mobile app or responsive mobile layout
- No user self-registration (admin creates accounts)
- No LDAP/SSO/OAuth integration
- No automated dispensing device integration
- No DEA Schedule II–V controlled substance specific workflows (unless NDC implies it)
- No HL7/FHIR interoperability
- No GPS/mapping for distance — distance is manually entered or CSV imported

## Project Structure
```
repo/
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go              # Entry point, starts Echo server
│   ├── internal/
│   │   ├── config/                   # App configuration
│   │   ├── middleware/               # Auth, RBAC, logging, recovery
│   │   ├── models/                   # Domain structs
│   │   ├── handlers/                 # HTTP handlers by domain
│   │   │   ├── auth.go
│   │   │   ├── inventory.go
│   │   │   ├── learning.go
│   │   │   ├── workorders.go
│   │   │   ├── members.go
│   │   │   ├── charges.go
│   │   │   ├── files.go
│   │   │   └── system.go
│   │   ├── services/                 # Business logic layer
│   │   │   ├── auth.go
│   │   │   ├── inventory.go
│   │   │   ├── learning.go
│   │   │   ├── workorders.go
│   │   │   ├── members.go
│   │   │   ├── charges.go
│   │   │   └── files.go
│   │   ├── repository/               # Database access layer
│   │   ├── crypto/                   # Encryption helpers + OS keyring
│   │   └── scheduler/                # Background tasks (reminders, SLA checks)
│   ├── migrations/                   # SQL migration files
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── src/
│   │   ├── main/                     # Electron main process
│   │   │   ├── main.ts
│   │   │   ├── tray.ts               # System tray, lock screen
│   │   │   └── ipc.ts                # IPC bridge
│   │   ├── renderer/                 # React app
│   │   │   ├── App.tsx
│   │   │   ├── components/
│   │   │   │   ├── common/           # Tables, forms, modals, context menus
│   │   │   │   ├── inventory/
│   │   │   │   ├── learning/
│   │   │   │   ├── workorders/
│   │   │   │   ├── members/
│   │   │   │   ├── charges/
│   │   │   │   └── admin/
│   │   │   ├── hooks/                # Custom React hooks
│   │   │   ├── services/             # API client layer
│   │   │   ├── store/                # State management
│   │   │   ├── types/                # TypeScript interfaces
│   │   │   └── utils/
│   │   └── preload/                  # Electron preload scripts
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   └── electron-builder.yml
├── installer/
│   ├── postgresql/                   # Bundled PostgreSQL binaries
│   └── wix/                          # WiX installer config
├── docker-compose.yml                # For development/testing
├── run_tests.sh                      # Integration test runner
├── Makefile
└── README.md
```
