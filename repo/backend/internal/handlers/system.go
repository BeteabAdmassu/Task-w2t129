package handlers

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"

	"medops/internal/middleware"
	"medops/internal/models"
	"medops/internal/repository"
)

// SystemHandler handles system-level requests.
type SystemHandler struct {
	repo        *repository.Repository
	dataDir     string
	databaseURL string
	startTime   time.Time
}

// NewSystemHandler creates a new SystemHandler.
func NewSystemHandler(repo *repository.Repository, dataDir, databaseURL string) *SystemHandler {
	return &SystemHandler{repo: repo, dataDir: dataDir, databaseURL: databaseURL, startTime: time.Now()}
}

// HealthCheck returns the system health status, version, and uptime.
func (h *SystemHandler) HealthCheck(c echo.Context) error {
	uptime := time.Since(h.startTime).Round(time.Second).String()
	version := extractPackageVersion(h.dataDir, "unknown")
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   version,
		"uptime":    uptime,
	})
}

// ─── Version history ──────────────────────────────────────────────────────────

// versionHistoryEntry records one update applied to the system.
// ArtifactDir is the path to a snapshot of the application binaries and
// frontend assets that were ACTIVE before this update was applied — it is the
// restore target for a full app+DB rollback.
type versionHistoryEntry struct {
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
	BackupFile  string `json:"backup_file"`  // pg_dump snapshot taken before migration
	ArtifactDir string `json:"artifact_dir"` // app artifact snapshot (backend + frontend)
	AppliedAt   string `json:"applied_at"`
}

// versionHistoryPath returns the path to the version history manifest.
func (h *SystemHandler) versionHistoryPath() string {
	return filepath.Join(h.dataDir, "updates", "version_history.json")
}

// activeDir is the directory where the currently active binary/asset overrides live.
// DATA_DIR/active/backend/ — backend binary override (checked by Electron before bundled binary)
// DATA_DIR/active/frontend/ — frontend asset override (checked by Electron before bundled assets)
func (h *SystemHandler) activeDir() string {
	return filepath.Join(h.dataDir, "active")
}

// versionsDir is where artifact snapshots are stored, one sub-directory per update epoch.
func (h *SystemHandler) versionsDir() string {
	return filepath.Join(h.dataDir, "versions")
}

// restartFlagPath returns the path of the restart sentinel file.
// Electron's main process polls for this file; when found, it stops the backend
// subprocess, starts it again from the (now-restored) active binary, and reloads
// the renderer.
func (h *SystemHandler) restartFlagPath() string {
	return filepath.Join(h.dataDir, "restart.flag")
}

// currentVersion reads the last known version from version history (or "baseline").
func (h *SystemHandler) currentVersion() string {
	data, err := os.ReadFile(h.versionHistoryPath())
	if err != nil {
		return "baseline"
	}
	var entries []versionHistoryEntry
	if json.Unmarshal(data, &entries) != nil || len(entries) == 0 {
		return "baseline"
	}
	return entries[len(entries)-1].ToVersion
}

// appendVersionHistory appends one entry to the version history manifest.
func (h *SystemHandler) appendVersionHistory(entry versionHistoryEntry) {
	path := h.versionHistoryPath()
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	var entries []versionHistoryEntry
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &entries)
	}
	entries = append(entries, entry)
	if data, err := json.MarshalIndent(entries, "", "  "); err == nil {
		_ = os.WriteFile(path, data, 0644)
	}
}

// ─── Artifact management ──────────────────────────────────────────────────────

// snapshotArtifacts captures the current active/ binary and asset overrides into a
// timestamped directory under versionsDir.  This snapshot becomes the restore target
// if the user rolls back after the next update.
//
// If active/backend or active/frontend do not yet exist (e.g. first-ever update),
// the corresponding subdirectory is simply omitted from the snapshot — a subsequent
// rollback will skip app-level restoration for those absent subdirectories and fall
// back to the bundled binary/assets.
func (h *SystemHandler) snapshotArtifacts(timestamp string) (string, error) {
	artifactDir := filepath.Join(h.versionsDir(), timestamp)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return "", fmt.Errorf("create artifact dir: %w", err)
	}
	for _, sub := range []string{"backend", "frontend"} {
		src := filepath.Join(h.activeDir(), sub)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue // nothing to snapshot yet
		}
		dst := filepath.Join(artifactDir, sub)
		if err := copyDir(src, dst); err != nil {
			return "", fmt.Errorf("snapshot %s artifacts: %w", sub, err)
		}
	}
	return artifactDir, nil
}

