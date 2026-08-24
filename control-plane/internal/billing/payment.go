package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
)

// ErrUnknownTier rejects tiers outside the pricing catalog.
var ErrUnknownTier = errors.New("unknown billing tier")

// PaymentProvider abstracts stripe-style checkout and callbacks; domestic
// channels implement the same two calls (design-doc interface slot).
type PaymentProvider interface {
	CreateCheckout(ctx context.Context, orgID, tier string) (string, error)
	HandleWebhook(ctx context.Context, payload []byte) (TierChange, error)
}

// TierChange is the normalized upgrade/downgrade event.
type TierChange struct {
	OrgID string `json:"org_id"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// MockProvider implements the provider face offline; it doubles as the
// dry-run channel exercising tier-switch flows locally.
type MockProvider struct{}

// CreateCheckout mints a deterministic reference without network calls.
func (MockProvider) CreateCheckout(_ context.Context, orgID, tier string) (string, error) {
	if _, pErr := PlanFor(tier); pErr != nil {
		return "", pErr
	}
	return "mock-checkout-" + orgID + "-" + tier, nil
}

// HandleWebhook validates and normalizes a tier-change callback.
func (MockProvider) HandleWebhook(_ context.Context, payload []byte) (TierChange, error) {
	var ch TierChange
	if uErr := json.Unmarshal(payload, &ch); uErr != nil {
		return TierChange{}, fmt.Errorf("decode tier change: %w", uErr)
	}
	if ch.OrgID == "" {
		return TierChange{}, errors.New("org_id required")
	}
	if ch.From == "" {
		ch.From = TierFree
	}
	if _, pErr := PlanFor(ch.To); pErr != nil {
		return TierChange{}, pErr
	}
	return ch, nil
}

// TierStore publishes the effective tier atomically so quota readers observe
// webhook-applied changes immediately (acceptance: 立即读新值).
type TierStore struct{ current atomic.Value }

// NewTierStore seeds the initial effective tier.
func NewTierStore(initial string) *TierStore {
	s := &TierStore{}
	s.current.Store(initial)
	return s
}

// Get returns the currently effective tier; an unset store reads as free.
func (s *TierStore) Get() string {
	v, ok := s.current.Load().(string)
	if !ok {
		return TierFree
	}
	return v
}

// Set flips the effective tier.
func (s *TierStore) Set(tier string) { s.current.Store(tier) }
