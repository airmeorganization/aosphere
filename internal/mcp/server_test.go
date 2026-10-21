package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aosphere/internal/auth"
	"aosphere/internal/db"
	"aosphere/internal/store"
)

func setupTestMCPServer(t *testing.T) (*MCPServer, *store.Store, *sql.DB, func()) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_mcp.db")

	database, err := db.InitDB(dbPath, true)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}

	appStore := store.NewStore(database)
	server := NewMCPServer(appStore, nil)

	cleanup := func() {
		database.Close()
	}
	return server, appStore, database, cleanup
}

func TestMCPServer_Authentication(t *testing.T) {
	server, _, _, cleanup := setupTestMCPServer(t)
	defer cleanup()

	// 1. Unauthenticated SSE request -> 401 Unauthorized
	reqUnauth := httptest.NewRequest(http.MethodGet, "/mcp/sse", nil)
	rrUnauth := httptest.NewRecorder()
	server.HandleSSE(rrUnauth, reqUnauth)

	if rrUnauth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated SSE, got %d", rrUnauth.Code)
	}

	// 2. Request with invalid key -> 401 Unauthorized
	reqBadKey := httptest.NewRequest(http.MethodGet, "/mcp/sse", nil)
	reqBadKey.Header.Set("Authorization", "Bearer ao_sk_invalid_fake_key_99999")
	rrBadKey := httptest.NewRecorder()
	server.HandleSSE(rrBadKey, reqBadKey)

	if rrBadKey.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid API key, got %d", rrBadKey.Code)
	}

	// 3. Request with valid seeded key -> Returns HTTP 200 with text/event-stream
	validKey := "ao_sk_agent_default_token_12345"
	reqAuth := httptest.NewRequest(http.MethodGet, "/mcp/sse?api_key="+validKey, nil)

	// Test authenticateRequest directly for clean verification
	authCtx, agentName, err := server.authenticateRequest(reqAuth)
	if err != nil {
		t.Fatalf("expected valid authentication, got error: %v", err)
	}
	if authCtx.OrgID != "org_default" || authCtx.Role != auth.RoleOperations || agentName != "Primary Agent Client" {
		t.Errorf("unexpected auth context: %+v, agent: %s", authCtx, agentName)
	}
}

func TestMCPServer_RBACPermissionEnforcement(t *testing.T) {
	server, appStore, _, cleanup := setupTestMCPServer(t)
	defer cleanup()

	// Create Read-Only API client session
	readOnlyCtx := &SessionContext{
		ID:      "sess_readonly",
		AgentID: "agent_auditor",
		AuthCtx: &auth.AuthContext{
			UserID:   "usr_auditor",
			OrgID:    "org_default",
			Email:    "auditor@aosphere.local",
			Name:     "auditor_bot",
			Role:     auth.RoleReadOnly,
			IsAPIKey: true,
		},
	}

	// 1. Read-Only client calling erp_check_stock -> MUST SUCCEED
	reqStock := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"erp_check_stock","arguments":{"below_threshold_only":false}}`),
	}

	respStock := server.ProcessRequestWithContext(reqStock, readOnlyCtx)
	if respStock.Error != nil {
		t.Fatalf("expected stock check success, got error: %v", respStock.Error)
	}
	resMap, _ := respStock.Result.(map[string]interface{})
	if isErr, _ := resMap["isError"].(bool); isErr {
		t.Fatalf("stock check marked as error: %+v", resMap)
	}

	// 2. Read-Only client calling erp_adjust_stock -> MUST BE DENIED BY RBAC
	reqAdjust := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"erp_adjust_stock","arguments":{"sku_id":"sku_01","delta":10,"reason":"Auditor adjustment attempt"}}`),
	}

	respAdjust := server.ProcessRequestWithContext(reqAdjust, readOnlyCtx)
	resAdjustMap, _ := respAdjust.Result.(map[string]interface{})
	if isErr, _ := resAdjustMap["isError"].(bool); !isErr {
		t.Errorf("expected permission denied isError=true for read-only client adjusting stock")
	}

	// Verify permission denied text in content
	contentList, _ := resAdjustMap["content"].([]map[string]interface{})
	if len(contentList) == 0 || !strings.Contains(contentList[0]["text"].(string), "Permission denied") {
		t.Errorf("expected Permission denied in response content, got: %+v", resAdjustMap)
	}

	// 3. Operations client calling erp_adjust_stock -> MUST SUCCEED
	opsCtx := &SessionContext{
		ID:      "sess_ops",
		AgentID: "agent_ops",
		AuthCtx: &auth.AuthContext{
			UserID:   "usr_ops",
			OrgID:    "org_default",
			Email:    "ops@aosphere.local",
			Name:     "warehouse_bot",
			Role:     auth.RoleOperations,
			IsAPIKey: true,
		},
	}

	respOpsAdjust := server.ProcessRequestWithContext(reqAdjust, opsCtx)
	resOpsMap, _ := respOpsAdjust.Result.(map[string]interface{})
	if isErr, _ := resOpsMap["isError"].(bool); isErr {
		t.Fatalf("operations client adjustment failed: %+v", resOpsMap)
	}

	// 4. Verify audit log entry was created for both calls
	logs, err := appStore.GetAuditLogsForOrg("org_default", 10)
	if err != nil {
		t.Fatalf("GetAuditLogsForOrg failed: %v", err)
	}
	hasForbiddenLog := false
	hasSuccessLog := false
	for _, l := range logs {
		if l.Action == "MCP_TOOL_FORBIDDEN" {
			hasForbiddenLog = true
		}
		if l.Action == "MCP_TOOL_CALL" && l.EntityID == "erp_adjust_stock" {
			hasSuccessLog = true
		}
	}
	if !hasForbiddenLog {
		t.Errorf("expected audit log record for mcp_tool_forbidden")
	}
	if !hasSuccessLog {
		t.Errorf("expected audit log record for mcp_tool_call")
	}
}

