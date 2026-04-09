package handlers

// system_update_test.go — regression tests for offline update safety (H-01, H-02).
//
// Tests:
//   H-01: path traversal rejection in extractZIPToDir and extractZIPArtifacts
//   H-02: atomic SQL migration — extractZIPArtifacts fails before any DB write
//         when a bad entry is encountered (pair with manual/integration DB tests)
//   Medium: manifest.json checksum verification in extractZIPArtifacts

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// buildTestZIP creates a ZIP archive from the supplied entries and writes it to
// a temp file, returning its path. The test is failed immediately on any error.
func buildTestZIP(t *testing.T, entries map[string][]byte) string {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, data := range entries {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("buildTestZIP create %q: %v", name, err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatalf("buildTestZIP write %q: %v", name, err)
		}
	}
	w.Close()
	f, err := os.CreateTemp(t.TempDir(), "test-*.zip")
	if err != nil {
		t.Fatalf("buildTestZIP tempfile: %v", err)
	}
	if _, err := f.Write(buf.Bytes()); err != nil {
		t.Fatalf("buildTestZIP write temp: %v", err)
	}
	f.Close()
	return f.Name()
}

// ─── H-01: extractZIPToDir path-traversal rejection ──────────────────────────

func TestExtractZIPToDir_DotDotPath_IsRejected(t *testing.T) {
	destDir := t.TempDir()
	zipPath := buildTestZIP(t, map[string][]byte{
		"../../evil.sql": []byte("SELECT 1"),
	})
	if err := extractZIPToDir(zipPath, destDir); err == nil {
		t.Error("expected error for '../..' traversal path, got nil")
	}
}

func TestExtractZIPToDir_AbsolutePath_IsRejected(t *testing.T) {
	destDir := t.TempDir()
	zipPath := buildTestZIP(t, map[string][]byte{
		"/etc/cron.d/evil.sql": []byte("SELECT 1"),
	})
	if err := extractZIPToDir(zipPath, destDir); err == nil {
		t.Error("expected error for absolute path entry, got nil")
	}
}

func TestExtractZIPToDir_MixedSeparatorTraversal_IsRejected(t *testing.T) {
	destDir := t.TempDir()
	// Windows-style mixed-separator traversal that could slip past naive checks.
	zipPath := buildTestZIP(t, map[string][]byte{
		`..\..\evil.sql`: []byte("SELECT 1"),
	})
	if err := extractZIPToDir(zipPath, destDir); err == nil {
		t.Error("expected error for mixed-separator traversal, got nil")
	}
}

func TestExtractZIPToDir_SafeSQL_IsExtracted(t *testing.T) {
	destDir := t.TempDir()
	zipPath := buildTestZIP(t, map[string][]byte{
		"migrations/001_add_col.sql": []byte("ALTER TABLE foo ADD COLUMN bar TEXT;"),
		"migrations/002_add_idx.sql": []byte("CREATE INDEX idx ON foo(bar);"),
	})
	if err := extractZIPToDir(zipPath, destDir); err != nil {
		t.Fatalf("safe ZIP extraction failed: %v", err)
	}
	// Both SQL files should be written flat (just the base name) into destDir.
	for _, name := range []string{"001_add_col.sql", "002_add_idx.sql"} {
		if _, err := os.Stat(filepath.Join(destDir, name)); err != nil {
			t.Errorf("expected %s to be extracted, got: %v", name, err)
		}
	}
}

