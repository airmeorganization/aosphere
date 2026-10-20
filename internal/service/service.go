package service

import (
	"aosphere/internal/store"
)

// WorkflowService is the central business logic layer shared identically across REST API and MCP tools
type WorkflowService struct {
	store  *store.Store
	policy *PolicyEngine
	idemp  *IdempotencyEngine
}

func NewWorkflowService(s *store.Store) *WorkflowService {
	return &WorkflowService{
		store:  s,
		policy: NewPolicyEngine(),
		idemp:  NewIdempotencyEngine(),
	}
}

// Store returns the underlying store for read-only convenience
func (s *WorkflowService) Store() *store.Store {
	return s.store
}

// Policy returns the business rules policy engine
func (s *WorkflowService) Policy() *PolicyEngine {
	return s.policy
}
