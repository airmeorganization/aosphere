package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/mcp"
	"aosphere/internal/metrics"
	"aosphere/internal/service"
	"aosphere/internal/store"
	"github.com/google/uuid"
)

func getOrgID(r *http.Request) string {
	if authCtx := GetAuthContext(r.Context()); authCtx != nil && authCtx.OrgID != "" {
		return authCtx.OrgID
	}
	return "org_default"
}

type API struct {
	store     *store.Store
	service   *service.WorkflowService
	env       string
	clients   map[chan string]string // map[msgChan]orgID
	clientsMu sync.RWMutex
}

func NewAPI(s *store.Store, svc *service.WorkflowService, env string) *API {
	if svc == nil {
		svc = service.NewWorkflowService(s)
	}
	return &API{
		store:   s,
		service: svc,
		env:     env,
		clients: make(map[chan string]string),
	}
}

// requireAuth validates that the request is authenticated and has the required permission
func (a *API) requireAuth(w http.ResponseWriter, r *http.Request, perm auth.Permission) (*auth.AuthContext, bool) {
	authCtx := GetAuthContext(r.Context())
	if authCtx == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]string{
			"error":   "unauthorized",
			"message": "Authentication required. Please provide a valid session token or API key.",
		})
		return nil, false
	}
	if perm != "" && !authCtx.Can(perm) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		writeJSON(w, map[string]string{
			"error":   "forbidden",
			"message": fmt.Sprintf("Permission %s required", perm),
		})
		return authCtx, false
	}
	return authCtx, true
}

// BroadcastForOrg sends an SSE event to clients belonging to that organization
func (a *API) BroadcastForOrg(orgID, eventType, payload string) {
	a.clientsMu.RLock()
	defer a.clientsMu.RUnlock()

	evtID := fmt.Sprintf("evt_%d_%s", time.Now().UnixNano(), uuid.New().String()[:8])
	msg := fmt.Sprintf("id: %s\nevent: %s\ndata: %s\n\n", evtID, eventType, payload)
	for ch, clientOrgID := range a.clients {
		if orgID == "" || orgID == "*" || clientOrgID == orgID {
			select {
			case ch <- msg:
			default:
			}
		}
	}
}

// Broadcast sends an event across all clients for backwards compatibility
func (a *API) Broadcast(eventType, payload string) {
	a.BroadcastForOrg("*", eventType, payload)
}

func (a *API) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	orgID := getOrgID(r)
	authCtx := GetAuthContext(r.Context())
	if a.env == "production" && authCtx == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]string{
			"error":   "unauthorized",
			"message": "Authentication required for live event stream.",
		})
		return
	}
	if authCtx != nil && authCtx.OrgID != "" {
		orgID = authCtx.OrgID
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	if origin := r.Header.Get("Origin"); origin != "" {
		if !auth.ApplyCORSHeaders(w, r) {
			http.Error(w, "CORS origin forbidden", http.StatusForbidden)
			return
		}
	}

	msgChan := make(chan string, 32)
	a.clientsMu.Lock()
	a.clients[msgChan] = orgID
	a.clientsMu.Unlock()

	defer func() {
		a.clientsMu.Lock()
		delete(a.clients, msgChan)
		a.clientsMu.Unlock()
		close(msgChan)
	}()

	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\",\"org_id\":\"%s\",\"time\":\"%s\"}\n\n", orgID, time.Now().Format(time.RFC3339))
	flusher.Flush()

	// Replay recent outbox events for THIS organization if Last-Event-ID header is present (resilient reconnection)
	lastEventID := r.Header.Get("Last-Event-ID")
	if lastEventID == "" {
		lastEventID = r.URL.Query().Get("lastEventId")
	}
	if lastEventID != "" {
		rows, err := a.store.DB().QueryContext(r.Context(),
			`SELECT id, event_type, payload_json FROM outbox_events WHERE org_id = ? AND status = 'published' ORDER BY created_at DESC LIMIT 20`, orgID)
		if err == nil {
			type historyEvent struct{ id, evt, data string }
			var history []historyEvent
			for rows.Next() {
				var h historyEvent
				if err := rows.Scan(&h.id, &h.evt, &h.data); err == nil {
					history = append(history, h)
				}
			}
			rows.Close()
			for i := len(history) - 1; i >= 0; i-- {
				fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", history[i].id, history[i].evt, history[i].data)
			}
			flusher.Flush()
		}
	}

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg := <-msgChan:
			fmt.Fprint(w, msg)
			flusher.Flush()
		}
	}
}

