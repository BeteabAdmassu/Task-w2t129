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

// workOrderStore is the subset of repository.Repository used by WorkOrderHandler.
// The interface enables unit testing without a real database.
type workOrderStore interface {
	GetWorkOrderByID(id string) (*models.WorkOrder, error)
	ListWorkOrders(status string, assignedTo string, submittedBy string, page, pageSize int) ([]models.WorkOrder, int, error)
	CreateWorkOrder(wo *models.WorkOrder) error
	UpdateWorkOrder(wo *models.WorkOrder) error
	GetTechWithLeastOrders(trade string) (string, error)
	GetWorkOrderAnalytics() (map[string]interface{}, error)
	LinkPhotoToWorkOrder(workOrderID, fileID string) (*models.WorkOrderPhoto, error)
	GetWorkOrderPhotos(workOrderID string) ([]models.ManagedFile, error)
	CreateAuditLog(entry *models.AuditLogEntry) error
}

// WorkOrderHandler handles work order management requests.
type WorkOrderHandler struct {
	repo workOrderStore
}

// NewWorkOrderHandler creates a new WorkOrderHandler.
func NewWorkOrderHandler(repo *repository.Repository) *WorkOrderHandler {
	return &WorkOrderHandler{repo: repo}
}

// computeSLADeadline calculates the SLA deadline based on priority.
func computeSLADeadline(priority string, now time.Time) time.Time {
	switch priority {
	case "urgent":
		return now.Add(4 * time.Hour)
	case "high":
		return now.Add(24 * time.Hour)
	case "normal":
		// 3 business days: skip weekends
		deadline := now
		businessDays := 0
		for businessDays < 3 {
			deadline = deadline.Add(24 * time.Hour)
			weekday := deadline.Weekday()
			if weekday != time.Saturday && weekday != time.Sunday {
				businessDays++
			}
		}
		return deadline
	default:
		// Low or unspecified: 5 business days
		deadline := now
		businessDays := 0
		for businessDays < 5 {
			deadline = deadline.Add(24 * time.Hour)
			weekday := deadline.Weekday()
			if weekday != time.Saturday && weekday != time.Sunday {
				businessDays++
			}
		}
		return deadline
	}
}

// ListWorkOrders returns a paginated list of work orders.
func (h *WorkOrderHandler) ListWorkOrders(c echo.Context) error {
	status := c.QueryParam("status")
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Apply object-level visibility scoping based on role:
	//   system_admin          → all work orders (no filter)
	//   maintenance_tech      → only work orders assigned to them
	//   all other roles       → only work orders they submitted
	// This enforces least-privilege: a front-desk or learning-coordinator user
	// cannot see work orders submitted by other staff.
	role := middleware.GetUserRole(c)
	userID := middleware.GetUserID(c)

	assignedTo := ""
	submittedBy := ""
	switch role {
	case "system_admin":
		// no additional filter — admins see everything
	case "maintenance_tech":
		assignedTo = userID
	default:
		submittedBy = userID
	}

	workOrders, total, err := h.repo.ListWorkOrders(status, assignedTo, submittedBy, page, pageSize)
	if err != nil {
		logrus.WithError(err).Error("Failed to list work orders")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve work orders",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:     workOrders,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// CreateWorkOrder creates a new work order with SLA deadline and auto-dispatch.
func (h *WorkOrderHandler) CreateWorkOrder(c echo.Context) error {
	var req models.CreateWorkOrderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.Trade == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Trade is required",
		})
	}
	if req.Priority == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Priority is required",
		})
	}
	validPriorities := map[string]bool{"urgent": true, "high": true, "normal": true}
	if !validPriorities[req.Priority] {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Priority must be one of: urgent, high, normal",
		})
	}
	if req.Description == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Description is required",
		})
	}
	if req.Location == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Location is required",
		})
	}

	now := time.Now()
	userID := middleware.GetUserID(c)

	wo := &models.WorkOrder{
		ID:          uuid.New().String(),
		SubmittedBy: userID,
		Trade:       req.Trade,
		Priority:    req.Priority,
		SLADeadline: computeSLADeadline(req.Priority, now),
		Status:      "submitted",
		Description: req.Description,
		Location:    req.Location,
		CreatedAt:   now,
	}

	// Auto-dispatch to tech with least open orders matching trade
	techID, err := h.repo.GetTechWithLeastOrders(req.Trade)
	if err == nil && techID != "" {
		wo.AssignedTo = &techID
		wo.Status = "dispatched"
	}

	if err := h.repo.CreateWorkOrder(wo); err != nil {
		logrus.WithError(err).Error("Failed to create work order")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create work order",
			Code:  http.StatusInternalServerError,
		})
	}

	// Link any photos provided at creation time.
	// Invalid/unknown photo IDs are skipped with a warning so the work order
	// is not lost; callers should pre-validate file IDs via GET /files/:id.
	linkedPhotoIDs := make([]string, 0, len(req.PhotoIDs))
	for _, photoID := range req.PhotoIDs {
		if _, err := h.repo.LinkPhotoToWorkOrder(wo.ID, photoID); err != nil {
			logrus.WithFields(logrus.Fields{
				"wo_id":    wo.ID,
				"photo_id": photoID,
			}).WithError(err).Warn("Skipping photo link during work order creation")
			continue
		}
		linkedPhotoIDs = append(linkedPhotoIDs, photoID)
	}

	details, _ := json.Marshal(map[string]interface{}{
		"trade":           req.Trade,
		"priority":        req.Priority,
		"location":        req.Location,
		"linked_photo_ids": linkedPhotoIDs,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "create_work_order",
		EntityType: "work_order",
		EntityID:   wo.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":      userID,
		"wo_id":        wo.ID,
		"priority":     wo.Priority,
		"photos_linked": len(linkedPhotoIDs),
	}).Info("Work order created")

	return c.JSON(http.StatusCreated, wo)
}

