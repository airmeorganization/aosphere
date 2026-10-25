package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/db"
	"aosphere/internal/finance"
	"github.com/google/uuid"
)

type GRNItem struct {
	SKUCode     string `json:"sku_code"`
	Quantity    int    `json:"quantity"`
	BinLocation string `json:"bin_location"`
	BatchNumber string `json:"batch_number,omitempty"`
	ExpiryDate  string `json:"expiry_date,omitempty"`
}

type ReceiveGRNRequest struct {
	POID        string    `json:"po_id"`
	WarehouseID string    `json:"warehouse_id"`
	ReceivedBy  string    `json:"received_by"`
	Notes       string    `json:"notes"`
	Items       []GRNItem `json:"items"`
}

type ReceiveGRNResult struct {
	GRN           *db.GoodsReceiptNote `json:"grn"`
	PurchaseOrder *db.PurchaseOrder    `json:"purchase_order"`
	IsPartial     bool                 `json:"is_partial"`
}

// ReceiveGoodsReceiptNote processes a delivery receipt against a PO, supporting partial receipts
func (s *WorkflowService) ReceiveGoodsReceiptNote(ctx context.Context, orgID string, req ReceiveGRNRequest, authCtx *auth.AuthContext, idempotencyKey string) (*ReceiveGRNResult, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("GRN must contain at least one received item")
	}

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Fetch Purchase Order
	var po db.PurchaseOrder
	err = tx.QueryRow(`SELECT id, org_id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes, version, created_at
		FROM purchase_orders WHERE id = ? AND org_id = ?`, req.POID, orgID).
		Scan(&po.ID, &po.OrgID, &po.PONumber, &po.SupplierID, &po.Status, &po.TotalAmount, &po.ItemsJSON,
			&po.CreatedByType, &po.ActorID, &po.Notes, &po.Version, &po.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("purchase order %s not found: %w", req.POID, err)
	}

	if po.Status != POStatusOrdered && po.Status != POStatusPartiallyReceived {
		return nil, fmt.Errorf("%w: PO %s is currently in status '%s', must be 'ordered' or 'partially_received'",
			ErrInvalidStateTransition, po.PONumber, po.Status)
	}

	// 2. Fetch Warehouse
	var whName string
	err = tx.QueryRow(`SELECT name FROM warehouses WHERE id = ? AND org_id = ?`, req.WarehouseID, orgID).Scan(&whName)
	if err != nil {
		return nil, fmt.Errorf("warehouse %s not found in org %s: %w", req.WarehouseID, orgID, err)
	}

	// 3. Parse PO Ordered Items
	var orderedItems []map[string]interface{}
	_ = json.Unmarshal([]byte(po.ItemsJSON), &orderedItems)
	orderedMap := make(map[string]int) // sku_code -> ordered qty
	for _, item := range orderedItems {
		code, _ := item["sku_code"].(string)
		qtyFloat, _ := item["quantity"].(float64)
		orderedMap[code] = int(qtyFloat)
	}

	// 4. Calculate previously received quantities from existing GRNs for this PO
	prevReceivedMap := make(map[string]int)
	grnRows, err := tx.Query(`SELECT items_json FROM goods_receipt_notes WHERE po_id = ? AND org_id = ? AND status = 'completed'`, req.POID, orgID)
	if err == nil {
		for grnRows.Next() {
			var ij string
			if err := grnRows.Scan(&ij); err == nil {
				var pastItems []GRNItem
				if err := json.Unmarshal([]byte(ij), &pastItems); err == nil {
					for _, pi := range pastItems {
						prevReceivedMap[pi.SKUCode] += pi.Quantity
					}
				}
			}
		}
		grnRows.Close()
	}

	// 5. Process This GRN's Received Items & Update Inventory
	now := time.Now()
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			continue
		}
		bin := item.BinLocation
		if bin == "" {
			bin = "A-01-01"
		}

		// Find SKU
		var skuID string
		var skuStock, skuReserved int
		err := tx.QueryRow(`SELECT id, stock_quantity, reserved_quantity FROM skus WHERE code = ? AND org_id = ?`, item.SKUCode, orgID).
			Scan(&skuID, &skuStock, &skuReserved)
		if err != nil {
			return nil, fmt.Errorf("sku code %s not found: %w", item.SKUCode, err)
		}

		// Update SKU total stock and reduce reserved
		newReserved := skuReserved - item.Quantity
		if newReserved < 0 {
			newReserved = 0
		}
		_, err = tx.Exec(`UPDATE skus SET stock_quantity = stock_quantity + ?, reserved_quantity = ?, version = version + 1, updated_at = ?
			WHERE id = ? AND org_id = ?`, item.Quantity, newReserved, now, skuID, orgID)
		if err != nil {
			return nil, fmt.Errorf("failed updating sku inventory: %w", err)
		}

		// Update warehouse-specific stock
		wsID := "ws_" + uuid.New().String()[:8]
		_, err = tx.Exec(`INSERT INTO warehouse_stocks (id, org_id, warehouse_id, sku_id, stock_quantity, reserved_quantity, bin_location, updated_at)
			VALUES (?, ?, ?, ?, ?, 0, ?, ?)
			ON CONFLICT(org_id, warehouse_id, sku_id) DO UPDATE SET
				stock_quantity = stock_quantity + excluded.stock_quantity,
				bin_location = excluded.bin_location,
				updated_at = excluded.updated_at`,
			wsID, orgID, req.WarehouseID, skuID, item.Quantity, bin, now)
		if err != nil {
			return nil, fmt.Errorf("failed updating warehouse stock: %w", err)
		}

		// If batch number is specified, record batch lot
		if item.BatchNumber != "" {
			lotID := "lot_" + uuid.New().String()[:8]
			_, err = tx.Exec(`INSERT INTO batch_lots (id, org_id, sku_id, batch_number, expiry_date, quantity, warehouse_id, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(org_id, sku_id, batch_number) DO UPDATE SET
					quantity = quantity + excluded.quantity`,
				lotID, orgID, skuID, item.BatchNumber, item.ExpiryDate, item.Quantity, req.WarehouseID, now)
			if err != nil {
				return nil, fmt.Errorf("failed recording batch lot: %w", err)
			}
		}

		prevReceivedMap[item.SKUCode] += item.Quantity
	}

	// 6. Check if All Items Are Fully Received or Partial
	isPartial := false
	for code, orderedQty := range orderedMap {
		receivedSoFar := prevReceivedMap[code]
		if receivedSoFar < orderedQty {
			isPartial = true
			break
		}
	}

	newPOStatus := POStatusReceived
	if isPartial {
		newPOStatus = POStatusPartiallyReceived
	}

	// Update PO Status
	newPOVersion := po.Version + 1
	_, err = tx.Exec(`UPDATE purchase_orders SET status = ?, version = ? WHERE id = ? AND org_id = ?`,
		newPOStatus, newPOVersion, po.ID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed updating purchase order status: %w", err)
	}
	po.Status = newPOStatus
	po.Version = newPOVersion

	// 7. Create Goods Receipt Note (GRN)
	grnID := "grn_" + uuid.New().String()[:8]
	grnNum := fmt.Sprintf("GRN-%s-%s", now.Format("0601"), grnID[4:])
	itemsJSONBytes, _ := json.Marshal(req.Items)
	itemsJSON := string(itemsJSONBytes)

	receivedBy := req.ReceivedBy
	if receivedBy == "" {
		receivedBy = "Warehouse Manager"
		if authCtx != nil && authCtx.Name != "" {
			receivedBy = authCtx.Name
		}
	}

	_, err = tx.Exec(`INSERT INTO goods_receipt_notes (id, org_id, grn_number, po_id, warehouse_id, received_by, status, items_json, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'completed', ?, ?, ?)`,
		grnID, orgID, grnNum, po.ID, req.WarehouseID, receivedBy, itemsJSON, req.Notes, now)
	if err != nil {
		return nil, fmt.Errorf("failed inserting GRN: %w", err)
	}

	grn := &db.GoodsReceiptNote{
		ID:            grnID,
		OrgID:         orgID,
		GRNNumber:     grnNum,
		POID:          po.ID,
		PONumber:      po.PONumber,
		WarehouseID:   req.WarehouseID,
		WarehouseName: whName,
		ReceivedBy:    receivedBy,
		Status:        "completed",
		ItemsJSON:     itemsJSON,
		Notes:         req.Notes,
		CreatedAt:     now,
	}

	// 8. Double-entry General Ledger Posting
	var receivedCents int64
	for _, item := range req.Items {
		var cost float64
		_ = tx.QueryRow(`SELECT unit_cost FROM skus WHERE code = ? AND org_id = ?`, item.SKUCode, orgID).Scan(&cost)
		receivedCents += finance.DollarsToCents(cost * float64(item.Quantity))
	}
	if receivedCents > 0 {
		_, _ = s.ledger.PostPOReceipt(ctx, tx, orgID, po.ID, po.PONumber, receivedCents, receivedBy)
	}

	// 9. Outbox & Audit Log
	_ = RecordOutboxEvent(ctx, tx, orgID, "grn_received", map[string]interface{}{
		"grn_id":     grnID,
		"grn_number": grnNum,
		"po_number":  po.PONumber,
		"is_partial": isPartial,
		"po_status":  newPOStatus,
	})

	actorType := "human"
	actorID := "system"
	if authCtx != nil {
		if authCtx.IsAPIKey {
			actorType = "agent"
		}
		actorID = authCtx.Name
	}
	_ = s.store.LogAuditTx(tx, orgID, actorType, actorID, "RECEIVE_GRN", "goods_receipt_notes", grnID,
		fmt.Sprintf("GRN %s received for PO %s at %s. Partial: %v", grnNum, po.PONumber, whName, isPartial), itemsJSON)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed committing GRN transaction: %w", err)
	}

	return &ReceiveGRNResult{
		GRN:           grn,
		PurchaseOrder: &po,
		IsPartial:     isPartial,
	}, nil
}

