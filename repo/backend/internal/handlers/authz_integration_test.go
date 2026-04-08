package handlers

// authz_integration_test.go — API-level authorization integration tests (F-010).
//
// These tests call the actual HTTP handler functions through an echo.Context,
// using a stub store so no database is needed. They verify that the handler
// layer enforces 401 / 403 at the HTTP response level — not just at the
// predicate level — covering the object-authorization paths identified in F-007.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"medops/internal/middleware"
	"medops/internal/models"
)

// ─── stub file store (F-003) ─────────────────────────────────────────────────

// stubFileStore implements fileStore for testing FileHandler.Download
// without a real database or filesystem (except for happy-path disk reads).
type stubFileStore struct {
	file          *models.ManagedFile // returned by GetFileByID
	fileGetErr    error               // error returned by GetFileByID
	linked        bool                // IsFileLinkedToUserWorkOrder result
	linkedErr     error               // IsFileLinkedToUserWorkOrder error
}

func (s *stubFileStore) GetFileByID(_ string) (*models.ManagedFile, error) {
	return s.file, s.fileGetErr
}
func (s *stubFileStore) IsFileLinkedToUserWorkOrder(_, _ string) (bool, error) {
	return s.linked, s.linkedErr
}
func (s *stubFileStore) GetFileByHash(_ string) (*models.ManagedFile, error) { return nil, nil }
func (s *stubFileStore) GetFilesByIDs(_ []string) ([]models.ManagedFile, error) {
	return nil, nil
}
func (s *stubFileStore) CreateFile(_ *models.ManagedFile) error { return nil }
func (s *stubFileStore) CreateAuditLog(_ *models.AuditLogEntry) error { return nil }

// ─── stub store ──────────────────────────────────────────────────────────────

// stubWorkOrderStore is a minimal in-memory store for testing WorkOrderHandler.
type stubWorkOrderStore struct {
	wo *models.WorkOrder
	// linkedPhotos records which (workOrderID, fileID) pairs were passed to
	// LinkPhotoToWorkOrder, enabling assertions in create-with-photos tests.
	linkedPhotos [][2]string
	// photosToReturn is the photo list returned by GetWorkOrderPhotos.
	photosToReturn []models.ManagedFile

	// F-002: capture the filter arguments passed to ListWorkOrders so tests can
	// assert that the handler applied the correct role-based scoping.
	lastListAssignedTo  string
	lastListSubmittedBy string
}

func (s *stubWorkOrderStore) GetWorkOrderByID(_ string) (*models.WorkOrder, error) {
	return s.wo, nil
}
func (s *stubWorkOrderStore) ListWorkOrders(_ string, assignedTo string, submittedBy string, _, _ int) ([]models.WorkOrder, int, error) {
	s.lastListAssignedTo = assignedTo
	s.lastListSubmittedBy = submittedBy
	return nil, 0, nil
}
func (s *stubWorkOrderStore) CreateWorkOrder(_ *models.WorkOrder) error   { return nil }
func (s *stubWorkOrderStore) UpdateWorkOrder(_ *models.WorkOrder) error   { return nil }
func (s *stubWorkOrderStore) GetTechWithLeastOrders(_ string) (string, error) { return "", nil }
func (s *stubWorkOrderStore) GetWorkOrderAnalytics() (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
func (s *stubWorkOrderStore) CreateAuditLog(_ *models.AuditLogEntry) error { return nil }

func (s *stubWorkOrderStore) LinkPhotoToWorkOrder(workOrderID, fileID string) (*models.WorkOrderPhoto, error) {
	s.linkedPhotos = append(s.linkedPhotos, [2]string{workOrderID, fileID})
	return &models.WorkOrderPhoto{WorkOrderID: workOrderID, FileID: fileID}, nil
}

func (s *stubWorkOrderStore) GetWorkOrderPhotos(_ string) ([]models.ManagedFile, error) {
	if s.photosToReturn != nil {
		return s.photosToReturn, nil
	}
	return []models.ManagedFile{}, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// echoCtx builds an echo.Context with the given method/path and sets
// user_id and user_role in the context (simulating passed JWT middleware).
func echoCtx(method, path, userID, role string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", userID)
	c.Set("user_role", role)
	return c, rec
}

// echoCtxJSON builds an echo.Context with a JSON request body.
func echoCtxJSON(method, path, userID, role string, body interface{}) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", userID)
	c.Set("user_role", role)
	return c, rec
}

// ─── GetWorkOrder authz tests ─────────────────────────────────────────────────

// TestGetWorkOrderAPI_Submitter_Gets200 verifies the submitter of a work order
// receives HTTP 200 from the handler (object-level authz passes).
func TestGetWorkOrderAPI_Submitter_Gets200(t *testing.T) {
	submitterID := "uid-submitter"
	wo := &models.WorkOrder{ID: "wo-001", SubmittedBy: submitterID}

	h := &WorkOrderHandler{repo: &stubWorkOrderStore{wo: wo}}

	c, rec := echoCtx(http.MethodGet, "/work-orders/wo-001", submitterID, "front_desk")
	c.SetParamNames("id")
	c.SetParamValues("wo-001")

	if err := h.GetWorkOrder(c); err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("submitter should get 200; got %d", rec.Code)
	}
}

