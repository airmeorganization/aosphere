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

type POItem struct {
	SKUCode  string  `json:"sku_code"`
	Quantity int     `json:"quantity"`
	UnitCost float64 `json:"unit_cost"`
}

type CreatePORequest struct {
	SupplierID string   `json:"supplier_id"`
	Items      []POItem `json:"items"`
	Notes      string   `json:"notes"`
}

type CreatePOResult struct {
	PurchaseOrder      *db.PurchaseOrder `json:"purchase_order"`
	Approval           *db.Approval      `json:"approval,omitempty"`
	RequiresApproval   bool              `json:"requires_approval"`
	IsIdempotentReplay bool              `json:"is_idempotent_replay"`
}

// CreatePurchaseOrder executes transactional purchase order creation with server-side validation and HITL routing
func (s *WorkflowService) CreatePurchaseOrder(ctx context.Context, orgID string, req CreatePORequest, authCtx *auth.AuthContext, idempotencyKey string) (*CreatePOResult, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("purchase order must contain at least one item")
	}

	payloadBytes, _ := json.Marshal(req)
	payloadStr := string(payloadBytes)

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Idempotency Check
	idempRec, err := s.idemp.GetOrCreate(tx, orgID, idempotencyKey, payloadStr)
	if err != nil {
		return nil, err
	}
	if idempRec != nil && idempRec.Status == "completed" {
		var replay CreatePOResult
		if err := json.Unmarshal([]byte(idempRec.ResponseBody), &replay); err == nil {
			replay.IsIdempotentReplay = true
			return &replay, nil
		}
	}

	// 2. Validate Supplier Reference
	if err := s.policy.ValidateSupplier(tx, orgID, req.SupplierID); err != nil {
		return nil, err
	}

	// 3. Validate SKUs and Calculate Server-Side Totals
	totalAmount := 0.0
	validatedItems := make([]map[string]interface{}, 0, len(req.Items))

	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("item %s has invalid quantity %d", item.SKUCode, item.Quantity)
		}

		var skuID, skuName string
		var catalogCost float64
		err := tx.QueryRow(`SELECT id, name, unit_cost FROM skus WHERE code = ? AND org_id = ?`, item.SKUCode, orgID).
			Scan(&skuID, &skuName, &catalogCost)
		if err != nil {
			return nil, fmt.Errorf("%w: SKU %q not found in organization", ErrInvalidEntity, item.SKUCode)
		}

		unitCost := item.UnitCost
		if unitCost <= 0.0 {
			unitCost = catalogCost
		}

		lineTotal := float64(item.Quantity) * unitCost
		totalAmount += lineTotal

		validatedItems = append(validatedItems, map[string]interface{}{
			"sku_id":     skuID,
			"sku_code":   item.SKUCode,
			"sku_name":   skuName,
			"quantity":   item.Quantity,
			"unit_cost":  unitCost,
			"line_total": lineTotal,
		})
	}

	itemsJSONBytes, _ := json.Marshal(validatedItems)
	itemsJSON := string(itemsJSONBytes)

	// 4. Policy Check: Determine if Human Approval is Required
	isAgent := authCtx != nil && (authCtx.IsAPIKey || authCtx.Role == auth.RoleOperations)
	requiresApproval := s.policy.RequiresApprovalForPO(totalAmount, isAgent)

	poID := "po_" + uuid.New().String()[:8]
	poNum := fmt.Sprintf("PO-%d", time.Now().Unix())
	now := time.Now()

	actorType := "human"
	actorID := "system"
	if authCtx != nil {
		if authCtx.IsAPIKey {
			actorType = "agent"
		}
		actorID = authCtx.Name
	}

	result := &CreatePOResult{
		RequiresApproval: requiresApproval,
	}

	if requiresApproval {
		// Route to Human Approval Inbox
		status := POStatusPendingApproval
		_, err = tx.Exec(`INSERT INTO purchase_orders (id, org_id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes, version, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
			poID, orgID, poNum, req.SupplierID, status, totalAmount, itemsJSON, actorType, actorID, req.Notes, now)
		if err != nil {
			return nil, fmt.Errorf("failed to create pending purchase order: %w", err)
		}

		payloadHash := auth.HashToken(itemsJSON)
		apprID := "appr_" + uuid.New().String()[:8]
		_, err = tx.Exec(`INSERT INTO approvals (id, org_id, action_type, title, description, payload_json, payload_hash, entity_type, entity_id, entity_version, status, execution_status, requested_by_type, requested_by_id, created_at)
			VALUES (?, ?, 'create_po', ?, ?, ?, ?, 'purchase_orders', ?, 1, 'pending', 'none', ?, ?, ?)`,
			apprID, orgID,
			fmt.Sprintf("Authorize Purchase Order %s ($%.2f)", poNum, totalAmount),
			fmt.Sprintf("Order of %d items totaling $%.2f exceeds safety limit of $%.2f. Pending approval.", len(req.Items), totalAmount, s.policy.POApprovalThreshold),
			itemsJSON, payloadHash, poID, actorType, actorID, now)
		if err != nil {
			return nil, fmt.Errorf("failed to create approval record: %w", err)
		}

		result.PurchaseOrder = &db.PurchaseOrder{
			ID:            poID,
			OrgID:         orgID,
			PONumber:      poNum,
			SupplierID:    req.SupplierID,
			Status:        status,
			TotalAmount:   totalAmount,
			ItemsJSON:     itemsJSON,
			CreatedByType: actorType,
			ActorID:       actorID,
			Notes:         req.Notes,
			Version:       1,
			CreatedAt:     now,
		}
		result.Approval = &db.Approval{
			ID:              apprID,
			OrgID:           orgID,
			ActionType:      "create_po",
			Title:           fmt.Sprintf("Authorize Purchase Order %s ($%.2f)", poNum, totalAmount),
			PayloadJSON:     itemsJSON,
			PayloadHash:     payloadHash,
			EntityType:      "purchase_orders",
			EntityID:        poID,
			EntityVersion:   1,
			Status:          "pending",
			ExecutionStatus: "none",
			RequestedByType: actorType,
			RequestedByID:   actorID,
			CreatedAt:       now,
		}

		_ = s.store.LogAuditForOrg(orgID, actorType, actorID, "CREATE_PURCHASE_ORDER_PENDING", "purchase_orders", poID,
			fmt.Sprintf("PO %s created pending approval ($%.2f > $%.2f)", poNum, totalAmount, s.policy.POApprovalThreshold), itemsJSON)
	} else {
		// Directly ordered
		status := POStatusOrdered
		_, err = tx.Exec(`INSERT INTO purchase_orders (id, org_id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes, version, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
			poID, orgID, poNum, req.SupplierID, status, totalAmount, itemsJSON, actorType, actorID, req.Notes, now)
		if err != nil {
			return nil, fmt.Errorf("failed to create purchase order: %w", err)
		}

		// Update reserved stock for ordered SKUs
		for _, item := range validatedItems {
			skuID := item["sku_id"].(string)
			qty := item["quantity"].(int)
			_, err = tx.Exec(`UPDATE skus SET reserved_quantity = reserved_quantity + ?, version = version + 1 WHERE id = ? AND org_id = ?`,
				qty, skuID, orgID)
			if err != nil {
				return nil, fmt.Errorf("failed to update reserved inventory for sku %s: %w", skuID, err)
			}
		}

		result.PurchaseOrder = &db.PurchaseOrder{
			ID:            poID,
			OrgID:         orgID,
			PONumber:      poNum,
			SupplierID:    req.SupplierID,
			Status:        status,
			TotalAmount:   totalAmount,
			ItemsJSON:     itemsJSON,
			CreatedByType: actorType,
			ActorID:       actorID,
			Notes:         req.Notes,
			Version:       1,
			CreatedAt:     now,
		}

		_ = s.store.LogAuditForOrg(orgID, actorType, actorID, "CREATE_PURCHASE_ORDER", "purchase_orders", poID,
			fmt.Sprintf("PO %s placed directly for $%.2f", poNum, totalAmount), itemsJSON)
	}

	// 5. Save Idempotency Result
	if idempotencyKey != "" {
		resBytes, _ := json.Marshal(result)
		_ = s.idemp.Complete(tx, orgID, idempotencyKey, 201, string(resBytes))
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit purchase order transaction: %w", err)
	}

	return result, nil
}

