package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/db"
	"aosphere/internal/finance"
	"github.com/google/uuid"
)

type InvoiceItem struct {
	Desc  string  `json:"desc"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

type CreateInvoiceRequest struct {
	CustomerID string        `json:"customer_id"`
	DueDate    string        `json:"due_date"`
	Recurring  string        `json:"recurring"`
	Items      []InvoiceItem `json:"items"`
}

// CreateInvoice validates entities, calculates server-side tax/totals, and persists invoices transactionally
func (s *WorkflowService) CreateInvoice(ctx context.Context, orgID string, req CreateInvoiceRequest, authCtx *auth.AuthContext, idempotencyKey string) (*db.Invoice, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("invoice must contain at least one item")
	}

	payloadBytes, _ := json.Marshal(req)
	payloadStr := string(payloadBytes)

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Idempotency Check
	idempRec, err := s.idemp.GetOrCreate(tx, orgID, idempotencyKey, payloadStr)
	if err != nil {
		return nil, err
	}
	if idempRec != nil && idempRec.Status == "completed" {
		var replay db.Invoice
		if err := json.Unmarshal([]byte(idempRec.ResponseBody), &replay); err == nil {
			return &replay, nil
		}
	}

	// 2. Validate Customer Reference
	if err := s.policy.ValidateCustomer(tx, orgID, req.CustomerID); err != nil {
		return nil, err
	}

	// 3. Compute Server-Side Line Items, Tax, and Subtotal in Minor Units (Cents)
	var subtotalCents int64
	for _, it := range req.Items {
		if it.Qty <= 0 {
			return nil, fmt.Errorf("invalid quantity %d for item %q", it.Qty, it.Desc)
		}
		if it.Price < 0.0 {
			return nil, fmt.Errorf("negative price %.2f not permitted for item %q", it.Price, it.Desc)
		}
		itemPriceCents := finance.DollarsToCents(it.Price)
		subtotalCents += int64(it.Qty) * itemPriceCents
	}

	taxCents := finance.CalculateTax(subtotalCents, finance.DefaultTaxRateBasisPoints)
	totalCents := subtotalCents + taxCents

	subtotal := finance.CentsToDollars(subtotalCents)
	tax := finance.CentsToDollars(taxCents)
	total := finance.CentsToDollars(totalCents)

	recurring := req.Recurring
	if recurring == "" {
		recurring = "none"
	}
	dueDate := req.DueDate
	if dueDate == "" {
		dueDate = time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	}

	invID := "inv_" + uuid.New().String()[:8]
	invNum, err := finance.GenerateSequenceNumber(ctx, tx, orgID, "INV")
	if err != nil {
		return nil, fmt.Errorf("failed to generate sequential invoice number: %w", err)
	}
	now := time.Now()
	issueDate := now.Format("2006-01-02")

	itemsJSONBytes, _ := json.Marshal(req.Items)
	itemsJSON := string(itemsJSONBytes)

	_, err = tx.Exec(`INSERT INTO invoices (id, org_id, invoice_number, customer_id, status, issue_date, due_date, subtotal, tax_amount, total_amount, recurring_frequency, items_json, version, created_at)
		VALUES (?, ?, ?, ?, 'sent', ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		invID, orgID, invNum, req.CustomerID, issueDate, dueDate, subtotal, tax, total, recurring, itemsJSON, now)
	if err != nil {
		return nil, fmt.Errorf("failed to persist invoice: %w", err)
	}

	actor := "system"
	if authCtx != nil {
		actor = authCtx.Name
	}

	// 4. Double-Entry General Ledger: Post Invoice Issued (Debit AR, Credit Sales Revenue & Tax)
	if _, err := s.ledger.PostInvoiceIssued(ctx, tx, orgID, invID, invNum, subtotalCents, taxCents, totalCents, actor); err != nil {
		return nil, fmt.Errorf("failed to post invoice to general ledger: %w", err)
	}

	// 5. Transactional Outbox Event
	if err := RecordOutboxEvent(ctx, tx, orgID, "invoice_created", map[string]interface{}{
		"invoice_id":     invID,
		"invoice_number": invNum,
		"customer_id":    req.CustomerID,
		"total_amount":   total,
	}); err != nil {
		return nil, fmt.Errorf("failed to record outbox event: %w", err)
	}

	_ = s.store.LogAuditTx(tx, orgID, "human", actor, "CREATE_INVOICE", "invoices", invID,
		fmt.Sprintf("Created invoice %s for total $%.2f (Subtotal: $%.2f, Tax: $%.2f)", invNum, total, subtotal, tax), itemsJSON)

	inv := &db.Invoice{
		ID:                 invID,
		OrgID:              orgID,
		InvoiceNumber:      invNum,
		CustomerID:         req.CustomerID,
		Status:             "sent",
		IssueDate:          issueDate,
		DueDate:            dueDate,
		Subtotal:           subtotal,
		TaxAmount:          tax,
		TotalAmount:        total,
		RecurringFrequency: recurring,
		ItemsJSON:          itemsJSON,
		Version:            1,
		CreatedAt:          now,
	}

	if idempotencyKey != "" {
		resBytes, _ := json.Marshal(inv)
		_ = s.idemp.Complete(tx, orgID, idempotencyKey, 201, string(resBytes))
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit invoice transaction: %w", err)
	}

	return inv, nil
}

