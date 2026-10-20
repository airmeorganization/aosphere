package mcp

import (
	"errors"
	"regexp"
	"strings"
	"sync"
)

var (
	ErrKillSwitchActive   = errors.New("emergency kill switch is active: all mutating agent tool calls are disabled by administrator")
	ErrPromptInjection    = errors.New("untrusted prompt injection detected in agent tool argument")
	ErrExecutionTimeout   = errors.New("agent tool execution timed out exceeding deadline")
)

// Suspicious prompt injection patterns that attempt to hijack LLM system prompts
var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(ignore\s+(all\s+)?(previous|prior|above)\s+instructions?)\b`),
	regexp.MustCompile(`(?i)\b(system\s+override)\b`),
	regexp.MustCompile(`(?i)<\s*\|\s*(system|im_start|im_end)\s*\|?\s*>`),
	regexp.MustCompile(`(?i)\[\s*(system|inst)\s*\]`),
	regexp.MustCompile(`(?i)###\s*(instruction|system)\b`),
}

// SanitizePromptInput cleanses input and checks for prompt injection attacks
func SanitizePromptInput(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	for _, pattern := range injectionPatterns {
		if pattern.MatchString(trimmed) {
			return trimmed, ErrPromptInjection
		}
	}
	return trimmed, nil
}

// KillSwitchManager coordinates the emergency stop mechanism across tenants
type KillSwitchManager struct {
	mu           sync.RWMutex
	globalActive bool
	orgSwitches  map[string]bool
}

var globalKillSwitch = &KillSwitchManager{
	orgSwitches: make(map[string]bool),
}

// DefaultKillSwitch returns the singleton kill switch manager
func DefaultKillSwitch() *KillSwitchManager {
	return globalKillSwitch
}

// IsActive checks if the kill switch is engaged globally or for the specific org
func (k *KillSwitchManager) IsActive(orgID string) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.globalActive {
		return true
	}
	if orgID != "" && k.orgSwitches[orgID] {
		return true
	}
	return false
}

// SetGlobal sets the global kill switch status
func (k *KillSwitchManager) SetGlobal(active bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.globalActive = active
}

// SetOrg sets the kill switch status for an individual organization
func (k *KillSwitchManager) SetOrg(orgID string, active bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if orgID != "" {
		k.orgSwitches[orgID] = active
	}
}

// Reset clears all kill switch locks
func (k *KillSwitchManager) Reset() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.globalActive = false
	k.orgSwitches = make(map[string]bool)
}
