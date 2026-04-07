package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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

// MemberHandler handles membership management requests.
type MemberHandler struct {
	repo *repository.Repository
}

// NewMemberHandler creates a new MemberHandler.
func NewMemberHandler(repo *repository.Repository) *MemberHandler {
	return &MemberHandler{repo: repo}
}

// ListMembers returns a paginated list of members with optional search.
func (h *MemberHandler) ListMembers(c echo.Context) error {
	search := c.QueryParam("search")
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	members, total, err := h.repo.ListMembers(search, page, pageSize)
	if err != nil {
		logrus.WithError(err).Error("Failed to list members")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve members",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:     members,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// CreateMember creates a new member.
func (h *MemberHandler) CreateMember(c echo.Context) error {
	var req models.CreateMemberRequest
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
	if req.TierID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Tier ID is required",
		})
	}

	// Encrypt ID number using base64 (simplified for Docker env)
	var encryptedID []byte
	if req.IDNumber != "" {
		encryptedID = []byte(base64.StdEncoding.EncodeToString([]byte(req.IDNumber)))
	}

	now := time.Now()
	member := &models.Member{
		ID:                uuid.New().String(),
		Name:              req.Name,
		IDNumberEncrypted: encryptedID,
		Phone:             req.Phone,
		TierID:            req.TierID,
		PointsBalance:     0,
		StoredValue:       0,
		Status:            "active",
		ExpiresAt:         now.AddDate(1, 0, 0), // 1 year from now
		CreatedAt:         now,
	}

	if err := h.repo.CreateMember(member); err != nil {
		logrus.WithError(err).Error("Failed to create member")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create member",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{
		"name":    member.Name,
		"tier_id": member.TierID,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "create_member",
		EntityType: "member",
		EntityID:   member.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":   userID,
		"member_id": member.ID,
	}).Info("Member created")

	return c.JSON(http.StatusCreated, member)
}

// GetMember returns a single member by ID.
func (h *MemberHandler) GetMember(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Member ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	member, err := h.repo.GetMember(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get member")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve member",
			Code:  http.StatusInternalServerError,
		})
	}
	if member == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Member not found",
			Code:    http.StatusNotFound,
			Details: "No member found with the given ID",
		})
	}

	return c.JSON(http.StatusOK, member)
}

// UpdateMember updates an existing member (partial update of name/phone/tier_id).
func (h *MemberHandler) UpdateMember(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Member ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	member, err := h.repo.GetMember(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get member for update")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve member",
			Code:  http.StatusInternalServerError,
		})
	}
	if member == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Member not found",
			Code:    http.StatusNotFound,
			Details: "No member found with the given ID",
		})
	}

	var body struct {
		Name   *string `json:"name"`
		Phone  *string `json:"phone"`
		TierID *string `json:"tier_id"`
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
		member.Name = *body.Name
	}
	if body.Phone != nil {
		member.Phone = *body.Phone
	}
	if body.TierID != nil {
		member.TierID = *body.TierID
	}

	if err := h.repo.UpdateMember(member); err != nil {
		logrus.WithError(err).Error("Failed to update member")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update member",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{"member_id": id})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "update_member",
		EntityType: "member",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":   userID,
		"member_id": id,
	}).Info("Member updated")

	return c.JSON(http.StatusOK, member)
}

// FreezeMember sets a member's status to frozen.
func (h *MemberHandler) FreezeMember(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Member ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	member, err := h.repo.GetMember(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get member for freeze")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve member",
			Code:  http.StatusInternalServerError,
		})
	}
	if member == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Member not found",
			Code:    http.StatusNotFound,
			Details: "No member found with the given ID",
		})
	}

	if member.Status == "frozen" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Member already frozen",
			Code:    http.StatusBadRequest,
			Details: "This member is already frozen",
		})
	}

	now := time.Now()
	member.Status = "frozen"
	member.FrozenAt = &now

	if err := h.repo.UpdateMember(member); err != nil {
		logrus.WithError(err).Error("Failed to freeze member")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to freeze member",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{"member_id": id})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "freeze_member",
		EntityType: "member",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":   userID,
		"member_id": id,
	}).Info("Member frozen")

	return c.JSON(http.StatusOK, member)
}

