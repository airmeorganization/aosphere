package finance

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// GenerateSequenceNumber atomically generates a collision-free sequential identifier (e.g. INV-2026-00001, PO-2026-00001, JE-2026-00001)
func GenerateSequenceNumber(ctx context.Context, tx *sql.Tx, orgID, seqType string) (string, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	year := time.Now().Year()

	// Ensure sequence row exists
	_, err := tx.ExecContext(ctx, `INSERT INTO sequences (org_id, sequence_type, year, current_value)
		VALUES (?, ?, ?, 0)
		ON CONFLICT(org_id, sequence_type, year) DO NOTHING`, orgID, seqType, year)
	if err != nil {
		return "", fmt.Errorf("failed to initialize sequence row: %w", err)
	}

	// Increment current_value atomically
	_, err = tx.ExecContext(ctx, `UPDATE sequences 
		SET current_value = current_value + 1 
		WHERE org_id = ? AND sequence_type = ? AND year = ?`, orgID, seqType, year)
	if err != nil {
		return "", fmt.Errorf("failed to increment sequence: %w", err)
	}

	var nextVal int64
	err = tx.QueryRowContext(ctx, `SELECT current_value 
		FROM sequences 
		WHERE org_id = ? AND sequence_type = ? AND year = ?`, orgID, seqType, year).Scan(&nextVal)
	if err != nil {
		return "", fmt.Errorf("failed to read sequence value: %w", err)
	}

	return fmt.Sprintf("%s-%d-%05d", seqType, year, nextVal), nil
}
