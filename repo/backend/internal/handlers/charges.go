package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"

	"medops/internal/middleware"
	"medops/internal/models"
	"medops/internal/repository"
)

// ChargeHandler handles charge and billing requests.
type ChargeHandler struct {
	repo    *repository.Repository
	hmacKey string
}

// NewChargeHandler creates a new ChargeHandler.
func NewChargeHandler(repo *repository.Repository, hmacKey string) *ChargeHandler {
	return &ChargeHandler{repo: repo, hmacKey: hmacKey}
}

// ListRateTables returns all rate tables.
func (h *ChargeHandler) ListRateTables(c echo.Context) error {
	tables, err := h.repo.ListRateTables()
	if err != nil {
		logrus.WithError(err).Error("Failed to list rate tables")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve rate tables",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": tables,
	})
}

// CreateRateTable creates a new rate table.
func (h *ChargeHandler) CreateRateTable(c echo.Context) error {
	var req struct {
		Name             string          `json:"name"`
		Type             string          `json:"type"`
		Tiers            json.RawMessage `json:"tiers"`
		FuelSurchargePct float64         `json:"fuel_surcharge_pct"`
		Taxable          bool            `json:"taxable"`
		EffectiveDate    string          `json:"effective_date"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Name is required",
		})
	}
	if req.Type == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Type is required",
		})
	}
	if req.EffectiveDate == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Effective date is required",
		})
	}
	if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Invalid effective date format. Use YYYY-MM-DD",
		})
	}

	rt := &models.RateTable{
		ID:               uuid.New().String(),
		Name:             req.Name,
		Type:             req.Type,
		Tiers:            req.Tiers,
		FuelSurchargePct: req.FuelSurchargePct,
		Taxable:          req.Taxable,
		EffectiveDate:    req.EffectiveDate,
	}

	if err := h.repo.CreateRateTable(rt); err != nil {
		logrus.WithError(err).Error("Failed to create rate table")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create rate table",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{"name": rt.Name})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "create_rate_table",
		EntityType: "rate_table",
		EntityID:   rt.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":       userID,
		"rate_table_id": rt.ID,
	}).Info("Rate table created")

	return c.JSON(http.StatusCreated, rt)
}

// findRateTableByID is a helper that finds a rate table by ID from the full list.
func (h *ChargeHandler) findRateTableByID(id string) (*models.RateTable, error) {
	tables, err := h.repo.ListRateTables()
	if err != nil {
		return nil, err
	}
	for i := range tables {
		if tables[i].ID == id {
			return &tables[i], nil
		}
	}
	return nil, nil
}

// UpdateRateTable updates an existing rate table.
func (h *ChargeHandler) UpdateRateTable(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Rate table ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	rt, err := h.findRateTableByID(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to find rate table")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve rate table",
			Code:  http.StatusInternalServerError,
		})
	}
	if rt == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Rate table not found",
			Code:    http.StatusNotFound,
			Details: "No rate table found with the given ID",
		})
	}

	var body struct {
		Name             *string          `json:"name"`
		Type             *string          `json:"type"`
		Tiers            *json.RawMessage `json:"tiers"`
		FuelSurchargePct *float64         `json:"fuel_surcharge_pct"`
		Taxable          *bool            `json:"taxable"`
		EffectiveDate    *string          `json:"effective_date"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if body.Name != nil {
		rt.Name = *body.Name
	}
	if body.Type != nil {
		rt.Type = *body.Type
	}
	if body.Tiers != nil {
		rt.Tiers = *body.Tiers
	}
	if body.FuelSurchargePct != nil {
		rt.FuelSurchargePct = *body.FuelSurchargePct
	}
	if body.Taxable != nil {
		rt.Taxable = *body.Taxable
	}
	if body.EffectiveDate != nil {
		if _, err := time.Parse("2006-01-02", *body.EffectiveDate); err != nil {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Validation failed",
				Code:    http.StatusBadRequest,
				Details: "Invalid effective date format. Use YYYY-MM-DD",
			})
		}
		rt.EffectiveDate = *body.EffectiveDate
	}

	if err := h.repo.UpdateRateTable(rt); err != nil {
		logrus.WithError(err).Error("Failed to update rate table")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update rate table",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{"rate_table_id": id})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "update_rate_table",
		EntityType: "rate_table",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":       userID,
		"rate_table_id": id,
	}).Info("Rate table updated")

	return c.JSON(http.StatusOK, rt)
}

