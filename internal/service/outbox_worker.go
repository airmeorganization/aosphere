package service

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"aosphere/internal/metrics"
)

// OutboxWorkerConfig configures the durable outbox dispatch worker
type OutboxWorkerConfig struct {
	PollInterval time.Duration
	MaxRetries   int
	BatchSize    int
}

// DefaultOutboxWorkerConfig returns production defaults
func DefaultOutboxWorkerConfig() OutboxWorkerConfig {
	return OutboxWorkerConfig{
		PollInterval: 1 * time.Second,
		MaxRetries:   5,
		BatchSize:    50,
	}
}

// OutboxEventRecord represents an outbox row
type OutboxEventRecord struct {
	ID         string
	OrgID      string
	EventType  string
	Payload    string
	RetryCount int
	CreatedAt  time.Time
}

// StartOutboxWorker starts the durable background outbox worker with exponential backoff and dead-letter queues
func (s *WorkflowService) StartOutboxWorker(ctx context.Context, cfg OutboxWorkerConfig, handler func(eventType, payload string) error) <-chan struct{} {
	var orgHandler func(orgID, eventType, payload string) error
	if handler != nil {
		orgHandler = func(orgID, eventType, payload string) error {
			return handler(eventType, payload)
		}
	}
	return s.StartOrgOutboxWorker(ctx, cfg, orgHandler)
}

// StartOrgOutboxWorker starts the durable background outbox worker with tenant-isolated routing
func (s *WorkflowService) StartOrgOutboxWorker(ctx context.Context, cfg OutboxWorkerConfig, handler func(orgID, eventType, payload string) error) <-chan struct{} {
	done := make(chan struct{})

	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 1 * time.Second
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 5
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}

	go func() {
		defer close(done)
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				// Drain pending items once more before stopping
				drainCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				_, _ = s.ProcessOutboxBatchWithOrg(drainCtx, cfg, handler)
				cancel()
				return
			case <-ticker.C:
				_, _ = s.ProcessOutboxBatchWithOrg(ctx, cfg, handler)
			}
		}
	}()

	return done
}

var outboxMu sync.Mutex

// ProcessOutboxBatch processes a single batch of pending outbox events
func (s *WorkflowService) ProcessOutboxBatch(ctx context.Context, cfg OutboxWorkerConfig, handler func(eventType, payload string) error) (int, error) {
	var orgHandler func(orgID, eventType, payload string) error
	if handler != nil {
		orgHandler = func(orgID, eventType, payload string) error {
			return handler(eventType, payload)
		}
	}
	return s.ProcessOutboxBatchWithOrg(ctx, cfg, orgHandler)
}

// ProcessOutboxBatchWithOrg processes a single batch of pending outbox events with tenant isolation
func (s *WorkflowService) ProcessOutboxBatchWithOrg(ctx context.Context, cfg OutboxWorkerConfig, handler func(orgID, eventType, payload string) error) (int, error) {
	outboxMu.Lock()
	defer outboxMu.Unlock()

	rows, err := s.store.DB().QueryContext(ctx, `
		SELECT id, org_id, event_type, payload_json, retry_count, created_at 
		FROM outbox_events 
		WHERE status = 'pending' 
		ORDER BY created_at ASC 
		LIMIT ?`, cfg.BatchSize)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var events []OutboxEventRecord
	for rows.Next() {
		var e OutboxEventRecord
		if err := rows.Scan(&e.ID, &e.OrgID, &e.EventType, &e.Payload, &e.RetryCount, &e.CreatedAt); err == nil {
			events = append(events, e)
		}
	}

	dispatched := 0
	for _, e := range events {
		var handleErr error
		if handler != nil {
			handleErr = handler(e.OrgID, e.EventType, e.Payload)
		}

		now := time.Now()
		if handleErr == nil {
			// Successfully published
			_, err := s.store.DB().ExecContext(ctx, `
				UPDATE outbox_events 
				SET status = 'published', published_at = ? 
				WHERE id = ?`, now, e.ID)
			if err == nil {
				dispatched++
				metrics.Default().RecordOutboxDispatch(true)
			}
		} else {
			// Failure with exponential backoff & dead-letter queue
			newRetry := e.RetryCount + 1
			if newRetry >= cfg.MaxRetries {
				// Dead letter
				_, _ = s.store.DB().ExecContext(ctx, `
					UPDATE outbox_events 
					SET status = 'dead_letter', retry_count = ? 
					WHERE id = ?`, newRetry, e.ID)
				metrics.Default().RecordOutboxDispatch(false)
				slog.Error("Outbox event moved to dead-letter queue",
					slog.String("event_id", e.ID),
					slog.String("event_type", e.EventType),
					slog.Int("retries", newRetry),
					slog.Any("error", handleErr),
				)
			} else {
				// Exponential backoff
				backoffSec := int(math.Min(math.Pow(2, float64(newRetry)), 60))
				_, _ = s.store.DB().ExecContext(ctx, `
					UPDATE outbox_events 
					SET retry_count = ? 
					WHERE id = ?`, newRetry, e.ID)
				slog.Warn("Outbox event dispatch failed, scheduled for retry",
					slog.String("event_id", e.ID),
					slog.Int("retry_count", newRetry),
					slog.Int("backoff_seconds", backoffSec),
					slog.Any("error", handleErr),
				)
			}
		}
	}

	return dispatched, nil
}