// restoreArtifacts copies a versioned artifact snapshot back to active/.
// It replaces active/backend and active/frontend entirely so the Electron process
// picks up the correct binary on its next start.  Sub-directories not present in
// the snapshot are left untouched (graceful partial restore).
func (h *SystemHandler) restoreArtifacts(artifactDir string) error {
	if artifactDir == "" {
		return nil // no artifact snapshot recorded for this history entry
	}
	for _, sub := range []string{"backend", "frontend"} {
		src := filepath.Join(artifactDir, sub)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue // sub-snapshot absent — leave active unchanged
		}
		dst := filepath.Join(h.activeDir(), sub)
		// Remove current active override before restoring snapshot.
		if err := os.RemoveAll(dst); err != nil {
			return fmt.Errorf("clear active %s: %w", sub, err)
		}
		if err := copyDir(src, dst); err != nil {
			return fmt.Errorf("restore %s artifacts: %w", sub, err)
		}
	}
	return nil
}

// writeRestartFlag writes a sentinel file that signals the Electron main process to
// stop the backend subprocess, start it from the restored active binary, and reload
// the renderer window.  The file content is the version being restored to so the
// notification shown to the user is meaningful.
func (h *SystemHandler) writeRestartFlag(version string) {
	_ = os.WriteFile(h.restartFlagPath(), []byte(version), 0644)
}

// copyDir recursively copies all files from src to dst, preserving permissions.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

// copyFile copies a single file from src to dst, creating parent directories as needed.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// ─── Backup helpers ───────────────────────────────────────────────────────────

// preUpdateBackup runs pg_dump before applying an update so rollback has a precise snapshot.
func (h *SystemHandler) preUpdateBackup() (string, error) {
	backupDir := filepath.Join(h.dataDir, "backups")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("pre_update_%s.sql", timestamp))
	cmd := exec.Command("pg_dump", "--dbname", h.databaseURL, "--file", backupFile, "--format=plain", "--no-password")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pg_dump: %w (output: %s)", err, string(output))
	}
	return backupFile, nil
}

// Backup performs a pg_dump of the database to the data directory.
func (h *SystemHandler) Backup(c echo.Context) error {
	userID := middleware.GetUserID(c)

	backupDir := filepath.Join(h.dataDir, "backups")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		logrus.WithError(err).Error("Failed to create backup directory")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to prepare backup directory",
			Code:  http.StatusInternalServerError,
		})
	}

	timestamp := time.Now().UTC().Format("20060102T150405Z")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("backup_%s.sql", timestamp))

	cmd := exec.Command("pg_dump", "--dbname", h.databaseURL, "--file", backupFile, "--format=plain", "--no-password")
	if output, err := cmd.CombinedOutput(); err != nil {
		logrus.WithError(err).WithField("output", string(output)).Error("pg_dump failed")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Backup failed",
			Code:    http.StatusInternalServerError,
			Details: "Database dump did not complete successfully",
		})
	}

	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "backup_completed",
		EntityType: "system",
		EntityID:   "backup",
	})

	logrus.WithFields(logrus.Fields{
		"user_id":     userID,
		"backup_file": backupFile,
	}).Info("Backup completed")

	return c.JSON(http.StatusOK, map[string]string{
		"message":     "Backup completed successfully",
		"backup_file": backupFile,
		"timestamp":   timestamp,
	})
}

// BackupStatus returns backup status by checking for recent backup files.
func (h *SystemHandler) BackupStatus(c echo.Context) error {
	backupDir := filepath.Join(h.dataDir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":      "no_backups",
			"last_backup": nil,
			"message":     "No backups found",
		})
	}

	var lastBackup *string
	for i := len(entries) - 1; i >= 0; i-- {
		if !entries[i].IsDir() {
			name := entries[i].Name()
			lastBackup = &name
			break
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":      "idle",
		"last_backup": lastBackup,
		"message":     "Backup system operational",
	})
}

// ─── Config ───────────────────────────────────────────────────────────────────

// GetConfig returns the system configuration.
func (h *SystemHandler) GetConfig(c echo.Context) error {
	config, err := h.repo.GetConfig()
	if err != nil {
		logrus.WithError(err).Error("Failed to get system config")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve system configuration",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"config": config,
	})
}

