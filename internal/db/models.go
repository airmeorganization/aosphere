package db

import "time"

// ----------------- AUTH & TENANCY MODELS -----------------

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID                string     `json:"id"`
	OrgID             string     `json:"org_id"`
	Email             string     `json:"email"`
	PasswordHash      string     `json:"-"`
	Name              string     `json:"name"`
	Role              string     `json:"role"` // owner, admin, finance, operations, hr, employee, readonly
	IsActive          bool       `json:"is_active"`
	TOTPSecret        string     `json:"-"`
	TOTPEnabled       bool       `json:"totp_enabled"`
	RecoveryCodesJSON string     `json:"-"`
	LastActiveAt      *time.Time `json:"last_active_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type Session struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	OrgID          string    `json:"org_id"`
	TokenHash      string    `json:"-"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	LastActivityAt time.Time `json:"last_activity_at"`
}

type APIKey struct {
	ID           string     `json:"id"`
	OrgID        string     `json:"org_id"`
	UserID       string     `json:"user_id,omitempty"`
	Name         string     `json:"name"`
	KeyHash      string     `json:"-"`
	Role         string     `json:"role"`
	AllowedTools string     `json:"allowed_tools"`
	IsActive     bool       `json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
}

// ----------------- BUSINESS ENTITY MODELS -----------------

type SKU struct {
	ID               string    `json:"id"`
	OrgID            string    `json:"org_id"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	Category         string    `json:"category"`
	StockQuantity    int       `json:"stock_quantity"`
	ReservedQuantity int       `json:"reserved_quantity"`
	MinThreshold     int       `json:"min_threshold"`
	ReorderQuantity  int       `json:"reorder_quantity"`
	UnitCost         float64   `json:"unit_cost"`
	SellingPrice     float64   `json:"selling_price"`
	SupplierID       string    `json:"supplier_id"`
	SupplierName     string    `json:"supplier_name,omitempty"`
	Version          int       `json:"version"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Supplier struct {
	ID           string  `json:"id"`
	OrgID        string  `json:"org_id"`
	Name         string  `json:"name"`
	ContactEmail string  `json:"contact_email"`
	Phone        string  `json:"phone"`
	LeadTimeDays int     `json:"lead_time_days"`
	Rating       float64 `json:"rating"`
}

type PurchaseOrder struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	PONumber      string    `json:"po_number"`
	SupplierID    string    `json:"supplier_id"`
	SupplierName  string    `json:"supplier_name,omitempty"`
	Status        string    `json:"status"` // draft, pending_approval, ordered, received, cancelled
	TotalAmount   float64   `json:"total_amount"`
	ItemsJSON     string    `json:"items_json"`
	CreatedByType string    `json:"created_by_type"` // human, agent
	ActorID       string    `json:"actor_id"`
	Notes         string    `json:"notes"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
}

