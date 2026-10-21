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
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/metrics"
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
	store       *store.Store
	service     *service.WorkflowService
	sessions    map[string]*SessionContext
	rateLimiter *RateLimiter
	killSwitch  *KillSwitchManager
	mu          sync.RWMutex
}

func NewMCPServer(s *store.Store, svc *service.WorkflowService) *MCPServer {
	if svc == nil {
		svc = service.NewWorkflowService(s)
	}
	return &MCPServer{
		store:       s,
		service:     svc,
		sessions:    make(map[string]*SessionContext),
		rateLimiter: NewRateLimiter(60, 5),
		killSwitch:  DefaultKillSwitch(),
	}
}

func (m *MCPServer) RateLimiter() *RateLimiter {
	return m.rateLimiter
}

func (m *MCPServer) KillSwitch() *KillSwitchManager {
	return m.killSwitch
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
	var list []map[string]interface{}
	for _, tool := range Registry {
		def := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"category":    string(tool.Category),
			"inputSchema": tool.InputSchema,
		}
		if tool.OutputSchema != nil {
			def["outputSchema"] = tool.OutputSchema
		}
		list = append(list, def)
	}
	return list
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
	if params.Arguments == nil {
		params.Arguments = make(map[string]interface{})
	}

	// 1. Tool Lookup in Centralized Registry
	toolDef, exists := Registry[params.Name]
	if !exists {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32601, Message: fmt.Sprintf("Unknown tool: %s", params.Name)},
		}
	}

	// Determine Tenant Org ID & Agent identity
	orgID := "org_default"
	agentID := "mcp_client"
	if sessCtx != nil && sessCtx.AuthCtx != nil {
		orgID = sessCtx.AuthCtx.OrgID
		agentID = sessCtx.AgentID
	}

	// 2. Rate Limiting and Concurrency Control
	releaseSlot, err := m.rateLimiter.Acquire(agentID)
	if err != nil {
		_ = m.store.LogAuditForOrg(orgID, "agent", agentID, "MCP_RATELIMIT_EXCEEDED", "tools", params.Name, err.Error(), "{}")
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32029, Message: err.Error()},
		}
	}
	defer releaseSlot()

	// 3. Emergency Kill Switch Enforcement
	if toolDef.Category != CategoryReadOnly && m.killSwitch.IsActive(orgID) {
		_ = m.store.LogAuditForOrg(orgID, "agent", agentID, "MCP_KILLSWITCH_BLOCKED", "tools", params.Name,
			"Tool execution blocked: emergency kill switch is active", "{}")
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": fmt.Sprintf("Error: %v", ErrKillSwitchActive),
					},
				},
				"isError": true,
			},
		}
	}

	// 4. RBAC Capability Validation
	if sessCtx == nil || sessCtx.AuthCtx == nil || !sessCtx.AuthCtx.Can(toolDef.RequiredPermission) {
		role := "anonymous"
		if sessCtx != nil && sessCtx.AuthCtx != nil {
			role = string(sessCtx.AuthCtx.Role)
		}

		_ = m.store.LogAuditForOrg(orgID, "agent", agentID, "MCP_TOOL_FORBIDDEN", "tools", params.Name,
			fmt.Sprintf("Forbidden: role %q lacks permission %q", role, toolDef.RequiredPermission), "{}")

		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": fmt.Sprintf("Error: Permission denied. Role %q lacks required permission %q to execute %q.",
							role, toolDef.RequiredPermission, params.Name),
					},
				},
				"isError": true,
			},
		}
	}

	// 5. Schema Validation against Typed Tool Registry
	if err := ValidateToolInput(params.Name, params.Arguments); err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: fmt.Sprintf("Invalid arguments: %v", err)},
		}
	}

	// 6. Untrusted Prompt Injection Detection & Input Cleansing
	for k, v := range params.Arguments {
		if s, ok := v.(string); ok {
			sanitized, err := SanitizePromptInput(s)
			if err != nil {
				_ = m.store.LogAuditForOrg(orgID, "agent", agentID, "MCP_PROMPT_INJECTION", "tools", params.Name,
					fmt.Sprintf("Prompt injection detected in parameter %q: %s", k, s), "{}")
				return &JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: map[string]interface{}{
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": fmt.Sprintf("Error: Security violation: %v in argument %q.", ErrPromptInjection, k),
							},
						},
						"isError": true,
					},
				}
			}
			params.Arguments[k] = sanitized
		}
	}

	// 7. Context Isolation: Strip caller-supplied org_id to prevent cross-tenant tampering
	delete(params.Arguments, "org_id")

	// 8. Execution Deadline and Duration Tracking
	startTime := time.Now()
	toolCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	textResult := ""
	isErr := false

	// 9. Execute Tool with Tenant Scoping and Deadline Context
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
		item, err := m.service.AdjustStock(toolCtx, orgID, skuID, int(deltaFloat), reason, authContext, expectedVer, idempKey)
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
		res, err := m.service.CreatePurchaseOrder(toolCtx, orgID, service.CreatePORequest{
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
		inv, err := m.service.CreateInvoice(toolCtx, orgID, service.CreateInvoiceRequest{
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
		report, err := m.service.ReconcileDiscrepancies(toolCtx, orgID)
		if err != nil {
			textResult = fmt.Sprintf("Error running reconciliation: %v", err)
			isErr = true
		} else {
			bytes, _ := json.MarshalIndent(report, "", "  ")
			textResult = string(bytes)
		}

	case "fin_trial_balance":
		report, err := m.service.TrialBalance(toolCtx, orgID)
		if err != nil {
			textResult = fmt.Sprintf("Error fetching trial balance: %v", err)
			isErr = true
		} else {
			bytes, _ := json.MarshalIndent(report, "", "  ")
			textResult = string(bytes)
		}

	case "sys_backup":
		prefix := "agent"
		if p, ok := params.Arguments["prefix"].(string); ok && p != "" {
			prefix = p
		}
		res, err := m.service.CreateBackup(toolCtx, prefix)
		if err != nil {
			textResult = fmt.Sprintf("Backup failed: %v", err)
			isErr = true
		} else {
			textResult = fmt.Sprintf("Snapshot backup created successfully at %s (%d bytes). Integrity verification: %t.",
				res.BackupPath, res.SizeBytes, res.IntegrityOK)
		}

	case "crm_convert_deal":
		dealID, _ := params.Arguments["deal_id"].(string)
		idempKey, _ := params.Arguments["idempotency_key"].(string)
		var authContext *auth.AuthContext
		if sessCtx != nil {
			authContext = sessCtx.AuthCtx
		}
		inv, err := m.service.ConvertDealToInvoice(toolCtx, orgID, dealID, authContext, idempKey)
		if err != nil {
			textResult = fmt.Sprintf("Error converting deal to invoice: %v", err)
			isErr = true
		} else {
			textResult = fmt.Sprintf("Deal %s successfully converted to Invoice %s ($%.2f). Deal marked as 'won'.",
				dealID, inv.InvoiceNumber, inv.TotalAmount)
		}

	case "erp_pay_purchase_order":
		poID, _ := params.Arguments["po_id"].(string)
		idempKey, _ := params.Arguments["idempotency_key"].(string)
		var authContext *auth.AuthContext
		if sessCtx != nil {
			authContext = sessCtx.AuthCtx
		}
		po, err := m.service.PayPurchaseOrder(toolCtx, orgID, poID, authContext, idempKey)
		if err != nil {
			textResult = fmt.Sprintf("Error paying purchase order: %v", err)
			isErr = true
		} else {
			textResult = fmt.Sprintf("Purchase Order %s (%s) marked 'paid'. Double-entry ledger transaction posted (Debit: AP, Credit: Cash).",
				po.PONumber, po.ID)
		}

	case "hr_lock_payroll":
		period, _ := params.Arguments["period"].(string)
		var authContext *auth.AuthContext
		if sessCtx != nil {
			authContext = sessCtx.AuthCtx
		}
		res, err := m.service.LockPayroll(toolCtx, orgID, period, authContext)
		if err != nil {
			textResult = fmt.Sprintf("Error locking payroll: %v", err)
			isErr = true
		} else {
			textResult = fmt.Sprintf("Payroll locked for period %s: %d timesheets disbursed, $%.2f gross total. Operating Expense recorded and posted to ledger (Entry: %s).",
				res.Period, res.TimesheetsPaid, res.TotalGrossDollars, res.JournalEntryID)
		}

	case "docs_search":
		query, _ := params.Arguments["query"].(string)
		results, err := m.service.GlobalSearch(toolCtx, orgID, query)
		if err != nil {
			textResult = fmt.Sprintf("Error searching docs: %v", err)
			isErr = true
		} else {
			bytes, _ := json.MarshalIndent(results, "", "  ")
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

	// 4. Durable Execution Logging in agent_executions table
	durationMS := time.Since(startTime).Milliseconds()
	argsJSON := TruncateJSON(params.Arguments, 1000)
	resJSON := TruncateJSON(textResult, 1000)
	_ = RecordAgentExecution(context.Background(), m.store.DB(), AgentExecution{
		OrgID:         orgID,
		AgentID:       agentID,
		ToolName:      params.Name,
		Category:      string(toolDef.Category),
		ArgumentsJSON: argsJSON,
		ResultJSON:    resJSON,
		IsError:       isErr,
		DurationMS:    durationMS,
		CreatedAt:     startTime,
	})

	// Record in Prometheus metrics
	statusMetric := "success"
	if isErr {
		statusMetric = "error"
	}
	metrics.Default().RecordAgentToolExecution(params.Name, statusMetric)

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
		{
			"uri":         "res://ledger/trial-balance",
			"name":        "General Ledger Trial Balance",
			"description": "Real-time debit/credit trial balance ensuring double-entry balance equality.",
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
	case "res://ledger/trial-balance":
		data, err = m.service.TrialBalance(context.Background(), orgID)
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