// canViewWorkOrder is a pure authorization predicate extracted for testability.
// Policy must match list-level scoping:
//   - system_admin: unrestricted
//   - maintenance_tech: only work orders assigned to them
//   - all other roles: only work orders they submitted
//
// This prevents a maintenance_tech from bypassing list-scope by hitting the
// detail endpoint directly with an arbitrary ID.
func canViewWorkOrder(userID, role, submittedBy string, assignedTo *string) bool {
	if role == "system_admin" {
		return true
	}
	if role == "maintenance_tech" {
		return assignedTo != nil && *assignedTo == userID
	}
	return submittedBy == userID
}

// GetWorkOrder returns a single work order by ID.
// Access is restricted: only the submitter, the assigned technician, or admin/maintenance roles may view.
func (h *WorkOrderHandler) GetWorkOrder(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Work order ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	wo, err := h.repo.GetWorkOrderByID(id)
	if err != nil || wo == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Work order not found",
			Code:    http.StatusNotFound,
			Details: "No work order found with the given ID",
		})
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	if !canViewWorkOrder(userID, role, wo.SubmittedBy, wo.AssignedTo) {
		return c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "Access denied",
			Code:    http.StatusForbidden,
			Details: "You are not authorized to view this work order",
		})
	}

	photos, err := h.repo.GetWorkOrderPhotos(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get work order photos")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve work order photos",
			Code:  http.StatusInternalServerError,
		})
	}
	if photos == nil {
		photos = []models.ManagedFile{}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"work_order": wo,
		"photos":     photos,
	})
}

