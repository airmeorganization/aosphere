package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"aosphere/internal/auth"
	_ "modernc.org/sqlite"
)

// InitDB initializes SQLite with WAL mode, creates tables, and optionally seeds starter data.
func InitDB(dbPath string, seedData bool) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db dir: %w", err)
		}
	}

	// Open with pragmas for WAL mode, busy timeout, and foreign keys
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1) // Single writer connection prevents SQLITE_BUSY deadlock
	db.SetMaxIdleConns(1)

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	if err := RunMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	if seedData {
		if err := SeedStarterData(db); err != nil {
			log.Printf("Seed starter data check: %v", err)
		}
	}

	return db, nil
}

// Ping verifies that SQLite is responsive
func Ping(db *sql.DB) error {
	return db.Ping()
}

func createTables(db *sql.DB) error {
	schema := `
	-- 0. AUTHENTICATION, ORGANIZATIONS & SESSIONS
	CREATE TABLE IF NOT EXISTS organizations (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		slug TEXT UNIQUE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL REFERENCES organizations(id),
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		name TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'employee',
		is_active BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id),
		org_id TEXT NOT NULL REFERENCES organizations(id),
		token_hash TEXT UNIQUE NOT NULL,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_activity_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS api_keys (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL REFERENCES organizations(id),
		user_id TEXT REFERENCES users(id),
		name TEXT NOT NULL,
		key_hash TEXT UNIQUE NOT NULL,
		role TEXT NOT NULL DEFAULT 'operations',
		allowed_tools TEXT NOT NULL DEFAULT '*',
		is_active BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_users_org ON users(org_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token_hash);
	CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);

	-- 1. ERP & SUPPLY CHAIN
	CREATE TABLE IF NOT EXISTS suppliers (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		name TEXT NOT NULL,
		contact_email TEXT,
		phone TEXT,
		lead_time_days INTEGER DEFAULT 3,
		rating REAL DEFAULT 5.0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS skus (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		code TEXT NOT NULL,
		name TEXT NOT NULL,
		category TEXT NOT NULL,
		stock_quantity INTEGER NOT NULL DEFAULT 0,
		reserved_quantity INTEGER NOT NULL DEFAULT 0,
		min_threshold INTEGER NOT NULL DEFAULT 10,
		reorder_quantity INTEGER NOT NULL DEFAULT 50,
		unit_cost REAL NOT NULL DEFAULT 0.0,
		selling_price REAL NOT NULL DEFAULT 0.0,
		supplier_id TEXT REFERENCES suppliers(id),
		version INTEGER NOT NULL DEFAULT 1,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(org_id, code)
	);

	CREATE TABLE IF NOT EXISTS purchase_orders (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		po_number TEXT NOT NULL,
		supplier_id TEXT REFERENCES suppliers(id),
		status TEXT NOT NULL DEFAULT 'draft', -- draft, pending_approval, ordered, received, cancelled
		total_amount REAL NOT NULL DEFAULT 0.0,
		items_json TEXT NOT NULL DEFAULT '[]',
		created_by_type TEXT NOT NULL DEFAULT 'human', -- human, agent
		actor_id TEXT NOT NULL DEFAULT 'system',
		notes TEXT,
		version INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(org_id, po_number)
	);

	-- 2. CRM & SALES PIPELINE
	CREATE TABLE IF NOT EXISTS customers (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		name TEXT NOT NULL,
		company TEXT,
		email TEXT,
		phone TEXT,
		status TEXT DEFAULT 'active',
		lifetime_value REAL DEFAULT 0.0,
		tags TEXT DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS deals (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		title TEXT NOT NULL,
		customer_id TEXT REFERENCES customers(id),
		value REAL NOT NULL DEFAULT 0.0,
		stage TEXT NOT NULL DEFAULT 'lead', -- lead, qualified, proposal, negotiation, won, lost
		probability INTEGER DEFAULT 50,
		expected_close_date TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS communications (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		customer_id TEXT REFERENCES customers(id),
		channel TEXT NOT NULL, -- email, sms, note, call
		direction TEXT NOT NULL DEFAULT 'outbound', -- inbound, outbound
		subject TEXT,
		body TEXT NOT NULL,
		sent_by TEXT NOT NULL DEFAULT 'agent',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS orders (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		order_number TEXT NOT NULL,
		customer_id TEXT REFERENCES customers(id),
		total_amount REAL NOT NULL,
		status TEXT NOT NULL DEFAULT 'completed',
		items_json TEXT NOT NULL DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(org_id, order_number)
	);

	-- 3. FINANCIAL & HRMS
	CREATE TABLE IF NOT EXISTS invoices (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		invoice_number TEXT NOT NULL,
		customer_id TEXT REFERENCES customers(id),
		status TEXT NOT NULL DEFAULT 'draft', -- draft, sent, paid, overdue
		issue_date TEXT NOT NULL,
		due_date TEXT NOT NULL,
		subtotal REAL NOT NULL DEFAULT 0.0,
		tax_amount REAL NOT NULL DEFAULT 0.0,
		total_amount REAL NOT NULL DEFAULT 0.0,
		recurring_frequency TEXT DEFAULT 'none', -- none, monthly, quarterly, annual
		items_json TEXT NOT NULL DEFAULT '[]',
		version INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(org_id, invoice_number)
	);

	CREATE TABLE IF NOT EXISTS expenses (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		category TEXT NOT NULL,
		amount REAL NOT NULL,
		vendor TEXT NOT NULL,
		description TEXT,
		date TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS employees (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		name TEXT NOT NULL,
		role TEXT NOT NULL,
		department TEXT NOT NULL,
		hourly_rate REAL NOT NULL DEFAULT 25.0,
		status TEXT NOT NULL DEFAULT 'active',
		email TEXT,
		phone TEXT,
		hired_at TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS timesheets (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		employee_id TEXT REFERENCES employees(id),
		work_date TEXT NOT NULL,
		hours_worked REAL NOT NULL DEFAULT 8.0,
		overtime_hours REAL NOT NULL DEFAULT 0.0,
		status TEXT NOT NULL DEFAULT 'logged', -- logged, approved, paid
		hourly_rate REAL NOT NULL DEFAULT 25.0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 4. WORKPLACE PRODUCTIVITY
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		title TEXT NOT NULL,
		description TEXT,
		project_name TEXT NOT NULL DEFAULT 'General',
		assignee_id TEXT REFERENCES employees(id),
		priority TEXT NOT NULL DEFAULT 'medium', -- low, medium, high, urgent
		status TEXT NOT NULL DEFAULT 'todo', -- todo, in_progress, review, done
		due_date TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS shifts (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		employee_id TEXT REFERENCES employees(id),
		shift_date TEXT NOT NULL,
		start_time TEXT NOT NULL,
		end_time TEXT NOT NULL,
		station TEXT NOT NULL DEFAULT 'Floor',
		status TEXT NOT NULL DEFAULT 'scheduled', -- scheduled, clocked_in, completed, missed
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS documents (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		title TEXT NOT NULL,
		category TEXT NOT NULL DEFAULT 'General',
		content TEXT NOT NULL,
		author TEXT NOT NULL DEFAULT 'Admin',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 5. DIGITAL PRESENCE & REPUTATION
	CREATE TABLE IF NOT EXISTS local_listings (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		directory TEXT NOT NULL, -- Google Business, Yelp, Apple Maps, Bing Places
		nap_business_name TEXT NOT NULL,
		nap_address TEXT NOT NULL,
		nap_phone TEXT NOT NULL,
		nap_website TEXT NOT NULL,
		sync_status TEXT NOT NULL DEFAULT 'synced', -- synced, pending, error
		last_synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(org_id, directory)
	);

	CREATE TABLE IF NOT EXISTS reviews (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		platform TEXT NOT NULL, -- Google, Yelp, Trustpilot
		author_name TEXT NOT NULL,
		rating INTEGER NOT NULL,
		review_text TEXT NOT NULL,
		sentiment TEXT NOT NULL DEFAULT 'neutral', -- positive, neutral, negative, urgent
		reply_text TEXT,
		reply_status TEXT NOT NULL DEFAULT 'unreplied', -- unreplied, drafted_by_agent, published
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 6. HITL APPROVALS & DUAL-ACTOR AUDIT LOG
	CREATE TABLE IF NOT EXISTS approvals (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		action_type TEXT NOT NULL, -- create_po, payroll_run, public_review_reply, bulk_email
		title TEXT NOT NULL,
		description TEXT NOT NULL,
		payload_json TEXT NOT NULL DEFAULT '{}',
		payload_hash TEXT NOT NULL DEFAULT '',
		entity_type TEXT NOT NULL DEFAULT '',
		entity_id TEXT NOT NULL DEFAULT '',
		entity_version INTEGER NOT NULL DEFAULT 1,
		status TEXT NOT NULL DEFAULT 'pending', -- pending, approved, rejected
		execution_status TEXT NOT NULL DEFAULT 'none', -- none, executing, completed, failed
		execution_error TEXT NOT NULL DEFAULT '',
		requested_by_type TEXT NOT NULL DEFAULT 'agent', -- agent, human
		requested_by_id TEXT NOT NULL DEFAULT 'agent_primary',
		decided_by TEXT,
		decided_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 7. TRANSACTIONAL IDEMPOTENCY
	CREATE TABLE IF NOT EXISTS idempotency_keys (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL REFERENCES organizations(id),
		key TEXT NOT NULL,
		request_hash TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'in_progress', -- in_progress, completed, failed
		response_code INTEGER DEFAULT 0,
		response_body TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL,
		UNIQUE(org_id, key)
	);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id TEXT PRIMARY KEY,
		org_id TEXT NOT NULL DEFAULT 'org_default' REFERENCES organizations(id),
		actor_type TEXT NOT NULL, -- human, agent
		actor_id TEXT NOT NULL,
		action TEXT NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		reasoning TEXT,
		metadata_json TEXT DEFAULT '{}',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Indices for Tenant Isolation, Concurrency and Performance
	CREATE INDEX IF NOT EXISTS idx_skus_org ON skus(org_id);
	CREATE INDEX IF NOT EXISTS idx_po_org ON purchase_orders(org_id);
	CREATE INDEX IF NOT EXISTS idx_invoices_org ON invoices(org_id);
	CREATE INDEX IF NOT EXISTS idx_approvals_org ON approvals(org_id);
	CREATE INDEX IF NOT EXISTS idx_approvals_pending ON approvals(org_id, status);
	CREATE INDEX IF NOT EXISTS idx_idempotency_org_key ON idempotency_keys(org_id, key);
	CREATE INDEX IF NOT EXISTS idx_audit_org ON audit_logs(org_id);
	`
	_, err := db.Exec(schema)
	return err
}

