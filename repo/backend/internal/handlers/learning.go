package handlers

import (
	"encoding/json"
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

// LearningHandler handles learning knowledge base requests.
type LearningHandler struct {
	repo *repository.Repository
}

// NewLearningHandler creates a new LearningHandler.
func NewLearningHandler(repo *repository.Repository) *LearningHandler {
	return &LearningHandler{repo: repo}
}

// ListSubjects returns all learning subjects.
func (h *LearningHandler) ListSubjects(c echo.Context) error {
	subjects, err := h.repo.ListSubjects()
	if err != nil {
		logrus.WithError(err).Error("Failed to list subjects")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve subjects",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": subjects,
	})
}

// CreateSubject creates a new learning subject.
func (h *LearningHandler) CreateSubject(c echo.Context) error {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		SortOrder   int    `json:"sort_order"`
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

	subject := &models.LearningSubject{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		SortOrder:   req.SortOrder,
		CreatedAt:   time.Now(),
	}

	if err := h.repo.CreateSubject(subject); err != nil {
		logrus.WithError(err).Error("Failed to create subject")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create subject",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{"name": subject.Name})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "create_subject",
		EntityType: "learning_subject",
		EntityID:   subject.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":    userID,
		"subject_id": subject.ID,
	}).Info("Learning subject created")

	return c.JSON(http.StatusCreated, subject)
}

// UpdateSubject updates an existing learning subject.
func (h *LearningHandler) UpdateSubject(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Subject ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	subjects, err := h.repo.ListSubjects()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve subjects",
			Code:  http.StatusInternalServerError,
		})
	}

	var existing *models.LearningSubject
	for i := range subjects {
		if subjects[i].ID == id {
			existing = &subjects[i]
			break
		}
	}
	if existing == nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Subject not found",
			Code:    http.StatusNotFound,
			Details: "No subject found with the given ID",
		})
	}

	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		SortOrder   *int    `json:"sort_order"`
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
		existing.Name = *body.Name
	}
	if body.Description != nil {
		existing.Description = *body.Description
	}
	if body.SortOrder != nil {
		existing.SortOrder = *body.SortOrder
	}

	if err := h.repo.UpdateSubject(existing); err != nil {
		logrus.WithError(err).Error("Failed to update subject")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update subject",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{"subject_id": id})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "update_subject",
		EntityType: "learning_subject",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":    userID,
		"subject_id": id,
	}).Info("Learning subject updated")

	return c.JSON(http.StatusOK, existing)
}

// ListChapters returns all chapters for a subject.
func (h *LearningHandler) ListChapters(c echo.Context) error {
	subjectID := c.Param("id")
	if subjectID == "" {
		subjectID = c.QueryParam("subject_id")
	}
	if subjectID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Subject ID is required",
		})
	}

	chapters, err := h.repo.ListChapters(subjectID)
	if err != nil {
		logrus.WithError(err).Error("Failed to list chapters")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve chapters",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": chapters,
	})
}

// CreateChapter creates a new chapter under a subject.
func (h *LearningHandler) CreateChapter(c echo.Context) error {
	var req struct {
		SubjectID string `json:"subject_id"`
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.SubjectID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Subject ID is required",
		})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Name is required",
		})
	}

	chapter := &models.LearningChapter{
		ID:        uuid.New().String(),
		SubjectID: req.SubjectID,
		Name:      req.Name,
		SortOrder: req.SortOrder,
	}

	if err := h.repo.CreateChapter(chapter); err != nil {
		logrus.WithError(err).Error("Failed to create chapter")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create chapter",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{
		"subject_id": req.SubjectID,
		"name":       chapter.Name,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "create_chapter",
		EntityType: "learning_chapter",
		EntityID:   chapter.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":    userID,
		"chapter_id": chapter.ID,
	}).Info("Learning chapter created")

	return c.JSON(http.StatusCreated, chapter)
}

