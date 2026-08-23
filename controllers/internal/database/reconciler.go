package database

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler drives DatabaseBinding objects through the credential chain.
// Failures return errors so controller-runtime applies exponential backoff;
// admin credentials only ever live inside provider processes.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Deps   Deps
}

func NewReconciler(mgr ctrl.Manager, deps Deps) *Reconciler {
	return &Reconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Deps: deps}
}

// SetupWithManager registers the DatabaseBinding controller.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&DatabaseBinding{}).Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var b DatabaseBinding
	if getErr := r.Get(ctx, req.NamespacedName, &b); getErr != nil {
		return ctrl.Result{}, client.IgnoreNotFound(getErr)
	}
	if b.Status.Phase == phaseReady {
		return ctrl.Result{}, nil
	}

	if b.Status.Phase != phasePending {
		b.Status.Phase = phasePending
		if updErr := r.Status().Update(ctx, &b); updErr != nil {
			return ctrl.Result{}, fmt.Errorf("set pending: %w", updErr)
		}
	}

	out, pErr := ProvisionBinding(ctx, r.Deps, b.Spec)
	if pErr != nil {
		b.Status.Phase = phasePending
		b.Status.Reason = "provision-failed"
		b.Status.Message = pErr.Error()
		if updErr := r.Status().Update(ctx, &b); updErr != nil {
			return ctrl.Result{}, fmt.Errorf("record failure: %w", updErr)
		}
		return ctrl.Result{}, pErr
	}

	b.Status.Phase = phaseReady
	b.Status.Reason = ""
	b.Status.Message = out.Namespace + "/" + out.SecretName + "/" + out.SecretKey
	if updErr := r.Status().Update(ctx, &b); updErr != nil {
		return ctrl.Result{}, fmt.Errorf("set ready: %w", updErr)
	}
	return ctrl.Result{}, nil
}
