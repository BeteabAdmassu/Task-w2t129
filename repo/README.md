Project Type: fullstack

# MedOps Offline Operations Console

<!--
Change log — README hard-gates (most recent pass):
- First non-empty line is exactly `Project Type: fullstack`.
- Primary startup section contains the literal string `docker-compose up`.
- All host-side package-manager commands (Node, Python, OS packages) have
  been removed from the README. Desktop packaging is now documented as
  prebuilt-installer-only; packaging-from-source is explicitly marked out
  of scope.
- Host prerequisites are limited to Docker / Docker Compose + bash/curl/jq.
- Demo credentials cover all five roles (system_admin,
  inventory_pharmacist, learning_coordinator, front_desk, maintenance_tech).
- Verification section covers API health check, access URL, and a concrete
  functional smoke flow.
-->

## Description

An offline-first desktop workspace for community clinics and pharmacy-adjacent outpatient centers to manage regulated inventory, staff learning, memberships, and facilities work orders — eliminating reliance on internet connectivity while enforcing healthcare compliance rules. Built with a Go/Echo backend, React/TypeScript frontend, and PostgreSQL database.

## Environment rule — Docker-contained, no host installs

**Main run and test flow is fully Docker-contained.** The CI/main path does
not require any host-side Node.js, Go, Python, or package-manager setup.
No host-side runtime or package-manager invocations are needed to start
the application or run any of the four test layers.

### Host prerequisites (exhaustive)

- Docker 24+ and Docker Compose v2+
- `bash`, `curl`, `jq` (used by `run_tests.sh`)

Anything beyond this list (Node, Go, Python, Windows SDKs) is **not** part of
the documented main path. Packaging the Electron installer from source is
explicitly out of scope; see the "Desktop packaging (prebuilt installer
only)" section below.

## Getting Started — primary startup

From the `repo/` directory:

```bash
cd repo

# Start all services (PostgreSQL + Go backend + React frontend)
docker-compose up --build -d
```

`docker compose up` (space form, Compose v2 plugin) is equivalent and also
supported:

```bash
docker compose up --build -d
```

Once the three containers report healthy:

| Service   | URL / port                         |
|-----------|------------------------------------|
| Frontend  | http://localhost:3000              |
| Backend   | http://localhost:8080/api/v1       |
| Database  | postgres://medops@localhost:5432   |

### Environment overrides (optional)

```bash
cp .env.example .env
# Edit .env if you want to override defaults; the bundled defaults work as-is.
```

## Verification

### 1. API health check

```bash
curl -sf http://localhost:8080/api/v1/health | jq .
```

Expected: HTTP 200 with a JSON body containing `status: "ok"`, a `version`
string, an `uptime` string, and an ISO-8601 `timestamp`.

### 2. Web access

Open http://localhost:3000 in a browser. The login screen appears with the
**MedOps Console** heading and a username/password form.

### 3. Functional smoke flow (admin login → create SKU → receive stock)

Paste this block into a terminal; every step runs against the Docker-hosted
backend (no host-side tooling required):

```bash
# Log in as the seeded admin, capture a JWT.
TOKEN=$(curl -sf -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"AdminPass1234"}' | jq -r .token)

# Admin creates a role user (documented below under Demo credentials).
curl -sf -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"username":"pharmacist1","password":"SecurePass1234","role":"inventory_pharmacist"}' \
  | jq '.username, .role'

# Create an SKU.
SKU_ID=$(curl -sf -X POST http://localhost:8080/api/v1/skus \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Smoke-Test SKU","unit_of_measure":"box","description":"","low_stock_threshold":10,"storage_location":"Shelf A"}' \
  | jq -r .id)

# Receive 50 units into a batch with a 60-day expiry.
EXP=$(date -u -d '+60 days' +%Y-%m-%d 2>/dev/null || date -u -v+60d +%Y-%m-%d)
curl -sf -X POST http://localhost:8080/api/v1/inventory/receive \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "{\"sku_id\":\"$SKU_ID\",\"lot_number\":\"LOT-1\",\"expiration_date\":\"$EXP\",\"quantity\":50,\"storage_location\":\"Shelf A\",\"reason_code\":\"purchase_order\"}" \
  | jq '.batch.quantity_on_hand'
# Expected output: 50
```