func TestMCPServer_TenantIsolation(t *testing.T) {
	server, appStore, database, cleanup := setupTestMCPServer(t)
	defer cleanup()

	// Create Tenant 2
	org, err := appStore.CreateOrganization("Titan Corp", "titan")
	if err != nil {
		t.Fatalf("failed to create org: %v", err)
	}

	// Add an SKU exclusively to Titan Corp
	_, err = database.Exec(
		`INSERT INTO skus (id, org_id, code, name, category, stock_quantity, min_threshold, unit_cost, selling_price)
		 VALUES ('sku_titan_01', ?, 'TITAN-HEAVY', 'Titanium Beam', 'Metals', 50, 10, 300.0, 500.0)`,
		org.ID,
	)
	if err != nil {
		t.Fatalf("failed to insert Titan SKU: %v", err)
	}

	// Client belonging to org_default checks stock
	reqDefault := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"erp_check_stock","arguments":{}}`),
	}
	defaultCtx := &SessionContext{
		ID:      "sess_def",
		AgentID: "agent_default",
		AuthCtx: &auth.AuthContext{
			UserID:   "usr_def",
			OrgID:    "org_default",
			Role:     auth.RoleOwner,
			IsAPIKey: true,
		},
	}

	respDef := server.ProcessRequestWithContext(reqDefault, defaultCtx)
	resDefMap, _ := respDef.Result.(map[string]interface{})
	contentDef, _ := resDefMap["content"].([]map[string]interface{})
	textDef := contentDef[0]["text"].(string)

	if strings.Contains(textDef, "TITAN-HEAVY") {
		t.Errorf("tenant leak: org_default agent saw Titan Corp SKU: %s", textDef)
	}

	// Client belonging to Titan Corp checks stock
	titanCtx := &SessionContext{
		ID:      "sess_titan",
		AgentID: "agent_titan",
		AuthCtx: &auth.AuthContext{
			UserID:   "usr_titan",
			OrgID:    org.ID,
			Role:     auth.RoleOwner,
			IsAPIKey: true,
		},
	}
	respTitan := server.ProcessRequestWithContext(reqDefault, titanCtx)
	resTitanMap, _ := respTitan.Result.(map[string]interface{})
	contentTitan, _ := resTitanMap["content"].([]map[string]interface{})
	textTitan := contentTitan[0]["text"].(string)

	if !strings.Contains(textTitan, "TITAN-HEAVY") {
		t.Errorf("Titan Corp agent should see TITAN-HEAVY, got: %s", textTitan)
	}
}

func TestMCPServer_WorkflowIntegrityAndReconciliation(t *testing.T) {
	server, _, database, cleanup := setupTestMCPServer(t)
	defer cleanup()

	agentCtx := &SessionContext{
		ID:      "sess_workflow",
		AgentID: "agent_ops",
		AuthCtx: &auth.AuthContext{
			UserID:   "usr_ops",
			OrgID:    "org_default",
			Role:     auth.RoleAdmin, // Has PermManageOrders, PermManageInventory, PermRead
			IsAPIKey: true,
		},
	}

	// 1. Create a PO over $1,000 via MCP tool -> Enforces HITL policy
	callPO := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: json.RawMessage(`{
			"name": "erp_create_purchase_order",
			"arguments": {
				"supplier_id": "sup_01",
				"items": [
					{"sku_code": "SKU-CORE-100", "quantity": 40, "unit_cost": 45.0}
				],
				"notes": "MCP automated order"
			}
		}`),
	}

	respPO := server.ProcessRequestWithContext(callPO, agentCtx)
	resPOMap, _ := respPO.Result.(map[string]interface{})
	contentPO, _ := resPOMap["content"].([]map[string]interface{})
	textPO := contentPO[0]["text"].(string)

	// 40 * $45 = $1,800 > $1,000 safety threshold
	if !strings.Contains(textPO, "pending_approval") || !strings.Contains(textPO, "policy threshold") {
		t.Errorf("expected pending_approval with policy routing, got: %s", textPO)
	}

	// 2. Adjust stock with optimistic locking via MCP tool
	var skuID string
	var ver int
	database.QueryRow(`SELECT id, version FROM skus WHERE code = 'SKU-CORE-100' AND org_id = 'org_default'`).Scan(&skuID, &ver)

	callAdj := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: json.RawMessage(fmt.Sprintf(`{
			"name": "erp_adjust_stock",
			"arguments": {
				"sku_id": "%s",
				"delta": -5,
				"reason": "Assembly consumption",
				"expected_version": %d
			}
		}`, skuID, ver)),
	}

	respAdj := server.ProcessRequestWithContext(callAdj, agentCtx)
	resAdjMap, _ := respAdj.Result.(map[string]interface{})
	contentAdj, _ := resAdjMap["content"].([]map[string]interface{})
	textAdj := contentAdj[0]["text"].(string)

	if !strings.Contains(textAdj, "Success") || !strings.Contains(textAdj, "Version: 2") {
		t.Errorf("expected stock adjust success with incremented version, got: %s", textAdj)
	}

	// 3. Stale version adjustment attempt via MCP tool -> Errors
	respStale := server.ProcessRequestWithContext(callAdj, agentCtx) // ver is now stale
	resStaleMap, _ := respStale.Result.(map[string]interface{})
	contentStale, _ := resStaleMap["content"].([]map[string]interface{})
	textStale := contentStale[0]["text"].(string)

	if !strings.Contains(textStale, "optimistic lock failure") {
		t.Errorf("expected optimistic lock failure on stale version, got: %s", textStale)
	}

	// 4. Run sys_reconcile tool
	callRec := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "sys_reconcile", "arguments": {}}`),
	}
	respRec := server.ProcessRequestWithContext(callRec, agentCtx)
	resRecMap, _ := respRec.Result.(map[string]interface{})
	contentRec, _ := resRecMap["content"].([]map[string]interface{})
	textRec := contentRec[0]["text"].(string)

	if !strings.Contains(textRec, "total_checked") {
		t.Errorf("expected valid reconciliation JSON report, got: %s", textRec)
	}
}

