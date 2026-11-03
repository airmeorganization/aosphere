package hr

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"aosphere/internal/db"
	"aosphere/internal/ledger"
	"github.com/google/uuid"
)

type ShiftConflict struct {
	ShiftID      string `json:"shift_id"`
	EmployeeID   string `json:"employee_id"`
	EmployeeName string `json:"employee_name"`
	ConflictType string `json:"conflict_type"` // double_booking, rest_period_violation, weekly_overtime
	Message      string `json:"message"`
	Severity     string `json:"severity"` // warning, error
}

type HRService struct {
	db     *sql.DB
	ledger *ledger.LedgerService
}

func NewHRService(database *sql.DB, ledgerService *ledger.LedgerService) *HRService {
	return &HRService{
		db:     database,
		ledger: ledgerService,
	}
}

// -------------------------------------------------------------
// 1. Employee Lifecycle & Profiles
// -------------------------------------------------------------

func (s *HRService) GetEmployees(ctx context.Context, orgID string) ([]db.Employee, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	query := `SELECT 
		id, org_id, name, role, department, hourly_rate, status, 
		COALESCE(email, ''), COALESCE(phone, ''), COALESCE(hired_at, ''), created_at,
		COALESCE(job_title, ''), COALESCE(reporting_manager_id, ''), COALESCE(employment_type, 'full_time'),
		COALESCE(salary_type, 'hourly'), COALESCE(monthly_salary, 0.0), COALESCE(tax_id, ''),
		COALESCE(bank_name, ''), COALESCE(bank_account_number, ''), COALESCE(bank_routing_number, ''),
		COALESCE(emergency_contact_name, ''), COALESCE(emergency_contact_phone, '')
		FROM employees WHERE org_id = ? ORDER BY name ASC`

	rows, err := s.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query employees: %w", err)
	}
	defer rows.Close()

	var emps []db.Employee
	for rows.Next() {
		var e db.Employee
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.Name, &e.Role, &e.Department, &e.HourlyRate, &e.Status,
			&e.Email, &e.Phone, &e.HiredAt, &e.CreatedAt,
			&e.JobTitle, &e.ReportingManagerID, &e.EmploymentType,
			&e.SalaryType, &e.MonthlySalary, &e.TaxID,
			&e.BankName, &e.BankAccountNumber, &e.BankRoutingNumber,
			&e.EmergencyContactName, &e.EmergencyContactPhone,
		); err != nil {
			return nil, fmt.Errorf("failed to scan employee: %w", err)
		}
		emps = append(emps, e)
	}
	return emps, nil
}