// PayInvoice transitions an invoice to paid state with deterministic lifecycle checks
func (s *WorkflowService) PayInvoice(ctx context.Context, orgID, invoiceID string, authCtx *auth.AuthContext, idempotencyKey string) (*db.Invoice, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Idempotency check
	idempRec, err := s.idemp.GetOrCreate(tx, orgID, idempotencyKey, invoiceID+":pay")
	if err != nil {
		return nil, err
	}
	if idempRec != nil && idempRec.Status == "completed" {
		var replay db.Invoice
		if err := json.Unmarshal([]byte(idempRec.ResponseBody), &replay); err == nil {
			return &replay, nil
		}
	}

	var inv db.Invoice
	err = tx.QueryRow(`SELECT id, org_id, invoice_number, customer_id, status, issue_date, due_date, subtotal, tax_amount, total_amount, recurring_frequency, items_json, version, created_at 
		FROM invoices WHERE id = ? AND org_id = ?`, invoiceID, orgID).
		Scan(&inv.ID, &inv.OrgID, &inv.InvoiceNumber, &inv.CustomerID, &inv.Status, &inv.IssueDate, &inv.DueDate,
			&inv.Subtotal, &inv.TaxAmount, &inv.TotalAmount, &inv.RecurringFrequency, &inv.ItemsJSON, &inv.Version, &inv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("invoice %s not found: %w", invoiceID, err)
	}

	if err := ValidateInvoiceTransition(inv.Status, InvoiceStatusPaid); err != nil {
		return nil, err
	}

	newVersion := inv.Version + 1
	_, err = tx.Exec(`UPDATE invoices SET status = ?, version = ? WHERE id = ? AND org_id = ? AND version = ?`,
		InvoiceStatusPaid, newVersion, invoiceID, orgID, inv.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to update invoice status: %w", err)
	}

	actor := "system"
	if authCtx != nil {
		actor = authCtx.Name
	}

	// Double-Entry General Ledger: Post Invoice Paid (Debit Cash, Credit AR)
	totalCents := finance.DollarsToCents(inv.TotalAmount)
	if _, err := s.ledger.PostInvoicePaid(ctx, tx, orgID, invoiceID, inv.InvoiceNumber, totalCents, actor); err != nil {
		return nil, fmt.Errorf("failed to post invoice payment to general ledger: %w", err)
	}

	// Transactional Outbox Event
	if err := RecordOutboxEvent(ctx, tx, orgID, "invoice_paid", map[string]interface{}{
		"invoice_id":     invoiceID,
		"invoice_number": inv.InvoiceNumber,
		"total_amount":   inv.TotalAmount,
	}); err != nil {
		return nil, fmt.Errorf("failed to record outbox event: %w", err)
	}

	_ = s.store.LogAuditTx(tx, orgID, "human", actor, "PAY_INVOICE", "invoices", invoiceID,
		fmt.Sprintf("Invoice %s marked as paid for $%.2f", inv.InvoiceNumber, inv.TotalAmount), "{}")

	inv.Status = InvoiceStatusPaid
	inv.Version = newVersion

	if idempotencyKey != "" {
		resBytes, _ := json.Marshal(inv)
		_ = s.idemp.Complete(tx, orgID, idempotencyKey, 200, string(resBytes))
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit pay invoice transaction: %w", err)
	}

	return &inv, nil
}

