package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"aosphere/internal/finance"
	"github.com/google/uuid"
)

var (
	ErrUnbalancedEntry   = errors.New("unbalanced journal entry: debits must equal credits")
	ErrAccountNotFound   = errors.New("ledger account not found")
	ErrEntryAlreadyPosted = errors.New("journal entry is already posted and immutable")
)

// Account Types
const (
	AccountTypeAsset     = "asset"     // Normal balance: Debit
	AccountTypeLiability = "liability" // Normal balance: Credit
	AccountTypeEquity    = "equity"    // Normal balance: Credit
	AccountTypeRevenue   = "revenue"   // Normal balance: Credit
	AccountTypeExpense   = "expense"   // Normal balance: Debit
)

// Standard Account Codes
const (
	AccountCash             = "1000" // Operating Cash / Bank
	AccountAccountsReceivable = "1100" // AR
	AccountInventoryAsset   = "1200" // Inventory
	AccountAccountsPayable  = "2000" // AP
	AccountSalesTaxPayable  = "2100" // Sales Tax
	AccountRetainedEarnings = "3000" // Equity
	AccountSalesRevenue     = "4000" // Revenue
	AccountCOGS             = "5000" // Cost of Goods Sold
	AccountOperatingExpense = "5100" // OpEx
)

type Account struct {
	ID            string `json:"id"`
	OrgID         string `json:"org_id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	NormalBalance string `json:"normal_balance"` // "debit" or "credit"
	IsActive      bool   `json:"is_active"`
}

type JournalLine struct {
	AccountCode string `json:"account_code"`
	EntryType   string `json:"entry_type"` // "debit" or "credit"
	AmountCents int64  `json:"amount_cents"`
	Description string `json:"description"`
}

type JournalEntry struct {
	ID          string        `json:"id"`
	OrgID       string        `json:"org_id"`
	EntryNumber string        `json:"entry_number"`
	SourceType  string        `json:"source_type"`
	SourceID    string        `json:"source_id"`
	Memo        string        `json:"memo"`
	Status      string        `json:"status"` // "posted", "reversed"
	PostedAt    time.Time     `json:"posted_at"`
	CreatedBy   string        `json:"created_by"`
	Lines       []JournalLine `json:"lines,omitempty"`
}

type LedgerService struct {
	db *sql.DB
}

func NewLedgerService(database *sql.DB) *LedgerService {
	return &LedgerService{db: database}
}

// PostJournalEntry transactionally posts a double-entry journal entry enforcing debits == credits
func (l *LedgerService) PostJournalEntry(ctx context.Context, tx *sql.Tx, req JournalEntry) (*JournalEntry, error) {
	if req.OrgID == "" {
		req.OrgID = "org_default"
	}
	if len(req.Lines) < 2 {
		return nil, fmt.Errorf("journal entry must contain at least two lines")
	}

	// 1. Invariant Check: Sum(Debits) == Sum(Credits)
	var totalDebits, totalCredits int64
	for _, line := range req.Lines {
		if line.AmountCents <= 0 {
			return nil, fmt.Errorf("journal line amount must be strictly positive, got %d", line.AmountCents)
		}
		switch line.EntryType {
		case "debit":
			totalDebits += line.AmountCents
		case "credit":
			totalCredits += line.AmountCents
		default:
			return nil, fmt.Errorf("invalid entry_type %q: must be 'debit' or 'credit'", line.EntryType)
		}
	}

	if totalDebits != totalCredits {
		return nil, fmt.Errorf("%w: total debits (%s) != total credits (%s)",
			ErrUnbalancedEntry, finance.FormatUSD(totalDebits), finance.FormatUSD(totalCredits))
	}

	// 2. Validate all referenced accounts exist in the organization
	accountIDs := make(map[string]string)
	for _, line := range req.Lines {
		if _, exists := accountIDs[line.AccountCode]; exists {
			continue
		}
		var accID string
		err := tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE org_id = ? AND code = ? AND is_active = 1`,
			req.OrgID, line.AccountCode).Scan(&accID)
		if err != nil {
			return nil, fmt.Errorf("%w: account code %q does not exist in org %s", ErrAccountNotFound, line.AccountCode, req.OrgID)
		}
		accountIDs[line.AccountCode] = accID
	}

	// 3. Generate Sequential Entry Number
	entryNumber, err := finance.GenerateSequenceNumber(ctx, tx, req.OrgID, "JE")
	if err != nil {
		return nil, fmt.Errorf("failed to generate journal entry sequence: %w", err)
	}

	entryID := "je_" + uuid.New().String()[:8]
	now := time.Now()
	postedAt := req.PostedAt
	if postedAt.IsZero() {
		postedAt = now
	}

	// 4. Insert Journal Entry Header
	_, err = tx.ExecContext(ctx, `INSERT INTO journal_entries (id, org_id, entry_number, source_type, source_id, memo, status, posted_at, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'posted', ?, ?, ?)`,
		entryID, req.OrgID, entryNumber, req.SourceType, req.SourceID, req.Memo, postedAt, req.CreatedBy, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert journal entry header: %w", err)
	}

	// 5. Insert Journal Lines
	for _, line := range req.Lines {
		lineID := "jl_" + uuid.New().String()[:8]
		accID := accountIDs[line.AccountCode]
		_, err = tx.ExecContext(ctx, `INSERT INTO journal_lines (id, org_id, entry_id, account_id, entry_type, amount_cents, description, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			lineID, req.OrgID, entryID, accID, line.EntryType, line.AmountCents, line.Description, now)
		if err != nil {
			return nil, fmt.Errorf("failed to insert journal line: %w", err)
		}
	}

	req.ID = entryID
	req.EntryNumber = entryNumber
	req.Status = "posted"
	req.PostedAt = postedAt

	return &req, nil
}

// PostInvoiceIssued records the standard accounting entry when an invoice is issued:
// Debit: Accounts Receivable (total)
// Credit: Sales Revenue (subtotal)
// Credit: Sales Tax Payable (tax)
func (l *LedgerService) PostInvoiceIssued(ctx context.Context, tx *sql.Tx, orgID, invoiceID, invoiceNumber string, subtotalCents, taxCents, totalCents int64, actor string) (*JournalEntry, error) {
	lines := []JournalLine{
		{
			AccountCode: AccountAccountsReceivable,
			EntryType:   "debit",
			AmountCents: totalCents,
			Description: fmt.Sprintf("AR for Invoice %s", invoiceNumber),
		},
		{
			AccountCode: AccountSalesRevenue,
			EntryType:   "credit",
			AmountCents: subtotalCents,
			Description: fmt.Sprintf("Sales revenue for Invoice %s", invoiceNumber),
		},
	}
	if taxCents > 0 {
		lines = append(lines, JournalLine{
			AccountCode: AccountSalesTaxPayable,
			EntryType:   "credit",
			AmountCents: taxCents,
			Description: fmt.Sprintf("Sales tax on Invoice %s", invoiceNumber),
		})
	}

	return l.PostJournalEntry(ctx, tx, JournalEntry{
		OrgID:      orgID,
		SourceType: "invoice",
		SourceID:   invoiceID,
		Memo:       fmt.Sprintf("Invoice %s Issued (%s)", invoiceNumber, finance.FormatUSD(totalCents)),
		CreatedBy:  actor,
		Lines:      lines,
	})
}

// PostInvoicePaid records payment collection:
// Debit: Operating Cash (total)
// Credit: Accounts Receivable (total)
func (l *LedgerService) PostInvoicePaid(ctx context.Context, tx *sql.Tx, orgID, invoiceID, invoiceNumber string, totalCents int64, actor string) (*JournalEntry, error) {
	lines := []JournalLine{
		{
			AccountCode: AccountCash,
			EntryType:   "debit",
			AmountCents: totalCents,
			Description: fmt.Sprintf("Payment received for Invoice %s", invoiceNumber),
		},
		{
			AccountCode: AccountAccountsReceivable,
			EntryType:   "credit",
			AmountCents: totalCents,
			Description: fmt.Sprintf("Relief of AR for Invoice %s", invoiceNumber),
		},
	}

	return l.PostJournalEntry(ctx, tx, JournalEntry{
		OrgID:      orgID,
		SourceType: "payment",
		SourceID:   invoiceID,
		Memo:       fmt.Sprintf("Payment collected on Invoice %s (%s)", invoiceNumber, finance.FormatUSD(totalCents)),
		CreatedBy:  actor,
		Lines:      lines,
	})
}

// PostPOReceipt records the receipt of goods:
// Debit: Inventory Asset (total)
// Credit: Accounts Payable (total)
func (l *LedgerService) PostPOReceipt(ctx context.Context, tx *sql.Tx, orgID, poID, poNumber string, totalCents int64, actor string) (*JournalEntry, error) {
	lines := []JournalLine{
		{
			AccountCode: AccountInventoryAsset,
			EntryType:   "debit",
			AmountCents: totalCents,
			Description: fmt.Sprintf("Inventory received on PO %s", poNumber),
		},
		{
			AccountCode: AccountAccountsPayable,
			EntryType:   "credit",
			AmountCents: totalCents,
			Description: fmt.Sprintf("AP liability for PO %s", poNumber),
		},
	}

	return l.PostJournalEntry(ctx, tx, JournalEntry{
		OrgID:      orgID,
		SourceType: "purchase_order",
		SourceID:   poID,
		Memo:       fmt.Sprintf("Goods received for Purchase Order %s (%s)", poNumber, finance.FormatUSD(totalCents)),
		CreatedBy:  actor,
		Lines:      lines,
	})
}

// TrialBalanceAccount holds net balance for one account
type TrialBalanceAccount struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	DebitCents  int64  `json:"debit_cents"`
	CreditCents int64  `json:"credit_cents"`
}

type TrialBalanceReport struct {
	OrgID             string                `json:"org_id"`
	AsOf              time.Time             `json:"as_of"`
	Accounts          []TrialBalanceAccount `json:"accounts"`
	TotalDebitsCents  int64                 `json:"total_debits_cents"`
	TotalCreditsCents int64                 `json:"total_credits_cents"`
	IsBalanced        bool                  `json:"is_balanced"`
}

// GetTrialBalance computes the trial balance across all ledger accounts for the organization
func (l *LedgerService) GetTrialBalance(ctx context.Context, orgID string) (*TrialBalanceReport, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	query := `
	SELECT a.code, a.name, a.type,
		COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE 0 END), 0) AS total_debits,
		COALESCE(SUM(CASE WHEN jl.entry_type = 'credit' THEN jl.amount_cents ELSE 0 END), 0) AS total_credits
	FROM accounts a
	LEFT JOIN journal_lines jl ON a.id = jl.account_id AND jl.org_id = a.org_id
	LEFT JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
	WHERE a.org_id = ? AND a.is_active = 1
	GROUP BY a.code, a.name, a.type
	ORDER BY a.code ASC`

	rows, err := l.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query trial balance: %w", err)
	}
	defer rows.Close()

	report := &TrialBalanceReport{
		OrgID:    orgID,
		AsOf:     time.Now(),
		Accounts: make([]TrialBalanceAccount, 0),
	}

	for rows.Next() {
		var acc TrialBalanceAccount
		if err := rows.Scan(&acc.Code, &acc.Name, &acc.Type, &acc.DebitCents, &acc.CreditCents); err != nil {
			return nil, fmt.Errorf("failed to scan trial balance row: %w", err)
		}
		report.Accounts = append(report.Accounts, acc)
		report.TotalDebitsCents += acc.DebitCents
		report.TotalCreditsCents += acc.CreditCents
	}

	report.IsBalanced = (report.TotalDebitsCents == report.TotalCreditsCents)
	return report, nil
}

// PostCreditNoteIssued records the standard accounting reversal when a credit note is issued:
// Debit: Sales Revenue (subtotal)
// Debit: Sales Tax Payable (tax, if any)
// Credit: Accounts Receivable (total)
func (l *LedgerService) PostCreditNoteIssued(ctx context.Context, tx *sql.Tx, orgID, creditNoteID, creditNoteNumber, invoiceNumber string, subtotalCents, taxCents, totalCents int64, actor string) (*JournalEntry, error) {
	lines := []JournalLine{
		{
			AccountCode: AccountSalesRevenue,
			EntryType:   "debit",
			AmountCents: subtotalCents,
			Description: fmt.Sprintf("Revenue reversal for Credit Note %s (Invoice %s)", creditNoteNumber, invoiceNumber),
		},
	}
	if taxCents > 0 {
		lines = append(lines, JournalLine{
			AccountCode: AccountSalesTaxPayable,
			EntryType:   "debit",
			AmountCents: taxCents,
			Description: fmt.Sprintf("Tax relief on Credit Note %s", creditNoteNumber),
		})
	}
	lines = append(lines, JournalLine{
		AccountCode: AccountAccountsReceivable,
		EntryType:   "credit",
		AmountCents: totalCents,
		Description: fmt.Sprintf("AR relief for Credit Note %s", creditNoteNumber),
	})

	return l.PostJournalEntry(ctx, tx, JournalEntry{
		OrgID:      orgID,
		SourceType: "credit_note",
		SourceID:   creditNoteID,
		Memo:       fmt.Sprintf("Credit Note %s Issued against Invoice %s (%s)", creditNoteNumber, invoiceNumber, finance.FormatUSD(totalCents)),
		CreatedBy:  actor,
		Lines:      lines,
	})
}

