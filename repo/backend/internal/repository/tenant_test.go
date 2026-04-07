// Package repository — tenant isolation tests.
//
// Strategy: register a lightweight capturing sql.Driver (standard library only,
// no external dependencies) that records every SQL statement and its bound
// arguments.  Each test builds a Repository pointed at that driver, calls the
// method under test, then asserts:
//
//  1. The SQL string sent to the DB contains "tenant_id" — structural check.
//  2. The tenant ID value configured on the Repository is present in the bound
//     args — behavioural check that would fail if the arg were removed or
//     swapped to a hardcoded literal.
//
// Because the driver returns no rows the repository functions return
// sql.ErrNoRows / a scan error — this is expected and does not affect the test
// assertions; we only care what SQL+args arrived at the driver before any result
// was processed.

package repository_test

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"medops/internal/models"
	"medops/internal/repository"
)

// ─── Capturing sql Driver ─────────────────────────────────────────────────────

type executedQuery struct {
	sql  string
	args []driver.Value
}

type capturedSQL struct {
	mu      sync.Mutex
	queries []executedQuery
}

func (c *capturedSQL) reset() {
	c.mu.Lock()
	c.queries = c.queries[:0]
	c.mu.Unlock()
}

func (c *capturedSQL) snapshot() []executedQuery {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]executedQuery, len(c.queries))
	copy(out, c.queries)
	return out
}

// sqlContains returns true if any captured query's SQL holds sub.
func (c *capturedSQL) sqlContains(sub string) bool {
	for _, q := range c.snapshot() {
		if strings.Contains(q.sql, sub) {
			return true
		}
	}
	return false
}

// argContains returns true if any captured query passed value v as a bound arg.
func (c *capturedSQL) argContains(v string) bool {
	for _, q := range c.snapshot() {
		for _, a := range q.args {
			if fmt.Sprintf("%v", a) == v {
				return true
			}
		}
	}
	return false
}

// minPlaceholders returns true if any captured SQL has at least n "$" placeholders.
func (c *capturedSQL) minPlaceholders(n int) bool {
	for _, q := range c.snapshot() {
		if strings.Count(q.sql, "$") >= n {
			return true
		}
	}
	return false
}

// ─── Driver / Conn / Stmt / Rows / Tx implementations ────────────────────────

type capDriver struct{ cap *capturedSQL }

func (d *capDriver) Open(_ string) (driver.Conn, error) {
	return &capConn{d: d}, nil
}

type capConn struct{ d *capDriver }

func (c *capConn) Prepare(query string) (driver.Stmt, error) {
	return &capStmt{conn: c, sql: query}, nil
}
func (c *capConn) Close() error              { return nil }
func (c *capConn) Begin() (driver.Tx, error) { return &capTx{}, nil }

type capStmt struct {
	conn *capConn
	sql  string
}

func (s *capStmt) Close() error    { return nil }
func (s *capStmt) NumInput() int   { return -1 } // variadic — no check

func (s *capStmt) record(args []driver.Value) {
	s.conn.d.cap.mu.Lock()
	s.conn.d.cap.queries = append(s.conn.d.cap.queries, executedQuery{sql: s.sql, args: args})
	s.conn.d.cap.mu.Unlock()
}

func (s *capStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.record(args)
	return driver.RowsAffected(0), nil
}

func (s *capStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.record(args)
	return &capRows{}, nil
}

// capRows — always empty so Scan immediately gets ErrNoRows / EOF.
type capRows struct{ done bool }

func (r *capRows) Columns() []string { return []string{} }
func (r *capRows) Close() error      { return nil }
func (r *capRows) Next(_ []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	return io.EOF
}

type capTx struct{}

func (t *capTx) Commit() error   { return nil }
func (t *capTx) Rollback() error { return nil }

// ─── Test harness ─────────────────────────────────────────────────────────────

const (
	testDriverName = "capture-tenant-v2"
	testTenantID   = "clinic-tenant-42"
	// encryptKey is padded/truncated to 32 bytes internally by repository.New.
	testEncryptKey = "test-encrypt-key-32-bytes-padding"
)

var (
	registerOnce sync.Once
	globalCap    = &capturedSQL{}
	testDB       *sql.DB
)

func openTestDB(t *testing.T) (*sql.DB, *capturedSQL) {
	t.Helper()
	registerOnce.Do(func() {
		sql.Register(testDriverName, &capDriver{cap: globalCap})
		var err error
		testDB, err = sql.Open(testDriverName, "")
		if err != nil {
			panic("capDriver open: " + err.Error())
		}
		// Keep a single open connection so Begin() works for transaction tests.
		testDB.SetMaxOpenConns(1)
	})
	globalCap.reset()
	return testDB, globalCap
}

func newRepo(t *testing.T) (*repository.Repository, *capturedSQL) {
	t.Helper()
	db, cap := openTestDB(t)
	return repository.New(db, testEncryptKey, testTenantID), cap
}