// TestGetWorkOrderAPI_AssignedTech_Gets200 verifies the assigned technician
// receives HTTP 200.
func TestGetWorkOrderAPI_AssignedTech_Gets200(t *testing.T) {
	techID := "uid-tech"
	wo := &models.WorkOrder{ID: "wo-002", SubmittedBy: "uid-other", AssignedTo: &techID}

	h := &WorkOrderHandler{repo: &stubWorkOrderStore{wo: wo}}

	c, rec := echoCtx(http.MethodGet, "/work-orders/wo-002", techID, "maintenance_tech")
	c.SetParamNames("id")
	c.SetParamValues("wo-002")

	if err := h.GetWorkOrder(c); err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("assigned tech should get 200; got %d", rec.Code)
	}
}

// TestGetWorkOrderAPI_SystemAdmin_Gets200 verifies system_admin bypasses
// object-level checks and always receives HTTP 200.
func TestGetWorkOrderAPI_SystemAdmin_Gets200(t *testing.T) {
	wo := &models.WorkOrder{ID: "wo-003", SubmittedBy: "uid-someone-else"}

	h := &WorkOrderHandler{repo: &stubWorkOrderStore{wo: wo}}

	c, rec := echoCtx(http.MethodGet, "/work-orders/wo-003", "uid-admin", "system_admin")
	c.SetParamNames("id")
	c.SetParamValues("wo-003")

	if err := h.GetWorkOrder(c); err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("system_admin should get 200; got %d", rec.Code)
	}
}

// TestGetWorkOrderAPI_UnrelatedUser_Gets403 verifies that a user who is neither
// the submitter, the assignee, nor a privileged role receives HTTP 403.
func TestGetWorkOrderAPI_UnrelatedUser_Gets403(t *testing.T) {
	assignee := "uid-tech"
	wo := &models.WorkOrder{
		ID:          "wo-004",
		SubmittedBy: "uid-submitter",
		AssignedTo:  &assignee,
	}

	h := &WorkOrderHandler{repo: &stubWorkOrderStore{wo: wo}}

	// "uid-stranger" is not the submitter, not the assignee, not admin/tech
	c, rec := echoCtx(http.MethodGet, "/work-orders/wo-004", "uid-stranger", "front_desk")
	c.SetParamNames("id")
	c.SetParamValues("wo-004")

	if err := h.GetWorkOrder(c); err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("unrelated user should get 403; got %d", rec.Code)
	}
}

// TestGetWorkOrderAPI_LearningCoordinator_Gets403 verifies that a
// learning_coordinator role (no work-order privileges) is denied access to
// another user's work order.
func TestGetWorkOrderAPI_LearningCoordinator_Gets403(t *testing.T) {
	wo := &models.WorkOrder{ID: "wo-005", SubmittedBy: "uid-other"}

	h := &WorkOrderHandler{repo: &stubWorkOrderStore{wo: wo}}

	c, rec := echoCtx(http.MethodGet, "/work-orders/wo-005", "uid-lc", "learning_coordinator")
	c.SetParamNames("id")
	c.SetParamValues("wo-005")

	if err := h.GetWorkOrder(c); err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("learning_coordinator should get 403; got %d", rec.Code)
	}
}

// TestGetWorkOrderAPI_NotFound_Gets404 verifies that a non-existent work order
// ID causes the handler to return 404 rather than panic or 500.
func TestGetWorkOrderAPI_NotFound_Gets404(t *testing.T) {
	// Store returns nil — simulates no matching row.
	h := &WorkOrderHandler{repo: &stubWorkOrderStore{wo: nil}}

	c, rec := echoCtx(http.MethodGet, "/work-orders/missing", "uid-admin", "system_admin")
	c.SetParamNames("id")
	c.SetParamValues("missing")

	if err := h.GetWorkOrder(c); err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing work order should get 404; got %d", rec.Code)
	}
}

// ─── GetWorkOrder response shape tests ────────────────────────────────────────

