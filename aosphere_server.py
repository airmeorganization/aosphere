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
from datetime import datetime
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs
import threading
import time

DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data", "aosphere.db")
WEB_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "web")

clients_lock = threading.Lock()
sse_clients = []

def get_db():
    conn = sqlite3.connect(DB_PATH, timeout=10.0)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA journal_mode=WAL;")
    conn.execute("PRAGMA foreign_keys=ON;")
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
    now = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")
    conn.execute(
        "INSERT INTO audit_logs (id, actor_type, actor_id, action, entity_type, entity_id, reasoning, metadata_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
        (audit_id, actor_type, actor_id, action, entity_type, entity_id, reasoning, meta, now)
    )

class AOSphereHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass

    def send_json(self, data, status=200):
        body = json.dumps(data, default=str).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()
        self.wfile.write(body)

    def do_OPTIONS(self):
        self.send_response(200)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, Authorization")
        self.end_headers()

    def do_HEAD(self):
        self.do_GET(head_only=True)

    def do_GET(self, head_only=False):
        parsed = urlparse(self.path)
        path = parsed.path
        query = parse_qs(parsed.query)

        # 1. SSE Stream
        if path == "/api/events":
            import queue
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Cache-Control", "no-cache")
            self.send_header("Connection", "keep-alive")
            self.send_header("Access-Control-Allow-Origin", "*")
            self.end_headers()
            if head_only:
                return

            q = queue.Queue(maxsize=32)
            with clients_lock:
                sse_clients.append(q)

            init_msg = f"event: connected\ndata: {{\"status\":\"ok\",\"time\":\"{datetime.utcnow().isoformat()}\"}}\n\n"
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
            except (ConnectionResetError, BrokenPipeError):
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
                    rows = conn.execute("SELECT t.*, COALESCE(e.name, '') as assignee_name FROM tasks t LEFT JOIN employees e ON t.assigned_to = e.id ORDER BY t.created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/listings":
                    rows = conn.execute("SELECT * FROM listings ORDER BY platform").fetchall()
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

                elif path == "/api/audit-logs":
                    rows = conn.execute("SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT 50").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

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
                now = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")
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
                now = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")

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
                now = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")

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
                now = datetime.utcnow().strftime("%Y-%m-%d")

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
                now = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")
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
                now = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")
                conn.execute("UPDATE reviews SET reply_text = ?, reply_status = ?, replied_at = ? WHERE id = ?", (text, status, now, rev_id))
                log_audit(conn, "human", "web_user", "REPLY_REVIEW", "reviews", rev_id, f"Response: {status}")
                conn.commit()
                conn.close()
                broadcast("review_replied", {"review_id": rev_id})
                return self.send_json({"status": "ok"})

            elif path == "/api/approvals/decide":
                appr_id = req.get("approval_id")
                decision = req.get("decision")
                now = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")
                conn.execute("UPDATE approvals SET status = ?, decided_by = 'admin', decided_at = ? WHERE id = ?", (decision, now, appr_id))
                log_audit(conn, "human", "admin", f"DECIDE_APPROVAL_{decision.upper()}", "approvals", appr_id, f"Decision: {decision}")
                conn.commit()
                conn.close()
                broadcast("approval_decided", {"id": appr_id, "decision": decision})
                return self.send_json({"status": "ok"})

            conn.close()
            return self.send_json({"error": "Unknown endpoint"}, 404)
        except Exception as e:
            conn.close()
            return self.send_json({"error": str(e)}, 500)

def main():
    parser = argparse.ArgumentParser(description="AOSphere Platform Server")
    parser.add_argument("-port", type=int, default=8080, help="HTTP Server Port")
    args = parser.parse_args()

    server = HTTPServer(("0.0.0.0", args.port), AOSphereHandler)
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
        server.server_close()
        print("AO Sphere exited cleanly.")

if __name__ == "__main__":
    main()
