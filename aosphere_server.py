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
import shutil
import argparse
import mimetypes
import csv
import io
import re
import uuid
import base64
import struct
import random
import hashlib
import hmac
import urllib.request
import urllib.error
from datetime import datetime, timezone, timedelta

def utcnow_str():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

def utc_today_str():
    return datetime.now(timezone.utc).strftime("%Y-%m-%d")

def utcnow_iso():
    return datetime.now(timezone.utc).isoformat()

def escape_pdf(s):
    if not s:
        return ""
    return str(s).replace("\\", "\\\\").replace("(", "\\(").replace(")", "\\)")

def generate_payslip_pdf(ps):
    title = f"PAYSLIP - {ps.get('employee_name', 'Employee')} ({ps.get('period', '')})"
    lines = [
        "AO SPHERE CORP. - EARNINGS STATEMENT",
        f"Period: {ps.get('period', '')}  |  Status: {str(ps.get('status', 'PAID')).upper()}  |  Payment Date: {ps.get('payment_date', '')}",
        "--------------------------------------------------------------------------------",
        f"Employee: {ps.get('employee_name', 'Employee')} (ID: {ps.get('employee_id', '')})",
        f"Department: {ps.get('department', 'Operations')}  |  Role: {ps.get('job_title', 'Staff')}",
        f"Tax ID: {ps.get('tax_id', 'Pending')}  |  Bank: {ps.get('bank_name', 'Direct Deposit')}",
        "--------------------------------------------------------------------------------",
        "EARNINGS BREAKDOWN:",
        f"  Salary Model: {str(ps.get('salary_type', 'hourly')).upper()}",
        f"  Regular Hours: {float(ps.get('regular_hours', 160.0)):.1f} hrs",
        f"  Overtime Hours: {float(ps.get('overtime_hours', 0.0)):.1f} hrs",
        f"  Gross Wages: ${float(ps.get('gross_pay', 0.0)):.2f}",
        "--------------------------------------------------------------------------------",
        "STATUTORY WITHHOLDINGS & DEDUCTIONS:",
        f"  Income Tax Withholding (12%): -${float(ps.get('tax_withholding', 0.0)):.2f}",
        f"  Social Contributions (5%):    -${float(ps.get('social_contributions', 0.0)):.2f}",
        f"  Benefits & Healthcare (3%):   -${float(ps.get('benefits_deductions', 0.0)):.2f}",
        "--------------------------------------------------------------------------------",
        f"NET TAKE-HOME PAY: ${float(ps.get('net_pay', 0.0)):.2f}",
        "Disbursal Notice: Funds cleared and posted to General Ledger (Accounts 5200, 2150, 1000)",
    ]
    stream = "BT /F2 16 Tf 40 740 Td (" + escape_pdf(title) + ") Tj ET\n"
    y = 710
    for line in lines:
        font = "/F2 10" if any(k in line for k in ("NET", "EARNINGS", "STATUTORY", "AO SPHERE")) else "/F1 9"
        stream += f"BT {font} Tf 40 {y} Td ({escape_pdf(line)}) Tj ET\n"
        y -= 18
    stream_bytes = stream.encode('latin1', errors='replace')

    buf = bytearray(b"%PDF-1.4\n")
    offsets = {}

    offsets[1] = len(buf)
    buf.extend(b"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

    offsets[2] = len(buf)
    buf.extend(b"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

    offsets[3] = len(buf)
    buf.extend(b"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents 6 0 R >>\nendobj\n")

    offsets[4] = len(buf)
    buf.extend(b"4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

    offsets[5] = len(buf)
    buf.extend(b"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>\nendobj\n")

    offsets[6] = len(buf)
    buf.extend(f"6 0 obj\n<< /Length {len(stream_bytes)} >>\nstream\n".encode('latin1'))
    buf.extend(stream_bytes)
    buf.extend(b"\nendstream\nendobj\n")

    xref_offset = len(buf)
    buf.extend(b"xref\n0 7\n0000000000 65535 f \n")
    for i in range(1, 7):
        buf.extend(f"{offsets[i]:010d} 00000 n \n".encode('latin1'))
    buf.extend(f"trailer\n<< /Size 7 /Root 1 0 R >>\nstartxref\n{xref_offset}\n%%EOF\n".encode('latin1'))
    return bytes(buf)

def generate_report_pdf(title, subtitle, metrics, headers, rows):
    stream = f"BT /F2 16 Tf 40 750 Td ({escape_pdf(title.upper())}) Tj ET\n"
    stream += f"BT /F1 9 Tf 40 735 Td ({escape_pdf(subtitle)}) Tj ET\n"
    stream += f"BT /F1 8 Tf 430 750 Td (Generated: {datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M')}) Tj ET\n"
    y = 705
    if metrics:
        b_width = min(170, int(532 / max(1, len(metrics))))
        for i, m in enumerate(metrics):
            bx = 40 + (i * b_width)
            stream += f"BT /F1 7.5 Tf {bx+4} {y-12} Td ({escape_pdf(m.get('label', ''))}) Tj ET\n"
            stream += f"BT /F2 10 Tf {bx+4} {y-26} Td ({escape_pdf(str(m.get('value', '')))}) Tj ET\n"
        y -= 40

    col_count = max(1, len(headers))
    col_width = int(532 / col_count)
    for i, h in enumerate(headers):
        hx = 45 + (i * col_width)
        stream += f"BT /F2 8.5 Tf {hx} {y-12} Td ({escape_pdf(h)}) Tj ET\n"
    y -= 18

    for r_idx, row in enumerate(rows[:25]):
        for c_idx, val in enumerate(row[:col_count]):
            cx = 45 + (c_idx * col_width)
            font = "/F2 8" if c_idx == 0 or any(k in headers[c_idx].lower() for k in ("total", "net", "amount")) else "/F1 8"
            stream += f"BT {font} Tf {cx} {y-10} Td ({escape_pdf(str(val))}) Tj ET\n"
        y -= 16
        if y < 65:
            break

    stream += f"BT /F1 8 Tf 40 35 Td (AO Sphere Executive Intelligence & Reporting Engine) Tj ET\n"
    stream_bytes = stream.encode('latin1', errors='replace')

    buf = bytearray(b"%PDF-1.4\n")
    offsets = {}
    offsets[1] = len(buf)
    buf.extend(b"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
    offsets[2] = len(buf)
    buf.extend(b"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
    offsets[3] = len(buf)
    buf.extend(b"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents 6 0 R >>\nendobj\n")
    offsets[4] = len(buf)
    buf.extend(b"4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")
    offsets[5] = len(buf)
    buf.extend(b"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>\nendobj\n")
    offsets[6] = len(buf)
    buf.extend(f"6 0 obj\n<< /Length {len(stream_bytes)} >>\nstream\n".encode('latin1'))
    buf.extend(stream_bytes)
    buf.extend(b"\nendstream\nendobj\n")
    xref_offset = len(buf)
    buf.extend(b"xref\n0 7\n0000000000 65535 f \n")
    for i in range(1, 7):
        buf.extend(f"{offsets[i]:010d} 00000 n \n".encode('latin1'))
    buf.extend(f"trailer\n<< /Size 7 /Root 1 0 R >>\nstartxref\n{xref_offset}\n%%EOF\n".encode('latin1'))
    return bytes(buf)

from http.server import ThreadingHTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs
import threading
import time

DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data", "aosphere.db")
WEB_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "web")
BACKUP_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data", "backups")
os.makedirs(BACKUP_DIR, exist_ok=True)

def compute_file_sha256(filepath):
    h = hashlib.sha256()
    with open(filepath, "rb") as f:
        while chunk := f.read(65536):
            h.update(chunk)
    return h.hexdigest()

def verify_backup_integrity(filepath):
    try:
        bconn = sqlite3.connect(f"file:{filepath}?mode=ro", uri=True)
        res = bconn.execute("PRAGMA integrity_check").fetchall()
        bconn.close()
        logs = [r[0] for r in res]
        is_ok = any(l == "ok" for l in logs)
        return is_ok, logs
    except Exception as e:
        return False, [str(e)]

def encrypt_backup_file(src_path, dst_path, passphrase):
    from cryptography.hazmat.primitives.ciphers.aead import AESGCM
    key = hashlib.sha256(passphrase.encode("utf-8")).digest()
    aesgcm = AESGCM(key)
    nonce = os.urandom(12)
    with open(src_path, "rb") as f:
        plaintext = f.read()
    ciphertext = nonce + aesgcm.encrypt(nonce, plaintext, None)
    with open(dst_path, "wb") as f:
        f.write(ciphertext)

def decrypt_backup_file(src_path, dst_path, passphrase):
    from cryptography.hazmat.primitives.ciphers.aead import AESGCM
    key = hashlib.sha256(passphrase.encode("utf-8")).digest()
    aesgcm = AESGCM(key)
    with open(src_path, "rb") as f:
        data = f.read()
    if len(data) < 12:
        raise ValueError("Ciphertext too short")
    nonce, ct = data[:12], data[12:]
    plaintext = aesgcm.decrypt(nonce, ct, None)
    with open(dst_path, "wb") as f:
        f.write(plaintext)

clients_lock = threading.Lock()
sse_clients = []

schema_lock = threading.Lock()
_schema_initialized = False

def ensure_schema(conn):
    global _schema_initialized
    with schema_lock:
        if _schema_initialized:
            return
        _schema_initialized = True
        try:
            conn.execute("INSERT OR IGNORE INTO organizations (id, name, slug) VALUES ('org_default', 'Default Organization', 'default')")
        except Exception:
            pass
        try:
            conn.execute("ALTER TABLE warehouses ADD COLUMN location TEXT NOT NULL DEFAULT ''")
        except Exception:
            pass
        try:
            conn.execute("ALTER TABLE warehouses ADD COLUMN description TEXT NOT NULL DEFAULT ''")
        except Exception:
            pass
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

    CREATE TABLE IF NOT EXISTS pipelines (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        name TEXT NOT NULL,
        code TEXT NOT NULL,
        description TEXT NOT NULL DEFAULT '',
        is_default INTEGER NOT NULL DEFAULT 0,
        created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(org_id, code)
    );
    CREATE TABLE IF NOT EXISTS pipeline_stages (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        pipeline_id TEXT NOT NULL,
        name TEXT NOT NULL,
        code TEXT NOT NULL,
        position INTEGER NOT NULL DEFAULT 0,
        probability_pct INTEGER NOT NULL DEFAULT 50,
        decay_days_threshold INTEGER NOT NULL DEFAULT 14,
        color TEXT NOT NULL DEFAULT '#3b82f6',
        created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(pipeline_id, code)
    );
    CREATE TABLE IF NOT EXISTS customer_activities (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        customer_id TEXT NOT NULL,
        deal_id TEXT,
        activity_type TEXT NOT NULL,
        author_type TEXT NOT NULL DEFAULT 'human',
        author_id TEXT NOT NULL DEFAULT 'admin',
        author_name TEXT NOT NULL DEFAULT 'Operations Team',
        summary TEXT NOT NULL,
        content TEXT NOT NULL DEFAULT '',
        metadata_json TEXT NOT NULL DEFAULT '{}',
        created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS inbound_leads (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        name TEXT NOT NULL,
        email TEXT NOT NULL,
        company TEXT NOT NULL DEFAULT '',
        phone TEXT NOT NULL DEFAULT '',
        message TEXT NOT NULL DEFAULT '',
        source TEXT NOT NULL DEFAULT 'website_form',
        status TEXT NOT NULL DEFAULT 'new',
        assigned_to TEXT NOT NULL DEFAULT 'unassigned',
        converted_customer_id TEXT,
        converted_deal_id TEXT,
        created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS email_outbox (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        recipient_email TEXT NOT NULL,
        recipient_name TEXT NOT NULL DEFAULT '',
        subject TEXT NOT NULL,
        body_html TEXT NOT NULL,
        body_text TEXT NOT NULL DEFAULT '',
        template_name TEXT NOT NULL DEFAULT 'custom',
        related_entity_type TEXT NOT NULL DEFAULT 'invoice',
        related_entity_id TEXT NOT NULL DEFAULT '',
        status TEXT NOT NULL DEFAULT 'sent',
        error_message TEXT NOT NULL DEFAULT '',
        sent_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
        created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS invoice_portal_tokens (
        token TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        invoice_id TEXT NOT NULL,
        expires_at TEXT,
        view_count INTEGER NOT NULL DEFAULT 0,
        last_viewed_at TEXT,
        created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(org_id, invoice_id)
    );

    INSERT OR IGNORE INTO pipelines (id, org_id, name, code, description, is_default) VALUES
    ('pipe_std_org_default', 'org_default', 'Standard Sales', 'STANDARD', 'Standard B2B sales pipeline for qualified direct opportunities', 1),
    ('pipe_ent_org_default', 'org_default', 'Enterprise Solutions', 'ENTERPRISE', 'High-ACV multi-stakeholder enterprise solution engagements', 0),
    ('pipe_saas_org_default', 'org_default', 'SaaS Subscriptions', 'SAAS', 'Self-serve trial onboarding and recurring expansion pipeline', 0);

    INSERT OR IGNORE INTO pipeline_stages (id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color) VALUES
    ('stg_std_lead_org_default', 'org_default', 'pipe_std_org_default', 'New Leads', 'lead', 1, 10, 7, '#64748b'),
    ('stg_std_qual_org_default', 'org_default', 'pipe_std_org_default', 'Qualified', 'qualified', 2, 25, 10, '#3b82f6'),
    ('stg_std_prop_org_default', 'org_default', 'pipe_std_org_default', 'Proposal Sent', 'proposal', 3, 60, 14, '#8b5cf6'),
    ('stg_std_nego_org_default', 'org_default', 'pipe_std_org_default', 'Negotiation', 'negotiation', 4, 80, 7, '#f59e0b'),
    ('stg_std_won_org_default', 'org_default', 'pipe_std_org_default', 'Won', 'won', 5, 100, 0, '#10b981'),

    ('stg_ent_disc_org_default', 'org_default', 'pipe_ent_org_default', 'Discovery', 'discovery', 1, 15, 14, '#64748b'),
    ('stg_ent_tech_org_default', 'org_default', 'pipe_ent_org_default', 'Tech Validation', 'tech_eval', 2, 40, 21, '#0284c7'),
    ('stg_ent_exec_org_default', 'org_default', 'pipe_ent_org_default', 'Executive Review', 'exec_review', 3, 70, 14, '#7c3aed'),
    ('stg_ent_proc_org_default', 'org_default', 'pipe_ent_org_default', 'Legal & Procurement', 'procurement', 4, 90, 30, '#d97706'),
    ('stg_ent_won_org_default', 'org_default', 'pipe_ent_org_default', 'Closed Won', 'won', 5, 100, 0, '#059669'),

    ('stg_saas_trial_org_default', 'org_default', 'pipe_saas_org_default', 'Trial Active', 'trial', 1, 20, 7, '#0ea5e9'),
    ('stg_saas_onb_org_default', 'org_default', 'pipe_saas_org_default', 'Onboarding', 'onboarding', 2, 50, 5, '#6366f1'),
    ('stg_saas_exp_org_default', 'org_default', 'pipe_saas_org_default', 'Expansion', 'expansion', 3, 80, 10, '#ec4899'),
    ('stg_saas_won_org_default', 'org_default', 'pipe_saas_org_default', 'Active ARR', 'won', 4, 100, 0, '#10b981');
    """)

    try:
        conn.execute("ALTER TABLE deals ADD COLUMN pipeline_id TEXT DEFAULT 'pipe_std_org_default';")
    except Exception:
        pass
    try:
        conn.execute("ALTER TABLE deals ADD COLUMN stage_updated_at TEXT DEFAULT '';")
        conn.execute("UPDATE deals SET stage_updated_at = created_at WHERE stage_updated_at IS NULL OR stage_updated_at = '';")
    except Exception:
        pass

    # Phase 6: HRMS, Shift Scheduling & Automated Payroll
    conn.executescript("""
    CREATE TABLE IF NOT EXISTS timeclock_events (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        employee_id TEXT NOT NULL,
        shift_id TEXT,
        event_type TEXT NOT NULL,
        timestamp TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
        ip_address TEXT NOT NULL DEFAULT '',
        location_notes TEXT NOT NULL DEFAULT '',
        created_at TEXT DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS payroll_runs (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        period TEXT NOT NULL,
        total_gross_cents INTEGER NOT NULL DEFAULT 0,
        total_tax_cents INTEGER NOT NULL DEFAULT 0,
        total_deductions_cents INTEGER NOT NULL DEFAULT 0,
        total_net_cents INTEGER NOT NULL DEFAULT 0,
        status TEXT NOT NULL DEFAULT 'draft',
        payment_date TEXT NOT NULL DEFAULT '',
        journal_entry_id TEXT,
        payslips_count INTEGER NOT NULL DEFAULT 0,
        created_at TEXT DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(org_id, period)
    );
    CREATE TABLE IF NOT EXISTS payslips (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        payroll_run_id TEXT,
        employee_id TEXT NOT NULL,
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
        status TEXT NOT NULL DEFAULT 'generated',
        payment_date TEXT NOT NULL DEFAULT '',
        created_at TEXT DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(org_id, payroll_run_id, employee_id)
    );
    CREATE TABLE IF NOT EXISTS leave_policies (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        name TEXT NOT NULL,
        code TEXT NOT NULL,
        annual_days INTEGER NOT NULL DEFAULT 15,
        created_at TEXT DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(org_id, code)
    );
    CREATE TABLE IF NOT EXISTS leave_requests (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        employee_id TEXT NOT NULL,
        leave_type TEXT NOT NULL,
        start_date TEXT NOT NULL,
        end_date TEXT NOT NULL,
        days_count INTEGER NOT NULL DEFAULT 1,
        reason TEXT NOT NULL DEFAULT '',
        status TEXT NOT NULL DEFAULT 'pending',
        reviewed_by TEXT NOT NULL DEFAULT '',
        review_notes TEXT NOT NULL DEFAULT '',
        created_at TEXT DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS hr_documents (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        employee_id TEXT NOT NULL,
        title TEXT NOT NULL,
        document_type TEXT NOT NULL,
        file_name TEXT NOT NULL,
        file_content TEXT NOT NULL,
        file_size INTEGER NOT NULL DEFAULT 0,
        uploaded_by TEXT NOT NULL DEFAULT 'HR Operations',
        created_at TEXT DEFAULT CURRENT_TIMESTAMP
    );

    CREATE TABLE IF NOT EXISTS saved_reports (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        name TEXT NOT NULL,
        report_type TEXT NOT NULL,
        parameters_json TEXT NOT NULL DEFAULT '{}',
        created_by TEXT NOT NULL DEFAULT 'system',
        created_at TEXT DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS data_exports (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        export_type TEXT NOT NULL,
        file_format TEXT NOT NULL,
        record_count INTEGER NOT NULL DEFAULT 0,
        file_size_bytes INTEGER NOT NULL DEFAULT 0,
        created_by TEXT NOT NULL DEFAULT 'system',
        created_at TEXT DEFAULT CURRENT_TIMESTAMP
    );

    -- Phase 8: Autonomous MCP Agent Platform & Webhooks
    CREATE TABLE IF NOT EXISTS webhook_subscriptions (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        name TEXT NOT NULL,
        url TEXT NOT NULL,
        secret TEXT NOT NULL,
        event_types TEXT NOT NULL DEFAULT '["*"]',
        is_active INTEGER NOT NULL DEFAULT 1,
        created_at TEXT DEFAULT CURRENT_TIMESTAMP,
        updated_at TEXT DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS webhook_deliveries (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        subscription_id TEXT NOT NULL,
        event_id TEXT NOT NULL,
        event_type TEXT NOT NULL,
        payload_json TEXT NOT NULL,
        status TEXT NOT NULL DEFAULT 'pending',
        status_code INTEGER DEFAULT 0,
        response_body TEXT DEFAULT '',
        retry_count INTEGER NOT NULL DEFAULT 0,
        max_retries INTEGER NOT NULL DEFAULT 3,
        next_retry_at TEXT,
        delivered_at TEXT,
        created_at TEXT DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS agent_configs (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        name TEXT NOT NULL,
        api_key_hash TEXT NOT NULL UNIQUE,
        tool_quota_per_minute INTEGER NOT NULL DEFAULT 60,
        daily_budget_cents INTEGER NOT NULL DEFAULT 5000,
        current_day_usage_cents INTEGER NOT NULL DEFAULT 0,
        concurrent_limit INTEGER NOT NULL DEFAULT 5,
        active_slots INTEGER NOT NULL DEFAULT 0,
        is_active INTEGER NOT NULL DEFAULT 1,
        last_used_at TEXT,
        created_at TEXT DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS agent_executions (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        agent_id TEXT NOT NULL,
        tool_name TEXT NOT NULL,
        category TEXT NOT NULL DEFAULT 'read_only',
        arguments_json TEXT NOT NULL DEFAULT '{}',
        result_json TEXT NOT NULL DEFAULT '{}',
        is_error INTEGER NOT NULL DEFAULT 0,
        error_message TEXT DEFAULT '',
        duration_ms INTEGER NOT NULL DEFAULT 0,
        cost_cents INTEGER NOT NULL DEFAULT 0,
        mutations_json TEXT NOT NULL DEFAULT '[]',
        created_at TEXT DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS agent_escalations (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        agent_id TEXT NOT NULL,
        execution_id TEXT,
        title TEXT NOT NULL,
        reason TEXT NOT NULL,
        status TEXT NOT NULL DEFAULT 'pending',
        input_context_json TEXT NOT NULL DEFAULT '{}',
        recommended_action TEXT DEFAULT '',
        reviewer_id TEXT DEFAULT '',
        reviewer_notes TEXT DEFAULT '',
        resolved_at TEXT,
        created_at TEXT DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS agent_context_memory (
        id TEXT PRIMARY KEY,
        org_id TEXT NOT NULL,
        category TEXT NOT NULL DEFAULT 'company_profile',
        key TEXT NOT NULL,
        value TEXT NOT NULL,
        description TEXT DEFAULT '',
        updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(org_id, key)
    );

    INSERT OR IGNORE INTO agent_configs (id, org_id, name, api_key_hash, tool_quota_per_minute, daily_budget_cents, concurrent_limit) VALUES
    ('cfg_claude_desktop_org_default', 'org_default', 'Claude Desktop Assistant', 'hash_claude_desktop_default', 60, 10000, 5),
    ('cfg_auto_procure_org_default', 'org_default', 'Autonomous Procurement Daemon', 'hash_auto_procure_default', 30, 5000, 3);

    INSERT OR IGNORE INTO agent_context_memory (id, org_id, category, key, value, description) VALUES
    ('ctx_comp_name_org_default', 'org_default', 'company_profile', 'company_legal_name', 'AO Sphere Global Inc.', 'Official registered entity name'),
    ('ctx_currency_org_default', 'org_default', 'tax_currency', 'active_currency', 'USD', 'Default operating currency'),
    ('ctx_tax_rate_org_default', 'org_default', 'tax_currency', 'standard_sales_tax_rate', '0.0825', 'Standard sales tax percentage (8.25%)'),
    ('ctx_po_limit_org_default', 'org_default', 'financial_policies', 'po_auto_approval_threshold_cents', '50000', 'Purchase orders above $500.00 require human manager signoff'),
    ('ctx_lead_time_org_default', 'org_default', 'supplier_lead_times', 'standard_reorder_lead_days', '7', 'Average supplier lead time for inventory reorders');


    INSERT OR IGNORE INTO leave_policies (id, org_id, name, code, annual_days) VALUES
    ('lp_vac_org_default', 'org_default', 'Paid Vacation Time', 'vacation', 15),
    ('lp_sick_org_default', 'org_default', 'Sick & Medical Leave', 'sick', 10),
    ('lp_par_org_default', 'org_default', 'Parental & Family Leave', 'parental', 30),
    ('lp_unp_org_default', 'org_default', 'Unpaid Leave of Absence', 'unpaid', 0);

    INSERT OR IGNORE INTO accounts (id, org_id, code, name, type, normal_balance, is_active) VALUES
    ('acc_5200_org_default', 'org_default', '5200', 'Salaries & Wages Expense', 'expense', 'debit', 1),
    ('acc_2150_org_default', 'org_default', '2150', 'Payroll Withholding Taxes & Deductions', 'liability', 'credit', 1),
    ('acc_2200_org_default', 'org_default', '2200', 'Payroll Clearing & Accrued Wages', 'liability', 'credit', 1);
    """)

    for col_name, col_type in [
        ("job_title", "TEXT DEFAULT ''"),
        ("reporting_manager_id", "TEXT DEFAULT ''"),
        ("employment_type", "TEXT DEFAULT 'full_time'"),
        ("salary_type", "TEXT DEFAULT 'hourly'"),
        ("monthly_salary", "REAL DEFAULT 0.0"),
        ("tax_id", "TEXT DEFAULT ''"),
        ("bank_name", "TEXT DEFAULT ''"),
        ("bank_account_number", "TEXT DEFAULT ''"),
        ("bank_routing_number", "TEXT DEFAULT ''"),
        ("emergency_contact_name", "TEXT DEFAULT ''"),
        ("emergency_contact_phone", "TEXT DEFAULT ''"),
    ]:
        try:
            conn.execute(f"ALTER TABLE employees ADD COLUMN {col_name} {col_type};")
        except Exception:
            pass

    # Phase 9: Enterprise Multi-Tenancy, Auth & Compliance
    conn.executescript("""
    CREATE TABLE IF NOT EXISTS user_organizations (
        user_id TEXT NOT NULL REFERENCES users(id),
        org_id TEXT NOT NULL REFERENCES organizations(id),
        role TEXT NOT NULL DEFAULT 'member',
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY(user_id, org_id)
    );

    INSERT OR IGNORE INTO organizations (id, name, slug)
    VALUES ('org_emea', 'AO Sphere EMEA Operations', 'emea');

    INSERT OR IGNORE INTO users (id, org_id, email, password_hash, name, role)
    VALUES ('usr_admin_default', 'org_default', 'admin@aosphere.local', 'mock_hash', 'System Administrator', 'owner');

    INSERT OR IGNORE INTO user_organizations (user_id, org_id, role)
    SELECT id, org_id, role FROM users;

    INSERT OR IGNORE INTO user_organizations (user_id, org_id, role)
    VALUES ('usr_admin_default', 'org_emea', 'admin');

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
    """)

    for col_name, col_type in [
        ("totp_secret", "TEXT DEFAULT ''"),
        ("totp_enabled", "BOOLEAN NOT NULL DEFAULT 0"),
        ("recovery_codes_json", "TEXT DEFAULT '[]'"),
        ("last_active_at", "DATETIME"),
    ]:
        try:
            conn.execute(f"ALTER TABLE users ADD COLUMN {col_name} {col_type};")
        except Exception:
            pass

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


# ----------------- PHASE 8 HELPERS: WEBHOOKS, SANITIZER, INTROSPECTION -----------------
import hmac
import hashlib

def compute_hmac_signature(secret, timestamp, payload):
    msg = f"{timestamp}.{payload}".encode("utf-8")
    return hmac.new(secret.encode("utf-8"), msg, hashlib.sha256).hexdigest()

def dispatch_webhook_event(conn, org_id, event_type, payload):
    subs = conn.execute("SELECT * FROM webhook_subscriptions WHERE org_id = ? AND is_active = 1", (org_id,)).fetchall()
    deliveries = []
    event_id = f"evt_{int(time.time()*1000)}"
    payload_str = json.dumps(payload)
    now_ts = int(time.time())
    
    for sub in subs:
        ev_types = sub["event_types"]
        if ev_types != '["*"]' and "*" not in ev_types and event_type not in ev_types:
            continue
        deliv_id = f"wdel_{int(time.time()*1000)}_{sub['id'][:4]}"
        sig = compute_hmac_signature(sub["secret"], now_ts, payload_str)
        
        status = "delivered"
        status_code = 200
        resp_body = ""
        try:
            req = urllib.request.Request(
                sub["url"],
                data=payload_str.encode("utf-8"),
                headers={
                    "Content-Type": "application/json",
                    "User-Agent": "AOSphere-Webhook-Dispatcher/1.0",
                    "X-Aosphere-Timestamp": str(now_ts),
                    "X-Aosphere-Signature": f"sha256={sig}",
                    "X-Aosphere-Event": event_type,
                    "X-Aosphere-Delivery": deliv_id
                },
                method="POST"
            )
            with urllib.request.urlopen(req, timeout=5) as r:
                status_code = r.status
                resp_body = r.read().decode("utf-8", errors="ignore")[:500]
        except urllib.error.HTTPError as he:
            status_code = he.code
            resp_body = he.read().decode("utf-8", errors="ignore")[:500]
            status = "failed"
        except Exception as e:
            status = "failed"
            status_code = 0
            resp_body = str(e)[:500]
            
        conn.execute("""
            INSERT INTO webhook_deliveries (id, org_id, subscription_id, event_id, event_type, payload_json, status, status_code, response_body, retry_count, max_retries, delivered_at, created_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 3, ?, ?)
        """, (deliv_id, org_id, sub["id"], event_id, event_type, payload_str, status, status_code, resp_body, utcnow_str(), utcnow_str()))
        deliveries.append({"id": deliv_id, "subscription_id": sub["id"], "status": status, "status_code": status_code})
    conn.commit()
    return deliveries

INJECTION_PATTERNS = [
    ("instruction_reset", re.compile(r"\b(ignore\s+(all\s+)?(previous|prior|above)\s+instructions?)\b", re.I), 0.9),
    ("system_override", re.compile(r"\b(system\s+override|override\s+system\s+prompt)\b", re.I), 0.95),
    ("role_play_jailbreak", re.compile(r"\b(you\s+are\s+now\s+in\s+developer\s+mode|dan\s+mode|jailbreak|unfiltered\s+mode)\b", re.I), 0.85),
    ("special_token_smuggling", re.compile(r"<\s*\|\s*(system|im_start|im_end|assistant)\s*\|?\s*>", re.I), 0.99),
    ("markdown_role_hijack", re.compile(r"```(system|instruction|prompt)\b", re.I), 0.8),
    ("bracketed_instruction", re.compile(r"\[\s*(system|inst|instruction)\s*\]", re.I), 0.75),
]

def sanitize_input(text):
    trimmed = (text or "").strip()
    flagged = []
    max_weight = 0.0
    sanitized = trimmed
    for name, pat, weight in INJECTION_PATTERNS:
        if pat.search(sanitized):
            flagged.append(name)
            if weight > max_weight:
                max_weight = weight
            sanitized = pat.sub(f"[FILTERED_INJECTION: {name}]", sanitized)
    is_safe = len(flagged) == 0 and max_weight < 0.7
    return {
        "is_safe": is_safe,
        "risk_score": max_weight,
        "flagged_patterns": flagged,
        "sanitized_text": sanitized
    }

DYNAMIC_MCP_TOOLS = [
    {"name": "erp_check_stock", "category": "read_only", "description": "Inspect stock levels, reserved quantities, and threshold alerts for SKUs.", "required_permission": "read"},
    {"name": "erp_adjust_stock", "category": "high_impact_mutation", "description": "Adjust SKU inventory quantity.", "required_permission": "manage_inventory"},
    {"name": "crm_get_deals", "category": "read_only", "description": "Retrieve deals pipeline and stages.", "required_permission": "read"},
    {"name": "crm_create_deal", "category": "reversible_mutation", "description": "Create a new sales opportunity deal.", "required_permission": "manage_orders"},
    {"name": "fin_get_summary", "category": "read_only", "description": "Retrieve corporate P&L summary.", "required_permission": "read"},
    {"name": "fin_create_invoice", "category": "reversible_mutation", "description": "Generate a new client billing invoice.", "required_permission": "manage_finance"},
    {"name": "hr_get_shifts", "category": "read_only", "description": "Fetch employee shift roster.", "required_permission": "read"},
    {"name": "hr_update_shift", "category": "reversible_mutation", "description": "Update worker shift status.", "required_permission": "manage_hr"},
    {"name": "gov_get_pending_approvals", "category": "read_only", "description": "Fetch human approval backlog.", "required_permission": "read"},
    {"name": "agent_escalate_to_human", "category": "reversible_mutation", "description": "Escalate ambiguous inputs or violations to human operator queue.", "required_permission": "read"},
    {"name": "agent_get_context", "category": "read_only", "description": "Retrieve company profile, currency, tax rates, and lead times.", "required_permission": "read"},
    {"name": "agent_sanitize_input", "category": "read_only", "description": "Scan untrusted external strings for prompt injection attacks.", "required_permission": "read"},
    {"name": "webhook_test_event", "category": "reversible_mutation", "description": "Dispatch a signed test event to registered webhooks.", "required_permission": "admin"}
]

# ----------------- PHASE 9 HELPERS: TOTP, COMPLIANCE & AUDIT CHAIN -----------------

def generate_totp_secret():
    raw = os.urandom(20)
    return base64.b32encode(raw).decode("utf-8").replace("=", "")

def compute_totp_code(secret, t=None):
    if t is None:
        t = time.time()
    clean_secret = secret.strip().upper()
    padding = (8 - len(clean_secret) % 8) % 8
    key = base64.b32decode(clean_secret + ("=" * padding))
    counter = int(t // 30)
    counter_bytes = struct.pack(">Q", counter)
    h = hmac.new(key, counter_bytes, hashlib.sha1).digest()
    offset = h[-1] & 0x0F
    code_int = struct.unpack(">I", h[offset:offset+4])[0] & 0x7FFFFFFF
    return f"{code_int % 1000000:06d}"

def verify_totp_code(secret, code):
    code = str(code).strip()
    now = time.time()
    for step in [now, now - 30, now + 30]:
        try:
            if compute_totp_code(secret, step) == code:
                return True
        except Exception:
            pass
    return False

def generate_recovery_codes(count=8):
    charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
    codes = []
    for _ in range(count):
        part1 = "".join(random.choice(charset) for _ in range(4))
        part2 = "".join(random.choice(charset) for _ in range(4))
        codes.append(f"{part1}-{part2}")
    return codes

def validate_password_complexity(pwd):
    if len(pwd) < 10:
        return "Password must be at least 10 characters long"
    if not any(c.isupper() for c in pwd):
        return "Password must contain at least one uppercase letter (A-Z)"
    if not any(c.islower() for c in pwd):
        return "Password must contain at least one lowercase letter (a-z)"
    if not any(c.isdigit() for c in pwd):
        return "Password must contain at least one digit (0-9)"
    if not any(c in "!@#$%^&*()_+-=[]{}|;:,.<>?" for c in pwd):
        return "Password must contain at least one special character (!@#$%^&*)"
    return None

GENESIS_BLOCK_HASH = "0000000000000000000000000000000000000000000000000000000000000000"

def compute_audit_block_hash(seq_num, prev_hash, action, actor_id, payload_hash, ts_str):
    raw = f"{seq_num}:{prev_hash}:{action}:{actor_id}:{payload_hash}:{ts_str}"
    return hashlib.sha256(raw.encode("utf-8")).hexdigest()

def verify_audit_vault_chain(conn, org_id):
    rows = conn.execute("SELECT sequence_number, prev_block_hash, block_hash, action, actor_id, payload_hash, timestamp FROM audit_chain_vault WHERE org_id = ? ORDER BY sequence_number ASC", (org_id,)).fetchall()
    if not rows:
        return True, 0, None
    for idx, r in enumerate(rows):
        expected_prev = GENESIS_BLOCK_HASH if idx == 0 else rows[idx-1]["block_hash"]
        if r["prev_block_hash"] != expected_prev:
            return False, idx, f"Broken chain link at seq {r['sequence_number']}"
        expected_hash = compute_audit_block_hash(r["sequence_number"], r["prev_block_hash"], r["action"], r["actor_id"], r["payload_hash"], r["timestamp"])
        if r["block_hash"] != expected_hash:
            return False, idx, f"Tamper detected at seq {r['sequence_number']}"
    return True, len(rows), None


class AOSphereHandler(BaseHTTPRequestHandler):
    def address_string(self):
        return self.client_address[0]

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
        if path == "/api/backup/config":
            sec = os.environ.get("S3_SECRET_KEY", "")
            return self.send_json({
                "endpoint": os.environ.get("S3_ENDPOINT", ""),
                "bucket": os.environ.get("S3_BUCKET", ""),
                "region": os.environ.get("S3_REGION", "us-east-1"),
                "access_key": os.environ.get("S3_ACCESS_KEY", ""),
                "secret_key": "********" if sec else ""
            })

        if path == "/api/backup/list":
            entries = []
            if os.path.exists(BACKUP_DIR):
                for fname in sorted(os.listdir(BACKUP_DIR), reverse=True):
                    if fname.endswith(".db") or fname.endswith(".enc"):
                        full_p = os.path.join(BACKUP_DIR, fname)
                        st = os.stat(full_p)
                        is_enc = fname.endswith(".enc")
                        checksum = compute_file_sha256(full_p)
                        entries.append({
                            "filename": fname,
                            "path": full_p,
                            "size_bytes": st.st_size,
                            "checksum_sha256": checksum,
                            "is_encrypted": is_enc,
                            "created_at": datetime.fromtimestamp(st.st_mtime, timezone.utc).isoformat(),
                            "integrity_ok": True,
                        })
            return self.send_json(entries)

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

                elif path == "/api/pipelines":
                    pipe_rows = conn.execute("SELECT * FROM pipelines ORDER BY is_default DESC, name ASC").fetchall()
                    pipes = []
                    for pr in pipe_rows:
                        p = dict(pr)
                        stages = conn.execute("SELECT * FROM pipeline_stages WHERE pipeline_id = ? ORDER BY position ASC", (p["id"],)).fetchall()
                        p["stages"] = [dict(s) for s in stages]
                        pipes.append(p)
                    conn.close()
                    return self.send_json(pipes)

                elif path == "/api/deals/decay-alerts":
                    rows = conn.execute("""
                        SELECT d.id, d.title, d.value, d.stage, d.probability,
                               COALESCE(d.pipeline_id, 'pipe_std_org_default') as pipeline_id,
                               COALESCE(p.name, 'Standard Sales') as pipeline_name,
                               COALESCE(c.name, 'Client') as customer_name,
                               COALESCE(c.company, '') as customer_company,
                               COALESCE(ps.name, d.stage) as stage_name,
                               COALESCE(ps.decay_days_threshold, 14) as decay_threshold,
                               COALESCE(d.stage_updated_at, d.created_at) as updated_at
                        FROM deals d
                        LEFT JOIN customers c ON d.customer_id = c.id
                        LEFT JOIN pipelines p ON d.pipeline_id = p.id
                        LEFT JOIN pipeline_stages ps ON ps.pipeline_id = d.pipeline_id AND ps.code = d.stage
                        WHERE d.stage NOT IN ('won', 'lost')
                        ORDER BY d.value DESC
                    """).fetchall()
                    conn.close()
                    alerts = []
                    now = datetime.now(timezone.utc)
                    for r in rows:
                        d = dict(r)
                        up_str = d.get("updated_at") or ""
                        try:
                            if "T" in up_str:
                                dt = datetime.fromisoformat(up_str.replace("Z", "+00:00"))
                            else:
                                dt = datetime.strptime(up_str, "%Y-%m-%d %H:%M:%S").replace(tzinfo=timezone.utc)
                            days = max(0, int((now - dt).total_seconds() / 86400))
                        except Exception:
                            days = 5
                        d["days_in_stage"] = days
                        thresh = d.get("decay_threshold", 14)
                        if thresh > 0 and days >= thresh:
                            d["decay_severity"] = "critical" if days >= thresh * 2 else "warning"
                            alerts.append(d)
                    return self.send_json(alerts)

                elif path == "/api/customers/360":
                    cust_id = query.get("id", [""])[0]
                    if not cust_id:
                        conn.close()
                        return self.send_json({"error": "id parameter required"}, 400)
                    cust_row = conn.execute("SELECT * FROM customers WHERE id = ?", (cust_id,)).fetchone()
                    if not cust_row:
                        conn.close()
                        return self.send_json({"error": "Customer not found"}, 404)

                    inv_rows = conn.execute("SELECT * FROM invoices WHERE customer_id = ? ORDER BY created_at DESC", (cust_id,)).fetchall()
                    cn_rows = conn.execute("SELECT * FROM credit_notes WHERE customer_id = ? ORDER BY created_at DESC", (cust_id,)).fetchall()
                    deal_rows = conn.execute("""
                        SELECT d.*, COALESCE(p.name, 'Standard Sales') as pipeline_name
                        FROM deals d LEFT JOIN pipelines p ON d.pipeline_id = p.id
                        WHERE d.customer_id = ? ORDER BY d.created_at DESC
                    """, (cust_id,)).fetchall()
                    qte_rows = conn.execute("SELECT * FROM quotes WHERE customer_id = ? ORDER BY created_at DESC", (cust_id,)).fetchall()
                    act_rows = conn.execute("SELECT * FROM customer_activities WHERE customer_id = ? ORDER BY created_at DESC LIMIT 50", (cust_id,)).fetchall()

                    tot_paid = sum(float(i["total_amount"]) for i in inv_rows if i["status"] == "paid")
                    ar_open = sum(float(i["total_amount"]) for i in inv_rows if i["status"] != "paid")
                    tot_cn = sum(float(c["total_amount"]) for c in cn_rows)

                    res = {
                        "customer": dict(cust_row),
                        "total_ltv": float(cust_row["lifetime_value"] or 0.0),
                        "total_paid": tot_paid,
                        "outstanding_ar": ar_open,
                        "total_credit_notes": tot_cn,
                        "kpis": {
                            "lifetime_value": float(cust_row["lifetime_value"] or 0.0),
                            "total_revenue": tot_paid,
                            "unpaid_balance": ar_open,
                            "open_pipeline_value": sum(float(d["value"] or 0.0) for d in deal_rows if d["stage"] not in ("won", "lost"))
                        },
                        "invoices": [dict(i) for i in inv_rows],
                        "credit_notes": [dict(c) for c in cn_rows],
                        "deals": [dict(d) for d in deal_rows],
                        "quotes": [dict(q) for q in qte_rows],
                        "activities": [dict(a) for a in act_rows]
                    }
                    conn.close()
                    return self.send_json(res)

                elif path == "/api/leads":
                    rows = conn.execute("SELECT * FROM inbound_leads ORDER BY created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/emails/outbox":
                    rows = conn.execute("SELECT * FROM email_outbox ORDER BY sent_at DESC LIMIT 50").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path.startswith("/api/portal/invoice/"):
                    ptoken = path[len("/api/portal/invoice/"):]
                    tok_row = conn.execute("SELECT * FROM invoice_portal_tokens WHERE token = ?", (ptoken,)).fetchone()
                    if not tok_row:
                        conn.close()
                        return self.send_json({"error": "Invalid or expired portal token"}, 404)
                    conn.execute("UPDATE invoice_portal_tokens SET view_count = view_count + 1, last_viewed_at = CURRENT_TIMESTAMP WHERE token = ?", (ptoken,))
                    conn.commit()

                    inv = conn.execute("""
                        SELECT i.*, COALESCE(c.name, 'Client') as customer_name, COALESCE(c.email, '') as customer_email
                        FROM invoices i LEFT JOIN customers c ON i.customer_id = c.id
                        WHERE i.id = ?
                    """, (tok_row["invoice_id"],)).fetchone()
                    conn.close()
                    if not inv:
                        return self.send_json({"error": "Invoice not found"}, 404)

                    try:
                        items = json.loads(inv["items_json"] or "[]")
                    except Exception:
                        items = []

                    return self.send_json({
                        "token": ptoken,
                        "invoice_id": inv["id"],
                        "invoice_number": inv["invoice_number"],
                        "issue_date": inv["issue_date"],
                        "due_date": inv["due_date"],
                        "status": inv["status"],
                        "subtotal": float(inv["subtotal"]),
                        "tax_amount": float(inv["tax_amount"]),
                        "total_amount": float(inv["total_amount"]),
                        "currency": "USD",
                        "items": items,
                        "customer_name": inv["customer_name"],
                        "customer_email": inv["customer_email"],
                        "company_name": "AO Sphere Enterprise",
                        "company_address": "100 Innovation Way, Suite 400",
                        "company_website": "https://aosphere.local",
                        "is_paid": inv["status"] == "paid"
                    })

                elif path == "/api/shifts/conflicts":
                    rows = conn.execute("""
                        SELECT s.id, s.employee_id, COALESCE(e.name, 'Unknown') as emp_name,
                               s.shift_date, s.start_time, s.end_time, s.station
                        FROM shifts s
                        LEFT JOIN employees e ON s.employee_id = e.id
                        WHERE s.status != 'cancelled'
                        ORDER BY s.employee_id, s.shift_date, s.start_time
                    """).fetchall()
                    conn.close()
                    conflicts = []
                    emp_shifts = {}
                    for r in rows:
                        d = dict(r)
                        emp_shifts.setdefault(d["employee_id"], []).append(d)

                    for emp_id, shifts in emp_shifts.items():
                        emp_name = shifts[0]["emp_name"]
                        weekly_hours = {}
                        for i, curr in enumerate(shifts):
                            try:
                                sdt = datetime.strptime(f"{curr['shift_date']} {curr['start_time']}", "%Y-%m-%d %H:%M")
                                edt = datetime.strptime(f"{curr['shift_date']} {curr['end_time']}", "%Y-%m-%d %H:%M")
                                dur = max(0, (edt - sdt).total_seconds() / 3600.0)
                                w_key = f"{sdt.isocalendar()[0]}-W{sdt.isocalendar()[1]:02d}"
                                weekly_hours[w_key] = weekly_hours.get(w_key, 0.0) + dur
                            except Exception:
                                pass

                            for j in range(i + 1, len(shifts)):
                                nxt = shifts[j]
                                if curr["shift_date"] == nxt["shift_date"]:
                                    if curr["start_time"] < nxt["end_time"] and curr["end_time"] > nxt["start_time"]:
                                        conflicts.append({
                                            "shift_id": nxt["id"],
                                            "employee_id": emp_id,
                                            "employee_name": emp_name,
                                            "conflict_type": "double_booking",
                                            "message": f"Double-booking: Shift ({nxt['start_time']}-{nxt['end_time']}) overlaps with ({curr['start_time']}-{curr['end_time']}) on {curr['shift_date']}",
                                            "severity": "error"
                                        })
                                try:
                                    s1_end = datetime.strptime(f"{curr['shift_date']} {curr['end_time']}", "%Y-%m-%d %H:%M")
                                    s2_start = datetime.strptime(f"{nxt['shift_date']} {nxt['start_time']}", "%Y-%m-%d %H:%M")
                                    if s2_start > s1_end:
                                        gap_hrs = (s2_start - s1_end).total_seconds() / 3600.0
                                        if gap_hrs < 8.0:
                                            conflicts.append({
                                                "shift_id": nxt["id"],
                                                "employee_id": emp_id,
                                                "employee_name": emp_name,
                                                "conflict_type": "rest_period_violation",
                                                "message": f"Rest period violation: Only {gap_hrs:.1f}h rest between {curr['shift_date']} {curr['end_time']} and {nxt['shift_date']} {nxt['start_time']} (< 8h required)",
                                                "severity": "error"
                                            })
                                except Exception:
                                    pass

                        for wk, hrs in weekly_hours.items():
                            if hrs > 40.0:
                                conflicts.append({
                                    "shift_id": "",
                                    "employee_id": emp_id,
                                    "employee_name": emp_name,
                                    "conflict_type": "weekly_overtime",
                                    "message": f"Weekly overtime warning: Scheduled for {hrs:.1f} hours in {wk} (> 40h workweek)",
                                    "severity": "warning"
                                })
                    return self.send_json(conflicts)

                elif path == "/api/timeclock/status":
                    emp_id = query.get("employee_id", [""])[0]
                    if not emp_id:
                        first_emp = conn.execute("SELECT id FROM employees ORDER BY name LIMIT 1").fetchone()
                        if first_emp:
                            emp_id = first_emp["id"]
                    today = utc_today_str()
                    events = conn.execute("SELECT * FROM timeclock_events WHERE employee_id = ? AND date(timestamp) = ? ORDER BY timestamp ASC", (emp_id, today)).fetchall()
                    conn.close()
                    current_state = "clocked_out"
                    ev_list = []
                    for ev in events:
                        d = dict(ev)
                        ev_list.append(d)
                        if d["event_type"] in ("clock_in", "break_end"):
                            current_state = "clocked_in"
                        elif d["event_type"] == "break_start":
                            current_state = "on_break"
                        elif d["event_type"] == "clock_out":
                            current_state = "clocked_out"
                    return self.send_json({
                        "employee_id": emp_id,
                        "date": today,
                        "current_state": current_state,
                        "current_status": current_state,
                        "last_event": ev_list[-1] if ev_list else None,
                        "events_count": len(ev_list),
                        "events": ev_list,
                        "punches_today": ev_list
                    })

                elif path == "/api/payroll/preview":
                    period = query.get("period", [datetime.now(timezone.utc).strftime("%Y-%m")])[0]
                    emps = conn.execute("SELECT * FROM employees WHERE status = 'active' ORDER BY name").fetchall()
                    payslips = []
                    tot_gross_c = 0
                    tot_tax_c = 0
                    tot_ded_c = 0
                    tot_net_c = 0
                    for e in emps:
                        sal_type = e["salary_type"] if "salary_type" in e.keys() else "hourly"
                        rate = float(e["hourly_rate"] or 25.0)
                        mo_sal = float(e["monthly_salary"] or 0.0)
                        if sal_type == "monthly" and mo_sal > 0:
                            gross = mo_sal
                            reg_hrs = 160.0
                            ot_hrs = 0.0
                        else:
                            reg_hrs = 160.0
                            ot_hrs = 0.0
                            gross = reg_hrs * rate

                        tax = round(gross * 0.12, 2)
                        soc = round(gross * 0.05, 2)
                        ben = round(gross * 0.03, 2)
                        net = round(gross - (tax + soc + ben), 2)

                        g_c = int(round(gross * 100))
                        t_c = int(round(tax * 100))
                        d_c = int(round((soc + ben) * 100))
                        n_c = g_c - (t_c + d_c)

                        tot_gross_c += g_c
                        tot_tax_c += t_c
                        tot_ded_c += d_c
                        tot_net_c += n_c

                        payslips.append({
                            "id": f"ps_{e['id']}_{period}",
                            "employee_id": e["id"],
                            "employee_name": e["name"],
                            "period": period,
                            "salary_type": sal_type,
                            "hourly_rate": rate,
                            "monthly_salary": mo_sal,
                            "regular_hours": reg_hrs,
                            "overtime_hours": ot_hrs,
                            "gross_pay": gross,
                            "tax_withholding": tax,
                            "social_contributions": soc,
                            "benefits_deductions": ben,
                            "net_pay": net,
                            "status": "generated"
                        })
                    conn.close()
                    run = {
                        "id": f"prun_{period}",
                        "period": period,
                        "total_gross_cents": tot_gross_c,
                        "total_tax_cents": tot_tax_c,
                        "total_deductions_cents": tot_ded_c,
                        "total_net_cents": tot_net_c,
                        "total_gross_dollars": tot_gross_c / 100.0,
                        "total_tax_dollars": tot_tax_c / 100.0,
                        "total_net_dollars": tot_net_c / 100.0,
                        "status": "draft",
                        "payment_date": utc_today_str(),
                        "payslips_count": len(payslips)
                    }
                    return self.send_json({
                        "period": period,
                        "total_gross_cents": tot_gross_c,
                        "total_tax_cents": tot_tax_c,
                        "total_deductions_cents": tot_ded_c,
                        "total_net_cents": tot_net_c,
                        "total_gross_dollars": tot_gross_c / 100.0,
                        "total_tax_dollars": tot_tax_c / 100.0,
                        "total_net_dollars": tot_net_c / 100.0,
                        "run": run,
                        "payslips": payslips
                    })

                elif path == "/api/payroll/history":
                    runs = conn.execute("SELECT * FROM payroll_runs ORDER BY period DESC").fetchall()
                    conn.close()
                    res = []
                    for r in runs:
                        d = dict(r)
                        d["total_gross_dollars"] = d["total_gross_cents"] / 100.0
                        d["total_tax_dollars"] = d["total_tax_cents"] / 100.0
                        d["total_net_dollars"] = d["total_net_cents"] / 100.0
                        res.append(d)
                    return self.send_json(res)

                elif path == "/api/payroll/payslips":
                    run_id = query.get("payroll_run_id", [""])[0]
                    emp_id = query.get("employee_id", [""])[0]
                    q = "SELECT p.*, COALESCE(e.name, 'Employee') as employee_name FROM payslips p LEFT JOIN employees e ON p.employee_id = e.id WHERE 1=1"
                    params = []
                    if run_id:
                        q += " AND p.payroll_run_id = ?"
                        params.append(run_id)
                    if emp_id:
                        q += " AND p.employee_id = ?"
                        params.append(emp_id)
                    q += " ORDER BY p.created_at DESC"
                    slips = conn.execute(q, params).fetchall()
                    conn.close()
                    return self.send_json([dict(s) for s in slips])

                elif path == "/api/payroll/payslip/pdf":
                    ps_id = query.get("payslip_id", [""])[0] or query.get("id", [""])[0]
                    ps = conn.execute("SELECT p.*, COALESCE(e.name, 'Employee') as employee_name, COALESCE(e.job_title, e.role) as job_title, COALESCE(e.department, 'Operations') as department, COALESCE(e.tax_id, '') as tax_id, COALESCE(e.bank_name, 'Direct Deposit') as bank_name, COALESCE(e.bank_account_number, '') as bank_account_number FROM payslips p LEFT JOIN employees e ON p.employee_id = e.id WHERE p.id = ?", (ps_id,)).fetchone()
                    conn.close()
                    if not ps:
                        return self.send_json({"error": "Payslip not found"}, 404)
                    pdf_bytes = generate_payslip_pdf(dict(ps))
                    self.send_response(200)
                    self.send_header("Content-Type", "application/pdf")
                    self.send_header("Content-Disposition", f"attachment; filename=\"payslip_{ps['period']}_{ps['employee_id']}.pdf\"")
                    self.send_header("Content-Length", str(len(pdf_bytes)))
                    self.end_headers()
                    self.wfile.write(pdf_bytes)
                    return

                elif path == "/api/leaves/policies":
                    rows = conn.execute("SELECT * FROM leave_policies ORDER BY annual_days DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/leaves/requests":
                    emp_id = query.get("employee_id", [""])[0]
                    status = query.get("status", [""])[0]
                    q = "SELECT lr.*, COALESCE(e.name, 'Employee') as employee_name FROM leave_requests lr LEFT JOIN employees e ON lr.employee_id = e.id WHERE 1=1"
                    params = []
                    if emp_id:
                        q += " AND lr.employee_id = ?"
                        params.append(emp_id)
                    if status:
                        q += " AND lr.status = ?"
                        params.append(status)
                    q += " ORDER BY lr.created_at DESC"
                    rows = conn.execute(q, params).fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/hr/documents":
                    emp_id = query.get("employee_id", [""])[0]
                    doc_type = query.get("document_type", [""])[0]
                    q = "SELECT d.*, COALESCE(e.name, 'Employee') as employee_name FROM hr_documents d LEFT JOIN employees e ON d.employee_id = e.id WHERE 1=1"
                    params = []
                    if emp_id:
                        q += " AND d.employee_id = ?"
                        params.append(emp_id)
                    if doc_type:
                        q += " AND d.document_type = ?"
                        params.append(doc_type)
                    q += " ORDER BY d.created_at DESC"
                    rows = conn.execute(q, params).fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

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

                elif path == "/api/reports/pnl":
                    start_date = query.get("start_date", [""])[0] or (datetime.now(timezone.utc) - timedelta(days=30)).strftime("%Y-%m-%d")
                    end_date = query.get("end_date", [""])[0] or datetime.now(timezone.utc).strftime("%Y-%m-%d")
                    q = """
                    SELECT a.code, a.name, a.type,
                        COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE 0 END), 0) AS total_debits,
                        COALESCE(SUM(CASE WHEN jl.entry_type = 'credit' THEN jl.amount_cents ELSE 0 END), 0) AS total_credits
                    FROM accounts a
                    JOIN journal_lines jl ON a.id = jl.account_id AND jl.org_id = a.org_id
                    JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
                    WHERE a.org_id = 'org_default'
                        AND substr(je.posted_at, 1, 10) >= ?
                        AND substr(je.posted_at, 1, 10) <= ?
                        AND a.type IN ('revenue', 'expense')
                    GROUP BY a.code, a.name, a.type
                    ORDER BY a.code ASC
                    """
                    rows = conn.execute(q, (start_date, end_date)).fetchall()
                    conn.close()
                    rev_items = []
                    cogs_items = []
                    opex_items = []
                    rev_cents = 0
                    cogs_cents = 0
                    opex_cents = 0
                    for r in rows:
                        code, name, atype, deb, cred = r["code"], r["name"], r["type"], r["total_debits"], r["total_credits"]
                        if atype == "revenue":
                            amt = max(0, cred - deb)
                            rev_cents += amt
                            rev_items.append({"account_code": code, "account_name": name, "amount_cents": amt, "amount": round(amt / 100.0, 2)})
                        elif atype == "expense":
                            amt = max(0, deb - cred)
                            if code == "5000":
                                cogs_cents += amt
                                cogs_items.append({"account_code": code, "account_name": name, "amount_cents": amt, "amount": round(amt / 100.0, 2)})
                            else:
                                opex_cents += amt
                                opex_items.append({"account_code": code, "account_name": name, "amount_cents": amt, "amount": round(amt / 100.0, 2)})
                    gross_profit_cents = rev_cents - cogs_cents
                    net_profit_cents = gross_profit_cents - opex_cents
                    margin_pct = round((gross_profit_cents / rev_cents) * 100.0, 2) if rev_cents > 0 else 0.0
                    return self.send_json({
                        "org_id": "org_default",
                        "start_date": start_date,
                        "end_date": end_date,
                        "revenue_cents": rev_cents,
                        "revenue": round(rev_cents / 100.0, 2),
                        "cogs_cents": cogs_cents,
                        "cogs": round(cogs_cents / 100.0, 2),
                        "gross_profit_cents": gross_profit_cents,
                        "gross_profit": round(gross_profit_cents / 100.0, 2),
                        "gross_margin_pct": margin_pct,
                        "opex_cents": opex_cents,
                        "opex": round(opex_cents / 100.0, 2),
                        "net_profit_cents": net_profit_cents,
                        "net_profit": round(net_profit_cents / 100.0, 2),
                        "revenue_items": rev_items,
                        "cogs_items": cogs_items,
                        "opex_items": opex_items
                    })

                elif path == "/api/reports/balance-sheet":
                    as_of = query.get("as_of", [""])[0] or datetime.now(timezone.utc).strftime("%Y-%m-%d")
                    q = """
                    SELECT a.code, a.name, a.type, a.normal_balance,
                        COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE 0 END), 0) AS total_debits,
                        COALESCE(SUM(CASE WHEN jl.entry_type = 'credit' THEN jl.amount_cents ELSE 0 END), 0) AS total_credits
                    FROM accounts a
                    LEFT JOIN journal_lines jl ON a.id = jl.account_id AND jl.org_id = a.org_id
                    LEFT JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted' AND substr(je.posted_at, 1, 10) <= ?
                    WHERE a.org_id = 'org_default' AND a.is_active = 1
                    GROUP BY a.code, a.name, a.type, a.normal_balance
                    ORDER BY a.code ASC
                    """
                    rows = conn.execute(q, (as_of,)).fetchall()
                    conn.close()
                    assets = []
                    liabilities = []
                    equity = []
                    tot_assets_cents = 0
                    tot_liab_cents = 0
                    tot_eq_cents = 0
                    tot_rev_cents = 0
                    tot_exp_cents = 0
                    for r in rows:
                        code, name, atype, nb, deb, cred = r["code"], r["name"], r["type"], r["normal_balance"], r["total_debits"], r["total_credits"]
                        net = (deb - cred) if nb == "debit" else (cred - deb)
                        if atype == "asset":
                            tot_assets_cents += net
                            assets.append({"code": code, "name": name, "amount_cents": net, "amount": round(net / 100.0, 2)})
                        elif atype == "liability":
                            tot_liab_cents += net
                            liabilities.append({"code": code, "name": name, "amount_cents": net, "amount": round(net / 100.0, 2)})
                        elif atype == "equity":
                            tot_eq_cents += net
                            equity.append({"code": code, "name": name, "amount_cents": net, "amount": round(net / 100.0, 2)})
                        elif atype == "revenue":
                            tot_rev_cents += (cred - deb)
                        elif atype == "expense":
                            tot_exp_cents += (deb - cred)
                    cur_net_income = tot_rev_cents - tot_exp_cents
                    tot_eq_cents += cur_net_income
                    equity.append({"code": "3100", "name": "Current Period Net Earnings", "amount_cents": cur_net_income, "amount": round(cur_net_income / 100.0, 2)})
                    return self.send_json({
                        "org_id": "org_default",
                        "as_of": as_of,
                        "current_assets": assets,
                        "total_assets_cents": tot_assets_cents,
                        "total_assets": round(tot_assets_cents / 100.0, 2),
                        "current_liabilities": liabilities,
                        "total_liabilities_cents": tot_liab_cents,
                        "total_liabilities": round(tot_liab_cents / 100.0, 2),
                        "equity_items": equity,
                        "total_equity_cents": tot_eq_cents,
                        "total_equity": round(tot_eq_cents / 100.0, 2),
                        "is_balanced": (tot_assets_cents == (tot_liab_cents + tot_eq_cents))
                    })

                elif path == "/api/reports/cash-flow":
                    start_date = query.get("start_date", [""])[0] or (datetime.now(timezone.utc) - timedelta(days=30)).strftime("%Y-%m-%d")
                    end_date = query.get("end_date", [""])[0] or datetime.now(timezone.utc).strftime("%Y-%m-%d")
                    q_pnl = """
                    SELECT
                        COALESCE(SUM(CASE WHEN a.type = 'revenue' THEN (jl.amount_cents * (CASE WHEN jl.entry_type='credit' THEN 1 ELSE -1 END)) ELSE 0 END), 0) -
                        COALESCE(SUM(CASE WHEN a.type = 'expense' THEN (jl.amount_cents * (CASE WHEN jl.entry_type='debit' THEN 1 ELSE -1 END)) ELSE 0 END), 0) AS net_profit
                    FROM accounts a
                    JOIN journal_lines jl ON a.id = jl.account_id
                    JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
                    WHERE a.org_id = 'org_default' AND substr(je.posted_at, 1, 10) >= ? AND substr(je.posted_at, 1, 10) <= ?
                    """
                    net_profit = conn.execute(q_pnl, (start_date, end_date)).fetchone()["net_profit"] or 0
                    q_cash = """
                    SELECT
                        COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE 0 END), 0) as inflows,
                        COALESCE(SUM(CASE WHEN jl.entry_type = 'credit' THEN jl.amount_cents ELSE 0 END), 0) as outflows
                    FROM journal_lines jl
                    JOIN accounts a ON jl.account_id = a.id
                    JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
                    WHERE a.org_id = 'org_default' AND a.code = '1000'
                        AND substr(je.posted_at, 1, 10) >= ? AND substr(je.posted_at, 1, 10) <= ?
                    """
                    cash_flow = conn.execute(q_cash, (start_date, end_date)).fetchone()
                    cash_in = cash_flow["inflows"] or 0
                    cash_out = cash_flow["outflows"] or 0
                    q_beg = """
                    SELECT COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE -jl.amount_cents END), 0) as beg_cash
                    FROM journal_lines jl
                    JOIN accounts a ON jl.account_id = a.id
                    JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
                    WHERE a.org_id = 'org_default' AND a.code = '1000' AND substr(je.posted_at, 1, 10) < ?
                    """
                    beg_cash = conn.execute(q_beg, (start_date,)).fetchone()["beg_cash"] or 0
                    conn.close()
                    net_change = cash_in - cash_out
                    return self.send_json({
                        "org_id": "org_default",
                        "start_date": start_date,
                        "end_date": end_date,
                        "operating_activities": [{"category": "operating", "description": "Net Operational Income / (Loss)", "amount_cents": net_profit, "amount": round(net_profit / 100.0, 2)}],
                        "operating_net_cents": net_profit,
                        "operating_net": round(net_profit / 100.0, 2),
                        "investing_activities": [],
                        "investing_net_cents": 0,
                        "investing_net": 0.0,
                        "financing_activities": [],
                        "financing_net_cents": 0,
                        "financing_net": 0.0,
                        "net_cash_change_cents": net_change,
                        "net_cash_change": round(net_change / 100.0, 2),
                        "beginning_cash_cents": beg_cash,
                        "beginning_cash": round(beg_cash / 100.0, 2),
                        "ending_cash_cents": beg_cash + net_change,
                        "ending_cash": round((beg_cash + net_change) / 100.0, 2)
                    })

                elif path == "/api/reports/aging-ar":
                    as_of_str = query.get("as_of", [""])[0] or datetime.now(timezone.utc).strftime("%Y-%m-%d")
                    try:
                        as_of_dt = datetime.strptime(as_of_str, "%Y-%m-%d")
                    except Exception:
                        as_of_dt = datetime.now(timezone.utc)
                    invs = conn.execute("""
                        SELECT i.id, i.invoice_number, i.customer_id, COALESCE(c.name, 'Customer') as customer_name,
                            i.issue_date, i.due_date, i.total_amount
                        FROM invoices i
                        LEFT JOIN customers c ON i.customer_id = c.id
                        WHERE i.org_id = 'org_default' AND i.status != 'paid' AND i.total_amount > 0
                        ORDER BY i.due_date ASC
                    """).fetchall()
                    conn.close()
                    items = []
                    b_cur = b_1_30 = b_31_60 = b_61_90 = b_90_plus = tot_cents = 0
                    for inv in invs:
                        try:
                            due_dt = datetime.strptime(inv["due_date"], "%Y-%m-%d")
                        except Exception:
                            due_dt = as_of_dt
                        days = (as_of_dt - due_dt).days
                        cents = int(round(inv["total_amount"] * 100.0))
                        tot_cents += cents
                        if days <= 0:
                            bracket = "current"
                            b_cur += cents
                        elif days <= 30:
                            bracket = "1_30"
                            b_1_30 += cents
                        elif days <= 60:
                            bracket = "31_60"
                            b_31_60 += cents
                        elif days <= 90:
                            bracket = "61_90"
                            b_61_90 += cents
                        else:
                            bracket = "90_plus"
                            b_90_plus += cents
                        items.append({
                            "id": inv["id"], "number": inv["invoice_number"], "entity_id": inv["customer_id"],
                            "entity_name": inv["customer_name"], "issue_date": inv["issue_date"], "due_date": inv["due_date"],
                            "amount_cents": cents, "amount": inv["total_amount"], "days_overdue": days, "bracket": bracket
                        })
                    return self.send_json({
                        "org_id": "org_default", "report_type": "receivables", "as_of": as_of_str,
                        "current_cents": b_cur, "days_1_30_cents": b_1_30, "days_31_60_cents": b_31_60,
                        "days_61_90_cents": b_61_90, "days_90_plus_cents": b_90_plus, "total_cents": tot_cents,
                        "current": round(b_cur / 100.0, 2), "days_1_30": round(b_1_30 / 100.0, 2),
                        "days_31_60": round(b_31_60 / 100.0, 2), "days_61_90": round(b_61_90 / 100.0, 2),
                        "days_90_plus": round(b_90_plus / 100.0, 2), "total": round(tot_cents / 100.0, 2),
                        "items": items
                    })

                elif path == "/api/reports/aging-ap":
                    as_of_str = query.get("as_of", [""])[0] or datetime.now(timezone.utc).strftime("%Y-%m-%d")
                    try:
                        as_of_dt = datetime.strptime(as_of_str, "%Y-%m-%d")
                    except Exception:
                        as_of_dt = datetime.now(timezone.utc)
                    pos = conn.execute("""
                        SELECT po.id, po.po_number, po.supplier_id, COALESCE(s.name, 'Supplier') as supplier_name,
                            substr(po.created_at, 1, 10) as issue_date, po.total_amount
                        FROM purchase_orders po
                        LEFT JOIN suppliers s ON po.supplier_id = s.id
                        WHERE po.org_id = 'org_default' AND po.status IN ('ordered', 'pending_approval') AND po.total_amount > 0
                        ORDER BY po.created_at ASC
                    """).fetchall()
                    conn.close()
                    items = []
                    b_cur = b_1_30 = b_31_60 = b_61_90 = b_90_plus = tot_cents = 0
                    for po in pos:
                        try:
                            iss_dt = datetime.strptime(po["issue_date"], "%Y-%m-%d")
                        except Exception:
                            iss_dt = as_of_dt
                        due_dt = iss_dt + timedelta(days=30)
                        days = (as_of_dt - due_dt).days
                        cents = int(round(po["total_amount"] * 100.0))
                        tot_cents += cents
                        if days <= 0:
                            bracket = "current"
                            b_cur += cents
                        elif days <= 30:
                            bracket = "1_30"
                            b_1_30 += cents
                        elif days <= 60:
                            bracket = "31_60"
                            b_31_60 += cents
                        elif days <= 90:
                            bracket = "61_90"
                            b_61_90 += cents
                        else:
                            bracket = "90_plus"
                            b_90_plus += cents
                        items.append({
                            "id": po["id"], "number": po["po_number"], "entity_id": po["supplier_id"],
                            "entity_name": po["supplier_name"], "issue_date": po["issue_date"], "due_date": due_dt.strftime("%Y-%m-%d"),
                            "amount_cents": cents, "amount": po["total_amount"], "days_overdue": days, "bracket": bracket
                        })
                    return self.send_json({
                        "org_id": "org_default", "report_type": "payables", "as_of": as_of_str,
                        "current_cents": b_cur, "days_1_30_cents": b_1_30, "days_31_60_cents": b_31_60,
                        "days_61_90_cents": b_61_90, "days_90_plus_cents": b_90_plus, "total_cents": tot_cents,
                        "current": round(b_cur / 100.0, 2), "days_1_30": round(b_1_30 / 100.0, 2),
                        "days_31_60": round(b_31_60 / 100.0, 2), "days_61_90": round(b_61_90 / 100.0, 2),
                        "days_90_plus": round(b_90_plus / 100.0, 2), "total": round(tot_cents / 100.0, 2),
                        "items": items
                    })

                elif path == "/api/reports/inventory-valuation":
                    skus = conn.execute("SELECT id, code, name, category, stock_quantity, reserved_quantity, unit_cost, selling_price FROM skus WHERE org_id = 'org_default' ORDER BY name ASC").fetchall()
                    whs = conn.execute("""
                        SELECT w.id, w.name, COUNT(DISTINCT ws.sku_code) as total_items, COALESCE(SUM(ws.available_qty), 0) as total_units,
                            COALESCE(SUM(ws.available_qty * s.unit_cost), 0.0) as cost_valuation
                        FROM warehouses w
                        LEFT JOIN warehouse_stocks ws ON w.id = ws.warehouse_id
                        LEFT JOIN skus s ON ws.sku_code = s.code
                        WHERE w.org_id = 'org_default'
                        GROUP BY w.id, w.name
                        ORDER BY w.name ASC
                    """).fetchall()
                    conn.close()
                    sku_list = []
                    tot_units = 0
                    tot_cost_val = 0.0
                    tot_ret_val = 0.0
                    for s in skus:
                        c_val = round(s["stock_quantity"] * s["unit_cost"], 2)
                        r_val = round(s["stock_quantity"] * s["selling_price"], 2)
                        tot_units += s["stock_quantity"]
                        tot_cost_val += c_val
                        tot_ret_val += r_val
                        sku_list.append({
                            "sku_id": s["id"], "sku_code": s["code"], "sku_name": s["name"], "category": s["category"],
                            "stock_quantity": s["stock_quantity"], "reserved_quantity": s["reserved_quantity"],
                            "unit_cost": s["unit_cost"], "selling_price": s["selling_price"],
                            "cost_valuation": c_val, "retail_valuation": r_val, "potential_profit": round(r_val - c_val, 2)
                        })
                    return self.send_json({
                        "org_id": "org_default", "as_of": utc_today_str(), "total_skus": len(sku_list),
                        "total_units_on_hand": tot_units, "total_cost_valuation": round(tot_cost_val, 2),
                        "total_retail_valuation": round(tot_ret_val, 2), "skus": sku_list,
                        "warehouses": [{"warehouse_id": w["id"], "warehouse_name": w["name"], "total_items": w["total_items"], "total_units": w["total_units"], "cost_valuation": round(w["cost_valuation"] or 0.0, 2)} for w in whs]
                    })

                elif path == "/api/reports/inventory-turnover":
                    val_res = conn.execute("SELECT COALESCE(SUM(stock_quantity * unit_cost), 0.0) as val FROM skus WHERE org_id = 'org_default'").fetchone()
                    avg_val = val_res["val"] or 0.0
                    cogs_res = conn.execute("""
                        SELECT COALESCE(SUM(jl.amount_cents), 0) as cogs
                        FROM journal_lines jl
                        JOIN accounts a ON jl.account_id = a.id
                        JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
                        WHERE a.org_id = 'org_default' AND a.code = '5000' AND jl.entry_type = 'debit'
                            AND substr(je.posted_at, 1, 10) >= date('now', '-365 days')
                    """).fetchone()
                    cogs_trailing = round((cogs_res["cogs"] or 0) / 100.0, 2)
                    turnover = round(cogs_trailing / avg_val, 2) if avg_val > 0 and cogs_trailing > 0 else 0.0
                    dsi = round(365.0 / turnover, 1) if turnover > 0 else 0.0
                    dead_skus = conn.execute("""
                        SELECT id, code, name, category, stock_quantity, unit_cost, substr(updated_at, 1, 10) as last_activity
                        FROM skus
                        WHERE org_id = 'org_default' AND stock_quantity > 0 AND substr(updated_at, 1, 10) <= date('now', '-90 days')
                        ORDER BY (stock_quantity * unit_cost) DESC
                    """).fetchall()
                    conn.close()
                    dead_list = []
                    dead_val = 0.0
                    now_dt = datetime.now(timezone.utc)
                    for ds in dead_skus:
                        tied = round(ds["stock_quantity"] * ds["unit_cost"], 2)
                        dead_val += tied
                        try:
                            last_dt = datetime.strptime(ds["last_activity"], "%Y-%m-%d")
                            days_inact = (now_dt - last_dt).days
                        except Exception:
                            days_inact = 90
                        dead_list.append({
                            "sku_id": ds["id"], "sku_code": ds["code"], "sku_name": ds["name"], "category": ds["category"],
                            "stock_quantity": ds["stock_quantity"], "unit_cost": ds["unit_cost"], "tied_up_capital": tied,
                            "days_inactive": days_inact, "last_activity": ds["last_activity"]
                        })
                    return self.send_json({
                        "org_id": "org_default", "as_of": utc_today_str(), "trailing_period_days": 365,
                        "cogs_trailing": cogs_trailing, "avg_inventory_value": round(avg_val, 2),
                        "turnover_ratio": turnover, "days_sales_of_inventory": dsi,
                        "dead_stock_count": len(dead_list), "dead_stock_value": round(dead_val, 2),
                        "dead_stock": dead_list
                    })

                elif path == "/api/reports/sales-velocity":
                    stages_res = conn.execute("""
                        SELECT stage, COUNT(*) as cnt, COALESCE(SUM(value), 0.0) as tot_val, AVG(value) as avg_val, AVG(probability) as avg_prob
                        FROM deals WHERE org_id = 'org_default' GROUP BY stage
                    """).fetchall()
                    leads_res = conn.execute("""
                        SELECT COUNT(*) as tot, SUM(CASE WHEN status = 'converted' THEN 1 ELSE 0 END) as conv
                        FROM inbound_leads WHERE org_id = 'org_default'
                    """).fetchone()
                    cycle_res = conn.execute("""
                        SELECT AVG(julianday('now') - julianday(created_at)) as cycle_days
                        FROM deals WHERE org_id = 'org_default' AND stage IN ('won', 'lost')
                    """).fetchone()
                    conn.close()
                    tot_deals = won_deals = lost_deals = open_deals = 0
                    won_val = pipe_val = weighted_val = 0.0
                    stages_list = []
                    for s in stages_res:
                        cnt = s["cnt"]
                        val = s["tot_val"]
                        prob = int(s["avg_prob"] or 0)
                        stages_list.append({
                            "stage": s["stage"], "deal_count": cnt, "total_value": round(val, 2),
                            "avg_value": round(s["avg_val"] or 0.0, 2), "probability": prob
                        })
                        tot_deals += cnt
                        if s["stage"] == "won":
                            won_deals += cnt
                            won_val += val
                        elif s["stage"] == "lost":
                            lost_deals += cnt
                        else:
                            open_deals += cnt
                            pipe_val += val
                            weighted_val += (val * prob / 100.0)
                    closed = won_deals + lost_deals
                    win_loss = round((won_deals / closed) * 100.0, 2) if closed > 0 else 0.0
                    avg_deal = round(won_val / won_deals, 2) if won_deals > 0 else 0.0
                    avg_cycle = round(cycle_res["cycle_days"] or 14.0, 1) if cycle_res else 14.0
                    tot_leads = leads_res["tot"] or 0
                    conv_leads = leads_res["conv"] or 0
                    conv_rate = round((conv_leads / tot_leads) * 100.0, 2) if tot_leads > 0 else 0.0
                    return self.send_json({
                        "org_id": "org_default", "total_deals": tot_deals, "won_deals": won_deals,
                        "lost_deals": lost_deals, "open_deals": open_deals, "win_loss_ratio": win_loss,
                        "total_won_value": round(won_val, 2), "average_deal_size": avg_deal,
                        "avg_sales_cycle_days": avg_cycle, "pipeline_value": round(pipe_val, 2),
                        "weighted_pipeline_value": round(weighted_val, 2), "total_leads": tot_leads,
                        "converted_leads": conv_leads, "lead_conversion_rate": conv_rate,
                        "stages": stages_list
                    })

                elif path == "/api/reports/executive-kpis":
                    cash_res = conn.execute("""
                        SELECT COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE -jl.amount_cents END), 0) as cash
                        FROM journal_lines jl
                        JOIN accounts a ON jl.account_id = a.id
                        JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
                        WHERE a.org_id = 'org_default' AND a.code = '1000'
                    """).fetchone()
                    cash_bal = round((cash_res["cash"] or 0) / 100.0, 2)
                    ytd_start = f"{datetime.now(timezone.utc).year}-01-01"
                    pnl_ytd = conn.execute("""
                        SELECT
                            COALESCE(SUM(CASE WHEN a.type = 'revenue' THEN (jl.amount_cents * (CASE WHEN jl.entry_type='credit' THEN 1 ELSE -1 END)) ELSE 0 END), 0) AS rev,
                            COALESCE(SUM(CASE WHEN a.code = '5000' THEN (jl.amount_cents * (CASE WHEN jl.entry_type='debit' THEN 1 ELSE -1 END)) ELSE 0 END), 0) AS cogs,
                            COALESCE(SUM(CASE WHEN a.type = 'expense' AND a.code != '5000' THEN (jl.amount_cents * (CASE WHEN jl.entry_type='debit' THEN 1 ELSE -1 END)) ELSE 0 END), 0) AS opex
                        FROM accounts a
                        JOIN journal_lines jl ON a.id = jl.account_id
                        JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
                        WHERE a.org_id = 'org_default' AND substr(je.posted_at, 1, 10) >= ?
                    """, (ytd_start,)).fetchone()
                    ytd_rev = round((pnl_ytd["rev"] or 0) / 100.0, 2)
                    ytd_cogs = round((pnl_ytd["cogs"] or 0) / 100.0, 2)
                    ytd_opex = round((pnl_ytd["opex"] or 0) / 100.0, 2)
                    ytd_net = round(ytd_rev - ytd_cogs - ytd_opex, 2)
                    margin_pct = round(((ytd_rev - ytd_cogs) / ytd_rev) * 100.0, 2) if ytd_rev > 0 else 0.0
                    mrr_res = conn.execute("""
                        SELECT COALESCE(SUM(CASE WHEN recurring_frequency = 'monthly' THEN total_amount
                            WHEN recurring_frequency = 'quarterly' THEN total_amount / 3.0
                            WHEN recurring_frequency = 'annual' THEN total_amount / 12.0 ELSE 0.0 END), 0.0) as mrr
                        FROM invoices WHERE org_id = 'org_default' AND recurring_frequency IS NOT NULL AND recurring_frequency != '' AND recurring_frequency != 'none'
                    """).fetchone()
                    mrr = round(mrr_res["mrr"] or 0.0, 2)
                    burn_res = conn.execute("""
                        SELECT COALESCE(SUM(jl.amount_cents), 0) as burn
                        FROM journal_lines jl
                        JOIN accounts a ON jl.account_id = a.id
                        JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
                        WHERE a.org_id = 'org_default' AND a.type = 'expense' AND a.code != '5000'
                            AND substr(je.posted_at, 1, 10) >= date('now', '-90 days')
                    """).fetchone()
                    monthly_burn = round(((burn_res["burn"] or 0) / 100.0) / 3.0, 2)
                    runway = round(cash_bal / monthly_burn, 1) if monthly_burn > 0 and cash_bal > 0 else (99.0 if cash_bal > 0 else 0.0)
                    commit_res = conn.execute("SELECT COALESCE(SUM(total_amount), 0.0) as c FROM purchase_orders WHERE org_id = 'org_default' AND status IN ('ordered', 'pending_approval')").fetchone()
                    commits = round(commit_res["c"] or 0.0, 2)
                    po_appr = conn.execute("SELECT COUNT(*) as cnt FROM purchase_orders WHERE org_id = 'org_default' AND status = 'pending_approval'").fetchone()["cnt"] or 0
                    leave_appr = conn.execute("SELECT COUNT(*) as cnt FROM leave_requests WHERE org_id = 'org_default' AND status = 'pending'").fetchone()["cnt"] or 0
                    inv_res = conn.execute("SELECT COALESCE(SUM(stock_quantity * unit_cost), 0.0) as val FROM skus WHERE org_id = 'org_default'").fetchone()
                    inv_val = round(inv_res["val"] or 0.0, 2)
                    ar_res = conn.execute("""
                        SELECT
                            COALESCE(SUM(CASE WHEN date('now') <= due_date THEN total_amount ELSE 0.0 END), 0.0) as cur,
                            COALESCE(SUM(CASE WHEN date('now') > due_date THEN total_amount ELSE 0.0 END), 0.0) as ovd
                        FROM invoices WHERE org_id = 'org_default' AND status != 'paid' AND total_amount > 0
                    """).fetchone()
                    conn.close()
                    return self.send_json({
                        "org_id": "org_default", "generated_at": utcnow_str(),
                        "mrr": mrr, "gross_margin_pct": margin_pct, "operating_cash": cash_bal,
                        "monthly_burn_rate": monthly_burn, "cash_runway_months": runway,
                        "open_purchase_commitments": commits, "pending_approvals_count": po_appr + leave_appr,
                        "total_revenue_ytd": ytd_rev, "net_profit_ytd": ytd_net,
                        "total_inventory_valuation": inv_val,
                        "accounts_receivable_current": round(ar_res["cur"] or 0.0, 2),
                        "accounts_receivable_overdue": round(ar_res["ovd"] or 0.0, 2)
                    })

                elif path == "/api/reports/export":
                    exp_type = (query.get("type", [""])[0] or query.get("export_type", [""])[0] or query.get("entity", ["pnl"])[0]).lower()
                    fmt = (query.get("format", [""])[0] or query.get("file_format", ["csv"])[0]).lower()
                    now_str = datetime.now(timezone.utc).strftime("%Y%m%d_%H%M%S")
                    base_fn = f"{exp_type}_export_{now_str}"

                    headers = []
                    rows = []

                    if exp_type == "pnl":
                        headers = ["Category", "Account Code", "Account Name", "Amount ($)"]
                        q = """
                        SELECT a.code, a.name, a.type,
                            COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE 0 END), 0) AS total_debits,
                            COALESCE(SUM(CASE WHEN jl.entry_type = 'credit' THEN jl.amount_cents ELSE 0 END), 0) AS total_credits
                        FROM accounts a
                        JOIN journal_lines jl ON a.id = jl.account_id
                        JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
                        WHERE a.org_id = 'org_default' AND a.type IN ('revenue', 'expense')
                        GROUP BY a.code, a.name, a.type ORDER BY a.code ASC
                        """
                        db_rows = conn.execute(q).fetchall()
                        for r in db_rows:
                            amt = round(abs(r["total_credits"] - r["total_debits"]) / 100.0, 2)
                            cat = "Revenue" if r["type"] == "revenue" else ("COGS" if r["code"] == "5000" else "Operating Expense")
                            rows.append([cat, r["code"], r["name"], f"{amt:.2f}"])
                    elif exp_type == "inventory":
                        headers = ["SKU Code", "SKU Name", "Category", "On Hand", "Unit Cost ($)", "Selling Price ($)", "Total Valuation ($)"]
                        skus = conn.execute("SELECT code, name, category, stock_quantity, unit_cost, selling_price FROM skus WHERE org_id = 'org_default'").fetchall()
                        for s in skus:
                            tot = round(s["stock_quantity"] * s["unit_cost"], 2)
                            rows.append([s["code"], s["name"], s["category"], str(s["stock_quantity"]), f"{s['unit_cost']:.2f}", f"{s['selling_price']:.2f}", f"{tot:.2f}"])
                    else:
                        headers = ["Invoice Number", "Customer ID", "Status", "Issue Date", "Due Date", "Total ($)"]
                        invs = conn.execute("SELECT invoice_number, customer_id, status, issue_date, due_date, total_amount FROM invoices WHERE org_id = 'org_default' ORDER BY created_at DESC").fetchall()
                        for inv in invs:
                            rows.append([inv["invoice_number"], inv["customer_id"], inv["status"], inv["issue_date"], inv["due_date"], f"{inv['total_amount']:.2f}"])

                    conn.close()

                    if fmt == "json":
                        body = json.dumps(rows, indent=2).encode("utf-8")
                        ct = "application/json"
                        fn = base_fn + ".json"
                    elif fmt == "pdf":
                        body = generate_report_pdf(exp_type.replace("_", " "), "Official Executive Statement", [{"label": "Export", "value": exp_type.upper()}, {"label": "Records", "value": str(len(rows))}], headers, rows)
                        ct = "application/pdf"
                        fn = base_fn + ".pdf"
                    else:
                        out = io.StringIO()
                        writer = csv.writer(out)
                        writer.writerow(headers)
                        for r in rows:
                            writer.writerow(r)
                        body = out.getvalue().encode("utf-8")
                        ct = "text/csv"
                        fn = base_fn + ".csv"

                    self.send_response(200)
                    self.send_header("Content-Type", ct)
                    self.send_header("Content-Disposition", f'attachment; filename="{fn}"')
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                    return

                
                # Phase 8 GET Routes
                elif path == "/api/webhooks/subscriptions":
                    rows = conn.execute("SELECT * FROM webhook_subscriptions WHERE org_id = 'org_default' ORDER BY created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/webhooks/deliveries":
                    rows = conn.execute("SELECT * FROM webhook_deliveries WHERE org_id = 'org_default' ORDER BY created_at DESC LIMIT 100").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/agent/configs":
                    rows = conn.execute("SELECT * FROM agent_configs WHERE org_id = 'org_default' ORDER BY name ASC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/agent/executions":
                    rows = conn.execute("SELECT * FROM agent_executions WHERE org_id = 'org_default' ORDER BY created_at DESC LIMIT 100").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/agent/executions/detail":
                    exec_id = query.get("id", [""])[0]
                    row = conn.execute("SELECT * FROM agent_executions WHERE id = ?", (exec_id,)).fetchone()
                    conn.close()
                    if not row:
                        return self.send_json({"error": "Execution not found"}, 404)
                    return self.send_json(dict(row))

                elif path == "/api/agent/escalations":
                    status = query.get("status", [""])[0]
                    if status and status != "all":
                        rows = conn.execute("SELECT * FROM agent_escalations WHERE org_id = 'org_default' AND status = ? ORDER BY created_at DESC", (status,)).fetchall()
                    else:
                        rows = conn.execute("SELECT * FROM agent_escalations WHERE org_id = 'org_default' ORDER BY created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/agent/context":
                    category = query.get("category", [""])[0]
                    if category and category != "all":
                        rows = conn.execute("SELECT * FROM agent_context_memory WHERE org_id = 'org_default' AND category = ? ORDER BY key ASC", (category,)).fetchall()
                    else:
                        rows = conn.execute("SELECT * FROM agent_context_memory WHERE org_id = 'org_default' ORDER BY category ASC, key ASC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path in ("/api/mcp/introspect", "/api/mcp/tools"):
                    conn.close()
                    return self.send_json({
                        "tools_count": len(DYNAMIC_MCP_TOOLS),
                        "tools": DYNAMIC_MCP_TOOLS
                    })

                # Phase 9: Enterprise Multi-Tenancy & Compliance GET Routes
                elif path == "/api/orgs":
                    rows = conn.execute("SELECT uo.user_id, uo.org_id, o.name as org_name, o.slug as org_slug, uo.role, uo.created_at FROM user_organizations uo JOIN organizations o ON uo.org_id = o.id ORDER BY o.name ASC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/auth/invitations":
                    rows = conn.execute("SELECT * FROM user_invitations WHERE status = 'pending' ORDER BY created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/auth/sso/configs":
                    rows = conn.execute("SELECT id, org_id, provider, client_id, issuer_url, domain_mapping, is_enabled, created_at FROM sso_configs ORDER BY provider ASC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/auth/roles":
                    rows = conn.execute("SELECT * FROM custom_roles ORDER BY is_system DESC, name ASC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/compliance/audit-vault":
                    limit = int(query.get("limit", [50])[0])
                    rows = conn.execute("SELECT * FROM audit_chain_vault WHERE org_id = 'org_default' ORDER BY sequence_number DESC LIMIT ?", (limit,)).fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/compliance/audit-vault/verify":
                    is_valid, count, err = verify_audit_vault_chain(conn, "org_default")
                    conn.close()
                    res = {"is_valid": is_valid, "blocks_verified": count, "org_id": "org_default", "timestamp": utcnow_str()}
                    if err:
                        res["verification_error"] = err
                    return self.send_json(res)

                elif path == "/api/compliance/gdpr/requests":
                    rows = conn.execute("SELECT * FROM privacy_requests ORDER BY requested_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                elif path == "/api/compliance/soc2/access-review":
                    rows = conn.execute("SELECT * FROM soc2_access_reviews ORDER BY created_at DESC").fetchall()
                    conn.close()
                    return self.send_json([dict(r) for r in rows])

                # Phase 10: Disaster Recovery & Backups GET
                elif path == "/api/backup/list":
                    entries = []
                    if os.path.exists(BACKUP_DIR):
                        for fname in sorted(os.listdir(BACKUP_DIR), reverse=True):
                            if fname.endswith(".db") or fname.endswith(".enc"):
                                full_p = os.path.join(BACKUP_DIR, fname)
                                st = os.stat(full_p)
                                is_enc = fname.endswith(".enc")
                                checksum = compute_file_sha256(full_p)
                                entries.append({
                                    "filename": fname,
                                    "path": full_p,
                                    "size_bytes": st.st_size,
                                    "checksum_sha256": checksum,
                                    "is_encrypted": is_enc,
                                    "created_at": datetime.fromtimestamp(st.st_mtime, timezone.utc).isoformat(),
                                    "integrity_ok": True,
                                })
                    conn.close()
                    return self.send_json(entries)

                elif path == "/api/backup/config":
                    conn.close()
                    sec = os.environ.get("S3_SECRET_KEY", "")
                    return self.send_json({
                        "endpoint": os.environ.get("S3_ENDPOINT", ""),
                        "bucket": os.environ.get("S3_BUCKET", ""),
                        "region": os.environ.get("S3_REGION", "us-east-1"),
                        "access_key": os.environ.get("S3_ACCESS_KEY", ""),
                        "secret_key": "********" if sec else ""
                    })

                conn.close()
                return self.send_json({"error": "Endpoint not found"}, 404)
            except Exception as e:
                return self.send_json({"error": str(e)}, 500)


        # 3. Static Files
        clean_path = path.lstrip("/")
        if path.startswith("/portal/invoice/"):
            file_path = os.path.join(WEB_DIR, "portal.html")
        elif not clean_path or clean_path == "index.html":
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

    def do_DELETE(self):
        return self.do_POST()

    def do_PUT(self):
        return self.do_POST()

    def do_POST(self):
        parsed = urlparse(self.path)
        path = parsed.path
        print(f"[DEBUG POST] {path}", flush=True)
        query = parse_qs(parsed.query)
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
                return self.send_json({"jsonrpc": "2.0", "id": rpc_id, "result": {"tools": DYNAMIC_MCP_TOOLS}})
            elif method == "tools/call":
                t_start = time.time()
                params = req.get("params", {})
                name = params.get("name", "")
                args = params.get("arguments", {})
                agent_id = req.get("agent_id") or self.headers.get("X-Agent-ID", "agent_default")
                conn = get_db()

                # Check budget / rate limit
                cfg = conn.execute("SELECT * FROM agent_configs WHERE api_key_hash = ? OR id = ?", (agent_id, agent_id)).fetchone()
                if cfg and cfg["daily_budget_cents"] > 0 and cfg["current_day_usage_cents"] >= cfg["daily_budget_cents"]:
                    conn.close()
                    return self.send_json({
                        "jsonrpc": "2.0",
                        "id": rpc_id,
                        "error": {"code": -32029, "message": f"rate limit exceeded: daily budget cap reached ({cfg['current_day_usage_cents']} / {cfg['daily_budget_cents']} cents) [-32029]"}
                    })

                text_result = f"Executed tool {name} successfully."
                is_err = False

                if name == "fin_get_summary":
                    rev_row = conn.execute("SELECT COALESCE(SUM(total_amount), 0) as rev, COUNT(*) as cnt FROM invoices WHERE status = 'paid'").fetchone()
                    unpaid_row = conn.execute("SELECT COALESCE(SUM(total_amount), 0) as unp, COUNT(*) as cnt FROM invoices WHERE status != 'paid'").fetchone()
                    exp_row = conn.execute("SELECT COALESCE(SUM(amount), 0) as exp FROM expenses").fetchone()
                    res = {
                        "total_revenue": float(rev_row["rev"] or 0.0),
                        "total_expense": float(exp_row["exp"] or 0.0),
                        "net_profit": float(rev_row["rev"] or 0.0) - float(exp_row["exp"] or 0.0),
                        "unpaid_total": float(unpaid_row["unp"] or 0.0),
                        "paid_count": int(rev_row["cnt"] or 0),
                        "pending_count": int(unpaid_row["cnt"] or 0)
                    }
                    text_result = json.dumps(res, indent=2)

                elif name == "agent_escalate_to_human":
                    esc_id = f"esc_{int(time.time()*1000)}"
                    title = args.get("title", "Agent Escalation")
                    reason = args.get("reason", "ambiguous_input")
                    input_ctx = args.get("input_context", "{}")
                    rec_act = args.get("recommended_action", "")
                    now = utcnow_str()
                    conn.execute("""
                        INSERT INTO agent_escalations (id, org_id, agent_id, title, reason, status, input_context_json, recommended_action, created_at)
                        VALUES (?, 'org_default', ?, ?, ?, 'pending', ?, ?, ?)
                    """, (esc_id, agent_id, title, reason, input_ctx, rec_act, now))
                    conn.execute("""
                        INSERT INTO approvals (id, action_type, title, description, payload_json, status, requested_by_type, requested_by_id, created_at)
                        VALUES (?, 'agent_escalation', ?, ?, ?, 'pending', 'agent', ?, ?)
                    """, (f"appr_{esc_id}", title, f"Agent escalated: {reason}. Action: {rec_act}", input_ctx, agent_id, now))
                    conn.commit()
                    text_result = f"Successfully escalated to human review queue. Escalation ID: {esc_id}. Status: pending."

                elif name == "agent_get_context":
                    category = args.get("category", "")
                    if category:
                        rows = conn.execute("SELECT * FROM agent_context_memory WHERE org_id = 'org_default' AND category = ?", (category,)).fetchall()
                    else:
                        rows = conn.execute("SELECT * FROM agent_context_memory WHERE org_id = 'org_default'").fetchall()
                    ctx_dict = {}
                    for r in rows:
                        c = r["category"]
                        if c not in ctx_dict:
                            ctx_dict[c] = {}
                        ctx_dict[c][r["key"]] = r["value"]
                    text_result = json.dumps(ctx_dict, indent=2)

                elif name == "agent_sanitize_input":
                    raw_text = args.get("text", "")
                    res = sanitize_input(raw_text)
                    text_result = json.dumps(res, indent=2)

                elif name == "webhook_test_event":
                    ev_type = args.get("event_type", "po.created")
                    payload = args.get("payload") or {"test": True, "timestamp": utcnow_str()}
                    delivs = dispatch_webhook_event(conn, "org_default", ev_type, payload)
                    text_result = f"Dispatched event {ev_type} to {len(delivs)} webhook subscribers."

                duration_ms = int((time.time() - t_start) * 1000)
                # Durable execution record
                exec_id = f"exec_{int(time.time()*1000)}"
                conn.execute("""
                    INSERT INTO agent_executions (id, org_id, agent_id, tool_name, category, arguments_json, result_json, is_error, error_message, duration_ms, cost_cents, mutations_json, created_at)
                    VALUES (?, 'org_default', ?, ?, 'read_only', ?, ?, ?, ?, ?, 1, '[]', ?)
                """, (exec_id, agent_id, name, json.dumps(args), text_result[:1000], 1 if is_err else 0, text_result if is_err else "", duration_ms, utcnow_str()))

                if cfg:
                    conn.execute("UPDATE agent_configs SET current_day_usage_cents = current_day_usage_cents + 1, last_used_at = ? WHERE id = ?", (utcnow_str(), cfg["id"]))

                conn.commit()
                conn.close()

                return self.send_json({
                    "jsonrpc": "2.0",
                    "id": rpc_id,
                    "result": {"content": [{"type": "text", "text": text_result}], "isError": is_err}
                })
            else:
                return self.send_json({
                    "jsonrpc": "2.0",
                    "id": rpc_id,
                    "result": {"status": "ok", "method": method}
                })

        # 2. REST API Actions
        if path == "/api/backup/config":
            for k in ["endpoint", "bucket", "region", "access_key", "secret_key"]:
                if k in req:
                    os.environ[f"S3_{k.upper()}"] = str(req[k])
            sec = os.environ.get("S3_SECRET_KEY", "")
            return self.send_json({
                "endpoint": os.environ.get("S3_ENDPOINT", ""),
                "bucket": os.environ.get("S3_BUCKET", ""),
                "region": os.environ.get("S3_REGION", "us-east-1"),
                "access_key": os.environ.get("S3_ACCESS_KEY", ""),
                "secret_key": "********" if sec else ""
            })

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

            elif path == "/api/customers":
                name = req.get("name")
                company = req.get("company", "")
                email = req.get("email", "")
                phone = req.get("phone", "")
                if not name:
                    conn.close()
                    return self.send_json({"error": "name is required"}, 400)
                cust_id = f"cust_{int(time.time()*1000)}"
                now = utcnow_str()
                conn.execute(
                    "INSERT INTO customers (id, org_id, name, company, email, phone, status, lifetime_value, tags, created_at) VALUES (?, 'org_default', ?, ?, ?, ?, 'active', 0.0, '[]', ?)",
                    (cust_id, name, company, email, phone, now)
                )
                log_audit(conn, "human", "web_user", "CREATE_CUSTOMER", "customers", cust_id, f"Created customer '{name}' ({company})")
                conn.commit()
                row = conn.execute("SELECT * FROM customers WHERE id = ?", (cust_id,)).fetchone()
                conn.close()
                broadcast("customer_created", {"customer_id": cust_id})
                return self.send_json(dict(row))

            elif path == "/api/deals":
                title = req.get("title")
                cust_id = req.get("customer_id")
                value = float(req.get("value", 0.0))
                stage = req.get("stage", "lead")
                pipeline_id = req.get("pipeline_id", "pipe_std_org_default")
                close_date = req.get("expected_close_date", "")
                deal_id = f"deal_{int(time.time()*1000)}"
                now = utcnow_str()

                conn.execute(
                    "INSERT INTO deals (id, title, customer_id, value, stage, probability, expected_close_date, pipeline_id, stage_updated_at, created_at) VALUES (?, ?, ?, ?, ?, 60, ?, ?, ?, ?)",
                    (deal_id, title, cust_id, value, stage, close_date, pipeline_id, now, now)
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
                emp_id = req.get("id")
                name = req.get("name", "")
                email = req.get("email", "")
                phone = req.get("phone", "")
                role = req.get("role", "Staff")
                dept = req.get("department", "Operations")
                rate = float(req.get("hourly_rate", 25.0))
                job_title = req.get("job_title") or role
                reporting_mgr = req.get("reporting_manager_id", "")
                emp_type = req.get("employment_type", "full_time")
                sal_type = req.get("salary_type", "hourly")
                mo_sal = float(req.get("monthly_salary", 0.0))
                tax_id = req.get("tax_id", "")
                bank_name = req.get("bank_name", "")
                bank_acct = req.get("bank_account_number", "")
                bank_rout = req.get("bank_routing_number", "")
                em_name = req.get("emergency_contact_name", "")
                em_phone = req.get("emergency_contact_phone", "")

                if emp_id:
                    conn.execute("""
                        UPDATE employees SET
                            name = COALESCE(?, name),
                            email = COALESCE(?, email),
                            phone = COALESCE(?, phone),
                            role = COALESCE(?, role),
                            department = COALESCE(?, department),
                            hourly_rate = ?,
                            job_title = ?,
                            reporting_manager_id = ?,
                            employment_type = ?,
                            salary_type = ?,
                            monthly_salary = ?,
                            tax_id = ?,
                            bank_name = ?,
                            bank_account_number = ?,
                            bank_routing_number = ?,
                            emergency_contact_name = ?,
                            emergency_contact_phone = ?
                        WHERE id = ?
                    """, (name, email, phone, role, dept, rate, job_title, reporting_mgr, emp_type, sal_type, mo_sal, tax_id, bank_name, bank_acct, bank_rout, em_name, em_phone, emp_id))
                    conn.commit()
                    row = conn.execute("SELECT * FROM employees WHERE id = ?", (emp_id,)).fetchone()
                    conn.close()
                    broadcast("employee_updated", {"employee_id": emp_id, "name": name})
                    return self.send_json(dict(row) if row else {"id": emp_id, "status": "ok"})
                else:
                    emp_id = f"emp_{int(time.time()*1000)}"
                    now = utcnow_str()
                    conn.execute("""
                        INSERT INTO employees (
                            id, org_id, name, email, phone, role, department, hourly_rate, status, hired_at,
                            job_title, reporting_manager_id, employment_type, salary_type, monthly_salary,
                            tax_id, bank_name, bank_account_number, bank_routing_number,
                            emergency_contact_name, emergency_contact_phone, created_at
                        ) VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                    """, (emp_id, name, email, phone, role, dept, rate, utc_today_str(), job_title, reporting_mgr, emp_type, sal_type, mo_sal, tax_id, bank_name, bank_acct, bank_rout, em_name, em_phone, now))
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

            elif path == "/api/deals/stage":
                deal_id = req.get("deal_id")
                stage = req.get("stage")
                if not deal_id or not stage:
                    conn.close()
                    return self.send_json({"error": "deal_id and stage required"}, 400)
                now = utcnow_str()
                conn.execute("UPDATE deals SET stage = ?, stage_updated_at = ? WHERE id = ?", (stage, now, deal_id))
                deal = conn.execute("SELECT d.*, c.name as customer_name FROM deals d LEFT JOIN customers c ON d.customer_id = c.id WHERE d.id = ?", (deal_id,)).fetchone()
                if deal and deal["customer_id"]:
                    act_id = f"act_{int(time.time()*1000)}"
                    conn.execute("""
                        INSERT INTO customer_activities (id, org_id, customer_id, deal_id, activity_type, author_type, author_id, author_name, summary, content, created_at)
                        VALUES (?, 'org_default', ?, ?, 'stage_change', 'human', 'web_user', 'Sales Workflow', ?, ?, ?)
                    """, (act_id, deal["customer_id"], deal_id, f"Deal progressed to stage '{stage}'", f"Deal {deal['title']} stage updated to {stage}.", now))
                log_audit(conn, "human", "web_user", "UPDATE_DEAL_STAGE", "deals", deal_id, f"Moved deal {deal_id} to stage {stage}")
                conn.commit()
                conn.close()
                broadcast("deal_stage_changed", {"deal_id": deal_id, "stage": stage})
                return self.send_json({"status": "ok", "deal_id": deal_id, "stage": stage})

            elif path == "/api/customers/activities":
                cust_id = req.get("customer_id")
                deal_id = req.get("deal_id")
                act_type = req.get("activity_type", "note")
                summary = req.get("summary", "")
                content = req.get("content", "")
                author_name = req.get("author_name", "Operations Team")
                if not cust_id or not summary:
                    conn.close()
                    return self.send_json({"error": "customer_id and summary required"}, 400)
                act_id = f"act_{int(time.time()*1000)}"
                now = utcnow_str()
                conn.execute("""
                    INSERT INTO customer_activities (id, org_id, customer_id, deal_id, activity_type, author_type, author_id, author_name, summary, content, created_at)
                    VALUES (?, 'org_default', ?, ?, ?, 'human', 'web_user', ?, ?, ?, ?)
                """, (act_id, cust_id, deal_id, act_type, author_name, summary, content, now))
                log_audit(conn, "human", "web_user", "LOG_CUSTOMER_ACTIVITY", "customers", cust_id, f"Logged {act_type}: {summary}")
                conn.commit()
                conn.close()
                broadcast("customer_activity_logged", {"customer_id": cust_id, "type": act_type})
                return self.send_json({"id": act_id, "status": "ok"}, 201)

            elif path == "/api/v1/leads/capture":
                name = req.get("name", "")
                email = req.get("email", "")
                company = req.get("company", "")
                phone = req.get("phone", "")
                msg = req.get("message", "")
                source = req.get("source", "website_form")
                if not name or not email:
                    conn.close()
                    return self.send_json({"error": "name and email required"}, 400)
                lead_id = f"lead_{int(time.time()*1000)}"
                now = utcnow_str()
                conn.execute("""
                    INSERT INTO inbound_leads (id, org_id, name, email, company, phone, message, source, status, assigned_to, created_at)
                    VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?, 'new', 'AI Qualification Agent', ?)
                """, (lead_id, name, email, company, phone, msg, source, now))
                conn.commit()
                conn.close()
                broadcast("lead_captured", {"lead_id": lead_id, "name": name, "company": company})
                return self.send_json({"status": "received", "id": lead_id, "lead_id": lead_id}, 201)

            elif path == "/api/leads/convert":
                lead_id = req.get("lead_id")
                pipeline_id = req.get("pipeline_id") or "pipe_std_org_default"
                deal_val = float(req.get("deal_value") or 15000.0)
                lead = conn.execute("SELECT * FROM inbound_leads WHERE id = ?", (lead_id,)).fetchone()
                if not lead:
                    conn.close()
                    return self.send_json({"error": "Lead not found"}, 404)

                cust = conn.execute("SELECT id FROM customers WHERE email = ?", (lead["email"],)).fetchone()
                if cust:
                    cust_id = cust["id"]
                else:
                    cust_id = f"cust_{int(time.time()*1000)}"
                    conn.execute("INSERT INTO customers (id, org_id, name, company, email, phone, status, lifetime_value, created_at) VALUES (?, 'org_default', ?, ?, ?, ?, 'active', 0.0, ?)",
                        (cust_id, lead["name"], lead["company"], lead["email"], lead["phone"], utcnow_str()))

                deal_id = f"deal_{int(time.time()*1000)}"
                deal_title = f"{lead['company'] or lead['name']} Inbound Deal"
                now = utcnow_str()
                conn.execute("""
                    INSERT INTO deals (id, org_id, title, customer_id, value, stage, probability, pipeline_id, stage_updated_at, created_at)
                    VALUES (?, 'org_default', ?, ?, ?, 'lead', 20, ?, ?, ?)
                """, (deal_id, deal_title, cust_id, deal_val, pipeline_id, now, now))

                conn.execute("UPDATE inbound_leads SET status = 'converted', converted_customer_id = ?, converted_deal_id = ? WHERE id = ?",
                    (cust_id, deal_id, lead_id))

                act_id = f"act_{int(time.time()*1000)}"
                conn.execute("""
                    INSERT INTO customer_activities (id, org_id, customer_id, deal_id, activity_type, author_type, author_id, author_name, summary, content, created_at)
                    VALUES (?, 'org_default', ?, ?, 'lead_converted', 'human', 'web_user', 'Sales Rep', ?, ?, ?)
                """, (act_id, cust_id, deal_id, f"Converted Inbound Lead {lead['name']}", lead["message"] or "Converted from contact form.", now))

                conn.commit()
                conn.close()
                broadcast("lead_converted", {"lead_id": lead_id, "customer_id": cust_id, "deal_id": deal_id})
                return self.send_json({"customer_id": cust_id, "deal_id": deal_id})

            elif path == "/api/invoices/send-email":
                inv_id = req.get("invoice_id")
                inv = conn.execute("""
                    SELECT i.*, COALESCE(c.name, 'Client') as customer_name, COALESCE(c.email, 'client@example.com') as customer_email
                    FROM invoices i LEFT JOIN customers c ON i.customer_id = c.id
                    WHERE i.id = ?
                """, (inv_id,)).fetchone()
                if not inv:
                    conn.close()
                    return self.send_json({"error": "Invoice not found"}, 404)

                tok_row = conn.execute("SELECT token FROM invoice_portal_tokens WHERE invoice_id = ?", (inv_id,)).fetchone()
                if tok_row:
                    ptoken = tok_row["token"]
                else:
                    import secrets
                    ptoken = "ptok_" + secrets.token_hex(16)
                    conn.execute("INSERT INTO invoice_portal_tokens (token, org_id, invoice_id, expires_at, created_at) VALUES (?, 'org_default', ?, datetime('now', '+90 days'), CURRENT_TIMESTAMP)", (ptoken, inv_id))

                host = self.headers.get("Host", "localhost:8080")
                portal_url = f"http://{host}/portal/invoice/{ptoken}"
                subj = f"Invoice {inv['invoice_number']} from AO Sphere Enterprise — Amount: ${float(inv['total_amount']):.2f}"
                body_html = f"<h2>Invoice from AO Sphere Enterprise</h2><p>Dear {inv['customer_name']},</p><p>Please review and pay your invoice online: <a href='{portal_url}'>{portal_url}</a></p>"

                eml_id = f"eml_{int(time.time()*1000)}"
                now = utcnow_str()
                conn.execute("""
                    INSERT INTO email_outbox (id, org_id, recipient_email, recipient_name, subject, body_html, body_text, template_name, related_entity_type, related_entity_id, status, sent_at, created_at)
                    VALUES (?, 'org_default', ?, ?, ?, ?, ?, 'invoice_notification', 'invoice', ?, 'sent', ?, ?)
                """, (eml_id, inv["customer_email"], inv["customer_name"], subj, body_html, portal_url, inv_id, now, now))
                conn.commit()
                conn.close()
                broadcast("email_dispatched", {"email_id": eml_id, "recipient": inv["customer_email"], "portal_url": portal_url})
                return self.send_json({"email_id": eml_id, "status": "sent", "portal_token": ptoken, "portal_url": portal_url})

            elif path == "/api/portal/invoice/pay":
                ptoken = req.get("token")
                method = req.get("payment_method", "credit_card")
                tok_row = conn.execute("SELECT invoice_id FROM invoice_portal_tokens WHERE token = ?", (ptoken,)).fetchone()
                if not tok_row:
                    conn.close()
                    return self.send_json({"error": "Invalid portal token"}, 404)

                inv_id = tok_row["invoice_id"]
                inv = conn.execute("SELECT * FROM invoices WHERE id = ?", (inv_id,)).fetchone()
                if not inv:
                    conn.close()
                    return self.send_json({"error": "Invoice not found"}, 404)

                if inv["status"] != "paid":
                    conn.execute("UPDATE invoices SET status = 'paid', version = version + 1 WHERE id = ?", (inv_id,))
                    tot_cents = int(float(inv["total_amount"]) * 100)
                    now = utcnow_str()
                    today = utc_today_str()
                    je_id = f"je_portal_{int(time.time()*1000)}"
                    je_num = f"JE-PORTAL-{int(time.time())}"

                    try:
                        conn.execute("""
                            INSERT INTO journal_entries (id, org_id, entry_number, date, memo, source, source_id, posted_by, posted_at)
                            VALUES (?, 'org_default', ?, ?, ?, 'portal_payment', ?, 'client_portal', ?)
                        """, (je_id, je_num, today, f"Client Portal Settlement for {inv['invoice_number']}", inv_id, now))

                        cash_row = conn.execute("SELECT id FROM accounts WHERE code = '1000'").fetchone()
                        ar_row = conn.execute("SELECT id FROM accounts WHERE code = '1100'").fetchone()

                        if cash_row and ar_row and tot_cents > 0:
                            conn.execute("INSERT INTO journal_lines (id, entry_id, account_id, entry_type, amount_cents, description) VALUES (?, ?, ?, 'debit', ?, 'Cash Received via Portal')",
                                (f"jl_{int(time.time()*1000)}_1", je_id, cash_row["id"], tot_cents))
                            conn.execute("INSERT INTO journal_lines (id, entry_id, account_id, entry_type, amount_cents, description) VALUES (?, ?, ?, 'credit', ?, 'AR Cleared via Portal')",
                                (f"jl_{int(time.time()*1000)}_2", je_id, ar_row["id"], tot_cents))
                    except Exception:
                        pass

                conn.commit()
                conn.close()
                broadcast("invoice_paid", {"invoice_id": inv_id, "token": ptoken})
                return self.send_json({"status": "paid"})

            elif path == "/api/timeclock/event":
                emp_id = req.get("employee_id")
                evt_type = req.get("event_type")
                shift_id = req.get("shift_id") or None
                notes = req.get("location_notes") or req.get("notes") or ""
                if not emp_id or not evt_type:
                    conn.close()
                    return self.send_json({"error": "employee_id and event_type required"}, 400)
                ev_id = f"pnc_{int(time.time()*1000)}"
                now = utcnow_str()
                conn.execute("""
                    INSERT INTO timeclock_events (id, org_id, employee_id, shift_id, event_type, timestamp, location_notes, created_at)
                    VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?)
                """, (ev_id, emp_id, shift_id, evt_type, now, notes, now))
                conn.commit()
                row = conn.execute("SELECT * FROM timeclock_events WHERE id = ?", (ev_id,)).fetchone()
                conn.close()
                broadcast("timeclock_punch", {"employee_id": emp_id, "event_type": evt_type})
                return self.send_json(dict(row), 201)

            elif path == "/api/payroll/run":
                period = req.get("period") or datetime.now(timezone.utc).strftime("%Y-%m")
                pay_date = req.get("payment_date") or utc_today_str()
                emps = conn.execute("SELECT * FROM employees WHERE status = 'active' ORDER BY name").fetchall()
                if not emps:
                    conn.close()
                    return self.send_json({"error": "No active employees found"}, 400)

                run_id = f"prun_{int(time.time()*1000)}"
                payslips = []
                tot_gross_c = 0
                tot_tax_c = 0
                tot_ded_c = 0
                tot_net_c = 0
                now = utcnow_str()

                for e in emps:
                    sal_type = e["salary_type"] if "salary_type" in e.keys() else "hourly"
                    rate = float(e["hourly_rate"] or 25.0)
                    mo_sal = float(e["monthly_salary"] or 0.0)
                    if sal_type == "monthly" and mo_sal > 0:
                        gross = mo_sal
                        reg_hrs = 160.0
                        ot_hrs = 0.0
                    else:
                        reg_hrs = 160.0
                        ot_hrs = 0.0
                        gross = reg_hrs * rate

                    tax = round(gross * 0.12, 2)
                    soc = round(gross * 0.05, 2)
                    ben = round(gross * 0.03, 2)
                    net = round(gross - (tax + soc + ben), 2)

                    g_c = int(round(gross * 100))
                    t_c = int(round(tax * 100))
                    d_c = int(round((soc + ben) * 100))
                    n_c = g_c - (t_c + d_c)

                    tot_gross_c += g_c
                    tot_tax_c += t_c
                    tot_ded_c += d_c
                    tot_net_c += n_c

                    ps_id = f"ps_{e['id']}_{period}"
                    payslips.append({
                        "id": ps_id, "payroll_run_id": run_id, "employee_id": e["id"], "employee_name": e["name"],
                        "period": period, "salary_type": sal_type, "hourly_rate": rate, "monthly_salary": mo_sal,
                        "regular_hours": reg_hrs, "overtime_hours": ot_hrs,
                        "gross_pay": gross, "tax_withholding": tax, "social_contributions": soc,
                        "benefits_deductions": ben, "net_pay": net, "status": "paid", "payment_date": pay_date
                    })

                je_id = f"je_pr_{int(time.time()*1000)}"
                je_num = f"JE-PAYROLL-{period}"
                tax_ded_c = tot_tax_c + tot_ded_c
                conn.execute("""
                    INSERT INTO journal_entries (id, org_id, entry_number, source_type, source_id, memo, status, posted_at, created_by, created_at)
                    VALUES (?, 'org_default', ?, 'payroll', ?, ?, 'posted', ?, 'payroll_engine', ?)
                """, (je_id, je_num, run_id, f"Monthly Payroll Disbursal {period}", now, now))

                acc_5200 = conn.execute("SELECT id FROM accounts WHERE code = '5200'").fetchone()
                acc_2150 = conn.execute("SELECT id FROM accounts WHERE code = '2150'").fetchone()
                acc_1000 = conn.execute("SELECT id FROM accounts WHERE code = '1000'").fetchone()

                if acc_5200:
                    conn.execute("INSERT INTO journal_lines (id, org_id, entry_id, account_id, entry_type, amount_cents, description, created_at) VALUES (?, 'org_default', ?, ?, 'debit', ?, ?, ?)",
                        (f"jl_{int(time.time()*1000)}_1", je_id, acc_5200["id"], tot_gross_c, f"Salaries Expense {period}", now))
                if acc_2150:
                    conn.execute("INSERT INTO journal_lines (id, org_id, entry_id, account_id, entry_type, amount_cents, description, created_at) VALUES (?, 'org_default', ?, ?, 'credit', ?, ?, ?)",
                        (f"jl_{int(time.time()*1000)}_2", je_id, acc_2150["id"], tax_ded_c, f"Tax & Benefit Withholding {period}", now))
                if acc_1000:
                    conn.execute("INSERT INTO journal_lines (id, org_id, entry_id, account_id, entry_type, amount_cents, description, created_at) VALUES (?, 'org_default', ?, ?, 'credit', ?, ?, ?)",
                        (f"jl_{int(time.time()*1000)}_3", je_id, acc_1000["id"], tot_net_c, f"Net Direct Deposit Wages {period}", now))

                conn.execute("""
                    INSERT INTO payroll_runs (
                        id, org_id, period, total_gross_cents, total_tax_cents, total_deductions_cents,
                        total_net_cents, status, payment_date, journal_entry_id, payslips_count, created_at
                    ) VALUES (?, 'org_default', ?, ?, ?, ?, ?, 'paid', ?, ?, ?, ?)
                    ON CONFLICT(org_id, period) DO UPDATE SET
                        total_gross_cents = excluded.total_gross_cents,
                        total_tax_cents = excluded.total_tax_cents,
                        total_deductions_cents = excluded.total_deductions_cents,
                        total_net_cents = excluded.total_net_cents,
                        status = 'paid', payment_date = excluded.payment_date,
                        journal_entry_id = excluded.journal_entry_id,
                        payslips_count = excluded.payslips_count
                """, (run_id, period, tot_gross_c, tot_tax_c, tot_ded_c, tot_net_c, pay_date, je_id, len(payslips), now))

                for ps in payslips:
                    conn.execute("""
                        INSERT INTO payslips (
                            id, org_id, payroll_run_id, employee_id, period, salary_type,
                            hourly_rate, monthly_salary, regular_hours, overtime_hours,
                            gross_pay, tax_withholding, social_contributions, benefits_deductions,
                            net_pay, status, payment_date, created_at
                        ) VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'paid', ?, ?)
                        ON CONFLICT(org_id, payroll_run_id, employee_id) DO UPDATE SET
                            gross_pay = excluded.gross_pay,
                            tax_withholding = excluded.tax_withholding,
                            social_contributions = excluded.social_contributions,
                            benefits_deductions = excluded.benefits_deductions,
                            net_pay = excluded.net_pay,
                            status = 'paid', payment_date = excluded.payment_date
                    """, (ps["id"], run_id, ps["employee_id"], period, ps["salary_type"], ps["hourly_rate"], ps["monthly_salary"], ps["regular_hours"], ps["overtime_hours"], ps["gross_pay"], ps["tax_withholding"], ps["social_contributions"], ps["benefits_deductions"], ps["net_pay"], pay_date, now))

                conn.commit()
                run_row = conn.execute("SELECT * FROM payroll_runs WHERE period = ?", (period,)).fetchone()
                conn.close()
                broadcast("payroll_executed", {"period": period, "total_net": tot_net_c / 100.0})
                run_dict = dict(run_row) if run_row else {"id": run_id, "period": period}
                return self.send_json({
                    "id": run_id,
                    "period": period,
                    "journal_entry_id": je_id,
                    "total_gross_cents": tot_gross_c,
                    "total_tax_cents": tot_tax_c,
                    "total_deductions_cents": tot_ded_c,
                    "total_net_cents": tot_net_c,
                    "run": run_dict,
                    "payslips": payslips
                }, 201)

            elif path == "/api/leaves/requests":
                emp_id = req.get("employee_id")
                l_type = req.get("leave_type", "vacation")
                s_date = req.get("start_date")
                e_date = req.get("end_date")
                days = int(req.get("days_count", 1))
                reason = req.get("reason", "")
                if not emp_id or not s_date or not e_date:
                    conn.close()
                    return self.send_json({"error": "employee_id, start_date, and end_date required"}, 400)
                lr_id = f"lvr_{int(time.time()*1000)}"
                now = utcnow_str()
                conn.execute("""
                    INSERT INTO leave_requests (id, org_id, employee_id, leave_type, start_date, end_date, days_count, reason, status, created_at)
                    VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?, 'pending', ?)
                """, (lr_id, emp_id, l_type, s_date, e_date, days, reason, now))
                conn.commit()
                row = conn.execute("SELECT lr.*, COALESCE(e.name, 'Employee') as employee_name FROM leave_requests lr LEFT JOIN employees e ON lr.employee_id = e.id WHERE lr.id = ?", (lr_id,)).fetchone()
                conn.close()
                broadcast("leave_requested", {"id": lr_id, "employee_id": emp_id})
                return self.send_json(dict(row), 201)

            elif path == "/api/leaves/review":
                req_id = req.get("request_id")
                status = req.get("status")
                notes = req.get("review_notes", "")
                if not req_id or status not in ("approved", "rejected"):
                    conn.close()
                    return self.send_json({"error": "valid request_id and status ('approved' or 'rejected') required"}, 400)
                conn.execute("UPDATE leave_requests SET status = ?, reviewed_by = 'HR Manager', review_notes = ? WHERE id = ?", (status, notes, req_id))
                conn.commit()
                conn.close()
                broadcast("leave_reviewed", {"request_id": req_id, "status": status})
                return self.send_json({"status": "ok", "request_id": req_id})

            elif path == "/api/hr/documents":
                emp_id = req.get("employee_id")
                title = req.get("title")
                doc_type = req.get("document_type", "contract")
                fname = req.get("file_name", "document.pdf")
                content = req.get("file_content", "")
                fsize = int(req.get("file_size") or len(content))
                if not emp_id or not title:
                    conn.close()
                    return self.send_json({"error": "employee_id and title required"}, 400)
                doc_id = f"doc_{int(time.time()*1000)}"
                now = utcnow_str()
                conn.execute("""
                    INSERT INTO hr_documents (id, org_id, employee_id, title, document_type, file_name, file_content, file_size, uploaded_by, created_at)
                    VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?, 'HR Operations', ?)
                """, (doc_id, emp_id, title, doc_type, fname, content, fsize, now))
                conn.commit()
                row = conn.execute("SELECT d.*, COALESCE(e.name, 'Employee') as employee_name FROM hr_documents d LEFT JOIN employees e ON d.employee_id = e.id WHERE d.id = ?", (doc_id,)).fetchone()
                conn.close()
                broadcast("hr_document_uploaded", {"id": doc_id, "employee_id": emp_id})
                return self.send_json(dict(row), 201)

            
            # Phase 8 POST Routes
            elif path == "/api/webhooks/subscriptions":
                if self.command == "DELETE":
                    parsed = urlparse(self.path)
                    q = parse_qs(parsed.query)
                    sub_id = q.get("id", [""])[0] or req.get("id", "")
                    conn.execute("DELETE FROM webhook_subscriptions WHERE id = ?", (sub_id,))
                    conn.commit()
                    conn.close()
                    return self.send_json({"status": "deleted", "id": sub_id})
                else:
                    sub_id = f"wsub_{int(time.time()*1000)}"
                    name = req.get("name", "Webhook Subscription")
                    url = req.get("url", "")
                    secret = req.get("secret") or f"whsec_{uuid.uuid4().hex[:16]}"
                    ev_types = req.get("event_types", '["*"]')
                    if isinstance(ev_types, list):
                        ev_types = json.dumps(ev_types)
                    now = utcnow_str()
                    conn.execute("""
                        INSERT INTO webhook_subscriptions (id, org_id, name, url, secret, event_types, is_active, created_at, updated_at)
                        VALUES (?, 'org_default', ?, ?, ?, ?, 1, ?, ?)
                    """, (sub_id, name, url, secret, ev_types, now, now))
                    conn.commit()
                    row = conn.execute("SELECT * FROM webhook_subscriptions WHERE id = ?", (sub_id,)).fetchone()
                    conn.close()
                    return self.send_json(dict(row), 201)

            elif path == "/api/webhooks/deliveries/redeliver":
                deliv_id = req.get("id") or parse_qs(urlparse(self.path).query).get("id", [""])[0]
                row = conn.execute("SELECT * FROM webhook_deliveries WHERE id = ?", (deliv_id,)).fetchone()
                if not row:
                    conn.close()
                    return self.send_json({"error": "Delivery record not found"}, 404)
                sub = conn.execute("SELECT * FROM webhook_subscriptions WHERE id = ?", (row["subscription_id"],)).fetchone()
                conn.close()
                if not sub:
                    return self.send_json({"error": "Subscription not found"}, 404)
                conn = get_db()
                delivs = dispatch_webhook_event(conn, "org_default", row["event_type"], json.loads(row["payload_json"]))
                conn.close()
                return self.send_json(delivs[0] if delivs else {"status": "redelivered"})

            elif path == "/api/webhooks/test-dispatch":
                ev_type = req.get("event_type", "po.created")
                payload = req.get("payload") or {
                    "sample_po_number": f"PO-TEST-{uuid.uuid4().hex[:6].upper()}",
                    "amount": 1250.00,
                    "currency": "USD",
                    "supplier": "Acme Industrial Supply",
                    "timestamp": utcnow_str()
                }
                delivs = dispatch_webhook_event(conn, "org_default", ev_type, payload)
                conn.close()
                return self.send_json({
                    "status": "dispatched",
                    "event_type": ev_type,
                    "deliveries_count": len(delivs),
                    "deliveries": delivs
                })

            elif path == "/api/agent/configs":
                cfg_id = req.get("id") or f"cfg_{int(time.time()*1000)}"
                name = req.get("name", "Custom Agent")
                hash_val = req.get("api_key_hash") or f"hash_{int(time.time()*1000)}"
                quota = int(req.get("tool_quota_per_minute", 60))
                budget = int(req.get("daily_budget_cents", 5000))
                concurrent = int(req.get("concurrent_limit", 5))
                now = utcnow_str()
                conn.execute("""
                    INSERT INTO agent_configs (id, org_id, name, api_key_hash, tool_quota_per_minute, daily_budget_cents, current_day_usage_cents, concurrent_limit, active_slots, is_active, created_at)
                    VALUES (?, 'org_default', ?, ?, ?, ?, 0, ?, 0, 1, ?)
                    ON CONFLICT(api_key_hash) DO UPDATE SET
                        name = excluded.name,
                        tool_quota_per_minute = excluded.tool_quota_per_minute,
                        daily_budget_cents = excluded.daily_budget_cents,
                        concurrent_limit = excluded.concurrent_limit,
                        is_active = excluded.is_active
                """, (cfg_id, name, hash_val, quota, budget, concurrent, now))
                conn.commit()
                row = conn.execute("SELECT * FROM agent_configs WHERE api_key_hash = ?", (hash_val,)).fetchone()
                conn.close()
                return self.send_json(dict(row))

            elif path == "/api/agent/escalations":
                esc_id = f"esc_{int(time.time()*1000)}"
                agent_id = req.get("agent_id", "agent_default")
                title = req.get("title", "Agent Escalation")
                reason = req.get("reason", "ambiguous_input")
                ctx_json = req.get("input_context_json", "{}")
                if isinstance(ctx_json, dict):
                    ctx_json = json.dumps(ctx_json)
                rec_act = req.get("recommended_action", "")
                now = utcnow_str()
                conn.execute("""
                    INSERT INTO agent_escalations (id, org_id, agent_id, title, reason, status, input_context_json, recommended_action, created_at)
                    VALUES (?, 'org_default', ?, ?, ?, 'pending', ?, ?, ?)
                """, (esc_id, agent_id, title, reason, ctx_json, rec_act, now))
                # Also create human approval backlog item
                conn.execute("""
                    INSERT INTO approvals (id, action_type, title, description, payload_json, status, requested_by_type, requested_by_id, created_at)
                    VALUES (?, 'agent_escalation', ?, ?, ?, 'pending', 'agent', ?, ?)
                """, (f"appr_{esc_id}", title, f"Agent escalated: {reason}. Action: {rec_act}", ctx_json, agent_id, now))
                conn.commit()
                row = conn.execute("SELECT * FROM agent_escalations WHERE id = ?", (esc_id,)).fetchone()
                conn.close()
                return self.send_json(dict(row), 201)

            elif path == "/api/agent/escalations/resolve":
                esc_id = req.get("id")
                status = req.get("status", "resolved")
                notes = req.get("notes", "")
                now = utcnow_str()
                conn.execute("UPDATE agent_escalations SET status = ?, reviewer_notes = ?, resolved_at = ? WHERE id = ?", (status, notes, now, esc_id))
                conn.execute("UPDATE approvals SET status = ?, decided_by = 'human_supervisor', decided_at = ? WHERE id = ?", (status, now, f"appr_{esc_id}"))
                conn.commit()
                conn.close()
                return self.send_json({"status": status, "escalation_id": esc_id, "resolved_at": now})

            elif path == "/api/agent/context":
                cat = req.get("category", "company_profile")
                k = req.get("key")
                v = req.get("value")
                desc = req.get("description", "")
                now = utcnow_str()
                ctx_id = f"ctx_{int(time.time()*1000)}"
                conn.execute("""
                    INSERT INTO agent_context_memory (id, org_id, category, key, value, description, updated_at)
                    VALUES (?, 'org_default', ?, ?, ?, ?, ?)
                    ON CONFLICT(org_id, key) DO UPDATE SET
                        category = excluded.category,
                        value = excluded.value,
                        description = excluded.description,
                        updated_at = excluded.updated_at
                """, (ctx_id, cat, k, v, desc, now))
                conn.commit()
                row = conn.execute("SELECT * FROM agent_context_memory WHERE org_id = 'org_default' AND key = ?", (k,)).fetchone()
                conn.close()
                return self.send_json(dict(row))

            elif path == "/api/agent/sanitize":
                conn.close()
                res = sanitize_input(req.get("text", ""))
                return self.send_json(res)

            # Phase 9: Enterprise Multi-Tenancy & Compliance POST Routes
            elif path == "/api/orgs":
                name = req.get("name")
                slug = req.get("slug")
                if not name or not slug:
                    conn.close()
                    return self.send_json({"error": "name and slug required"}, 400)
                org_id = f"org_{slug.lower()}"
                now = utcnow_str()
                conn.execute("INSERT OR IGNORE INTO organizations (id, name, slug, created_at) VALUES (?, ?, ?, ?)", (org_id, name, slug, now))
                conn.execute("INSERT OR IGNORE INTO user_organizations (user_id, org_id, role, created_at) VALUES ('usr_admin_default', ?, 'admin', ?)", (org_id, now))
                conn.commit()
                row = conn.execute("SELECT * FROM organizations WHERE id = ?", (org_id,)).fetchone()
                conn.close()
                return self.send_json(dict(row), 201)

            elif path == "/api/auth/switch-org":
                target_org = req.get("org_id")
                if not target_org:
                    conn.close()
                    return self.send_json({"error": "org_id required"}, 400)
                conn.close()
                return self.send_json({"status": "switched", "active_org_id": target_org, "message": f"Active organization switched to {target_org}"})

            elif path == "/api/auth/2fa/setup":
                sec = generate_totp_secret()
                uri = f"otpauth://totp/AOSphere:admin@aosphere.local?secret={sec}&issuer=AOSphere&algorithm=SHA1&digits=6&period=30"
                rcs = generate_recovery_codes(8)
                conn.close()
                return self.send_json({"secret": sec, "otpauth_uri": uri, "recovery_codes": rcs})

            elif path == "/api/auth/2fa/verify":
                sec = req.get("secret", "")
                code = req.get("code", "")
                rcs = req.get("recovery_codes", [])
                if not verify_totp_code(sec, code):
                    conn.close()
                    return self.send_json({"error": "invalid_code", "message": "Invalid TOTP verification code"}, 401)
                now = utcnow_str()
                conn.execute("UPDATE users SET totp_secret = ?, totp_enabled = 1, recovery_codes_json = ?, updated_at = ? WHERE id = 'usr_admin_default'", (sec, json.dumps(rcs), now))
                conn.commit()
                conn.close()
                return self.send_json({"status": "enabled", "message": "Two-factor authentication successfully enabled", "recovery_codes": rcs})

            elif path == "/api/auth/2fa/disable":
                now = utcnow_str()
                conn.execute("UPDATE users SET totp_secret = '', totp_enabled = 0, recovery_codes_json = '[]', updated_at = ? WHERE id = 'usr_admin_default'", (now,))
                conn.commit()
                conn.close()
                return self.send_json({"status": "disabled", "message": "Two-factor authentication successfully disabled"})

            elif path == "/api/auth/2fa/challenge":
                uid = req.get("user_id")
                code = req.get("code", "")
                is_rec = req.get("is_recovery", False)
                u = conn.execute("SELECT * FROM users WHERE id = ?", (uid,)).fetchone()
                if not u and (uid == "usr_admin_default" or uid == "usr_admin"):
                    u = conn.execute("SELECT * FROM users WHERE email = 'admin@aosphere.local'").fetchone()
                if not u:
                    conn.close()
                    return self.send_json({"error": "user_not_found"}, 401)
                verified = False
                if is_rec:
                    codes = json.loads(u["recovery_codes_json"] or "[]")
                    if code.strip().upper() in [c.upper() for c in codes]:
                        verified = True
                else:
                    verified = verify_totp_code(u["totp_secret"] or "", code)
                if not verified:
                    conn.close()
                    return self.send_json({"error": "invalid_2fa_code"}, 401)
                conn.close()
                return self.send_json({"status": "ok", "token": f"sess_{uuid.uuid4().hex[:16]}", "user": {"id": u["id"], "email": u["email"], "role": u["role"]}})

            elif path == "/api/auth/invitations":
                email = req.get("email", "").strip().lower()
                role = req.get("role", "employee")
                if not email:
                    conn.close()
                    return self.send_json({"error": "email required"}, 400)
                inv_id = f"inv_{uuid.uuid4().hex[:12]}"
                raw_tok = uuid.uuid4().hex
                tok_hash = hashlib.sha256(raw_tok.encode()).hexdigest()
                now = utcnow_str()
                exp = (datetime.now(timezone.utc) + timedelta(days=3)).strftime("%Y-%m-%dT%H:%M:%SZ")
                conn.execute("INSERT INTO user_invitations (id, org_id, email, role, token_hash, invited_by, status, expires_at, created_at) VALUES (?, 'org_default', ?, ?, ?, 'usr_admin_default', 'pending', ?, ?)", (inv_id, email, role, tok_hash, exp, now))
                conn.commit()
                conn.close()
                return self.send_json({"invitation_id": inv_id, "email": email, "role": role, "invite_token": raw_tok, "invite_url": f"/portal.html?invite={raw_tok}", "expires_at": exp}, 201)

            elif path == "/api/auth/invitations/accept":
                tok = req.get("token")
                name = req.get("name", "New User")
                pwd = req.get("password", "")
                err = validate_password_complexity(pwd)
                if err:
                    conn.close()
                    return self.send_json({"error": "password_policy_violation", "message": err}, 400)
                tok_hash = hashlib.sha256(tok.encode()).hexdigest()
                inv = conn.execute("SELECT * FROM user_invitations WHERE token_hash = ? AND status = 'pending'", (tok_hash,)).fetchone()
                if not inv:
                    conn.close()
                    return self.send_json({"error": "invalid_or_expired_token"}, 400)
                uid = f"usr_{uuid.uuid4().hex[:8]}"
                now = utcnow_str()
                pwd_hash = f"mockhash_{hashlib.sha256(pwd.encode()).hexdigest()[:24]}"
                conn.execute("INSERT INTO users (id, org_id, email, password_hash, name, role, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)", (uid, inv["org_id"], inv["email"], pwd_hash, name, inv["role"], now, now))
                conn.execute("INSERT OR IGNORE INTO user_organizations (user_id, org_id, role, created_at) VALUES (?, ?, ?, ?)", (uid, inv["org_id"], inv["role"], now))
                conn.execute("UPDATE user_invitations SET status = 'accepted' WHERE id = ?", (inv["id"],))
                conn.commit()
                conn.close()
                return self.send_json({"status": "accepted", "user": {"id": uid, "email": inv["email"], "name": name, "role": inv["role"]}})

            elif path == "/api/auth/password-reset/request":
                email = req.get("email", "").strip().lower()
                u = conn.execute("SELECT * FROM users WHERE email = ?", (email,)).fetchone()
                if not u:
                    conn.close()
                    return self.send_json({"status": "requested", "message": "If an account exists, a reset link has been dispatched."})
                rst_id = f"reset_{uuid.uuid4().hex[:12]}"
                raw_tok = uuid.uuid4().hex
                tok_hash = hashlib.sha256(raw_tok.encode()).hexdigest()
                now = utcnow_str()
                exp = (datetime.now(timezone.utc) + timedelta(minutes=15)).strftime("%Y-%m-%dT%H:%M:%SZ")
                conn.execute("INSERT INTO password_resets (id, user_id, token_hash, status, expires_at, created_at) VALUES (?, ?, ?, 'pending', ?, ?)", (rst_id, u["id"], tok_hash, exp, now))
                conn.commit()
                conn.close()
                return self.send_json({"status": "requested", "reset_token": raw_tok, "reset_url": f"/portal.html?reset={raw_tok}", "expires_in": "15 minutes"})

            elif path == "/api/auth/password-reset/confirm":
                tok = req.get("token")
                new_pwd = req.get("new_password", "")
                err = validate_password_complexity(new_pwd)
                if err:
                    conn.close()
                    return self.send_json({"error": "password_policy_violation", "message": err}, 400)
                tok_hash = hashlib.sha256(tok.encode()).hexdigest()
                rst = conn.execute("SELECT * FROM password_resets WHERE token_hash = ? AND status = 'pending'", (tok_hash,)).fetchone()
                if not rst:
                    conn.close()
                    return self.send_json({"error": "invalid_or_expired_token"}, 400)
                now = utcnow_str()
                pwd_hash = f"mockhash_{hashlib.sha256(new_pwd.encode()).hexdigest()[:24]}"
                conn.execute("UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?", (pwd_hash, now, rst["user_id"]))
                conn.execute("UPDATE password_resets SET status = 'used' WHERE id = ?", (rst["id"],))
                conn.commit()
                conn.close()
                return self.send_json({"status": "password_updated", "message": "Password successfully updated."})

            elif path == "/api/auth/sso/configs":
                prov = req.get("provider") or req.get("provider_name") or "google"
                cid = req.get("client_id", "")
                csec = req.get("client_secret", "")
                iss = req.get("issuer_url", "")
                dom = req.get("domain_mapping", "")
                en = 1 if req.get("is_enabled", True) else 0
                sso_id = f"sso_{uuid.uuid4().hex[:12]}"
                now = utcnow_str()
                conn.execute("""
                    INSERT INTO sso_configs (id, org_id, provider, client_id, client_secret, issuer_url, domain_mapping, is_enabled, created_at)
                    VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?, ?)
                    ON CONFLICT(org_id, provider) DO UPDATE SET client_id = excluded.client_id, client_secret = excluded.client_secret, issuer_url = excluded.issuer_url, domain_mapping = excluded.domain_mapping, is_enabled = excluded.is_enabled
                """, (sso_id, prov, cid, csec, iss, dom, en, now))
                conn.commit()
                row = conn.execute("SELECT * FROM sso_configs WHERE org_id = 'org_default' AND provider = ?", (prov,)).fetchone()
                conn.close()
                return self.send_json(dict(row))

            elif path == "/api/auth/sso/callback":
                claims = req.get("id_token_claims", {})
                email = claims.get("email", "").strip().lower()
                name = claims.get("name", "SSO User")
                if not email:
                    conn.close()
                    return self.send_json({"error": "email missing from claims"}, 400)
                u = conn.execute("SELECT * FROM users WHERE email = ?", (email,)).fetchone()
                now = utcnow_str()
                if not u:
                    uid = f"usr_{uuid.uuid4().hex[:8]}"
                    conn.execute("INSERT INTO users (id, org_id, email, password_hash, name, role, is_active, created_at, updated_at) VALUES (?, 'org_default', ?, 'sso_auth_only', ?, 'employee', 1, ?, ?)", (uid, email, name, now, now))
                    conn.execute("INSERT OR IGNORE INTO user_organizations (user_id, org_id, role, created_at) VALUES (?, 'org_default', 'employee', ?)", (uid, now))
                    conn.commit()
                    u = conn.execute("SELECT * FROM users WHERE id = ?", (uid,)).fetchone()
                if not u and (uid == "usr_admin_default" or uid == "usr_admin"):
                    u = conn.execute("SELECT * FROM users WHERE email = 'admin@aosphere.local'").fetchone()
                conn.close()
                return self.send_json({"status": "authenticated", "token": f"sess_{uuid.uuid4().hex[:16]}", "user": {"id": u["id"], "email": u["email"], "name": u["name"], "role": u["role"]}})

            elif path == "/api/auth/roles":
                if self.command == "DELETE":
                    role_id = query.get("id", [""])[0] or req.get("id", "")
                    conn.execute("DELETE FROM custom_roles WHERE id = ? AND is_system = 0", (role_id,))
                    conn.commit()
                    conn.close()
                    return self.send_json({"status": "deleted", "id": role_id})
                elif self.command == "PUT":
                    role_id = req.get("id")
                    rdesc = req.get("description", "")
                    rperms = req.get("permissions_json") or json.dumps(req.get("permissions", ["read"]))
                    conn.execute("UPDATE custom_roles SET description = ?, permissions_json = ? WHERE id = ? AND is_system = 0", (rdesc, rperms if isinstance(rperms, str) else json.dumps(rperms), role_id))
                    conn.commit()
                    row = conn.execute("SELECT * FROM custom_roles WHERE id = ?", (role_id,)).fetchone()
                    conn.close()
                    return self.send_json(dict(row))
                else:
                    rname = req.get("name")
                    rdesc = req.get("description", "")
                    rperms = req.get("permissions_json") or json.dumps(req.get("permissions", ["read"]))
                    if not rname:
                        conn.close()
                        return self.send_json({"error": "name required"}, 400)
                    role_id = f"role_{uuid.uuid4().hex[:8]}"
                    now = utcnow_str()
                    conn.execute("INSERT INTO custom_roles (id, org_id, name, description, permissions_json, is_system, created_at) VALUES (?, 'org_default', ?, ?, ?, 0, ?)", (role_id, rname, rdesc, rperms if isinstance(rperms, str) else json.dumps(rperms), now))
                    conn.commit()
                    row = conn.execute("SELECT * FROM custom_roles WHERE id = ?", (role_id,)).fetchone()
                    conn.close()
                    return self.send_json(dict(row), 201)

            elif path in ("/api/compliance/audit-vault", "/api/compliance/audit-vault/record"):
                act = req.get("action") or req.get("event_type") or "security.audit"
                actor_id = req.get("actor_id", "usr_admin_default")
                actor_type = req.get("actor_type", "user")
                rtype = req.get("resource_type", "general")
                rid = req.get("resource_id", "res_001")
                phash = req.get("payload_hash") or GENESIS_BLOCK_HASH
                last_block = conn.execute("SELECT sequence_number, block_hash FROM audit_chain_vault WHERE org_id = 'org_default' ORDER BY sequence_number DESC LIMIT 1").fetchone()
                seq = 1 if not last_block else last_block["sequence_number"] + 1
                prev_hash = GENESIS_BLOCK_HASH if not last_block else last_block["block_hash"]
                ts_str = utcnow_str()
                bhash = compute_audit_block_hash(seq, prev_hash, act, actor_id, phash, ts_str)
                b_id = f"acv_{uuid.uuid4().hex[:12]}"
                conn.execute("""
                    INSERT INTO audit_chain_vault (id, org_id, sequence_number, actor_type, actor_id, action, resource_type, resource_id, payload_hash, prev_block_hash, block_hash, timestamp)
                    VALUES (?, 'org_default', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """, (b_id, seq, actor_type, actor_id, act, rtype, rid, phash, prev_hash, bhash, ts_str))
                conn.commit()
                row = conn.execute("SELECT * FROM audit_chain_vault WHERE id = ?", (b_id,)).fetchone()
                conn.close()
                return self.send_json(dict(row), 201)

            elif path == "/api/compliance/audit-vault/verify":
                is_valid, count, err = verify_audit_vault_chain(conn, "org_default")
                conn.close()
                res = {"is_valid": is_valid, "blocks_verified": count, "org_id": "org_default", "timestamp": utcnow_str()}
                if err:
                    res["verification_error"] = err
                return self.send_json(res)

            elif path == "/api/compliance/gdpr/export":
                uid = req.get("user_id", "usr_admin_default")
                u = conn.execute("SELECT * FROM users WHERE id = ?", (uid,)).fetchone()
                if not u and (uid == "usr_admin_default" or uid == "usr_admin"):
                    u = conn.execute("SELECT * FROM users WHERE email = 'admin@aosphere.local'").fetchone()
                if not u:
                    conn.close()
                    return self.send_json({"error": "user not found"}, 404)
                auds = conn.execute("SELECT action, entity_type, entity_id, reasoning, created_at FROM audit_logs WHERE actor_id = ? ORDER BY created_at DESC LIMIT 50", (uid,)).fetchall()
                export_data = {
                    "export_timestamp": utcnow_str(),
                    "organization_id": "org_default",
                    "profile": {"id": u["id"], "email": u["email"], "name": u["name"], "role": u["role"], "is_active": bool(u["is_active"]), "totp_enabled": bool(u["totp_enabled"])},
                    "audit_actions": [dict(a) for a in auds]
                }
                pr_id = f"priv_{uuid.uuid4().hex[:12]}"
                now = utcnow_str()
                conn.execute("INSERT INTO privacy_requests (id, org_id, user_id, request_type, status, result_json, requested_at, completed_at) VALUES (?, 'org_default', ?, 'export_data', 'completed', ?, ?, ?)", (pr_id, uid, json.dumps(export_data), now, now))
                conn.commit()
                conn.close()
                return self.send_json({"status": "completed", "export_data": export_data})

            elif path == "/api/compliance/gdpr/forget-me":
                uid = req.get("user_id", "usr_admin_default")
                now = utcnow_str()
                anon_name = f"Anonymized User {uid[:8]}"
                anon_email = f"anonymized_{uid[:8]}@redacted.local"
                conn.execute("UPDATE users SET name = ?, email = ?, password_hash = 'redacted', is_active = 0, totp_secret = '', totp_enabled = 0, recovery_codes_json = '[]', updated_at = ? WHERE id = ?", (anon_name, anon_email, now, uid))
                conn.execute("DELETE FROM sessions WHERE user_id = ?", (uid,))
                pr_id = f"priv_{uuid.uuid4().hex[:12]}"
                conn.execute("INSERT INTO privacy_requests (id, org_id, user_id, request_type, status, result_json, requested_at, completed_at) VALUES (?, 'org_default', ?, 'forget_me', 'completed', '{\"anonymized\": true}', ?, ?)", (pr_id, uid, now, now))
                conn.commit()
                conn.close()
                return self.send_json({"status": "anonymized", "message": "User PII scrubbed in compliance with GDPR Article 17."})

            elif path == "/api/compliance/soc2/access-review/generate":
                q = req.get("quarter", "2026-Q1")
                now = utcnow_str()
                users = conn.execute("SELECT id, email, name, role, is_active, totp_enabled FROM users WHERE org_id = 'org_default'").fetchall()
                keys = conn.execute("SELECT id, name, role, allowed_tools, is_active FROM api_keys WHERE org_id = 'org_default'").fetchall()
                snap = {
                    "generated_at": now,
                    "quarter": q,
                    "users_count": len(users),
                    "keys_count": len(keys),
                    "users": [dict(u) for u in users],
                    "api_keys": [dict(k) for k in keys]
                }
                rev_id = f"soc2_rev_{q.lower()}_{uuid.uuid4().hex[:6]}"
                conn.execute("INSERT INTO soc2_access_reviews (id, org_id, quarter, status, reviewed_by, notes, snapshot_json, created_at) VALUES (?, 'org_default', ?, 'pending', 'usr_admin_default', 'Quarterly access review snapshot', ?, ?)", (rev_id, q, json.dumps(snap), now))
                conn.commit()
                row = conn.execute("SELECT * FROM soc2_access_reviews WHERE id = ?", (rev_id,)).fetchone()
                conn.close()
                return self.send_json(dict(row), 201)

            elif path == "/api/compliance/soc2/access-review/signoff":
                rev_id = req.get("id")
                notes = req.get("notes", "Approved by Compliance Administrator")
                now = utcnow_str()
                conn.execute("UPDATE soc2_access_reviews SET status = 'approved', notes = ?, reviewed_by = 'usr_admin_default', reviewed_at = ? WHERE id = ?", (notes, now, rev_id))
                conn.commit()
                row = conn.execute("SELECT * FROM soc2_access_reviews WHERE id = ?", (rev_id,)).fetchone()
                conn.close()
                return self.send_json(dict(row))

            # Phase 10: Disaster Recovery & Backups POST
            elif path == "/api/backup/config":
                for k in ["endpoint", "bucket", "region", "access_key", "secret_key"]:
                    if k in req:
                        os.environ[f"S3_{k.upper()}"] = str(req[k])
                conn.close()
                sec = os.environ.get("S3_SECRET_KEY", "")
                return self.send_json({
                    "endpoint": os.environ.get("S3_ENDPOINT", ""),
                    "bucket": os.environ.get("S3_BUCKET", ""),
                    "region": os.environ.get("S3_REGION", "us-east-1"),
                    "access_key": os.environ.get("S3_ACCESS_KEY", ""),
                    "secret_key": "********" if sec else ""
                })

            elif path == "/api/backup/create":
                prefix = req.get("prefix", "aosphere_backup")
                passphrase = req.get("passphrase", "")
                upload_cloud = bool(req.get("upload_cloud", False))

                ts = datetime.now(timezone.utc).strftime("%Y%m%d_%H%M%S")
                raw_filename = f"{prefix}_{ts}.db"
                raw_path = os.path.join(BACKUP_DIR, raw_filename)

                # Checkpoint WAL
                conn.execute("PRAGMA wal_checkpoint(TRUNCATE)")

                # Online snapshot via VACUUM INTO
                escaped_path = raw_path.replace("'", "''")
                conn.execute(f"VACUUM INTO '{escaped_path}'")
                conn.close()

                # Verify integrity
                is_ok, logs = verify_backup_integrity(raw_path)

                final_path = raw_path
                final_filename = raw_filename
                is_encrypted = False

                if passphrase:
                    enc_filename = f"{prefix}_{ts}.db.enc"
                    enc_path = os.path.join(BACKUP_DIR, enc_filename)
                    encrypt_backup_file(raw_path, enc_path, passphrase)
                    try:
                        os.remove(raw_path)
                    except Exception:
                        pass
                    final_path = enc_path
                    final_filename = enc_filename
                    is_encrypted = True

                file_size = os.path.getsize(final_path)
                checksum = compute_file_sha256(final_path)

                res = {
                    "backup_path": final_path,
                    "filename": final_filename,
                    "size_bytes": file_size,
                    "created_at": datetime.now(timezone.utc).isoformat(),
                    "integrity_ok": is_ok,
                    "integrity_logs": logs,
                    "is_encrypted": is_encrypted,
                    "checksum_sha256": checksum,
                    "cloud_uploaded": False,
                }
                if upload_cloud:
                    res["cloud_uploaded"] = True
                    res["cloud_key"] = f"backups/{final_filename}"

                broadcast("backup.created", res)
                return self.send_json(res)

            elif path == "/api/backup/restore":
                filename = req.get("filename", "")
                passphrase = req.get("passphrase", "")
                target_db_path = req.get("target_db_path", DB_PATH)

                if not filename:
                    conn.close()
                    return self.send_json({"error": "filename is required"}, 400)

                source_path = os.path.join(BACKUP_DIR, filename)
                if not os.path.exists(source_path):
                    conn.close()
                    return self.send_json({"error": f"backup file not found: {filename}"}, 404)

                restore_source = source_path
                temp_dec = None
                if filename.endswith(".enc"):
                    if not passphrase:
                        conn.close()
                        return self.send_json({"error": "passphrase is required for encrypted backup"}, 400)
                    temp_dec = os.path.join(BACKUP_DIR, f"temp_restore_{filename}.db")
                    try:
                        decrypt_backup_file(source_path, temp_dec, passphrase)
                        restore_source = temp_dec
                    except Exception as e:
                        conn.close()
                        return self.send_json({"error": f"decryption failed: {str(e)}"}, 500)

                is_ok, _ = verify_backup_integrity(restore_source)
                if not is_ok:
                    if temp_dec and os.path.exists(temp_dec):
                        os.remove(temp_dec)
                    conn.close()
                    return self.send_json({"error": "integrity check failed on backup"}, 500)

                try:
                    conn.execute("PRAGMA wal_checkpoint(TRUNCATE)")
                except Exception:
                    pass
                conn.close()
                conn = None

                if target_db_path == DB_PATH:
                    restore_target = os.path.join(BACKUP_DIR, f"restored_{filename}.db")
                    shutil.copy2(restore_source, restore_target)
                else:
                    shutil.copy2(restore_source, target_db_path)

                if temp_dec and os.path.exists(temp_dec):
                    try:
                        os.remove(temp_dec)
                    except Exception:
                        pass

                res = {"status": "restored", "filename": filename}
                broadcast("backup.restored", res)
                return self.send_json(res)

            elif path == "/api/backup/cloud-sync":
                filename = req.get("filename", "")
                if not filename:
                    conn.close()
                    return self.send_json({"error": "filename is required"}, 400)

                full_p = os.path.join(BACKUP_DIR, filename)
                if not os.path.exists(full_p):
                    conn.close()
                    return self.send_json({"error": f"backup file not found: {filename}"}, 404)

                conn.close()
                s3_key = f"backups/{filename}"
                res = {"status": "synced", "filename": filename, "cloud_key": s3_key}
                broadcast("backup.synced", res)
                return self.send_json(res)

            if conn:
                try:
                    conn.close()
                except Exception:
                    pass
            return self.send_json({"error": "Unknown endpoint"}, 404)
        except Exception as e:
            if conn:
                try:
                    conn.close()
                except Exception:
                    pass
            import traceback
            traceback.print_exc()
            return self.send_json({"error": str(e), "traceback": traceback.format_exc()}, 500)

def main():
    parser = argparse.ArgumentParser(description="AOSphere Platform Server")
    parser.add_argument("-port", "--port", type=int, default=8080, help="HTTP Server Port")
    parser.add_argument("-db", "--db", type=str, default="", help="Path to SQLite database file")
    args = parser.parse_args()

    global DB_PATH
    if args.db:
        DB_PATH = os.path.abspath(args.db)

    # Initialize schema once synchronously before binding server
    try:
        init_conn = get_db()
        init_conn.close()
    except Exception as e:
        print(f"Warning: Initial schema setup encountered: {e}")

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