func (s *HRService) GetEmployee(ctx context.Context, orgID, employeeID string) (*db.Employee, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	query := `SELECT 
		id, org_id, name, role, department, hourly_rate, status, 
		COALESCE(email, ''), COALESCE(phone, ''), COALESCE(hired_at, ''), created_at,
		COALESCE(job_title, ''), COALESCE(reporting_manager_id, ''), COALESCE(employment_type, 'full_time'),
		COALESCE(salary_type, 'hourly'), COALESCE(monthly_salary, 0.0), COALESCE(tax_id, ''),
		COALESCE(bank_name, ''), COALESCE(bank_account_number, ''), COALESCE(bank_routing_number, ''),
		COALESCE(emergency_contact_name, ''), COALESCE(emergency_contact_phone, '')
		FROM employees WHERE org_id = ? AND id = ? LIMIT 1`

	var e db.Employee
	err := s.db.QueryRowContext(ctx, query, orgID, employeeID).Scan(
		&e.ID, &e.OrgID, &e.Name, &e.Role, &e.Department, &e.HourlyRate, &e.Status,
		&e.Email, &e.Phone, &e.HiredAt, &e.CreatedAt,
		&e.JobTitle, &e.ReportingManagerID, &e.EmploymentType,
		&e.SalaryType, &e.MonthlySalary, &e.TaxID,
		&e.BankName, &e.BankAccountNumber, &e.BankRoutingNumber,
		&e.EmergencyContactName, &e.EmergencyContactPhone,
	)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *HRService) CreateEmployee(ctx context.Context, orgID string, emp db.Employee) (*db.Employee, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if emp.ID == "" {
		emp.ID = "emp_" + uuid.New().String()[:8]
	}
	emp.OrgID = orgID
	if emp.Status == "" {
		emp.Status = "active"
	}
	if emp.EmploymentType == "" {
		emp.EmploymentType = "full_time"
	}
	if emp.SalaryType == "" {
		emp.SalaryType = "hourly"
	}
	now := time.Now()
	if emp.HiredAt == "" {
		emp.HiredAt = now.Format("2006-01-02")
	}
	emp.CreatedAt = now

	query := `INSERT INTO employees (
		id, org_id, name, role, department, hourly_rate, status, email, phone, hired_at,
		job_title, reporting_manager_id, employment_type, salary_type, monthly_salary,
		tax_id, bank_name, bank_account_number, bank_routing_number,
		emergency_contact_name, emergency_contact_phone, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		emp.ID, emp.OrgID, emp.Name, emp.Role, emp.Department, emp.HourlyRate, emp.Status,
		emp.Email, emp.Phone, emp.HiredAt, emp.JobTitle, emp.ReportingManagerID,
		emp.EmploymentType, emp.SalaryType, emp.MonthlySalary, emp.TaxID,
		emp.BankName, emp.BankAccountNumber, emp.BankRoutingNumber,
		emp.EmergencyContactName, emp.EmergencyContactPhone, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert employee: %w", err)
	}
	return &emp, nil
}

func (s *HRService) UpdateEmployee(ctx context.Context, orgID string, emp db.Employee) (*db.Employee, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	query := `UPDATE employees SET 
		name = ?, role = ?, department = ?, hourly_rate = ?, status = ?,
		email = ?, phone = ?, job_title = ?, reporting_manager_id = ?,
		employment_type = ?, salary_type = ?, monthly_salary = ?, tax_id = ?,
		bank_name = ?, bank_account_number = ?, bank_routing_number = ?,
		emergency_contact_name = ?, emergency_contact_phone = ?
		WHERE org_id = ? AND id = ?`

	res, err := s.db.ExecContext(ctx, query,
		emp.Name, emp.Role, emp.Department, emp.HourlyRate, emp.Status,
		emp.Email, emp.Phone, emp.JobTitle, emp.ReportingManagerID,
		emp.EmploymentType, emp.SalaryType, emp.MonthlySalary, emp.TaxID,
		emp.BankName, emp.BankAccountNumber, emp.BankRoutingNumber,
		emp.EmergencyContactName, emp.EmergencyContactPhone,
		orgID, emp.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update employee: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return nil, fmt.Errorf("employee %s not found in org %s", emp.ID, orgID)
	}
	return s.GetEmployee(ctx, orgID, emp.ID)
}

// -------------------------------------------------------------
// 2. Shift Roster & Conflict Detection
// -------------------------------------------------------------

func (s *HRService) DetectShiftConflicts(ctx context.Context, orgID string) ([]ShiftConflict, error) {
	if orgID == "" {
		orgID = "org_default"
	}

	query := `SELECT s.id, s.employee_id, COALESCE(e.name, 'Unknown') as emp_name,
		s.shift_date, s.start_time, s.end_time, s.station
		FROM shifts s
		LEFT JOIN employees e ON s.employee_id = e.id
		WHERE s.org_id = ? AND s.status != 'cancelled'
		ORDER BY s.employee_id, s.shift_date, s.start_time`

	rows, err := s.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch shifts for conflict detection: %w", err)
	}
	defer rows.Close()

	type rawShift struct {
		ID        string
		EmpID     string
		EmpName   string
		DateStr   string
		StartTime string
		EndTime   string
		Station   string
	}

	empShifts := make(map[string][]rawShift)
	for rows.Next() {
		var sh rawShift
		if err := rows.Scan(&sh.ID, &sh.EmpID, &sh.EmpName, &sh.DateStr, &sh.StartTime, &sh.EndTime, &sh.Station); err != nil {
			return nil, err
		}
		empShifts[sh.EmpID] = append(empShifts[sh.EmpID], sh)
	}

	var conflicts []ShiftConflict

	for _, shifts := range empShifts {
		if len(shifts) == 0 {
			continue
		}
		empName := shifts[0].EmpName
		empID := shifts[0].EmpID

		// Track weekly hours: map[ISOWeekYear]hours
		weeklyHours := make(map[string]float64)

		for i, curr := range shifts {
			// Parse start and end time
			currStart, err1 := time.Parse("2006-01-02 15:04", curr.DateStr+" "+curr.StartTime)
			currEnd, err2 := time.Parse("2006-01-02 15:04", curr.DateStr+" "+curr.EndTime)
			if err1 == nil && err2 == nil {
				if currEnd.Before(currStart) {
					currEnd = currEnd.Add(24 * time.Hour) // Cross-midnight shift
				}
				durHours := currEnd.Sub(currStart).Hours()
				year, week := currStart.ISOWeek()
				weekKey := fmt.Sprintf("%d-W%02d", year, week)
				weeklyHours[weekKey] += durHours
			}

			// Check pairwise conflicts with subsequent shifts
			for j := i + 1; j < len(shifts); j++ {
				next := shifts[j]
				if curr.DateStr == next.DateStr {
					// 1. Double Booking check on same date
					if curr.StartTime < next.EndTime && curr.EndTime > next.StartTime {
						conflicts = append(conflicts, ShiftConflict{
							ShiftID:      next.ID,
							EmployeeID:   empID,
							EmployeeName: empName,
							ConflictType: "double_booking",
							Message:      fmt.Sprintf("Double-booking: Shift on %s (%s-%s) overlaps with another shift (%s-%s)", curr.DateStr, next.StartTime, next.EndTime, curr.StartTime, curr.EndTime),
							Severity:     "error",
						})
					}
				}

				// 2. Rest Period Violation (< 8 hours) check
				if err1 == nil && err2 == nil {
					nextStart, nextErr := time.Parse("2006-01-02 15:04", next.DateStr+" "+next.StartTime)
					if nextErr == nil && nextStart.After(currEnd) {
						gapHours := nextStart.Sub(currEnd).Hours()
						if gapHours < 8.0 {
							conflicts = append(conflicts, ShiftConflict{
								ShiftID:      next.ID,
								EmployeeID:   empID,
								EmployeeName: empName,
								ConflictType: "rest_period_violation",
								Message:      fmt.Sprintf("Rest period violation: Only %.1f hours rest between shift ending %s %s and next shift starting %s %s (< 8h required)", gapHours, curr.DateStr, curr.EndTime, next.DateStr, next.StartTime),
								Severity:     "error",
							})
						}
					}
				}
			}
		}

		// 3. Weekly Overtime Check (> 40h)
		for weekKey, totalHours := range weeklyHours {
			if totalHours > 40.0 {
				conflicts = append(conflicts, ShiftConflict{
					ShiftID:      "",
					EmployeeID:   empID,
					EmployeeName: empName,
					ConflictType: "weekly_overtime",
					Message:      fmt.Sprintf("Weekly overtime alert: Scheduled for %.1f hours in %s (exceeds standard 40h workweek)", totalHours, weekKey),
					Severity:     "warning",
				})
			}
		}
	}

	return conflicts, nil
}

// -------------------------------------------------------------
// 3. Time & Attendance (Web Timeclock)
// -------------------------------------------------------------

func (s *HRService) RecordTimeclockEvent(ctx context.Context, orgID string, evt db.TimeclockEvent) (*db.TimeclockEvent, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if evt.ID == "" {
		evt.ID = "pnc_" + uuid.New().String()[:8]
	}
	evt.OrgID = orgID
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}
	evt.CreatedAt = evt.Timestamp

	// Valid event types: clock_in, clock_out, break_start, break_end
	switch evt.EventType {
	case "clock_in", "clock_out", "break_start", "break_end":
	default:
		return nil, fmt.Errorf("invalid event_type %q: must be clock_in, clock_out, break_start, or break_end", evt.EventType)
	}

	var shiftIDArg interface{}
	if evt.ShiftID != "" {
		shiftIDArg = evt.ShiftID
	}

	tsFormatted := evt.Timestamp.UTC().Format("2006-01-02 15:04:05")

	// Insert event
	query := `INSERT INTO timeclock_events (id, org_id, employee_id, shift_id, event_type, timestamp, ip_address, location_notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query,
		evt.ID, evt.OrgID, evt.EmployeeID, shiftIDArg, evt.EventType,
		tsFormatted, evt.IPAddress, evt.LocationNotes, tsFormatted,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to record punch event: %w", err)
	}

	// If event is clock_out, auto-calculate total hours worked today and sync to timesheet
	if evt.EventType == "clock_out" {
		_ = s.syncTimesheetFromPunches(ctx, orgID, evt.EmployeeID, evt.Timestamp)
	}

	return &evt, nil
}

