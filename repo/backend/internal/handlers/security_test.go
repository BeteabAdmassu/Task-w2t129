package handlers

// security_test.go — tests for object-level authorization, status-machine
// constraints, and inventory adjustment type correctness.
//
// All tests exercise pure functions extracted from the handlers so they run
// without a database, while still covering the critical decision logic that
// was flagged by the audit (F-007, F-004, F-005).

import (
	"testing"
)

// ─── Work-order object-level authorization (F-007) ────────────────────────────

func TestCanViewWorkOrder_SubmitterCanView(t *testing.T) {
	assignee := "other-user"
	if !canViewWorkOrder("user-1", "front_desk", "user-1", &assignee) {
		t.Error("submitter should be able to view their own work order")
	}
}

// H-04 fix: assignee-based view access is ONLY granted to maintenance_tech
// role. Other roles (e.g. front_desk) still require being the submitter —
// being assigned as the point of contact does not grant view rights.
func TestCanViewWorkOrder_FrontDeskAssigneeWithoutSubmit_IsDenied(t *testing.T) {
	assignee := "frontdesk-user"
	if canViewWorkOrder("frontdesk-user", "front_desk", "submitter-user", &assignee) {
		t.Error("front_desk assignee (not submitter) must NOT be able to view (H-04)")
	}
}

func TestCanViewWorkOrder_SystemAdminCanView(t *testing.T) {
	if !canViewWorkOrder("admin-user", "system_admin", "someone-else", nil) {
		t.Error("system_admin should be able to view any work order")
	}
}

// H-04 fix: maintenance_tech is now assignment-bound (consistent with list scope).
func TestCanViewWorkOrder_MaintenanceTech_AssignedToSelf_CanView(t *testing.T) {
	assignee := "tech-user"
	if !canViewWorkOrder("tech-user", "maintenance_tech", "someone-else", &assignee) {
		t.Error("maintenance_tech assigned to the WO should be able to view it")
	}
}

func TestCanViewWorkOrder_MaintenanceTech_NotAssigned_IsDenied(t *testing.T) {
	if canViewWorkOrder("tech-user", "maintenance_tech", "someone-else", nil) {
		t.Error("maintenance_tech not assigned to the WO must be denied (H-04 fix)")
	}
}

func TestCanViewWorkOrder_MaintenanceTech_DifferentAssignee_IsDenied(t *testing.T) {
	otherTech := "other-tech"
	if canViewWorkOrder("tech-user", "maintenance_tech", "someone-else", &otherTech) {
		t.Error("maintenance_tech must be denied when a different tech is assigned (H-04 fix)")
	}
}

func TestCanViewWorkOrder_UnrelatedUserIsDenied(t *testing.T) {
	assignee := "tech-user"
	if canViewWorkOrder("stranger", "front_desk", "submitter-user", &assignee) {
		t.Error("unrelated user should NOT be able to view the work order")
	}
}

func TestCanViewWorkOrder_UnrelatedUser_NilAssignee_IsDenied(t *testing.T) {
	if canViewWorkOrder("stranger", "inventory_pharmacist", "submitter-user", nil) {
		t.Error("unrelated user (nil assignee) should NOT be able to view the work order")
	}
}

func TestCanViewWorkOrder_LearningCoordinatorIsDenied(t *testing.T) {
	// learning_coordinator has no special work-order access
	if canViewWorkOrder("learn-user", "learning_coordinator", "other-user", nil) {
		t.Error("learning_coordinator should not be able to view arbitrary work orders")
	}
}

func TestCanViewWorkOrder_AssigneeNilSlot_OtherUser_IsDenied(t *testing.T) {
	// assignedTo is nil — the calling user is neither submitter nor admin
	if canViewWorkOrder("user-x", "front_desk", "user-y", nil) {
		t.Error("non-submitter with nil assignedTo should be denied")
	}
}

// ─── File download object-level authorization (F-007) ─────────────────────────

func TestCanDownloadFile_UploaderCanDownload(t *testing.T) {
	uid := "user-abc"
	if !canDownloadFile("user-abc", "front_desk", &uid) {
		t.Error("uploader should be able to download their own file")
	}
}

func TestCanDownloadFile_SystemAdminCanDownload(t *testing.T) {
	uid := "other-user"
	if !canDownloadFile("admin-user", "system_admin", &uid) {
		t.Error("system_admin should be able to download any file")
	}
}

func TestCanDownloadFile_InventoryPharmacistCanDownload(t *testing.T) {
	uid := "other-user"
	if !canDownloadFile("pharm-user", "inventory_pharmacist", &uid) {
		t.Error("inventory_pharmacist should be able to download any file")
	}
}

func TestCanDownloadFile_StrangerIsDenied(t *testing.T) {
	uid := "real-uploader"
	if canDownloadFile("stranger", "front_desk", &uid) {
		t.Error("unrelated user should NOT be able to download the file")
	}
}

