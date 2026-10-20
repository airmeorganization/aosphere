package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type BackupResult struct {
	BackupPath    string    `json:"backup_path"`
	SizeBytes     int64     `json:"size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
	IntegrityOK   bool      `json:"integrity_ok"`
	IntegrityLogs []string  `json:"integrity_logs"`
}

type BackupService struct {
	db        *sql.DB
	backupDir string
}

func NewBackupService(database *sql.DB, backupDir string) *BackupService {
	if backupDir == "" {
		backupDir = filepath.Join("data", "backups")
	}
	return &BackupService{
		db:        database,
		backupDir: backupDir,
	}
}

// CreateBackup creates an atomic online snapshot using SQLite VACUUM INTO and verifies its integrity
func (b *BackupService) CreateBackup(ctx context.Context, prefix string) (*BackupResult, error) {
	if err := os.MkdirAll(b.backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	if prefix == "" {
		prefix = "aosphere_backup"
	}
	timestamp := time.Now().Format("20060102_150405")
	backupFilename := fmt.Sprintf("%s_%s.db", prefix, timestamp)
	backupPath := filepath.Join(b.backupDir, backupFilename)

	// In Windows SQLite, paths for VACUUM INTO require forward slashes
	sqlPath := filepath.ToSlash(backupPath)

	// Execute atomic snapshot
	query := fmt.Sprintf("VACUUM INTO '%s'", sqlPath)
	if _, err := b.db.ExecContext(ctx, query); err != nil {
		return nil, fmt.Errorf("failed to execute VACUUM INTO: %w", err)
	}

	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("backup file created but unable to stat: %w", err)
	}

	// Verification Drill: open the backup file and run integrity_check
	integrityOK, logs, err := VerifyBackup(ctx, backupPath)
	if err != nil {
		return nil, fmt.Errorf("backup verification failed: %w", err)
	}

	return &BackupResult{
		BackupPath:    backupPath,
		SizeBytes:     fileInfo.Size(),
		CreatedAt:     time.Now(),
		IntegrityOK:   integrityOK,
		IntegrityLogs: logs,
	}, nil
}

// VerifyBackup opens a backup file and runs PRAGMA integrity_check
func VerifyBackup(ctx context.Context, backupPath string) (bool, []string, error) {
	dsn := fmt.Sprintf("%s?_pragma=query_only(1)", filepath.ToSlash(backupPath))
	backupDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return false, nil, fmt.Errorf("failed to open backup for verification: %w", err)
	}
	defer backupDB.Close()

	rows, err := backupDB.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return false, nil, fmt.Errorf("failed to run integrity_check on backup: %w", err)
	}
	defer rows.Close()

	var logs []string
	isOK := false
	for rows.Next() {
		var msg string
		if err := rows.Scan(&msg); err == nil {
			logs = append(logs, msg)
			if msg == "ok" {
				isOK = true
			}
		}
	}

	return isOK, logs, nil
}
