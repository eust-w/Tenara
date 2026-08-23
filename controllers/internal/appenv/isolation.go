package appenv

import (
	"fmt"
)

// IsolationUpgradeReason marks AppEnvs requesting sandboxing beyond the MVP
// shared pool; real enforcement lands with P2 (todo88).
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

// RequiresUpgradeNotice reports whether the requested level exceeds MVP
// shared semantics and therefore must surface an explicit event instead of
// silently pretending stronger isolation exists locally.
func RequiresUpgradeNotice(lvl IsolationLevel) bool {
	return lvl == IsolationIsolated || lvl == IsolationDedicated
}
