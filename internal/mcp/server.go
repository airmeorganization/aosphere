package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"aosphere/internal/auth"
	"aosphere/internal/service"
	"aosphere/internal/store"
	"github.com/google/uuid"
)

// Standard JSON-RPC 2.0 structures
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// SessionContext stores authenticated identity and SSE stream channel
type SessionContext struct {
	ID      string
	MsgChan chan []byte
	AuthCtx *auth.AuthContext
	AgentID string
}

type MCPServer struct {
	store    *store.Store
	service  *service.WorkflowService
	sessions map[string]*SessionContext
	mu       sync.RWMutex
}

func NewMCPServer(s *store.Store, svc *service.WorkflowService) *MCPServer {
	if svc == nil {
		svc = service.NewWorkflowService(s)
	}
	return &MCPServer{
		store:    s,
		service:  svc,
		sessions: make(map[string]*SessionContext),
	}
}

// authenticateRequest extracts and validates the API key or session token
func (m *MCPServer) authenticateRequest(r *http.Request) (*auth.AuthContext, string, error) {
	token := auth.ExtractToken(r)
	if token == "" {
		return nil, "", fmt.Errorf("missing authentication token or API key")
	}

	tokenHash := auth.HashToken(token)

	// 1. API key lookup
	if strings.HasPrefix(token, "ao_sk_") {
		key, err := m.store.GetAPIKeyByHash(tokenHash)
		if err != nil || key == nil {
			return nil, "", fmt.Errorf("invalid or revoked API key")
		}
		role, _ := auth.ParseRole(key.Role)
		return &auth.AuthContext{
			UserID:   key.UserID,
			OrgID:    key.OrgID,
			Email:    "agent@" + key.Name,
			Name:     key.Name,
			Role:     role,
			IsAPIKey: true,
		}, key.Name, nil
	}

	// 2. User session lookup
	sess, user, err := m.store.GetSessionAndUser(tokenHash)
	if err != nil || user == nil || sess == nil {
		return nil, "", fmt.Errorf("invalid or expired session token")
	}
	role, _ := auth.ParseRole(user.Role)
	return &auth.AuthContext{
		UserID:   user.ID,
		OrgID:    user.OrgID,
		Email:    user.Email,
		Name:     user.Name,
		Role:     role,
		IsAPIKey: false,
	}, user.Email, nil
}

// HandleStdio runs MCP over stdin/stdout for desktop AI agents (Claude Desktop, Antigravity, Cursor)
func (m *MCPServer) HandleStdio(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)

	cliAuthCtx := &auth.AuthContext{
		UserID:   "usr_admin",
		OrgID:    "org_default",
		Email:    "cli@aosphere.local",
		Name:     "stdio_agent",
		Role:     auth.RoleOwner,
		IsAPIKey: true,
	}
	sessCtx := &SessionContext{
		ID:      "stdio_session",
		AuthCtx: cliAuthCtx,
		AgentID: "stdio_agent",
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var req JSONRPCRequest
			if err := json.Unmarshal(line, &req); err != nil {
				continue
			}

			resp := m.ProcessRequestWithContext(&req, sessCtx)
			if resp != nil {
				out, _ := json.Marshal(resp)
				writer.Write(out)
				writer.WriteByte('\n')
				writer.Flush()
			}
		}
	}
	return scanner.Err()
}