func (s *HRService) syncTimesheetFromPunches(ctx context.Context, orgID, employeeID string, punchTime time.Time) error {
	todayStr := punchTime.Format("2006-01-02")

	// Get all punch events for this employee today
	rows, err := s.db.QueryContext(ctx, `SELECT event_type, timestamp FROM timeclock_events 
		WHERE org_id = ? AND employee_id = ? AND (date(timestamp) = ? OR substr(timestamp, 1, 10) = ?)
		ORDER BY timestamp ASC`, orgID, employeeID, todayStr, todayStr)
	if err != nil {
		return err
	}
	defer rows.Close()

	type pEvent struct {
		evtType string
		ts      time.Time
	}
	var events []pEvent
	for rows.Next() {
		var evtType string
		var rawTs string
		if err := rows.Scan(&evtType, &rawTs); err == nil {
			var parsedTime time.Time
			if t, err := time.Parse("2006-01-02 15:04:05", rawTs); err == nil {
				parsedTime = t
			} else if len(rawTs) >= 19 {
				parsedTime, _ = time.Parse("2006-01-02 15:04:05", rawTs[:19])
			}
			events = append(events, pEvent{evtType: evtType, ts: parsedTime})
		}
	}

	var totalWorkDuration time.Duration
	var inTime time.Time

	for _, e := range events {
		switch e.evtType {
		case "clock_in":
			inTime = e.ts
		case "break_start":
			if !inTime.IsZero() {
				totalWorkDuration += e.ts.Sub(inTime)
				inTime = time.Time{}
			}
		case "break_end":
			inTime = e.ts
		case "clock_out":
			if !inTime.IsZero() {
				totalWorkDuration += e.ts.Sub(inTime)
				inTime = time.Time{}
			}
		}
	}

	hoursWorked := math.Round(totalWorkDuration.Hours()*100) / 100
	regularHours := hoursWorked
	overtimeHours := 0.0
	if hoursWorked > 8.0 {
		regularHours = 8.0
		overtimeHours = math.Round((hoursWorked-8.0)*100) / 100
	}

	// Fetch employee hourly rate
	var hourlyRate float64
	_ = s.db.QueryRowContext(ctx, `SELECT hourly_rate FROM employees WHERE id = ?`, employeeID).Scan(&hourlyRate)

	// Update or insert into timesheets
	tsID := "ts_" + uuid.New().String()[:8]
	_, err = s.db.ExecContext(ctx, `INSERT INTO timesheets (id, org_id, employee_id, work_date, hours_worked, overtime_hours, status, hourly_rate, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'submitted', ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET hours_worked = excluded.hours_worked, overtime_hours = excluded.overtime_hours`,
		tsID, orgID, employeeID, todayStr, regularHours, overtimeHours, hourlyRate,
	)
	return err
}