type Customer struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	Name          string    `json:"name"`
	Company       string    `json:"company"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	Status        string    `json:"status"`
	LifetimeValue float64   `json:"lifetime_value"`
	Tags          string    `json:"tags"`
	CreatedAt     time.Time `json:"created_at"`
}

type Deal struct {
	ID                string    `json:"id"`
	OrgID             string    `json:"org_id"`
	Title             string    `json:"title"`
	CustomerID        string    `json:"customer_id"`
	CustomerName      string    `json:"customer_name,omitempty"`
	CustomerCompany   string    `json:"customer_company,omitempty"`
	Value             float64   `json:"value"`
	Stage             string    `json:"stage"` // lead, qualified, proposal, negotiation, won, lost
	Probability       int       `json:"probability"`
	ExpectedCloseDate string    `json:"expected_close_date"`
	CreatedAt         time.Time `json:"created_at"`
}

type Invoice struct {
	ID                 string    `json:"id"`
	OrgID              string    `json:"org_id"`
	InvoiceNumber      string    `json:"invoice_number"`
	CustomerID         string    `json:"customer_id"`
	CustomerName       string    `json:"customer_name,omitempty"`
	Status             string    `json:"status"` // draft, sent, paid, overdue
	IssueDate          string    `json:"issue_date"`
	DueDate            string    `json:"due_date"`
	Subtotal           float64   `json:"subtotal"`
	TaxAmount          float64   `json:"tax_amount"`
	TotalAmount        float64   `json:"total_amount"`
	RecurringFrequency string    `json:"recurring_frequency"`
	ItemsJSON          string    `json:"items_json"`
	Version            int       `json:"version"`
	CreatedAt          time.Time `json:"created_at"`
}

type Expense struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Category    string    `json:"category"`
	Amount      float64   `json:"amount"`
	Vendor      string    `json:"vendor"`
	Description string    `json:"description"`
	Date        string    `json:"date"`
	CreatedAt   time.Time `json:"created_at"`
}

type Employee struct {
	ID                    string    `json:"id"`
	OrgID                 string    `json:"org_id"`
	Name                  string    `json:"name"`
	Role                  string    `json:"role"`
	JobTitle              string    `json:"job_title,omitempty"`
	Department            string    `json:"department"`
	ReportingManagerID    string    `json:"reporting_manager_id,omitempty"`
	EmploymentType        string    `json:"employment_type,omitempty"` // full_time, part_time, contractor
	SalaryType            string    `json:"salary_type,omitempty"`     // hourly, monthly_fixed
	HourlyRate            float64   `json:"hourly_rate"`
	MonthlySalary         float64   `json:"monthly_salary,omitempty"`
	Status                string    `json:"status"`
	Email                 string    `json:"email"`
	Phone                 string    `json:"phone"`
	TaxID                 string    `json:"tax_id,omitempty"`
	BankName              string    `json:"bank_name,omitempty"`
	BankAccountNumber     string    `json:"bank_account_number,omitempty"`
	BankRoutingNumber     string    `json:"bank_routing_number,omitempty"`
	EmergencyContactName  string    `json:"emergency_contact_name,omitempty"`
	EmergencyContactPhone string    `json:"emergency_contact_phone,omitempty"`
	HiredAt               string    `json:"hired_at"`
	CreatedAt             time.Time `json:"created_at"`
}

type TimeclockEvent struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	EmployeeID    string    `json:"employee_id"`
	ShiftID       string    `json:"shift_id,omitempty"`
	EventType     string    `json:"event_type"` // clock_in, clock_out, break_start, break_end
	Timestamp     time.Time `json:"timestamp"`
	IPAddress     string    `json:"ip_address,omitempty"`
	LocationNotes string    `json:"location_notes,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type PayrollRun struct {
	ID                  string    `json:"id"`
	OrgID               string    `json:"org_id"`
	Period              string    `json:"period"` // YYYY-MM
	TotalGrossCents     int64     `json:"total_gross_cents"`
	TotalTaxCents       int64     `json:"total_tax_cents"`
	TotalDeductionsCents int64    `json:"total_deductions_cents"`
	TotalNetCents       int64     `json:"total_net_cents"`
	TotalGrossDollars   float64   `json:"total_gross_dollars"`
	TotalTaxDollars     float64   `json:"total_tax_dollars"`
	TotalNetDollars     float64   `json:"total_net_dollars"`
	Status              string    `json:"status"` // draft, approved, paid
	PaymentDate         string    `json:"payment_date"`
	JournalEntryID      string    `json:"journal_entry_id,omitempty"`
	PayslipsCount       int       `json:"payslips_count"`
	CreatedAt           time.Time `json:"created_at"`
}

type Payslip struct {
	ID                  string    `json:"id"`
	OrgID               string    `json:"org_id"`
	PayrollRunID        string    `json:"payroll_run_id"`
	EmployeeID          string    `json:"employee_id"`
	EmployeeName        string    `json:"employee_name,omitempty"`
	Period              string    `json:"period"`
	SalaryType          string    `json:"salary_type"`
	HourlyRate          float64   `json:"hourly_rate"`
	MonthlySalary       float64   `json:"monthly_salary"`
	RegularHours        float64   `json:"regular_hours"`
	OvertimeHours       float64   `json:"overtime_hours"`
	GrossPay            float64   `json:"gross_pay"`
	TaxWithholding      float64   `json:"tax_withholding"`
	SocialContributions float64   `json:"social_contributions"`
	BenefitsDeductions  float64   `json:"benefits_deductions"`
	NetPay              float64   `json:"net_pay"`
	Status              string    `json:"status"` // generated, paid
	PaymentDate         string    `json:"payment_date,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

type LeavePolicy struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	Name       string    `json:"name"`
	Code       string    `json:"code"` // vacation, sick, parental, unpaid
	AnnualDays int       `json:"annual_days"`
	CreatedAt  time.Time `json:"created_at"`
}

type LeaveRequest struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	EmployeeID   string    `json:"employee_id"`
	EmployeeName string    `json:"employee_name,omitempty"`
	LeaveType    string    `json:"leave_type"`
	StartDate    string    `json:"start_date"`
	EndDate      string    `json:"end_date"`
	DaysCount    int       `json:"days_count"`
	Reason       string    `json:"reason"`
	Status       string    `json:"status"` // pending, approved, rejected
	ReviewedBy   string    `json:"reviewed_by,omitempty"`
	ReviewNotes  string    `json:"review_notes,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type HRDocument struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	EmployeeID   string    `json:"employee_id"`
	EmployeeName string    `json:"employee_name,omitempty"`
	Title        string    `json:"title"`
	DocumentType string    `json:"document_type"` // contract, nda, id_proof, evaluation, tax_form
	FileName     string    `json:"file_name"`
	FileContent  string    `json:"file_content"`
	FileSize     int64     `json:"file_size"`
	UploadedBy   string    `json:"uploaded_by"`
	CreatedAt    time.Time `json:"created_at"`
}