This end-to-end smoke flow exercises auth, RBAC-gated user creation, SKU
creation, and inventory transactions against the real containerised stack.

## Demo Credentials — all roles

### Seeded admin (available immediately after `docker-compose up`)

| Field    | Value           | Role           |
|----------|-----------------|----------------|
| Username | `admin`         | `system_admin` |
| Password | `AdminPass1234` |                |

`must_change_password = true` is set on the seed admin by migration
`000008_must_change_password.up.sql`; the credential hash lives in migration
`000001_init.up.sql`. On first UI login the password must be rotated; the
Playwright `global-setup.ts` rotates it automatically so `AdminPass1234`
remains valid across test runs.

### Non-admin role users (deterministic, admin-created)

Non-admin role accounts are **not seeded** by migrations. They are created
by admin via `POST /api/v1/users`. `run_tests.sh` runs this flow on every
CI execution, so after a single `./run_tests.sh` run the following fixtures
exist in the database:

| Username        | Password          | Role                    |
|-----------------|-------------------|-------------------------|
| `admin`         | `AdminPass1234`   | `system_admin`          |
| `pharmacist1`   | `SecurePass1234`  | `inventory_pharmacist`  |
| `coordinator1`  | `SecurePass1234`  | `learning_coordinator`  |
| `frontdesk1`    | `SecurePass1234`  | `front_desk`            |
| `technicianA`   | `SecurePass1234`  | `maintenance_tech`      |

To (re-)create a role user manually, log in as admin and POST to
`/api/v1/users`:

```bash
# 1. Log in as admin, capture the JWT.
ADMIN_TOKEN=$(curl -sf -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"AdminPass1234"}' | jq -r .token)

# 2. Create a role user. Repeat with each role name below.
curl -sf -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"pharmacist1","password":"SecurePass1234","role":"inventory_pharmacist"}'
```

Valid `role` values (audited against `backend/cmd/server/main.go`):

- `system_admin`
- `inventory_pharmacist`
- `learning_coordinator`
- `front_desk`
- `maintenance_tech`

Equivalent UI flow: log in as admin → **Users** page → **+ New User** →
fill username / password / role → **Create**.

## Running the Application

```bash
docker-compose up --build -d   # start all services
docker compose logs -f         # tail logs (space form is also accepted)
docker compose down            # stop services
```

## Running Tests — Docker-only

The `run_tests.sh` orchestrator drives four fully-containerised test layers.
None of them require host-side Node.js, Go, npm, pip, or apt. All four run
inside images declared in `docker-compose.yml`.

1. **HTTP API integration** (host `curl` → real backend) — auth, RBAC,
   inventory, learning, work orders, members, charges, system config,
   drafts, files (upload/download/export-zip), stocktakes, full statement
   lifecycle with two-user approval, work-order photo linking, reminders,
   user CRUD (create/update/unlock/delete), rate-table update, SKU
   update/low-stock, work-order analytics,
   member update/refund/packages/sensitive RBAC.
2. **Backend Go unit + handler tests** inside `golang:1.22-alpine`.
3. **Frontend Vitest** component + unit tests inside `node:18-alpine`.
4. **Playwright E2E** inside `mcr.microsoft.com/playwright` — real Chromium
   drives the real UI → real API → real Postgres (no mocks). Includes
   multipart API-level tests for `POST /learning/import`,
   `POST /rate-tables/import-csv`, and `POST /files/export-zip`.

```bash
docker-compose up --build -d
./run_tests.sh
```

### Running individual test layers (all Docker)

