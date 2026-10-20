package service

import (
	"database/sql"
	"fmt"
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/db"
	"github.com/google/uuid"
)

type IdempotencyEngine struct{}

func NewIdempotencyEngine() *IdempotencyEngine {
	return &IdempotencyEngine{}
}

// GetOrCreate checks for an existing idempotency record or claims the key with 'in_progress'
func (e *IdempotencyEngine) GetOrCreate(tx *sql.Tx, orgID, key, payload string) (*db.IdempotencyRecord, error) {
	if key == "" {
		return nil, nil // No idempotency requested
	}

	payloadHash := auth.HashToken(payload)
	now := time.Now()

	var record db.IdempotencyRecord
	err := tx.QueryRow(`SELECT id, org_id, key, request_hash, status, response_code, response_body, created_at, expires_at 
		FROM idempotency_keys WHERE org_id = ? AND key = ?`, orgID, key).
		Scan(&record.ID, &record.OrgID, &record.Key, &record.RequestHash, &record.Status,
			&record.ResponseCode, &record.ResponseBody, &record.CreatedAt, &record.ExpiresAt)

	if err == nil {
		// Key already exists
		if record.Status == "completed" {
			return &record, nil
		}
		if record.Status == "in_progress" {
			// If locked for more than 2 minutes, consider previous worker abandoned and allow takeover
			if now.Sub(record.CreatedAt) > 2*time.Minute {
				_, _ = tx.Exec(`UPDATE idempotency_keys SET created_at = ?, request_hash = ? WHERE id = ?`, now, payloadHash, record.ID)
				record.Status = "in_progress"
				return &record, nil
			}
			return nil, ErrConcurrentRequest
		}
		// If previously failed, allow retry
		_, _ = tx.Exec(`UPDATE idempotency_keys SET status = 'in_progress', created_at = ?, request_hash = ? WHERE id = ?`, now, payloadHash, record.ID)
		record.Status = "in_progress"
		return &record, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query idempotency key: %w", err)
	}

	// Insert new in-progress claim
	newID := "idemp_" + uuid.New().String()[:8]
	expiresAt := now.Add(24 * time.Hour)
	_, err = tx.Exec(`INSERT INTO idempotency_keys (id, org_id, key, request_hash, status, response_code, response_body, created_at, expires_at)
		VALUES (?, ?, ?, ?, 'in_progress', 0, '', ?, ?)`,
		newID, orgID, key, payloadHash, now, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to record idempotency claim: %w", err)
	}

	return &db.IdempotencyRecord{
		ID:          newID,
		OrgID:       orgID,
		Key:         key,
		RequestHash: payloadHash,
		Status:      "in_progress",
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
	}, nil
}

// Complete updates the idempotency key with the final HTTP status and serialized body
func (e *IdempotencyEngine) Complete(tx *sql.Tx, orgID, key string, statusCode int, responseBody string) error {
	if key == "" {
		return nil
	}
	_, err := tx.Exec(`UPDATE idempotency_keys SET status = 'completed', response_code = ?, response_body = ? 
		WHERE org_id = ? AND key = ?`, statusCode, responseBody, orgID, key)
	return err
}

// Fail clears or marks failed status so a client can re-attempt cleanly
func (e *IdempotencyEngine) Fail(tx *sql.Tx, orgID, key string) error {
	if key == "" {
		return nil
	}
	_, err := tx.Exec(`UPDATE idempotency_keys SET status = 'failed' WHERE org_id = ? AND key = ?`, orgID, key)
	return err
}