type Shift struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	EmployeeID   string    `json:"employee_id"`
	EmployeeName string    `json:"employee_name,omitempty"`
	ShiftDate    string    `json:"shift_date"`
	StartTime    string    `json:"start_time"`
	EndTime      string    `json:"end_time"`
	Station      string    `json:"station"`
	Status       string    `json:"status"` // scheduled, clocked_in, completed, missed
	CreatedAt    time.Time `json:"created_at"`
}

type Task struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	ProjectName  string    `json:"project_name"`
	AssigneeID   string    `json:"assignee_id"`
	AssigneeName string    `json:"assignee_name,omitempty"`
	Priority     string    `json:"priority"` // low, medium, high, urgent
	Status       string    `json:"status"`   // todo, in_progress, review, done
	DueDate      string    `json:"due_date"`
	CreatedAt    time.Time `json:"created_at"`
}

type LocalListing struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id"`
	Directory       string    `json:"directory"`
	NAPBusinessName string    `json:"nap_business_name"`
	NAPAddress      string    `json:"nap_address"`
	NAPPhone        string    `json:"nap_phone"`
	NAPWebsite      string    `json:"nap_website"`
	SyncStatus      string    `json:"sync_status"` // synced, pending, error
	LastSyncedAt    time.Time `json:"last_synced_at"`
}

type Review struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Platform    string    `json:"platform"`
	AuthorName  string    `json:"author_name"`
	Rating      int       `json:"rating"`
	ReviewText  string    `json:"review_text"`
	Sentiment   string    `json:"sentiment"` // positive, neutral, negative, urgent
	ReplyText   string    `json:"reply_text"`
	ReplyStatus string    `json:"reply_status"` // unreplied, drafted_by_agent, published
	CreatedAt   time.Time `json:"created_at"`
}

type Approval struct {
	ID              string     `json:"id"`
	OrgID           string     `json:"org_id"`
	ActionType      string     `json:"action_type"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	PayloadJSON     string     `json:"payload_json"`
	PayloadHash     string     `json:"payload_hash"`
	EntityType      string     `json:"entity_type"`
	EntityID        string     `json:"entity_id"`
	EntityVersion   int        `json:"entity_version"`
	Status          string     `json:"status"` // pending, approved, rejected
	ExecutionStatus string     `json:"execution_status"` // none, executing, completed, failed
	ExecutionError  string     `json:"execution_error,omitempty"`
	RequestedByType string     `json:"requested_by_type"`
	RequestedByID   string     `json:"requested_by_id"`
	DecidedBy       *string    `json:"decided_by,omitempty"`
	DecidedAt       *time.Time `json:"decided_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type IdempotencyRecord struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	Key          string    `json:"key"`
	RequestHash  string    `json:"request_hash"`
	Status       string    `json:"status"` // in_progress, completed, failed
	ResponseCode int       `json:"response_code"`
	ResponseBody string    `json:"response_body"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type AuditLog struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	ActorType    string    `json:"actor_type"`
	ActorID      string    `json:"actor_id"`
	Action       string    `json:"action"`
	EntityType   string    `json:"entity_type"`
	EntityID     string    `json:"entity_id"`
	Reasoning    string    `json:"reasoning"`
	MetadataJSON string    `json:"metadata_json"`
	CreatedAt    time.Time `json:"created_at"`
}

type FinancialSummary struct {
	TotalRevenue float64 `json:"total_revenue"`
	TotalExpense float64 `json:"total_expense"`
	NetProfit    float64 `json:"net_profit"`
	UnpaidTotal  float64 `json:"unpaid_total"`
	PaidCount    int     `json:"paid_count"`
	PendingCount int     `json:"pending_count"`
}

type Document struct {
	ID        string `json:"id"`
	OrgID     string `json:"org_id"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	Content   string `json:"content"`
	Author    string `json:"author"`
	UpdatedAt string `json:"updated_at"`
}