func SeedStarterData(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM organizations").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil // already seeded
	}

	// 1. Seed Organization
	_, err := db.Exec(`INSERT INTO organizations (id, name, slug) VALUES ('org_default', 'AOSphere Global Corp', 'aosphere')`)
	if err != nil {
		return fmt.Errorf("failed to seed organization: %w", err)
	}

	// Seed Company Profile
	_, _ = db.Exec(`INSERT OR IGNORE INTO company_profiles (org_id, company_name, currency, timezone, tax_rate, address, phone, website, onboarding_completed)
		VALUES ('org_default', 'AO Sphere Enterprise', 'USD', 'America/New_York', 0.10, '100 Innovation Way, Suite 400', '+1 (555) 019-2834', 'https://aosphere.local', 1)`)


	// 2. Seed Admin User (Password: "Admin1234!")
	adminHash, err := auth.HashPassword("Admin1234!")
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}
	_, err = db.Exec(`INSERT INTO users (id, org_id, email, password_hash, name, role) VALUES
		('usr_admin_default', 'org_default', 'admin@aosphere.local', ?, 'System Administrator', 'owner')`, adminHash)
	if err != nil {
		return fmt.Errorf("failed to seed admin user: %w", err)
	}

	// 3. Seed Default Agent API Key (Token: "ao_sk_agent_default_token_12345")
	keyHash := auth.HashToken("ao_sk_agent_default_token_12345")
	_, err = db.Exec(`INSERT INTO api_keys (id, org_id, user_id, name, key_hash, role, allowed_tools) VALUES
		('key_agent_default', 'org_default', 'usr_admin_default', 'Primary Agent Client', ?, 'operations', '*')`, keyHash)
	if err != nil {
		return fmt.Errorf("failed to seed agent api key: %w", err)
	}

	now := time.Now().Format("2006-01-02")

	// Seed Suppliers
	if _, err := db.Exec(`INSERT INTO suppliers (id, name, contact_email, phone, lead_time_days, rating) VALUES
		('sup_01', 'Apex Components Ltd', 'orders@apexcomponents.io', '+1-555-0192', 2, 4.9),
		('sup_02', 'Global Logistics Co', 'dispatch@globallogistics.com', '+1-555-0841', 4, 4.6)
	`); err != nil {
		return fmt.Errorf("failed to seed suppliers: %w", err)
	}

	// Seed SKUs (with one item below threshold to demonstrate automated workflow!)
	skuList := []struct {
		id, code, name, category, supID string
		stock, reserved, min, reorder   int
		cost, price                     float64
	}{
		{"sku_01", "SKU-CORE-100", "Quantum Micro-Controller V2", "Electronics", "sup_01", 6, 2, 15, 50, 45.00, 120.00},
		{"sku_02", "SKU-SENS-200", "Ultrasonic Precision Sensor", "Sensors", "sup_01", 85, 10, 20, 100, 18.50, 48.00},
		{"sku_03", "SKU-POWR-300", "Lithium Polymer Battery 5Ah", "Power", "sup_02", 42, 5, 25, 60, 22.00, 65.00},
		{"sku_04", "SKU-CASE-400", "Anodized Aluminum Enclosure", "Hardware", "sup_02", 140, 20, 30, 80, 12.00, 34.00},
	}
	skuStmt, err := db.Prepare(`INSERT INTO skus (id, org_id, code, name, category, stock_quantity, reserved_quantity, min_threshold, reorder_quantity, unit_cost, selling_price, supplier_id)
		VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare sku insert: %w", err)
	}
	defer skuStmt.Close()

	for _, item := range skuList {
		if _, err := skuStmt.Exec(item.id, item.code, item.name, item.category, item.stock, item.reserved, item.min, item.reorder, item.cost, item.price, item.supID); err != nil {
			return fmt.Errorf("failed to insert sku %s: %w", item.code, err)
		}
	}

	// Seed Customers
	if _, err := db.Exec(`INSERT INTO customers (id, name, company, email, phone, status, lifetime_value) VALUES
		('cust_01', 'Elena Rostova', 'NeuroTech Labs', 'elena@neurotech.com', '+1-555-7721', 'active', 38500.00),
		('cust_02', 'Marcus Vance', 'HyperScale Robotics', 'marcus@hyperscale.ai', '+1-555-4493', 'active', 64200.00),
		('cust_03', 'Sarah Jenkins', 'Aura Aerospace', 's.jenkins@aura-aero.com', '+1-555-9012', 'active', 19800.00)
	`); err != nil {
		return fmt.Errorf("failed to seed customers: %w", err)
	}

	// Seed Deals (Kanban)
	if _, err := db.Exec(`INSERT INTO deals (id, title, customer_id, value, stage, probability, expected_close_date) VALUES
		('deal_01', '500x Controller Batch for Drone Fleet', 'cust_02', 60000.00, 'proposal', 80, '2026-11-15'),
		('deal_02', 'Custom Enclosure Annual Supply Contract', 'cust_01', 34000.00, 'qualified', 50, '2026-12-01'),
		('deal_03', 'Pilot Sensor Integration Kit', 'cust_03', 12500.00, 'lead', 30, '2026-11-30'),
		('deal_04', 'Q3 Fleet Hardware Renewal', 'cust_01', 28000.00, 'won', 100, '2026-10-10')
	`); err != nil {
		return fmt.Errorf("failed to seed deals: %w", err)
	}

	// Seed Invoices & Expenses for Real-time P&L
	if _, err := db.Exec(fmt.Sprintf(`INSERT INTO invoices (id, invoice_number, customer_id, status, issue_date, due_date, subtotal, tax_amount, total_amount, items_json) VALUES
		('inv_01', 'INV-2026-001', 'cust_01', 'paid', '%s', '%s', 25000.00, 2000.00, 27000.00, '[{"desc":"Hardware Delivery Batch 1","qty":200,"price":125}]'),
		('inv_02', 'INV-2026-002', 'cust_02', 'sent', '%s', '%s', 18000.00, 1440.00, 19440.00, '[{"desc":"Sensors & Controllers","qty":150,"price":120}]'),
		('inv_03', 'INV-2026-003', 'cust_03', 'draft', '%s', '%s', 8500.00, 680.00, 9180.00, '[{"desc":"Prototyping Kit","qty":50,"price":170}]')
	`, now, now, now, now, now, now)); err != nil {
		return fmt.Errorf("failed to seed invoices: %w", err)
	}

	if _, err := db.Exec(fmt.Sprintf(`INSERT INTO expenses (id, category, amount, vendor, description, date) VALUES
		('exp_01', 'Procurement', 8400.00, 'Apex Components', 'Raw micro-controllers bulk batch', '%s'),
		('exp_02', 'Cloud Infrastructure', 1250.00, 'AWS Cloud Services', 'Compute, GPU instances & storage', '%s'),
		('exp_03', 'Facilities & Logistics', 2300.00, 'City Warehouse Hub', 'Logistics fulfillment center rent', '%s')
	`, now, now, now)); err != nil {
		return fmt.Errorf("failed to seed expenses: %w", err)
	}

	// Seed Employees & Shifts
	if _, err := db.Exec(`INSERT INTO employees (id, name, role, department, hourly_rate, email, phone, status, hired_at) VALUES
		('emp_01', 'David Chen', 'Lead Hardware Engineer', 'Engineering', 65.00, 'david@aosphere.internal', '+1-555-1101', 'active', '2024-03-01'),
		('emp_02', 'Maya Lin', 'Supply Chain Operations', 'Operations', 42.00, 'maya@aosphere.internal', '+1-555-1102', 'active', '2024-06-15'),
		('emp_03', 'Liam O''Connor', 'Client Success & Sales', 'Sales', 38.00, 'liam@aosphere.internal', '+1-555-1103', 'active', '2025-01-10'),
		('emp_04', 'Aria Patel', 'Assembly Specialist', 'Production', 30.00, 'aria@aosphere.internal', '+1-555-1104', 'active', '2025-04-01')
	`); err != nil {
		return fmt.Errorf("failed to seed employees: %w", err)
	}

	if _, err := db.Exec(fmt.Sprintf(`INSERT INTO shifts (id, employee_id, shift_date, start_time, end_time, station, status) VALUES
		('shf_01', 'emp_02', '%s', '08:00', '16:30', 'Inventory & Logistics Dock', 'clocked_in'),
		('shf_02', 'emp_04', '%s', '09:00', '17:30', 'Electronics Assembly Line A', 'clocked_in'),
		('shf_03', 'emp_01', '%s', '09:30', '18:00', 'R&D Hardware Lab', 'scheduled'),
		('shf_04', 'emp_03', '%s', '10:00', '18:30', 'Client Comms Station', 'scheduled')
	`, now, now, now, now)); err != nil {
		return fmt.Errorf("failed to seed shifts: %w", err)
	}

	// Seed Tasks
	if _, err := db.Exec(`INSERT INTO tasks (id, title, description, project_name, assignee_id, priority, status, due_date) VALUES
		('tsk_01', 'Calibrate Batch 4 Sensor Array', 'Run diagnostic scripts on new ultrasonic sensor shipments', 'Quality Control', 'emp_01', 'high', 'in_progress', '2026-10-22'),
		('tsk_02', 'Review Apex Components Lead Time SLA', 'Negotiate 2-day delivery guarantee for Q4', 'Supply Chain', 'emp_02', 'medium', 'todo', '2026-10-25'),
		('tsk_03', 'Draft Proposal for Aura Aerospace', 'Finalize quotation for 500 unit kit', 'Enterprise Sales', 'emp_03', 'urgent', 'in_progress', '2026-10-21')
	`); err != nil {
		return fmt.Errorf("failed to seed tasks: %w", err)
	}

	// Seed Local Listings (NAP)
	if _, err := db.Exec(`INSERT INTO local_listings (id, directory, nap_business_name, nap_address, nap_phone, nap_website, sync_status) VALUES
		('lst_01', 'Google Business Profile', 'AOSphere Technologies Ltd', '742 Innovation Blvd, Suite 400, Austin TX 78701', '+1 (512) 555-0199', 'https://aosphere.example.com', 'synced'),
		('lst_02', 'Yelp Business', 'AOSphere Technologies Ltd', '742 Innovation Blvd, Suite 400, Austin TX 78701', '+1 (512) 555-0199', 'https://aosphere.example.com', 'synced'),
		('lst_03', 'Apple Maps', 'AOSphere Technologies Ltd', '742 Innovation Blvd, Suite 400, Austin TX 78701', '+1 (512) 555-0199', 'https://aosphere.example.com', 'synced'),
		('lst_04', 'Bing Places for Business', 'AOSphere Technologies Ltd', '742 Innovation Blvd, Suite 400, Austin TX 78701', '+1 (512) 555-0199', 'https://aosphere.example.com', 'pending')
	`); err != nil {
		return fmt.Errorf("failed to seed local listings: %w", err)
	}

	// Seed Reviews with sentiment
	if _, err := db.Exec(`INSERT INTO reviews (id, platform, author_name, rating, review_text, sentiment, reply_text, reply_status) VALUES
		('rev_01', 'Google', 'Jordan Thorne', 5, 'Unbelievable turnaround time on the micro-controller custom enclosures. Fitted perfectly into our flight test units.', 'positive', 'Thank you Jordan! Glad we could deliver ahead of your flight window.', 'published'),
		('rev_02', 'Trustpilot', 'Kavita Sundaram', 4, 'High quality sensor array. Documentation was slightly terse on pinout configs, but support helped within an hour.', 'positive', 'Appreciate the feedback Kavita! We just updated the v2 pinout schematics in our developer portal.', 'published'),
		('rev_03', 'Google', 'Alex Tremblay', 2, 'Delivery was held up at regional logistics depot for 3 days without tracking status updates. Hardware is solid though.', 'urgent', 'Alex, we sincerely apologize for the depot courier delay. Our logistics agent has flagged this with the carrier, and we have credited your account with priority shipping on your next batch.', 'drafted_by_agent')
	`); err != nil {
		return fmt.Errorf("failed to seed reviews: %w", err)
	}

	// Seed Pending HITL Approval (Agent generated low stock reorder!)
	if _, err := db.Exec(`INSERT INTO approvals (id, action_type, title, description, payload_json, status, requested_by_type, requested_by_id) VALUES
		('appr_01', 'create_po', 'Automated Purchase Order for SKU-CORE-100', 'Stock fell to 6 units (Min threshold: 15). Agent generated PO for 50 units ($2,250.00) from Apex Components Ltd.', '{"sku_code":"SKU-CORE-100","supplier_id":"sup_01","quantity":50,"unit_cost":45.0,"total":2250.0}', 'pending', 'agent', 'agt_inventory_watchdog')
	`); err != nil {
		return fmt.Errorf("failed to seed approvals: %w", err)
	}

	// Seed initial audit log
	if _, err := db.Exec(`INSERT INTO audit_logs (id, actor_type, actor_id, action, entity_type, entity_id, reasoning, metadata_json) VALUES
		('aud_01', 'agent', 'agt_inventory_watchdog', 'TRIGGER_LOW_STOCK_REORDER', 'skus', 'sku_01', 'Stock level (6) breached minimum safety threshold (15). PO created and routed to human approval queue due to amount > $1,000 safety rule.', '{"amount":2250.00,"rule_triggered":"SAFETY_THRESHOLD_1000"}')
	`); err != nil {
		return fmt.Errorf("failed to seed audit logs: %w", err)
	}

	// Seed Standard Chart of Accounts
	if _, err := db.Exec(`INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance) VALUES
		('acc_1000', 'org_default', '1000', 'Operating Cash / Bank', 'asset', 'debit'),
		('acc_1100', 'org_default', '1100', 'Accounts Receivable', 'asset', 'debit'),
		('acc_1200', 'org_default', '1200', 'Inventory Asset', 'asset', 'debit'),
		('acc_2000', 'org_default', '2000', 'Accounts Payable', 'liability', 'credit'),
		('acc_2100', 'org_default', '2100', 'Sales Tax Payable', 'liability', 'credit'),
		('acc_3000', 'org_default', '3000', 'Retained Earnings', 'equity', 'credit'),
		('acc_4000', 'org_default', '4000', 'Sales Revenue', 'revenue', 'credit'),
		('acc_5000', 'org_default', '5000', 'Cost of Goods Sold', 'expense', 'debit'),
		('acc_5100', 'org_default', '5100', 'Operating Expenses', 'expense', 'debit')
	`); err != nil {
		return fmt.Errorf("failed to seed chart of accounts: %w", err)
	}

	return nil
}