// UpdateConfig updates a system configuration value.
func (h *SystemHandler) UpdateConfig(c echo.Context) error {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.Key == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Configuration key is required",
		})
	}

	if err := h.repo.UpdateConfig(req.Key, req.Value); err != nil {
		logrus.WithError(err).Error("Failed to update system config")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to update system configuration",
			Code:  http.StatusInternalServerError,
		})
	}

	userID := middleware.GetUserID(c)
	details, _ := json.Marshal(map[string]string{
		"key": req.Key,
	})
	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "update_config",
		EntityType: "system_config",
		EntityID:   req.Key,
		Details:    details,
	})

	logrus.WithFields(logrus.Fields{
		"user_id": userID,
		"key":     req.Key,
	}).Info("System config updated")

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Configuration updated successfully",
		"key":     req.Key,
	})
}

// ─── Package helpers ──────────────────────────────────────────────────────────

// extractPackageVersion reads the VERSION or version.txt file from a directory.
// Falls back to the supplied fallback string when the file is absent or empty.
func extractPackageVersion(dir, fallback string) string {
	for _, name := range []string{"VERSION", "version.txt"} {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			v := strings.TrimSpace(string(data))
			if v != "" {
				return v
			}
		}
	}
	return fallback
}

// extractZIPToDir extracts .sql files from a ZIP archive into destDir.
func extractZIPToDir(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(f.Name)) != ".sql" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open entry %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("read entry %s: %w", f.Name, err)
		}
		dest := filepath.Join(destDir, filepath.Base(f.Name))
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", filepath.Base(f.Name), err)
		}
	}
	return nil
}

// extractZIPArtifacts extracts the backend/ and frontend/ subtrees from a ZIP
// package into activeDir.  Files outside those two subdirectories are ignored.
// This is what installs new application binaries and frontend assets when an
// offline update package is applied.
func extractZIPArtifacts(zipPath, activeDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()
	for _, f := range r.File {
		// Normalise separators for prefix matching.
		name := filepath.ToSlash(f.Name)
		if !strings.HasPrefix(name, "backend/") && !strings.HasPrefix(name, "frontend/") {
			continue
		}
		dest := filepath.Join(activeDir, filepath.FromSlash(name))
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(dest, 0755)
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("read %s: %w", f.Name, err)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, f.Mode()); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
	}
	return nil
}

// ─── Update ───────────────────────────────────────────────────────────────────