// Helper to write JSON responses
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (a *API) RegisterRoutes(mux *http.ServeMux) {
	// 0. Observability & Prometheus Metrics
	if a.store != nil && a.store.DB() != nil {
		metrics.Default().RegisterDBMetrics(a.store.DB())
		metrics.Default().RegisterGauge("aosphere_pending_approvals_total", func() float64 {
			var count int
			_ = a.store.DB().QueryRow("SELECT COUNT(*) FROM approvals WHERE status = 'pending'").Scan(&count)
			return float64(count)
		})
		metrics.Default().RegisterGauge("aosphere_outbox_pending_total", func() float64 {
			var count int
			_ = a.store.DB().QueryRow("SELECT COUNT(*) FROM outbox_events WHERE status = 'pending'").Scan(&count)
			return float64(count)
		})
		metrics.Default().RegisterGauge("aosphere_outbox_dead_letter_total", func() float64 {
			var count int
			_ = a.store.DB().QueryRow("SELECT COUNT(*) FROM outbox_events WHERE status = 'dead_letter'").Scan(&count)
			return float64(count)
		})
	}
	mux.HandleFunc("/metrics", metrics.Default().PrometheusHandler())

	// Health & Diagnostics Endpoints
	mux.HandleFunc("/healthz", a.HandleHealthz)
	mux.HandleFunc("/readyz", a.HandleReadyz)
	mux.HandleFunc("/api/version", a.HandleInfo)
	mux.HandleFunc("/api/v1/info", a.HandleInfo)

	// Authentication & Identity
	mux.HandleFunc("/api/auth/register", a.HandleRegister)
	mux.HandleFunc("/api/auth/login", a.HandleLogin)
	mux.HandleFunc("/api/auth/logout", a.HandleLogout)
	mux.HandleFunc("/api/auth/me", a.HandleMe)
	mux.HandleFunc("/api/auth/api-keys", a.HandleAPIKeys)

	// SSE Live Feed
	mux.HandleFunc("/api/events", a.HandleSSE)

	// ERP
	mux.HandleFunc("/api/skus", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authCtx, ok := a.requireAuth(w, r, auth.PermManageInventory)
			if !ok {
				return
			}
			var req struct {
				Code            string  `json:"code"`
				Name            string  `json:"name"`
				Category        string  `json:"category"`
				StockQuantity   int     `json:"stock_quantity"`
				MinThreshold    int     `json:"min_threshold"`
				ReorderQuantity int     `json:"reorder_quantity"`
				UnitCost        float64 `json:"unit_cost"`
				SellingPrice    float64 `json:"selling_price"`
				SupplierID      string  `json:"supplier_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid request payload: "+err.Error(), http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" {
				http.Error(w, "code and name are required", http.StatusBadRequest)
				return
			}
			if req.MinThreshold <= 0 {
				req.MinThreshold = 10
			}
			if req.ReorderQuantity <= 0 {
				req.ReorderQuantity = 50
			}

			sku, err := a.store.CreateSKUForOrg(authCtx.OrgID, req.Code, req.Name, req.Category, req.SupplierID,
				req.StockQuantity, req.MinThreshold, req.ReorderQuantity, req.UnitCost, req.SellingPrice)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			actorType := "human"
			if authCtx.IsAPIKey {
				actorType = "agent"
			}
			_ = a.store.LogAuditForOrg(authCtx.OrgID, actorType, authCtx.UserID, "SKU_CREATED", "skus", sku.ID,
				fmt.Sprintf("Created SKU %s (%s)", sku.Code, sku.Name), fmt.Sprintf("{\"cost\":%.2f,\"price\":%.2f,\"stock\":%d}", sku.UnitCost, sku.SellingPrice, sku.StockQuantity))

			a.BroadcastForOrg(authCtx.OrgID, "sku_created", fmt.Sprintf("{\"sku_id\":\"%s\",\"code\":\"%s\",\"name\":\"%s\"}", sku.ID, sku.Code, sku.Name))
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, sku)
			return
		}

		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		below := r.URL.Query().Get("below_threshold") == "true"
		items, err := a.store.GetSKUsForOrg(authCtx.OrgID, below)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, items)
	})

	mux.HandleFunc("/api/skus/adjust", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageInventory)
		if !ok {
			return
		}
		var req struct {
			SKUID           string `json:"sku_id"`
			Delta           int    `json:"delta"`
			Reason          string `json:"reason"`
			ExpectedVersion int    `json:"expected_version"`
			IdempotencyKey  string `json:"idempotency_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		idempKey := r.Header.Get("Idempotency-Key")
		if idempKey == "" {
			idempKey = req.IdempotencyKey
		}
		updated, err := a.service.AdjustStock(r.Context(), authCtx.OrgID, req.SKUID, req.Delta, req.Reason, authCtx, req.ExpectedVersion, idempKey)
		if err != nil {
			if errors.Is(err, service.ErrStaleVersion) {
				w.WriteHeader(http.StatusConflict)
				writeJSON(w, map[string]string{"error": "conflict", "message": err.Error()})
				return
			}
			if errors.Is(err, service.ErrInsufficientStock) {
				w.WriteHeader(http.StatusBadRequest)
				writeJSON(w, map[string]string{"error": "insufficient_stock", "message": err.Error()})
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "stock_updated", fmt.Sprintf("{\"sku_id\":\"%s\",\"new_stock\":%d}", updated.ID, updated.StockQuantity))
		writeJSON(w, updated)
	})

	mux.HandleFunc("/api/suppliers", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		sups, err := a.store.GetSuppliersForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, sups)
	})

	mux.HandleFunc("/api/purchase-orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			authCtx, ok := a.requireAuth(w, r, auth.PermManageOrders)
			if !ok {
				return
			}
			var req struct {
				SupplierID     string           `json:"supplier_id"`
				Items          []service.POItem `json:"items"`
				ItemsJSON      string           `json:"items_json"`
				Total          float64          `json:"total"`
				Notes          string           `json:"notes"`
				IdempotencyKey string           `json:"idempotency_key"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if len(req.Items) == 0 && req.ItemsJSON != "" {
				_ = json.Unmarshal([]byte(req.ItemsJSON), &req.Items)
			}
			idempKey := r.Header.Get("Idempotency-Key")
			if idempKey == "" {
				idempKey = req.IdempotencyKey
			}
			poReq := service.CreatePORequest{
				SupplierID: req.SupplierID,
				Items:      req.Items,
				Notes:      req.Notes,
			}
			res, err := a.service.CreatePurchaseOrder(r.Context(), authCtx.OrgID, poReq, authCtx, idempKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			a.BroadcastForOrg(authCtx.OrgID, "po_created", fmt.Sprintf("{\"po_number\":\"%s\"}", res.PurchaseOrder.PONumber))
			if res.RequiresApproval {
				a.BroadcastForOrg(authCtx.OrgID, "approval_required", fmt.Sprintf("{\"approval_id\":\"%s\"}", res.Approval.ID))
			}
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, res.PurchaseOrder)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		pos, err := a.store.GetPurchaseOrdersForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, pos)
	})

	mux.HandleFunc("/api/purchase-orders/receive", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageOrders)
		if !ok {
			return
		}
		var req struct {
			POID           string `json:"po_id"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		idempKey := r.Header.Get("Idempotency-Key")
		if idempKey == "" {
			idempKey = req.IdempotencyKey
		}
		po, err := a.service.ReceivePurchaseOrder(r.Context(), authCtx.OrgID, req.POID, authCtx, idempKey)
		if err != nil {
			if errors.Is(err, service.ErrInvalidStateTransition) {
				w.WriteHeader(http.StatusConflict)
				writeJSON(w, map[string]string{"error": "invalid_transition", "message": err.Error()})
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "po_received", fmt.Sprintf("{\"po_number\":\"%s\"}", po.PONumber))
		writeJSON(w, po)
	})

	mux.HandleFunc("/api/purchase-orders/pay", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageFinance)
		if !ok {
			return
		}
		var req struct {
			POID           string `json:"po_id"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		idempKey := r.Header.Get("Idempotency-Key")
		if idempKey == "" {
			idempKey = req.IdempotencyKey
		}
		po, err := a.service.PayPurchaseOrder(r.Context(), authCtx.OrgID, req.POID, authCtx, idempKey)
		if err != nil {
			if errors.Is(err, service.ErrPONotReceived) {
				w.WriteHeader(http.StatusConflict)
				writeJSON(w, map[string]string{"error": "invalid_transition", "message": err.Error()})
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "po_paid", fmt.Sprintf("{\"po_number\":\"%s\"}", po.PONumber))
		writeJSON(w, po)
	})

	// CRM
	mux.HandleFunc("/api/customers", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		custs, err := a.store.GetCustomersForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, custs)
	})

	mux.HandleFunc("/api/deals", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			authCtx, ok := a.requireAuth(w, r, auth.PermManageOrders)
			if !ok {
				return
			}
			var req struct {
				Title      string  `json:"title"`
				CustomerID string  `json:"customer_id"`
				Value      float64 `json:"value"`
				Stage      string  `json:"stage"`
				CloseDate  string  `json:"expected_close_date"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			actorType := "human"
			if authCtx.IsAPIKey {
				actorType = "agent"
			}
			deal, err := a.store.CreateDealForOrg(authCtx.OrgID, req.Title, req.CustomerID, req.Value, req.Stage, 60, req.CloseDate, actorType, authCtx.UserID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			a.BroadcastForOrg(authCtx.OrgID, "deal_created", fmt.Sprintf("{\"deal_id\":\"%s\"}", deal.ID))
			writeJSON(w, deal)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		deals, err := a.store.GetDealsForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, deals)
	})

	mux.HandleFunc("/api/deals/stage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "PUT" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageOrders)
		if !ok {
			return
		}
		var req struct {
			DealID string `json:"deal_id"`
			Stage  string `json:"stage"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		actorType := "human"
		if authCtx.IsAPIKey {
			actorType = "agent"
		}
		if err := a.store.UpdateDealStageForOrg(authCtx.OrgID, req.DealID, req.Stage, actorType, authCtx.UserID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "deal_stage_changed", fmt.Sprintf("{\"deal_id\":\"%s\",\"stage\":\"%s\"}", req.DealID, req.Stage))
		writeJSON(w, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/deals/quote", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageOrders)
		if !ok {
			return
		}
		var req struct {
			DealID string                `json:"deal_id"`
			Items  []service.InvoiceItem `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		quote, err := a.service.GenerateQuote(r.Context(), authCtx.OrgID, req.DealID, req.Items, authCtx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "quote_generated", fmt.Sprintf("{\"quote_number\":\"%s\",\"deal_id\":\"%s\"}", quote.QuoteNumber, req.DealID))
		writeJSON(w, quote)
	})

	mux.HandleFunc("/api/deals/convert", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageFinance)
		if !ok {
			return
		}
		var req struct {
			DealID         string `json:"deal_id"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		idempKey := r.Header.Get("Idempotency-Key")
		if idempKey == "" {
			idempKey = req.IdempotencyKey
		}
		inv, err := a.service.ConvertDealToInvoice(r.Context(), authCtx.OrgID, req.DealID, authCtx, idempKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "deal_converted", fmt.Sprintf("{\"deal_id\":\"%s\",\"invoice_number\":\"%s\"}", req.DealID, inv.InvoiceNumber))
		writeJSON(w, inv)
	})

	// FINANCIAL
	mux.HandleFunc("/api/invoices", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			authCtx, ok := a.requireAuth(w, r, auth.PermManageFinance)
			if !ok {
				return
			}
			var req struct {
				CustomerID     string                `json:"customer_id"`
				Items          []service.InvoiceItem `json:"items"`
				ItemsJSON      string                `json:"items_json"`
				Subtotal       float64               `json:"subtotal"`
				Tax            float64               `json:"tax"`
				DueDate        string                `json:"due_date"`
				Recurring      string                `json:"recurring"`
				IdempotencyKey string                `json:"idempotency_key"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if len(req.Items) == 0 && req.ItemsJSON != "" {
				_ = json.Unmarshal([]byte(req.ItemsJSON), &req.Items)
			}
			idempKey := r.Header.Get("Idempotency-Key")
			if idempKey == "" {
				idempKey = req.IdempotencyKey
			}
			invReq := service.CreateInvoiceRequest{
				CustomerID: req.CustomerID,
				Items:      req.Items,
				DueDate:    req.DueDate,
				Recurring:  req.Recurring,
			}
			inv, err := a.service.CreateInvoice(r.Context(), authCtx.OrgID, invReq, authCtx, idempKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			a.BroadcastForOrg(authCtx.OrgID, "invoice_created", fmt.Sprintf("{\"invoice_number\":\"%s\"}", inv.InvoiceNumber))
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, inv)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		invs, err := a.store.GetInvoicesForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, invs)
	})

	mux.HandleFunc("/api/invoices/pay", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageFinance)
		if !ok {
			return
		}
		var req struct {
			InvoiceID      string `json:"invoice_id"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		idempKey := r.Header.Get("Idempotency-Key")
		if idempKey == "" {
			idempKey = req.IdempotencyKey
		}
		inv, err := a.service.PayInvoice(r.Context(), authCtx.OrgID, req.InvoiceID, authCtx, idempKey)
		if err != nil {
			if errors.Is(err, service.ErrInvalidStateTransition) {
				w.WriteHeader(http.StatusConflict)
				writeJSON(w, map[string]string{"error": "invalid_transition", "message": err.Error()})
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "invoice_paid", fmt.Sprintf("{\"invoice_number\":\"%s\"}", inv.InvoiceNumber))
		writeJSON(w, inv)
	})

	mux.HandleFunc("/api/financial-summary", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		summary, err := a.store.GetFinancialSummaryForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, summary)
	})

	mux.HandleFunc("/api/ledger/trial-balance", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermManageFinance)
		if !ok {
			return
		}
		report, err := a.service.TrialBalance(r.Context(), authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, report)
	})

	mux.HandleFunc("/api/expenses", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		exps, err := a.store.GetExpensesForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, exps)
	})

	// HRMS & PRODUCTIVITY
	mux.HandleFunc("/api/employees", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			authCtx, ok := a.requireAuth(w, r, auth.PermManageHR)
			if !ok {
				return
			}
			var req struct {
				Name       string  `json:"name"`
				Role       string  `json:"role"`
				Department string  `json:"department"`
				Email      string  `json:"email"`
				Phone      string  `json:"phone"`
				HourlyRate float64 `json:"hourly_rate"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			emp, err := a.store.CreateEmployee(authCtx.OrgID, req.Name, req.Role, req.Department, req.Email, req.Phone, req.HourlyRate)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			a.BroadcastForOrg(authCtx.OrgID, "employee_added", fmt.Sprintf("{\"id\":\"%s\",\"name\":\"%s\"}", emp.ID, emp.Name))
			writeJSON(w, emp)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		emps, err := a.store.GetEmployeesForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, emps)
	})

	mux.HandleFunc("/api/timesheets", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		ts, err := a.store.GetTimesheetsForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, ts)
	})

	mux.HandleFunc("/api/timesheets/approve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageHR)
		if !ok {
			return
		}
		var req struct {
			TimesheetID string `json:"timesheet_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := a.service.ApproveTimesheet(r.Context(), authCtx.OrgID, req.TimesheetID, authCtx); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "timesheet_approved", fmt.Sprintf("{\"timesheet_id\":\"%s\"}", req.TimesheetID))
		writeJSON(w, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/payroll/lock", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageHR)
		if !ok {
			return
		}
		var req struct {
			Period string `json:"period"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := a.service.LockPayroll(r.Context(), authCtx.OrgID, req.Period, authCtx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "payroll_locked", fmt.Sprintf("{\"period\":\"%s\",\"total_gross\":%.2f}", res.Period, res.TotalGrossDollars))
		writeJSON(w, res)
	})

	mux.HandleFunc("/api/shifts", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		shifts, err := a.store.GetShiftsForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, shifts)
	})

	mux.HandleFunc("/api/shifts/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageHR)
		if !ok {
			return
		}
		var req struct {
			ShiftID string `json:"shift_id"`
			Status  string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		actorType := "human"
		if authCtx.IsAPIKey {
			actorType = "agent"
		}
		if err := a.store.UpdateShiftStatusForOrg(authCtx.OrgID, req.ShiftID, req.Status, actorType, authCtx.UserID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "shift_updated", fmt.Sprintf("{\"shift_id\":\"%s\"}", req.ShiftID))
		writeJSON(w, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		tasks, err := a.store.GetTasksForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, tasks)
	})

	mux.HandleFunc("/api/tasks/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		var req struct {
			TaskID string `json:"task_id"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		actorType := "human"
		if authCtx.IsAPIKey {
			actorType = "agent"
		}
		if err := a.store.UpdateTaskStatusForOrg(authCtx.OrgID, req.TaskID, req.Status, actorType, authCtx.UserID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "task_updated", fmt.Sprintf("{\"task_id\":\"%s\"}", req.TaskID))
		writeJSON(w, map[string]string{"status": "ok"})
	})

	// DIGITAL PRESENCE & REPUTATION
	mux.HandleFunc("/api/listings", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		listings, err := a.store.GetListingsForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, listings)
	})

	mux.HandleFunc("/api/listings/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageOrders)
		if !ok {
			return
		}
		var req struct {
			BusinessName string `json:"business_name"`
			Address      string `json:"address"`
			Phone        string `json:"phone"`
			Website      string `json:"website"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		actorType := "human"
		if authCtx.IsAPIKey {
			actorType = "agent"
		}
		if err := a.store.SyncNAPForOrg(authCtx.OrgID, req.BusinessName, req.Address, req.Phone, req.Website, actorType, authCtx.UserID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "nap_synced", "{\"status\":\"synced\"}")
		writeJSON(w, map[string]string{"status": "synced"})
	})

	mux.HandleFunc("/api/reviews", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		revs, err := a.store.GetReviewsForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, revs)
	})

	mux.HandleFunc("/api/reviews/reply", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageOrders)
		if !ok {
			return
		}
		var req struct {
			ReviewID  string `json:"review_id"`
			ReplyText string `json:"reply_text"`
			Publish   bool   `json:"publish"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		status := "drafted_by_agent"
		if req.Publish {
			status = "published"
		}
		actorType := "human"
		if authCtx.IsAPIKey {
			actorType = "agent"
		}
		if err := a.store.ReplyReviewForOrg(authCtx.OrgID, req.ReviewID, req.ReplyText, status, actorType, authCtx.UserID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "review_replied", fmt.Sprintf("{\"review_id\":\"%s\"}", req.ReviewID))
		writeJSON(w, map[string]string{"status": "ok"})
	})

	// APPROVALS & AUDIT
	mux.HandleFunc("/api/approvals", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		status := r.URL.Query().Get("status")
		apprs, err := a.store.GetApprovalsForOrg(authCtx.OrgID, status)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, apprs)
	})

	mux.HandleFunc("/api/approvals/decide", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermApproveOrders)
		if !ok {
			return
		}
		var req struct {
			ApprovalID     string `json:"approval_id"`
			Decision       string `json:"decision"` // approved, rejected
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		idempKey := r.Header.Get("Idempotency-Key")
		if idempKey == "" {
			idempKey = req.IdempotencyKey
		}
		appr, err := a.service.DecideApproval(r.Context(), authCtx.OrgID, req.ApprovalID, req.Decision, authCtx, idempKey)
		if err != nil {
			if errors.Is(err, service.ErrUnauthorizedAction) {
				w.WriteHeader(http.StatusForbidden)
				writeJSON(w, map[string]string{"error": "unauthorized", "message": err.Error()})
				return
			}
			if errors.Is(err, service.ErrApprovalAlreadyDecided) {
				w.WriteHeader(http.StatusConflict)
				writeJSON(w, map[string]string{"error": "conflict", "message": err.Error()})
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "approval_decided", fmt.Sprintf("{\"id\":\"%s\",\"decision\":\"%s\",\"execution\":\"%s\"}", req.ApprovalID, req.Decision, appr.ExecutionStatus))
		writeJSON(w, map[string]interface{}{"status": "ok", "approval": appr})
	})

	mux.HandleFunc("/api/audit-logs", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		logs, err := a.store.GetAuditLogsForOrg(authCtx.OrgID, 50)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, logs)
	})

	// Business Integrity & Reconciliation Scan
	mux.HandleFunc("/api/reconciliation", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermAdmin)
		if !ok {
			return
		}
		report, err := a.service.ReconcileDiscrepancies(r.Context(), authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, report)
	})

	// Database Backup & Integrity Drills
	mux.HandleFunc("/api/backup/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermAdmin)
		if !ok {
			return
		}
		res, err := a.service.CreateBackup(r.Context(), "manual")
		if err != nil {
			http.Error(w, fmt.Sprintf("Backup failed: %v", err), http.StatusInternalServerError)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "backup_completed", fmt.Sprintf("{\"backup_path\":\"%s\",\"size_bytes\":%d,\"integrity_ok\":%t}",
			res.BackupPath, res.SizeBytes, res.IntegrityOK))
		writeJSON(w, res)
	})

	// Emergency Kill Switch Control
	mux.HandleFunc("/api/admin/kill-switch", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermAdmin)
		if !ok {
			return
		}
		orgID := authCtx.OrgID

		if r.Method == "POST" {
			var req struct {
				Active bool `json:"active"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mcp.DefaultKillSwitch().SetOrg(orgID, req.Active)
			a.BroadcastForOrg(orgID, "kill_switch_toggled", fmt.Sprintf("{\"active\":%t,\"org_id\":\"%s\"}", req.Active, orgID))
			_ = a.store.LogAuditForOrg(orgID, "human", authCtx.UserID, "KILL_SWITCH_TOGGLE", "system", "kill_switch",
				fmt.Sprintf("Kill switch set to %t", req.Active), "{}")
			writeJSON(w, map[string]interface{}{"status": "ok", "active": req.Active})
			return
		}

		active := mcp.DefaultKillSwitch().IsActive(orgID)
		writeJSON(w, map[string]interface{}{"kill_switch_active": active, "org_id": orgID})
	})

	// Agent Run History Audit
	mux.HandleFunc("/api/admin/agent-executions", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermAdmin)
		if !ok {
			return
		}
		execs, err := mcp.GetAgentExecutions(r.Context(), a.store.DB(), authCtx.OrgID, 50)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, execs)
	})

	// Document Repository
	mux.HandleFunc("/api/documents", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		orgID := authCtx.OrgID
		switch r.Method {
		case "GET":
			docs, err := a.service.ListDocuments(r.Context(), orgID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, docs)
		case "POST":
			var req struct {
				Title    string `json:"title"`
				Category string `json:"category"`
				Content  string `json:"content"`
				Author   string `json:"author"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.Author == "" {
				req.Author = authCtx.Name
			}
			doc, err := a.service.CreateDocument(r.Context(), orgID, req.Title, req.Category, req.Content, req.Author)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			a.BroadcastForOrg(orgID, "document_created", fmt.Sprintf("{\"id\":\"%s\",\"title\":\"%s\"}", doc.ID, doc.Title))
			writeJSON(w, doc)
		case "DELETE":
			docID := r.URL.Query().Get("id")
			if docID == "" {
				http.Error(w, "missing document id", http.StatusBadRequest)
				return
			}
			if err := a.service.DeleteDocument(r.Context(), orgID, docID); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			a.BroadcastForOrg(orgID, "document_deleted", fmt.Sprintf("{\"id\":\"%s\"}", docID))
			writeJSON(w, map[string]string{"status": "ok"})
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Global Unified Search
	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		query := r.URL.Query().Get("q")
		results, err := a.service.GlobalSearch(r.Context(), authCtx.OrgID, query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, results)
	})

	// CSV Export
	mux.HandleFunc("/api/export", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		entityType := r.URL.Query().Get("type")
		data, err := a.service.ExportCSV(r.Context(), authCtx.OrgID, entityType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s_export.csv", entityType))
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})

	// In-App Notifications
	mux.HandleFunc("/api/notifications", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		unreadOnly := r.URL.Query().Get("unread") == "true"
		notifs, err := a.service.ListNotifications(r.Context(), authCtx.OrgID, unreadOnly)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, notifs)
	})

	mux.HandleFunc("/api/notifications/read", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := a.service.MarkNotificationRead(r.Context(), authCtx.OrgID, req.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	})

	// Organization Profile & Settings
	mux.HandleFunc("/api/organization", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" || r.Method == "PUT" {
			authCtx, ok := a.requireAuth(w, r, auth.PermAdmin)
			if !ok {
				return
			}
			var p service.CompanyProfile
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			p.OrgID = authCtx.OrgID
			if err := a.service.UpdateCompanyProfile(r.Context(), p); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			a.BroadcastForOrg(authCtx.OrgID, "organization_updated", fmt.Sprintf("{\"org_id\":\"%s\",\"name\":\"%s\"}", authCtx.OrgID, p.CompanyName))
			writeJSON(w, map[string]string{"status": "ok"})
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		profile, err := a.service.GetCompanyProfile(r.Context(), authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, profile)
	})
}