func TestCanDownloadFile_NilUploadedBy_StrangerIsDenied(t *testing.T) {
	// uploadedBy is nil (legacy record) — non-privileged user must be denied
	if canDownloadFile("stranger", "front_desk", nil) {
		t.Error("non-privileged user should be denied when uploadedBy is nil")
	}
}

func TestCanDownloadFile_NilUploadedBy_AdminAllowed(t *testing.T) {
	if !canDownloadFile("admin", "system_admin", nil) {
		t.Error("system_admin should still download even when uploadedBy is nil")
	}
}

// ─── Statement status-machine ─────────────────────────────────────────────────
// Canonical states: pending → reconciled → approved → paid (no direct pending→paid)

func TestStatementIsReconcilable_PendingIsReconcilable(t *testing.T) {
	if !statementIsReconcilable("pending") {
		t.Error("'pending' statement should be reconcilable")
	}
}

func TestStatementIsReconcilable_ReconciledIsNot(t *testing.T) {
	if statementIsReconcilable("reconciled") {
		t.Error("'reconciled' statement should NOT be reconcilable again")
	}
}

func TestStatementIsReconcilable_ApprovedIsNot(t *testing.T) {
	if statementIsReconcilable("approved") {
		t.Error("'approved' statement should NOT be reconcilable")
	}
}

func TestStatementIsReconcilable_PaidIsNot(t *testing.T) {
	if statementIsReconcilable("paid") {
		t.Error("'paid' statement should NOT be reconcilable")
	}
}

func TestStatementIsApprovable_ReconciledIsApprovable(t *testing.T) {
	if !statementIsApprovable("reconciled") {
		t.Error("'reconciled' statement should be approvable")
	}
}

func TestStatementIsApprovable_PendingIsNot(t *testing.T) {
	if statementIsApprovable("pending") {
		t.Error("'pending' statement should NOT be approvable (must be reconciled first)")
	}
}

func TestStatementIsApprovable_ApprovedIsNot(t *testing.T) {
	if statementIsApprovable("approved") {
		t.Error("'approved' statement should NOT be approvable again")
	}
}

func TestStatementIsApprovable_PaidIsNot(t *testing.T) {
	if statementIsApprovable("paid") {
		t.Error("'paid' statement should NOT be approvable")
	}
}

// TestApproverAllowed_* tests the approverAllowed helper that guards the two-step
// approval distinctness rule in ApproveStatement. The rule: the person who reconciled
// (approved_by_1) cannot be the same person who performs the final approval.

func TestApproverAllowed_SameUserForbidden(t *testing.T) {
	id := "user-alice"
	if approverAllowed(&id, "user-alice") {
		t.Error("same user as reconciler must not be allowed to approve (approverAllowed must return false)")
	}
}

func TestApproverAllowed_DifferentUserPermitted(t *testing.T) {
	id := "user-alice"
	if !approverAllowed(&id, "user-bob") {
		t.Error("different user from reconciler must be allowed to approve")
	}
}

func TestApproverAllowed_NotYetReconciled(t *testing.T) {
	// When approved_by_1 is nil the statement has not been reconciled yet;
	// any user may perform the first-step approval.
	if !approverAllowed(nil, "user-alice") {
		t.Error("unreconciled statement (approved_by_1 == nil) must be approvable by any user")
	}
}

// ─── Variance threshold (reconciliation rule) ─────────────────────────────────

func TestVarianceThreshold_ExactlyAtBoundary_NotExceeded(t *testing.T) {
	// ABS(100 - 75) = 25 — equals threshold, must NOT require notes
	if statementVarianceExceedsThreshold(100, 75) {
		t.Error("variance of exactly 25 should NOT exceed the threshold")
	}
}

func TestVarianceThreshold_OneAboveBoundary_Exceeded(t *testing.T) {
	// ABS(100 - 74.99) = 25.01 — exceeds threshold
	if !statementVarianceExceedsThreshold(100, 74.99) {
		t.Error("variance of 25.01 should exceed the threshold")
	}
}

func TestVarianceThreshold_NegativeDelta_UsesAbsoluteValue(t *testing.T) {
	// ABS(74.99 - 100) = 25.01 — direction does not matter
	if !statementVarianceExceedsThreshold(74.99, 100) {
		t.Error("negative variance of 25.01 should exceed the threshold (ABS required)")
	}
}

func TestVarianceThreshold_TotalExceeds25_ButCloseToExpected_NotFlagged(t *testing.T) {
	// Old buggy check: total > 25 would flag this incorrectly.
	// New correct check: ABS(1000 - 999) = 1, which is fine.
	if statementVarianceExceedsThreshold(1000, 999) {
		t.Error("total > 25 alone should NOT trigger threshold; only ABS(diff) > 25 should")
	}
}

func TestApprovalStep_BothNil_ReturnsStep1(t *testing.T) {
	if got := approvalStep(nil, nil); got != 1 {
		t.Errorf("both approvers nil → step 1, got %d", got)
	}
}

func TestApprovalStep_FirstSet_ReturnsStep2(t *testing.T) {
	uid := "user-1"
	if got := approvalStep(&uid, nil); got != 2 {
		t.Errorf("first approver set, second nil → step 2, got %d", got)
	}
}