// TestGetWorkOrderAPI_ResponseContainsWorkOrderAndPhotos verifies that a
// successful GET /work-orders/:id response includes both "work_order" and
// "photos" keys, matching the documented API contract.
func TestGetWorkOrderAPI_ResponseContainsWorkOrderAndPhotos(t *testing.T) {
	submitterID := "uid-submitter"
	fileID := "file-abc"
	wo := &models.WorkOrder{ID: "wo-010", SubmittedBy: submitterID}
	photos := []models.ManagedFile{{ID: fileID, OriginalName: "damage.jpg"}}

	store := &stubWorkOrderStore{wo: wo, photosToReturn: photos}
	h := &WorkOrderHandler{repo: store}

	c, rec := echoCtx(http.MethodGet, "/work-orders/wo-010", submitterID, "front_desk")
	c.SetParamNames("id")
	c.SetParamValues("wo-010")

	if err := h.GetWorkOrder(c); err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d — body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := resp["work_order"]; !ok {
		t.Error(`response missing "work_order" key`)
	}
	if _, ok := resp["photos"]; !ok {
		t.Error(`response missing "photos" key`)
	}
}

// TestGetWorkOrderAPI_PhotosEmptyArrayWhenNone verifies that the "photos" key
// is present and is an empty array when no photos are linked.
func TestGetWorkOrderAPI_PhotosEmptyArrayWhenNone(t *testing.T) {
	submitterID := "uid-submitter"
	wo := &models.WorkOrder{ID: "wo-011", SubmittedBy: submitterID}

	h := &WorkOrderHandler{repo: &stubWorkOrderStore{wo: wo}}

	c, rec := echoCtx(http.MethodGet, "/work-orders/wo-011", submitterID, "front_desk")
	c.SetParamNames("id")
	c.SetParamValues("wo-011")

	if err := h.GetWorkOrder(c); err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	photosVal, ok := resp["photos"]
	if !ok {
		t.Fatal(`response missing "photos" key`)
	}
	photos, ok := photosVal.([]interface{})
	if !ok {
		t.Fatalf(`"photos" is not an array; got %T`, photosVal)
	}
	if len(photos) != 0 {
		t.Errorf("expected empty photos array; got %d items", len(photos))
	}
}

// TestGetWorkOrderAPI_UnauthorizedUserCannotSeePhotos verifies that an
// unauthorized user is still turned away with 403 and does NOT receive photos,
// even if the work order has linked photos.
func TestGetWorkOrderAPI_UnauthorizedUserCannotSeePhotos(t *testing.T) {
	wo := &models.WorkOrder{ID: "wo-012", SubmittedBy: "uid-owner"}
	photos := []models.ManagedFile{{ID: "file-secret", OriginalName: "confidential.jpg"}}

	store := &stubWorkOrderStore{wo: wo, photosToReturn: photos}
	h := &WorkOrderHandler{repo: store}

	c, rec := echoCtx(http.MethodGet, "/work-orders/wo-012", "uid-stranger", "inventory_pharmacist")
	c.SetParamNames("id")
	c.SetParamValues("wo-012")

	if err := h.GetWorkOrder(c); err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("unauthorized user should get 403; got %d", rec.Code)
	}
	// The response body must not contain the photo data.
	body := rec.Body.String()
	if strings.Contains(body, "file-secret") {
		t.Error("403 response body must not contain photo ID of unauthorized work order")
	}
}

// ─── CreateWorkOrder photo-linking tests ──────────────────────────────────────

// TestCreateWorkOrderAPI_LinksProvidedPhotoIDs verifies that when photo_ids
// are included in a create request, the handler calls LinkPhotoToWorkOrder for
// each ID after the work order is persisted.
func TestCreateWorkOrderAPI_LinksProvidedPhotoIDs(t *testing.T) {
	store := &stubWorkOrderStore{}
	h := &WorkOrderHandler{repo: store}

	payload := map[string]interface{}{
		"trade":       "electrical",
		"priority":    "high",
		"description": "Broken outlet in exam room",
		"location":    "Building A, Room 101",
		"photo_ids":   []string{"file-001", "file-002"},
	}

	c, rec := echoCtxJSON(http.MethodPost, "/work-orders", "uid-user", "front_desk", payload)

	if err := h.CreateWorkOrder(c); err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201; got %d — body: %s", rec.Code, rec.Body.String())
	}
	if len(store.linkedPhotos) != 2 {
		t.Errorf("expected 2 photo links; got %d", len(store.linkedPhotos))
	}
	seen := map[string]bool{}
	for _, pair := range store.linkedPhotos {
		seen[pair[1]] = true
	}
	for _, id := range []string{"file-001", "file-002"} {
		if !seen[id] {
			t.Errorf("photo %q was not linked", id)
		}
	}
}

