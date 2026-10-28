package crm

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"aosphere/internal/auth"
	"aosphere/internal/finance"
	"aosphere/internal/ledger"
	"github.com/google/uuid"
)

var (
	ErrPipelineNotFound = errors.New("pipeline not found")
	ErrStageNotFound    = errors.New("pipeline stage not found")
	ErrCustomerNotFound = errors.New("customer not found")
	ErrLeadNotFound     = errors.New("inbound lead not found")
	ErrInvoiceNotFound  = errors.New("invoice not found")
	ErrPortalTokenInvalid = errors.New("portal token is invalid or expired")
)

type Pipeline struct {
	ID          string          `json:"id"`
	OrgID       string          `json:"org_id"`
	Name        string          `json:"name"`
	Code        string          `json:"code"`
	Description string          `json:"description"`
	IsDefault   bool            `json:"is_default"`
	Stages      []PipelineStage `json:"stages"`
	CreatedAt   time.Time       `json:"created_at"`
}

type PipelineStage struct {
	ID                  string    `json:"id"`
	OrgID               string    `json:"org_id"`
	PipelineID          string    `json:"pipeline_id"`
	Name                string    `json:"name"`
	Code                string    `json:"code"`
	Position            int       `json:"position"`
	ProbabilityPct      int       `json:"probability_pct"`
	DecayDaysThreshold  int       `json:"decay_days_threshold"`
	Color               string    `json:"color"`
	CreatedAt           time.Time `json:"created_at"`
}

type CustomerActivity struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"org_id"`
	CustomerID   string    `json:"customer_id"`
	DealID       string    `json:"deal_id,omitempty"`
	ActivityType string    `json:"activity_type"` // note, call, meeting, email
	AuthorType   string    `json:"author_type"`   // human, agent
	AuthorID     string    `json:"author_id"`
	AuthorName   string    `json:"author_name"`
	Summary      string    `json:"summary"`
	Content      string    `json:"content"`
	MetadataJSON string    `json:"metadata_json"`
	CreatedAt    time.Time `json:"created_at"`
}

type InboundLead struct {
	ID                  string    `json:"id"`
	OrgID               string    `json:"org_id"`
	Name                string    `json:"name"`
	Email               string    `json:"email"`
	Company             string    `json:"company"`
	Phone               string    `json:"phone"`
	Message             string    `json:"message"`
	Source              string    `json:"source"`
	Status              string    `json:"status"` // new, qualified, contacted, converted, rejected
	AssignedTo          string    `json:"assigned_to"`
	ConvertedCustomerID string    `json:"converted_customer_id,omitempty"`
	ConvertedDealID     string    `json:"converted_deal_id,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

type EmailOutboxEntry struct {
	ID                string    `json:"id"`
	OrgID             string    `json:"org_id"`
	RecipientEmail    string    `json:"recipient_email"`
	RecipientName     string    `json:"recipient_name"`
	Subject           string    `json:"subject"`
	BodyHTML          string    `json:"body_html"`
	BodyText          string    `json:"body_text"`
	TemplateName      string    `json:"template_name"`
	RelatedEntityType string    `json:"related_entity_type"`
	RelatedEntityID   string    `json:"related_entity_id"`
	Status            string    `json:"status"`
	ErrorMessage      string    `json:"error_message,omitempty"`
	SentAt            time.Time `json:"sent_at"`
	CreatedAt         time.Time `json:"created_at"`
}

type DealDecayAlert struct {
	DealID             string    `json:"deal_id"`
	Title              string    `json:"title"`
	CustomerName       string    `json:"customer_name"`
	CustomerCompany    string    `json:"customer_company"`
	Value              float64   `json:"value"`
	PipelineID         string    `json:"pipeline_id"`
	PipelineName       string    `json:"pipeline_name"`
	StageCode          string    `json:"stage_code"`
	StageName          string    `json:"stage_name"`
	ProbabilityPct     int       `json:"probability_pct"`
	DaysInStage        int       `json:"days_in_stage"`
	DecayThresholdDays int       `json:"decay_threshold_days"`
	DecaySeverity      string    `json:"decay_severity"` // warning, critical
	StageUpdatedAt     time.Time `json:"stage_updated_at"`
}

