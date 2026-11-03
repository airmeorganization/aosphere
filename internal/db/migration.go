package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
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
	{
		Version: 8,
		Name:    "008_phase5_crm_customer360",
		SQL: `
		CREATE TABLE IF NOT EXISTS pipelines (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			is_default BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, code)
		);

		CREATE TABLE IF NOT EXISTS pipeline_stages (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			pipeline_id TEXT NOT NULL REFERENCES pipelines(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			code TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			probability_pct INTEGER NOT NULL DEFAULT 50,
			decay_days_threshold INTEGER NOT NULL DEFAULT 14,
			color TEXT NOT NULL DEFAULT '#3b82f6',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(pipeline_id, code)
		);

		CREATE TABLE IF NOT EXISTS customer_activities (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			customer_id TEXT NOT NULL REFERENCES customers(id),
			deal_id TEXT REFERENCES deals(id),
			activity_type TEXT NOT NULL, -- note, call, meeting, email
			author_type TEXT NOT NULL DEFAULT 'human', -- human, agent
			author_id TEXT NOT NULL DEFAULT 'admin',
			author_name TEXT NOT NULL DEFAULT 'Operations Team',
			summary TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			metadata_json TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS inbound_leads (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			company TEXT NOT NULL DEFAULT '',
			phone TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT 'website_form',
			status TEXT NOT NULL DEFAULT 'new', -- new, qualified, contacted, converted, rejected
			assigned_to TEXT NOT NULL DEFAULT 'unassigned',
			converted_customer_id TEXT REFERENCES customers(id),
			converted_deal_id TEXT REFERENCES deals(id),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS email_outbox (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			recipient_email TEXT NOT NULL,
			recipient_name TEXT NOT NULL DEFAULT '',
			subject TEXT NOT NULL,
			body_html TEXT NOT NULL,
			body_text TEXT NOT NULL DEFAULT '',
			template_name TEXT NOT NULL DEFAULT 'custom',
			related_entity_type TEXT NOT NULL DEFAULT 'invoice', -- invoice, quote, receipt
			related_entity_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'sent', -- queued, sent, failed
			error_message TEXT NOT NULL DEFAULT '',
			sent_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS invoice_portal_tokens (
			token TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			invoice_id TEXT NOT NULL REFERENCES invoices(id),
			expires_at DATETIME,
			view_count INTEGER NOT NULL DEFAULT 0,
			last_viewed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, invoice_id)
		);

		CREATE INDEX IF NOT EXISTS idx_pipelines_org ON pipelines(org_id);
		CREATE INDEX IF NOT EXISTS idx_pipeline_stages_pipe ON pipeline_stages(pipeline_id, position);
		CREATE INDEX IF NOT EXISTS idx_customer_activities_cust ON customer_activities(org_id, customer_id, created_at);
		CREATE INDEX IF NOT EXISTS idx_inbound_leads_org ON inbound_leads(org_id, status);
		CREATE INDEX IF NOT EXISTS idx_email_outbox_org ON email_outbox(org_id, status);
		CREATE INDEX IF NOT EXISTS idx_invoice_portal_inv ON invoice_portal_tokens(invoice_id);

		ALTER TABLE deals ADD COLUMN pipeline_id TEXT DEFAULT 'pipe_std_org_default';
		ALTER TABLE deals ADD COLUMN stage_updated_at DATETIME DEFAULT '';
		UPDATE deals SET stage_updated_at = created_at WHERE stage_updated_at IS NULL OR stage_updated_at = '';

		-- Seed Default Pipelines
		INSERT OR IGNORE INTO pipelines (id, org_id, name, code, description, is_default)
		SELECT 'pipe_std_' || id, id, 'Standard Sales', 'STANDARD', 'Standard B2B sales pipeline for qualified direct opportunities', 1 FROM organizations;

		INSERT OR IGNORE INTO pipelines (id, org_id, name, code, description, is_default)
		SELECT 'pipe_ent_' || id, id, 'Enterprise Solutions', 'ENTERPRISE', 'High-ACV multi-stakeholder enterprise solution engagements', 0 FROM organizations;

		INSERT OR IGNORE INTO pipelines (id, org_id, name, code, description, is_default)
		SELECT 'pipe_saas_' || id, id, 'SaaS Subscriptions', 'SAAS', 'Self-serve trial onboarding and recurring expansion pipeline', 0 FROM organizations;

		-- Seed Stages for Standard Sales
		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_std_lead_' || id, id, 'pipe_std_' || id, 'New Leads', 'lead', 1, 10, 7, '#64748b' FROM organizations;

		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_std_qual_' || id, id, 'pipe_std_' || id, 'Qualified', 'qualified', 2, 25, 10, '#3b82f6' FROM organizations;

		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_std_prop_' || id, id, 'pipe_std_' || id, 'Proposal Sent', 'proposal', 3, 60, 14, '#8b5cf6' FROM organizations;

		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_std_nego_' || id, id, 'pipe_std_' || id, 'Negotiation', 'negotiation', 4, 80, 7, '#f59e0b' FROM organizations;

		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_std_won_' || id, id, 'pipe_std_' || id, 'Won', 'won', 5, 100, 0, '#10b981' FROM organizations;

		-- Seed Stages for Enterprise Solutions
		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_ent_disc_' || id, id, 'pipe_ent_' || id, 'Discovery', 'discovery', 1, 15, 14, '#64748b' FROM organizations;

		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_ent_tech_' || id, id, 'pipe_ent_' || id, 'Tech Validation', 'tech_eval', 2, 40, 21, '#0284c7' FROM organizations;

		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_ent_exec_' || id, id, 'pipe_ent_' || id, 'Executive Review', 'exec_review', 3, 70, 14, '#7c3aed' FROM organizations;

		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_ent_proc_' || id, id, 'pipe_ent_' || id, 'Legal & Procurement', 'procurement', 4, 90, 30, '#d97706' FROM organizations;

		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_ent_won_' || id, id, 'pipe_ent_' || id, 'Closed Won', 'won', 5, 100, 0, '#059669' FROM organizations;

		-- Seed Stages for SaaS Subscriptions
		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_saas_trial_' || id, id, 'pipe_saas_' || id, 'Trial Active', 'trial', 1, 20, 7, '#0ea5e9' FROM organizations;

		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_saas_onb_' || id, id, 'pipe_saas_' || id, 'Onboarding', 'onboarding', 2, 50, 5, '#6366f1' FROM organizations;

		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_saas_exp_' || id, id, 'pipe_saas_' || id, 'Expansion', 'expansion', 3, 80, 10, '#ec4899' FROM organizations;

		INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color)
		SELECT 'stg_saas_won_' || id, id, 'pipe_saas_' || id, 'Active ARR', 'won', 4, 100, 0, '#10b981' FROM organizations;
		`,
	},
	{
		Version: 9,
		Name:    "009_phase6_hrms_payroll_leaves",
		SQL: `
		-- 1. Enhance Employees schema with compensation, direct deposit, tax ID & emergency contacts
		ALTER TABLE employees ADD COLUMN job_title TEXT DEFAULT '';
		ALTER TABLE employees ADD COLUMN reporting_manager_id TEXT DEFAULT '';
		ALTER TABLE employees ADD COLUMN employment_type TEXT DEFAULT 'full_time';
		ALTER TABLE employees ADD COLUMN salary_type TEXT DEFAULT 'hourly';
		ALTER TABLE employees ADD COLUMN monthly_salary REAL DEFAULT 0.0;
		ALTER TABLE employees ADD COLUMN tax_id TEXT DEFAULT '';
		ALTER TABLE employees ADD COLUMN bank_name TEXT DEFAULT '';
		ALTER TABLE employees ADD COLUMN bank_account_number TEXT DEFAULT '';
		ALTER TABLE employees ADD COLUMN bank_routing_number TEXT DEFAULT '';
		ALTER TABLE employees ADD COLUMN emergency_contact_name TEXT DEFAULT '';
		ALTER TABLE employees ADD COLUMN emergency_contact_phone TEXT DEFAULT '';

		-- Update existing seed employees with realistic compensation & compliance metadata
		UPDATE employees SET job_title = role WHERE job_title IS NULL OR job_title = '';
		UPDATE employees SET salary_type = 'hourly', monthly_salary = hourly_rate * 160 WHERE (monthly_salary IS NULL OR monthly_salary = 0.0);
		UPDATE employees SET tax_id = 'TAX-US-' || substr(id, 5, 8), bank_name = 'Silicon Valley National Bank', bank_account_number = '8829103948', bank_routing_number = '121000358' WHERE tax_id IS NULL OR tax_id = '';
		UPDATE employees SET emergency_contact_name = 'Family Contact', emergency_contact_phone = '+1 (555) 019-2831' WHERE emergency_contact_name IS NULL OR emergency_contact_name = '';

		-- 2. Time & Attendance (Digital Timeclock Punch Events)
		CREATE TABLE IF NOT EXISTS timeclock_events (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			employee_id TEXT NOT NULL REFERENCES employees(id),
			shift_id TEXT REFERENCES shifts(id),
			event_type TEXT NOT NULL, -- clock_in, clock_out, break_start, break_end
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			ip_address TEXT NOT NULL DEFAULT '',
			location_notes TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- 3. Payroll Runs & Itemized Payslips
		CREATE TABLE IF NOT EXISTS payroll_runs (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			period TEXT NOT NULL, -- YYYY-MM
			total_gross_cents INTEGER NOT NULL DEFAULT 0,
			total_tax_cents INTEGER NOT NULL DEFAULT 0,
			total_deductions_cents INTEGER NOT NULL DEFAULT 0,
			total_net_cents INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'draft', -- draft, approved, paid
			payment_date TEXT NOT NULL DEFAULT '',
			journal_entry_id TEXT REFERENCES journal_entries(id),
			payslips_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, period)
		);

		CREATE TABLE IF NOT EXISTS payslips (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			payroll_run_id TEXT REFERENCES payroll_runs(id) ON DELETE CASCADE,
			employee_id TEXT NOT NULL REFERENCES employees(id),
			period TEXT NOT NULL,
			salary_type TEXT NOT NULL DEFAULT 'hourly',
			hourly_rate REAL NOT NULL DEFAULT 0.0,
			monthly_salary REAL NOT NULL DEFAULT 0.0,
			regular_hours REAL NOT NULL DEFAULT 0.0,
			overtime_hours REAL NOT NULL DEFAULT 0.0,
			gross_pay REAL NOT NULL DEFAULT 0.0,
			tax_withholding REAL NOT NULL DEFAULT 0.0,
			social_contributions REAL NOT NULL DEFAULT 0.0,
			benefits_deductions REAL NOT NULL DEFAULT 0.0,
			net_pay REAL NOT NULL DEFAULT 0.0,
			status TEXT NOT NULL DEFAULT 'generated', -- generated, paid
			payment_date TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, payroll_run_id, employee_id)
		);

		-- 4. Leave Policies & PTO Management
		CREATE TABLE IF NOT EXISTS leave_policies (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			name TEXT NOT NULL,
			code TEXT NOT NULL, -- vacation, sick, parental, unpaid
			annual_days INTEGER NOT NULL DEFAULT 15,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, code)
		);

		CREATE TABLE IF NOT EXISTS leave_requests (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			employee_id TEXT NOT NULL REFERENCES employees(id),
			leave_type TEXT NOT NULL,
			start_date TEXT NOT NULL,
			end_date TEXT NOT NULL,
			days_count INTEGER NOT NULL DEFAULT 1,
			reason TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending', -- pending, approved, rejected
			reviewed_by TEXT NOT NULL DEFAULT '',
			review_notes TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- 5. HR Document Vault & Compliance
		CREATE TABLE IF NOT EXISTS hr_documents (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			employee_id TEXT NOT NULL REFERENCES employees(id),
			title TEXT NOT NULL,
			document_type TEXT NOT NULL, -- contract, nda, id_proof, evaluation, tax_form
			file_name TEXT NOT NULL,
			file_content TEXT NOT NULL,
			file_size INTEGER NOT NULL DEFAULT 0,
			uploaded_by TEXT NOT NULL DEFAULT 'HR Operations',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- 6. Indexes
		CREATE INDEX IF NOT EXISTS idx_timeclock_emp ON timeclock_events(org_id, employee_id, timestamp);
		CREATE INDEX IF NOT EXISTS idx_payroll_runs_period ON payroll_runs(org_id, period);
		CREATE INDEX IF NOT EXISTS idx_payslips_emp ON payslips(org_id, employee_id, period);
		CREATE INDEX IF NOT EXISTS idx_leave_requests_emp ON leave_requests(org_id, employee_id, status);
		CREATE INDEX IF NOT EXISTS idx_hr_documents_emp ON hr_documents(org_id, employee_id, document_type);

		-- 7. Chart of Accounts: Seed Standard Payroll Accounts
		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_5200_' || id, id, '5200', 'Salaries & Wages Expense', 'expense', 'debit', 1 FROM organizations;

		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_2150_' || id, id, '2150', 'Payroll Withholding Taxes & Deductions', 'liability', 'credit', 1 FROM organizations;

		INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active)
		SELECT 'acc_2200_' || id, id, '2200', 'Payroll Clearing & Accrued Wages', 'liability', 'credit', 1 FROM organizations;

		-- 8. Seed Default Leave Policies
		INSERT OR IGNORE INTO leave_policies (id, org_id, name, code, annual_days)
		SELECT 'lp_vac_' || id, id, 'Paid Vacation Time', 'vacation', 15 FROM organizations;

		INSERT OR IGNORE INTO leave_policies (id, org_id, name, code, annual_days)
		SELECT 'lp_sick_' || id, id, 'Sick & Medical Leave', 'sick', 10 FROM organizations;

		INSERT OR IGNORE INTO leave_policies (id, org_id, name, code, annual_days)
		SELECT 'lp_par_' || id, id, 'Parental & Family Leave', 'parental', 30 FROM organizations;

		INSERT OR IGNORE INTO leave_policies (id, org_id, name, code, annual_days)
		SELECT 'lp_unp_' || id, id, 'Unpaid Leave of Absence', 'unpaid', 0 FROM organizations;
		`,
	},
	{
		Version: 10,
		Name:    "010_phase7_financial_analytics_and_exports",
		SQL: `
		CREATE TABLE IF NOT EXISTS saved_reports (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			name TEXT NOT NULL,
			report_type TEXT NOT NULL, -- pnl, balance_sheet, cash_flow, aging_ar, aging_ap, inventory_valuation, sales_velocity
			parameters_json TEXT NOT NULL DEFAULT '{}',
			created_by TEXT NOT NULL DEFAULT 'system',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS data_exports (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			export_type TEXT NOT NULL, -- pnl, balance_sheet, invoices, purchase_orders, employees, inventory
			file_format TEXT NOT NULL, -- csv, pdf, json
			record_count INTEGER NOT NULL DEFAULT 0,
			file_size_bytes INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL DEFAULT 'system',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_saved_reports_org ON saved_reports(org_id, report_type);
		CREATE INDEX IF NOT EXISTS idx_data_exports_org ON data_exports(org_id, created_at);
		`,
	},
	{
		Version: 11,
		Name:    "011_phase8_autonomous_mcp_platform_and_webhooks",
		SQL: `
		CREATE TABLE IF NOT EXISTS webhook_subscriptions (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			secret TEXT NOT NULL,
			event_types TEXT NOT NULL DEFAULT '["*"]',
			is_active BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS webhook_deliveries (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			subscription_id TEXT NOT NULL REFERENCES webhook_subscriptions(id),
			event_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			payload_json TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending', -- pending, delivered, failed
			status_code INTEGER DEFAULT 0,
			response_body TEXT DEFAULT '',
			retry_count INTEGER NOT NULL DEFAULT 0,
			max_retries INTEGER NOT NULL DEFAULT 3,
			next_retry_at DATETIME,
			delivered_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS agent_configs (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			name TEXT NOT NULL,
			api_key_hash TEXT NOT NULL UNIQUE,
			tool_quota_per_minute INTEGER NOT NULL DEFAULT 60,
			daily_budget_cents INTEGER NOT NULL DEFAULT 5000,
			current_day_usage_cents INTEGER NOT NULL DEFAULT 0,
			concurrent_limit INTEGER NOT NULL DEFAULT 5,
			active_slots INTEGER NOT NULL DEFAULT 0,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			last_used_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS agent_executions (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			agent_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT 'read_only',
			arguments_json TEXT NOT NULL DEFAULT '{}',
			result_json TEXT NOT NULL DEFAULT '{}',
			is_error BOOLEAN NOT NULL DEFAULT 0,
			error_message TEXT DEFAULT '',
			duration_ms INTEGER NOT NULL DEFAULT 0,
			cost_cents INTEGER NOT NULL DEFAULT 0,
			mutations_json TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		ALTER TABLE agent_executions ADD COLUMN error_message TEXT DEFAULT '';
		ALTER TABLE agent_executions ADD COLUMN cost_cents INTEGER NOT NULL DEFAULT 0;
		ALTER TABLE agent_executions ADD COLUMN mutations_json TEXT NOT NULL DEFAULT '[]';

		CREATE TABLE IF NOT EXISTS agent_escalations (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			agent_id TEXT NOT NULL,
			execution_id TEXT,
			title TEXT NOT NULL,
			reason TEXT NOT NULL, -- ambiguous_input, negative_sentiment, high_risk_threshold, policy_violation
			status TEXT NOT NULL DEFAULT 'pending', -- pending, approved, rejected, resolved
			input_context_json TEXT NOT NULL DEFAULT '{}',
			recommended_action TEXT DEFAULT '',
			reviewer_id TEXT DEFAULT '',
			reviewer_notes TEXT DEFAULT '',
			resolved_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS agent_context_memory (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			category TEXT NOT NULL DEFAULT 'company_profile', -- company_profile, financial_policies, supplier_lead_times, tax_currency
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			description TEXT DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, key)
		);

		CREATE INDEX IF NOT EXISTS idx_webhook_sub_org ON webhook_subscriptions(org_id, is_active);
		CREATE INDEX IF NOT EXISTS idx_webhook_deliv_sub ON webhook_deliveries(subscription_id, status);
		CREATE INDEX IF NOT EXISTS idx_agent_configs_org ON agent_configs(org_id);
		CREATE INDEX IF NOT EXISTS idx_agent_exec_org ON agent_executions(org_id, created_at);
		CREATE INDEX IF NOT EXISTS idx_agent_escal_org ON agent_escalations(org_id, status);
		CREATE INDEX IF NOT EXISTS idx_agent_ctx_org ON agent_context_memory(org_id, category);

		-- Seed default agent config
		INSERT OR IGNORE INTO agent_configs (id, org_id, name, api_key_hash, tool_quota_per_minute, daily_budget_cents, concurrent_limit)
		SELECT 'cfg_claude_desktop_' || id, id, 'Claude Desktop Assistant', 'hash_claude_desktop_default', 60, 10000, 5 FROM organizations;

		INSERT OR IGNORE INTO agent_configs (id, org_id, name, api_key_hash, tool_quota_per_minute, daily_budget_cents, concurrent_limit)
		SELECT 'cfg_auto_procure_' || id, id, 'Autonomous Procurement Daemon', 'hash_auto_procure_default', 30, 5000, 3 FROM organizations;

		-- Seed default context memory
		INSERT OR IGNORE INTO agent_context_memory (id, org_id, category, key, value, description)
		SELECT 'ctx_comp_name_' || id, id, 'company_profile', 'company_legal_name', 'AO Sphere Global Inc.', 'Official registered entity name' FROM organizations;

		INSERT OR IGNORE INTO agent_context_memory (id, org_id, category, key, value, description)
		SELECT 'ctx_currency_' || id, id, 'tax_currency', 'active_currency', 'USD', 'Default operating currency' FROM organizations;

		INSERT OR IGNORE INTO agent_context_memory (id, org_id, category, key, value, description)
		SELECT 'ctx_tax_rate_' || id, id, 'tax_currency', 'standard_sales_tax_rate', '0.0825', 'Standard sales tax percentage (8.25%)' FROM organizations;

		INSERT OR IGNORE INTO agent_context_memory (id, org_id, category, key, value, description)
		SELECT 'ctx_po_limit_' || id, id, 'financial_policies', 'po_auto_approval_threshold_cents', '50000', 'Purchase orders above $500.00 require human manager signoff' FROM organizations;

		INSERT OR IGNORE INTO agent_context_memory (id, org_id, category, key, value, description)
		SELECT 'ctx_lead_time_' || id, id, 'supplier_lead_times', 'standard_reorder_lead_days', '7', 'Average supplier lead time for inventory reorders' FROM organizations;
		`,
	},
	{
		Version: 12,
		Name:    "012_phase9_enterprise_multitenancy_auth_compliance",
		SQL: `
		-- 1. Multi-Tenant Organization Associations
		CREATE TABLE IF NOT EXISTS user_organizations (
			user_id TEXT NOT NULL REFERENCES users(id),
			org_id TEXT NOT NULL REFERENCES organizations(id),
			role TEXT NOT NULL DEFAULT 'member',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(user_id, org_id)
		);

		-- Seed EMEA organization for multi-tenant switching
		INSERT OR IGNORE INTO organizations (id, name, slug)
		VALUES ('org_emea', 'AO Sphere EMEA Operations', 'emea');

		-- Associate existing users with their default org and seed EMEA membership for admin
		INSERT OR IGNORE INTO user_organizations (user_id, org_id, role)
		SELECT id, org_id, role FROM users;

		INSERT OR IGNORE INTO user_organizations (user_id, org_id, role)
		SELECT id, 'org_emea', 'admin' FROM users WHERE id IN ('usr_admin', 'usr_dev');

		-- 2. 2FA / TOTP and Session Activity Enhancements
		ALTER TABLE users ADD COLUMN totp_secret TEXT DEFAULT '';
		ALTER TABLE users ADD COLUMN totp_enabled BOOLEAN NOT NULL DEFAULT 0;
		ALTER TABLE users ADD COLUMN recovery_codes_json TEXT DEFAULT '[]';
		ALTER TABLE users ADD COLUMN last_active_at DATETIME;
		UPDATE users SET last_active_at = CURRENT_TIMESTAMP WHERE last_active_at IS NULL;

		-- 3. User Invitations & Password Resets
		CREATE TABLE IF NOT EXISTS user_invitations (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			email TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			token_hash TEXT NOT NULL UNIQUE,
			invited_by TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS password_resets (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			token_hash TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL DEFAULT 'pending',
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		-- 4. Enterprise SSO Configs (OIDC / SAML)
		CREATE TABLE IF NOT EXISTS sso_configs (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			provider TEXT NOT NULL,
			client_id TEXT NOT NULL DEFAULT '',
			client_secret TEXT NOT NULL DEFAULT '',
			issuer_url TEXT NOT NULL DEFAULT '',
			domain_mapping TEXT NOT NULL DEFAULT '',
			is_enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, provider)
		);

		-- 5. Granular Custom Role Builder
		CREATE TABLE IF NOT EXISTS custom_roles (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			permissions_json TEXT NOT NULL DEFAULT '[]',
			is_system BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(org_id, name)
		);

		-- Seed system roles
		INSERT OR IGNORE INTO custom_roles (id, org_id, name, description, permissions_json, is_system)
		SELECT 'role_admin_' || id, id, 'Administrator', 'Full system access and tenant administration', '["read","manage_inventory","manage_orders","approve_orders","manage_finance","manage_payroll","manage_hr","manage_users","admin"]', 1 FROM organizations;

		INSERT OR IGNORE INTO custom_roles (id, org_id, name, description, permissions_json, is_system)
		SELECT 'role_finance_' || id, id, 'Finance Director', 'Invoicing, general ledger, payroll and budget approval', '["read","manage_orders","approve_orders","manage_finance","manage_payroll"]', 1 FROM organizations;

		INSERT OR IGNORE INTO custom_roles (id, org_id, name, description, permissions_json, is_system)
		SELECT 'role_hr_' || id, id, 'HR Specialist', 'Employee lifecycle, shifts, timesheets and leave management', '["read","manage_payroll","manage_hr","manage_users"]', 1 FROM organizations;

		INSERT OR IGNORE INTO custom_roles (id, org_id, name, description, permissions_json, is_system)
		SELECT 'role_inventory_' || id, id, 'Inventory Specialist', 'SKU catalog, multi-warehouse stock adjustments, and GRN 3-way matching', '["read","manage_inventory","manage_orders"]', 1 FROM organizations;

		INSERT OR IGNORE INTO custom_roles (id, org_id, name, description, permissions_json, is_system)
		SELECT 'role_auditor_' || id, id, 'Compliance Auditor', 'Read-only access to audit trail, financial ledger, and SOC2 reviews', '["read"]', 1 FROM organizations;

		-- 6. Immutable Security Audit Vault with SHA-256 Chain Hashing
		CREATE TABLE IF NOT EXISTS audit_chain_vault (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			sequence_number INTEGER NOT NULL,
			actor_type TEXT NOT NULL DEFAULT 'user',
			actor_id TEXT NOT NULL,
			action TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			payload_hash TEXT NOT NULL,
			prev_block_hash TEXT NOT NULL,
			block_hash TEXT NOT NULL,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		-- 7. GDPR Privacy Requests (Export & Forget Me)
		CREATE TABLE IF NOT EXISTS privacy_requests (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			user_id TEXT NOT NULL REFERENCES users(id),
			request_type TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			result_json TEXT NOT NULL DEFAULT '{}',
			requested_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		);

		-- 8. SOC2 Access Reviews
		CREATE TABLE IF NOT EXISTS soc2_access_reviews (
			id TEXT PRIMARY KEY,
			org_id TEXT NOT NULL REFERENCES organizations(id),
			quarter TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			reviewed_by TEXT DEFAULT '',
			notes TEXT DEFAULT '',
			snapshot_json TEXT NOT NULL DEFAULT '{}',
			reviewed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_user_orgs ON user_organizations(user_id, org_id);
		CREATE INDEX IF NOT EXISTS idx_user_invites ON user_invitations(org_id, status);
		CREATE INDEX IF NOT EXISTS idx_pwd_resets ON password_resets(user_id, status);
		CREATE INDEX IF NOT EXISTS idx_audit_chain ON audit_chain_vault(org_id, sequence_number);
		CREATE INDEX IF NOT EXISTS idx_privacy_req ON privacy_requests(org_id, user_id);
		CREATE INDEX IF NOT EXISTS idx_soc2_rev ON soc2_access_reviews(org_id, quarter);
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

		statements := strings.Split(m.SQL, ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.Exec(stmt); err != nil {
				if strings.Contains(err.Error(), "duplicate column name") {
					continue
				}
				tx.Rollback()
				return fmt.Errorf("failed executing migration %d (%s) on stmt [%s]: %w", m.Version, m.Name, stmt, err)
			}
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
