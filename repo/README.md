# MedOps Offline Operations Console

## Description

An offline-first desktop workspace for community clinics and pharmacy-adjacent outpatient centers to manage regulated inventory, staff learning, memberships, and facilities work orders — eliminating reliance on internet connectivity while enforcing healthcare compliance rules. Built with a Go/Echo backend, React/TypeScript frontend, and PostgreSQL database.

## Prerequisites

### Docker / CI mode
- Docker 24+ and Docker Compose v2+
- bash, curl, jq (for running integration tests)

### Desktop / Electron mode (Windows installer)
- Go 1.22+ (to cross-compile the backend binary)
- Node.js 18+ and npm
- PostgreSQL is bundled automatically (no separate installation required)
- Windows 10/11 x64 (for the MSI/NSIS installer target)

## Getting Started

### Quick Start with Docker

```bash
# Clone and enter the repo directory
cd repo

# Start all services
docker compose up --build -d

# Wait for services to be healthy (backend and frontend)
# Backend: http://localhost:8080/api/v1/health
# Frontend: http://localhost:3000
```

### Environment Setup

```bash
cp .env.example .env
# Edit .env with your configuration (optional — defaults work out of the box)
```

### Default Login Credentials

The seed admin account is created automatically by database migration `000001_init.up.sql`. The default credentials are:

| Field    | Value          |
|----------|----------------|
| Username | `admin`        |
| Password | `AdminPass1234`|

**The application forces a password change on first login.** The seed credential hash is embedded in `000001_init.up.sql`; the `must_change_password = true` flag is set by migration `000008_must_change_password.up.sql`. No environment variable is needed.

### Running the Application

```bash
# Start all services (PostgreSQL, backend, frontend)
docker compose up --build -d

# View logs
docker compose logs -f

# Stop services
docker compose down
```

### Running Tests

The `run_tests.sh` orchestrator drives three layers of tests — all fully containerised:

1. **HTTP API integration** (host `curl` → real backend) covering auth, RBAC, inventory, learning, work orders, members, charges, system config, **drafts, file upload/download, stocktakes, full statement lifecycle with two-user approval, work-order photo linking, reminders**
2. **Backend Go unit + handler tests** executed inside a `golang:1.22-alpine` throwaway container (no Go needed on host)
3. **Playwright end-to-end** tests executed inside `mcr.microsoft.com/playwright` — a real Chromium drives the real UI, which makes real API calls, which write to the real database (no mocks)

```bash
# Start the services
docker compose up --build -d

# Run the full test suite (API + Go + Playwright E2E, all inside Docker)
./run_tests.sh
```

Host dependencies: only `docker`, `docker compose`, `bash`, `curl`, and `jq`.

### Running individual test layers

**Playwright E2E only** (services must be up):
```bash
docker compose --profile test run --rm e2e
```

**Backend Go tests only** (no Go on host required):
```bash
docker compose --profile test run --rm backend-tests
```

