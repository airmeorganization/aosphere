package agentplatform

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"aosphere/internal/db"
	"aosphere/internal/store"
	"github.com/google/uuid"
)

// WebhookDispatcher manages webhook subscriptions, event publication, and HMAC-signed delivery
type WebhookDispatcher struct {
	store      *store.Store
	httpClient *http.Client
	mu         sync.RWMutex
}

func NewWebhookDispatcher(s *store.Store) *WebhookDispatcher {
	return &WebhookDispatcher{
		store: s,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ComputeSignature calculates HMAC-SHA256 of timestamp + "." + payload
func ComputeSignature(secret string, timestamp int64, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	msg := fmt.Sprintf("%d.%s", timestamp, payload)
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature validates a received webhook signature
func VerifySignature(secret string, timestamp int64, payload, receivedSig string) bool {
	expected := ComputeSignature(secret, timestamp, payload)
	return hmac.Equal([]byte(expected), []byte(receivedSig))
}

// DispatchEvent distributes a business event to all matching webhook subscriptions
func (wd *WebhookDispatcher) DispatchEvent(ctx context.Context, orgID, eventType string, payload interface{}) ([]db.WebhookDelivery, error) {
	if orgID == "" {
		orgID = "org_default"
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed marshaling webhook payload: %w", err)
	}
	payloadStr := string(payloadBytes)

	// Fetch active subscriptions
	subs, err := wd.store.GetWebhookSubscriptions(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed fetching subscriptions: %w", err)
	}

	eventID := "evt_" + uuid.New().String()[:12]
	var deliveries []db.WebhookDelivery

	for _, sub := range subs {
		if !sub.IsActive {
			continue
		}
		// Match event type
		matches := false
		if sub.EventTypes == "" || sub.EventTypes == `["*"]` || strings.Contains(sub.EventTypes, "*") {
			matches = true
		} else if strings.Contains(sub.EventTypes, eventType) {
			matches = true
		}

		if !matches {
			continue
		}

		deliv := db.WebhookDelivery{
			ID:             "wdel_" + uuid.New().String()[:12],
			OrgID:          orgID,
			SubscriptionID: sub.ID,
			EventID:        eventID,
			EventType:      eventType,
			PayloadJSON:    payloadStr,
			Status:         "pending",
			MaxRetries:     3,
		}

		createdDeliv, err := wd.store.CreateWebhookDelivery(deliv)
		if err != nil {
			continue
		}

		// Perform delivery attempt
		wd.deliver(ctx, sub, createdDeliv)
		deliveries = append(deliveries, *createdDeliv)
	}

	return deliveries, nil
}

// deliver executes a single delivery attempt with HMAC signature and exponential backoff scheduling
func (wd *WebhookDispatcher) deliver(ctx context.Context, sub db.WebhookSubscription, deliv *db.WebhookDelivery) {
	timestamp := time.Now().Unix()
	signature := ComputeSignature(sub.Secret, timestamp, deliv.PayloadJSON)

	req, err := http.NewRequestWithContext(ctx, "POST", sub.URL, bytes.NewBufferString(deliv.PayloadJSON))
	if err != nil {
		nextRetry := time.Now().Add(1 * time.Minute)
		_ = wd.store.UpdateWebhookDeliveryStatus(deliv.ID, "failed", 0, err.Error(), &nextRetry)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AOSphere-Webhook-Dispatcher/1.0")
	req.Header.Set("X-Aosphere-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-Aosphere-Signature", fmt.Sprintf("sha256=%s", signature))
	req.Header.Set("X-Aosphere-Event", deliv.EventType)
	req.Header.Set("X-Aosphere-Delivery", deliv.ID)

	resp, err := wd.httpClient.Do(req)
	if err != nil {
		// Schedule exponential backoff retry: 1m, 5m, 15m
		backoffMinutes := []int{1, 5, 15}
		var nextRetry *time.Time
		status := "pending"
		if deliv.RetryCount+1 >= deliv.MaxRetries {
			status = "failed"
		} else {
			retryIdx := deliv.RetryCount
			if retryIdx >= len(backoffMinutes) {
				retryIdx = len(backoffMinutes) - 1
			}
			t := time.Now().Add(time.Duration(backoffMinutes[retryIdx]) * time.Minute)
			nextRetry = &t
		}
		_ = wd.store.UpdateWebhookDeliveryStatus(deliv.ID, status, 0, err.Error(), nextRetry)
		return
	}
	defer resp.Body.Close()

	respBodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	respBody := string(respBodyBytes)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_ = wd.store.UpdateWebhookDeliveryStatus(deliv.ID, "delivered", resp.StatusCode, respBody, nil)
	} else {
		status := "pending"
		var nextRetry *time.Time
		if deliv.RetryCount+1 >= deliv.MaxRetries {
			status = "failed"
		} else {
			t := time.Now().Add(2 * time.Minute)
			nextRetry = &t
		}
		_ = wd.store.UpdateWebhookDeliveryStatus(deliv.ID, status, resp.StatusCode, respBody, nextRetry)
	}
}

// Redeliver manually re-attempts delivery for a given delivery ID
func (wd *WebhookDispatcher) Redeliver(ctx context.Context, deliveryID string) (*db.WebhookDelivery, error) {
	deliv, err := wd.store.GetWebhookDelivery(deliveryID)
	if err != nil {
		return nil, fmt.Errorf("delivery record not found: %w", err)
	}
	sub, err := wd.store.GetWebhookSubscription(deliv.SubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	wd.deliver(ctx, *sub, deliv)
	return wd.store.GetWebhookDelivery(deliveryID)
}

// ----------------- PROMPT INJECTION DEFENSE & SANITIZATION -----------------

type InjectionPattern struct {
	Name    string
	Pattern *regexp.Regexp
	Weight  float64
}

var injectionRules = []InjectionPattern{
	{
		Name:    "instruction_reset",
		Pattern: regexp.MustCompile(`(?i)\b(ignore\s+(all\s+)?(previous|prior|above)\s+instructions?)\b`),
		Weight:  0.9,
	},
	{
		Name:    "system_override",
		Pattern: regexp.MustCompile(`(?i)\b(system\s+override|override\s+system\s+prompt)\b`),
		Weight:  0.95,
	},
	{
		Name:    "role_play_jailbreak",
		Pattern: regexp.MustCompile(`(?i)\b(you\s+are\s+now\s+in\s+developer\s+mode|dan\s+mode|jailbreak|unfiltered\s+mode)\b`),
		Weight:  0.85,
	},
	{
		Name:    "special_token_smuggling",
		Pattern: regexp.MustCompile(`(?i)<\s*\|\s*(system|im_start|im_end|assistant)\s*\|?\s*>`),
		Weight:  0.99,
	},
	{
		Name:    "markdown_role_hijack",
		Pattern: regexp.MustCompile("(?i)\\x60{3}(system|instruction|prompt)\\b"),
		Weight:  0.8,
	},
	{
		Name:    "bracketed_instruction",
		Pattern: regexp.MustCompile(`(?i)\[\s*(system|inst|instruction)\s*\]`),
		Weight:  0.75,
	},
}

// SanitizeInput inspects untrusted strings, calculates injection risk, and neutralizes hazardous patterns
func SanitizeInput(input string) db.SanitizeResult {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return db.SanitizeResult{
			IsSafe:          true,
			RiskScore:       0.0,
			FlaggedPatterns: []string{},
			SanitizedText:   "",
		}
	}

	var flagged []string
	maxWeight := 0.0
	sanitized := trimmed

	for _, rule := range injectionRules {
		if rule.Pattern.MatchString(sanitized) {
			flagged = append(flagged, rule.Name)
			if rule.Weight > maxWeight {
				maxWeight = rule.Weight
			}
			// Neutralize matching tokens
			sanitized = rule.Pattern.ReplaceAllStringFunc(sanitized, func(match string) string {
				return "[FILTERED_INJECTION: " + rule.Name + "]"
			})
		}
	}

	isSafe := len(flagged) == 0 && maxWeight < 0.7

	return db.SanitizeResult{
		IsSafe:          isSafe,
		RiskScore:       maxWeight,
		FlaggedPatterns: flagged,
		SanitizedText:   sanitized,
	}
}

// ----------------- AGENT EXECUTION BUDGET & RATE QUOTA MANAGER -----------------

type AgentBudgetManager struct {
	store     *store.Store
	mu        sync.Mutex
	history   map[string][]time.Time
	inFlight  map[string]int
}

func NewAgentBudgetManager(s *store.Store) *AgentBudgetManager {
	return &AgentBudgetManager{
		store:    s,
		history:  make(map[string][]time.Time),
		inFlight: make(map[string]int),
	}
}

// CheckAndAcquire checks rate quota, concurrency, and daily budget for an agent
func (ab *AgentBudgetManager) CheckAndAcquire(ctx context.Context, agentKeyOrHash string) (func(), error) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	// 1. Fetch agent configuration
	cfg, err := ab.store.GetAgentConfigByHash(agentKeyOrHash)
	if err != nil || cfg == nil {
		// Fallback check by ID
		cfg, _ = ab.store.GetAgentConfigByID(agentKeyOrHash)
	}

	quotaPerMin := 60
	maxConcurrent := 5
	dailyBudget := 5000
	currentUsage := 0
	agentID := agentKeyOrHash

	if cfg != nil {
		agentID = cfg.ID
		quotaPerMin = cfg.ToolQuotaPerMinute
		maxConcurrent = cfg.ConcurrentLimit
		dailyBudget = cfg.DailyBudgetCents
		currentUsage = cfg.CurrentDayUsageCents
		if !cfg.IsActive {
			return nil, fmt.Errorf("agent %s is deactivated by administrator", cfg.Name)
		}
	}

	// 2. Daily Budget check
	if dailyBudget > 0 && currentUsage >= dailyBudget {
		return nil, fmt.Errorf("rate limit exceeded: daily budget cap reached (%d / %d cents) [-32029]", currentUsage, dailyBudget)
	}

	// 3. Sliding 1-minute window check
	now := time.Now()
	oneMinAgo := now.Add(-1 * time.Minute)
	var recent []time.Time
	for _, t := range ab.history[agentID] {
		if t.After(oneMinAgo) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= quotaPerMin {
		ab.history[agentID] = recent
		return nil, fmt.Errorf("rate limit exceeded: max requests per minute reached (%d / %d) [-32029]", len(recent), quotaPerMin)
	}

	// 4. Concurrency slot check
	active := ab.inFlight[agentID]
	if active >= maxConcurrent {
		return nil, fmt.Errorf("rate limit exceeded: concurrent execution limit reached (%d / %d) [-32029]", active, maxConcurrent)
	}

	// Reserve slot
	recent = append(recent, now)
	ab.history[agentID] = recent
	ab.inFlight[agentID] = active + 1

	released := false
	release := func() {
		ab.mu.Lock()
		defer ab.mu.Unlock()
		if !released {
			ab.inFlight[agentID]--
			if ab.inFlight[agentID] < 0 {
				ab.inFlight[agentID] = 0
			}
			released = true
		}
	}

	return release, nil
}