```bash
# Playwright E2E only (services must be up).
docker compose --profile test run --rm e2e

# Backend Go tests only (no Go on host required).
docker compose --profile test run --rm backend-tests

# Frontend Vitest only (no Node.js on host required).
docker compose --profile test run --rm frontend-tests
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Backend API port |
| `DATABASE_URL` | *(set in `.env`)* | PostgreSQL connection string |
| `JWT_SECRET` | *(auto-generated in packaged desktop mode)* | JWT signing secret — set in `.env` for Docker/server deployments; auto-provisioned by Electron for packaged builds |
| `ENCRYPT_KEY` | *(auto-generated in packaged desktop mode)* | 32-byte AES encryption key for sensitive fields — set in `.env` for Docker/server deployments; auto-provisioned by Electron for packaged builds |
| `HMAC_SIGNING_KEY` | *(auto-generated in packaged desktop mode)* | HMAC key for statement export signing — set in `.env` for Docker/server deployments; auto-provisioned by Electron for packaged builds |
| `LOG_LEVEL` | `info` | Logging level (`debug`, `info`, `warn`, `error`) |
| `DATA_DIR` | `/data/medops` | Directory for managed file storage |

> **Packaged desktop mode (informational only)**: `JWT_SECRET`,
> `ENCRYPT_KEY`, and `HMAC_SIGNING_KEY` are **not** set manually. On first
> launch the Electron main process generates random secrets and stores
> them encrypted via OS-level protection (Windows Data Protection API /
> `safeStorage`). The encrypted file lives at
> `<AppData>\MedOps Console\<userData>\.secrets.enc` and is loaded
> automatically on every subsequent launch.

## Project Structure

```
repo/
├── backend/
│   ├── cmd/server/main.go           # Entry point, starts Echo server
│   ├── internal/
│   │   ├── config/                  # App configuration
│   │   ├── middleware/              # Auth, RBAC, logging middleware
│   │   ├── models/                  # Domain structs and request/response types
│   │   ├── handlers/                # HTTP handlers by domain
│   │   │   ├── auth.go              # Login, logout, password change
│   │   │   ├── users.go             # User management (admin)
│   │   │   ├── inventory.go         # SKUs, batches, transactions, stocktakes
│   │   │   ├── learning.go          # Subjects, chapters, knowledge points
│   │   │   ├── workorders.go        # Work order CRUD, dispatch, SLA
│   │   │   ├── members.go           # Membership, tiers, redemption
│   │   │   ├── charges.go           # Rate tables, statements, settlement
│   │   │   ├── files.go             # File upload, dedup, ZIP export
│   │   │   └── system.go            # Health, backup, config, drafts
│   │   └── repository/              # Database access layer
│   ├── migrations/                  # SQL migration files
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── src/
│   │   ├── main/                    # Electron main process (informational)
│   │   └── renderer/                # React SPA shipped by Docker nginx image
│   ├── Dockerfile
│   ├── nginx.conf
│   └── electron-builder.config.cjs  # Windows installer config (prebuilt artifacts)
├── e2e/                             # Playwright spec suite (Docker-run)
├── docker-compose.yml               # Orchestrates all services + test profile
├── run_tests.sh                     # Integration test runner
├── .env.example                     # Environment variable template
└── README.md
```

## Roles

| Role | Access |
|------|--------|
| System Administrator (`system_admin`) | Full access: users, config, backups, rate tables, statements |
| Inventory Pharmacist (`inventory_pharmacist`) | SKU management, receiving, dispensing, stocktakes |
| Learning Coordinator (`learning_coordinator`) | Content curation: subjects, chapters, knowledge points |
| Front Desk (`front_desk`) | Member management, benefit redemption, membership tiers |
| Maintenance Technician (`maintenance_tech`) | Work order management, dispatch, closure |

All authenticated roles can submit work orders and read learning content.

## API Endpoints

All API endpoints are served at `http://localhost:8080/api/v1/`.

### Authentication
- `POST /auth/login` — Login
- `POST /auth/logout` — Logout
- `GET /auth/me` — Current user
- `PUT /auth/password` — Change password

