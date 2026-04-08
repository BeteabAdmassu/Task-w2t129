// Package repository — atomicity tests for CreateStatementWithLineItems.
//
// Strategy: register a lightweight transaction-tracking sql.Driver (stdlib only,
// no external dependencies) that records whether Begin/Commit/Rollback were called
// and can be configured to inject a failure on any Exec that targets a given SQL
// fragment.  Tests verify:
//
//  1. Success path: transaction is started, statement + all line items are inserted,
//     transaction is committed, and no rollback occurs.
//
//  2. Line-item failure path: when a line-item INSERT fails, Rollback is called
//     and Commit is not — ensuring the partial statement header is not persisted.
//
// No live database or docker service is required; the driver handles everything
// in memory.
package repository_test

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"medops/internal/models"
	"medops/internal/repository"
)

// ─── Transaction-tracking driver ─────────────────────────────────────────────

// txTrackState holds per-test state for the tracking driver.
// Reset it with reset() before each test.
type txTrackState struct {
	mu          sync.Mutex
	began       bool
	committed   bool
	rolledBack  bool
	executedSQL []string
	failOnSQL   string // if non-empty, Exec fails when SQL contains this fragment
}

func (s *txTrackState) reset(failOnSQL string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.began = false
	s.committed = false
	s.rolledBack = false
	s.executedSQL = nil
	s.failOnSQL = failOnSQL
}

func (s *txTrackState) recordSQL(q string) {
	s.mu.Lock()
	s.executedSQL = append(s.executedSQL, q)
	s.mu.Unlock()
}

func (s *txTrackState) snapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]string, len(s.executedSQL))
	copy(cp, s.executedSQL)
	return cp
}

func (s *txTrackState) hasSQL(fragment string) bool {
	for _, q := range s.snapshot() {
		if strings.Contains(q, fragment) {
			return true
		}
	}
	return false
}

// ─── driver.Driver ────────────────────────────────────────────────────────────

type txTrackDriver struct{ state *txTrackState }

func (d *txTrackDriver) Open(_ string) (driver.Conn, error) {
	return &txTrackConn{state: d.state}, nil
}

// ─── driver.Conn ─────────────────────────────────────────────────────────────

type txTrackConn struct{ state *txTrackState }

func (c *txTrackConn) Prepare(query string) (driver.Stmt, error) {
	return &txTrackStmt{state: c.state, sql: query}, nil
}

func (c *txTrackConn) Close() error { return nil }

func (c *txTrackConn) Begin() (driver.Tx, error) {
	c.state.mu.Lock()
	c.state.began = true
	c.state.mu.Unlock()
	return &txTrackTx{state: c.state}, nil
}

// ─── driver.Stmt ─────────────────────────────────────────────────────────────

type txTrackStmt struct {
	state *txTrackState
	sql   string
}

func (s *txTrackStmt) Close() error  { return nil }
func (s *txTrackStmt) NumInput() int { return -1 } // variadic

func (s *txTrackStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.state.recordSQL(s.sql)
	s.state.mu.Lock()
	fail := s.state.failOnSQL
	s.state.mu.Unlock()
	if fail != "" && strings.Contains(s.sql, fail) {
		return nil, errors.New("injected failure for: " + fail)
	}
	// Return RowsAffected(1) so the n==0 guard in CreateStatementWithLineItems does
	// not trigger a spurious rollback on the success path.
	return driver.RowsAffected(1), nil
}

func (s *txTrackStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.state.recordSQL(s.sql)
	return &txEmptyRows{}, nil
}

// ─── driver.Tx ───────────────────────────────────────────────────────────────

type txTrackTx struct{ state *txTrackState }

func (tx *txTrackTx) Commit() error {
	tx.state.mu.Lock()
	tx.state.committed = true
	tx.state.mu.Unlock()
	return nil
}

func (tx *txTrackTx) Rollback() error {
	tx.state.mu.Lock()
	tx.state.rolledBack = true
	tx.state.mu.Unlock()
	return nil
}

// ─── driver.Rows (always empty) ───────────────────────────────────────────────

type txEmptyRows struct{ done bool }

func (r *txEmptyRows) Columns() []string { return []string{} }
func (r *txEmptyRows) Close() error      { return nil }
func (r *txEmptyRows) Next(_ []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	return io.EOF
}

// ─── Shared driver registration ───────────────────────────────────────────────

const txTrackDriverName = "tx-track-atomicity-v1"

var (
	txTrackOnce  sync.Once
	globalTxStat = &txTrackState{}
	txTrackDB    *sql.DB
)

func openTxTrackDB(t *testing.T) (*sql.DB, *txTrackState) {
	t.Helper()
	txTrackOnce.Do(func() {
		sql.Register(txTrackDriverName, &txTrackDriver{state: globalTxStat})
		var err error
		txTrackDB, err = sql.Open(txTrackDriverName, "")
		if err != nil {
			panic("txTrackDriver open: " + err.Error())
		}
		// Single connection ensures Begin() is always available.
		txTrackDB.SetMaxOpenConns(1)
	})
	return txTrackDB, globalTxStat
}

