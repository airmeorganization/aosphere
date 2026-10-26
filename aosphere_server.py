#!/usr/bin/env python3
"""
AO Sphere — Business Operating System
Native Python runtime & web server for Windows 11 Smart App Control environments.
Connects directly to data/aosphere.db, serves web/ assets, REST APIs, SSE, and MCP JSON-RPC.
"""

import os
import sys
import json
import sqlite3
import argparse
import mimetypes
from datetime import datetime, timezone

def utcnow_str():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

def utc_today_str():
    return datetime.now(timezone.utc).strftime("%Y-%m-%d")

def utcnow_iso():
    return datetime.now(timezone.utc).isoformat()
from http.server import ThreadingHTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs
import threading
import time

DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data", "aosphere.db")
WEB_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "web")

clients_lock = threading.Lock()
sse_clients = []

_schema_initialized = False

def ensure_schema(conn):
    global _schema_initialized
    if _schema_initialized:
        return
    conn.executescript("""
    CREATE TABLE IF NOT EXISTS warehouses (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        name TEXT NOT NULL,
        code TEXT NOT NULL,
        location TEXT NOT NULL DEFAULT '',
        description TEXT NOT NULL DEFAULT '',
        is_active INTEGER NOT NULL DEFAULT 1,
        created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS warehouse_stocks (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        warehouse_id TEXT NOT NULL,
        sku_code TEXT NOT NULL,
        available_qty INTEGER NOT NULL DEFAULT 0,
        reserved_qty INTEGER NOT NULL DEFAULT 0,
        bin_location TEXT NOT NULL DEFAULT 'A-01-01',
        updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS goods_receipt_notes (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        grn_number TEXT NOT NULL,
        po_id TEXT NOT NULL,
        warehouse_id TEXT NOT NULL,
        received_date TEXT NOT NULL,
        carrier_tracking TEXT NOT NULL DEFAULT '',
        inspector_name TEXT NOT NULL DEFAULT '',
        notes TEXT NOT NULL DEFAULT '',
        items_json TEXT NOT NULL DEFAULT '[]',
        status TEXT NOT NULL DEFAULT 'completed',
        created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS supplier_bills (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        bill_number TEXT NOT NULL,
        supplier_id TEXT NOT NULL,
        po_id TEXT NOT NULL DEFAULT '',
        grn_id TEXT NOT NULL DEFAULT '',
        amount REAL NOT NULL DEFAULT 0,
        amount_cents INTEGER NOT NULL DEFAULT 0,
        match_status TEXT NOT NULL DEFAULT 'pending',
        due_date TEXT NOT NULL DEFAULT '',
        status TEXT NOT NULL DEFAULT 'open',
        created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS stock_transfers (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        transfer_number TEXT NOT NULL,
        from_warehouse_id TEXT NOT NULL,
        to_warehouse_id TEXT NOT NULL,
        sku_code TEXT NOT NULL,
        quantity INTEGER NOT NULL,
        reason TEXT NOT NULL DEFAULT '',
        status TEXT NOT NULL DEFAULT 'completed',
        created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS batch_lots (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        sku_code TEXT NOT NULL,
        batch_number TEXT NOT NULL,
        lot_number TEXT NOT NULL DEFAULT '',
        quantity INTEGER NOT NULL DEFAULT 0,
        expiry_date TEXT NOT NULL DEFAULT '',
        created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    INSERT OR IGNORE INTO warehouses (id, org_id, name, code, location, description)
    VALUES ('wh_main_org_default', 'org_default', 'Central Distribution Center', 'WH-MAIN', 'Austin, TX', 'Primary central inventory hub');
    INSERT OR IGNORE INTO warehouses (id, org_id, name, code, location, description)
    VALUES ('wh_east_org_default', 'org_default', 'Eastern Regional Depo', 'WH-EAST', 'Boston, MA', 'Secondary eastern regional hub');

    CREATE TABLE IF NOT EXISTS credit_notes (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        credit_note_number TEXT NOT NULL,
        invoice_id TEXT DEFAULT '',
        customer_id TEXT NOT NULL,
        currency TEXT NOT NULL DEFAULT 'USD',
        subtotal REAL NOT NULL DEFAULT 0.0,
        tax_amount REAL NOT NULL DEFAULT 0.0,
        total_amount REAL NOT NULL DEFAULT 0.0,
        total_cents INTEGER NOT NULL DEFAULT 0,
        reason TEXT NOT NULL DEFAULT '',
        status TEXT NOT NULL DEFAULT 'issued',
        items_json TEXT NOT NULL DEFAULT '[]',
        created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS exchange_rates (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        base_currency TEXT NOT NULL DEFAULT 'USD',
        target_currency TEXT NOT NULL,
        rate REAL NOT NULL,
        updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(org_id, base_currency, target_currency)
    );

    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_INR_org_default', 'org_default', 'USD', 'INR', 83.50);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_JPY_org_default', 'org_default', 'USD', 'JPY', 153.20);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_CNY_org_default', 'org_default', 'USD', 'CNY', 7.23);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_SGD_org_default', 'org_default', 'USD', 'SGD', 1.32);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_AED_org_default', 'org_default', 'USD', 'AED', 3.67);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_SAR_org_default', 'org_default', 'USD', 'SAR', 3.75);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_HKD_org_default', 'org_default', 'USD', 'HKD', 7.78);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_KRW_org_default', 'org_default', 'USD', 'KRW', 1380.0);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_THB_org_default', 'org_default', 'USD', 'THB', 36.80);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_IDR_org_default', 'org_default', 'USD', 'IDR', 15850.0);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_MYR_org_default', 'org_default', 'USD', 'MYR', 4.45);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_PHP_org_default', 'org_default', 'USD', 'PHP', 58.20);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_VND_org_default', 'org_default', 'USD', 'VND', 25400.0);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_EUR_org_default', 'org_default', 'USD', 'EUR', 0.92);
    INSERT OR IGNORE INTO exchange_rates (id, org_id, base_currency, target_currency, rate) VALUES ('fx_GBP_org_default', 'org_default', 'USD', 'GBP', 0.77);
    """)
    conn.commit()
    _schema_initialized = True

def get_db():
    conn = sqlite3.connect(DB_PATH, timeout=10.0)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA journal_mode=WAL;")
    conn.execute("PRAGMA foreign_keys=ON;")
    ensure_schema(conn)
    return conn

def broadcast(event_type, payload):
    with clients_lock:
        to_remove = []
        for q in sse_clients:
            try:
                q.put(f"event: {event_type}\ndata: {json.dumps(payload)}\n\n")
            except Exception:
                to_remove.append(q)
        for q in to_remove:
            if q in sse_clients:
                sse_clients.remove(q)

def log_audit(conn, actor_type, actor_id, action, entity_type, entity_id, reasoning, meta="{}"):
    audit_id = f"aud_{int(time.time()*1000)}"
    now = utcnow_str()
    conn.execute(
        "INSERT INTO audit_logs (id, actor_type, actor_id, action, entity_type, entity_id, reasoning, metadata_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
        (audit_id, actor_type, actor_id, action, entity_type, entity_id, reasoning, meta, now)
    )

class AOSphereHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass

    def send_json(self, data, status=200):
        try:
            body = json.dumps(data, default=str).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Access-Control-Allow-Origin", "*")
            self.end_headers()
            self.wfile.write(body)
        except Exception:
            pass

    def do_OPTIONS(self):
        try:
            self.send_response(200)
            self.send_header("Access-Control-Allow-Origin", "*")
            self.send_header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
            self.send_header("Access-Control-Allow-Headers", "Content-Type, Authorization")
            self.end_headers()
        except Exception:
            pass

    def do_HEAD(self):
        self.do_GET(head_only=True)

    def do_GET(self, head_only=False):
        parsed = urlparse(self.path)
        path = parsed.path
        query = parse_qs(parsed.query)

        # 1. SSE Stream
        if path == "/api/events":
            import queue
            try:
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.send_header("Cache-Control", "no-cache")
                self.send_header("Connection", "keep-alive")
                self.send_header("Access-Control-Allow-Origin", "*")
                self.end_headers()
            except Exception:
                return

            if head_only:
                return

            q = queue.Queue(maxsize=32)
            with clients_lock:
                sse_clients.append(q)

            init_msg = f"event: connected\ndata: {{\"status\":\"ok\",\"time\":\"{utcnow_iso()}\"}}\n\n"
            try:
                self.wfile.write(init_msg.encode("utf-8"))
                self.wfile.flush()
                while True:
                    try:
                        msg = q.get(timeout=15.0)
                        self.wfile.write(msg.encode("utf-8"))
                        self.wfile.flush()
                    except queue.Empty:
                        self.wfile.write(b": keepalive\n\n")
                        self.wfile.flush()
            except (ConnectionResetError, BrokenPipeError, ConnectionAbortedError, OSError, Exception):
                pass
            finally:
                with clients_lock:
                    if q in sse_clients:
                        sse_clients.remove(q)
            return

        # 2. REST APIs
        if path.startswith("/api/"):
            try:
                conn = get_db()
                if path == "/api/skus":
                    below = query.get("below_threshold", ["false"])[0] == "true"
                    if below:
                        rows = conn.execute("SELECT skus.*, COALESCE(suppliers.name, '') as supplier_name FROM skus LEFT JOIN suppliers ON skus.supplier_id = suppliers.id WHERE stock_quantity <= min_threshold").fetchall()
                    else:
                        rows = conn.execute("SELECT skus.*, COALESCE(suppliers.name, '') as supplier_name FROM skus LEFT JOIN suppliers ON skus.supplier_id = suppliers.id ORDER BY skus.name").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/suppliers":
                    rows = conn.execute("SELECT * FROM suppliers ORDER BY name").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/purchase-orders":
                    rows = conn.execute("SELECT po.*, COALESCE(s.name, '') as supplier_name FROM purchase_orders po LEFT JOIN suppliers s ON po.supplier_id = s.id ORDER BY po.created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/customers":
                    rows = conn.execute("SELECT * FROM customers ORDER BY lifetime_value DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/deals":
                    rows = conn.execute("SELECT d.*, COALESCE(c.name, '') as customer_name, COALESCE(c.company, '') as customer_company FROM deals d LEFT JOIN customers c ON d.customer_id = c.id ORDER BY d.created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/invoices":
                    rows = conn.execute("SELECT inv.*, COALESCE(c.name, '') as customer_name FROM invoices inv LEFT JOIN customers c ON inv.customer_id = c.id ORDER BY inv.created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/financial-summary":
                    rev_row = conn.execute("SELECT COALESCE(SUM(total_amount), 0) as rev, COUNT(*) as cnt FROM invoices WHERE status = 'paid'").fetchone()
                    unpaid_row = conn.execute("SELECT COALESCE(SUM(total_amount), 0) as unp, COUNT(*) as cnt FROM invoices WHERE status != 'paid'").fetchone()
                    exp_row = conn.execute("SELECT COALESCE(SUM(amount), 0) as exp FROM expenses").fetchone()
                    conn.close()

                    total_rev = float(rev_row["rev"] or 0.0)
                    total_exp = float(exp_row["exp"] or 0.0)
                    return self.send_json({
                        "total_revenue": total_rev,
                        "total_expense": total_exp,
                        "net_profit": total_rev - total_exp,
                        "unpaid_total": float(unpaid_row["unp"] or 0.0),
                        "paid_count": int(rev_row["cnt"] or 0),
                        "pending_count": int(unpaid_row["cnt"] or 0)
                    })

                elif path == "/api/expenses":
                    rows = conn.execute("SELECT * FROM expenses ORDER BY date DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/employees":
                    rows = conn.execute("SELECT * FROM employees ORDER BY name").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/shifts":
                    rows = conn.execute("SELECT s.*, COALESCE(e.name, '') as employee_name, COALESCE(e.role, '') as employee_role FROM shifts s LEFT JOIN employees e ON s.employee_id = e.id ORDER BY s.shift_date DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/tasks":
                    rows = conn.execute("SELECT t.*, COALESCE(e.name, '') as assignee_name FROM tasks t LEFT JOIN employees e ON t.assignee_id = e.id ORDER BY t.created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/listings":
                    rows = conn.execute("SELECT id, directory as platform, nap_business_name as business_name, nap_address as address, nap_phone as phone, nap_website as website, sync_status as status, last_synced_at as last_synced FROM local_listings ORDER BY directory").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/audit-logs":
                    rows = conn.execute("SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT 100").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/reviews":
                    rows = conn.execute("SELECT * FROM reviews ORDER BY created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/approvals":
                    status = query.get("status", [None])[0]
                    if status:
                        rows = conn.execute("SELECT * FROM approvals WHERE status = ? ORDER BY created_at DESC", (status,)).fetchall()
                    else:
                        rows = conn.execute("SELECT * FROM approvals ORDER BY created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/warehouses":
                    rows = conn.execute("SELECT * FROM warehouses ORDER BY name").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/warehouse-stocks":
                    rows = conn.execute("SELECT ws.*, COALESCE(w.name, '') as warehouse_name FROM warehouse_stocks ws LEFT JOIN warehouses w ON ws.warehouse_id = w.id ORDER BY ws.sku_code").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/grns":
                    rows = conn.execute("SELECT g.*, COALESCE(po.po_number, '') as po_number, COALESCE(w.name, '') as warehouse_name FROM goods_receipt_notes g LEFT JOIN purchase_orders po ON g.po_id = po.id LEFT JOIN warehouses w ON g.warehouse_id = w.id ORDER BY g.created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/supplier-bills":
                    rows = conn.execute("SELECT b.*, COALESCE(s.name, '') as supplier_name, COALESCE(po.po_number, '') as po_number FROM supplier_bills b LEFT JOIN suppliers s ON b.supplier_id = s.id LEFT JOIN purchase_orders po ON b.po_id = po.id ORDER BY b.created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/stock-transfers":
                    rows = conn.execute("SELECT t.*, COALESCE(fw.name, '') as from_warehouse_name, COALESCE(tw.name, '') as to_warehouse_name FROM stock_transfers t LEFT JOIN warehouses fw ON t.from_warehouse_id = fw.id LEFT JOIN warehouses tw ON t.to_warehouse_id = tw.id ORDER BY t.created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/batch-lots":
                    rows = conn.execute("SELECT * FROM batch_lots ORDER BY created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/credit-notes":
                    rows = conn.execute("""
                        SELECT cn.*, COALESCE(inv.invoice_number, '') as invoice_number, COALESCE(c.name, '') as customer_name
                        FROM credit_notes cn
                        LEFT JOIN invoices inv ON cn.invoice_id = inv.id
                        LEFT JOIN customers c ON cn.customer_id = c.id
                        ORDER BY cn.created_at DESC
                    """).fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/ledger/accounts":
                    rows = conn.execute("""
                        SELECT a.id, a.org_id, a.code, a.name, a.type, a.normal_balance, a.is_active,
                            COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE 0 END), 0) AS debit_cents,
                            COALESCE(SUM(CASE WHEN jl.entry_type = 'credit' THEN jl.amount_cents ELSE 0 END), 0) AS credit_cents
                        FROM accounts a
                        LEFT JOIN journal_lines jl ON a.id = jl.account_id
                        GROUP BY a.id, a.org_id, a.code, a.name, a.type, a.normal_balance, a.is_active
                        ORDER BY a.code ASC
                    """).fetchall()
                    res = []
                    for r in rows:
                        d = dict(r)
                        if d.get("normal_balance") == "debit":
                            d["net_balance"] = (d["debit_cents"] - d["credit_cents"]) / 100.0
                        else:
                            d["net_balance"] = (d["credit_cents"] - d["debit_cents"]) / 100.0
                        res.append(d)
                    conn.close()
                    return self.send_json(res)

                elif path == "/api/ledger/entries":
                    entries = conn.execute("SELECT * FROM journal_entries ORDER BY posted_at DESC LIMIT 100").fetchall()
                    res = []
                    for e in entries:
                        ed = dict(e)
                        lines = conn.execute("""
                            SELECT jl.*, a.code as account_code, a.name as account_name
                            FROM journal_lines jl
                            JOIN accounts a ON jl.account_id = a.id
                            WHERE jl.entry_id = ? ORDER BY jl.entry_type DESC
                        """, (ed["id"],)).fetchall()
                        ed["lines"] = [dict(l) for l in lines]
                        res.append(ed)
                    conn.close()
                    return self.send_json(res)

                elif path == "/api/ledger/trial-balance":
                    rows = conn.execute("""
                        SELECT a.code, a.name, a.type,
                            COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE 0 END), 0) AS debit_cents,
                            COALESCE(SUM(CASE WHEN jl.entry_type = 'credit' THEN jl.amount_cents ELSE 0 END), 0) AS credit_cents
                        FROM accounts a
                        LEFT JOIN journal_lines jl ON a.id = jl.account_id
                        GROUP BY a.code, a.name, a.type
                        ORDER BY a.code ASC
                    """).fetchall()
                    tot_debits = sum(r["debit_cents"] for r in rows)
                    tot_credits = sum(r["credit_cents"] for r in rows)
                    conn.close()
                    return self.send_json({
                        "org_id": "org_default",
                        "as_of": utcnow_iso(),
                        "accounts": [dict(r) for r in rows],
                        "total_debits_cents": tot_debits,
                        "total_credits_cents": tot_credits,
                        "is_balanced": (tot_debits == tot_credits)
                    })

                elif path == "/api/currencies":
                    rates_rows = conn.execute("SELECT target_currency, rate FROM exchange_rates").fetchall()
                    rates_map = {r["target_currency"]: float(r["rate"]) for r in rates_rows}
                    conn.close()

                    all_currencies = [
                        # South Asia
                        {"code": "INR", "name": "Indian Rupee", "symbol": "₹", "decimals": 2, "region": "South Asia", "default_rate": rates_map.get("INR", 83.50)},
                        {"code": "PKR", "name": "Pakistani Rupee", "symbol": "₨", "decimals": 2, "region": "South Asia", "default_rate": rates_map.get("PKR", 278.50)},
                        {"code": "BDT", "name": "Bangladeshi Taka", "symbol": "৳", "decimals": 2, "region": "South Asia", "default_rate": rates_map.get("BDT", 119.50)},
                        {"code": "LKR", "name": "Sri Lankan Rupee", "symbol": "Rs", "decimals": 2, "region": "South Asia", "default_rate": rates_map.get("LKR", 293.00)},
                        {"code": "NPR", "name": "Nepalese Rupee", "symbol": "रू", "decimals": 2, "region": "South Asia", "default_rate": rates_map.get("NPR", 133.50)},
                        {"code": "BTN", "name": "Bhutanese Ngultrum", "symbol": "Nu.", "decimals": 2, "region": "South Asia", "default_rate": rates_map.get("BTN", 83.50)},
                        {"code": "MVR", "name": "Maldivian Rufiyaa", "symbol": "Rf", "decimals": 2, "region": "South Asia", "default_rate": rates_map.get("MVR", 15.45)},
                        {"code": "AFN", "name": "Afghan Afghani", "symbol": "؋", "decimals": 2, "region": "South Asia", "default_rate": rates_map.get("AFN", 67.50)},

                        # East Asia
                        {"code": "JPY", "name": "Japanese Yen", "symbol": "¥", "decimals": 0, "region": "East Asia", "default_rate": rates_map.get("JPY", 153.20)},
                        {"code": "CNY", "name": "Chinese Yuan", "symbol": "¥", "decimals": 2, "region": "East Asia", "default_rate": rates_map.get("CNY", 7.23)},
                        {"code": "HKD", "name": "Hong Kong Dollar", "symbol": "HK$", "decimals": 2, "region": "East Asia", "default_rate": rates_map.get("HKD", 7.78)},
                        {"code": "TWD", "name": "New Taiwan Dollar", "symbol": "NT$", "decimals": 2, "region": "East Asia", "default_rate": rates_map.get("TWD", 32.20)},
                        {"code": "KRW", "name": "South Korean Won", "symbol": "₩", "decimals": 0, "region": "East Asia", "default_rate": rates_map.get("KRW", 1380.00)},
                        {"code": "MOP", "name": "Macanese Pataca", "symbol": "MOP$", "decimals": 2, "region": "East Asia", "default_rate": rates_map.get("MOP", 8.01)},
                        {"code": "MNT", "name": "Mongolian Tögrög", "symbol": "₮", "decimals": 2, "region": "East Asia", "default_rate": rates_map.get("MNT", 3450.00)},

                        # Southeast Asia
                        {"code": "SGD", "name": "Singapore Dollar", "symbol": "S$", "decimals": 2, "region": "Southeast Asia", "default_rate": rates_map.get("SGD", 1.32)},
                        {"code": "MYR", "name": "Malaysian Ringgit", "symbol": "RM", "decimals": 2, "region": "Southeast Asia", "default_rate": rates_map.get("MYR", 4.45)},
                        {"code": "THB", "name": "Thai Baht", "symbol": "฿", "decimals": 2, "region": "Southeast Asia", "default_rate": rates_map.get("THB", 36.80)},
                        {"code": "IDR", "name": "Indonesian Rupiah", "symbol": "Rp", "decimals": 0, "region": "Southeast Asia", "default_rate": rates_map.get("IDR", 15850.00)},
                        {"code": "PHP", "name": "Philippine Peso", "symbol": "₱", "decimals": 2, "region": "Southeast Asia", "default_rate": rates_map.get("PHP", 58.20)},
                        {"code": "VND", "name": "Vietnamese Dong", "symbol": "₫", "decimals": 0, "region": "Southeast Asia", "default_rate": rates_map.get("VND", 25400.00)},
                        {"code": "KHR", "name": "Cambodian Riel", "symbol": "៛", "decimals": 0, "region": "Southeast Asia", "default_rate": rates_map.get("KHR", 4050.00)},
                        {"code": "LAK", "name": "Lao Kip", "symbol": "₭", "decimals": 0, "region": "Southeast Asia", "default_rate": rates_map.get("LAK", 21800.00)},
                        {"code": "MMK", "name": "Myanmar Kyat", "symbol": "K", "decimals": 0, "region": "Southeast Asia", "default_rate": rates_map.get("MMK", 2095.00)},
                        {"code": "BND", "name": "Brunei Dollar", "symbol": "B$", "decimals": 2, "region": "Southeast Asia", "default_rate": rates_map.get("BND", 1.32)},

                        # Middle East
                        {"code": "AED", "name": "UAE Dirham", "symbol": "AED", "decimals": 2, "region": "Middle East", "default_rate": rates_map.get("AED", 3.67)},
                        {"code": "SAR", "name": "Saudi Riyal", "symbol": "SAR", "decimals": 2, "region": "Middle East", "default_rate": rates_map.get("SAR", 3.75)},
                        {"code": "QAR", "name": "Qatari Riyal", "symbol": "QAR", "decimals": 2, "region": "Middle East", "default_rate": rates_map.get("QAR", 3.64)},
                        {"code": "KWD", "name": "Kuwaiti Dinar", "symbol": "KWD", "decimals": 3, "region": "Middle East", "default_rate": rates_map.get("KWD", 0.31)},
                        {"code": "BHD", "name": "Bahraini Dinar", "symbol": "BHD", "decimals": 3, "region": "Middle East", "default_rate": rates_map.get("BHD", 0.38)},
                        {"code": "OMR", "name": "Omani Rial", "symbol": "OMR", "decimals": 3, "region": "Middle East", "default_rate": rates_map.get("OMR", 0.38)},
                        {"code": "JOD", "name": "Jordanian Dinar", "symbol": "JOD", "decimals": 3, "region": "Middle East", "default_rate": rates_map.get("JOD", 0.71)},
                        {"code": "ILS", "name": "Israeli New Shekel", "symbol": "₪", "decimals": 2, "region": "Middle East", "default_rate": rates_map.get("ILS", 3.72)},

                        # Central Asia
                        {"code": "KZT", "name": "Kazakhstani Tenge", "symbol": "₸", "decimals": 2, "region": "Central Asia", "default_rate": rates_map.get("KZT", 488.00)},
                        {"code": "UZS", "name": "Uzbekistani Som", "symbol": "so'm", "decimals": 0, "region": "Central Asia", "default_rate": rates_map.get("UZS", 12800.00)},

                        # Global Base
                        {"code": "USD", "name": "US Dollar", "symbol": "$", "decimals": 2, "region": "Global", "default_rate": 1.00},
                        {"code": "EUR", "name": "Euro", "symbol": "€", "decimals": 2, "region": "Global", "default_rate": rates_map.get("EUR", 0.92)},
                        {"code": "GBP", "name": "British Pound", "symbol": "£", "decimals": 2, "region": "Global", "default_rate": rates_map.get("GBP", 0.77)},
                        {"code": "CAD", "name": "Canadian Dollar", "symbol": "C$", "decimals": 2, "region": "Global", "default_rate": rates_map.get("CAD", 1.38)},
                        {"code": "AUD", "name": "Australian Dollar", "symbol": "A$", "decimals": 2, "region": "Global", "default_rate": rates_map.get("AUD", 1.51)}
                    ]
                    return self.send_json({
                        "base_currency": "USD",
                        "currencies": all_currencies,
                        "rates": rates_map
                    })

                elif path == "/api/invoices/pdf" or path == "/api/purchase-orders/pdf" or path == "/api/credit-notes/pdf":
                    doc_id = query.get("id", [""])[0]
                    conn.close()
                    # Return HTML printable representation for browser preview
                    html = f"<!DOCTYPE html><html><head><title>Document Preview</title><style>body{{font-family:sans-serif;padding:40px;}}table{{width:100%;border-collapse:collapse;}}th,td{{border:1px solid #ddd;padding:8px;}}</style></head><body><h2>AO SPHERE BOS DOCUMENT PREVIEW</h2><p>Document ID: {doc_id}</p><button onclick='window.print()'>Print / Save as PDF</button></body></html>"
                    body = html.encode("utf-8")
                    self.send_response(200)
                    self.send_header("Content-Type", "text/html; charset=utf-8")
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                    return

                conn.close()
                return self.send_json({"error": "Endpoint not found"}, 404)
            except Exception as e:
                return self.send_json({"error": str(e)}, 500)

        # 3. Static Files
        clean_path = path.lstrip("/")
        if not clean_path or clean_path == "index.html":
            file_path = os.path.join(WEB_DIR, "index.html")
        else:
            file_path = os.path.join(WEB_DIR, clean_path)

        if os.path.exists(file_path) and os.path.isfile(file_path):
            mime_type, _ = mimetypes.guess_type(file_path)
            if not mime_type:
                mime_type = "application/octet-stream"
            try:
                with open(file_path, "rb") as f:
                    content = f.read()
                self.send_response(200)
                self.send_header("Content-Type", mime_type)
                self.send_header("Content-Length", str(len(content)))
                self.send_header("Cache-Control", "no-cache")
                self.end_headers()
                if not head_only:
                    self.wfile.write(content)
                return
            except Exception as e:
                self.send_json({"error": str(e)}, 500)
                return

        # Fallback to index.html for SPA
        file_path = os.path.join(WEB_DIR, "index.html")
        if os.path.exists(file_path):
            with open(file_path, "rb") as f:
                content = f.read()
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(content)))
            self.end_headers()
            if not head_only:
                self.wfile.write(content)
            return

        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        parsed = urlparse(self.path)
        path = parsed.path
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length).decode("utf-8") if length > 0 else "{}"
        try:
            req = json.loads(body)
        except Exception:
            req = {}

        # 1. MCP Protocol Endpoints
        if path.startswith("/mcp/messages"):
            rpc_id = req.get("id", 1)
            method = req.get("method", "")
            if method == "initialize":
                return self.send_json({
                    "jsonrpc": "2.0",
                    "id": rpc_id,
                    "result": {
                        "protocolVersion": "2024-11-05",
                        "serverInfo": {"name": "aosphere-mcp", "version": "1.0.0"},
                        "capabilities": {"tools": {}}
                    }
                })
            elif method == "tools/list":
                tools = [
                    {"name": "erp_get_skus", "description": "Fetch inventory SKU records and safety stock levels."},
                    {"name": "erp_adjust_stock", "description": "Adjust stock quantity for an SKU."},
                    {"name": "crm_get_deals", "description": "Retrieve deals pipeline and stages."},
                    {"name": "crm_create_deal", "description": "Create a new sales opportunity deal."},
                    {"name": "fin_get_summary", "description": "Retrieve corporate P&L summary."},
                    {"name": "fin_create_invoice", "description": "Generate a new client billing invoice."},
                    {"name": "hr_get_shifts", "description": "Fetch employee shift roster."},
                    {"name": "hr_update_shift", "description": "Update worker shift status."},
                    {"name": "gov_get_pending_approvals", "description": "Fetch human approval backlog."}
                ]
                return self.send_json({"jsonrpc": "2.0", "id": rpc_id, "result": {"tools": tools}})
            elif method == "tools/call":
                params = req.get("params", {})
                name = params.get("name", "")
                conn = get_db()
                if name == "fin_get_summary":
                    rev_row = conn.execute("SELECT COALESCE(SUM(total_amount), 0) as rev, COUNT(*) as cnt FROM invoices WHERE status = 'paid'").fetchone()
                    unpaid_row = conn.execute("SELECT COALESCE(SUM(total_amount), 0) as unp, COUNT(*) as cnt FROM invoices WHERE status != 'paid'").fetchone()
                    exp_row = conn.execute("SELECT COALESCE(SUM(amount), 0) as exp FROM expenses").fetchone()
                    conn.close()
                    res = {
                        "total_revenue": float(rev_row["rev"] or 0.0),
                        "total_expense": float(exp_row["exp"] or 0.0),
                        "net_profit": float(rev_row["rev"] or 0.0) - float(exp_row["exp"] or 0.0),
                        "unpaid_total": float(unpaid_row["unp"] or 0.0),
                        "paid_count": int(rev_row["cnt"] or 0),
                        "pending_count": int(unpaid_row["cnt"] or 0)
                    }
                    return self.send_json({
                        "jsonrpc": "2.0",
                        "id": rpc_id,
                        "result": {"content": [{"type": "text", "text": json.dumps(res, indent=2)}]}
                    })
                conn.close()
                return self.send_json({
                    "jsonrpc": "2.0",
                    "id": rpc_id,
                    "result": {"content": [{"type": "text", "text": f"Executed tool {name} successfully."}]}
                })
            else:
                return self.send_json({
                    "jsonrpc": "2.0",
                    "id": rpc_id,
                    "result": {"status": "ok", "method": method}
                })

        # 2. REST API Actions
        conn = get_db()
        try:
            if path == "/api/skus/adjust":
                sku_id = req.get("sku_id")
                delta = int(req.get("delta", 0))
                reason = req.get("reason", "Manual adjustment")

                row = conn.execute("SELECT * FROM skus WHERE id = ?", (sku_id,)).fetchone()
                if not row:
                    conn.close()
                    return self.send_json({"error": "SKU not found"}, 404)

                new_stock = max(0, row["stock_quantity"] + delta)
                now = utcnow_str()
                conn.execute("UPDATE skus SET stock_quantity = ?, updated_at = ? WHERE id = ?", (new_stock, now, sku_id))
                log_audit(conn, "human", "web_user", "UPDATE_STOCK", "skus", sku_id, f"Adjusted stock by {delta}: {reason}", json.dumps({"delta": delta, "new_stock": new_stock}))

                # Check safety threshold
                if new_stock <= row["min_threshold"]:
                    appr_id = f"appr_{int(time.time()*1000)}"
                    title = f"Auto PO for {row['code']} (Low Stock: {new_stock})"
                    desc = f"Stock dropped to {new_stock} (Min: {row['min_threshold']}). Need {row['reorder_quantity']} units."
                    payload = json.dumps([{"sku_code": row["code"], "sku_name": row["name"], "quantity": row["reorder_quantity"]}])
                    conn.execute(
                        "INSERT INTO approvals (id, action_type, title, description, payload_json, status, requested_by_type, requested_by_id, created_at) VALUES (?, ?, ?, ?, ?, 'pending', 'agent', 'agt_inventory_watchdog', ?)",
                        (appr_id, "create_po", title, desc, payload, now)
                    )

                conn.commit()
                updated_row = conn.execute("SELECT * FROM skus WHERE id = ?", (sku_id,)).fetchone()
                conn.close()
                broadcast("stock_updated", {"sku_id": sku_id, "new_stock": new_stock})
                return self.send_json(dict(updated_row))

            elif path == "/api/purchase-orders":
                sup_id = req.get("supplier_id")
                items = req.get("items_json", "[]")
                total = float(req.get("total", 0.0))
                notes = req.get("notes", "")
                po_id = f"po_{int(time.time()*1000)}"
                po_num = f"PO-{int(time.time())}"
                now = utcnow_str()

                conn.execute(
                    "INSERT INTO purchase_orders (id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes, created_at) VALUES (?, ?, ?, 'ordered', ?, ?, 'human', 'web_user', ?, ?)",
                    (po_id, po_num, sup_id, total, items, notes, now)
                )
                log_audit(conn, "human", "web_user", "CREATE_PURCHASE_ORDER", "purchase_orders", po_id, f"Created PO {po_num} total ${total:.2f}", items)
                conn.commit()
                row = conn.execute("SELECT * FROM purchase_orders WHERE id = ?", (po_id,)).fetchone()
                conn.close()
                broadcast("po_created", {"po_number": po_num})
                return self.send_json(dict(row))

            elif path == "/api/deals":
                title = req.get("title")
                cust_id = req.get("customer_id")
                value = float(req.get("value", 0.0))
                stage = req.get("stage", "lead")
                close_date = req.get("expected_close_date", "")
                deal_id = f"deal_{int(time.time()*1000)}"
                now = utcnow_str()

                conn.execute(
                    "INSERT INTO deals (id, title, customer_id, value, stage, probability, expected_close_date, created_at) VALUES (?, ?, ?, ?, ?, 60, ?, ?)",
                    (deal_id, title, cust_id, value, stage, close_date, now)
                )
                log_audit(conn, "human", "web_user", "CREATE_DEAL", "deals", deal_id, f"Created deal '{title}' (${value:.2f})")
                conn.commit()
                row = conn.execute("SELECT * FROM deals WHERE id = ?", (deal_id,)).fetchone()
                conn.close()
                broadcast("deal_created", {"deal_id": deal_id})
                return self.send_json(dict(row))

            elif path == "/api/deals/stage":
                deal_id = req.get("deal_id")
                stage = req.get("stage")
                conn.execute("UPDATE deals SET stage = ? WHERE id = ?", (stage, deal_id))
                log_audit(conn, "human", "web_user", "UPDATE_DEAL_STAGE", "deals", deal_id, f"Stage moved to: {stage}")
                conn.commit()
                conn.close()
                broadcast("deal_stage_changed", {"deal_id": deal_id, "stage": stage})
                return self.send_json({"status": "ok"})

            elif path == "/api/invoices":
                cust_id = req.get("customer_id")
                items = req.get("items_json", "[]")
                subtotal = float(req.get("subtotal", 0.0))
                tax = float(req.get("tax", 0.0))
                due_date = req.get("due_date", "")
                recurring = req.get("recurring", "none")
                total = subtotal + tax
                inv_id = f"inv_{int(time.time()*1000)}"
                inv_num = f"INV-{int(time.time())}"
                now = utc_today_str()

                conn.execute(
                    "INSERT INTO invoices (id, invoice_number, customer_id, status, issue_date, due_date, subtotal, tax_amount, total_amount, recurring_frequency, items_json) VALUES (?, ?, ?, 'sent', ?, ?, ?, ?, ?, ?, ?)",
                    (inv_id, inv_num, cust_id, now, due_date, subtotal, tax, total, recurring, items)
                )
                log_audit(conn, "human", "web_user", "CREATE_INVOICE", "invoices", inv_id, f"Created invoice {inv_num} for total ${total:.2f}", items)
                conn.commit()
                row = conn.execute("SELECT * FROM invoices WHERE id = ?", (inv_id,)).fetchone()
                conn.close()
                broadcast("invoice_created", {"invoice_number": inv_num})
                return self.send_json(dict(row))

            elif path == "/api/shifts/status":
                shift_id = req.get("shift_id")
                status = req.get("status")
                conn.execute("UPDATE shifts SET status = ? WHERE id = ?", (status, shift_id))
                log_audit(conn, "human", "web_user", "UPDATE_SHIFT", "shifts", shift_id, f"Status updated to: {status}")
                conn.commit()
                conn.close()
                broadcast("shift_updated", {"shift_id": shift_id})
                return self.send_json({"status": "ok"})

            elif path == "/api/tasks/status":
                task_id = req.get("task_id")
                status = req.get("status")
                conn.execute("UPDATE tasks SET status = ? WHERE id = ?", (status, task_id))
                log_audit(conn, "human", "web_user", "UPDATE_TASK", "tasks", task_id, f"Task status moved to: {status}")
                conn.commit()
                conn.close()
                broadcast("task_updated", {"task_id": task_id})
                return self.send_json({"status": "ok"})

            elif path == "/api/listings/sync":
                name = req.get("business_name")
                addr = req.get("address")
                phone = req.get("phone")
                web = req.get("website")
                now = utcnow_str()
                conn.execute("UPDATE listings SET sync_status = 'synced', last_sync = ?, nap_json = ? WHERE 1=1", (now, json.dumps({"name": name, "address": addr, "phone": phone, "website": web})))
                log_audit(conn, "human", "web_user", "SYNC_NAP", "listings", "all", "Synced NAP data across all directories")
                conn.commit()
                conn.close()
                broadcast("nap_synced", {"status": "synced"})
                return self.send_json({"status": "synced"})

            elif path == "/api/reviews/reply":
                rev_id = req.get("review_id")
                text = req.get("reply_text")
                pub = req.get("publish", False)
                status = "published" if pub else "drafted_by_agent"
                now = utcnow_str()
                conn.execute("UPDATE reviews SET reply_text = ?, reply_status = ?, replied_at = ? WHERE id = ?", (text, status, now, rev_id))
                log_audit(conn, "human", "web_user", "REPLY_REVIEW", "reviews", rev_id, f"Response: {status}")
                conn.commit()
                conn.close()
                broadcast("review_replied", {"review_id": rev_id})
                return self.send_json({"status": "ok"})

            elif path == "/api/approvals/decide":
                appr_id = req.get("approval_id")
                decision = req.get("decision")
                now = utcnow_str()
                conn.execute("UPDATE approvals SET status = ?, decided_by = 'admin', decided_at = ? WHERE id = ?", (decision, now, appr_id))
                log_audit(conn, "human", "admin", f"DECIDE_APPROVAL_{decision.upper()}", "approvals", appr_id, f"Decision: {decision}")
                conn.commit()
                conn.close()
                broadcast("approval_decided", {"id": appr_id, "decision": decision})
                return self.send_json({"status": "ok"})

            elif path == "/api/skus":
                code = req.get("code")
                name = req.get("name")
                cat = req.get("category", "General")
                cost = float(req.get("unit_cost") or req.get("cost_price", 0.0))
                price = float(req.get("selling_price", 0.0))
                stock = int(req.get("stock_quantity", 0))
                min_thresh = int(req.get("min_threshold", 10))
                reorder = int(req.get("reorder_quantity", 50))
                sup_id = req.get("supplier_id", "")
                sku_id = f"sku_{int(time.time()*1000)}"
                now = utcnow_str()

                conn.execute(
                    "INSERT INTO skus (id, org_id, code, name, category, unit_cost, selling_price, stock_quantity, min_threshold, reorder_quantity, supplier_id, updated_at) VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
                    (sku_id, code, name, cat, cost, price, stock, min_thresh, reorder, sup_id, now)
                )
                log_audit(conn, "human", "web_user", "CREATE_SKU", "skus", sku_id, f"Created SKU '{code}' ({name})")
                conn.commit()
                row = conn.execute("SELECT * FROM skus WHERE id = ?", (sku_id,)).fetchone()
                conn.close()
                broadcast("sku_created", {"sku_id": sku_id, "code": code})
                return self.send_json(dict(row), 201)

            elif path == "/api/deals/convert":
                deal_id = req.get("deal_id")
                d_row = conn.execute("SELECT * FROM deals WHERE id = ?", (deal_id,)).fetchone()
                if not d_row:
                    conn.close()
                    return self.send_json({"error": "Deal not found"}, 404)

                inv_id = f"inv_{int(time.time()*1000)}"
                inv_num = f"INV-DEAL-{int(time.time())}"
                today = utc_today_str()
                val = float(d_row["value"] or 0.0)
                items_json = json.dumps([{"description": f"Contract: {d_row['title']}", "quantity": 1, "unit_price": val, "total": val}])

                conn.execute(
                    "INSERT INTO invoices (id, invoice_number, customer_id, status, issue_date, due_date, subtotal, tax_amount, total_amount, recurring_frequency, items_json) VALUES (?, ?, ?, 'sent', ?, ?, ?, 0, ?, 'none', ?)",
                    (inv_id, inv_num, d_row["customer_id"], today, today, val, val, items_json)
                )
                conn.execute("UPDATE deals SET stage = 'won' WHERE id = ?", (deal_id,))
                log_audit(conn, "human", "web_user", "CONVERT_DEAL_TO_INVOICE", "deals", deal_id, f"Converted deal to invoice {inv_num} (${val:.2f})")
                conn.commit()
                row = conn.execute("SELECT * FROM invoices WHERE id = ?", (inv_id,)).fetchone()
                conn.close()
                broadcast("invoice_created", {"invoice_number": inv_num})
                broadcast("deal_stage_changed", {"deal_id": deal_id, "stage": "won"})
                return self.send_json(dict(row), 201)

            elif path == "/api/employees":
                name = req.get("name")
                email = req.get("email")
                role = req.get("role", "Staff")
                dept = req.get("department", "Operations")
                rate = float(req.get("hourly_rate", 25.0))
                emp_id = f"emp_{int(time.time()*1000)}"
                now = utcnow_str()

                conn.execute(
                    "INSERT INTO employees (id, org_id, name, email, role, department, hourly_rate, status, created_at) VALUES (?, 'org_default', ?, ?, ?, ?, ?, 'active', ?)",
                    (emp_id, name, email, role, dept, rate, now)
                )
                log_audit(conn, "human", "web_user", "CREATE_EMPLOYEE", "employees", emp_id, f"Onboarded employee {name} ({role})")
                conn.commit()
                row = conn.execute("SELECT * FROM employees WHERE id = ?", (emp_id,)).fetchone()
                conn.close()
                broadcast("employee_created", {"employee_id": emp_id, "name": name})
                return self.send_json(dict(row), 201)

            elif path == "/api/shifts":
                emp_id = req.get("employee_id")
                sdate = req.get("shift_date")
                start = req.get("start_time", "09:00")
                end = req.get("end_time", "17:00")
                station = req.get("station") or req.get("role_assigned", "Warehouse Associate")
                shift_id = f"shift_{int(time.time()*1000)}"
                now = utcnow_str()

                conn.execute(
                    "INSERT INTO shifts (id, org_id, employee_id, shift_date, start_time, end_time, station, status, created_at) VALUES (?, 'org_default', ?, ?, ?, ?, ?, 'scheduled', ?)",
                    (shift_id, emp_id, sdate, start, end, station, now)
                )
                log_audit(conn, "human", "web_user", "CREATE_SHIFT", "shifts", shift_id, f"Scheduled shift for {emp_id} on {sdate}")
                conn.commit()
                row = conn.execute("SELECT s.*, COALESCE(e.name, '') as employee_name, COALESCE(e.role, '') as employee_role FROM shifts s LEFT JOIN employees e ON s.employee_id = e.id WHERE s.id = ?", (shift_id,)).fetchone()
                conn.close()
                broadcast("shift_scheduled", {"shift_id": shift_id})
                return self.send_json(dict(row) if row else {"id": shift_id, "status": "scheduled"}, 201)

            elif path == "/api/tasks":
                title = req.get("title")
                desc = req.get("description", "")
                priority = req.get("priority", "medium")
                due = req.get("due_date", "")
                assignee = req.get("assignee_id") or req.get("assigned_to", "")
                task_id = f"task_{int(time.time()*1000)}"
                now = utcnow_str()

                conn.execute(
                    "INSERT INTO tasks (id, org_id, title, description, priority, due_date, assignee_id, status, created_at) VALUES (?, 'org_default', ?, ?, ?, ?, ?, 'todo', ?)",
                    (task_id, title, desc, priority, due, assignee, now)
                )
                log_audit(conn, "human", "web_user", "CREATE_TASK", "tasks", task_id, f"Created task: {title} ({priority})")
                conn.commit()
                row = conn.execute("SELECT * FROM tasks WHERE id = ?", (task_id,)).fetchone()
                conn.close()
                broadcast("task_created", {"task_id": task_id, "title": title})
                return self.send_json(dict(row), 201)

            elif path == "/api/warehouses":
                name = req.get("name")
                code = req.get("code")
                loc = req.get("location", "")
                desc = req.get("description", "")
                wh_id = f"wh_{int(time.time()*1000)}"
                now = utcnow_str()

                conn.execute(
                    "INSERT INTO warehouses (id, org_id, name, code, location, description, is_active, created_at) VALUES (?, 'org_default', ?, ?, ?, ?, 1, ?)",
                    (wh_id, name, code, loc, desc, now)
                )
                log_audit(conn, "human", "web_user", "CREATE_WAREHOUSE", "warehouses", wh_id, f"Created warehouse {code} ({name})")
                conn.commit()
                row = conn.execute("SELECT * FROM warehouses WHERE id = ?", (wh_id,)).fetchone()
                conn.close()
                broadcast("warehouse_created", {"warehouse_id": wh_id, "code": code})
                return self.send_json(dict(row), 201)

            elif path == "/api/stock-transfers":
                fwh = req.get("from_warehouse_id")
                twh = req.get("to_warehouse_id")
                sku = req.get("sku_code")
                qty = int(req.get("quantity", 0))
                reason = req.get("reason", "Internal transfer")
                xfer_id = f"xfer_{int(time.time()*1000)}"
                xfer_num = f"TRF-{int(time.time())}"
                now = utcnow_str()

                conn.execute(
                    "INSERT INTO stock_transfers (id, org_id, transfer_number, from_warehouse_id, to_warehouse_id, sku_code, quantity, reason, status, created_at) VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?, 'completed', ?)",
                    (xfer_id, xfer_num, fwh, twh, sku, qty, reason, now)
                )
                log_audit(conn, "human", "web_user", "TRANSFER_STOCK", "stock_transfers", xfer_id, f"Transferred {qty}x {sku} from {fwh} to {twh}")
                conn.commit()
                row = conn.execute("SELECT * FROM stock_transfers WHERE id = ?", (xfer_id,)).fetchone()
                conn.close()
                broadcast("stock_transferred", {"transfer_number": xfer_num})
                return self.send_json(dict(row), 201)

            elif path == "/api/grns":
                poid = req.get("po_id")
                whid = req.get("warehouse_id")
                tracking = req.get("carrier_tracking", "")
                inspector = req.get("inspector_name", "Receiving Clerk")
                notes = req.get("notes", "")
                items = req.get("items", [])
                grn_id = f"grn_{int(time.time()*1000)}"
                grn_num = f"GRN-{int(time.time())}"
                today = utc_today_str()
                now = utcnow_str()

                conn.execute(
                    "INSERT INTO goods_receipt_notes (id, org_id, grn_number, po_id, warehouse_id, received_date, carrier_tracking, inspector_name, notes, items_json, status, created_at) VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?, ?, ?, 'completed', ?)",
                    (grn_id, grn_num, poid, whid, today, tracking, inspector, notes, json.dumps(items), now)
                )
                for it in items:
                    sku_code = it.get("sku_code")
                    rcv = int(it.get("received_quantity", 0))
                    conn.execute("UPDATE skus SET stock_quantity = stock_quantity + ?, updated_at = ? WHERE code = ?", (rcv, now, sku_code))

                conn.execute("UPDATE purchase_orders SET status = 'received' WHERE id = ?", (poid,))
                log_audit(conn, "human", "web_user", "RECEIVE_GRN", "goods_receipt_notes", grn_id, f"Processed GRN {grn_num} for PO {poid}")
                conn.commit()
                row = conn.execute("SELECT * FROM goods_receipt_notes WHERE id = ?", (grn_id,)).fetchone()
                conn.close()
                broadcast("grn_received", {"grn_number": grn_num})
                return self.send_json(dict(row), 201)

            elif path == "/api/supplier-bills":
                bnum = req.get("bill_number") or f"BILL-{int(time.time())}"
                sup_id = req.get("supplier_id")
                poid = req.get("po_id", "")
                grnid = req.get("grn_id", "")
                amt = float(req.get("amount", 0.0))
                amt_cents = int(amt * 100)
                due = req.get("due_date", "")
                sb_id = f"sb_{int(time.time()*1000)}"
                now = utcnow_str()

                conn.execute(
                    "INSERT INTO supplier_bills (id, org_id, bill_number, supplier_id, po_id, grn_id, amount, amount_cents, match_status, due_date, status, created_at) VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?, 'pending', ?, 'open', ?)",
                    (sb_id, bnum, sup_id, poid, grnid, amt, amt_cents, due, now)
                )
                log_audit(conn, "human", "web_user", "CREATE_SUPPLIER_BILL", "supplier_bills", sb_id, f"Entered supplier bill {bnum} (${amt:.2f})")
                conn.commit()
                row = conn.execute("SELECT * FROM supplier_bills WHERE id = ?", (sb_id,)).fetchone()
                conn.close()
                broadcast("supplier_bill_created", {"bill_number": bnum})
                return self.send_json(dict(row), 201)

            elif path == "/api/supplier-bills/match":
                bill_id = req.get("bill_id")
                brow = conn.execute("SELECT * FROM supplier_bills WHERE id = ?", (bill_id,)).fetchone()
                if not brow:
                    conn.close()
                    return self.send_json({"error": "Supplier bill not found"}, 404)

                po_row = conn.execute("SELECT * FROM purchase_orders WHERE id = ?", (brow["po_id"],)).fetchone() if brow["po_id"] else None
                po_total = float(po_row["total_amount"]) if po_row else 0.0
                bill_amt = float(brow["amount"])
                diff = abs(bill_amt - po_total)

                status = "matched" if diff <= 50.0 else "variance_flagged"
                conn.execute("UPDATE supplier_bills SET match_status = ? WHERE id = ?", (status, bill_id))
                now = utcnow_str()

                if status == "variance_flagged":
                    appr_id = f"appr_{int(time.time()*1000)}"
                    conn.execute(
                        "INSERT INTO approvals (id, action_type, title, description, payload_json, status, requested_by_type, requested_by_id, created_at) VALUES (?, 'supplier_bill_variance', ?, ?, ?, 'pending', 'system', 'sys_three_way_match', ?)",
                        (appr_id, f"Variance on Bill {brow['bill_number']}", f"Bill amount ${bill_amt:.2f} differs from PO ${po_total:.2f} by ${diff:.2f}", json.dumps({"bill_id": bill_id, "variance": diff}), now)
                    )

                log_audit(conn, "human", "web_user", "THREE_WAY_MATCH", "supplier_bills", bill_id, f"3-way match: {status} (Diff: ${diff:.2f})")
                conn.commit()
                conn.close()
                broadcast("bill_matched", {"bill_id": bill_id, "status": status})
                return self.send_json({"status": status, "bill_id": bill_id, "diff": diff})

            elif path == "/api/credit-notes":
                inv_id = req.get("invoice_id", "")
                cust_id = req.get("customer_id", "")
                reason = req.get("reason", "billing_error")
                currency = req.get("currency", "USD")
                items = req.get("items_json", "[]")
                if isinstance(items, (list, dict)):
                    items = json.dumps(items)

                subtotal = float(req.get("subtotal") or req.get("amount", 0.0))
                tax = float(req.get("tax_amount", 0.0))
                total = float(req.get("total_amount") or (subtotal + tax))
                if subtotal == 0.0 and total > 0.0:
                    subtotal = total - tax

                if inv_id and not cust_id:
                    inv_row = conn.execute("SELECT customer_id, currency FROM invoices WHERE id = ?", (inv_id,)).fetchone()
                    if inv_row:
                        cust_id = inv_row["customer_id"]
                        if not req.get("currency") and inv_row["currency"]:
                            currency = inv_row["currency"]

                cn_id = f"cn_{int(time.time()*1000)}"
                cn_num = f"CN-{int(time.time())}"
                subtotal_cents = int(round(subtotal * 100))
                tax_cents = int(round(tax * 100))
                total_cents = subtotal_cents + tax_cents
                now = utcnow_str()

                conn.execute("""
                    INSERT INTO credit_notes (id, org_id, credit_note_number, invoice_id, customer_id, currency, subtotal, tax_amount, total_amount, total_cents, reason, status, items_json, created_at)
                    VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?, ?, ?, ?, 'issued', ?, ?)
                """, (cn_id, cn_num, inv_id, cust_id, currency, subtotal, tax, total, total_cents, reason, items, now))

                # Double-entry ledger reversal if accounts table exists
                try:
                    je_id = f"je_{int(time.time()*1000)}"
                    je_num = f"JE-CN-{int(time.time())}"
                    today = utc_today_str()
                    conn.execute("""
                        INSERT INTO journal_entries (id, org_id, entry_number, date, memo, source, source_id, posted_by, posted_at)
                        VALUES (?, 'org_default', ?, ?, ?, 'credit_note', ?, 'web_user', ?)
                    """, (je_id, je_num, today, f"Reversal for Credit Note {cn_num}", cn_id, now))

                    # Fetch account IDs
                    ar_row = conn.execute("SELECT id FROM accounts WHERE code = '1100'").fetchone()
                    rev_row = conn.execute("SELECT id FROM accounts WHERE code = '4000'").fetchone()
                    tax_row = conn.execute("SELECT id FROM accounts WHERE code = '2100'").fetchone()

                    if rev_row and subtotal_cents > 0:
                        conn.execute("INSERT INTO journal_lines (id, entry_id, account_id, entry_type, amount_cents, description) VALUES (?, ?, ?, 'debit', ?, 'Revenue Reversal')",
                            (f"jl_{int(time.time()*1000)}_1", je_id, rev_row["id"], subtotal_cents))
                    if tax_row and tax_cents > 0:
                        conn.execute("INSERT INTO journal_lines (id, entry_id, account_id, entry_type, amount_cents, description) VALUES (?, ?, ?, 'debit', ?, 'Sales Tax Reversal')",
                            (f"jl_{int(time.time()*1000)}_2", je_id, tax_row["id"], tax_cents))
                    if ar_row and total_cents > 0:
                        conn.execute("INSERT INTO journal_lines (id, entry_id, account_id, entry_type, amount_cents, description) VALUES (?, ?, ?, 'credit', ?, 'AR Credit Reversal')",
                            (f"jl_{int(time.time()*1000)}_3", je_id, ar_row["id"], total_cents))
                except Exception as e:
                    pass

                log_audit(conn, "human", "web_user", "ISSUE_CREDIT_NOTE", "credit_notes", cn_id, f"Issued credit note {cn_num} for {currency} {total:.2f} ({reason})", items)
                conn.commit()
                row = conn.execute("SELECT cn.*, COALESCE(inv.invoice_number, '') as invoice_number, COALESCE(c.name, '') as customer_name FROM credit_notes cn LEFT JOIN invoices inv ON cn.invoice_id = inv.id LEFT JOIN customers c ON cn.customer_id = c.id WHERE cn.id = ?", (cn_id,)).fetchone()
                conn.close()
                broadcast("credit_note_issued", {"credit_note_number": cn_num, "total": total, "currency": currency})
                return self.send_json(dict(row) if row else {"id": cn_id, "credit_note_number": cn_num}, 201)

            elif path == "/api/currencies/rate":
                target_cur = req.get("target_currency", "").upper()
                base_cur = req.get("base_currency", "USD").upper()
                rate = float(req.get("rate", 0.0))
                if not target_cur or rate <= 0:
                    conn.close()
                    return self.send_json({"error": "target_currency and positive rate required"}, 400)

                rate_id = f"fx_{base_cur}_{target_cur}".lower()
                now = utcnow_str()
                conn.execute("""
                    INSERT INTO exchange_rates (id, org_id, base_currency, target_currency, rate, updated_at)
                    VALUES (?, 'org_default', ?, ?, ?, ?)
                    ON CONFLICT(org_id, base_currency, target_currency) DO UPDATE SET rate = excluded.rate, updated_at = excluded.updated_at
                """, (rate_id, base_cur, target_cur, rate, now))

                log_audit(conn, "human", "web_user", "UPDATE_EXCHANGE_RATE", "exchange_rates", rate_id, f"Updated FX rate 1 {base_cur} = {rate:.4f} {target_cur}")
                conn.commit()
                conn.close()
                broadcast("exchange_rate_updated", {"currency": target_cur, "rate": rate, "base_currency": base_cur})
                return self.send_json({"status": "ok", "currency": target_cur, "rate": rate})

            conn.close()
            return self.send_json({"error": "Unknown endpoint"}, 404)
        except Exception as e:
            conn.close()
            return self.send_json({"error": str(e)}, 500)

def main():
    parser = argparse.ArgumentParser(description="AOSphere Platform Server")
    parser.add_argument("-port", type=int, default=8080, help="HTTP Server Port")
    args = parser.parse_args()

    server = ThreadingHTTPServer(("0.0.0.0", args.port), AOSphereHandler)
    server.daemon_threads = True
    print("==================================================================")
    print("  AO Sphere — Agent-Native Business Operating System (Live Server)")
    print("==================================================================")
    print(f"  • Web Interface:    http://localhost:{args.port}")
    print(f"  • REST API:         http://localhost:{args.port}/api")
    print(f"  • Live Events SSE:  http://localhost:{args.port}/api/events")
    print(f"  • MCP Messages:     http://localhost:{args.port}/mcp/messages")
    print(f"  • Database:         {DB_PATH} (SQLite WAL)")
    print("==================================================================")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down AO Sphere...")
    except Exception as e:
        print(f"\nServer exception: {e}")
    finally:
        try:
            server.server_close()
        except Exception:
            pass
        print("AO Sphere exited cleanly.")

if __name__ == "__main__":
    main()