// ─── Assertion helpers ────────────────────────────────────────────────────────

func assertSQL(t *testing.T, cap *capturedSQL, method, sub string) {
	t.Helper()
	if !cap.sqlContains(sub) {
		t.Errorf("%s: SQL does not contain %q\n  captured: %v", method, sub, cap.snapshot())
	}
}

func assertArg(t *testing.T, cap *capturedSQL, method, value string) {
	t.Helper()
	if !cap.argContains(value) {
		t.Errorf("%s: tenant arg %q not found in bound arguments\n  captured: %v", method, value, cap.snapshot())
	}
}

func assertPlaceholders(t *testing.T, cap *capturedSQL, method string, min int) {
	t.Helper()
	if !cap.minPlaceholders(min) {
		t.Errorf("%s: expected ≥%d '$' placeholders in SQL (tenant arg missing?)", method, min)
	}
}

// ─── Tests: 6 methods from audit Objective 1 ─────────────────────────────────

func TestGetBatch_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.GetBatch("batch-1")
	assertSQL(t, cap, "GetBatch", "tenant_id")
	assertArg(t, cap, "GetBatch", testTenantID)
	assertPlaceholders(t, cap, "GetBatch", 2)
}

func TestListStockTransactions_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.ListStockTransactions("sku-1", 1, 20)
	assertSQL(t, cap, "ListStockTransactions", "tenant_id")
	assertArg(t, cap, "ListStockTransactions", testTenantID)
}

func TestGetStocktake_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.GetStocktake("st-1")
	assertSQL(t, cap, "GetStocktake", "tenant_id")
	assertArg(t, cap, "GetStocktake", testTenantID)
	assertPlaceholders(t, cap, "GetStocktake", 2)
}

func TestGetSessionPackage_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.GetSessionPackage("pkg-1")
	assertSQL(t, cap, "GetSessionPackage", "tenant_id")
	assertArg(t, cap, "GetSessionPackage", testTenantID)
	assertPlaceholders(t, cap, "GetSessionPackage", 2)
}

func TestGetWorkOrderPhotos_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.GetWorkOrderPhotos("wo-1")
	assertSQL(t, cap, "GetWorkOrderPhotos", "tenant_id")
	assertArg(t, cap, "GetWorkOrderPhotos", testTenantID)
}

func TestDeleteFileRecord_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.DeleteFileRecord("file-1")
	assertSQL(t, cap, "DeleteFileRecord", "tenant_id")
	assertArg(t, cap, "DeleteFileRecord", testTenantID)
	assertPlaceholders(t, cap, "DeleteFileRecord", 2)
}

// ─── Tests: methods fixed in follow-up recommendations ───────────────────────

func TestGetBatchesBySKU_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.GetBatchesBySKU("sku-1")
	assertSQL(t, cap, "GetBatchesBySKU", "tenant_id")
	assertArg(t, cap, "GetBatchesBySKU", testTenantID)
}

func TestCompleteStocktake_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.CompleteStocktake("st-1")
	assertSQL(t, cap, "CompleteStocktake", "tenant_id")
	assertArg(t, cap, "CompleteStocktake", testTenantID)
	assertPlaceholders(t, cap, "CompleteStocktake", 2)
}

// TestUpdateStocktakeLines_TenantGuard verifies that the method sends a
// tenant-scoped ownership check before modifying stocktake lines.
func TestUpdateStocktakeLines_TenantGuard(t *testing.T) {
	repo, cap := newRepo(t)
	// The ownership check will return "not found" (empty rows), so the function
	// returns an error — that's expected.  We only assert on the SQL sent.
	repo.UpdateStocktakeLines("st-1", nil)
	assertSQL(t, cap, "UpdateStocktakeLines", "tenant_id")
	assertArg(t, cap, "UpdateStocktakeLines", testTenantID)
}

func TestCreateStocktake_TenantPersisted(t *testing.T) {
	repo, cap := newRepo(t)
	repo.CreateStocktake(&models.Stocktake{})
	assertSQL(t, cap, "CreateStocktake", "tenant_id")
	assertArg(t, cap, "CreateStocktake", testTenantID)
}

func TestCreateStockTransaction_TenantPersisted(t *testing.T) {
	repo, cap := newRepo(t)
	repo.CreateStockTransaction(&models.StockTransaction{})
	assertSQL(t, cap, "CreateStockTransaction", "tenant_id")
	assertArg(t, cap, "CreateStockTransaction", testTenantID)
}

// ─── Tests: auth methods fixed in I-001 ──────────────────────────────────────

func TestGetUserByUsername_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.GetUserByUsername("admin")
	assertSQL(t, cap, "GetUserByUsername", "tenant_id")
	assertArg(t, cap, "GetUserByUsername", testTenantID)
	assertPlaceholders(t, cap, "GetUserByUsername", 2)
}

