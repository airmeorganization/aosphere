package store

import (
	"encoding/json"
	"path/filepath"
	"testing"

	dbmodel "aosphere/internal/db"
)

func newApprovalTestStore(t *testing.T) (*Store, func()) {
	t.Helper()
	database, err := dbmodel.InitDB(filepath.Join(t.TempDir(), "aosphere-test.db"))
	if err != nil {
		t.Fatalf("init database: %v", err)
	}
	return NewStore(database), func() { _ = database.Close() }
}

func findCreatedApproval(t *testing.T, s *Store, actorID string) (string, string) {
	t.Helper()
	approvals, err := s.GetApprovals("pending")
	if err != nil {
		t.Fatalf("get approvals: %v", err)
	}
	for _, approval := range approvals {
		if approval.ActionType != "create_po" || approval.RequestedByID != actorID {
			continue
		}
		var payload struct {
			PurchaseOrderID string `json:"purchase_order_id"`
		}
		if err := json.Unmarshal([]byte(approval.PayloadJSON), &payload); err != nil {
			t.Fatalf("decode approval payload: %v", err)
		}
		return approval.ID, payload.PurchaseOrderID
	}
	t.Fatalf("could not find pending purchase-order approval for actor %q", actorID)
	return "", ""
}

func TestHighValuePurchaseOrderApprovalTransitionsPO(t *testing.T) {
	for _, decision := range []string{"approved", "rejected"} {
		t.Run(decision, func(t *testing.T) {
			s, cleanup := newApprovalTestStore(t)
			defer cleanup()

			const actorID = "test-agent"
			po, err := s.CreatePurchaseOrder(
				"sup_01",
				`[{"sku_code":"SKU-CORE-100","quantity":30,"unit_cost":50}]`,
				1500,
				"agent",
				actorID,
				"test approval workflow",
			)
			if err != nil {
				t.Fatalf("create purchase order: %v", err)
			}
			if po.Status != "pending_approval" {
				t.Fatalf("expected pending_approval, got %q", po.Status)
			}

			approvalID, linkedPOID := findCreatedApproval(t, s, actorID)
			if linkedPOID != po.ID {
				t.Fatalf("approval linked to %q, want %q", linkedPOID, po.ID)
			}

			if err := s.DecideApproval(approvalID, decision, "test-reviewer"); err != nil {
				t.Fatalf("decide approval: %v", err)
			}

			orders, err := s.GetPurchaseOrders()
			if err != nil {
				t.Fatalf("get purchase orders: %v", err)
			}
			wantStatus := "ordered"
			if decision == "rejected" {
				wantStatus = "cancelled"
			}
			found := false
			for _, order := range orders {
				if order.ID == po.ID {
					found = true
					if order.Status != wantStatus {
						t.Fatalf("purchase order status = %q, want %q", order.Status, wantStatus)
					}
				}
			}
			if !found {
				t.Fatalf("purchase order %q not found after decision", po.ID)
			}

			if err := s.DecideApproval(approvalID, decision, "test-reviewer"); err == nil {
				t.Fatal("expected duplicate decision to fail")
			}
		})
	}
}

func TestPurchaseOrderRejectsUntrustedTotalAndInvalidItems(t *testing.T) {
	s, cleanup := newApprovalTestStore(t)
	defer cleanup()

	tests := []struct {
		name  string
		items string
		total float64
	}{
		{
			name:  "client total mismatch",
			items: `[{"sku_code":"SKU-CORE-100","quantity":30,"unit_cost":50}]`,
			total: 10,
		},
		{
			name:  "zero quantity",
			items: `[{"sku_code":"SKU-CORE-100","quantity":0,"unit_cost":50}]`,
			total: 0,
		},
		{
			name:  "negative unit cost",
			items: `[{"sku_code":"SKU-CORE-100","quantity":1,"unit_cost":-5}]`,
			total: -5,
		},
		{
			name:  "invalid JSON",
			items: `not-json`,
			total: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := s.CreatePurchaseOrder("sup_01", tt.items, tt.total, "agent", "test-agent", ""); err == nil {
				t.Fatal("expected invalid purchase order to be rejected")
			}
		})
	}
}
