package reports

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"aosphere/internal/db"
	"aosphere/internal/ledger"
	"aosphere/internal/pdf"
	"github.com/google/uuid"
)

type ReportService struct {
	db     *sql.DB
	ledger *ledger.LedgerService
}

func NewReportService(database *sql.DB, ledgerSvc *ledger.LedgerService) *ReportService {
	return &ReportService{
		db:     database,
		ledger: ledgerSvc,
	}
}

// ---------------- 1. PROFIT & LOSS (P&L) ----------------

func (s *ReportService) GetProfitAndLoss(ctx context.Context, orgID, startDate, endDate string) (*db.PnLReport, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if startDate == "" {
		startDate = time.Now().UTC().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().UTC().Format("2006-01-02")
	}

	report := &db.PnLReport{
		OrgID:        orgID,
		StartDate:    startDate,
		EndDate:      endDate,
		RevenueItems: make([]db.PnLLineItem, 0),
		COGSItems:    make([]db.PnLLineItem, 0),
		OpExItems:    make([]db.PnLLineItem, 0),
	}

	// Query journal lines between startDate and endDate
	// Revenue: account type 'revenue' (credit - debit)
	// COGS / Expenses: account type 'expense' (debit - credit)
	query := `
	SELECT a.code, a.name, a.type,
		COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE 0 END), 0) AS total_debits,
		COALESCE(SUM(CASE WHEN jl.entry_type = 'credit' THEN jl.amount_cents ELSE 0 END), 0) AS total_credits
	FROM accounts a
	JOIN journal_lines jl ON a.id = jl.account_id AND jl.org_id = a.org_id
	JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
	WHERE a.org_id = ?
		AND substr(je.posted_at, 1, 10) >= ?
		AND substr(je.posted_at, 1, 10) <= ?
		AND a.type IN ('revenue', 'expense')
	GROUP BY a.code, a.name, a.type
	ORDER BY a.code ASC`

	rows, err := s.db.QueryContext(ctx, query, orgID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query pnl: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var code, name, accType string
		var debits, credits int64
		if err := rows.Scan(&code, &name, &accType, &debits, &credits); err != nil {
			return nil, fmt.Errorf("failed to scan pnl row: %w", err)
		}

		if accType == "revenue" {
			netRevenueCents := credits - debits
			if netRevenueCents < 0 {
				netRevenueCents = 0
			}
			report.RevenueCents += netRevenueCents
			report.RevenueItems = append(report.RevenueItems, db.PnLLineItem{
				AccountCode: code,
				AccountName: name,
				AmountCents: netRevenueCents,
				Amount:      float64(netRevenueCents) / 100.0,
			})
		} else if accType == "expense" {
			netExpenseCents := debits - credits
			if netExpenseCents < 0 {
				netExpenseCents = 0
			}
			if code == "5000" { // COGS
				report.COGSCents += netExpenseCents
				report.COGSItems = append(report.COGSItems, db.PnLLineItem{
					AccountCode: code,
					AccountName: name,
					AmountCents: netExpenseCents,
					Amount:      float64(netExpenseCents) / 100.0,
				})
			} else { // Operating Expenses
				report.OpExCents += netExpenseCents
				report.OpExItems = append(report.OpExItems, db.PnLLineItem{
					AccountCode: code,
					AccountName: name,
					AmountCents: netExpenseCents,
					Amount:      float64(netExpenseCents) / 100.0,
				})
			}
		}
	}

	report.Revenue = float64(report.RevenueCents) / 100.0
	report.COGS = float64(report.COGSCents) / 100.0
	report.GrossProfitCents = report.RevenueCents - report.COGSCents
	report.GrossProfit = float64(report.GrossProfitCents) / 100.0
	if report.RevenueCents > 0 {
		report.GrossMarginPct = math.Round((float64(report.GrossProfitCents)/float64(report.RevenueCents))*10000.0) / 100.0
	}
	report.OpEx = float64(report.OpExCents) / 100.0
	report.NetProfitCents = report.GrossProfitCents - report.OpExCents
	report.NetProfit = float64(report.NetProfitCents) / 100.0

	return report, nil
}

// ---------------- 2. BALANCE SHEET ----------------

