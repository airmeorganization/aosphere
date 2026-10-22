package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/db"
	"aosphere/internal/finance"
	"aosphere/internal/ledger"
	"github.com/google/uuid"
)

var (
	ErrPONotReceived    = errors.New("purchase order must be in 'received' status before payment can be recorded")
	ErrDealNotFound     = errors.New("deal not found")
	ErrTimesheetNotFound = errors.New("timesheet not found")
)

type CompanyProfile struct {
	OrgID               string    `json:"org_id"`
	CompanyName         string    `json:"company_name"`
	Currency            string    `json:"currency"`
	Timezone            string    `json:"timezone"`
	TaxRate             float64   `json:"tax_rate"`
	Address             string    `json:"address"`
	Phone               string    `json:"phone"`
	Website             string    `json:"website"`
	OnboardingCompleted bool      `json:"onboarding_completed"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type Quote struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	QuoteNumber string    `json:"quote_number"`
	DealID      string    `json:"deal_id"`
	CustomerID  string    `json:"customer_id"`
	Subtotal    float64   `json:"subtotal"`
	TaxAmount   float64   `json:"tax_amount"`
	TotalAmount float64   `json:"total_amount"`
	Status      string    `json:"status"`
	ItemsJSON   string    `json:"items_json"`
	CreatedAt   time.Time `json:"created_at"`
}

type NotificationItem struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Category  string    `json:"category"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

type SearchResults struct {
	Query     string                   `json:"query"`
	SKUs      []map[string]interface{} `json:"skus"`
	Deals     []map[string]interface{} `json:"deals"`
	Invoices  []map[string]interface{} `json:"invoices"`
	Customers []map[string]interface{} `json:"customers"`
	Employees []map[string]interface{} `json:"employees"`
	Tasks     []map[string]interface{} `json:"tasks"`
	Documents []map[string]interface{} `json:"documents"`
}

type PayrollRunResult struct {
	Period            string  `json:"period"`
	TotalHours        float64 `json:"total_hours"`
	TotalGrossDollars float64 `json:"total_gross_dollars"`
	TimesheetsPaid    int     `json:"timesheets_paid"`
	JournalEntryID    string  `json:"journal_entry_id"`
}

// ----------------- ERP: PO PAYMENT LIFECYCLE -----------------

// PayPurchaseOrder records payment of goods received for a PO and posts a balancing ledger entry:
// Debit: Accounts Payable (2000)
// Credit: Cash / Bank (1000)
func (s *WorkflowService) PayPurchaseOrder(ctx context.Context, orgID, poID string, authCtx *auth.AuthContext, idempKey string) (*db.PurchaseOrder, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Fetch current PO state with row lock
	var po db.PurchaseOrder
	err = tx.QueryRowContext(ctx, `SELECT id, org_id, po_number, supplier_id, status, total_amount, items_json, version
		FROM purchase_orders WHERE id = ? AND org_id = ?`, poID, orgID).
		Scan(&po.ID, &po.OrgID, &po.PONumber, &po.SupplierID, &po.Status, &po.TotalAmount, &po.ItemsJSON, &po.Version)
	if err != nil {
		return nil, fmt.Errorf("purchase order not found: %w", err)
	}

	if po.Status != POStatusReceived {
		return nil, fmt.Errorf("%w: current status is %q", ErrPONotReceived, po.Status)
	}

	actor := "system"
	if authCtx != nil {
		actor = authCtx.UserID
	}

	// 2. Transition status to 'paid'
	_, err = tx.ExecContext(ctx, `UPDATE purchase_orders SET status = 'paid', version = version + 1 WHERE id = ?`, po.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update PO status: %w", err)
	}
	po.Status = "paid"
	po.Version++

	// 3. Post double-entry General Ledger entry
	totalCents := finance.DollarsToCents(po.TotalAmount)
	lines := []ledger.JournalLine{
		{
			AccountCode: ledger.AccountAccountsPayable,
			EntryType:   "debit",
			AmountCents: totalCents,
			Description: fmt.Sprintf("Payment to supplier for PO %s", po.PONumber),
		},
		{
			AccountCode: ledger.AccountCash,
			EntryType:   "credit",
			AmountCents: totalCents,
			Description: fmt.Sprintf("Cash disbursement for PO %s", po.PONumber),
		},
	}

	_, err = s.ledger.PostJournalEntry(ctx, tx, ledger.JournalEntry{
		OrgID:      orgID,
		SourceType: "purchase_order_payment",
		SourceID:   po.ID,
		Memo:       fmt.Sprintf("Disbursement for PO %s (%s)", po.PONumber, finance.FormatUSD(totalCents)),
		CreatedBy:  actor,
		Lines:      lines,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to post payment journal entry: %w", err)
	}

	// 4. Record transactional outbox event
	_ = RecordOutboxEvent(ctx, tx, orgID, "po_paid", map[string]interface{}{
		"po_id":     po.ID,
		"po_number": po.PONumber,
		"total":     po.TotalAmount,
	})

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit PO payment: %w", err)
	}

	_ = s.store.LogAuditForOrg(orgID, "human", actor, "PAY_PURCHASE_ORDER", "purchase_orders", po.ID,
		fmt.Sprintf("Paid PO %s for %s", po.PONumber, finance.FormatUSD(totalCents)), "{}")

	return &po, nil
}

// ----------------- CRM: QUOTE GENERATION & CONVERSION -----------------

// GenerateQuote creates a formal customer quote linked to a sales deal
func (s *WorkflowService) GenerateQuote(ctx context.Context, orgID, dealID string, items []InvoiceItem, authCtx *auth.AuthContext) (*Quote, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	var customerID, dealTitle string
	err := s.store.DB().QueryRowContext(ctx, `SELECT customer_id, title FROM deals WHERE id = ? AND org_id = ?`, dealID, orgID).
		Scan(&customerID, &dealTitle)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDealNotFound, err)
	}

	// Calculate subtotal, tax, total
	var subtotalCents int64
	for _, it := range items {
		lineCents := finance.DollarsToCents(it.Price) * int64(it.Qty)
		subtotalCents += lineCents
	}
	taxCents := finance.CalculateTax(subtotalCents, 1000)
	totalCents := subtotalCents + taxCents

	itemsBytes, _ := json.Marshal(items)
	quoteNum := fmt.Sprintf("QTE-%d-%04d", time.Now().Year(), time.Now().UnixNano()%10000)
	id := fmt.Sprintf("qte_%s", uuid.New().String()[:8])

	query := `INSERT INTO quotes (id, org_id, quote_number, deal_id, customer_id, subtotal, tax_amount, total_amount, status, items_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'sent', ?)`

	_, err = s.store.DB().ExecContext(ctx, query, id, orgID, quoteNum, dealID, customerID,
		finance.CentsToDollars(subtotalCents), finance.CentsToDollars(taxCents), finance.CentsToDollars(totalCents), string(itemsBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to save quote: %w", err)
	}

	return &Quote{
		ID:          id,
		OrgID:       orgID,
		QuoteNumber: quoteNum,
		DealID:      dealID,
		CustomerID:  customerID,
		Subtotal:    finance.CentsToDollars(subtotalCents),
		TaxAmount:   finance.CentsToDollars(taxCents),
		TotalAmount: finance.CentsToDollars(totalCents),
		Status:      "sent",
		ItemsJSON:   string(itemsBytes),
		CreatedAt:   time.Now(),
	}, nil
}