// ApplyUpdate accepts an offline update package (multipart .zip or .sql upload),
// stages and applies all SQL migrations in lexicographic order, and installs any
// included backend binary / frontend asset overrides into DATA_DIR/active/.
//
// Before any changes are made:
//   - A pg_dump snapshot of the current database is written to DATA_DIR/backups/.
//   - The current application artifacts in DATA_DIR/active/ (if any) are copied to
//     DATA_DIR/versions/<timestamp>/ so a subsequent rollback can restore both DB
//     and application binaries atomically.
//
// If no file is uploaded the handler falls back to a pre-staged pending directory.
// After a successful apply the staging directory is renamed applied_<timestamp>.
func (h *SystemHandler) ApplyUpdate(c echo.Context) error {
	userID := middleware.GetUserID(c)

	// Use a single timestamp throughout so DB snapshot, artifact snapshot, and
	// version history all share the same epoch label.
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	updateDir := filepath.Join(h.dataDir, "updates", "pending")

	// ── Stage the uploaded package ────────────────────────────────────────────
	var tmpZip string // only set when a ZIP is uploaded

	uploadedFile, uploadErr := c.FormFile("file")
	if uploadErr == nil {
		src, err := uploadedFile.Open()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to read uploaded file",
				Code:  http.StatusInternalServerError,
			})
		}
		defer src.Close()

		if err := os.MkdirAll(updateDir, 0755); err != nil {
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error: "Failed to prepare update directory",
				Code:  http.StatusInternalServerError,
			})
		}

		ext := strings.ToLower(filepath.Ext(uploadedFile.Filename))
		switch ext {
		case ".zip":
			tmpZip = filepath.Join(h.dataDir, "updates", "upload_"+timestamp+".zip")
			dst, err := os.Create(tmpZip)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
					Error: "Failed to save uploaded package",
					Code:  http.StatusInternalServerError,
				})
			}
			if _, err := io.Copy(dst, src); err != nil {
				dst.Close()
				os.Remove(tmpZip)
				return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
					Error: "Failed to write uploaded package",
					Code:  http.StatusInternalServerError,
				})
			}
			dst.Close()
			if err := extractZIPToDir(tmpZip, updateDir); err != nil {
				os.Remove(tmpZip)
				return c.JSON(http.StatusBadRequest, models.ErrorResponse{
					Error:   "Failed to extract SQL migrations from package",
					Code:    http.StatusBadRequest,
					Details: err.Error(),
				})
			}
		case ".sql":
			dst, err := os.Create(filepath.Join(updateDir, filepath.Base(uploadedFile.Filename)))
			if err != nil {
				return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
					Error: "Failed to stage SQL migration",
					Code:  http.StatusInternalServerError,
				})
			}
			if _, err := io.Copy(dst, src); err != nil {
				dst.Close()
				return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
					Error: "Failed to write SQL migration",
					Code:  http.StatusInternalServerError,
				})
			}
			dst.Close()
		default:
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Unsupported package format",
				Code:    http.StatusBadRequest,
				Details: "Upload a .zip package or a single .sql migration file",
			})
		}
	} else {
		// No upload — require a pre-staged pending directory.
		if _, err := os.Stat(updateDir); os.IsNotExist(err) {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "No pending update found",
				Code:    http.StatusNotFound,
				Details: "Upload a .zip or .sql package via multipart, or stage files in " + updateDir,
			})
		}
	}

	// ── Pre-update snapshots (DB + artifacts) ─────────────────────────────────
	// Capture the CURRENT state before any mutation so Rollback has an exact target.

	fromVersion := h.currentVersion()

	preBackupFile, backupErr := h.preUpdateBackup()
	if backupErr != nil {
		logrus.WithError(backupErr).Warn("Pre-update DB snapshot failed; DB rollback to this version will not be available")
		preBackupFile = ""
	}

	artifactDir, artifactErr := h.snapshotArtifacts(timestamp)
	if artifactErr != nil {
		logrus.WithError(artifactErr).Warn("Pre-update artifact snapshot failed; app rollback to this version will not be available")
		artifactDir = ""
	}

	// ── Install new application artifacts from ZIP ────────────────────────────
	// Extract backend/ and frontend/ subdirectories from the uploaded ZIP into
	// DATA_DIR/active/ so Electron picks them up on next launch.
	if tmpZip != "" {
		if err := extractZIPArtifacts(tmpZip, h.activeDir()); err != nil {
			logrus.WithError(err).Warn("Failed to extract application artifacts from package; SQL migrations will still be applied")
		}
		os.Remove(tmpZip)
	}

	// ── Apply SQL migrations ──────────────────────────────────────────────────
	entries, err := os.ReadDir(updateDir)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to read update package",
			Code:  http.StatusInternalServerError,
		})
	}

	applied := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		sqlPath := filepath.Join(updateDir, entry.Name())
		sqlBytes, err := os.ReadFile(sqlPath)
		if err != nil {
			logrus.WithError(err).WithField("file", entry.Name()).Error("Failed to read SQL migration file")
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Failed to read migration file",
				Code:    http.StatusInternalServerError,
				Details: entry.Name(),
			})
		}
		if _, err := h.repo.DB.Exec(string(sqlBytes)); err != nil {
			logrus.WithError(err).WithField("file", entry.Name()).Error("SQL migration failed")
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Migration failed",
				Code:    http.StatusInternalServerError,
				Details: entry.Name() + ": " + err.Error(),
			})
		}
		applied++
	}

	// ── Finalise ──────────────────────────────────────────────────────────────
	version := extractPackageVersion(updateDir, timestamp)

	// Rename pending → applied_<timestamp> for auditability.
	appliedDir := filepath.Join(h.dataDir, "updates", "applied_"+timestamp)
	if err := os.Rename(updateDir, appliedDir); err != nil {
		logrus.WithError(err).Warn("Failed to rename applied update directory")
	}

	// Record version history — both DB snapshot and artifact snapshot paths.
	if preBackupFile != "" || artifactDir != "" {
		h.appendVersionHistory(versionHistoryEntry{
			FromVersion: fromVersion,
			ToVersion:   version,
			BackupFile:  preBackupFile,
			ArtifactDir: artifactDir,
			AppliedAt:   timestamp,
		})
	}

	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "apply_update",
		EntityType: "system",
		EntityID:   version,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":       userID,
		"version":       version,
		"migrations":    applied,
		"applied_path":  appliedDir,
		"artifact_dir":  artifactDir,
	}).Info("Offline update applied")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"version":          version,
		"status":           "applied",
		"migrations":       applied,
		"applied_at":       timestamp,
		"restart_required": true, // new binary/assets always need a backend restart to take effect
	})
}