func (s *ReportService) GetBalanceSheet(ctx context.Context, orgID, asOfDate string) (*db.BalanceSheetReport, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if asOfDate == "" {
		asOfDate = time.Now().UTC().Format("2006-01-02")
	}

	report := &db.BalanceSheetReport{
		OrgID:              orgID,
		AsOf:               asOfDate,
		CurrentAssets:      make([]db.BalanceSheetAccount, 0),
		CurrentLiabilities: make([]db.BalanceSheetAccount, 0),
		EquityItems:        make([]db.BalanceSheetAccount, 0),
	}

	// 1. Calculate cumulative balance for Assets, Liabilities, and Equity accounts as of date
	query := `
	SELECT a.code, a.name, a.type, a.normal_balance,
		COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE 0 END), 0) AS total_debits,
		COALESCE(SUM(CASE WHEN jl.entry_type = 'credit' THEN jl.amount_cents ELSE 0 END), 0) AS total_credits
	FROM accounts a
	LEFT JOIN journal_lines jl ON a.id = jl.account_id AND jl.org_id = a.org_id
	LEFT JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted' AND substr(je.posted_at, 1, 10) <= ?
	WHERE a.org_id = ? AND a.is_active = 1
	GROUP BY a.code, a.name, a.type, a.normal_balance
	ORDER BY a.code ASC`

	rows, err := s.db.QueryContext(ctx, query, asOfDate, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query balance sheet accounts: %w", err)
	}
	defer rows.Close()

	var totalRevenueCents, totalExpenseCents int64

	for rows.Next() {
		var code, name, accType, normalBalance string
		var debits, credits int64
		if err := rows.Scan(&code, &name, &accType, &normalBalance, &debits, &credits); err != nil {
			return nil, fmt.Errorf("failed to scan balance sheet row: %w", err)
		}

		var netCents int64
		if normalBalance == "debit" {
			netCents = debits - credits
		} else {
			netCents = credits - debits
		}

		switch accType {
		case "asset":
			report.TotalAssetsCents += netCents
			report.CurrentAssets = append(report.CurrentAssets, db.BalanceSheetAccount{
				Code:        code,
				Name:        name,
				AmountCents: netCents,
				Amount:      float64(netCents) / 100.0,
			})
		case "liability":
			report.TotalLiabilitiesCents += netCents
			report.CurrentLiabilities = append(report.CurrentLiabilities, db.BalanceSheetAccount{
				Code:        code,
				Name:        name,
				AmountCents: netCents,
				Amount:      float64(netCents) / 100.0,
			})
		case "equity":
			report.TotalEquityCents += netCents
			report.EquityItems = append(report.EquityItems, db.BalanceSheetAccount{
				Code:        code,
				Name:        name,
				AmountCents: netCents,
				Amount:      float64(netCents) / 100.0,
			})
		case "revenue":
			totalRevenueCents += (credits - debits)
		case "expense":
			totalExpenseCents += (debits - credits)
		}
	}

	// 2. Net Income closes into Equity
	periodNetIncomeCents := totalRevenueCents - totalExpenseCents
	report.TotalEquityCents += periodNetIncomeCents
	report.EquityItems = append(report.EquityItems, db.BalanceSheetAccount{
		Code:        "3100",
		Name:        "Current Period Net Earnings",
		AmountCents: periodNetIncomeCents,
		Amount:      float64(periodNetIncomeCents) / 100.0,
	})

	report.TotalAssets = float64(report.TotalAssetsCents) / 100.0
	report.TotalLiabilities = float64(report.TotalLiabilitiesCents) / 100.0
	report.TotalEquity = float64(report.TotalEquityCents) / 100.0
	report.IsBalanced = (report.TotalAssetsCents == (report.TotalLiabilitiesCents + report.TotalEquityCents))

	return report, nil
}

// ---------------- 3. CASH FLOW STATEMENT ----------------

