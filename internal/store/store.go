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

// DB returns the underlying sql.DB instance for transactional workflows
func (s *Store) DB() *sql.DB {
	return s.db
}

// Ping verifies database connectivity
func (s *Store) Ping() error {
	return s.db.Ping()
}

// ----------------- AUDIT & APPROVALS -----------------

func (s *Store) LogAudit(actorType, actorID, action, entityType, entityID, reasoning, meta string) error {
	return s.LogAuditForOrg("org_default", actorType, actorID, action, entityType, entityID, reasoning, meta)
}

func (s *Store) LogAuditForOrg(orgID, actorType, actorID, action, entityType, entityID, reasoning, meta string) error {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "aud_" + uuid.New().String()[:8]
	_, err := s.db.Exec(`INSERT INTO audit_logs (id, org_id, actor_type, actor_id, action, entity_type, entity_id, reasoning, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, orgID, actorType, actorID, action, entityType, entityID, reasoning, meta)
	return err
}

func (s *Store) LogAuditTx(tx *sql.Tx, orgID, actorType, actorID, action, entityType, entityID, reasoning, meta string) error {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "aud_" + uuid.New().String()[:8]
	_, err := tx.Exec(`INSERT INTO audit_logs (id, org_id, actor_type, actor_id, action, entity_type, entity_id, reasoning, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, orgID, actorType, actorID, action, entityType, entityID, reasoning, meta)
	return err
}

func (s *Store) GetAuditLogs(limit int) ([]db.AuditLog, error) {
	return s.GetAuditLogsForOrg("org_default", limit)
}

func (s *Store) GetAuditLogsForOrg(orgID string, limit int) ([]db.AuditLog, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, org_id, actor_type, actor_id, action, entity_type, entity_id, reasoning, metadata_json, created_at
		FROM audit_logs WHERE org_id = ? ORDER BY created_at DESC, rowid DESC LIMIT ?`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []db.AuditLog
	for rows.Next() {
		var l db.AuditLog
		if err := rows.Scan(&l.ID, &l.OrgID, &l.ActorType, &l.ActorID, &l.Action, &l.EntityType, &l.EntityID, &l.Reasoning, &l.MetadataJSON, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (s *Store) GetApprovals(status string) ([]db.Approval, error) {
	return s.GetApprovalsForOrg("org_default", status)
}

func (s *Store) GetApprovalsForOrg(orgID, status string) ([]db.Approval, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	query := `SELECT id, org_id, action_type, title, description, payload_json, payload_hash, entity_type, entity_id, entity_version, status, execution_status, execution_error, requested_by_type, requested_by_id, decided_by, decided_at, created_at
		FROM approvals WHERE org_id = ?`
	var rows *sql.Rows
	var err error
	if status != "" {
		query += " AND status = ? ORDER BY created_at DESC"
		rows, err = s.db.Query(query, orgID, status)
	} else {
		query += " ORDER BY created_at DESC"
		rows, err = s.db.Query(query, orgID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Approval
	for rows.Next() {
		var a db.Approval
		if err := rows.Scan(&a.ID, &a.OrgID, &a.ActionType, &a.Title, &a.Description, &a.PayloadJSON,
			&a.PayloadHash, &a.EntityType, &a.EntityID, &a.EntityVersion, &a.Status, &a.ExecutionStatus, &a.ExecutionError,
			&a.RequestedByType, &a.RequestedByID, &a.DecidedBy, &a.DecidedAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, nil
}

func (s *Store) DecideApproval(id, decision, decidedBy string) error {
	return s.DecideApprovalForOrg("org_default", id, decision, decidedBy)
}

func (s *Store) DecideApprovalForOrg(orgID, id, decision, decidedBy string) error {
	if orgID == "" {
		orgID = "org_default"
	}
	now := time.Now()
	res, err := s.db.Exec(`UPDATE approvals SET status = ?, decided_by = ?, decided_at = ? WHERE id = ? AND org_id = ?`,
		decision, decidedBy, now, id, orgID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("approval %s not found in org %s", id, orgID)
	}

	_ = s.LogAuditForOrg(orgID, "human", decidedBy, "DECIDE_APPROVAL", "approvals", id, fmt.Sprintf("Decision: %s", decision), "{}")
	return nil
}

// ----------------- ERP & SUPPLY CHAIN -----------------

func (s *Store) GetSKUs(belowThresholdOnly bool) ([]db.SKU, error) {
	return s.GetSKUsForOrg("org_default", belowThresholdOnly)
}

func (s *Store) GetSKUsForOrg(orgID string, belowThresholdOnly bool) ([]db.SKU, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	query := `SELECT s.id, s.org_id, s.code, s.name, s.category, s.stock_quantity, s.reserved_quantity, s.min_threshold,
		s.reorder_quantity, s.unit_cost, s.selling_price, COALESCE(s.supplier_id, '') as supplier_id, COALESCE(sup.name, '') as supplier_name, s.version, s.updated_at
		FROM skus s LEFT JOIN suppliers sup ON s.supplier_id = sup.id WHERE s.org_id = ?`
	if belowThresholdOnly {
		query += " AND s.stock_quantity <= s.min_threshold"
	}
	query += " ORDER BY s.code ASC"

	rows, err := s.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.SKU
	for rows.Next() {
		var item db.SKU
		if err := rows.Scan(&item.ID, &item.OrgID, &item.Code, &item.Name, &item.Category, &item.StockQuantity, &item.ReservedQuantity,
			&item.MinThreshold, &item.ReorderQuantity, &item.UnitCost, &item.SellingPrice, &item.SupplierID, &item.SupplierName, &item.Version, &item.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

func (s *Store) CreateSKU(code, name, category, supID string, stock, min, reorder int, cost, price float64) (*db.SKU, error) {
	return s.CreateSKUForOrg("org_default", code, name, category, supID, stock, min, reorder, cost, price)
}

func (s *Store) CreateSKUForOrg(orgID, code, name, category, supID string, stock, min, reorder int, cost, price float64) (*db.SKU, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "sku_" + uuid.New().String()[:8]
	now := time.Now()

	var supIDVal interface{}
	if supID != "" {
		supIDVal = supID
	} else {
		supIDVal = nil
	}

	_, err := s.db.Exec(`INSERT INTO skus (id, org_id, code, name, category, stock_quantity, reserved_quantity, min_threshold, reorder_quantity, unit_cost, selling_price, supplier_id, version, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, 1, ?)`,
		id, orgID, code, name, category, stock, min, reorder, cost, price, supIDVal, now.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to create sku: %w", err)
	}

	return &db.SKU{
		ID:               id,
		OrgID:            orgID,
		Code:             code,
		Name:             name,
		Category:         category,
		StockQuantity:    stock,
		ReservedQuantity: 0,
		MinThreshold:     min,
		ReorderQuantity:  reorder,
		UnitCost:         cost,
		SellingPrice:     price,
		SupplierID:       supID,
		Version:          1,
		UpdatedAt:        now,
	}, nil
}

func (s *Store) UpdateStock(skuID string, delta int, reason, actorType, actorID string) (*db.SKU, error) {
	return s.UpdateStockForOrg("org_default", skuID, delta, reason, actorType, actorID)
}

func (s *Store) UpdateStockForOrg(orgID, skuID string, delta int, reason, actorType, actorID string) (*db.SKU, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var currentStock, minThresh, reorderQty int
	var code, name, supplierID string
	var unitCost float64

	err = tx.QueryRow(`SELECT code, name, stock_quantity, min_threshold, reorder_quantity, unit_cost, supplier_id
		FROM skus WHERE id = ? AND org_id = ?`, skuID, orgID).Scan(&code, &name, &currentStock, &minThresh, &reorderQty, &unitCost, &supplierID)
	if err != nil {
		return nil, fmt.Errorf("sku %s not found in org %s: %w", skuID, orgID, err)
	}

	newStock := currentStock + delta
	if newStock < 0 {
		return nil, fmt.Errorf("insufficient stock (has %d, attempted delta %d)", currentStock, delta)
	}

	now := time.Now()
	_, err = tx.Exec(`UPDATE skus SET stock_quantity = ?, updated_at = ? WHERE id = ? AND org_id = ?`, newStock, now, skuID, orgID)
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
			tx.Exec(`INSERT INTO approvals (id, org_id, action_type, title, description, payload_json, status, requested_by_type, requested_by_id)
				VALUES (?, ?, 'create_po', ?, ?, ?, 'pending', 'agent', ?)`,
				apprID, orgID,
				fmt.Sprintf("Auto PO for %s (Low Stock: %d)", code, newStock),
				fmt.Sprintf("Stock dropped to %d (Min: %d). Need %d units at $%.2f ($%.2f total).", newStock, minThresh, reorderQty, unitCost, totalAmount),
				string(itemsJSON), actorID)

			tx.Exec(`INSERT INTO purchase_orders (id, org_id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes)
				VALUES (?, ?, ?, ?, 'pending_approval', ?, ?, ?, ?, 'Triggered by low stock workflow. Pending human safety sign-off.')`,
				poID, orgID, poNumber, supplierID, totalAmount, string(itemsJSON), actorType, actorID)
		} else {
			tx.Exec(`INSERT INTO purchase_orders (id, org_id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes)
				VALUES (?, ?, ?, ?, 'draft', ?, ?, ?, ?, 'Auto-generated under $1000 threshold.')`,
				poID, orgID, poNumber, supplierID, totalAmount, string(itemsJSON), actorType, actorID)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if autoReorderTriggered {
		_ = s.LogAuditForOrg(orgID, "agent", "workflow_engine", "AUTO_REORDER_TRIGGERED", "skus", skuID,
			fmt.Sprintf("Stock %d <= threshold %d. Triggered PO %s", newStock, minThresh, autoReorderPONumber), autoReorderItems)
	}

	_ = s.LogAuditForOrg(orgID, actorType, actorID, "STOCK_UPDATE", "skus", skuID,
		fmt.Sprintf("Stock adjusted by %d (New: %d). Reason: %s", delta, newStock, reason), "{}")

	return &db.SKU{
		ID:            skuID,
		OrgID:         orgID,
		Code:          code,
		Name:          name,
		StockQuantity: newStock,
		MinThreshold:  minThresh,
	}, nil
}

func (s *Store) GetSuppliers() ([]db.Supplier, error) {
	return s.GetSuppliersForOrg("org_default")
}

func (s *Store) GetSuppliersForOrg(orgID string) ([]db.Supplier, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT id, org_id, name, contact_email, phone, lead_time_days, rating FROM suppliers WHERE org_id = ? ORDER BY name ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Supplier
	for rows.Next() {
		var sup db.Supplier
		if err := rows.Scan(&sup.ID, &sup.OrgID, &sup.Name, &sup.ContactEmail, &sup.Phone, &sup.LeadTimeDays, &sup.Rating); err != nil {
			return nil, err
		}
		list = append(list, sup)
	}
	return list, nil
}

func (s *Store) GetPurchaseOrders() ([]db.PurchaseOrder, error) {
	return s.GetPurchaseOrdersForOrg("org_default")
}

func (s *Store) GetPurchaseOrdersForOrg(orgID string) ([]db.PurchaseOrder, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT po.id, po.org_id, po.po_number, po.supplier_id, COALESCE(sup.name, '') as supplier_name,
		po.status, po.total_amount, po.items_json, po.created_by_type, po.actor_id, po.notes, po.version, po.created_at
		FROM purchase_orders po LEFT JOIN suppliers sup ON po.supplier_id = sup.id
		WHERE po.org_id = ?
		ORDER BY po.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.PurchaseOrder
	for rows.Next() {
		var po db.PurchaseOrder
		if err := rows.Scan(&po.ID, &po.OrgID, &po.PONumber, &po.SupplierID, &po.SupplierName, &po.Status, &po.TotalAmount,
			&po.ItemsJSON, &po.CreatedByType, &po.ActorID, &po.Notes, &po.Version, &po.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, po)
	}
	return list, nil
}

func (s *Store) CreatePurchaseOrder(supplierID string, itemsJSON string, total float64, actorType, actorID, notes string) (*db.PurchaseOrder, error) {
	return s.CreatePurchaseOrderForOrg("org_default", supplierID, itemsJSON, total, actorType, actorID, notes)
}

func (s *Store) CreatePurchaseOrderForOrg(orgID, supplierID string, itemsJSON string, total float64, actorType, actorID, notes string) (*db.PurchaseOrder, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "po_" + uuid.New().String()[:8]
	poNum := fmt.Sprintf("PO-%d", time.Now().Unix())
	status := "ordered"
	if total > 1000.0 && actorType == "agent" {
		status = "pending_approval"
	}

	_, err := s.db.Exec(`INSERT INTO purchase_orders (id, org_id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, orgID, poNum, supplierID, status, total, itemsJSON, actorType, actorID, notes)
	if err != nil {
		return nil, err
	}

	_ = s.LogAuditForOrg(orgID, actorType, actorID, "CREATE_PURCHASE_ORDER", "purchase_orders", id,
		fmt.Sprintf("Created PO %s total $%.2f (status: %s)", poNum, total, status), itemsJSON)

	return &db.PurchaseOrder{
		ID:            id,
		OrgID:         orgID,
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
	return s.GetCustomersForOrg("org_default")
}

func (s *Store) GetCustomersForOrg(orgID string) ([]db.Customer, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT id, org_id, name, COALESCE(company, ''), COALESCE(email, ''), COALESCE(phone, ''), COALESCE(status, 'active'), lifetime_value, COALESCE(tags, '[]'), created_at FROM customers WHERE org_id = ? ORDER BY lifetime_value DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Customer
	for rows.Next() {
		var c db.Customer
		if err := rows.Scan(&c.ID, &c.OrgID, &c.Name, &c.Company, &c.Email, &c.Phone, &c.Status, &c.LifetimeValue, &c.Tags, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

func (s *Store) CreateCustomerForOrg(orgID, name, company, email, phone, status string, ltv float64, tags string) (*db.Customer, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "cust_" + uuid.New().String()[:8]
	now := time.Now()
	_, err := s.db.Exec(`INSERT INTO customers (id, org_id, name, company, email, phone, status, lifetime_value, tags, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, orgID, name, company, email, phone, status, ltv, tags, now)
	if err != nil {
		return nil, err
	}
	return &db.Customer{
		ID:            id,
		OrgID:         orgID,
		Name:          name,
		Company:       company,
		Email:         email,
		Phone:         phone,
		Status:        status,
		LifetimeValue: ltv,
		Tags:          tags,
		CreatedAt:     now,
	}, nil
}

func (s *Store) GetDeals() ([]db.Deal, error) {
	return s.GetDealsForOrg("org_default")
}

func (s *Store) GetDealsForOrg(orgID string) ([]db.Deal, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT d.id, d.org_id, d.title, d.customer_id, COALESCE(c.name, '') as customer_name,
		COALESCE(c.company, '') as customer_company, d.value, d.stage, d.probability, COALESCE(d.expected_close_date, ''), d.created_at
		FROM deals d LEFT JOIN customers c ON d.customer_id = c.id
		WHERE d.org_id = ?
		ORDER BY d.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Deal
	for rows.Next() {
		var d db.Deal
		if err := rows.Scan(&d.ID, &d.OrgID, &d.Title, &d.CustomerID, &d.CustomerName, &d.CustomerCompany, &d.Value, &d.Stage, &d.Probability, &d.ExpectedCloseDate, &d.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, nil
}

func (s *Store) UpdateDealStage(dealID, stage, actorType, actorID string) error {
	return s.UpdateDealStageForOrg("org_default", dealID, stage, actorType, actorID)
}

func (s *Store) UpdateDealStageForOrg(orgID, dealID, stage, actorType, actorID string) error {
	if orgID == "" {
		orgID = "org_default"
	}
	res, err := s.db.Exec(`UPDATE deals SET stage = ? WHERE id = ? AND org_id = ?`, stage, dealID, orgID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("deal %s not found in org %s", dealID, orgID)
	}

	_ = s.LogAuditForOrg(orgID, actorType, actorID, "UPDATE_DEAL_STAGE", "deals", dealID, fmt.Sprintf("Stage moved to: %s", stage), "{}")
	return nil
}

func (s *Store) CreateDeal(title, customerID string, value float64, stage string, prob int, closeDate, actorType, actorID string) (*db.Deal, error) {
	return s.CreateDealForOrg("org_default", title, customerID, value, stage, prob, closeDate, actorType, actorID)
}

func (s *Store) CreateDealForOrg(orgID, title, customerID string, value float64, stage string, prob int, closeDate, actorType, actorID string) (*db.Deal, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "deal_" + uuid.New().String()[:8]
	_, err := s.db.Exec(`INSERT INTO deals (id, org_id, title, customer_id, value, stage, probability, expected_close_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, orgID, title, customerID, value, stage, prob, closeDate)
	if err != nil {
		return nil, err
	}

	_ = s.LogAuditForOrg(orgID, actorType, actorID, "CREATE_DEAL", "deals", id, fmt.Sprintf("Created deal '%s' ($%.2f)", title, value), "{}")

	return &db.Deal{
		ID:                id,
		OrgID:             orgID,
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
	return s.GetInvoicesForOrg("org_default")
}

func (s *Store) GetInvoicesForOrg(orgID string) ([]db.Invoice, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT inv.id, inv.org_id, inv.invoice_number, inv.customer_id, COALESCE(c.name, '') as customer_name,
		inv.status, inv.issue_date, inv.due_date, inv.subtotal, inv.tax_amount, inv.total_amount,
		inv.recurring_frequency, inv.items_json, inv.version, inv.created_at
		FROM invoices inv LEFT JOIN customers c ON inv.customer_id = c.id
		WHERE inv.org_id = ?
		ORDER BY inv.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Invoice
	for rows.Next() {
		var i db.Invoice
		if err := rows.Scan(&i.ID, &i.OrgID, &i.InvoiceNumber, &i.CustomerID, &i.CustomerName, &i.Status, &i.IssueDate, &i.DueDate,
			&i.Subtotal, &i.TaxAmount, &i.TotalAmount, &i.RecurringFrequency, &i.ItemsJSON, &i.Version, &i.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, i)
	}
	return list, nil
}

func (s *Store) CreateInvoice(customerID string, itemsJSON string, subtotal, tax float64, dueDate, recurring, actorType, actorID string) (*db.Invoice, error) {
	return s.CreateInvoiceForOrg("org_default", customerID, itemsJSON, subtotal, tax, dueDate, recurring, actorType, actorID)
}

func (s *Store) CreateInvoiceForOrg(orgID, customerID string, itemsJSON string, subtotal, tax float64, dueDate, recurring, actorType, actorID string) (*db.Invoice, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "inv_" + uuid.New().String()[:8]
	invNum := fmt.Sprintf("INV-%d", time.Now().Unix())
	total := subtotal + tax
	now := time.Now().Format("2006-01-02")

	_, err := s.db.Exec(`INSERT INTO invoices (id, org_id, invoice_number, customer_id, status, issue_date, due_date, subtotal, tax_amount, total_amount, recurring_frequency, items_json)
		VALUES (?, ?, ?, ?, 'sent', ?, ?, ?, ?, ?, ?, ?)`,
		id, orgID, invNum, customerID, now, dueDate, subtotal, tax, total, recurring, itemsJSON)
	if err != nil {
		return nil, err
	}

	_ = s.LogAuditForOrg(orgID, actorType, actorID, "CREATE_INVOICE", "invoices", id,
		fmt.Sprintf("Created invoice %s for total $%.2f", invNum, total), itemsJSON)

	return &db.Invoice{
		ID:                 id,
		OrgID:              orgID,
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
	return s.GetExpensesForOrg("org_default")
}

func (s *Store) GetExpensesForOrg(orgID string) ([]db.Expense, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT id, org_id, category, amount, COALESCE(vendor, ''), COALESCE(description, ''), date, created_at FROM expenses WHERE org_id = ? ORDER BY date DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Expense
	for rows.Next() {
		var e db.Expense
		if err := rows.Scan(&e.ID, &e.OrgID, &e.Category, &e.Amount, &e.Vendor, &e.Description, &e.Date, &e.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, nil
}

func (s *Store) CreateExpenseForOrg(orgID, category string, amount float64, vendor, description, date string) (*db.Expense, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "exp_" + uuid.New().String()[:8]
	now := time.Now()
	_, err := s.db.Exec(`INSERT INTO expenses (id, org_id, category, amount, vendor, description, date, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, orgID, category, amount, vendor, description, date, now)
	if err != nil {
		return nil, err
	}
	return &db.Expense{
		ID:          id,
		OrgID:       orgID,
		Category:    category,
		Amount:      amount,
		Vendor:      vendor,
		Description: description,
		Date:        date,
		CreatedAt:   now,
	}, nil
}

func (s *Store) GetFinancialSummary() (*db.FinancialSummary, error) {
	return s.GetFinancialSummaryForOrg("org_default")
}

func (s *Store) GetFinancialSummaryForOrg(orgID string) (*db.FinancialSummary, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	var summary db.FinancialSummary

	// Revenue from paid invoices scoped to org
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(total_amount), 0), COUNT(*) FROM invoices WHERE org_id = ? AND status = 'paid'`, orgID).
		Scan(&summary.TotalRevenue, &summary.PaidCount)

	// Unpaid / pending invoices scoped to org
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(total_amount), 0), COUNT(*) FROM invoices WHERE org_id = ? AND status != 'paid'`, orgID).
		Scan(&summary.UnpaidTotal, &summary.PendingCount)

	// Expenses scoped to org
	_ = s.db.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE org_id = ?`, orgID).Scan(&summary.TotalExpense)

	summary.NetProfit = summary.TotalRevenue - summary.TotalExpense
	return &summary, nil
}

// ----------------- HRMS & WORKPLACE -----------------

func (s *Store) GetEmployees() ([]db.Employee, error) {
	return s.GetEmployeesForOrg("org_default")
}

func (s *Store) GetEmployeesForOrg(orgID string) ([]db.Employee, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT id, org_id, name, role, department, hourly_rate, status, COALESCE(email, ''), COALESCE(phone, ''), COALESCE(hired_at, ''), created_at FROM employees WHERE org_id = ? ORDER BY name ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Employee
	for rows.Next() {
		var emp db.Employee
		if err := rows.Scan(&emp.ID, &emp.OrgID, &emp.Name, &emp.Role, &emp.Department, &emp.HourlyRate, &emp.Status, &emp.Email, &emp.Phone, &emp.HiredAt, &emp.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, emp)
	}
	return list, nil
}

func (s *Store) CreateEmployee(orgID, name, role, department, email, phone string, hourlyRate float64) (*db.Employee, error) {
	return s.CreateEmployeeForOrg(orgID, name, role, department, email, phone, hourlyRate, "human", "system")
}

func (s *Store) CreateEmployeeForOrg(orgID, name, role, department, email, phone string, hourlyRate float64, actorType, actorID string) (*db.Employee, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "emp_" + uuid.New().String()[:8]
	now := time.Now()
	_, err := s.db.Exec(`INSERT INTO employees (id, org_id, name, role, department, hourly_rate, email, phone, status, hired_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active', ?)`,
		id, orgID, name, role, department, hourlyRate, email, phone, now.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	_ = s.LogAuditForOrg(orgID, actorType, actorID, "CREATE_EMPLOYEE", "employees", id, fmt.Sprintf("Created employee %s (%s)", name, role), "{}")
	return &db.Employee{
		ID:         id,
		OrgID:      orgID,
		Name:       name,
		Role:       role,
		Department: department,
		HourlyRate: hourlyRate,
		Status:     "active",
		Email:      email,
		Phone:      phone,
		HiredAt:    now.Format("2006-01-02"),
		CreatedAt:  now,
	}, nil
}

func (s *Store) GetTimesheetsForOrg(orgID string) ([]db.Timesheet, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT t.id, t.org_id, t.employee_id, COALESCE(e.name, '') as employee_name,
		t.work_date, t.hours_worked, t.overtime_hours, t.status, t.hourly_rate, t.created_at
		FROM timesheets t LEFT JOIN employees e ON t.employee_id = e.id
		WHERE t.org_id = ?
		ORDER BY t.work_date DESC, t.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Timesheet
	for rows.Next() {
		var t db.Timesheet
		if err := rows.Scan(&t.ID, &t.OrgID, &t.EmployeeID, &t.EmployeeName, &t.WorkDate, &t.HoursWorked, &t.OvertimeHours, &t.Status, &t.HourlyRate, &t.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, nil
}

func (s *Store) GetShifts() ([]db.Shift, error) {
	return s.GetShiftsForOrg("org_default")
}

func (s *Store) GetShiftsForOrg(orgID string) ([]db.Shift, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT sh.id, sh.org_id, sh.employee_id, COALESCE(e.name, '') as employee_name,
		sh.shift_date, sh.start_time, sh.end_time, COALESCE(sh.station, ''), sh.status, sh.created_at
		FROM shifts sh LEFT JOIN employees e ON sh.employee_id = e.id
		WHERE sh.org_id = ?
		ORDER BY sh.shift_date DESC, sh.start_time ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Shift
	for rows.Next() {
		var sh db.Shift
		if err := rows.Scan(&sh.ID, &sh.OrgID, &sh.EmployeeID, &sh.EmployeeName, &sh.ShiftDate, &sh.StartTime, &sh.EndTime, &sh.Station, &sh.Status, &sh.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, sh)
	}
	return list, nil
}

func (s *Store) UpdateShiftStatus(shiftID, status, actorType, actorID string) error {
	return s.UpdateShiftStatusForOrg("org_default", shiftID, status, actorType, actorID)
}

func (s *Store) UpdateShiftStatusForOrg(orgID, shiftID, status, actorType, actorID string) error {
	if orgID == "" {
		orgID = "org_default"
	}
	res, err := s.db.Exec(`UPDATE shifts SET status = ? WHERE id = ? AND org_id = ?`, status, shiftID, orgID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("shift %s not found in org %s", shiftID, orgID)
	}
	_ = s.LogAuditForOrg(orgID, actorType, actorID, "UPDATE_SHIFT", "shifts", shiftID, fmt.Sprintf("Status updated to %s", status), "{}")
	return nil
}

func (s *Store) CreateShiftForOrg(orgID, employeeID, shiftDate, startTime, endTime, station, status, actorType, actorID string) (*db.Shift, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if status == "" {
		status = "scheduled"
	}
	if station == "" {
		station = "Floor"
	}
	id := "shf_" + uuid.New().String()[:8]
	now := time.Now()

	var empName string
	if employeeID != "" {
		_ = s.db.QueryRow(`SELECT name FROM employees WHERE id = ? AND org_id = ?`, employeeID, orgID).Scan(&empName)
	}

	_, err := s.db.Exec(`INSERT INTO shifts (id, org_id, employee_id, shift_date, start_time, end_time, station, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, orgID, employeeID, shiftDate, startTime, endTime, station, status, now)
	if err != nil {
		return nil, err
	}

	_ = s.LogAuditForOrg(orgID, actorType, actorID, "CREATE_SHIFT", "shifts", id, fmt.Sprintf("Scheduled shift for %s on %s (%s-%s)", empName, shiftDate, startTime, endTime), "{}")

	return &db.Shift{
		ID:           id,
		OrgID:        orgID,
		EmployeeID:   employeeID,
		EmployeeName: empName,
		ShiftDate:    shiftDate,
		StartTime:    startTime,
		EndTime:      endTime,
		Station:      station,
		Status:       status,
		CreatedAt:    now,
	}, nil
}

func (s *Store) GetTasks() ([]db.Task, error) {
	return s.GetTasksForOrg("org_default")
}

func (s *Store) GetTasksForOrg(orgID string) ([]db.Task, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT t.id, t.org_id, t.title, COALESCE(t.description, ''), t.project_name, COALESCE(t.assignee_id, ''),
		COALESCE(e.name, 'Unassigned') as assignee_name, t.priority, t.status, COALESCE(t.due_date, ''), t.created_at
		FROM tasks t LEFT JOIN employees e ON t.assignee_id = e.id
		WHERE t.org_id = ?
		ORDER BY t.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Task
	for rows.Next() {
		var t db.Task
		if err := rows.Scan(&t.ID, &t.OrgID, &t.Title, &t.Description, &t.ProjectName, &t.AssigneeID, &t.AssigneeName, &t.Priority, &t.Status, &t.DueDate, &t.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, nil
}

func (s *Store) UpdateTaskStatus(taskID, status, actorType, actorID string) error {
	return s.UpdateTaskStatusForOrg("org_default", taskID, status, actorType, actorID)
}

func (s *Store) UpdateTaskStatusForOrg(orgID, taskID, status, actorType, actorID string) error {
	if orgID == "" {
		orgID = "org_default"
	}
	res, err := s.db.Exec(`UPDATE tasks SET status = ? WHERE id = ? AND org_id = ?`, status, taskID, orgID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task %s not found in org %s", taskID, orgID)
	}
	_ = s.LogAuditForOrg(orgID, actorType, actorID, "UPDATE_TASK", "tasks", taskID, fmt.Sprintf("Status set to %s", status), "{}")
	return nil
}

func (s *Store) CreateTaskForOrg(orgID, title, description, projectName, assigneeID, priority, dueDate, status, actorType, actorID string) (*db.Task, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if projectName == "" {
		projectName = "General"
	}
	if priority == "" {
		priority = "medium"
	}
	if status == "" {
		status = "todo"
	}
	id := "tsk_" + uuid.New().String()[:8]
	now := time.Now()

	var assigneeName string
	if assigneeID != "" {
		_ = s.db.QueryRow(`SELECT name FROM employees WHERE id = ? AND org_id = ?`, assigneeID, orgID).Scan(&assigneeName)
	}

	_, err := s.db.Exec(`INSERT INTO tasks (id, org_id, title, description, project_name, assignee_id, priority, status, due_date, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, orgID, title, description, projectName, assigneeID, priority, status, dueDate, now)
	if err != nil {
		return nil, err
	}

	_ = s.LogAuditForOrg(orgID, actorType, actorID, "CREATE_TASK", "tasks", id, fmt.Sprintf("Task '%s' created (Priority: %s)", title, priority), "{}")

	return &db.Task{
		ID:           id,
		OrgID:        orgID,
		Title:        title,
		Description:  description,
		ProjectName:  projectName,
		AssigneeID:   assigneeID,
		AssigneeName: assigneeName,
		Priority:     priority,
		Status:       status,
		DueDate:      dueDate,
		CreatedAt:    now,
	}, nil
}

// ----------------- DIGITAL PRESENCE & REPUTATION -----------------

func (s *Store) GetListings() ([]db.LocalListing, error) {
	return s.GetListingsForOrg("org_default")
}

func (s *Store) GetListingsForOrg(orgID string) ([]db.LocalListing, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT id, org_id, directory, COALESCE(nap_business_name, ''), COALESCE(nap_address, ''), COALESCE(nap_phone, ''), COALESCE(nap_website, ''), sync_status, last_synced_at FROM local_listings WHERE org_id = ? ORDER BY directory ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.LocalListing
	for rows.Next() {
		var l db.LocalListing
		if err := rows.Scan(&l.ID, &l.OrgID, &l.Directory, &l.NAPBusinessName, &l.NAPAddress, &l.NAPPhone, &l.NAPWebsite, &l.SyncStatus, &l.LastSyncedAt); err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, nil
}

func (s *Store) SyncNAP(businessName, address, phone, website, actorType, actorID string) error {
	return s.SyncNAPForOrg("org_default", businessName, address, phone, website, actorType, actorID)
}

func (s *Store) SyncNAPForOrg(orgID, businessName, address, phone, website, actorType, actorID string) error {
	if orgID == "" {
		orgID = "org_default"
	}
	now := time.Now()
	_, err := s.db.Exec(`UPDATE local_listings SET nap_business_name = ?, nap_address = ?, nap_phone = ?, nap_website = ?, sync_status = 'synced', last_synced_at = ? WHERE org_id = ?`,
		businessName, address, phone, website, now, orgID)
	if err != nil {
		return err
	}
	_ = s.LogAuditForOrg(orgID, actorType, actorID, "SYNC_NAP_DATA", "local_listings", "all",
		fmt.Sprintf("Synchronized NAP info across all directories: %s | %s", businessName, phone), "{}")
	return nil
}

func (s *Store) GetReviews() ([]db.Review, error) {
	return s.GetReviewsForOrg("org_default")
}

func (s *Store) GetReviewsForOrg(orgID string) ([]db.Review, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT id, org_id, platform, author_name, rating, review_text, sentiment, COALESCE(reply_text, ''), reply_status, created_at FROM reviews WHERE org_id = ? ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Review
	for rows.Next() {
		var r db.Review
		if err := rows.Scan(&r.ID, &r.OrgID, &r.Platform, &r.AuthorName, &r.Rating, &r.ReviewText, &r.Sentiment, &r.ReplyText, &r.ReplyStatus, &r.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

func (s *Store) ReplyReview(reviewID, replyText, status, actorType, actorID string) error {
	return s.ReplyReviewForOrg("org_default", reviewID, replyText, status, actorType, actorID)
}

func (s *Store) ReplyReviewForOrg(orgID, reviewID, replyText, status, actorType, actorID string) error {
	if orgID == "" {
		orgID = "org_default"
	}
	res, err := s.db.Exec(`UPDATE reviews SET reply_text = ?, reply_status = ? WHERE id = ? AND org_id = ?`, replyText, status, reviewID, orgID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("review %s not found in org %s", reviewID, orgID)
	}
	_ = s.LogAuditForOrg(orgID, actorType, actorID, "REPLY_REVIEW", "reviews", reviewID, fmt.Sprintf("Reply status: %s", status), replyText)
	return nil
}

// ----------------- SUPPLY CHAIN & MULTI-WAREHOUSE METHODS -----------------

func (s *Store) GetWarehousesForOrg(orgID string) ([]db.Warehouse, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT id, org_id, name, code, COALESCE(address, ''), is_active, created_at FROM warehouses WHERE org_id = ? ORDER BY name ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.Warehouse
	for rows.Next() {
		var w db.Warehouse
		if err := rows.Scan(&w.ID, &w.OrgID, &w.Name, &w.Code, &w.Address, &w.IsActive, &w.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, nil
}

func (s *Store) CreateWarehouseForOrg(orgID, name, code, address, actorType, actorID string) (*db.Warehouse, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "wh_" + uuid.New().String()[:8]
	now := time.Now()
	_, err := s.db.Exec(`INSERT INTO warehouses (id, org_id, name, code, address, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?)`,
		id, orgID, name, code, address, now)
	if err != nil {
		return nil, err
	}

	_ = s.LogAuditForOrg(orgID, actorType, actorID, "CREATE_WAREHOUSE", "warehouses", id, fmt.Sprintf("Created warehouse %s (%s)", name, code), "{}")

	return &db.Warehouse{
		ID:        id,
		OrgID:     orgID,
		Name:      name,
		Code:      code,
		Address:   address,
		IsActive:  true,
		CreatedAt: now,
	}, nil
}

func (s *Store) GetWarehouseStocksForOrg(orgID string) ([]db.WarehouseStock, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT ws.id, ws.org_id, ws.warehouse_id, COALESCE(w.name, ''), COALESCE(w.code, ''),
		ws.sku_id, COALESCE(sk.code, ''), COALESCE(sk.name, ''), ws.stock_quantity, ws.reserved_quantity, ws.bin_location, ws.updated_at
		FROM warehouse_stocks ws
		LEFT JOIN warehouses w ON ws.warehouse_id = w.id
		LEFT JOIN skus sk ON ws.sku_id = sk.id
		WHERE ws.org_id = ?
		ORDER BY w.name ASC, sk.code ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.WarehouseStock
	for rows.Next() {
		var ws db.WarehouseStock
		if err := rows.Scan(&ws.ID, &ws.OrgID, &ws.WarehouseID, &ws.WarehouseName, &ws.WarehouseCode,
			&ws.SKUID, &ws.SKUCode, &ws.SKUName, &ws.StockQuantity, &ws.ReservedQuantity, &ws.BinLocation, &ws.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, ws)
	}
	return list, nil
}

func (s *Store) SetWarehouseStockForOrg(orgID, warehouseID, skuID string, stockQty, reservedQty int, binLocation string) error {
	if orgID == "" {
		orgID = "org_default"
	}
	if binLocation == "" {
		binLocation = "A-01-01"
	}
	id := "ws_" + uuid.New().String()[:8]
	now := time.Now()
	_, err := s.db.Exec(`INSERT INTO warehouse_stocks (id, org_id, warehouse_id, sku_id, stock_quantity, reserved_quantity, bin_location, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(org_id, warehouse_id, sku_id) DO UPDATE SET
			stock_quantity = excluded.stock_quantity,
			reserved_quantity = excluded.reserved_quantity,
			bin_location = excluded.bin_location,
			updated_at = excluded.updated_at`,
		id, orgID, warehouseID, skuID, stockQty, reservedQty, binLocation, now)
	return err
}

func (s *Store) GetGRNsForOrg(orgID string) ([]db.GoodsReceiptNote, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT g.id, g.org_id, g.grn_number, g.po_id, COALESCE(p.po_number, ''),
		g.warehouse_id, COALESCE(w.name, ''), g.received_by, g.status, g.items_json, COALESCE(g.notes, ''), g.created_at
		FROM goods_receipt_notes g
		LEFT JOIN purchase_orders p ON g.po_id = p.id
		LEFT JOIN warehouses w ON g.warehouse_id = w.id
		WHERE g.org_id = ?
		ORDER BY g.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.GoodsReceiptNote
	for rows.Next() {
		var g db.GoodsReceiptNote
		if err := rows.Scan(&g.ID, &g.OrgID, &g.GRNNumber, &g.POID, &g.PONumber, &g.WarehouseID, &g.WarehouseName, &g.ReceivedBy, &g.Status, &g.ItemsJSON, &g.Notes, &g.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, g)
	}
	return list, nil
}

func (s *Store) CreateGRNForOrg(orgID, poID, warehouseID, receivedBy, notes, itemsJSON, actorType, actorID string) (*db.GoodsReceiptNote, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "grn_" + uuid.New().String()[:8]
	now := time.Now()
	grnNum := fmt.Sprintf("GRN-%s-%s", now.Format("0601"), id[4:])

	var poNumber string
	_ = s.db.QueryRow(`SELECT po_number FROM purchase_orders WHERE id = ? AND org_id = ?`, poID, orgID).Scan(&poNumber)

	var whName string
	_ = s.db.QueryRow(`SELECT name FROM warehouses WHERE id = ? AND org_id = ?`, warehouseID, orgID).Scan(&whName)

	_, err := s.db.Exec(`INSERT INTO goods_receipt_notes (id, org_id, grn_number, po_id, warehouse_id, received_by, status, items_json, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'completed', ?, ?, ?)`,
		id, orgID, grnNum, poID, warehouseID, receivedBy, itemsJSON, notes, now)
	if err != nil {
		return nil, err
	}

	_ = s.LogAuditForOrg(orgID, actorType, actorID, "CREATE_GRN", "goods_receipt_notes", id, fmt.Sprintf("GRN %s created for PO %s at %s", grnNum, poNumber, whName), itemsJSON)

	return &db.GoodsReceiptNote{
		ID:            id,
		OrgID:         orgID,
		GRNNumber:     grnNum,
		POID:          poID,
		PONumber:      poNumber,
		WarehouseID:   warehouseID,
		WarehouseName: whName,
		ReceivedBy:    receivedBy,
		Status:        "completed",
		ItemsJSON:     itemsJSON,
		Notes:         notes,
		CreatedAt:     now,
	}, nil
}

func (s *Store) GetSupplierBillsForOrg(orgID string) ([]db.SupplierBill, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT b.id, b.org_id, b.bill_number, b.supplier_id, COALESCE(sup.name, ''),
		COALESCE(b.po_id, ''), COALESCE(p.po_number, ''), b.amount, b.status, b.match_status, b.items_json, COALESCE(b.notes, ''), b.created_at
		FROM supplier_bills b
		LEFT JOIN suppliers sup ON b.supplier_id = sup.id
		LEFT JOIN purchase_orders p ON b.po_id = p.id
		WHERE b.org_id = ?
		ORDER BY b.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.SupplierBill
	for rows.Next() {
		var b db.SupplierBill
		if err := rows.Scan(&b.ID, &b.OrgID, &b.BillNumber, &b.SupplierID, &b.SupplierName, &b.POID, &b.PONumber, &b.Amount, &b.Status, &b.MatchStatus, &b.ItemsJSON, &b.Notes, &b.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, nil
}

func (s *Store) CreateSupplierBillForOrg(orgID, supplierID, poID string, amount float64, itemsJSON, notes, actorType, actorID string) (*db.SupplierBill, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "bill_" + uuid.New().String()[:8]
	now := time.Now()
	billNum := fmt.Sprintf("BILL-%s-%s", now.Format("0601"), id[5:])

	var supName string
	_ = s.db.QueryRow(`SELECT name FROM suppliers WHERE id = ? AND org_id = ?`, supplierID, orgID).Scan(&supName)

	var poNumber string
	if poID != "" {
		_ = s.db.QueryRow(`SELECT po_number FROM purchase_orders WHERE id = ? AND org_id = ?`, poID, orgID).Scan(&poNumber)
	}

	_, err := s.db.Exec(`INSERT INTO supplier_bills (id, org_id, bill_number, supplier_id, po_id, amount, status, match_status, items_json, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'open', 'pending', ?, ?, ?)`,
		id, orgID, billNum, supplierID, poID, amount, itemsJSON, notes, now)
	if err != nil {
		return nil, err
	}

	_ = s.LogAuditForOrg(orgID, actorType, actorID, "CREATE_SUPPLIER_BILL", "supplier_bills", id, fmt.Sprintf("Supplier Bill %s from %s ($%.2f)", billNum, supName, amount), itemsJSON)

	return &db.SupplierBill{
		ID:           id,
		OrgID:        orgID,
		BillNumber:   billNum,
		SupplierID:   supplierID,
		SupplierName: supName,
		POID:         poID,
		PONumber:     poNumber,
		Amount:       amount,
		Status:       "open",
		MatchStatus:  "pending",
		ItemsJSON:    itemsJSON,
		Notes:        notes,
		CreatedAt:    now,
	}, nil
}

func (s *Store) UpdateSupplierBillMatchStatusForOrg(orgID, billID, matchStatus, actorType, actorID string) error {
	if orgID == "" {
		orgID = "org_default"
	}
	res, err := s.db.Exec(`UPDATE supplier_bills SET match_status = ? WHERE id = ? AND org_id = ?`, matchStatus, billID, orgID)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("supplier bill %s not found in org %s", billID, orgID)
	}
	_ = s.LogAuditForOrg(orgID, actorType, actorID, "MATCH_SUPPLIER_BILL", "supplier_bills", billID, fmt.Sprintf("Match status updated to %s", matchStatus), "{}")
	return nil
}

func (s *Store) GetStockTransfersForOrg(orgID string) ([]db.StockTransfer, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT t.id, t.org_id, t.transfer_number, t.from_warehouse_id, COALESCE(wf.name, ''),
		t.to_warehouse_id, COALESCE(wt.name, ''), t.status, t.items_json, COALESCE(t.notes, ''), t.created_by, t.created_at
		FROM stock_transfers t
		LEFT JOIN warehouses wf ON t.from_warehouse_id = wf.id
		LEFT JOIN warehouses wt ON t.to_warehouse_id = wt.id
		WHERE t.org_id = ?
		ORDER BY t.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.StockTransfer
	for rows.Next() {
		var t db.StockTransfer
		if err := rows.Scan(&t.ID, &t.OrgID, &t.TransferNumber, &t.FromWarehouseID, &t.FromWarehouseName, &t.ToWarehouseID, &t.ToWarehouseName, &t.Status, &t.ItemsJSON, &t.Notes, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, nil
}

func (s *Store) CreateStockTransferForOrg(orgID, fromWarehouseID, toWarehouseID, itemsJSON, notes, createdBy, actorType, actorID string) (*db.StockTransfer, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "xfer_" + uuid.New().String()[:8]
	now := time.Now()
	xferNum := fmt.Sprintf("XFER-%s-%s", now.Format("0601"), id[5:])

	var wfName, wtName string
	_ = s.db.QueryRow(`SELECT name FROM warehouses WHERE id = ? AND org_id = ?`, fromWarehouseID, orgID).Scan(&wfName)
	_ = s.db.QueryRow(`SELECT name FROM warehouses WHERE id = ? AND org_id = ?`, toWarehouseID, orgID).Scan(&wtName)

	_, err := s.db.Exec(`INSERT INTO stock_transfers (id, org_id, transfer_number, from_warehouse_id, to_warehouse_id, status, items_json, notes, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, 'completed', ?, ?, ?, ?)`,
		id, orgID, xferNum, fromWarehouseID, toWarehouseID, itemsJSON, notes, createdBy, now)
	if err != nil {
		return nil, err
	}

	_ = s.LogAuditForOrg(orgID, actorType, actorID, "CREATE_STOCK_TRANSFER", "stock_transfers", id, fmt.Sprintf("Transferred stock from %s to %s", wfName, wtName), itemsJSON)

	return &db.StockTransfer{
		ID:                id,
		OrgID:             orgID,
		TransferNumber:    xferNum,
		FromWarehouseID:   fromWarehouseID,
		FromWarehouseName: wfName,
		ToWarehouseID:     toWarehouseID,
		ToWarehouseName:   wtName,
		Status:            "completed",
		ItemsJSON:         itemsJSON,
		Notes:             notes,
		CreatedBy:         createdBy,
		CreatedAt:         now,
	}, nil
}

func (s *Store) GetBatchLotsForOrg(orgID string) ([]db.BatchLot, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`SELECT bl.id, bl.org_id, bl.sku_id, COALESCE(sk.code, ''), COALESCE(sk.name, ''),
		bl.batch_number, COALESCE(bl.expiry_date, ''), bl.quantity, COALESCE(bl.warehouse_id, ''), COALESCE(w.name, ''), bl.created_at
		FROM batch_lots bl
		LEFT JOIN skus sk ON bl.sku_id = sk.id
		LEFT JOIN warehouses w ON bl.warehouse_id = w.id
		WHERE bl.org_id = ?
		ORDER BY bl.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []db.BatchLot
	for rows.Next() {
		var bl db.BatchLot
		if err := rows.Scan(&bl.ID, &bl.OrgID, &bl.SKUID, &bl.SKUCode, &bl.SKUName, &bl.BatchNumber, &bl.ExpiryDate, &bl.Quantity, &bl.WarehouseID, &bl.WarehouseName, &bl.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, bl)
	}
	return list, nil
}

func (s *Store) CreateBatchLotForOrg(orgID, skuID, batchNumber, expiryDate string, quantity int, warehouseID, actorType, actorID string) (*db.BatchLot, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	id := "lot_" + uuid.New().String()[:8]
	now := time.Now()

	var skuCode, skuName string
	_ = s.db.QueryRow(`SELECT code, name FROM skus WHERE id = ? AND org_id = ?`, skuID, orgID).Scan(&skuCode, &skuName)

	var whName string
	if warehouseID != "" {
		_ = s.db.QueryRow(`SELECT name FROM warehouses WHERE id = ? AND org_id = ?`, warehouseID, orgID).Scan(&whName)
	}

	_, err := s.db.Exec(`INSERT INTO batch_lots (id, org_id, sku_id, batch_number, expiry_date, quantity, warehouse_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, orgID, skuID, batchNumber, expiryDate, quantity, warehouseID, now)
	if err != nil {
		return nil, err
	}

	_ = s.LogAuditForOrg(orgID, actorType, actorID, "CREATE_BATCH_LOT", "batch_lots", id, fmt.Sprintf("Batch %s for SKU %s (Qty: %d)", batchNumber, skuCode, quantity), "{}")

	return &db.BatchLot{
		ID:            id,
		OrgID:         orgID,
		SKUID:         skuID,
		SKUCode:       skuCode,
		SKUName:       skuName,
		BatchNumber:   batchNumber,
		ExpiryDate:    expiryDate,
		Quantity:      quantity,
		WarehouseID:   warehouseID,
		WarehouseName: whName,
		CreatedAt:     now,
	}, nil
}

// ----------------- PHASE 4: CREDIT NOTES & REVERSALS -----------------

func (s *Store) GetCreditNotesForOrg(orgID string) ([]db.CreditNote, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`
		SELECT cn.id, cn.org_id, cn.credit_note_number, cn.invoice_id, COALESCE(inv.invoice_number, '') as invoice_number,
			cn.customer_id, COALESCE(c.name, '') as customer_name, cn.currency, cn.subtotal, cn.tax_amount,
			cn.total_amount, cn.total_cents, cn.reason, cn.status, cn.items_json, cn.created_at
		FROM credit_notes cn
		LEFT JOIN invoices inv ON cn.invoice_id = inv.id
		LEFT JOIN customers c ON cn.customer_id = c.id
		WHERE cn.org_id = ?
		ORDER BY cn.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []db.CreditNote
	for rows.Next() {
		var cn db.CreditNote
		if err := rows.Scan(&cn.ID, &cn.OrgID, &cn.CreditNoteNumber, &cn.InvoiceID, &cn.InvoiceNumber,
			&cn.CustomerID, &cn.CustomerName, &cn.Currency, &cn.Subtotal, &cn.TaxAmount,
			&cn.TotalAmount, &cn.TotalCents, &cn.Reason, &cn.Status, &cn.ItemsJSON, &cn.CreatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, cn)
	}
	return notes, nil
}

func (s *Store) GetCreditNoteByIDForOrg(orgID, id string) (*db.CreditNote, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	var cn db.CreditNote
	err := s.db.QueryRow(`
		SELECT cn.id, cn.org_id, cn.credit_note_number, cn.invoice_id, COALESCE(inv.invoice_number, '') as invoice_number,
			cn.customer_id, COALESCE(c.name, '') as customer_name, cn.currency, cn.subtotal, cn.tax_amount,
			cn.total_amount, cn.total_cents, cn.reason, cn.status, cn.items_json, cn.created_at
		FROM credit_notes cn
		LEFT JOIN invoices inv ON cn.invoice_id = inv.id
		LEFT JOIN customers c ON cn.customer_id = c.id
		WHERE cn.org_id = ? AND cn.id = ?`, orgID, id).Scan(
		&cn.ID, &cn.OrgID, &cn.CreditNoteNumber, &cn.InvoiceID, &cn.InvoiceNumber,
		&cn.CustomerID, &cn.CustomerName, &cn.Currency, &cn.Subtotal, &cn.TaxAmount,
		&cn.TotalAmount, &cn.TotalCents, &cn.Reason, &cn.Status, &cn.ItemsJSON, &cn.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &cn, nil
}

func (s *Store) CreateCreditNoteForOrg(orgID, invoiceID, customerID, currency, reason, itemsJSON string, subtotal, taxAmount, totalAmount float64, totalCents int64, actorType, actorID string) (*db.CreditNote, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if currency == "" {
		currency = "USD"
	}
	id := "cn_" + uuid.New().String()[:8]
	cnNumber := fmt.Sprintf("CN-%d", time.Now().Unix())
	now := time.Now()

	_, err := s.db.Exec(`
		INSERT INTO credit_notes (id, org_id, credit_note_number, invoice_id, customer_id, currency, subtotal, tax_amount, total_amount, total_cents, reason, status, items_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'issued', ?, ?)`,
		id, orgID, cnNumber, invoiceID, customerID, currency, subtotal, taxAmount, totalAmount, totalCents, reason, itemsJSON, now)
	if err != nil {
		return nil, err
	}

	_ = s.LogAuditForOrg(orgID, actorType, actorID, "CREATE_CREDIT_NOTE", "credit_notes", id,
		fmt.Sprintf("Issued credit note %s (%s %.2f) for reason %s", cnNumber, currency, totalAmount, reason), itemsJSON)

	return s.GetCreditNoteByIDForOrg(orgID, id)
}

// ----------------- PHASE 4: MULTI-CURRENCY EXCHANGE RATES -----------------

func (s *Store) GetExchangeRatesForOrg(orgID string) ([]db.ExchangeRate, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.Query(`
		SELECT id, org_id, base_currency, target_currency, rate, updated_at
		FROM exchange_rates WHERE org_id = ? ORDER BY target_currency ASC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rates []db.ExchangeRate
	for rows.Next() {
		var r db.ExchangeRate
		if err := rows.Scan(&r.ID, &r.OrgID, &r.BaseCurrency, &r.TargetCurrency, &r.Rate, &r.UpdatedAt); err != nil {
			return nil, err
		}
		rates = append(rates, r)
	}
	return rates, nil
}

func (s *Store) SetExchangeRateForOrg(orgID, baseCur, targetCur string, rate float64) error {
	if orgID == "" {
		orgID = "org_default"
	}
	id := fmt.Sprintf("fx_%s_%s", targetCur, orgID)
	now := time.Now()

	_, err := s.db.Exec(`
		INSERT INTO exchange_rates (id, org_id, base_currency, target_currency, rate, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(org_id, base_currency, target_currency) DO UPDATE SET rate = excluded.rate, updated_at = excluded.updated_at`,
		id, orgID, baseCur, targetCur, rate, now)
	return err
}

// ----------------- PHASE 4: CHART OF ACCOUNTS & GENERAL LEDGER -----------------

func (s *Store) GetAccountsForOrg(orgID string) ([]db.Account, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	query := `
		SELECT a.id, a.org_id, a.code, a.name, a.type, a.normal_balance, a.is_active, a.created_at,
			COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE 0 END), 0) AS total_debits,
			COALESCE(SUM(CASE WHEN jl.entry_type = 'credit' THEN jl.amount_cents ELSE 0 END), 0) AS total_credits
		FROM accounts a
		LEFT JOIN journal_lines jl ON a.id = jl.account_id AND jl.org_id = a.org_id
		LEFT JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
		WHERE a.org_id = ? AND a.is_active = 1
		GROUP BY a.id, a.org_id, a.code, a.name, a.type, a.normal_balance, a.is_active, a.created_at
		ORDER BY a.code ASC`

	rows, err := s.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []db.Account
	for rows.Next() {
		var a db.Account
		if err := rows.Scan(&a.ID, &a.OrgID, &a.Code, &a.Name, &a.Type, &a.NormalBalance, &a.IsActive, &a.CreatedAt, &a.DebitCents, &a.CreditCents); err != nil {
			return nil, err
		}
		if a.NormalBalance == "debit" {
			a.NetBalance = float64(a.DebitCents-a.CreditCents) / 100.0
		} else {
			a.NetBalance = float64(a.CreditCents-a.DebitCents) / 100.0
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (s *Store) GetJournalEntriesForOrg(orgID string, limit int) ([]db.JournalEntry, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(`
		SELECT id, org_id, entry_number, source_type, COALESCE(source_id, ''), memo, status, posted_at, created_by, created_at
		FROM journal_entries WHERE org_id = ? ORDER BY posted_at DESC, rowid DESC LIMIT ?`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []db.JournalEntry
	for rows.Next() {
		var je db.JournalEntry
		if err := rows.Scan(&je.ID, &je.OrgID, &je.EntryNumber, &je.SourceType, &je.SourceID, &je.Memo, &je.Status, &je.PostedAt, &je.CreatedBy, &je.CreatedAt); err != nil {
			return nil, err
		}

		// Fetch lines for this entry
		lineRows, err := s.db.Query(`
			SELECT jl.id, jl.org_id, jl.entry_id, jl.account_id, a.code, a.name, jl.entry_type, jl.amount_cents, jl.description, jl.created_at
			FROM journal_lines jl
			JOIN accounts a ON jl.account_id = a.id
			WHERE jl.entry_id = ? ORDER BY jl.entry_type DESC, jl.rowid ASC`, je.ID)
		if err == nil {
			for lineRows.Next() {
				var l db.JournalLine
				if err := lineRows.Scan(&l.ID, &l.OrgID, &l.EntryID, &l.AccountID, &l.AccountCode, &l.AccountName, &l.EntryType, &l.AmountCents, &l.Description, &l.CreatedAt); err == nil {
					l.Amount = float64(l.AmountCents) / 100.0
					je.Lines = append(je.Lines, l)
				}
			}
			lineRows.Close()
		}

		entries = append(entries, je)
	}
	return entries, nil
}