// TestCreateWorkOrderAPI_NoPhotosIsValid verifies that omitting photo_ids
// creates the work order successfully with no link calls.
func TestCreateWorkOrderAPI_NoPhotosIsValid(t *testing.T) {
	store := &stubWorkOrderStore{}
	h := &WorkOrderHandler{repo: store}

	payload := map[string]interface{}{
		"trade":       "plumbing",
		"priority":    "normal",
		"description": "Leaking sink",
		"location":    "Building B, Room 202",
	}

	c, rec := echoCtxJSON(http.MethodPost, "/work-orders", "uid-user", "front_desk", payload)

	if err := h.CreateWorkOrder(c); err != nil {
		t.Fatalf("handler returned unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201; got %d — body: %s", rec.Code, rec.Body.String())
	}
	if len(store.linkedPhotos) != 0 {
		t.Errorf("expected no photo links; got %d", len(store.linkedPhotos))
	}
}

// ─── JWT middleware authz tests ───────────────────────────────────────────────

// TestJWTMiddleware_NoToken_Returns401 verifies the JWT middleware rejects
// requests that carry no Authorization header with HTTP 401.
func TestJWTMiddleware_NoToken_Returns401(t *testing.T) {
	e := echo.New()
	e.GET("/protected", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, middleware.JWTAuth("test-secret"))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("no token should yield 401; got %d", rec.Code)
	}
}

// TestJWTMiddleware_InvalidToken_Returns401 verifies the middleware rejects
// a malformed token with HTTP 401.
func TestJWTMiddleware_InvalidToken_Returns401(t *testing.T) {
	e := echo.New()
	e.GET("/protected", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, middleware.JWTAuth("test-secret"))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("invalid token should yield 401; got %d", rec.Code)
	}
}

// TestJWTMiddleware_ValidToken_Passes verifies the middleware allows a correctly
// signed JWT through and populates user_id / user_role in the context.
func TestJWTMiddleware_ValidToken_Passes(t *testing.T) {
	const secret = "test-secret-32-chars-long-enough!"
	token, err := middleware.GenerateToken("uid-1", "system_admin", secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	e := echo.New()
	var capturedRole string
	e.GET("/protected", func(c echo.Context) error {
		capturedRole = middleware.GetUserRole(c)
		return c.String(http.StatusOK, "ok")
	}, middleware.JWTAuth(secret))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("valid token should yield 200; got %d", rec.Code)
	}
	if capturedRole != "system_admin" {
		t.Errorf("expected role system_admin in context, got %q", capturedRole)
	}
}

// TestRoleMiddleware_WrongRole_Returns403 verifies the role-enforcement
// middleware rejects users whose role is not in the allowed set with 403.
func TestRoleMiddleware_WrongRole_Returns403(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)
	c.Set("user_role", "front_desk") // simulate authenticated non-admin

	handler := middleware.RequireRole("system_admin")(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("wrong role should yield 403; got %d", rec.Code)
	}
}

// TestRoleMiddleware_CorrectRole_Passes verifies that a user with the required
// role is allowed through the role-enforcement middleware.
func TestRoleMiddleware_CorrectRole_Passes(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)
	c.Set("user_role", "system_admin")

	handler := middleware.RequireRole("system_admin")(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("correct role should yield 200; got %d", rec.Code)
	}
}

// ─── Sensitive-field reveal endpoint authz tests ──────────────────────────────

// TestRevealSensitiveFields_FrontDesk_Forbidden verifies that a front_desk user
// receives HTTP 403 when trying to access the sensitive-field reveal endpoint.
// The adminRole middleware fires before the handler, so the nil repo is never reached.
func TestRevealSensitiveFields_FrontDesk_Forbidden(t *testing.T) {
	h := &MemberHandler{repo: nil, encryptKey: make([]byte, 32)}
	adminMW := middleware.RequireRole("system_admin")

	c, rec := echoCtx(http.MethodGet, "/members/some-id/sensitive", "u-frontdesk", "front_desk")
	c.SetParamNames("id")
	c.SetParamValues("some-id")

	wrapped := adminMW(h.RevealSensitiveFields)
	if err := wrapped(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("front_desk should get 403 on reveal endpoint; got %d", rec.Code)
	}
}