func (s *HRService) GetTimeclockStatus(ctx context.Context, orgID, employeeID string) (map[string]interface{}, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	todayStr := time.Now().Format("2006-01-02")

	rows, err := s.db.QueryContext(ctx, `SELECT id, event_type, timestamp, location_notes FROM timeclock_events
		WHERE org_id = ? AND employee_id = ? AND (date(timestamp) = ? OR substr(timestamp, 1, 10) = ?)
		ORDER BY timestamp ASC`, orgID, employeeID, todayStr, todayStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeclock status: %w", err)
	}
	defer rows.Close()

	type punchRecord struct {
		ID        string `json:"id"`
		EventType string `json:"event_type"`
		Timestamp string `json:"timestamp"`
		Notes     string `json:"notes"`
	}

	events := make([]punchRecord, 0)
	currentState := "clocked_out"
	var lastEvent *punchRecord

	for rows.Next() {
		var pr punchRecord
		var rawTs string
		var rawNotes sql.NullString
		if err := rows.Scan(&pr.ID, &pr.EventType, &rawTs, &rawNotes); err == nil {
			pr.Notes = rawNotes.String
			if t, err := time.Parse("2006-01-02 15:04:05", rawTs); err == nil {
				pr.Timestamp = t.Format("15:04:05")
			} else if len(rawTs) >= 19 {
				pr.Timestamp = rawTs[11:19]
			} else {
				pr.Timestamp = rawTs
			}
			events = append(events, pr)
			curr := pr
			lastEvent = &curr

			switch pr.EventType {
			case "clock_in", "break_end":
				currentState = "clocked_in"
			case "break_start":
				currentState = "on_break"
			case "clock_out":
				currentState = "clocked_out"
			}
		}
	}

	return map[string]interface{}{
		"employee_id":    employeeID,
		"date":           todayStr,
		"current_state":  currentState,
		"current_status": currentState,
		"last_event":     lastEvent,
		"events_count":   len(events),
		"events":         events,
		"punches_today":  events,
	}, nil
}

// -------------------------------------------------------------
// 4. Salary Structures & Payroll Computation
// -------------------------------------------------------------

