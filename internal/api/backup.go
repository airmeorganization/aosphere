package api

import (
	"encoding/json"
	"net/http"

	"aosphere/internal/auth"
	"aosphere/internal/backup"
)

// SetBackupService allows overriding or injecting the backup service
func (a *API) SetBackupService(b *backup.BackupService) {
	a.backup = b
}

// GetBackupService returns the active backup service
func (a *API) GetBackupService() *backup.BackupService {
	return a.backup
}

// HandleBackupCreate triggers an online snapshot, optionally encrypted and uploaded to cloud
func (a *API) HandleBackupCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, ok := a.requireAuth(w, r, auth.PermAdmin)
	if !ok {
		return
	}
	if a.backup == nil {
		http.Error(w, "backup service not configured", http.StatusInternalServerError)
		return
	}

	var req struct {
		Prefix      string `json:"prefix"`
		Passphrase  string `json:"passphrase"`
		UploadCloud bool   `json:"upload_cloud"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	result, err := a.backup.CreateEncryptedBackup(r.Context(), req.Prefix, req.Passphrase, req.UploadCloud)
	if err != nil {
		http.Error(w, "backup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	payload, _ := json.Marshal(result)
	a.Broadcast("backup.created", string(payload))

	writeJSON(w, result)
}

// HandleBackupList returns all snapshots found in the local backup directory
func (a *API) HandleBackupList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, ok := a.requireAuth(w, r, auth.PermAdmin)
	if !ok {
		return
	}
	if a.backup == nil {
		writeJSON(w, []backup.BackupEntry{})
		return
	}

	entries, err := a.backup.ListBackups()
	if err != nil {
		http.Error(w, "failed listing backups: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, entries)
}

// HandleBackupRestore restores a backup snapshot onto the SQLite database
func (a *API) HandleBackupRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, ok := a.requireAuth(w, r, auth.PermAdmin)
	if !ok {
		return
	}
	if a.backup == nil {
		http.Error(w, "backup service not configured", http.StatusInternalServerError)
		return
	}

	var req struct {
		Filename     string `json:"filename"`
		Passphrase   string `json:"passphrase"`
		TargetDBPath string `json:"target_db_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Filename == "" {
		http.Error(w, "filename is required", http.StatusBadRequest)
		return
	}

	if err := a.backup.RestoreBackup(r.Context(), req.Filename, req.Passphrase, req.TargetDBPath); err != nil {
		http.Error(w, "restore failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	res := map[string]interface{}{
		"status":   "restored",
		"filename": req.Filename,
	}
	payload, _ := json.Marshal(res)
	a.Broadcast("backup.restored", string(payload))

	writeJSON(w, res)
}

// HandleBackupCloudSync uploads an existing local backup to the configured S3 bucket
func (a *API) HandleBackupCloudSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, ok := a.requireAuth(w, r, auth.PermAdmin)
	if !ok {
		return
	}
	if a.backup == nil {
		http.Error(w, "backup service not configured", http.StatusInternalServerError)
		return
	}

	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Filename == "" {
		http.Error(w, "filename is required", http.StatusBadRequest)
		return
	}

	s3Key, err := a.backup.UploadBackupToCloud(r.Context(), req.Filename)
	if err != nil {
		http.Error(w, "cloud sync failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	res := map[string]interface{}{
		"status":    "synced",
		"filename":  req.Filename,
		"cloud_key": s3Key,
	}
	payload, _ := json.Marshal(res)
	a.Broadcast("backup.synced", string(payload))

	writeJSON(w, res)
}

// HandleBackupConfig handles getting and setting the S3/R2 backup credentials
func (a *API) HandleBackupConfig(w http.ResponseWriter, r *http.Request) {
	_, ok := a.requireAuth(w, r, auth.PermAdmin)
	if !ok {
		return
	}
	if a.backup == nil {
		http.Error(w, "backup service not configured", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, a.backup.GetS3Config())
	case http.MethodPost:
		var cfg backup.S3Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "invalid config: "+err.Error(), http.StatusBadRequest)
			return
		}
		a.backup.SetS3Config(cfg)
		writeJSON(w, a.backup.GetS3Config())
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

