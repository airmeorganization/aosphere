package service

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidStateTransition     = errors.New("invalid state transition")
	ErrApprovalAlreadyDecided     = errors.New("approval has already been decided or is not pending")
	ErrStaleVersion               = errors.New("optimistic lock failure: entity has been modified concurrently (version mismatch)")
	ErrDuplicateReorderSuppressed = errors.New("duplicate reorder suppressed: open order or approval already exists for this SKU")
	ErrConcurrentRequest          = errors.New("conflict: a request with this idempotency key is already in progress")
	ErrInsufficientStock          = errors.New("insufficient stock: resulting quantity would be negative")
	ErrInvalidEntity              = errors.New("invalid entity reference")
	ErrUnauthorizedAction         = errors.New("unauthorized action for this caller")
)

// Purchase Order Lifecycle States
const (
	POStatusDraft             = "draft"
	POStatusPendingApproval   = "pending_approval"
	POStatusOrdered           = "ordered"
	POStatusPartiallyReceived = "partially_received"
	POStatusReceived          = "received"
	POStatusCancelled         = "cancelled"
)

var validPOTransitions = map[string]map[string]bool{
	POStatusDraft: {
		POStatusPendingApproval: true,
		POStatusOrdered:         true,
		POStatusCancelled:       true,
	},
	POStatusPendingApproval: {
		POStatusOrdered:   true, // Upon human approval
		POStatusCancelled: true, // Upon human rejection or cancel
	},
	POStatusOrdered: {
		POStatusPartiallyReceived: true, // Partial delivery received
		POStatusReceived:          true, // Goods fully arrived
		POStatusCancelled:         true, // Supplier cancel
	},
	POStatusPartiallyReceived: {
		POStatusPartiallyReceived: true, // Another partial shipment
		POStatusReceived:          true, // Final remaining goods arrived
		POStatusCancelled:         true, // Cancel remainder
	},
	POStatusReceived:  {}, // Terminal state
	POStatusCancelled: {}, // Terminal state
}

// CanTransitionPO returns true if moving from state 'from' to 'to' is allowed
func CanTransitionPO(from, to string) bool {
	if from == to {
		return true
	}
	nextStates, ok := validPOTransitions[from]
	if !ok {
		return false
	}
	return nextStates[to]
}

// Invoice Lifecycle States
const (
	InvoiceStatusDraft     = "draft"
	InvoiceStatusSent      = "sent"
	InvoiceStatusPaid      = "paid"
	InvoiceStatusOverdue   = "overdue"
	InvoiceStatusCancelled = "cancelled"
)

var validInvoiceTransitions = map[string]map[string]bool{
	InvoiceStatusDraft: {
		InvoiceStatusSent:      true,
		InvoiceStatusCancelled: true,
	},
	InvoiceStatusSent: {
		InvoiceStatusPaid:      true,
		InvoiceStatusOverdue:   true,
		InvoiceStatusCancelled: true,
	},
	InvoiceStatusOverdue: {
		InvoiceStatusPaid:      true,
		InvoiceStatusCancelled: true,
	},
	InvoiceStatusPaid:      {}, // Terminal
	InvoiceStatusCancelled: {}, // Terminal
}

// CanTransitionInvoice checks validity of invoice lifecycle steps
func CanTransitionInvoice(from, to string) bool {
	if from == to {
		return true
	}
	nextStates, ok := validInvoiceTransitions[from]
	if !ok {
		return false
	}
	return nextStates[to]
}

// Approval Lifecycle States
const (
	ApprovalStatusPending  = "pending"
	ApprovalStatusApproved = "approved"
	ApprovalStatusRejected = "rejected"

	ExecutionStatusNone      = "none"
	ExecutionStatusExecuting = "executing"
	ExecutionStatusCompleted = "completed"
	ExecutionStatusFailed    = "failed"
)

// Deal Lifecycle Stages
var validDealStages = map[string]bool{
	"lead":        true,
	"qualified":   true,
	"proposal":    true,
	"negotiation": true,
	"won":         true,
	"lost":        true,
}

// IsValidDealStage checks if a deal stage name is supported
func IsValidDealStage(stage string) bool {
	return validDealStages[stage]
}

func ValidatePOTransition(from, to string) error {
	if !CanTransitionPO(from, to) {
		return fmt.Errorf("%w: cannot transition purchase order from %q to %q", ErrInvalidStateTransition, from, to)
	}
	return nil
}

func ValidateInvoiceTransition(from, to string) error {
	if !CanTransitionInvoice(from, to) {
		return fmt.Errorf("%w: cannot transition invoice from %q to %q", ErrInvalidStateTransition, from, to)
	}
	return nil
}
