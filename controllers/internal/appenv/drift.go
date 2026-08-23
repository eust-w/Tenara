package appenv

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// DriftAction describes one self-healing step derived from comparing the
// rendered spec against live state (RB§34).
type DriftAction struct {
	Name    string
	Kind    string
	Field   string
	Desired string
}

// DriftReport bundles actions with the controller-owned audit trail entry.
type DriftReport struct {
	AuditedAt time.Time
	Actions   []DriftAction
	AppID     string
	Namespace string
	ActorType string
}

// OwnedByUs guards self-healing: only Tenara-managed objects may ever be
// mutated by drift repair; third-party resources are never touched.
func OwnedByUs(labels map[string]string) bool {
	return labels[LabelManagedBy] == LabelManagedVal
}

// DetectReplicaDrift reports divergence and the replica count to restore.
func DetectReplicaDrift(desired, live int32) (bool, int32) {
	if live != desired {
		return true, desired
	}
	return false, desired
}

func eqBool(a, b *bool) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	return a == nil || *a == *b
}

func capsDropEqual(d, l *corev1.Capabilities) bool {
	if d == nil || l == nil {
		return d == nil && l == nil
	}
	if len(d.Drop) != len(l.Drop) {
		return false
	}
	for i := range d.Drop {
		if d.Drop[i] != l.Drop[i] {
			return false
		}
	}
	return true
}

func seccompEqual(d, l *corev1.SeccompProfile) bool {
	if d == nil || l == nil {
		return d == nil && l == nil
	}
	return d.Type == l.Type
}

// SecurityDrift lists the hardened fields that diverge between the rendered
// container spec and live state; an empty slice means fully compliant.
func SecurityDrift(desired, live *corev1.SecurityContext) []string {
	if desired == nil || live == nil {
		return []string{"securityContext"}
	}
	var drifted []string
	add := func(field string, same bool) {
		if !same {
			drifted = append(drifted, field)
		}
	}
	add("runAsNonRoot", eqBool(desired.RunAsNonRoot, live.RunAsNonRoot))
	add("allowPrivilegeEscalation", eqBool(desired.AllowPrivilegeEscalation, live.AllowPrivilegeEscalation))
	add("readOnlyRootFilesystem", eqBool(desired.ReadOnlyRootFilesystem, live.ReadOnlyRootFilesystem))
	add("capabilities.drop", capsDropEqual(desired.Capabilities, live.Capabilities))
	add("seccompProfile", seccompEqual(desired.SeccompProfile, live.SeccompProfile))
	return drifted
}

// MissingNetPols returns which expected policies have no live counterpart
// and therefore need recreation.
func MissingNetPols(expected []string, liveNames map[string]bool) []string {
	var missing []string
	for _, name := range expected {
		if !liveNames[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

// ReportDrift stamps the action list with controller audit metadata.
func ReportDrift(appID, namespace string, actions []DriftAction, at time.Time) DriftReport {
	return DriftReport{
		Actions:   actions,
		AppID:     appID,
		Namespace: namespace,
		AuditedAt: at,
		ActorType: "controller",
	}
}
