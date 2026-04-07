package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

// ---------- SLA deadline tests (pure function, no DB) ----------

func TestComputeSLADeadline_Urgent(t *testing.T) {
	now := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC) // Monday 10:00
	deadline := computeSLADeadline("urgent", now)
	expected := now.Add(4 * time.Hour)
	if !deadline.Equal(expected) {
		t.Errorf("urgent SLA: expected %v, got %v", expected, deadline)
	}
}

func TestComputeSLADeadline_High(t *testing.T) {
	now := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	deadline := computeSLADeadline("high", now)
	expected := now.Add(24 * time.Hour)
	if !deadline.Equal(expected) {
		t.Errorf("high SLA: expected %v, got %v", expected, deadline)
	}
}

func TestComputeSLADeadline_Normal_SkipsWeekend(t *testing.T) {
	// Friday 10:00 → 3 business days skipping Sat+Sun → Wednesday next week
	now := time.Date(2026, 1, 9, 10, 0, 0, 0, time.UTC) // Friday Jan 9
	deadline := computeSLADeadline("normal", now)
	// Mon Jan 12 = day 1, Tue Jan 13 = day 2, Wed Jan 14 = day 3
	if deadline.Weekday() != time.Wednesday {
		t.Errorf("normal SLA from Friday: expected Wednesday, got %v (%v)", deadline.Weekday(), deadline)
	}
}

func TestComputeSLADeadline_Normal_Weekday(t *testing.T) {
	// Monday 10:00 → 3 business days → Thursday same week
	now := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC) // Monday Jan 5
	deadline := computeSLADeadline("normal", now)
	// Tue Jan 6 = day 1, Wed Jan 7 = day 2, Thu Jan 8 = day 3
	if deadline.Weekday() != time.Thursday {
		t.Errorf("normal SLA from Monday: expected Thursday, got %v (%v)", deadline.Weekday(), deadline)
	}
}

// ---------- MemberHandler AES encryption tests ----------

func TestEncryptField_ProducesNonEmptyOutput(t *testing.T) {
	h := NewMemberHandler(nil, "0123456789abcdef0123456789abcdef")
	enc, err := h.encryptField("ID-123456")
	if err != nil {
		t.Fatalf("encryptField failed: %v", err)
	}
	if len(enc) == 0 {
		t.Error("encryptField returned empty output")
	}
}

func TestEncryptField_DifferentPlaintextsProduceDifferentCiphertexts(t *testing.T) {
	h := NewMemberHandler(nil, "0123456789abcdef0123456789abcdef")
	enc1, _ := h.encryptField("ID-111111")
	enc2, _ := h.encryptField("ID-222222")
	if string(enc1) == string(enc2) {
		t.Error("different plaintexts produced identical ciphertext")
	}
}

func TestEncryptField_RandomNonce_SamePlaintextDifferentCiphertext(t *testing.T) {
	// AES-GCM uses a random nonce, so same plaintext → different ciphertext each call
	h := NewMemberHandler(nil, "0123456789abcdef0123456789abcdef")
	enc1, _ := h.encryptField("ID-123456")
	enc2, _ := h.encryptField("ID-123456")
	if string(enc1) == string(enc2) {
		t.Error("AES-GCM should produce different ciphertexts for same plaintext (random nonce)")
	}
}

// ---------- Health check handler ----------

