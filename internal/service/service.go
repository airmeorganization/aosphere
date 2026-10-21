package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"aosphere/internal/backup"
	"aosphere/internal/ledger"
	"aosphere/internal/store"
	"github.com/google/uuid"
)

// WorkflowService is the central business logic layer shared identically across REST API and MCP tools
type WorkflowService struct {
	store   *store.Store
	policy  *PolicyEngine
	idemp   *IdempotencyEngine
	ledger  *ledger.LedgerService
	backup  *backup.BackupService
}

func NewWorkflowService(s *store.Store) *WorkflowService {
	return &WorkflowService{
		store:   s,
		policy:  NewPolicyEngine(),
		idemp:   NewIdempotencyEngine(),
		ledger:  ledger.NewLedgerService(s.DB()),
		backup:  backup.NewBackupService(s.DB(), ""),
	}
}

// Store returns the underlying store for read-only convenience
func (s *WorkflowService) Store() *store.Store {
	return s.store
}

// Policy returns the business rules policy engine
func (s *WorkflowService) Policy() *PolicyEngine {
	return s.policy
}

// Ledger returns the double-entry general ledger service
func (s *WorkflowService) Ledger() *ledger.LedgerService {
	return s.ledger
}

// Backup returns the database backup service
func (s *WorkflowService) Backup() *backup.BackupService {
	return s.backup
}

// TrialBalance returns the financial trial balance verifying debits == credits
func (s *WorkflowService) TrialBalance(ctx context.Context, orgID string) (*ledger.TrialBalanceReport, error) {
	return s.ledger.GetTrialBalance(ctx, orgID)
}

// CreateBackup creates an atomic online SQLite snapshot and verifies integrity
func (s *WorkflowService) CreateBackup(ctx context.Context, prefix string) (*backup.BackupResult, error) {
	return s.backup.CreateBackup(ctx, prefix)
}

// RecordOutboxEvent records an event inside the caller's active database transaction
func RecordOutboxEvent(ctx context.Context, tx *sql.Tx, orgID, eventType string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	id := fmt.Sprintf("evt_%d_%s", time.Now().UnixNano(), uuid.New().String()[:8])
	_, err = tx.ExecContext(ctx, `INSERT INTO outbox_events (id, org_id, event_type, payload_json, status, created_at)
		VALUES (?, ?, ?, ?, 'pending', ?)`, id, orgID, eventType, string(payloadBytes), time.Now())
	return err
}

// DispatchOutboxEvents processes pending outbox events and broadcasts them
func (s *WorkflowService) DispatchOutboxEvents(ctx context.Context, broadcast func(eventType, payload string)) (int, error) {
	rows, err := s.store.DB().QueryContext(ctx, `SELECT id, event_type, payload_json FROM outbox_events WHERE status = 'pending' ORDER BY created_at ASC LIMIT 100`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type pendingEvent struct {
		id, eventType, payload string
	}
	var events []pendingEvent
	for rows.Next() {
		var e pendingEvent
		if err := rows.Scan(&e.id, &e.eventType, &e.payload); err == nil {
			events = append(events, e)
		}
	}

	dispatched := 0
	for _, e := range events {
		if broadcast != nil {
			broadcast(e.eventType, e.payload)
		}
		_, _ = s.store.DB().ExecContext(ctx, `UPDATE outbox_events SET status = 'published', published_at = ? WHERE id = ?`, time.Now(), e.id)
		dispatched++
	}

	return dispatched, nil
}
