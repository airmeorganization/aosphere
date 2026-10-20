package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type AgentExecution struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	AgentID       string    `json:"agent_id"`
	ToolName      string    `json:"tool_name"`
	Category      string    `json:"category"`
	ArgumentsJSON string    `json:"arguments_json"`
	ResultJSON    string    `json:"result_json"`
	IsError       bool      `json:"is_error"`
	DurationMS    int64     `json:"duration_ms"`
	CreatedAt     time.Time `json:"created_at"`
}

// RecordAgentExecution logs complete tool invocation history to agent_executions table
func RecordAgentExecution(ctx context.Context, db *sql.DB, exec AgentExecution) error {
	if db == nil {
		return nil
	}
	if exec.ID == "" {
		exec.ID = fmt.Sprintf("exec_%d", time.Now().UnixNano())
	}
	if exec.OrgID == "" {
		exec.OrgID = "org_default"
	}
	if exec.CreatedAt.IsZero() {
		exec.CreatedAt = time.Now()
	}

	query := `INSERT INTO agent_executions 
		(id, org_id, agent_id, tool_name, category, arguments_json, result_json, is_error, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.ExecContext(ctx, query,
		exec.ID, exec.OrgID, exec.AgentID, exec.ToolName, exec.Category,
		exec.ArgumentsJSON, exec.ResultJSON, exec.IsError, exec.DurationMS, exec.CreatedAt)
	return err
}

// GetAgentExecutions retrieves recent execution history for auditing
func GetAgentExecutions(ctx context.Context, db *sql.DB, orgID string, limit int) ([]AgentExecution, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT id, org_id, agent_id, tool_name, category, arguments_json, result_json, is_error, duration_ms, created_at 
		FROM agent_executions 
		WHERE org_id = ? 
		ORDER BY created_at DESC 
		LIMIT ?`

	rows, err := db.QueryContext(ctx, query, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AgentExecution
	for rows.Next() {
		var e AgentExecution
		if err := rows.Scan(&e.ID, &e.OrgID, &e.AgentID, &e.ToolName, &e.Category, &e.ArgumentsJSON, &e.ResultJSON, &e.IsError, &e.DurationMS, &e.CreatedAt); err == nil {
			list = append(list, e)
		}
	}
	return list, nil
}

// TruncateJSON safely limits JSON payload sizes for audit storage
func TruncateJSON(raw interface{}, maxChars int) string {
	bytes, err := json.Marshal(raw)
	if err != nil {
		return "{}"
	}
	s := string(bytes)
	if len(s) > maxChars {
		return s[:maxChars] + "...[truncated]"
	}
	return s
}
