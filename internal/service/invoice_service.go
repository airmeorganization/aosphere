package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/db"
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

	// 3. Compute Server-Side Line Items, Tax, and Subtotal
	subtotal := 0.0
	for _, it := range req.Items {
		if it.Qty <= 0 {
			return nil, fmt.Errorf("invalid quantity %d for item %q", it.Qty, it.Desc)
		}
		if it.Price < 0.0 {
			return nil, fmt.Errorf("negative price %.2f not permitted for item %q", it.Price, it.Desc)
		}
		subtotal += float64(it.Qty) * it.Price
	}

	tax := subtotal * 0.08 // standard 8% tax calculation
	total := subtotal + tax

	recurring := req.Recurring
	if recurring == "" {
		recurring = "none"
	}
	dueDate := req.DueDate
	if dueDate == "" {
		dueDate = time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	}

	invID := "inv_" + uuid.New().String()[:8]
	invNum := fmt.Sprintf("INV-%d", time.Now().Unix())
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
	_ = s.store.LogAuditForOrg(orgID, "human", actor, "CREATE_INVOICE", "invoices", invID,
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
	_ = s.store.LogAuditForOrg(orgID, "human", actor, "PAY_INVOICE", "invoices", invoiceID,
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
