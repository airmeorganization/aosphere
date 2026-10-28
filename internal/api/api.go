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
	"aosphere/internal/crm"
	"aosphere/internal/db"
	"aosphere/internal/finance"
	"aosphere/internal/ledger"
	"aosphere/internal/mcp"
	"aosphere/internal/metrics"
	"aosphere/internal/pdf"
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
	crm       *crm.CRMService
	env       string
	clients   map[chan string]string // map[msgChan]orgID
	clientsMu sync.RWMutex
}

func NewAPI(s *store.Store, svc *service.WorkflowService, env string) *API {
	if svc == nil {
		svc = service.NewWorkflowService(s)
	}
	var crmSvc *crm.CRMService
	if s != nil && s.DB() != nil {
		crmSvc = crm.NewCRMService(s.DB(), ledger.NewLedgerService(s.DB()))
	}
	return &API{
		store:   s,
		service: svc,
		crm:     crmSvc,
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

	// SUPPLY CHAIN & MULTI-WAREHOUSE
	mux.HandleFunc("/api/warehouses", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authCtx, ok := a.requireAuth(w, r, auth.PermManageInventory)
			if !ok {
				return
			}
			var req struct {
				Name    string `json:"name"`
				Code    string `json:"code"`
				Address string `json:"address"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Code) == "" {
				http.Error(w, "name and code are required", http.StatusBadRequest)
				return
			}
			actorType := "human"
			if authCtx.IsAPIKey {
				actorType = "agent"
			}
			wh, err := a.store.CreateWarehouseForOrg(authCtx.OrgID, req.Name, req.Code, req.Address, actorType, authCtx.UserID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			a.BroadcastForOrg(authCtx.OrgID, "warehouse_created", fmt.Sprintf("{\"id\":\"%s\",\"code\":\"%s\"}", wh.ID, wh.Code))
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, wh)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		whs, err := a.store.GetWarehousesForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, whs)
	})

	mux.HandleFunc("/api/warehouse-stocks", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		stocks, err := a.store.GetWarehouseStocksForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, stocks)
	})

	mux.HandleFunc("/api/grns", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authCtx, ok := a.requireAuth(w, r, auth.PermManageInventory)
			if !ok {
				return
			}
			var req service.ReceiveGRNRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			idempKey := r.Header.Get("Idempotency-Key")
			res, err := a.service.ReceiveGoodsReceiptNote(r.Context(), authCtx.OrgID, req, authCtx, idempKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			a.BroadcastForOrg(authCtx.OrgID, "grn_received", fmt.Sprintf("{\"grn_number\":\"%s\",\"po_number\":\"%s\"}", res.GRN.GRNNumber, res.PurchaseOrder.PONumber))
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, res)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		grns, err := a.store.GetGRNsForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, grns)
	})

	mux.HandleFunc("/api/supplier-bills", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authCtx, ok := a.requireAuth(w, r, auth.PermManageFinance)
			if !ok {
				return
			}
			var req struct {
				SupplierID string  `json:"supplier_id"`
				POID       string  `json:"po_id"`
				Amount     float64 `json:"amount"`
				ItemsJSON  string  `json:"items_json"`
				Notes      string  `json:"notes"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.SupplierID == "" || req.Amount <= 0 {
				http.Error(w, "supplier_id and positive amount are required", http.StatusBadRequest)
				return
			}
			actorType := "human"
			if authCtx.IsAPIKey {
				actorType = "agent"
			}
			bill, err := a.store.CreateSupplierBillForOrg(authCtx.OrgID, req.SupplierID, req.POID, req.Amount, req.ItemsJSON, req.Notes, actorType, authCtx.UserID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			a.BroadcastForOrg(authCtx.OrgID, "supplier_bill_created", fmt.Sprintf("{\"bill_number\":\"%s\"}", bill.BillNumber))
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, bill)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		bills, err := a.store.GetSupplierBillsForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, bills)
	})

	mux.HandleFunc("/api/supplier-bills/match", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageFinance)
		if !ok {
			return
		}
		var req struct {
			BillID string `json:"bill_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := a.service.PerformThreeWayMatch(r.Context(), authCtx.OrgID, req.BillID, authCtx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "bill_matched", fmt.Sprintf("{\"bill_id\":\"%s\",\"status\":\"%s\"}", req.BillID, res.Status))
		writeJSON(w, res)
	})

	mux.HandleFunc("/api/stock-transfers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authCtx, ok := a.requireAuth(w, r, auth.PermManageInventory)
			if !ok {
				return
			}
			var req service.CreateStockTransferRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			xfer, err := a.service.TransferStock(r.Context(), authCtx.OrgID, req, authCtx)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			a.BroadcastForOrg(authCtx.OrgID, "stock_transferred", fmt.Sprintf("{\"transfer_number\":\"%s\"}", xfer.TransferNumber))
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, xfer)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		xfers, err := a.store.GetStockTransfersForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, xfers)
	})

	mux.HandleFunc("/api/batch-lots", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authCtx, ok := a.requireAuth(w, r, auth.PermManageInventory)
			if !ok {
				return
			}
			var req struct {
				SKUID       string `json:"sku_id"`
				BatchNumber string `json:"batch_number"`
				ExpiryDate  string `json:"expiry_date"`
				Quantity    int    `json:"quantity"`
				WarehouseID string `json:"warehouse_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.SKUID == "" || req.BatchNumber == "" || req.Quantity <= 0 {
				http.Error(w, "sku_id, batch_number, and quantity > 0 are required", http.StatusBadRequest)
				return
			}
			actorType := "human"
			if authCtx.IsAPIKey {
				actorType = "agent"
			}
			lot, err := a.store.CreateBatchLotForOrg(authCtx.OrgID, req.SKUID, req.BatchNumber, req.ExpiryDate, req.Quantity, req.WarehouseID, actorType, authCtx.UserID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, lot)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		lots, err := a.store.GetBatchLotsForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, lots)
	})

	// CRM
	mux.HandleFunc("/api/customers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authCtx, ok := a.requireAuth(w, r, auth.PermManageOrders)
			if !ok {
				return
			}
			var req struct {
				Name    string `json:"name"`
				Company string `json:"company"`
				Email   string `json:"email"`
				Phone   string `json:"phone"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.Name == "" {
				http.Error(w, "customer name is required", http.StatusBadRequest)
				return
			}
			cust, err := a.store.CreateCustomerForOrg(authCtx.OrgID, req.Name, req.Company, req.Email, req.Phone, "active", 0, "[]")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = a.store.LogAuditForOrg(authCtx.OrgID, "human", authCtx.UserID, "CUSTOMER_CREATE", "customers", cust.ID, "Created customer "+cust.Name, "{}")
			writeJSON(w, cust)
			return
		}

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

	// PHASE 5: MULTI-PIPELINE CRM, CUSTOMER 360 & PORTAL
	mux.HandleFunc("/api/pipelines", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		if a.crm == nil {
			http.Error(w, "CRM service unavailable", http.StatusServiceUnavailable)
			return
		}
		pipes, err := a.crm.ListPipelines(r.Context(), authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, pipes)
	})

	mux.HandleFunc("/api/deals/decay-alerts", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		if a.crm == nil {
			http.Error(w, "CRM service unavailable", http.StatusServiceUnavailable)
			return
		}
		alerts, err := a.crm.GetDealDecayAlerts(r.Context(), authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, alerts)
	})

	mux.HandleFunc("/api/customers/360", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		customerID := r.URL.Query().Get("id")
		if customerID == "" {
			http.Error(w, "missing customer id parameter", http.StatusBadRequest)
			return
		}
		if a.crm == nil {
			http.Error(w, "CRM service unavailable", http.StatusServiceUnavailable)
			return
		}
		hub, err := a.crm.GetCustomer360(r.Context(), authCtx.OrgID, customerID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, hub)
	})

	mux.HandleFunc("/api/customers/activities", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageOrders)
		if !ok {
			return
		}
		var act crm.CustomerActivity
		if err := json.NewDecoder(r.Body).Decode(&act); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if act.CustomerID == "" {
			http.Error(w, "customer_id is required", http.StatusBadRequest)
			return
		}
		if authCtx != nil {
			act.AuthorID = authCtx.UserID
			act.AuthorType = string(authCtx.Role)
		}
		if a.crm == nil {
			http.Error(w, "CRM service unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := a.crm.LogActivity(r.Context(), authCtx.OrgID, act.CustomerID, act); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "customer_activity_logged", fmt.Sprintf("{\"customer_id\":\"%s\",\"type\":\"%s\"}", act.CustomerID, act.ActivityType))
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]string{"status": "ok"})
	})

	// Public Webhook for Inbound Lead Capture (no auth required)
	mux.HandleFunc("/api/v1/leads/capture", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var lead crm.InboundLead
		if err := json.NewDecoder(r.Body).Decode(&lead); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(lead.Name) == "" || strings.TrimSpace(lead.Email) == "" {
			http.Error(w, "name and email are required", http.StatusBadRequest)
			return
		}
		if a.crm == nil {
			http.Error(w, "CRM service unavailable", http.StatusServiceUnavailable)
			return
		}
		ingested, err := a.crm.IngestInboundLead(r.Context(), "org_default", lead)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.BroadcastForOrg("org_default", "lead_captured", fmt.Sprintf("{\"lead_id\":\"%s\",\"name\":\"%s\"}", ingested.ID, ingested.Name))
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]interface{}{"status": "received", "id": ingested.ID, "lead_id": ingested.ID, "lead": ingested})
	})

	mux.HandleFunc("/api/leads", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		rows, err := a.store.DB().QueryContext(r.Context(),
			`SELECT id, org_id, name, email, company, phone, message, source, status, assigned_to, COALESCE(converted_customer_id, ''), COALESCE(converted_deal_id, ''), created_at
			FROM inbound_leads WHERE org_id = ? ORDER BY created_at DESC`, authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var leads []crm.InboundLead
		for rows.Next() {
			var l crm.InboundLead
			var cAt string
			if err := rows.Scan(&l.ID, &l.OrgID, &l.Name, &l.Email, &l.Company, &l.Phone, &l.Message, &l.Source, &l.Status, &l.AssignedTo, &l.ConvertedCustomerID, &l.ConvertedDealID, &cAt); err == nil {
				l.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", cAt)
				leads = append(leads, l)
			}
		}
		writeJSON(w, leads)
	})

	mux.HandleFunc("/api/leads/convert", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageOrders)
		if !ok {
			return
		}
		var req struct {
			LeadID     string  `json:"lead_id"`
			PipelineID string  `json:"pipeline_id"`
			DealValue  float64 `json:"deal_value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if a.crm == nil {
			http.Error(w, "CRM service unavailable", http.StatusServiceUnavailable)
			return
		}
		custID, dealID, err := a.crm.ConvertLead(r.Context(), authCtx.OrgID, req.LeadID, req.PipelineID, req.DealValue)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "lead_converted", fmt.Sprintf("{\"lead_id\":\"%s\",\"customer_id\":\"%s\",\"deal_id\":\"%s\"}", req.LeadID, custID, dealID))
		writeJSON(w, map[string]string{"customer_id": custID, "deal_id": dealID})
	})

	mux.HandleFunc("/api/invoices/send-email", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageFinance)
		if !ok {
			return
		}
		var req struct {
			InvoiceID string `json:"invoice_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if a.crm == nil {
			http.Error(w, "CRM service unavailable", http.StatusServiceUnavailable)
			return
		}
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)
		eml, err := a.crm.SendInvoiceEmail(r.Context(), authCtx.OrgID, req.InvoiceID, baseURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "email_dispatched", fmt.Sprintf("{\"email_id\":\"%s\",\"recipient\":\"%s\"}", eml.ID, eml.RecipientEmail))
		writeJSON(w, eml)
	})

	mux.HandleFunc("/api/emails/outbox", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		rows, err := a.store.DB().QueryContext(r.Context(),
			`SELECT id, org_id, recipient_email, recipient_name, subject, body_html, body_text, template_name, related_entity_type, related_entity_id, status, error_message, sent_at, created_at
			FROM email_outbox WHERE org_id = ? ORDER BY sent_at DESC LIMIT 50`, authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var emails []crm.EmailOutboxEntry
		for rows.Next() {
			var e crm.EmailOutboxEntry
			var sAt, cAt string
			if err := rows.Scan(&e.ID, &e.OrgID, &e.RecipientEmail, &e.RecipientName, &e.Subject, &e.BodyHTML, &e.BodyText, &e.TemplateName, &e.RelatedEntityType, &e.RelatedEntityID, &e.Status, &e.ErrorMessage, &sAt, &cAt); err == nil {
				e.SentAt, _ = time.Parse("2006-01-02 15:04:05", sAt)
				e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", cAt)
				emails = append(emails, e)
			}
		}
		writeJSON(w, emails)
	})

	// Client Self-Service Portal APIs (public, authenticated via secure cryptographic token)
	mux.HandleFunc("/api/portal/invoice/", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.URL.Path, "/api/portal/invoice/")
		if token == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}
		if a.crm == nil {
			http.Error(w, "CRM service unavailable", http.StatusServiceUnavailable)
			return
		}
		view, err := a.crm.GetPortalInvoice(r.Context(), token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, view)
	})

	mux.HandleFunc("/api/portal/invoice/pay", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Token         string `json:"token"`
			PaymentMethod string `json:"payment_method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.PaymentMethod == "" {
			req.PaymentMethod = "credit_card"
		}
		if a.crm == nil {
			http.Error(w, "CRM service unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := a.crm.PayPortalInvoice(r.Context(), req.Token, req.PaymentMethod); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.Broadcast("invoice_paid", fmt.Sprintf("{\"token\":\"%s\"}", req.Token))
		writeJSON(w, map[string]string{"status": "paid"})
	})

	// Portal UI Page serving web/portal.html
	mux.HandleFunc("/portal/invoice/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/portal.html")
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

	// PDF GENERATION: INVOICES
	mux.HandleFunc("/api/invoices/pdf", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		invID := r.URL.Query().Get("id")
		if invID == "" {
			http.Error(w, "missing invoice id", http.StatusBadRequest)
			return
		}
		invs, err := a.store.GetInvoicesForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var targetInv *db.Invoice
		for _, inv := range invs {
			if inv.ID == invID || inv.InvoiceNumber == invID {
				targetInv = &inv
				break
			}
		}
		if targetInv == nil {
			http.Error(w, "invoice not found", http.StatusNotFound)
			return
		}

		var cust db.Customer
		custs, _ := a.store.GetCustomersForOrg(authCtx.OrgID)
		for _, c := range custs {
			if c.ID == targetInv.CustomerID {
				cust = c
				break
			}
		}

		comp, _ := a.service.GetCompanyProfile(r.Context(), authCtx.OrgID)
		if comp == nil {
			comp = &db.CompanyProfile{
				CompanyName: "AO Sphere Enterprise",
				Address:     "100 Innovation Way, Suite 400",
				Phone:       "+1 (555) 019-2834",
			}
		}

		curCode := r.URL.Query().Get("currency")
		if curCode == "" {
			curCode = comp.Currency
		}
		if curCode == "" {
			curCode = "USD"
		}
		curInfo := finance.GetCurrency(curCode)

		pdfBytes, err := pdf.GenerateInvoicePDF(*targetInv, cust, *comp, curInfo)
		if err != nil {
			http.Error(w, "failed to generate invoice PDF: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"invoice-%s.pdf\"", targetInv.InvoiceNumber))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pdfBytes)
	})

	// PDF GENERATION: PURCHASE ORDERS
	mux.HandleFunc("/api/purchase-orders/pdf", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		poID := r.URL.Query().Get("id")
		if poID == "" {
			http.Error(w, "missing purchase order id", http.StatusBadRequest)
			return
		}
		pos, err := a.store.GetPurchaseOrdersForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var targetPO *db.PurchaseOrder
		for _, p := range pos {
			if p.ID == poID || p.PONumber == poID {
				targetPO = &p
				break
			}
		}
		if targetPO == nil {
			http.Error(w, "purchase order not found", http.StatusNotFound)
			return
		}

		var sup db.Supplier
		sups, _ := a.store.GetSuppliersForOrg(authCtx.OrgID)
		for _, s := range sups {
			if s.ID == targetPO.SupplierID {
				sup = s
				break
			}
		}

		comp, _ := a.service.GetCompanyProfile(r.Context(), authCtx.OrgID)
		if comp == nil {
			comp = &db.CompanyProfile{
				CompanyName: "AO Sphere Enterprise",
				Address:     "100 Innovation Way, Suite 400",
			}
		}

		curCode := r.URL.Query().Get("currency")
		if curCode == "" {
			curCode = "USD"
		}
		curInfo := finance.GetCurrency(curCode)

		pdfBytes, err := pdf.GeneratePurchaseOrderPDF(*targetPO, sup, *comp, curInfo)
		if err != nil {
			http.Error(w, "failed to generate purchase order PDF: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"po-%s.pdf\"", targetPO.PONumber))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pdfBytes)
	})

	// PDF GENERATION: CREDIT NOTES
	mux.HandleFunc("/api/credit-notes/pdf", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		cnID := r.URL.Query().Get("id")
		if cnID == "" {
			http.Error(w, "missing credit note id", http.StatusBadRequest)
			return
		}
		cn, err := a.store.GetCreditNoteByIDForOrg(authCtx.OrgID, cnID)
		if err != nil || cn == nil {
			http.Error(w, "credit note not found", http.StatusNotFound)
			return
		}

		var cust db.Customer
		custs, _ := a.store.GetCustomersForOrg(authCtx.OrgID)
		for _, c := range custs {
			if c.ID == cn.CustomerID {
				cust = c
				break
			}
		}

		comp, _ := a.service.GetCompanyProfile(r.Context(), authCtx.OrgID)
		if comp == nil {
			comp = &db.CompanyProfile{CompanyName: "AO Sphere Enterprise"}
		}
		curInfo := finance.GetCurrency(cn.Currency)

		pdfBytes, err := pdf.GenerateCreditNotePDF(*cn, cust, *comp, curInfo)
		if err != nil {
			http.Error(w, "failed to generate credit note PDF: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"credit-note-%s.pdf\"", cn.CreditNoteNumber))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pdfBytes)
	})

	// CREDIT NOTES CRUD
	mux.HandleFunc("/api/credit-notes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authCtx, ok := a.requireAuth(w, r, auth.PermManageFinance)
			if !ok {
				return
			}
			var req service.CreateCreditNoteRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			idempKey := r.Header.Get("Idempotency-Key")
			cn, err := a.service.IssueCreditNote(r.Context(), authCtx.OrgID, req, authCtx, idempKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			a.BroadcastForOrg(authCtx.OrgID, "credit_note_issued", fmt.Sprintf("{\"credit_note_number\":\"%s\",\"total\":%.2f}", cn.CreditNoteNumber, cn.TotalAmount))
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, cn)
			return
		}

		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		notes, err := a.store.GetCreditNotesForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, notes)
	})

	// GENERAL LEDGER: CHART OF ACCOUNTS
	mux.HandleFunc("/api/ledger/accounts", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		accounts, err := a.store.GetAccountsForOrg(authCtx.OrgID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, accounts)
	})

	// GENERAL LEDGER: JOURNAL ENTRIES
	mux.HandleFunc("/api/ledger/entries", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		entries, err := a.store.GetJournalEntriesForOrg(authCtx.OrgID, 100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, entries)
	})

	// MULTI-CURRENCY: ALL SUPPORTED ASIAN & GLOBAL CURRENCIES
	mux.HandleFunc("/api/currencies", func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := a.requireAuth(w, r, auth.PermRead)
		if !ok {
			return
		}
		rates, _ := a.store.GetExchangeRatesForOrg(authCtx.OrgID)
		ratesMap := make(map[string]float64)
		for _, r := range rates {
			ratesMap[r.TargetCurrency] = r.Rate
		}

		allCurs := finance.GetAllCurrencies()
		for i := range allCurs {
			if liveRate, ok := ratesMap[allCurs[i].Code]; ok && liveRate > 0 {
				allCurs[i].DefaultRate = liveRate
			}
		}

		writeJSON(w, map[string]interface{}{
			"base_currency": "USD",
			"currencies":    allCurs,
			"rates":         ratesMap,
		})
	})

	mux.HandleFunc("/api/currencies/rate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authCtx, ok := a.requireAuth(w, r, auth.PermManageFinance)
		if !ok {
			return
		}
		var req struct {
			BaseCurrency   string  `json:"base_currency"`
			TargetCurrency string  `json:"target_currency"`
			Rate           float64 `json:"rate"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.TargetCurrency == "" || req.Rate <= 0 {
			http.Error(w, "target_currency and positive rate required", http.StatusBadRequest)
			return
		}
		if req.BaseCurrency == "" {
			req.BaseCurrency = "USD"
		}
		if err := a.store.SetExchangeRateForOrg(authCtx.OrgID, req.BaseCurrency, req.TargetCurrency, req.Rate); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.BroadcastForOrg(authCtx.OrgID, "exchange_rate_updated", fmt.Sprintf("{\"currency\":\"%s\",\"rate\":%.4f}", req.TargetCurrency, req.Rate))
		writeJSON(w, map[string]interface{}{"status": "ok", "currency": req.TargetCurrency, "rate": req.Rate})
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
			if strings.TrimSpace(req.Name) == "" {
				http.Error(w, "name is required", http.StatusBadRequest)
				return
			}
			actorType := "human"
			if authCtx.IsAPIKey {
				actorType = "agent"
			}
			emp, err := a.store.CreateEmployeeForOrg(authCtx.OrgID, req.Name, req.Role, req.Department, req.Email, req.Phone, req.HourlyRate, actorType, authCtx.UserID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			a.BroadcastForOrg(authCtx.OrgID, "employee_added", fmt.Sprintf("{\"id\":\"%s\",\"name\":\"%s\"}", emp.ID, emp.Name))
			w.WriteHeader(http.StatusCreated)
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
		if r.Method == http.MethodPost {
			authCtx, ok := a.requireAuth(w, r, auth.PermManageHR)
			if !ok {
				return
			}
			var req struct {
				EmployeeID string `json:"employee_id"`
				ShiftDate  string `json:"shift_date"`
				StartTime  string `json:"start_time"`
				EndTime    string `json:"end_time"`
				Station    string `json:"station"`
				Status     string `json:"status"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(req.ShiftDate) == "" {
				http.Error(w, "shift_date is required", http.StatusBadRequest)
				return
			}
			actorType := "human"
			if authCtx.IsAPIKey {
				actorType = "agent"
			}
			shf, err := a.store.CreateShiftForOrg(authCtx.OrgID, req.EmployeeID, req.ShiftDate, req.StartTime, req.EndTime, req.Station, req.Status, actorType, authCtx.UserID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			a.BroadcastForOrg(authCtx.OrgID, "shift_created", fmt.Sprintf("{\"id\":\"%s\",\"shift_date\":\"%s\"}", shf.ID, shf.ShiftDate))
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, shf)
			return
		}
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
		if r.Method == http.MethodPost {
			authCtx, ok := a.requireAuth(w, r, auth.PermRead)
			if !ok {
				return
			}
			var req struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				ProjectName string `json:"project_name"`
				AssigneeID  string `json:"assignee_id"`
				Priority    string `json:"priority"`
				Status      string `json:"status"`
				DueDate     string `json:"due_date"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(req.Title) == "" {
				http.Error(w, "title is required", http.StatusBadRequest)
				return
			}
			actorType := "human"
			if authCtx.IsAPIKey {
				actorType = "agent"
			}
			tsk, err := a.store.CreateTaskForOrg(authCtx.OrgID, req.Title, req.Description, req.ProjectName, req.AssigneeID, req.Priority, req.DueDate, req.Status, actorType, authCtx.UserID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			a.BroadcastForOrg(authCtx.OrgID, "task_created", fmt.Sprintf("{\"id\":\"%s\",\"title\":\"%s\"}", tsk.ID, tsk.Title))
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, tsk)
			return
		}
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
