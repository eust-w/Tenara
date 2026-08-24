package appenv

import (
	"fmt"
)

// IsolationUpgradeReason marks AppEnvs whose isolation cannot be honored
// locally; since todo88 isolated is enforced via RuntimeClass injection and
// only dedicated waits on the dedicated-node tier (todo95).
const IsolationUpgradeReason = "IsolationUpgradeRequired"

// ValidateIsolation enforces the RB§17 enum at reconcile time; the CRD schema
// equivalent follows once codegen lands.
func ValidateIsolation(lvl IsolationLevel) error {
	switch lvl {
	case IsolationShared, IsolationIsolated, IsolationDedicated:
		return nil
	default:
		return fmt.Errorf("invalid isolation %q: want shared|isolated|dedicated", lvl)
	}
}

// RequiresUpgradeNotice reports whether the level still exceeds locally
// enforced guarantees. Isolated has been enforced through sandbox injection
// since todo88; dedicated remains gated on the node-tier work (todo95).
func RequiresUpgradeNotice(lvl IsolationLevel) bool {
	return lvl == IsolationDedicated
}