// ConvertDealToInvoice converts an accepted deal into a compliant customer invoice
func (s *WorkflowService) ConvertDealToInvoice(ctx context.Context, orgID, dealID string, authCtx *auth.AuthContext, idempKey string) (*db.Invoice, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	var customerID, dealTitle string
	var dealValue float64
	err := s.store.DB().QueryRowContext(ctx, `SELECT customer_id, title, value FROM deals WHERE id = ? AND org_id = ?`, dealID, orgID).
		Scan(&customerID, &dealTitle, &dealValue)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDealNotFound, err)
	}

	// Create invoice with line item
	invReq := CreateInvoiceRequest{
		CustomerID: customerID,
		DueDate:    time.Now().AddDate(0, 0, 30).Format("2006-01-02"),
		Recurring:  "none",
		Items: []InvoiceItem{
			{
				Desc:  fmt.Sprintf("Contract deliverables: %s", dealTitle),
				Qty:   1,
				Price: dealValue,
			},
		},
	}

	inv, err := s.CreateInvoice(ctx, orgID, invReq, authCtx, idempKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate invoice from deal: %w", err)
	}

	// Advance deal to 'won'
	actor := "system"
	if authCtx != nil {
		actor = authCtx.UserID
	}
	_ = s.store.UpdateDealStageForOrg(orgID, dealID, "won", "human", actor)

	// Create in-app notification
	_ = s.CreateNotification(ctx, orgID, "Deal Converted to Invoice",
		fmt.Sprintf("Deal %q converted into Invoice %s ($%.2f)", dealTitle, inv.InvoiceNumber, inv.TotalAmount), "finance")

	return inv, nil
}