type ThreeWayMatchResult struct {
	BillID           string  `json:"bill_id"`
	BillNumber       string  `json:"bill_number"`
	POID             string  `json:"po_id"`
	PONumber         string  `json:"po_number"`
	SupplierName     string  `json:"supplier_name"`
	Status           string  `json:"status"` // matched, discrepancy_price, discrepancy_qty, pending_receipt
	POTotal          float64 `json:"po_total"`
	BillTotal        float64 `json:"bill_total"`
	PriceVariance    float64 `json:"price_variance"`
	QuantityOrdered  int     `json:"quantity_ordered"`
	QuantityReceived int     `json:"quantity_received"`
	RequiresApproval bool    `json:"requires_approval"`
	ApprovalID       string  `json:"approval_id,omitempty"`
	Details          string  `json:"details"`
}

// PerformThreeWayMatch automates verification between PO items, GRN received quantities, and Supplier Bill totals
func (s *WorkflowService) PerformThreeWayMatch(ctx context.Context, orgID, billID string, authCtx *auth.AuthContext) (*ThreeWayMatchResult, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Fetch Supplier Bill
	var bill db.SupplierBill
	err = tx.QueryRow(`SELECT b.id, b.org_id, b.bill_number, b.supplier_id, COALESCE(sup.name, ''),
		COALESCE(b.po_id, ''), COALESCE(p.po_number, ''), b.amount, b.status, b.match_status, b.items_json, COALESCE(b.notes, ''), b.created_at
		FROM supplier_bills b
		LEFT JOIN suppliers sup ON b.supplier_id = sup.id
		LEFT JOIN purchase_orders p ON b.po_id = p.id
		WHERE b.id = ? AND b.org_id = ?`, billID, orgID).
		Scan(&bill.ID, &bill.OrgID, &bill.BillNumber, &bill.SupplierID, &bill.SupplierName,
			&bill.POID, &bill.PONumber, &bill.Amount, &bill.Status, &bill.MatchStatus, &bill.ItemsJSON, &bill.Notes, &bill.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("supplier bill %s not found: %w", billID, err)
	}

	if bill.POID == "" {
		return &ThreeWayMatchResult{
			BillID:       bill.ID,
			BillNumber:   bill.BillNumber,
			SupplierName: bill.SupplierName,
			Status:       "unmatched_no_po",
			BillTotal:    bill.Amount,
			Details:      "Bill is not linked to any Purchase Order",
		}, nil
	}

	// 2. Fetch Linked Purchase Order
	var po db.PurchaseOrder
	err = tx.QueryRow(`SELECT id, po_number, total_amount, items_json FROM purchase_orders WHERE id = ? AND org_id = ?`,
		bill.POID, orgID).Scan(&po.ID, &po.PONumber, &po.TotalAmount, &po.ItemsJSON)
	if err != nil {
		return nil, fmt.Errorf("linked PO %s not found: %w", bill.POID, err)
	}

	var poItems []map[string]interface{}
	_ = json.Unmarshal([]byte(po.ItemsJSON), &poItems)
	totalOrderedQty := 0
	for _, item := range poItems {
		q, _ := item["quantity"].(float64)
		totalOrderedQty += int(q)
	}

	// 3. Fetch Linked GRNs
	totalReceivedQty := 0
	grnRows, err := tx.Query(`SELECT items_json FROM goods_receipt_notes WHERE po_id = ? AND org_id = ? AND status = 'completed'`,
		po.ID, orgID)
	grnCount := 0
	if err == nil {
		for grnRows.Next() {
			var ij string
			if err := grnRows.Scan(&ij); err == nil {
				grnCount++
				var grnItems []GRNItem
				if err := json.Unmarshal([]byte(ij), &grnItems); err == nil {
					for _, gi := range grnItems {
						totalReceivedQty += gi.Quantity
					}
				}
			}
		}
		grnRows.Close()
	}

	// 4. Evaluate 3-Way Match Rules
	priceDiff := bill.Amount - po.TotalAmount
	absPriceDiff := math.Abs(priceDiff)
	matchStatus := "matched"
	details := fmt.Sprintf("PO #%s matched. PO: $%.2f, Bill: $%.2f, Ordered Qty: %d, Received Qty: %d",
		po.PONumber, po.TotalAmount, bill.Amount, totalOrderedQty, totalReceivedQty)
	requiresApproval := false
	approvalID := ""

	if grnCount == 0 {
		matchStatus = "pending_receipt"
		details = fmt.Sprintf("Goods have not yet been received in warehouse for PO #%s (0 GRNs)", po.PONumber)
	} else if absPriceDiff > 0.05*po.TotalAmount && absPriceDiff > 10.0 { // Price variance exceeds 5% and $10
		matchStatus = "discrepancy_price"
		requiresApproval = true
		details = fmt.Sprintf("Price variance detected: Bill $%.2f exceeds PO $%.2f by $%.2f (%.1f%%)",
			bill.Amount, po.TotalAmount, priceDiff, (absPriceDiff/po.TotalAmount)*100)
	} else if totalReceivedQty < totalOrderedQty {
		matchStatus = "discrepancy_qty"
		requiresApproval = true
		details = fmt.Sprintf("Quantity shortfall: %d units received out of %d ordered on PO #%s",
			totalReceivedQty, totalOrderedQty, po.PONumber)
	}

	// Update Supplier Bill match_status
	_, err = tx.Exec(`UPDATE supplier_bills SET match_status = ? WHERE id = ? AND org_id = ?`,
		matchStatus, bill.ID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed updating bill match status: %w", err)
	}

	// If discrepancy requires human approval, register an approval record
	if requiresApproval {
		apprID := "appr_" + uuid.New().String()[:8]
		approvalID = apprID
		actorType := "agent"
		actorID := "matching_engine"
		if authCtx != nil {
			actorID = authCtx.Name
		}
		now := time.Now()
		payloadMap := map[string]interface{}{
			"bill_id":           bill.ID,
			"bill_number":       bill.BillNumber,
			"po_id":             po.ID,
			"po_number":         po.PONumber,
			"po_total":          po.TotalAmount,
			"bill_amount":       bill.Amount,
			"price_variance":    priceDiff,
			"ordered_quantity":  totalOrderedQty,
			"received_quantity": totalReceivedQty,
			"discrepancy_type":  matchStatus,
		}
		payloadBytes, _ := json.Marshal(payloadMap)
		payloadJSON := string(payloadBytes)
		payloadHash := auth.HashToken(payloadJSON)

		_, err = tx.Exec(`INSERT INTO approvals (id, org_id, action_type, title, description, payload_json, payload_hash, entity_type, entity_id, entity_version, status, execution_status, requested_by_type, requested_by_id, created_at)
			VALUES (?, ?, 'match_bill_discrepancy', ?, ?, ?, ?, 'supplier_bills', ?, 1, 'pending', 'none', ?, ?, ?)`,
			apprID, orgID,
			fmt.Sprintf("Authorize Bill Variance: %s ($%.2f vs PO $%.2f)", bill.BillNumber, bill.Amount, po.TotalAmount),
			details, payloadJSON, payloadHash, bill.ID, actorType, actorID, now)
		if err != nil {
			return nil, fmt.Errorf("failed creating approval record for bill discrepancy: %w", err)
		}
	}

	actor := "system"
	if authCtx != nil {
		actor = authCtx.Name
	}
	_ = s.store.LogAuditTx(tx, orgID, "human", actor, "THREE_WAY_MATCH", "supplier_bills", bill.ID,
		fmt.Sprintf("3-Way Match executed for Bill %s: %s", bill.BillNumber, matchStatus), details)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed committing 3-way match transaction: %w", err)
	}

	return &ThreeWayMatchResult{
		BillID:           bill.ID,
		BillNumber:       bill.BillNumber,
		POID:             po.ID,
		PONumber:         po.PONumber,
		SupplierName:     bill.SupplierName,
		Status:           matchStatus,
		POTotal:          po.TotalAmount,
		BillTotal:        bill.Amount,
		PriceVariance:    priceDiff,
		QuantityOrdered:  totalOrderedQty,
		QuantityReceived: totalReceivedQty,
		RequiresApproval: requiresApproval,
		ApprovalID:       approvalID,
		Details:          details,
	}, nil
}

