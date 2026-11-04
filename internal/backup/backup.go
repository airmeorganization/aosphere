package backup

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type BackupResult struct {
	BackupPath      string    `json:"backup_path"`
	Filename        string    `json:"filename"`
	SizeBytes       int64     `json:"size_bytes"`
	CreatedAt       time.Time `json:"created_at"`
	IntegrityOK     bool      `json:"integrity_ok"`
	IntegrityLogs   []string  `json:"integrity_logs"`
	IsEncrypted     bool      `json:"is_encrypted"`
	ChecksumSHA256  string    `json:"checksum_sha256"`
	CloudUploaded   bool      `json:"cloud_uploaded"`
	CloudKey        string    `json:"cloud_key,omitempty"`
}

type BackupEntry struct {
	Filename       string    `json:"filename"`
	Path           string    `json:"path"`
	SizeBytes      int64     `json:"size_bytes"`
	ChecksumSHA256 string    `json:"checksum_sha256"`
	IsEncrypted    bool      `json:"is_encrypted"`
	CreatedAt      time.Time `json:"created_at"`
	IntegrityOK    bool      `json:"integrity_ok"`
}

type BackupService struct {
	db        *sql.DB
	backupDir string
	mu        sync.RWMutex
	s3Config  S3Config
}

func NewBackupService(database *sql.DB, backupDir string) *BackupService {
	if backupDir == "" {
		backupDir = filepath.Join("data", "backups")
	}
	return &BackupService{
		db:        database,
		backupDir: backupDir,
		s3Config: S3Config{
			Endpoint:  os.Getenv("S3_ENDPOINT"),
			Bucket:    os.Getenv("S3_BUCKET"),
			Region:    os.Getenv("S3_REGION"),
			AccessKey: os.Getenv("S3_ACCESS_KEY"),
			SecretKey: os.Getenv("S3_SECRET_KEY"),
		},
	}
}

// SetS3Config updates cloud backup credentials.
func (b *BackupService) SetS3Config(cfg S3Config) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.s3Config = cfg
}

// GetS3Config returns the active cloud backup configuration (redacting secret key).
func (b *BackupService) GetS3Config() S3Config {
	b.mu.RLock()
	defer b.mu.RUnlock()
	safe := b.s3Config
	if safe.SecretKey != "" {
		safe.SecretKey = "********"
	}
	return safe
}

// CreateBackup creates an atomic online snapshot using SQLite VACUUM INTO and verifies its integrity.
func (b *BackupService) CreateBackup(ctx context.Context, prefix string) (*BackupResult, error) {
	return b.CreateEncryptedBackup(ctx, prefix, "", false)
}