// ----------------- HRMS: TIMESHEET APPROVAL & PAYROLL LOCK -----------------

// ApproveTimesheet transitions a timesheet to 'approved'
func (s *WorkflowService) ApproveTimesheet(ctx context.Context, orgID, timesheetID string, authCtx *auth.AuthContext) error {
	res, err := s.store.DB().ExecContext(ctx, `UPDATE timesheets SET status = 'approved' WHERE id = ? AND org_id = ? AND status = 'logged'`,
		timesheetID, orgID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("timesheet not found or not in 'logged' state")
	}
	return nil
}

// LockPayroll aggregates all approved timesheets, creates an expense entry, posts to ledger, and marks timesheets 'paid'
func (s *WorkflowService) LockPayroll(ctx context.Context, orgID, period string, authCtx *auth.AuthContext) (*PayrollRunResult, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if period == "" {
		period = time.Now().Format("2006-01")
	}

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Query approved timesheets
	rows, err := tx.QueryContext(ctx, `SELECT id, hours_worked + overtime_hours, hourly_rate FROM timesheets WHERE org_id = ? AND status = 'approved'`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timesheetIDs []string
	var totalHours float64
	var totalGrossCents int64

	for rows.Next() {
		var id string
		var hours, rate float64
		if err := rows.Scan(&id, &hours, &rate); err == nil {
			timesheetIDs = append(timesheetIDs, id)
			totalHours += hours
			totalGrossCents += finance.DollarsToCents(hours * rate)
		}
	}
	rows.Close()

	if len(timesheetIDs) == 0 {
		return nil, fmt.Errorf("no approved timesheets available to lock for period %s", period)
	}

	// 2. Mark timesheets paid
	for _, id := range timesheetIDs {
		_, _ = tx.ExecContext(ctx, `UPDATE timesheets SET status = 'paid' WHERE id = ?`, id)
	}

	// 3. Post double-entry General Ledger entry:
	// Debit: Operating Expense (5100)
	// Credit: Cash / Bank (1000)
	actor := "payroll_admin"
	if authCtx != nil {
		actor = authCtx.UserID
	}
	lines := []ledger.JournalLine{
		{
			AccountCode: ledger.AccountOperatingExpense,
			EntryType:   "debit",
			AmountCents: totalGrossCents,
			Description: fmt.Sprintf("Payroll disbursement for period %s (%d employees/shifts)", period, len(timesheetIDs)),
		},
		{
			AccountCode: ledger.AccountCash,
			EntryType:   "credit",
			AmountCents: totalGrossCents,
			Description: fmt.Sprintf("Cash payroll payout for period %s", period),
		},
	}

	je, err := s.ledger.PostJournalEntry(ctx, tx, ledger.JournalEntry{
		OrgID:      orgID,
		SourceType: "payroll",
		SourceID:   period,
		Memo:       fmt.Sprintf("Payroll Lock for Period %s (%s)", period, finance.FormatUSD(totalGrossCents)),
		CreatedBy:  actor,
		Lines:      lines,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to post payroll ledger entry: %w", err)
	}

	// 4. Record expense record in expenses table
	expID := fmt.Sprintf("exp_pay_%d", time.Now().UnixNano())
	_, _ = tx.ExecContext(ctx, `INSERT INTO expenses (id, org_id, category, amount, vendor, description, date)
		VALUES (?, ?, 'Payroll', ?, 'Internal Payroll', ?, ?)`,
		expID, orgID, finance.CentsToDollars(totalGrossCents),
		fmt.Sprintf("Payroll for period %s", period), time.Now().Format("2006-01-02"))

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &PayrollRunResult{
		Period:            period,
		TotalHours:        totalHours,
		TotalGrossDollars: finance.CentsToDollars(totalGrossCents),
		TimesheetsPaid:    len(timesheetIDs),
		JournalEntryID:    je.ID,
	}, nil
}

// ----------------- DOCUMENT REPOSITORY -----------------

func (s *WorkflowService) ListDocuments(ctx context.Context, orgID string) ([]db.Document, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.store.DB().QueryContext(ctx, `SELECT id, org_id, title, category, content, author, updated_at FROM documents WHERE org_id = ? ORDER BY updated_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Document
	for rows.Next() {
		var d db.Document
		if err := rows.Scan(&d.ID, &d.OrgID, &d.Title, &d.Category, &d.Content, &d.Author, &d.UpdatedAt); err == nil {
			list = append(list, d)
		}
	}
	return list, nil
}

func (s *WorkflowService) CreateDocument(ctx context.Context, orgID, title, category, content, author string) (*db.Document, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	id := fmt.Sprintf("doc_%s", uuid.New().String()[:8])
	now := time.Now()
	_, err := s.store.DB().ExecContext(ctx, `INSERT INTO documents (id, org_id, title, category, content, author, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, id, orgID, title, category, content, author, now)
	if err != nil {
		return nil, err
	}
	return &db.Document{
		ID:        id,
		OrgID:     orgID,
		Title:     title,
		Category:  category,
		Content:   content,
		Author:    author,
		UpdatedAt: now.Format(time.RFC3339),
	}, nil
}

func (s *WorkflowService) DeleteDocument(ctx context.Context, orgID, docID string) error {
	_, err := s.store.DB().ExecContext(ctx, `DELETE FROM documents WHERE id = ? AND org_id = ?`, docID, orgID)
	return err
}

// ----------------- GLOBAL SEARCH & CSV EXPORT -----------------

func (s *WorkflowService) GlobalSearch(ctx context.Context, orgID, query string) (*SearchResults, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	qLike := "%" + strings.TrimSpace(query) + "%"
	res := &SearchResults{
		Query: query,
	}

	// 1. Search SKUs
	skuRows, err := s.store.DB().QueryContext(ctx, `SELECT id, code, name, stock_quantity, selling_price FROM skus WHERE org_id = ? AND (code LIKE ? OR name LIKE ?) LIMIT 5`, orgID, qLike, qLike)
	if err == nil {
		defer skuRows.Close()
		for skuRows.Next() {
			var id, code, name string
			var stock int
			var price float64
			if err := skuRows.Scan(&id, &code, &name, &stock, &price); err == nil {
				res.SKUs = append(res.SKUs, map[string]interface{}{"id": id, "code": code, "name": name, "stock": stock, "price": price})
			}
		}
	}

	// 2. Search Deals
	dealRows, err := s.store.DB().QueryContext(ctx, `SELECT id, title, stage, value FROM deals WHERE org_id = ? AND title LIKE ? LIMIT 5`, orgID, qLike)
	if err == nil {
		defer dealRows.Close()
		for dealRows.Next() {
			var id, title, stage string
			var val float64
			if err := dealRows.Scan(&id, &title, &stage, &val); err == nil {
				res.Deals = append(res.Deals, map[string]interface{}{"id": id, "title": title, "stage": stage, "value": val})
			}
		}
	}

	// 3. Search Invoices
	invRows, err := s.store.DB().QueryContext(ctx, `SELECT id, invoice_number, status, total_amount FROM invoices WHERE org_id = ? AND invoice_number LIKE ? LIMIT 5`, orgID, qLike)
	if err == nil {
		defer invRows.Close()
		for invRows.Next() {
			var id, invNum, status string
			var total float64
			if err := invRows.Scan(&id, &invNum, &status, &total); err == nil {
				res.Invoices = append(res.Invoices, map[string]interface{}{"id": id, "invoice_number": invNum, "status": status, "total": total})
			}
		}
	}

	// 4. Search Customers
	custRows, err := s.store.DB().QueryContext(ctx, `SELECT id, name, company, email FROM customers WHERE org_id = ? AND (name LIKE ? OR company LIKE ? OR email LIKE ?) LIMIT 5`, orgID, qLike, qLike, qLike)
	if err == nil {
		defer custRows.Close()
		for custRows.Next() {
			var id, name string
			var comp, email sql.NullString
			if err := custRows.Scan(&id, &name, &comp, &email); err == nil {
				res.Customers = append(res.Customers, map[string]interface{}{"id": id, "name": name, "company": comp.String, "email": email.String})
			}
		}
	}

	return res, nil
}

func (s *WorkflowService) ExportCSV(ctx context.Context, orgID, entityType string) ([]byte, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	buf := new(bytes.Buffer)
	writer := csv.NewWriter(buf)

	switch entityType {
	case "invoices":
		writer.Write([]string{"ID", "Invoice Number", "Status", "Issue Date", "Due Date", "Subtotal", "Tax", "Total Amount"})
		rows, err := s.store.DB().QueryContext(ctx, `SELECT id, invoice_number, status, issue_date, due_date, subtotal, tax_amount, total_amount FROM invoices WHERE org_id = ?`, orgID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id, num, status, issue, due string
			var sub, tax, tot float64
			if err := rows.Scan(&id, &num, &status, &issue, &due, &sub, &tax, &tot); err == nil {
				writer.Write([]string{id, num, status, issue, due, fmt.Sprintf("%.2f", sub), fmt.Sprintf("%.2f", tax), fmt.Sprintf("%.2f", tot)})
			}
		}

	case "skus":
		writer.Write([]string{"Code", "Name", "Category", "Stock Quantity", "Reserved", "Min Threshold", "Selling Price"})
		rows, err := s.store.DB().QueryContext(ctx, `SELECT code, name, category, stock_quantity, reserved_quantity, min_threshold, selling_price FROM skus WHERE org_id = ?`, orgID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var code, name, cat string
			var stock, res, min int
			var price float64
			if err := rows.Scan(&code, &name, &cat, &stock, &res, &min, &price); err == nil {
				writer.Write([]string{code, name, cat, fmt.Sprintf("%d", stock), fmt.Sprintf("%d", res), fmt.Sprintf("%d", min), fmt.Sprintf("%.2f", price)})
			}
		}

	default:
		return nil, fmt.Errorf("unsupported export entity: %s", entityType)
	}

	writer.Flush()
	return buf.Bytes(), nil
}

// ----------------- NOTIFICATIONS -----------------

func (s *WorkflowService) ListNotifications(ctx context.Context, orgID string, unreadOnly bool) ([]NotificationItem, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	query := `SELECT id, org_id, COALESCE(user_id, ''), title, message, category, is_read, created_at FROM notifications WHERE org_id = ?`
	if unreadOnly {
		query += " AND is_read = 0"
	}
	query += " ORDER BY created_at DESC LIMIT 50"

	rows, err := s.store.DB().QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []NotificationItem
	for rows.Next() {
		var n NotificationItem
		if err := rows.Scan(&n.ID, &n.OrgID, &n.UserID, &n.Title, &n.Message, &n.Category, &n.IsRead, &n.CreatedAt); err == nil {
			list = append(list, n)
		}
	}
	return list, nil
}

func (s *WorkflowService) CreateNotification(ctx context.Context, orgID, title, message, category string) error {
	if orgID == "" {
		orgID = "org_default"
	}
	id := fmt.Sprintf("notif_%s", uuid.New().String()[:8])
	_, err := s.store.DB().ExecContext(ctx, `INSERT INTO notifications (id, org_id, title, message, category, is_read)
		VALUES (?, ?, ?, ?, ?, 0)`, id, orgID, title, message, category)
	return err
}

func (s *WorkflowService) MarkNotificationRead(ctx context.Context, orgID, notifID string) error {
	_, err := s.store.DB().ExecContext(ctx, `UPDATE notifications SET is_read = 1 WHERE id = ? AND org_id = ?`, notifID, orgID)
	return err
}

// ----------------- COMPANY PROFILE -----------------

func (s *WorkflowService) GetCompanyProfile(ctx context.Context, orgID string) (*CompanyProfile, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	var p CompanyProfile
	err := s.store.DB().QueryRowContext(ctx, `SELECT org_id, company_name, currency, timezone, tax_rate, address, phone, website, onboarding_completed, updated_at
		FROM company_profiles WHERE org_id = ?`, orgID).
		Scan(&p.OrgID, &p.CompanyName, &p.Currency, &p.Timezone, &p.TaxRate, &p.Address, &p.Phone, &p.Website, &p.OnboardingCompleted, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &CompanyProfile{
				OrgID:               orgID,
				CompanyName:         "AO Sphere Business",
				Currency:            "USD",
				Timezone:            "UTC",
				TaxRate:             0.10,
				OnboardingCompleted: true,
				UpdatedAt:           time.Now(),
			}, nil
		}
		return nil, err
	}
	return &p, nil
}

func (s *WorkflowService) UpdateCompanyProfile(ctx context.Context, p CompanyProfile) error {
	if p.OrgID == "" {
		p.OrgID = "org_default"
	}
	query := `INSERT INTO company_profiles (org_id, company_name, currency, timezone, tax_rate, address, phone, website, onboarding_completed, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(org_id) DO UPDATE SET
			company_name=excluded.company_name,
			currency=excluded.currency,
			timezone=excluded.timezone,
			tax_rate=excluded.tax_rate,
			address=excluded.address,
			phone=excluded.phone,
			website=excluded.website,
			onboarding_completed=excluded.onboarding_completed,
			updated_at=excluded.updated_at`

	_, err := s.store.DB().ExecContext(ctx, query,
		p.OrgID, p.CompanyName, p.Currency, p.Timezone, p.TaxRate, p.Address, p.Phone, p.Website, p.OnboardingCompleted, time.Now())
	return err
}