// TestRevealSensitiveFields_Admin_PassesMiddleware verifies that a system_admin
// user is allowed through the adminRole middleware on the reveal endpoint.
// (Handler will fail because repo is nil — we only check middleware passes.)
func TestRevealSensitiveFields_Admin_PassesMiddleware(t *testing.T) {
	adminMW := middleware.RequireRole("system_admin")

	c, rec := echoCtx(http.MethodGet, "/members/some-id/sensitive", "u-admin", "system_admin")
	c.SetParamNames("id")
	c.SetParamValues("some-id")

	// Replace handler with a sentinel that records it was called
	called := false
	wrapped := adminMW(func(c echo.Context) error {
		called = true
		return c.String(http.StatusOK, `{"member_id":"some-id"}`)
	})
	if err := wrapped(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("system_admin should pass middleware; got %d", rec.Code)
	}
	if !called {
		t.Error("handler was not called for system_admin")
	}
}

// ─── ApplyUpdate versioned response tests ────────────────────────────────────

// TestExtractPackageVersion_ReturnsVersionFile verifies extractPackageVersion
// reads VERSION file content from a directory.
func TestExtractPackageVersion_ReturnsVersionFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("2.3.1\n"), 0644); err != nil {
		t.Fatalf("failed to create VERSION file: %v", err)
	}
	got := extractPackageVersion(dir, "fallback")
	if got != "2.3.1" {
		t.Errorf("expected version 2.3.1; got %q", got)
	}
}

// TestExtractPackageVersion_FallbackWhenMissing verifies extractPackageVersion
// returns the fallback when no VERSION file is present.
func TestExtractPackageVersion_FallbackWhenMissing(t *testing.T) {
	dir := t.TempDir()
	got := extractPackageVersion(dir, "20260101T000000Z")
	if got != "20260101T000000Z" {
		t.Errorf("expected fallback; got %q", got)
	}
}

// ─── helpers for cancelled-status tests ──────────────────────────────────────

func echoCtxWithBody(method, path, userID, role, jsonBody string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, path, strings.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", userID)
	c.Set("user_role", role)
	return c, rec
}

func strPtr(s string) *string { return &s }

// ─── cancelled status tests ───────────────────────────────────────────────────

