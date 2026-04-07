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

// WorkOrderHandler handles work order management requests.
type WorkOrderHandler struct {
	repo *repository.Repository
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

	// If the user is a maintenance_tech, filter by assigned_to
	assignedTo := ""
	role := middleware.GetUserRole(c)
	if role == "maintenance_tech" {
		assignedTo = middleware.GetUserID(c)
	}

	workOrders, total, err := h.repo.ListWorkOrders(status, assignedTo, page, pageSize)
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
	validPriorities := map[string]bool{"urgent": true, "high": true, "normal": true, "low": true}
	if !validPriorities[req.Priority] {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Priority must be one of: urgent, high, normal, low",
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
		Status:      "open",
		Description: req.Description,
		Location:    req.Location,
		CreatedAt:   now,
	}

	// Auto-dispatch to tech with least open orders matching trade
	techID, err := h.repo.GetTechWithLeastOrders(req.Trade)
	if err == nil && techID != "" {
		wo.AssignedTo = &techID
		wo.Status = "assigned"
	}

	if err := h.repo.CreateWorkOrder(wo); err != nil {
		logrus.WithError(err).Error("Failed to create work order")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create work order",
			Code:  http.StatusInternalServerError,
		})
	}

	details, _ := json.Marshal(map[string]interface{}{
		"trade":    req.Trade,
		"priority": req.Priority,
		"location": req.Location,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "create_work_order",
		EntityType: "work_order",
		EntityID:   wo.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":  userID,
		"wo_id":    wo.ID,
		"priority": wo.Priority,
	}).Info("Work order created")

	return c.JSON(http.StatusCreated, wo)
}

// GetWorkOrder returns a single work order by ID.
func (h *WorkOrderHandler) GetWorkOrder(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Work order ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	wo, err := h.repo.GetWorkOrderByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Work order not found",
			Code:    http.StatusNotFound,
			Details: "No work order found with the given ID",
		})
	}

	return c.JSON(http.StatusOK, wo)
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
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Work order not found",
			Code:    http.StatusNotFound,
			Details: "No work order found with the given ID",
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
		validStatuses := map[string]bool{
			"open": true, "assigned": true, "in_progress": true,
			"on_hold": true, "completed": true, "cancelled": true,
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

	userID := middleware.GetUserID(c)
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
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Work order not found",
			Code:    http.StatusNotFound,
			Details: "No work order found with the given ID",
		})
	}

	if wo.Status == "completed" || wo.Status == "cancelled" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Work order already closed",
			Code:    http.StatusBadRequest,
			Details: "This work order has already been completed or cancelled",
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

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]interface{}{
		"wo_id":      id,
		"parts_cost": req.PartsCost,
		"labor_cost": req.LaborCost,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "close_work_order",
		EntityType: "work_order",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id": userID,
		"wo_id":   id,
	}).Info("Work order closed")

	return c.JSON(http.StatusOK, wo)
}

// RateWorkOrder rates a completed work order.
func (h *WorkOrderHandler) RateWorkOrder(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Work order ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	wo, err := h.repo.GetWorkOrderByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Work order not found",
			Code:    http.StatusNotFound,
			Details: "No work order found with the given ID",
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

	userID := middleware.GetUserID(c)
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