func (s *HRService) CalculatePayrollPreview(ctx context.Context, orgID, period string) (*db.PayrollRun, []db.Payslip, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if period == "" {
		period = time.Now().Format("2006-01")
	}

	emps, err := s.GetEmployees(ctx, orgID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load employees: %w", err)
	}

	var payslips []db.Payslip
	var totalGrossCents, totalTaxCents, totalDeductionsCents, totalNetCents int64

	for _, emp := range emps {
		if emp.Status != "active" {
			continue
		}

		var regHours, otHours, grossPay float64

		if emp.SalaryType == "monthly" {
			// Fixed monthly salary
			grossPay = emp.MonthlySalary
			if grossPay <= 0 && emp.HourlyRate > 0 {
				grossPay = emp.HourlyRate * 160.0
			}
			regHours = 160.0
			otHours = 0.0
		} else {
			// Hourly wage with 1.5x overtime multiplier
			// Query actual timesheets for this employee in this month
			var thWorked, thOT sql.NullFloat64
			err := s.db.QueryRowContext(ctx, `SELECT SUM(hours_worked), SUM(overtime_hours) FROM timesheets
				WHERE org_id = ? AND employee_id = ? AND strftime('%Y-%m', work_date) = ?`,
				orgID, emp.ID, period).Scan(&thWorked, &thOT)

			if err == nil && thWorked.Valid && thWorked.Float64 > 0 {
				regHours = thWorked.Float64
				otHours = thOT.Float64
			} else {
				// Default standard monthly allocation if no manual timesheets punched
				regHours = 160.0
				otHours = 0.0
			}

			hourlyRate := emp.HourlyRate
			if hourlyRate <= 0 {
				hourlyRate = 25.0
			}
			regGross := regHours * hourlyRate
			otGross := otHours * hourlyRate * 1.5
			grossPay = regGross + otGross
		}

		// Statutory Deductions:
		// 1. Income Tax Withholding: 12%
		taxWithholding := math.Round(grossPay*0.12*100) / 100
		// 2. Social Contributions: 5%
		socialContrib := math.Round(grossPay*0.05*100) / 100
		// 3. Benefits & Healthcare: 3%
		benefitsDeduct := math.Round(grossPay*0.03*100) / 100

		totalEmpDeductions := taxWithholding + socialContrib + benefitsDeduct
		netPay := math.Round((grossPay-totalEmpDeductions)*100) / 100

		// Cents conversions
		gCents := int64(math.Round(grossPay * 100))
		tCents := int64(math.Round(taxWithholding * 100))
		dCents := int64(math.Round((socialContrib + benefitsDeduct) * 100))
		nCents := gCents - (tCents + dCents)

		totalGrossCents += gCents
		totalTaxCents += tCents
		totalDeductionsCents += dCents
		totalNetCents += nCents

		ps := db.Payslip{
			ID:                  "ps_" + uuid.New().String()[:8],
			OrgID:               orgID,
			EmployeeID:          emp.ID,
			EmployeeName:        emp.Name,
			Period:              period,
			SalaryType:          emp.SalaryType,
			HourlyRate:          emp.HourlyRate,
			MonthlySalary:       emp.MonthlySalary,
			RegularHours:        regHours,
			OvertimeHours:       otHours,
			GrossPay:            grossPay,
			TaxWithholding:      taxWithholding,
			SocialContributions: socialContrib,
			BenefitsDeductions:  benefitsDeduct,
			NetPay:              netPay,
			Status:              "generated",
			CreatedAt:           time.Now(),
		}
		payslips = append(payslips, ps)
	}

	run := &db.PayrollRun{
		ID:                   "prun_" + uuid.New().String()[:8],
		OrgID:                orgID,
		Period:               period,
		TotalGrossCents:      totalGrossCents,
		TotalTaxCents:        totalTaxCents,
		TotalDeductionsCents: totalDeductionsCents,
		TotalNetCents:        totalNetCents,
		TotalGrossDollars:    float64(totalGrossCents) / 100.0,
		TotalTaxDollars:      float64(totalTaxCents) / 100.0,
		TotalNetDollars:      float64(totalNetCents) / 100.0,
		Status:               "draft",
		PaymentDate:          time.Now().Format("2006-01-02"),
		PayslipsCount:        len(payslips),
		CreatedAt:            time.Now(),
	}

	return run, payslips, nil
}

// -------------------------------------------------------------
// 5. Automated Payroll Execution & Double-Entry Ledger Posting
// -------------------------------------------------------------