// ListKnowledgePoints returns knowledge points for a chapter.
func (h *LearningHandler) ListKnowledgePoints(c echo.Context) error {
	chapterID := c.QueryParam("chapter_id")
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	kps, total, err := h.repo.ListKnowledgePoints(chapterID, page, pageSize)
	if err != nil {
		logrus.WithError(err).Error("Failed to list knowledge points")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve knowledge points",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:     kps,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// CreateKnowledgePoint creates a new knowledge point.
func (h *LearningHandler) CreateKnowledgePoint(c echo.Context) error {
	var req struct {
		ChapterID       string          `json:"chapter_id"`
		Title           string          `json:"title"`
		Content         string          `json:"content"`
		Tags            []string        `json:"tags"`
		Classifications json.RawMessage `json:"classifications"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.ChapterID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Chapter ID is required",
		})
	}
	if req.Title == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Title is required",
		})
	}
	if req.Content == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Content is required",
		})
	}

	now := time.Now()
	kp := &models.KnowledgePoint{
		ID:              uuid.New().String(),
		ChapterID:       req.ChapterID,
		Title:           req.Title,
		Content:         req.Content,
		Tags:            req.Tags,
		Classifications: req.Classifications,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if kp.Tags == nil {
		kp.Tags = []string{}
	}
	if kp.Classifications == nil {
		kp.Classifications = json.RawMessage("{}")
	}

	if err := h.repo.CreateKnowledgePoint(kp); err != nil {
		logrus.WithError(err).Error("Failed to create knowledge point")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create knowledge point",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{"title": kp.Title})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "create_knowledge_point",
		EntityType: "knowledge_point",
		EntityID:   kp.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id": userID,
		"kp_id":   kp.ID,
	}).Info("Knowledge point created")

	return c.JSON(http.StatusCreated, kp)
}

// UpdateKnowledgePoint updates an existing knowledge point.
func (h *LearningHandler) UpdateKnowledgePoint(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Knowledge point ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	kp, err := h.repo.GetKnowledgePointByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Knowledge point not found",
			Code:    http.StatusNotFound,
			Details: "No knowledge point found with the given ID",
		})
	}

	var body struct {
		Title           *string          `json:"title"`
		Content         *string          `json:"content"`
		Tags            *[]string        `json:"tags"`
		Classifications *json.RawMessage `json:"classifications"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if body.Title != nil {
		if *body.Title == "" {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Validation failed",
				Code:    http.StatusBadRequest,
				Details: "Title cannot be empty",
			})
		}
		kp.Title = *body.Title
	}
	if body.Content != nil {
		kp.Content = *body.Content
	}
	if body.Tags != nil {
		kp.Tags = *body.Tags
	}
	if body.Classifications != nil {
		kp.Classifications = *body.Classifications
	}

	kp.UpdatedAt = time.Now()
	if err := h.repo.UpdateKnowledgePoint(kp); err != nil {
		logrus.WithError(err).Error("Failed to update knowledge point")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update knowledge point",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{"kp_id": id})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "update_knowledge_point",
		EntityType: "knowledge_point",
		EntityID:   id,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id": userID,
		"kp_id":   id,
	}).Info("Knowledge point updated")

	return c.JSON(http.StatusOK, kp)
}

// SearchKnowledgePoints performs a full-text search on knowledge points.
func (h *LearningHandler) SearchKnowledgePoints(c echo.Context) error {
	query := c.QueryParam("q")
	if query == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Search query 'q' is required",
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

	kps, total, err := h.repo.SearchKnowledgePoints(query, page, pageSize)
	if err != nil {
		logrus.WithError(err).Error("Failed to search knowledge points")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to search knowledge points",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, models.PaginatedResponse{
		Data:     kps,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// ImportContent accepts a multipart file upload and creates a knowledge point.
// Required fields: file, category, title, chapter_id.
func (h *LearningHandler) ImportContent(c echo.Context) error {
	chapterID := c.FormValue("chapter_id")
	category := c.FormValue("category")
	title := c.FormValue("title")

	if category == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "category is required",
		})
	}
	if title == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "title is required",
		})
	}
	if chapterID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "chapter_id is required",
		})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "File upload required",
			Code:    http.StatusBadRequest,
			Details: "A markdown or HTML file must be uploaded",
		})
	}

	// Validate file type
	filename := strings.ToLower(file.Filename)
	if !strings.HasSuffix(filename, ".md") && !strings.HasSuffix(filename, ".html") && !strings.HasSuffix(filename, ".htm") {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid file type",
			Code:    http.StatusBadRequest,
			Details: "Only markdown (.md) and HTML (.html, .htm) files are supported",
		})
	}

	src, err := file.Open()
	if err != nil {
		logrus.WithError(err).Error("Failed to open uploaded file")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to process uploaded file",
			Code:  http.StatusInternalServerError,
		})
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		logrus.WithError(err).Error("Failed to read uploaded file")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to read uploaded file",
			Code:  http.StatusInternalServerError,
		})
	}

	tagsStr := c.FormValue("tags")
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	} else {
		tags = []string{}
	}
	// Prepend category as a structured tag so it is stored and queryable.
	tags = append([]string{"category:" + category}, tags...)

	now := time.Now()
	kp := &models.KnowledgePoint{
		ID:        uuid.New().String(),
		ChapterID: chapterID,
		Title:     title,
		Content:   string(content),
		Tags:      tags,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.repo.CreateKnowledgePoint(kp); err != nil {
		logrus.WithError(err).Error("Failed to create knowledge point from import")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to create knowledge point",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{
		"title":    kp.Title,
		"filename": file.Filename,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "import_knowledge_point",
		EntityType: "knowledge_point",
		EntityID:   kp.ID,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":  userID,
		"kp_id":    kp.ID,
		"filename": file.Filename,
	}).Info("Knowledge point imported from file")

	return c.JSON(http.StatusCreated, kp)
}

// ExportContent returns a knowledge point's content as markdown.
func (h *LearningHandler) ExportContent(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Knowledge point ID is required",
			Code:  http.StatusBadRequest,
		})
	}

	kp, err := h.repo.GetKnowledgePointByID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Knowledge point not found",
			Code:    http.StatusNotFound,
			Details: "No knowledge point found with the given ID",
		})
	}

	// Build markdown output
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(kp.Title)
	sb.WriteString("\n\n")
	sb.WriteString(kp.Content)
	sb.WriteString("\n")

	if len(kp.Tags) > 0 {
		sb.WriteString("\n---\n")
		sb.WriteString("Tags: ")
		sb.WriteString(strings.Join(kp.Tags, ", "))
		sb.WriteString("\n")
	}

	c.Response().Header().Set("Content-Type", "text/markdown; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=\""+kp.Title+".md\"")
	return c.String(http.StatusOK, sb.String())
}
