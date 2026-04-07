package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"

	"medops/internal/middleware"
	"medops/internal/models"
	"medops/internal/repository"
)

// InventoryHandler handles inventory management requests.
type InventoryHandler struct {
	repo *repository.Repository
}

// NewInventoryHandler creates a new InventoryHandler.
func NewInventoryHandler(repo *repository.Repository) *InventoryHandler {
	return &InventoryHandler{repo: repo}
}

// ListSKUs returns a paginated list of SKUs with optional search.
func (h *InventoryHandler) ListSKUs(c echo.Context) error {
	search := c.QueryParam("search")
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	skus, total, err := h.repo.ListSKUs(search, page, pageSize)
	if err != nil {
		logrus.WithError(err).Error("Failed to list SKUs")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve SKUs",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:     skus,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// CreateSKU creates a new SKU.
func (h *InventoryHandler) CreateSKU(c echo.Context) error {
	var req models.CreateSKURequest
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
	if req.UnitOfMeasure == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Unit of measure is required",
		})
	}

	sku := &models.SKU{
		ID:                uuid.New().String(),
		NDC:               req.NDC,
		UPC:               req.UPC,
		Name:              req.Name,
		Description:       req.Description,
		UnitOfMeasure:     req.UnitOfMeasure,
		LowStockThreshold: req.LowStockThreshold,
		StorageLocation:   req.StorageLocation,
		IsActive:          true,
		CreatedAt:         time.Now(),
	}

	if err := h.repo.CreateSKU(sku); err != nil {
		logrus.WithError(err).Error("Failed to create SKU")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create SKU",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{"name": sku.Name})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "create_sku",
		EntityType: "sku",
		EntityID:   sku.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id": userID,
		"sku_id":  sku.ID,
	}).Info("SKU created")

	return c.JSON(http.StatusCreated, sku)
}

// GetSKU returns a single SKU with its batches.
func (h *InventoryHandler) GetSKU(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "SKU ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	sku, err := h.repo.GetSKUByID(id)
	if err != nil || sku == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "SKU not found",
			Code:    http.StatusNotFound,
			Details: "No SKU found with the given ID",
		})
	}

	batches, err := h.repo.GetBatchesBySKUID(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get batches for SKU")
		batches = []models.InventoryBatch{}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"sku":     sku,
		"batches": batches,
	})
}