func (s *HRService) ExecutePayrollRun(ctx context.Context, orgID, period, paymentDate, userID string) (*db.PayrollRun, []db.Payslip, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if period == "" {
		period = time.Now().Format("2006-01")
	}
	if paymentDate == "" {
		paymentDate = time.Now().Format("2006-01-02")
	}

	// 1. Check if run for this period already exists
	var existingID, existingStatus string
	err := s.db.QueryRowContext(ctx, `SELECT id, status FROM payroll_runs WHERE org_id = ? AND period = ?`, orgID, period).Scan(&existingID, &existingStatus)
	if err == nil && existingStatus == "paid" {
		return nil, nil, fmt.Errorf("payroll for period %s has already been paid and settled (Run ID: %s)", period, existingID)
	}

	// 2. Compute payroll calculations
	run, payslips, err := s.CalculatePayrollPreview(ctx, orgID, period)
	if err != nil {
		return nil, nil, err
	}
	if len(payslips) == 0 {
		return nil, nil, fmt.Errorf("no active employees found for payroll execution in org %s", orgID)
	}

	run.PaymentDate = paymentDate
	run.Status = "paid"

	// 3. Begin Transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 4. Double-Entry General Ledger Posting
	// Invariant: Debits == Credits
	// Debit Account 5200 (Salaries & Wages Expense) for TotalGrossCents
	// Credit Account 2150 (Payroll Withholding Taxes & Deductions) for TotalTaxCents + TotalDeductionsCents
	// Credit Account 1000 (Operating Cash / Bank) for TotalNetCents
	var journalEntryID string
	if run.TotalGrossCents > 0 && s.ledger != nil {
		taxAndDeductionsCents := run.TotalTaxCents + run.TotalDeductionsCents
		// Mathematical safety check: Debits == Credits
		if run.TotalGrossCents != (taxAndDeductionsCents + run.TotalNetCents) {
			// Align rounding delta to net pay
			delta := run.TotalGrossCents - (taxAndDeductionsCents + run.TotalNetCents)
			run.TotalNetCents += delta
			run.TotalNetDollars = float64(run.TotalNetCents) / 100.0
		}

		jeReq := ledger.JournalEntry{
			OrgID:      orgID,
			SourceType: "payroll",
			SourceID:   run.ID,
			Memo:       fmt.Sprintf("Automated Monthly Payroll Disbursement - Period %s (%d employees)", period, len(payslips)),
			CreatedBy:  userID,
			PostedAt:   time.Now(),
			Lines: []ledger.JournalLine{
				{
					AccountCode: "5200", // Salaries & Wages Expense
					EntryType:   "debit",
					AmountCents: run.TotalGrossCents,
					Description: fmt.Sprintf("Gross Payroll Expense - Period %s", period),
				},
				{
					AccountCode: "2150", // Payroll Withholding Taxes & Deductions
					EntryType:   "credit",
					AmountCents: taxAndDeductionsCents,
					Description: fmt.Sprintf("Statutory Tax & Benefit Withholdings - Period %s", period),
				},
				{
					AccountCode: "1000", // Operating Cash / Bank
					EntryType:   "credit",
					AmountCents: run.TotalNetCents,
					Description: fmt.Sprintf("Direct Deposit Net Wage Disbursement - Period %s", period),
				},
			},
		}

		postedJE, err := s.ledger.PostJournalEntry(ctx, tx, jeReq)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to post double-entry payroll journal: %w", err)
		}
		journalEntryID = postedJE.ID
		run.JournalEntryID = journalEntryID
	}

	var jeIDArg interface{}
	if run.JournalEntryID != "" {
		jeIDArg = run.JournalEntryID
	}

	// 5. Insert or replace payroll_runs
	_, err = tx.ExecContext(ctx, `INSERT INTO payroll_runs (
		id, org_id, period, total_gross_cents, total_tax_cents, total_deductions_cents,
		total_net_cents, status, payment_date, journal_entry_id, payslips_count, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(org_id, period) DO UPDATE SET 
		total_gross_cents = excluded.total_gross_cents,
		total_tax_cents = excluded.total_tax_cents,
		total_deductions_cents = excluded.total_deductions_cents,
		total_net_cents = excluded.total_net_cents,
		status = excluded.status,
		payment_date = excluded.payment_date,
		journal_entry_id = excluded.journal_entry_id,
		payslips_count = excluded.payslips_count`,
		run.ID, run.OrgID, run.Period, run.TotalGrossCents, run.TotalTaxCents, run.TotalDeductionsCents,
		run.TotalNetCents, run.Status, run.PaymentDate, jeIDArg, run.PayslipsCount,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to save payroll run: %w", err)
	}

	// 6. Insert payslips
	for i := range payslips {
		payslips[i].PayrollRunID = run.ID
		payslips[i].Status = "paid"
		payslips[i].PaymentDate = paymentDate

		_, err = tx.ExecContext(ctx, `INSERT INTO payslips (
			id, org_id, payroll_run_id, employee_id, period, salary_type,
			hourly_rate, monthly_salary, regular_hours, overtime_hours,
			gross_pay, tax_withholding, social_contributions, benefits_deductions,
			net_pay, status, payment_date, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(org_id, payroll_run_id, employee_id) DO UPDATE SET
			gross_pay = excluded.gross_pay,
			tax_withholding = excluded.tax_withholding,
			social_contributions = excluded.social_contributions,
			benefits_deductions = excluded.benefits_deductions,
			net_pay = excluded.net_pay,
			status = excluded.status,
			payment_date = excluded.payment_date`,
			payslips[i].ID, payslips[i].OrgID, payslips[i].PayrollRunID, payslips[i].EmployeeID,
			payslips[i].Period, payslips[i].SalaryType, payslips[i].HourlyRate, payslips[i].MonthlySalary,
			payslips[i].RegularHours, payslips[i].OvertimeHours, payslips[i].GrossPay,
			payslips[i].TaxWithholding, payslips[i].SocialContributions, payslips[i].BenefitsDeductions,
			payslips[i].NetPay, payslips[i].Status, payslips[i].PaymentDate,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to save payslip for %s: %w", payslips[i].EmployeeID, err)
		}
	}

	// Commit Transaction
	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("failed to commit payroll run: %w", err)
	}

	return run, payslips, nil
}