// UnfreezeMember sets a member's status back to active and extends expiry.
func (h *MemberHandler) UnfreezeMember(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Member ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	member, err := h.repo.GetMember(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get member for unfreeze")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve member",
			Code:  http.StatusInternalServerError,
		})
	}
	if member == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Member not found",
			Code:    http.StatusNotFound,
			Details: "No member found with the given ID",
		})
	}

	if member.Status != "frozen" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Member not frozen",
			Code:    http.StatusBadRequest,
			Details: "Can only unfreeze a frozen member",
		})
	}

	now := time.Now()
	// Extend expires_at by the frozen duration
	if member.FrozenAt != nil {
		frozenDuration := now.Sub(*member.FrozenAt)
		member.ExpiresAt = member.ExpiresAt.Add(frozenDuration)
	}

	member.Status = "active"
	member.FrozenAt = nil

	if err := h.repo.UpdateMember(member); err != nil {
		logrus.WithError(err).Error("Failed to unfreeze member")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to unfreeze member",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{"member_id": id})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "unfreeze_member",
		EntityType: "member",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":   userID,
		"member_id": id,
	}).Info("Member unfrozen")

	return c.JSON(http.StatusOK, member)
}

// RedeemBenefit redeems a benefit for a member.
func (h *MemberHandler) RedeemBenefit(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Member ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	member, err := h.repo.GetMember(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get member for redemption")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve member",
			Code:  http.StatusInternalServerError,
		})
	}
	if member == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Member not found",
			Code:    http.StatusNotFound,
			Details: "No member found with the given ID",
		})
	}

	// Check member is not expired
	if time.Now().After(member.ExpiresAt) {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Membership expired",
			Code:    http.StatusBadRequest,
			Details: "This membership has expired",
		})
	}

	// Check member is not frozen
	if member.Status == "frozen" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Membership frozen",
			Code:    http.StatusBadRequest,
			Details: "Cannot redeem benefits while membership is frozen",
		})
	}

	var req models.RedeemRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	userID := middleware.GetUserID(c)

	switch req.Type {
	case "points_redeem":
		if req.Amount <= 0 {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Validation failed",
				Code:    http.StatusBadRequest,
				Details: "Amount must be greater than 0",
			})
		}
		pointsNeeded := int(req.Amount)
		if member.PointsBalance < pointsNeeded {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Insufficient points",
				Code:    http.StatusBadRequest,
				Details: fmt.Sprintf("Available: %d, Requested: %d", member.PointsBalance, pointsNeeded),
			})
		}
		member.PointsBalance -= pointsNeeded
		if err := h.repo.UpdateMember(member); err != nil {
			logrus.WithError(err).Error("Failed to update member points")
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to redeem points",
				Code:  http.StatusInternalServerError,
			})
		}

		tx := &models.MemberTransaction{
			ID:          uuid.New().String(),
			MemberID:    id,
			Type:        "points_redeem",
			Amount:      -req.Amount,
			Description: fmt.Sprintf("Redeemed %d points", pointsNeeded),
			PerformedBy: userID,
			CreatedAt:   time.Now(),
		}
		if err := h.repo.CreateMemberTransaction(tx); err != nil {
			logrus.WithError(err).Error("Failed to create points redeem transaction")
		}

	case "stored_value_use":
		if req.Amount <= 0 {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Validation failed",
				Code:    http.StatusBadRequest,
				Details: "Amount must be greater than 0",
			})
		}
		if member.StoredValue < req.Amount {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Insufficient stored value",
				Code:    http.StatusBadRequest,
				Details: fmt.Sprintf("Available: %.2f, Requested: %.2f", member.StoredValue, req.Amount),
			})
		}
		member.StoredValue -= req.Amount
		if err := h.repo.UpdateMember(member); err != nil {
			logrus.WithError(err).Error("Failed to update member stored value")
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to redeem stored value",
				Code:  http.StatusInternalServerError,
			})
		}

		tx := &models.MemberTransaction{
			ID:          uuid.New().String(),
			MemberID:    id,
			Type:        "stored_value_use",
			Amount:      -req.Amount,
			Description: fmt.Sprintf("Used stored value: $%.2f", req.Amount),
			PerformedBy: userID,
			CreatedAt:   time.Now(),
		}
		if err := h.repo.CreateMemberTransaction(tx); err != nil {
			logrus.WithError(err).Error("Failed to create stored value use transaction")
		}

	case "session_redeem":
		if req.PackageID == "" {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Validation failed",
				Code:    http.StatusBadRequest,
				Details: "Package ID is required for session redemption",
			})
		}

		// Find the session package from the member's packages
		packages, err := h.repo.GetSessionPackages(id)
		if err != nil {
			logrus.WithError(err).Error("Failed to get session packages")
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to retrieve session packages",
				Code:  http.StatusInternalServerError,
			})
		}

		var pkg *models.SessionPackage
		for i := range packages {
			if packages[i].ID == req.PackageID {
				pkg = &packages[i]
				break
			}
		}
		if pkg == nil {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "Session package not found",
				Code:    http.StatusNotFound,
				Details: "No session package found with the given ID",
			})
		}

		if pkg.RemainingSessions <= 0 {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "No sessions remaining",
				Code:    http.StatusBadRequest,
				Details: "This session package has no remaining sessions",
			})
		}
		if time.Now().After(pkg.ExpiresAt) {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Session package expired",
				Code:    http.StatusBadRequest,
				Details: "This session package has expired",
			})
		}

		pkg.RemainingSessions -= 1
		if err := h.repo.UpdateSessionPackage(pkg); err != nil {
			logrus.WithError(err).Error("Failed to update session package")
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to redeem session",
				Code:  http.StatusInternalServerError,
			})
		}

		tx := &models.MemberTransaction{
			ID:          uuid.New().String(),
			MemberID:    id,
			Type:        "session_redeem",
			Amount:      -1,
			Description: fmt.Sprintf("Redeemed 1 session from package %s", pkg.PackageName),
			PerformedBy: userID,
			CreatedAt:   time.Now(),
		}
		if err := h.repo.CreateMemberTransaction(tx); err != nil {
			logrus.WithError(err).Error("Failed to create session redeem transaction")
		}

	default:
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Type must be one of: points_redeem, stored_value_use, session_redeem",
		})
	}

	details, _ := json.Marshal(map[string]interface{}{
		"type":   req.Type,
		"amount": req.Amount,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "redeem_benefit",
		EntityType: "member",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":   userID,
		"member_id": id,
		"type":      req.Type,
	}).Info("Benefit redeemed")

	// Re-fetch member for updated state
	member, _ = h.repo.GetMember(id)
	return c.JSON(http.StatusOK, member)
}

