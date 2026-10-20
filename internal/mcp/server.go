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

type MCPServer struct {
	store    *store.Store
	sessions map[string]chan []byte
	mu       sync.RWMutex
}

func NewMCPServer(s *store.Store) *MCPServer {
	return &MCPServer{
		store:    s,
		sessions: make(map[string]chan []byte),
	}
}

// HandleStdio runs MCP over stdin/stdout for desktop AI agents (Claude Desktop, Antigravity, Cursor)
func (m *MCPServer) HandleStdio(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	writer := bufio.NewWriter(os.Stdout)

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

			resp := m.ProcessRequest(&req, "stdio_agent")
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

	sessionID := uuid.New().String()
	msgChan := make(chan []byte, 32)

	m.mu.Lock()
	m.sessions[sessionID] = msgChan
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.sessions, sessionID)
		m.mu.Unlock()
		close(msgChan)
	}()

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
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := m.ProcessRequest(&req, "sse_agent_"+sessionID)
	if resp == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	respBytes, _ := json.Marshal(resp)

	// Enqueue under the session read lock. Cleanup needs the write lock
	// before closing the channel, so a concurrent disconnect cannot cause a
	// send-on-closed-channel panic. Never block on a full/abandoned session.
	if sessionID != "" {
		m.mu.RLock()
		ch, ok := m.sessions[sessionID]
		if ok {
			select {
			case ch <- respBytes:
				m.mu.RUnlock()
				w.WriteHeader(http.StatusAccepted)
				return
			default:
				m.mu.RUnlock()
				http.Error(w, "MCP session queue is full", http.StatusServiceUnavailable)
				return
			}
		}
		m.mu.RUnlock()
	}

	// Direct HTTP response fallback
	w.Header().Set("Content-Type", "application/json")
	w.Write(respBytes)
}

// ProcessRequest executes the JSON-RPC request and returns the JSONRPCResponse
func (m *MCPServer) ProcessRequest(req *JSONRPCRequest, agentID string) *JSONRPCResponse {
	// Notifications (no ID)
	if req.ID == nil {
		if req.Method == "notifications/initialized" {
			log.Printf("MCP Client initialized: %s", agentID)
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
		return m.handleToolCall(req, agentID)

	case "resources/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"resources": m.getResourceDefinitions(),
			},
		}

	case "resources/read":
		return m.handleResourceRead(req)

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
			"description": "Adjust SKU inventory quantity. If stock drops below threshold, automatically triggers reorder workflows.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sku_id": map[string]interface{}{"type": "string"},
					"delta":  map[string]interface{}{"type": "integer", "description": "Positive to add stock, negative to consume/sell"},
					"reason": map[string]interface{}{"type": "string"},
				},
				"required": []string{"sku_id", "delta", "reason"},
			},
		},
		{
			"name":        "erp_create_purchase_order",
			"description": "Create a purchase order for a supplier. Orders > $1000 automatically route to the human approval inbox.",
			"inputSchema": map[string]interface{}{
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
					"notes": map[string]interface{}{"type": "string"},
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
					"due_date":  map[string]interface{}{"type": "string", "description": "YYYY-MM-DD"},
					"recurring": map[string]interface{}{"type": "string", "enum": []string{"none", "monthly", "quarterly", "annual"}},
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
	}
}