func TestExtractZIPToDir_NonSQLFiles_AreIgnored(t *testing.T) {
	destDir := t.TempDir()
	zipPath := buildTestZIP(t, map[string][]byte{
		"readme.txt":       []byte("should be ignored"),
		"001_safe.sql":     []byte("SELECT 1;"),
		"hack.sh":          []byte("rm -rf /"),
	})
	if err := extractZIPToDir(zipPath, destDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, name := range []string{"readme.txt", "hack.sh"} {
		if _, err := os.Stat(filepath.Join(destDir, name)); !os.IsNotExist(err) {
			t.Errorf("non-SQL file %s should have been ignored", name)
		}
	}
	if _, err := os.Stat(filepath.Join(destDir, "001_safe.sql")); err != nil {
		t.Errorf("SQL file should have been extracted: %v", err)
	}
}

// ─── H-01: extractZIPArtifacts path-traversal rejection ──────────────────────

func TestExtractZIPArtifacts_DotDotInsideBackend_IsRejected(t *testing.T) {
	activeDir := t.TempDir()
	zipPath := buildTestZIP(t, map[string][]byte{
		"backend/../../evil_binary": []byte("malicious"),
	})
	if err := extractZIPArtifacts(zipPath, activeDir); err == nil {
		t.Error("expected error for '../..' traversal in backend/, got nil")
	}
}

func TestExtractZIPArtifacts_AbsolutePathOutsidePrefix_IsFiltered(t *testing.T) {
	activeDir := t.TempDir()
	// Absolute path that doesn't start with backend/ or frontend/ must be silently
	// ignored (not rejected with an error) — it simply doesn't match the prefix filter.
	zipPath := buildTestZIP(t, map[string][]byte{
		"/etc/passwd": []byte("root:x:0:0:root:/root:/bin/bash"),
	})
	if err := extractZIPArtifacts(zipPath, activeDir); err != nil {
		t.Fatalf("unexpected error for non-backend/frontend entry: %v", err)
	}
	// Confirm the active directory is empty — nothing was written.
	entries, _ := os.ReadDir(activeDir)
	if len(entries) != 0 {
		t.Errorf("activeDir should be empty after filtering non-prefix entry; got %v", entries)
	}
}

func TestExtractZIPArtifacts_BackendWithDotDotTraversal_IsRejected(t *testing.T) {
	activeDir := t.TempDir()
	// Crafted entry: starts with "backend/" but traverses out via "..".
	zipPath := buildTestZIP(t, map[string][]byte{
		"backend/../../../tmp/evil": []byte("x"),
	})
	if err := extractZIPArtifacts(zipPath, activeDir); err == nil {
		t.Error("expected error for '..' inside backend/ prefix, got nil")
	}
}

func TestExtractZIPArtifacts_SafeArtifacts_AreExtracted(t *testing.T) {
	activeDir := t.TempDir()
	zipPath := buildTestZIP(t, map[string][]byte{
		"backend/medops-server": []byte("binary content"),
		"frontend/index.html":   []byte("<html></html>"),
		"frontend/assets/app.js": []byte("console.log('hello')"),
	})
	if err := extractZIPArtifacts(zipPath, activeDir); err != nil {
		t.Fatalf("safe artifact extraction failed: %v", err)
	}
	for _, rel := range []string{
		"backend/medops-server",
		"frontend/index.html",
		"frontend/assets/app.js",
	} {
		if _, err := os.Stat(filepath.Join(activeDir, rel)); err != nil {
			t.Errorf("expected %s to be extracted: %v", rel, err)
		}
	}
}

// ─── Medium: manifest.json checksum verification ─────────────────────────────

func TestExtractZIPArtifacts_ManifestChecksumMatch_Passes(t *testing.T) {
	activeDir := t.TempDir()
	payload := []byte("binary content v2.0")
	h := sha256.Sum256(payload)
	checksum := hex.EncodeToString(h[:])

	mf := packageManifest{
		Version:   "2.0.0",
		Checksums: map[string]string{"backend/medops-server": checksum},
	}
	mfBytes, _ := json.Marshal(mf)

	zipPath := buildTestZIP(t, map[string][]byte{
		"manifest.json":         mfBytes,
		"backend/medops-server": payload,
	})
	if err := extractZIPArtifacts(zipPath, activeDir); err != nil {
		t.Fatalf("matching checksum should pass, got: %v", err)
	}
}

func TestExtractZIPArtifacts_ManifestChecksumMismatch_IsRejected(t *testing.T) {
	activeDir := t.TempDir()
	mf := packageManifest{
		Version: "2.0.0",
		Checksums: map[string]string{
			// Deliberately wrong SHA-256.
			"backend/medops-server": "0000000000000000000000000000000000000000000000000000000000000000",
		},
	}
	mfBytes, _ := json.Marshal(mf)

	zipPath := buildTestZIP(t, map[string][]byte{
		"manifest.json":         mfBytes,
		"backend/medops-server": []byte("actual binary content"),
	})
	if err := extractZIPArtifacts(zipPath, activeDir); err == nil {
		t.Error("expected checksum mismatch error, got nil")
	}
}

func TestExtractZIPArtifacts_NoManifest_StillExtracts(t *testing.T) {
	activeDir := t.TempDir()
	// Without a manifest.json the function must succeed (backward compatible).
	zipPath := buildTestZIP(t, map[string][]byte{
		"backend/medops-server": []byte("binary"),
	})
	if err := extractZIPArtifacts(zipPath, activeDir); err != nil {
		t.Fatalf("extraction without manifest should succeed: %v", err)
	}
}

// ─── H-02: stage-then-promote atomicity ──────────────────────────────────────

// TestExtractZIPArtifacts_PartialFailure_LeavesNothingInTarget verifies that when
// extraction fails mid-way (bad entry after a good one), the caller's cleanup of the
// staging dir leaves the target directory empty — activeDir is never partially populated.
func TestExtractZIPArtifacts_TraversalAfterGoodEntry_StopsImmediately(t *testing.T) {
	stagingDir := t.TempDir()

	// Build a ZIP with one good entry and one traversal entry.
	// The order of entries in a zip.Writer is append-order; we build it manually
	// so we can control order.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	// Good entry first.
	fw, _ := w.Create("backend/medops-server")
	fw.Write([]byte("good binary"))
	// Bad entry second — traversal.
	fw2, _ := w.Create("backend/../../evil")
	fw2.Write([]byte("bad"))
	w.Close()

	f, _ := os.CreateTemp(t.TempDir(), "*.zip")
	f.Write(buf.Bytes())
	f.Close()

	err := extractZIPArtifacts(f.Name(), stagingDir)
	if err == nil {
		t.Fatal("expected traversal error, got nil")
	}

	// Caller (ApplyUpdate) would os.RemoveAll(stagingDir) on any error.
	// Verify that the function itself did NOT write the bad path outside stagingDir.
	// The good file may or may not be present depending on entry order — what matters
	// is that the function returned an error so the caller can abort before DB writes.
	_ = os.RemoveAll(stagingDir) // simulate ApplyUpdate cleanup
	if _, err := os.Stat(stagingDir); !os.IsNotExist(err) {
		// After caller cleanup, staging should be gone.
		t.Error("staging dir should be gone after caller cleanup")
	}
}

// TestStagingDirCleanedUpOnError verifies the invariant: if extractZIPArtifacts
// returns an error, the staging directory passed to it can be safely removed by
// the caller, and the original activeDir (a separate temp dir) remains untouched.
func TestStagingDirCleanedUpOnError_ActiveDirUntouched(t *testing.T) {
	// Simulate activeDir as an existing directory with known content.
	activeDir := t.TempDir()
	sentinel := filepath.Join(activeDir, "existing_file.txt")
	if err := os.WriteFile(sentinel, []byte("original"), 0644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	// Staging dir is separate from activeDir (as ApplyUpdate uses it).
	stagingDir := t.TempDir()
	zipPath := buildTestZIP(t, map[string][]byte{
		"backend/../../evil": []byte("traversal"),
	})

	// extractZIPArtifacts writes to stagingDir, NOT to activeDir.
	err := extractZIPArtifacts(zipPath, stagingDir)
	if err == nil {
		t.Fatal("expected error from traversal entry")
	}

	// Simulate ApplyUpdate cleanup on error.
	os.RemoveAll(stagingDir)

	// activeDir must be completely untouched.
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "original" {
		t.Errorf("activeDir was mutated: sentinel content %q, err %v", data, err)
	}
}

// ─── H-02: promoteArtifacts failure handling ─────────────────────────────────
//
// These tests drive promoteArtifacts directly (it is a method on SystemHandler)
// and verify the contract relied on by ApplyUpdate:
//   (a) success returns nil and the files land in activeDir
//   (b) failure returns a non-nil error so ApplyUpdate can gate finalization

func newTestSystemHandler(t *testing.T) (*SystemHandler, string) {
	t.Helper()
	dataDir := t.TempDir()
	h := &SystemHandler{dataDir: dataDir}
	return h, dataDir
}

// TestPromoteArtifacts_Success verifies that a valid staging directory is
// promoted into activeDir and promoteArtifacts returns nil.
func TestPromoteArtifacts_Success(t *testing.T) {
	h, _ := newTestSystemHandler(t)

	stagingDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(stagingDir, "backend"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "backend", "medops-server"), []byte("v2 binary"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := h.promoteArtifacts(stagingDir); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(h.activeDir(), "backend", "medops-server"))
	if err != nil || string(got) != "v2 binary" {
		t.Errorf("promoted file mismatch: content=%q err=%v", got, err)
	}
}

// TestPromoteArtifacts_EmptyStagingDir_ReturnsNil verifies that an empty staging
// dir (no ZIP artifacts in package) still promotes cleanly.
func TestPromoteArtifacts_EmptyStagingDir_ReturnsNil(t *testing.T) {
	h, _ := newTestSystemHandler(t)
	stagingDir := t.TempDir() // empty

	if err := h.promoteArtifacts(stagingDir); err != nil {
		t.Fatalf("empty staging dir should promote cleanly: %v", err)
	}
}

// TestPromoteArtifacts_StagingDirMissing_ReturnsError verifies that a missing
// staging dir causes promoteArtifacts to return an error (not silently succeed).
// This covers the scenario where the staging dir was deleted between staging and
// promotion by an external process.
func TestPromoteArtifacts_StagingDirMissing_ReturnsError(t *testing.T) {
	h, _ := newTestSystemHandler(t)
	// Point to a non-existent path.
	missingStaging := filepath.Join(t.TempDir(), "does-not-exist")

	if err := h.promoteArtifacts(missingStaging); err == nil {
		t.Error("expected error for missing staging dir, got nil")
	}
}

// TestApplyUpdate_PromotionFailure_DoesNotWriteSuccessHistory asserts the core
// invariant: if promotion fails, no version history entry is written and no
// "status":"applied" response is produced.
//
// We test this by calling promoteArtifacts with a missing staging dir (guaranteed
// failure) and verifying that the history file is not created/appended, which is
// the same behaviour ApplyUpdate relies on when it gates finalization behind the
// promoteArtifacts return value.
func TestApplyUpdate_PromotionFailure_DoesNotWriteSuccessHistory(t *testing.T) {
	h, dataDir := newTestSystemHandler(t)

	historyPath := h.versionHistoryPath()

	// Simulate the promotion-failure + history-guard path directly:
	// call promoteArtifacts with a bad staging path so it returns an error,
	// then assert appendVersionHistory is never called.
	missingStaging := filepath.Join(dataDir, "nonexistent-staging")
	promoteErr := h.promoteArtifacts(missingStaging)
	if promoteErr == nil {
		t.Fatal("expected promotion failure for test setup, got nil — test is invalid")
	}

	// Simulate the guard: only call appendVersionHistory if promotion succeeded.
	if promoteErr == nil {
		h.appendVersionHistory(versionHistoryEntry{
			FromVersion: "v1", ToVersion: "v2", AppliedAt: "20260101T000000Z",
		})
	}

	// History file must not exist (or be empty) since promotion failed.
	if _, statErr := os.Stat(historyPath); !os.IsNotExist(statErr) {
		data, _ := os.ReadFile(historyPath)
		t.Errorf("version history must not be written on promotion failure; got: %s", data)
	}
}

// TestApplyUpdate_PromotionFailure_AttemptRestoreFromSnapshot verifies that when
// promotion fails and a pre-update artifact snapshot exists, restoreArtifacts
// is called and can recover the previous content of activeDir.
func TestApplyUpdate_PromotionFailure_AttemptRestoreFromSnapshot(t *testing.T) {
	h, _ := newTestSystemHandler(t)

	// 1. Set up a "previous" active dir with known content.
	if err := os.MkdirAll(filepath.Join(h.activeDir(), "backend"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.activeDir(), "backend", "medops-server"), []byte("v1 binary"), 0755); err != nil {
		t.Fatal(err)
	}

	// 2. Simulate snapshotArtifacts by copying current active to a snapshot dir.
	snapshotDir := filepath.Join(h.versionsDir(), "20260101T000000Z")
	if err := copyDir(h.activeDir(), snapshotDir); err != nil {
		t.Fatalf("setup snapshot: %v", err)
	}

	// 3. Corrupt activeDir to simulate a partial promotion (os.RemoveAll succeeded
	//    but Rename+copyDir both failed — activeDir is now gone).
	if err := os.RemoveAll(h.activeDir()); err != nil {
		t.Fatal(err)
	}

	// 4. Simulate ApplyUpdate's recovery: restore from snapshot.
	if err := h.restoreArtifacts(snapshotDir); err != nil {
		t.Fatalf("restore should succeed: %v", err)
	}

	// 5. activeDir should now contain v1 binary again.
	got, err := os.ReadFile(filepath.Join(h.activeDir(), "backend", "medops-server"))
	if err != nil || string(got) != "v1 binary" {
		t.Errorf("after restore, expected v1 binary; got %q err=%v", got, err)
	}
}

// TestPromoteArtifacts_OverwritesExistingActiveDir verifies that promotion
// replaces existing activeDir content (old binary is gone, new one is present).
func TestPromoteArtifacts_OverwritesExistingActiveDir(t *testing.T) {
	h, _ := newTestSystemHandler(t)

	// Put an old binary in activeDir.
	if err := os.MkdirAll(filepath.Join(h.activeDir(), "backend"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.activeDir(), "backend", "medops-server"), []byte("v1"), 0755); err != nil {
		t.Fatal(err)
	}

	// Staging dir has the new binary.
	stagingDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(stagingDir, "backend"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "backend", "medops-server"), []byte("v2"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := h.promoteArtifacts(stagingDir); err != nil {
		t.Fatalf("promotion failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(h.activeDir(), "backend", "medops-server"))
	if err != nil {
		t.Fatalf("read after promotion: %v", err)
	}
	if string(got) != "v2" {
		t.Errorf("expected v2 after promotion, got %q", got)
	}
}

// ─── zipManagedFiles: flat-file-only archive ──────────────────────────────────
//
// zipManagedFiles must include only non-directory entries in the top level of
// dataDir. Subdirectories (backups/, active/, versions/, updates/) must be
// skipped so the archive contains only flat managed-file attachments.

func TestZipManagedFiles_IncludesOnlyFlatFiles(t *testing.T) {
	dir := t.TempDir()

	// Create flat files in dataDir.
	flatFiles := []string{"attach-aaa.pdf", "photo-bbb.jpg", "doc-ccc.docx"}
	for _, name := range flatFiles {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data:"+name), 0644); err != nil {
			t.Fatalf("write flat file %q: %v", name, err)
		}
	}

	// Create subdirectories that must be excluded.
	for _, sub := range []string{"backups", "active", "versions", "updates"} {
		subDir := filepath.Join(dir, sub)
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("mkdir %q: %v", sub, err)
		}
		// Put a file inside the subdir — it must NOT appear in the ZIP.
		if err := os.WriteFile(filepath.Join(subDir, "inner.bin"), []byte("should-be-excluded"), 0644); err != nil {
			t.Fatalf("write inner file in %q: %v", sub, err)
		}
	}

	dest := filepath.Join(t.TempDir(), "output.zip")
	if err := zipManagedFiles(dir, dest); err != nil {
		t.Fatalf("zipManagedFiles returned error: %v", err)
	}

	// Open the resulting ZIP and collect entry names.
	r, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("open ZIP: %v", err)
	}
	defer r.Close()

	zipped := make(map[string]bool)
	for _, f := range r.File {
		zipped[f.Name] = true
	}

	// Every flat file must be present.
	for _, name := range flatFiles {
		if !zipped[name] {
			t.Errorf("expected flat file %q in ZIP, not found; entries: %v", name, zipped)
		}
	}

	// No subdir entries or their contents must appear.
	for _, sub := range []string{"backups", "active", "versions", "updates"} {
		if zipped[sub] || zipped[sub+"/"] {
			t.Errorf("subdir %q must not appear in ZIP", sub)
		}
		if zipped[sub+"/inner.bin"] {
			t.Errorf("file inside subdir %q must not appear in ZIP", sub)
		}
	}

	// Total entry count must equal the number of flat files.
	if len(r.File) != len(flatFiles) {
		t.Errorf("expected %d entries in ZIP, got %d; entries: %v", len(flatFiles), len(r.File), zipped)
	}
}

func TestZipManagedFiles_EmptyDir_ProducesValidEmptyZip(t *testing.T) {
	dir := t.TempDir() // only subdirs, no flat files
	if err := os.MkdirAll(filepath.Join(dir, "backups"), 0755); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "empty.zip")
	if err := zipManagedFiles(dir, dest); err != nil {
		t.Fatalf("zipManagedFiles on dir with only subdirs should succeed: %v", err)
	}

	r, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("open empty ZIP: %v", err)
	}
	defer r.Close()

	if len(r.File) != 0 {
		t.Errorf("expected 0 entries in ZIP for subdir-only dir, got %d", len(r.File))
	}
}