// ─── Rollback ─────────────────────────────────────────────────────────────────

// Rollback restores the system to the version recorded in the most recent
// version_history.json entry.  It performs a full version rollback:
//
//  1. Application artifacts (backend binary + frontend assets) are restored from
//     the artifact snapshot recorded during the preceding ApplyUpdate call.
//  2. The database is restored from the pg_dump snapshot taken immediately before
//     that update was applied.
//  3. A restart.flag sentinel file is written so the Electron main process polls
//     for it, kills the running backend subprocess, starts it again from the
//     restored binary in DATA_DIR/active/backend/, and reloads the renderer.
//
// Falls back to DB-only restore when the history entry has no artifact_dir (e.g.
// history written by an older version of the handler) and to the most recent .sql
// backup when no version history exists at all.
func (h *SystemHandler) Rollback(c echo.Context) error {
	userID := middleware.GetUserID(c)

	// ── Locate the rollback target ────────────────────────────────────────────
	var (
		latestBackup      string
		rollbackToVersion string
		artifactDir       string
	)

	if data, err := os.ReadFile(h.versionHistoryPath()); err == nil {
		var history []versionHistoryEntry
		if json.Unmarshal(data, &history) == nil && len(history) > 0 {
			latest := history[len(history)-1]
			if _, statErr := os.Stat(latest.BackupFile); statErr == nil {
				latestBackup = latest.BackupFile
				rollbackToVersion = latest.FromVersion
				artifactDir = latest.ArtifactDir
			}
		}
	}

	// Fall back to most recent .sql file in backups dir.
	if latestBackup == "" {
		backupDir := filepath.Join(h.dataDir, "backups")
		entries, err := os.ReadDir(backupDir)
		if err != nil || len(entries) == 0 {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "No backups available",
				Code:    http.StatusNotFound,
				Details: "Create a backup via POST /system/backup before attempting rollback",
			})
		}
		for i := len(entries) - 1; i >= 0; i-- {
			if !entries[i].IsDir() && filepath.Ext(entries[i].Name()) == ".sql" {
				latestBackup = filepath.Join(backupDir, entries[i].Name())
				break
			}
		}
		if latestBackup == "" {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: "No SQL backup files found",
				Code:  http.StatusNotFound,
			})
		}
	}

	// ── Restore application artifacts ─────────────────────────────────────────
	// Done before the DB restore so that if artifact restore fails the DB is
	// still intact and the operator can investigate without data loss.
	artifactsRestored := false
	if artifactErr := h.restoreArtifacts(artifactDir); artifactErr != nil {
		logrus.WithError(artifactErr).Error("Artifact restore failed during rollback")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Rollback failed: could not restore application artifacts",
			Code:    http.StatusInternalServerError,
			Details: artifactErr.Error(),
		})
	}
	if artifactDir != "" {
		artifactsRestored = true
	}

	// ── Restore database ──────────────────────────────────────────────────────
	cmd := exec.Command("psql", "--dbname", h.databaseURL, "--file", latestBackup, "--no-password", "--single-transaction")
	if output, err := cmd.CombinedOutput(); err != nil {
		logrus.WithError(err).WithField("output", string(output)).Error("Rollback (psql restore) failed")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Rollback failed: database restore did not complete successfully",
			Code:    http.StatusInternalServerError,
			Details: "Database restore did not complete successfully",
		})
	}

	// ── Write restart flag ────────────────────────────────────────────────────
	// Electron's polling loop will detect this, stop the backend subprocess,
	// start it from the restored DATA_DIR/active/backend binary, and reload
	// the renderer window.
	rollbackVersion := rollbackToVersion
	if rollbackVersion == "" {
		backupBase := filepath.Base(latestBackup)
		rollbackVersion = strings.TrimSuffix(strings.TrimPrefix(backupBase, "backup_"), ".sql")
		if rollbackVersion == backupBase {
			rollbackVersion = backupBase
		}
	}
	h.writeRestartFlag(rollbackVersion)

	// ── Update version history ────────────────────────────────────────────────
	// Remove the applied history entry so subsequent rollbacks chain correctly.
	if rollbackToVersion != "" {
		if data, err := os.ReadFile(h.versionHistoryPath()); err == nil {
			var history []versionHistoryEntry
			if json.Unmarshal(data, &history) == nil && len(history) > 0 {
				history = history[:len(history)-1]
				if updated, err := json.MarshalIndent(history, "", "  "); err == nil {
					_ = os.WriteFile(h.versionHistoryPath(), updated, 0644)
				}
			}
		}
	}

	rolledBackAt := time.Now().UTC().Format(time.RFC3339)

	h.repo.CreateAuditLog(&models.AuditLogEntry{
		UserID:     userID,
		Action:     "rollback_completed",
		EntityType: "system",
		EntityID:   rollbackVersion,
	})

	logrus.WithFields(logrus.Fields{
		"user_id":            userID,
		"rollback_version":   rollbackVersion,
		"restored_from":      latestBackup,
		"artifacts_restored": artifactsRestored,
	}).Info("Rollback completed")

	return c.JSON(http.StatusOK, map[string]interface{}{
		"version":            rollbackVersion,
		"status":             "rolled_back",
		"restored_from":      latestBackup,
		"rolled_back_at":     rolledBackAt,
		"artifacts_restored": artifactsRestored,
		"restart_required":   true,
	})
}