type Customer360 struct {
	Customer         map[string]interface{}   `json:"customer"`
	TotalLTV         float64                  `json:"total_ltv"`
	TotalPaid        float64                  `json:"total_paid"`
	OutstandingAR    float64                  `json:"outstanding_ar"`
	TotalCreditNotes float64                  `json:"total_credit_notes"`
	KPIs             map[string]interface{}   `json:"kpis"`
	Deals            []map[string]interface{} `json:"deals"`
	Quotes           []map[string]interface{} `json:"quotes"`
	Invoices         []map[string]interface{} `json:"invoices"`
	CreditNotes      []map[string]interface{} `json:"credit_notes"`
	Activities       []CustomerActivity       `json:"activities"`
}

type PortalInvoiceView struct {
	Token          string                   `json:"token"`
	InvoiceID      string                   `json:"invoice_id"`
	InvoiceNumber  string                   `json:"invoice_number"`
	IssueDate      string                   `json:"issue_date"`
	DueDate        string                   `json:"due_date"`
	Status         string                   `json:"status"`
	Subtotal       float64                  `json:"subtotal"`
	TaxAmount      float64                  `json:"tax_amount"`
	TotalAmount    float64                  `json:"total_amount"`
	Currency       string                   `json:"currency"`
	Items          []map[string]interface{} `json:"items"`
	CustomerName   string                   `json:"customer_name"`
	CustomerEmail  string                   `json:"customer_email"`
	CompanyName    string                   `json:"company_name"`
	CompanyAddress string                   `json:"company_address"`
	CompanyWebsite string                   `json:"company_website"`
	IsPaid         bool                     `json:"is_paid"`
}

