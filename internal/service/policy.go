package service

import (
	"database/sql"
	"fmt"
	"strings"

	"aosphere/internal/auth"
)

type PolicyEngine struct {
	POApprovalThreshold float64
}

func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{
		POApprovalThreshold: 1000.0, // Orders over $1,000 require human sign-off
	}
}

// RequiresApprovalForPO determines if a purchase order must pass through the Human-In-The-Loop inbox
func (p *PolicyEngine) RequiresApprovalForPO(total float64, isAgent bool) bool {
	// Financial threshold: require human sign-off on transactions over $1,000
	if total > p.POApprovalThreshold {
		return true
	}
	return false
}

// CanDecideApproval enforces that only authorized human personnel can decide HITL approvals
func (p *PolicyEngine) CanDecideApproval(authCtx *auth.AuthContext) error {
	if authCtx == nil {
		return fmt.Errorf("%w: authentication required", ErrUnauthorizedAction)
	}

	// Agents are strictly prohibited from approving their own or other agent actions
	if authCtx.IsAPIKey {
		return fmt.Errorf("%w: AI agents cannot authorize human approval requests", ErrUnauthorizedAction)
	}

	// Caller must possess the approve_orders capability (Owner, Admin, Finance)
	if !authCtx.Can(auth.PermApproveOrders) {
		return fmt.Errorf("%w: role %q lacks permission to approve business actions", ErrUnauthorizedAction, authCtx.Role)
	}

	return nil
}

// HasOpenReorderForSKU checks if there is already an active order or pending approval for this SKU
func (p *PolicyEngine) HasOpenReorderForSKU(tx *sql.Tx, orgID, skuCode string) (bool, error) {
	// 1. Check open purchase orders (draft, pending_approval, ordered)
	pattern := "%" + skuCode + "%"
	var count int
	err := tx.QueryRow(`SELECT COUNT(*) FROM purchase_orders 
		WHERE org_id = ? AND status IN ('draft', 'pending_approval', 'ordered') 
		AND items_json LIKE ?`, orgID, pattern).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check active POs for SKU %s: %w", skuCode, err)
	}
	if count > 0 {
		return true, nil
	}

	// 2. Check pending approvals
	var apprCount int
	err = tx.QueryRow(`SELECT COUNT(*) FROM approvals
		WHERE org_id = ? AND status = 'pending' AND action_type = 'create_po'
		AND payload_json LIKE ?`, orgID, pattern).Scan(&apprCount)
	if err != nil {
		return false, fmt.Errorf("failed to check pending approvals for SKU %s: %w", skuCode, err)
	}

	return apprCount > 0, nil
}

// ValidateSupplier checks if the given supplier exists within the tenant organization
func (p *PolicyEngine) ValidateSupplier(tx *sql.Tx, orgID, supplierID string) error {
	if strings.TrimSpace(supplierID) == "" {
		return fmt.Errorf("%w: supplier_id cannot be empty", ErrInvalidEntity)
	}
	var exists int
	err := tx.QueryRow(`SELECT 1 FROM suppliers WHERE id = ? AND org_id = ?`, supplierID, orgID).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("%w: supplier %q does not exist in organization", ErrInvalidEntity, supplierID)
		}
		return err
	}
	return nil
}

// ValidateCustomer checks if the given customer exists within the tenant organization
func (p *PolicyEngine) ValidateCustomer(tx *sql.Tx, orgID, customerID string) error {
	if strings.TrimSpace(customerID) == "" {
		return fmt.Errorf("%w: customer_id cannot be empty", ErrInvalidEntity)
	}
	var exists int
	err := tx.QueryRow(`SELECT 1 FROM customers WHERE id = ? AND org_id = ?`, customerID, orgID).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("%w: customer %q does not exist in organization", ErrInvalidEntity, customerID)
		}
		return err
	}
	return nil
}