// ----------------- CREDIT NOTE SERVICE -----------------

type CreateCreditNoteRequest struct {
	InvoiceID  string  `json:"invoice_id"`
	CustomerID string  `json:"customer_id"`
	Currency   string  `json:"currency"`
	Reason     string  `json:"reason"`
	Amount     float64 `json:"amount"`
	TaxAmount  float64 `json:"tax_amount"`
	ItemsJSON  string  `json:"items_json"`
}

func (s *WorkflowService) IssueCreditNote(ctx context.Context, orgID string, req CreateCreditNoteRequest, authCtx *auth.AuthContext, idempotencyKey string) (*db.CreditNote, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	invoiceNumber := ""
	customerID := req.CustomerID
	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}

	subtotal := req.Amount
	taxAmount := req.TaxAmount
	totalAmount := subtotal + taxAmount

	if req.InvoiceID != "" {
		var inv db.Invoice
		err := tx.QueryRow(`SELECT id, invoice_number, customer_id, subtotal, tax_amount, total_amount, items_json
			FROM invoices WHERE id = ? AND org_id = ?`, req.InvoiceID, orgID).
			Scan(&inv.ID, &inv.InvoiceNumber, &inv.CustomerID, &inv.Subtotal, &inv.TaxAmount, &inv.TotalAmount, &inv.ItemsJSON)
		if err == nil {
			invoiceNumber = inv.InvoiceNumber
			if customerID == "" {
				customerID = inv.CustomerID
			}
			if totalAmount <= 0 {
				subtotal = inv.Subtotal
				taxAmount = inv.TaxAmount
				totalAmount = inv.TotalAmount
			}
			if req.ItemsJSON == "" {
				req.ItemsJSON = inv.ItemsJSON
			}
		}
	}

	if customerID == "" {
		return nil, fmt.Errorf("customer_id is required to issue a credit note")
	}
	if totalAmount <= 0 {
		return nil, fmt.Errorf("credit note amount must be greater than zero")
	}

	subtotalCents := finance.DollarsToCents(subtotal)
	taxCents := finance.DollarsToCents(taxAmount)
	totalCents := subtotalCents + taxCents

	actorType := "human"
	actorID := "web_user"
	if authCtx != nil {
		actorID = authCtx.Name
		if authCtx.IsAPIKey {
			actorType = "agent"
		}
	}

	reason := req.Reason
	if reason == "" {
		reason = "billing_error"
	}

	cnID := "cn_" + uuid.New().String()[:8]
	cnNumber, err := finance.GenerateSequenceNumber(ctx, tx, orgID, "CN")
	if err != nil {
		cnNumber = fmt.Sprintf("CN-%d", time.Now().Unix())
	}
	now := time.Now()

	_, err = tx.Exec(`
		INSERT INTO credit_notes (id, org_id, credit_note_number, invoice_id, customer_id, currency, subtotal, tax_amount, total_amount, total_cents, reason, status, items_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'issued', ?, ?)`,
		cnID, orgID, cnNumber, req.InvoiceID, customerID, currency, subtotal, taxAmount, totalAmount, totalCents, reason, req.ItemsJSON, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert credit note: %w", err)
	}

	// Double-entry ledger reversal: Debit Sales Revenue / Sales Tax, Credit Accounts Receivable
	if _, err := s.ledger.PostCreditNoteIssued(ctx, tx, orgID, cnID, cnNumber, invoiceNumber, subtotalCents, taxCents, totalCents, actorID); err != nil {
		return nil, fmt.Errorf("failed to post credit note reversal to general ledger: %w", err)
	}

	// Transactional outbox event
	_ = RecordOutboxEvent(ctx, tx, orgID, "credit_note_issued", map[string]interface{}{
		"credit_note_id":     cnID,
		"credit_note_number": cnNumber,
		"invoice_id":         req.InvoiceID,
		"total_amount":       totalAmount,
		"currency":           currency,
	})

	_ = s.store.LogAuditTx(tx, orgID, actorType, actorID, "ISSUE_CREDIT_NOTE", "credit_notes", cnID,
		fmt.Sprintf("Issued Credit Note %s for %s %.2f (Reason: %s)", cnNumber, currency, totalAmount, reason), req.ItemsJSON)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit credit note transaction: %w", err)
	}

	return s.store.GetCreditNoteByIDForOrg(orgID, cnID)
}