// ImportRateTableCSV parses a CSV upload (columns: min,max,rate) into rate table tiers.
func (h *ChargeHandler) ImportRateTableCSV(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "File upload required",
			Code:    http.StatusBadRequest,
			Details: "A CSV file must be uploaded",
		})
	}

	src, err := file.Open()
	if err != nil {
		logrus.WithError(err).Error("Failed to open uploaded CSV")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to process uploaded file",
			Code:  http.StatusInternalServerError,
		})
	}
	defer src.Close()

	reader := csv.NewReader(src)
	records, err := reader.ReadAll()
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid CSV file",
			Code:    http.StatusBadRequest,
			Details: "Failed to parse CSV data",
		})
	}

	if len(records) < 2 {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Empty CSV",
			Code:    http.StatusBadRequest,
			Details: "CSV must contain a header row and at least one data row",
		})
	}

	// Parse CSV rows into tier objects (columns: min, max, rate)
	var tiers []map[string]interface{}
	headers := records[0]
	for _, row := range records[1:] {
		tier := make(map[string]interface{})
		for i, header := range headers {
			if i < len(row) {
				header = strings.TrimSpace(header)
				val := strings.TrimSpace(row[i])
				if num, err := strconv.ParseFloat(val, 64); err == nil {
					tier[header] = num
				} else {
					tier[header] = val
				}
			}
		}
		tiers = append(tiers, tier)
	}

	tiersJSON, err := json.Marshal(tiers)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal tiers")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to process tier data",
			Code:  http.StatusInternalServerError,
		})
	}

	// Create a new rate table with the parsed tiers
	// Type defaults to "distance" if not provided via form field
	rtType := c.FormValue("type")
	validTypes := map[string]bool{"distance": true, "weight": true, "volume": true}
	if !validTypes[rtType] {
		rtType = "distance"
	}
	rt := &models.RateTable{
		ID:    uuid.New().String(),
		Name:  strings.TrimSuffix(file.Filename, ".csv"),
		Type:  rtType,
		Tiers: tiersJSON,
	}

	if err := h.repo.CreateRateTable(rt); err != nil {
		logrus.WithError(err).Error("Failed to create rate table from CSV")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to import rate table from CSV",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]interface{}{
		"rate_table_id": rt.ID,
		"tiers_count":   len(tiers),
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "import_rate_table_csv",
		EntityType: "rate_table",
		EntityID:   rt.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":       userID,
		"rate_table_id": rt.ID,
		"tiers":         len(tiers),
	}).Info("Rate table tiers imported from CSV")

	return c.JSON(http.StatusCreated, rt)
}

// ListStatements returns a paginated list of charge statements.
func (h *ChargeHandler) ListStatements(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	statements, total, err := h.repo.ListStatements(page, pageSize)
	if err != nil {
		logrus.WithError(err).Error("Failed to list statements")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve statements",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:     statements,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetStatement returns a single statement with its line items.
func (h *ChargeHandler) GetStatement(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Statement ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	statement, err := h.repo.GetStatement(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get statement")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve statement",
			Code:  http.StatusInternalServerError,
		})
	}
	if statement == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Statement not found",
			Code:    http.StatusNotFound,
			Details: "No statement found with the given ID",
		})
	}

	lineItems, err := h.repo.GetStatementLineItems(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get statement line items")
		lineItems = []models.ChargeLineItem{}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"statement":  statement,
		"line_items": lineItems,
	})
}