func TestMCPServer_TrialBalanceAndBackup(t *testing.T) {
	server, _, _, cleanup := setupTestMCPServer(t)
	defer cleanup()

	agentCtx := &SessionContext{
		ID:      "sess_ops",
		AgentID: "ops_agent",
		AuthCtx: &auth.AuthContext{
			UserID:   "usr_ops",
			OrgID:    "org_default",
			Role:     auth.RoleOperations,
			IsAPIKey: true,
		},
	}

	adminCtx := &SessionContext{
		ID:      "sess_admin",
		AgentID: "admin_agent",
		AuthCtx: &auth.AuthContext{
			UserID:   "usr_admin",
			OrgID:    "org_default",
			Role:     auth.RoleAdmin,
			IsAPIKey: true,
		},
	}

	// 1. Call fin_trial_balance via MCP tool (Operations role has PermRead)
	callTB := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "fin_trial_balance", "arguments": {}}`),
	}
	respTB := server.ProcessRequestWithContext(callTB, agentCtx)
	resTBMap, _ := respTB.Result.(map[string]interface{})
	contentTB, _ := resTBMap["content"].([]map[string]interface{})
	textTB := contentTB[0]["text"].(string)

	if !strings.Contains(textTB, `"is_balanced": true`) {
		t.Errorf("expected trial balance to be balanced, got: %s", textTB)
	}

	// 2. Call sys_backup with non-admin operations agent -> Forbidden
	callBackup := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "sys_backup", "arguments": {}}`),
	}
	respBackupForbidden := server.ProcessRequestWithContext(callBackup, agentCtx)
	resForbiddenMap, _ := respBackupForbidden.Result.(map[string]interface{})
	contentForbidden, _ := resForbiddenMap["content"].([]map[string]interface{})
	textForbidden := contentForbidden[0]["text"].(string)

	if !strings.Contains(textForbidden, "Permission denied") {
		t.Errorf("expected Permission denied for operations agent calling sys_backup, got: %s", textForbidden)
	}

	// 3. Call sys_backup with Admin agent -> Succeeds
	respBackupAdmin := server.ProcessRequestWithContext(callBackup, adminCtx)
	resAdminMap, _ := respBackupAdmin.Result.(map[string]interface{})
	contentAdmin, _ := resAdminMap["content"].([]map[string]interface{})
	textAdmin := contentAdmin[0]["text"].(string)

	if !strings.Contains(textAdmin, "Snapshot backup created successfully") {
		t.Errorf("expected backup success for admin agent, got: %s", textAdmin)
	}

	// 4. Read resource res://ledger/trial-balance
	readRes := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "resources/read",
		Params:  json.RawMessage(`{"uri": "res://ledger/trial-balance"}`),
	}
	respRes := server.ProcessRequestWithContext(readRes, agentCtx)
	resMap, _ := respRes.Result.(map[string]interface{})
	contentsList, _ := resMap["contents"].([]map[string]interface{})
	resText := contentsList[0]["text"].(string)

	if !strings.Contains(resText, `"is_balanced":true`) && !strings.Contains(resText, `"is_balanced": true`) {
		t.Errorf("expected resource read to contain is_balanced, got: %s", resText)
	}
}