func (m *MCPServer) handleToolCall(req *JSONRPCRequest, agentID string) *JSONRPCResponse {
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

	textResult := ""

	switch params.Name {
	case "erp_check_stock":
		belowOnly := false
		if v, ok := params.Arguments["below_threshold_only"].(bool); ok {
			belowOnly = v
		}
		skus, err := m.store.GetSKUs(belowOnly)
		if err != nil {
			textResult = fmt.Sprintf("Error fetching SKUs: %v", err)
		} else {
			bytes, _ := json.MarshalIndent(skus, "", "  ")
			textResult = string(bytes)
		}

	case "erp_adjust_stock":
		skuID, _ := params.Arguments["sku_id"].(string)
		deltaFloat, _ := params.Arguments["delta"].(float64)
		reason, _ := params.Arguments["reason"].(string)
		item, err := m.store.UpdateStock(skuID, int(deltaFloat), reason, "agent", agentID)
		if err != nil {
			textResult = fmt.Sprintf("Error updating stock: %v", err)
		} else {
			textResult = fmt.Sprintf("Success. SKU %s stock is now %d (Min: %d). Automated workflows evaluated.",
				item.Code, item.StockQuantity, item.MinThreshold)
		}

	case "erp_create_purchase_order":
		supplierID, _ := params.Arguments["supplier_id"].(string)
		notes, _ := params.Arguments["notes"].(string)
		itemsList, _ := params.Arguments["items"].([]interface{})

		total := 0.0
		for _, raw := range itemsList {
			if m, ok := raw.(map[string]interface{}); ok {
				qty, _ := m["quantity"].(float64)
				cost, _ := m["unit_cost"].(float64)
				total += qty * cost
			}
		}

		itemsBytes, _ := json.Marshal(itemsList)
		po, err := m.store.CreatePurchaseOrder(supplierID, string(itemsBytes), total, "agent", agentID, notes)
		if err != nil {
			textResult = fmt.Sprintf("Error creating PO: %v", err)
		} else {
			textResult = fmt.Sprintf("Purchase Order %s created. Status: %s. Total Amount: $%.2f. Safety checks passed.",
				po.PONumber, po.Status, po.TotalAmount)
		}

	case "crm_manage_deal":
		dealID, _ := params.Arguments["deal_id"].(string)
		stage, _ := params.Arguments["stage"].(string)
		err := m.store.UpdateDealStage(dealID, stage, "agent", agentID)
		if err != nil {
			textResult = fmt.Sprintf("Error updating deal: %v", err)
		} else {
			textResult = fmt.Sprintf("Deal %s moved to stage '%s'", dealID, stage)
		}

	case "fin_get_summary":
		summary, err := m.store.GetFinancialSummary()
		if err != nil {
			textResult = fmt.Sprintf("Error getting financial summary: %v", err)
		} else {
			bytes, _ := json.MarshalIndent(summary, "", "  ")
			textResult = string(bytes)
		}

	case "fin_create_invoice":
		custID, _ := params.Arguments["customer_id"].(string)
		dueDate, _ := params.Arguments["due_date"].(string)
		recurring, _ := params.Arguments["recurring"].(string)
		if recurring == "" {
			recurring = "none"
		}
		itemsList, _ := params.Arguments["items"].([]interface{})
		subtotal := 0.0
		for _, raw := range itemsList {
			if m, ok := raw.(map[string]interface{}); ok {
				qty, _ := m["qty"].(float64)
				price, _ := m["price"].(float64)
				subtotal += qty * price
			}
		}
		tax := subtotal * 0.08 // standard 8% tax calculation
		itemsBytes, _ := json.Marshal(itemsList)
		inv, err := m.store.CreateInvoice(custID, string(itemsBytes), subtotal, tax, dueDate, recurring, "agent", agentID)
		if err != nil {
			textResult = fmt.Sprintf("Error creating invoice: %v", err)
		} else {
			textResult = fmt.Sprintf("Invoice %s created for $%.2f (Subtotal: $%.2f, Tax: $%.2f). Due: %s.",
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
		} else {
			textResult = "Successfully synchronized NAP data across Google, Yelp, Apple Maps, and Bing."
		}

	case "reputation_draft_reply":
		revID, _ := params.Arguments["review_id"].(string)
		reply, _ := params.Arguments["reply_text"].(string)
		err := m.store.ReplyReview(revID, reply, "drafted_by_agent", "agent", agentID)
		if err != nil {
			textResult = fmt.Sprintf("Error drafting reply: %v", err)
		} else {
			textResult = fmt.Sprintf("Draft reply saved for review %s. Ready for human authorization.", revID)
		}

	case "approvals_list_pending":
		approvals, err := m.store.GetApprovals("pending")
		if err != nil {
			textResult = fmt.Sprintf("Error getting pending approvals: %v", err)
		} else {
			bytes, _ := json.MarshalIndent(approvals, "", "  ")
			textResult = string(bytes)
		}

	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32601, Message: fmt.Sprintf("Unknown tool: %s", params.Name)},
		}
	}

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
			"isError": strings.HasPrefix(textResult, "Error"),
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

func (m *MCPServer) handleResourceRead(req *JSONRPCRequest) *JSONRPCResponse {
	var params struct {
		URI string `json:"uri"`
	}
	_ = json.Unmarshal(req.Params, &params)

	var data interface{}
	var err error

	switch params.URI {
	case "res://erp/inventory":
		data, err = m.store.GetSKUs(false)
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