// TestUpdateWorkOrder_CancelledStatus_IsAccepted verifies that setting status
// to "cancelled" is accepted by the UpdateWorkOrder handler (backend validStatuses).
func TestUpdateWorkOrder_CancelledStatus_IsAccepted(t *testing.T) {
	wo := &models.WorkOrder{ID: "wo-cancel-01", SubmittedBy: "uid-sub", AssignedTo: strPtr("uid-tech"), Status: "dispatched"}
	store := &stubWorkOrderStore{wo: wo}
	h := &WorkOrderHandler{repo: store}

	body := `{"status":"cancelled"}`
	c, rec := echoCtxWithBody(http.MethodPut, "/work-orders/wo-cancel-01", "uid-tech", "maintenance_tech", body)
	c.SetParamNames("id")
	c.SetParamValues("wo-cancel-01")

	if err := h.UpdateWorkOrder(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200; got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestCloseWorkOrder_CancelledOrder_IsRejected verifies that the close endpoint
// rejects a work order that has already been cancelled.
func TestCloseWorkOrder_CancelledOrder_IsRejected(t *testing.T) {
	wo := &models.WorkOrder{ID: "wo-cancel-02", SubmittedBy: "uid-sub", Status: "cancelled"}
	store := &stubWorkOrderStore{wo: wo}
	h := &WorkOrderHandler{repo: store}

	body := `{"parts_cost":0,"labor_cost":0}`
	c, rec := echoCtxWithBody(http.MethodPost, "/work-orders/wo-cancel-02/close", "uid-tech", "maintenance_tech", body)
	c.SetParamNames("id")
	c.SetParamValues("wo-cancel-02")

	if err := h.CloseWorkOrder(c); err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for cancelled order; got %d", rec.Code)
	}
}

// TestUpdateWorkOrder_AdvanceFromCancelled_IsRejected verifies that a cancelled
// work order cannot be advanced to another status (no resurrection).
func TestUpdateWorkOrder_AdvanceFromCancelled_IsRejected(t *testing.T) {
	wo := &models.WorkOrder{ID: "wo-cancel-03", SubmittedBy: "uid-sub", Status: "cancelled"}
	store := &stubWorkOrderStore{wo: wo}
	h := &WorkOrderHandler{repo: store}

	body := `{"status":"in_progress"}`
	c, rec := echoCtxWithBody(http.MethodPut, "/work-orders/wo-cancel-03", "uid-tech", "maintenance_tech", body)
	c.SetParamNames("id")
	c.SetParamValues("wo-cancel-03")

	if err := h.UpdateWorkOrder(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when advancing from cancelled; got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// ─── F-002: ListWorkOrders role-based scoping tests ──────────────────────────
//
// The handler must pass different filter arguments to the store based on the
// caller's role:
//   - system_admin   → no assignedTo / submittedBy filter (sees all)
//   - maintenance_tech → assignedTo=userID, submittedBy=""
//   - all others     → submittedBy=userID, assignedTo=""

// TestListWorkOrders_SystemAdmin_NoFilter verifies that a system_admin call
// reaches the store with empty assignedTo and submittedBy (unscoped list).
func TestListWorkOrders_SystemAdmin_NoFilter(t *testing.T) {
	store := &stubWorkOrderStore{}
	h := &WorkOrderHandler{repo: store}

	c, rec := echoCtx(http.MethodGet, "/work-orders", "uid-admin", "system_admin")
	if err := h.ListWorkOrders(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200; got %d", rec.Code)
	}
	if store.lastListAssignedTo != "" {
		t.Errorf("system_admin: assignedTo should be empty; got %q", store.lastListAssignedTo)
	}
	if store.lastListSubmittedBy != "" {
		t.Errorf("system_admin: submittedBy should be empty; got %q", store.lastListSubmittedBy)
	}
}

// TestListWorkOrders_MaintenanceTech_AssignedToFilter verifies that a
// maintenance_tech call passes their userID as assignedTo, not submittedBy.
func TestListWorkOrders_MaintenanceTech_AssignedToFilter(t *testing.T) {
	store := &stubWorkOrderStore{}
	h := &WorkOrderHandler{repo: store}

	c, rec := echoCtx(http.MethodGet, "/work-orders", "uid-tech-1", "maintenance_tech")
	if err := h.ListWorkOrders(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200; got %d", rec.Code)
	}
	if store.lastListAssignedTo != "uid-tech-1" {
		t.Errorf("maintenance_tech: assignedTo should be userID; got %q", store.lastListAssignedTo)
	}
	if store.lastListSubmittedBy != "" {
		t.Errorf("maintenance_tech: submittedBy should be empty; got %q", store.lastListSubmittedBy)
	}
}

// TestListWorkOrders_FrontDesk_SubmitterFilter verifies that a front_desk user
// can only list work orders they submitted (submittedBy=userID, assignedTo="").
func TestListWorkOrders_FrontDesk_SubmitterFilter(t *testing.T) {
	store := &stubWorkOrderStore{}
	h := &WorkOrderHandler{repo: store}

	c, rec := echoCtx(http.MethodGet, "/work-orders", "uid-fd-1", "front_desk")
	if err := h.ListWorkOrders(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200; got %d", rec.Code)
	}
	if store.lastListSubmittedBy != "uid-fd-1" {
		t.Errorf("front_desk: submittedBy should be userID; got %q", store.lastListSubmittedBy)
	}
	if store.lastListAssignedTo != "" {
		t.Errorf("front_desk: assignedTo should be empty; got %q", store.lastListAssignedTo)
	}
}

// TestListWorkOrders_LearningCoordinator_SubmitterFilter verifies that a
// learning_coordinator (non-privileged role) is also scoped to submittedBy.
func TestListWorkOrders_LearningCoordinator_SubmitterFilter(t *testing.T) {
	store := &stubWorkOrderStore{}
	h := &WorkOrderHandler{repo: store}

	c, rec := echoCtx(http.MethodGet, "/work-orders", "uid-lc-1", "learning_coordinator")
	if err := h.ListWorkOrders(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200; got %d", rec.Code)
	}
	if store.lastListSubmittedBy != "uid-lc-1" {
		t.Errorf("learning_coordinator: submittedBy should be userID; got %q", store.lastListSubmittedBy)
	}
	if store.lastListAssignedTo != "" {
		t.Errorf("learning_coordinator: assignedTo should be empty; got %q", store.lastListAssignedTo)
	}
}

// ─── F-005: CreateWorkOrder priority validation tests ────────────────────────
//
// The backend validPriorities map must accept urgent/high/normal and reject
// anything else (including "low", which was erroneously accepted before F-005).

// TestCreateWorkOrder_LowPriority_Rejected verifies that priority "low" is
// rejected with HTTP 400 (removed from validPriorities in F-005 fix).
func TestCreateWorkOrder_LowPriority_Rejected(t *testing.T) {
	store := &stubWorkOrderStore{}
	h := &WorkOrderHandler{repo: store}

	payload := map[string]interface{}{
		"trade":       "plumbing",
		"priority":    "low",
		"description": "Dripping tap",
		"location":    "Room 1",
	}
	c, rec := echoCtxJSON(http.MethodPost, "/work-orders", "uid-user", "front_desk", payload)

	if err := h.CreateWorkOrder(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("priority 'low' should yield 400; got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestCreateWorkOrder_InvalidPriority_Rejected verifies that an arbitrary
// invalid priority string is rejected with 400.
func TestCreateWorkOrder_InvalidPriority_Rejected(t *testing.T) {
	store := &stubWorkOrderStore{}
	h := &WorkOrderHandler{repo: store}

	payload := map[string]interface{}{
		"trade":       "electrical",
		"priority":    "critical",
		"description": "Power outage",
		"location":    "Room 2",
	}
	c, rec := echoCtxJSON(http.MethodPost, "/work-orders", "uid-user", "front_desk", payload)

	if err := h.CreateWorkOrder(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("priority 'critical' should yield 400; got %d", rec.Code)
	}
}

// TestCreateWorkOrder_ValidPriorities_AllAccepted verifies that all three
// schema-valid priority values (urgent, high, normal) result in 201.
func TestCreateWorkOrder_ValidPriorities_AllAccepted(t *testing.T) {
	for _, priority := range []string{"urgent", "high", "normal"} {
		priority := priority
		t.Run(priority, func(t *testing.T) {
			store := &stubWorkOrderStore{}
			h := &WorkOrderHandler{repo: store}

			payload := map[string]interface{}{
				"trade":       "hvac",
				"priority":    priority,
				"description": "HVAC issue",
				"location":    "Room 3",
			}
			c, rec := echoCtxJSON(http.MethodPost, "/work-orders", "uid-user", "front_desk", payload)

			if err := h.CreateWorkOrder(c); err != nil {
				t.Fatalf("handler error for priority %q: %v", priority, err)
			}
			if rec.Code != http.StatusCreated {
				t.Errorf("priority %q should yield 201; got %d — body: %s", priority, rec.Code, rec.Body.String())
			}
		})
	}
}

// ─── F-003: Download handler secondary authorization tests ───────────────────
//
// F-003 added a secondary authorization path in the Download handler: when
// canDownloadFile() returns false (primary check fails), the handler calls
// repo.IsFileLinkedToUserWorkOrder. If that returns true, access is granted.
//
// These tests exercise the full Download handler code path with stub stores
// so no real database or permanent filesystem is needed.

// makeTestFile writes content to a temp file and returns its path.
// The file is automatically cleaned up when t finishes.
func makeTestFile(t *testing.T, content []byte) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "test-file.bin")
	if err := os.WriteFile(p, content, 0644); err != nil {
		t.Fatalf("makeTestFile: %v", err)
	}
	return p
}

// newDownloadCtx builds an echo.Context for a GET /files/:id request.
func newDownloadCtx(fileID, userID, role string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/files/"+fileID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", userID)
	c.Set("user_role", role)
	c.SetParamNames("id")
	c.SetParamValues(fileID)
	return c, rec
}

// TestDownload_PrimaryAuth_Admin_Allowed verifies that system_admin bypasses both
// primary and secondary checks and receives 200 when the file exists on disk.
func TestDownload_PrimaryAuth_Admin_Allowed(t *testing.T) {
	storagePath := makeTestFile(t, []byte("binary content"))
	uploaderID := "uid-uploader"
	mf := &models.ManagedFile{
		ID:           "file-admin-01",
		OriginalName: "report.pdf",
		MimeType:     "application/pdf",
		StoragePath:  storagePath,
		UploadedBy:   &uploaderID,
	}
	h := &FileHandler{repo: &stubFileStore{file: mf}, dataDir: t.TempDir()}

	c, rec := newDownloadCtx("file-admin-01", "uid-admin", "system_admin")
	if err := h.Download(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("system_admin should get 200; got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestDownload_PrimaryAuth_Uploader_Allowed verifies that the original uploader
// is granted access (primary check), regardless of role.
func TestDownload_PrimaryAuth_Uploader_Allowed(t *testing.T) {
	storagePath := makeTestFile(t, []byte("photo data"))
	uploaderID := "uid-tech"
	mf := &models.ManagedFile{
		ID:           "file-uploader-01",
		OriginalName: "photo.jpg",
		MimeType:     "image/jpeg",
		StoragePath:  storagePath,
		UploadedBy:   &uploaderID,
	}
	h := &FileHandler{repo: &stubFileStore{file: mf}, dataDir: t.TempDir()}

	c, rec := newDownloadCtx("file-uploader-01", "uid-tech", "maintenance_tech")
	if err := h.Download(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("uploader should get 200; got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestDownload_SecondaryAuth_LinkedWO_Allowed verifies scenario (1): a non-uploader,
// non-admin user who is linked to a work order containing this file gets 200.
// This is the core F-003 secondary authorization path.
func TestDownload_SecondaryAuth_LinkedWO_Allowed(t *testing.T) {
	storagePath := makeTestFile(t, []byte("damage photo"))
	uploaderID := "uid-different-user"
	mf := &models.ManagedFile{
		ID:           "file-linked-01",
		OriginalName: "damage.jpg",
		MimeType:     "image/jpeg",
		StoragePath:  storagePath,
		UploadedBy:   &uploaderID,
	}
	// Stub: primary check fails (different uploader), secondary returns linked=true
	store := &stubFileStore{file: mf, linked: true}
	h := &FileHandler{repo: store, dataDir: t.TempDir()}

	// maintenance_tech who didn't upload but is assigned to the work order
	c, rec := newDownloadCtx("file-linked-01", "uid-tech-assigned", "maintenance_tech")
	if err := h.Download(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("secondary auth via WO link should give 200; got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestDownload_SecondaryAuth_NotLinkedWO_Denied verifies scenario (2): a non-uploader,
// non-admin user with no linked work order photo receives 403.
// Both primary and secondary checks must fail for the deny to fire.
func TestDownload_SecondaryAuth_NotLinkedWO_Denied(t *testing.T) {
	uploaderID := "uid-uploader"
	mf := &models.ManagedFile{
		ID:          "file-notlinked-01",
		OriginalName: "document.pdf",
		MimeType:    "application/pdf",
		StoragePath: "/nonexistent/path.pdf", // won't be reached — auth fails first
		UploadedBy:  &uploaderID,
	}
	// Stub: primary check fails (different user), secondary returns linked=false
	store := &stubFileStore{file: mf, linked: false}
	h := &FileHandler{repo: store, dataDir: t.TempDir()}

	c, rec := newDownloadCtx("file-notlinked-01", "uid-stranger", "front_desk")
	if err := h.Download(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-linked user should get 403; got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestDownload_SecondaryAuth_LinkCheckError_Denied verifies scenario (4): when
// IsFileLinkedToUserWorkOrder returns an error, the handler logs the error and
// defaults to deny (does NOT grant access on error).
func TestDownload_SecondaryAuth_LinkCheckError_Denied(t *testing.T) {
	uploaderID := "uid-uploader"
	mf := &models.ManagedFile{
		ID:          "file-linkerr-01",
		OriginalName: "doc.pdf",
		MimeType:    "application/pdf",
		StoragePath: "/nonexistent/path.pdf",
		UploadedBy:  &uploaderID,
	}
	// Stub: secondary check returns an error (db outage scenario)
	store := &stubFileStore{
		file:      mf,
		linked:    false, // error path must not grant access
		linkedErr: fmt.Errorf("simulated db error"),
	}
	h := &FileHandler{repo: store, dataDir: t.TempDir()}

	c, rec := newDownloadCtx("file-linkerr-01", "uid-stranger", "front_desk")
	if err := h.Download(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// On error, the handler logs a warning and falls through to the deny branch
	// (linked is still false after the error, so !linked → 403).
	if rec.Code != http.StatusForbidden {
		t.Errorf("linkage error should default to 403 deny; got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestDownload_SecondaryAuth_InventoryPharmacist_Allowed verifies that
// inventory_pharmacist is covered by the primary check (blanket access).
func TestDownload_SecondaryAuth_InventoryPharmacist_Allowed(t *testing.T) {
	storagePath := makeTestFile(t, []byte("label data"))
	someUploader := "uid-other"
	mf := &models.ManagedFile{
		ID:           "file-pharm-01",
		OriginalName: "label.pdf",
		MimeType:     "application/pdf",
		StoragePath:  storagePath,
		UploadedBy:   &someUploader,
	}
	// Secondary check never needed — primary grants access for inventory_pharmacist
	store := &stubFileStore{file: mf, linked: false}
	h := &FileHandler{repo: store, dataDir: t.TempDir()}

	c, rec := newDownloadCtx("file-pharm-01", "uid-pharm", "inventory_pharmacist")
	if err := h.Download(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("inventory_pharmacist should get 200; got %d — body: %s", rec.Code, rec.Body.String())
	}
}

// TestDownload_FileNotFound_Returns404 verifies that a missing file record
// yields 404 before authorization is even checked.
func TestDownload_FileNotFound_Returns404(t *testing.T) {
	store := &stubFileStore{file: nil} // GetFileByID returns nil
	h := &FileHandler{repo: store, dataDir: t.TempDir()}

	c, rec := newDownloadCtx("file-missing", "uid-admin", "system_admin")
	if err := h.Download(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing file should get 404; got %d", rec.Code)
	}
}