**Frontend Vitest** (requires Node.js 18+, for local dev):
```bash
cd frontend && npm test
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Backend API port |
| `DATABASE_URL` | *(set in .env)* | PostgreSQL connection string |
| `JWT_SECRET` | *(auto-generated in desktop mode)* | JWT signing secret — set in `.env` for Docker/server deployments; auto-provisioned by Electron for packaged builds |
| `ENCRYPT_KEY` | *(auto-generated in desktop mode)* | 32-byte AES encryption key for sensitive fields — set in `.env` for Docker/server deployments; auto-provisioned by Electron for packaged builds |
| `HMAC_SIGNING_KEY` | *(auto-generated in desktop mode)* | HMAC key for statement export signing — set in `.env` for Docker/server deployments; auto-provisioned by Electron for packaged builds |
| `LOG_LEVEL` | `info` | Logging level (debug, info, warn, error) |
| `DATA_DIR` | `/data/medops` | Directory for managed file storage |

> **Desktop mode (Electron packaged build)**: `JWT_SECRET`, `ENCRYPT_KEY`, and `HMAC_SIGNING_KEY` are **not** set manually. On first launch the Electron main process generates cryptographically random secrets and stores them encrypted via OS-level protection (Windows Data Protection API / `safeStorage`). The encrypted file lives at `<AppData>\MedOps Console\<userData>\.secrets.enc`. These secrets are loaded automatically on every subsequent launch and injected into the backend process environment — no manual configuration is required.

## Project Structure

```
repo/
├── backend/
│   ├── cmd/server/main.go           # Entry point, starts Echo server
│   ├── internal/
│   │   ├── config/                   # App configuration
│   │   ├── middleware/               # Auth, RBAC, logging middleware
│   │   ├── models/                   # Domain structs and request/response types
│   │   ├── handlers/                 # HTTP handlers by domain
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
│   ├── migrations/                   # SQL migration files
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── src/
│   │   ├── main/                    # Electron main process
│   │   │   ├── main.ts              # Window management, backend spawn, IPC
│   │   │   ├── preload.ts           # Context bridge (renderer ↔ main IPC)
│   │   │   └── tray.ts              # System tray, lock, reminders
│   │   └── renderer/
│   │       ├── App.tsx              # Router and app shell
│   │       ├── main.tsx             # React entry point
│   │       ├── components/
│   │       │   ├── common/          # DataTable, Modal, Pagination, etc.
│   │       │   ├── admin/           # Login, Dashboard, Users pages
│   │       │   ├── inventory/       # SKU list, detail, stocktake
│   │       │   ├── learning/        # Subject/chapter/knowledge point mgmt
│   │       │   ├── workorders/      # Work order list and detail
│   │       │   ├── members/         # Member list, detail, redemption
│   │       │   └── charges/         # Rate tables, statements
│   │       ├── hooks/               # useAuth, useFetch
│   │       ├── services/api.ts      # API client
│   │       └── types/               # TypeScript interfaces
│   ├── Dockerfile
│   ├── nginx.conf
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   └── electron-builder.config.cjs  # Windows installer config
├── scripts/
│   └── build-backend-win.sh         # Cross-compile Go for Electron packaging
├── docker-compose.yml               # Orchestrates all services
├── run_tests.sh                     # Integration test runner
├── .env.example                     # Environment variable template
└── README.md
```

## Roles

| Role | Access |
|------|--------|
| System Administrator | Full access: users, config, backups, rate tables, statements |
| Inventory Pharmacist | SKU management, receiving, dispensing, stocktakes |
| Learning Coordinator | Content curation: subjects, chapters, knowledge points |
| Front Desk | Member management, benefit redemption, membership tiers |
| Maintenance Technician | Work order management, dispatch, closure |

All roles can submit work orders and view learning content.

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
- `POST /inventory/receive` — Stock in
- `POST /inventory/dispense` — Stock out
- `GET /inventory/transactions` — Transaction history
- `POST /stocktakes` — Create stocktake

### Learning
- `GET/POST /learning/subjects` — Subjects
- `GET/POST /learning/chapters` — Chapters
- `GET/POST /learning/knowledge-points` — Knowledge points
- `GET /learning/search?q=` — Full-text search

### Work Orders
- `GET/POST /work-orders` — List/create
- `POST /work-orders/:id/close` — Close with costs
- `POST /work-orders/:id/rate` — Rate 1-5

### Members
- `GET/POST /members` — List/create
- `POST /members/:id/freeze` — Freeze membership
- `POST /members/:id/unfreeze` — Unfreeze
- `POST /members/:id/redeem` — Redeem benefit
- `POST /members/:id/add-value` — Add points/stored value

### System
- `GET /health` — Health check (returns `status`, `version`, `uptime`, `timestamp`)
- `POST /system/backup` — Trigger database backup (pg_dump to DATA_DIR/backups/)
- `GET /system/backup/status` — Last backup file info
- `POST /system/update` — Import offline update package (.zip or .sql); installs SQL migrations and optional backend/frontend artifacts; returns `version`, `status`, `migrations`, `restart_required`
- `POST /system/rollback` — Roll back to the previous installed version: restores database from pre-update pg_dump snapshot **and** restores backend binary + frontend assets from artifact snapshot; writes a restart flag so Electron automatically stops/starts the backend subprocess; returns `version`, `status`, `artifacts_restored`, `restart_required`, `rolled_back_at`
- `GET /system/config` — Get system configuration key-value pairs
- `PUT /system/config` — Update a single config key (`{ key, value }`)
- `PUT /drafts/:formType` — Save a form draft checkpoint (auto-saved every 30 s)
- `GET /drafts` — List all drafts for the authenticated user
- `GET /drafts/:formType/:formId` — Get a specific draft
- `DELETE /drafts/:formType/:formId` — Delete a specific draft

## Desktop (Electron) Build

The application ships as a native Windows desktop app via Electron. The Electron shell wraps the Go backend (which it starts as a subprocess) and the React SPA (loaded from `file://`).

### Building the Windows installer

```bash
# 1. Cross-compile the Go backend for Windows
bash scripts/build-backend-win.sh

# 2. Install frontend dependencies
cd frontend && npm install

# 3. Build and package (produces NSIS .exe and MSI in frontend/dist-installer/)
npm run dist:win
```

### Running the desktop app in development mode

Two terminals are required — one for the Vite renderer dev server, one for the Electron shell.

**Terminal 1 — Vite renderer dev server (port 3000):**
```bash
cd frontend
npm install
npm run dev
```

**Terminal 2 — Electron shell + backend (once Terminal 1 is ready):**
```bash
# Start PostgreSQL and backend via Docker
docker compose up -d db backend

# Start Electron (connects to Vite dev server on localhost:3000)
cd frontend
npm run electron:dev
```