// GenerateStatement creates a new charge statement with status=draft.
func (h *ChargeHandler) GenerateStatement(c echo.Context) error {
	var req struct {
		PeriodStart string                  `json:"period_start"`
		PeriodEnd   string                  `json:"period_end"`
		LineItems   []models.ChargeLineItem `json:"line_items"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.PeriodStart == "" || req.PeriodEnd == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Period start and end dates are required",
		})
	}

	if _, err := time.Parse("2006-01-02", req.PeriodStart); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Invalid period_start format. Use YYYY-MM-DD",
		})
	}
	if _, err := time.Parse("2006-01-02", req.PeriodEnd); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Invalid period_end format. Use YYYY-MM-DD",
		})
	}

	// Calculate total amount from line items
	var totalAmount float64
	for i := range req.LineItems {
		item := &req.LineItems[i]
		item.Total = (item.Quantity * item.UnitPrice) + item.Surcharge + item.Tax
		totalAmount += item.Total
	}

	statement := &models.ChargeStatement{
		ID:          uuid.New().String(),
		PeriodStart: req.PeriodStart,
		PeriodEnd:   req.PeriodEnd,
		TotalAmount: totalAmount,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}

	if err := h.repo.CreateStatement(statement); err != nil {
		logrus.WithError(err).Error("Failed to create statement")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create statement",
			Code:  http.StatusInternalServerError,
		})
	}

	// Create line items
	for i := range req.LineItems {
		item := &req.LineItems[i]
		item.ID = uuid.New().String()
		item.StatementID = statement.ID
		if err := h.repo.CreateLineItem(item); err != nil {
			logrus.WithError(err).Error("Failed to create line item")
		}
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]interface{}{
		"period_start": req.PeriodStart,
		"period_end":   req.PeriodEnd,
		"total_amount": totalAmount,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "generate_statement",
		EntityType: "charge_statement",
		EntityID:   statement.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"statement_id": statement.ID,
	}).Info("Statement generated")

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"statement":  statement,
		"line_items": req.LineItems,
	})
}

// statementIsReconcilable returns true only when the statement is in pending status.
func statementIsReconcilable(status string) bool { return status == "pending" }

// statementIsApprovable returns true only when the statement is in pending status.
func statementIsApprovable(status string) bool { return status == "pending" }

// statementVarianceExceedsThreshold returns true when ABS(total - expected) > 25.
// This is the correct reconciliation escalation check per the business rule.
func statementVarianceExceedsThreshold(totalAmount, expectedTotal float64) bool {
	diff := totalAmount - expectedTotal
	if diff < 0 {
		diff = -diff
	}
	return diff > 25.0
}

// approvalStep returns 1 if no approver has been set, or 0 if fully approved.
func approvalStep(approvedBy1, approvedBy2 *string) int {
	if approvedBy1 == nil {
		return 1
	}
	if approvedBy2 == nil {
		return 2
	}
	return 0
}

// ReconcileStatement reconciles a statement, requiring variance notes if delta > $25.
func (h *ChargeHandler) ReconcileStatement(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Statement ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	statement, err := h.repo.GetStatement(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get statement for reconciliation")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve statement",
			Code:  http.StatusInternalServerError,
		})
	}
	if statement == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Statement not found",
			Code:    http.StatusNotFound,
			Details: "No statement found with the given ID",
		})
	}

	if !statementIsReconcilable(statement.Status) {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid statement state",
			Code:    http.StatusBadRequest,
			Details: "Only pending statements can be reconciled and approved",
		})
	}

	var req models.ReconcileRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	// Enforce ABS(total - expected) > 25 → variance notes required.
	if statementVarianceExceedsThreshold(statement.TotalAmount, req.ExpectedTotal) && req.VarianceNotes == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Variance notes required",
			Code:    http.StatusBadRequest,
			Details: fmt.Sprintf("Variance notes are required when ABS(total - expected) exceeds $25.00 (actual: $%.2f, expected: $%.2f)", statement.TotalAmount, req.ExpectedTotal),
		})
	}

	statement.ExpectedTotal = req.ExpectedTotal
	if req.VarianceNotes != "" {
		statement.VarianceNotes = &req.VarianceNotes
	}
	// Reconcile transitions pending → approved.
	statement.Status = "approved"

	if err := h.repo.UpdateStatement(statement); err != nil {
		logrus.WithError(err).Error("Failed to reconcile statement")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to reconcile statement",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]interface{}{
		"statement_id":   id,
		"variance_notes": req.VarianceNotes,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "reconcile_statement",
		EntityType: "charge_statement",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"statement_id": id,
	}).Info("Statement reconciled")

	return c.JSON(http.StatusOK, statement)
}

// ApproveStatement implements two-step approval for statements.
func (h *ChargeHandler) ApproveStatement(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Statement ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	statement, err := h.repo.GetStatement(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get statement for approval")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve statement",
			Code:  http.StatusInternalServerError,
		})
	}
	if statement == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Statement not found",
			Code:    http.StatusNotFound,
			Details: "No statement found with the given ID",
		})
	}

	userID := middleware.GetUserID(c)

	if !statementIsApprovable(statement.Status) {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid statement state",
			Code:    http.StatusBadRequest,
			Details: "Only pending statements can be approved",
		})
	}

	// Single-step approval: pending → approved.
	statement.ApprovedBy = &userID
	statement.Status = "approved"

	if err := h.repo.UpdateStatement(statement); err != nil {
		logrus.WithError(err).Error("Failed to approve statement")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to approve statement",
			Code:  http.StatusInternalServerError,
		})
	}

	details, _ := json.Marshal(map[string]interface{}{
		"statement_id": id,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "approve_statement",
		EntityType: "charge_statement",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"statement_id": id,
	}).Info("Statement approved")

	return c.JSON(http.StatusOK, statement)
}

// ExportStatement exports a statement as CSV with HMAC-SHA256 signature.
func (h *ChargeHandler) ExportStatement(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Statement ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	statement, err := h.repo.GetStatement(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get statement for export")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve statement",
			Code:  http.StatusInternalServerError,
		})
	}
	if statement == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Statement not found",
			Code:    http.StatusNotFound,
			Details: "No statement found with the given ID",
		})
	}

	// Enforce approved → paid transition (no direct pending→paid).
	if statement.Status != "approved" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid statement state",
			Code:    http.StatusBadRequest,
			Details: "Only approved statements can be exported and marked as paid",
		})
	}

	lineItems, err := h.repo.GetStatementLineItems(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get statement line items for export")
		lineItems = []models.ChargeLineItem{}
	}

	format := c.QueryParam("format")
	if format == "" {
		format = "csv"
	}

	var content []byte
	var contentType string
	var filename string

	if format == "json" {
		// Signed JSON export
		payload := map[string]interface{}{
			"statement":  statement,
			"line_items": lineItems,
			"paid_at": time.Now().UTC().Format(time.RFC3339),
		}
		content, err = json.Marshal(payload)
		if err != nil {
			logrus.WithError(err).Error("Failed to marshal JSON export")
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to generate JSON export",
				Code:  http.StatusInternalServerError,
			})
		}
		contentType = "application/json"
		filename = fmt.Sprintf("statement_%s.json", id)
	} else {
		// CSV export
		var sb strings.Builder
		writer := csv.NewWriter(&sb)

		// Header row
		writer.Write([]string{"ID", "Period Start", "Period End", "Total Amount", "Status"})
		writer.Write([]string{statement.ID, statement.PeriodStart, statement.PeriodEnd,
			fmt.Sprintf("%.2f", statement.TotalAmount), statement.Status})

		writer.Write([]string{}) // blank line
		writer.Write([]string{"Line Item ID", "Description", "Quantity", "Unit Price", "Surcharge", "Tax", "Total"})

		for _, item := range lineItems {
			writer.Write([]string{
				item.ID,
				item.Description,
				fmt.Sprintf("%.2f", item.Quantity),
				fmt.Sprintf("%.2f", item.UnitPrice),
				fmt.Sprintf("%.2f", item.Surcharge),
				fmt.Sprintf("%.2f", item.Tax),
				fmt.Sprintf("%.2f", item.Total),
			})
		}
		writer.Flush()
		content = []byte(sb.String())
		contentType = "text/csv"
		filename = fmt.Sprintf("statement_%s.csv", id)
	}

	// Compute HMAC-SHA256 signature over content
	mac := hmac.New(sha256.New, []byte(h.hmacKey))
	mac.Write(content)
	signature := hex.EncodeToString(mac.Sum(nil))

	// Transition approved → paid.
	now := time.Now()
	statement.Status = "paid"
	statement.PaidAt = &now
	h.repo.UpdateStatement(statement)

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{
		"statement_id": id,
		"format":       format,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "export_statement",
		EntityType: "charge_statement",
		EntityID:   id,
		Details:    details,
	})

	c.Response().Header().Set("Content-Type", contentType)
	c.Response().Header().Set("X-HMAC-Signature", signature)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	return c.Blob(http.StatusOK, contentType, content)
}

// Ensure io import is used (for ImportRateTableCSV)
var _ = io.EOF