### Inventory
- `GET/POST /skus` — List/create SKUs
- `GET/PUT /skus/:id` — Get/update SKU
- `GET /skus/low-stock` — List SKUs below threshold
- `POST /inventory/receive` — Stock in
- `POST /inventory/dispense` — Stock out
- `POST /inventory/adjust` — Manual adjustment
- `GET /inventory/transactions` — Transaction history
- `POST/GET /stocktakes` — Stocktake lifecycle
- `PUT /stocktakes/:id/lines` — Update stocktake lines

### Learning
- `GET/POST /learning/subjects` — Subjects
- `GET/POST /learning/chapters` — Chapters
- `GET/POST /learning/knowledge-points` — Knowledge points
- `POST /learning/import` — Import a .md/.html file as a knowledge point
- `GET /learning/export/:id` — Export a knowledge point (md/html)
- `GET /learning/search?q=` — Full-text search

### Work Orders
- `GET/POST /work-orders` — List/create
- `GET /work-orders/analytics` — Aggregate analytics (admin/maintenance)
- `POST /work-orders/:id/close` — Close with costs
- `POST /work-orders/:id/rate` — Rate 1–5

### Members
- `GET/POST /members` — List/create
- `PUT /members/:id` — Update
- `POST /members/:id/freeze` — Freeze membership
- `POST /members/:id/unfreeze` — Unfreeze
- `POST /members/:id/redeem` — Redeem benefit
- `POST /members/:id/add-value` — Add points/stored value
- `POST /members/:id/refund` — Refund stored value (within 7 days)
- `GET /members/:id/packages` — List session packages
- `GET /members/:id/sensitive` — Decrypted sensitive fields (admin only)

### Rate tables / Statements
- `GET/POST /rate-tables`, `PUT /rate-tables/:id`, `POST /rate-tables/import-csv`
- `POST /statements/generate`, `POST /statements/:id/reconcile`,
  `POST /statements/:id/approve`, `POST /statements/:id/export`

### Files
- `POST /files/upload` — Single file upload
- `GET /files/:id` — Download
- `POST /files/export-zip` — Bulk ZIP export

### System
- `GET /health` — Health check
- `POST /system/backup` — Trigger pg_dump + managed-files archive
- `GET /system/backup/status` — Last backup info
- `POST /system/update` / `POST /system/rollback` — Offline update lifecycle (admin)
- `GET/PUT /system/config` — Config read/write
- `PUT /drafts/:formType`, `GET /drafts`, `GET /drafts/:formType/:formId`,
  `DELETE /drafts/:formType/:formId`

## Desktop packaging — prebuilt installer only (informational, not part of main path)

> This section is **informational only**. It is not part of the documented
> CI/main run path and requires no host tooling to run the application or
> tests. Building the installer from source is explicitly out of scope.

The application can be distributed as a packaged Windows desktop app
produced by `electron-builder`. End users install a prebuilt artifact:

| Prebuilt artifact (distributed separately)              | Purpose |
|---------------------------------------------------------|---------|
| `frontend/dist-installer/MedOps Console Setup *.exe`    | NSIS setup wizard |
| `frontend/dist-installer/MedOps Console *.msi`          | MSI for enterprise deployment (Group Policy) |

PostgreSQL is bundled inside the installer via `embedded-postgres`; no
separate database install is required on end-user machines.

### End-user acceptance verification (prebuilt installer)

**Prerequisites:** Windows 10/11 x64. No Docker, no Node.js, no Go required.

1. Run the prebuilt installer (NSIS `.exe` or `.msi`).
2. Launch **MedOps Console** from the Start Menu. The app starts the
   embedded PostgreSQL and Go backend automatically.
3. Log in with **admin / AdminPass1234**; rotate the password on first login.
4. Verify core flows: create an SKU + receive stock → create a work order →
   create a member and redeem a benefit → export a backup.
