# Test Coverage Audit

## Project Type Detection
- README top declaration exists: `Project Type: fullstack` (`repo/README.md:1`).
- Effective project type used for audit: **fullstack**.

## Backend Endpoint Inventory
Source: `repo/backend/cmd/server/main.go:136-262`.

Resolved API prefix: `/api/v1`

Total unique endpoints (method + fully resolved path): **84**

### API Test Mapping Table

| Endpoint | Covered | Test type | Test files | Evidence |
|---|---|---|---|---|
| GET /api/v1/health | yes | true no-mock HTTP | `repo/run_tests.sh` | health check block `run_tests.sh:80-87` |
| POST /api/v1/auth/login | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/auth.spec.ts` | `run_tests.sh:94-102`; `auth.spec.ts` test `admin can log in and land on dashboard` |
| POST /api/v1/auth/logout | yes | true no-mock HTTP | `repo/run_tests.sh` | logout check `run_tests.sh:111` |
| GET /api/v1/auth/me | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/work-orders.spec.ts` | `run_tests.sh:115-120`; API call in work-orders spec |
| PUT /api/v1/auth/password | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/auth.spec.ts` | `run_tests.sh:127-154`; test `new user: create via API, login, and rotate password` |
| GET /api/v1/users | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/auth.spec.ts` | `run_tests.sh:208`; RBAC API check in `auth.spec.ts:91-95` |
| POST /api/v1/users | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/auth.spec.ts` | `run_tests.sh:175`; `auth.spec.ts:135-138` |
| PUT /api/v1/users/:id | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1181-1199` |
| DELETE /api/v1/users/:id | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1210-1226` |
| POST /api/v1/users/:id/unlock | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1202-1208` |
| GET /api/v1/skus | yes | true no-mock HTTP | `repo/run_tests.sh` | `run_tests.sh:269` |
| POST /api/v1/skus | yes | true no-mock HTTP | `repo/run_tests.sh` | `run_tests.sh:240`; additional seeds in coverage section |
| GET /api/v1/skus/low-stock | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1260-1268` |
| GET /api/v1/skus/:id | yes | true no-mock HTTP | `repo/run_tests.sh` | `run_tests.sh:261` |
| PUT /api/v1/skus/:id | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1241-1257` |
| GET /api/v1/skus/:id/batches | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/inventory.spec.ts` | `run_tests.sh:1298`; inventory spec uses `/skus/${skuId}/batches` |
| POST /api/v1/inventory/receive | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/inventory.spec.ts` | `run_tests.sh:282`; inventory spec receive tests |
| POST /api/v1/inventory/dispense | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/inventory.spec.ts` | `run_tests.sh:303`; inventory spec dispense tests |
| GET /api/v1/inventory/transactions | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/inventory.spec.ts` | `run_tests.sh:323`; inventory spec reads transactions |
| POST /api/v1/inventory/adjust | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1283-1315` |
| GET /api/v1/stocktakes | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/inventory.spec.ts` | `run_tests.sh:919`; inventory spec list test |
| POST /api/v1/stocktakes | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/inventory.spec.ts` | `run_tests.sh:880`; inventory spec create test |
| GET /api/v1/stocktakes/:id | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/inventory.spec.ts` | `run_tests.sh:893`; inventory spec get test |
| PUT /api/v1/stocktakes/:id/lines | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1318-1347` |
| POST /api/v1/stocktakes/:id/complete | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/inventory.spec.ts` | `run_tests.sh:902`; inventory spec complete test |
| GET /api/v1/learning/subjects | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1363-1370` |
| POST /api/v1/learning/subjects | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:345`; system spec seed subject |
| PUT /api/v1/learning/subjects/:id | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1390-1400` |
| GET /api/v1/learning/chapters | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1372-1379` |
| POST /api/v1/learning/chapters | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:356`; system spec seed chapter |
| GET /api/v1/learning/knowledge-points | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1381-1388` |
| POST /api/v1/learning/knowledge-points | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:367`; system spec seed KP |
| PUT /api/v1/learning/knowledge-points/:id | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1402-1420` |
| GET /api/v1/learning/search | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:378`; system spec search test |
| POST /api/v1/learning/import | yes | true no-mock HTTP | `repo/e2e/tests/endpoints-coverage.spec.ts` | describe `POST /learning/import` + multipart tests |
| GET /api/v1/learning/export/:id | yes | true no-mock HTTP | `repo/e2e/tests/system.spec.ts` | system spec export markdown API call |
| GET /api/v1/work-orders | yes | true no-mock HTTP | `repo/e2e/tests/work-orders.spec.ts`, `repo/run_tests.sh` | work-orders spec list call; run_tests coverage throughout WO flow |
| POST /api/v1/work-orders | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/work-orders.spec.ts` | `run_tests.sh:396`; work-orders spec create tests |
| GET /api/v1/work-orders/analytics | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1426-1443` |
| GET /api/v1/work-orders/:id | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/work-orders.spec.ts` | `run_tests.sh:407`; work-orders spec reads created order |
| PUT /api/v1/work-orders/:id | yes | true no-mock HTTP | `repo/e2e/tests/work-orders.spec.ts` | spec uses `api.put(/work-orders/${wo.id})` |
| POST /api/v1/work-orders/:id/close | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/work-orders.spec.ts` | `run_tests.sh:421`; close behavior tests in spec |
| POST /api/v1/work-orders/:id/rate | yes | true no-mock HTTP | `repo/run_tests.sh` | `run_tests.sh:431-441` |
| POST /api/v1/work-orders/:id/photos | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:848`; system spec photo-link API |
| GET /api/v1/work-orders/:id/photos | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:861`; system spec list photos |
| GET /api/v1/members | yes | true no-mock HTTP | `repo/e2e/tests/members.spec.ts` | members spec list/search call |
| POST /api/v1/members | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/members.spec.ts` | `run_tests.sh:464`; members spec create flows |
| GET /api/v1/members/:id | yes | true no-mock HTTP | `repo/e2e/tests/members.spec.ts`, `repo/run_tests.sh` | members spec poll/read; run_tests post-refund read |
| PUT /api/v1/members/:id | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1456-1473` |
| POST /api/v1/members/:id/freeze | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/members.spec.ts` | `run_tests.sh:505`; members spec freeze path |
| POST /api/v1/members/:id/unfreeze | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/members.spec.ts` | `run_tests.sh:524`; members spec unfreeze path |
| POST /api/v1/members/:id/redeem | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/members.spec.ts` | `run_tests.sh:485`; members spec redeem denial path |
| POST /api/v1/members/:id/add-value | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/members.spec.ts` | `run_tests.sh:475`; members spec add-value setup |
| POST /api/v1/members/:id/refund | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1480-1509` |
| GET /api/v1/members/:id/transactions | yes | true no-mock HTTP | `repo/run_tests.sh` | `run_tests.sh:533` |
| GET /api/v1/members/:id/packages | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1511-1536` |
| POST /api/v1/members/:id/packages | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/members.spec.ts` | `run_tests.sh:1523`; members spec package validation |
| GET /api/v1/members/:id/sensitive | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1538-1555` |
| GET /api/v1/membership-tiers | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/members.spec.ts` | `run_tests.sh:455`; members spec tiers read |
| GET /api/v1/reminders/memberships | yes | true no-mock HTTP | `repo/run_tests.sh` | `run_tests.sh:1115` |
| GET /api/v1/reminders/low-stock | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/inventory.spec.ts` | `run_tests.sh:1122`; inventory spec reminder check |
| GET /api/v1/rate-tables | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1561-1567` |
| POST /api/v1/rate-tables | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/statements.spec.ts` | `run_tests.sh:545`; statements spec seeds rate table |
| PUT /api/v1/rate-tables/:id | yes | true no-mock HTTP | `repo/run_tests.sh` | coverage block `run_tests.sh:1575-1592` |
| POST /api/v1/rate-tables/import-csv | yes | true no-mock HTTP | `repo/e2e/tests/endpoints-coverage.spec.ts` | describe `POST /rate-tables/import-csv` |
| GET /api/v1/statements | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/statements.spec.ts` | `run_tests.sh:1058`; statements spec list call |
| POST /api/v1/statements/generate | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/statements.spec.ts` | `run_tests.sh:556`; statements spec generate |
| GET /api/v1/statements/:id | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/statements.spec.ts` | `run_tests.sh:1037`; statements spec final read |
| POST /api/v1/statements/:id/reconcile | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/statements.spec.ts` | `run_tests.sh:956`; statements spec reconcile |
| POST /api/v1/statements/:id/approve | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/statements.spec.ts` | `run_tests.sh:974`; statements spec approve negative/success |
| POST /api/v1/statements/:id/export | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/statements.spec.ts` | `run_tests.sh:1020`; statements spec export |
| POST /api/v1/files/upload | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:768`; system spec upload |
| GET /api/v1/files/:id | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:782`; system spec download |
| POST /api/v1/files/export-zip | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/endpoints-coverage.spec.ts` | `run_tests.sh:1607-1624`; describe `POST /files/export-zip` |
| POST /api/v1/system/backup | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:611`; system spec backup |
| GET /api/v1/system/backup/status | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:216`; system spec status read |
| POST /api/v1/system/update | yes | true no-mock HTTP | `repo/e2e/tests/system.spec.ts` | system spec update endpoint tests |
| POST /api/v1/system/rollback | yes | true no-mock HTTP | `repo/e2e/tests/system.spec.ts` | system spec rollback endpoint tests |
| GET /api/v1/system/config | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:574`; system spec get config |
| PUT /api/v1/system/config | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:584`; system spec update config |
| GET /api/v1/drafts | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:693`; system spec list drafts |
| PUT /api/v1/drafts/:formType | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:676`; system spec save draft |
| GET /api/v1/drafts/:formType/:formId | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:703`; system spec get draft |
| DELETE /api/v1/drafts/:formType/:formId | yes | true no-mock HTTP | `repo/run_tests.sh`, `repo/e2e/tests/system.spec.ts` | `run_tests.sh:720`; system spec delete draft |

## API Test Classification

### 1) True No-Mock HTTP
- `repo/run_tests.sh` (curl against live app surface on `localhost`) exercises all route families, then runs containerized test layers.
- `repo/e2e/tests/*.spec.ts` uses Playwright real browser and `APIRequestContext` against real backend/frontend/DB.

### 2) HTTP with Mocking
- `repo/backend/internal/handlers/authz_integration_test.go` uses `echo.Context` with stub repositories (`stubWorkOrderStore`, `stubFileStore`) and direct handler invocations (e.g., `TestGetWorkOrderAPI_Submitter_Gets200`).
- `repo/backend/internal/handlers/handlers_test.go` constructs request/recorder/context and invokes handlers directly (e.g., `TestCreateSKU_MissingName_Returns400`, `TestImportContent_MissingCategory_Returns400`) without full router/runtime/repo stack.

### 3) Non-HTTP (unit/integration without HTTP)
- `repo/backend/internal/handlers/security_test.go` pure logic/predicate tests.
- `repo/backend/internal/repository/tenant_test.go` SQL capture-driver tests.
- `repo/backend/internal/repository/statement_tx_test.go` transaction atomicity via custom SQL driver.

## Mock Detection
- Frontend mocks detected:
  - `vi.mock('../hooks/useAuth', ...)` in `repo/frontend/src/renderer/__tests__/components.test.tsx:25`
  - `vi.mock('../services/api', ...)` in `repo/frontend/src/renderer/__tests__/components.test.tsx:55`
- Backend stubbing/bypass of real infra detected:
  - Stub stores (`stubWorkOrderStore`, `stubFileStore`) in `repo/backend/internal/handlers/authz_integration_test.go:31-95`
  - Direct handler calls (bypassing full HTTP router bootstrapping) in `repo/backend/internal/handlers/handlers_test.go`.

## Coverage Summary
- Total endpoints: **84**
- Endpoints with HTTP tests: **84**
- Endpoints with TRUE no-mock HTTP tests: **84**
- HTTP coverage: **100.0%**
- True API coverage: **100.0%**

## Unit Test Summary

### Backend Unit Tests
- Test files:
  - `repo/backend/internal/handlers/handlers_test.go`
  - `repo/backend/internal/handlers/authz_integration_test.go`
  - `repo/backend/internal/handlers/security_test.go`
  - `repo/backend/internal/handlers/system_update_test.go`
  - `repo/backend/internal/repository/tenant_test.go`
  - `repo/backend/internal/repository/statement_tx_test.go`
- Modules covered:
  - Controllers/handlers: auth, users validation, inventory validation, learning validation/import constraints, system update extraction safety.
  - Repositories: tenant scoping and transaction atomicity.
  - Auth/authorization/middleware-adjacent logic: object auth predicates and status-machine guards.
- Important backend modules still weaker at unit depth:
  - Full handler-runtime interactions (middleware chain + repo + DB) remain more integration-driven than unit-isolated.
  - Some update/rollback behavior assertions are contract-level and file-safety focused rather than full lifecycle simulation.

### Frontend Unit Tests (STRICT REQUIREMENT)
- Frontend test files found:
  - `repo/frontend/src/renderer/__tests__/components.test.tsx`
  - `repo/frontend/src/renderer/__tests__/draft.test.ts`
  - `repo/frontend/src/renderer/routes.test.ts`
  - `repo/frontend/src/renderer/utils.test.ts`
- Framework/tooling evidence:
  - Vitest (`describe/it/vi`) and React Testing Library (`render/screen/fireEvent`) in `components.test.tsx:10-12`.
- Components/modules covered (direct imports/render):
  - `LoginPage`, `DashboardPage`, `StocktakePage`, plus many route and state logic checks.
- Important frontend components/modules not strongly unit-tested without mocks:
  - Real `services/api.ts` contract behavior under unit tests (API layer is mocked in component tests).
  - Some route and utility suites test local logic mirrors rather than the exact production implementation path.

**Mandatory Verdict:** **Frontend unit tests: PRESENT**

### Cross-Layer Observation
- Coverage is now broad across backend API, backend unit, frontend unit, and E2E.
- Balance improved; not backend-only.
- Remaining imbalance: frontend unit tests are still mock-heavy compared to true integration behavior.

## API Observability Check
- Strong observability in `run_tests.sh`: explicit method/path, request payloads, and response-body assertions (`jq` semantic checks).
- Strong observability in Playwright API suites (`endpoints-coverage.spec.ts`, `statements.spec.ts`, `system.spec.ts`) with concrete response invariants.
- Weak spots remain where assertions are status-only or permissive shape checks in parts of large shell flow.

## Tests Check
- `run_tests.sh` exists (`repo/run_tests.sh`) and is Docker-oriented for backend/frontend/e2e layers (`docker compose --profile test run ...`).
- Main flow does **not** depend on local Python/Node.
- Host dependencies do exist (`bash`, `curl`, `jq`, plus shell utilities such as `mktemp`, `grep`, `awk`, `xxd`, `wc`, `head`, `date`).
- Relevant test categories for this fullstack repo are materially present: API, integration, backend unit, frontend unit/component, and E2E.
- Tests are meaningful and non-placeholder; suite breadth is now high.

## Test Coverage Score (0–100)
- **93/100**

## Score Rationale
- Full endpoint HTTP and true no-mock API coverage is now statically evidenced.
- Real E2E + backend unit + frontend unit layers are all present.
- Score is reduced from perfect due to remaining mock-heavy frontend unit strategy and uneven assertion depth in parts of the large bash suite.

## Key Gaps
- Frontend component tests still rely on mocked auth/API modules (`vi.mock`), reducing contract-drift detection at unit level.
- Several checks in shell-based integration remain schema/light-shape oriented vs strict business-state invariants.
- Handler-direct tests with stubs are still present (useful, but not substitutes for full runtime behavior).

## Confidence & Assumptions
- Confidence: **high** for endpoint inventory and route-to-test mapping.
- Assumption: endpoint counted as covered when an explicit request to exact method+path exists in inspected test code/script and would route through real HTTP layer by static design.

---

# README Audit

Target: `repo/README.md`

## High Priority Issues
- None blocking. README now includes strict project-type label, Docker startup literal, explicit verification, and all-role credential guidance.

## Medium Priority Issues
- README is very long and mixes operational guidance with informational desktop packaging notes; quickstart signal/noise can be improved.
- Some sections repeat equivalent commands (`docker-compose` and `docker compose` forms), increasing maintenance burden.

## Low Priority Issues
- Minor verbosity and repetition in explanatory notes could be condensed.

## Hard Gate Failures
- **None detected**.
  - `repo/README.md` exists.
  - Contains `docker-compose up` literal (`repo/README.md:49`).
  - Contains access URLs/ports (`repo/README.md:61-66`).
  - Contains verification method (API curl + UI/smoke flow) (`repo/README.md:74-123`).
  - No disallowed install commands (`npm install`, `pip install`, `apt-get`) found.
  - Auth credentials include all roles (`repo/README.md:147-154`, `repo/README.md:171-177`).

## README Verdict
- **PASS**