// SSE Handler for web-connected AI agents (HTTP SSE)
func (m *MCPServer) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	authCtx, agentID, err := m.authenticateRequest(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "unauthorized",
			"message": fmt.Sprintf("MCP SSE authentication failed: %v. Provide valid ao_sk_... key or session token.", err),
		})
		return
	}

	sessionID := uuid.New().String()
	msgChan := make(chan []byte, 32)
	sessCtx := &SessionContext{
		ID:      sessionID,
		MsgChan: msgChan,
		AuthCtx: authCtx,
		AgentID: agentID,
	}

	m.mu.Lock()
	m.sessions[sessionID] = sessCtx
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.sessions, sessionID)
		m.mu.Unlock()
		close(msgChan)
	}()

	_ = m.store.LogAuditForOrg(authCtx.OrgID, "agent", agentID, "MCP_CONNECT", "sse_session", sessionID,
		fmt.Sprintf("Client connected with role %s", authCtx.Role), "{}")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send endpoint event according to MCP SSE spec
	endpointURL := fmt.Sprintf("/mcp/messages?sessionId=%s", sessionID)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg := <-msgChan:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			flusher.Flush()
		}
	}
}

// HandlePostMessage handles messages for active SSE sessions or direct JSON-RPC POST
func (m *MCPServer) HandlePostMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	var sessCtx *SessionContext
	if sessionID != "" {
		m.mu.RLock()
		sessCtx = m.sessions[sessionID]
		m.mu.RUnlock()
	}

	// If no existing session found, authenticate directly from request headers
	if sessCtx == nil {
		authCtx, agentID, err := m.authenticateRequest(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "unauthorized",
				"message": fmt.Sprintf("MCP authentication required: %v", err),
			})
			return
		}
		sessCtx = &SessionContext{
			ID:      "direct_" + uuid.New().String()[:8],
			AuthCtx: authCtx,
			AgentID: agentID,
		}
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := m.ProcessRequestWithContext(&req, sessCtx)
	if resp == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	respBytes, _ := json.Marshal(resp)

	// If session has active SSE channel, send through channel and reply 202
	if sessCtx.MsgChan != nil {
		sessCtx.MsgChan <- respBytes
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Direct HTTP response fallback
	w.Header().Set("Content-Type", "application/json")
	w.Write(respBytes)
}

// ProcessRequest executes the JSON-RPC request with default compatibility context
func (m *MCPServer) ProcessRequest(req *JSONRPCRequest, agentID string) *JSONRPCResponse {
	return m.ProcessRequestWithContext(req, &SessionContext{
		ID:      "compat_" + agentID,
		AgentID: agentID,
		AuthCtx: &auth.AuthContext{
			UserID:   "usr_admin",
			OrgID:    "org_default",
			Email:    agentID + "@aosphere.local",
			Name:     agentID,
			Role:     auth.RoleOwner,
			IsAPIKey: true,
		},
	})
}

// ProcessRequestWithContext executes the JSON-RPC request enforcing RBAC and tenant scoping
func (m *MCPServer) ProcessRequestWithContext(req *JSONRPCRequest, sessCtx *SessionContext) *JSONRPCResponse {
	// Notifications (no ID)
	if req.ID == nil {
		if req.Method == "notifications/initialized" {
			log.Printf("MCP Client initialized: %s", sessCtx.AgentID)
		}
		return nil
	}

	switch req.Method {
	case "initialize":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]interface{}{
					"name":    "aosphere-mcp",
					"version": "1.0.0",
				},
				"capabilities": map[string]interface{}{
					"tools":     map[string]interface{}{},
					"resources": map[string]interface{}{},
					"prompts":   map[string]interface{}{},
				},
			},
		}

	case "tools/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"tools": m.getToolDefinitions(),
			},
		}

	case "tools/call":
		return m.handleToolCall(req, sessCtx)

	case "resources/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"resources": m.getResourceDefinitions(),
			},
		}

	case "resources/read":
		return m.handleResourceRead(req, sessCtx)

	case "prompts/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"prompts": m.getPromptDefinitions(),
			},
		}

	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &RPCError{
				Code:    -32601,
				Message: fmt.Sprintf("Method '%s' not found", req.Method),
			},
		}
	}
}