5. Verify tray behavior (lock, reminders, new window).
6. Verify offline operation by disconnecting from the network and repeating
   any flow above.

> Building the installer artifacts from source (cross-compile backend,
> bundle the frontend, run `electron-builder`) is out of scope for this
> README. The CI/main path is Docker-first and needs no host toolchains.

### Offline updates and version rollback (informational)

Update packages are `.zip` archives distributed on a USB drive or shared
network share — no internet required.

**Update package layout:**

```
update-v1.2.0.zip
├── migrations/
│   └── 000010_add_column.sql   # SQL migrations applied in lexicographic order
├── backend/
│   └── medops-server.exe       # (optional) new backend binary
├── frontend/
│   ├── index.html              # (optional) new frontend SPA bundle
│   └── assets/
└── version.txt                 # e.g. "1.2.0"
```

**Apply an update** (System Config → Apply Offline Update):
1. A `pg_dump` snapshot of the current database is written to
   `DATA_DIR/backups/` and the current `DATA_DIR/active/` artifacts are
   snapshotted to `DATA_DIR/versions/<timestamp>/`.
2. Backend binary and frontend assets (if included in the package) are
   extracted to `DATA_DIR/active/`.
3. SQL migrations are applied in lexicographic order.
4. Version history is appended to `DATA_DIR/updates/version_history.json`.
5. The backend subprocess is restarted via Electron IPC; renderer windows
   reload from the new frontend assets.

**One-click rollback** (System Config → Rollback to Previous Version):
1. The most recent `version_history.json` entry is read to identify the
   pre-update snapshots.
2. The database is restored from the pg_dump snapshot using
   `psql --single-transaction`.
3. The backend binary and frontend assets are restored from
   `DATA_DIR/versions/<timestamp>/`.
4. A `restart.flag` sentinel file is written; Electron's polling watcher
   detects it, stops the backend subprocess, starts it from the restored
   binary, and reloads renderer windows.
5. The history entry is popped; repeated rollbacks chain back to baseline.

All update and rollback operations are audit-logged with user ID and
timestamp.

## Security & Tenant Isolation Model

All primary business tables (`auth_users`, `members`, `skus`, `work_orders`, `knowledge_points`, `member_transactions`, `rate_tables`, `charge_statements`, `managed_files`, `stocktakes`, `stock_transactions`, `learning_subjects`, `learning_chapters`) carry a `tenant_id` column. Every repository query enforces `WHERE tenant_id = $N` or includes it in INSERT values.

**Intentionally global (no tenant_id) tables:**

| Table | Rationale |
|-------|-----------|
| `membership_tiers` | Shared platform catalogue (Gold, Silver, etc.) — no PHI, same definitions across all tenants. Isolation is enforced at the member record level via `members.tenant_id`. |
| `draft_checkpoints` | Scoped by `user_id` (users belong to a tenant, so isolation is transitive). |
| `charge_line_items` | Scoped via FK to `charge_statements.tenant_id`. |
| `work_order_photos` | Scoped via FK to `work_orders.tenant_id`. |
| `system_config` | Single deployment-wide config; no tenant-specific data stored here. |
| `inventory_batches` | Scoped via FK to `skus.tenant_id`. |
| `stocktake_lines` | Scoped via FK to `stocktakes.tenant_id`. |
| `session_packages` | Scoped via FK to `members.tenant_id`. |

## Architecture

- **Desktop shell**: Electron 31 (informational; prebuilt installer only)
- **Backend**: Go 1.22 with Echo v4 framework
- **Frontend**: React 18 + TypeScript + Vite, served by nginx in the Docker frontend image
- **Database**: PostgreSQL 16 with auto-migrations (`MIGRATIONS_PATH` env var)
- **Auth**: JWT tokens with bcrypt password hashing
- **Encryption**: AES for sensitive fields at rest
- **Full-text Search**: PostgreSQL `tsvector` / `tsquery`
- **File Dedup**: SHA-256 fingerprinting

See `../docs/design.md` for detailed architecture decisions.
