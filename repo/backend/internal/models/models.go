package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

type User struct {
	ID             string     `json:"id"`
	Username       string     `json:"username"`
	PasswordHash   string     `json:"-"`
	Role           string     `json:"role"`
	FailedAttempts int        `json:"failed_attempts"`
	LockedUntil    *time.Time `json:"locked_until,omitempty"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Role struct {
	ID          string          `json:"id"`
	DisplayName string          `json:"display_name"`
	Permissions json.RawMessage `json:"permissions"`
}

type SKU struct {
	ID                string    `json:"id"`
	NDC               *string   `json:"ndc,omitempty"`
	UPC               *string   `json:"upc,omitempty"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	UnitOfMeasure     string    `json:"unit_of_measure"`
	LowStockThreshold int       `json:"low_stock_threshold"`
	StorageLocation   string    `json:"storage_location"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
}

type InventoryBatch struct {
	ID             string    `json:"id"`
	SKUID          string    `json:"sku_id"`
	LotNumber      string    `json:"lot_number"`
	ExpirationDate string    `json:"expiration_date"`
	QuantityOnHand int       `json:"quantity_on_hand"`
	CreatedAt      time.Time `json:"created_at"`
}

type StockTransaction struct {
	ID             string    `json:"id"`
	SKUID          string    `json:"sku_id"`
	BatchID        string    `json:"batch_id"`
	Type           string    `json:"type"`
	Quantity       int       `json:"quantity"`
	ReasonCode     string    `json:"reason_code"`
	PrescriptionID *string   `json:"prescription_id,omitempty"`
	PerformedBy    string    `json:"performed_by"`
	CreatedAt      time.Time `json:"created_at"`
}

type Stocktake struct {
	ID          string         `json:"id"`
	PeriodStart string         `json:"period_start"`
	PeriodEnd   string         `json:"period_end"`
	Status      string         `json:"status"`
	CreatedBy   string         `json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	Lines       []StocktakeLine `json:"lines,omitempty"`
}

type StocktakeLine struct {
	ID          string  `json:"id"`
	StocktakeID string  `json:"stocktake_id"`
	SKUID       string  `json:"sku_id"`
	BatchID     string  `json:"batch_id"`
	SystemQty   int     `json:"system_qty"`
	CountedQty  int     `json:"counted_qty"`
	Variance    int     `json:"variance"`
	LossReason  *string `json:"loss_reason,omitempty"`
}

type LearningSubject struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
}

type LearningChapter struct {
	ID        string `json:"id"`
	SubjectID string `json:"subject_id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type KnowledgePoint struct {
	ID              string          `json:"id"`
	ChapterID       string          `json:"chapter_id"`
	Title           string          `json:"title"`
	Content         string          `json:"content"`
	Tags            []string        `json:"tags"`
	Classifications json.RawMessage `json:"classifications,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type WorkOrder struct {
	ID          string     `json:"id"`
	SubmittedBy string     `json:"submitted_by"`
	AssignedTo  *string    `json:"assigned_to,omitempty"`
	Trade       string     `json:"trade"`
	Priority    string     `json:"priority"`
	SLADeadline time.Time  `json:"sla_deadline"`
	Status      string     `json:"status"`
	Description string     `json:"description"`
	Location    string     `json:"location"`
	PartsCost   float64    `json:"parts_cost"`
	LaborCost   float64    `json:"labor_cost"`
	Rating      *int       `json:"rating,omitempty"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Member struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	IDNumberEncrypted  []byte     `json:"-"`
	Phone              string     `json:"phone"`
	TierID             string     `json:"tier_id"`
	PointsBalance      int        `json:"points_balance"`
	StoredValue        float64    `json:"stored_value"`
	Status             string     `json:"status"`
	FrozenAt           *time.Time `json:"frozen_at,omitempty"`
	ExpiresAt          time.Time  `json:"expires_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

type MembershipTier struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Benefits  json.RawMessage `json:"benefits"`
	SortOrder int             `json:"sort_order"`
}

type SessionPackage struct {
	ID                string    `json:"id"`
	MemberID          string    `json:"member_id"`
	PackageName       string    `json:"package_name"`
	TotalSessions     int       `json:"total_sessions"`
	RemainingSessions int       `json:"remaining_sessions"`
	ExpiresAt         time.Time `json:"expires_at"`
}