func TestHealthCheck_ReturnsStatusOK(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewSystemHandler(nil, "", "")
	if err := h.HealthCheck(c); err != nil {
		t.Fatalf("HealthCheck returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`expected status="ok", got %q`, body["status"])
	}
	if body["timestamp"] == "" {
		t.Error("expected non-empty timestamp in health response")
	}
}

// ---------- Auth handler — input validation (before DB call) ----------

func TestLogin_EmptyUsername_Returns400(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		strings.NewReader(`{"username":"","password":""}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewAuthHandler(nil, "test-secret")
	if err := h.Login(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty credentials, got %d", rec.Code)
	}
}

func TestChangePassword_ShortPassword_Returns400(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/auth/password",
		strings.NewReader(`{"old_password":"OldPass1234","new_password":"short"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "some-user-id")

	h := NewAuthHandler(nil, "test-secret")
	if err := h.ChangePassword(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short password, got %d", rec.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["error"] == nil {
		t.Error("expected error field in response body")
	}
}

// ---------- Inventory handler — input validation (before DB call) ----------

func TestCreateSKU_MissingName_Returns400(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/skus",
		strings.NewReader(`{"unit_of_measure":"capsule"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewInventoryHandler(nil)
	if err := h.CreateSKU(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name, got %d", rec.Code)
	}
}

func TestCreateSKU_MissingUnitOfMeasure_Returns400(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/skus",
		strings.NewReader(`{"name":"Amoxicillin 500mg"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewInventoryHandler(nil)
	if err := h.CreateSKU(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing unit_of_measure, got %d", rec.Code)
	}
}

func TestReceive_ExpiredDate_Returns400(t *testing.T) {
	e := echo.New()
	body := `{"sku_id":"some-id","lot_number":"LOT-001","expiration_date":"2020-01-01","quantity":50,"reason_code":"purchase"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/inventory/receive",
		strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewInventoryHandler(nil)
	if err := h.Receive(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for expired date, got %d", rec.Code)
	}
}

func TestReceive_ZeroQuantity_Returns400(t *testing.T) {
	e := echo.New()
	body := `{"sku_id":"some-id","lot_number":"LOT-001","expiration_date":"2030-01-01","quantity":0,"reason_code":"purchase"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/inventory/receive",
		strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewInventoryHandler(nil)
	if err := h.Receive(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for zero quantity, got %d", rec.Code)
	}
}

// ---------- Work order handler — input validation (before DB call) ----------

func TestCreateWorkOrder_MissingTrade_Returns400(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/work-orders",
		strings.NewReader(`{"priority":"high","description":"fix light","location":"Room 1"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewWorkOrderHandler(nil)
	if err := h.CreateWorkOrder(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing trade, got %d", rec.Code)
	}
}

func TestCreateWorkOrder_InvalidPriority_Returns400(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/work-orders",
		strings.NewReader(`{"trade":"electrical","priority":"critical","description":"fix","location":"Room 1"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewWorkOrderHandler(nil)
	if err := h.CreateWorkOrder(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid priority, got %d", rec.Code)
	}
}

func TestCreateWorkOrder_MissingDescription_Returns400(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/work-orders",
		strings.NewReader(`{"trade":"electrical","priority":"high","location":"Room 1"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewWorkOrderHandler(nil)
	if err := h.CreateWorkOrder(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing description, got %d", rec.Code)
	}
}

// ---------- Member handler — input validation (before DB call) ----------

func TestCreateMember_MissingName_Returns400(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/members",
		strings.NewReader(`{"tier_id":"some-tier-id"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewMemberHandler(nil, "0123456789abcdef0123456789abcdef")
	if err := h.CreateMember(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name, got %d", rec.Code)
	}
}

func TestCreateMember_MissingTierID_Returns400(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/members",
		strings.NewReader(`{"name":"John Doe"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewMemberHandler(nil, "0123456789abcdef0123456789abcdef")
	if err := h.CreateMember(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing tier_id, got %d", rec.Code)
	}
}

// ---------- Learning handler — import validation (before DB call) ----------

func TestImportContent_MissingCategory_Returns400(t *testing.T) {
	e := echo.New()
	// multipart with title + file but no category
	body := "--boundary\r\nContent-Disposition: form-data; name=\"title\"\r\n\r\nTest Title\r\n" +
		"--boundary\r\nContent-Disposition: form-data; name=\"chapter_id\"\r\n\r\nsome-chapter-id\r\n" +
		"--boundary--"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/learning/import",
		strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, "multipart/form-data; boundary=boundary")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewLearningHandler(nil)
	if err := h.ImportContent(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing category, got %d", rec.Code)
	}
}

func TestImportContent_MissingTitle_Returns400(t *testing.T) {
	e := echo.New()
	body := "--boundary\r\nContent-Disposition: form-data; name=\"category\"\r\n\r\nPharmacology\r\n" +
		"--boundary\r\nContent-Disposition: form-data; name=\"chapter_id\"\r\n\r\nsome-chapter-id\r\n" +
		"--boundary--"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/learning/import",
		strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, "multipart/form-data; boundary=boundary")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewLearningHandler(nil)
	if err := h.ImportContent(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing title, got %d", rec.Code)
	}
}

func TestImportContent_MissingChapterID_Returns400(t *testing.T) {
	e := echo.New()
	// category + title present but no chapter_id
	body := "--boundary\r\nContent-Disposition: form-data; name=\"category\"\r\n\r\nPharmacology\r\n" +
		"--boundary\r\nContent-Disposition: form-data; name=\"title\"\r\n\r\nTest Title\r\n" +
		"--boundary--"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/learning/import",
		strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, "multipart/form-data; boundary=boundary")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewLearningHandler(nil)
	if err := h.ImportContent(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing chapter_id, got %d", rec.Code)
	}

	var body2 map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body2); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if body2["error"] == nil {
		t.Error("expected error field in response body")
	}
}

func TestImportContent_MissingFile_Returns400(t *testing.T) {
	e := echo.New()
	body := "--boundary\r\nContent-Disposition: form-data; name=\"category\"\r\n\r\nPharmacology\r\n" +
		"--boundary\r\nContent-Disposition: form-data; name=\"title\"\r\n\r\nTest Title\r\n" +
		"--boundary\r\nContent-Disposition: form-data; name=\"chapter_id\"\r\n\r\nsome-chapter-id\r\n" +
		"--boundary--"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/learning/import",
		strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, "multipart/form-data; boundary=boundary")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewLearningHandler(nil)
	if err := h.ImportContent(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing file, got %d", rec.Code)
	}
}
