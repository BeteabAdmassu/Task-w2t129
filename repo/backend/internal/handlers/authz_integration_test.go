package handlers

// authz_integration_test.go — API-level authorization integration tests (F-010).
//
// These tests call the actual HTTP handler functions through an echo.Context,
// using a stub store so no database is needed. They verify that the handler
// layer enforces 401 / 403 at the HTTP response level — not just at the
// predicate level — covering the object-authorization paths identified in F-007.

import (
	"encoding/json"
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

// ─── stub store ──────────────────────────────────────────────────────────────

// stubWorkOrderStore is a minimal in-memory store for testing WorkOrderHandler.
type stubWorkOrderStore struct {
	wo *models.WorkOrder
	// linkedPhotos records which (workOrderID, fileID) pairs were passed to
	// LinkPhotoToWorkOrder, enabling assertions in create-with-photos tests.
	linkedPhotos [][2]string
	// photosToReturn is the photo list returned by GetWorkOrderPhotos.
	photosToReturn []models.ManagedFile
}

func (s *stubWorkOrderStore) GetWorkOrderByID(_ string) (*models.WorkOrder, error) {
	return s.wo, nil
}
func (s *stubWorkOrderStore) ListWorkOrders(_ string, _ string, _, _ int) ([]models.WorkOrder, int, error) {
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

