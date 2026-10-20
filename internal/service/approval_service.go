package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/db"
)

// DecideApproval processes an authorized human decision on a pending HITL approval
func (s *WorkflowService) DecideApproval(ctx context.Context, orgID, approvalID, decision string, authCtx *auth.AuthContext, idempotencyKey string) (*db.Approval, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if decision != ApprovalStatusApproved && decision != ApprovalStatusRejected {
		return nil, fmt.Errorf("decision must be either 'approved' or 'rejected'")
	}

	// 1. Policy check: Ensure human authorization and approval permission
	if err := s.policy.CanDecideApproval(authCtx); err != nil {
		return nil, err
	}

	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 2. Idempotency Check
	idempRec, err := s.idemp.GetOrCreate(tx, orgID, idempotencyKey, approvalID+":"+decision)
	if err != nil {
		return nil, err
	}
	if idempRec != nil && idempRec.Status == "completed" {
		var replay db.Approval
		if err := json.Unmarshal([]byte(idempRec.ResponseBody), &replay); err == nil {
			return &replay, nil
		}
	}

	// 3. Retrieve Pending Approval
	var appr db.Approval
	err = tx.QueryRow(`SELECT id, org_id, action_type, title, description, payload_json, payload_hash, entity_type, entity_id, entity_version, status, execution_status, execution_error, requested_by_type, requested_by_id, created_at 
		FROM approvals WHERE id = ? AND org_id = ?`, approvalID, orgID).
		Scan(&appr.ID, &appr.OrgID, &appr.ActionType, &appr.Title, &appr.Description, &appr.PayloadJSON,
			&appr.PayloadHash, &appr.EntityType, &appr.EntityID, &appr.EntityVersion, &appr.Status,
			&appr.ExecutionStatus, &appr.ExecutionError, &appr.RequestedByType, &appr.RequestedByID, &appr.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("approval %s not found: %w", approvalID, err)
	}

	if appr.Status != ApprovalStatusPending {
		return nil, fmt.Errorf("%w (current status: %s)", ErrApprovalAlreadyDecided, appr.Status)
	}

	now := time.Now()
	decidedBy := authCtx.Email

	// 4. Atomic conditional status update ensuring exact 'pending' state
	res, err := tx.Exec(`UPDATE approvals 
		SET status = ?, execution_status = 'executing', decided_by = ?, decided_at = ?
		WHERE id = ? AND org_id = ? AND status = 'pending'`,
		decision, decidedBy, now, approvalID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to update approval status: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return nil, ErrApprovalAlreadyDecided
	}

	executionStatus := ExecutionStatusCompleted
	var executionError string

	// 5. Execute Underlying Business Action Idempotently
	if decision == ApprovalStatusApproved {
		switch appr.ActionType {
		case "create_po":
			if appr.EntityID != "" {
				// Verify PO entity version hasn't drifted
				var currentVersion int
				var currentStatus string
				var itemsJSON string
				err := tx.QueryRow(`SELECT version, status, items_json FROM purchase_orders WHERE id = ? AND org_id = ?`,
					appr.EntityID, orgID).Scan(&currentVersion, &currentStatus, &itemsJSON)
				if err != nil {
					executionStatus = ExecutionStatusFailed
					executionError = fmt.Sprintf("underlying PO %s not found", appr.EntityID)
				} else if currentVersion != appr.EntityVersion {
					executionStatus = ExecutionStatusFailed
					executionError = fmt.Sprintf("%v: PO version %d does not match approved version %d",
						ErrStaleVersion, currentVersion, appr.EntityVersion)
				} else if currentStatus != POStatusPendingApproval {
					executionStatus = ExecutionStatusFailed
					executionError = fmt.Sprintf("PO is in status %q, expected %q", currentStatus, POStatusPendingApproval)
				} else {
					// Transition PO to 'ordered'
					newPOVersion := currentVersion + 1
					_, err = tx.Exec(`UPDATE purchase_orders SET status = ?, version = ? WHERE id = ? AND org_id = ?`,
						POStatusOrdered, newPOVersion, appr.EntityID, orgID)
					if err != nil {
						executionStatus = ExecutionStatusFailed
						executionError = fmt.Sprintf("failed to update PO status to ordered: %v", err)
					} else {
						// Reserve inventory for approved order
						var items []map[string]interface{}
						if err := json.Unmarshal([]byte(itemsJSON), &items); err == nil {
							for _, it := range items {
								skuCode, _ := it["sku_code"].(string)
								qtyFloat, _ := it["quantity"].(float64)
								qty := int(qtyFloat)
								_, _ = tx.Exec(`UPDATE skus SET reserved_quantity = reserved_quantity + ?, version = version + 1 
									WHERE code = ? AND org_id = ?`, qty, skuCode, orgID)
							}
						}
					}
				}
			}

		case "payroll_run":
			// Approve timesheets
			_, _ = tx.Exec(`UPDATE timesheets SET status = 'approved' WHERE org_id = ? AND status = 'logged'`, orgID)

		case "public_review_reply":
			// Publish drafted review reply
			var reviewID string
			var payload struct {
				ReviewID string `json:"review_id"`
			}
			if err := json.Unmarshal([]byte(appr.PayloadJSON), &payload); err == nil && payload.ReviewID != "" {
				reviewID = payload.ReviewID
				_, _ = tx.Exec(`UPDATE reviews SET reply_status = 'published' WHERE id = ? AND org_id = ?`, reviewID, orgID)
			}
		}
	} else {
		// Decision is REJECTED
		if appr.ActionType == "create_po" && appr.EntityID != "" {
			_, _ = tx.Exec(`UPDATE purchase_orders SET status = ?, version = version + 1 WHERE id = ? AND org_id = ?`,
				POStatusCancelled, appr.EntityID, orgID)
		}
	}

	// 6. Update Final Execution Status
	_, err = tx.Exec(`UPDATE approvals SET execution_status = ?, execution_error = ? WHERE id = ? AND org_id = ?`,
		executionStatus, executionError, approvalID, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to finalize approval execution status: %w", err)
	}

	appr.Status = decision
	appr.ExecutionStatus = executionStatus
	appr.ExecutionError = executionError
	appr.DecidedBy = &decidedBy
	appr.DecidedAt = &now

	_ = s.store.LogAuditForOrg(orgID, "human", decidedBy, "DECIDE_APPROVAL", "approvals", approvalID,
		fmt.Sprintf("Decision: %s (Execution: %s)", decision, executionStatus), appr.PayloadJSON)

	// 7. Save Idempotency Result
	if idempotencyKey != "" {
		resBytes, _ := json.Marshal(appr)
		_ = s.idemp.Complete(tx, orgID, idempotencyKey, 200, string(resBytes))
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit approval transaction: %w", err)
	}

	return &appr, nil
}
