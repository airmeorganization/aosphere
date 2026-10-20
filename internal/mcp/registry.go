package mcp

import (
	"fmt"

	"aosphere/internal/auth"
)

type ToolCategory string

const (
	CategoryReadOnly           ToolCategory = "read_only"
	CategoryReversibleMutation ToolCategory = "reversible_mutation"
	CategoryHighImpactMutation ToolCategory = "high_impact_mutation"
)

type ToolDefinition struct {
	Name               string                 `json:"name"`
	Category           ToolCategory           `json:"category"`
	Description        string                 `json:"description"`
	RequiredPermission auth.Permission        `json:"required_permission"`
	InputSchema        map[string]interface{} `json:"inputSchema"`
	OutputSchema       map[string]interface{} `json:"outputSchema,omitempty"`
}

var Registry = map[string]ToolDefinition{
	"erp_check_stock": {
		Name:               "erp_check_stock",
		Category:           CategoryReadOnly,
		Description:        "Inspect stock levels, reserved quantities, and threshold alerts for SKUs.",
		RequiredPermission: auth.PermRead,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"below_threshold_only": map[string]interface{}{
					"type":        "boolean",
					"description": "If true, only returns items where stock_quantity <= min_threshold",
				},
			},
		},
	},
	"erp_adjust_stock": {
		Name:               "erp_adjust_stock",
		Category:           CategoryHighImpactMutation,
		Description:        "Adjust SKU inventory quantity with optimistic locking. If stock drops below threshold, automatically triggers reorder workflows.",
		RequiredPermission: auth.PermManageInventory,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"sku_id":           map[string]interface{}{"type": "string"},
				"delta":            map[string]interface{}{"type": "integer", "description": "Positive to add stock, negative to consume/sell"},
				"reason":           map[string]interface{}{"type": "string"},
				"expected_version": map[string]interface{}{"type": "integer", "description": "Optional version for optimistic concurrency control"},
				"idempotency_key":  map[string]interface{}{"type": "string", "description": "Unique request key for safe retries"},
			},
			"required": []string{"sku_id", "delta", "reason"},
		},
	},
	"erp_create_purchase_order": {
		Name:               "erp_create_purchase_order",
		Category:           CategoryHighImpactMutation,
		Description:        "Create a purchase order for a supplier with server-side pricing and entity verification. Orders > $1,000 route to human approval.",
		RequiredPermission: auth.PermManageOrders,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"supplier_id": map[string]interface{}{"type": "string"},
				"items": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"sku_code":  map[string]interface{}{"type": "string"},
							"quantity":  map[string]interface{}{"type": "integer"},
							"unit_cost": map[string]interface{}{"type": "number"},
						},
						"required": []string{"sku_code", "quantity", "unit_cost"},
					},
				},
				"notes":           map[string]interface{}{"type": "string"},
				"idempotency_key": map[string]interface{}{"type": "string", "description": "Unique request key for safe retries"},
			},
			"required": []string{"supplier_id", "items"},
		},
	},
	"crm_manage_deal": {
		Name:               "crm_manage_deal",
		Category:           CategoryReversibleMutation,
		Description:        "Update deal stage (lead, qualified, proposal, negotiation, won, lost) or log client notes.",
		RequiredPermission: auth.PermManageOrders,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"deal_id": map[string]interface{}{"type": "string"},
				"stage":   map[string]interface{}{"type": "string", "enum": []string{"lead", "qualified", "proposal", "negotiation", "won", "lost"}},
			},
			"required": []string{"deal_id", "stage"},
		},
	},
	"fin_get_summary": {
		Name:               "fin_get_summary",
		Category:           CategoryReadOnly,
		Description:        "Get real-time Profit & Loss (P&L), unpaid client receivables, and financial health.",
		RequiredPermission: auth.PermRead,
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	},
	"fin_create_invoice": {
		Name:               "fin_create_invoice",
		Category:           CategoryHighImpactMutation,
		Description:        "Generate a compliant client invoice with line items, tax, and optional recurring schedule.",
		RequiredPermission: auth.PermManageFinance,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"customer_id": map[string]interface{}{"type": "string"},
				"items": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"desc":  map[string]interface{}{"type": "string"},
							"qty":   map[string]interface{}{"type": "integer"},
							"price": map[string]interface{}{"type": "number"},
						},
						"required": []string{"desc", "qty", "price"},
					},
				},
				"due_date":        map[string]interface{}{"type": "string", "description": "YYYY-MM-DD"},
				"recurring":       map[string]interface{}{"type": "string", "enum": []string{"none", "monthly", "quarterly", "annual"}},
				"idempotency_key": map[string]interface{}{"type": "string", "description": "Unique request key for safe retries"},
			},
			"required": []string{"customer_id", "items", "due_date"},
		},
	},
	"fin_trial_balance": {
		Name:               "fin_trial_balance",
		Category:           CategoryReadOnly,
		Description:        "Inspect the General Ledger trial balance, showing debits, credits, and balance verification across all accounts.",
		RequiredPermission: auth.PermRead,
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	},
	"presence_sync_nap": {
		Name:               "presence_sync_nap",
		Category:           CategoryReversibleMutation,
		Description:        "Synchronize official business Name, Address, Phone (NAP) across search engines and directories.",
		RequiredPermission: auth.PermManageInventory,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"business_name": map[string]interface{}{"type": "string"},
				"address":       map[string]interface{}{"type": "string"},
				"phone":         map[string]interface{}{"type": "string"},
				"website":       map[string]interface{}{"type": "string"},
			},
			"required": []string{"business_name", "address", "phone", "website"},
		},
	},
	"reputation_draft_reply": {
		Name:               "reputation_draft_reply",
		Category:           CategoryReadOnly,
		Description:        "Analyze review sentiment and draft an empathetic response for human review.",
		RequiredPermission: auth.PermRead,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"review_id":  map[string]interface{}{"type": "string"},
				"reply_text": map[string]interface{}{"type": "string"},
			},
			"required": []string{"review_id", "reply_text"},
		},
	},
	"approvals_list_pending": {
		Name:               "approvals_list_pending",
		Category:           CategoryReadOnly,
		Description:        "View actions waiting for human authorization in the Approval Inbox.",
		RequiredPermission: auth.PermApproveOrders,
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	},
	"sys_reconcile": {
		Name:               "sys_reconcile",
		Category:           CategoryReadOnly,
		Description:        "Run business workflow reconciliation scan to detect orphan approvals, unlinked pending POs, and inventory reservation anomalies.",
		RequiredPermission: auth.PermRead,
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	},
	"sys_backup": {
		Name:               "sys_backup",
		Category:           CategoryHighImpactMutation,
		Description:        "Trigger an atomic online SQLite snapshot backup and run integrity check verification.",
		RequiredPermission: auth.PermAdmin,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"prefix": map[string]interface{}{
					"type":        "string",
					"description": "Optional filename prefix for the backup",
				},
			},
		},
	},
	"crm_convert_deal": {
		Name:               "crm_convert_deal",
		Category:           CategoryHighImpactMutation,
		Description:        "Converts an accepted sales deal into an official customer invoice with line items and financial tracking.",
		RequiredPermission: auth.PermManageFinance,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"deal_id":         map[string]interface{}{"type": "string"},
				"idempotency_key": map[string]interface{}{"type": "string", "description": "Unique request key for safe retries"},
			},
			"required": []string{"deal_id"},
		},
	},
	"erp_pay_purchase_order": {
		Name:               "erp_pay_purchase_order",
		Category:           CategoryHighImpactMutation,
		Description:        "Records vendor payment for a received Purchase Order and posts balancing entries to Accounts Payable and Cash ledger accounts.",
		RequiredPermission: auth.PermManageFinance,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"po_id":           map[string]interface{}{"type": "string"},
				"idempotency_key": map[string]interface{}{"type": "string", "description": "Unique request key for safe retries"},
			},
			"required": []string{"po_id"},
		},
	},
	"hr_lock_payroll": {
		Name:               "hr_lock_payroll",
		Category:           CategoryHighImpactMutation,
		Description:        "Locks all approved timesheets for a billing period, records internal payroll expense, and posts disbursements to General Ledger.",
		RequiredPermission: auth.PermManageHR,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"period": map[string]interface{}{"type": "string", "description": "Period in YYYY-MM format (e.g. 2026-10)"},
			},
		},
	},
	"docs_search": {
		Name:               "docs_search",
		Category:           CategoryReadOnly,
		Description:        "Searches documents and internal knowledge repository by keywords.",
		RequiredPermission: auth.PermRead,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{"type": "string"},
			},
			"required": []string{"query"},
		},
	},
}

// ValidateToolInput validates that the arguments map satisfies required fields and basic schema rules
func ValidateToolInput(toolName string, args map[string]interface{}) error {
	tool, exists := Registry[toolName]
	if !exists {
		return fmt.Errorf("unknown tool: %s", toolName)
	}

	required, ok := tool.InputSchema["required"].([]string)
	if !ok {
		// Try []interface{}
		if rawReq, okRaw := tool.InputSchema["required"].([]interface{}); okRaw {
			for _, item := range rawReq {
				if s, okS := item.(string); okS {
					required = append(required, s)
				}
			}
		}
	}

	for _, reqKey := range required {
		val, found := args[reqKey]
		if !found || val == nil {
			return fmt.Errorf("missing required argument %q for tool %s", reqKey, toolName)
		}
		// String empty check
		if str, isStr := val.(string); isStr && str == "" {
			return fmt.Errorf("argument %q cannot be empty for tool %s", reqKey, toolName)
		}
	}

	return nil
}
