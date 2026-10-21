package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"aosphere/internal/db"
	"aosphere/internal/store"
)

func TestOutboxWorker_SuccessAndDeadLetter(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_outbox.db")
	database, err := db.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	appStore := store.NewStore(database)
	svc := NewWorkflowService(appStore)

	ctx := context.Background()

	// 1. Insert 2 events: one will succeed, one will fail into dead-letter
	tx, err := database.Begin()
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	if err := RecordOutboxEvent(ctx, tx, "org_default", "evt_good", map[string]string{"msg": "hello"}); err != nil {
		t.Fatalf("RecordOutboxEvent failed: %v", err)
	}
	if err := RecordOutboxEvent(ctx, tx, "org_default", "evt_bad", map[string]string{"msg": "fail"}); err != nil {
		t.Fatalf("RecordOutboxEvent failed: %v", err)
	}
	_ = tx.Commit()

	cfg := OutboxWorkerConfig{
		PollInterval: 50 * time.Millisecond,
		MaxRetries:   2,
		BatchSize:    10,
	}

	handler := func(eventType, payload string) error {
		if eventType == "evt_bad" {
			return errors.New("simulated webhook timeout")
		}
		return nil
	}

	// First pass: good succeeds, bad gets retry_count = 1
	n, err := svc.ProcessOutboxBatch(ctx, cfg, handler)
	if err != nil {
		t.Fatalf("ProcessOutboxBatch failed: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 dispatched event, got %d", n)
	}

	// Verify good is published
	var goodStatus string
	_ = database.QueryRow("SELECT status FROM outbox_events WHERE event_type = 'evt_good'").Scan(&goodStatus)
	if goodStatus != "published" {
		t.Errorf("expected good status 'published', got %s", goodStatus)
	}

	// Second pass: bad reaches maxRetries (2) -> moved to dead_letter
	_, _ = svc.ProcessOutboxBatch(ctx, cfg, handler)

	var badStatus string
	_ = database.QueryRow("SELECT status FROM outbox_events WHERE event_type = 'evt_bad'").Scan(&badStatus)
	if badStatus != "dead_letter" {
		t.Errorf("expected bad status 'dead_letter', got %s", badStatus)
	}
}