func (m *MCPServer) getToolDefinitions() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "erp_check_stock",
			"description": "Inspect stock levels, reserved quantities, and threshold alerts for SKUs.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"below_threshold_only": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, only returns items where stock_quantity <= min_threshold",
					},
				},
			},
		},
		{
			"name":        "erp_adjust_stock",
			"description": "Adjust SKU inventory quantity with optimistic locking. If stock drops below threshold, automatically triggers reorder workflows.",
			"inputSchema": map[string]interface{}{
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
		{
			"name":        "erp_create_purchase_order",
			"description": "Create a purchase order for a supplier with server-side pricing and entity verification. Orders > $1,000 route to human approval.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"supplier_id":     map[string]interface{}{"type": "string"},
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
		{
			"name":        "crm_manage_deal",
			"description": "Update deal stage (lead, qualified, proposal, negotiation, won, lost) or log client notes.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"deal_id": map[string]interface{}{"type": "string"},
					"stage":   map[string]interface{}{"type": "string", "enum": []string{"lead", "qualified", "proposal", "negotiation", "won", "lost"}},
				},
				"required": []string{"deal_id", "stage"},
			},
		},
		{
			"name":        "fin_get_summary",
			"description": "Get real-time Profit & Loss (P&L), unpaid client receivables, and financial health.",
			"inputSchema": map[string]interface{}{
				"type": "object",
			},
		},
		{
			"name":        "fin_create_invoice",
			"description": "Generate a compliant client invoice with line items, tax, and optional recurring schedule.",
			"inputSchema": map[string]interface{}{
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
		{
			"name":        "presence_sync_nap",
			"description": "Synchronize official business Name, Address, Phone (NAP) across search engines and directories.",
			"inputSchema": map[string]interface{}{
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
		{
			"name":        "reputation_draft_reply",
			"description": "Analyze review sentiment and draft an empathetic response for human review.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"review_id":  map[string]interface{}{"type": "string"},
					"reply_text": map[string]interface{}{"type": "string"},
				},
				"required": []string{"review_id", "reply_text"},
			},
		},
		{
			"name":        "approvals_list_pending",
			"description": "View actions waiting for human authorization in the Approval Inbox.",
			"inputSchema": map[string]interface{}{
				"type": "object",
			},
		},
		{
			"name":        "sys_reconcile",
			"description": "Run business workflow reconciliation scan to detect orphan approvals, unlinked pending POs, and inventory reservation anomalies.",
			"inputSchema": map[string]interface{}{
				"type": "object",
			},
		},
	}
}

// toolPermissions maps each tool to the required RBAC capability
var toolPermissions = map[string]auth.Permission{
	"erp_check_stock":           auth.PermRead,
	"erp_adjust_stock":          auth.PermManageInventory,
	"erp_create_purchase_order": auth.PermManageOrders,
	"crm_manage_deal":           auth.PermManageOrders,
	"fin_get_summary":           auth.PermRead,
	"fin_create_invoice":        auth.PermManageFinance,
	"presence_sync_nap":         auth.PermManageInventory,
	"reputation_draft_reply":    auth.PermRead,
	"approvals_list_pending":    auth.PermApproveOrders,
	"sys_reconcile":             auth.PermRead,
}

func (m *MCPServer) handleToolCall(req *JSONRPCRequest, sessCtx *SessionContext) *JSONRPCResponse {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: "Invalid parameters"},
		}
	}

	// 1. RBAC Capability Validation
	requiredPerm, hasPerm := toolPermissions[params.Name]
	if hasPerm && (sessCtx == nil || sessCtx.AuthCtx == nil || !sessCtx.AuthCtx.Can(requiredPerm)) {
		role := "anonymous"
		agentID := "unknown"
		orgID := "org_default"
		if sessCtx != nil && sessCtx.AuthCtx != nil {
			role = string(sessCtx.AuthCtx.Role)
			agentID = sessCtx.AgentID
			orgID = sessCtx.AuthCtx.OrgID
		}

		_ = m.store.LogAuditForOrg(orgID, "agent", agentID, "MCP_TOOL_FORBIDDEN", "tools", params.Name,
			fmt.Sprintf("Forbidden: role %q lacks permission %q", role, requiredPerm), "{}")

		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": fmt.Sprintf("Error: Permission denied. Role %q lacks required permission %q to execute %q.",
							role, requiredPerm, params.Name),
					},
				},
				"isError": true,
			},
		}
	}

	// Determine Tenant Org ID & Agent identity
	orgID := "org_default"
	agentID := "mcp_client"
	if sessCtx != nil && sessCtx.AuthCtx != nil {
		orgID = sessCtx.AuthCtx.OrgID
		agentID = sessCtx.AgentID
	}

	textResult := ""
	isErr := false

	// 2. Execute Tool with Tenant Scoping
	switch params.Name {
	case "erp_check_stock":
		belowOnly := false
		if v, ok := params.Arguments["below_threshold_only"].(bool); ok {
			belowOnly = v
		}
		skus, err := m.store.GetSKUsForOrg(orgID, belowOnly)
		if err != nil {
			textResult = fmt.Sprintf("Error fetching SKUs: %v", err)
			isErr = true
		} else {
			bytes, _ := json.MarshalIndent(skus, "", "  ")
			textResult = string(bytes)
		}

	case "erp_adjust_stock":
		skuID, _ := params.Arguments["sku_id"].(string)
		deltaFloat, _ := params.Arguments["delta"].(float64)
		reason, _ := params.Arguments["reason"].(string)
		expectedVer := 0
		if ev, ok := params.Arguments["expected_version"].(float64); ok {
			expectedVer = int(ev)
		}
		idempKey, _ := params.Arguments["idempotency_key"].(string)

		var authContext *auth.AuthContext
		if sessCtx != nil {
			authContext = sessCtx.AuthCtx
		}
		item, err := m.service.AdjustStock(context.Background(), orgID, skuID, int(deltaFloat), reason, authContext, expectedVer, idempKey)
		if err != nil {
			textResult = fmt.Sprintf("Error updating stock: %v", err)
			isErr = true
		} else {
			textResult = fmt.Sprintf("Success. SKU %s (%s) stock is now %d (Reserved: %d, Min: %d, Version: %d). Automated workflows evaluated.",
				item.Code, item.Name, item.StockQuantity, item.ReservedQuantity, item.MinThreshold, item.Version)
		}

	case "erp_create_purchase_order":
		supplierID, _ := params.Arguments["supplier_id"].(string)
		notes, _ := params.Arguments["notes"].(string)
		idempKey, _ := params.Arguments["idempotency_key"].(string)
		itemsList, _ := params.Arguments["items"].([]interface{})

		var poItems []service.POItem
		for _, raw := range itemsList {
			if itemMap, ok := raw.(map[string]interface{}); ok {
				skuCode, _ := itemMap["sku_code"].(string)
				qtyFloat, _ := itemMap["quantity"].(float64)
				costFloat, _ := itemMap["unit_cost"].(float64)
				poItems = append(poItems, service.POItem{
					SKUCode:  skuCode,
					Quantity: int(qtyFloat),
					UnitCost: costFloat,
				})
			}
		}

		var authContext *auth.AuthContext
		if sessCtx != nil {
			authContext = sessCtx.AuthCtx
		}
		res, err := m.service.CreatePurchaseOrder(context.Background(), orgID, service.CreatePORequest{
			SupplierID: supplierID,
			Items:      poItems,
			Notes:      notes,
		}, authContext, idempKey)
		if err != nil {
			textResult = fmt.Sprintf("Error creating PO: %v", err)
			isErr = true
		} else {
			if res.RequiresApproval {
				textResult = fmt.Sprintf("Purchase Order %s created in status 'pending_approval' (Total: $%.2f exceeds policy threshold of $%.2f). Queued for human approval (Approval ID: %s).",
					res.PurchaseOrder.PONumber, res.PurchaseOrder.TotalAmount, m.service.Policy().POApprovalThreshold, res.Approval.ID)
			} else {
				textResult = fmt.Sprintf("Purchase Order %s placed directly. Status: %s. Total Amount: $%.2f. Safety and entity checks passed.",
					res.PurchaseOrder.PONumber, res.PurchaseOrder.Status, res.PurchaseOrder.TotalAmount)
			}
		}

	case "crm_manage_deal":
		dealID, _ := params.Arguments["deal_id"].(string)
		stage, _ := params.Arguments["stage"].(string)
		err := m.store.UpdateDealStage(dealID, stage, "agent", agentID)
		if err != nil {
			textResult = fmt.Sprintf("Error updating deal: %v", err)
			isErr = true
		} else {
			textResult = fmt.Sprintf("Deal %s moved to stage '%s'", dealID, stage)
		}

	case "fin_get_summary":
		summary, err := m.store.GetFinancialSummary()
		if err != nil {
			textResult = fmt.Sprintf("Error getting financial summary: %v", err)
			isErr = true
		} else {
			bytes, _ := json.MarshalIndent(summary, "", "  ")
			textResult = string(bytes)
		}

	case "fin_create_invoice":
		custID, _ := params.Arguments["customer_id"].(string)
		dueDate, _ := params.Arguments["due_date"].(string)
		recurring, _ := params.Arguments["recurring"].(string)
		idempKey, _ := params.Arguments["idempotency_key"].(string)
		itemsList, _ := params.Arguments["items"].([]interface{})

		var invItems []service.InvoiceItem
		for _, raw := range itemsList {
			if itemMap, ok := raw.(map[string]interface{}); ok {
				desc, _ := itemMap["desc"].(string)
				qtyFloat, _ := itemMap["qty"].(float64)
				priceFloat, _ := itemMap["price"].(float64)
				invItems = append(invItems, service.InvoiceItem{
					Desc:  desc,
					Qty:   int(qtyFloat),
					Price: priceFloat,
				})
			}
		}

		var authContext *auth.AuthContext
		if sessCtx != nil {
			authContext = sessCtx.AuthCtx
		}
		inv, err := m.service.CreateInvoice(context.Background(), orgID, service.CreateInvoiceRequest{
			CustomerID: custID,
			Items:      invItems,
			DueDate:    dueDate,
			Recurring:  recurring,
		}, authContext, idempKey)
		if err != nil {
			textResult = fmt.Sprintf("Error creating invoice: %v", err)
			isErr = true
		} else {
			textResult = fmt.Sprintf("Invoice %s created for total $%.2f (Subtotal: $%.2f, Tax: $%.2f). Due: %s.",
				inv.InvoiceNumber, inv.TotalAmount, inv.Subtotal, inv.TaxAmount, inv.DueDate)
		}

	case "presence_sync_nap":
		bName, _ := params.Arguments["business_name"].(string)
		addr, _ := params.Arguments["address"].(string)
		phone, _ := params.Arguments["phone"].(string)
		web, _ := params.Arguments["website"].(string)
		err := m.store.SyncNAP(bName, addr, phone, web, "agent", agentID)
		if err != nil {
			textResult = fmt.Sprintf("Error syncing NAP: %v", err)
			isErr = true
		} else {
			textResult = "Successfully synchronized NAP data across Google, Yelp, Apple Maps, and Bing."
		}

	case "reputation_draft_reply":
		revID, _ := params.Arguments["review_id"].(string)
		reply, _ := params.Arguments["reply_text"].(string)
		err := m.store.ReplyReview(revID, reply, "drafted_by_agent", "agent", agentID)
		if err != nil {
			textResult = fmt.Sprintf("Error drafting reply: %v", err)
			isErr = true
		} else {
			textResult = fmt.Sprintf("Draft reply saved for review %s. Ready for human authorization.", revID)
		}

	case "approvals_list_pending":
		approvals, err := m.store.GetApprovalsForOrg(orgID, "pending")
		if err != nil {
			textResult = fmt.Sprintf("Error getting pending approvals: %v", err)
			isErr = true
		} else {
			bytes, _ := json.MarshalIndent(approvals, "", "  ")
			textResult = string(bytes)
		}

	case "sys_reconcile":
		report, err := m.service.ReconcileDiscrepancies(context.Background(), orgID)
		if err != nil {
			textResult = fmt.Sprintf("Error running reconciliation: %v", err)
			isErr = true
		} else {
			bytes, _ := json.MarshalIndent(report, "", "  ")
			textResult = string(bytes)
		}

	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32601, Message: fmt.Sprintf("Unknown tool: %s", params.Name)},
		}
	}

	// 3. Audit Log Tool Invocation
	auditStatus := "success"
	if isErr {
		auditStatus = "error"
	}
	argsBytes, _ := json.Marshal(params.Arguments)
	argsStr := string(argsBytes)
	if len(argsStr) > 120 {
		argsStr = argsStr[:120] + "..."
	}
	resStr := textResult
	if len(resStr) > 120 {
		resStr = resStr[:120] + "..."
	}
	_ = m.store.LogAuditForOrg(
		orgID,
		"agent",
		agentID,
		"MCP_TOOL_CALL",
		"tools",
		params.Name,
		fmt.Sprintf("args=%s res=%s", argsStr, resStr),
		fmt.Sprintf(`{"status":"%s"}`, auditStatus),
	)

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": textResult,
				},
			},
			"isError": isErr,
		},
	}
}