type Timesheet struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	EmployeeID    string    `json:"employee_id"`
	EmployeeName  string    `json:"employee_name,omitempty"`
	WorkDate      string    `json:"work_date"`
	HoursWorked   float64   `json:"hours_worked"`
	OvertimeHours float64   `json:"overtime_hours"`
	Status        string    `json:"status"` // logged, approved, paid
	HourlyRate    float64   `json:"hourly_rate"`
	CreatedAt     time.Time `json:"created_at"`
}

// ----------------- SUPPLY CHAIN & MULTI-WAREHOUSE MODELS -----------------

type Warehouse struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Address   string    `json:"address"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type WarehouseStock struct {
	ID               string    `json:"id"`
	OrgID            string    `json:"org_id"`
	WarehouseID      string    `json:"warehouse_id"`
	WarehouseName    string    `json:"warehouse_name,omitempty"`
	WarehouseCode    string    `json:"warehouse_code,omitempty"`
	SKUID            string    `json:"sku_id"`
	SKUCode          string    `json:"sku_code,omitempty"`
	SKUName          string    `json:"sku_name,omitempty"`
	StockQuantity    int       `json:"stock_quantity"`
	ReservedQuantity int       `json:"reserved_quantity"`
	BinLocation      string    `json:"bin_location"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type GoodsReceiptNote struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	GRNNumber     string    `json:"grn_number"`
	POID          string    `json:"po_id"`
	PONumber      string    `json:"po_number,omitempty"`
	WarehouseID   string    `json:"warehouse_id"`
	WarehouseName string    `json:"warehouse_name,omitempty"`
	ReceivedBy    string    `json:"received_by"`
	Status        string    `json:"status"` // completed, cancelled
	ItemsJSON     string    `json:"items_json"`
	Notes         string    `json:"notes"`
	CreatedAt     time.Time `json:"created_at"`
}

type SupplierBill struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	BillNumber   string    `json:"bill_number"`
	SupplierID   string    `json:"supplier_id"`
	SupplierName string    `json:"supplier_name,omitempty"`
	POID         string    `json:"po_id"`
	PONumber     string    `json:"po_number,omitempty"`
	Amount       float64   `json:"amount"`
	Status       string    `json:"status"`       // open, paid, void
	MatchStatus  string    `json:"match_status"` // pending, matched, discrepancy_price, discrepancy_qty, unreceived
	ItemsJSON    string    `json:"items_json"`
	Notes        string    `json:"notes"`
	CreatedAt    time.Time `json:"created_at"`
}