// AddValue adds points or stored value to a member.
func (h *MemberHandler) AddValue(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Member ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	member, err := h.repo.GetMember(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get member for add value")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve member",
			Code:  http.StatusInternalServerError,
		})
	}
	if member == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Member not found",
			Code:    http.StatusNotFound,
			Details: "No member found with the given ID",
		})
	}

	var req models.AddValueRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Amount must be greater than 0",
		})
	}

	userID := middleware.GetUserID(c)
	var tx *models.MemberTransaction

	switch req.Type {
	case "points_earn":
		member.PointsBalance += int(req.Amount)
		if err := h.repo.UpdateMember(member); err != nil {
			logrus.WithError(err).Error("Failed to add points")
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to add points",
				Code:  http.StatusInternalServerError,
			})
		}

		tx = &models.MemberTransaction{
			ID:          uuid.New().String(),
			MemberID:    id,
			Type:        "points_earn",
			Amount:      req.Amount,
			Description: fmt.Sprintf("Earned %d points", int(req.Amount)),
			PerformedBy: userID,
			CreatedAt:   time.Now(),
		}

	case "stored_value_add":
		member.StoredValue += req.Amount
		if err := h.repo.UpdateMember(member); err != nil {
			logrus.WithError(err).Error("Failed to add stored value")
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to add stored value",
				Code:  http.StatusInternalServerError,
			})
		}

		tx = &models.MemberTransaction{
			ID:          uuid.New().String(),
			MemberID:    id,
			Type:        "stored_value_add",
			Amount:      req.Amount,
			Description: fmt.Sprintf("Added stored value: $%.2f", req.Amount),
			PerformedBy: userID,
			CreatedAt:   time.Now(),
		}

	default:
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Type must be one of: points_earn, stored_value_add",
		})
	}

	if err := h.repo.CreateMemberTransaction(tx); err != nil {
		logrus.WithError(err).Error("Failed to create add value transaction")
	}

	details, _ := json.Marshal(map[string]interface{}{
		"type":   req.Type,
		"amount": req.Amount,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "add_value",
		EntityType: "member",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":   userID,
		"member_id": id,
		"type":      req.Type,
		"amount":    req.Amount,
	}).Info("Value added to member")

	return c.JSON(http.StatusOK, tx)
}

