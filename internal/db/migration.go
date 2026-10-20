package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

type Migration struct {
	Version int
	Name    string
	SQL     string
}

// Migrations defines the ordered database schema versions
var Migrations = []Migration{
	{
		Version: 1,
		Name:    "001_initial_schema",
		SQL: `
		CREATE TABLE IF NOT EXISTS organizations (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		`,
	},
	{
		Version: 2,
		Name:    "002_sequences_and_outbox",
		SQL: `
		CREATE TABLE IF NOT EXISTS sequences (
			org_id TEXT NOT NULL REFERENCES organizations(id),
			sequence_type TEXT NOT NULL,
			year INTEGER NOT NULL,
			current_value INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (org_id, sequence_type, year)
		);

		CREATE TABLE IF NOT EXISTS outbox_events (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			event_type TEXT NOT NULL,
			payload_json TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			retry_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			published_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox_events(status, created_at);
		`,
	},
	{
		Version: 3,
		Name:    "003_double_entry_general_ledger",
		SQL: `
		CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			code TEXT NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL, -- asset, liability, equity, revenue, expense
			normal_balance TEXT NOT NULL, -- debit, credit
			is_active BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, code)
		);

		CREATE TABLE IF NOT EXISTS journal_entries (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			entry_number TEXT NOT NULL,
			source_type TEXT NOT NULL, -- invoice, payment, purchase_order, manual
			source_id TEXT,
			memo TEXT,
			status TEXT NOT NULL DEFAULT 'posted', -- posted, reversed
			posted_at DATETIME NOT NULL,
			created_by TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, entry_number)
		);

		CREATE TABLE IF NOT EXISTS journal_lines (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			entry_id TEXT NOT NULL REFERENCES journal_entries(id),
			account_id TEXT NOT NULL REFERENCES accounts(id),
			entry_type TEXT NOT NULL, -- debit, credit
			amount_cents INTEGER NOT NULL,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_accounts_org ON accounts(org_id);
		CREATE INDEX IF NOT EXISTS idx_journal_entries_org ON journal_entries(org_id, posted_at);
		CREATE INDEX IF NOT EXISTS idx_journal_lines_entry ON journal_lines(entry_id);
		`,
	},
}

// RunMigrations applies all pending migrations in sequential order
func RunMigrations(db *sql.DB) error {
	// 1. Ensure migrations tracking table exists
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("failed to initialize schema_migrations: %w", err)
	}

	for _, m := range Migrations {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)`, m.Version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration status for version %d: %w", m.Version, err)
		}
		if exists {
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to start migration transaction: %w", err)
		}

		if _, err := tx.Exec(m.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed executing migration %d (%s): %w", m.Version, m.Name, err)
		}

		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
			m.Version, m.Name, time.Now()); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed recording migration %d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed committing migration %d: %w", m.Version, err)
		}
		log.Printf("Applied schema migration %03d: %s", m.Version, m.Name)
	}

	return nil
}
