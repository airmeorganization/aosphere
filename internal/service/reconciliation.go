package service

import (
	"context"
	"fmt"
)

type Discrepancy struct {
	Type        string `json:"type"`
	EntityID    string `json:"entity_id"`
	Description string `json:"description"`
}

type ReconciliationReport struct {
	TotalChecked  int           `json:"total_checked"`
	Discrepancies []Discrepancy `json:"discrepancies"`
}

// ReconcileDiscrepancies inspects system records for orphaned approvals, unlinked POs, or inventory reservation mismatches
func (s *WorkflowService) ReconcileDiscrepancies(ctx context.Context, orgID string) (*ReconciliationReport, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	report := &ReconciliationReport{
		Discrepancies: make([]Discrepancy, 0),
	}

	// 1. Check for Approved Approvals with Incomplete Execution
	rows, err := s.store.DB().QueryContext(ctx, `SELECT id, action_type, entity_id, execution_status, execution_error 
		FROM approvals WHERE org_id = ? AND status = 'approved' AND execution_status != 'completed'`, orgID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, actionType, entityID, execStatus, execErr string
			if err := rows.Scan(&id, &actionType, &entityID, &execStatus, &execErr); err == nil {
				report.Discrepancies = append(report.Discrepancies, Discrepancy{
					Type:     "APPROVAL_EXECUTION_STALLED",
					EntityID: id,
					Description: fmt.Sprintf("Approval %s is approved but execution_status is %q (Entity %s): %s",
						id, execStatus, entityID, execErr),
				})
			}
			report.TotalChecked++
		}
	}

	// 2. Check for Pending Purchase Orders without an Active Approval
	poRows, err := s.store.DB().QueryContext(ctx, `SELECT po.id, po.po_number 
		FROM purchase_orders po 
		WHERE po.org_id = ? AND po.status = 'pending_approval' 
		AND NOT EXISTS (
			SELECT 1 FROM approvals a 
			WHERE a.org_id = po.org_id AND a.entity_id = po.id AND a.status = 'pending'
		)`, orgID)
	if err == nil {
		defer poRows.Close()
		for poRows.Next() {
			var poID, poNum string
			if err := poRows.Scan(&poID, &poNum); err == nil {
				report.Discrepancies = append(report.Discrepancies, Discrepancy{
					Type:        "ORPHAN_PENDING_PO",
					EntityID:    poID,
					Description: fmt.Sprintf("Purchase order %s (%s) is pending_approval but has no open approval inbox record", poNum, poID),
				})
			}
			report.TotalChecked++
		}
	}

	// 3. Check for Over-Reserved SKU Inventory
	skuRows, err := s.store.DB().QueryContext(ctx, `SELECT id, code, stock_quantity, reserved_quantity 
		FROM skus WHERE org_id = ? AND reserved_quantity > stock_quantity`, orgID)
	if err == nil {
		defer skuRows.Close()
		for skuRows.Next() {
			var skuID, code string
			var stock, reserved int
			if err := skuRows.Scan(&skuID, &code, &stock, &reserved); err == nil {
				report.Discrepancies = append(report.Discrepancies, Discrepancy{
					Type:        "OVER_RESERVED_INVENTORY",
					EntityID:    skuID,
					Description: fmt.Sprintf("SKU %s (%s) has reserved_quantity (%d) exceeding total stock (%d)", code, skuID, reserved, stock),
				})
			}
			report.TotalChecked++
		}
	}

	if len(report.Discrepancies) > 0 {
		_ = s.store.LogAuditForOrg(orgID, "system", "reconciliation_engine", "RECONCILIATION_ANOMALIES_DETECTED",
			"system", "health", fmt.Sprintf("Found %d anomalies during reconciliation", len(report.Discrepancies)), "{}")
	}

	return report, nil
}