// RefundStoredValue refunds stored value if within 7 days of the latest addition.
func (h *MemberHandler) RefundStoredValue(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Member ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	member, err := h.repo.GetMember(id)
	if err != nil {
		logrus.WithError(err).Error("Failed to get member for refund")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve member",
			Code:  http.StatusInternalServerError,
		})
	}
	if member == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Member not found",
			Code:    http.StatusNotFound,
			Details: "No member found with the given ID",
		})
	}

	var req struct {
		Amount float64 `json:"amount"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Amount must be greater than 0",
		})
	}

	// Find the latest stored_value_add transaction by looking through recent transactions
	transactions, _, err := h.repo.ListMemberTransactions(id, 1, 100)
	if err != nil {
		logrus.WithError(err).Error("Failed to list member transactions for refund check")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to check transaction history",
			Code:  http.StatusInternalServerError,
		})
	}

	var latestAdd *models.MemberTransaction
	for i := range transactions {
		if transactions[i].Type == "stored_value_add" {
			latestAdd = &transactions[i]
			break
		}
	}

	if latestAdd == nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "No stored value addition found",
			Code:    http.StatusBadRequest,
			Details: "No recent stored value addition to refund",
		})
	}

	// Check within 7 days
	if time.Since(latestAdd.CreatedAt) > 7*24*time.Hour {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Refund period expired",
			Code:    http.StatusBadRequest,
			Details: "Refunds must be requested within 7 days of the stored value addition",
		})
	}

	// Check member still has enough balance
	if member.StoredValue < req.Amount {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Insufficient balance for refund",
			Code:    http.StatusBadRequest,
			Details: "The stored value has already been partially or fully used",
		})
	}

	member.StoredValue -= req.Amount
	if err := h.repo.UpdateMember(member); err != nil {
		logrus.WithError(err).Error("Failed to refund stored value")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to process refund",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	tx := &models.MemberTransaction{
		ID:          uuid.New().String(),
		MemberID:    id,
		Type:        "stored_value_refund",
		Amount:      -req.Amount,
		Description: fmt.Sprintf("Refunded stored value: $%.2f", req.Amount),
		PerformedBy: userID,
		CreatedAt:   time.Now(),
	}
	if err := h.repo.CreateMemberTransaction(tx); err != nil {
		logrus.WithError(err).Error("Failed to create refund transaction")
	}

	details, _ := json.Marshal(map[string]interface{}{
		"amount": req.Amount,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "refund_stored_value",
		EntityType: "member",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":   userID,
		"member_id": id,
		"amount":    req.Amount,
	}).Info("Stored value refunded")

	return c.JSON(http.StatusOK, tx)
}

// ListTransactions returns paginated transactions for a member.
func (h *MemberHandler) ListTransactions(c echo.Context) error {
	memberID := c.Param("id")
	if memberID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Member ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	// Verify member exists
	member, err := h.repo.GetMember(memberID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get member for transactions")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve member",
			Code:  http.StatusInternalServerError,
		})
	}
	if member == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Member not found",
			Code:    http.StatusNotFound,
			Details: "No member found with the given ID",
		})
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	transactions, total, err := h.repo.ListMemberTransactions(memberID, page, pageSize)
	if err != nil {
		logrus.WithError(err).Error("Failed to list member transactions")
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

// ListTiers returns all membership tiers.
func (h *MemberHandler) ListTiers(c echo.Context) error {
	tiers, err := h.repo.ListMembershipTiers()
	if err != nil {
		logrus.WithError(err).Error("Failed to list tiers")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve tiers",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": tiers,
	})
}
