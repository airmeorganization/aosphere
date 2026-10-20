package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"aosphere/internal/db"
	"github.com/google/uuid"
)

type Store struct {
	db *sql.DB
}

func NewStore(database *sql.DB) *Store {
	return &Store{db: database}
}

// ----------------- AUDIT & APPROVALS -----------------

func (s *Store) LogAudit(actorType, actorID, action, entityType, entityID, reasoning, meta string) error {
	id := "aud_" + uuid.New().String()[:8]
	_, err := s.db.Exec(`INSERT INTO audit_logs (id, actor_type, actor_id, action, entity_type, entity_id, reasoning, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, actorType, actorID, action, entityType, entityID, reasoning, meta)
	return err
}

func (s *Store) GetAuditLogs(limit int) ([]db.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, actor_type, actor_id, action, entity_type, entity_id, reasoning, metadata_json, created_at
		FROM audit_logs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []db.AuditLog
	for rows.Next() {
		var l db.AuditLog
		if err := rows.Scan(&l.ID, &l.ActorType, &l.ActorID, &l.Action, &l.EntityType, &l.EntityID, &l.Reasoning, &l.MetadataJSON, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (s *Store) GetApprovals(status string) ([]db.Approval, error) {
	query := `SELECT id, action_type, title, description, payload_json, status, requested_by_type, requested_by_id, decided_by, decided_at, created_at
		FROM approvals`
	var rows *sql.Rows
	var err error
	if status != "" {
		query += " WHERE status = ? ORDER BY created_at DESC"
		rows, err = s.db.Query(query, status)
	} else {
		query += " ORDER BY created_at DESC"
		rows, err = s.db.Query(query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Approval
	for rows.Next() {
		var a db.Approval
		if err := rows.Scan(&a.ID, &a.ActionType, &a.Title, &a.Description, &a.PayloadJSON, &a.Status, &a.RequestedByType, &a.RequestedByID, &a.DecidedBy, &a.DecidedAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, nil
}

func (s *Store) DecideApproval(id, decision, decidedBy string) error {
	if decision != "approved" && decision != "rejected" {
		return fmt.Errorf("invalid approval decision %q: expected approved or rejected", decision)
	}
	if id == "" || decidedBy == "" {
		return fmt.Errorf("approval id and decision-maker are required")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var actionType, payloadJSON, currentStatus, requestedByID string
	err = tx.QueryRow(`SELECT action_type, payload_json, status, requested_by_id FROM approvals WHERE id = ?`, id).
		Scan(&actionType, &payloadJSON, &currentStatus, &requestedByID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("approval %s not found", id)
	}
	if err != nil {
		return err
	}
	if currentStatus != "pending" {
		return fmt.Errorf("approval %s is already %s", id, currentStatus)
	}

	// Approvals for purchase orders carry the exact PO ID in the payload. The
	// decision and the protected PO state change are committed atomically.
	if actionType == "create_po" {
		var payload struct {
			PurchaseOrderID string `json:"purchase_order_id"`
		}
		if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
			return fmt.Errorf("invalid purchase-order approval payload: %w", err)
		}
		if payload.PurchaseOrderID == "" && decision == "approved" {
			// Compatibility for approvals created before payloads carried a PO ID:
			// link by the exact item JSON and creator. Never authorize an unlinked PO.
			err := tx.QueryRow(`SELECT id FROM purchase_orders
				WHERE status = 'pending_approval' AND items_json = ? AND actor_id = ?
				ORDER BY created_at DESC LIMIT 1`, payloadJSON, requestedByID).Scan(&payload.PurchaseOrderID)
			if err != nil {
				return fmt.Errorf("approval %s has no safely identifiable purchase order; cannot authorize", id)
			}
		}
		if payload.PurchaseOrderID != "" {
			poStatus := "cancelled"
			if decision == "approved" {
				poStatus = "ordered"
			}
			res, err := tx.Exec(`UPDATE purchase_orders SET status = ? WHERE id = ? AND status = 'pending_approval'`,
				poStatus, payload.PurchaseOrderID)
			if err != nil {
				return err
			}
			changed, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if changed != 1 {
				return fmt.Errorf("purchase order %s is missing or no longer pending approval", payload.PurchaseOrderID)
			}
		}
	}

	now := time.Now()
	res, err := tx.Exec(`UPDATE approvals SET status = ?, decided_by = ?, decided_at = ?
		WHERE id = ? AND status = 'pending'`, decision, decidedBy, now, id)
	if err != nil {
		return err
	}
	changed, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("approval %s has already been decided", id)
	}

	auditID := "aud_" + uuid.New().String()[:8]
	_, err = tx.Exec(`INSERT INTO audit_logs
		(id, actor_type, actor_id, action, entity_type, entity_id, reasoning, metadata_json)
		VALUES (?, 'human', ?, 'DECIDE_APPROVAL', 'approvals', ?, ?, ?)`,
		auditID, decidedBy, id, fmt.Sprintf("Decision: %s", decision),
		fmt.Sprintf(`{"decision":%q,"action_type":%q}`, decision, actionType))
	if err != nil {
		return fmt.Errorf("record approval audit event: %w", err)
	}
	return tx.Commit()
}

// ----------------- ERP & SUPPLY CHAIN -----------------

func (s *Store) GetSKUs(belowThresholdOnly bool) ([]db.SKU, error) {
	query := `SELECT s.id, s.code, s.name, s.category, s.stock_quantity, s.reserved_quantity, s.min_threshold,
		s.reorder_quantity, s.unit_cost, s.selling_price, s.supplier_id, COALESCE(sup.name, '') as supplier_name, s.updated_at
		FROM skus s LEFT JOIN suppliers sup ON s.supplier_id = sup.id`
	if belowThresholdOnly {
		query += " WHERE s.stock_quantity <= s.min_threshold"
	}
	query += " ORDER BY s.code ASC"

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.SKU
	for rows.Next() {
		var item db.SKU
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Category, &item.StockQuantity, &item.ReservedQuantity,
			&item.MinThreshold, &item.ReorderQuantity, &item.UnitCost, &item.SellingPrice, &item.SupplierID, &item.SupplierName, &item.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

func (s *Store) UpdateStock(skuID string, delta int, reason, actorType, actorID string) (*db.SKU, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var currentStock, minThresh, reorderQty int
	var code, name, supplierID string
	var unitCost float64

	err = tx.QueryRow(`SELECT code, name, stock_quantity, min_threshold, reorder_quantity, unit_cost, supplier_id
		FROM skus WHERE id = ?`, skuID).Scan(&code, &name, &currentStock, &minThresh, &reorderQty, &unitCost, &supplierID)
	if err != nil {
		return nil, fmt.Errorf("sku %s not found: %w", skuID, err)
	}

	newStock := currentStock + delta
	if newStock < 0 {
		return nil, fmt.Errorf("insufficient stock (has %d, attempted delta %d)", currentStock, delta)
	}

	now := time.Now()
	_, err = tx.Exec(`UPDATE skus SET stock_quantity = ?, updated_at = ? WHERE id = ?`, newStock, now, skuID)
	if err != nil {
		return nil, err
	}

	var autoReorderTriggered bool
	var autoReorderPONumber, autoReorderItems string

	// Automated Workflow: If newStock <= minThreshold, trigger automatic PO draft or approval!
	if newStock <= minThresh {
		totalAmount := float64(reorderQty) * unitCost
		itemsJSON, _ := json.Marshal([]map[string]interface{}{
			{"sku_code": code, "sku_name": name, "quantity": reorderQty, "unit_cost": unitCost},
		})

		poID := "po_" + uuid.New().String()[:8]
		poNumber := fmt.Sprintf("PO-%d", time.Now().Unix())
		autoReorderTriggered = true
		autoReorderPONumber = poNumber
		autoReorderItems = string(itemsJSON)

		// Safety threshold: If PO > $1,000, route to HITL approval. Otherwise auto-draft.
		if totalAmount > 1000.0 {
			apprID := "appr_" + uuid.New().String()[:8]
			payloadJSON, err := json.Marshal(map[string]interface{}{
				"purchase_order_id": poID,
				"po_number":         poNumber,
				"supplier_id":       supplierID,
				"total_amount":      totalAmount,
				"items":             json.RawMessage(itemsJSON),
			})
			if err != nil {
				return nil, fmt.Errorf("build reorder approval payload: %w", err)
			}
			if _, err := tx.Exec(`INSERT INTO approvals (id, action_type, title, description, payload_json, status, requested_by_type, requested_by_id)
				VALUES (?, 'create_po', ?, ?, ?, 'pending', 'agent', ?)`,
				apprID,
				fmt.Sprintf("Auto PO for %s (Low Stock: %d)", code, newStock),
				fmt.Sprintf("Stock dropped to %d (Min: %d). Need %d units at $%.2f ($%.2f total).", newStock, minThresh, reorderQty, unitCost, totalAmount),
				string(payloadJSON), actorID); err != nil {
				return nil, fmt.Errorf("create reorder approval: %w", err)
			}

			if _, err := tx.Exec(`INSERT INTO purchase_orders (id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes)
				VALUES (?, ?, ?, 'pending_approval', ?, ?, ?, ?, 'Triggered by low stock workflow. Pending human safety sign-off.')`,
				poID, poNumber, supplierID, totalAmount, string(itemsJSON), actorType, actorID); err != nil {
				return nil, fmt.Errorf("create reorder purchase order: %w", err)
			}
		} else {
			if _, err := tx.Exec(`INSERT INTO purchase_orders (id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes)
				VALUES (?, ?, ?, 'draft', ?, ?, ?, ?, 'Auto-generated under $1000 threshold.')`,
				poID, poNumber, supplierID, totalAmount, string(itemsJSON), actorType, actorID); err != nil {
				return nil, fmt.Errorf("create draft reorder purchase order: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if autoReorderTriggered {
		_ = s.LogAudit("agent", "workflow_engine", "AUTO_REORDER_TRIGGERED", "skus", skuID,
			fmt.Sprintf("Stock %d <= threshold %d. Triggered PO %s", newStock, minThresh, autoReorderPONumber), autoReorderItems)
	}

	_ = s.LogAudit(actorType, actorID, "STOCK_UPDATE", "skus", skuID,
		fmt.Sprintf("Stock adjusted by %d (New: %d). Reason: %s", delta, newStock, reason), "{}")

	return &db.SKU{
		ID:            skuID,
		Code:          code,
		Name:          name,
		StockQuantity: newStock,
		MinThreshold:  minThresh,
	}, nil
}

func (s *Store) GetSuppliers() ([]db.Supplier, error) {
	rows, err := s.db.Query(`SELECT id, name, contact_email, phone, lead_time_days, rating FROM suppliers ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Supplier
	for rows.Next() {
		var sup db.Supplier
		if err := rows.Scan(&sup.ID, &sup.Name, &sup.ContactEmail, &sup.Phone, &sup.LeadTimeDays, &sup.Rating); err != nil {
			return nil, err
		}
		list = append(list, sup)
	}
	return list, nil
}

func (s *Store) GetPurchaseOrders() ([]db.PurchaseOrder, error) {
	rows, err := s.db.Query(`SELECT po.id, po.po_number, po.supplier_id, COALESCE(sup.name, '') as supplier_name,
		po.status, po.total_amount, po.items_json, po.created_by_type, po.actor_id, po.notes, po.created_at
		FROM purchase_orders po LEFT JOIN suppliers sup ON po.supplier_id = sup.id
		ORDER BY po.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.PurchaseOrder
	for rows.Next() {
		var po db.PurchaseOrder
		if err := rows.Scan(&po.ID, &po.PONumber, &po.SupplierID, &po.SupplierName, &po.Status, &po.TotalAmount,
			&po.ItemsJSON, &po.CreatedByType, &po.ActorID, &po.Notes, &po.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, po)
	}
	return list, nil
}

func (s *Store) CreatePurchaseOrder(supplierID string, itemsJSON string, total float64, actorType, actorID, notes string) (*db.PurchaseOrder, error) {
	if supplierID == "" || actorID == "" {
		return nil, fmt.Errorf("supplier id and actor id are required")
	}
	var items []struct {
		SKUCode  string  `json:"sku_code"`
		Quantity int     `json:"quantity"`
		UnitCost float64 `json:"unit_cost"`
	}
	if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
		return nil, fmt.Errorf("invalid purchase-order items JSON: %w", err)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("purchase order must contain at least one item")
	}
	calculatedTotal := 0.0
	for i, item := range items {
		if item.Quantity <= 0 || item.UnitCost < 0 {
			return nil, fmt.Errorf("item %d must have a positive quantity and non-negative unit cost", i+1)
		}
		calculatedTotal += float64(item.Quantity) * item.UnitCost
	}
	// Never use a caller-supplied total as the value on which the safety policy depends.
	// A mismatch is rejected so the caller can correct stale or malformed inputs.
	if total < 0 || (calculatedTotal-total > 0.01) || (total-calculatedTotal > 0.01) {
		return nil, fmt.Errorf("purchase-order total mismatch: server calculated %.2f but request supplied %.2f", calculatedTotal, total)
	}
	total = calculatedTotal

	id := "po_" + uuid.New().String()[:8]
	poNum := fmt.Sprintf("PO-%s", uuid.New().String()[:8])
	status := "ordered"
	if total > 1000.0 {
		status = "pending_approval"
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO purchase_orders
		(id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, poNum, supplierID, status, total, itemsJSON, actorType, actorID, notes); err != nil {
		return nil, err
	}

	if status == "pending_approval" {
		apprID := "appr_" + uuid.New().String()[:8]
		payload, err := json.Marshal(map[string]interface{}{
			"purchase_order_id": id,
			"po_number":         poNum,
			"supplier_id":       supplierID,
			"total_amount":      total,
			"items":             json.RawMessage(itemsJSON),
		})
		if err != nil {
			return nil, fmt.Errorf("build purchase-order approval payload: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO approvals
			(id, action_type, title, description, payload_json, status, requested_by_type, requested_by_id)
			VALUES (?, 'create_po', ?, ?, ?, 'pending', ?, ?)`,
			apprID, fmt.Sprintf("Purchase order %s requires approval", poNum),
			fmt.Sprintf("Purchase order total is %.2f, above the 1000.00 approval threshold.", total),
			string(payload), actorType, actorID); err != nil {
			return nil, fmt.Errorf("create purchase-order approval: %w", err)
		}
	}

	auditID := "aud_" + uuid.New().String()[:8]
	if _, err := tx.Exec(`INSERT INTO audit_logs
		(id, actor_type, actor_id, action, entity_type, entity_id, reasoning, metadata_json)
		VALUES (?, ?, ?, 'CREATE_PURCHASE_ORDER', 'purchase_orders', ?, ?, ?)`,
		auditID, actorType, actorID, id,
		fmt.Sprintf("Created PO %s total $%.2f (status: %s)", poNum, total, status), itemsJSON); err != nil {
		return nil, fmt.Errorf("record purchase-order audit event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &db.PurchaseOrder{
		ID:            id,
		PONumber:      poNum,
		SupplierID:    supplierID,
		Status:        status,
		TotalAmount:   total,
		ItemsJSON:     itemsJSON,
		CreatedByType: actorType,
		ActorID:       actorID,
		Notes:         notes,
	}, nil
}

// ----------------- CRM & SALES -----------------

func (s *Store) GetCustomers() ([]db.Customer, error) {
	rows, err := s.db.Query(`SELECT id, name, company, email, phone, status, lifetime_value, tags, created_at FROM customers ORDER BY lifetime_value DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Customer
	for rows.Next() {
		var c db.Customer
		if err := rows.Scan(&c.ID, &c.Name, &c.Company, &c.Email, &c.Phone, &c.Status, &c.LifetimeValue, &c.Tags, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

func (s *Store) GetDeals() ([]db.Deal, error) {
	rows, err := s.db.Query(`SELECT d.id, d.title, d.customer_id, COALESCE(c.name, '') as customer_name,
		COALESCE(c.company, '') as customer_company, d.value, d.stage, d.probability, d.expected_close_date, d.created_at
		FROM deals d LEFT JOIN customers c ON d.customer_id = c.id
		ORDER BY d.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Deal
	for rows.Next() {
		var d db.Deal
		if err := rows.Scan(&d.ID, &d.Title, &d.CustomerID, &d.CustomerName, &d.CustomerCompany, &d.Value, &d.Stage, &d.Probability, &d.ExpectedCloseDate, &d.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, nil
}

func (s *Store) UpdateDealStage(dealID, stage, actorType, actorID string) error {
	res, err := s.db.Exec(`UPDATE deals SET stage = ? WHERE id = ?`, stage, dealID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("deal %s not found", dealID)
	}

	_ = s.LogAudit(actorType, actorID, "UPDATE_DEAL_STAGE", "deals", dealID, fmt.Sprintf("Stage moved to: %s", stage), "{}")
	return nil
}

func (s *Store) CreateDeal(title, customerID string, value float64, stage string, prob int, closeDate, actorType, actorID string) (*db.Deal, error) {
	id := "deal_" + uuid.New().String()[:8]
	_, err := s.db.Exec(`INSERT INTO deals (id, title, customer_id, value, stage, probability, expected_close_date)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, id, title, customerID, value, stage, prob, closeDate)
	if err != nil {
		return nil, err
	}

	_ = s.LogAudit(actorType, actorID, "CREATE_DEAL", "deals", id, fmt.Sprintf("Created deal '%s' ($%.2f)", title, value), "{}")

	return &db.Deal{
		ID:                id,
		Title:             title,
		CustomerID:        customerID,
		Value:             value,
		Stage:             stage,
		Probability:       prob,
		ExpectedCloseDate: closeDate,
	}, nil
}

// ----------------- FINANCIAL & INVOICING -----------------

func (s *Store) GetInvoices() ([]db.Invoice, error) {
	rows, err := s.db.Query(`SELECT inv.id, inv.invoice_number, inv.customer_id, COALESCE(c.name, '') as customer_name,
		inv.status, inv.issue_date, inv.due_date, inv.subtotal, inv.tax_amount, inv.total_amount,
		inv.recurring_frequency, inv.items_json, inv.created_at
		FROM invoices inv LEFT JOIN customers c ON inv.customer_id = c.id
		ORDER BY inv.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Invoice
	for rows.Next() {
		var i db.Invoice
		if err := rows.Scan(&i.ID, &i.InvoiceNumber, &i.CustomerID, &i.CustomerName, &i.Status, &i.IssueDate, &i.DueDate,
			&i.Subtotal, &i.TaxAmount, &i.TotalAmount, &i.RecurringFrequency, &i.ItemsJSON, &i.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, i)
	}
	return list, nil
}

func (s *Store) CreateInvoice(customerID string, itemsJSON string, subtotal, tax float64, dueDate, recurring, actorType, actorID string) (*db.Invoice, error) {
	id := "inv_" + uuid.New().String()[:8]
	invNum := fmt.Sprintf("INV-%d", time.Now().Unix())
	total := subtotal + tax
	now := time.Now().Format("2006-01-02")

	_, err := s.db.Exec(`INSERT INTO invoices (id, invoice_number, customer_id, status, issue_date, due_date, subtotal, tax_amount, total_amount, recurring_frequency, items_json)
		VALUES (?, ?, ?, 'sent', ?, ?, ?, ?, ?, ?, ?)`,
		id, invNum, customerID, now, dueDate, subtotal, tax, total, recurring, itemsJSON)
	if err != nil {
		return nil, err
	}

	_ = s.LogAudit(actorType, actorID, "CREATE_INVOICE", "invoices", id,
		fmt.Sprintf("Created invoice %s for total $%.2f", invNum, total), itemsJSON)

	return &db.Invoice{
		ID:                 id,
		InvoiceNumber:      invNum,
		CustomerID:         customerID,
		Status:             "sent",
		IssueDate:          now,
		DueDate:            dueDate,
		Subtotal:           subtotal,
		TaxAmount:          tax,
		TotalAmount:        total,
		RecurringFrequency: recurring,
		ItemsJSON:          itemsJSON,
	}, nil
}

func (s *Store) GetExpenses() ([]db.Expense, error) {
	rows, err := s.db.Query(`SELECT id, category, amount, vendor, description, date, created_at FROM expenses ORDER BY date DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Expense
	for rows.Next() {
		var e db.Expense
		if err := rows.Scan(&e.ID, &e.Category, &e.Amount, &e.Vendor, &e.Description, &e.Date, &e.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, nil
}

func (s *Store) GetFinancialSummary() (*db.FinancialSummary, error) {
	var summary db.FinancialSummary

	// Revenue from paid invoices
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(total_amount), 0), COUNT(*) FROM invoices WHERE status = 'paid'`).
		Scan(&summary.TotalRevenue, &summary.PaidCount)

	// Unpaid / pending invoices
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(total_amount), 0), COUNT(*) FROM invoices WHERE status != 'paid'`).
		Scan(&summary.UnpaidTotal, &summary.PendingCount)

	// Expenses
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM expenses`).Scan(&summary.TotalExpense)

	summary.NetProfit = summary.TotalRevenue - summary.TotalExpense
	return &summary, nil
}

// ----------------- HRMS & WORKPLACE -----------------

func (s *Store) GetEmployees() ([]db.Employee, error) {
	rows, err := s.db.Query(`SELECT id, name, role, department, hourly_rate, status, email, phone, hired_at, created_at FROM employees ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Employee
	for rows.Next() {
		var emp db.Employee
		if err := rows.Scan(&emp.ID, &emp.Name, &emp.Role, &emp.Department, &emp.HourlyRate, &emp.Status, &emp.Email, &emp.Phone, &emp.HiredAt, &emp.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, emp)
	}
	return list, nil
}

func (s *Store) GetShifts() ([]db.Shift, error) {
	rows, err := s.db.Query(`SELECT sh.id, sh.employee_id, COALESCE(e.name, '') as employee_name,
		sh.shift_date, sh.start_time, sh.end_time, sh.station, sh.status, sh.created_at
		FROM shifts sh LEFT JOIN employees e ON sh.employee_id = e.id
		ORDER BY sh.shift_date DESC, sh.start_time ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Shift
	for rows.Next() {
		var sh db.Shift
		if err := rows.Scan(&sh.ID, &sh.EmployeeID, &sh.EmployeeName, &sh.ShiftDate, &sh.StartTime, &sh.EndTime, &sh.Station, &sh.Status, &sh.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, sh)
	}
	return list, nil
}

func (s *Store) UpdateShiftStatus(shiftID, status, actorType, actorID string) error {
	_, err := s.db.Exec(`UPDATE shifts SET status = ? WHERE id = ?`, status, shiftID)
	if err != nil {
		return err
	}
	_ = s.LogAudit(actorType, actorID, "UPDATE_SHIFT", "shifts", shiftID, fmt.Sprintf("Status updated to %s", status), "{}")
	return nil
}

func (s *Store) GetTasks() ([]db.Task, error) {
	rows, err := s.db.Query(`SELECT t.id, t.title, t.description, t.project_name, t.assignee_id,
		COALESCE(e.name, 'Unassigned') as assignee_name, t.priority, t.status, t.due_date, t.created_at
		FROM tasks t LEFT JOIN employees e ON t.assignee_id = e.id
		ORDER BY t.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Task
	for rows.Next() {
		var t db.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.ProjectName, &t.AssigneeID, &t.AssigneeName, &t.Priority, &t.Status, &t.DueDate, &t.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, nil
}

func (s *Store) UpdateTaskStatus(taskID, status, actorType, actorID string) error {
	_, err := s.db.Exec(`UPDATE tasks SET status = ? WHERE id = ?`, status, taskID)
	if err != nil {
		return err
	}
	_ = s.LogAudit(actorType, actorID, "UPDATE_TASK", "tasks", taskID, fmt.Sprintf("Status set to %s", status), "{}")
	return nil
}

// ----------------- DIGITAL PRESENCE & REPUTATION -----------------

func (s *Store) GetListings() ([]db.LocalListing, error) {
	rows, err := s.db.Query(`SELECT id, directory, nap_business_name, nap_address, nap_phone, nap_website, sync_status, last_synced_at FROM local_listings ORDER BY directory ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.LocalListing
	for rows.Next() {
		var l db.LocalListing
		if err := rows.Scan(&l.ID, &l.Directory, &l.NAPBusinessName, &l.NAPAddress, &l.NAPPhone, &l.NAPWebsite, &l.SyncStatus, &l.LastSyncedAt); err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, nil
}

func (s *Store) SyncNAP(businessName, address, phone, website, actorType, actorID string) error {
	now := time.Now()
	_, err := s.db.Exec(`UPDATE local_listings SET nap_business_name = ?, nap_address = ?, nap_phone = ?, nap_website = ?, sync_status = 'synced', last_synced_at = ?`,
		businessName, address, phone, website, now)
	if err != nil {
		return err
	}
	_ = s.LogAudit(actorType, actorID, "SYNC_NAP_DATA", "local_listings", "all",
		fmt.Sprintf("Synchronized NAP info across all directories: %s | %s", businessName, phone), "{}")
	return nil
}

func (s *Store) GetReviews() ([]db.Review, error) {
	rows, err := s.db.Query(`SELECT id, platform, author_name, rating, review_text, sentiment, reply_text, reply_status, created_at FROM reviews ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Review
	for rows.Next() {
		var r db.Review
		if err := rows.Scan(&r.ID, &r.Platform, &r.AuthorName, &r.Rating, &r.ReviewText, &r.Sentiment, &r.ReplyText, &r.ReplyStatus, &r.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

func (s *Store) ReplyReview(reviewID, replyText, status, actorType, actorID string) error {
	_, err := s.db.Exec(`UPDATE reviews SET reply_text = ?, reply_status = ? WHERE id = ?`, replyText, status, reviewID)
	if err != nil {
		return err
	}
	_ = s.LogAudit(actorType, actorID, "REPLY_REVIEW", "reviews", reviewID, fmt.Sprintf("Reply status: %s", status), replyText)
	return nil
}