func TestMCPServer_Phase6LifecycleTools(t *testing.T) {
	server, _, _, cleanup := setupTestMCPServer(t)
	defer cleanup()

	adminCtx := &SessionContext{
		ID:      "sess_admin_lifecycle",
		AgentID: "agent_admin",
		AuthCtx: &auth.AuthContext{
			UserID:   "usr_admin",
			OrgID:    "org_default",
			Email:    "admin@aosphere.local",
			Name:     "admin_bot",
			Role:     auth.RoleOwner,
			IsAPIKey: true,
		},
	}

	// 1. Tool docs_search
	reqDocs := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "docs_search", "arguments": {"query": "sensor"}}`),
	}
	respDocs := server.ProcessRequestWithContext(reqDocs, adminCtx)
	if respDocs.Error != nil {
		t.Fatalf("expected docs_search success, got: %v", respDocs.Error)
	}
	resDocsMap, _ := respDocs.Result.(map[string]interface{})
	if isErr, _ := resDocsMap["isError"].(bool); isErr {
		t.Fatalf("docs_search returned error: %+v", resDocsMap)
	}

	// 2. Tool crm_convert_deal
	reqConvert := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "crm_convert_deal", "arguments": {"deal_id": "deal_03"}}`),
	}
	respConvert := server.ProcessRequestWithContext(reqConvert, adminCtx)
	if respConvert.Error != nil {
		t.Fatalf("expected crm_convert_deal success, got: %v", respConvert.Error)
	}
	resConvMap, _ := respConvert.Result.(map[string]interface{})
	if isErr, _ := resConvMap["isError"].(bool); isErr {
		t.Fatalf("crm_convert_deal returned error: %+v", resConvMap)
	}

	// 3. Tool erp_pay_purchase_order with non-existent PO returns isError
	reqPay := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name": "erp_pay_purchase_order", "arguments": {"po_id": "po_fake_999"}}`),
	}
	respPay := server.ProcessRequestWithContext(reqPay, adminCtx)
	resPayMap, _ := respPay.Result.(map[string]interface{})
	if isErr, _ := resPayMap["isError"].(bool); !isErr {
		t.Errorf("expected isError=true for non-existent PO payment")
	}
}

func TestMCPServer_StdioAuthentication(t *testing.T) {
	server, appStore, _, cleanup := setupTestMCPServer(t)
	defer cleanup()

	// 1. Create a user and scoped API key
	user, err := appStore.CreateUser("org_default", "agent_owner@aosphere.local", "hashedpassword123", "Agent Owner", "admin")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	rawKey, _ := auth.GenerateRandomToken("ao_sk")
	keyHash := auth.HashToken(rawKey)
	_, err = appStore.CreateAPIKey("org_default", user.ID, "Automation Agent", keyHash, "operations", "erp_list_skus,erp_adjust_stock")
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	// 2. Test AuthenticateToken with valid key
	authCtx, agentName, err := server.AuthenticateToken(rawKey)
	if err != nil {
		t.Fatalf("AuthenticateToken with valid key failed: %v", err)
	}
	if authCtx.Role != auth.RoleOperations || authCtx.OrgID != "org_default" || agentName != "Automation Agent" {
		t.Errorf("unexpected identity from AuthenticateToken: %+v, agent: %s", authCtx, agentName)
	}

	// 3. Test AuthenticateToken with invalid key
	_, _, err = server.AuthenticateToken("ao_sk_invalid_bogus_token")
	if err == nil {
		t.Error("expected error for bogus API key, got nil")
	}

	// 4. Test Stdio in production environment with no key -> must reject
	server.SetEnv("production")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so if it proceeds it exits
	err = server.HandleStdio(ctx, "")
	if err == nil || !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("expected authentication required error for production stdio without key, got: %v", err)
	}
}