func (s *ReportService) GetCashFlowStatement(ctx context.Context, orgID, startDate, endDate string) (*db.CashFlowReport, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if startDate == "" {
		startDate = time.Now().UTC().AddDate(0, -1, 0).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = time.Now().UTC().Format("2006-01-02")
	}

	pnl, err := s.GetProfitAndLoss(ctx, orgID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	report := &db.CashFlowReport{
		OrgID:               orgID,
		StartDate:           startDate,
		EndDate:             endDate,
		OperatingActivities: make([]db.CashFlowActivity, 0),
		InvestingActivities: make([]db.CashFlowActivity, 0),
		FinancingActivities: make([]db.CashFlowActivity, 0),
	}

	// 1. Operating: Net Profit from PnL
	report.OperatingActivities = append(report.OperatingActivities, db.CashFlowActivity{
		Category:    "operating",
		Description: "Net Operational Income / (Loss)",
		AmountCents: pnl.NetProfitCents,
		Amount:      pnl.NetProfit,
	})
	report.OperatingNetCents += pnl.NetProfitCents

	// 2. Changes in Cash via cash account (1000) transactions in period
	cashQuery := `
	SELECT
		COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE 0 END), 0) AS total_inflows,
		COALESCE(SUM(CASE WHEN jl.entry_type = 'credit' THEN jl.amount_cents ELSE 0 END), 0) AS total_outflows
	FROM journal_lines jl
	JOIN accounts a ON jl.account_id = a.id
	JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
	WHERE a.org_id = ? AND a.code = '1000'
		AND substr(je.posted_at, 1, 10) >= ? AND substr(je.posted_at, 1, 10) <= ?`

	var cashIn, cashOut int64
	_ = s.db.QueryRowContext(ctx, cashQuery, orgID, startDate, endDate).Scan(&cashIn, &cashOut)

	// Beginning cash prior to startDate
	begCashQuery := `
	SELECT
		COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE -jl.amount_cents END), 0)
	FROM journal_lines jl
	JOIN accounts a ON jl.account_id = a.id
	JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
	WHERE a.org_id = ? AND a.code = '1000'
		AND substr(je.posted_at, 1, 10) < ?`
	_ = s.db.QueryRowContext(ctx, begCashQuery, orgID, startDate).Scan(&report.BeginningCashCents)

	netCashFlowCents := cashIn - cashOut
	report.NetCashChangeCents = netCashFlowCents
	report.NetCashChange = float64(netCashFlowCents) / 100.0
	report.BeginningCash = float64(report.BeginningCashCents) / 100.0
	report.EndingCashCents = report.BeginningCashCents + report.NetCashChangeCents
	report.EndingCash = float64(report.EndingCashCents) / 100.0

	report.OperatingNet = float64(report.OperatingNetCents) / 100.0
	report.InvestingNet = float64(report.InvestingNetCents) / 100.0
	report.FinancingNet = float64(report.FinancingNetCents) / 100.0

	return report, nil
}

// ---------------- 4. AGED RECEIVABLES (A/R) & AGED PAYABLES (A/P) ----------------

func (s *ReportService) GetAgedReceivables(ctx context.Context, orgID, asOfDate string) (*db.AgedReport, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if asOfDate == "" {
		asOfDate = time.Now().UTC().Format("2006-01-02")
	}

	report := &db.AgedReport{
		OrgID:      orgID,
		ReportType: "receivables",
		AsOf:       asOfDate,
		Items:      make([]db.AgedItem, 0),
	}

	asOfTime, err := time.Parse("2006-01-02", asOfDate)
	if err != nil {
		asOfTime = time.Now().UTC()
	}

	query := `
	SELECT i.id, i.invoice_number, i.customer_id, COALESCE(c.name, 'Customer') as customer_name,
		i.issue_date, i.due_date, i.total_amount
	FROM invoices i
	LEFT JOIN customers c ON i.customer_id = c.id
	WHERE i.org_id = ? AND i.status != 'paid' AND i.total_amount > 0
	ORDER BY i.due_date ASC`

	rows, err := s.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query open invoices: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, invNum, custID, custName, issueDate, dueDate string
		var totalAmount float64
		if err := rows.Scan(&id, &invNum, &custID, &custName, &issueDate, &dueDate, &totalAmount); err != nil {
			return nil, fmt.Errorf("failed to scan invoice row: %w", err)
		}

		dueTime, err := time.Parse("2006-01-02", dueDate)
		if err != nil {
			dueTime = asOfTime
		}

		daysOverdue := int(asOfTime.Sub(dueTime).Hours() / 24.0)
		amountCents := int64(math.Round(totalAmount * 100.0))

		var bracket string
		if daysOverdue <= 0 {
			bracket = "current"
			report.CurrentCents += amountCents
		} else if daysOverdue <= 30 {
			bracket = "1_30"
			report.Days1To30Cents += amountCents
		} else if daysOverdue <= 60 {
			bracket = "31_60"
			report.Days31To60Cents += amountCents
		} else if daysOverdue <= 90 {
			bracket = "61_90"
			report.Days61To90Cents += amountCents
		} else {
			bracket = "90_plus"
			report.Days90PlusCents += amountCents
		}
		report.TotalCents += amountCents

		report.Items = append(report.Items, db.AgedItem{
			ID:          id,
			Number:      invNum,
			EntityID:    custID,
			EntityName:  custName,
			IssueDate:   issueDate,
			DueDate:     dueDate,
			AmountCents: amountCents,
			Amount:      totalAmount,
			DaysOverdue: daysOverdue,
			Bracket:     bracket,
		})
	}

	report.Current = float64(report.CurrentCents) / 100.0
	report.Days1To30 = float64(report.Days1To30Cents) / 100.0
	report.Days31To60 = float64(report.Days31To60Cents) / 100.0
	report.Days61To90 = float64(report.Days61To90Cents) / 100.0
	report.Days90Plus = float64(report.Days90PlusCents) / 100.0
	report.Total = float64(report.TotalCents) / 100.0

	return report, nil
}