func newTxRepo(t *testing.T) (*repository.Repository, *txTrackState) {
	t.Helper()
	db, state := openTxTrackDB(t)
	return repository.New(db, testEncryptKey, testTenantID), state
}

// ─── Tests ───────────────────────────────────────────────────────────────────

// TestCreateStatementWithLineItems_Success_CommitsAll verifies that the happy
// path opens a transaction, executes the statement INSERT, executes an INSERT
// per line item, and then commits — with no rollback.
func TestCreateStatementWithLineItems_Success_CommitsAll(t *testing.T) {
	repo, state := newTxRepo(t)
	state.reset("") // no injected failure

	stmt := &models.ChargeStatement{
		PeriodStart: "2026-01-01",
		PeriodEnd:   "2026-01-31",
		TotalAmount: 150.0,
		Status:      "pending",
	}
	items := []models.ChargeLineItem{
		{Description: "Long-haul transport", Quantity: 10, UnitPrice: 8.0, Surcharge: 1.6, Tax: 0.82, Total: 10.42},
		{Description: "Last-mile delivery", Quantity: 3, UnitPrice: 5.0, Surcharge: 0.75, Tax: 0, Total: 15.75},
	}

	if err := repo.CreateStatementWithLineItems(stmt, items); err != nil {
		t.Fatalf("unexpected error on success path: %v", err)
	}

	if !state.began {
		t.Error("expected DB transaction to be started (Begin not called)")
	}
	if !state.committed {
		t.Error("expected transaction to be committed on success path")
	}
	if state.rolledBack {
		t.Error("unexpected rollback on success path")
	}

	// Verify both statement and line-item SQL were executed within the transaction.
	if !state.hasSQL("charge_statements") {
		t.Error("expected INSERT into charge_statements to be executed")
	}
	if !state.hasSQL("charge_line_items") {
		t.Error("expected INSERT into charge_line_items to be executed")
	}

	// Statement ID must have been assigned by the method.
	if stmt.ID == "" {
		t.Error("expected statement.ID to be populated after CreateStatementWithLineItems")
	}
	// Each line item must carry the statement ID.
	for i, item := range items {
		if item.StatementID != stmt.ID {
			t.Errorf("items[%d].StatementID = %q, want %q", i, item.StatementID, stmt.ID)
		}
	}
}

// TestCreateStatementWithLineItems_LineItemFailure_RollsBack verifies that when
// a line-item INSERT returns an error, the repository method rolls back the
// transaction (preventing the orphan statement header from being committed) and
// returns a non-nil error to the caller.
func TestCreateStatementWithLineItems_LineItemFailure_RollsBack(t *testing.T) {
	repo, state := newTxRepo(t)
	// Inject a failure whenever SQL targets charge_line_items.
	state.reset("charge_line_items")

	stmt := &models.ChargeStatement{
		PeriodStart: "2026-02-01",
		PeriodEnd:   "2026-02-28",
		TotalAmount: 50.0,
		Status:      "pending",
	}
	items := []models.ChargeLineItem{
		{Description: "Fuel charge", Quantity: 1, UnitPrice: 50.0, Surcharge: 5.0, Tax: 0, Total: 55.0},
	}

	err := repo.CreateStatementWithLineItems(stmt, items)
	if err == nil {
		t.Fatal("expected error when line-item INSERT fails, got nil")
	}

	if !state.began {
		t.Error("expected transaction to have been started before the failure")
	}
	if state.committed {
		t.Error("expected no COMMIT when line-item insert fails (partial data would escape)")
	}
	if !state.rolledBack {
		t.Error("expected ROLLBACK when line-item insert fails to prevent orphan statement")
	}
}

// TestCreateStatementWithLineItems_StatementFailure_RollsBack verifies that when
// the statement header INSERT itself fails, the transaction is rolled back immediately.
func TestCreateStatementWithLineItems_StatementFailure_RollsBack(t *testing.T) {
	repo, state := newTxRepo(t)
	// Fail on the statement header insert (charge_statements table).
	state.reset("charge_statements")

	stmt := &models.ChargeStatement{
		PeriodStart: "2026-03-01",
		PeriodEnd:   "2026-03-31",
		TotalAmount: 20.0,
		Status:      "pending",
	}
	items := []models.ChargeLineItem{
		{Description: "Tax charge", Quantity: 1, UnitPrice: 20.0, Total: 20.0},
	}

	err := repo.CreateStatementWithLineItems(stmt, items)
	if err == nil {
		t.Fatal("expected error when statement INSERT fails, got nil")
	}

	if !state.began {
		t.Error("expected transaction to have been started")
	}
	if state.committed {
		t.Error("expected no COMMIT when statement insert fails")
	}
	if !state.rolledBack {
		t.Error("expected ROLLBACK when statement insert fails")
	}

	// Line-item SQL must not have been attempted after the statement insert failed.
	if state.hasSQL("charge_line_items") {
		t.Error("line-item INSERT must not be attempted after statement INSERT failure")
	}
}