// UpdateSKU updates an existing SKU.
func (h *InventoryHandler) UpdateSKU(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "SKU ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	sku, err := h.repo.GetSKUByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "SKU not found",
			Code:    http.StatusNotFound,
			Details: "No SKU found with the given ID",
		})
	}

	var body struct {
		Name              *string `json:"name"`
		Description       *string `json:"description"`
		UnitOfMeasure     *string `json:"unit_of_measure"`
		LowStockThreshold *int    `json:"low_stock_threshold"`
		StorageLocation   *string `json:"storage_location"`
		IsActive          *bool   `json:"is_active"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if body.Name != nil {
		if *body.Name == "" {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Validation failed",
				Code:    http.StatusBadRequest,
				Details: "Name cannot be empty",
			})
		}
		sku.Name = *body.Name
	}
	if body.Description != nil {
		sku.Description = *body.Description
	}
	if body.UnitOfMeasure != nil {
		sku.UnitOfMeasure = *body.UnitOfMeasure
	}
	if body.LowStockThreshold != nil {
		sku.LowStockThreshold = *body.LowStockThreshold
	}
	if body.StorageLocation != nil {
		sku.StorageLocation = *body.StorageLocation
	}
	if body.IsActive != nil {
		sku.IsActive = *body.IsActive
	}

	if err := h.repo.UpdateSKU(sku); err != nil {
		logrus.WithError(err).Error("Failed to update SKU")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update SKU",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{"sku_id": id})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "update_sku",
		EntityType: "sku",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id": userID,
		"sku_id":  id,
	}).Info("SKU updated")

	return c.JSON(http.StatusOK, sku)
}

// GetBatches returns all batches for a given SKU.
func (h *InventoryHandler) GetBatches(c echo.Context) error {
	skuID := c.Param("id")
	if skuID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "SKU ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	// Verify SKU exists
	if _, err := h.repo.GetSKUByID(skuID); err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "SKU not found",
			Code:    http.StatusNotFound,
			Details: "No SKU found with the given ID",
		})
	}

	batches, err := h.repo.GetBatchesBySKUID(skuID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get batches")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve batches",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": batches,
	})
}

// GetLowStock returns SKUs at or below their low stock threshold.
func (h *InventoryHandler) GetLowStock(c echo.Context) error {
	skus, err := h.repo.GetLowStockSKUs()
	if err != nil {
		logrus.WithError(err).Error("Failed to get low stock SKUs")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve low stock SKUs",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": skus,
	})
}

// Receive handles inventory receipt (stock in).
func (h *InventoryHandler) Receive(c echo.Context) error {
	var req models.ReceiveRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	// Validate
	if req.SKUID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "SKU ID is required",
		})
	}
	if req.Quantity <= 0 {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Quantity must be greater than 0",
		})
	}
	if req.ReasonCode == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Reason code is required",
		})
	}
	if req.ExpirationDate == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Expiration date is required",
		})
	}

	// Parse and validate expiration date is in the future
	expDate, err := time.Parse("2006-01-02", req.ExpirationDate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Invalid expiration date format. Use YYYY-MM-DD",
		})
	}
	if !expDate.After(time.Now().Truncate(24 * time.Hour)) {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Expiration date must be in the future",
		})
	}

	// Verify SKU exists
	if _, err := h.repo.GetSKUByID(req.SKUID); err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "SKU not found",
			Code:    http.StatusNotFound,
			Details: "No SKU found with the given ID",
		})
	}

	// Create or update batch
	batch := &models.InventoryBatch{
		ID:             uuid.New().String(),
		SKUID:          req.SKUID,
		LotNumber:      req.LotNumber,
		ExpirationDate: req.ExpirationDate,
		QuantityOnHand: req.Quantity,
		CreatedAt:      time.Now(),
	}

	if err := h.repo.CreateBatch(batch); err != nil {
		logrus.WithError(err).Error("Failed to create batch")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create batch",
			Code:  http.StatusInternalServerError,
		})
	}

	// Create stock transaction
	userID := middleware.GetUserID(c)
	tx := &models.StockTransaction{
		ID:          uuid.New().String(),
		SKUID:       req.SKUID,
		BatchID:     batch.ID,
		Type:        "in",
		Quantity:    req.Quantity,
		ReasonCode:  req.ReasonCode,
		PerformedBy: userID,
		CreatedAt:   time.Now(),
	}

	if err := h.repo.CreateStockTransaction(tx); err != nil {
		logrus.WithError(err).Error("Failed to create stock transaction")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to record transaction",
			Code:  http.StatusInternalServerError,
		})
	}

	details, _ := json.Marshal(map[string]interface{}{
		"sku_id":   req.SKUID,
		"quantity": req.Quantity,
		"batch_id": batch.ID,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "receive_inventory",
		EntityType: "inventory_batch",
		EntityID:   batch.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":  userID,
		"sku_id":   req.SKUID,
		"batch_id": batch.ID,
		"quantity": req.Quantity,
	}).Info("Inventory received")

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"batch":       batch,
		"transaction": tx,
	})
}

// Dispense handles inventory dispensing (stock out).
func (h *InventoryHandler) Dispense(c echo.Context) error {
	var req models.DispenseRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	// Validate
	if req.SKUID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "SKU ID is required",
		})
	}
	if req.BatchID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Batch ID is required",
		})
	}
	if req.Quantity <= 0 {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Quantity must be greater than 0",
		})
	}
	if req.ReasonCode == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Reason code is required",
		})
	}

	// Get batch
	batch, err := h.repo.GetBatchByID(req.BatchID)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Batch not found",
			Code:    http.StatusNotFound,
			Details: "No batch found with the given ID",
		})
	}

	// Check batch not expired
	expDate, err := time.Parse("2006-01-02", batch.ExpirationDate)
	if err == nil && !expDate.After(time.Now().Truncate(24*time.Hour)) {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Batch expired",
			Code:    http.StatusBadRequest,
			Details: "Cannot dispense from an expired batch",
		})
	}

	// Check quantity available
	if batch.QuantityOnHand < req.Quantity {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Insufficient quantity",
			Code:    http.StatusBadRequest,
			Details: "Not enough stock in this batch to dispense the requested quantity",
		})
	}

	// Update batch quantity
	batch.QuantityOnHand -= req.Quantity
	if err := h.repo.UpdateBatch(batch); err != nil {
		logrus.WithError(err).Error("Failed to update batch")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update batch",
			Code:  http.StatusInternalServerError,
		})
	}

	// Create stock transaction
	userID := middleware.GetUserID(c)
	tx := &models.StockTransaction{
		ID:             uuid.New().String(),
		SKUID:          req.SKUID,
		BatchID:        req.BatchID,
		Type:           "out",
		Quantity:       req.Quantity,
		ReasonCode:     req.ReasonCode,
		PrescriptionID: req.PrescriptionID,
		PerformedBy:    userID,
		CreatedAt:      time.Now(),
	}

	if err := h.repo.CreateStockTransaction(tx); err != nil {
		logrus.WithError(err).Error("Failed to create stock transaction")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to record transaction",
			Code:  http.StatusInternalServerError,
		})
	}

	details, _ := json.Marshal(map[string]interface{}{
		"sku_id":   req.SKUID,
		"batch_id": req.BatchID,
		"quantity": req.Quantity,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "dispense_inventory",
		EntityType: "inventory_batch",
		EntityID:   req.BatchID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":  userID,
		"sku_id":   req.SKUID,
		"batch_id": req.BatchID,
		"quantity": req.Quantity,
	}).Info("Inventory dispensed")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"batch":       batch,
		"transaction": tx,
	})
}

// ListTransactions returns a paginated list of stock transactions.
func (h *InventoryHandler) ListTransactions(c echo.Context) error {
	skuID := c.QueryParam("sku_id")
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	transactions, total, err := h.repo.ListStockTransactions(skuID, page, pageSize)
	if err != nil {
		logrus.WithError(err).Error("Failed to list transactions")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve transactions",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:     transactions,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// adjustTxType returns the schema-valid transaction type ("in" or "out") for an adjustment quantity.
// Positive or zero quantities are stock-ins; negative quantities are stock-outs.
// The schema CHECK constraint only permits 'in' and 'out' — never "adjustment_in/out".
func adjustTxType(quantity int) string {
	if quantity >= 0 {
		return "in"
	}
	return "out"
}

// Adjust handles inventory adjustments with a reason code.
func (h *InventoryHandler) Adjust(c echo.Context) error {
	var req struct {
		SKUID      string `json:"sku_id"`
		BatchID    string `json:"batch_id"`
		Quantity   int    `json:"quantity"`
		ReasonCode string `json:"reason_code"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.SKUID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "SKU ID is required",
		})
	}
	if req.BatchID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Batch ID is required",
		})
	}
	if req.Quantity == 0 {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Quantity must be non-zero",
		})
	}
	if req.ReasonCode == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Reason code is required",
		})
	}

	// Get batch
	batch, err := h.repo.GetBatchByID(req.BatchID)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Batch not found",
			Code:    http.StatusNotFound,
			Details: "No batch found with the given ID",
		})
	}

	// For negative adjustments, check we have enough stock
	if req.Quantity < 0 && batch.QuantityOnHand+req.Quantity < 0 {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Insufficient quantity",
			Code:    http.StatusBadRequest,
			Details: "Adjustment would result in negative stock",
		})
	}

	batch.QuantityOnHand += req.Quantity
	if err := h.repo.UpdateBatch(batch); err != nil {
		logrus.WithError(err).Error("Failed to update batch for adjustment")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to adjust batch",
			Code:  http.StatusInternalServerError,
		})
	}

	// Determine transaction type — schema allows only 'in' or 'out';
	// adjustment direction is conveyed via the reason_code field.
	txType := adjustTxType(req.Quantity)
	absQty := req.Quantity
	if req.Quantity < 0 {
		absQty = -req.Quantity
	}

	userID := middleware.GetUserID(c)
	tx := &models.StockTransaction{
		ID:          uuid.New().String(),
		SKUID:       req.SKUID,
		BatchID:     req.BatchID,
		Type:        txType,
		Quantity:    absQty,
		ReasonCode:  req.ReasonCode,
		PerformedBy: userID,
		CreatedAt:   time.Now(),
	}

	if err := h.repo.CreateStockTransaction(tx); err != nil {
		logrus.WithError(err).Error("Failed to create adjustment transaction")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to record adjustment transaction",
			Code:  http.StatusInternalServerError,
		})
	}

	details, _ := json.Marshal(map[string]interface{}{
		"sku_id":      req.SKUID,
		"batch_id":    req.BatchID,
		"quantity":    req.Quantity,
		"reason_code": req.ReasonCode,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "adjust_inventory",
		EntityType: "inventory_batch",
		EntityID:   req.BatchID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":  userID,
		"batch_id": req.BatchID,
		"quantity": req.Quantity,
	}).Info("Inventory adjusted")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"batch":       batch,
		"transaction": tx,
	})
}