// ReceivePurchaseOrder records delivery of ordered goods and updates stock levels atomically
func (s *WorkflowService) ReceivePurchaseOrder(ctx context.Context, orgID, poID string, authCtx *auth.AuthContext, idempotencyKey string) (*db.PurchaseOrder, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Idempotency check
	idempRec, err := s.idemp.GetOrCreate(tx, orgID, idempotencyKey, poID)
	if err != nil {
		return nil, err
	}
	if idempRec != nil && idempRec.Status == "completed" {
		var replay db.PurchaseOrder
		if err := json.Unmarshal([]byte(idempRec.ResponseBody), &replay); err == nil {
			return &replay, nil
		}
	}

	var po db.PurchaseOrder
	err = tx.QueryRow(`SELECT id, org_id, po_number, supplier_id, status, total_amount, items_json, created_by_type, actor_id, notes, version, created_at 
		FROM purchase_orders WHERE id = ? AND org_id = ?`, poID, orgID).
		Scan(&po.ID, &po.OrgID, &po.PONumber, &po.SupplierID, &po.Status, &po.TotalAmount, &po.ItemsJSON,
			&po.CreatedByType, &po.ActorID, &po.Notes, &po.Version, &po.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("purchase order %s not found: %w", poID, err)
	}

	if err := ValidatePOTransition(po.Status, POStatusReceived); err != nil {
		return nil, err
	}

	now := time.Now()
	newVersion := po.Version + 1
	_, err = tx.Exec(`UPDATE purchase_orders SET status = ?, version = ? WHERE id = ? AND org_id = ? AND version = ?`,
		POStatusReceived, newVersion, poID, orgID, po.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to update purchase order status: %w", err)
	}

	// Adjust inventory quantities: increase stock, release reserved quantity
	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(po.ItemsJSON), &items); err == nil {
		for _, item := range items {
			skuCode, _ := item["sku_code"].(string)
			qtyFloat, _ := item["quantity"].(float64)
			qty := int(qtyFloat)

			_, err = tx.Exec(`UPDATE skus SET stock_quantity = stock_quantity + ?, reserved_quantity = MAX(0, reserved_quantity - ?),
				version = version + 1, updated_at = ? WHERE code = ? AND org_id = ?`,
				qty, qty, now, skuCode, orgID)
			if err != nil {
				return nil, fmt.Errorf("failed to update inventory on PO receipt: %w", err)
			}
		}
	}

	actor := "system"
	if authCtx != nil {
		actor = authCtx.Name
	}
	_ = s.store.LogAuditForOrg(orgID, "human", actor, "RECEIVE_PURCHASE_ORDER", "purchase_orders", poID,
		fmt.Sprintf("PO %s received. Stock levels replenished.", po.PONumber), po.ItemsJSON)

	po.Status = POStatusReceived
	po.Version = newVersion

	if idempotencyKey != "" {
		resBytes, _ := json.Marshal(po)
		_ = s.idemp.Complete(tx, orgID, idempotencyKey, 200, string(resBytes))
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit receive order transaction: %w", err)
	}

	return &po, nil
}