// GenerateSecureToken creates a cryptographically random hex token
func GenerateSecureToken(lengthBytes int) string {
	b := make([]byte, lengthBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// CRMService provides operations for Pipelines, Customer 360, Inbound Leads, and Client Portal
type CRMService struct {
	db     *sql.DB
	ledger *ledger.LedgerService
}

func NewCRMService(db *sql.DB, l *ledger.LedgerService) *CRMService {
	if l == nil {
		l = ledger.NewLedgerService(db)
	}
	return &CRMService{
		db:     db,
		ledger: l,
	}
}

// ListPipelines retrieves all sales pipelines and nested stages for an organization
func (s *CRMService) ListPipelines(ctx context.Context, orgID string) ([]Pipeline, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	rows, err := s.db.QueryContext(ctx, `SELECT id, org_id, name, code, description, is_default, created_at
		FROM pipelines WHERE org_id = ? ORDER BY is_default DESC, name ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query pipelines: %w", err)
	}
	defer rows.Close()

	var pipelines []Pipeline
	for rows.Next() {
		var p Pipeline
		var createdAt string
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.Code, &p.Description, &p.IsDefault, &createdAt); err != nil {
			return nil, err
		}
		p.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)

		// Fetch stages for pipeline
		stageRows, err := s.db.QueryContext(ctx, `SELECT id, org_id, pipeline_id, name, code, position, probability_pct, decay_days_threshold, color, created_at
			FROM pipeline_stages WHERE pipeline_id = ? ORDER BY position ASC`, p.ID)
		if err == nil {
			var stages []PipelineStage
			for stageRows.Next() {
				var st PipelineStage
				var stCreatedAt string
				if err := stageRows.Scan(&st.ID, &st.OrgID, &st.PipelineID, &st.Name, &st.Code, &st.Position, &st.ProbabilityPct, &st.DecayDaysThreshold, &st.Color, &stCreatedAt); err == nil {
					st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", stCreatedAt)
					stages = append(stages, st)
				}
			}
			stageRows.Close()
			p.Stages = stages
		}
		pipelines = append(pipelines, p)
	}
	return pipelines, nil
}

// UpdateDealStage updates a deal's stage, records stage_updated_at, and logs audit trail
func (s *CRMService) UpdateDealStage(ctx context.Context, orgID, dealID, targetStage string, authCtx *auth.AuthContext) error {
	if orgID == "" {
		orgID = "org_default"
	}

	actorID := "admin"
	actorType := "human"
	if authCtx != nil {
		actorID = authCtx.UserID
		actorType = string(authCtx.Role)
	}

	// Update deal stage
	res, err := s.db.ExecContext(ctx, `UPDATE deals SET stage = ?, stage_updated_at = CURRENT_TIMESTAMP WHERE id = ? AND org_id = ?`,
		targetStage, dealID, orgID)
	if err != nil {
		return fmt.Errorf("failed to update deal stage: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("deal not found")
	}

	// Fetch deal customer
	var customerID, dealTitle string
	_ = s.db.QueryRowContext(ctx, `SELECT customer_id, title FROM deals WHERE id = ?`, dealID).Scan(&customerID, &dealTitle)

	// Log activity
	if customerID != "" {
		_ = s.LogActivity(ctx, orgID, customerID, CustomerActivity{
			DealID:       dealID,
			ActivityType: "stage_change",
			AuthorType:   actorType,
			AuthorID:     actorID,
			AuthorName:   "Sales Workflow",
			Summary:      fmt.Sprintf("Deal %q progressed to stage %q", dealTitle, targetStage),
			Content:      fmt.Sprintf("Deal %s stage updated to %s by %s (%s).", dealID, targetStage, actorID, actorType),
		})
	}

	return nil
}

// GetDealDecayAlerts calculates stagnant deals exceeding stage decay thresholds
func (s *CRMService) GetDealDecayAlerts(ctx context.Context, orgID string) ([]DealDecayAlert, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	query := `
	SELECT d.id, d.title, d.value, d.stage, d.probability,
	       COALESCE(d.pipeline_id, 'pipe_std_' || d.org_id) as pipe_id,
	       COALESCE(p.name, 'Standard Sales') as pipe_name,
	       COALESCE(c.name, 'Client') as cust_name,
	       COALESCE(c.company, '') as cust_company,
	       COALESCE(ps.name, d.stage) as stage_name,
	       COALESCE(ps.decay_days_threshold, 14) as decay_threshold,
	       COALESCE(d.stage_updated_at, d.created_at) as updated_at
	FROM deals d
	LEFT JOIN customers c ON d.customer_id = c.id
	LEFT JOIN pipelines p ON d.pipeline_id = p.id
	LEFT JOIN pipeline_stages ps ON ps.pipeline_id = d.pipeline_id AND ps.code = d.stage
	WHERE d.org_id = ? AND d.stage NOT IN ('won', 'lost')
	ORDER BY d.value DESC
	`

	rows, err := s.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query deal decay alerts: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	var alerts []DealDecayAlert

	for rows.Next() {
		var a DealDecayAlert
		var updatedAtStr string
		var threshold int
		if err := rows.Scan(&a.DealID, &a.Title, &a.Value, &a.StageCode, &a.ProbabilityPct,
			&a.PipelineID, &a.PipelineName, &a.CustomerName, &a.CustomerCompany,
			&a.StageName, &threshold, &updatedAtStr); err != nil {
			continue
		}

		a.DecayThresholdDays = threshold
		t, err := time.Parse("2006-01-02 15:04:05", updatedAtStr)
		if err != nil {
			t, err = time.Parse(time.RFC3339, updatedAtStr)
			if err != nil {
				t = now.AddDate(0, 0, -5)
			}
		}
		a.StageUpdatedAt = t

		daysInStage := int(now.Sub(t).Hours() / 24)
		if daysInStage < 0 {
			daysInStage = 0
		}
		a.DaysInStage = daysInStage

		// Stagnant if daysInStage exceeds threshold
		if a.DaysInStage >= a.DecayThresholdDays && a.DecayThresholdDays > 0 {
			if a.DaysInStage >= a.DecayThresholdDays*2 {
				a.DecaySeverity = "critical"
			} else {
				a.DecaySeverity = "warning"
			}
			alerts = append(alerts, a)
		}
	}

	return alerts, nil
}

// LogActivity saves a communication or note on a customer record
func (s *CRMService) LogActivity(ctx context.Context, orgID, customerID string, act CustomerActivity) error {
	if orgID == "" {
		orgID = "org_default"
	}
	if act.ID == "" {
		act.ID = fmt.Sprintf("act_%s", uuid.New().String()[:8])
	}
	if act.AuthorType == "" {
		act.AuthorType = "human"
	}
	if act.AuthorID == "" {
		act.AuthorID = "admin"
	}
	if act.AuthorName == "" {
		act.AuthorName = "Operations Team"
	}
	if act.MetadataJSON == "" {
		act.MetadataJSON = "{}"
	}

	query := `INSERT INTO customer_activities (id, org_id, customer_id, deal_id, activity_type, author_type, author_id, author_name, summary, content, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`

	_, err := s.db.ExecContext(ctx, query, act.ID, orgID, customerID, act.DealID, act.ActivityType,
		act.AuthorType, act.AuthorID, act.AuthorName, act.Summary, act.Content, act.MetadataJSON)
	return err
}

// GetCustomer360 aggregates complete omni-channel customer intelligence
func (s *CRMService) GetCustomer360(ctx context.Context, orgID, customerID string) (*Customer360, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	// 1. Fetch Customer Record
	custRow := s.db.QueryRowContext(ctx, `SELECT id, org_id, name, company, email, phone, status, lifetime_value, tags, created_at
		FROM customers WHERE id = ? AND org_id = ?`, customerID, orgID)
	
	cust := make(map[string]interface{})
	var id, org, name, company, email, phone, status, tags, createdAt string
	var ltv float64
	if err := custRow.Scan(&id, &org, &name, &company, &email, &phone, &status, &ltv, &tags, &createdAt); err != nil {
		return nil, ErrCustomerNotFound
	}
	cust["id"] = id
	cust["org_id"] = org
	cust["name"] = name
	cust["company"] = company
	cust["email"] = email
	cust["phone"] = phone
	cust["status"] = status
	cust["lifetime_value"] = ltv
	cust["tags"] = tags
	cust["created_at"] = createdAt

	// 2. Fetch Invoices and calculate Total Paid & Outstanding AR
	var totalInvoiced, totalPaid, outstandingAR float64
	invRows, err := s.db.QueryContext(ctx, `SELECT id, invoice_number, status, issue_date, due_date, subtotal, tax_amount, total_amount, created_at
		FROM invoices WHERE customer_id = ? AND org_id = ? ORDER BY created_at DESC`, customerID, orgID)
	var invoices []map[string]interface{}
	if err == nil {
		defer invRows.Close()
		for invRows.Next() {
			var iID, iNum, iStat, iIssue, iDue, iCreated string
			var sub, tax, tot float64
			if err := invRows.Scan(&iID, &iNum, &iStat, &iIssue, &iDue, &sub, &tax, &tot, &iCreated); err == nil {
				invoices = append(invoices, map[string]interface{}{
					"id":             iID,
					"invoice_number": iNum,
					"status":         iStat,
					"issue_date":     iIssue,
					"due_date":       iDue,
					"subtotal":       sub,
					"tax_amount":     tax,
					"total_amount":   tot,
					"created_at":     iCreated,
				})
				totalInvoiced += tot
				if iStat == "paid" {
					totalPaid += tot
				} else {
					outstandingAR += tot
				}
			}
		}
	}

	// 3. Fetch Credit Notes
	var totalCreditNotes float64
	var creditNotes []map[string]interface{}
	cnRows, err := s.db.QueryContext(ctx, `SELECT id, credit_note_number, invoice_id, total_amount, status, reason, created_at
		FROM credit_notes WHERE customer_id = ? AND org_id = ? ORDER BY created_at DESC`, customerID, orgID)
	if err == nil {
		defer cnRows.Close()
		for cnRows.Next() {
			var cID, cNum, cInv, cStat, cReason, cCreated string
			var cTot float64
			if err := cnRows.Scan(&cID, &cNum, &cInv, &cTot, &cStat, &cReason, &cCreated); err == nil {
				creditNotes = append(creditNotes, map[string]interface{}{
					"id":                 cID,
					"credit_note_number": cNum,
					"invoice_id":         cInv,
					"total_amount":       cTot,
					"status":             cStat,
					"reason":             cReason,
					"created_at":         cCreated,
				})
				totalCreditNotes += cTot
			}
		}
	}

	// 4. Fetch Linked Deals
	var deals []map[string]interface{}
	dealRows, err := s.db.QueryContext(ctx, `SELECT d.id, d.title, d.value, d.stage, d.probability, d.expected_close_date,
		COALESCE(p.name, 'Standard Sales') as pipeline_name
		FROM deals d LEFT JOIN pipelines p ON d.pipeline_id = p.id
		WHERE d.customer_id = ? AND d.org_id = ? ORDER BY d.created_at DESC`, customerID, orgID)
	if err == nil {
		defer dealRows.Close()
		for dealRows.Next() {
			var dID, dTitle, dStage, dClose, dPipe string
			var dVal float64
			var dProb int
			if err := dealRows.Scan(&dID, &dTitle, &dVal, &dStage, &dProb, &dClose, &dPipe); err == nil {
				deals = append(deals, map[string]interface{}{
					"id":                  dID,
					"title":               dTitle,
					"value":               dVal,
					"stage":               dStage,
					"probability":         dProb,
					"expected_close_date": dClose,
					"pipeline_name":       dPipe,
				})
			}
		}
	}

	// 5. Fetch Quotes
	var quotes []map[string]interface{}
	qteRows, err := s.db.QueryContext(ctx, `SELECT id, quote_number, deal_id, total_amount, status, created_at
		FROM quotes WHERE customer_id = ? AND org_id = ? ORDER BY created_at DESC`, customerID, orgID)
	if err == nil {
		defer qteRows.Close()
		for qteRows.Next() {
			var qID, qNum, qDeal, qStat, qCreated string
			var qTot float64
			if err := qteRows.Scan(&qID, &qNum, &qDeal, &qTot, &qStat, &qCreated); err == nil {
				quotes = append(quotes, map[string]interface{}{
					"id":           qID,
					"quote_number": qNum,
					"deal_id":      qDeal,
					"total_amount": qTot,
					"status":       qStat,
					"created_at":   qCreated,
				})
			}
		}
	}

	// 6. Fetch Activities Timeline
	var activities []CustomerActivity
	actRows, err := s.db.QueryContext(ctx, `SELECT id, org_id, customer_id, COALESCE(deal_id, ''), activity_type, author_type, author_id, author_name, summary, content, metadata_json, created_at
		FROM customer_activities WHERE customer_id = ? AND org_id = ? ORDER BY created_at DESC LIMIT 50`, customerID, orgID)
	if err == nil {
		defer actRows.Close()
		for actRows.Next() {
			var act CustomerActivity
			var createdAtStr string
			if err := actRows.Scan(&act.ID, &act.OrgID, &act.CustomerID, &act.DealID, &act.ActivityType, &act.AuthorType, &act.AuthorID, &act.AuthorName, &act.Summary, &act.Content, &act.MetadataJSON, &createdAtStr); err == nil {
				act.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
				activities = append(activities, act)
			}
		}
	}

	var openPipeline float64
	for _, d := range deals {
		if stage, ok := d["stage"].(string); ok && stage != "won" && stage != "lost" {
			if v, ok := d["value"].(float64); ok {
				openPipeline += v
			}
		}
	}
	kpis := map[string]interface{}{
		"lifetime_value":      ltv,
		"total_revenue":       totalPaid,
		"unpaid_balance":      outstandingAR,
		"open_pipeline_value": openPipeline,
	}

	return &Customer360{
		Customer:         cust,
		TotalLTV:         ltv,
		TotalPaid:        totalPaid,
		OutstandingAR:    outstandingAR,
		TotalCreditNotes: totalCreditNotes,
		KPIs:             kpis,
		Deals:            deals,
		Quotes:           quotes,
		Invoices:         invoices,
		CreditNotes:      creditNotes,
		Activities:       activities,
	}, nil
}

// IngestInboundLead records a lead from a website webhook or public form
func (s *CRMService) IngestInboundLead(ctx context.Context, orgID string, lead InboundLead) (*InboundLead, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if lead.ID == "" {
		lead.ID = fmt.Sprintf("lead_%s", uuid.New().String()[:8])
	}
	if lead.Status == "" {
		lead.Status = "new"
	}
	if lead.Source == "" {
		lead.Source = "website_form"
	}
	if lead.AssignedTo == "" {
		lead.AssignedTo = "AI Qualification Agent"
	}

	query := `INSERT INTO inbound_leads (id, org_id, name, email, company, phone, message, source, status, assigned_to, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`

	_, err := s.db.ExecContext(ctx, query, lead.ID, orgID, lead.Name, lead.Email, lead.Company, lead.Phone, lead.Message, lead.Source, lead.Status, lead.AssignedTo)
	if err != nil {
		return nil, fmt.Errorf("failed to save inbound lead: %w", err)
	}

	lead.CreatedAt = time.Now()
	return &lead, nil
}

// ConvertLead transforms an inbound lead into a Customer and a Deal
func (s *CRMService) ConvertLead(ctx context.Context, orgID, leadID, targetPipelineID string, dealValue float64) (string, string, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	var lead InboundLead
	err := s.db.QueryRowContext(ctx, `SELECT id, org_id, name, email, company, phone, message, status FROM inbound_leads WHERE id = ? AND org_id = ?`, leadID, orgID).
		Scan(&lead.ID, &lead.OrgID, &lead.Name, &lead.Email, &lead.Company, &lead.Phone, &lead.Message, &lead.Status)
	if err != nil {
		return "", "", ErrLeadNotFound
	}

	// 1. Create or Find Customer
	var custID string
	err = s.db.QueryRowContext(ctx, `SELECT id FROM customers WHERE email = ? AND org_id = ?`, lead.Email, orgID).Scan(&custID)
	if err != nil {
		custID = fmt.Sprintf("cust_%s", uuid.New().String()[:8])
		_, err = s.db.ExecContext(ctx, `INSERT INTO customers (id, org_id, name, company, email, phone, status, lifetime_value)
			VALUES (?, ?, ?, ?, ?, ?, 'active', 0.0)`, custID, orgID, lead.Name, lead.Company, lead.Email, lead.Phone)
		if err != nil {
			return "", "", fmt.Errorf("failed to create customer: %w", err)
		}
	}

	// 2. Create Deal
	if targetPipelineID == "" {
		targetPipelineID = "pipe_std_" + orgID
	}
	if dealValue <= 0 {
		dealValue = 12000.0
	}
	dealID := fmt.Sprintf("deal_%s", uuid.New().String()[:8])
	dealTitle := fmt.Sprintf("%s - Inbound Opportunity", lead.Company)
	if lead.Company == "" {
		dealTitle = fmt.Sprintf("%s Project", lead.Name)
	}

	_, err = s.db.ExecContext(ctx, `INSERT INTO deals (id, org_id, title, customer_id, value, stage, probability, pipeline_id, stage_updated_at, created_at)
		VALUES (?, ?, ?, ?, ?, 'lead', 20, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		dealID, orgID, dealTitle, custID, dealValue, targetPipelineID)
	if err != nil {
		return "", "", fmt.Errorf("failed to create deal from lead: %w", err)
	}

	// 3. Mark lead as converted
	_, _ = s.db.ExecContext(ctx, `UPDATE inbound_leads SET status = 'converted', converted_customer_id = ?, converted_deal_id = ? WHERE id = ?`,
		custID, dealID, leadID)

	// 4. Log Customer Activity
	_ = s.LogActivity(ctx, orgID, custID, CustomerActivity{
		DealID:       dealID,
		ActivityType: "lead_converted",
		AuthorType:   "human",
		AuthorID:     "admin",
		AuthorName:   "Sales Rep",
		Summary:      fmt.Sprintf("Converted Inbound Lead %s to Deal %s", lead.Name, dealTitle),
		Content:      fmt.Sprintf("Initial message: %s", lead.Message),
	})

	return custID, dealID, nil
}

// EnsureInvoicePortalToken generates or returns the secure portal token for an invoice
func (s *CRMService) EnsureInvoicePortalToken(ctx context.Context, orgID, invoiceID string) (string, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	var existingToken string
	err := s.db.QueryRowContext(ctx, `SELECT token FROM invoice_portal_tokens WHERE invoice_id = ? AND org_id = ?`, invoiceID, orgID).Scan(&existingToken)
	if err == nil && existingToken != "" {
		return existingToken, nil
	}

	token := "ptok_" + GenerateSecureToken(16)
	_, err = s.db.ExecContext(ctx, `INSERT INTO invoice_portal_tokens (token, org_id, invoice_id, expires_at, created_at)
		VALUES (?, ?, ?, datetime('now', '+90 days'), CURRENT_TIMESTAMP)`, token, orgID, invoiceID)
	if err != nil {
		return "", fmt.Errorf("failed to generate invoice portal token: %w", err)
	}
	return token, nil
}

// SendInvoiceEmail dispatches a professional transactional email with payment portal link
func (s *CRMService) SendInvoiceEmail(ctx context.Context, orgID, invoiceID, baseURL string) (*EmailOutboxEntry, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Fetch invoice and customer
	var invNumber, custName, custEmail, status string
	var total float64
	err := s.db.QueryRowContext(ctx, `
		SELECT i.invoice_number, i.total_amount, i.status, COALESCE(c.name, 'Valued Client'), COALESCE(c.email, 'client@example.com')
		FROM invoices i LEFT JOIN customers c ON i.customer_id = c.id
		WHERE i.id = ? AND i.org_id = ?`, invoiceID, orgID).Scan(&invNumber, &total, &status, &custName, &custEmail)
	if err != nil {
		return nil, ErrInvoiceNotFound
	}

	token, err := s.EnsureInvoicePortalToken(ctx, orgID, invoiceID)
	if err != nil {
		return nil, err
	}

	portalURL := fmt.Sprintf("%s/portal/invoice/%s", baseURL, token)
	subject := fmt.Sprintf("Invoice %s from AO Sphere Enterprise — Amount: %s", invNumber, finance.FormatUSD(finance.DollarsToCents(total)))

	bodyHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><style>body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;color:#1e293b;line-height:1.5;margin:0;padding:24px;background:#f8fafc;}
.card{background:#ffffff;border:1px solid #e2e8f0;border-radius:12px;padding:32px;max-width:580px;margin:0 auto;box-shadow:0 4px 6px -1px rgba(0,0,0,0.05);}
.btn{display:inline-block;background:#2563eb;color:#ffffff;padding:12px 28px;border-radius:8px;font-weight:600;text-decoration:none;margin-top:20px;}
.total{font-size:28px;font-weight:800;color:#0f172a;margin:16px 0;}
.footer{font-size:12px;color:#94a3b8;margin-top:32px;text-align:center;}
</style></head>
<body>
<div class="card">
  <h2>Invoice from AO Sphere Enterprise</h2>
  <p>Dear %s,</p>
  <p>Please find attached your invoice <strong>%s</strong> for recent products/services delivered.</p>
  <div class="total">%s</div>
  <p>You can review your detailed line items, download your compliant PDF invoice, or pay directly via our secure Client Self-Service Portal:</p>
  <p><a href="%s" class="btn">View & Pay Invoice Online</a></p>
  <p class="footer">AO Sphere Enterprise • Autonomous & Human-in-the-Loop Business Operating System<br>Link: %s</p>
</div>
</body></html>`, custName, invNumber, finance.FormatUSD(finance.DollarsToCents(total)), portalURL, portalURL)

	emailEntry := EmailOutboxEntry{
		ID:                fmt.Sprintf("eml_%s", uuid.New().String()[:8]),
		OrgID:             orgID,
		RecipientEmail:    custEmail,
		RecipientName:     custName,
		Subject:           subject,
		BodyHTML:          bodyHTML,
		BodyText:          fmt.Sprintf("Invoice %s for %s. View & pay online: %s", invNumber, finance.FormatUSD(finance.DollarsToCents(total)), portalURL),
		TemplateName:      "invoice_notification",
		RelatedEntityType: "invoice",
		RelatedEntityID:   invoiceID,
		Status:            "sent",
		SentAt:            time.Now(),
		CreatedAt:         time.Now(),
	}

	query := `INSERT INTO email_outbox (id, org_id, recipient_email, recipient_name, subject, body_html, body_text, template_name, related_entity_type, related_entity_id, status, sent_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
	_, err = s.db.ExecContext(ctx, query, emailEntry.ID, orgID, emailEntry.RecipientEmail, emailEntry.RecipientName,
		emailEntry.Subject, emailEntry.BodyHTML, emailEntry.BodyText, emailEntry.TemplateName, emailEntry.RelatedEntityType, emailEntry.RelatedEntityID, emailEntry.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to save email to outbox: %w", err)
	}

	return &emailEntry, nil
}

// GetPortalInvoice retrieves invoice details via public secure portal token
func (s *CRMService) GetPortalInvoice(ctx context.Context, token string) (*PortalInvoiceView, error) {
	var invoiceID, orgID string
	err := s.db.QueryRowContext(ctx, `SELECT invoice_id, org_id FROM invoice_portal_tokens WHERE token = ?`, token).Scan(&invoiceID, &orgID)
	if err != nil {
		return nil, ErrPortalTokenInvalid
	}

	// Update view count
	_, _ = s.db.ExecContext(ctx, `UPDATE invoice_portal_tokens SET view_count = view_count + 1, last_viewed_at = CURRENT_TIMESTAMP WHERE token = ?`, token)

	var invNumber, issueDate, dueDate, status, itemsJSON, custName, custEmail string
	var subtotal, taxAmount, totalAmount float64
	query := `
		SELECT i.invoice_number, i.issue_date, i.due_date, i.status, i.subtotal, i.tax_amount, i.total_amount, i.items_json,
		       COALESCE(c.name, 'Client'), COALESCE(c.email, '')
		FROM invoices i LEFT JOIN customers c ON i.customer_id = c.id
		WHERE i.id = ? AND i.org_id = ?`

	err = s.db.QueryRowContext(ctx, query, invoiceID, orgID).Scan(
		&invNumber, &issueDate, &dueDate, &status, &subtotal, &taxAmount, &totalAmount, &itemsJSON, &custName, &custEmail)
	if err != nil {
		return nil, ErrInvoiceNotFound
	}

	var items []map[string]interface{}
	_ = json.Unmarshal([]byte(itemsJSON), &items)

	return &PortalInvoiceView{
		Token:          token,
		InvoiceID:      invoiceID,
		InvoiceNumber:  invNumber,
		IssueDate:      issueDate,
		DueDate:        dueDate,
		Status:         status,
		Subtotal:       subtotal,
		TaxAmount:      taxAmount,
		TotalAmount:    totalAmount,
		Currency:       "USD",
		Items:          items,
		CustomerName:   custName,
		CustomerEmail:  custEmail,
		CompanyName:    "AO Sphere Enterprise",
		CompanyAddress: "100 Innovation Way, Suite 400, New York, NY",
		CompanyWebsite: "https://aosphere.local",
		IsPaid:         status == "paid",
	}, nil
}

// PayPortalInvoice records payment of an invoice from the client self-service portal
func (s *CRMService) PayPortalInvoice(ctx context.Context, token, paymentMethod string) error {
	var invoiceID, orgID string
	err := s.db.QueryRowContext(ctx, `SELECT invoice_id, org_id FROM invoice_portal_tokens WHERE token = ?`, token).Scan(&invoiceID, &orgID)
	if err != nil {
		return ErrPortalTokenInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var invNumber, status string
	var totalAmount float64
	err = tx.QueryRowContext(ctx, `SELECT invoice_number, status, total_amount FROM invoices WHERE id = ? AND org_id = ?`, invoiceID, orgID).
		Scan(&invNumber, &status, &totalAmount)
	if err != nil {
		return ErrInvoiceNotFound
	}

	if status == "paid" {
		return nil // already paid
	}

	// Update invoice status to 'paid'
	_, err = tx.ExecContext(ctx, `UPDATE invoices SET status = 'paid', version = version + 1 WHERE id = ?`, invoiceID)
	if err != nil {
		return fmt.Errorf("failed to mark invoice paid: %w", err)
	}

	// Post balancing double-entry General Ledger entry:
	// Debit: Operating Cash / Bank (1000)
	// Credit: Accounts Receivable (1100)
	totalCents := finance.DollarsToCents(totalAmount)
	lines := []ledger.JournalLine{
		{
			AccountCode: ledger.AccountCash,
			EntryType:   "debit",
			AmountCents: totalCents,
			Description: fmt.Sprintf("Client Portal Payment for %s via %s", invNumber, paymentMethod),
		},
		{
			AccountCode: ledger.AccountAccountsReceivable,
			EntryType:   "credit",
			AmountCents: totalCents,
			Description: fmt.Sprintf("AR Settlement for %s", invNumber),
		},
	}

	_, err = s.ledger.PostJournalEntry(ctx, tx, ledger.JournalEntry{
		OrgID:      orgID,
		SourceType: "portal_payment",
		SourceID:   invoiceID,
		Memo:       fmt.Sprintf("Self-Service Portal Payment for %s (%s)", invNumber, finance.FormatUSD(totalCents)),
		CreatedBy:  "client_portal",
		Lines:      lines,
	})
	if err != nil {
		return fmt.Errorf("failed to post portal payment journal entry: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit portal payment: %w", err)
	}

	return nil
}
