package build

import (
	"context"
	"fmt"
	"time"

	"tenara/controllers/internal/appenv"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler drives Build objects through their phase lifecycle.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func NewReconciler(mgr ctrl.Manager) *Reconciler {
	return &Reconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}
}

// +kubebuilder:rbac:groups=tenara.io,resources=builds,verbs=get;list;watch;create;update;patch;delete

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var b Build
	if getErr := r.Get(ctx, req.NamespacedName, &b); getErr != nil {
		return ctrl.Result{}, client.IgnoreNotFound(getErr)
	}
	if err := r.maybeBackfillDigest(ctx, &b); err != nil {
		return ctrl.Result{}, err
	}
	return r.reconcilePhase(ctx, &b)
}

// maybeBackfillDigest implements phase-2 digest backfill (P2-C): when a
// labeled Build reaches PUSHED, patch every same-app AppEnv service image
// so renderServices rolls the workload forward — no control plane involved.
func (r *Reconciler) maybeBackfillDigest(ctx context.Context, b *Build) error {
	appID := b.Labels["tenara.io/app-id"]
	if b.Status.Phase != PhasePushed || b.Status.ImageDigest == "" || appID == "" {
		return nil
	}
	var list appenv.AppEnvList
	listErr := r.List(ctx, &list, client.InNamespace(b.Namespace),
		client.MatchingLabels{"tenara.io/app-id": appID})
	if listErr != nil {
		return fmt.Errorf("list appenvs: %w", listErr)
	}
	patched := false
	for i := range list.Items {
		ae := &list.Items[i]
		changed := false
		for si := range ae.Spec.Services {
			if ae.Spec.Services[si].Image != b.Status.ImageDigest {
				ae.Spec.Services[si].Image = b.Status.ImageDigest
				changed = true
			}
		}
		if !changed {
			continue
		}
		if updErr := r.Update(ctx, ae); updErr != nil {
			return fmt.Errorf("backfill %s/%s: %w",
				ae.Namespace, ae.Name, updErr)
		}
		patched = true
	}
	_ = patched // success: renderServices reacts to the spec update
	return nil
}

// reconcilePhase advances the build through its lifecycle one step per pass.
func (r *Reconciler) reconcilePhase(ctx context.Context, b *Build) (reconcile.Result, error) {
	switch b.Status.Phase {
	case "":
		b.Status.Phase = PhaseCreated
		if updateErr := r.Status().Update(ctx, b); updateErr != nil {
			return ctrl.Result{}, fmt.Errorf("set CREATED: %w", updateErr)
		}
		// Pod writes the next phase; poll shortly.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil

	case PhaseCreated, PhaseCloning, PhaseBuilding, PhaseScanning, PhaseSigning:
		// Actual work runs in pod init containers; the controller only polls
		// for the next phase transition written by the pod itself.
		if b.Status.Phase != PhaseCreated && b.Status.Phase != PhaseCloning {
			DeleteEphemeralTokenSecret(ctx, r.Client, b.Namespace, b.Name)
		}
		return ctrl.Result{RequeueAfter: 30 * 1000000000}, nil

	case PhasePushed, PhaseFailed:
		DeleteEphemeralTokenSecret(ctx, r.Client, b.Namespace, b.Name)
		return ctrl.Result{}, nil

	default:
		// Terminal or long-running phases need no further reconciliation.
		return ctrl.Result{}, nil
	}
}

// SetupWithManager registers the Build controller with the manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&Build{}).
		Complete(r)
}