// CreateStocktake creates a new stocktake with period dates.
func (h *InventoryHandler) CreateStocktake(c echo.Context) error {
	var req struct {
		PeriodStart string `json:"period_start"`
		PeriodEnd   string `json:"period_end"`
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

	// Validate date formats
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

	userID := middleware.GetUserID(c)
	stocktake := &models.Stocktake{
		ID:          uuid.New().String(),
		PeriodStart: req.PeriodStart,
		PeriodEnd:   req.PeriodEnd,
		Status:      "draft",
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
	}

	if err := h.repo.CreateStocktake(stocktake); err != nil {
		logrus.WithError(err).Error("Failed to create stocktake")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create stocktake",
			Code:  http.StatusInternalServerError,
		})
	}

	details, _ := json.Marshal(map[string]string{
		"period_start": req.PeriodStart,
		"period_end":   req.PeriodEnd,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "create_stocktake",
		EntityType: "stocktake",
		EntityID:   stocktake.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"stocktake_id": stocktake.ID,
	}).Info("Stocktake created")

	return c.JSON(http.StatusCreated, stocktake)
}

// GetStocktake returns a stocktake with its lines.
func (h *InventoryHandler) GetStocktake(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Stocktake ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	stocktake, err := h.repo.GetStocktakeByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Stocktake not found",
			Code:    http.StatusNotFound,
			Details: "No stocktake found with the given ID",
		})
	}

	return c.JSON(http.StatusOK, stocktake)
}

