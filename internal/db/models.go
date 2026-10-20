package db

import "time"

type SKU struct {
	ID               string    `json:"id"`
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
	UpdatedAt        time.Time `json:"updated_at"`
}

type Supplier struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	ContactEmail string  `json:"contact_email"`
	Phone        string  `json:"phone"`
	LeadTimeDays int     `json:"lead_time_days"`
	Rating       float64 `json:"rating"`
}

type PurchaseOrder struct {
	ID            string    `json:"id"`
	PONumber      string    `json:"po_number"`
	SupplierID    string    `json:"supplier_id"`
	SupplierName  string    `json:"supplier_name,omitempty"`
	Status        string    `json:"status"` // draft, pending_approval, ordered, received, cancelled
	TotalAmount   float64   `json:"total_amount"`
	ItemsJSON     string    `json:"items_json"`
	CreatedByType string    `json:"created_by_type"` // human, agent
	ActorID       string    `json:"actor_id"`
	Notes         string    `json:"notes"`
	CreatedAt     time.Time `json:"created_at"`
}

type Customer struct {
	ID            string    `json:"id"`
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
	CreatedAt          time.Time `json:"created_at"`
}

type Expense struct {
	ID          string    `json:"id"`
	Category    string    `json:"category"`
	Amount      float64   `json:"amount"`
	Vendor      string    `json:"vendor"`
	Description string    `json:"description"`
	Date        string    `json:"date"`
	CreatedAt   time.Time `json:"created_at"`
}

type Employee struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Role       string    `json:"role"`
	Department string    `json:"department"`
	HourlyRate float64   `json:"hourly_rate"`
	Status     string    `json:"status"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	HiredAt    string    `json:"hired_at"`
	CreatedAt  time.Time `json:"created_at"`
}

type Shift struct {
	ID           string    `json:"id"`
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
	ActionType      string     `json:"action_type"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	PayloadJSON     string     `json:"payload_json"`
	Status          string     `json:"status"` // pending, approved, rejected
	RequestedByType string     `json:"requested_by_type"`
	RequestedByID   string     `json:"requested_by_id"`
	DecidedBy       *string    `json:"decided_by,omitempty"`
	DecidedAt       *time.Time `json:"decided_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type AuditLog struct {
	ID           string    `json:"id"`
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
