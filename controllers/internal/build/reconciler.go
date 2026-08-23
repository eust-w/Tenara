package build

import (
	"context"
	"fmt"

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
	if err := r.Get(ctx, req.NamespacedName, &b); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return r.reconcilePhase(ctx, &b)
}

// reconcilePhase advances the build through its lifecycle one step per pass.
func (r *Reconciler) reconcilePhase(ctx context.Context, b *Build) (reconcile.Result, error) {
	switch b.Status.Phase {
	case "":
		b.Status.Phase = PhaseCreated
		if updateErr := r.Status().Update(ctx, b); updateErr != nil {
			return ctrl.Result{}, fmt.Errorf("set CREATED: %w", updateErr)
		}
		return ctrl.Result{Requeue: true}, nil

	case PhaseCreated, PhaseCloning, PhaseBuilding, PhaseScanning, PhaseSigning:
		// Actual work runs in pod init containers; the controller only polls
		// for the next phase transition written by the pod itself.
		return ctrl.Result{RequeueAfter: 30 * 1000000000}, nil

	case PhasePushed, PhaseFailed:
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
