package appenv

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler materializes one tenant namespace per AppEnv object.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func NewReconciler(mgr ctrl.Manager) *Reconciler {
	return &Reconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}
}

// SetupWithManager registers the AppEnv controller with the manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&AppEnv{}).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var ae AppEnv
	if getErr := r.Get(ctx, req.NamespacedName, &ae); getErr != nil {
		return ctrl.Result{}, client.IgnoreNotFound(getErr)
	}
	return r.ensureNamespace(ctx, &ae)
}

// ensureNamespace creates the tenant namespace owned by the AppEnv (cascade
// delete) or adopts an existing one only when Tenara-managed.
func (r *Reconciler) ensureNamespace(ctx context.Context, ae *AppEnv) (reconcile.Result, error) {
	name := NamespaceName(ae.Spec.AppID, ae.Spec.Env)

	var ns corev1.Namespace
	getErr := r.Get(ctx, client.ObjectKey{Name: name}, &ns)
	switch {
	case client.IgnoreNotFound(getErr) != nil:
		return ctrl.Result{}, fmt.Errorf("get namespace %s: %w", name, getErr)

	case getErr != nil:
		desired := DesiredNamespace(ae.Spec.AppID, ae.Spec.Env)
		desired.OwnerReferences = append(desired.OwnerReferences, metav1.OwnerReference{
			APIVersion: APIVersion,
			Kind:       Kind,
			Name:       ae.Name,
			UID:        ae.UID,
			Controller: boolPtr(true),
		})
		if createErr := r.Create(ctx, desired); createErr != nil {
			return ctrl.Result{}, fmt.Errorf("create namespace %s: %w", name, createErr)
		}

	default:
		if adoptErr := EnsurePlatformOwned(&ns); adoptErr != nil {
			return ctrl.Result{}, adoptErr
		}
	}

	if ae.Status.Namespace != name {
		ae.Status.Namespace = name
		ae.Status.Phase = "READY"
		if updErr := r.Status().Update(ctx, ae); updErr != nil {
			return ctrl.Result{}, fmt.Errorf("update status: %w", updErr)
		}
	}
	return ctrl.Result{}, nil
}

func boolPtr(b bool) *bool { return &b }