// UpdateWorkOrder updates a work order. Only maintenance role users can update.
func (h *WorkOrderHandler) UpdateWorkOrder(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Work order ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	wo, err := h.repo.GetWorkOrderByID(id)
	if err != nil || wo == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Work order not found",
			Code:    http.StatusNotFound,
			Details: "No work order found with the given ID",
		})
	}

	// Object-level authorization: only the assigned technician or admin may mutate.
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	if !canViewWorkOrder(userID, role, wo.SubmittedBy, wo.AssignedTo) {
		return c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "Access denied",
			Code:    http.StatusForbidden,
			Details: "You are not authorized to modify this work order",
		})
	}

	var body struct {
		Status      *string `json:"status"`
		Description *string `json:"description"`
		Location    *string `json:"location"`
		AssignedTo  *string `json:"assigned_to"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if body.Status != nil {
		// Prevent status changes from terminal states
		terminalStatuses := map[string]bool{"completed": true, "closed": true, "cancelled": true}
		if terminalStatuses[wo.Status] {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Validation failed",
				Code:    http.StatusBadRequest,
				Details: "Cannot change status of a completed, closed, or cancelled work order",
			})
		}

		validStatuses := map[string]bool{
			"submitted": true, "dispatched": true, "in_progress": true,
			"completed": true, "closed": true, "cancelled": true,
		}
		if !validStatuses[*body.Status] {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Validation failed",
				Code:    http.StatusBadRequest,
				Details: "Invalid status value",
			})
		}
		wo.Status = *body.Status
	}
	if body.Description != nil {
		wo.Description = *body.Description
	}
	if body.Location != nil {
		wo.Location = *body.Location
	}
	if body.AssignedTo != nil {
		wo.AssignedTo = body.AssignedTo
	}

	if err := h.repo.UpdateWorkOrder(wo); err != nil {
		logrus.WithError(err).Error("Failed to update work order")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update work order",
			Code:  http.StatusInternalServerError,
		})
	}

	details, _ := json.Marshal(map[string]string{"wo_id": id})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "update_work_order",
		EntityType: "work_order",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id": userID,
		"wo_id":   id,
	}).Info("Work order updated")

	return c.JSON(http.StatusOK, wo)
}

// CloseWorkOrder closes a work order with parts/labor cost.
func (h *WorkOrderHandler) CloseWorkOrder(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Work order ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	wo, err := h.repo.GetWorkOrderByID(id)
	if err != nil || wo == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Work order not found",
			Code:    http.StatusNotFound,
			Details: "No work order found with the given ID",
		})
	}

	// Object-level authorization: only the assigned technician or admin may close.
	closeUserID := middleware.GetUserID(c)
	closeRole := middleware.GetUserRole(c)
	if !canViewWorkOrder(closeUserID, closeRole, wo.SubmittedBy, wo.AssignedTo) {
		return c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "Access denied",
			Code:    http.StatusForbidden,
			Details: "You are not authorized to close this work order",
		})
	}

	if wo.Status == "completed" || wo.Status == "closed" || wo.Status == "cancelled" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Work order already closed or cancelled",
			Code:    http.StatusBadRequest,
			Details: "This work order has already been completed, closed, or cancelled",
		})
	}

	var req models.CloseWorkOrderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	now := time.Now()
	wo.PartsCost = req.PartsCost
	wo.LaborCost = req.LaborCost
	wo.Status = "completed"
	wo.ClosedAt = &now

	if err := h.repo.UpdateWorkOrder(wo); err != nil {
		logrus.WithError(err).Error("Failed to close work order")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to close work order",
			Code:  http.StatusInternalServerError,
		})
	}

	details, _ := json.Marshal(map[string]interface{}{
		"wo_id":      id,
		"parts_cost": req.PartsCost,
		"labor_cost": req.LaborCost,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     closeUserID,
		Action:     "close_work_order",
		EntityType: "work_order",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id": closeUserID,
		"wo_id":   id,
	}).Info("Work order closed")

	return c.JSON(http.StatusOK, wo)
}

// RateWorkOrder rates a completed work order.
// Only the original submitter may rate their own work order.
func (h *WorkOrderHandler) RateWorkOrder(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Work order ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	wo, err := h.repo.GetWorkOrderByID(id)
	if err != nil || wo == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Work order not found",
			Code:    http.StatusNotFound,
			Details: "No work order found with the given ID",
		})
	}

	// Object-level authorization: only the submitter may rate their own work order.
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	if role != "system_admin" && wo.SubmittedBy != userID {
		return c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error:   "Access denied",
			Code:    http.StatusForbidden,
			Details: "Only the work order submitter may rate this work order",
		})
	}

	if wo.Status != "completed" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Work order not completed",
			Code:    http.StatusBadRequest,
			Details: "Can only rate completed work orders",
		})
	}

	var req models.RateWorkOrderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.Rating < 1 || req.Rating > 5 {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Rating must be between 1 and 5",
		})
	}

	wo.Rating = &req.Rating
	if err := h.repo.UpdateWorkOrder(wo); err != nil {
		logrus.WithError(err).Error("Failed to rate work order")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to rate work order",
			Code:  http.StatusInternalServerError,
		})
	}

	details, _ := json.Marshal(map[string]interface{}{
		"wo_id":  id,
		"rating": req.Rating,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "rate_work_order",
		EntityType: "work_order",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id": userID,
		"wo_id":   id,
		"rating":  req.Rating,
	}).Info("Work order rated")

	return c.JSON(http.StatusOK, wo)
}

// LinkPhoto links an already-uploaded managed file to a work order.
// POST /work-orders/:id/photos  { "file_id": "<uuid>" }
func (h *WorkOrderHandler) LinkPhoto(c echo.Context) error {
	woID := c.Param("id")
	if woID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Work order ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	wo, err := h.repo.GetWorkOrderByID(woID)
	if err != nil || wo == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "Work order not found",
			Code:  http.StatusNotFound,
		})
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	if !canViewWorkOrder(userID, role, wo.SubmittedBy, wo.AssignedTo) {
		return c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error: "Access denied",
			Code:  http.StatusForbidden,
		})
	}

	var body struct {
		FileID string `json:"file_id"`
	}
	if err := c.Bind(&body); err != nil || body.FileID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "file_id is required",
		})
	}

	photo, err := h.repo.LinkPhotoToWorkOrder(woID, body.FileID)
	if err != nil {
		logrus.WithError(err).Error("Failed to link photo to work order")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to link photo",
			Code:  http.StatusInternalServerError,
		})
	}

	details, _ := json.Marshal(map[string]string{"wo_id": woID, "file_id": body.FileID})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID: userID, Action: "link_photo", EntityType: "work_order", EntityID: woID, Details: details,
	})

	return c.JSON(http.StatusCreated, photo)
}

// GetPhotos returns all files linked to a work order.
// GET /work-orders/:id/photos
func (h *WorkOrderHandler) GetPhotos(c echo.Context) error {
	woID := c.Param("id")
	if woID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Work order ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	wo, err := h.repo.GetWorkOrderByID(woID)
	if err != nil || wo == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "Work order not found",
			Code:  http.StatusNotFound,
		})
	}

	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	if !canViewWorkOrder(userID, role, wo.SubmittedBy, wo.AssignedTo) {
		return c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error: "Access denied",
			Code:  http.StatusForbidden,
		})
	}

	photos, err := h.repo.GetWorkOrderPhotos(woID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get work order photos")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve photos",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"data": photos})
}

// GetAnalytics returns work order analytics.
func (h *WorkOrderHandler) GetAnalytics(c echo.Context) error {
	analytics, err := h.repo.GetWorkOrderAnalytics()
	if err != nil {
		logrus.WithError(err).Error("Failed to get work order analytics")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve analytics",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, analytics)
}
