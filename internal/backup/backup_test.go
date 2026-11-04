package backup

import (
	"context"
	"os"
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
	if res.ChecksumSHA256 == "" {
		t.Errorf("expected SHA-256 checksum, got empty string")
	}
	t.Logf("Backup created at %s (%d bytes). Integrity OK: %t, SHA256: %s", res.BackupPath, res.SizeBytes, res.IntegrityOK, res.ChecksumSHA256)
}

func TestBackupService_EncryptedBackupAndRestore(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "source_enc.db")
	backupDir := filepath.Join(tempDir, "backups_enc")
	restorePath := filepath.Join(tempDir, "restored.db")

	database, err := db.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	// Seed custom table to verify restoration
	_, err = database.Exec("CREATE TABLE test_data (id TEXT PRIMARY KEY, val TEXT); INSERT INTO test_data VALUES ('1', 'secret_payload');")
	if err != nil {
		t.Fatalf("failed creating test table: %v", err)
	}

	svc := NewBackupService(database, backupDir)
	passphrase := "SuperSecureEnterprisePassphrase2026!"

	// 1. Create encrypted backup
	res, err := svc.CreateEncryptedBackup(context.Background(), "snap_enc", passphrase, false)
	if err != nil {
		t.Fatalf("CreateEncryptedBackup failed: %v", err)
	}
	if !res.IsEncrypted {
		t.Errorf("expected backup to be marked encrypted")
	}

	// 2. List backups
	list, err := svc.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 backup in list, got %d", len(list))
	}
	if !list[0].IsEncrypted {
		t.Errorf("expected listed backup to be marked encrypted")
	}

	// 3. Restore backup
	err = svc.RestoreBackup(context.Background(), list[0].Filename, passphrase, restorePath)
	if err != nil {
		t.Fatalf("RestoreBackup failed: %v", err)
	}

	// 4. Verify restored DB content
	restoredDB, err := db.InitDB(restorePath, false)
	if err != nil {
		t.Fatalf("failed opening restored DB: %v", err)
	}
	defer restoredDB.Close()

	var val string
	err = restoredDB.QueryRow("SELECT val FROM test_data WHERE id = '1'").Scan(&val)
	if err != nil || val != "secret_payload" {
		t.Fatalf("restored data mismatch: expected 'secret_payload', got %q (err: %v)", val, err)
	}

	// 5. Attempt restore with wrong passphrase - must fail
	err = svc.RestoreBackup(context.Background(), list[0].Filename, "WrongPassword!", filepath.Join(tempDir, "fail.db"))
	if err == nil {
		t.Errorf("expected restore with invalid passphrase to fail, but it succeeded")
	}
}

func TestBackupService_CryptoHelpers(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "sample.txt")
	enc := filepath.Join(tempDir, "sample.enc")
	dec := filepath.Join(tempDir, "sample.dec")

	origText := "Enterprise mission-critical data payload"
	if err := os.WriteFile(src, []byte(origText), 0600); err != nil {
		t.Fatalf("failed writing sample: %v", err)
	}

	pass := "TestingPassphrase123"
	if err := EncryptFile(src, enc, pass); err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	if err := DecryptFile(enc, dec, pass); err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	decBytes, err := os.ReadFile(dec)
	if err != nil {
		t.Fatalf("failed reading decrypted file: %v", err)
	}
	if string(decBytes) != origText {
		t.Fatalf("decrypted text mismatch: expected %q, got %q", origText, string(decBytes))
	}
}
