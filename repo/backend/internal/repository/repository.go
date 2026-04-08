package repository

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	log "github.com/sirupsen/logrus"

	"medops/internal/models"
)

// Repository wraps a *sql.DB and provides methods for all database operations.
type Repository struct {
	DB         *sql.DB
	encryptKey []byte
	tenantID   string
}

// New creates a new Repository with the given database connection, AES-256 encryption key, and tenant ID.
func New(db *sql.DB, encryptKey, tenantID string) *Repository {
	var k [32]byte
	copy(k[:], []byte(encryptKey))
	return &Repository{DB: db, encryptKey: k[:], tenantID: tenantID}
}

// encryptDecimal encrypts a float64 monetary value using AES-256-GCM.
func (r *Repository) encryptDecimal(value float64) ([]byte, error) {
	block, err := aes.NewCipher(r.encryptKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	plaintext := []byte(strconv.FormatFloat(value, 'f', 2, 64))
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return []byte(hex.EncodeToString(ciphertext)), nil
}

// decryptDecimal decrypts an AES-256-GCM encrypted monetary value back to float64.
func (r *Repository) decryptDecimal(data []byte) (float64, error) {
	ciphertext, err := hex.DecodeString(string(data))
	if err != nil {
		return 0, err
	}
	block, err := aes.NewCipher(r.encryptKey)
	if err != nil {
		return 0, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return 0, fmt.Errorf("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(string(plaintext), 64)
}

// ---------- Auth ----------

// GetUserByUsername retrieves a user by their username.
func (r *Repository) GetUserByUsername(username string) (*models.User, error) {
	u := &models.User{}
	err := r.DB.QueryRow(
		`SELECT id, username, password_hash, role, failed_attempts, locked_until, is_active, must_change_password, created_at, updated_at
		 FROM auth_users WHERE username = $1 AND tenant_id = $2`, username, r.tenantID,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.FailedAttempts, &u.LockedUntil, &u.IsActive, &u.MustChangePassword, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return u, nil
}

// GetUserByID retrieves a user by their ID.
func (r *Repository) GetUserByID(id string) (*models.User, error) {
	u := &models.User{}
	err := r.DB.QueryRow(
		`SELECT id, username, password_hash, role, failed_attempts, locked_until, is_active, must_change_password, created_at, updated_at
		 FROM auth_users WHERE id = $1 AND tenant_id = $2`, id, r.tenantID,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.FailedAttempts, &u.LockedUntil, &u.IsActive, &u.MustChangePassword, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// CreateUser inserts a new user into the database.
func (r *Repository) CreateUser(user *models.User) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	_, err := r.DB.Exec(
		`INSERT INTO auth_users (id, username, password_hash, role, failed_attempts, locked_until, is_active, must_change_password, created_at, updated_at, tenant_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		user.ID, user.Username, user.PasswordHash, user.Role, user.FailedAttempts, user.LockedUntil, user.IsActive, user.MustChangePassword, user.CreatedAt, user.UpdatedAt, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// UpdateUser updates an existing user.
func (r *Repository) UpdateUser(user *models.User) error {
	user.UpdatedAt = time.Now()
	_, err := r.DB.Exec(
		`UPDATE auth_users SET username=$1, password_hash=$2, role=$3, is_active=$4, must_change_password=$5, updated_at=$6 WHERE id=$7 AND tenant_id=$8`,
		user.Username, user.PasswordHash, user.Role, user.IsActive, user.MustChangePassword, user.UpdatedAt, user.ID, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// ListUsers returns all users.
func (r *Repository) ListUsers() ([]models.User, error) {
	rows, err := r.DB.Query(
		`SELECT id, username, password_hash, role, failed_attempts, locked_until, is_active, must_change_password, created_at, updated_at
		 FROM auth_users WHERE tenant_id = $1 ORDER BY created_at DESC`, r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.FailedAttempts, &u.LockedUntil, &u.IsActive, &u.MustChangePassword, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list users scan: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// IncrementFailedAttempts increments the failed login attempts counter.
func (r *Repository) IncrementFailedAttempts(userID string) error {
	_, err := r.DB.Exec(
		`UPDATE auth_users SET failed_attempts = failed_attempts + 1, updated_at = NOW() WHERE id = $1 AND tenant_id = $2`, userID, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("increment failed attempts: %w", err)
	}
	return nil
}

// LockUser locks a user account until the specified time.
func (r *Repository) LockUser(userID string, minutes int) error {
	until := time.Now().Add(time.Duration(minutes) * time.Minute)
	_, err := r.DB.Exec(
		`UPDATE auth_users SET locked_until = $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3`, until, userID, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("lock user: %w", err)
	}
	return nil
}

// UnlockUser removes the lock on a user account and resets failed attempts.
func (r *Repository) UnlockUser(userID string) error {
	_, err := r.DB.Exec(
		`UPDATE auth_users SET locked_until = NULL, failed_attempts = 0, updated_at = NOW() WHERE id = $1 AND tenant_id = $2`, userID, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("unlock user: %w", err)
	}
	return nil
}

// ResetFailedAttempts resets the failed login attempts counter to zero.
func (r *Repository) ResetFailedAttempts(userID string) error {
	_, err := r.DB.Exec(
		`UPDATE auth_users SET failed_attempts = 0, updated_at = NOW() WHERE id = $1 AND tenant_id = $2`, userID, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("reset failed attempts: %w", err)
	}
	return nil
}

// GetRoles returns all roles.
func (r *Repository) GetRoles() ([]models.Role, error) {
	rows, err := r.DB.Query(`SELECT id, display_name, permissions FROM roles ORDER BY display_name`)
	if err != nil {
		return nil, fmt.Errorf("get roles: %w", err)
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		if err := rows.Scan(&role.ID, &role.DisplayName, &role.Permissions); err != nil {
			return nil, fmt.Errorf("get roles scan: %w", err)
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

// ---------- SKUs ----------

// ListSKUs returns paginated SKUs with optional search on name/ndc/upc.
func (r *Repository) ListSKUs(search string, page, pageSize int) ([]models.SKU, int, error) {
	offset := (page - 1) * pageSize
	var rows *sql.Rows
	var err error

	if search != "" {
		pattern := "%" + search + "%"
		rows, err = r.DB.Query(
			`SELECT id, ndc, upc, name, description, unit_of_measure, low_stock_threshold, storage_location, is_active, created_at,
			        COUNT(*) OVER() AS total
			 FROM skus
			 WHERE (name ILIKE $1 OR ndc ILIKE $1 OR upc ILIKE $1) AND tenant_id = $2
			 ORDER BY name
			 LIMIT $3 OFFSET $4`, pattern, r.tenantID, pageSize, offset,
		)
	} else {
		rows, err = r.DB.Query(
			`SELECT id, ndc, upc, name, description, unit_of_measure, low_stock_threshold, storage_location, is_active, created_at,
			        COUNT(*) OVER() AS total
			 FROM skus
			 WHERE tenant_id = $1
			 ORDER BY name
			 LIMIT $2 OFFSET $3`, r.tenantID, pageSize, offset,
		)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list skus: %w", err)
	}
	defer rows.Close()

	var skus []models.SKU
	var total int
	for rows.Next() {
		var s models.SKU
		if err := rows.Scan(&s.ID, &s.NDC, &s.UPC, &s.Name, &s.Description, &s.UnitOfMeasure, &s.LowStockThreshold, &s.StorageLocation, &s.IsActive, &s.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("list skus scan: %w", err)
		}
		skus = append(skus, s)
	}
	return skus, total, rows.Err()
}

// GetSKU retrieves a single SKU by ID, scoped to the current tenant.
func (r *Repository) GetSKU(id string) (*models.SKU, error) {
	s := &models.SKU{}
	err := r.DB.QueryRow(
		`SELECT id, ndc, upc, name, description, unit_of_measure, low_stock_threshold, storage_location, is_active, created_at
		 FROM skus WHERE id = $1 AND tenant_id = $2`, id, r.tenantID,
	).Scan(&s.ID, &s.NDC, &s.UPC, &s.Name, &s.Description, &s.UnitOfMeasure, &s.LowStockThreshold, &s.StorageLocation, &s.IsActive, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get sku: %w", err)
	}
	return s, nil
}

// CreateSKU inserts a new SKU.
func (r *Repository) CreateSKU(sku *models.SKU) error {
	if sku.ID == "" {
		sku.ID = uuid.New().String()
	}
	sku.CreatedAt = time.Now()
	_, err := r.DB.Exec(
		`INSERT INTO skus (id, ndc, upc, name, description, unit_of_measure, low_stock_threshold, storage_location, is_active, created_at, tenant_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		sku.ID, sku.NDC, sku.UPC, sku.Name, sku.Description, sku.UnitOfMeasure, sku.LowStockThreshold, sku.StorageLocation, sku.IsActive, sku.CreatedAt, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create sku: %w", err)
	}
	return nil
}

// UpdateSKU updates an existing SKU.
func (r *Repository) UpdateSKU(sku *models.SKU) error {
	_, err := r.DB.Exec(
		`UPDATE skus SET ndc=$1, upc=$2, name=$3, description=$4, unit_of_measure=$5, low_stock_threshold=$6, storage_location=$7, is_active=$8
		 WHERE id=$9 AND tenant_id=$10`,
		sku.NDC, sku.UPC, sku.Name, sku.Description, sku.UnitOfMeasure, sku.LowStockThreshold, sku.StorageLocation, sku.IsActive, sku.ID, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("update sku: %w", err)
	}
	return nil
}

// GetLowStockSKUs returns active SKUs where total batch quantity is below threshold.
func (r *Repository) GetLowStockSKUs() ([]models.SKU, error) {
	rows, err := r.DB.Query(
		`SELECT s.id, s.ndc, s.upc, s.name, s.description, s.unit_of_measure, s.low_stock_threshold, s.storage_location, s.is_active, s.created_at
		 FROM skus s
		 WHERE s.is_active = true
		   AND s.tenant_id = $1
		   AND (SELECT COALESCE(SUM(b.quantity_on_hand), 0) FROM inventory_batches b WHERE b.sku_id = s.id) < s.low_stock_threshold
		 ORDER BY s.name`, r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("get low stock skus: %w", err)
	}
	defer rows.Close()

	var skus []models.SKU
	for rows.Next() {
		var s models.SKU
		if err := rows.Scan(&s.ID, &s.NDC, &s.UPC, &s.Name, &s.Description, &s.UnitOfMeasure, &s.LowStockThreshold, &s.StorageLocation, &s.IsActive, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("get low stock skus scan: %w", err)
		}
		skus = append(skus, s)
	}
	return skus, rows.Err()
}

// ---------- Batches ----------

// GetBatchesBySKU retrieves all batches for a given SKU, scoped to the current tenant via the parent SKU.
func (r *Repository) GetBatchesBySKU(skuID string) ([]models.InventoryBatch, error) {
	rows, err := r.DB.Query(
		`SELECT b.id, b.sku_id, b.lot_number, b.expiration_date, b.quantity_on_hand, b.created_at
		 FROM inventory_batches b
		 JOIN skus s ON s.id = b.sku_id AND s.tenant_id = $2
		 WHERE b.sku_id = $1
		 ORDER BY b.expiration_date`, skuID, r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("get batches by sku: %w", err)
	}
	defer rows.Close()

	var batches []models.InventoryBatch
	for rows.Next() {
		var b models.InventoryBatch
		if err := rows.Scan(&b.ID, &b.SKUID, &b.LotNumber, &b.ExpirationDate, &b.QuantityOnHand, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("get batches scan: %w", err)
		}
		batches = append(batches, b)
	}
	return batches, rows.Err()
}

// GetBatch retrieves a single batch by ID.
func (r *Repository) GetBatch(id string) (*models.InventoryBatch, error) {
	b := &models.InventoryBatch{}
	// inventory_batches has no tenant_id column; scope via the parent SKU's tenant_id.
	err := r.DB.QueryRow(
		`SELECT b.id, b.sku_id, b.lot_number, b.expiration_date, b.quantity_on_hand, b.created_at
		 FROM inventory_batches b
		 JOIN skus s ON s.id = b.sku_id AND s.tenant_id = $2
		 WHERE b.id = $1`, id, r.tenantID,
	).Scan(&b.ID, &b.SKUID, &b.LotNumber, &b.ExpirationDate, &b.QuantityOnHand, &b.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get batch: %w", err)
	}
	return b, nil
}

// CreateBatch inserts a new inventory batch.
func (r *Repository) CreateBatch(batch *models.InventoryBatch) error {
	if batch.ID == "" {
		batch.ID = uuid.New().String()
	}
	batch.CreatedAt = time.Now()
	_, err := r.DB.Exec(
		`INSERT INTO inventory_batches (id, sku_id, lot_number, expiration_date, quantity_on_hand, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		batch.ID, batch.SKUID, batch.LotNumber, batch.ExpirationDate, batch.QuantityOnHand, batch.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create batch: %w", err)
	}
	return nil
}

// UpdateBatchQuantity updates the quantity on hand for a batch.
func (r *Repository) UpdateBatchQuantity(batchID string, newQty int) error {
	_, err := r.DB.Exec(
		`UPDATE inventory_batches SET quantity_on_hand = $1 WHERE id = $2`, newQty, batchID,
	)
	if err != nil {
		return fmt.Errorf("update batch quantity: %w", err)
	}
	return nil
}

// ---------- Stock Transactions ----------

// CreateStockTransaction inserts a new stock transaction record.
func (r *Repository) CreateStockTransaction(tx *models.StockTransaction) error {
	if tx.ID == "" {
		tx.ID = uuid.New().String()
	}
	tx.CreatedAt = time.Now()
	_, err := r.DB.Exec(
		`INSERT INTO stock_transactions (id, sku_id, batch_id, type, quantity, reason_code, prescription_id, performed_by, created_at, tenant_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		tx.ID, tx.SKUID, tx.BatchID, tx.Type, tx.Quantity, tx.ReasonCode, tx.PrescriptionID, tx.PerformedBy, tx.CreatedAt, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create stock transaction: %w", err)
	}
	return nil
}

// ListStockTransactions returns paginated stock transactions for a SKU.
func (r *Repository) ListStockTransactions(skuID string, page, pageSize int) ([]models.StockTransaction, int, error) {
	offset := (page - 1) * pageSize
	rows, err := r.DB.Query(
		`SELECT id, sku_id, batch_id, type, quantity, reason_code,
		        prescription_id, performed_by, created_at,
		        COUNT(*) OVER() AS total
		 FROM stock_transactions
		 WHERE sku_id = $1 AND tenant_id = $4
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, skuID, pageSize, offset, r.tenantID,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list stock transactions: %w", err)
	}
	defer rows.Close()

	var txs []models.StockTransaction
	var total int
	for rows.Next() {
		var t models.StockTransaction
		if err := rows.Scan(&t.ID, &t.SKUID, &t.BatchID, &t.Type, &t.Quantity, &t.ReasonCode, &t.PrescriptionID, &t.PerformedBy, &t.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("list stock transactions scan: %w", err)
		}
		txs = append(txs, t)
	}
	return txs, total, rows.Err()
}

// ---------- Stocktakes ----------

// ListStocktakes returns all stocktakes for the current tenant, ordered newest first.
func (r *Repository) ListStocktakes() ([]models.Stocktake, error) {
	rows, err := r.DB.Query(
		`SELECT id, period_start, period_end, status, created_by, created_at
		 FROM stocktakes WHERE tenant_id = $1 ORDER BY created_at DESC`, r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list stocktakes: %w", err)
	}
	defer rows.Close()

	var stocktakes []models.Stocktake
	for rows.Next() {
		var st models.Stocktake
		if err := rows.Scan(&st.ID, &st.PeriodStart, &st.PeriodEnd, &st.Status, &st.CreatedBy, &st.CreatedAt); err != nil {
			return nil, fmt.Errorf("list stocktakes scan: %w", err)
		}
		stocktakes = append(stocktakes, st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list stocktakes iterate: %w", err)
	}
	if stocktakes == nil {
		stocktakes = []models.Stocktake{}
	}
	return stocktakes, nil
}

// CreateStocktake inserts a new stocktake record.
func (r *Repository) CreateStocktake(st *models.Stocktake) error {
	if st.ID == "" {
		st.ID = uuid.New().String()
	}
	st.CreatedAt = time.Now()
	_, err := r.DB.Exec(
		`INSERT INTO stocktakes (id, period_start, period_end, status, created_by, created_at, tenant_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		st.ID, st.PeriodStart, st.PeriodEnd, st.Status, st.CreatedBy, st.CreatedAt, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create stocktake: %w", err)
	}
	return nil
}

// GetStocktake retrieves a stocktake by ID, including its lines.
func (r *Repository) GetStocktake(id string) (*models.Stocktake, error) {
	st := &models.Stocktake{}
	err := r.DB.QueryRow(
		`SELECT id, period_start, period_end, status, created_by, created_at
		 FROM stocktakes WHERE id = $1 AND tenant_id = $2`, id, r.tenantID,
	).Scan(&st.ID, &st.PeriodStart, &st.PeriodEnd, &st.Status, &st.CreatedBy, &st.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get stocktake: %w", err)
	}

	// Load associated lines
	rows, err := r.DB.Query(
		`SELECT id, stocktake_id, sku_id, batch_id, system_qty, counted_qty, variance, loss_reason
		 FROM stocktake_lines WHERE stocktake_id = $1`, id,
	)
	if err != nil {
		return nil, fmt.Errorf("get stocktake lines: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var l models.StocktakeLine
		if err := rows.Scan(&l.ID, &l.StocktakeID, &l.SKUID, &l.BatchID, &l.SystemQty, &l.CountedQty, &l.Variance, &l.LossReason); err != nil {
			return nil, fmt.Errorf("get stocktake lines scan: %w", err)
		}
		st.Lines = append(st.Lines, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get stocktake lines iterate: %w", err)
	}
	return st, nil
}

// UpdateStocktakeLines replaces all lines for a stocktake within a transaction.
func (r *Repository) UpdateStocktakeLines(stocktakeID string, lines []models.StocktakeLine) error {
	dbTx, err := r.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer dbTx.Rollback()

	// Guard: confirm the stocktake belongs to this tenant before writing.
	var exists bool
	if err := dbTx.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM stocktakes WHERE id = $1 AND tenant_id = $2)`,
		stocktakeID, r.tenantID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check stocktake tenant: %w", err)
	}
	if !exists {
		return fmt.Errorf("stocktake %s not found for current tenant", stocktakeID)
	}

	// Remove existing lines
	if _, err := dbTx.Exec(`DELETE FROM stocktake_lines WHERE stocktake_id = $1`, stocktakeID); err != nil {
		return fmt.Errorf("delete stocktake lines: %w", err)
	}

	// Insert new lines
	for i := range lines {
		l := &lines[i]
		if l.ID == "" {
			l.ID = uuid.New().String()
		}
		l.StocktakeID = stocktakeID
		l.Variance = l.CountedQty - l.SystemQty
		if _, err := dbTx.Exec(
			`INSERT INTO stocktake_lines (id, stocktake_id, sku_id, batch_id, system_qty, counted_qty, variance, loss_reason)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			l.ID, l.StocktakeID, l.SKUID, l.BatchID, l.SystemQty, l.CountedQty, l.Variance, l.LossReason,
		); err != nil {
			return fmt.Errorf("insert stocktake line: %w", err)
		}
	}
	return dbTx.Commit()
}

// CompleteStocktake marks a stocktake as completed, scoped to the current tenant.
func (r *Repository) CompleteStocktake(id string) error {
	_, err := r.DB.Exec(
		`UPDATE stocktakes SET status = 'completed' WHERE id = $1 AND tenant_id = $2`,
		id, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("complete stocktake: %w", err)
	}
	return nil
}

// ---------- Learning ----------

// ListSubjects returns all learning subjects ordered by sort_order.
func (r *Repository) ListSubjects() ([]models.LearningSubject, error) {
	rows, err := r.DB.Query(
		`SELECT id, name, description, sort_order, created_at FROM learning_subjects WHERE tenant_id = $1 ORDER BY sort_order`, r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list subjects: %w", err)
	}
	defer rows.Close()

	var subjects []models.LearningSubject
	for rows.Next() {
		var s models.LearningSubject
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.SortOrder, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("list subjects scan: %w", err)
		}
		subjects = append(subjects, s)
	}
	return subjects, rows.Err()
}

// CreateSubject inserts a new learning subject.
func (r *Repository) CreateSubject(s *models.LearningSubject) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	s.CreatedAt = time.Now()
	_, err := r.DB.Exec(
		`INSERT INTO learning_subjects (id, name, description, sort_order, created_at, tenant_id)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		s.ID, s.Name, s.Description, s.SortOrder, s.CreatedAt, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create subject: %w", err)
	}
	return nil
}

// UpdateSubject updates an existing learning subject.
func (r *Repository) UpdateSubject(s *models.LearningSubject) error {
	_, err := r.DB.Exec(
		`UPDATE learning_subjects SET name=$1, description=$2, sort_order=$3 WHERE id=$4 AND tenant_id=$5`,
		s.Name, s.Description, s.SortOrder, s.ID, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("update subject: %w", err)
	}
	return nil
}

// ListChapters returns all chapters for a given subject.
func (r *Repository) ListChapters(subjectID string) ([]models.LearningChapter, error) {
	rows, err := r.DB.Query(
		`SELECT id, subject_id, name, sort_order FROM learning_chapters WHERE subject_id = $1 AND tenant_id = $2 ORDER BY sort_order`, subjectID, r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list chapters: %w", err)
	}
	defer rows.Close()

	var chapters []models.LearningChapter
	for rows.Next() {
		var c models.LearningChapter
		if err := rows.Scan(&c.ID, &c.SubjectID, &c.Name, &c.SortOrder); err != nil {
			return nil, fmt.Errorf("list chapters scan: %w", err)
		}
		chapters = append(chapters, c)
	}
	return chapters, rows.Err()
}

// CreateChapter inserts a new learning chapter.
func (r *Repository) CreateChapter(c *models.LearningChapter) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	_, err := r.DB.Exec(
		`INSERT INTO learning_chapters (id, subject_id, name, sort_order, tenant_id)
		 VALUES ($1, $2, $3, $4, $5)`,
		c.ID, c.SubjectID, c.Name, c.SortOrder, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create chapter: %w", err)
	}
	return nil
}

// ListKnowledgePoints returns paginated knowledge points for a chapter.
func (r *Repository) ListKnowledgePoints(chapterID string, page, pageSize int) ([]models.KnowledgePoint, int, error) {
	offset := (page - 1) * pageSize
	rows, err := r.DB.Query(
		`SELECT id, chapter_id, title, content, tags, classifications, created_at, updated_at,
		        COUNT(*) OVER() AS total
		 FROM knowledge_points
		 WHERE chapter_id = $1 AND tenant_id = $4
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, chapterID, pageSize, offset, r.tenantID,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list knowledge points: %w", err)
	}
	defer rows.Close()

	var kps []models.KnowledgePoint
	var total int
	for rows.Next() {
		var kp models.KnowledgePoint
		if err := rows.Scan(&kp.ID, &kp.ChapterID, &kp.Title, &kp.Content, pq.Array(&kp.Tags), &kp.Classifications, &kp.CreatedAt, &kp.UpdatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("list knowledge points scan: %w", err)
		}
		kps = append(kps, kp)
	}
	return kps, total, rows.Err()
}

// CreateKnowledgePoint inserts a new knowledge point.
func (r *Repository) CreateKnowledgePoint(kp *models.KnowledgePoint) error {
	if kp.ID == "" {
		kp.ID = uuid.New().String()
	}
	now := time.Now()
	kp.CreatedAt = now
	kp.UpdatedAt = now
	_, err := r.DB.Exec(
		`INSERT INTO knowledge_points (id, chapter_id, title, content, tags, classifications, created_at, updated_at, tenant_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		kp.ID, kp.ChapterID, kp.Title, kp.Content, pq.Array(kp.Tags), kp.Classifications, kp.CreatedAt, kp.UpdatedAt, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create knowledge point: %w", err)
	}
	return nil
}

// UpdateKnowledgePoint updates an existing knowledge point.
func (r *Repository) UpdateKnowledgePoint(kp *models.KnowledgePoint) error {
	kp.UpdatedAt = time.Now()
	_, err := r.DB.Exec(
		`UPDATE knowledge_points SET title=$1, content=$2, tags=$3, classifications=$4, updated_at=$5 WHERE id=$6 AND tenant_id=$7`,
		kp.Title, kp.Content, pq.Array(kp.Tags), kp.Classifications, kp.UpdatedAt, kp.ID, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("update knowledge point: %w", err)
	}
	return nil
}

// GetKnowledgePoint retrieves a single knowledge point by ID, scoped to the current tenant.
func (r *Repository) GetKnowledgePoint(id string) (*models.KnowledgePoint, error) {
	kp := &models.KnowledgePoint{}
	err := r.DB.QueryRow(
		`SELECT id, chapter_id, title, content, tags, classifications, created_at, updated_at
		 FROM knowledge_points WHERE id = $1 AND tenant_id = $2`, id, r.tenantID,
	).Scan(&kp.ID, &kp.ChapterID, &kp.Title, &kp.Content, pq.Array(&kp.Tags), &kp.Classifications, &kp.CreatedAt, &kp.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get knowledge point: %w", err)
	}
	return kp, nil
}

// SearchKnowledgePoints performs full-text search using tsvector.
func (r *Repository) SearchKnowledgePoints(query string, page, pageSize int) ([]models.KnowledgePoint, int, error) {
	offset := (page - 1) * pageSize

	// Build tsquery: join terms with & for AND semantics
	terms := strings.Fields(query)
	tsQuery := strings.Join(terms, " & ")

	rows, err := r.DB.Query(
		`SELECT id, chapter_id, title, content, tags, classifications, created_at, updated_at,
		        COUNT(*) OVER() AS total
		 FROM knowledge_points
		 WHERE search_vector @@ to_tsquery('english', $1) AND tenant_id = $4
		 ORDER BY ts_rank(search_vector, to_tsquery('english', $1)) DESC
		 LIMIT $2 OFFSET $3`, tsQuery, pageSize, offset, r.tenantID,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("search knowledge points: %w", err)
	}
	defer rows.Close()

	var kps []models.KnowledgePoint
	var total int
	for rows.Next() {
		var kp models.KnowledgePoint
		if err := rows.Scan(&kp.ID, &kp.ChapterID, &kp.Title, &kp.Content, pq.Array(&kp.Tags), &kp.Classifications, &kp.CreatedAt, &kp.UpdatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("search knowledge points scan: %w", err)
		}
		kps = append(kps, kp)
	}
	return kps, total, rows.Err()
}

// ---------- Work Orders ----------

// ListWorkOrders returns paginated work orders with optional status, assignee, and submitter filters.
// Pass submittedBy to restrict results to a specific submitter (used for non-admin/non-maintenance roles
// so they see only the work orders they personally created).
func (r *Repository) ListWorkOrders(status string, assignedTo string, submittedBy string, page, pageSize int) ([]models.WorkOrder, int, error) {
	offset := (page - 1) * pageSize

	query := `SELECT id, submitted_by, assigned_to, trade, priority, sla_deadline, status, description, location, parts_cost, labor_cost, rating, closed_at, created_at,
	                 COUNT(*) OVER() AS total
	          FROM work_orders WHERE tenant_id = $1`
	args := []interface{}{r.tenantID}
	argIdx := 2

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	if assignedTo != "" {
		query += fmt.Sprintf(" AND assigned_to = $%d", argIdx)
		args = append(args, assignedTo)
		argIdx++
	}
	if submittedBy != "" {
		query += fmt.Sprintf(" AND submitted_by = $%d", argIdx)
		args = append(args, submittedBy)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list work orders: %w", err)
	}
	defer rows.Close()

	var orders []models.WorkOrder
	var total int
	for rows.Next() {
		var wo models.WorkOrder
		if err := rows.Scan(&wo.ID, &wo.SubmittedBy, &wo.AssignedTo, &wo.Trade, &wo.Priority, &wo.SLADeadline, &wo.Status, &wo.Description, &wo.Location, &wo.PartsCost, &wo.LaborCost, &wo.Rating, &wo.ClosedAt, &wo.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("list work orders scan: %w", err)
		}
		orders = append(orders, wo)
	}
	return orders, total, rows.Err()
}

// GetWorkOrder retrieves a single work order by ID.
func (r *Repository) GetWorkOrder(id string) (*models.WorkOrder, error) {
	wo := &models.WorkOrder{}
	err := r.DB.QueryRow(
		`SELECT id, submitted_by, assigned_to, trade, priority, sla_deadline, status, description, location, parts_cost, labor_cost, rating, closed_at, created_at
		 FROM work_orders WHERE id = $1 AND tenant_id = $2`, id, r.tenantID,
	).Scan(&wo.ID, &wo.SubmittedBy, &wo.AssignedTo, &wo.Trade, &wo.Priority, &wo.SLADeadline, &wo.Status, &wo.Description, &wo.Location, &wo.PartsCost, &wo.LaborCost, &wo.Rating, &wo.ClosedAt, &wo.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get work order: %w", err)
	}
	return wo, nil
}

// CreateWorkOrder inserts a new work order.
func (r *Repository) CreateWorkOrder(wo *models.WorkOrder) error {
	if wo.ID == "" {
		wo.ID = uuid.New().String()
	}
	wo.CreatedAt = time.Now()
	_, err := r.DB.Exec(
		`INSERT INTO work_orders (id, submitted_by, assigned_to, trade, priority, sla_deadline, status, description, location, parts_cost, labor_cost, rating, closed_at, created_at, tenant_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		wo.ID, wo.SubmittedBy, wo.AssignedTo, wo.Trade, wo.Priority, wo.SLADeadline, wo.Status, wo.Description, wo.Location, wo.PartsCost, wo.LaborCost, wo.Rating, wo.ClosedAt, wo.CreatedAt, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create work order: %w", err)
	}
	return nil
}

// UpdateWorkOrder updates an existing work order.
func (r *Repository) UpdateWorkOrder(wo *models.WorkOrder) error {
	_, err := r.DB.Exec(
		`UPDATE work_orders SET assigned_to=$1, trade=$2, priority=$3, sla_deadline=$4, status=$5, description=$6, location=$7, parts_cost=$8, labor_cost=$9, rating=$10, closed_at=$11
		 WHERE id=$12 AND tenant_id=$13`,
		wo.AssignedTo, wo.Trade, wo.Priority, wo.SLADeadline, wo.Status, wo.Description, wo.Location, wo.PartsCost, wo.LaborCost, wo.Rating, wo.ClosedAt, wo.ID, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("update work order: %w", err)
	}
	return nil
}

// GetTechnicianWithLeastOrders finds the active technician with the fewest open/in-progress orders for auto-dispatch.
// Only work orders belonging to the current tenant are counted so cross-tenant workload does not affect selection.
func (r *Repository) GetTechnicianWithLeastOrders(trade string) (*models.User, error) {
	u := &models.User{}
	err := r.DB.QueryRow(
		`SELECT u.id, u.username, u.password_hash, u.role, u.failed_attempts, u.locked_until, u.is_active, u.must_change_password, u.created_at, u.updated_at
		 FROM auth_users u
		 WHERE u.role = 'maintenance_tech' AND u.is_active = true AND u.tenant_id = $1
		 ORDER BY (
		   SELECT COUNT(*) FROM work_orders wo
		   WHERE wo.assigned_to = u.id AND wo.status IN ('submitted', 'dispatched', 'in_progress')
		     AND wo.tenant_id = $1
		 ) ASC
		 LIMIT 1`, r.tenantID,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.FailedAttempts, &u.LockedUntil, &u.IsActive, &u.MustChangePassword, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get technician with least orders: %w", err)
	}
	return u, nil
}

// GetWorkOrderAnalytics returns basic work order statistics scoped to the current tenant.
func (r *Repository) GetWorkOrderAnalytics() (map[string]interface{}, error) {
	analytics := make(map[string]interface{})

	// Count by status (tenant-scoped)
	rows, err := r.DB.Query(
		`SELECT status, COUNT(*) FROM work_orders WHERE tenant_id = $1 GROUP BY status`, r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("work order analytics by_status: %w", err)
	}
	defer rows.Close()

	statusCounts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("work order analytics by_status scan: %w", err)
		}
		statusCounts[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	analytics["by_status"] = statusCounts

	// Average resolution time (hours) for closed orders (tenant-scoped)
	var avgHours sql.NullFloat64
	err = r.DB.QueryRow(
		`SELECT AVG(EXTRACT(EPOCH FROM (closed_at - created_at)) / 3600)
		 FROM work_orders WHERE tenant_id = $1 AND status = 'closed' AND closed_at IS NOT NULL`, r.tenantID,
	).Scan(&avgHours)
	if err != nil {
		return nil, fmt.Errorf("work order analytics avg_resolution: %w", err)
	}
	if avgHours.Valid {
		analytics["avg_resolution_hours"] = avgHours.Float64
	} else {
		analytics["avg_resolution_hours"] = 0
	}

	// Total costs for closed orders (tenant-scoped)
	var totalParts, totalLabor sql.NullFloat64
	err = r.DB.QueryRow(
		`SELECT COALESCE(SUM(parts_cost), 0), COALESCE(SUM(labor_cost), 0)
		 FROM work_orders WHERE tenant_id = $1 AND status = 'closed'`, r.tenantID,
	).Scan(&totalParts, &totalLabor)
	if err != nil {
		return nil, fmt.Errorf("work order analytics costs: %w", err)
	}
	analytics["total_parts_cost"] = totalParts.Float64
	analytics["total_labor_cost"] = totalLabor.Float64

	// Average rating (tenant-scoped)
	var avgRating sql.NullFloat64
	err = r.DB.QueryRow(
		`SELECT AVG(rating) FROM work_orders WHERE tenant_id = $1 AND rating IS NOT NULL`, r.tenantID,
	).Scan(&avgRating)
	if err != nil {
		return nil, fmt.Errorf("work order analytics avg_rating: %w", err)
	}
	if avgRating.Valid {
		analytics["avg_rating"] = avgRating.Float64
	} else {
		analytics["avg_rating"] = 0
	}

	return analytics, nil
}

// ---------- Members ----------

// scanMemberStoredValue decrypts stored_value_encrypted if present, otherwise
// falls back to the plaintext stored_value column (backward-compat for migrated rows).
func (r *Repository) scanMemberStoredValue(encBytes []byte, plaintext float64) float64 {
	if len(encBytes) > 0 {
		if v, err := r.decryptDecimal(encBytes); err == nil {
			return v
		}
	}
	return plaintext
}

// ListMembers returns paginated members with optional search on name/phone.
func (r *Repository) ListMembers(search string, page, pageSize int) ([]models.Member, int, error) {
	offset := (page - 1) * pageSize
	var rows *sql.Rows
	var err error

	if search != "" {
		pattern := "%" + search + "%"
		rows, err = r.DB.Query(
			`SELECT id, name, id_number_encrypted, phone, tier_id, points_balance, stored_value, stored_value_encrypted, status, frozen_at, expires_at, created_at,
			        verification_status_encrypted, deposits_encrypted, violation_notes_encrypted,
			        COUNT(*) OVER() AS total
			 FROM members
			 WHERE (name ILIKE $1 OR phone ILIKE $1) AND tenant_id = $2
			 ORDER BY name
			 LIMIT $3 OFFSET $4`, pattern, r.tenantID, pageSize, offset,
		)
	} else {
		rows, err = r.DB.Query(
			`SELECT id, name, id_number_encrypted, phone, tier_id, points_balance, stored_value, stored_value_encrypted, status, frozen_at, expires_at, created_at,
			        verification_status_encrypted, deposits_encrypted, violation_notes_encrypted,
			        COUNT(*) OVER() AS total
			 FROM members
			 WHERE tenant_id = $1
			 ORDER BY name
			 LIMIT $2 OFFSET $3`, r.tenantID, pageSize, offset,
		)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	var members []models.Member
	var total int
	for rows.Next() {
		var m models.Member
		var plainSV float64
		var encSV []byte
		if err := rows.Scan(&m.ID, &m.Name, &m.IDNumberEncrypted, &m.Phone, &m.TierID, &m.PointsBalance, &plainSV, &encSV, &m.Status, &m.FrozenAt, &m.ExpiresAt, &m.CreatedAt,
			&m.VerificationStatusEncrypted, &m.DepositsEncrypted, &m.ViolationNotesEncrypted, &total); err != nil {
			return nil, 0, fmt.Errorf("list members scan: %w", err)
		}
		m.StoredValue = r.scanMemberStoredValue(encSV, plainSV)
		members = append(members, m)
	}
	return members, total, rows.Err()
}

// GetMember retrieves a single member by ID.
func (r *Repository) GetMember(id string) (*models.Member, error) {
	m := &models.Member{}
	var plainSV float64
	var encSV []byte
	err := r.DB.QueryRow(
		`SELECT id, name, id_number_encrypted, phone, tier_id, points_balance, stored_value, stored_value_encrypted, status, frozen_at, expires_at, created_at,
		        verification_status_encrypted, deposits_encrypted, violation_notes_encrypted
		 FROM members WHERE id = $1 AND tenant_id = $2`, id, r.tenantID,
	).Scan(&m.ID, &m.Name, &m.IDNumberEncrypted, &m.Phone, &m.TierID, &m.PointsBalance, &plainSV, &encSV, &m.Status, &m.FrozenAt, &m.ExpiresAt, &m.CreatedAt,
		&m.VerificationStatusEncrypted, &m.DepositsEncrypted, &m.ViolationNotesEncrypted)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get member: %w", err)
	}
	m.StoredValue = r.scanMemberStoredValue(encSV, plainSV)
	return m, nil
}

// CreateMember inserts a new member, encrypting stored_value into stored_value_encrypted.
func (r *Repository) CreateMember(m *models.Member) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	m.CreatedAt = time.Now()
	encSV, err := r.encryptDecimal(m.StoredValue)
	if err != nil {
		return fmt.Errorf("create member: encrypt stored_value: %w", err)
	}
	_, err = r.DB.Exec(
		`INSERT INTO members (id, name, id_number_encrypted, phone, tier_id, points_balance, stored_value, stored_value_encrypted, status, frozen_at, expires_at, created_at, tenant_id, verification_status_encrypted, deposits_encrypted, violation_notes_encrypted)
		 VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		m.ID, m.Name, m.IDNumberEncrypted, m.Phone, m.TierID, m.PointsBalance, encSV, m.Status, m.FrozenAt, m.ExpiresAt, m.CreatedAt, r.tenantID,
		m.VerificationStatusEncrypted, m.DepositsEncrypted, m.ViolationNotesEncrypted,
	)
	if err != nil {
		return fmt.Errorf("create member: %w", err)
	}
	return nil
}

// UpdateMember updates an existing member, encrypting stored_value into stored_value_encrypted.
func (r *Repository) UpdateMember(m *models.Member) error {
	encSV, err := r.encryptDecimal(m.StoredValue)
	if err != nil {
		return fmt.Errorf("update member: encrypt stored_value: %w", err)
	}
	_, err = r.DB.Exec(
		`UPDATE members SET name=$1, id_number_encrypted=$2, phone=$3, tier_id=$4, points_balance=$5, stored_value=0, stored_value_encrypted=$6, status=$7, frozen_at=$8, expires_at=$9
		 WHERE id=$10 AND tenant_id=$11`,
		m.Name, m.IDNumberEncrypted, m.Phone, m.TierID, m.PointsBalance, encSV, m.Status, m.FrozenAt, m.ExpiresAt, m.ID, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("update member: %w", err)
	}
	return nil
}

// ListMemberTransactions returns paginated transactions for a member.
func (r *Repository) ListMemberTransactions(memberID string, page, pageSize int) ([]models.MemberTransaction, int, error) {
	offset := (page - 1) * pageSize
	rows, err := r.DB.Query(
		`SELECT id, member_id, type, amount, description, performed_by, created_at,
		        COUNT(*) OVER() AS total
		 FROM member_transactions
		 WHERE member_id = $1 AND tenant_id = $4
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, memberID, pageSize, offset, r.tenantID,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list member transactions: %w", err)
	}
	defer rows.Close()

	var txs []models.MemberTransaction
	var total int
	for rows.Next() {
		var t models.MemberTransaction
		if err := rows.Scan(&t.ID, &t.MemberID, &t.Type, &t.Amount, &t.Description, &t.PerformedBy, &t.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("list member transactions scan: %w", err)
		}
		txs = append(txs, t)
	}
	return txs, total, rows.Err()
}

// CreateMemberTransaction inserts a new member transaction.
func (r *Repository) CreateMemberTransaction(tx *models.MemberTransaction) error {
	if tx.ID == "" {
		tx.ID = uuid.New().String()
	}
	tx.CreatedAt = time.Now()
	_, err := r.DB.Exec(
		`INSERT INTO member_transactions (id, member_id, type, amount, description, performed_by, created_at, tenant_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		tx.ID, tx.MemberID, tx.Type, tx.Amount, tx.Description, tx.PerformedBy, tx.CreatedAt, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create member transaction: %w", err)
	}
	return nil
}

// GetSessionPackages retrieves all session packages for a member, scoped to the current tenant
// via a JOIN through the members table.
func (r *Repository) GetSessionPackages(memberID string) ([]models.SessionPackage, error) {
	rows, err := r.DB.Query(
		`SELECT sp.id, sp.member_id, sp.package_name, sp.total_sessions, sp.remaining_sessions, sp.expires_at
		 FROM session_packages sp
		 JOIN members m ON m.id = sp.member_id
		 WHERE sp.member_id = $1 AND m.tenant_id = $2
		 ORDER BY sp.expires_at`, memberID, r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("get session packages: %w", err)
	}
	defer rows.Close()

	var pkgs []models.SessionPackage
	for rows.Next() {
		var p models.SessionPackage
		if err := rows.Scan(&p.ID, &p.MemberID, &p.PackageName, &p.TotalSessions, &p.RemainingSessions, &p.ExpiresAt); err != nil {
			return nil, fmt.Errorf("get session packages scan: %w", err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, rows.Err()
}

// UpdateSessionPackage updates an existing session package, scoped to the current tenant
// by requiring the associated member to belong to this tenant.
func (r *Repository) UpdateSessionPackage(pkg *models.SessionPackage) error {
	_, err := r.DB.Exec(
		`UPDATE session_packages SET package_name=$1, total_sessions=$2, remaining_sessions=$3, expires_at=$4
		 WHERE id=$5 AND member_id IN (SELECT id FROM members WHERE tenant_id=$6)`,
		pkg.PackageName, pkg.TotalSessions, pkg.RemainingSessions, pkg.ExpiresAt, pkg.ID, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("update session package: %w", err)
	}
	return nil
}

// CreateSessionPackage inserts a new session package, enforcing that the member belongs
// to the current tenant before inserting.
func (r *Repository) CreateSessionPackage(pkg *models.SessionPackage) error {
	if pkg.ID == "" {
		pkg.ID = uuid.New().String()
	}
	res, err := r.DB.Exec(
		`INSERT INTO session_packages (id, member_id, package_name, total_sessions, remaining_sessions, expires_at)
		 SELECT $1, $2, $3, $4, $5, $6
		 WHERE EXISTS (SELECT 1 FROM members WHERE id = $2 AND tenant_id = $7)`,
		pkg.ID, pkg.MemberID, pkg.PackageName, pkg.TotalSessions, pkg.RemainingSessions, pkg.ExpiresAt, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create session package: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("create session package: member not found in tenant")
	}
	return nil
}

// ListMembershipTiers returns all membership tiers ordered by sort_order.
// Membership tiers are intentionally a shared global catalog — the same tier
// definitions (e.g. "Gold", "Silver") apply across every tenant on the
// deployment. They carry no PHI and contain no per-tenant data. Scoping them
// per-tenant would require duplicating catalogue rows for each tenant, which
// adds operational burden with no security benefit. This is a deliberate
// architectural decision: tier definitions are managed at the platform level,
// while the per-tenant data (member.tier_id FK + member.tenant_id) enforces
// isolation for actual member records.
func (r *Repository) ListMembershipTiers() ([]models.MembershipTier, error) {
	rows, err := r.DB.Query(
		`SELECT id, name, benefits, sort_order FROM membership_tiers ORDER BY sort_order`,
	)
	if err != nil {
		return nil, fmt.Errorf("list membership tiers: %w", err)
	}
	defer rows.Close()

	var tiers []models.MembershipTier
	for rows.Next() {
		var t models.MembershipTier
		if err := rows.Scan(&t.ID, &t.Name, &t.Benefits, &t.SortOrder); err != nil {
			return nil, fmt.Errorf("list membership tiers scan: %w", err)
		}
		tiers = append(tiers, t)
	}
	return tiers, rows.Err()
}

// ---------- Rate Tables & Charges ----------

// ListRateTables returns all rate tables.
func (r *Repository) ListRateTables() ([]models.RateTable, error) {
	rows, err := r.DB.Query(
		`SELECT id, name, type, tiers, fuel_surcharge_pct, taxable, effective_date FROM rate_tables WHERE tenant_id = $1 ORDER BY name`, r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list rate tables: %w", err)
	}
	defer rows.Close()

	var tables []models.RateTable
	for rows.Next() {
		var rt models.RateTable
		if err := rows.Scan(&rt.ID, &rt.Name, &rt.Type, &rt.Tiers, &rt.FuelSurchargePct, &rt.Taxable, &rt.EffectiveDate); err != nil {
			return nil, fmt.Errorf("list rate tables scan: %w", err)
		}
		tables = append(tables, rt)
	}
	return tables, rows.Err()
}

// CreateRateTable inserts a new rate table.
func (r *Repository) CreateRateTable(rt *models.RateTable) error {
	if rt.ID == "" {
		rt.ID = uuid.New().String()
	}
	_, err := r.DB.Exec(
		`INSERT INTO rate_tables (id, name, type, tiers, fuel_surcharge_pct, taxable, effective_date, tenant_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		rt.ID, rt.Name, rt.Type, rt.Tiers, rt.FuelSurchargePct, rt.Taxable, rt.EffectiveDate, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create rate table: %w", err)
	}
	return nil
}

// UpdateRateTable updates an existing rate table.
func (r *Repository) UpdateRateTable(rt *models.RateTable) error {
	_, err := r.DB.Exec(
		`UPDATE rate_tables SET name=$1, type=$2, tiers=$3, fuel_surcharge_pct=$4, taxable=$5, effective_date=$6 WHERE id=$7 AND tenant_id=$8`,
		rt.Name, rt.Type, rt.Tiers, rt.FuelSurchargePct, rt.Taxable, rt.EffectiveDate, rt.ID, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("update rate table: %w", err)
	}
	return nil
}

// ListStatements returns paginated charge statements.
func (r *Repository) ListStatements(page, pageSize int) ([]models.ChargeStatement, int, error) {
	offset := (page - 1) * pageSize
	rows, err := r.DB.Query(
		`SELECT id, period_start, period_end, total_amount, expected_total, status,
		        approved_by_1, approved_by_2, reconciled_at, variance_notes, paid_at, created_at,
		        COUNT(*) OVER() AS total
		 FROM charge_statements
		 WHERE tenant_id = $3
		 ORDER BY created_at DESC
		 LIMIT $1 OFFSET $2`, pageSize, offset, r.tenantID,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list statements: %w", err)
	}
	defer rows.Close()

	var stmts []models.ChargeStatement
	var total int
	for rows.Next() {
		var s models.ChargeStatement
		if err := rows.Scan(&s.ID, &s.PeriodStart, &s.PeriodEnd, &s.TotalAmount, &s.ExpectedTotal,
			&s.Status, &s.ApprovedBy1, &s.ApprovedBy2, &s.ReconciledAt, &s.VarianceNotes, &s.PaidAt, &s.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("list statements scan: %w", err)
		}
		stmts = append(stmts, s)
	}
	return stmts, total, rows.Err()
}

// GetStatement retrieves a single charge statement by ID, scoped to the current tenant.
func (r *Repository) GetStatement(id string) (*models.ChargeStatement, error) {
	s := &models.ChargeStatement{}
	err := r.DB.QueryRow(
		`SELECT id, period_start, period_end, total_amount, expected_total, status,
		        approved_by_1, approved_by_2, reconciled_at, variance_notes, paid_at, created_at
		 FROM charge_statements WHERE id = $1 AND tenant_id = $2`, id, r.tenantID,
	).Scan(&s.ID, &s.PeriodStart, &s.PeriodEnd, &s.TotalAmount, &s.ExpectedTotal, &s.Status,
		&s.ApprovedBy1, &s.ApprovedBy2, &s.ReconciledAt, &s.VarianceNotes, &s.PaidAt, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get statement: %w", err)
	}
	return s, nil
}

// CreateStatement inserts a new charge statement.
func (r *Repository) CreateStatement(stmt *models.ChargeStatement) error {
	if stmt.ID == "" {
		stmt.ID = uuid.New().String()
	}
	stmt.CreatedAt = time.Now()
	_, err := r.DB.Exec(
		`INSERT INTO charge_statements (id, period_start, period_end, total_amount, expected_total, status,
		        approved_by_1, approved_by_2, reconciled_at, variance_notes, paid_at, created_at, tenant_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		stmt.ID, stmt.PeriodStart, stmt.PeriodEnd, stmt.TotalAmount, stmt.ExpectedTotal, stmt.Status,
		stmt.ApprovedBy1, stmt.ApprovedBy2, stmt.ReconciledAt, stmt.VarianceNotes, stmt.PaidAt, stmt.CreatedAt, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create statement: %w", err)
	}
	return nil
}

// UpdateStatement updates an existing charge statement.
func (r *Repository) UpdateStatement(stmt *models.ChargeStatement) error {
	_, err := r.DB.Exec(
		`UPDATE charge_statements
		 SET period_start=$1, period_end=$2, total_amount=$3, expected_total=$4,
		     status=$5, approved_by_1=$6, approved_by_2=$7, reconciled_at=$8,
		     variance_notes=$9, paid_at=$10
		 WHERE id=$11 AND tenant_id=$12`,
		stmt.PeriodStart, stmt.PeriodEnd, stmt.TotalAmount, stmt.ExpectedTotal,
		stmt.Status, stmt.ApprovedBy1, stmt.ApprovedBy2, stmt.ReconciledAt,
		stmt.VarianceNotes, stmt.PaidAt, stmt.ID, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("update statement: %w", err)
	}
	return nil
}

// GetStatementLineItems retrieves all line items for a charge statement, scoped to the
// current tenant via a JOIN through the charge_statements table.
func (r *Repository) GetStatementLineItems(statementID string) ([]models.ChargeLineItem, error) {
	rows, err := r.DB.Query(
		`SELECT cli.id, cli.statement_id, cli.description, cli.quantity, cli.unit_price, cli.surcharge, cli.tax, cli.total
		 FROM charge_line_items cli
		 JOIN charge_statements cs ON cs.id = cli.statement_id
		 WHERE cli.statement_id = $1 AND cs.tenant_id = $2
		 ORDER BY cli.id`, statementID, r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("get statement line items: %w", err)
	}
	defer rows.Close()

	var items []models.ChargeLineItem
	for rows.Next() {
		var item models.ChargeLineItem
		if err := rows.Scan(&item.ID, &item.StatementID, &item.Description, &item.Quantity, &item.UnitPrice, &item.Surcharge, &item.Tax, &item.Total); err != nil {
			return nil, fmt.Errorf("get statement line items scan: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// CreateLineItem inserts a new charge line item, enforcing that the parent statement
// belongs to the current tenant before inserting.
func (r *Repository) CreateLineItem(item *models.ChargeLineItem) error {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	res, err := r.DB.Exec(
		`INSERT INTO charge_line_items (id, statement_id, description, quantity, unit_price, surcharge, tax, total)
		 SELECT $1, $2, $3, $4, $5, $6, $7, $8
		 WHERE EXISTS (SELECT 1 FROM charge_statements WHERE id = $2 AND tenant_id = $9)`,
		item.ID, item.StatementID, item.Description, item.Quantity, item.UnitPrice, item.Surcharge, item.Tax, item.Total, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create line item: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("create line item: statement not found in tenant")
	}
	return nil
}

// ---------- Files ----------

// GetFileByHash retrieves a managed file by its SHA-256 hash.
func (r *Repository) GetFileByHash(sha256 string) (*models.ManagedFile, error) {
	f := &models.ManagedFile{}
	err := r.DB.QueryRow(
		`SELECT id, sha256, original_name, mime_type, size_bytes, storage_path, uploaded_by, retention_until, created_at
		 FROM managed_files WHERE sha256 = $1 AND tenant_id = $2`, sha256, r.tenantID,
	).Scan(&f.ID, &f.SHA256, &f.OriginalName, &f.MimeType, &f.SizeBytes, &f.StoragePath, &f.UploadedBy, &f.RetentionUntil, &f.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get file by hash: %w", err)
	}
	return f, nil
}

// CreateFile inserts a new managed file record.
func (r *Repository) CreateFile(f *models.ManagedFile) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	f.CreatedAt = time.Now()
	_, err := r.DB.Exec(
		`INSERT INTO managed_files (id, sha256, original_name, mime_type, size_bytes, storage_path, uploaded_by, retention_until, created_at, tenant_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		f.ID, f.SHA256, f.OriginalName, f.MimeType, f.SizeBytes, f.StoragePath, f.UploadedBy, f.RetentionUntil, f.CreatedAt, r.tenantID,
	)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	return nil
}

// GetFile retrieves a managed file by ID.
func (r *Repository) GetFile(id string) (*models.ManagedFile, error) {
	f := &models.ManagedFile{}
	err := r.DB.QueryRow(
		`SELECT id, sha256, original_name, mime_type, size_bytes, storage_path, uploaded_by, retention_until, created_at
		 FROM managed_files WHERE id = $1 AND tenant_id = $2`, id, r.tenantID,
	).Scan(&f.ID, &f.SHA256, &f.OriginalName, &f.MimeType, &f.SizeBytes, &f.StoragePath, &f.UploadedBy, &f.RetentionUntil, &f.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get file: %w", err)
	}
	return f, nil
}

// ---------- Drafts ----------

// SaveDraft upserts a draft checkpoint by user_id+form_type+form_id.
func (r *Repository) SaveDraft(d *models.DraftCheckpoint) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	d.SavedAt = time.Now()
	_, err := r.DB.Exec(
		`INSERT INTO draft_checkpoints (id, user_id, form_type, form_id, state_json, saved_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (user_id, form_type, form_id) DO UPDATE
		 SET state_json = EXCLUDED.state_json, saved_at = EXCLUDED.saved_at`,
		d.ID, d.UserID, d.FormType, d.FormID, d.StateJSON, d.SavedAt,
	)
	if err != nil {
		return fmt.Errorf("save draft: %w", err)
	}
	return nil
}

// ListDrafts returns all drafts for a given user.
func (r *Repository) ListDrafts(userID string) ([]models.DraftCheckpoint, error) {
	rows, err := r.DB.Query(
		`SELECT id, user_id, form_type, form_id, state_json, saved_at
		 FROM draft_checkpoints WHERE user_id = $1 ORDER BY saved_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}
	defer rows.Close()

	var drafts []models.DraftCheckpoint
	for rows.Next() {
		var d models.DraftCheckpoint
		if err := rows.Scan(&d.ID, &d.UserID, &d.FormType, &d.FormID, &d.StateJSON, &d.SavedAt); err != nil {
			return nil, fmt.Errorf("list drafts scan: %w", err)
		}
		drafts = append(drafts, d)
	}
	return drafts, rows.Err()
}

// GetDraft retrieves a specific draft by user_id, form_type, and form_id.
func (r *Repository) GetDraft(userID, formType, formID string) (*models.DraftCheckpoint, error) {
	d := &models.DraftCheckpoint{}
	err := r.DB.QueryRow(
		`SELECT id, user_id, form_type, form_id, state_json, saved_at
		 FROM draft_checkpoints WHERE user_id = $1 AND form_type = $2 AND form_id = $3`,
		userID, formType, formID,
	).Scan(&d.ID, &d.UserID, &d.FormType, &d.FormID, &d.StateJSON, &d.SavedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get draft: %w", err)
	}
	return d, nil
}

// DeleteDraft removes a draft by user_id, form_type, and form_id.
func (r *Repository) DeleteDraft(userID, formType, formID string) error {
	_, err := r.DB.Exec(
		`DELETE FROM draft_checkpoints WHERE user_id = $1 AND form_type = $2 AND form_id = $3`,
		userID, formType, formID,
	)
	if err != nil {
		return fmt.Errorf("delete draft: %w", err)
	}
	return nil
}

// ---------- Audit ----------

// CreateAuditLog inserts a new audit log entry.
// F-003: always logs a structured warning on failure so audit gaps are never silent.
func (r *Repository) CreateAuditLog(entry *models.AuditLogEntry) error {
	entry.CreatedAt = time.Now()
	_, err := r.DB.Exec(
		`INSERT INTO audit_log (user_id, action, entity_type, entity_id, details, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		entry.UserID, entry.Action, entry.EntityType, entry.EntityID, entry.Details, entry.CreatedAt,
	)
	if err != nil {
		wrapped := fmt.Errorf("create audit log: %w", err)
		log.WithError(wrapped).WithFields(log.Fields{
			"action":      entry.Action,
			"entity_type": entry.EntityType,
			"entity_id":   entry.EntityID,
		}).Warn("Audit log insert failed — compliance gap")
		return wrapped
	}
	return nil
}

// ---------- Aliases for handler compatibility ----------

// GetSKUByID is an alias for GetSKU.
func (r *Repository) GetSKUByID(id string) (*models.SKU, error) {
	return r.GetSKU(id)
}

// GetBatchesBySKUID is an alias for GetBatchesBySKU.
func (r *Repository) GetBatchesBySKUID(skuID string) ([]models.InventoryBatch, error) {
	return r.GetBatchesBySKU(skuID)
}

// GetBatchByID is an alias for GetBatch.
func (r *Repository) GetBatchByID(id string) (*models.InventoryBatch, error) {
	return r.GetBatch(id)
}

// UpdateBatch updates a batch by setting its quantity_on_hand.
func (r *Repository) UpdateBatch(batch *models.InventoryBatch) error {
	return r.UpdateBatchQuantity(batch.ID, batch.QuantityOnHand)
}

// GetStocktakeByID is an alias for GetStocktake.
func (r *Repository) GetStocktakeByID(id string) (*models.Stocktake, error) {
	return r.GetStocktake(id)
}

// GetKnowledgePointByID is an alias for GetKnowledgePoint.
func (r *Repository) GetKnowledgePointByID(id string) (*models.KnowledgePoint, error) {
	return r.GetKnowledgePoint(id)
}

// GetWorkOrderByID is an alias for GetWorkOrder.
func (r *Repository) GetWorkOrderByID(id string) (*models.WorkOrder, error) {
	return r.GetWorkOrder(id)
}

// GetTechWithLeastOrders returns the ID of the technician with the fewest active orders.
func (r *Repository) GetTechWithLeastOrders(trade string) (string, error) {
	u, err := r.GetTechnicianWithLeastOrders(trade)
	if err != nil {
		return "", err
	}
	if u == nil {
		return "", fmt.Errorf("no available technician found")
	}
	return u.ID, nil
}

// GetMemberByID is an alias for GetMember.
func (r *Repository) GetMemberByID(id string) (*models.Member, error) {
	return r.GetMember(id)
}

// GetSessionPackage retrieves a single session package by its ID.
func (r *Repository) GetSessionPackage(id string) (*models.SessionPackage, error) {
	p := &models.SessionPackage{}
	// session_packages has no tenant_id column; scope via the parent member's tenant_id.
	err := r.DB.QueryRow(
		`SELECT sp.id, sp.member_id, sp.package_name, sp.total_sessions, sp.remaining_sessions, sp.expires_at
		 FROM session_packages sp
		 JOIN members m ON m.id = sp.member_id AND m.tenant_id = $2
		 WHERE sp.id = $1`, id, r.tenantID,
	).Scan(&p.ID, &p.MemberID, &p.PackageName, &p.TotalSessions, &p.RemainingSessions, &p.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session package: %w", err)
	}
	return p, nil
}

// GetLatestStoredValueAdd retrieves the most recent stored_value_add transaction for a member,
// scoped to the current tenant.
func (r *Repository) GetLatestStoredValueAdd(memberID string) (*models.MemberTransaction, error) {
	t := &models.MemberTransaction{}
	err := r.DB.QueryRow(
		`SELECT id, member_id, type, amount, description, performed_by, created_at
		 FROM member_transactions
		 WHERE member_id = $1 AND type = 'stored_value_add' AND tenant_id = $2
		 ORDER BY created_at DESC LIMIT 1`, memberID, r.tenantID,
	).Scan(&t.ID, &t.MemberID, &t.Type, &t.Amount, &t.Description, &t.PerformedBy, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest stored value add: %w", err)
	}
	return t, nil
}

// ListTiers is an alias for ListMembershipTiers.
func (r *Repository) ListTiers() ([]models.MembershipTier, error) {
	return r.ListMembershipTiers()
}

// GetRateTableByID retrieves a single rate table by ID.
func (r *Repository) GetRateTableByID(id string) (*models.RateTable, error) {
	rt := &models.RateTable{}
	err := r.DB.QueryRow(
		`SELECT id, name, type, tiers, fuel_surcharge_pct, taxable, effective_date FROM rate_tables WHERE id = $1 AND tenant_id = $2`, id, r.tenantID,
	).Scan(&rt.ID, &rt.Name, &rt.Type, &rt.Tiers, &rt.FuelSurchargePct, &rt.Taxable, &rt.EffectiveDate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get rate table by id: %w", err)
	}
	return rt, nil
}

// GetStatementByID is an alias for GetStatement.
func (r *Repository) GetStatementByID(id string) (*models.ChargeStatement, error) {
	return r.GetStatement(id)
}

// CreateChargeLineItem is an alias for CreateLineItem.
func (r *Repository) CreateChargeLineItem(item *models.ChargeLineItem) error {
	return r.CreateLineItem(item)
}

// GetFileByID is an alias for GetFile.
func (r *Repository) GetFileByID(id string) (*models.ManagedFile, error) {
	return r.GetFile(id)
}

// GetFilesByIDs retrieves multiple managed files by their IDs.
func (r *Repository) GetFilesByIDs(ids []string) ([]models.ManagedFile, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.DB.Query(
		`SELECT id, sha256, original_name, mime_type, size_bytes, storage_path, uploaded_by, retention_until, created_at
		 FROM managed_files WHERE id = ANY($1) AND tenant_id = $2`, pq.Array(ids), r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("get files by ids: %w", err)
	}
	defer rows.Close()

	var files []models.ManagedFile
	for rows.Next() {
		var f models.ManagedFile
		if err := rows.Scan(&f.ID, &f.SHA256, &f.OriginalName, &f.MimeType, &f.SizeBytes, &f.StoragePath, &f.UploadedBy, &f.RetentionUntil, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("get files by ids scan: %w", err)
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// LinkPhotoToWorkOrder creates a persistent link between a work order and a managed file,
// enforcing that the work order belongs to the current tenant.
func (r *Repository) LinkPhotoToWorkOrder(workOrderID, fileID string) (*models.WorkOrderPhoto, error) {
	photo := &models.WorkOrderPhoto{
		ID:          uuid.New().String(),
		WorkOrderID: workOrderID,
		FileID:      fileID,
		CreatedAt:   time.Now(),
	}
	_, err := r.DB.Exec(
		`INSERT INTO work_order_photos (id, work_order_id, file_id, created_at)
		 SELECT $1, $2, $3, $4
		 WHERE EXISTS (SELECT 1 FROM work_orders   WHERE id = $2 AND tenant_id = $5)
		   AND EXISTS (SELECT 1 FROM managed_files WHERE id = $3 AND tenant_id = $5)
		 ON CONFLICT (work_order_id, file_id) DO NOTHING`,
		photo.ID, photo.WorkOrderID, photo.FileID, photo.CreatedAt, r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("link photo to work order: %w", err)
	}
	return photo, nil
}

// GetWorkOrderPhotos returns all managed files linked to the given work order.
func (r *Repository) GetWorkOrderPhotos(workOrderID string) ([]models.ManagedFile, error) {
	rows, err := r.DB.Query(
		`SELECT f.id, f.sha256, f.original_name, f.mime_type, f.size_bytes, f.storage_path,
		        f.uploaded_by, f.retention_until, f.created_at
		 FROM managed_files f
		 JOIN work_order_photos wop ON wop.file_id = f.id
		 JOIN work_orders wo ON wo.id = wop.work_order_id AND wo.tenant_id = $2
		 WHERE wop.work_order_id = $1 AND f.tenant_id = $2
		 ORDER BY wop.created_at`, workOrderID, r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("get work order photos: %w", err)
	}
	defer rows.Close()

	var files []models.ManagedFile
	for rows.Next() {
		var f models.ManagedFile
		if err := rows.Scan(&f.ID, &f.SHA256, &f.OriginalName, &f.MimeType, &f.SizeBytes,
			&f.StoragePath, &f.UploadedBy, &f.RetentionUntil, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("get work order photos scan: %w", err)
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// IsFileLinkedToUserWorkOrder returns true when the given file is attached (via
// work_order_photos) to any work order where userID is either the submitter or the
// assigned technician, within this tenant.
// This is used to grant maintenance technicians download access to photos on their
// assigned orders even when they are not the original file uploader.
func (r *Repository) IsFileLinkedToUserWorkOrder(fileID, userID string) (bool, error) {
	var count int
	err := r.DB.QueryRow(
		`SELECT COUNT(*) FROM work_order_photos wop
		 JOIN work_orders wo ON wo.id = wop.work_order_id
		 WHERE wop.file_id = $1
		   AND (wo.submitted_by = $2 OR wo.assigned_to = $2)
		   AND wo.tenant_id = $3`,
		fileID, userID, r.tenantID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("is file linked to user work order: %w", err)
	}
	return count > 0, nil
}

// ListExpiredFiles returns managed files whose retention_until is set and has passed.
func (r *Repository) ListExpiredFiles() ([]models.ManagedFile, error) {
	rows, err := r.DB.Query(
		`SELECT id, sha256, original_name, mime_type, size_bytes, storage_path, uploaded_by, retention_until, created_at
		 FROM managed_files
		 WHERE retention_until IS NOT NULL AND retention_until < NOW() AND tenant_id = $1`, r.tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list expired files: %w", err)
	}
	defer rows.Close()

	var files []models.ManagedFile
	for rows.Next() {
		var f models.ManagedFile
		if err := rows.Scan(&f.ID, &f.SHA256, &f.OriginalName, &f.MimeType, &f.SizeBytes, &f.StoragePath, &f.UploadedBy, &f.RetentionUntil, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("list expired files scan: %w", err)
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// DeleteFileRecord removes a managed file record from the database.
func (r *Repository) DeleteFileRecord(id string) error {
	_, err := r.DB.Exec(`DELETE FROM managed_files WHERE id = $1 AND tenant_id = $2`, id, r.tenantID)
	if err != nil {
		return fmt.Errorf("delete file record %s: %w", id, err)
	}
	return nil
}


// GetConfig returns system configuration as a key-value map.
func (r *Repository) GetConfig() (map[string]string, error) {
	rows, err := r.DB.Query(`SELECT key, value FROM system_config ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}
	defer rows.Close()

	cfg := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("get config scan: %w", err)
		}
		cfg[k] = v
	}
	return cfg, rows.Err()
}


// UpdateConfig upserts a system configuration key-value pair.
func (r *Repository) UpdateConfig(key, value string) error {
	_, err := r.DB.Exec(
		`INSERT INTO system_config (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("update config: %w", err)
	}
	return nil
}
