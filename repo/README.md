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

The seed admin account is created on first startup. Credentials are configured via the `ADMIN_PASSWORD` environment variable (see `.env.example`). Do **not** expose default credentials in production — change them immediately after first login.

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

```bash
# Make sure services are running first
docker compose up --build -d

# Run integration tests from the host
./run_tests.sh
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Backend API port |
| `DATABASE_URL` | *(set in .env)* | PostgreSQL connection string |
| `JWT_SECRET` | *(required — set in .env)* | JWT signing secret — **must be overridden in production** |
| `ENCRYPT_KEY` | *(required — set in .env)* | 32-byte AES encryption key for sensitive fields — **must be overridden in production** |
| `HMAC_SIGNING_KEY` | *(required — set in .env)* | HMAC key for statement export signing — **must be overridden in production** |
| `LOG_LEVEL` | `info` | Logging level (debug, info, warn, error) |
| `DATA_DIR` | `/data/medops` | Directory for managed file storage |

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
- `GET /health` — Health check
- `POST /system/backup` — Trigger backup

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

```bash
# Start PostgreSQL and backend via Docker (or run manually)
docker compose up -d db backend

# Launch Electron dev mode (connects to the running backend)
cd frontend
npm install
npm run electron:dev
```

### Desktop features

- **System tray**: minimize to tray, lock screen, configurable reminders (15 / 30 / 60 min)
- **Multi-window**: open additional workspace windows via tray menu or `Ctrl+N`
- **Keyboard shortcuts**:
  - `Ctrl+K` — command palette (navigate to any section)
  - `Ctrl+N` — open new window / navigate to list
  - `Ctrl+Enter` — submit active form
  - `F2` — focus first input on page
  - `Alt+D/U/S/K/L/M/W/R/T` — jump to section
  - `Ctrl+L` (tray) — lock all windows
- **Offline operation**: backend runs locally, no internet required
- **Single-instance**: second launch focuses the existing window

### Installer files

| File | Description |
|------|-------------|
| `frontend/dist-installer/MedOps Console Setup *.exe` | NSIS setup wizard |
| `frontend/dist-installer/MedOps Console *.msi` | MSI for enterprise deployment (Group Policy) |

PostgreSQL is bundled automatically via embedded-postgres — no separate installation is required.

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