type StockTransferItem struct {
	SKUCode  string `json:"sku_code"`
	Quantity int    `json:"quantity"`
}

type CreateStockTransferRequest struct {
	FromWarehouseID string              `json:"from_warehouse_id"`
	ToWarehouseID   string              `json:"to_warehouse_id"`
	Items           []StockTransferItem `json:"items"`
	Notes           string              `json:"notes"`
}

// TransferStock moves physical inventory between warehouses atomically with stock verification
func (s *WorkflowService) TransferStock(ctx context.Context, orgID string, req CreateStockTransferRequest, authCtx *auth.AuthContext) (*db.StockTransfer, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if req.FromWarehouseID == req.ToWarehouseID {
		return nil, fmt.Errorf("cannot transfer stock to the same warehouse")
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("transfer must contain at least one item")
	}

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Verify both warehouses exist
	var wfName, wtName string
	err = tx.QueryRow(`SELECT name FROM warehouses WHERE id = ? AND org_id = ?`, req.FromWarehouseID, orgID).Scan(&wfName)
	if err != nil {
		return nil, fmt.Errorf("source warehouse not found: %w", err)
	}
	err = tx.QueryRow(`SELECT name FROM warehouses WHERE id = ? AND org_id = ?`, req.ToWarehouseID, orgID).Scan(&wtName)
	if err != nil {
		return nil, fmt.Errorf("destination warehouse not found: %w", err)
	}

	now := time.Now()
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			continue
		}
		var skuID string
		err := tx.QueryRow(`SELECT id FROM skus WHERE code = ? AND org_id = ?`, item.SKUCode, orgID).Scan(&skuID)
		if err != nil {
			return nil, fmt.Errorf("sku code %s not found: %w", item.SKUCode, err)
		}

		// Check source stock
		var sourceStock int
		err = tx.QueryRow(`SELECT stock_quantity FROM warehouse_stocks WHERE warehouse_id = ? AND sku_id = ? AND org_id = ?`,
			req.FromWarehouseID, skuID, orgID).Scan(&sourceStock)
		if err != nil || sourceStock < item.Quantity {
			return nil, fmt.Errorf("insufficient stock at %s for SKU %s (available: %d, requested: %d)",
				wfName, item.SKUCode, sourceStock, item.Quantity)
		}

		// Deduct from source warehouse
		_, err = tx.Exec(`UPDATE warehouse_stocks SET stock_quantity = stock_quantity - ?, updated_at = ?
			WHERE warehouse_id = ? AND sku_id = ? AND org_id = ?`,
			item.Quantity, now, req.FromWarehouseID, skuID, orgID)
		if err != nil {
			return nil, fmt.Errorf("failed deducting source warehouse stock: %w", err)
		}

		// Add to destination warehouse
		wsID := "ws_" + uuid.New().String()[:8]
		_, err = tx.Exec(`INSERT INTO warehouse_stocks (id, org_id, warehouse_id, sku_id, stock_quantity, reserved_quantity, bin_location, updated_at)
			VALUES (?, ?, ?, ?, ?, 0, 'A-01-01', ?)
			ON CONFLICT(org_id, warehouse_id, sku_id) DO UPDATE SET
				stock_quantity = stock_quantity + excluded.stock_quantity,
				updated_at = excluded.updated_at`,
			wsID, orgID, req.ToWarehouseID, skuID, item.Quantity, now)
		if err != nil {
			return nil, fmt.Errorf("failed incrementing destination warehouse stock: %w", err)
		}
	}

	// Create Stock Transfer record
	xferID := "xfer_" + uuid.New().String()[:8]
	xferNum := fmt.Sprintf("XFER-%s-%s", now.Format("0601"), xferID[5:])
	itemsJSONBytes, _ := json.Marshal(req.Items)
	itemsJSON := string(itemsJSONBytes)

	createdBy := "system"
	if authCtx != nil {
		createdBy = authCtx.Name
	}

	_, err = tx.Exec(`INSERT INTO stock_transfers (id, org_id, transfer_number, from_warehouse_id, to_warehouse_id, status, items_json, notes, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, 'completed', ?, ?, ?, ?)`,
		xferID, orgID, xferNum, req.FromWarehouseID, req.ToWarehouseID, itemsJSON, req.Notes, createdBy, now)
	if err != nil {
		return nil, fmt.Errorf("failed inserting stock transfer record: %w", err)
	}

	_ = RecordOutboxEvent(ctx, tx, orgID, "stock_transferred", map[string]interface{}{
		"transfer_id":     xferID,
		"transfer_number": xferNum,
		"from_warehouse":  wfName,
		"to_warehouse":    wtName,
	})

	_ = s.store.LogAuditTx(tx, orgID, "human", createdBy, "CREATE_STOCK_TRANSFER", "stock_transfers", xferID,
		fmt.Sprintf("Stock transfer %s from %s to %s completed", xferNum, wfName, wtName), itemsJSON)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed committing stock transfer: %w", err)
	}

	return &db.StockTransfer{
		ID:                xferID,
		OrgID:             orgID,
		TransferNumber:    xferNum,
		FromWarehouseID:   req.FromWarehouseID,
		FromWarehouseName: wfName,
		ToWarehouseID:     req.ToWarehouseID,
		ToWarehouseName:   wtName,
		Status:            "completed",
		ItemsJSON:         itemsJSON,
		Notes:             req.Notes,
		CreatedBy:         createdBy,
		CreatedAt:         now,
	}, nil
}
