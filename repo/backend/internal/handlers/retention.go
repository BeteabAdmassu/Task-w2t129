package handlers

// retention.go — scheduled purge of files that have passed their retention_until date.
//
// Design:
//   - Runs as a background goroutine started by StartRetentionScheduler.
//   - Ticks once on startup then every 24 hours.
//   - For each expired file: delete from disk, delete DB record, write audit log.
//   - Audit entries use a sentinel system user ID ("system") so they appear in the
//     audit trail without requiring a real authenticated user.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	"medops/internal/models"
	"medops/internal/repository"
)

const retentionInterval = 24 * time.Hour

// StartRetentionScheduler launches a goroutine that purges expired managed files
// once per day. It returns immediately; pass a done channel to stop it.
func StartRetentionScheduler(repo *repository.Repository, dataDir string, done <-chan struct{}) {
	go runRetentionLoop(repo, dataDir, done)
}

func runRetentionLoop(repo *repository.Repository, dataDir string, done <-chan struct{}) {
	// Run once immediately on startup, then tick every 24 h.
	purgeExpiredFiles(repo, dataDir)

	ticker := time.NewTicker(retentionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			purgeExpiredFiles(repo, dataDir)
		case <-done:
			log.Info("Retention scheduler stopped")
			return
		}
	}
}

func purgeExpiredFiles(repo *repository.Repository, dataDir string) {
	files, err := repo.ListExpiredFiles()
	if err != nil {
		log.WithError(err).Error("retention: failed to list expired files")
		return
	}
	if len(files) == 0 {
		return
	}

	log.WithField("count", len(files)).Info("retention: purging expired files")

	for _, f := range files {
		purgeFile(repo, dataDir, f)
	}
}

func purgeFile(repo *repository.Repository, dataDir string, f models.ManagedFile) {
	logger := log.WithFields(log.Fields{
		"file_id":   f.ID,
		"file_name": f.OriginalName,
	})

	// Remove from disk.
	diskPath := f.StoragePath
	if !filepath.IsAbs(diskPath) {
		diskPath = filepath.Join(dataDir, diskPath)
	}
	if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
		logger.WithError(err).Error("retention: failed to delete file from disk")
		// Continue: remove the DB record even if the file is already gone.
	}

	// Remove DB record.
	if err := repo.DeleteFileRecord(f.ID); err != nil {
		logger.WithError(err).Error("retention: failed to delete file record")
		return
	}

	// Write audit log entry.
	detailsJSON, _ := json.Marshal(map[string]string{
		"reason":         "retention_until exceeded",
		"original_name":  f.OriginalName,
		"purged_by":      "system",
	})
	entry := &models.AuditLogEntry{
		UserID:     "system",
		Action:     "retention_purge",
		EntityType: "managed_file",
		EntityID:   f.ID,
		Details:    detailsJSON,
	}
	if err := repo.CreateAuditLog(entry); err != nil {
		logger.WithError(err).Warn("retention: audit log write failed after successful purge")
	}

	logger.Info("retention: expired file purged")
}