> `npm run electron:dev` builds the Electron main/preload scripts then launches
> Electron. In dev mode the renderer window loads `http://localhost:3000` (Vite)
> so hot-module reload works. Set `VITE_DEV_URL` env var to override the default
> dev server address.

### Desktop features

- **System tray**: minimize to tray, lock screen, configurable reminders (15 / 30 / 60 min)
- **Multi-window**: open additional workspace windows via tray menu or `Ctrl+N`
- **Keyboard shortcuts**:
  - `Ctrl+K` — command palette (navigate to any section)
  - `Ctrl+N` — dispatch `medops:create-new` event → opens the create modal on the active page
  - `Ctrl+Enter` — submit the currently focused form
  - `F2` — dispatch `medops:edit-row` event → opens the edit modal for the first row on the active page (UsersPage opens the Edit Role modal; other pages implement their own listener)
  - `Alt+D/U/S/K/L/M/W/R/T/Y` — jump to Dashboard / Users / SKUs / Stocktakes / Learning / Members / Work Orders / Rate Tables / Statements / System Config
  - `Ctrl+L` (tray) — lock all windows
- **Offline operation**: backend runs locally, no internet required
- **Single-instance**: second launch focuses the existing window

### Installer files

| File | Description |
|------|-------------|
| `frontend/dist-installer/MedOps Console Setup *.exe` | NSIS setup wizard |
| `frontend/dist-installer/MedOps Console *.msi` | MSI for enterprise deployment (Group Policy) |

PostgreSQL is bundled automatically via embedded-postgres — no separate installation is required.

### Desktop Acceptance Verification (packaged app — no Docker required)

This path verifies the fully packaged desktop application, independent of Docker or any development toolchain.

**Prerequisites:** Windows 10/11 x64, no Docker, no Node.js, no Go required.

**Steps:**

1. Run the installer from `frontend/dist-installer/MedOps Console Setup *.exe` (or the `.msi`).
2. Launch **MedOps Console** from the Start Menu or Desktop shortcut.
3. The app starts the embedded PostgreSQL and Go backend automatically. Wait for the main window to appear (typically a few seconds).
4. Log in with the default credentials: **admin / AdminPass1234**. You will be prompted to change your password on first login.
5. Verify core flows:
   - **Inventory** — create a SKU, receive stock, dispense stock.
   - **Work Orders** — submit a work order, update status to dispatched, close it.
   - **Members** — create a member, redeem a benefit, freeze and unfreeze.
   - **Learning** — create a subject, chapter, and knowledge point; export the knowledge point as Markdown.
   - **Backup** — navigate to System Config → Backup, trigger a backup, confirm the `.sql` file and managed-files `.zip` are listed.
6. Verify tray behavior: minimize the window; the system tray icon should be visible. Right-click the tray icon to access Lock, Reminders, and New Window.
7. Verify offline operation: disconnect from the network and repeat any of the above flows — all functions should work without internet access.

> No Docker commands are needed for this path. The Docker setup in the sections above is for development/CI use only.

### Offline updates and version rollback

Update packages are `.zip` archives distributed on a USB drive or shared network share — no internet required.

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
1. Before any changes: a `pg_dump` snapshot of the current database is written to `DATA_DIR/backups/` and the current `DATA_DIR/active/` artifacts (binary + frontend) are snapshotted to `DATA_DIR/versions/<timestamp>/`.
2. Backend binary and frontend assets (if included in the package) are extracted to `DATA_DIR/active/`.
3. SQL migrations are applied in lexicographic order.
4. Version history is appended to `DATA_DIR/updates/version_history.json`.
5. The backend subprocess is restarted via Electron IPC; renderer windows reload from the new frontend assets.

**One-click rollback** (System Config → Rollback to Previous Version):
1. The most recent `version_history.json` entry is read to identify the pre-update snapshots.
2. The database is restored from the pg_dump snapshot using `psql --single-transaction`.
3. The backend binary and frontend assets are restored from `DATA_DIR/versions/<timestamp>/`.
4. A `restart.flag` sentinel file is written; Electron's polling watcher detects it, stops the backend subprocess, starts it from the restored binary, and reloads renderer windows.
5. The history entry is popped; repeated rollbacks chain back to baseline.

All update and rollback operations are audit-logged with user ID and timestamp.

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

- **Desktop shell**: Electron 31 (main + preload + renderer)
- **Backend**: Go 1.22 with Echo v4 framework, runs as a subprocess in packaged app
- **Frontend**: React 18 + TypeScript + Vite
- **Database**: PostgreSQL 16 with auto-migrations (`MIGRATIONS_PATH` env var)
- **Auth**: JWT tokens with bcrypt password hashing
- **Encryption**: AES for sensitive fields at rest
- **Full-text Search**: PostgreSQL tsvector/tsquery
- **File Dedup**: SHA-256 fingerprinting
- **Packaging**: electron-builder (NSIS + MSI targets)

See `../docs/design.md` for detailed architecture decisions.