func (m *MCPServer) getResourceDefinitions() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"uri":         "res://erp/inventory",
			"name":        "ERP Inventory Status",
			"description": "Real-time stock quantities, reorder thresholds, and SKU catalogs.",
			"mimeType":    "application/json",
		},
		{
			"uri":         "res://crm/pipeline",
			"name":        "CRM Deal Pipeline",
			"description": "Active sales deals sorted by stage and expected value.",
			"mimeType":    "application/json",
		},
		{
			"uri":         "res://financial/summary",
			"name":        "Financial P&L Summary",
			"description": "Total revenue, expenses, net profit, and receivables.",
			"mimeType":    "application/json",
		},
		{
			"uri":         "res://reputation/feed",
			"name":        "Customer Review & Reputation Feed",
			"description": "Aggregated reviews across platforms with sentiment flags.",
			"mimeType":    "application/json",
		},
	}
}

func (m *MCPServer) handleResourceRead(req *JSONRPCRequest, sessCtx *SessionContext) *JSONRPCResponse {
	var params struct {
		URI string `json:"uri"`
	}
	_ = json.Unmarshal(req.Params, &params)

	orgID := "org_default"
	if sessCtx != nil && sessCtx.AuthCtx != nil {
		orgID = sessCtx.AuthCtx.OrgID
	}

	var data interface{}
	var err error

	switch params.URI {
	case "res://erp/inventory":
		data, err = m.store.GetSKUsForOrg(orgID, false)
	case "res://crm/pipeline":
		data, err = m.store.GetDeals()
	case "res://financial/summary":
		data, err = m.store.GetFinancialSummary()
	case "res://reputation/feed":
		data, err = m.store.GetReviews()
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: fmt.Sprintf("Unknown resource URI: %s", params.URI)},
		}
	}

	if err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32000, Message: err.Error()},
		}
	}

	bytes, _ := json.Marshal(data)
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"uri":      params.URI,
					"mimeType": "application/json",
					"text":     string(bytes),
				},
			},
		},
	}
}

func (m *MCPServer) getPromptDefinitions() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "daily_operations_brief",
			"description": "Generates a complete executive briefing across Inventory, P&L, Active Deals, and Escalated Reviews.",
		},
		{
			"name":        "reorder_and_procure",
			"description": "Scans all inventory items against safety thresholds and drafts purchase orders for suppliers.",
		},
	}
}