// UpdateStocktakeLines updates the counted quantities for stocktake lines.
func (h *InventoryHandler) UpdateStocktakeLines(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Stocktake ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	// Verify stocktake exists and is open
	stocktake, err := h.repo.GetStocktakeByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Stocktake not found",
			Code:    http.StatusNotFound,
			Details: "No stocktake found with the given ID",
		})
	}

	if stocktake.Status != "draft" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Stocktake not open",
			Code:    http.StatusBadRequest,
			Details: "Can only update lines for an open stocktake",
		})
	}

	var req struct {
		Lines []models.StocktakeLine `json:"lines"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if len(req.Lines) == 0 {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "At least one line is required",
		})
	}

	// Set stocktake ID and compute variance for each line
	for i := range req.Lines {
		req.Lines[i].StocktakeID = id
		if req.Lines[i].ID == "" {
			req.Lines[i].ID = uuid.New().String()
		}
		req.Lines[i].Variance = req.Lines[i].CountedQty - req.Lines[i].SystemQty
	}

	if err := h.repo.UpdateStocktakeLines(id, req.Lines); err != nil {
		logrus.WithError(err).Error("Failed to update stocktake lines")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update stocktake lines",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]interface{}{
		"stocktake_id": id,
		"lines_count":  len(req.Lines),
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "update_stocktake_lines",
		EntityType: "stocktake",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"stocktake_id": id,
		"lines":        len(req.Lines),
	}).Info("Stocktake lines updated")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Stocktake lines updated successfully",
		"lines":   req.Lines,
	})
}

// CompleteStocktake finalizes a stocktake and computes variance.
func (h *InventoryHandler) CompleteStocktake(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Stocktake ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	stocktake, err := h.repo.GetStocktakeByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Stocktake not found",
			Code:    http.StatusNotFound,
			Details: "No stocktake found with the given ID",
		})
	}

	if stocktake.Status != "draft" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Stocktake not open",
			Code:    http.StatusBadRequest,
			Details: "Can only complete an open stocktake",
		})
	}

	if err := h.repo.CompleteStocktake(id); err != nil {
		logrus.WithError(err).Error("Failed to complete stocktake")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to complete stocktake",
			Code:  http.StatusInternalServerError,
		})
	}

	// Fetch the completed stocktake
	completed, err := h.repo.GetStocktakeByID(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to fetch completed stocktake")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Stocktake completed but failed to retrieve result",
			Code:  http.StatusInternalServerError,
		})
	}

	// Compute total variance summary
	totalVariance := 0
	for _, line := range completed.Lines {
		totalVariance += line.Variance
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]interface{}{
		"stocktake_id":   id,
		"total_variance": totalVariance,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "complete_stocktake",
		EntityType: "stocktake",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":        userID,
		"stocktake_id":   id,
		"total_variance": totalVariance,
	}).Info("Stocktake completed")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"stocktake":      completed,
		"total_variance": totalVariance,
	})
}