// ─── Draft management ─────────────────────────────────────────────────────────

// SaveDraft saves a draft checkpoint.
// The form_type is taken from the route parameter ":formType", not the body.
func (h *SystemHandler) SaveDraft(c echo.Context) error {
	userID := middleware.GetUserID(c)
	formType := c.Param("formType")
	if formType == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "Form type is required in the URL path",
		})
	}

	var req struct {
		FormID    *string         `json:"form_id"`
		StateJSON json.RawMessage `json:"state_json"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request body",
			Code:  http.StatusBadRequest,
		})
	}

	if req.StateJSON == nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Code:    http.StatusBadRequest,
			Details: "State JSON is required",
		})
	}

	draft := &models.DraftCheckpoint{
		ID:        uuid.New().String(),
		UserID:    userID,
		FormType:  formType,
		FormID:    req.FormID,
		StateJSON: req.StateJSON,
		SavedAt:   time.Now(),
	}

	if err := h.repo.SaveDraft(draft); err != nil {
		logrus.WithError(err).Error("Failed to save draft")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to save draft",
			Code:  http.StatusInternalServerError,
		})
	}

	logrus.WithFields(logrus.Fields{
		"user_id":   userID,
		"form_type": formType,
		"draft_id":  draft.ID,
	}).Info("Draft saved")

	return c.JSON(http.StatusCreated, draft)
}

// ListDrafts returns all drafts for the authenticated user.
func (h *SystemHandler) ListDrafts(c echo.Context) error {
	userID := middleware.GetUserID(c)

	drafts, err := h.repo.ListDrafts(userID)
	if err != nil {
		logrus.WithError(err).Error("Failed to list drafts")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to retrieve drafts",
			Code:  http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"data": drafts,
	})
}

// GetDraft returns a draft identified by (userID, formType, formId).
func (h *SystemHandler) GetDraft(c echo.Context) error {
	userID := middleware.GetUserID(c)
	formType := c.Param("formType")
	formID := c.Param("formId")
	if formType == "" || formID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "formType and formId path parameters are required",
			Code:  http.StatusBadRequest,
		})
	}

	draft, err := h.repo.GetDraft(userID, formType, formID)
	if err != nil {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Draft not found",
			Code:    http.StatusNotFound,
			Details: "No draft found for the given user, form type, and form ID",
		})
	}

	return c.JSON(http.StatusOK, draft)
}

// DeleteDraft deletes a draft identified by (userID, formType, formId).
func (h *SystemHandler) DeleteDraft(c echo.Context) error {
	userID := middleware.GetUserID(c)
	formType := c.Param("formType")
	formID := c.Param("formId")
	if formType == "" || formID == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "formType and formId path parameters are required",
			Code:  http.StatusBadRequest,
		})
	}

	if err := h.repo.DeleteDraft(userID, formType, formID); err != nil {
		logrus.WithError(err).Error("Failed to delete draft")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Failed to delete draft",
			Code:  http.StatusInternalServerError,
		})
	}

	logrus.WithFields(logrus.Fields{
		"user_id":   userID,
		"form_type": formType,
		"form_id":   formID,
	}).Info("Draft deleted")

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Draft deleted successfully",
	})
}