func (s *HRService) GetPayrollRuns(ctx context.Context, orgID string) ([]db.PayrollRun, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT 
		id, org_id, period, total_gross_cents, total_tax_cents, total_deductions_cents,
		total_net_cents, status, payment_date, COALESCE(journal_entry_id, ''), payslips_count, created_at
		FROM payroll_runs WHERE org_id = ? ORDER BY period DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query payroll runs: %w", err)
	}
	defer rows.Close()

	var runs []db.PayrollRun
	for rows.Next() {
		var r db.PayrollRun
		if err := rows.Scan(
			&r.ID, &r.OrgID, &r.Period, &r.TotalGrossCents, &r.TotalTaxCents, &r.TotalDeductionsCents,
			&r.TotalNetCents, &r.Status, &r.PaymentDate, &r.JournalEntryID, &r.PayslipsCount, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		r.TotalGrossDollars = float64(r.TotalGrossCents) / 100.0
		r.TotalTaxDollars = float64(r.TotalTaxCents) / 100.0
		r.TotalNetDollars = float64(r.TotalNetCents) / 100.0
		runs = append(runs, r)
	}
	return runs, nil
}

func (s *HRService) GetPayslips(ctx context.Context, orgID, runID, employeeID string) ([]db.Payslip, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	query := `SELECT p.id, p.org_id, p.payroll_run_id, p.employee_id, COALESCE(e.name, 'Employee') as emp_name,
		p.period, p.salary_type, p.hourly_rate, p.monthly_salary, p.regular_hours, p.overtime_hours,
		p.gross_pay, p.tax_withholding, p.social_contributions, p.benefits_deductions,
		p.net_pay, p.status, p.payment_date, p.created_at
		FROM payslips p
		LEFT JOIN employees e ON p.employee_id = e.id
		WHERE p.org_id = ?`

	var args []interface{}
	args = append(args, orgID)

	if runID != "" {
		query += " AND p.payroll_run_id = ?"
		args = append(args, runID)
	}
	if employeeID != "" {
		query += " AND p.employee_id = ?"
		args = append(args, employeeID)
	}
	query += " ORDER BY p.created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query payslips: %w", err)
	}
	defer rows.Close()

	var list []db.Payslip
	for rows.Next() {
		var p db.Payslip
		if err := rows.Scan(
			&p.ID, &p.OrgID, &p.PayrollRunID, &p.EmployeeID, &p.EmployeeName,
			&p.Period, &p.SalaryType, &p.HourlyRate, &p.MonthlySalary, &p.RegularHours, &p.OvertimeHours,
			&p.GrossPay, &p.TaxWithholding, &p.SocialContributions, &p.BenefitsDeductions,
			&p.NetPay, &p.Status, &p.PaymentDate, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, nil
}

func (s *HRService) GetPayslip(ctx context.Context, orgID, payslipID string) (*db.Payslip, *db.Employee, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	query := `SELECT p.id, p.org_id, p.payroll_run_id, p.employee_id, COALESCE(e.name, 'Employee') as emp_name,
		p.period, p.salary_type, p.hourly_rate, p.monthly_salary, p.regular_hours, p.overtime_hours,
		p.gross_pay, p.tax_withholding, p.social_contributions, p.benefits_deductions,
		p.net_pay, p.status, p.payment_date, p.created_at
		FROM payslips p
		LEFT JOIN employees e ON p.employee_id = e.id
		WHERE p.org_id = ? AND p.id = ? LIMIT 1`

	var p db.Payslip
	err := s.db.QueryRowContext(ctx, query, orgID, payslipID).Scan(
		&p.ID, &p.OrgID, &p.PayrollRunID, &p.EmployeeID, &p.EmployeeName,
		&p.Period, &p.SalaryType, &p.HourlyRate, &p.MonthlySalary, &p.RegularHours, &p.OvertimeHours,
		&p.GrossPay, &p.TaxWithholding, &p.SocialContributions, &p.BenefitsDeductions,
		&p.NetPay, &p.Status, &p.PaymentDate, &p.CreatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("payslip not found: %w", err)
	}

	emp, _ := s.GetEmployee(ctx, orgID, p.EmployeeID)
	return &p, emp, nil
}

// -------------------------------------------------------------
// 6. Leave & Paid Time Off (PTO) Management
// -------------------------------------------------------------

func (s *HRService) GetLeavePolicies(ctx context.Context, orgID string) ([]db.LeavePolicy, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, org_id, name, code, annual_days, created_at 
		FROM leave_policies WHERE org_id = ? ORDER BY annual_days DESC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query leave policies: %w", err)
	}
	defer rows.Close()

	var policies []db.LeavePolicy
	for rows.Next() {
		var p db.LeavePolicy
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.Code, &p.AnnualDays, &p.CreatedAt); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, nil
}

