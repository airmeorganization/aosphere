package backup

import (
	"context"
	"path/filepath"
	"testing"

	"aosphere/internal/db"
)

func TestBackupService_CreateAndVerify(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "source.db")
	backupDir := filepath.Join(tempDir, "backups")

	database, err := db.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	svc := NewBackupService(database, backupDir)
	res, err := svc.CreateBackup(context.Background(), "test_snap")
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	if !res.IntegrityOK {
		t.Errorf("backup failed integrity check: %+v", res.IntegrityLogs)
	}
	if res.SizeBytes <= 0 {
		t.Errorf("expected backup file size > 0, got %d", res.SizeBytes)
	}
	t.Logf("Backup created at %s (%d bytes). Integrity OK: %t", res.BackupPath, res.SizeBytes, res.IntegrityOK)
}
