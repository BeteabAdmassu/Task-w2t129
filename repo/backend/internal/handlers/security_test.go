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

func TestCanViewWorkOrder_AssigneeCanView(t *testing.T) {
	assignee := "tech-user"
	if !canViewWorkOrder("tech-user", "front_desk", "submitter-user", &assignee) {
		t.Error("assigned technician should be able to view the work order")
	}
}

func TestCanViewWorkOrder_SystemAdminCanView(t *testing.T) {
	if !canViewWorkOrder("admin-user", "system_admin", "someone-else", nil) {
		t.Error("system_admin should be able to view any work order")
	}
}

func TestCanViewWorkOrder_MaintenanceTechCanView(t *testing.T) {
	if !canViewWorkOrder("tech-user", "maintenance_tech", "someone-else", nil) {
		t.Error("maintenance_tech should be able to view any work order")
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

// ─── Statement status-machine (F-004) ─────────────────────────────────────────

func TestStatementIsReconcilable_DraftIsReconcilable(t *testing.T) {
	if !statementIsReconcilable("draft") {
		t.Error("'draft' statement should be reconcilable")
	}
}

func TestStatementIsReconcilable_PendingApprovalIsNot(t *testing.T) {
	if statementIsReconcilable("pending_approval") {
		t.Error("'pending_approval' statement should NOT be reconcilable")
	}
}

func TestStatementIsReconcilable_ApprovedIsNot(t *testing.T) {
	if statementIsReconcilable("approved") {
		t.Error("'approved' statement should NOT be reconcilable")
	}
}

func TestStatementIsReconcilable_ExportedIsNot(t *testing.T) {
	if statementIsReconcilable("exported") {
		t.Error("'exported' statement should NOT be reconcilable")
	}
}

func TestStatementIsApprovable_PendingApprovalIsApprovable(t *testing.T) {
	if !statementIsApprovable("pending_approval") {
		t.Error("'pending_approval' statement should be approvable")
	}
}

func TestStatementIsApprovable_DraftIsNot(t *testing.T) {
	if statementIsApprovable("draft") {
		t.Error("'draft' statement should NOT be directly approvable (must reconcile first)")
	}
}

func TestStatementIsApprovable_ApprovedIsNot(t *testing.T) {
	if statementIsApprovable("approved") {
		t.Error("'approved' statement should NOT be approvable again")
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