func (s *HRService) GetLeaveRequests(ctx context.Context, orgID, employeeID, status string) ([]db.LeaveRequest, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	query := `SELECT lr.id, lr.org_id, lr.employee_id, COALESCE(e.name, 'Employee') as emp_name,
		lr.leave_type, lr.start_date, lr.end_date, lr.days_count, lr.reason,
		lr.status, lr.reviewed_by, lr.review_notes, lr.created_at
		FROM leave_requests lr
		LEFT JOIN employees e ON lr.employee_id = e.id
		WHERE lr.org_id = ?`

	var args []interface{}
	args = append(args, orgID)
	if employeeID != "" {
		query += " AND lr.employee_id = ?"
		args = append(args, employeeID)
	}
	if status != "" {
		query += " AND lr.status = ?"
		args = append(args, status)
	}
	query += " ORDER BY lr.created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query leave requests: %w", err)
	}
	defer rows.Close()

	var reqs []db.LeaveRequest
	for rows.Next() {
		var r db.LeaveRequest
		if err := rows.Scan(
			&r.ID, &r.OrgID, &r.EmployeeID, &r.EmployeeName,
			&r.LeaveType, &r.StartDate, &r.EndDate, &r.DaysCount, &r.Reason,
			&r.Status, &r.ReviewedBy, &r.ReviewNotes, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		reqs = append(reqs, r)
	}
	return reqs, nil
}

func (s *HRService) SubmitLeaveRequest(ctx context.Context, orgID string, req db.LeaveRequest) (*db.LeaveRequest, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if req.ID == "" {
		req.ID = "lvr_" + uuid.New().String()[:8]
	}
	req.OrgID = orgID
	if req.Status == "" {
		req.Status = "pending"
	}
	if req.DaysCount <= 0 {
		// Calculate calendar days between start and end
		st, err1 := time.Parse("2006-01-02", req.StartDate)
		et, err2 := time.Parse("2006-01-02", req.EndDate)
		if err1 == nil && err2 == nil && !et.Before(st) {
			req.DaysCount = int(et.Sub(st).Hours()/24) + 1
		} else {
			req.DaysCount = 1
		}
	}
	req.CreatedAt = time.Now()

	query := `INSERT INTO leave_requests (
		id, org_id, employee_id, leave_type, start_date, end_date, days_count, reason, status, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		req.ID, req.OrgID, req.EmployeeID, req.LeaveType,
		req.StartDate, req.EndDate, req.DaysCount, req.Reason, req.Status, req.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit leave request: %w", err)
	}
	return &req, nil
}

func (s *HRService) ReviewLeaveRequest(ctx context.Context, orgID, requestID, status, reviewer, notes string) error {
	if orgID == "" {
		orgID = "org_default"
	}
	if status != "approved" && status != "rejected" {
		return fmt.Errorf("invalid review status %q: must be 'approved' or 'rejected'", status)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE leave_requests SET status = ?, reviewed_by = ?, review_notes = ? 
		WHERE org_id = ? AND id = ?`, status, reviewer, notes, orgID, requestID)
	if err != nil {
		return fmt.Errorf("failed to review leave request: %w", err)
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("leave request %s not found in org %s", requestID, orgID)
	}
	return nil
}

// -------------------------------------------------------------
// 7. HR Document Vault & Compliance
// -------------------------------------------------------------

func (s *HRService) GetHRDocuments(ctx context.Context, orgID, employeeID, docType string) ([]db.HRDocument, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	query := `SELECT d.id, d.org_id, d.employee_id, COALESCE(e.name, 'Employee') as emp_name,
		d.title, d.document_type, d.file_name, d.file_content, d.file_size, d.uploaded_by, d.created_at
		FROM hr_documents d
		LEFT JOIN employees e ON d.employee_id = e.id
		WHERE d.org_id = ?`

	var args []interface{}
	args = append(args, orgID)
	if employeeID != "" {
		query += " AND d.employee_id = ?"
		args = append(args, employeeID)
	}
	if docType != "" {
		query += " AND d.document_type = ?"
		args = append(args, docType)
	}
	query += " ORDER BY d.created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query HR documents: %w", err)
	}
	defer rows.Close()

	var docs []db.HRDocument
	for rows.Next() {
		var d db.HRDocument
		if err := rows.Scan(
			&d.ID, &d.OrgID, &d.EmployeeID, &d.EmployeeName,
			&d.Title, &d.DocumentType, &d.FileName, &d.FileContent, &d.FileSize, &d.UploadedBy, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, nil
}

func (s *HRService) UploadHRDocument(ctx context.Context, orgID string, doc db.HRDocument) (*db.HRDocument, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	if doc.ID == "" {
		doc.ID = "doc_" + uuid.New().String()[:8]
	}
	doc.OrgID = orgID
	if doc.UploadedBy == "" {
		doc.UploadedBy = "HR Operations"
	}
	if doc.FileSize == 0 && doc.FileContent != "" {
		doc.FileSize = int64(len(doc.FileContent))
	}
	doc.CreatedAt = time.Now()

	query := `INSERT INTO hr_documents (
		id, org_id, employee_id, title, document_type, file_name, file_content, file_size, uploaded_by, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		doc.ID, doc.OrgID, doc.EmployeeID, doc.Title, doc.DocumentType,
		doc.FileName, doc.FileContent, doc.FileSize, doc.UploadedBy, doc.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upload HR document: %w", err)
	}
	return &doc, nil
}
