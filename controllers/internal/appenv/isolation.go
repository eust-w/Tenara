package appenv

import (
	"fmt"
)

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
