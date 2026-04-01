package service

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	AccountRuleMatchModeAny = "any"
	AccountRuleMatchModeAll = "all"

	defaultAccountRuleForwardMaxAttempts = 3
)

type AccountRuleBinding struct {
	ID                int64     `json:"id"`
	Platform          string    `json:"platform"`
	BusinessType      string    `json:"business_type"`
	Enabled           bool      `json:"enabled"`
	ModelCollectionID *int64    `json:"model_collection_id,omitempty"`
	ErrorCollectionID *int64    `json:"error_collection_id,omitempty"`
	Description       string    `json:"description"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type AccountRuleModelCollection struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Models      []string  `json:"models"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AccountRuleErrorCollection struct {
	ID          int64                   `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
	Rules       []*AccountRuleErrorRule `json:"rules"`
}

type AccountRuleErrorRule struct {
	ID                int64     `json:"id"`
	ErrorCollectionID int64     `json:"error_collection_id"`
	Name              string    `json:"name"`
	Enabled           bool      `json:"enabled"`
	Priority          int       `json:"priority"`
	StatusCodes       []int     `json:"status_codes"`
	Keywords          []string  `json:"keywords"`
	MatchMode         string    `json:"match_mode"`
	ActionDisable     bool      `json:"action_disable"`
	ActionFailover    bool      `json:"action_failover"`
	ActionDelete      bool      `json:"action_delete"`
	ActionOverride    bool      `json:"action_override"`
	PassthroughCode   bool      `json:"passthrough_code"`
	ResponseCode      *int      `json:"response_code"`
	PassthroughBody   bool      `json:"passthrough_body"`
	CustomMessage     *string   `json:"custom_message"`
	SkipMonitoring    bool      `json:"skip_monitoring"`
	Description       string    `json:"description"`
	SampleResponse    string    `json:"sample_response"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type AccountRuleObservedBinding struct {
	Platform     string `json:"platform"`
	BusinessType string `json:"business_type"`
	AccountCount int64  `json:"account_count"`
}

type AccountRuleSettings struct {
	ForwardMaxAttempts int  `json:"forward_max_attempts"`
	FailoverOn429      bool `json:"failover_on_429"`
}

type AccountRuleCatalog struct {
	Bindings         []*AccountRuleBinding         `json:"bindings"`
	ModelCollections []*AccountRuleModelCollection `json:"model_collections"`
	ErrorCollections []*AccountRuleErrorCollection `json:"error_collections"`
	ObservedBindings []*AccountRuleObservedBinding `json:"observed_bindings"`
	Settings         AccountRuleSettings           `json:"settings"`
}

type AccountRuleDraft struct {
	Platform                 string                `json:"platform"`
	BusinessType             string                `json:"business_type"`
	MatchedBindingID         *int64                `json:"matched_binding_id,omitempty"`
	MatchedErrorCollectionID *int64                `json:"matched_error_collection_id,omitempty"`
	AccountID                *int64                `json:"account_id,omitempty"`
	AccountName              string                `json:"account_name,omitempty"`
	Rule                     *AccountRuleErrorRule `json:"rule"`
}

type AccountRuleMatch struct {
	Binding *AccountRuleBinding
	Rule    *AccountRuleErrorRule
}

type AccountRuleActionResult struct {
	Matched            bool
	ShouldFailover     bool
	MaxForwardAttempts int
	SkipMonitoring     bool
	Rule               *AccountRuleErrorRule
}

func normalizeAccountRulePlatform(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func normalizeAccountRuleType(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func normalizeAccountRuleTextList(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func normalizeAccountRuleStatusCodes(values []int) []int {
	if len(values) == 0 {
		return []int{}
	}
	seen := make(map[int]struct{}, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value < 0 || value > 999 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func (b *AccountRuleBinding) Normalize() {
	if b == nil {
		return
	}
	b.Platform = normalizeAccountRulePlatform(b.Platform)
	b.BusinessType = normalizeAccountRuleType(b.BusinessType)
	b.Description = strings.TrimSpace(b.Description)
}

func (b *AccountRuleBinding) Validate() error {
	if b == nil {
		return fmt.Errorf("binding is required")
	}
	b.Normalize()
	if b.Platform == "" {
		return fmt.Errorf("platform is required")
	}
	return nil
}

func (c *AccountRuleModelCollection) Normalize() {
	if c == nil {
		return
	}
	c.Name = strings.TrimSpace(c.Name)
	c.Description = strings.TrimSpace(c.Description)
	c.Models = normalizeAccountRuleTextList(c.Models)
}

func (c *AccountRuleModelCollection) Validate() error {
	if c == nil {
		return fmt.Errorf("model collection is required")
	}
	c.Normalize()
	if c.Name == "" {
		return fmt.Errorf("model collection name is required")
	}
	return nil
}

func (c *AccountRuleErrorCollection) Normalize() {
	if c == nil {
		return
	}
	c.Name = strings.TrimSpace(c.Name)
	c.Description = strings.TrimSpace(c.Description)
}

func (c *AccountRuleErrorCollection) Validate() error {
	if c == nil {
		return fmt.Errorf("error collection is required")
	}
	c.Normalize()
	if c.Name == "" {
		return fmt.Errorf("error collection name is required")
	}
	return nil
}

func (r *AccountRuleErrorRule) Normalize() {
	if r == nil {
		return
	}
	r.Name = strings.TrimSpace(r.Name)
	r.Priority = max(r.Priority, 0)
	r.StatusCodes = normalizeAccountRuleStatusCodes(r.StatusCodes)
	r.Keywords = normalizeAccountRuleTextList(r.Keywords)
	r.MatchMode = strings.ToLower(strings.TrimSpace(r.MatchMode))
	if r.MatchMode == "" {
		r.MatchMode = AccountRuleMatchModeAny
	}
	r.Description = strings.TrimSpace(r.Description)
	r.SampleResponse = strings.TrimSpace(r.SampleResponse)
	if r.CustomMessage != nil {
		msg := strings.TrimSpace(*r.CustomMessage)
		r.CustomMessage = &msg
	}
}

func (r *AccountRuleErrorRule) Validate() error {
	if r == nil {
		return fmt.Errorf("rule is required")
	}
	r.Normalize()
	if r.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if len(r.StatusCodes) == 0 && len(r.Keywords) == 0 {
		return fmt.Errorf("at least one status code or keyword is required")
	}
	if r.MatchMode != AccountRuleMatchModeAny && r.MatchMode != AccountRuleMatchModeAll {
		return fmt.Errorf("match_mode must be any or all")
	}
	if !r.ActionDisable && !r.ActionFailover && !r.ActionDelete && !r.ActionOverride {
		return fmt.Errorf("at least one action must be enabled")
	}
	if r.ActionOverride {
		if !r.PassthroughCode && (r.ResponseCode == nil || *r.ResponseCode <= 0 || *r.ResponseCode > http.StatusNetworkAuthenticationRequired) {
			return fmt.Errorf("response_code is required when override response code passthrough is disabled")
		}
		if !r.PassthroughBody {
			if r.CustomMessage == nil || strings.TrimSpace(*r.CustomMessage) == "" {
				return fmt.Errorf("custom_message is required when override response body passthrough is disabled")
			}
		}
	}
	return nil
}
