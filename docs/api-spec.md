# API Specification

Base URL: `http://localhost:{PORT}/api/v1`

## Authentication

Local session-based authentication. Login returns a session token sent as `Authorization: Bearer <token>` header on subsequent requests.

- Passwords: minimum 12 characters
- Lockout: 5 failed attempts → 15-minute lockout
- Sessions are stored server-side; token is an opaque random string

## Error Response Format
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Human-readable message",
    "details": {}
  }
}
```

Standard error codes:
- `VALIDATION_ERROR` (400) — invalid input
- `UNAUTHORIZED` (401) — not logged in or session expired
- `FORBIDDEN` (403) — role lacks permission
- `NOT_FOUND` (404) — resource doesn't exist
- `CONFLICT` (409) — duplicate or state conflict
- `ACCOUNT_LOCKED` (423) — too many failed login attempts
- `INTERNAL_ERROR` (500) — unexpected server error

## Pagination

All list endpoints accept:
- `page` (int, default 1)
- `page_size` (int, default 25, max 100)

Response includes:
```json
{
  "data": [...],
  "pagination": {
    "page": 1,
    "page_size": 25,
    "total_items": 142,
    "total_pages": 6
  }
}
```

## Endpoints

### Auth

| Method | Path | Auth | Description | Request Body | Response | Errors |
|--------|------|------|-------------|-------------|----------|--------|
| POST | /auth/login | No | Login | `{username, password}` | `{token, user}` | 401, 423 |
| POST | /auth/logout | Yes | Logout | — | 204 | 401 |
| GET | /auth/me | Yes | Current user | — | `{user}` | 401 |
| PUT | /auth/password | Yes | Change password | `{current_password, new_password}` | 204 | 400, 401 |

### Users (Admin only)

| Method | Path | Auth | Description | Request Body | Response | Errors |
|--------|------|------|-------------|-------------|----------|--------|
| GET | /users | Admin | List users | — | `{data[], pagination}` | 403 |
| POST | /users | Admin | Create user | `{username, password, role}` | `{user}` | 400, 409 |
| PUT | /users/:id | Admin | Update user | `{username?, role?, is_active?}` | `{user}` | 400, 404 |
| DELETE | /users/:id | Admin | Deactivate | — | 204 | 404 |
| POST | /users/:id/unlock | Admin | Unlock account | — | 204 | 404 |

### Inventory — SKUs

| Method | Path | Auth | Description | Request Body | Response | Errors |
|--------|------|------|-------------|-------------|----------|--------|
| GET | /skus | Inventory | List/search SKUs | query: `?q=&ndc=&upc=` | `{data[], pagination}` | — |
| POST | /skus | Inventory | Create SKU | `{ndc?, upc?, name, unit_of_measure, low_stock_threshold, storage_location}` | `{sku}` | 400, 409 |
| GET | /skus/:id | Inventory | SKU detail + batches | — | `{sku, batches[]}` | 404 |
| PUT | /skus/:id | Inventory | Update SKU | partial fields | `{sku}` | 400, 404 |
| GET | /skus/low-stock | Inventory | Below-threshold SKUs | — | `{data[]}` | — |

### Inventory — Transactions

| Method | Path | Auth | Description | Request Body | Response | Errors |
|--------|------|------|-------------|-------------|----------|--------|
| POST | /inventory/receive | Inventory | Stock in | `{sku_id, lot_number, expiration_date, quantity, storage_location, reason_code}` | `{transaction, batch}` | 400 |
| POST | /inventory/dispense | Inventory | Stock out | `{sku_id, batch_id, quantity, reason_code, prescription_id?}` | `{transaction}` | 400 (expired/insufficient) |
| GET | /inventory/transactions | Inventory | List with filters | query: `?sku_id=&type=&from=&to=` | `{data[], pagination}` | — |
| POST | /inventory/adjust | Inventory | Adjustment | `{sku_id, batch_id, quantity, reason_code}` | `{transaction}` | 400 |

### Inventory — Stocktake

| Method | Path | Auth | Description | Request Body | Response | Errors |
|--------|------|------|-------------|-------------|----------|--------|
| POST | /stocktakes | Inventory | Start stocktake | `{period_start, period_end}` | `{stocktake}` | 400 |
| GET | /stocktakes/:id | Inventory | Get with lines | — | `{stocktake, lines[]}` | 404 |
| PUT | /stocktakes/:id/lines | Inventory | Submit counts | `{lines: [{sku_id, batch_id, counted_qty}]}` | `{lines[]}` | 400 |
| POST | /stocktakes/:id/complete | Inventory | Finalize | — | `{stocktake, variance_summary}` | 400, 404 |

### Learning

| Method | Path | Auth | Description | Request Body | Response | Errors |
|--------|------|------|-------------|-------------|----------|--------|
| GET | /learning/subjects | Any | List subjects | — | `{data[]}` | — |
| POST | /learning/subjects | Learning | Create | `{name, description}` | `{subject}` | 400 |
| PUT | /learning/subjects/:id | Learning | Update | partial fields | `{subject}` | 404 |
| GET | /learning/chapters | Any | List (filter by subject) | query: `?subject_id=` | `{data[]}` | — |
| POST | /learning/chapters | Learning | Create | `{subject_id, name}` | `{chapter}` | 400 |
| GET | /learning/knowledge-points | Any | List/search | query: `?chapter_id=&tag=` | `{data[], pagination}` | — |
| POST | /learning/knowledge-points | Learning | Create | `{chapter_id, title, content, tags[], classifications}` | `{knowledge_point}` | 400 |
| PUT | /learning/knowledge-points/:id | Learning | Update | partial fields | `{knowledge_point}` | 404 |
| GET | /learning/search | Any | Full-text search | query: `?q=` | `{data[], pagination}` | — |
| POST | /learning/import | Learning | Import file | multipart: `file` (required), `category` (required), `title` (required), `chapter_id` (required) | `{knowledge_point}` | 400 |
| GET | /learning/export/:id | Learning | Export | query: `?format=md|html` | file download | 404 |

### Work Orders

| Method | Path | Auth | Description | Request Body | Response | Errors |
|--------|------|------|-------------|-------------|----------|--------|
| GET | /work-orders | Any | List (role-filtered) | query: `?status=&priority=&assigned_to=` | `{data[], pagination}` | — |
| POST | /work-orders | Any | Submit request | `{description, location, priority, trade, photos[]}` | `{work_order}` | 400 |
| GET | /work-orders/:id | Any | Detail | — | `{work_order, photos[]}` | 404 |
| PUT | /work-orders/:id | Maintenance | Update | partial fields | `{work_order}` | 400, 404 |
| POST | /work-orders/:id/close | Maintenance | Close | `{parts_cost, labor_cost, notes}` | `{work_order}` | 400 |
| POST | /work-orders/:id/rate | Any | Rate | `{rating}` (1-5) | `{work_order}` | 400 |
| GET | /work-orders/analytics | Maintenance | Trends | query: `?from=&to=` | `{analytics}` | — |

### Memberships

| Method | Path | Auth | Description | Request Body | Response | Errors |
|--------|------|------|-------------|-------------|----------|--------|
| GET | /members | FrontDesk | List/search | query: `?q=&tier=&status=` | `{data[], pagination}` | — |
| POST | /members | FrontDesk | Create | `{name, phone, tier_id, expires_at}` | `{member}` | 400 |
| GET | /members/:id | FrontDesk | Detail | — | `{member, packages[], recent_transactions[]}` | 404 |
| PUT | /members/:id | FrontDesk | Update | partial fields | `{member}` | 400, 404 |
| POST | /members/:id/freeze | FrontDesk | Freeze | — | `{member}` | 400 (already frozen) |
| POST | /members/:id/unfreeze | FrontDesk | Unfreeze | — | `{member}` | 400 (not frozen) |
| POST | /members/:id/redeem | FrontDesk | Redeem benefit | `{type, package_id?, amount?}` | `{transaction}` | 400 (expired/frozen/insufficient) |
| POST | /members/:id/add-value | FrontDesk | Top up | `{type, amount}` | `{transaction}` | 400 |
| POST | /members/:id/refund | FrontDesk | Refund stored value | `{deposit_id, amount}` | `{transaction}` | 400 (>7 days/used) |
| GET | /members/:id/transactions | FrontDesk | History | — | `{data[], pagination}` | 404 |
| GET | /membership-tiers | FrontDesk | List tiers | — | `{data[]}` | — |

### Charges & Settlement

| Method | Path | Auth | Description | Request Body | Response | Errors |
|--------|------|------|-------------|-------------|----------|--------|
| GET | /rate-tables | Admin | List | — | `{data[]}` | — |
| POST | /rate-tables | Admin | Create | `{name, type, tiers, fuel_surcharge_pct, taxable}` | `{rate_table}` | 400 |
| PUT | /rate-tables/:id | Admin | Update | partial fields | `{rate_table}` | 404 |
| POST | /rate-tables/import-csv | Admin | Import from CSV | multipart: file | `{rate_table}` | 400 |
| GET | /statements | Admin | List | query: `?status=&from=&to=` | `{data[], pagination}` | — |
| POST | /statements/generate | Admin | Generate | `{period_start, period_end}` | `{statement}` | 400 |
| GET | /statements/:id | Admin | Detail + lines | — | `{statement, line_items[]}` | 404 |
| POST | /statements/:id/reconcile | Admin | Reconcile & approve (`pending`→`approved`) | `{expected_total, variance_notes?}` — `variance_notes` required if ABS(total-expected)>$25 | `{statement}` | 400 |
| POST | /statements/:id/approve | Admin | Approve (alias, same state transition as reconcile) | — | `{statement}` | 400 |
| POST | /statements/:id/export | Admin | Export & mark paid (`approved`→`paid`) | query: `?format=csv\|json` | file download | 400 (if not `approved`) |

### Files

| Method | Path | Auth | Description | Request Body | Response | Errors |
|--------|------|------|-------------|-------------|----------|--------|
| POST | /files/upload | Yes | Upload (SHA-256 dedup) | multipart: file | `{file}` | 400 |
| GET | /files/:id | Yes | Download | — | file stream | 404 |
| POST | /files/export-zip | Yes | Audit ZIP bundle | `{file_ids[]}` | ZIP download | 400 |

### System

| Method | Path | Auth | Description | Request Body | Response | Errors |
|--------|------|------|-------------|-------------|----------|--------|
| GET | /health | No | Health check | — | `{status, version, uptime}` | — |
| POST | /system/backup | Admin | Trigger backup | — | `{backup_id, status}` | 500 |
| GET | /system/backup/status | Admin | Backup progress | — | `{status, last_backup_at}` | — |
| POST | /system/update | Admin | Import update pkg | multipart: file | `{version, status}` | 400 |
| POST | /system/rollback | Admin | Rollback | — | `{version, status}` | 400 (no previous) |
| GET | /system/config | Admin | Get config | — | `{config}` | — |
| PUT | /system/config | Admin | Update config | `{key: value, ...}` | `{config}` | 400 |

### Drafts

| Method | Path | Auth | Description | Request Body | Response | Errors |
|--------|------|------|-------------|-------------|----------|--------|
| PUT | /drafts/:formType | Yes | Save/upsert checkpoint — `formType` comes from the URL path, not the body | `{form_id?, state_json}` | `{draft}` | 400 |
| GET | /drafts | Yes | List user's drafts | — | `{data[]}` | — |
| GET | /drafts/:formType/:formId | Yes | Get draft by (user, formType, formId) — `formId` is a user-defined key, not a DB UUID | — | `{draft}` | 404 |
| DELETE | /drafts/:formType/:formId | Yes | Delete draft by (user, formType, formId) | — | `{message}` | 500 |
