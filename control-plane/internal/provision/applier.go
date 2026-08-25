// Package provision materializes platform decisions onto the target cluster.
// Phase 1 ships a kubectl-exec applier so the control plane stays free of
// client-go dependencies; production swaps in a dynamic-client (in-cluster
// SA) implementation behind the same Applier interface.
package provision

import (
	"bytes"
	"context"
	"os/exec"
)

// Object is the minimal manifest contract accepted by Applier. Callers hand
// over an already-decoded JSON document (map form) carrying apiVersion/kind/
// metadata/spec.
type Object = map[string]any

// Applier performs idempotent server-side apply of one manifest.
type Applier interface {
	Apply(ctx context.Context, obj Object) error
}

// KubectlApplier shells out to `kubectl apply -f -` using the process
// environment (KUBECONFIG / in-cluster config when running inside a pod).
type KubectlApplier struct {
	// Binary overrides the kubectl path (tests); empty uses "kubectl".
	Binary string
}

// Apply pipes the serialized manifest through kubectl apply.
func (k KubectlApplier) Apply(ctx context.Context, obj Object) error {
	bin := k.Binary
	if bin == "" {
		bin = "kubectl"
	}
	manifest, err := marshal(obj)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "apply", "-f", "-")
	cmd.Stdin = bytes.NewReader(manifest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmtWrapped(string(out), err)
	}
	return nil
}