func TestApprovalStep_BothSet_ReturnsZero(t *testing.T) {
	u1, u2 := "user-1", "user-2"
	if got := approvalStep(&u1, &u2); got != 0 {
		t.Errorf("both approvers set → 0 (fully approved), got %d", got)
	}
}

// ─── Inventory adjustment transaction type (F-005) ────────────────────────────

func TestAdjustTxType_PositiveQuantity_IsIn(t *testing.T) {
	if got := adjustTxType(10); got != "in" {
		t.Errorf("positive quantity → 'in', got %q", got)
	}
}

func TestAdjustTxType_NegativeQuantity_IsOut(t *testing.T) {
	if got := adjustTxType(-5); got != "out" {
		t.Errorf("negative quantity → 'out', got %q", got)
	}
}

func TestAdjustTxType_ZeroQuantity_IsIn(t *testing.T) {
	// Zero adjustment is a no-op in/out — schema requires 'in' or 'out', we default to 'in'
	if got := adjustTxType(0); got != "in" {
		t.Errorf("zero quantity → 'in', got %q", got)
	}
}

func TestAdjustTxType_NeverReturnsAdjustmentIn(t *testing.T) {
	// Regression: old code used "adjustment_in" which violates DB CHECK constraint
	for _, qty := range []int{1, 5, 100, 0} {
		if got := adjustTxType(qty); got == "adjustment_in" {
			t.Errorf("adjustTxType(%d) returned 'adjustment_in' — violates DB schema", qty)
		}
	}
}

func TestAdjustTxType_NeverReturnsAdjustmentOut(t *testing.T) {
	// Regression: old code used "adjustment_out" which violates DB CHECK constraint
	for _, qty := range []int{-1, -5, -100} {
		if got := adjustTxType(qty); got == "adjustment_out" {
			t.Errorf("adjustTxType(%d) returned 'adjustment_out' — violates DB schema", qty)
		}
	}
}

func TestAdjustTxType_OnlyReturnsInOrOut(t *testing.T) {
	// Exhaustive: all returned values must be 'in' or 'out'
	valid := map[string]bool{"in": true, "out": true}
	for _, qty := range []int{-1000, -1, 0, 1, 1000} {
		got := adjustTxType(qty)
		if !valid[got] {
			t.Errorf("adjustTxType(%d) = %q — only 'in' or 'out' are schema-valid", qty, got)
		}
	}
}

// ─── F-003: canDownloadFile secondary authorization (work-order linked) ───────
//
// F-003 added a secondary authorization path so that maintenance_tech (and
// other non-privileged users) who are linked to a work order can download
// photos attached to that work order — even when canDownloadFile returns false.
//
// These tests verify the PRIMARY predicate behaviour. The secondary check
// (IsFileLinkedToUserWorkOrder) is a repository concern tested separately in
// repository/tenant_test.go because it requires SQL query assertion.

// TestCanDownloadFile_MaintenanceTech_NotUploader_Denied verifies that the
// primary predicate alone denies a maintenance_tech who didn't upload the file.
// The secondary WO-link check is what would ultimately grant them access —
// this test documents that the primary check is deliberately strict.
func TestCanDownloadFile_MaintenanceTech_NotUploader_Denied(t *testing.T) {
	uploader := "uid-uploader"
	// maintenance_tech calling with a DIFFERENT userID than the uploader
	if canDownloadFile("uid-tech", "maintenance_tech", &uploader) {
		t.Error("maintenance_tech who is NOT the uploader must NOT pass the primary canDownloadFile check; " +
			"access should come via the secondary work-order linkage check in the handler")
	}
}

// TestCanDownloadFile_MaintenanceTech_IsUploader_Allowed verifies that a
// maintenance_tech who uploaded the file themselves does pass the primary check.
func TestCanDownloadFile_MaintenanceTech_IsUploader_Allowed(t *testing.T) {
	techID := "uid-tech"
	if !canDownloadFile("uid-tech", "maintenance_tech", &techID) {
		t.Error("maintenance_tech who uploaded the file should pass the primary canDownloadFile check")
	}
}

// TestCanDownloadFile_FrontDesk_NotUploader_Denied verifies that a front_desk
// user who is not the uploader is denied by the primary check.
func TestCanDownloadFile_FrontDesk_NotUploader_Denied(t *testing.T) {
	uploader := "uid-other"
	if canDownloadFile("uid-fd", "front_desk", &uploader) {
		t.Error("front_desk who is not the uploader should be denied")
	}
}

// ─── Tenant isolation tests ───────────────────────────────────────────────────
// Real tenant isolation tests (query-capturing driver) live in:
//   repo/backend/internal/repository/tenant_test.go
//
// Those tests build a Repository backed by a lightweight capturing sql.Driver
// and assert that the SQL sent to the DB contains "tenant_id" and the correct
// number of parameter placeholders — tests that would genuinely FAIL if the
// tenant predicate were removed from the query.
//
// The string-literal stubs that were here previously have been removed because
// they tested hardcoded strings, not the actual repository code.