type StockTransfer struct {
	ID                string    `json:"id"`
	OrgID             string    `json:"org_id"`
	TransferNumber    string    `json:"transfer_number"`
	FromWarehouseID   string    `json:"from_warehouse_id"`
	FromWarehouseName string    `json:"from_warehouse_name,omitempty"`
	ToWarehouseID     string    `json:"to_warehouse_id"`
	ToWarehouseName   string    `json:"to_warehouse_name,omitempty"`
	Status            string    `json:"status"` // completed, in_transit, cancelled
	ItemsJSON         string    `json:"items_json"`
	Notes             string    `json:"notes"`
	CreatedBy         string    `json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
}

type BatchLot struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	SKUID         string    `json:"sku_id"`
	SKUCode       string    `json:"sku_code,omitempty"`
	SKUName       string    `json:"sku_name,omitempty"`
	BatchNumber   string    `json:"batch_number"`
	ExpiryDate    string    `json:"expiry_date"`
	Quantity      int       `json:"quantity"`
	WarehouseID   string    `json:"warehouse_id"`
	WarehouseName string    `json:"warehouse_name,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// ----------------- PHASE 4: FINANCES, CREDIT NOTES & GENERAL LEDGER -----------------

type CompanyProfile struct {
	OrgID               string    `json:"org_id"`
	CompanyName         string    `json:"company_name"`
	Currency            string    `json:"currency"`
	Timezone            string    `json:"timezone"`
	TaxRate             float64   `json:"tax_rate"`
	Address             string    `json:"address"`
	Phone               string    `json:"phone"`
	Website             string    `json:"website"`
	OnboardingCompleted bool      `json:"onboarding_completed"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type CreditNote struct {
	ID               string    `json:"id"`
	OrgID            string    `json:"org_id"`
	CreditNoteNumber string    `json:"credit_note_number"`
	InvoiceID        string    `json:"invoice_id"`
	InvoiceNumber    string    `json:"invoice_number,omitempty"`
	CustomerID       string    `json:"customer_id"`
	CustomerName     string    `json:"customer_name,omitempty"`
	Currency         string    `json:"currency"`
	Subtotal         float64   `json:"subtotal"`
	TaxAmount        float64   `json:"tax_amount"`
	TotalAmount      float64   `json:"total_amount"`
	TotalCents       int64     `json:"total_cents"`
	Reason           string    `json:"reason"` // returned_goods, billing_error, discount_adjustment, service_credit
	Status           string    `json:"status"` // issued, applied, refunded, void
	ItemsJSON        string    `json:"items_json"`
	CreatedAt        time.Time `json:"created_at"`
}

type ExchangeRate struct {
	ID             string    `json:"id"`
	OrgID          string    `json:"org_id"`
	BaseCurrency   string    `json:"base_currency"`
	TargetCurrency string    `json:"target_currency"`
	Rate           float64   `json:"rate"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CurrencyInfo struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Symbol      string  `json:"symbol"`
	Decimals    int     `json:"decimals"`
	Region      string  `json:"region"`
	DefaultRate float64 `json:"default_rate"`
}

type Account struct {
	ID            string  `json:"id"`
	OrgID         string  `json:"org_id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Type          string  `json:"type"` // asset, liability, equity, revenue, expense
	NormalBalance string  `json:"normal_balance"` // debit, credit
	DebitCents    int64   `json:"debit_cents"`
	CreditCents   int64   `json:"credit_cents"`
	NetBalance    float64 `json:"net_balance"`
	IsActive      bool    `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
}

type JournalEntry struct {
	ID          string        `json:"id"`
	OrgID       string        `json:"org_id"`
	EntryNumber string        `json:"entry_number"`
	SourceType  string        `json:"source_type"`
	SourceID    string        `json:"source_id"`
	Memo        string        `json:"memo"`
	Status      string        `json:"status"` // posted, reversed
	PostedAt    time.Time     `json:"posted_at"`
	CreatedBy   string        `json:"created_by"`
	Lines       []JournalLine `json:"lines,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
}

type JournalLine struct {
	ID          string    `json:"id,omitempty"`
	OrgID       string    `json:"org_id,omitempty"`
	EntryID     string    `json:"entry_id,omitempty"`
	AccountID   string    `json:"account_id,omitempty"`
	AccountCode string    `json:"account_code"`
	AccountName string    `json:"account_name,omitempty"`
	EntryType   string    `json:"entry_type"` // debit, credit
	AmountCents int64     `json:"amount_cents"`
	Amount      float64   `json:"amount"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

// ----------------- PHASE 7 REPORTING & ANALYTICS MODELS -----------------

type SavedReport struct {
	ID             string    `json:"id"`
	OrgID          string    `json:"org_id"`
	Name           string    `json:"name"`
	ReportType     string    `json:"report_type"`
	ParametersJSON string    `json:"parameters_json"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

type DataExport struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	ExportType    string    `json:"export_type"`
	FileFormat    string    `json:"file_format"`
	RecordCount   int       `json:"record_count"`
	FileSizeBytes int64     `json:"file_size_bytes"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

type PnLLineItem struct {
	AccountCode string  `json:"account_code"`
	AccountName string  `json:"account_name"`
	AmountCents int64   `json:"amount_cents"`
	Amount      float64 `json:"amount"`
}

type PnLReport struct {
	OrgID             string        `json:"org_id"`
	StartDate         string        `json:"start_date"`
	EndDate           string        `json:"end_date"`
	RevenueCents      int64         `json:"revenue_cents"`
	Revenue           float64       `json:"revenue"`
	COGSCents         int64         `json:"cogs_cents"`
	COGS              float64       `json:"cogs"`
	GrossProfitCents  int64         `json:"gross_profit_cents"`
	GrossProfit       float64       `json:"gross_profit"`
	GrossMarginPct    float64       `json:"gross_margin_pct"`
	OpExCents         int64         `json:"opex_cents"`
	OpEx              float64       `json:"opex"`
	NetProfitCents    int64         `json:"net_profit_cents"`
	NetProfit         float64       `json:"net_profit"`
	RevenueItems      []PnLLineItem `json:"revenue_items"`
	COGSItems         []PnLLineItem `json:"cogs_items"`
	OpExItems         []PnLLineItem `json:"opex_items"`
}

type BalanceSheetAccount struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	AmountCents int64   `json:"amount_cents"`
	Amount      float64 `json:"amount"`
}

type BalanceSheetReport struct {
	OrgID                 string                `json:"org_id"`
	AsOf                  string                `json:"as_of"`
	CurrentAssets         []BalanceSheetAccount `json:"current_assets"`
	TotalAssetsCents      int64                 `json:"total_assets_cents"`
	TotalAssets           float64               `json:"total_assets"`
	CurrentLiabilities    []BalanceSheetAccount `json:"current_liabilities"`
	TotalLiabilitiesCents int64                 `json:"total_liabilities_cents"`
	TotalLiabilities      float64               `json:"total_liabilities"`
	EquityItems           []BalanceSheetAccount `json:"equity_items"`
	TotalEquityCents      int64                 `json:"total_equity_cents"`
	TotalEquity           float64               `json:"total_equity"`
	IsBalanced            bool                  `json:"is_balanced"`
}

type CashFlowActivity struct {
	Category    string  `json:"category"`
	Description string  `json:"description"`
	AmountCents int64   `json:"amount_cents"`
	Amount      float64 `json:"amount"`
}

type CashFlowReport struct {
	OrgID               string             `json:"org_id"`
	StartDate           string             `json:"start_date"`
	EndDate             string             `json:"end_date"`
	OperatingActivities []CashFlowActivity `json:"operating_activities"`
	OperatingNetCents   int64              `json:"operating_net_cents"`
	OperatingNet        float64            `json:"operating_net"`
	InvestingActivities []CashFlowActivity `json:"investing_activities"`
	InvestingNetCents   int64              `json:"investing_net_cents"`
	InvestingNet        float64            `json:"investing_net"`
	FinancingActivities []CashFlowActivity `json:"financing_activities"`
	FinancingNetCents   int64              `json:"financing_net_cents"`
	FinancingNet        float64            `json:"financing_net"`
	NetCashChangeCents  int64              `json:"net_cash_change_cents"`
	NetCashChange       float64            `json:"net_cash_change"`
	BeginningCashCents  int64              `json:"beginning_cash_cents"`
	BeginningCash       float64            `json:"beginning_cash"`
	EndingCashCents     int64              `json:"ending_cash_cents"`
	EndingCash          float64            `json:"ending_cash"`
}

type AgedItem struct {
	ID           string  `json:"id"`
	Number       string  `json:"number"`
	EntityID     string  `json:"entity_id"`
	EntityName   string  `json:"entity_name"`
	IssueDate    string  `json:"issue_date"`
	DueDate      string  `json:"due_date"`
	AmountCents  int64   `json:"amount_cents"`
	Amount       float64 `json:"amount"`
	DaysOverdue  int     `json:"days_overdue"`
	Bracket      string  `json:"bracket"` // current, 1_30, 31_60, 61_90, 90_plus
}

type AgedReport struct {
	OrgID         string     `json:"org_id"`
	ReportType    string     `json:"report_type"` // receivables, payables
	AsOf          string     `json:"as_of"`
	CurrentCents  int64      `json:"current_cents"`
	Days1To30Cents int64     `json:"days_1_30_cents"`
	Days31To60Cents int64    `json:"days_31_60_cents"`
	Days61To90Cents int64    `json:"days_61_90_cents"`
	Days90PlusCents int64    `json:"days_90_plus_cents"`
	TotalCents    int64      `json:"total_cents"`
	Current       float64    `json:"current"`
	Days1To30     float64    `json:"days_1_30"`
	Days31To60    float64    `json:"days_31_60"`
	Days61To90    float64    `json:"days_61_90"`
	Days90Plus    float64    `json:"days_90_plus"`
	Total         float64    `json:"total"`
	Items         []AgedItem `json:"items"`
}

type SKUValuation struct {
	SKUID             string  `json:"sku_id"`
	SKUCode           string  `json:"sku_code"`
	SKUName           string  `json:"sku_name"`
	Category          string  `json:"category"`
	StockQuantity     int     `json:"stock_quantity"`
	ReservedQuantity  int     `json:"reserved_quantity"`
	UnitCost          float64 `json:"unit_cost"`
	SellingPrice      float64 `json:"selling_price"`
	CostValuation     float64 `json:"cost_valuation"`
	RetailValuation   float64 `json:"retail_valuation"`
	PotentialProfit   float64 `json:"potential_profit"`
}

type WarehouseValuation struct {
	WarehouseID   string  `json:"warehouse_id"`
	WarehouseName string  `json:"warehouse_name"`
	TotalItems    int     `json:"total_items"`
	TotalUnits    int     `json:"total_units"`
	CostValuation float64 `json:"cost_valuation"`
}

type InventoryValuationReport struct {
	OrgID                string               `json:"org_id"`
	AsOf                 string               `json:"as_of"`
	TotalSKUs            int                  `json:"total_skus"`
	TotalUnitsOnHand     int                  `json:"total_units_on_hand"`
	TotalCostValuation   float64              `json:"total_cost_valuation"`
	TotalRetailValuation float64              `json:"total_retail_valuation"`
	SKUs                 []SKUValuation       `json:"skus"`
	Warehouses           []WarehouseValuation `json:"warehouses"`
}

type DeadStockItem struct {
	SKUID         string  `json:"sku_id"`
	SKUCode       string  `json:"sku_code"`
	SKUName       string  `json:"sku_name"`
	Category      string  `json:"category"`
	StockQuantity int     `json:"stock_quantity"`
	UnitCost      float64 `json:"unit_cost"`
	TiedUpCapital float64 `json:"tied_up_capital"`
	DaysInactive  int     `json:"days_inactive"`
	LastActivity  string  `json:"last_activity"`
}

type InventoryTurnoverReport struct {
	OrgID               string          `json:"org_id"`
	AsOf                string          `json:"as_of"`
	TrailingPeriodDays  int             `json:"trailing_period_days"`
	COGSTrailing        float64         `json:"cogs_trailing"`
	AvgInventoryValue   float64         `json:"avg_inventory_value"`
	TurnoverRatio       float64         `json:"turnover_ratio"`
	DaysSalesOfInventory float64        `json:"days_sales_of_inventory"` // DSI
	DeadStockCount      int             `json:"dead_stock_count"`
	DeadStockValue      float64         `json:"dead_stock_value"`
	DeadStock           []DeadStockItem `json:"dead_stock"`
}

type StageMetric struct {
	Stage       string  `json:"stage"`
	DealCount   int     `json:"deal_count"`
	TotalValue  float64 `json:"total_value"`
	AvgValue    float64 `json:"avg_value"`
	Probability int     `json:"probability"`
}

type SalesVelocityReport struct {
	OrgID                  string        `json:"org_id"`
	TotalDeals             int           `json:"total_deals"`
	WonDeals               int           `json:"won_deals"`
	LostDeals              int           `json:"lost_deals"`
	OpenDeals              int           `json:"open_deals"`
	WinLossRatio           float64       `json:"win_loss_ratio"`
	TotalWonValue          float64       `json:"total_won_value"`
	AverageDealSize        float64       `json:"average_deal_size"`
	AvgSalesCycleDays      float64       `json:"avg_sales_cycle_days"`
	PipelineValue          float64       `json:"pipeline_value"`
	WeightedPipelineValue  float64       `json:"weighted_pipeline_value"`
	TotalLeads             int           `json:"total_leads"`
	ConvertedLeads         int           `json:"converted_leads"`
	LeadConversionRate     float64       `json:"lead_conversion_rate"`
	Stages                 []StageMetric `json:"stages"`
}

type ExecutiveKPIsReport struct {
	OrgID                     string  `json:"org_id"`
	GeneratedAt               string  `json:"generated_at"`
	MRR                       float64 `json:"mrr"`
	GrossMarginPct            float64 `json:"gross_margin_pct"`
	OperatingCash             float64 `json:"operating_cash"`
	MonthlyBurnRate           float64 `json:"monthly_burn_rate"`
	CashRunwayMonths          float64 `json:"cash_runway_months"`
	OpenPurchaseCommitments   float64 `json:"open_purchase_commitments"`
	PendingApprovalsCount     int     `json:"pending_approvals_count"`
	TotalRevenueYTD           float64 `json:"total_revenue_ytd"`
	NetProfitYTD              float64 `json:"net_profit_ytd"`
	TotalInventoryValuation   float64 `json:"total_inventory_valuation"`
	AccountsReceivableCurrent float64 `json:"accounts_receivable_current"`
	AccountsReceivableOverdue float64 `json:"accounts_receivable_overdue"`
}

// ----------------- PHASE 8: AUTONOMOUS MCP AGENT PLATFORM & WEBHOOKS -----------------

type WebhookSubscription struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	Secret     string    `json:"secret"`
	EventTypes string    `json:"event_types"` // JSON array or comma separated
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type WebhookDelivery struct {
	ID             string     `json:"id"`
	OrgID          string     `json:"org_id"`
	SubscriptionID string     `json:"subscription_id"`
	EventID        string     `json:"event_id"`
	EventType      string     `json:"event_type"`
	PayloadJSON    string     `json:"payload_json"`
	Status         string     `json:"status"` // pending, delivered, failed
	StatusCode     int        `json:"status_code"`
	ResponseBody   string     `json:"response_body"`
	RetryCount     int        `json:"retry_count"`
	MaxRetries     int        `json:"max_retries"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type AgentConfig struct {
	ID                   string     `json:"id"`
	OrgID                string     `json:"org_id"`
	Name                 string     `json:"name"`
	APIKeyHash           string     `json:"api_key_hash"`
	ToolQuotaPerMinute   int        `json:"tool_quota_per_minute"`
	DailyBudgetCents     int        `json:"daily_budget_cents"`
	CurrentDayUsageCents int        `json:"current_day_usage_cents"`
	ConcurrentLimit      int        `json:"concurrent_limit"`
	ActiveSlots          int        `json:"active_slots"`
	IsActive             bool       `json:"is_active"`
	LastUsedAt           *time.Time `json:"last_used_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}

type AgentExecutionRecord struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	AgentID       string    `json:"agent_id"`
	ToolName      string    `json:"tool_name"`
	Category      string    `json:"category"`
	ArgumentsJSON string    `json:"arguments_json"`
	ResultJSON    string    `json:"result_json"`
	IsError       bool      `json:"is_error"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	DurationMS    int64     `json:"duration_ms"`
	CostCents     int       `json:"cost_cents"`
	MutationsJSON string    `json:"mutations_json"`
	CreatedAt     time.Time `json:"created_at"`
}

type AgentEscalation struct {
	ID                string     `json:"id"`
	OrgID             string     `json:"org_id"`
	AgentID           string     `json:"agent_id"`
	ExecutionID       string     `json:"execution_id,omitempty"`
	Title             string     `json:"title"`
	Reason            string     `json:"reason"`
	Status            string     `json:"status"` // pending, approved, rejected, resolved
	InputContextJSON  string     `json:"input_context_json"`
	RecommendedAction string     `json:"recommended_action"`
	ReviewerID        string     `json:"reviewer_id,omitempty"`
	ReviewerNotes     string     `json:"reviewer_notes,omitempty"`
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

type AgentContextMemory struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Category    string    `json:"category"`
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SanitizeResult struct {
	IsSafe          bool     `json:"is_safe"`
	RiskScore       float64  `json:"risk_score"`
	FlaggedPatterns []string `json:"flagged_patterns"`
	SanitizedText   string   `json:"sanitized_text"`
}

// ----------------- PHASE 9: ENTERPRISE MULTI-TENANCY, AUTH & COMPLIANCE -----------------

type UserOrganization struct {
	UserID    string    `json:"user_id"`
	OrgID     string    `json:"org_id"`
	OrgName   string    `json:"org_name,omitempty"`
	OrgSlug   string    `json:"org_slug,omitempty"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type UserInvitation struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	TokenHash string    `json:"-"`
	InvitedBy string    `json:"invited_by"`
	Status    string    `json:"status"` // pending, accepted, expired
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type PasswordReset struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	Status    string    `json:"status"` // pending, used, expired
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type SSOConfig struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	Provider      string    `json:"provider"` // google, azure, okta, oidc, saml
	ClientID      string    `json:"client_id"`
	ClientSecret  string    `json:"-"`
	IssuerURL     string    `json:"issuer_url"`
	DomainMapping string    `json:"domain_mapping"`
	IsEnabled     bool      `json:"is_enabled"`
	CreatedAt     time.Time `json:"created_at"`
}

type CustomRole struct {
	ID              string    `json:"id"`
	OrgID           string    `json:"org_id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	PermissionsJSON string    `json:"permissions_json"`
	IsSystem        bool      `json:"is_system"`
	CreatedAt       time.Time `json:"created_at"`
}

type AuditChainBlock struct {
	ID             string    `json:"id"`
	OrgID          string    `json:"org_id"`
	SequenceNumber int64     `json:"sequence_number"`
	ActorType      string    `json:"actor_type"` // user, agent, system
	ActorID        string    `json:"actor_id"`
	Action         string    `json:"action"`
	ResourceType   string    `json:"resource_type"`
	ResourceID     string    `json:"resource_id"`
	PayloadHash    string    `json:"payload_hash"`
	PrevBlockHash  string    `json:"prev_block_hash"`
	BlockHash      string    `json:"block_hash"`
	Timestamp      time.Time `json:"timestamp"`
}

type PrivacyRequest struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"org_id"`
	UserID      string     `json:"user_id"`
	RequestType string     `json:"request_type"` // export_data, forget_me
	Status      string     `json:"status"`       // pending, completed, rejected
	ResultJSON  string     `json:"result_json"`
	RequestedAt time.Time  `json:"requested_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type SOC2AccessReview struct {
	ID           string     `json:"id"`
	OrgID        string     `json:"org_id"`
	Quarter      string     `json:"quarter"`
	Status       string     `json:"status"` // pending, approved
	ReviewedBy   string     `json:"reviewed_by"`
	Notes        string     `json:"notes"`
	SnapshotJSON string     `json:"snapshot_json"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}