type MemberTransaction struct {
	ID          string    `json:"id"`
	MemberID    string    `json:"member_id"`
	Type        string    `json:"type"`
	Amount      float64   `json:"amount"`
	Description string    `json:"description"`
	PerformedBy string    `json:"performed_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type RateTable struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Type              string          `json:"type"`
	Tiers             json.RawMessage `json:"tiers"`
	FuelSurchargePct  float64         `json:"fuel_surcharge_pct"`
	Taxable           bool            `json:"taxable"`
	EffectiveDate     string          `json:"effective_date"`
}

// ChargeStatement lifecycle: pending → approved → paid (no direct pending→paid).
type ChargeStatement struct {
	ID            string     `json:"id"`
	PeriodStart   string     `json:"period_start"`
	PeriodEnd     string     `json:"period_end"`
	TotalAmount   float64    `json:"total_amount"`
	ExpectedTotal float64    `json:"expected_total"`
	Status        string     `json:"status"` // pending | approved | paid
	ApprovedBy    *string    `json:"approved_by,omitempty"`
	VarianceNotes *string    `json:"variance_notes,omitempty"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type ChargeLineItem struct {
	ID          string  `json:"id"`
	StatementID string  `json:"statement_id"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Surcharge   float64 `json:"surcharge"`
	Tax         float64 `json:"tax"`
	Total       float64 `json:"total"`
}

// WorkOrderPhoto links a managed file to a work order.
type WorkOrderPhoto struct {
	ID          string    `json:"id"`
	WorkOrderID string    `json:"work_order_id"`
	FileID      string    `json:"file_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type ManagedFile struct {
	ID             string     `json:"id"`
	SHA256         string     `json:"sha256"`
	OriginalName   string     `json:"original_name"`
	MimeType       string     `json:"mime_type"`
	SizeBytes      int64      `json:"size_bytes"`
	StoragePath    string     `json:"storage_path"`
	UploadedBy     *string    `json:"uploaded_by,omitempty"`
	RetentionUntil *time.Time `json:"retention_until,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type DraftCheckpoint struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	FormType  string          `json:"form_type"`
	FormID    *string         `json:"form_id,omitempty"`
	StateJSON json.RawMessage `json:"state_json"`
	SavedAt   time.Time       `json:"saved_at"`
}

type AuditLogEntry struct {
	ID         int64           `json:"id"`
	UserID     string          `json:"user_id"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	Details    json.RawMessage `json:"details,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// Request/Response types
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details,omitempty"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
}

type ReceiveRequest struct {
	SKUID          string  `json:"sku_id"`
	LotNumber      string  `json:"lot_number"`
	ExpirationDate string  `json:"expiration_date"`
	Quantity       int     `json:"quantity"`
	StorageLocation string `json:"storage_location"`
	ReasonCode     string  `json:"reason_code"`
}

type DispenseRequest struct {
	SKUID          string  `json:"sku_id"`
	BatchID        string  `json:"batch_id"`
	Quantity       int     `json:"quantity"`
	ReasonCode     string  `json:"reason_code"`
	PrescriptionID *string `json:"prescription_id,omitempty"`
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type CreateSKURequest struct {
	NDC               *string `json:"ndc,omitempty"`
	UPC               *string `json:"upc,omitempty"`
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	UnitOfMeasure     string  `json:"unit_of_measure"`
	LowStockThreshold int     `json:"low_stock_threshold"`
	StorageLocation   string  `json:"storage_location"`
}

type CreateWorkOrderRequest struct {
	Trade       string   `json:"trade"`
	Priority    string   `json:"priority"`
	Description string   `json:"description"`
	Location    string   `json:"location"`
	PhotoIDs    []string `json:"photo_ids,omitempty"`
}

type CloseWorkOrderRequest struct {
	PartsCost float64 `json:"parts_cost"`
	LaborCost float64 `json:"labor_cost"`
}

type RateWorkOrderRequest struct {
	Rating int `json:"rating"`
}

type CreateMemberRequest struct {
	Name     string  `json:"name"`
	IDNumber string  `json:"id_number"`
	Phone    string  `json:"phone"`
	TierID   string  `json:"tier_id"`
}

type RedeemRequest struct {
	Type      string  `json:"type"`
	Amount    float64 `json:"amount,omitempty"`
	PackageID string  `json:"package_id,omitempty"`
}

type AddValueRequest struct {
	Type   string  `json:"type"`
	Amount float64 `json:"amount"`
}

type ReconcileRequest struct {
	// ExpectedTotal is the operator's expected statement amount.
	// When ABS(TotalAmount - ExpectedTotal) > 25, VarianceNotes is required.
	ExpectedTotal float64 `json:"expected_total"`
	VarianceNotes string  `json:"variance_notes"`
}

// Unused import guard
var _ = sql.ErrNoRows