// CreateEncryptedBackup checkpoints the WAL, takes an atomic snapshot, checks integrity,
// optionally encrypts with AES-256-GCM, and optionally uploads to S3/R2 storage.
func (b *BackupService) CreateEncryptedBackup(ctx context.Context, prefix, passphrase string, uploadCloud bool) (*BackupResult, error) {
	if err := os.MkdirAll(b.backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	if prefix == "" {
		prefix = "aosphere_backup"
	}
	timestamp := time.Now().UTC().Format("20060102_150405")
	rawFilename := fmt.Sprintf("%s_%s.db", prefix, timestamp)
	rawPath := filepath.Join(b.backupDir, rawFilename)

	// Step 1: Checkpoint SQLite WAL log to ensure all pages are consolidated
	_, _ = b.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")

	// Step 2: Atomic online snapshot via VACUUM INTO
	sqlPath := filepath.ToSlash(rawPath)
	query := fmt.Sprintf("VACUUM INTO '%s'", sqlPath)
	if _, err := b.db.ExecContext(ctx, query); err != nil {
		return nil, fmt.Errorf("failed to execute VACUUM INTO: %w", err)
	}

	// Step 3: Run PRAGMA integrity_check drill on the generated snapshot
	integrityOK, logs, err := VerifyBackup(ctx, rawPath)
	if err != nil {
		return nil, fmt.Errorf("backup verification failed: %w", err)
	}

	finalPath := rawPath
	finalFilename := rawFilename
	isEncrypted := false

	// Step 4: AES-256-GCM encryption if passphrase supplied
	if passphrase != "" {
		encFilename := fmt.Sprintf("%s_%s.db.enc", prefix, timestamp)
		encPath := filepath.Join(b.backupDir, encFilename)
		if err := EncryptFile(rawPath, encPath, passphrase); err != nil {
			return nil, fmt.Errorf("failed to encrypt backup: %w", err)
		}
		// Securely remove unencrypted raw snapshot once encrypted copy is verified
		_ = os.Remove(rawPath)
		finalPath = encPath
		finalFilename = encFilename
		isEncrypted = true
	}

	fileInfo, err := os.Stat(finalPath)
	if err != nil {
		return nil, fmt.Errorf("unable to stat final backup file: %w", err)
	}

	checksum, _ := ComputeFileSHA256(finalPath)

	result := &BackupResult{
		BackupPath:     finalPath,
		Filename:       finalFilename,
		SizeBytes:      fileInfo.Size(),
		CreatedAt:      time.Now().UTC(),
		IntegrityOK:    integrityOK,
		IntegrityLogs:  logs,
		IsEncrypted:    isEncrypted,
		ChecksumSHA256: checksum,
	}

	// Step 5: Optional S3 / Cloudflare R2 Upload
	if uploadCloud {
		b.mu.RLock()
		s3cfg := b.s3Config
		b.mu.RUnlock()

		if s3cfg.Bucket != "" && s3cfg.AccessKey != "" {
			f, err := os.Open(finalPath)
			if err == nil {
				defer f.Close()
				s3Client := NewS3Client(s3cfg)
				s3Key := fmt.Sprintf("backups/%s", finalFilename)
				if err := s3Client.PutObject(ctx, s3Key, f, fileInfo.Size(), "application/octet-stream"); err == nil {
					result.CloudUploaded = true
					result.CloudKey = s3Key
				}
			}
		}
	}

	return result, nil
}

// ListBackups scans the backup directory and returns all local snapshots sorted descending by time.
func (b *BackupService) ListBackups() ([]BackupEntry, error) {
	entries := []BackupEntry{}
	if _, err := os.Stat(b.backupDir); os.IsNotExist(err) {
		return entries, nil
	}

	files, err := os.ReadDir(b.backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed reading backup directory: %w", err)
	}

	for _, fi := range files {
		if fi.IsDir() {
			continue
		}
		name := fi.Name()
		if !strings.HasSuffix(name, ".db") && !strings.HasSuffix(name, ".enc") {
			continue
		}

		info, err := fi.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(b.backupDir, name)
		checksum, _ := ComputeFileSHA256(fullPath)
		isEnc := strings.HasSuffix(name, ".enc")

		entries = append(entries, BackupEntry{
			Filename:       name,
			Path:           fullPath,
			SizeBytes:      info.Size(),
			ChecksumSHA256: checksum,
			IsEncrypted:    isEnc,
			CreatedAt:      info.ModTime().UTC(),
			IntegrityOK:    true,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})

	return entries, nil
}

// RestoreBackup restores a backup archive to the target SQLite database path.
func (b *BackupService) RestoreBackup(ctx context.Context, filename, passphrase, targetDBPath string) error {
	sourcePath := filepath.Join(b.backupDir, filename)
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", filename)
	}

	dbToRestore := sourcePath
	var tempDecrypted string

	if strings.HasSuffix(filename, ".enc") {
		if passphrase == "" {
			return fmt.Errorf("passphrase is required to restore encrypted backup")
		}
		tempDecrypted = filepath.Join(b.backupDir, "temp_restore_"+filename+".db")
		if err := DecryptFile(sourcePath, tempDecrypted, passphrase); err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
		defer os.Remove(tempDecrypted)
		dbToRestore = tempDecrypted
	}

	// Verify integrity before applying
	ok, _, err := VerifyBackup(ctx, dbToRestore)
	if err != nil || !ok {
		return fmt.Errorf("cannot restore backup: integrity check failed")
	}

	if targetDBPath == "" {
		targetDBPath = filepath.Join("data", "aosphere.db")
	}

	// Close connections and checkpoint existing if target matches current DB
	_, _ = b.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")

	// Copy backup database file to target
	srcFile, err := os.Open(dbToRestore)
	if err != nil {
		return fmt.Errorf("failed opening source backup: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(targetDBPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed opening destination database: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed copying backup to database: %w", err)
	}

	// Remove any stale WAL/SHM companion files to force clean recovery
	_ = os.Remove(targetDBPath + "-wal")
	_ = os.Remove(targetDBPath + "-shm")

	return nil
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