func TestCreateUser_TenantPersisted(t *testing.T) {
	repo, cap := newRepo(t)
	repo.CreateUser(&models.User{Username: "newuser", PasswordHash: "hash", Role: "front_desk"})
	assertSQL(t, cap, "CreateUser", "tenant_id")
	assertArg(t, cap, "CreateUser", testTenantID)
}

func TestIncrementFailedAttempts_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.IncrementFailedAttempts("user-1")
	assertSQL(t, cap, "IncrementFailedAttempts", "tenant_id")
	assertArg(t, cap, "IncrementFailedAttempts", testTenantID)
}

func TestLockUser_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.LockUser("user-1", 15)
	assertSQL(t, cap, "LockUser", "tenant_id")
	assertArg(t, cap, "LockUser", testTenantID)
}

func TestUnlockUser_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.UnlockUser("user-1")
	assertSQL(t, cap, "UnlockUser", "tenant_id")
	assertArg(t, cap, "UnlockUser", testTenantID)
}

func TestResetFailedAttempts_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.ResetFailedAttempts("user-1")
	assertSQL(t, cap, "ResetFailedAttempts", "tenant_id")
	assertArg(t, cap, "ResetFailedAttempts", testTenantID)
}

func TestUpdateSKU_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.UpdateSKU(&models.SKU{ID: "sku-1", Name: "Test"})
	assertSQL(t, cap, "UpdateSKU", "tenant_id")
	assertArg(t, cap, "UpdateSKU", testTenantID)
	assertPlaceholders(t, cap, "UpdateSKU", 2)
}

func TestGetLowStockSKUs_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.GetLowStockSKUs()
	assertSQL(t, cap, "GetLowStockSKUs", "tenant_id")
	assertArg(t, cap, "GetLowStockSKUs", testTenantID)
}

func TestListRateTables_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.ListRateTables()
	assertSQL(t, cap, "ListRateTables", "tenant_id")
	assertArg(t, cap, "ListRateTables", testTenantID)
}

func TestListExpiredFiles_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.ListExpiredFiles()
	assertSQL(t, cap, "ListExpiredFiles", "tenant_id")
	assertArg(t, cap, "ListExpiredFiles", testTenantID)
}

func TestListSubjects_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.ListSubjects()
	assertSQL(t, cap, "ListSubjects", "tenant_id")
	assertArg(t, cap, "ListSubjects", testTenantID)
}

func TestCreateSubject_TenantPersisted(t *testing.T) {
	repo, cap := newRepo(t)
	repo.CreateSubject(&models.LearningSubject{Name: "First Aid"})
	assertSQL(t, cap, "CreateSubject", "tenant_id")
	assertArg(t, cap, "CreateSubject", testTenantID)
}

func TestListChapters_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.ListChapters("subj-1")
	assertSQL(t, cap, "ListChapters", "tenant_id")
	assertArg(t, cap, "ListChapters", testTenantID)
}

// ─── Tests: knowledge point methods fixed in I-001 (BLOCKER) ─────────────────

func TestListKnowledgePoints_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.ListKnowledgePoints("chapter-1", 1, 20)
	assertSQL(t, cap, "ListKnowledgePoints", "tenant_id")
	assertArg(t, cap, "ListKnowledgePoints", testTenantID)
}

func TestSearchKnowledgePoints_TenantScoped(t *testing.T) {
	repo, cap := newRepo(t)
	repo.SearchKnowledgePoints("medication safety", 1, 20)
	assertSQL(t, cap, "SearchKnowledgePoints", "tenant_id")
	assertArg(t, cap, "SearchKnowledgePoints", testTenantID)
}

func TestCreateKnowledgePoint_TenantPersisted(t *testing.T) {
	repo, cap := newRepo(t)
	repo.CreateKnowledgePoint(&models.KnowledgePoint{ChapterID: "chapter-1", Title: "Test KP"})
	assertSQL(t, cap, "CreateKnowledgePoint", "tenant_id")
	assertArg(t, cap, "CreateKnowledgePoint", testTenantID)
}

// ─── Isolation invariant: different tenant IDs produce different bound args ───

// TestTenantArgIsConfiguredValue fails if the repository passes a hardcoded
// string (e.g. "default") instead of the runtime-configured tenant ID.
func TestTenantArgIsConfiguredValue(t *testing.T) {
	db, _ := openTestDB(t)
	cap := globalCap // already reset by openTestDB

	const specificTenant = "hospital-unit-7"
	repo := repository.New(db, testEncryptKey, specificTenant)

	cap.reset()
	repo.GetStocktake("st-x")

	if !cap.argContains(specificTenant) {
		t.Errorf("GetStocktake bound arg should be %q (the configured tenant), got: %v",
			specificTenant, cap.snapshot())
	}
	if cap.argContains(testTenantID) {
		t.Errorf("GetStocktake must not pass previous repo's tenant %q", testTenantID)
	}
}
