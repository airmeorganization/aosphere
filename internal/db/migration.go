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
	{
		Version: 4,
		Name:    "004_agent_executions_and_settings",
		SQL: `
		CREATE TABLE IF NOT EXISTS agent_executions (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			agent_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			category TEXT NOT NULL, -- read_only, reversible_mutation, high_impact_mutation
			arguments_json TEXT NOT NULL,
			result_json TEXT NOT NULL,
			is_error BOOLEAN NOT NULL DEFAULT 0,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_agent_exec_org ON agent_executions(org_id, created_at);
		CREATE INDEX IF NOT EXISTS idx_agent_exec_agent ON agent_executions(agent_id, created_at);

		CREATE TABLE IF NOT EXISTS system_settings (
			org_id TEXT NOT NULL REFERENCES organizations(id),
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (org_id, key)
		);
		`,
	},
	{
		Version: 5,
		Name:    "005_business_lifecycles",
		SQL: `
		CREATE TABLE IF NOT EXISTS company_profiles (
			org_id TEXT PRIMARY KEY REFERENCES organizations(id),
			company_name TEXT NOT NULL,
			currency TEXT NOT NULL DEFAULT 'USD',
			timezone TEXT NOT NULL DEFAULT 'UTC',
			tax_rate REAL NOT NULL DEFAULT 0.10,
			address TEXT NOT NULL DEFAULT '',
			phone TEXT NOT NULL DEFAULT '',
			website TEXT NOT NULL DEFAULT '',
			onboarding_completed BOOLEAN NOT NULL DEFAULT 1,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS notifications (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			user_id TEXT,
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT 'system', -- alert, approval, inventory, finance
			is_read BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_notifications_org ON notifications(org_id, is_read, created_at);

		CREATE TABLE IF NOT EXISTS quotes (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			quote_number TEXT NOT NULL,
			deal_id TEXT REFERENCES deals(id),
			customer_id TEXT REFERENCES customers(id),
			subtotal REAL NOT NULL DEFAULT 0.0,
			tax_amount REAL NOT NULL DEFAULT 0.0,
			total_amount REAL NOT NULL DEFAULT 0.0,
			status TEXT NOT NULL DEFAULT 'draft', -- draft, sent, accepted, rejected, converted
			items_json TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, quote_number)
		);

		INSERT OR IGNORE INTO company_profiles (org_id, company_name, currency, timezone, tax_rate, address, phone, website, onboarding_completed)
		SELECT id, 'AO Sphere Enterprise', 'USD', 'America/New_York', 0.10, '100 Innovation Way, Suite 400', '+1 (555) 019-2834', 'https://aosphere.local', 1
		FROM organizations WHERE id = 'org_default';
		`,
	},
	{
		Version: 6,
		Name:    "006_warehouses_and_grn",
		SQL: `
		CREATE TABLE IF NOT EXISTS warehouses (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			address TEXT NOT NULL DEFAULT '',
			is_active BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, code)
		);

		CREATE TABLE IF NOT EXISTS warehouse_stocks (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			warehouse_id TEXT NOT NULL REFERENCES warehouses(id),
			sku_id TEXT NOT NULL REFERENCES skus(id),
			stock_quantity INTEGER NOT NULL DEFAULT 0,
			reserved_quantity INTEGER NOT NULL DEFAULT 0,
			bin_location TEXT NOT NULL DEFAULT 'A-01-01',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, warehouse_id, sku_id)
		);

		CREATE TABLE IF NOT EXISTS goods_receipt_notes (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			grn_number TEXT NOT NULL,
			po_id TEXT NOT NULL REFERENCES purchase_orders(id),
			warehouse_id TEXT NOT NULL REFERENCES warehouses(id),
			received_by TEXT NOT NULL DEFAULT 'warehouse_staff',
			status TEXT NOT NULL DEFAULT 'completed',
			items_json TEXT NOT NULL DEFAULT '[]',
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, grn_number)
		);

		CREATE TABLE IF NOT EXISTS supplier_bills (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			bill_number TEXT NOT NULL,
			supplier_id TEXT NOT NULL REFERENCES suppliers(id),
			po_id TEXT REFERENCES purchase_orders(id),
			amount REAL NOT NULL DEFAULT 0.0,
			status TEXT NOT NULL DEFAULT 'open',
			match_status TEXT NOT NULL DEFAULT 'pending',
			items_json TEXT NOT NULL DEFAULT '[]',
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, bill_number)
		);

		CREATE TABLE IF NOT EXISTS stock_transfers (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			transfer_number TEXT NOT NULL,
			from_warehouse_id TEXT NOT NULL REFERENCES warehouses(id),
			to_warehouse_id TEXT NOT NULL REFERENCES warehouses(id),
			status TEXT NOT NULL DEFAULT 'completed',
			items_json TEXT NOT NULL DEFAULT '[]',
			notes TEXT,
			created_by TEXT NOT NULL DEFAULT 'system',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, transfer_number)
		);

		CREATE TABLE IF NOT EXISTS batch_lots (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			sku_id TEXT NOT NULL REFERENCES skus(id),
			batch_number TEXT NOT NULL,
			expiry_date TEXT,
			quantity INTEGER NOT NULL DEFAULT 0,
			warehouse_id TEXT REFERENCES warehouses(id),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, sku_id, batch_number)
		);

		CREATE INDEX IF NOT EXISTS idx_warehouses_org ON warehouses(org_id);
		CREATE INDEX IF NOT EXISTS idx_warehouse_stocks_org ON warehouse_stocks(org_id, warehouse_id);
		CREATE INDEX IF NOT EXISTS idx_grn_org ON goods_receipt_notes(org_id, po_id);
		CREATE INDEX IF NOT EXISTS idx_supplier_bills_org ON supplier_bills(org_id, po_id);
		CREATE INDEX IF NOT EXISTS idx_transfers_org ON stock_transfers(org_id);
		CREATE INDEX IF NOT EXISTS idx_batches_org ON batch_lots(org_id, sku_id);

		INSERT OR IGNORE INTO warehouses (id, org_id, name, code, address, is_active)
		SELECT 'wh_main_' || id, id, 'Main Logistics Hub', 'WH-MAIN', '100 Innovation Way, Logistics Gate 1', 1
		FROM organizations;

		INSERT OR IGNORE INTO warehouses (id, org_id, name, code, address, is_active)
		SELECT 'wh_east_' || id, id, 'East Coast Fulfillment Center', 'WH-EAST', '450 Harbor Boulevard, Dock 12', 1
		FROM organizations;
		`,
	},
	{
		Version: 7,
		Name:    "007_phase4_finances_and_currencies",
		SQL: `
		CREATE TABLE IF NOT EXISTS credit_notes (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			credit_note_number TEXT NOT NULL,
			invoice_id TEXT REFERENCES invoices(id),
			customer_id TEXT NOT NULL REFERENCES customers(id),
			currency TEXT NOT NULL DEFAULT 'USD',
			subtotal REAL NOT NULL DEFAULT 0.0,
			tax_amount REAL NOT NULL DEFAULT 0.0,
			total_amount REAL NOT NULL DEFAULT 0.0,
			total_cents INTEGER NOT NULL DEFAULT 0,
			reason TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'issued',
			items_json TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, credit_note_number)
		);

		CREATE TABLE IF NOT EXISTS exchange_rates (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			base_currency TEXT NOT NULL DEFAULT 'USD',
			target_currency TEXT NOT NULL,
			rate REAL NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, base_currency, target_currency)
		);

		CREATE INDEX IF NOT EXISTS idx_credit_notes_org ON credit_notes(org_id, customer_id);
		CREATE INDEX IF NOT EXISTS idx_exchange_rates_org ON exchange_rates(org_id, target_currency);

		-- Seed standard accounts if not existing
		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_1000_' || id, id, '1000', 'Operating Cash / Bank', 'asset', 'debit', 1 FROM organizations;

		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_1100_' || id, id, '1100', 'Accounts Receivable (AR)', 'asset', 'debit', 1 FROM organizations;

		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_1200_' || id, id, '1200', 'Inventory Asset', 'asset', 'debit', 1 FROM organizations;

		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_2000_' || id, id, '2000', 'Accounts Payable (AP)', 'liability', 'credit', 1 FROM organizations;

		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_2100_' || id, id, '2100', 'Sales Tax Payable', 'liability', 'credit', 1 FROM organizations;

		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_2110_' || id, id, '2110', 'Goods Received Not Billed (GRNB)', 'liability', 'credit', 1 FROM organizations;

		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_3000_' || id, id, '3000', 'Retained Earnings', 'equity', 'credit', 1 FROM organizations;

		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_4000_' || id, id, '4000', 'Sales Revenue', 'revenue', 'credit', 1 FROM organizations;

		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_5000_' || id, id, '5000', 'Cost of Goods Sold (COGS)', 'expense', 'debit', 1 FROM organizations;

		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_5100_' || id, id, '5100', 'Operating Expenses', 'expense', 'debit', 1 FROM organizations;

		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_7000_' || id, id, '7000', 'Realized FX Gain/Loss', 'revenue', 'credit', 1 FROM organizations;

		-- Seed base exchange rates for Asian and Global currencies
		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_INR_' || id, id, 'USD', 'INR', 83.50 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_JPY_' || id, id, 'USD', 'JPY', 153.20 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_CNY_' || id, id, 'USD', 'CNY', 7.23 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_SGD_' || id, id, 'USD', 'SGD', 1.32 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_AED_' || id, id, 'USD', 'AED', 3.67 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_SAR_' || id, id, 'USD', 'SAR', 3.75 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_HKD_' || id, id, 'USD', 'HKD', 7.78 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_KRW_' || id, id, 'USD', 'KRW', 1380.0 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_THB_' || id, id, 'USD', 'THB', 36.80 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_IDR_' || id, id, 'USD', 'IDR', 15850.0 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_MYR_' || id, id, 'USD', 'MYR', 4.45 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_PHP_' || id, id, 'USD', 'PHP', 58.20 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_VND_' || id, id, 'USD', 'VND', 25400.0 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_BDT_' || id, id, 'USD', 'BDT', 119.50 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_PKR_' || id, id, 'USD', 'PKR', 278.50 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_LKR_' || id, id, 'USD', 'LKR', 293.0 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_EUR_' || id, id, 'USD', 'EUR', 0.92 FROM organizations;

		INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate)
		SELECT 'fx_GBP_' || id, id, 'USD', 'GBP', 0.77 FROM organizations;
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
