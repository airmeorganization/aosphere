package mcp

import (
	"database/sql"
	"encoding/json"
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
	server := NewMCPServer(appStore)

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