func (s *ReportService) GetAgedPayables(ctx context.Context, orgID, asOfDate string) (*db.AgedReport, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if asOfDate == "" {
		asOfDate = time.Now().UTC().Format("2006-01-02")
	}

	report := &db.AgedReport{
		OrgID:      orgID,
		ReportType: "payables",
		AsOf:       asOfDate,
		Items:      make([]db.AgedItem, 0),
	}

	asOfTime, err := time.Parse("2006-01-02", asOfDate)
	if err != nil {
		asOfTime = time.Now().UTC()
	}

	// Query open supplier bills or purchase orders
	query := `
	SELECT po.id, po.po_number, po.supplier_id, COALESCE(s.name, 'Supplier') as supplier_name,
		substr(po.created_at, 1, 10) as issue_date, po.total_amount
	FROM purchase_orders po
	LEFT JOIN suppliers s ON po.supplier_id = s.id
	WHERE po.org_id = ? AND po.status IN ('ordered', 'pending_approval') AND po.total_amount > 0
	ORDER BY po.created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query open payables: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, poNum, supID, supName, issueDate string
		var totalAmount float64
		if err := rows.Scan(&id, &poNum, &supID, &supName, &issueDate, &totalAmount); err != nil {
			return nil, fmt.Errorf("failed to scan payable row: %w", err)
		}

		issueTime, err := time.Parse("2006-01-02", issueDate)
		if err != nil {
			issueTime = asOfTime
		}

		// Terms default 30 days
		dueTime := issueTime.AddDate(0, 0, 30)
		daysOverdue := int(asOfTime.Sub(dueTime).Hours() / 24.0)
		amountCents := int64(math.Round(totalAmount * 100.0))

		var bracket string
		if daysOverdue <= 0 {
			bracket = "current"
			report.CurrentCents += amountCents
		} else if daysOverdue <= 30 {
			bracket = "1_30"
			report.Days1To30Cents += amountCents
		} else if daysOverdue <= 60 {
			bracket = "31_60"
			report.Days31To60Cents += amountCents
		} else if daysOverdue <= 90 {
			bracket = "61_90"
			report.Days61To90Cents += amountCents
		} else {
			bracket = "90_plus"
			report.Days90PlusCents += amountCents
		}
		report.TotalCents += amountCents

		report.Items = append(report.Items, db.AgedItem{
			ID:          id,
			Number:      poNum,
			EntityID:    supID,
			EntityName:  supName,
			IssueDate:   issueDate,
			DueDate:     dueTime.Format("2006-01-02"),
			AmountCents: amountCents,
			Amount:      totalAmount,
			DaysOverdue: daysOverdue,
			Bracket:     bracket,
		})
	}

	report.Current = float64(report.CurrentCents) / 100.0
	report.Days1To30 = float64(report.Days1To30Cents) / 100.0
	report.Days31To60 = float64(report.Days31To60Cents) / 100.0
	report.Days61To90 = float64(report.Days61To90Cents) / 100.0
	report.Days90Plus = float64(report.Days90PlusCents) / 100.0
	report.Total = float64(report.TotalCents) / 100.0

	return report, nil
}

// ---------------- 5. INVENTORY VALUATION & TURNOVER ----------------

func (s *ReportService) GetInventoryValuation(ctx context.Context, orgID string) (*db.InventoryValuationReport, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	report := &db.InventoryValuationReport{
		OrgID:      orgID,
		AsOf:       time.Now().UTC().Format("2006-01-02"),
		SKUs:       make([]db.SKUValuation, 0),
		Warehouses: make([]db.WarehouseValuation, 0),
	}

	// 1. SKU Valuations
	query := `
	SELECT id, code, name, category, stock_quantity, reserved_quantity, unit_cost, selling_price
	FROM skus
	WHERE org_id = ?
	ORDER BY name ASC`

	rows, err := s.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query skus for valuation: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sku db.SKUValuation
		if err := rows.Scan(&sku.SKUID, &sku.SKUCode, &sku.SKUName, &sku.Category, &sku.StockQuantity, &sku.ReservedQuantity, &sku.UnitCost, &sku.SellingPrice); err != nil {
			return nil, fmt.Errorf("failed to scan sku valuation: %w", err)
		}

		sku.CostValuation = math.Round(float64(sku.StockQuantity)*sku.UnitCost*100.0) / 100.0
		sku.RetailValuation = math.Round(float64(sku.StockQuantity)*sku.SellingPrice*100.0) / 100.0
		sku.PotentialProfit = math.Round((sku.RetailValuation-sku.CostValuation)*100.0) / 100.0

		report.TotalSKUs++
		report.TotalUnitsOnHand += sku.StockQuantity
		report.TotalCostValuation += sku.CostValuation
		report.TotalRetailValuation += sku.RetailValuation

		report.SKUs = append(report.SKUs, sku)
	}

	// 2. Warehouse breakdown
	whQuery := `
	SELECT w.id, w.name, COUNT(DISTINCT ws.sku_id) as total_items, COALESCE(SUM(ws.stock_quantity), 0) as total_units,
		COALESCE(SUM(ws.stock_quantity * s.unit_cost), 0.0) as cost_valuation
	FROM warehouses w
	LEFT JOIN warehouse_stocks ws ON w.id = ws.warehouse_id
	LEFT JOIN skus s ON ws.sku_id = s.id
	WHERE w.org_id = ?
	GROUP BY w.id, w.name
	ORDER BY w.name ASC`

	whRows, err := s.db.QueryContext(ctx, whQuery, orgID)
	if err == nil {
		defer whRows.Close()
		for whRows.Next() {
			var wh db.WarehouseValuation
			if err := whRows.Scan(&wh.WarehouseID, &wh.WarehouseName, &wh.TotalItems, &wh.TotalUnits, &wh.CostValuation); err == nil {
				report.Warehouses = append(report.Warehouses, wh)
			}
		}
	}

	report.TotalCostValuation = math.Round(report.TotalCostValuation*100.0) / 100.0
	report.TotalRetailValuation = math.Round(report.TotalRetailValuation*100.0) / 100.0

	return report, nil
}

func (s *ReportService) GetInventoryTurnover(ctx context.Context, orgID string) (*db.InventoryTurnoverReport, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	report := &db.InventoryTurnoverReport{
		OrgID:              orgID,
		AsOf:               time.Now().UTC().Format("2006-01-02"),
		TrailingPeriodDays: 365,
		DeadStock:          make([]db.DeadStockItem, 0),
	}

	val, err := s.GetInventoryValuation(ctx, orgID)
	if err != nil {
		return nil, err
	}
	report.AvgInventoryValue = val.TotalCostValuation

	// COGS from Account 5000 in trailing 365 days
	sinceDate := time.Now().UTC().AddDate(0, 0, -365).Format("2006-01-02")
	cogsQuery := `
	SELECT COALESCE(SUM(jl.amount_cents), 0)
	FROM journal_lines jl
	JOIN accounts a ON jl.account_id = a.id
	JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
	WHERE a.org_id = ? AND a.code = '5000' AND jl.entry_type = 'debit'
		AND substr(je.posted_at, 1, 10) >= ?`

	var cogsCents int64
	_ = s.db.QueryRowContext(ctx, cogsQuery, orgID, sinceDate).Scan(&cogsCents)
	report.COGSTrailing = float64(cogsCents) / 100.0

	if report.AvgInventoryValue > 0 && report.COGSTrailing > 0 {
		report.TurnoverRatio = math.Round((report.COGSTrailing/report.AvgInventoryValue)*100.0) / 100.0
		if report.TurnoverRatio > 0 {
			report.DaysSalesOfInventory = math.Round((365.0/report.TurnoverRatio)*10.0) / 10.0
		}
	}

	// Dead stock: SKUs with stock > 0 and no update in > 90 days
	deadStockCutoff := time.Now().UTC().AddDate(0, 0, -90).Format("2006-01-02")
	deadQuery := `
	SELECT id, code, name, category, stock_quantity, unit_cost, substr(updated_at, 1, 10) as last_activity
	FROM skus
	WHERE org_id = ? AND stock_quantity > 0 AND substr(updated_at, 1, 10) <= ?
	ORDER BY (stock_quantity * unit_cost) DESC`

	deadRows, err := s.db.QueryContext(ctx, deadQuery, orgID, deadStockCutoff)
	if err == nil {
		defer deadRows.Close()
		now := time.Now().UTC()
		for deadRows.Next() {
			var ds db.DeadStockItem
			if err := deadRows.Scan(&ds.SKUID, &ds.SKUCode, &ds.SKUName, &ds.Category, &ds.StockQuantity, &ds.UnitCost, &ds.LastActivity); err == nil {
				ds.TiedUpCapital = math.Round(float64(ds.StockQuantity)*ds.UnitCost*100.0) / 100.0
				lastTime, err := time.Parse("2006-01-02", ds.LastActivity)
				if err == nil {
					ds.DaysInactive = int(now.Sub(lastTime).Hours() / 24.0)
				} else {
					ds.DaysInactive = 90
				}
				report.DeadStockCount++
				report.DeadStockValue += ds.TiedUpCapital
				report.DeadStock = append(report.DeadStock, ds)
			}
		}
	}
	report.DeadStockValue = math.Round(report.DeadStockValue*100.0) / 100.0

	return report, nil
}

// ---------------- 6. SALES VELOCITY & PIPELINE ANALYTICS ----------------

func (s *ReportService) GetSalesVelocity(ctx context.Context, orgID string) (*db.SalesVelocityReport, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	report := &db.SalesVelocityReport{
		OrgID:  orgID,
		Stages: make([]db.StageMetric, 0),
	}

	// 1. Deals aggregation
	query := `
	SELECT stage, COUNT(*), COALESCE(SUM(value), 0.0), AVG(value), AVG(probability)
	FROM deals
	WHERE org_id = ?
	GROUP BY stage`

	rows, err := s.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query deal stages: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sm db.StageMetric
		var avgVal sql.NullFloat64
		var avgProb sql.NullFloat64
		if err := rows.Scan(&sm.Stage, &sm.DealCount, &sm.TotalValue, &avgVal, &avgProb); err != nil {
			return nil, fmt.Errorf("failed to scan stage metric: %w", err)
		}
		if avgVal.Valid {
			sm.AvgValue = math.Round(avgVal.Float64*100.0) / 100.0
		}
		if avgProb.Valid {
			sm.Probability = int(avgProb.Float64)
		}

		report.TotalDeals += sm.DealCount
		if sm.Stage == "won" {
			report.WonDeals += sm.DealCount
			report.TotalWonValue += sm.TotalValue
		} else if sm.Stage == "lost" {
			report.LostDeals += sm.DealCount
		} else {
			report.OpenDeals += sm.DealCount
			report.PipelineValue += sm.TotalValue
			report.WeightedPipelineValue += (sm.TotalValue * float64(sm.Probability) / 100.0)
		}
		report.Stages = append(report.Stages, sm)
	}

	totalClosed := report.WonDeals + report.LostDeals
	if totalClosed > 0 {
		report.WinLossRatio = math.Round((float64(report.WonDeals)/float64(totalClosed))*10000.0) / 100.0
	}
	if report.WonDeals > 0 {
		report.AverageDealSize = math.Round((report.TotalWonValue/float64(report.WonDeals))*100.0) / 100.0
	}
	report.PipelineValue = math.Round(report.PipelineValue*100.0) / 100.0
	report.WeightedPipelineValue = math.Round(report.WeightedPipelineValue*100.0) / 100.0

	// 2. Average Sales Cycle duration (days from created_at to updated_at for won/lost deals)
	cycleQuery := `
	SELECT AVG(julianday('now') - julianday(created_at))
	FROM deals
	WHERE org_id = ? AND stage IN ('won', 'lost')`
	var avgCycle sql.NullFloat64
	_ = s.db.QueryRowContext(ctx, cycleQuery, orgID).Scan(&avgCycle)
	if avgCycle.Valid {
		report.AvgSalesCycleDays = math.Round(avgCycle.Float64*10.0) / 10.0
	} else {
		report.AvgSalesCycleDays = 14.0 // Baseline average
	}

	// 3. Leads conversion
	leadQuery := `
	SELECT COUNT(*),
		SUM(CASE WHEN status = 'converted' THEN 1 ELSE 0 END)
	FROM inbound_leads
	WHERE org_id = ?`
	_ = s.db.QueryRowContext(ctx, leadQuery, orgID).Scan(&report.TotalLeads, &report.ConvertedLeads)
	if report.TotalLeads > 0 {
		report.LeadConversionRate = math.Round((float64(report.ConvertedLeads)/float64(report.TotalLeads))*10000.0) / 100.0
	}

	return report, nil
}

// ---------------- 7. EXECUTIVE DASHBOARD & KPI CARDS ----------------

func (s *ReportService) GetExecutiveKPIs(ctx context.Context, orgID string) (*db.ExecutiveKPIsReport, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	kpis := &db.ExecutiveKPIsReport{
		OrgID:       orgID,
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04:05"),
	}

	// 1. Operating Cash from Account 1000
	cashQuery := `
	SELECT COALESCE(SUM(CASE WHEN jl.entry_type = 'debit' THEN jl.amount_cents ELSE -jl.amount_cents END), 0)
	FROM journal_lines jl
	JOIN accounts a ON jl.account_id = a.id
	JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
	WHERE a.org_id = ? AND a.code = '1000'`
	var cashCents int64
	_ = s.db.QueryRowContext(ctx, cashQuery, orgID).Scan(&cashCents)
	kpis.OperatingCash = float64(cashCents) / 100.0

	// 2. YTD Revenue & Net Profit
	yearStart := fmt.Sprintf("%d-01-01", time.Now().UTC().Year())
	today := time.Now().UTC().Format("2006-01-02")
	pnlYTD, err := s.GetProfitAndLoss(ctx, orgID, yearStart, today)
	if err == nil {
		kpis.TotalRevenueYTD = pnlYTD.Revenue
		kpis.NetProfitYTD = pnlYTD.NetProfit
		kpis.GrossMarginPct = pnlYTD.GrossMarginPct
	}

	// 3. Monthly Recurring Revenue (MRR) from active recurring invoices
	mrrQuery := `
	SELECT COALESCE(SUM(CASE
		WHEN recurring_frequency = 'monthly' THEN total_amount
		WHEN recurring_frequency = 'quarterly' THEN total_amount / 3.0
		WHEN recurring_frequency = 'annual' THEN total_amount / 12.0
		ELSE 0.0 END), 0.0)
	FROM invoices
	WHERE org_id = ? AND recurring_frequency IS NOT NULL AND recurring_frequency != '' AND recurring_frequency != 'none'`
	_ = s.db.QueryRowContext(ctx, mrrQuery, orgID).Scan(&kpis.MRR)
	kpis.MRR = math.Round(kpis.MRR*100.0) / 100.0

	// 4. Monthly Burn Rate (Average OpEx in trailing 90 days normalized to 30 days)
	burnQuery := `
	SELECT COALESCE(SUM(jl.amount_cents), 0)
	FROM journal_lines jl
	JOIN accounts a ON jl.account_id = a.id
	JOIN journal_entries je ON jl.entry_id = je.id AND je.status = 'posted'
	WHERE a.org_id = ? AND a.type = 'expense' AND a.code != '5000'
		AND substr(je.posted_at, 1, 10) >= date('now', '-90 days')`
	var burnCents int64
	_ = s.db.QueryRowContext(ctx, burnQuery, orgID).Scan(&burnCents)
	monthlyBurn := (float64(burnCents) / 100.0) / 3.0
	kpis.MonthlyBurnRate = math.Round(monthlyBurn*100.0) / 100.0

	// 5. Cash Runway in Months
	if kpis.MonthlyBurnRate > 0 && kpis.OperatingCash > 0 {
		kpis.CashRunwayMonths = math.Round((kpis.OperatingCash/kpis.MonthlyBurnRate)*10.0) / 10.0
	} else if kpis.OperatingCash > 0 {
		kpis.CashRunwayMonths = 99.0 // Infinite runway if zero burn
	}

	// 6. Open Purchase Commitments
	poCommitQuery := `
	SELECT COALESCE(SUM(total_amount), 0.0)
	FROM purchase_orders
	WHERE org_id = ? AND status IN ('ordered', 'pending_approval')`
	_ = s.db.QueryRowContext(ctx, poCommitQuery, orgID).Scan(&kpis.OpenPurchaseCommitments)
	kpis.OpenPurchaseCommitments = math.Round(kpis.OpenPurchaseCommitments*100.0) / 100.0

	// 7. Pending Approvals Count (POs > $1000 + pending leave requests)
	var poApprovals, leaveApprovals int
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM purchase_orders WHERE org_id = ? AND status = 'pending_approval'`, orgID).Scan(&poApprovals)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM leave_requests WHERE org_id = ? AND status = 'pending'`, orgID).Scan(&leaveApprovals)
	kpis.PendingApprovalsCount = poApprovals + leaveApprovals

	// 8. Total Inventory Valuation
	invVal, err := s.GetInventoryValuation(ctx, orgID)
	if err == nil {
		kpis.TotalInventoryValuation = invVal.TotalCostValuation
	}

	// 9. Accounts Receivable Current vs Overdue
	agedAR, err := s.GetAgedReceivables(ctx, orgID, today)
	if err == nil {
		kpis.AccountsReceivableCurrent = agedAR.Current
		kpis.AccountsReceivableOverdue = agedAR.Total - agedAR.Current
	}

	return kpis, nil
}

// ---------------- 8. UNIVERSAL DATA EXPORT (CSV, JSON, PDF) ----------------

func (s *ReportService) ExportData(ctx context.Context, orgID, exportType, fileFormat, startDate, endDate string) ([]byte, string, string, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if fileFormat == "" {
		fileFormat = "csv"
	}
	exportType = strings.ToLower(strings.TrimSpace(exportType))
	fileFormat = strings.ToLower(strings.TrimSpace(fileFormat))

	nowStr := time.Now().UTC().Format("20060102_150405")
	baseFilename := fmt.Sprintf("%s_export_%s", exportType, nowStr)

	exportType = strings.ToLower(strings.ReplaceAll(exportType, "-", "_"))
	fileFormat = strings.ToLower(fileFormat)

	var headers []string
	var rows [][]string
	var jsonPayload interface{}
	var reportTitle = strings.ToUpper(strings.ReplaceAll(exportType, "_", " "))

	switch exportType {
	case "pnl":
		pnl, err := s.GetProfitAndLoss(ctx, orgID, startDate, endDate)
		if err != nil {
			return nil, "", "", err
		}
		jsonPayload = pnl
		headers = []string{"Category", "Account Code", "Account Name", "Amount ($)"}
		for _, item := range pnl.RevenueItems {
			rows = append(rows, []string{"Revenue", item.AccountCode, item.AccountName, fmt.Sprintf("%.2f", item.Amount)})
		}
		rows = append(rows, []string{"Summary", "---", "Total Revenue", fmt.Sprintf("%.2f", pnl.Revenue)})
		for _, item := range pnl.COGSItems {
			rows = append(rows, []string{"COGS", item.AccountCode, item.AccountName, fmt.Sprintf("%.2f", item.Amount)})
		}
		rows = append(rows, []string{"Summary", "---", "Gross Profit", fmt.Sprintf("%.2f", pnl.GrossProfit)})
		for _, item := range pnl.OpExItems {
			rows = append(rows, []string{"Operating Expense", item.AccountCode, item.AccountName, fmt.Sprintf("%.2f", item.Amount)})
		}
		rows = append(rows, []string{"Summary", "---", "Total OpEx", fmt.Sprintf("%.2f", pnl.OpEx)})
		rows = append(rows, []string{"Summary", "---", "Net Profit", fmt.Sprintf("%.2f", pnl.NetProfit)})

	case "balance_sheet":
		bs, err := s.GetBalanceSheet(ctx, orgID, endDate)
		if err != nil {
			return nil, "", "", err
		}
		jsonPayload = bs
		headers = []string{"Section", "Account Code", "Account Name", "Amount ($)"}
		for _, acc := range bs.CurrentAssets {
			rows = append(rows, []string{"Assets", acc.Code, acc.Name, fmt.Sprintf("%.2f", acc.Amount)})
		}
		rows = append(rows, []string{"Summary", "---", "Total Assets", fmt.Sprintf("%.2f", bs.TotalAssets)})
		for _, acc := range bs.CurrentLiabilities {
			rows = append(rows, []string{"Liabilities", acc.Code, acc.Name, fmt.Sprintf("%.2f", acc.Amount)})
		}
		rows = append(rows, []string{"Summary", "---", "Total Liabilities", fmt.Sprintf("%.2f", bs.TotalLiabilities)})
		for _, acc := range bs.EquityItems {
			rows = append(rows, []string{"Equity", acc.Code, acc.Name, fmt.Sprintf("%.2f", acc.Amount)})
		}
		rows = append(rows, []string{"Summary", "---", "Total Equity", fmt.Sprintf("%.2f", bs.TotalEquity)})

	case "aging_ar":
		ar, err := s.GetAgedReceivables(ctx, orgID, endDate)
		if err != nil {
			return nil, "", "", err
		}
		jsonPayload = ar
		headers = []string{"Invoice Number", "Customer Name", "Issue Date", "Due Date", "Amount ($)", "Days Overdue", "Aging Bracket"}
		for _, item := range ar.Items {
			rows = append(rows, []string{item.Number, item.EntityName, item.IssueDate, item.DueDate, fmt.Sprintf("%.2f", item.Amount), fmt.Sprintf("%d", item.DaysOverdue), item.Bracket})
		}

	case "aging_ap":
		ap, err := s.GetAgedPayables(ctx, orgID, endDate)
		if err != nil {
			return nil, "", "", err
		}
		jsonPayload = ap
		headers = []string{"Bill Number", "Supplier Name", "Issue Date", "Due Date", "Amount ($)", "Days Overdue", "Aging Bracket"}
		for _, item := range ap.Items {
			rows = append(rows, []string{item.Number, item.EntityName, item.IssueDate, item.DueDate, fmt.Sprintf("%.2f", item.Amount), fmt.Sprintf("%d", item.DaysOverdue), item.Bracket})
		}

	case "inventory":
		inv, err := s.GetInventoryValuation(ctx, orgID)
		if err != nil {
			return nil, "", "", err
		}
		jsonPayload = inv
		headers = []string{"SKU Code", "SKU Name", "Category", "On Hand", "Unit Cost ($)", "Selling Price ($)", "Total Cost Valuation ($)"}
		for _, item := range inv.SKUs {
			rows = append(rows, []string{item.SKUCode, item.SKUName, item.Category, fmt.Sprintf("%d", item.StockQuantity), fmt.Sprintf("%.2f", item.UnitCost), fmt.Sprintf("%.2f", item.SellingPrice), fmt.Sprintf("%.2f", item.CostValuation)})
		}

	case "invoices":
		headers = []string{"Invoice Number", "Customer ID", "Status", "Issue Date", "Due Date", "Subtotal ($)", "Tax ($)", "Total ($)"}
		invRows, err := s.db.QueryContext(ctx, `SELECT invoice_number, customer_id, status, issue_date, due_date, subtotal, tax_amount, total_amount FROM invoices WHERE org_id = ? ORDER BY created_at DESC`, orgID)
		if err == nil {
			defer invRows.Close()
			for invRows.Next() {
				var invNum, custID, status, issDate, dueDate string
				var sub, tax, tot float64
				if err := invRows.Scan(&invNum, &custID, &status, &issDate, &dueDate, &sub, &tax, &tot); err == nil {
					rows = append(rows, []string{invNum, custID, status, issDate, dueDate, fmt.Sprintf("%.2f", sub), fmt.Sprintf("%.2f", tax), fmt.Sprintf("%.2f", tot)})
				}
			}
		}

	case "purchase_orders":
		headers = []string{"PO Number", "Supplier ID", "Status", "Total Amount ($)", "Created At"}
		poRows, err := s.db.QueryContext(ctx, `SELECT po_number, supplier_id, status, total_amount, substr(created_at, 1, 10) FROM purchase_orders WHERE org_id = ? ORDER BY created_at DESC`, orgID)
		if err == nil {
			defer poRows.Close()
			for poRows.Next() {
				var poNum, supID, status, crDate string
				var tot float64
				if err := poRows.Scan(&poNum, &supID, &status, &tot, &crDate); err == nil {
					rows = append(rows, []string{poNum, supID, status, fmt.Sprintf("%.2f", tot), crDate})
				}
			}
		}

	case "employees":
		headers = []string{"Name", "Role", "Department", "Employment Type", "Salary Model", "Hourly Rate ($)", "Monthly Salary ($)", "Status"}
		empRows, err := s.db.QueryContext(ctx, `SELECT name, role, department, employment_type, salary_type, hourly_rate, monthly_salary, status FROM employees WHERE org_id = ? ORDER BY name ASC`, orgID)
		if err == nil {
			defer empRows.Close()
			for empRows.Next() {
				var name, role, dept, empType, salType, status string
				var hrRate, moSal float64
				if err := empRows.Scan(&name, &role, &dept, &empType, &salType, &hrRate, &moSal, &status); err == nil {
					rows = append(rows, []string{name, role, dept, empType, salType, fmt.Sprintf("%.2f", hrRate), fmt.Sprintf("%.2f", moSal), status})
				}
			}
		}

	default:
		return nil, "", "", fmt.Errorf("unsupported export type: %s", exportType)
	}

	// Format Output
	var content []byte
	var contentType string
	var filename string

	switch fileFormat {
	case "json":
		contentType = "application/json"
		filename = baseFilename + ".json"
		if jsonPayload != nil {
			content, _ = json.MarshalIndent(jsonPayload, "", "  ")
		} else {
			content, _ = json.MarshalIndent(rows, "", "  ")
		}

	case "pdf":
		contentType = "application/pdf"
		filename = baseFilename + ".pdf"
		metrics := []struct{ Label, Value string }{
			{Label: "Export Type", Value: strings.ToUpper(exportType)},
			{Label: "Total Records", Value: fmt.Sprintf("%d", len(rows))},
			{Label: "As Of", Value: time.Now().UTC().Format("2006-01-02")},
		}
		var err error
		content, err = pdf.GenerateReportPDF(nil, reportTitle, "Official Executive Statement", metrics, headers, rows)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to generate report PDF: %w", err)
		}

	default: // CSV
		contentType = "text/csv"
		filename = baseFilename + ".csv"
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		_ = w.Write(headers)
		for _, r := range rows {
			_ = w.Write(r)
		}
		w.Flush()
		content = buf.Bytes()
	}

	// Record export metadata in data_exports table
	exportID := "exp_" + uuid.New().String()[:8]
	_, _ = s.db.ExecContext(ctx, `INSERT INTO data_exports (id, org_id, export_type, file_format, record_count, file_size_bytes, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'system', ?)`,
		exportID, orgID, exportType, fileFormat, len(rows), len(content), time.Now().UTC())

	return content, contentType, filename, nil
}
