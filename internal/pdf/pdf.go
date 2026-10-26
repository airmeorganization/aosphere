package pdf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aosphere/internal/db"
)

// Item represents a generic line item in an invoice, PO, or credit note
type Item struct {
	Description string  `json:"description"`
	SKUCode     string  `json:"sku_code"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Total       float64 `json:"total"`
}

// PDFWriter generates pure vector PDF 1.4 documents without external dependencies
type PDFWriter struct {
	buf     bytes.Buffer
	offsets []int
}

func NewPDFWriter() *PDFWriter {
	pw := &PDFWriter{offsets: make([]int, 0, 16)}
	pw.buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	return pw
}

func (pw *PDFWriter) recordObj(num int) {
	for len(pw.offsets) <= num {
		pw.offsets = append(pw.offsets, 0)
	}
	pw.offsets[num] = pw.buf.Len()
	pw.buf.WriteString(fmt.Sprintf("%d 0 obj\n", num))
}

func (pw *PDFWriter) closeObj() {
	pw.buf.WriteString("endobj\n")
}

func escapePDF(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	return s
}

// BuildSinglePagePDF packages drawing commands into a standards-compliant single-page PDF
func (pw *PDFWriter) BuildSinglePagePDF(streamContent string) []byte {
	// Obj 1: Catalog
	pw.recordObj(1)
	pw.buf.WriteString("<< /Type /Catalog /Pages 2 0 R >>\n")
	pw.closeObj()

	// Obj 2: Pages
	pw.recordObj(2)
	pw.buf.WriteString("<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n")
	pw.closeObj()

	// Obj 3: Page (US Letter: 612 x 792 pt)
	pw.recordObj(3)
	pw.buf.WriteString("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents 6 0 R >>\n")
	pw.closeObj()

	// Obj 4: Regular Font (Helvetica)
	pw.recordObj(4)
	pw.buf.WriteString("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\n")
	pw.closeObj()

	// Obj 5: Bold Font (Helvetica-Bold)
	pw.recordObj(5)
	pw.buf.WriteString("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>\n")
	pw.closeObj()

	// Obj 6: Contents Stream
	pw.recordObj(6)
	streamBytes := []byte(streamContent)
	pw.buf.WriteString(fmt.Sprintf("<< /Length %d >>\nstream\n", len(streamBytes)))
	pw.buf.Write(streamBytes)
	pw.buf.WriteString("\nendstream\n")
	pw.closeObj()

	// Cross-Reference Table
	xrefOffset := pw.buf.Len()
	pw.buf.WriteString("xref\n0 7\n")
	pw.buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= 6; i++ {
		pw.buf.WriteString(fmt.Sprintf("%010d 00000 n \n", pw.offsets[i]))
	}

	// Trailer
	pw.buf.WriteString("trailer\n<< /Size 7 /Root 1 0 R >>\n")
	pw.buf.WriteString("startxref\n")
	pw.buf.WriteString(fmt.Sprintf("%d\n%%%%EOF\n", xrefOffset))

	return pw.buf.Bytes()
}

// ----------------- INVOICE PDF GENERATOR -----------------

func GenerateInvoicePDF(inv db.Invoice, cust db.Customer, comp db.CompanyProfile, cur db.CurrencyInfo) ([]byte, error) {
	pw := NewPDFWriter()
	var s bytes.Buffer

	// Parse items
	var items []Item
	if inv.ItemsJSON != "" {
		_ = json.Unmarshal([]byte(inv.ItemsJSON), &items)
	}
	if len(items) == 0 {
		items = append(items, Item{
			Description: "Professional Services / Merchandise",
			Quantity:    1,
			UnitPrice:   inv.Subtotal,
			Total:       inv.Subtotal,
		})
	}

	curCode := "USD"
	if cur.Code != "" {
		curCode = cur.Code
	}

	// 1. Top Brand Banner
	s.WriteString("0.05 0.09 0.16 rg 0 740 612 52 re f\n")
	s.WriteString("BT /F2 14 Tf 1 1 1 rg 40 760 Td (AO SPHERE BOS) Tj ET\n")
	s.WriteString("BT /F1 9 Tf 0.7 0.78 0.9 rg 40 748 Td (Agent-Native Business Operating System | Dual-First Architecture) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F2 10 Tf 1 1 1 rg 460 756 Td (CURRENCY: %s) Tj ET\n", escapePDF(curCode)))

	// 2. Document Title & Metadata
	s.WriteString("BT /F2 24 Tf 0.08 0.12 0.22 rg 40 700 Td (INVOICE) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F2 12 Tf 0.1 0.3 0.7 rg 40 682 Td (#%s) Tj ET\n", escapePDF(inv.InvoiceNumber)))

	statusColor := "0.1 0.6 0.2" // green for paid
	if inv.Status != "paid" {
		statusColor = "0.8 0.4 0.0" // amber for open/sent
	}
	s.WriteString(fmt.Sprintf("BT /F2 10 Tf %s rg 480 702 Td (STATUS: %s) Tj ET\n", statusColor, strings.ToUpper(inv.Status)))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.4 0.45 0.5 rg 430 686 Td (Issue Date: %s) Tj ET\n", escapePDF(inv.IssueDate)))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.4 0.45 0.5 rg 430 672 Td (Due Date: %s) Tj ET\n", escapePDF(inv.DueDate)))

	// Divider
	s.WriteString("0.85 0.88 0.92 RG 1 w 40 660 532 0 re S\n")

	// 3. Parties Block
	// From (Issuer)
	compName := comp.CompanyName
	if compName == "" {
		compName = "AO Sphere Enterprise Inc."
	}
	s.WriteString("BT /F2 9 Tf 0.45 0.5 0.6 rg 40 642 Td (ISSUED BY:) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F2 11 Tf 0.1 0.15 0.25 rg 40 628 Td (%s) Tj ET\n", escapePDF(compName)))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.45 rg 40 614 Td (%s) Tj ET\n", escapePDF(comp.Address)))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.45 rg 40 600 Td (Tax ID / GSTIN: %s | %s) Tj ET\n", "GSTIN-AO99283-GLOBAL", escapePDF(comp.Phone)))

	// Billed To (Customer)
	custName := cust.Name
	if custName == "" {
		custName = inv.CustomerName
	}
	s.WriteString("BT /F2 9 Tf 0.45 0.5 0.6 rg 320 642 Td (BILLED TO:) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F2 11 Tf 0.1 0.15 0.25 rg 320 628 Td (%s) Tj ET\n", escapePDF(custName)))
	if cust.Company != "" {
		s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.45 rg 320 614 Td (%s) Tj ET\n", escapePDF(cust.Company)))
	}
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.45 rg 320 600 Td (Email: %s) Tj ET\n", escapePDF(cust.Email)))

	// 4. Line Items Table Header
	s.WriteString("0.1 0.15 0.25 rg 40 565 532 20 re f\n")
	s.WriteString("BT /F2 9 Tf 1 1 1 rg 48 571 Td (#) Tj ET\n")
	s.WriteString("BT /F2 9 Tf 1 1 1 rg 75 571 Td (ITEM / DESCRIPTION) Tj ET\n")
	s.WriteString("BT /F2 9 Tf 1 1 1 rg 340 571 Td (QTY) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F2 9 Tf 1 1 1 rg 410 571 Td (UNIT PRICE (%s)) Tj ET\n", escapePDF(curCode)))
	s.WriteString(fmt.Sprintf("BT /F2 9 Tf 1 1 1 rg 500 571 Td (TOTAL (%s)) Tj ET\n", escapePDF(curCode)))

	// Rows
	y := 545.0
	for idx, it := range items {
		if idx%2 == 1 {
			s.WriteString(fmt.Sprintf("0.96 0.97 0.99 rg 40 %.1f 532 18 re f\n", y-3))
		}
		desc := it.Description
		if len(desc) > 42 {
			desc = desc[:39] + "..."
		}
		s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.4 rg 48 %.1f Td (%d) Tj ET\n", y+2, idx+1))
		s.WriteString(fmt.Sprintf("BT /F2 9 Tf 0.1 0.15 0.2 rg 75 %.1f Td (%s) Tj ET\n", y+2, escapePDF(desc)))
		s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.4 rg 345 %.1f Td (%.0f) Tj ET\n", y+2, it.Quantity))
		s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.4 rg 415 %.1f Td (%.2f) Tj ET\n", y+2, it.UnitPrice))
		s.WriteString(fmt.Sprintf("BT /F2 9 Tf 0.1 0.15 0.2 rg 505 %.1f Td (%.2f) Tj ET\n", y+2, it.Total))

		s.WriteString(fmt.Sprintf("0.92 0.94 0.96 RG 0.5 w 40 %.1f 532 0 re S\n", y-4))
		y -= 20.0
		if y < 200 {
			break
		}
	}

	// 5. Totals Block (Bottom Right)
	yTotals := y - 10
	s.WriteString(fmt.Sprintf("0.96 0.97 0.99 rg 340 %.1f 232 75 re f\n", yTotals-55))
	s.WriteString(fmt.Sprintf("0.85 0.88 0.92 RG 1 w 340 %.1f 232 75 re S\n", yTotals-55))

	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.4 0.45 0.5 rg 355 %.1f Td (Subtotal:) Tj ET\n", yTotals+5))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.1 0.15 0.2 rg 490 %.1f Td (%s %.2f) Tj ET\n", yTotals+5, curCode, inv.Subtotal))

	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.4 0.45 0.5 rg 355 %.1f Td (Sales Tax / GST:) Tj ET\n", yTotals-12))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.1 0.15 0.2 rg 490 %.1f Td (%s %.2f) Tj ET\n", yTotals-12, curCode, inv.TaxAmount))

	s.WriteString(fmt.Sprintf("0.85 0.88 0.92 RG 0.5 w 350 %.1f 212 0 re S\n", yTotals-22))

	s.WriteString(fmt.Sprintf("BT /F2 11 Tf 0.08 0.12 0.22 rg 355 %.1f Td (TOTAL DUE:) Tj ET\n", yTotals-38))
	s.WriteString(fmt.Sprintf("BT /F2 13 Tf 0.08 0.35 0.8 rg 480 %.1f Td (%s %.2f) Tj ET\n", yTotals-38, curCode, inv.TotalAmount))

	// 6. Payment & Bank Information
	s.WriteString("BT /F2 9 Tf 0.3 0.35 0.45 rg 40 120 Td (PAYMENT INSTRUCTIONS & BANK DETAILS) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F1 8 Tf 0.4 0.45 0.5 rg 40 106 Td (Bank: Global Commercial Bank | IFSC/SWIFT: AOIN99201 | A/C: 994820102941) Tj ET\n"))
	s.WriteString(fmt.Sprintf("BT /F1 8 Tf 0.4 0.45 0.5 rg 40 94 Td (UPI / Wire Reference: %s | Online Payment: https://aosphere.local/pay/%s) Tj ET\n", escapePDF(inv.InvoiceNumber), escapePDF(inv.InvoiceNumber)))

	// 7. Footer
	s.WriteString("0.85 0.88 0.92 RG 1 w 40 60 532 0 re S\n")
	s.WriteString("BT /F1 8 Tf 0.5 0.55 0.6 rg 40 45 Td (Generated by AO Sphere BOS | Dual-Actor Cryptographic Audit Ledger Active) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F1 8 Tf 0.5 0.55 0.6 rg 460 45 Td (Timestamp: %s) Tj ET\n", time.Now().UTC().Format("2006-01-02 15:04 UTC")))

	return pw.BuildSinglePagePDF(s.String()), nil
}

// ----------------- PURCHASE ORDER PDF GENERATOR -----------------

func GeneratePurchaseOrderPDF(po db.PurchaseOrder, sup db.Supplier, comp db.CompanyProfile, cur db.CurrencyInfo) ([]byte, error) {
	pw := NewPDFWriter()
	var s bytes.Buffer

	var items []Item
	if po.ItemsJSON != "" {
		_ = json.Unmarshal([]byte(po.ItemsJSON), &items)
	}
	if len(items) == 0 {
		items = append(items, Item{
			Description: "Procurement Items / Raw Materials",
			Quantity:    1,
			UnitPrice:   po.TotalAmount,
			Total:       po.TotalAmount,
		})
	}

	curCode := "USD"
	if cur.Code != "" {
		curCode = cur.Code
	}

	// 1. Top Brand Banner
	s.WriteString("0.08 0.15 0.22 rg 0 740 612 52 re f\n")
	s.WriteString("BT /F2 14 Tf 1 1 1 rg 40 760 Td (AO SPHERE BOS — PROCUREMENT) Tj ET\n")
	s.WriteString("BT /F1 9 Tf 0.7 0.85 0.95 rg 40 748 Td (Supply Chain & Inventory Management Monolith) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F2 10 Tf 1 1 1 rg 460 756 Td (CURRENCY: %s) Tj ET\n", escapePDF(curCode)))

	// 2. Title & Metadata
	s.WriteString("BT /F2 24 Tf 0.08 0.12 0.22 rg 40 700 Td (PURCHASE ORDER) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F2 12 Tf 0.1 0.4 0.6 rg 40 682 Td (#%s) Tj ET\n", escapePDF(po.PONumber)))

	s.WriteString(fmt.Sprintf("BT /F2 10 Tf 0.1 0.5 0.3 rg 480 702 Td (STATUS: %s) Tj ET\n", strings.ToUpper(po.Status)))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.4 0.45 0.5 rg 430 686 Td (Date: %s) Tj ET\n", po.CreatedAt.Format("2006-01-02")))

	s.WriteString("0.85 0.88 0.92 RG 1 w 40 660 532 0 re S\n")

	// 3. Parties Block
	compName := comp.CompanyName
	if compName == "" {
		compName = "AO Sphere Enterprise Inc."
	}
	s.WriteString("BT /F2 9 Tf 0.45 0.5 0.6 rg 40 642 Td (BUYER / SHIP TO:) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F2 11 Tf 0.1 0.15 0.25 rg 40 628 Td (%s) Tj ET\n", escapePDF(compName)))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.45 rg 40 614 Td (%s) Tj ET\n", escapePDF(comp.Address)))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.45 rg 40 600 Td (Central Logistics Hub — Receiving Dock 1) Tj ET\n"))

	s.WriteString("BT /F2 9 Tf 0.45 0.5 0.6 rg 320 642 Td (VENDOR / SUPPLIER:) Tj ET\n")
	supName := sup.Name
	if supName == "" {
		supName = po.SupplierName
	}
	s.WriteString(fmt.Sprintf("BT /F2 11 Tf 0.1 0.15 0.25 rg 320 628 Td (%s) Tj ET\n", escapePDF(supName)))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.45 rg 320 614 Td (Email: %s) Tj ET\n", escapePDF(sup.ContactEmail)))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.45 rg 320 600 Td (Lead Time: %d business days) Tj ET\n", sup.LeadTimeDays))

	// 4. Line Items Table Header
	s.WriteString("0.08 0.15 0.25 rg 40 565 532 20 re f\n")
	s.WriteString("BT /F2 9 Tf 1 1 1 rg 48 571 Td (#) Tj ET\n")
	s.WriteString("BT /F2 9 Tf 1 1 1 rg 75 571 Td (ORDERED ITEM / SKU) Tj ET\n")
	s.WriteString("BT /F2 9 Tf 1 1 1 rg 340 571 Td (QTY) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F2 9 Tf 1 1 1 rg 410 571 Td (UNIT COST (%s)) Tj ET\n", escapePDF(curCode)))
	s.WriteString(fmt.Sprintf("BT /F2 9 Tf 1 1 1 rg 500 571 Td (TOTAL (%s)) Tj ET\n", escapePDF(curCode)))

	y := 545.0
	for idx, it := range items {
		if idx%2 == 1 {
			s.WriteString(fmt.Sprintf("0.96 0.97 0.99 rg 40 %.1f 532 18 re f\n", y-3))
		}
		desc := it.Description
		if desc == "" {
			desc = it.SKUCode
		}
		if len(desc) > 42 {
			desc = desc[:39] + "..."
		}
		s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.4 rg 48 %.1f Td (%d) Tj ET\n", y+2, idx+1))
		s.WriteString(fmt.Sprintf("BT /F2 9 Tf 0.1 0.15 0.2 rg 75 %.1f Td (%s) Tj ET\n", y+2, escapePDF(desc)))
		s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.4 rg 345 %.1f Td (%.0f) Tj ET\n", y+2, it.Quantity))
		s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.4 rg 415 %.1f Td (%.2f) Tj ET\n", y+2, it.UnitPrice))
		s.WriteString(fmt.Sprintf("BT /F2 9 Tf 0.1 0.15 0.2 rg 505 %.1f Td (%.2f) Tj ET\n", y+2, it.Total))

		s.WriteString(fmt.Sprintf("0.92 0.94 0.96 RG 0.5 w 40 %.1f 532 0 re S\n", y-4))
		y -= 20.0
		if y < 200 {
			break
		}
	}

	// 5. Total Block
	yTotals := y - 10
	s.WriteString(fmt.Sprintf("0.96 0.97 0.99 rg 340 %.1f 232 50 re f\n", yTotals-35))
	s.WriteString(fmt.Sprintf("0.85 0.88 0.92 RG 1 w 340 %.1f 232 50 re S\n", yTotals-35))

	s.WriteString(fmt.Sprintf("BT /F2 11 Tf 0.08 0.12 0.22 rg 355 %.1f Td (PO COMMITMENT:) Tj ET\n", yTotals-20))
	s.WriteString(fmt.Sprintf("BT /F2 13 Tf 0.08 0.45 0.3 rg 480 %.1f Td (%s %.2f) Tj ET\n", yTotals-20, curCode, po.TotalAmount))

	// Notes & Author
	if po.Notes != "" {
		s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.4 0.45 0.5 rg 40 140 Td (Notes: %s) Tj ET\n", escapePDF(po.Notes)))
	}
	s.WriteString(fmt.Sprintf("BT /F2 9 Tf 0.3 0.35 0.45 rg 40 120 Td (Authorized By: %s [%s]) Tj ET\n", escapePDF(po.ActorID), strings.ToUpper(po.CreatedByType)))

	// Footer
	s.WriteString("0.85 0.88 0.92 RG 1 w 40 60 532 0 re S\n")
	s.WriteString("BT /F1 8 Tf 0.5 0.55 0.6 rg 40 45 Td (AO Sphere BOS Procurement Engine | 3-Way Match Verification Active) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F1 8 Tf 0.5 0.55 0.6 rg 460 45 Td (Timestamp: %s) Tj ET\n", time.Now().UTC().Format("2006-01-02 15:04 UTC")))

	return pw.BuildSinglePagePDF(s.String()), nil
}

// ----------------- CREDIT NOTE PDF GENERATOR -----------------

func GenerateCreditNotePDF(cn db.CreditNote, cust db.Customer, comp db.CompanyProfile, cur db.CurrencyInfo) ([]byte, error) {
	pw := NewPDFWriter()
	var s bytes.Buffer

	curCode := "USD"
	if cn.Currency != "" {
		curCode = cn.Currency
	}

	// 1. Top Brand Banner
	s.WriteString("0.25 0.05 0.08 rg 0 740 612 52 re f\n")
	s.WriteString("BT /F2 14 Tf 1 1 1 rg 40 760 Td (AO SPHERE BOS — CREDIT MEMO) Tj ET\n")
	s.WriteString("BT /F1 9 Tf 0.95 0.8 0.8 rg 40 748 Td (General Ledger Reversal & Customer Credit Document) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F2 10 Tf 1 1 1 rg 460 756 Td (CURRENCY: %s) Tj ET\n", escapePDF(curCode)))

	// 2. Title & Metadata
	s.WriteString("BT /F2 24 Tf 0.7 0.1 0.15 rg 40 700 Td (CREDIT NOTE) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F2 12 Tf 0.4 0.45 0.5 rg 40 682 Td (#%s) Tj ET\n", escapePDF(cn.CreditNoteNumber)))
	if cn.InvoiceNumber != "" {
		s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.45 rg 40 668 Td (Ref. Invoice: %s) Tj ET\n", escapePDF(cn.InvoiceNumber)))
	}

	s.WriteString("BT /F2 10 Tf 0.8 0.2 0.2 rg 480 702 Td (STATUS: ISSUED) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.4 0.45 0.5 rg 430 686 Td (Issue Date: %s) Tj ET\n", cn.CreatedAt.Format("2006-01-02")))

	s.WriteString("0.85 0.88 0.92 RG 1 w 40 655 532 0 re S\n")

	// 3. Parties Block
	compName := comp.CompanyName
	if compName == "" {
		compName = "AO Sphere Enterprise Inc."
	}
	s.WriteString("BT /F2 9 Tf 0.45 0.5 0.6 rg 40 638 Td (CREDITED BY:) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F2 11 Tf 0.1 0.15 0.25 rg 40 624 Td (%s) Tj ET\n", escapePDF(compName)))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.45 rg 40 610 Td (%s) Tj ET\n", escapePDF(comp.Address)))

	s.WriteString("BT /F2 9 Tf 0.45 0.5 0.6 rg 320 638 Td (CREDITED TO:) Tj ET\n")
	custName := cust.Name
	if custName == "" {
		custName = cn.CustomerName
	}
	s.WriteString(fmt.Sprintf("BT /F2 11 Tf 0.1 0.15 0.25 rg 320 624 Td (%s) Tj ET\n", escapePDF(custName)))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.3 0.35 0.45 rg 320 610 Td (Email: %s) Tj ET\n", escapePDF(cust.Email)))

	// Reason Banner
	s.WriteString("0.98 0.95 0.95 rg 40 565 532 30 re f\n")
	s.WriteString("0.85 0.7 0.7 RG 1 w 40 565 532 30 re S\n")
	s.WriteString(fmt.Sprintf("BT /F2 9 Tf 0.7 0.1 0.15 rg 50 580 Td (CREDIT REASON: %s) Tj ET\n", strings.ToUpper(cn.Reason)))
	s.WriteString("BT /F1 8 Tf 0.4 0.45 0.5 rg 50 570 Td (Amount has been credited to client account and reversed from General Ledger Sales Revenue) Tj ET\n")

	// Total Box
	s.WriteString("0.96 0.97 0.99 rg 340 460 232 80 re f\n")
	s.WriteString("0.85 0.88 0.92 RG 1 w 340 460 232 80 re S\n")

	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.4 0.45 0.5 rg 355 520 Td (Credit Subtotal:) Tj ET\n"))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.1 0.15 0.2 rg 490 520 Td (%s %.2f) Tj ET\n", curCode, cn.Subtotal))

	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.4 0.45 0.5 rg 355 505 Td (Tax Adjustment:) Tj ET\n"))
	s.WriteString(fmt.Sprintf("BT /F1 9 Tf 0.1 0.15 0.2 rg 490 505 Td (%s %.2f) Tj ET\n", curCode, cn.TaxAmount))

	s.WriteString("0.85 0.88 0.92 RG 0.5 w 350 495 212 0 re S\n")
	s.WriteString(fmt.Sprintf("BT /F2 11 Tf 0.7 0.1 0.15 rg 355 475 Td (TOTAL CREDITED:) Tj ET\n"))
	s.WriteString(fmt.Sprintf("BT /F2 13 Tf 0.7 0.1 0.15 rg 475 475 Td (%s %.2f) Tj ET\n", curCode, cn.TotalAmount))

	// Footer
	s.WriteString("0.85 0.88 0.92 RG 1 w 40 60 532 0 re S\n")
	s.WriteString("BT /F1 8 Tf 0.5 0.55 0.6 rg 40 45 Td (AO Sphere BOS Accounting Engine | General Ledger Reversal Entry Posted) Tj ET\n")
	s.WriteString(fmt.Sprintf("BT /F1 8 Tf 0.5 0.55 0.6 rg 460 45 Td (Timestamp: %s) Tj ET\n", time.Now().UTC().Format("2006-01-02 15:04 UTC")))

	return pw.BuildSinglePagePDF(s.String()), nil
}
