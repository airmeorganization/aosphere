package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"aosphere/internal/store"
)

type API struct {
	store      *store.Store
	env        string
	clients    map[chan string]bool
	clientsMu  sync.RWMutex
}

func NewAPI(s *store.Store, env string) *API {
	return &API{
		store:   s,
		env:     env,
		clients: make(map[chan string]bool),
	}
}

// Broadcast sends an SSE event to all connected web UI browsers
func (a *API) Broadcast(eventType, payload string) {
	a.clientsMu.RLock()
	defer a.clientsMu.RUnlock()

	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, payload)
	for ch := range a.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (a *API) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	msgChan := make(chan string, 16)
	a.clientsMu.Lock()
	a.clients[msgChan] = true
	a.clientsMu.Unlock()

	defer func() {
		a.clientsMu.Lock()
		delete(a.clients, msgChan)
		a.clientsMu.Unlock()
		close(msgChan)
	}()

	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\",\"time\":\"%s\"}\n\n", time.Now().Format(time.RFC3339))
	flusher.Flush()

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
	// Health & Diagnostics Endpoints
	mux.HandleFunc("/healthz", a.HandleHealthz)
	mux.HandleFunc("/readyz", a.HandleReadyz)
	mux.HandleFunc("/api/version", a.HandleInfo)
	mux.HandleFunc("/api/v1/info", a.HandleInfo)

	// SSE Live Feed
	mux.HandleFunc("/api/events", a.HandleSSE)

	// ERP
	mux.HandleFunc("/api/skus", func(w http.ResponseWriter, r *http.Request) {
		below := r.URL.Query().Get("below_threshold") == "true"
		items, err := a.store.GetSKUs(below)
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
		var req struct {
			SKUID  string `json:"sku_id"`
			Delta  int    `json:"delta"`
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		updated, err := a.store.UpdateStock(req.SKUID, req.Delta, req.Reason, "human", "web_user")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.Broadcast("stock_updated", fmt.Sprintf("{\"sku_id\":\"%s\",\"new_stock\":%d}", updated.ID, updated.StockQuantity))
		writeJSON(w, updated)
	})

	mux.HandleFunc("/api/suppliers", func(w http.ResponseWriter, r *http.Request) {
		sups, err := a.store.GetSuppliers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, sups)
	})

	mux.HandleFunc("/api/purchase-orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var req struct {
				SupplierID string `json:"supplier_id"`
				ItemsJSON  string `json:"items_json"`
				Total      float64 `json:"total"`
				Notes      string  `json:"notes"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			po, err := a.store.CreatePurchaseOrder(req.SupplierID, req.ItemsJSON, req.Total, "human", "web_user", req.Notes)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			a.Broadcast("po_created", fmt.Sprintf("{\"po_number\":\"%s\"}", po.PONumber))
			writeJSON(w, po)
			return
		}
		pos, err := a.store.GetPurchaseOrders()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, pos)
	})

	// CRM
	mux.HandleFunc("/api/customers", func(w http.ResponseWriter, r *http.Request) {
		custs, err := a.store.GetCustomers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, custs)
	})

	mux.HandleFunc("/api/deals", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
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
			deal, err := a.store.CreateDeal(req.Title, req.CustomerID, req.Value, req.Stage, 60, req.CloseDate, "human", "web_user")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			a.Broadcast("deal_created", fmt.Sprintf("{\"deal_id\":\"%s\"}", deal.ID))
			writeJSON(w, deal)
			return
		}
		deals, err := a.store.GetDeals()
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
		var req struct {
			DealID string `json:"deal_id"`
			Stage  string `json:"stage"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := a.store.UpdateDealStage(req.DealID, req.Stage, "human", "web_user"); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.Broadcast("deal_stage_changed", fmt.Sprintf("{\"deal_id\":\"%s\",\"stage\":\"%s\"}", req.DealID, req.Stage))
		writeJSON(w, map[string]string{"status": "ok"})
	})

	// FINANCIAL
	mux.HandleFunc("/api/invoices", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var req struct {
				CustomerID string  `json:"customer_id"`
				ItemsJSON  string  `json:"items_json"`
				Subtotal   float64 `json:"subtotal"`
				Tax        float64 `json:"tax"`
				DueDate    string  `json:"due_date"`
				Recurring  string  `json:"recurring"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			inv, err := a.store.CreateInvoice(req.CustomerID, req.ItemsJSON, req.Subtotal, req.Tax, req.DueDate, req.Recurring, "human", "web_user")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			a.Broadcast("invoice_created", fmt.Sprintf("{\"invoice_number\":\"%s\"}", inv.InvoiceNumber))
			writeJSON(w, inv)
			return
		}
		invs, err := a.store.GetInvoices()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, invs)
	})

	mux.HandleFunc("/api/financial-summary", func(w http.ResponseWriter, r *http.Request) {
		summary, err := a.store.GetFinancialSummary()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, summary)
	})

	mux.HandleFunc("/api/expenses", func(w http.ResponseWriter, r *http.Request) {
		exps, err := a.store.GetExpenses()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, exps)
	})

	// HRMS & PRODUCTIVITY
	mux.HandleFunc("/api/employees", func(w http.ResponseWriter, r *http.Request) {
		emps, err := a.store.GetEmployees()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, emps)
	})

	mux.HandleFunc("/api/shifts", func(w http.ResponseWriter, r *http.Request) {
		shifts, err := a.store.GetShifts()
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
		var req struct {
			ShiftID string `json:"shift_id"`
			Status  string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := a.store.UpdateShiftStatus(req.ShiftID, req.Status, "human", "web_user"); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.Broadcast("shift_updated", fmt.Sprintf("{\"shift_id\":\"%s\"}", req.ShiftID))
		writeJSON(w, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		tasks, err := a.store.GetTasks()
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
		var req struct {
			TaskID string `json:"task_id"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := a.store.UpdateTaskStatus(req.TaskID, req.Status, "human", "web_user"); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.Broadcast("task_updated", fmt.Sprintf("{\"task_id\":\"%s\"}", req.TaskID))
		writeJSON(w, map[string]string{"status": "ok"})
	})

	// DIGITAL PRESENCE & REPUTATION
	mux.HandleFunc("/api/listings", func(w http.ResponseWriter, r *http.Request) {
		listings, err := a.store.GetListings()
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
		if err := a.store.SyncNAP(req.BusinessName, req.Address, req.Phone, req.Website, "human", "web_user"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.Broadcast("nap_synced", "{\"status\":\"synced\"}")
		writeJSON(w, map[string]string{"status": "synced"})
	})

	mux.HandleFunc("/api/reviews", func(w http.ResponseWriter, r *http.Request) {
		revs, err := a.store.GetReviews()
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
		if err := a.store.ReplyReview(req.ReviewID, req.ReplyText, status, "human", "web_user"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.Broadcast("review_replied", fmt.Sprintf("{\"review_id\":\"%s\"}", req.ReviewID))
		writeJSON(w, map[string]string{"status": "ok"})
	})

	// APPROVALS & AUDIT
	mux.HandleFunc("/api/approvals", func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		apprs, err := a.store.GetApprovals(status)
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
		var req struct {
			ApprovalID string `json:"approval_id"`
			Decision   string `json:"decision"` // approved, rejected
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := a.store.DecideApproval(req.ApprovalID, req.Decision, "admin"); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.Broadcast("approval_decided", fmt.Sprintf("{\"id\":\"%s\",\"decision\":\"%s\"}", req.ApprovalID, req.Decision))
		writeJSON(w, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/api/audit-logs", func(w http.ResponseWriter, r *http.Request) {
		logs, err := a.store.GetAuditLogs(50)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, logs)
	})
}
