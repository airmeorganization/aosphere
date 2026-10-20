package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// InitDB initializes SQLite with WAL mode and creates all tables.
func InitDB(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db dir: %w", err)
		}
	}

	// Open with pragmas for WAL mode and performance
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(4) // Concurrency-friendly in WAL mode with busy_timeout
	db.SetMaxIdleConns(4)

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	if err := seedStarterData(db); err != nil {
		log.Printf("Seed starter data check: %v", err)
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	schema := `
	-- 1. ERP & SUPPLY CHAIN
	CREATE TABLE IF NOT EXISTS suppliers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		contact_email TEXT,
		phone TEXT,
		lead_time_days INTEGER DEFAULT 3,
		rating REAL DEFAULT 5.0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS skus (
		id TEXT PRIMARY KEY,
		code TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		category TEXT NOT NULL,
		stock_quantity INTEGER NOT NULL DEFAULT 0,
		reserved_quantity INTEGER NOT NULL DEFAULT 0,
		min_threshold INTEGER NOT NULL DEFAULT 10,
		reorder_quantity INTEGER NOT NULL DEFAULT 50,
		unit_cost REAL NOT NULL DEFAULT 0.0,
		selling_price REAL NOT NULL DEFAULT 0.0,
		supplier_id TEXT REFERENCES suppliers(id),
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS purchase_orders (
		id TEXT PRIMARY KEY,
		po_number TEXT UNIQUE NOT NULL,
		supplier_id TEXT REFERENCES suppliers(id),
		status TEXT NOT NULL DEFAULT 'draft', -- draft, pending_approval, ordered, received, cancelled
		total_amount REAL NOT NULL DEFAULT 0.0,
		items_json TEXT NOT NULL DEFAULT '[]',
		created_by_type TEXT NOT NULL DEFAULT 'human', -- human, agent
		actor_id TEXT NOT NULL DEFAULT 'system',
		notes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 2. CRM & SALES PIPELINE
	CREATE TABLE IF NOT EXISTS customers (
		id TEXT PRIMARY KEY,
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
		order_number TEXT UNIQUE NOT NULL,
		customer_id TEXT REFERENCES customers(id),
		total_amount REAL NOT NULL,
		status TEXT NOT NULL DEFAULT 'completed',
		items_json TEXT NOT NULL DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 3. FINANCIAL & HRMS
	CREATE TABLE IF NOT EXISTS invoices (
		id TEXT PRIMARY KEY,
		invoice_number TEXT UNIQUE NOT NULL,
		customer_id TEXT REFERENCES customers(id),
		status TEXT NOT NULL DEFAULT 'draft', -- draft, sent, paid, overdue
		issue_date TEXT NOT NULL,
		due_date TEXT NOT NULL,
		subtotal REAL NOT NULL DEFAULT 0.0,
		tax_amount REAL NOT NULL DEFAULT 0.0,
		total_amount REAL NOT NULL DEFAULT 0.0,
		recurring_frequency TEXT DEFAULT 'none', -- none, monthly, quarterly, annual
		items_json TEXT NOT NULL DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS expenses (
		id TEXT PRIMARY KEY,
		category TEXT NOT NULL,
		amount REAL NOT NULL,
		vendor TEXT NOT NULL,
		description TEXT,
		date TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS employees (
		id TEXT PRIMARY KEY,
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
		title TEXT NOT NULL,
		category TEXT NOT NULL DEFAULT 'General',
		content TEXT NOT NULL,
		author TEXT NOT NULL DEFAULT 'Admin',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 5. DIGITAL PRESENCE & REPUTATION
	CREATE TABLE IF NOT EXISTS local_listings (
		id TEXT PRIMARY KEY,
		directory TEXT NOT NULL UNIQUE, -- Google Business, Yelp, Apple Maps, Bing Places
		nap_business_name TEXT NOT NULL,
		nap_address TEXT NOT NULL,
		nap_phone TEXT NOT NULL,
		nap_website TEXT NOT NULL,
		sync_status TEXT NOT NULL DEFAULT 'synced', -- synced, pending, error
		last_synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS reviews (
		id TEXT PRIMARY KEY,
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
		action_type TEXT NOT NULL, -- create_po, payroll_run, public_review_reply, bulk_email
		title TEXT NOT NULL,
		description TEXT NOT NULL,
		payload_json TEXT NOT NULL DEFAULT '{}',
		status TEXT NOT NULL DEFAULT 'pending', -- pending, approved, rejected
		requested_by_type TEXT NOT NULL DEFAULT 'agent', -- agent, human
		requested_by_id TEXT NOT NULL DEFAULT 'agent_primary',
		decided_by TEXT,
		decided_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id TEXT PRIMARY KEY,
		actor_type TEXT NOT NULL, -- human, agent
		actor_id TEXT NOT NULL,
		action TEXT NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		reasoning TEXT,
		metadata_json TEXT DEFAULT '{}',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(schema)
	return err
}

func seedStarterData(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM skus").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil // already seeded
	}

	now := time.Now().Format("2006-01-02")

	// Seed Suppliers
	db.Exec(`INSERT INTO suppliers (id, name, contact_email, phone, lead_time_days, rating) VALUES
		('sup_01', 'Apex Components Ltd', 'orders@apexcomponents.io', '+1-555-0192', 2, 4.9),
		('sup_02', 'Global Logistics Co', 'dispatch@globallogistics.com', '+1-555-0841', 4, 4.6)
	`)

	// Seed SKUs (with one item below threshold to demonstrate automated workflow!)
	db.Exec(`INSERT INTO skus (id, code, name, category, stock_quantity, reserved_quantity, min_threshold, reorder_quantity, unit_cost, selling_price, supplier_id) VALUES
		('sku_01', 'SKU-CORE-100', 'Quantum Micro-Controller V2', 'Electronics', 6, 2, 15, 50, 45.00, 120.00, 'sup_01'),
		('sku_02', 'SKU-SENS-200', 'Ultrasonic Precision Sensor', 'Sensors', 85, 10, 20, 100, 18.50, 48.00, 'sup_01'),
		('sku_03', 'SKU-POWR-300', 'Lithium Polymer Battery 5Ah', 'Power', 42, 5, 25, 60, 22.00, 65.00, 'sup_02'),
		('sku_04', 'SKU-CASE-400', 'Anodized Aluminum Enclosure', 'Hardware', 140, 20, 30, 80, 12.00, 34.00, 'sup_02')
	`)

	// Seed Customers
	db.Exec(`INSERT INTO customers (id, name, company, email, phone, status, lifetime_value) VALUES
		('cust_01', 'Elena Rostova', 'NeuroTech Labs', 'elena@neurotech.com', '+1-555-7721', 'active', 38500.00),
		('cust_02', 'Marcus Vance', 'HyperScale Robotics', 'marcus@hyperscale.ai', '+1-555-4493', 'active', 64200.00),
		('cust_03', 'Sarah Jenkins', 'Aura Aerospace', 's.jenkins@aura-aero.com', '+1-555-9012', 'active', 19800.00)
	`)

	// Seed Deals (Kanban)
	db.Exec(`INSERT INTO deals (id, title, customer_id, value, stage, probability, expected_close_date) VALUES
		('deal_01', '500x Controller Batch for Drone Fleet', 'cust_02', 60000.00, 'proposal', 80, '2026-11-15'),
		('deal_02', 'Custom Enclosure Annual Supply Contract', 'cust_01', 34000.00, 'qualified', 50, '2026-12-01'),
		('deal_03', 'Pilot Sensor Integration Kit', 'cust_03', 12500.00, 'lead', 30, '2026-11-30'),
		('deal_04', 'Q3 Fleet Hardware Renewal', 'cust_01', 28000.00, 'won', 100, '2026-10-10')
	`)

	// Seed Invoices & Expenses for Real-time P&L
	db.Exec(fmt.Sprintf(`INSERT INTO invoices (id, invoice_number, customer_id, status, issue_date, due_date, subtotal, tax_amount, total_amount, items_json) VALUES
		('inv_01', 'INV-2026-001', 'cust_01', 'paid', '%s', '%s', 25000.00, 2000.00, 27000.00, '[{"desc":"Hardware Delivery Batch 1","qty":200,"price":125}]'),
		('inv_02', 'INV-2026-002', 'cust_02', 'sent', '%s', '%s', 18000.00, 1440.00, 19440.00, '[{"desc":"Sensors & Controllers","qty":150,"price":120}]'),
		('inv_03', 'INV-2026-003', 'cust_03', 'draft', '%s', '%s', 8500.00, 680.00, 9180.00, '[{"desc":"Prototyping Kit","qty":50,"price":170}]')
	`, now, now, now, now, now, now))

	db.Exec(fmt.Sprintf(`INSERT INTO expenses (id, category, amount, vendor, description, date) VALUES
		('exp_01', 'Procurement', 8400.00, 'Apex Components', 'Raw micro-controllers bulk batch', '%s'),
		('exp_02', 'Cloud Infrastructure', 1250.00, 'AWS Cloud Services', 'Compute, GPU instances & storage', '%s'),
		('exp_03', 'Facilities & Logistics', 2300.00, 'City Warehouse Hub', 'Logistics fulfillment center rent', '%s')
	`, now, now, now))

	// Seed Employees & Shifts
	db.Exec(`INSERT INTO employees (id, name, role, department, hourly_rate, email, phone, status, hired_at) VALUES
		('emp_01', 'David Chen', 'Lead Hardware Engineer', 'Engineering', 65.00, 'david@aosphere.internal', '+1-555-1101', 'active', '2024-03-01'),
		('emp_02', 'Maya Lin', 'Supply Chain Operations', 'Operations', 42.00, 'maya@aosphere.internal', '+1-555-1102', 'active', '2024-06-15'),
		('emp_03', 'Liam O''Connor', 'Client Success & Sales', 'Sales', 38.00, 'liam@aosphere.internal', '+1-555-1103', 'active', '2025-01-10'),
		('emp_04', 'Aria Patel', 'Assembly Specialist', 'Production', 30.00, 'aria@aosphere.internal', '+1-555-1104', 'active', '2025-04-01')
	`)

	db.Exec(fmt.Sprintf(`INSERT INTO shifts (id, employee_id, shift_date, start_time, end_time, station, status) VALUES
		('shf_01', 'emp_02', '%s', '08:00', '16:30', 'Inventory & Logistics Dock', 'clocked_in'),
		('shf_02', 'emp_04', '%s', '09:00', '17:30', 'Electronics Assembly Line A', 'clocked_in'),
		('shf_03', 'emp_01', '%s', '09:30', '18:00', 'R&D Hardware Lab', 'scheduled'),
		('shf_04', 'emp_03', '%s', '10:00', '18:30', 'Client Comms Station', 'scheduled')
	`, now, now, now, now))

	// Seed Tasks
	db.Exec(`INSERT INTO tasks (id, title, description, project_name, assignee_id, priority, status, due_date) VALUES
		('tsk_01', 'Calibrate Batch 4 Sensor Array', 'Run diagnostic scripts on new ultrasonic sensor shipments', 'Quality Control', 'emp_01', 'high', 'in_progress', '2026-10-22'),
		('tsk_02', 'Review Apex Components Lead Time SLA', 'Negotiate 2-day delivery guarantee for Q4', 'Supply Chain', 'emp_02', 'medium', 'todo', '2026-10-25'),
		('tsk_03', 'Draft Proposal for Aura Aerospace', 'Finalize quotation for 500 unit kit', 'Enterprise Sales', 'emp_03', 'urgent', 'in_progress', '2026-10-21')
	`)

	// Seed Local Listings (NAP)
	db.Exec(`INSERT INTO local_listings (id, directory, nap_business_name, nap_address, nap_phone, nap_website, sync_status) VALUES
		('lst_01', 'Google Business Profile', 'AOSphere Technologies Ltd', '742 Innovation Blvd, Suite 400, Austin TX 78701', '+1 (512) 555-0199', 'https://aosphere.example.com', 'synced'),
		('lst_02', 'Yelp Business', 'AOSphere Technologies Ltd', '742 Innovation Blvd, Suite 400, Austin TX 78701', '+1 (512) 555-0199', 'https://aosphere.example.com', 'synced'),
		('lst_03', 'Apple Maps', 'AOSphere Technologies Ltd', '742 Innovation Blvd, Suite 400, Austin TX 78701', '+1 (512) 555-0199', 'https://aosphere.example.com', 'synced'),
		('lst_04', 'Bing Places for Business', 'AOSphere Technologies Ltd', '742 Innovation Blvd, Suite 400, Austin TX 78701', '+1 (512) 555-0199', 'https://aosphere.example.com', 'pending')
	`)

	// Seed Reviews with sentiment
	db.Exec(`INSERT INTO reviews (id, platform, author_name, rating, review_text, sentiment, reply_text, reply_status) VALUES
		('rev_01', 'Google', 'Jordan Thorne', 5, 'Unbelievable turnaround time on the micro-controller custom enclosures. Fitted perfectly into our flight test units.', 'positive', 'Thank you Jordan! Glad we could deliver ahead of your flight window.', 'published'),
		('rev_02', 'Trustpilot', 'Kavita Sundaram', 4, 'High quality sensor array. Documentation was slightly terse on pinout configs, but support helped within an hour.', 'positive', 'Appreciate the feedback Kavita! We just updated the v2 pinout schematics in our developer portal.', 'published'),
		('rev_03', 'Google', 'Alex Tremblay', 2, 'Delivery was held up at regional logistics depot for 3 days without tracking status updates. Hardware is solid though.', 'urgent', 'Alex, we sincerely apologize for the depot courier delay. Our logistics agent has flagged this with the carrier, and we have credited your account with priority shipping on your next batch.', 'drafted_by_agent')
	`)

	// Seed a real pending PO and link its human approval to that exact record.
	db.Exec(`INSERT INTO purchase_orders (id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes) VALUES
		('po_01', 'PO-2026-001', 'sup_01', 'pending_approval', 2250.00, '[{"sku_code":"SKU-CORE-100","sku_name":"Quantum Micro-Controller V2","quantity":50,"unit_cost":45.0}]', 'agent', 'agt_inventory_watchdog', 'Seeded low-stock reorder awaiting human approval.')
	`)

	// Seed Pending HITL Approval (Agent generated low stock reorder!)
	db.Exec(`INSERT INTO approvals (id, action_type, title, description, payload_json, status, requested_by_type, requested_by_id) VALUES
		('appr_01', 'create_po', 'Automated Purchase Order for SKU-CORE-100', 'Stock fell to 6 units (Min threshold: 15). Agent generated PO for 50 units ($2,250.00) from Apex Components Ltd.', '{"purchase_order_id":"po_01","po_number":"PO-2026-001","supplier_id":"sup_01","total_amount":2250.0,"items":[{"sku_code":"SKU-CORE-100","sku_name":"Quantum Micro-Controller V2","quantity":50,"unit_cost":45.0}]}', 'pending', 'agent', 'agt_inventory_watchdog')
	`)

	// Seed initial audit log
	db.Exec(`INSERT INTO audit_logs (id, actor_type, actor_id, action, entity_type, entity_id, reasoning, metadata_json) VALUES
		('aud_01', 'agent', 'agt_inventory_watchdog', 'TRIGGER_LOW_STOCK_REORDER', 'skus', 'sku_01', 'Stock level (6) breached minimum safety threshold (15). PO created and routed to human approval queue due to amount > $1,000 safety rule.', '{"amount":2250.00,"rule_triggered":"SAFETY_THRESHOLD_1000"}')
	`)

	return nil
}
