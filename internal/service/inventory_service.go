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

// AdjustStock applies an inventory adjustment with optimistic locking, atomic bounds checks, and deduplicated reorders
func (s *WorkflowService) AdjustStock(ctx context.Context, orgID, skuID string, delta int, reason string, authCtx *auth.AuthContext, expectedVersion int, idempotencyKey string) (*db.SKU, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Idempotency Check
	idempPayload := fmt.Sprintf("%s:%d:%s:%d", skuID, delta, reason, expectedVersion)
	idempRec, err := s.idemp.GetOrCreate(tx, orgID, idempotencyKey, idempPayload)
	if err != nil {
		return nil, err
	}
	if idempRec != nil && idempRec.Status == "completed" {
		var replay db.SKU
		if err := json.Unmarshal([]byte(idempRec.ResponseBody), &replay); err == nil {
			return &replay, nil
		}
	}

	// 2. Fetch SKU & Validate Optimistic Version
	var sku db.SKU
	err = tx.QueryRow(`SELECT id, org_id, code, name, category, stock_quantity, reserved_quantity, min_threshold, reorder_quantity, unit_cost, selling_price, COALESCE(supplier_id, ''), version 
		FROM skus WHERE id = ? AND org_id = ?`, skuID, orgID).
		Scan(&sku.ID, &sku.OrgID, &sku.Code, &sku.Name, &sku.Category, &sku.StockQuantity, &sku.ReservedQuantity,
			&sku.MinThreshold, &sku.ReorderQuantity, &sku.UnitCost, &sku.SellingPrice, &sku.SupplierID, &sku.Version)
	if err != nil {
		return nil, fmt.Errorf("sku %s not found: %w", skuID, err)
	}

	if expectedVersion > 0 && sku.Version != expectedVersion {
		return nil, fmt.Errorf("%w: sku current version is %d, expected %d", ErrStaleVersion, sku.Version, expectedVersion)
	}

	newStock := sku.StockQuantity + delta
	if newStock < 0 {
		return nil, fmt.Errorf("%w: attempted delta %d from current stock %d", ErrInsufficientStock, delta, sku.StockQuantity)
	}

	now := time.Now()
	newVersion := sku.Version + 1
	_, err = tx.Exec(`UPDATE skus SET stock_quantity = ?, version = ?, updated_at = ? WHERE id = ? AND org_id = ? AND version = ?`,
		newStock, newVersion, now, skuID, orgID, sku.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to update stock quantity: %w", err)
	}

	actorType := "human"
	actorID := "system"
	if authCtx != nil {
		if authCtx.IsAPIKey {
			actorType = "agent"
		}
		actorID = authCtx.Name
	}

	// 3. Automated Reorder Evaluation with Duplicate Prevention
	if newStock <= sku.MinThreshold {
		hasOpen, err := s.policy.HasOpenReorderForSKU(tx, orgID, sku.Code)
		if err != nil {
			return nil, err
		}

		if hasOpen {
			// Suppress duplicate: an active PO or pending approval already exists!
			_ = s.store.LogAuditTx(tx, orgID, "agent", "workflow_engine", "AUTO_REORDER_SUPPRESSED", "skus", skuID,
				fmt.Sprintf("Stock %d <= threshold %d, but open order/approval already exists for SKU %s", newStock, sku.MinThreshold, sku.Code), "{}")
		} else {
			// Trigger new reorder
			totalAmount := float64(sku.ReorderQuantity) * sku.UnitCost
			itemsJSONBytes, _ := json.Marshal([]map[string]interface{}{
				{"sku_id": sku.ID, "sku_code": sku.Code, "sku_name": sku.Name, "quantity": sku.ReorderQuantity, "unit_cost": sku.UnitCost},
			})
			itemsJSON := string(itemsJSONBytes)

			poID := "po_" + uuid.New().String()[:8]
			poNum, err := finance.GenerateSequenceNumber(ctx, tx, orgID, "PO")
			if err != nil {
				return nil, fmt.Errorf("failed to generate sequential PO number for auto-reorder: %w", err)
			}

			var supID interface{} = sku.SupplierID
			if sku.SupplierID == "" {
				supID = nil
			}

			if totalAmount > s.policy.POApprovalThreshold {
				// Above safety threshold: Route to HITL Approval
				_, err = tx.Exec(`INSERT INTO purchase_orders (id, org_id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes, version, created_at)
					VALUES (?, ?, ?, ?, 'pending_approval', ?, ?, 'agent', 'workflow_engine', 'Automated low stock reorder. Pending human approval.', 1, ?)`,
					poID, orgID, poNum, supID, totalAmount, itemsJSON, now)
				if err != nil {
					return nil, fmt.Errorf("failed to create automated PO: %w", err)
				}

				payloadHash := auth.HashToken(itemsJSON)
				apprID := "appr_" + uuid.New().String()[:8]
				_, err = tx.Exec(`INSERT INTO approvals (id, org_id, action_type, title, description, payload_json, payload_hash, entity_type, entity_id, entity_version, status, execution_status, requested_by_type, requested_by_id, created_at)
					VALUES (?, ?, 'create_po', ?, ?, ?, ?, 'purchase_orders', ?, 1, 'pending', 'none', 'agent', 'workflow_engine', ?)`,
					apprID, orgID,
					fmt.Sprintf("Auto PO for %s (Low Stock: %d)", sku.Code, newStock),
					fmt.Sprintf("Stock dropped to %d (Min: %d). Reorder quantity: %d units ($%.2f total).", newStock, sku.MinThreshold, sku.ReorderQuantity, totalAmount),
					itemsJSON, payloadHash, poID, now)
				if err != nil {
					return nil, fmt.Errorf("failed to create automated approval: %w", err)
				}
			} else {
				// Under threshold: Auto-draft
				_, err = tx.Exec(`INSERT INTO purchase_orders (id, org_id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes, version, created_at)
					VALUES (?, ?, ?, ?, 'draft', ?, ?, 'agent', 'workflow_engine', 'Automated low stock draft under threshold.', 1, ?)`,
					poID, orgID, poNum, supID, totalAmount, itemsJSON, now)
				if err != nil {
					return nil, fmt.Errorf("failed to create automated PO draft: %w", err)
				}
			}

			_ = s.store.LogAuditTx(tx, orgID, "agent", "workflow_engine", "AUTO_REORDER_TRIGGERED", "skus", skuID,
				fmt.Sprintf("Stock %d <= threshold %d. Triggered PO %s for $%.2f", newStock, sku.MinThreshold, poNum, totalAmount), itemsJSON)
		}
	}

	_ = s.store.LogAuditTx(tx, orgID, actorType, actorID, "STOCK_UPDATE", "skus", skuID,
		fmt.Sprintf("Stock delta: %d, new quantity: %d. Reason: %s", delta, newStock, reason), "{}")

	sku.StockQuantity = newStock
	sku.Version = newVersion

	if idempotencyKey != "" {
		resBytes, _ := json.Marshal(sku)
		_ = s.idemp.Complete(tx, orgID, idempotencyKey, 200, string(resBytes))
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit stock adjustment transaction: %w", err)
	}

	return &sku, nil
}
